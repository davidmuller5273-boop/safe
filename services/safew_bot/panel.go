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
// part of callback_data ("s:c6:1" or "s#10000852380:c6:1") so a refresh keeps
// the same button set; the clicker's own permission is always re-checked.
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
	levelDeveloper
	levelInvalid
)

// actionLevel maps a callback action (without the tier / target prefix) to the permission it needs.
// Mirrors the equivalent commands: push / 冠军 / 亚军 = CanTogglePush; 开奖订阅 / 广告状态 = CanControlSensitive.
func actionLevel(action string) permLevel {
	switch {
	case action == "m" || action == "q:lt" || action == "q:ps" || action == "q:ls" || action == "q:lh":
		return levelAnyone
	case action == "gs" || strings.HasPrefix(action, "gs:"):
		return levelSuper
	case action == "ex":
		return levelDeveloper
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
	IsDeveloper(userID string) bool
}

// authorizeCallback re-checks the clicker's permission with the same helpers the
// text commands use. chatID must be the *target* group when acting from a DM.
// Buttons being visible never grants anything.
func authorizeCallback(perms permChecker, userID, chatID, action string) (bool, error) {
	switch actionLevel(action) {
	case levelAnyone:
		return true, nil
	case levelGroupAdmin:
		return perms.CanControlSensitive(userID, chatID)
	case levelSuper:
		return perms.CanTogglePush(userID)
	case levelDeveloper:
		return perms.IsDeveloper(userID), nil
	}
	return false, nil
}

// parseCallbackData accepts:
//   - tier:action            (in-group panels, e.g. "s:c6:1")
//   - tier#chatID:action     (DM targeting a group, e.g. "s#10000852380:c6:1")
func parseCallbackData(data string) (tier, targetChat, action string, ok bool) {
	if data == "" {
		return "", "", "", false
	}
	hash := strings.IndexByte(data, '#')
	colon := strings.IndexByte(data, ':')
	if hash > 0 && hash < colon {
		tier = data[:hash]
		rest := data[hash+1:]
		ci := strings.IndexByte(rest, ':')
		if ci <= 0 {
			return "", "", "", false
		}
		targetChat = rest[:ci]
		action = rest[ci+1:]
	} else {
		if colon <= 0 {
			return "", "", "", false
		}
		tier = data[:colon]
		action = data[colon+1:]
	}
	switch tier {
	case tierSuper, tierGroup, tierUser:
		return tier, targetChat, action, action != "" && (hash <= 0 || targetChat != "")
	}
	return "", "", "", false
}

// panelState is what the buttons display (✅ on enabled modes).
type panelState struct {
	IsGroup       bool
	Push          bool
	C6, C7        bool
	R6, R7        bool
	SubscribedCnt int
}

// namedGroup is a short label for private group-pick / admin-group buttons.
type namedGroup struct {
	ChatID, Title string
}

func btn(tier, text, action string) safew.InlineKeyboardButton {
	return btnScoped(tier, "", text, action)
}

func btnScoped(tier, targetChat, text, action string) safew.InlineKeyboardButton {
	data := tier + ":" + action
	if targetChat != "" {
		data = tier + "#" + targetChat + ":" + action
	}
	return safew.InlineKeyboardButton{Text: text, CallbackData: data}
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

func truncateBtnLabel(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes-1]) + "…"
}

