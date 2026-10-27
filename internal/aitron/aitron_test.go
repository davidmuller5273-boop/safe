package aitron

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

func TestFormatMatchesTemplate(t *testing.T) {
	s := Snapshot{
		NextBlock: 87473360, NextPick: Double, WindowPeriods: 180,
		Window: Rate{Hits: 88, Total: 179}, Overall: Rate{Hits: 4402, Total: 8966},
		MaxWin: intp(5), MaxLoss: intp(4),
		Recent: []Row{
			{87473300, Double, Single},
			{87473320, Single, Single},
		},
	}
	got := Format(AI, s)
	want := strings.Join([]string{
		"<b>波场一分单双</b>",
		"",
		"<b>AI预测（近15期）</b>",
		"<pre>区块号 | 结果 | 预测 | 状态",
		"87473300 | 🔴双 | 🟢单 | ❌",
		"87473320 | 🟢单 | 🟢单 | ✅",
		"87473360 | ⏳待开奖 | 🔴双 | ⏳</pre>",
		"────────────────",
		"本期预测：🔴双",
		"<b>📊 周期胜率概览</b>",
		"180期：49.2%（已预测 8966 期 49.1%）",
		"近三小时 最大连中 5✅ 最大连错 4❌",
		"走势预测统计网址  188.ms",
	}, "\n")
	if got != want {
		t.Fatalf("Format mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestFormatKeepsOnlyLast15AndNewestAtBottom(t *testing.T) {
	var rows []Row
	for i := 0; i < 20; i++ {
		rows = append(rows, Row{Block: int64(1000 + 20*i), Result: Single, Pick: Single})
	}
	got := Format(AI, Snapshot{NextBlock: 2000, NextPick: Single, Recent: rows})
	lines := strings.Split(got, "\n")
	var body []string
	for _, l := range lines {
		if strings.Contains(l, " | ") {
			body = append(body, l)
		}
	}
	if len(body) != 1+15+1 { // header + 15 history + pending
		t.Fatalf("table lines = %d: %v", len(body), body)
	}
	if !strings.HasPrefix(body[1], "1100 |") || !strings.HasPrefix(body[15], "1380 |") || !strings.HasPrefix(body[16], "2000 | ⏳待开奖") {
		t.Fatalf("order wrong: %v", body)
	}
}

func TestFormatNoDataAndTruncatedOverall(t *testing.T) {
	got := Format(AI, Snapshot{NextBlock: 20, NextPick: Single})
	if !strings.Contains(got, "180期：--（已预测 0 期 --）") || strings.Contains(got, "最大连中") || strings.Contains(got, "连中") {
		t.Fatalf("empty format: %s", got)
	}
	got = Format(AI, Snapshot{NextBlock: 20, NextPick: Single, Overall: Rate{Hits: 2500, Total: 5000}, OverallLimit: 5000})
	if !strings.Contains(got, "（近5000期 50.0%）") {
		t.Fatalf("truncated format: %s", got)
	}
}

// ---- fake tg-directory-bot ----

type fakeDraw struct {
	windowMax bool // ai.window carries server-computed max_win / max_loss
	periods   int  // number of recorded draws
	aiFrom    int  // index of the first draw with a stored AI prediction
	total     int  // total_recorded
	noNext    bool // ai.next missing (pick not generated yet)
	aiOff     bool // ai: null
	lastBlock int64
}

func (f fakeDraw) pick(i int) string {
	if (i*7+i/3)%2 == 0 {
		return Single
	}
	return Double
}
func (f fakeDraw) result(i int) string {
	if (i*5+i/4)%3 == 0 {
		return Double
	}
	return f.pick(i) // many hits
}

func (f fakeDraw) block(i int) int64 { return f.lastBlock - int64(f.periods-1-i)*20 }

func (f fakeDraw) json(aiLimit int) map[string]any {
	out := map[string]any{
		"total_recorded": f.total, "next_draw_block": f.lastBlock + 20,
		"recent": []map[string]any{{"block": f.lastBlock, "ts": time.Now().UnixMilli()}},
	}
	if f.aiOff {
		out["ai"] = nil
		return out
	}
	ai := map[string]any{"enabled": true, "next": nil, "trend": nil}
	if f.windowMax {
		ai["window"] = map[string]any{"limit": 180, "max_win": map[string]any{"len": 7, "start": 3}, "max_loss": 6}
	}
	if !f.noNext {
		ai["next"] = map[string]any{"block": f.lastBlock + 20, "pick": Double}
	}
	start := f.periods - aiLimit
	if start < 0 {
		start = 0
	}
	var items []map[string]any
	for i := start; i < f.periods; i++ {
		var pred any
		if i >= f.aiFrom {
			pred = f.pick(i)
		}
		items = append(items, map[string]any{"block": f.block(i), "predicted": pred, "result": f.result(i)})
	}
	ai["trend"] = map[string]any{"items": items}
	out["ai"] = ai
	return out
}

type fakeServer struct {
	*httptest.Server
	draw        fakeDraw
	proposed    string // body for the AI endpoint; "" = 404
	auto        string // body for the auto endpoint; "" = 404
	autoStatus  int    // when set, the auto endpoint answers this status
	autoHit     atomic.Int32
	heavyCalls  atomic.Int32
	lightCalls  atomic.Int32
	proposedHit atomic.Int32
}

func newFakeServer(t *testing.T, d fakeDraw) *fakeServer {
	fs := &fakeServer{draw: d}
	mux := http.NewServeMux()
	mux.HandleFunc(AI.Path, func(w http.ResponseWriter, r *http.Request) {
		fs.proposedHit.Add(1)
		if fs.proposed == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, fs.proposed)
	})
	mux.HandleFunc(Auto.Path, func(w http.ResponseWriter, r *http.Request) {
		fs.autoHit.Add(1)
		if fs.autoStatus != 0 {
			http.Error(w, "boom", fs.autoStatus)
			return
		}
		if fs.auto == "" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, fs.auto)
	})
	mux.HandleFunc("/api/tron-draw", func(w http.ResponseWriter, r *http.Request) {
		aiLimit := 0
		fmt.Sscan(r.URL.Query().Get("ai_limit"), &aiLimit)
		if aiLimit > 1 {
			fs.heavyCalls.Add(1)
		} else {
			fs.lightCalls.Add(1)
		}
		_ = json.NewEncoder(w).Encode(fs.draw.json(aiLimit))
	})
	fs.Server = httptest.NewServer(mux)
	t.Cleanup(fs.Close)
	return fs
}

