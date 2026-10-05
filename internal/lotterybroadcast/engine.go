package lotterybroadcast

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Repo is the storage the refresh engine needs (Store implements it; tests use a fake).
type Repo interface {
	LatestIssue(code string) (string, error)
	// SaveResult inserts a new (game, issue) row or updates an existing one;
	// inserted is true only for a brand-new row.
	SaveResult(r Result) (inserted bool, err error)
	SubscriberChatIDs(code string) ([]string, error)
	// Enqueue adds one outbox message; false when that chat already has this issue queued.
	Enqueue(chatID, code, issue, body string) (bool, error)
	SetSourceStatus(source string, ok bool, errText string) error
	SubscribedCodes() ([]string, error)
}

// Fetcher returns the latest results per code (Service implements it).
type Fetcher interface {
	LatestAll(ctx context.Context, codes []string, known map[string]string) ([]Result, map[string]string)
}

// RefreshOutcome summarises one refresh run.
type RefreshOutcome struct {
	Errors map[string]string
	Saved  int
	Queued int
	// NewIssues lists "code:issue" that were detected as new this run.
	NewIssues []string
}

// Refresh mirrors refresh_lottery_results: fetch → record source status →
// reject invalid numbers → normalise → save; only a brand-new issue that is
// strictly newer than the previously stored one is queued for every
// subscribed chat, and only when allow(code) is true. The very first result
// seen for a game is stored as a baseline without broadcasting.
func Refresh(ctx context.Context, repo Repo, f Fetcher, codes []string, allow func(code string) bool) (RefreshOutcome, error) {
	if len(codes) == 0 {
		codes = append([]string(nil), GameOrder...)
	}
	known := make(map[string]string, len(codes))
	for _, code := range codes {
		issue, err := repo.LatestIssue(code)
		if err != nil {
			return RefreshOutcome{}, err
		}
		if issue != "" {
			known[code] = issue
		}
	}
	results, errs := f.LatestAll(ctx, codes, known)
	out := RefreshOutcome{Errors: errs}
	if out.Errors == nil {
		out.Errors = map[string]string{}
	}
	bySource := map[string][]string{}
	for _, code := range codes {
		if g, ok := Games[code]; ok {
			bySource[g.Source] = append(bySource[g.Source], code)
		}
	}
	sources := make([]string, 0, len(bySource))
	for s := range bySource {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	for _, source := range sources {
		var msgs []string
		for _, code := range bySource[source] {
			if e, ok := out.Errors[code]; ok {
				msgs = append(msgs, e)
			}
		}
		_ = repo.SetSourceStatus(source, len(msgs) == 0, strings.Join(msgs, "；"))
	}
	for _, r := range results {
		if !IsValidResult(r) {
			continue // reject incomplete / illegal numbers so a good row is never overwritten
		}
		nr, err := NormalizeResult(r)
		if err != nil {
			continue
		}
		prev, err := repo.LatestIssue(nr.GameCode)
		if err != nil {
			return out, err
		}
		if prev != "" && prev != nr.Issue && !IsNewerIssue(nr.Issue, prev) {
			continue // older or differently-formatted issue from a lagging source: ignore
		}
		inserted, err := repo.SaveResult(nr)
		if err != nil {
			return out, err
		}
		out.Saved++
		if !inserted || prev == "" {
			continue
		}
		out.NewIssues = append(out.NewIssues, nr.GameCode+":"+nr.Issue)
		if allow == nil || !allow(nr.GameCode) {
			continue
		}
		chats, err := repo.SubscriberChatIDs(nr.GameCode)
		if err != nil {
			return out, err
		}
		body := BroadcastBody(nr)
		for _, chatID := range chats {
			queued, err := repo.Enqueue(chatID, nr.GameCode, nr.Issue, body)
			if err != nil {
				return out, err
			}
			if queued {
				out.Queued++
			}
		}
	}
	return out, nil
}

// Switches are the admin 「开奖播报」 switches.
type Switches struct {
	QueryEnabled     bool            // 启用开奖查询与轮询 (Python lottery_enabled)
	BroadcastEnabled bool            // 启用新期开奖自动播报 (Python lottery_broadcast_enabled)
	WithAds          bool            // 播报附带本群前后广告
	Games            map[string]bool // per-lottery broadcast switch; missing = on
}

// DefaultSwitches: everything on (Python defaults lottery_enabled / lottery_broadcast_enabled to "1").
func DefaultSwitches() Switches {
	return Switches{QueryEnabled: true, BroadcastEnabled: true, WithAds: true, Games: map[string]bool{}}
}

// GameEnabled reports the per-lottery switch (default on).
func (s Switches) GameEnabled(code string) bool {
	v, ok := s.Games[code]
	return !ok || v
}

// AllowBroadcast is the master switch AND the per-lottery switch.
func (s Switches) AllowBroadcast(code string) bool {
	return s.BroadcastEnabled && s.GameEnabled(code)
}

// Poller mirrors poll_lottery_results: called every 5s; polls subscribed games
// inside their fast window (2 min before → 45 min after the draw) and does a
// full safety poll every SlowInterval otherwise.
type Poller struct {
	Repo         Repo
	Fetcher      Fetcher
	Switches     func() (Switches, error)
	Now          func() time.Time
	SlowInterval time.Duration
	OnQueued     func(n int)

	mu       sync.Mutex
	lastSlow time.Time
}

// Tick runs one polling round and returns the codes that were polled.
func (p *Poller) Tick(ctx context.Context) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sw := DefaultSwitches()
	if p.Switches != nil {
		s, err := p.Switches()
		if err != nil {
			return nil, err
		}
		sw = s
	}
	if !sw.QueryEnabled {
		return nil, nil
	}
	subscribed, err := p.Repo.SubscribedCodes()
	if err != nil || len(subscribed) == 0 {
		return nil, err
	}
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	var fast []string
	for _, code := range subscribed {
		if _, ok := PollWindow(code, now); ok {
			fast = append(fast, code)
		}
	}
	slow := p.SlowInterval
	if slow <= 0 {
		slow = 300 * time.Second
	}
	var codes []string
	switch {
	case len(fast) > 0:
		codes = fast
	case p.lastSlow.IsZero() || now.Sub(p.lastSlow) >= slow:
		p.lastSlow = now
		codes = subscribed
	default:
		return nil, nil
	}
	outcome, err := Refresh(ctx, p.Repo, p.Fetcher, codes, sw.AllowBroadcast)
	if err != nil {
		return codes, err
	}
	if outcome.Queued > 0 && p.OnQueued != nil {
		p.OnQueued(outcome.Queued)
	}
	return codes, nil
}
