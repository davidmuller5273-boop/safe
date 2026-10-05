package lotterybroadcast

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// HistoryCacheSize is how many recent issues we keep per game (「缓存100期」).
const HistoryCacheSize = 100

// DefaultBackfillInterval is how often the periodic backfill re-checks all games.
const DefaultBackfillInterval = 3 * time.Hour

// HistoryStore is the persistence Backfill needs (Store implements it).
type HistoryStore interface {
	LatestIssue(code string) (string, error)
	SaveResult(r Result) (inserted bool, err error)
	CountResults(code string) (int, error)
	TrimHistory(code string, keep int) (int64, error)
	SetSourceStatus(source string, ok bool, errText string) error
}

// HistoryFetcher returns up to limit issues, newest first (Service.CacheHistory).
type HistoryFetcher interface {
	CacheHistory(ctx context.Context, code string, limit int) ([]Result, error)
}

// BackfillOutcome summarises one BackfillGame / BackfillAll run.
type BackfillOutcome struct {
	Code    string
	Saved   int
	Trimmed int64
	Err     string
}

// SaveHistoryRows persists history without broadcasting. Issues strictly newer
// than known are skipped so a live new draw is not claimed before Refresh can
// broadcast it. When known is empty every valid row is saved (baseline).
func SaveHistoryRows(store HistoryStore, rows []Result, known string) (int, error) {
	ordered := append([]Result(nil), rows...)
	// Oldest first, so auto-increment ids follow issue order.
	sort.SliceStable(ordered, func(i, j int) bool { return CompareIssues(ordered[i].Issue, ordered[j].Issue) < 0 })
	saved := 0
	for _, r := range ordered {
		if known != "" && IsNewerIssue(r.Issue, known) {
			continue
		}
		if !IsValidResult(r) {
			continue
		}
		nr, err := NormalizeResult(r)
		if err != nil {
			continue
		}
		if _, err := store.SaveResult(nr); err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// TrimIDsBeyondKeep returns ids of rows older than the newest `keep` by issue
// (pure helper for tests and TrimHistory).
func TrimIDsBeyondKeep(rows []ResultRow, keep int) []uint64 {
	if keep < 1 || len(rows) <= keep {
		return nil
	}
	sorted := append([]ResultRow(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return CompareIssues(sorted[i].Issue, sorted[j].Issue) > 0
	})
	out := make([]uint64, 0, len(sorted)-keep)
	for _, r := range sorted[keep:] {
		out = append(out, r.ID)
	}
	return out
}

// CodesNeedingBackfill returns games whose stored row count is below keep.
func CodesNeedingBackfill(store HistoryStore, codes []string, keep int) ([]string, error) {
	if len(codes) == 0 {
		codes = GameOrder
	}
	if keep < 1 {
		keep = HistoryCacheSize
	}
	var need []string
	for _, code := range codes {
		if _, ok := Games[code]; !ok {
			continue
		}
		n, err := store.CountResults(code)
		if err != nil {
			return nil, err
		}
		if n < keep {
			need = append(need, code)
		}
	}
	return need, nil
}

// BackfillGame fetches up to HistoryCacheSize issues for one game, inserts
// idempotently, trims older rows, and never enqueues broadcasts.
func BackfillGame(ctx context.Context, store HistoryStore, fetcher HistoryFetcher, code string) BackfillOutcome {
	out := BackfillOutcome{Code: code}
	game, ok := Games[code]
	if !ok {
		out.Err = "未知彩种"
		return out
	}
	known, err := store.LatestIssue(code)
	if err != nil {
		out.Err = err.Error()
		_ = store.SetSourceStatus(game.Source, false, out.Err)
		return out
	}
	rows, err := fetcher.CacheHistory(ctx, code, HistoryCacheSize)
	if err != nil {
		out.Err = err.Error()
		_ = store.SetSourceStatus(game.Source, false, out.Err)
		return out
	}
	saved, err := SaveHistoryRows(store, rows, known)
	if err != nil {
		out.Err = err.Error()
		_ = store.SetSourceStatus(game.Source, false, out.Err)
		return out
	}
	out.Saved = saved
	trimmed, err := store.TrimHistory(code, HistoryCacheSize)
	if err != nil {
		out.Err = err.Error()
		_ = store.SetSourceStatus(game.Source, false, out.Err)
		return out
	}
	out.Trimmed = trimmed
	_ = store.SetSourceStatus(game.Source, true, "")
	return out
}

// BackfillAll backfills (sequentially) every code with fewer than
// HistoryCacheSize rows. Each failure is logged once and never aborts the rest.
func BackfillAll(ctx context.Context, store HistoryStore, fetcher HistoryFetcher, codes []string) []BackfillOutcome {
	need, err := CodesNeedingBackfill(store, codes, HistoryCacheSize)
	if err != nil {
		log.Printf("开奖历史回填检查失败: %v", err)
		return []BackfillOutcome{{Err: err.Error()}}
	}
	var outs []BackfillOutcome
	for _, code := range need {
		if ctx.Err() != nil {
			break
		}
		o := BackfillGame(ctx, store, fetcher, code)
		outs = append(outs, o)
		if o.Err != "" {
			log.Printf("开奖历史回填失败 game=%s: %s", code, o.Err)
		}
	}
	return outs
}

// Backfiller keeps every game's history cache at HistoryCacheSize issues:
// a cycle on start, then every Interval (RetryInterval while any game is short).
type Backfiller struct {
	Store   HistoryStore
	Fetcher HistoryFetcher
	// Interval between full cycles when all games are full (0 = 3h).
	Interval time.Duration
	// RetryInterval when a game is still short / failed (0 = 15m).
	RetryInterval time.Duration
	// GameTimeout bounds one game's fetch+store (0 = 45s).
	GameTimeout time.Duration
	// Concurrency is how many games are fetched at once (0 = 3).
	Concurrency int
	// MinTopUpGap throttles on-demand top-ups per game (0 = 10m).
	MinTopUpGap time.Duration
	// Codes defaults to GameOrder.
	Codes []string

	mu        sync.Mutex
	inflight  map[string]bool
	lastTopUp map[string]time.Time
}

func (b *Backfiller) gameTimeout() time.Duration {
	if b.GameTimeout > 0 {
		return b.GameTimeout
	}
	return 45 * time.Second
}

// TryBackfillGame runs BackfillGame unless the same code is already in flight
// (started=false). Bounded by GameTimeout.
func (b *Backfiller) TryBackfillGame(ctx context.Context, code string) (out BackfillOutcome, started bool) {
	b.mu.Lock()
	if b.inflight == nil {
		b.inflight = map[string]bool{}
	}
	if b.inflight[code] {
		b.mu.Unlock()
		return BackfillOutcome{Code: code}, false
	}
	b.inflight[code] = true
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		delete(b.inflight, code)
		b.mu.Unlock()
	}()
	gctx, cancel := context.WithTimeout(ctx, b.gameTimeout())
	defer cancel()
	return BackfillGame(gctx, b.Store, b.Fetcher, code), true
}

// TopUp backfills code if it has fewer than HistoryCacheSize rows, at most once
// per MinTopUpGap per game. Safe to call from request handlers in a goroutine.
func (b *Backfiller) TopUp(ctx context.Context, code string) {
	gap := b.MinTopUpGap
	if gap <= 0 {
		gap = 10 * time.Minute
	}
	b.mu.Lock()
	if b.lastTopUp == nil {
		b.lastTopUp = map[string]time.Time{}
	}
	if time.Since(b.lastTopUp[code]) < gap {
		b.mu.Unlock()
		return
	}
	b.lastTopUp[code] = time.Now()
	b.mu.Unlock()
	n, err := b.Store.CountResults(code)
	if err != nil || n >= HistoryCacheSize {
		return
	}
	if out, started := b.TryBackfillGame(ctx, code); started && out.Err != "" {
		log.Printf("开奖历史补充失败 game=%s: %s", code, out.Err)
	}
}

// Run blocks until ctx is done: one cycle immediately, then periodic cycles.
// It never returns an error and never blocks polling or sending.
func (b *Backfiller) Run(ctx context.Context) {
	interval := b.Interval
	if interval <= 0 {
		interval = DefaultBackfillInterval
	}
	retry := b.RetryInterval
	if retry <= 0 {
		retry = 15 * time.Minute
	}
	for {
		short := b.Cycle(ctx)
		wait := interval
		if short > 0 && retry < interval {
			wait = retry
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Cycle backfills every short game (bounded concurrency) and returns how many
// games are still below HistoryCacheSize afterwards. Each failing game is
// logged exactly once per cycle.
func (b *Backfiller) Cycle(ctx context.Context) int {
	codes := b.Codes
	if len(codes) == 0 {
		codes = GameOrder
	}
	need, err := CodesNeedingBackfill(b.Store, codes, HistoryCacheSize)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("开奖历史回填检查失败: %v", err)
		}
		return len(codes)
	}
	if len(need) == 0 {
		return 0
	}
	conc := b.Concurrency
	if conc <= 0 {
		conc = 3
	}
	started := time.Now()
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var mu sync.Mutex
	saved, failed := 0, 0
	for _, code := range need {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			defer func() { <-sem }()
			out, ok := b.TryBackfillGame(ctx, code)
			if !ok {
				return // an on-demand backfill for this game is already running
			}
			mu.Lock()
			defer mu.Unlock()
			saved += out.Saved
			if out.Err != "" {
				failed++
				if ctx.Err() == nil {
					log.Printf("开奖历史回填失败 game=%s: %s", code, out.Err)
				}
			}
		}(code)
	}
	wg.Wait()
	short, _ := CodesNeedingBackfill(b.Store, codes, HistoryCacheSize)
	log.Printf("开奖历史回填完成: 检查=%d 写入=%d 失败=%d 未满100期=%d 用时=%s",
		len(need), saved, failed, len(short), time.Since(started).Round(time.Second))
	return len(short)
}

// ErrBackfillSlow is returned to callers that should tell the user to retry.
var ErrBackfillSlow = fmt.Errorf("正在加载历史，请稍后再试")