func TestFallbackDerivesEverythingFromExistingAPI(t *testing.T) {
	d := fakeDraw{periods: 6000, aiFrom: 0, total: 8000, lastBlock: 87473340}
	fs := newFakeServer(t, d)
	snap, err := NewClient(fs.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.NextBlock != 87473360 || snap.NextPick != Double {
		t.Fatalf("next = %d %s", snap.NextBlock, snap.NextPick)
	}
	if len(snap.Recent) != 15 || snap.Recent[14].Block != 87473340 || snap.Recent[0].Block != 87473340-14*20 {
		t.Fatalf("recent = %+v", snap.Recent)
	}
	// independent recomputation
	var wHits, wTotal, oHits, oTotal int
	for i := d.periods - 5000; i < d.periods; i++ {
		ok := d.pick(i) == d.result(i)
		oTotal++
		if ok {
			oHits++
		}
		if i >= d.periods-180 {
			wTotal++
			if ok {
				wHits++
			}
		}
	}
	if snap.Window != (Rate{wHits, wTotal}) || snap.Overall != (Rate{oHits, oTotal}) {
		t.Fatalf("window=%+v overall=%+v want %d/%d %d/%d", snap.Window, snap.Overall, wHits, wTotal, oHits, oTotal)
	}
	if snap.OverallLimit != 5000 {
		t.Fatalf("OverallLimit = %d, want 5000 (truncated)", snap.OverallLimit)
	}
	// the old API has no window.max_win/max_loss: the line is omitted, never computed locally
	if snap.MaxWin != nil || snap.MaxLoss != nil {
		t.Fatalf("max streak must be absent in fallback without server values: %v %v", snap.MaxWin, snap.MaxLoss)
	}
	if strings.Contains(Format(AI, *snap), "连中") {
		t.Fatal("streak line must be omitted")
	}
}

func TestFallbackUsesServerProvidedMaxStreak(t *testing.T) {
	d := fakeDraw{periods: 300, total: 300, lastBlock: 87473340, windowMax: true}
	fs := newFakeServer(t, d)
	snap, err := NewClient(fs.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.MaxWin == nil || snap.MaxLoss == nil || *snap.MaxWin != 7 || *snap.MaxLoss != 6 {
		t.Fatalf("max = %v %v", snap.MaxWin, snap.MaxLoss)
	}
	if !strings.Contains(Format(AI, *snap), "近三小时 最大连中 7✅ 最大连错 6❌") {
		t.Fatal(Format(AI, *snap))
	}
}

func TestFallbackSkipsPeriodsWithoutStoredPrediction(t *testing.T) {
	// AI only started 4 periods before the end; total_recorded == len(items) -> not truncated.
	d := fakeDraw{periods: 50, aiFrom: 46, total: 50, lastBlock: 1000}
	fs := newFakeServer(t, d)
	snap, err := NewClient(fs.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Recent) != 4 || snap.Overall.Total != 4 || snap.Window.Total != 4 || snap.OverallLimit != 0 {
		t.Fatalf("snap = %+v", snap)
	}
}

func TestFallbackLightProbeAvoidsRepeatedHeavyFetch(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 300, total: 300, lastBlock: 1000})
	c := NewClient(fs.URL)
	for i := 0; i < 5; i++ {
		if _, err := c.Fetch(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if fs.heavyCalls.Load() != 1 || fs.lightCalls.Load() != 5 {
		t.Fatalf("heavy=%d light=%d", fs.heavyCalls.Load(), fs.lightCalls.Load())
	}
	fs.draw.periods, fs.draw.lastBlock = 301, 1020 // new period
	if _, err := c.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fs.heavyCalls.Load() != 2 {
		t.Fatalf("heavy=%d, want 2 after a new period", fs.heavyCalls.Load())
	}
}

func TestFetchPendingAndUnavailable(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000, noNext: true})
	if _, err := NewClient(fs.URL).Fetch(context.Background()); !errors.Is(err, ErrPending) {
		t.Fatalf("err = %v, want ErrPending", err)
	}
	fs.draw = fakeDraw{periods: 30, total: 30, lastBlock: 1000, aiOff: true}
	_, err := NewClient(fs.URL).Fetch(context.Background())
	var ue *UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %v, want UnavailableError", err)
	}
	// service down
	url := fs.URL
	fs.Close()
	if _, err := NewClient(url).Fetch(context.Background()); err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestProposedEndpointPreferredWhenPresent(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000})
	fs.proposed = `{"enabled":true,"next_block":87473360,"next_pick":"双","latest_ts":0,
	 "recent":[{"block":87473340,"result":"单","pick":"单"},{"block":87473320,"result":"双","pick":"单"}],
	 "window":{"periods":180,"hits":88,"total":179,"max_win":9,"max_loss":0},"overall":{"hits":4402,"total":8966},
	 "streak":{"kind":"win","count":3}}`
	snap, err := NewClient(fs.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.NextBlock != 87473360 || snap.Overall.Total != 8966 || snap.MaxWin == nil || *snap.MaxWin != 9 || snap.MaxLoss == nil || *snap.MaxLoss != 0 || snap.OverallLimit != 0 {
		t.Fatalf("snap = %+v", snap)
	}
	// only 2 recent rows from the endpoint (< HistoryRows) → the table is topped up
	// from /api/tron-draw, but only when its next block matches; here it does not.
	if len(snap.Recent) != 2 {
		t.Fatalf("recent = %+v", snap.Recent)
	}
	fs.proposed = `{"enabled":false,"reason":"AI_API_KEY 未配置"}`
	_, err = NewClient(fs.URL).Fetch(context.Background())
	var ue *UnavailableError
	if !errors.As(err, &ue) || !strings.Contains(err.Error(), "AI_API_KEY") {
		t.Fatalf("err = %v", err)
	}
	fs.proposed = `{"enabled":true,"next_block":87473360,"next_pick":null,"recent":[]}`
	if _, err = NewClient(fs.URL).Fetch(context.Background()); !errors.Is(err, ErrPending) {
		t.Fatalf("err = %v, want ErrPending", err)
	}
}

