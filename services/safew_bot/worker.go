// Package safewbot consumes notification jobs and handles SafeW bot commands.
package safewbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/config"
	"github.com/davidmuller5273-boop/safe/internal/platform/database"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/queue"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"

	"gorm.io/gorm"
)

const retryInterval = 3 * time.Second

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.Open(cfg.MySQLHost, cfg.MySQLPort, cfg.MySQLUser, cfg.MySQLPassword, cfg.MySQLDatabase)
	if err != nil {
		return err
	}
	drawQueue := queue.NewLotteryDrawQueue(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer drawQueue.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := drawQueue.Ping(ctx); err != nil {
		return err
	}
	if err := drawQueue.Recover(ctx); err != nil {
		return err
	}
	worker := worker{
		db:     db,
		queue:  drawQueue,
		client: safew.NewClient(cfg.SafeWAPIBaseURL),
		perms:  botperm.NewStore(db, cfg.DeveloperUserIDs),
	}
	log.Printf("SafeW 机器人消息服务已启动 (developers=%d)", len(cfg.DeveloperUserIDs))

	errCh := make(chan error, 2)
	go func() { errCh <- worker.runQueue(ctx) }()
	go func() { errCh <- worker.runCommands(ctx) }()

	var first error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil && first == nil {
			first = err
			stop()
		}
	}
	return first
}

type worker struct {
	db     *gorm.DB
	queue  *queue.LotteryDrawQueueClient
	client *safew.Client
	perms  *botperm.Store
}

func (w worker) runQueue(ctx context.Context) error {
	for {
		payload, message, err := w.queue.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("读取彩票消息失败: %v", err)
			if !wait(ctx, retryInterval) {
				return nil
			}
			continue
		}

		for {
			var botConfig systemconfig.SafeW
			texts, ready, err := w.predictionMessage(message)
			if err == nil && ready {
				botConfig, err = systemconfig.LoadSafeW(w.db)
				if err == nil {
					err = w.sendToChats(ctx, botConfig, message, texts)
				}
			}
			if err == nil {
				if err := w.queue.Ack(ctx, payload); err != nil {
					log.Printf("确认消息失败，将在重启后重试: issue=%s error=%v", message.IssueNumber, err)
					return err
				}
				if ready {
					log.Printf("SafeW 热号预测消息发送成功: issue=%s sizes=%v", message.IssueNumber, mapKeys(texts))
				} else {
					log.Printf("热号样本尚不足，本期不发送: issue=%s", message.IssueNumber)
				}
				break
			}
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("SafeW 开奖消息发送失败，%s 后重试: issue=%s error=%v", retryInterval, message.IssueNumber, err)
			if !wait(ctx, retryInterval) {
				return nil
			}
		}
	}
}

func (w worker) sendToChats(ctx context.Context, botConfig systemconfig.SafeW, message queue.LotteryDrawMessage, texts map[int]string) error {
	targets, err := w.perms.ListPushTargets()
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		log.Printf("没有已开启推送的群，跳过开奖推送: issue=%s", message.IssueNumber)
		return nil
	}
	for _, group := range targets {
		chatID := group.ChatID
		want6 := botperm.Effective6Code(group)
		want7 := botperm.Effective7Code(group)
		if !want6 && !want7 {
			log.Printf("群已开推送但未开启6/7码，跳过: chat=%s issue=%s", chatID, message.IssueNumber)
			continue
		}
		sizes := make([]int, 0, 2)
		if want6 {
			sizes = append(sizes, 6)
		}
		if want7 {
			sizes = append(sizes, 7)
		}
		for _, size := range sizes {
			body, ok := texts[size]
			if !ok || body == "" {
				log.Printf("群需要 %d码 但本期文本未就绪，跳过: chat=%s issue=%s", size, chatID, message.IssueNumber)
				continue
			}
			kind := fmt.Sprintf("%d", size)
			sent, err := w.queue.WasSent(ctx, message.RecordID, chatID, kind)
			if err != nil {
				return err
			}
			if sent {
				continue
			}
			outbound, err := ads.Wrap(w.db, chatID, body)
			if err != nil {
				return err
			}
			if err := w.client.SendMessage(ctx, botConfig.Token, chatID, outbound); err != nil {
				return err
			}
			if err := w.queue.MarkSent(ctx, message.RecordID, chatID, kind); err != nil {
				return err
			}
		}
	}
	return nil
}

func mapKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
