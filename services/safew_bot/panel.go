package safewbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/lotterybroadcast"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"
)

// Panel tiers: the audience a panel was rendered for. Encoded as the first
// part of callback_data ("s:c6:1") so a refresh keeps the same button set no
// matter who clicked; the clicker's own permission is always re-checked.
const (
	tierSuper = "s" // developer / 超级管理员
	tierGroup = "g" // 群管理员
	tierUser  = "u" // 普通用户
)

func tierForRole(role string) string {
	switch role {
	case botperm.RoleDeveloper, botperm.RoleAdmin:
		return tierSuper
	case botperm.RoleGroupAdmin:
		return tierGroup
	}
	return tierUser
}

func roleForTier(tier string) string {
	switch tier {
	case tierSuper:
		return botperm.RoleAdmin
	case tierGroup:
		return botperm.RoleGroupAdmin
	}
	return botperm.RoleNone
}

// permission levels for callback actions
type permLevel int

const (
	levelAnyone permLevel = iota
	levelGroupAdmin
	levelSuper
	levelInvalid
)

// actionLevel maps a callback action (without the tier prefix) to the permission it needs.
// Mirrors the equivalent commands: push / 冠军 / 亚军 = CanTogglePush; 开奖订阅 / 广告状态 = CanControlSensitive.
func actionLevel(action string) permLevel {
	switch {
	case action == "m" || action == "q:lt" || action == "q:ps" || action == "q:ls":
		return levelAnyone
	case strings.HasPrefix(action, "lh:"):
		return levelAnyone
	case action == "p:1" || action == "p:0" || action == "c:0" || action == "r:0" ||
		action == "c6:1" || action == "c6:0" || action == "c7:1" || action == "c7:0" ||
		action == "r6:1" || action == "r6:0" || action == "r7:1" || action == "r7:0":
		return levelSuper
	case action == "ad" || action == "lb:1" || action == "lb:0" || action == "ls" || action == "la":
		return levelGroupAdmin
	case strings.HasPrefix(action, "lt:"):
		if _, ok := lotterybroadcast.Games[strings.TrimPrefix(action, "lt:")]; ok {
			return levelGroupAdmin
		}
	}
	return levelInvalid
}

// permChecker is the subset of *botperm.Store used for callback authorization.
type permChecker interface {
	CanTogglePush(userID string) (bool, error)
	CanControlSensitive(userID, chatID string) (bool, error)
}

// authorizeCallback re-checks the clicker's permission with the same helpers the
// text commands use. Buttons being visible never grants anything.
func authorizeCallback(perms permChecker, userID, chatID, action string) (bool, error) {
	switch actionLevel(action) {
	case levelAnyone:
		return true, nil
	case levelGroupAdmin:
		return perms.CanControlSensitive(userID, chatID)
	case levelSuper:
		return perms.CanTogglePush(userID)
	}
	return false, nil
}

func parseCallbackData(data string) (tier, action string, ok bool) {
	i := strings.Index(data, ":")
	if i <= 0 {
		return "", "", false
	}
	tier, action = data[:i], data[i+1:]
	switch tier {
	case tierSuper, tierGroup, tierUser:
		return tier, action, action != ""
	}
	return "", "", false
}

// panelState is what the buttons display (✅ on enabled modes).
type panelState struct {
	IsGroup       bool
	Push          bool
	C6, C7        bool
	R6, R7        bool
	SubscribedCnt int
}

func btn(tier, text, action string) safew.InlineKeyboardButton {
	return safew.InlineKeyboardButton{Text: text, CallbackData: tier + ":" + action}
}

func check(on bool, label string) string {
	if on {
		return "✅ " + label
	}
	return label
}

func flip(on bool) string {
	if on {
		return "0"
	}
	return "1"
}