// ---- runner: dedupe / retries / permanent errors ----

type memStore struct {
	mu       sync.Mutex
	chats    []string
	rows     []*memRow
	disabled []string
}
type memRow struct {
	OutboxItem
	status string
}

func (s *memStore) EnabledChats() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.chats...), nil
}
func (s *memStore) Enqueue(chat string, block int64, body string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.ChatID == chat && r.Block == block {
			return false, nil
		}
	}
	s.rows = append(s.rows, &memRow{OutboxItem{ID: uint64(len(s.rows) + 1), ChatID: chat, Block: block, Body: body, CreatedAt: time.Now()}, "pending"})
	return true, nil
}
func (s *memStore) Pending(limit int) ([]OutboxItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []OutboxItem
	for _, r := range s.rows {
		if r.status == "pending" {
			out = append(out, r.OutboxItem)
		}
	}
	return out, nil
}
func (s *memStore) find(id uint64) *memRow { return s.rows[id-1] }
func (s *memStore) MarkSent(id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.find(id).status = "sent"
	return nil
}
func (s *memStore) MarkAttemptFailed(id uint64, _ string, max int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(id)
	r.Attempts++
	if r.Attempts >= max {
		r.status = "failed"
	}
	return nil
}
func (s *memStore) MarkFailed(id uint64, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.find(id).status = "failed"
	return nil
}
func (s *memStore) DisableChat(chat string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disabled = append(s.disabled, chat)
	var keep []string
	for _, c := range s.chats {
		if c != chat {
			keep = append(keep, c)
		}
	}
	s.chats = keep
	for _, r := range s.rows {
		if r.ChatID == chat && r.status == "pending" {
			r.status = "failed"
		}
	}
	return nil
}
func (s *memStore) Prune(time.Duration) error { return nil }

