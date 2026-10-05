// Package safewbot consumes notification jobs and handles SafeW bot commands.
package safewbot

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/config"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/lottery/hotnumber"
	"github.com/davidmuller5273-boop/safe/internal/lotterybroadcast"
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
		lb:     newLotteryRuntime(lotterybroadcast.NewStore(db), lotterybroadcast.NewService(lotterybroadcast.SourcesFromEnv())),
	}
	log.Printf("SafeW 机器人消息服务已启动 (developers=%d)", len(cfg.DeveloperUserIDs))

	const workers = 5
	errCh := make(chan error, workers)
	go func() { errCh <- worker.runQueue(ctx) }()
	go func() { errCh <- worker.runCommands(ctx) }()
	go func() { errCh <- worker.runLotteryPoller(ctx) }()
	go func() { errCh <- worker.runLotterySender(ctx) }()
	go func() { errCh <- worker.runLotteryBackfiller(ctx) }()

	var first error
	for i := 0; i < workers; i++ {
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
	lb     *lotteryRuntime // 开奖播报 (nil disables it)
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
					log.Printf("SafeW 热号预测消息发送成功: issue=%s keys=%v", message.IssueNumber, texts.keys())
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

func (w worker) sendToChats(ctx context.Context, botConfig systemconfig.SafeW, message queue.LotteryDrawMessage, texts predictionTexts) error {
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
		jobs := groupPushJobs(group)
		if len(jobs) == 0 {
			log.Printf("群已开推送但未开启冠军/亚军6/7码，跳过: chat=%s issue=%s", chatID, message.IssueNumber)
			continue
		}
		for _, job := range jobs {
			body, ok := texts.get(job.position, job.size)
			if !ok {
				log.Printf("群需要 %s%d码 但本期文本未就绪，跳过: chat=%s issue=%s",
					job.label, job.size, chatID, message.IssueNumber)
				continue
			}
			kind := sendKind(job.position, job.size)
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

type pushJob struct {
	position int
	size     int
	label    string
}

// groupPushJobs lists champion/runner-up × size combinations enabled for a group.
// Champion and runner-up are independent; a group with only runner-up on still pushes.
func groupPushJobs(group domain.BotGroupSettings) []pushJob {
	var jobs []pushJob
	if botperm.Effective6Code(group) {
		jobs = append(jobs, pushJob{position: hotnumber.PositionChampion, size: 6, label: "冠军"})
	}
	if botperm.Effective7Code(group) {
		jobs = append(jobs, pushJob{position: hotnumber.PositionChampion, size: 7, label: "冠军"})
	}
	if botperm.EffectiveRunnerUp6Code(group) {
		jobs = append(jobs, pushJob{position: hotnumber.PositionRunnerUp, size: 6, label: "亚军"})
	}
	if botperm.EffectiveRunnerUp7Code(group) {
		jobs = append(jobs, pushJob{position: hotnumber.PositionRunnerUp, size: 7, label: "亚军"})
	}
	return jobs
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