// panelKeyboard builds the main inline panel for a tier. Ordinary users only
// get query buttons; control buttons appear only for roles allowed to use them.
func panelKeyboard(tier string, st panelState) *safew.InlineKeyboardMarkup {
	var rows [][]safew.InlineKeyboardButton
	if !st.IsGroup {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "🎟 查看开奖", "q:lt")})
		return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
	}
	rows = append(rows, []safew.InlineKeyboardButton{
		btn(tier, "🎟 查看开奖", "q:lt"),
		btn(tier, "📊 推送状态", "q:ps"),
		btn(tier, "🎫 本群订阅", "q:ls"),
	})
	if tier == tierSuper {
		push := "⛔ 推送已关（点此开启）"
		if st.Push {
			push = "✅ 推送已开（点此关闭）"
		}
		rows = append(rows,
			[]safew.InlineKeyboardButton{btn(tier, push, "p:"+flip(st.Push))},
			[]safew.InlineKeyboardButton{
				btn(tier, check(st.C6, "冠军6码"), "c6:"+flip(st.C6)),
				btn(tier, check(st.C7, "冠军7码"), "c7:"+flip(st.C7)),
				btn(tier, "关闭冠军", "c:0"),
			},
			[]safew.InlineKeyboardButton{
				btn(tier, check(st.R6, "亚军6码"), "r6:"+flip(st.R6)),
				btn(tier, check(st.R7, "亚军7码"), "r7:"+flip(st.R7)),
				btn(tier, "关闭亚军", "r:0"),
			},
		)
	}
	if tier == tierSuper || tier == tierGroup {
		sub := "🔕 订阅开奖播报"
		subAction := "lb:1"
		if st.SubscribedCnt > 0 {
			sub = fmt.Sprintf("✅ 开奖播报 %d/%d（点此取消）", st.SubscribedCnt, len(lotterybroadcast.GameOrder))
			subAction = "lb:0"
		}
		rows = append(rows,
			[]safew.InlineKeyboardButton{btn(tier, sub, subAction), btn(tier, "⚙️ 选择彩种", "ls")},
			[]safew.InlineKeyboardButton{btn(tier, "📢 广告状态", "ad"), btn(tier, "🔄 刷新", "m")},
		)
	} else {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "🔄 刷新", "m")})
	}
	return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// lotterySubKeyboard mirrors lottery_subscription_keyboard (✅/❌ per game).
func lotterySubKeyboard(tier string, subscribed map[string]bool) *safew.InlineKeyboardMarkup {
	all := "✅ 开启全部播报"
	if len(subscribed) > 0 {
		all = "⛔ 关闭全部播报"
	}
	rows := [][]safew.InlineKeyboardButton{{btn(tier, all, "la")}}
	var line []safew.InlineKeyboardButton
	for _, code := range lotterybroadcast.GameOrder {
		mark := "❌"
		if subscribed[code] {
			mark = "✅"
		}
		line = append(line, btn(tier, mark+" "+lotterybroadcast.Games[code].Name, "lt:"+code))
		if len(line) == 2 {
			rows = append(rows, line)
			line = nil
		}
	}
	if len(line) > 0 {
		rows = append(rows, line)
	}
	rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "⬅️ 返回菜单", "m")})
	return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func lotterySubText(subscribed map[string]bool) string {
	return fmt.Sprintf("🎟 群开奖播报\n\n请选择需要订阅的彩种：\n【❌ 未订阅 · ✅ 已订阅】\n\n当前已订阅 %d/%d 个彩种。新期开奖后自动播报并置顶。\n香港六合彩使用香港赛马会官方数据；澳门与新澳为第三方数据，非澳门官方。",
		len(subscribed), len(lotterybroadcast.GameOrder))
}

func isGroupChat(chat safew.Chat) bool {
	t := strings.ToLower(chat.Type)
	if t == "group" || t == "supergroup" {
		return true
	}
	return t == "" && strings.HasPrefix(chat.IDString(), "-")
}

