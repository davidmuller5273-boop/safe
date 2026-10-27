// Package aitron implements the per-group 「AI波场单双一分」 broadcast: after each
// new TRON draw period (one per minute) the SafeW bot posts the AI prediction
// table, taken from the tg-directory-bot web service on localhost.
package aitron

import (
	"errors"
	"fmt"
	"html"
	"strings"
)

const (
	// HistoryRows is how many finished periods the table shows.
	HistoryRows = 15
	// WindowPeriods is the hit-rate window: 180 periods ≈ 3 hours.
	WindowPeriods = 180
	// SiteText is the footer line of the message.
	SiteText = "走势预测统计网址  188.ms"

	Single = "单"
	Double = "双"
)

// Row is one finished period with a stored AI prediction.
type Row struct {
	Block  int64
	Result string // 单 / 双
	Pick   string // 单 / 双
}

// OK reports whether the AI pick matched the draw result.
func (r Row) OK() bool { return r.Pick == r.Result }

// Rate is hits/total; Total==0 means "no data".
type Rate struct {
	Hits  int
	Total int
}

// Percent returns the hit rate in percent (ok=false when Total is 0).
func (r Rate) Percent() (float64, bool) {
	if r.Total <= 0 {
		return 0, false
	}
	return float64(r.Hits) * 100 / float64(r.Total), true
}

// Snapshot is everything one broadcast needs.
type Snapshot struct {
	NextBlock int64  // block height of the pending (next) draw
	NextPick  string // AI pick for NextBlock (单/双)
	LatestTS  int64  // unix ms of the newest recorded draw (0 = unknown)

	Recent []Row // finished periods WITH a stored AI pick, oldest first (≤ HistoryRows used)

	WindowPeriods int  // size of the 近期 window (default 180)
	Window        Rate // hits within the last WindowPeriods periods
	Overall       Rate // every stored AI prediction
	// OverallLimit > 0 means Overall only covers the last OverallLimit periods
	// (fallback API cannot return more); the message then says 近N期 instead of 已预测.
	OverallLimit int

	// MaxWin / MaxLoss are the longest consecutive hits / misses within the newest
	// 180 drawn blocks (近三小时), exactly as computed and returned by the
	// tg-directory-bot web program (window.max_win / window.max_loss). nil =
	// the server did not provide them: the line is then omitted. The bot never
	// computes these itself.
	MaxWin  *int
	MaxLoss *int
}

// Kind describes one broadcast flavour (AI 预测 / 自动最优 预测); both share
// the same snapshot, template, runner and outbox code.
type Kind struct {
	Key      string // short id used in logs/tests: "ai" / "auto"
	Name     string // display + command name, e.g. "AI波场单双一分"
	Subtitle string // table heading, e.g. "AI预测（近15期）"
	Path     string // tg-directory-bot endpoint
	Fallback bool   // may fall back to the existing /api/tron-draw JSON when Path is 404
	Table    string // outbox table
}

var (
	// AI is the 「AI波场单双一分」 broadcast.
	AI = Kind{Key: "ai", Name: "AI波场单双一分", Subtitle: "AI预测（近15期）", Path: "/api/tron-draw/ai-broadcast", Fallback: true, Table: "ai_tron_outbox"}
	// Auto is the 「自动最优波场单双一分」 broadcast.
	Auto = Kind{Key: "auto", Name: "自动最优波场单双一分", Subtitle: "自动最优预测（近15期）", Path: "/api/tron-draw/auto-broadcast", Fallback: false, Table: "auto_tron_outbox"}
)

// ErrPending means the draw is recorded but the AI pick for the next period is
// not stored yet (it is generated in the background); try again shortly.
var ErrPending = errors.New("AI 预测尚未生成")

// UnavailableError means the AI data cannot be served (no AI key, API down...).
type UnavailableError struct{ Reason string }

func (e *UnavailableError) Error() string { return "AI 数据不可用: " + e.Reason }

func pickLabel(v string) string {
	switch v {
	case Single:
		return "🟢单"
	case Double:
		return "🔴双"
	default:
		return "·"
	}
}

func statusLabel(ok bool) string {
	if ok {
		return "✅"
	}
	return "❌"
}

func pct(r Rate) string {
	p, ok := r.Percent()
	if !ok {
		return "--"
	}
	return fmt.Sprintf("%.1f%%", p)
}

// Format renders the SafeW HTML message (same <b>/<pre> style as the 热号预测
// push). Table rows are oldest first, the pending period is the last row.
func Format(k Kind, s Snapshot) string {
	rows := s.Recent
	if len(rows) > HistoryRows {
		rows = rows[len(rows)-HistoryRows:]
	}
	var table strings.Builder
	table.WriteString("区块号 | 结果 | 预测 | 状态")
	for _, r := range rows {
		fmt.Fprintf(&table, "\n%d | %s | %s | %s", r.Block, pickLabel(r.Result), pickLabel(r.Pick), statusLabel(r.OK()))
	}
	fmt.Fprintf(&table, "\n%d | ⏳待开奖 | %s | ⏳", s.NextBlock, pickLabel(s.NextPick))

	window := s.WindowPeriods
	if window <= 0 {
		window = WindowPeriods
	}
	overall := fmt.Sprintf("已预测 %d 期 %s", s.Overall.Total, pct(s.Overall))
	if s.OverallLimit > 0 {
		overall = fmt.Sprintf("近%d期 %s", s.OverallLimit, pct(s.Overall))
	}

	var b strings.Builder
	b.WriteString("<b>波场一分单双</b>\n\n")
	b.WriteString("<b>" + html.EscapeString(k.Subtitle) + "</b>\n")
	b.WriteString("<pre>" + html.EscapeString(table.String()) + "</pre>\n")
	b.WriteString("────────────────\n")
	fmt.Fprintf(&b, "本期预测：%s\n", pickLabel(s.NextPick))
	b.WriteString("<b>📊 周期胜率概览</b>\n")
	fmt.Fprintf(&b, "%d期：%s（%s）\n", window, pct(s.Window), overall)
	if s.MaxWin != nil && s.MaxLoss != nil { // values come from the web program; never computed here
		fmt.Fprintf(&b, "近三小时 最大连中 %d✅ 最大连错 %d❌\n", *s.MaxWin, *s.MaxLoss)
	}
	b.WriteString(html.EscapeString(SiteText))
	return b.String()
}

// Validate rejects snapshots that cannot be rendered sensibly.
func (s Snapshot) Validate() error {
	if s.NextBlock <= 0 {
		return &UnavailableError{Reason: "缺少下一期区块号"}
	}
	if s.NextPick != Single && s.NextPick != Double {
		return ErrPending
	}
	for _, r := range s.Recent {
		if (r.Result != Single && r.Result != Double) || (r.Pick != Single && r.Pick != Double) {
			return &UnavailableError{Reason: fmt.Sprintf("区块 %d 的结果/预测不是 单/双", r.Block)}
		}
	}
	return nil
}
