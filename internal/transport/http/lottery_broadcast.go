package http

import (
	_ "embed"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/davidmuller5273-boop/safe/internal/lotterybroadcast"
)

//go:embed lottery_broadcast.html
var lotteryBroadcastPage []byte

// 开奖播报 admin API: switches (master / per-lottery / ads), latest results,
// data-source status, group subscriptions and recent broadcasts.

type lbGame struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Source  string `json:"source"`
	Enabled bool   `json:"enabled"`
}

type lbLatest struct {
	GameCode  string    `json:"game_code"`
	GameName  string    `json:"game_name"`
	Issue     string    `json:"issue"`
	DrawTime  string    `json:"draw_time"`
	Numbers   string    `json:"numbers"`
	Source    string    `json:"source"`
	FetchedAt time.Time `json:"fetched_at"`
}

type lbSource struct {
	Source        string     `json:"source"`
	Label         string     `json:"label"`
	LastCheckedAt time.Time  `json:"last_checked_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	LastError     string     `json:"last_error"`
}

type lbSubscription struct {
	lotterybroadcast.Subscription
	Label string `json:"label"`
}

type lbOutbox struct {
	lotterybroadcast.OutboxItem
	GameName string     `json:"game_name"`
	SentAt   *time.Time `json:"sent_at"`
}

func (h Handler) lotteryBroadcastUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", lotteryBroadcastPage)
}

func (h Handler) lotteryBroadcast(c *gin.Context) {
	store := lotterybroadcast.NewStore(h.DB)
	sw, err := store.LoadSwitches()
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	games := make([]lbGame, 0, len(lotterybroadcast.GameOrder))
	for _, code := range lotterybroadcast.GameOrder {
		g := lotterybroadcast.Games[code]
		games = append(games, lbGame{Code: code, Name: g.Name, Source: lotterybroadcast.SourceLabel(g.Source), Enabled: sw.GameEnabled(code)})
	}
	rows, err := store.LatestResults(nil)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	latest := make([]lbLatest, 0, len(rows))
	for _, r := range rows {
		numbers := r.PrimaryNumbers
		if strings.TrimSpace(r.SecondaryNumbers) != "" {
			numbers += " + " + r.SecondaryNumbers
		}
		latest = append(latest, lbLatest{GameCode: r.GameCode, GameName: r.GameName, Issue: r.Issue, DrawTime: r.DrawTime,
			Numbers: numbers, Source: r.Source, FetchedAt: r.FetchedAt})
	}
	statuses, err := store.SourceStatuses()
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	sources := make([]lbSource, 0, len(statuses))
	for _, s := range statuses {
		item := lbSource{Source: s.Source, Label: lotterybroadcast.SourceLabel(s.Source), LastCheckedAt: s.LastCheckedAt, LastError: s.LastError}
		if s.LastSuccessAt.Valid {
			t := s.LastSuccessAt.Time
			item.LastSuccessAt = &t
		}
		sources = append(sources, item)
	}
	subs, err := store.Subscriptions("", 500)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	subscriptions := make([]lbSubscription, 0, len(subs))
	for _, s := range subs {
		subscriptions = append(subscriptions, lbSubscription{Subscription: s, Label: lotterybroadcast.SelectorLabel(s.Selector)})
	}
	recent, err := store.RecentOutbox(50)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	outbox := make([]lbOutbox, 0, len(recent))
	for _, o := range recent {
		item := lbOutbox{OutboxItem: o, GameName: lotterybroadcast.Games[o.GameCode].Name}
		if o.SentAt.Valid {
			t := o.SentAt.Time
			item.SentAt = &t
		}
		outbox = append(outbox, item)
	}
	ok(c, gin.H{
		"query_enabled":     sw.QueryEnabled,
		"broadcast_enabled": sw.BroadcastEnabled,
		"with_ads":          sw.WithAds,
		"games":             games,
		"latest":            latest,
		"sources":           sources,
		"subscriptions":     subscriptions,
		"outbox":            outbox,
	})
}

func (h Handler) updateLotteryBroadcastSwitches(c *gin.Context) {
	var body struct {
		QueryEnabled     *bool           `json:"query_enabled"`
		BroadcastEnabled *bool           `json:"broadcast_enabled"`
		WithAds          *bool           `json:"with_ads"`
		Games            map[string]bool `json:"games"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	store := lotterybroadcast.NewStore(h.DB)
	sw, err := store.LoadSwitches()
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	if body.QueryEnabled != nil {
		sw.QueryEnabled = *body.QueryEnabled
	}
	if body.BroadcastEnabled != nil {
		sw.BroadcastEnabled = *body.BroadcastEnabled
	}
	if body.WithAds != nil {
		sw.WithAds = *body.WithAds
	}
	for code, on := range body.Games {
		if _, exists := lotterybroadcast.Games[code]; !exists {
			fail(c, http.StatusBadRequest, errors.New("未知彩种: "+code))
			return
		}
		sw.Games[code] = on
	}
	if err := store.SaveSwitches(sw); err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, nil)
}

func (h Handler) deleteLotteryBroadcastSubscription(c *gin.Context) {
	var body struct {
		ChatID   string `json:"chat_id" binding:"required,max=64"`
		Selector string `json:"selector" binding:"max=32"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if body.Selector != "" && !lotterybroadcast.IsSelector(body.Selector) {
		fail(c, http.StatusBadRequest, errors.New("未知订阅范围"))
		return
	}
	n, err := lotterybroadcast.NewStore(h.DB).RemoveSubscription(body.ChatID, body.Selector)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, gin.H{"removed": n})
}
