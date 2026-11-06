package safewbot

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Telegram 母机器人: a second mother on official Telegram with the same full
// permissions as the SafeW mother. Token: env TELEGRAM_BOT_TOKEN, else
// system_configs telegram_bot_token (set via /设置TG母机器人).

const telegramTokenKey = "telegram_bot_token"

func telegramAPIBase() string {
	if v := strings.TrimSpace(os.Getenv("TELEGRAM_API_BASE")); v != "" {
		return v
	}
	return botclone.DefaultAPIBase
}

// tgMotherToken returns the configured token and its source ("env"/"db"/"").
func (w worker) tgMotherToken() (string, string) {
	if v := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")); v != "" {
		return v, "env"
	}
	if w.db == nil {
		return "", ""
	}
	var c domain.SystemConfig
	if w.db.Session(&gorm.Session{Logger: logger.Discard}).Where("`key` = ?", telegramTokenKey).First(&c).Error == nil {
		if v := strings.TrimSpace(c.Value); v != "" {
			return v, "db"
		}
	}
	return "", ""
}

type tgGetMe func(ctx context.Context, base, token string) (id, username string, err error)

func defaultTGGetMe(ctx context.Context, base, token string) (string, string, error) {
	gctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	me, err := safew.NewClient(base).GetMe(gctx, token)
	if err != nil {
		return "", "", err
	}
	return me.IDString(), me.Username, nil
}

// startTGMother validates token via getMe and (re)starts the Telegram mother poller.
func (w worker) startTGMother(ctx context.Context, token string, getMe tgGetMe) (*botInstance, error) {
	token = strings.TrimSpace(token)
	if !botclone.LooksLikeToken(token) {
		return nil, botclone.ErrBadToken
	}
	base := telegramAPIBase()
	id, username, err := getMe(ctx, base, token)
	if err != nil || id == "" {
		return nil, botclone.ErrBadToken
	}
	if w.reg.motherID() == id {
		return nil, errors.New("这是 SafeW 母机器人的 Token，不能再设为 Telegram 母机器人")
	}
	if cur := w.reg.get(id); cur != nil && !cur.IsMother {
		w.reg.remove(id) // was running as a clone: promote to mother
	}
	pctx, cancel := context.WithCancel(w.reg.root)
	inst := &botInstance{ID: id, Username: username, IsMother: true, APIBase: base,
		client: safew.NewClient(base), token: token, cancel: cancel}
	if !w.reg.setTGMother(inst) {
		cancel()
		return nil, errors.New("该机器人已在运行")
	}
	iw := w.forInstance(inst)
	go func() {
		dctx, dcancel := context.WithTimeout(pctx, 20*time.Second)
		_ = inst.client.DeleteWebhook(dctx, token)
		dcancel()
		log.Printf("Telegram 母机器人已启动: bot_id=%s @%s", id, username)
		_ = iw.runCommands(pctx)
		log.Printf("Telegram 母机器人已停止: bot_id=%s @%s", id, username)
	}()
	return inst, nil
}

// bootTGMother starts the Telegram mother at startup (after the SafeW mother).
func (w worker) bootTGMother(ctx context.Context) {
	token, _ := w.tgMotherToken()
	if token == "" {
		log.Printf("未配置 Telegram 母机器人（TELEGRAM_BOT_TOKEN / /设置TG母机器人），仅运行 SafeW 母机器人")
		return
	}
	for attempt := 1; attempt <= 5; attempt++ {
		_, err := w.startTGMother(ctx, token, defaultTGGetMe)
		if err == nil {
			return
		}
		log.Printf("Telegram 母机器人启动失败(%d/5): %v", attempt, err)
		if !wait(ctx, time.Duration(attempt)*5*time.Second) {
			return
		}
	}
}

func (w worker) canManageTGMother(userID string) bool {
	return w.isMother() && w.reg != nil && w.perms.IsDeveloper(userID)
}

func tgPendingKey(userID string) string { return "tgm|" + userID }

