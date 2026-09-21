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

// commandHelp is intentionally short and must NOT include docs/使用教程 content.
const commandHelp = `可用命令（权限控制）：
/help — 显示本帮助
/whoami — 查看你的权限
/adstatus — 查看当前广告配置
/resetads group — 删除本群广告覆盖（回落全局）

广告（developer / admin / 本群 group_admin）：
/setprefix [global|group] <文本> — 设置前广告
/setsuffix [global|group] <文本> — 设置后广告
/clearprefix [global|group] — 清空前广告
/clearsuffix [global|group] — 清空后广告
/say <文本> — 在本会话发送一条带广告的消息
/broadcast <文本> — 向系统配置的全部群发送（仅 developer/admin）

角色：
/addadmin <user_id> — 添加 admin（仅 developer）
/removeadmin <user_id> — 移除 admin（仅 developer）
/addgroupadmin <user_id> [chat_id] — 添加 group_admin（developer/admin）
/removegroupadmin <user_id> [chat_id] — 移除 group_admin
/listadmins — 列出 admin
/listgroupadmins [chat_id] — 列出 group_admin

说明：所有对外正文均通过广告组装后一次 sendMessage 发送；使用教程仅在仓库 docs/ 中，不会作为机器人消息下发。`

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
	// Strip @BotName suffix on command token.
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

	switch cmd {
	case "/help", "/start":
		return w.reply(ctx, botConfig.Token, chatID, commandHelp)
	case "/whoami":
		role, err := w.perms.RoleInChat(userID, chatID)
		if err != nil {
			return err
		}
		if role == "" {
			role = "none"
		}
		return w.reply(ctx, botConfig.Token, chatID, fmt.Sprintf("user_id=%s\nchat_id=%s\nrole=%s", userID, chatID, role))
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
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /resetads group （删除群覆盖以回落全局；全局请用 clearprefix/clearsuffix global）")
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
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要 developer 或 admin）")
		}
		if strings.TrimSpace(args) == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /broadcast <文本>")
		}
		n := 0
		for _, id := range systemconfig.NormalizeChatIDs(botConfig.ChatIDs) {
			if err := w.reply(ctx, botConfig.Token, id, args); err != nil {
				return err
			}
			n++
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已广播到 %d 个群", n))
	case "/addadmin":
		if !w.perms.CanManageAdmins(userID) {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（仅 developer）")
		}
		target := firstToken(args)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /addadmin <user_id>")
		}
		if err := w.perms.AddAdmin(target, ""); err != nil {
			return w.replyPlain(ctx, botConfig.Token, chatID, err.Error())
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已添加 admin: "+target)
	case "/removeadmin":
		if !w.perms.CanManageAdmins(userID) {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（仅 developer）")
		}
		target := firstToken(args)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /removeadmin <user_id>")
		}
		if err := w.perms.RemoveAdmin(target); err != nil {
			return err
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, "已移除 admin: "+target)
	case "/addgroupadmin":
		ok, err := w.perms.CanManageGroupAdmins(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要 developer 或 admin）")
		}
		target, scopeChat := parseUserAndChat(args, chatID)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /addgroupadmin <user_id> [chat_id]")
		}
		if err := w.perms.AddGroupAdmin(target, scopeChat, ""); err != nil {
			return w.replyPlain(ctx, botConfig.Token, chatID, err.Error())
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已添加 group_admin user=%s chat=%s", target, scopeChat))
	case "/removegroupadmin":
		ok, err := w.perms.CanManageGroupAdmins(userID)
		if err != nil {
			return err
		}
		if !ok {
			return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要 developer 或 admin）")
		}
		target, scopeChat := parseUserAndChat(args, chatID)
		if target == "" {
			return w.replyPlain(ctx, botConfig.Token, chatID, "用法: /removegroupadmin <user_id> [chat_id]")
		}
		if err := w.perms.RemoveGroupAdmin(target, scopeChat); err != nil {
			return err
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, fmt.Sprintf("已移除 group_admin user=%s chat=%s", target, scopeChat))
	case "/listadmins":
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
		b.WriteString("developers (config):\n")
		for id := range w.perms.DeveloperUserIDs {
			b.WriteString("- " + id + "\n")
		}
		b.WriteString("admins (db):\n")
		if len(list) == 0 {
			b.WriteString("(empty)\n")
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
		b.WriteString("group_admins:\n")
		if len(list) == 0 {
			b.WriteString("(empty)\n")
		}
		for _, a := range list {
			fmt.Fprintf(&b, "- user=%s chat=%s\n", a.UserID, a.ChatID)
		}
		return w.replyPlain(ctx, botConfig.Token, chatID, b.String())
	default:
		return nil
	}
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
			return w.replyPlain(ctx, botConfig.Token, chatID, "设置全局广告需要 developer 或 admin")
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
			return w.replyPlain(ctx, botConfig.Token, chatID, "清空全局广告需要 developer 或 admin")
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

// reply sends body wrapped with ads in a SINGLE sendMessage.
func (w worker) reply(ctx context.Context, token, chatID, body string) error {
	text, err := ads.Wrap(w.db, chatID, body)
	if err != nil {
		return err
	}
	return w.client.SendMessage(ctx, token, chatID, text)
}

// replyPlain sends without ads (for permission errors / short ACKs).
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