type fakeSource struct {
	mu   sync.Mutex
	snap *Snapshot
	err  error
}

func (f *fakeSource) Fetch(context.Context) (*Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	c := *f.snap
	return &c, nil
}

type fakeSender struct {
	mu    sync.Mutex
	sent  map[string][]string
	errFn func(chat string) error
}

func (f *fakeSender) Send(_ context.Context, chat, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errFn != nil {
		if err := f.errFn(chat); err != nil {
			return err
		}
	}
	if f.sent == nil {
		f.sent = map[string][]string{}
	}
	f.sent[chat] = append(f.sent[chat], body)
	return nil
}

func newTestRunner(chats []string, snap *Snapshot) (*Runner, *memStore, *fakeSource, *fakeSender) {
	st := &memStore{chats: chats}
	src := &fakeSource{snap: snap}
	snd := &fakeSender{}
	return &Runner{Source: src, Store: st, Sender: snd, Sleep: func(context.Context, time.Duration) bool { return true }}, st, src, snd
}

func snapFor(block int64) *Snapshot {
	return &Snapshot{NextBlock: block, NextPick: Double, Recent: []Row{{block - 20, Single, Single}}}
}

func TestRunnerSendsEachPeriodOncePerGroup(t *testing.T) {
	r, st, src, snd := newTestRunner([]string{"-1", "-2"}, snapFor(1000))
	ctx := context.Background()
	for i := 0; i < 5; i++ { // many polls within the same minute
		r.Tick(ctx)
	}
	if len(snd.sent["-1"]) != 1 || len(snd.sent["-2"]) != 1 {
		t.Fatalf("sent = %v", snd.sent)
	}
	// bot restart: a fresh runner over the same store must not resend the period
	r2 := &Runner{Source: src, Store: st, Sender: snd, Sleep: r.Sleep}
	r2.Tick(ctx)
	if len(snd.sent["-1"]) != 1 {
		t.Fatalf("restart re-sent: %v", snd.sent)
	}
	// next minute
	src.snap = snapFor(1020)
	r.Tick(ctx)
	r.Tick(ctx)
	if len(snd.sent["-1"]) != 2 || !strings.Contains(snd.sent["-1"][1], "1020 | ⏳待开奖") {
		t.Fatalf("second period: %v", snd.sent["-1"])
	}
	// a group enabled in the middle of a period gets that period once
	st.chats = append(st.chats, "-3")
	r.Tick(ctx)
	r.Tick(ctx)
	if len(snd.sent["-3"]) != 1 || len(snd.sent["-1"]) != 2 {
		t.Fatalf("late group: %v", snd.sent)
	}
}