// handleTGMotherCommand: /设置TG母机器人 [token], /关闭TG母机器人 (mother developers, private chat).
func (w worker) handleTGMotherCommand(ctx context.Context, token string, msg *safew.Message, userID, cmd, args string) (bool, error) {
	set := cmd == "/设置tg母机器人" || cmd == "/settgmother"
	off := cmd == "/关闭tg母机器人" || cmd == "/offtgmother"
	if cmd == "/取消" && w.reg != nil && msg.From != nil && w.reg.isPending(w.botID(), tgPendingKey(userID)) {
		w.reg.setPending(w.botID(), tgPendingKey(userID), false)
		return true, w.replyPlain(ctx, token, msg.Chat.IDString(), "已取消")
	}
	if !set && !off {
		return false, nil
	}
	chatID := msg.Chat.IDString()
	if strings.TrimSpace(args) != "" {
		_ = w.client.DeleteMessage(ctx, token, chatID, msg.MessageID)
	}
	if !w.canManageTGMother(userID) {
		return true, nil
	}
	if isGroupChat(msg.Chat) {
		msgText := "请私聊我操作"
		if strings.TrimSpace(args) != "" {
			msgText = "⚠️ 不要在群里发送 Token！已尝试删除，请到 @BotFather 重置后私聊我。"
		}
		return true, w.replyPlain(ctx, token, chatID, msgText)
	}
	if off {
		w.reg.stopTGMother()
		_ = systemconfig.SaveValue(w.db, telegramTokenKey, "")
		text := "✅ 已关闭 Telegram 母机器人并清除保存的 Token"
		if _, src := w.tgMotherToken(); src == "env" {
			text += "\n⚠️ 环境变量 TELEGRAM_BOT_TOKEN 仍存在，重启后会再次启动，请从 .env 删除"
		}
		return true, w.replyPlain(ctx, token, chatID, text)
	}
	if t := strings.TrimSpace(args); t != "" {
		return true, w.submitTGMotherToken(ctx, token, chatID, userID, t, defaultTGGetMe)
	}
	w.reg.setPending(w.botID(), tgPendingKey(userID), true)
	return true, w.replyPlain(ctx, token, chatID, "🤖 设置 Telegram 母机器人\n\n把 @BotFather 给你的 Bot Token 直接发给我。它将拥有和 SafeW 母机器人一样的全部权限。\n10 分钟内有效，发送 /取消 放弃。")
}

// maybeTGMotherToken consumes a pasted token when the developer is in the flow.
func (w worker) maybeTGMotherToken(ctx context.Context, token string, msg *safew.Message) bool {
	if w.reg == nil || msg == nil || msg.From == nil || isGroupChat(msg.Chat) {
		return false
	}
	userID := msg.From.IDString()
	if !w.reg.isPending(w.botID(), tgPendingKey(userID)) {
		return false
	}
	chatID := msg.Chat.IDString()
	text := strings.TrimSpace(msg.Text)
	if text == "取消" {
		w.reg.setPending(w.botID(), tgPendingKey(userID), false)
		_ = w.replyPlain(ctx, token, chatID, "已取消")
		return true
	}
	_ = w.client.DeleteMessage(ctx, token, chatID, msg.MessageID)
	if err := w.submitTGMotherToken(ctx, token, chatID, userID, text, defaultTGGetMe); err != nil {
		log.Printf("设置 Telegram 母机器人失败 user=%s: %v", userID, err)
	}
	return true
}

func (w worker) submitTGMotherToken(ctx context.Context, token, chatID, userID, tgToken string, getMe tgGetMe) error {
	inst, err := w.applyTGMotherToken(ctx, tgToken, getMe)
	if err != nil {
		return w.replyPlain(ctx, token, chatID, "❌ "+err.Error())
	}
	w.reg.setPending(w.botID(), tgPendingKey(userID), false)
	log.Printf("Telegram 母机器人已设置: bot_id=%s @%s by=%s", inst.ID, inst.Username, userID)
	return w.replyPlain(ctx, token, chatID, "✅ Telegram 母机器人已启动：@"+inst.Username+"\n拥有全部权限（含开发者功能），群数据独立。")
}

// applyTGMotherToken validates, starts, then saves. Invalid tokens are never saved.
func (w worker) applyTGMotherToken(ctx context.Context, tgToken string, getMe tgGetMe) (*botInstance, error) {
	inst, err := w.startTGMother(ctx, tgToken, getMe)
	if err != nil {
		return nil, err
	}
	if w.db != nil {
		if err := systemconfig.SaveValue(w.db, telegramTokenKey, strings.TrimSpace(tgToken)); err != nil {
			log.Printf("保存 Telegram 母机器人 Token 失败: %v", err)
		}
	}
	return inst, nil
}

// tgMotherPanelAction runs the tgs/tgo buttons (developers on a mother only).
func (w worker) tgMotherPanelAction(ctx context.Context, token string, msgChat safew.Chat, userID, action string) (string, bool, error) {
	if !w.canManageTGMother(userID) {
		return "权限不足", false, nil
	}
	if isGroupChat(msgChat) {
		return "请私聊我操作", true, nil
	}
	chatID := msgChat.IDString()
	if action == "tgo" {
		w.reg.stopTGMother()
		_ = systemconfig.SaveValue(w.db, telegramTokenKey, "")
		return "已关闭 Telegram 母机器人", false, w.replyPlain(ctx, token, chatID, "✅ 已关闭 Telegram 母机器人并清除保存的 Token（若 .env 里有 TELEGRAM_BOT_TOKEN，重启后仍会启动）")
	}
	w.reg.setPending(w.botID(), tgPendingKey(userID), true)
	return "请发送 Bot Token", false, w.replyPlain(ctx, token, chatID, "🤖 设置 Telegram 母机器人\n\n把 @BotFather 给你的 Bot Token 直接发给我，验证通过立即启动，拥有全部权限。\n10 分钟内有效，发送 /取消 放弃。")
}
