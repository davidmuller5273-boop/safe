package safewbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"
)

// help text must NOT include docs/使用教程 content.
func helpForRole(role string) string {
	common := `/help — 显示你当前权限可见的菜单
/whoami — 查看 user_id / chat_id / 角色
/pushstatus — 查看本群推送是否开启`
	groupAdmin := `
广告（本群）：
/setprefix [group] <文本>
/setsuffix [group] <文本>
/clearprefix [group]
/clearsuffix [group]
/adstatus
/resetads group
/say <文本>`
	super := `
推送（开发者 / 超级管理员）：
/push on|off [chat_id] — 开启或关闭群推送（进群默认关闭）
/broadcast <文本> — 向「已开启推送」的群发送

超级管理员管理（仅开发者）：
/addsuperadmin <user_id>
/removesuperadmin <user_id>
/listsuperadmins

群管理员（开发者 / 超级管理员）：
/addgroupadmin <user_id> [chat_id]
/removegroupadmin <user_id> [chat_id]
/listgroupadmins [chat_id]`
	adsGlobal := `
全局广告（开发者 / 超级管理员）：
/setprefix global <文本>
/setsuffix global <文本>
/clearprefix global
/clearsuffix global`
	switch role {
	case botperm.RoleDeveloper:
		return "【开发者菜单】\n" + common + groupAdmin + adsGlobal + super + "\n\n说明：机器人进群后默认不推送；对外正文一次 sendMessage 发送。"
	case botperm.RoleAdmin:
		return "【超级管理员菜单】\n" + common + groupAdmin + adsGlobal + super + "\n\n说明：进群默认不推送；不能添加/移除其他超级管理员。"
	case botperm.RoleGroupAdmin:
		return "【群管理员菜单】\n" + common + groupAdmin + "\n\n说明：仅可管理本群广告与 /say；不能开关推送。"
	default:
		return "【普通用户菜单】\n" + common + "\n\n无更多权限请联系开发者或超级管理员。"
	}
}

func (w worker) runCommands(ctx context.Context) error {
	var offset int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		botConfig, err := systemconfig.LoadSafeW(w.db)
		if err != nil {
			log.Printf("命令轮询等待 SafeW 配置: %v", err)
			if !wait(ctx, 5*time.Second) {
				return nil
			}
			continue
		}
		updates, err := w.client.GetUpdates(ctx, botConfig.Token, offset, 50, 25)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("getUpdates 失败: %v", err)
			if !wait(ctx, 3*time.Second) {
				return nil
			}
			continue
		}
		for _, update := range updates {
			if int64(update.UpdateID)+1 > offset {
				offset = int64(update.UpdateID) + 1
			}
			if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
				continue
			}
			if err := w.handleCommand(ctx, botConfig, update.Message); err != nil {
				log.Printf("处理命令失败: %v", err)
			}
		}
	}
}