func TestRunnerNoEnabledGroupsDoesNotFetch(t *testing.T) {
	r, _, src, snd := newTestRunner(nil, snapFor(1000))
	src.err = errors.New("must not be called")
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())
	r.Tick(context.Background())
	if len(snd.sent) != 0 || buf.Len() != 0 {
		t.Fatalf("sent=%v log=%q", snd.sent, buf.String())
	}
}

func TestRunnerPendingPickWaitsSilentlyAndUnavailableLogsOnce(t *testing.T) {
	r, _, src, snd := newTestRunner([]string{"-1"}, snapFor(1000))
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())
	ctx := context.Background()

	src.err = ErrPending
	r.Tick(ctx)
	if buf.Len() != 0 || len(snd.sent) != 0 {
		t.Fatalf("pending must be silent: %q", buf.String())
	}
	src.err = &UnavailableError{Reason: "AI 未启用"}
	for i := 0; i < 4; i++ {
		r.Tick(ctx)
	}
	if n := strings.Count(buf.String(), "数据暂不可用"); n != 1 {
		t.Fatalf("logged %d times: %q", n, buf.String())
	}
	if len(snd.sent) != 0 {
		t.Fatal("must not send while unavailable")
	}
	src.err = nil
	r.Tick(ctx)
	if len(snd.sent["-1"]) != 1 || !strings.Contains(buf.String(), "数据已恢复") {
		t.Fatalf("recovery: sent=%v log=%q", snd.sent, buf.String())
	}
}

func TestRunnerStaleDataSkipped(t *testing.T) {
	snap := snapFor(1000)
	snap.LatestTS = time.Now().Add(-time.Hour).UnixMilli()
	r, _, _, snd := newTestRunner([]string{"-1"}, snap)
	r.Tick(context.Background())
	if len(snd.sent) != 0 {
		t.Fatal("stale data must not be broadcast")
	}
}

func TestRunnerPermanentErrorDisablesGroupOthersStillserved(t *testing.T) {
	r, st, _, snd := newTestRunner([]string{"-dead", "-ok"}, snapFor(1000))
	snd.errFn = func(chat string) error {
		if chat == "-dead" {
			return errors.New("Forbidden: bot was kicked from the group chat")
		}
		return nil
	}
	r.Tick(context.Background())
	r.Tick(context.Background())
	if len(snd.sent["-ok"]) != 1 {
		t.Fatalf("healthy group starved: %v", snd.sent)
	}
	if len(st.disabled) != 1 || st.disabled[0] != "-dead" {
		t.Fatalf("disabled = %v", st.disabled)
	}
}

func TestRunnerTransientErrorRetriesThenGivesUp(t *testing.T) {
	r, st, _, snd := newTestRunner([]string{"-1", "-2"}, snapFor(1000))
	calls := 0
	snd.errFn = func(chat string) error {
		if chat == "-1" {
			calls++
			return errors.New("network timeout")
		}
		return nil
	}
	for i := 0; i < 6; i++ {
		r.Tick(context.Background())
	}
	if calls != MaxSendAttempts {
		t.Fatalf("attempts = %d, want %d", calls, MaxSendAttempts)
	}
	if st.rows[0].status != "failed" || len(st.disabled) != 0 || len(snd.sent["-2"]) != 1 {
		t.Fatalf("rows=%+v disabled=%v sent=%v", st.rows[0], st.disabled, snd.sent)
	}
}

func TestRunnerDropsStaleQueuedPeriods(t *testing.T) {
	r, st, src, snd := newTestRunner([]string{"-1"}, snapFor(1000))
	snd.errFn = func(string) error { return errors.New("temporary") }
	r.Tick(context.Background()) // period 1000 queued, send fails (attempt 1)
	snd.errFn = nil
	src.snap = snapFor(1020)
	r.Tick(context.Background())
	if len(snd.sent["-1"]) != 1 || !strings.Contains(snd.sent["-1"][0], "1020 | ⏳待开奖") {
		t.Fatalf("sent = %v", snd.sent)
	}
	if st.rows[0].status != "failed" {
		t.Fatalf("old period should be dropped: %+v", st.rows[0])
	}
}

