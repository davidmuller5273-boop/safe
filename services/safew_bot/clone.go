package safewbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
)

// 克隆机器人: any user may paste a @BotFather token in a private chat with the
// mother or any clone; the new bot starts immediately inside this process with
// admin-tier features, and the requester becomes its 超级管理员. Developer-only
// management (/克隆列表 …) exists only on the mother.

const maxClonesPerUser = 5

const clonePrompt = "🤖 克隆机器人\n\n" +
	"1. 打开 @BotFather，发送 /newbot 创建一个新机器人\n" +
	"2. 把它给你的 Bot Token 直接发给我（形如 123456789:AAxxxx）\n\n" +
	"克隆出来的机器人拥有管理员全部功能（推送、冠军/亚军6/7码、广告、开奖播报、波场播报、按钮菜单），群数据独立，你自动成为它的超级管理员。\n" +
	"⚠️ 请只在私聊里发送 Token。10 分钟内有效，发送 /取消 放弃。"

func isCloneCommand(cmd string) bool { return cmd == "/克隆" || cmd == "/clone" }

func cloneAPIBase() string {
	if v := strings.TrimSpace(os.Getenv("CLONE_API_BASE")); v != "" {
		return v
	}
	return botclone.DefaultAPIBase
}

// cmdClone handles /克隆 [token].
func (w worker) cmdClone(ctx context.Context, token string, msg *safew.Message, userID, args string) error {
	chatID := msg.Chat.IDString()
	if w.clones == nil || w.reg == nil {
		return w.replyPlain(ctx, token, chatID, "克隆功能暂不可用")
	}
	if isGroupChat(msg.Chat) {
		if strings.TrimSpace(args) != "" {
			_ = w.client.DeleteMessage(ctx, token, chatID, msg.MessageID)
			return w.replyPlain(ctx, token, chatID, "⚠️ 不要在群里发送 Token！已尝试删除该消息，请尽快到 @BotFather 重置 Token，然后私聊我发送 /克隆。")
		}
		return w.replyPlain(ctx, token, chatID, "请私聊我发送 /克隆")
	}
	if t := strings.TrimSpace(args); t != "" {
		return w.submitCloneToken(ctx, token, msg, userID, t)
	}
	w.reg.setPending(w.botID(), userID, true)
	return w.replyPlain(ctx, token, chatID, clonePrompt)
}

// maybeCloneToken consumes a plain private message when the user is in the
// clone flow, or when it simply looks like a Bot Token. Returns true if handled.
func (w worker) maybeCloneToken(ctx context.Context, token string, msg *safew.Message) bool {
	if w.reg == nil || w.clones == nil || msg == nil || msg.From == nil || isGroupChat(msg.Chat) {
		return false
	}
	text := strings.TrimSpace(msg.Text)
	userID := msg.From.IDString()
	pending := w.reg.isPending(w.botID(), userID)
	if !pending && !botclone.LooksLikeToken(text) {
		return false
	}
	if pending && (text == "/取消" || text == "取消") {
		w.reg.setPending(w.botID(), userID, false)
		_ = w.replyPlain(ctx, token, msg.Chat.IDString(), "已取消克隆")
		return true
	}
	if strings.HasPrefix(text, "/") {
		return false
	}
	if err := w.submitCloneToken(ctx, token, msg, userID, text); err != nil {
		log.Printf("克隆失败 user=%s: %v", userID, err)
	}
	return true
}

func (w worker) submitCloneToken(ctx context.Context, token string, msg *safew.Message, userID, cloneToken string) error {
	chatID := msg.Chat.IDString()
	// Remove the pasted token from the chat history (best effort).
	_ = w.client.DeleteMessage(ctx, token, chatID, msg.MessageID)
	if !w.perms.IsDeveloper(userID) {
		if n, err := w.clones.CountActiveBy(userID); err == nil && n >= maxClonesPerUser {
			return w.replyPlain(ctx, token, chatID, fmt.Sprintf("每人最多克隆 %d 个机器人", maxClonesPerUser))
		}
	}
	username := ""
	if msg.From != nil {
		username = msg.From.Username
	}
	c, err := w.registerClone(ctx, cloneToken, userID, username)
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			return w.replyPlain(ctx, token, chatID, "❌ "+ue.Error())
		}
		return w.replyPlain(ctx, token, chatID, "❌ 克隆失败，请稍后再试")
	}
	w.reg.setPending(w.botID(), userID, false)
	return w.replyPlain(ctx, token, chatID, fmt.Sprintf(
		"✅ 克隆成功：@%s\n\n已立即启动，拥有管理员全部功能（推送、冠军/亚军6/7码、广告、开奖播报、波场播报、按钮菜单），群数据与本机器人互不影响。\n你已是它的超级管理员：把 @%s 拉进群并设为管理员，在群里发 /菜单 即可。",
		c.Username, c.Username))
}

