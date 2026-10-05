package safewbot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/lotterybroadcast"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"
)

const (
	lotteryPollInterval   = 5 * time.Second  // Python: run_repeating(poll_lottery_results, interval=5)
	lotterySendInterval   = 3 * time.Second  // Python: outbox processed every 3s
	lotterySendBatch      = 15               // Python: pending_outbox(15)
	lotteryMaxAttempts    = 3                // give up on a chat after 3 failed sends
	lotteryOutboxMaxAge   = 6 * time.Hour    // stale broadcasts (e.g. bot was down) are dropped
	lotteryQueryThrottle  = 20 * time.Second // per-game refresh throttle for /开奖 queries
	lotteryMessageMaxRune = 3500
)

// lotteryRuntime holds the 开奖播报 state that lives inside safew-bot (it owns
// the bot token, so polling + sending + pinning happen in the same process).
type lotteryRuntime struct {
	store   *lotterybroadcast.Store
	service *lotterybroadcast.Service
	poller  *lotterybroadcast.Poller
	nudge   chan struct{}

	mu        sync.Mutex
	lastQuery map[string]time.Time
}

func newLotteryRuntime(store *lotterybroadcast.Store, service *lotterybroadcast.Service) *lotteryRuntime {
	rt := &lotteryRuntime{store: store, service: service, nudge: make(chan struct{}, 1), lastQuery: map[string]time.Time{}}
	rt.poller = &lotterybroadcast.Poller{
		Repo:     store,
		Fetcher:  service,
		Switches: store.LoadSwitches,
		OnQueued: func(int) { rt.wake() },
	}
	return rt
}

func (rt *lotteryRuntime) wake() {
	select {
	case rt.nudge <- struct{}{}:
	default:
	}
}

// dueForQuery returns the codes not refreshed by a query in the last lotteryQueryThrottle.
func (rt *lotteryRuntime) dueForQuery(codes []string, now time.Time) []string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	var due []string
	for _, c := range codes {
		if now.Sub(rt.lastQuery[c]) >= lotteryQueryThrottle {
			rt.lastQuery[c] = now
			due = append(due, c)
		}
	}
	return due
}

