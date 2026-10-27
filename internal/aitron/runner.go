package aitron

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
)

const (
	DefaultPollInterval = 5 * time.Second
	// MaxSendAttempts bounds per-group retries of transient send errors.
	MaxSendAttempts = 3
	// MaxAge: a queued prediction older than this is useless (next period is out).
	MaxAge = 3 * time.Minute
	// MaxDataAge: if the newest recorded draw is older than this the data source is stale.
	MaxDataAge = 10 * time.Minute
	// SendGap spaces consecutive sends so hundreds of groups stay under rate limits.
	SendGap      = 50 * time.Millisecond
	SendTimeout  = 20 * time.Second
	flushBatch   = 300
	pruneEvery   = time.Hour
	pruneKeepFor = 7 * 24 * time.Hour
)

// Source provides snapshots (Client implements it).
type Source interface {
	Fetch(ctx context.Context) (*Snapshot, error)
}

// Store is the persistence the runner needs (MySQLStore implements it).
type Store interface {
	EnabledChats() ([]string, error)
	Enqueue(chatID string, block int64, body string) (bool, error)
	Pending(limit int) ([]OutboxItem, error)
	MarkSent(id uint64) error
	MarkAttemptFailed(id uint64, errText string, maxAttempts int) error
	MarkFailed(id uint64, errText string) error
	DisableChat(chatID string) error
	Prune(keep time.Duration) error
}

// Sender delivers one message (adds the group's ads, HTML mode, silent).
type Sender interface {
	Send(ctx context.Context, chatID, body string) error
}

// Runner polls the source and fans each new period out to the enabled groups.
type Runner struct {
	Kind   Kind // which broadcast (zero value = AI)
	Source Source
	Store  Store
	Sender Sender

	PollInterval time.Duration
	Now          func() time.Time
	Sleep        func(ctx context.Context, d time.Duration) bool // nil = real sleep

	lastUnavailable string // last logged "unavailable" reason ("" = healthy)
	currentBlock    int64  // newest pending block seen
	enqueuedBlock   int64
	enqueued        map[string]struct{} // chats already queued for enqueuedBlock
	lastPrune       time.Time
}

func (r *Runner) kind() Kind {
	if r.Kind.Key == "" {
		return AI
	}
	return r.Kind
}

func (r *Runner) label() string { return r.kind().Name }

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) sleep(ctx context.Context, d time.Duration) bool {
	if r.Sleep != nil {
		return r.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Run ticks until ctx ends. It never returns an error: this feature must not
// take the other safew-bot workers down.
func (r *Runner) Run(ctx context.Context) error {
	interval := r.PollInterval
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	for {
		r.Tick(ctx)
		if !r.sleep(ctx, interval) {
			return nil
		}
	}
}

// Tick runs one poll + enqueue + flush cycle.
func (r *Runner) Tick(ctx context.Context) {
	chats, err := r.Store.EnabledChats()
	if err != nil {
		log.Printf("%s: 读取已开启群失败: %v", r.label(), err)
	} else if len(chats) > 0 {
		r.poll(ctx, chats)
	}
	r.flush(ctx)
	if r.now().Sub(r.lastPrune) > pruneEvery {
		r.lastPrune = r.now()
		if err := r.Store.Prune(pruneKeepFor); err != nil {
			log.Printf("%s: 清理旧记录失败: %v", r.label(), err)
		}
	}
}

func (r *Runner) poll(ctx context.Context, chats []string) {
	snap, err := r.Source.Fetch(ctx)
	if err != nil {
		if errors.Is(err, ErrPending) || ctx.Err() != nil {
			return // pick still being generated: just wait for the next tick
		}
		r.noteUnavailable(err.Error())
		return
	}
	if snap.LatestTS > 0 && r.now().Sub(time.UnixMilli(snap.LatestTS)) > MaxDataAge {
		r.noteUnavailable("开奖数据已过期（最新一期早于 10 分钟前）")
		return
	}
	r.noteHealthy()
	if snap.NextBlock > r.currentBlock {
		r.currentBlock = snap.NextBlock
	}
	if snap.NextBlock < r.enqueuedBlock {
		return // never go back to an older period
	}
	if snap.NextBlock != r.enqueuedBlock || r.enqueued == nil {
		r.enqueuedBlock = snap.NextBlock
		r.enqueued = map[string]struct{}{}
	}
	body := Format(r.kind(), *snap)
	for _, chat := range chats {
		if _, done := r.enqueued[chat]; done {
			continue
		}
		if _, err := r.Store.Enqueue(chat, snap.NextBlock, body); err != nil {
			log.Printf("%s: 入队失败 chat=%s block=%d: %v", r.label(), chat, snap.NextBlock, err)
			continue
		}
		r.enqueued[chat] = struct{}{} // duplicates are rejected by the DB unique key too
	}
}

func (r *Runner) noteUnavailable(reason string) {
	if reason != r.lastUnavailable {
		log.Printf("%s: 数据暂不可用，跳过（同一原因不再重复记录）: %s", r.label(), reason)
		r.lastUnavailable = reason
	}
}

func (r *Runner) noteHealthy() {
	if r.lastUnavailable != "" {
		log.Printf("%s: 数据已恢复", r.label())
		r.lastUnavailable = ""
	}
}

func (r *Runner) flush(ctx context.Context) {
	items, err := r.Store.Pending(flushBatch)
	if err != nil {
		log.Printf("%s: 读取待发队列失败: %v", r.label(), err)
		return
	}
	dead := map[string]struct{}{}
	first := true
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		if _, gone := dead[item.ChatID]; gone {
			continue
		}
		if (!item.CreatedAt.IsZero() && r.now().Sub(item.CreatedAt) > MaxAge) ||
			(r.currentBlock > 0 && item.Block < r.currentBlock) {
			_ = r.Store.MarkFailed(item.ID, "本期预测已过期，未发送")
			continue
		}
		if !first && !r.sleep(ctx, SendGap) {
			return
		}
		first = false
		sctx, cancel := context.WithTimeout(ctx, SendTimeout)
		err := r.Sender.Send(sctx, item.ChatID, item.Body)
		cancel()
		if err == nil {
			if merr := r.Store.MarkSent(item.ID); merr != nil {
				log.Printf("%s: 更新发送状态失败 id=%d: %v", r.label(), item.ID, merr)
			}
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if safew.IsPermanentChatError(err) {
			log.Printf("%s: 群不可达（永久错误），关闭该群播报 chat=%s block=%d: %v", r.label(), item.ChatID, item.Block, err)
			_ = r.Store.MarkFailed(item.ID, err.Error())
			if derr := r.Store.DisableChat(item.ChatID); derr != nil {
				log.Printf("%s: 关闭群播报失败 chat=%s: %v", r.label(), item.ChatID, derr)
			}
			dead[item.ChatID] = struct{}{}
			continue
		}
		log.Printf("%s: 发送失败(%d/%d) chat=%s block=%d: %v", r.label(), item.Attempts+1, MaxSendAttempts, item.ChatID, item.Block, err)
		_ = r.Store.MarkAttemptFailed(item.ID, err.Error(), MaxSendAttempts)
	}
}