type userError struct{ error }

// registerClone validates (getMe), stores encrypted, grants the requester
// 超级管理员 on the clone, and starts its poller. Invalid tokens are never stored.
func (w worker) registerClone(ctx context.Context, cloneToken, userID, username string) (botclone.Clone, error) {
	getMe := func(ctx context.Context, base, tok string) (botclone.BotIdentity, error) {
		gctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		me, err := safew.NewClient(base).GetMe(gctx, tok)
		if err != nil {
			return botclone.BotIdentity{}, err
		}
		return botclone.BotIdentity{ID: me.IDString(), Username: me.Username, FirstName: me.FirstName}, nil
	}
	c, err := botclone.Register(ctx, w.clones, getMe, botclone.Request{
		ParentBotID:     w.botID(),
		MotherBotID:     w.reg.motherID(),
		IsMotherID:      w.reg.isMotherID,
		APIBase:         cloneAPIBase(),
		Token:           cloneToken,
		RequestedBy:     userID,
		RequestedByName: username,
		IsReserved:      w.reg.running,
	})
	if err != nil {
		if errors.Is(err, botclone.ErrBadToken) || errors.Is(err, botclone.ErrIsMother) || errors.Is(err, botclone.ErrAlreadyActive) {
			return c, userError{err}
		}
		return c, err
	}
	if err := w.basePerms.ForBot(c.BotID, false).AddAdmin(userID, "克隆申请人"); err != nil {
		log.Printf("克隆 bot_id=%s 设置申请人超级管理员失败: %v", c.BotID, err)
	}
	if !w.startClone(c, cloneToken) {
		log.Printf("克隆 bot_id=%s 已在运行", c.BotID)
	}
	log.Printf("新克隆机器人: bot_id=%s @%s parent=%s requested_by=%s", c.BotID, c.Username, c.ParentBotID, userID)
	return c, nil
}

// ---- mother developer management ----

func (w worker) canManageClones(userID string) bool {
	return w.isMother() && w.clones != nil && w.perms.IsDeveloper(userID)
}

func cloneListText(rows []botclone.Clone, running func(string) bool) string {
	if len(rows) == 0 {
		return "🗂 克隆列表\n\n暂无克隆机器人"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "🗂 克隆列表（共 %d 个）\n\n", len(rows))
	for _, c := range rows {
		state := "已停用"
		if c.Status == botclone.StatusActive {
			state = "运行中"
			if !running(c.BotID) {
				state = "未运行"
			}
		}
		req := c.RequestedBy
		if c.RequestedByUsername != "" {
			req += " @" + c.RequestedByUsername
		}
		fmt.Fprintf(&b, "@%s | ID %s | %s | 申请人 %s | 上级 %s | %s\n",
			c.Username, c.BotID, state, req, c.ParentBotID, c.CreatedAt.Format("2006-01-02"))
		if c.LastError != "" {
			fmt.Fprintf(&b, "  └ %s\n", c.LastError)
		}
	}
	b.WriteString("\n命令：/停用克隆 ID　/启用克隆 ID　/删除克隆 ID")
	return b.String()
}

func cloneListKeyboard(tier string, rows []botclone.Clone) *safew.InlineKeyboardMarkup {
	var kb [][]safew.InlineKeyboardButton
	for i, c := range rows {
		if i >= 30 {
			break
		}
		name := truncateBtnLabel("@"+c.Username, 20)
		if c.Status == botclone.StatusActive {
			kb = append(kb, []safew.InlineKeyboardButton{btn(tier, "⏸ 停用 "+name, "klo:"+c.BotID), btn(tier, "🗑 删除", "kld:"+c.BotID)})
		} else {
			kb = append(kb, []safew.InlineKeyboardButton{btn(tier, "▶️ 启用 "+name, "kln:"+c.BotID), btn(tier, "🗑 删除", "kld:"+c.BotID)})
		}
	}
	kb = append(kb, []safew.InlineKeyboardButton{btn(tier, "🔄 刷新列表", "kll"), btn(tier, "⬅️ 返回菜单", "m")})
	return &safew.InlineKeyboardMarkup{InlineKeyboard: kb}
}