// runLotteryPoller polls draw results every 5s (fast window near draw time,
// 300s safety poll otherwise) and enqueues new issues for subscribed groups.
func (w worker) runLotteryPoller(ctx context.Context) error {
	if w.lb == nil {
		return nil
	}
	ticker := time.NewTicker(lotteryPollInterval)
	defer ticker.Stop()
	for {
		tickCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		if _, err := w.lb.poller.Tick(tickCtx); err != nil && ctx.Err() == nil {
			log.Printf("开奖轮询失败: %v", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// runLotterySender delivers queued broadcasts every 3s (or immediately when the
// poller queued something) and pins each delivered message.
func (w worker) runLotterySender(ctx context.Context) error {
	if w.lb == nil {
		return nil
	}
	ticker := time.NewTicker(lotterySendInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-w.lb.nudge:
		}
		w.flushLotteryOutbox(ctx)
	}
}

func (w worker) flushLotteryOutbox(ctx context.Context) {
	items, err := w.lb.store.PendingOutbox(lotterySendBatch)
	if err != nil {
		log.Printf("读取开奖播报队列失败: %v", err)
		return
	}
	if len(items) == 0 {
		return
	}
	botConfig, err := systemconfig.LoadSafeW(w.db)
	if err != nil {
		log.Printf("开奖播报等待 SafeW 配置: %v", err)
		return
	}
	sw, err := w.lb.store.LoadSwitches()
	if err != nil {
		sw = lotterybroadcast.DefaultSwitches()
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		if !item.CreatedAt.IsZero() && time.Since(item.CreatedAt) > lotteryOutboxMaxAge {
			_ = w.lb.store.FinishOutbox(item.ID, false, 0, false, "播报已过期，未发送", 0)
			continue
		}
		msgID, pinned, err := w.deliverBroadcast(ctx, botConfig.Token, item.ChatID, item.Body, sw.WithAds)
		if err != nil {
			log.Printf("开奖播报发送失败 chat=%s game=%s issue=%s: %v", item.ChatID, item.GameCode, item.Issue, err)
			_ = w.lb.store.FinishOutbox(item.ID, false, 0, false, err.Error(), lotteryMaxAttempts)
			continue
		}
		note := ""
		if !pinned {
			note = "已发送，置顶失败（请把机器人设为管理员并授予置顶权限）"
		}
		if err := w.lb.store.FinishOutbox(item.ID, true, int64(msgID), pinned, note, lotteryMaxAttempts); err != nil {
			log.Printf("更新开奖播报状态失败 id=%d: %v", item.ID, err)
		}
		log.Printf("开奖播报已发送 chat=%s game=%s issue=%s message_id=%d pinned=%v", item.ChatID, item.GameCode, item.Issue, msgID, pinned)
	}
}

// pinner is the subset of the SafeW client needed to deliver one broadcast.
type broadcastClient interface {
	SendMessageEx(ctx context.Context, token, chatID, text string, opts safew.SendOptions) (safew.Message, error)
	PinChatMessage(ctx context.Context, token, chatID string, messageID int, disableNotification bool) error
}

// sendAndPin sends a broadcast and pins it. A pin failure (bot not admin / no
// pin right) is logged and never fails the broadcast.
func sendAndPin(ctx context.Context, c broadcastClient, token, chatID, text string) (int, bool, error) {
	msg, err := c.SendMessageEx(ctx, token, chatID, text, safew.SendOptions{})
	if err != nil {
		return 0, false, err
	}
	if msg.MessageID == 0 {
		log.Printf("开奖播报置顶跳过 chat=%s: 接口未返回 message_id", chatID)
		return 0, false, nil
	}
	if err := c.PinChatMessage(ctx, token, chatID, msg.MessageID, false); err != nil {
		log.Printf("开奖播报置顶失败 chat=%s message_id=%d: %v（机器人需为群管理员并有置顶权限）", chatID, msg.MessageID, err)
		return msg.MessageID, false, nil
	}
	return msg.MessageID, true, nil
}

func (w worker) deliverBroadcast(ctx context.Context, token, chatID, body string, withAds bool) (int, bool, error) {
	text := html.EscapeString(body)
	if withAds {
		wrapped, err := ads.Wrap(w.db, chatID, text)
		if err != nil {
			return 0, false, err
		}
		text = wrapped
	}
	return sendAndPin(ctx, w.client, token, chatID, text)
}

// sendLotteryText sends escaped text, optionally wrapped with the chat's ads.
func (w worker) sendLotteryText(ctx context.Context, token, chatID, body string, withAds bool, markup *safew.InlineKeyboardMarkup) error {
	text := html.EscapeString(body)
	if withAds {
		wrapped, err := ads.Wrap(w.db, chatID, text)
		if err != nil {
			return err
		}
		text = wrapped
	}
	_, err := w.client.SendMessageEx(ctx, token, chatID, text, safew.SendOptions{ReplyMarkup: markup})
	return err
}

// runAsync runs a slow command (network fetch) off the update loop; failures
// still reply 「命令失败: …」.
func (w worker) runAsync(ctx context.Context, token, chatID string, fn func() error) {
	if err := fn(); err != nil && ctx.Err() == nil {
		log.Printf("处理命令失败: %v", err)
		_ = w.replyPlain(ctx, token, chatID, html.EscapeString("命令失败: "+err.Error()))
	}
}

func (w worker) lotterySwitches() lotterybroadcast.Switches {
	sw, err := w.lb.store.LoadSwitches()
	if err != nil {
		log.Printf("读取开奖播报开关失败，使用默认: %v", err)
		return lotterybroadcast.DefaultSwitches()
	}
	return sw
}

// refreshForQuery refreshes the given games (throttled). New issues found this
// way are broadcast like the poller would (Python: refresh(broadcast=True)).
func (w worker) refreshForQuery(ctx context.Context, codes []string, sw lotterybroadcast.Switches, broadcast bool) map[string]string {
	due := w.lb.dueForQuery(codes, time.Now())
	if len(due) == 0 {
		return nil
	}
	allow := sw.AllowBroadcast
	if !broadcast {
		allow = func(string) bool { return false }
	}
	rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := lotterybroadcast.Refresh(rctx, w.lb.store, w.lb.service, due, allow)
	if err != nil {
		return map[string]string{"db": err.Error()}
	}
	if out.Queued > 0 {
		w.lb.wake()
	}
	return out.Errors
}

func firstError(errs map[string]string) string {
	for _, code := range lotterybroadcast.GameOrder {
		if e, ok := errs[code]; ok {
			return e
		}
	}
	for _, e := range errs {
		return e
	}
	return "数据源尚未初始化"
}

// chunkText splits blocks into messages under the SafeW length limit.
func chunkText(blocks []string, sep string, max int) []string {
	var out []string
	var cur strings.Builder
	for _, b := range blocks {
		if cur.Len() > 0 && len([]rune(cur.String()))+len([]rune(sep))+len([]rune(b)) > max {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteString(sep)
		}
		cur.WriteString(b)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func (w worker) lotteryCodesForChat(chatID string) []string {
	codes, err := w.lb.store.SubscribedCodesForChat(chatID)
	if err != nil || len(codes) == 0 {
		return append([]string(nil), lotterybroadcast.GameOrder...)
	}
	return codes
}

// lotteryQuery implements /lottery /开奖 [彩种] (anyone).
func (w worker) lotteryQuery(ctx context.Context, token string, chat safew.Chat, raw string) error {
	chatID := chat.IDString()
	sw := w.lotterySwitches()
	if !sw.QueryEnabled {
		return w.replyPlain(ctx, token, chatID, "开奖结果查询目前已关闭。")
	}
	var codes []string
	raw = strings.TrimSpace(raw)
	if raw == "" {
		codes = w.lotteryCodesForChat(chatID)
	} else {
		selector := lotterybroadcast.ResolveCode(raw)
		if selector == "" {
			return w.replyPlain(ctx, token, chatID, "未知彩种。可选现有福彩、体彩、香港六合彩、澳门六合彩和新澳六合彩。")
		}
		codes = lotterybroadcast.CodesForSelector(selector)
	}
	errs := w.refreshForQuery(ctx, codes, sw, true)
	rows, err := w.lb.store.LatestResults(codes)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return w.replyPlain(ctx, token, chatID, html.EscapeString("暂时无法取得开奖结果："+firstError(errs)))
	}
	blocks := make([]string, 0, len(rows)+1)
	for _, row := range rows {
		blocks = append(blocks, lotterybroadcast.FormatResult(row.ToResult()))
	}
	if len(errs) > 0 {
		blocks = append(blocks, fmt.Sprintf("部分数据源暂不可用（%d 个彩种），已显示数据库内最近结果。", len(errs)))
	}
	for _, text := range chunkText(blocks, "\n\n", lotteryMessageMaxRune) {
		if err := w.sendLotteryText(ctx, token, chatID, text, sw.WithAds, nil); err != nil {
			return err
		}
	}
	return nil
}

// lotteryKeyword implements the group keyword 「开奖」: only for groups with
// subscriptions; replies with the subscribed games' latest results, no broadcast.
func (w worker) lotteryKeyword(ctx context.Context, token string, chat safew.Chat) error {
	chatID := chat.IDString()
	codes, err := w.lb.store.SubscribedCodesForChat(chatID)
	if err != nil || len(codes) == 0 {
		return err
	}
	sw := w.lotterySwitches()
	if !sw.QueryEnabled {
		return nil
	}
	// Python refreshes without broadcasting here; we allow it so a draw first
	// seen via this keyword is still broadcast to subscribers (never twice).
	errs := w.refreshForQuery(ctx, codes, sw, true)
	rows, err := w.lb.store.LatestResults(codes)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return w.replyPlain(ctx, token, chatID, html.EscapeString("暂时无法取得已开启彩种的开奖结果："+firstError(errs)))
	}
	blocks := make([]string, 0, len(rows))
	for i, row := range rows {
		prefix := ""
		if row.GameCode != "ssq" && i == 0 {
			prefix = lotterybroadcast.BroadcastPrefix
		}
		blocks = append(blocks, prefix+lotterybroadcast.FormatResult(row.ToResult()))
	}
	for _, text := range chunkText(blocks, "\n\n", lotteryMessageMaxRune) {
		if err := w.sendLotteryText(ctx, token, chatID, text, sw.WithAds, nil); err != nil {
			return err
		}
	}
	return nil
}

func historyKeyboard(code string, page, pageCount int) *safew.InlineKeyboardMarkup {
	if pageCount <= 1 {
		return nil
	}
	var row []safew.InlineKeyboardButton
	if page > 0 {
		row = append(row, btn(tierUser, "上一页", fmt.Sprintf("lh:%s:%d", code, page-1)))
	}
	if page+1 < pageCount {
		row = append(row, btn(tierUser, "下一页", fmt.Sprintf("lh:%s:%d", code, page+1)))
	}
	return &safew.InlineKeyboardMarkup{InlineKeyboard: [][]safew.InlineKeyboardButton{row}}
}

func (w worker) storedHistory(code string) ([]lotterybroadcast.Result, error) {
	rows, err := w.lb.store.History(code, 100)
	if err != nil {
		return nil, err
	}
	out := make([]lotterybroadcast.Result, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToResult())
	}
	return out, nil
}

// lotteryHistory implements /lotteryhistory /开奖历史 彩种 (anyone).
func (w worker) lotteryHistory(ctx context.Context, token string, chat safew.Chat, raw string) error {
	chatID := chat.IDString()
	code := lotterybroadcast.ResolveCode(raw)
	if _, ok := lotterybroadcast.Games[code]; !ok {
		return w.replyPlain(ctx, token, chatID, "用法：/开奖历史 彩种（如 双色球、快乐8、香港六合彩、澳门六合彩、新澳六合彩）")
	}
	sw := w.lotterySwitches()
	if !sw.QueryEnabled {
		return w.replyPlain(ctx, token, chatID, "开奖结果查询目前已关闭。")
	}
	source := lotterybroadcast.Games[code].Source
	fetchErr := ""
	hctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	results, err := w.lb.service.History(hctx, code, 100)
	cancel()
	if err != nil {
		fetchErr = err.Error()
		_ = w.lb.store.SetSourceStatus(source, false, fetchErr)
	} else {
		// History is saved without broadcasting (same as Python send_lottery_history).
		// Rows newer than the stored latest issue are skipped so a history query
		// can never swallow a fresh draw before the poller broadcasts it.
		known, err := w.lb.store.LatestIssue(code)
		if err != nil {
			return err
		}
		for _, r := range results {
			if known != "" && lotterybroadcast.IsNewerIssue(r.Issue, known) {
				continue
			}
			if !lotterybroadcast.IsValidResult(r) {
				continue
			}
			nr, nerr := lotterybroadcast.NormalizeResult(r)
			if nerr != nil {
				continue
			}
			if _, err := w.lb.store.SaveResult(nr); err != nil {
				return err
			}
		}
		_ = w.lb.store.SetSourceStatus(source, true, "")
	}
	rows, err := w.storedHistory(code)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		if fetchErr == "" {
			fetchErr = "数据源尚未初始化"
		}
		return w.replyPlain(ctx, token, chatID, html.EscapeString("暂时无法取得历史开奖："+fetchErr))
	}
	text, page, pageCount := lotterybroadcast.HistoryPage(code, rows, 0)
	if fetchErr != "" {
		text += "\n\n当前数据源暂不可用，以上为最近一次成功缓存。"
	}
	_, err = w.client.SendMessageEx(ctx, token, chatID, html.EscapeString(text), safew.SendOptions{ReplyMarkup: historyKeyboard(code, page, pageCount)})
	return err
}

// editHistoryPage handles the 上一页/下一页 buttons of a history message.
func (w worker) editHistoryPage(ctx context.Context, token, chatID string, messageID int, code string, page int) error {
	if _, ok := lotterybroadcast.Games[code]; !ok {
		return errors.New("未知彩种")
	}
	rows, err := w.storedHistory(code)
	if err != nil {
		return err
	}
	text, page, pageCount := lotterybroadcast.HistoryPage(code, rows, page)
	err = w.client.EditMessageText(ctx, token, chatID, messageID, html.EscapeString(text), historyKeyboard(code, page, pageCount))
	if safew.IsNotModified(err) {
		return nil
	}
	return err
}

// lotteryControlCheck enforces: group chat only + CanControlSensitive.
func (w worker) lotteryControlCheck(ctx context.Context, token string, chat safew.Chat, userID string) (bool, error) {
	chatID := chat.IDString()
	if !isGroupChat(chat) {
		return false, w.replyPlain(ctx, token, chatID, "开奖播报只能订阅到群组，请在群内使用。")
	}
	ok, err := w.perms.CanControlSensitive(userID, chatID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, w.replyPlain(ctx, token, chatID, "权限不足（需要群管理员、超级管理员或开发者）")
	}
	return true, nil
}

// lotterySubscribe implements /lotterysub /订阅开奖 [全部|福彩|体彩|六合彩|彩种].
func (w worker) lotterySubscribe(ctx context.Context, token string, chat safew.Chat, userID, raw string) error {
	if ok, err := w.lotteryControlCheck(ctx, token, chat, userID); !ok || err != nil {
		return err
	}
	if strings.TrimSpace(raw) == "" {
		raw = "全部"
	}
	selector := lotterybroadcast.ResolveCode(raw)
	if !lotterybroadcast.IsSelector(selector) {
		return w.replyPlain(ctx, token, chat.IDString(), "用法：/订阅开奖 全部、福彩、体彩、六合彩或彩种名称")
	}
	if err := w.lb.store.AddSubscription(chat.IDString(), selector, userID); err != nil {
		return err
	}
	return w.replyPlain(ctx, token, chat.IDString(), fmt.Sprintf("已订阅：%s。新期开奖后将自动播报并置顶（机器人需为群管理员并有置顶权限）。", lotterybroadcast.SelectorLabel(selector)))
}

// lotteryUnsubscribe implements /lotteryunsub /取消订阅开奖 [全部|…].
func (w worker) lotteryUnsubscribe(ctx context.Context, token string, chat safew.Chat, userID, raw string) error {
	if ok, err := w.lotteryControlCheck(ctx, token, chat, userID); !ok || err != nil {
		return err
	}
	if strings.TrimSpace(raw) == "" {
		raw = "全部"
	}
	selector := lotterybroadcast.ResolveCode(raw)
	if !lotterybroadcast.IsSelector(selector) {
		return w.replyPlain(ctx, token, chat.IDString(), "用法：/取消订阅开奖 全部、福彩、体彩或彩种名称")
	}
	target := selector
	if selector == "all" {
		target = ""
	}
	n, err := w.lb.store.RemoveSubscription(chat.IDString(), target)
	if err != nil {
		return err
	}
	return w.replyPlain(ctx, token, chat.IDString(), fmt.Sprintf("已取消 %d 条开奖订阅。", n))
}

// lotterySubscriptions implements /lotterysubs /开奖订阅 (anyone, group only).
func (w worker) lotterySubscriptions(ctx context.Context, token string, chat safew.Chat) error {
	chatID := chat.IDString()
	if !isGroupChat(chat) {
		return w.replyPlain(ctx, token, chatID, "该命令只能在群组中使用。")
	}
	rows, err := w.lb.store.Subscriptions(chatID, 100)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return w.replyPlain(ctx, token, chatID, "当前群没有开奖播报订阅。")
	}
	var b strings.Builder
	b.WriteString("当前开奖订阅：")
	for _, r := range rows {
		b.WriteString("\n- " + lotterybroadcast.SelectorLabel(r.Selector))
	}
	return w.replyPlain(ctx, token, chatID, html.EscapeString(b.String()))
}

