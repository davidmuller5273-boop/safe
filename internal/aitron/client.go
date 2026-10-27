package aitron

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the tg-directory-bot web service on the same server.
const DefaultBaseURL = "http://127.0.0.1:8080"

const (
	// ProposedPath is the (not yet built) tg-directory-bot endpoint this client
	// prefers; see docs/AI波场单双一分.md for the JSON contract.
	ProposedPath = "/api/tron-draw/ai-broadcast"
	// fallbackAILimit is the largest ai_limit /api/tron-draw accepts.
	fallbackAILimit = 5000
	maxBody         = 16 << 20
)

// Client reads AI prediction data from tg-directory-bot. It first tries the
// proposed dedicated endpoint; when that is missing (HTTP 404) it derives the
// same numbers from the existing /api/tron-draw JSON.
type Client struct {
	BaseURL string
	HTTP    *http.Client

	mu     sync.Mutex
	cached *Snapshot // fallback mode: full snapshot for cached.NextBlock
}

// NewClient builds a client; empty baseURL uses DefaultBaseURL.
func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) get(ctx context.Context, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	return resp.StatusCode, body, err
}

// Fetch returns the current snapshot. Errors: ErrPending (pick not stored yet),
// *UnavailableError (AI off / bad data), other errors are transport failures.
func (c *Client) Fetch(ctx context.Context) (*Snapshot, error) {
	status, body, err := c.get(ctx, ProposedPath)
	if err != nil {
		return nil, err
	}
	switch {
	case status == http.StatusOK:
		snap, err := parseProposed(body)
		if err != nil || len(snap.Recent) >= HistoryRows {
			return snap, err
		}
		// The dedicated endpoint may still return fewer than HistoryRows rows
		// (older tg-directory-bot builds return 10): top the table up from
		// /api/tron-draw, keeping the endpoint's own statistics.
		if fb, ferr := c.fetchFallback(ctx); ferr == nil && fb.NextBlock == snap.NextBlock && len(fb.Recent) > len(snap.Recent) {
			snap.Recent = fb.Recent
		}
		return snap, nil
	case status == http.StatusNotFound:
		return c.fetchFallback(ctx)
	default:
		return nil, fmt.Errorf("%s 返回 HTTP %d", ProposedPath, status)
	}
}

// ---- proposed endpoint: GET /api/tron-draw/ai-broadcast ----

type rateJSON struct {
	Hits  int `json:"hits"`
	Total int `json:"total"`
}

type proposedJSON struct {
	Enabled   bool   `json:"enabled"`
	Reason    string `json:"reason"`
	NextBlock int64  `json:"next_block"`
	NextPick  string `json:"next_pick"`
	LatestTS  int64  `json:"latest_ts"`
	Recent    []struct {
		Block  int64  `json:"block"`
		Result string `json:"result"`
		Pick   string `json:"pick"`
	} `json:"recent"`
	Window struct {
		Periods int `json:"periods"`
		rateJSON
	} `json:"window"`
	Overall rateJSON `json:"overall"`
	Streak  struct {
		Kind  string `json:"kind"` // "win" | "loss" | "none"
		Count int    `json:"count"`
	} `json:"streak"`
}

func parseProposed(body []byte) (*Snapshot, error) {
	var p proposedJSON
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, &UnavailableError{Reason: "ai-broadcast 返回的不是有效 JSON: " + err.Error()}
	}
	if !p.Enabled {
		reason := p.Reason
		if reason == "" {
			reason = "AI 未启用"
		}
		return nil, &UnavailableError{Reason: reason}
	}
	s := &Snapshot{
		NextBlock: p.NextBlock, NextPick: p.NextPick, LatestTS: p.LatestTS,
		WindowPeriods: p.Window.Periods,
		Window:        Rate{Hits: p.Window.Hits, Total: p.Window.Total},
		Overall:       Rate{Hits: p.Overall.Hits, Total: p.Overall.Total},
	}
	for _, r := range p.Recent {
		s.Recent = append(s.Recent, Row{Block: r.Block, Result: r.Result, Pick: r.Pick})
	}
	switch p.Streak.Kind {
	case "win":
		s.StreakWin = p.Streak.Count
	case "loss":
		s.StreakLoss = p.Streak.Count
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// ---- fallback: existing GET /api/tron-draw ----

type drawJSON struct {
	TotalRecorded int64 `json:"total_recorded"`
	NextDrawBlock int64 `json:"next_draw_block"`
	Recent        []struct {
		TS int64 `json:"ts"`
	} `json:"recent"`
	AI *struct {
		Enabled bool `json:"enabled"`
		Next    *struct {
			Block int64  `json:"block"`
			Pick  string `json:"pick"`
		} `json:"next"`
		Trend *struct {
			Items []struct {
				Block     int64   `json:"block"`
				Predicted *string `json:"predicted"`
				Result    string  `json:"result"`
			} `json:"items"`
		} `json:"trend"`
	} `json:"ai"`
}

func (c *Client) fetchDraw(ctx context.Context, path string) (*drawJSON, error) {
	status, body, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("%s 返回 HTTP %d", strings.SplitN(path, "?", 2)[0], status)
	}
	var d drawJSON
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, &UnavailableError{Reason: "/api/tron-draw 返回的不是有效 JSON: " + err.Error()}
	}
	if d.AI == nil || !d.AI.Enabled {
		return nil, &UnavailableError{Reason: "AI 未启用（tg-directory-bot 未配置 AI_API_KEY 或 TRON_AI_PREDICT=0）"}
	}
	return &d, nil
}