func TestRunnerEndToEndWithFakeHTTPServer(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 400, total: 400, lastBlock: 87473340, windowMax: true})
	st := &memStore{chats: []string{"-1001"}}
	snd := &fakeSender{}
	r := &Runner{Source: NewClient(fs.URL), Store: st, Sender: snd, Sleep: func(context.Context, time.Duration) bool { return true }}
	for i := 0; i < 3; i++ {
		r.Tick(context.Background())
	}
	if len(snd.sent["-1001"]) != 1 {
		t.Fatalf("sent = %v", snd.sent)
	}
	msg := snd.sent["-1001"][0]
	for _, want := range []string{"波场一分单双", "87473360 | ⏳待开奖 | 🔴双 | ⏳", "本期预测：🔴双", "180期：", "近三小时 最大连中 7✅ 最大连错 6❌", "188.ms"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("missing %q in:\n%s", want, msg)
		}
	}
	fs.draw.periods, fs.draw.lastBlock = 401, 87473360
	r.Tick(context.Background())
	if len(snd.sent["-1001"]) != 2 {
		t.Fatalf("next period not sent: %d", len(snd.sent["-1001"]))
	}
}

func TestProposedWithoutMaxStreakOmitsLine(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000})
	for name, window := range map[string]string{
		"neither":  `{"periods":180,"hits":88,"total":179}`,
		"win only": `{"periods":180,"hits":88,"total":179,"max_win":4}`,
		"null":     `{"periods":180,"hits":88,"total":179,"max_win":null,"max_loss":null}`,
		"negative": `{"periods":180,"hits":88,"total":179,"max_win":-1,"max_loss":2}`,
	} {
		fs.proposed = `{"enabled":true,"next_block":87473360,"next_pick":"双","recent":[],"window":` + window +
			`,"overall":{"hits":1,"total":2},"streak":{"kind":"win","count":3}}`
		snap, err := NewClient(fs.URL).Fetch(context.Background())
		if err != nil {
			t.Fatal(name, err)
		}
		if msg := Format(AI, *snap); strings.Contains(msg, "连中") || strings.Contains(msg, "连错") {
			t.Fatalf("%s: line must be omitted (old streak field must not be used):\n%s", name, msg)
		}
		if !strings.Contains(Format(AI, *snap), "180期：") {
			t.Fatalf("%s: win-rate lines must stay", name)
		}
	}
	// zero values are real values and are shown
	fs.proposed = `{"enabled":true,"next_block":87473360,"next_pick":"双","recent":[],"window":{"periods":180,"hits":0,"total":0,"max_win":0,"max_loss":12},"overall":{"hits":0,"total":0}}`
	snap, err := NewClient(fs.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(Format(AI, *snap), "近三小时 最大连中 0✅ 最大连错 12❌") {
		t.Fatal(Format(AI, *snap))
	}
}

const autoBody = `{"enabled":true,"next_block":87473360,"next_pick":"单","latest_ts":0,
 "recent":[{"block":87473340,"result":"单","pick":"单"},{"block":87473320,"result":"双","pick":"单"}],
 "window":{"periods":180,"hits":100,"total":180,"max_win":8,"max_loss":5},"overall":{"hits":4000,"total":8000}}`