// handleLotteryCommand dispatches the 开奖播报 commands; handled=false when cmd is not one of them.
func (w worker) handleLotteryCommand(ctx context.Context, botConfig systemconfig.SafeW, chat safew.Chat, userID, cmd, args string) (bool, error) {
	switch cmd {
	case "/lottery", "/开奖", "/lotteryhistory", "/开奖历史", "/lotterysub", "/订阅开奖",
		"/lotteryunsub", "/取消订阅开奖", "/lotterysubs", "/开奖订阅":
	default:
		return false, nil
	}
	token := botConfig.Token
	if w.lb == nil {
		return true, w.replyPlain(ctx, token, chat.IDString(), "开奖播报未启用")
	}
	switch cmd {
	case "/lottery", "/开奖":
		go w.runAsync(ctx, token, chat.IDString(), func() error { return w.lotteryQuery(ctx, token, chat, args) })
		return true, nil
	case "/lotteryhistory", "/开奖历史":
		go w.runAsync(ctx, token, chat.IDString(), func() error { return w.lotteryHistory(ctx, token, chat, args) })
		return true, nil
	case "/lotterysub", "/订阅开奖":
		return true, w.lotterySubscribe(ctx, token, chat, userID, args)
	case "/lotteryunsub", "/取消订阅开奖":
		return true, w.lotteryUnsubscribe(ctx, token, chat, userID, args)
	default:
		return true, w.lotterySubscriptions(ctx, token, chat)
	}
}

// handleLotteryKeyword handles non-command group text: 「开奖」 and 「<彩种>历史」.
func (w worker) handleLotteryKeyword(ctx context.Context, botConfig systemconfig.SafeW, chat safew.Chat, text string) bool {
	if w.lb == nil || !isGroupChat(chat) {
		return false
	}
	normalized := strings.Join(strings.Fields(text), " ")
	token := botConfig.Token
	chatID := chat.IDString()
	if normalized == "开奖" {
		go w.runAsync(ctx, token, chatID, func() error { return w.lotteryKeyword(ctx, token, chat) })
		return true
	}
	if code := lotterybroadcast.ResolveHistoryKeyword(normalized); code != "" {
		go w.runAsync(ctx, token, chatID, func() error {
			if !w.lotterySwitches().QueryEnabled {
				return nil
			}
			return w.lotteryHistory(ctx, token, chat, code)
		})
		return true
	}
	return false
}