func (c *Client) fetchFallback(ctx context.Context) (*Snapshot, error) {
	// Cheap probe first: one tiny request per poll; the 5000-row request only
	// runs once per new period.
	light, err := c.fetchDraw(ctx, "/api/tron-draw?limit=1&ai_limit=1")
	if err != nil {
		return nil, err
	}
	if light.AI.Next == nil || (light.AI.Next.Pick != Single && light.AI.Next.Pick != Double) {
		return nil, ErrPending
	}
	c.mu.Lock()
	cached := c.cached
	c.mu.Unlock()
	if cached != nil && cached.NextBlock == light.AI.Next.Block && cached.NextPick == light.AI.Next.Pick {
		return cached, nil
	}
	full, err := c.fetchDraw(ctx, fmt.Sprintf("/api/tron-draw?limit=%d&ai_limit=%d", WindowPeriods, fallbackAILimit))
	if err != nil {
		return nil, err
	}
	snap, err := snapshotFromDraw(full)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cached = snap
	c.mu.Unlock()
	return snap, nil
}

// snapshotFromDraw derives every number of the message from /api/tron-draw
// (limit=180, ai_limit=5000): ai.trend.items (block, predicted, result) and ai.next.
func snapshotFromDraw(d *drawJSON) (*Snapshot, error) {
	if d.AI == nil || d.AI.Next == nil {
		return nil, ErrPending
	}
	if d.NextDrawBlock != 0 && d.NextDrawBlock != d.AI.Next.Block {
		return nil, ErrPending // a new draw was recorded between the two fields; retry next poll
	}
	s := &Snapshot{NextBlock: d.AI.Next.Block, NextPick: d.AI.Next.Pick, WindowPeriods: WindowPeriods}
	if len(d.Recent) > 0 {
		s.LatestTS = d.Recent[0].TS // /api/tron-draw lists newest first
	}
	if d.AI.Trend == nil {
		return nil, &UnavailableError{Reason: "/api/tron-draw 缺少 ai.trend（ai_limit 参数未生效）"}
	}
	items := d.AI.Trend.Items
	var judged []Row // all stored predictions, oldest first
	windowStart := len(items) - WindowPeriods
	if windowStart < 0 {
		windowStart = 0
	}
	var windowRows []Row // judged rows inside the last 180 periods
	for i, it := range items {
		if it.Predicted == nil || *it.Predicted == "" {
			continue // no stored prediction (before AI was enabled / API failed): skipped
		}
		row := Row{Block: it.Block, Result: it.Result, Pick: *it.Predicted}
		judged = append(judged, row)
		if i >= windowStart {
			windowRows = append(windowRows, row)
		}
	}
	s.Recent = judged
	if len(s.Recent) > HistoryRows {
		s.Recent = s.Recent[len(s.Recent)-HistoryRows:]
	}
	for _, r := range windowRows {
		s.Window.Total++
		if r.OK() {
			s.Window.Hits++
		}
	}
	for _, r := range judged {
		s.Overall.Total++
		if r.OK() {
			s.Overall.Hits++
		}
	}
	// The API returns at most fallbackAILimit periods; when the oldest returned
	// period still has a prediction, earlier ones may exist → totals are truncated.
	if len(items) > 0 && items[0].Predicted != nil && d.TotalRecorded > int64(len(items)) {
		s.OverallLimit = len(items)
	}
	for i := len(windowRows) - 1; i >= 0; i-- {
		ok := windowRows[i].OK()
		if i == len(windowRows)-1 {
			if ok {
				s.StreakWin = 1
			} else {
				s.StreakLoss = 1
			}
			continue
		}
		if ok && s.StreakWin > 0 {
			s.StreakWin++
		} else if !ok && s.StreakLoss > 0 {
			s.StreakLoss++
		} else {
			break
		}
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}