func (w worker) sendCloneList(ctx context.Context, token, chatID string, editMessageID int) error {
	rows, err := w.clones.List()
	if err != nil {
		return err
	}
	text := html.EscapeString(cloneListText(rows, w.reg.running))
	kb := cloneListKeyboard(tierSuper, rows)
	if editMessageID > 0 {
		err := w.client.EditMessageText(ctx, token, chatID, editMessageID, text, kb)
		if err == nil || safew.IsNotModified(err) {
			return nil
		}
	}
	_, err = w.client.SendMessageEx(ctx, token, chatID, text, safew.SendOptions{ReplyMarkup: kb})
	return err
}

// setCloneState: "off" disable (stop poller), "on" re-enable, "del" stop + delete row.
func (w worker) setCloneState(ctx context.Context, botID, op string) (string, error) {
	botID = strings.TrimSpace(botID)
	if botID == "" {
		return "", userError{errors.New("请提供克隆机器人 ID（见 /克隆列表）")}
	}
	if w.reg.isMotherID(botID) {
		return "", userError{errors.New("不能操作母机器人")}
	}
	c, found, err := w.clones.Get(botID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", userError{errors.New("没有这个克隆机器人：" + botID)}
	}
	switch op {
	case "off":
		w.reg.remove(botID)
		if err := w.clones.SetStatus(botID, botclone.StatusDisabled, "开发者已停用"); err != nil {
			return "", err
		}
		return "已停用 @" + c.Username, nil
	case "on":
		tok, err := w.clones.Token(c)
		if err != nil {
			return "", userError{errors.New("Token 解密失败，请让申请人重新 /克隆")}
		}
		if err := w.clones.SetStatus(botID, botclone.StatusActive, ""); err != nil {
			return "", err
		}
		w.startClone(c, tok)
		return "已启用 @" + c.Username, nil
	case "del":
		w.reg.remove(botID)
		if err := w.clones.Delete(botID); err != nil {
			return "", err
		}
		return "已删除 @" + c.Username + "（其群数据保留，重新克隆同一机器人可恢复）", nil
	}
	return "", errors.New("未知操作")
}

// handleCloneAdminCommand handles /克隆列表 /停用克隆 /启用克隆 /删除克隆 (mother developers).
func (w worker) handleCloneAdminCommand(ctx context.Context, token, chatID, userID, cmd, args string) (bool, error) {
	op := ""
	switch cmd {
	case "/克隆列表", "/clones":
		op = "list"
	case "/停用克隆":
		op = "off"
	case "/启用克隆":
		op = "on"
	case "/删除克隆":
		op = "del"
	default:
		return false, nil
	}
	if !w.canManageClones(userID) {
		return true, nil // silent like other developer commands
	}
	if op == "list" {
		return true, w.sendCloneList(ctx, token, chatID, 0)
	}
	text, err := w.setCloneState(ctx, firstToken(args), op)
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			return true, w.replyPlain(ctx, token, chatID, ue.Error())
		}
		return true, err
	}
	return true, w.replyPlain(ctx, token, chatID, text)
}

// clonePanelAction runs the kl* buttons (permission already checked).
func (w worker) clonePanelAction(ctx context.Context, token string, msgChat safew.Chat, messageID int, userID, action string) (string, bool, error) {
	chatID := msgChat.IDString()
	switch {
	case action == "kl":
		if w.clones == nil || w.reg == nil {
			return "克隆功能暂不可用", true, nil
		}
		if isGroupChat(msgChat) {
			return "请私聊我发送 /克隆", true, nil
		}
		w.reg.setPending(w.botID(), userID, true)
		return "请发送 Bot Token", false, w.replyPlain(ctx, token, chatID, clonePrompt)
	case !w.canManageClones(userID):
		return "权限不足", false, nil
	case action == "kll":
		return "", false, w.sendCloneList(ctx, token, chatID, messageID)
	}
	op := map[string]string{"klo": "off", "kln": "on", "kld": "del"}[action[:3]]
	text, err := w.setCloneState(ctx, strings.TrimPrefix(action[3:], ":"), op)
	if err != nil {
		var ue userError
		if errors.As(err, &ue) {
			return ue.Error(), true, nil
		}
		return "", false, err
	}
	_ = w.sendCloneList(ctx, token, chatID, messageID)
	return text, false, nil
}