func TestAutoEndpointUsesOwnPathAndTemplate(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000})
	fs.auto = autoBody
	snap, err := NewKindClient(fs.URL, Auto).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fs.autoHit.Load() != 1 || fs.proposedHit.Load() != 0 || fs.heavyCalls.Load()+fs.lightCalls.Load() != 0 {
		t.Fatalf("auto must only call its own endpoint: auto=%d ai=%d", fs.autoHit.Load(), fs.proposedHit.Load())
	}
	if len(snap.Recent) != 2 { // no top-up from /api/tron-draw for auto
		t.Fatalf("recent = %+v", snap.Recent)
	}
	got := Format(Auto, *snap)
	want := strings.Join([]string{
		"<b>波场一分单双</b>",
		"",
		"<b>自动最优预测（近15期）</b>",
		"<pre>区块号 | 结果 | 预测 | 状态",
		"87473320 | 🔴双 | 🟢单 | ❌",
		"87473340 | 🟢单 | 🟢单 | ✅",
		"87473360 | ⏳待开奖 | 🟢单 | ⏳</pre>",
		"────────────────",
		"本期预测：🟢单",
		"<b>📊 周期胜率概览</b>",
		"180期：55.6%（已预测 8000 期 50.0%）",
		"近三小时 最大连中 8✅ 最大连错 5❌",
		"走势预测统计网址  188.ms",
	}, "\n")
	if got != want {
		t.Fatalf("--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestAutoEndpointDisabledOr404IsUnavailableNoFallback(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000})
	var ue *UnavailableError
	// 404 (endpoint not shipped yet / stats page off)
	_, err := NewKindClient(fs.URL, Auto).Fetch(context.Background())
	if !errors.As(err, &ue) || !strings.Contains(err.Error(), "404") {
		t.Fatalf("404: err = %v", err)
	}
	if fs.heavyCalls.Load()+fs.lightCalls.Load() != 0 {
		t.Fatal("auto must never fall back to /api/tron-draw")
	}
	fs.auto = `{"enabled":false,"reason":"自动最优 未开启"}`
	_, err = NewKindClient(fs.URL, Auto).Fetch(context.Background())
	if !errors.As(err, &ue) || !strings.Contains(err.Error(), "自动最优 未开启") {
		t.Fatalf("disabled: err = %v", err)
	}
	fs.auto = `{"enabled":true,"next_block":87473360,"next_pick":null,"recent":[]}`
	if _, err = NewKindClient(fs.URL, Auto).Fetch(context.Background()); !errors.Is(err, ErrPending) {
		t.Fatalf("err = %v, want ErrPending", err)
	}
	fs.autoStatus = 500
	if _, err = NewKindClient(fs.URL, Auto).Fetch(context.Background()); err == nil || errors.As(err, &ue) {
		t.Fatalf("500 must be a plain transport-style error, got %v", err)
	}
}

func TestAutoRunnerSendsOwnTemplateAndLogsEachReasonOnce(t *testing.T) {
	fs := newFakeServer(t, fakeDraw{periods: 30, total: 30, lastBlock: 1000})
	st := &memStore{chats: []string{"-1001"}}
	snd := &fakeSender{}
	r := &Runner{Kind: Auto, Source: NewKindClient(fs.URL, Auto), Store: st, Sender: snd, Sleep: func(context.Context, time.Duration) bool { return true }}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())
	ctx := context.Background()

	for i := 0; i < 4; i++ { // 404
		r.Tick(ctx)
	}
	fs.auto = `{"enabled":false,"reason":"自动最优 未开启"}`
	for i := 0; i < 4; i++ { // enabled=false: a different reason, logged once more
		r.Tick(ctx)
	}
	if len(snd.sent) != 0 {
		t.Fatal("nothing may be sent while unavailable")
	}
	if n := strings.Count(buf.String(), "自动最优波场单双一分: 数据暂不可用"); n != 2 {
		t.Fatalf("want one log line per reason (2), got %d:\n%s", n, buf.String())
	}
	fs.auto = autoBody
	for i := 0; i < 3; i++ {
		r.Tick(ctx)
	}
	if len(snd.sent["-1001"]) != 1 {
		t.Fatalf("sent = %v", snd.sent)
	}
	if msg := snd.sent["-1001"][0]; !strings.Contains(msg, "自动最优预测（近15期）") || strings.Contains(msg, "AI预测") {
		t.Fatalf("wrong template:\n%s", msg)
	}
	if !strings.Contains(buf.String(), "自动最优波场单双一分: 数据已恢复") {
		t.Fatalf("recovery log missing:\n%s", buf.String())
	}
}

func TestOutboxDDLIsPerKind(t *testing.T) {
	ai, auto := OutboxDDL(AI.Table), OutboxDDL(Auto.Table)
	if AI.Table == Auto.Table || !strings.Contains(ai, "`ai_tron_outbox`") || !strings.Contains(auto, "`auto_tron_outbox`") {
		t.Fatalf("tables: %s / %s", AI.Table, Auto.Table)
	}
	if !strings.Contains(auto, "UNIQUE KEY `uk_tron_chat_block` (`chat_id`,`block`)") {
		t.Fatalf("dedupe key missing: %s", auto)
	}
}