func (w worker) panelStateFor(chat safew.Chat) panelState {
	st := panelState{IsGroup: isGroupChat(chat)}
	if !st.IsGroup {
		return st
	}
	chatID := chat.IDString()
	if s, err := w.perms.GetGroupSettings(chatID); err == nil {
		st.Push, st.C6, st.C7, st.R6, st.R7 = s.PushEnabled, s.Enable6Code, s.Enable7Code, s.EnableRunnerUp6Code, s.EnableRunnerUp7Code
	}
	if w.lb != nil {
		if codes, err := w.lb.store.SubscribedCodesForChat(chatID); err == nil {
			st.SubscribedCnt = len(codes)
		}
	}
	return st
}

// sendPanel replies with the role-specific help text plus the inline panel.
func (w worker) sendPanel(ctx context.Context, token string, chat safew.Chat, role string) error {
	tier := tierForRole(role)
	_, err := w.client.SendMessageEx(ctx, token, chat.IDString(), html.EscapeString(helpForRole(role)),
		safew.SendOptions{ReplyMarkup: panelKeyboard(tier, w.panelStateFor(chat))})
	return err
}

func (w worker) subscribedSet(chatID string) map[string]bool {
	set := map[string]bool{}
	if w.lb == nil {
		return set
	}
	codes, _ := w.lb.store.SubscribedCodesForChat(chatID)
	for _, c := range codes {
		set[c] = true
	}
	return set
}

// handleCallback processes an inline button click: re-check permission, run the
// same code path as the command, toast, then refresh the panel.
func (w worker) handleCallback(ctx context.Context, botConfig systemconfig.SafeW, cq *safew.CallbackQuery) {
	token := botConfig.Token
	answer := func(text string, alert bool) {
		if err := w.client.AnswerCallbackQuery(ctx, token, cq.ID, text, alert); err != nil {
			log.Printf("answerCallbackQuery 失败: %v", err)
		}
	}
	if cq.Message == nil {
		answer("该按钮已失效，请重新发送 /菜单", false)
		return
	}
	tier, action, ok := parseCallbackData(cq.Data)
	if !ok || actionLevel(action) == levelInvalid {
		answer("该按钮已失效，请重新发送 /菜单", false)
		return
	}
	chat := cq.Message.Chat
	chatID := chat.IDString()
	userID := cq.From.IDString()
	_ = w.perms.EnsureGroup(chatID)
	allowed, err := authorizeCallback(w.perms, userID, chatID, action)
	if err != nil {
		answer(shortErr(err), true)
		return
	}
	if !allowed {
		answer("权限不足", false)
		return
	}
	toast, alert, view, err := w.runPanelAction(ctx, token, chat, cq.Message.MessageID, userID, tier, action)
	if err != nil {
		log.Printf("按钮操作失败 chat=%s data=%s: %v", chatID, cq.Data, err)
		answer(shortErr(err), true)
		return
	}
	answer(toast, alert)
	w.refreshPanel(ctx, token, chat, cq.Message.MessageID, tier, view)
}

func shortErr(err error) string {
	msg := "命令失败: " + err.Error()
	if r := []rune(msg); len(r) > 190 {
		msg = string(r[:190]) + "…"
	}
	return msg
}

// panelView tells refreshPanel what to show after an action.
type panelView int

const (
	viewKeep panelView = iota // main panel: refresh keyboard only
	viewMain                  // switch text + keyboard back to the main panel
	viewSubs                  // per-game subscription menu
	viewNone                  // leave the message untouched
)