// panelKeyboard builds the main inline panel for a tier.
// targetChatID non-empty embeds that chat into every callback (DM remote control).
// When !st.IsGroup the private home query row is returned (callers may append more).
func panelKeyboard(tier string, st panelState, targetChatID string) *safew.InlineKeyboardMarkup {
	mk := func(text, action string) safew.InlineKeyboardButton {
		return btnScoped(tier, targetChatID, text, action)
	}
	var rows [][]safew.InlineKeyboardButton
	if !st.IsGroup {
		rows = append(rows, []safew.InlineKeyboardButton{
			mk("🎟 查看开奖", "q:lt"),
			mk("📜 开奖历史", "q:lh"),
			mk("🔄 刷新", "m"),
		})
		return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
	}
	rows = append(rows, []safew.InlineKeyboardButton{
		mk("🎟 查看开奖", "q:lt"),
		mk("📊 推送状态", "q:ps"),
		mk("🎫 本群订阅", "q:ls"),
	})
	if tier == tierSuper {
		push := "⛔ 推送已关（点此开启）"
		if st.Push {
			push = "✅ 推送已开（点此关闭）"
		}
		rows = append(rows,
			[]safew.InlineKeyboardButton{mk(push, "p:"+flip(st.Push))},
			[]safew.InlineKeyboardButton{
				mk(check(st.C6, "冠军6码"), "c6:"+flip(st.C6)),
				mk(check(st.C7, "冠军7码"), "c7:"+flip(st.C7)),
				mk("关闭冠军", "c:0"),
			},
			[]safew.InlineKeyboardButton{
				mk(check(st.R6, "亚军6码"), "r6:"+flip(st.R6)),
				mk(check(st.R7, "亚军7码"), "r7:"+flip(st.R7)),
				mk("关闭亚军", "r:0"),
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
			[]safew.InlineKeyboardButton{mk(sub, subAction), mk("⚙️ 选择彩种", "ls")},
			[]safew.InlineKeyboardButton{mk("📢 广告状态", "ad"), mk("🔄 刷新", "m")},
		)
	} else {
		rows = append(rows, []safew.InlineKeyboardButton{mk("🔄 刷新", "m")})
	}
	if targetChatID != "" && tier == tierSuper {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "⬅️ 重选群组", "gs")})
	} else if targetChatID != "" && tier == tierGroup {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "⬅️ 返回菜单", "m")})
	}
	return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// privateHomeKeyboard is the DM /菜单 board: query row + role extras.
func privateHomeKeyboard(tier string, adminGroups []namedGroup, showGroupPick, showExport bool) *safew.InlineKeyboardMarkup {
	base := panelKeyboard(tier, panelState{IsGroup: false}, "")
	rows := append([][]safew.InlineKeyboardButton{}, base.InlineKeyboard...)
	if showExport {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "📤 导出所有群成员", "ex")})
	}
	if showGroupPick {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "📂 群组选择", "gs")})
	}
	for _, g := range adminGroups {
		label := g.Title
		if label == "" {
			label = g.ChatID
		}
		rows = append(rows, []safew.InlineKeyboardButton{
			btnScoped(tier, g.ChatID, "群·"+truncateBtnLabel(label, 28), "m"),
		})
	}
	return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
}

const groupPickPageSize = 8

// groupPickKeyboard lists known groups for developer/super DM control.
func groupPickKeyboard(tier string, groups []namedGroup, page int) *safew.InlineKeyboardMarkup {
	if page < 0 {
		page = 0
	}
	start := page * groupPickPageSize
	var rows [][]safew.InlineKeyboardButton
	if start >= len(groups) {
		rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "（无更多群）", "gs")})
	} else {
		end := start + groupPickPageSize
		if end > len(groups) {
			end = len(groups)
		}
		for _, g := range groups[start:end] {
			label := g.Title
			if label == "" {
				label = "(无标题)"
			}
			rows = append(rows, []safew.InlineKeyboardButton{
				btnScoped(tier, g.ChatID, truncateBtnLabel(label, 40)+" · "+g.ChatID, "m"),
			})
		}
	}
	var nav []safew.InlineKeyboardButton
	if page > 0 {
		nav = append(nav, btn(tier, "上一页", fmt.Sprintf("gs:%d", page-1)))
	}
	if (page+1)*groupPickPageSize < len(groups) {
		nav = append(nav, btn(tier, "下一页", fmt.Sprintf("gs:%d", page+1)))
	}
	if len(nav) > 0 {
		rows = append(rows, nav)
	}
	rows = append(rows, []safew.InlineKeyboardButton{btn(tier, "⬅️ 返回菜单", "m")})
	return &safew.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// historyPickKeyboard lets anyone pick a game for /开奖历史 from the private menu.