func (w worker) handleCommand(ctx context.Context, botConfig systemconfig.SafeW, msg *safew.Message) error {
	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(text, "/") {
		return nil
	}
	parts := strings.Fields(text)
	cmd := strings.ToLower(parts[0])
	if i := strings.Index(cmd, "@"); i > 0 {
		cmd = cmd[:i]
	}
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	}
	userID := ""
	if msg.From != nil {
		userID = msg.From.IDString()
	}
	chatID := msg.Chat.IDString()

	// First time we see a chat/group: register with push OFF by default.
	_ = w.perms.EnsureGroup(chatID)

	role, err := w.perms.RoleInChat(userID, chatID)
	if err != nil {
		return err
	}

	switch cmd {
	case "/help", "/start":
		return w.replyPlain(ctx, botConfig.Token, chatID, helpForRole(role))
	case "/whoami":
		pushOn, _ := w.perms.IsPushEnabled(chatID)
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf(
			"user_id=%s\nchat_id=%s\n角色=%s (%s)\n本群推送=%v",
			userID, chatID, botperm.RoleLabel(role), roleOrNone(role), pushOn,
		))
	case "/pushstatus":
		pushOn, err := w.perms.IsPushEnabled(chatID)
		if err != nil {
			return err
		}
		state := "关闭（默认）"
		if pushOn {
			state = "已开启"
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "本群推送："+state)
	case "/push":
		ok, err := w.perms.CanTogglePush(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
		}
		fields := strings.Fields(args)
		if len(fields) == 0 {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /push on|off [chat_id]")
		}
		action := strings.ToLower(fields[0])
		targetChat := chatID
		if len(fields) > 1 {
			targetChat = fields[1]
		}
		enabled := action == "on" || action == "enable" || action == "1" || action == "开启"
		if action == "off" || action == "disable" || action == "0" || action == "关闭" {
			enabled = false
		} else if !enabled && action != "on" && action != "enable" && action != "1" && action != "开启" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /push on|off [chat_id]")
		}
		if err := w.perms.SetPushEnabled(targetChat, enabled); err != nil {
			return err
		}
		if enabled {
			return w.replyPlain(ctx, botConfig.Token, chatID, "已开启推送: "+targetChat)
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已关闭推送: "+targetChat)
	case "/adstatus":
		ok, err := w.perms.CanControlSensitive(userID, chatID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
		}
		s, err := ads.LoadForChat(w.db, chatID)
		if err != nil {
			return err
		}
		body := fmt.Sprintf("scope=%s\nprefix:\n%s\n\nsuffix:\n%s", s.Scope, emptyMark(s.PrefixAd), emptyMark(s.SuffixAd))
		return w.reply(ctx, botConfig.Token, chatID, body)
	case "/setprefix":
		return w.cmdSetAd(ctx, botConfig, userID, chatID, args, true)
	case "/setsuffix":
		return w.cmdSetAd(ctx, botConfig, userID, chatID, args, false)
	case "/clearprefix":
		return w.cmdClearAd(ctx, botConfig, userID, chatID, args, true)
	case "/clearsuffix":
		return w.cmdClearAd(ctx, botConfig, userID, chatID, args, false)
	case "/resetads":
		ok, err := w.perms.CanControlSensitive(userID, chatID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
		}
		scope := strings.ToLower(firstToken(args))
		if scope == "" {
			scope = "group"
		}
		if scope != "group" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /resetads group")
		}
		if err := ads.ClearGroup(w.db, chatID); err != nil {
			return err
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已删除本群广告覆盖，将回落全局配置")
	case "/say":
		ok, err := w.perms.CanControlSensitive(userID, chatID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
		}
		if strings.TrimSpace(args) == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /say <文本>")
		}
		return w.reply(ctx, botConfig.Token, chatID, args)
	case "/broadcast":
		ok, err := w.perms.IsAdmin(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
		}
		if strings.TrimSpace(args) == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /broadcast <文本>")
		}
		ids, err := w.perms.ListPushEnabledChatIDs()
		if err != nil {
			return err
		}
		n := 0
		for _, id := range ids {
			if err := w.reply(ctx, botConfig.Token, id, args); err != nil {
				return err
			}
			n++
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已广播到 %d 个已开启推送的群", n))
	case "/addadmin", "/addsuperadmin":
		if !w.perms.CanManageAdmins(userID) {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（仅开发者）")
		}
		target := firstToken(args)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /addsuperadmin <user_id>")
		}
		if err := w.perms.AddAdmin(target, ""); err != nil {
			return w.replyPlain(ctx, botConfig.Token, chatID, err.Error())
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已添加超级管理员: "+target)
	case "/removeadmin", "/removesuperadmin":
		if !w.perms.CanManageAdmins(userID) {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（仅开发者）")
		}
		target := firstToken(args)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /removesuperadmin <user_id>")
		}
		if err := w.perms.RemoveAdmin(target); err != nil {
			return err
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已移除超级管理员: "+target)
	case "/addgroupadmin":
		ok, err := w.perms.CanManageGroupAdmins(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
		}
		target, scopeChat := parseUserAndChat(args, chatID)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /addgroupadmin <user_id> [chat_id]")
		}
		if err := w.perms.AddGroupAdmin(target, scopeChat, ""); err != nil {
			return w.replyPlain(ctx, botConfig.Token, chatID, err.Error())
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已添加群管理员 user=%s chat=%s", target, scopeChat))
	case "/removegroupadmin":
		ok, err := w.perms.CanManageGroupAdmins(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
		}
		target, scopeChat := parseUserAndChat(args, chatID)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /removegroupadmin <user_id> [chat_id]")
		}
		if err := w.perms.RemoveGroupAdmin(target, scopeChat); err != nil {
			return err
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已移除群管理员 user=%s chat=%s", target, scopeChat))
	case "/listadmins", "/listsuperadmins":
		ok, err := w.perms.IsAdmin(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
		}
		list, err := w.perms.ListAdmins()
		if err != nil {
			return err
		}
		var b strings.Builder
		b.WriteString("开发者 (配置):\n")
		for id := range w.perms.DeveloperUserIDs {
			b.WriteString("- " + id + "\n")
		}
		b.WriteString("超级管理员 (数据库):\n")
		if len(list) == 0 {
			b.WriteString("(空)\n")
		}
		for _, a := range list {
			b.WriteString("- " + a.UserID + "\n")
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, b.String())
	case "/listgroupadmins":
		ok, err := w.perms.IsAdmin(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
		}
		filter := firstToken(args)
		list, err := w.perms.ListGroupAdmins(filter)
		if err != nil {
			return err
		}
		var b strings.Builder
		b.WriteString("群管理员:\n")
		if len(list) == 0 {
			b.WriteString("(空)\n")
		}
		for _, a := range list {
			fmt.Fprintf(&b, "- user=%s chat=%s\n", a.UserID, a.ChatID)
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, b.String())
	default:
		return nil
	}
}