func (w worker) refreshPanel(ctx context.Context, token string, chat safew.Chat, messageID int, tier string, view panelView) {
	chatID := chat.IDString()
	var err error
	switch view {
	case viewKeep:
		err = w.client.EditMessageReplyMarkup(ctx, token, chatID, messageID, panelKeyboard(tier, w.panelStateFor(chat)))
	case viewMain:
		err = w.client.EditMessageText(ctx, token, chatID, messageID, html.EscapeString(helpForRole(roleForTier(tier))), panelKeyboard(tier, w.panelStateFor(chat)))
	case viewSubs:
		set := w.subscribedSet(chatID)
		err = w.client.EditMessageText(ctx, token, chatID, messageID, html.EscapeString(lotterySubText(set)), lotterySubKeyboard(tier, set))
	}
	if err != nil && !safew.IsNotModified(err) {
		log.Printf("刷新按钮面板失败 chat=%s: %v", chatID, err)
	}
}

var errGroupOnly = errors.New("开奖播报只能订阅到群组")

// runPanelAction executes an authorized action and returns the toast text.
func (w worker) runPanelAction(ctx context.Context, token string, chat safew.Chat, messageID int, userID, tier, action string) (toast string, alert bool, view panelView, err error) {
	chatID := chat.IDString()
	if strings.HasPrefix(action, "lh:") {
		parts := strings.Split(action, ":")
		page, perr := 0, error(nil)
		if len(parts) == 3 {
			page, perr = strconv.Atoi(parts[2])
		}
		if len(parts) != 3 || perr != nil || w.lb == nil {
			return "无效页码", false, viewNone, nil
		}
		return "", false, viewNone, w.editHistoryPage(ctx, token, chatID, messageID, parts[1], page)
	}
	switch action {
	case "m":
		return "已刷新", false, viewMain, nil
	case "q:lt":
		if w.lb == nil {
			return "", false, viewNone, errors.New("开奖播报未启用")
		}
		go w.runAsync(ctx, token, chatID, func() error { return w.lotteryQuery(ctx, token, chat, "") })
		return "正在查询最新开奖…", false, viewNone, nil
	case "q:ps":
		return w.statusSummary(chatID), true, viewKeep, nil
	case "q:ls":
		return w.subscriptionSummary(chatID), true, viewKeep, nil
	case "p:1", "p:0":
		_, toast, err := w.applyPush(chatID, action == "p:1")
		return toast, false, viewKeep, err
	case "c6:1", "c6:0", "c7:1", "c7:0":
		size := 6
		if action[1] == '7' {
			size = 7
		}
		_, toast, err := w.applyCodeMode(chatID, size, strings.HasSuffix(action, ":1"))
		return toast, false, viewKeep, err
	case "r6:1", "r6:0", "r7:1", "r7:0":
		size := 6
		if action[1] == '7' {
			size = 7
		}
		_, toast, err := w.applyRunnerUpCodeMode(chatID, size, strings.HasSuffix(action, ":1"))
		return toast, false, viewKeep, err
	case "c:0":
		if _, _, err := w.applyCodeMode(chatID, 6, false); err != nil {
			return "", false, viewKeep, err
		}
		if _, _, err := w.applyCodeMode(chatID, 7, false); err != nil {
			return "", false, viewKeep, err
		}
		return "已关闭冠军推送", false, viewKeep, nil
	case "r:0":
		if _, _, err := w.applyRunnerUpCodeMode(chatID, 6, false); err != nil {
			return "", false, viewKeep, err
		}
		if _, _, err := w.applyRunnerUpCodeMode(chatID, 7, false); err != nil {
			return "", false, viewKeep, err
		}
		return "已关闭亚军推送", false, viewKeep, nil
	case "ad":
		s, err := ads.LoadForChat(w.db, chatID)
		if err != nil {
			return "", false, viewKeep, err
		}
		body := fmt.Sprintf("scope=%s\nprefix:\n%s\n\nsuffix:\n%s", s.Scope, emptyMark(s.PrefixAd), emptyMark(s.SuffixAd))
		if err := w.reply(ctx, token, chatID, body); err != nil {
			return "", false, viewKeep, err
		}
		return "已发送广告状态", false, viewKeep, nil
	}
	// lottery subscription actions (group only)
	if w.lb == nil {
		return "", false, viewNone, errors.New("开奖播报未启用")
	}
	if !isGroupChat(chat) {
		return "", false, viewNone, errGroupOnly
	}
	switch {
	case action == "lb:1":
		if err := w.lb.store.AddSubscription(chatID, "all", userID); err != nil {
			return "", false, viewKeep, err
		}
		return "✅ 已订阅全部彩种，新期开奖后自动播报并置顶", false, viewKeep, nil
	case action == "lb:0":
		n, err := w.lb.store.RemoveSubscription(chatID, "")
		if err != nil {
			return "", false, viewKeep, err
		}
		return fmt.Sprintf("已取消 %d 条开奖订阅", n), false, viewKeep, nil
	case action == "ls":
		return "请选择要订阅的彩种", false, viewSubs, nil
	case action == "la":
		set := w.subscribedSet(chatID)
		var codes []string
		if len(set) == 0 {
			codes = []string{"all"}
		}
		if err := w.lb.store.SetSubscribedCodes(chatID, codes, userID); err != nil {
			return "", false, viewSubs, err
		}
		return "开奖订阅已更新", false, viewSubs, nil
	case strings.HasPrefix(action, "lt:"):
		code := strings.TrimPrefix(action, "lt:")
		set := w.subscribedSet(chatID)
		if set[code] {
			delete(set, code)
		} else {
			set[code] = true
		}
		codes := make([]string, 0, len(set))
		for c := range set {
			codes = append(codes, c)
		}
		if err := w.lb.store.SetSubscribedCodes(chatID, codes, userID); err != nil {
			return "", false, viewSubs, err
		}
		return "开奖订阅已更新", false, viewSubs, nil
	}
	return "", false, viewNone, errors.New("未知操作")
}