func historyPickKeyboard(tier string) *safew.InlineKeyboardMarkup {
	var rows [][]safew.InlineKeyboardButton
	var line []safew.InlineKeyboardButton
	for _, code := range lotterybroadcast.GameOrder {
		line = append(line, btn(tier, lotterybroadcast.Games[code].Name, "lh:"+code+":0"))
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

// lotterySubKeyboard mirrors lottery_subscription_keyboard (✅/❌ per game).
func lotterySubKeyboard(tier, targetChatID string, subscribed map[string]bool) *safew.InlineKeyboardMarkup {
	mk := func(text, action string) safew.InlineKeyboardButton {
		return btnScoped(tier, targetChatID, text, action)
	}
	all := "✅ 开启全部播报"
	if len(subscribed) > 0 {
		all = "⛔ 关闭全部播报"
	}
	rows := [][]safew.InlineKeyboardButton{{mk(all, "la")}}
	var line []safew.InlineKeyboardButton
	for _, code := range lotterybroadcast.GameOrder {
		mark := "❌"
		if subscribed[code] {
			mark = "✅"
		}
		line = append(line, mk(mark+" "+lotterybroadcast.Games[code].Name, "lt:"+code))
		if len(line) == 2 {
			rows = append(rows, line)
			line = nil
		}
	}
	if len(line) > 0 {
		rows = append(rows, line)
	}
	rows = append(rows, []safew.InlineKeyboardButton{mk("⬅️ 返回菜单", "m")})
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

func (w worker) panelStateForChat(chatID string, asGroup bool) panelState {
	st := panelState{IsGroup: asGroup}
	if !asGroup {
		return st
	}
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

// elevatePrivateRole promotes a DM caller who is a group admin of any group so
// the private /菜单 shows group-admin controls (RoleInChat alone would be none).
func (w worker) elevatePrivateRole(userID string, chat safew.Chat, role string) string {
	if isGroupChat(chat) || role != botperm.RoleNone {
		return role
	}
	list, err := w.perms.ListGroupAdminsForUser(userID)
	if err == nil && len(list) > 0 {
		return botperm.RoleGroupAdmin
	}
	return role
}

func (w worker) listNamedGroupsForPick() ([]namedGroup, error) {
	const pageSize = 500
	var out []namedGroup
	for offset := 0; ; offset += pageSize {
		rows, total, err := w.perms.ListGroups(offset, pageSize)
		if err != nil {
			return nil, err
		}
		for _, g := range rows {
			if isPrivateGroupRow(g) {
				continue
			}
			out = append(out, namedGroup{ChatID: g.ChatID, Title: g.Title})
		}
		if len(rows) < pageSize || int64(offset+pageSize) >= total {
			break
		}
	}
	return out, nil
}

func (w worker) listAdminGroupsForUser(userID string) ([]namedGroup, error) {
	list, err := w.perms.ListGroupAdminsForUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]namedGroup, 0, len(list))
	seen := map[string]bool{}
	for _, a := range list {
		if a.ChatID == "" || seen[a.ChatID] {
			continue
		}
		seen[a.ChatID] = true
		title := ""
		if s, err := w.perms.GetGroupSettings(a.ChatID); err == nil {
			title = s.Title
			if isPrivateGroupRow(s) {
				continue
			}
		}
		out = append(out, namedGroup{ChatID: a.ChatID, Title: title})
	}
	return out, nil
}

func (w worker) privateHomeMarkup(tier, userID, role string) *safew.InlineKeyboardMarkup {
	showExport := role == botperm.RoleDeveloper
	showGroupPick := tier == tierSuper
	var adminGroups []namedGroup
	if tier == tierGroup {
		adminGroups, _ = w.listAdminGroupsForUser(userID)
	}
	return privateHomeKeyboard(tier, adminGroups, showGroupPick, showExport)
}

func groupPanelText(tier, targetChatID, title string) string {
	label := strings.TrimSpace(title)
	if label == "" {
		label = "(无标题)"
	}
	head := fmt.Sprintf("📖 群组控制面板\n群：%s\nID：%s\n\n", label, targetChatID)
	return head + helpForRole(roleForTier(tier))
}

// sendPanel replies with the role-specific help text plus the inline panel.
func (w worker) sendPanel(ctx context.Context, token string, chat safew.Chat, userID, role string) error {
	role = w.elevatePrivateRole(userID, chat, role)
	tier := tierForRole(role)
	var markup *safew.InlineKeyboardMarkup
	text := helpForRole(role)
	if isGroupChat(chat) {
		markup = panelKeyboard(tier, w.panelStateForChat(chat.IDString(), true), "")
	} else {
		markup = w.privateHomeMarkup(tier, userID, role)
	}
	_, err := w.client.SendMessageEx(ctx, token, chat.IDString(), html.EscapeString(text),
		safew.SendOptions{ReplyMarkup: markup})
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
	tier, targetChat, action, ok := parseCallbackData(cq.Data)
	if !ok || actionLevel(action) == levelInvalid {
		answer("该按钮已失效，请重新发送 /菜单", false)
		return
	}
	msgChat := cq.Message.Chat
	msgChatID := msgChat.IDString()
	userID := cq.From.IDString()
	authChat := msgChatID
	if targetChat != "" {
		authChat = targetChat
	}
	_ = w.perms.EnsureGroup(authChat)
	allowed, err := authorizeCallback(w.perms, userID, authChat, action)
	if err != nil {
		answer(shortErr(err), true)
		return
	}
	if !allowed {
		answer("权限不足", false)
		return
	}
	toast, alert, view, err := w.runPanelAction(ctx, token, msgChat, targetChat, cq.Message.MessageID, userID, tier, action)
	if err != nil {
		log.Printf("按钮操作失败 chat=%s target=%s data=%s: %v", msgChatID, targetChat, cq.Data, err)
		answer(shortErr(err), true)
		return
	}
	answer(toast, alert)
	w.refreshPanel(ctx, token, msgChat, cq.Message.MessageID, userID, tier, targetChat, view)
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
	viewKeep      panelView = iota // main panel: refresh keyboard only
	viewMain                       // switch text + keyboard back to the main panel
	viewSubs                       // per-game subscription menu
	viewHistPick                   // private 开奖历史 game picker
	viewGroupPick                  // developer/super 群组选择
	viewNone                       // leave the message untouched
)

func (w worker) refreshPanel(ctx context.Context, token string, msgChat safew.Chat, messageID int, userID, tier, targetChat string, view panelView) {
	msgChatID := msgChat.IDString()
	inGroup := isGroupChat(msgChat)
	var err error
	switch view {
	case viewKeep:
		if targetChat != "" {
			err = w.client.EditMessageReplyMarkup(ctx, token, msgChatID, messageID,
				panelKeyboard(tier, w.panelStateForChat(targetChat, true), targetChat))
		} else if inGroup {
			err = w.client.EditMessageReplyMarkup(ctx, token, msgChatID, messageID,
				panelKeyboard(tier, w.panelStateForChat(msgChatID, true), ""))
		} else {
			role := roleForTier(tier)
			if tier == tierSuper {
				// distinguish developer export button via IsDeveloper
				if w.perms.IsDeveloper(userID) {
					role = botperm.RoleDeveloper
				}
			}
			err = w.client.EditMessageReplyMarkup(ctx, token, msgChatID, messageID, w.privateHomeMarkup(tier, userID, role))
		}
	case viewMain:
		if targetChat != "" {
			title := ""
			if s, e := w.perms.GetGroupSettings(targetChat); e == nil {
				title = s.Title
			}
			err = w.client.EditMessageText(ctx, token, msgChatID, messageID,
				html.EscapeString(groupPanelText(tier, targetChat, title)),
				panelKeyboard(tier, w.panelStateForChat(targetChat, true), targetChat))
		} else if inGroup {
			err = w.client.EditMessageText(ctx, token, msgChatID, messageID,
				html.EscapeString(helpForRole(roleForTier(tier))),
				panelKeyboard(tier, w.panelStateForChat(msgChatID, true), ""))
		} else {
			role := roleForTier(tier)
			if tier == tierSuper && w.perms.IsDeveloper(userID) {
				role = botperm.RoleDeveloper
			}
			err = w.client.EditMessageText(ctx, token, msgChatID, messageID,
				html.EscapeString(helpForRole(role)),
				w.privateHomeMarkup(tier, userID, role))
		}
	case viewSubs:
		opChat := msgChatID
		if targetChat != "" {
			opChat = targetChat
		}
		set := w.subscribedSet(opChat)
		err = w.client.EditMessageText(ctx, token, msgChatID, messageID, html.EscapeString(lotterySubText(set)), lotterySubKeyboard(tier, targetChat, set))
	case viewHistPick:
		err = w.client.EditMessageText(ctx, token, msgChatID, messageID,
			html.EscapeString("📜 开奖历史\n\n请选择彩种："), historyPickKeyboard(tier))
	case viewGroupPick:
		page := 0
		groups, gerr := w.listNamedGroupsForPick()
		if gerr != nil {
			log.Printf("群组选择列表失败: %v", gerr)
		}
		err = w.client.EditMessageText(ctx, token, msgChatID, messageID,
			html.EscapeString("📂 群组选择\n\n点选一个群以打开该群的控制面板（推送/冠军/亚军/订阅/广告）："),
			groupPickKeyboard(tier, groups, page))
	}
	if err != nil && !safew.IsNotModified(err) {
		log.Printf("刷新按钮面板失败 chat=%s: %v", msgChatID, err)
	}
}

var errGroupOnly = errors.New("开奖播报只能订阅到群组")

// runPanelAction executes an authorized action and returns the toast text.
// targetChat is the group id from callback_data when acting from a DM; empty means the message chat.
func (w worker) runPanelAction(ctx context.Context, token string, msgChat safew.Chat, targetChat string, messageID int, userID, tier, action string) (toast string, alert bool, view panelView, err error) {
	msgChatID := msgChat.IDString()
	opChatID := msgChatID
	if targetChat != "" {
		opChatID = targetChat
	}
	opIsGroup := targetChat != "" || isGroupChat(msgChat)

	if strings.HasPrefix(action, "lh:") {
		parts := strings.Split(action, ":")
		page, perr := 0, error(nil)
		if len(parts) == 3 {
			page, perr = strconv.Atoi(parts[2])
		}
		if len(parts) != 3 || perr != nil || w.lb == nil {
			return "无效页码", false, viewNone, nil
		}
		return "", false, viewNone, w.editHistoryPage(ctx, token, msgChatID, messageID, parts[1], page)
	}
	if action == "gs" || strings.HasPrefix(action, "gs:") {
		if strings.HasPrefix(action, "gs:") {
			page, _ := strconv.Atoi(strings.TrimPrefix(action, "gs:"))
			groups, gerr := w.listNamedGroupsForPick()
			if gerr != nil {
				return "", false, viewNone, gerr
			}
			if err := w.client.EditMessageText(ctx, token, msgChatID, messageID,
				html.EscapeString("📂 群组选择\n\n点选一个群以打开该群的控制面板（推送/冠军/亚军/订阅/广告）："),
				groupPickKeyboard(tier, groups, page)); err != nil && !safew.IsNotModified(err) {
				return "", false, viewNone, err
			}
			return "请选择群组", false, viewNone, nil
		}
		return "请选择群组", false, viewGroupPick, nil
	}
	switch action {
	case "m":
		if targetChat != "" {
			return "已打开群面板", false, viewMain, nil
		}
		return "已刷新", false, viewMain, nil
	case "q:lt":
		if w.lb == nil {
			return "", false, viewNone, errors.New("开奖播报未启用")
		}
		go w.runAsync(ctx, token, msgChatID, func() error { return w.lotteryQuery(ctx, token, msgChat, "") })
		return "正在查询最新开奖…", false, viewNone, nil
	case "q:lh":
		return "请选择彩种", false, viewHistPick, nil
	case "q:ps":
		return w.statusSummary(opChatID), true, viewKeep, nil
	case "q:ls":
		return w.subscriptionSummary(opChatID), true, viewKeep, nil
	case "ex":
		go w.runAsync(ctx, token, msgChatID, func() error { return w.cmdExportAllMembers(ctx, token, msgChatID) })
		return "正在导出所有群成员…", false, viewNone, nil
	case "p:1", "p:0":
		_, toast, err := w.applyPush(opChatID, action == "p:1")
		return toast, false, viewKeep, err
	case "c6:1", "c6:0", "c7:1", "c7:0":
		size := 6
		if action[1] == '7' {
			size = 7
		}
		_, toast, err := w.applyCodeMode(opChatID, size, strings.HasSuffix(action, ":1"))
		return toast, false, viewKeep, err
	case "r6:1", "r6:0", "r7:1", "r7:0":
		size := 6
		if action[1] == '7' {
			size = 7
		}
		_, toast, err := w.applyRunnerUpCodeMode(opChatID, size, strings.HasSuffix(action, ":1"))
		return toast, false, viewKeep, err
	case "c:0":
		if _, _, err := w.applyCodeMode(opChatID, 6, false); err != nil {
			return "", false, viewKeep, err
		}
		if _, _, err := w.applyCodeMode(opChatID, 7, false); err != nil {
			return "", false, viewKeep, err
		}
		return "已关闭冠军推送", false, viewKeep, nil
	case "r:0":
		if _, _, err := w.applyRunnerUpCodeMode(opChatID, 6, false); err != nil {
			return "", false, viewKeep, err
		}
		if _, _, err := w.applyRunnerUpCodeMode(opChatID, 7, false); err != nil {
			return "", false, viewKeep, err
		}
		return "已关闭亚军推送", false, viewKeep, nil
	case "ad":
		s, err := ads.LoadForChat(w.db, opChatID)
		if err != nil {
			return "", false, viewKeep, err
		}
		body := fmt.Sprintf("scope=%s\nprefix:\n%s\n\nsuffix:\n%s", s.Scope, emptyMark(s.PrefixAd), emptyMark(s.SuffixAd))
		if err := w.reply(ctx, token, msgChatID, body); err != nil {
			return "", false, viewKeep, err
		}
		return "已发送广告状态", false, viewKeep, nil
	}
	// lottery subscription actions (group only — including DM with target chat)
	if w.lb == nil {
		return "", false, viewNone, errors.New("开奖播报未启用")
	}
	if !opIsGroup {
		return "", false, viewNone, errGroupOnly
	}
	switch {
	case action == "lb:1":
		if err := w.lb.store.AddSubscription(opChatID, "all", userID); err != nil {
			return "", false, viewKeep, err
		}
		return "✅ 已订阅全部彩种，新期开奖后自动播报并置顶", false, viewKeep, nil
	case action == "lb:0":
		n, err := w.lb.store.RemoveSubscription(opChatID, "")
		if err != nil {
			return "", false, viewKeep, err
		}
		return fmt.Sprintf("已取消 %d 条开奖订阅", n), false, viewKeep, nil
	case action == "ls":
		return "请选择要订阅的彩种", false, viewSubs, nil
	case action == "la":
		set := w.subscribedSet(opChatID)
		var codes []string
		if len(set) == 0 {
			codes = []string{"all"}
		}
		if err := w.lb.store.SetSubscribedCodes(opChatID, codes, userID); err != nil {
			return "", false, viewSubs, err
		}
		return "开奖订阅已更新", false, viewSubs, nil
	case strings.HasPrefix(action, "lt:"):
		code := strings.TrimPrefix(action, "lt:")
		set := w.subscribedSet(opChatID)
		if set[code] {
			delete(set, code)
		} else {
			set[code] = true
		}
		codes := make([]string, 0, len(set))
		for c := range set {
			codes = append(codes, c)
		}
		if err := w.lb.store.SetSubscribedCodes(opChatID, codes, userID); err != nil {
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