func roleOrNone(role string) string {
	if role == "" {
		return "none"
	}
	return role
}

func (w worker) cmdSetAd(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID, args string, isPrefix bool) error {
	ok, err := w.perms.CanControlSensitive(userID, chatID)
	if err != nil {
		return err
	}
	if !ok {
		return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
	}
	scope, content := parseAdScope(args)
	if content == "" && scope != "group" && scope != "global" {
		return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /setprefix|setsuffix [global|group] <文本>")
	}
	if scope == "global" {
		role, err := w.perms.RoleInChat(userID, chatID)
		if err != nil {
			return err
		}
		if role != botperm.RoleDeveloper && role != botperm.RoleAdmin {
			return w.replyPlain(ctx, botConfig.Token, chatID, "设置全局广告需要开发者或超级管理员")
		}
		if isPrefix {
			err = ads.SaveGlobal(w.db, &content, nil)
		} else {
			err = ads.SaveGlobal(w.db, nil, &content)
		}
	} else {
		if isPrefix {
			err = ads.SaveGroup(w.db, chatID, &content, nil)
		} else {
			err = ads.SaveGroup(w.db, chatID, nil, &content)
		}
	}
	if err != nil {
		return err
	}
	which := "suffix"
	if isPrefix {
		which = "prefix"
	}
	return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已更新 %s 广告（scope=%s）", which, scope))
}

func (w worker) cmdClearAd(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID, args string, isPrefix bool) error {
	ok, err := w.perms.CanControlSensitive(userID, chatID)
	if err != nil {
		return err
	}
	if !ok {
		return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足")
	}
	scope := strings.ToLower(firstToken(args))
	if scope == "" {
		scope = "group"
	}
	empty := ""
	if scope == "global" {
		role, err := w.perms.RoleInChat(userID, chatID)
		if err != nil {
			return err
		}
		if role != botperm.RoleDeveloper && role != botperm.RoleAdmin {
			return w.replyPlain(ctx, botConfig.Token, chatID, "清空全局广告需要开发者或超级管理员")
		}
		if isPrefix {
			err = ads.SaveGlobal(w.db, &empty, nil)
		} else {
			err = ads.SaveGlobal(w.db, nil, &empty)
		}
	} else {
		if isPrefix {
			err = ads.SaveGroup(w.db, chatID, &empty, nil)
		} else {
			err = ads.SaveGroup(w.db, chatID, nil, &empty)
		}
	}
	if err != nil {
		return err
	}
	which := "suffix"
	if isPrefix {
		which = "prefix"
	}
	return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已清空 %s 广告（scope=%s）", which, scope))
}

func (w worker) reply(ctx context.Context, token, chatID, body string) error {
	text, err := ads.Wrap(w.db, chatID, body)
	if err != nil {
		return err
	}
	return w.client.SendMessage(ctx, token, chatID, text)
}

func (w worker) replyPlain(ctx context.Context, token, chatID, body string) error {
	return w.client.SendMessage(ctx, token, chatID, body)
}

func firstToken(s string) string {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseUserAndChat(args, defaultChat string) (userID, chatID string) {
	fields := strings.Fields(strings.TrimSpace(args))
	if len(fields) == 0 {
		return "", defaultChat
	}
	userID = fields[0]
	chatID = defaultChat
	if len(fields) > 1 {
		chatID = fields[1]
	}
	return userID, chatID
}

func parseAdScope(args string) (scope, content string) {
	args = strings.TrimSpace(args)
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return "group", ""
	}
	switch strings.ToLower(fields[0]) {
	case "global", "group":
		scope = strings.ToLower(fields[0])
		content = strings.TrimSpace(strings.TrimPrefix(args, fields[0]))
		return scope, content
	default:
		return "group", args
	}
}

func emptyMark(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(empty)"
	}
	return v
}