func onOff(v bool) string {
	if v {
		return "开"
	}
	return "关"
}

// statusSummary is a short (≤200 chars) push status for a toast.
func (w worker) statusSummary(chatID string) string {
	s, err := w.perms.GetGroupSettings(chatID)
	if err != nil {
		return shortErr(err)
	}
	champion, runnerUp := "关", "关"
	switch {
	case s.Enable6Code && s.Enable7Code:
		champion = "6码+7码"
	case s.Enable6Code:
		champion = "6码"
	case s.Enable7Code:
		champion = "7码"
	}
	switch {
	case s.EnableRunnerUp6Code && s.EnableRunnerUp7Code:
		runnerUp = "6码+7码"
	case s.EnableRunnerUp6Code:
		runnerUp = "6码"
	case s.EnableRunnerUp7Code:
		runnerUp = "7码"
	}
	sub := 0
	if w.lb != nil {
		if codes, err := w.lb.store.SubscribedCodesForChat(chatID); err == nil {
			sub = len(codes)
		}
	}
	return fmt.Sprintf("本群推送：%s\n冠军：%s\n亚军：%s\n开奖播报：已订阅 %d/%d 个彩种", onOff(s.PushEnabled), champion, runnerUp, sub, len(lotterybroadcast.GameOrder))
}

func (w worker) subscriptionSummary(chatID string) string {
	if w.lb == nil {
		return "开奖播报未启用"
	}
	codes, err := w.lb.store.SubscribedCodesForChat(chatID)
	if err != nil {
		return shortErr(err)
	}
	if len(codes) == 0 {
		return "本群没有开奖播报订阅"
	}
	if len(codes) == len(lotterybroadcast.GameOrder) {
		return "本群已订阅：全部彩种（新期开奖自动播报并置顶）"
	}
	names := make([]string, 0, len(codes))
	for _, c := range codes {
		names = append(names, lotterybroadcast.Games[c].Name)
	}
	msg := "本群已订阅：" + strings.Join(names, "、")
	if r := []rune(msg); len(r) > 190 {
		msg = string(r[:190]) + "…"
	}
	return msg
}
