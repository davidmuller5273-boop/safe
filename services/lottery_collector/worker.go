// Package lotterycollector provides the standalone lottery collection process.
package lotterycollector

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/config"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/lottery/bianjsft"
	"github.com/davidmuller5273-boop/safe/internal/platform/bsc"
	"github.com/davidmuller5273-boop/safe/internal/platform/database"
	"github.com/davidmuller5273-boop/safe/internal/queue"

	"gorm.io/gorm"
)

const collectionPollInterval = time.Second

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
	if err := drawQueue.Ping(context.Background()); err != nil {
		return err
	}

	lotteryType := domain.LotteryType{Symbol: bianjsft.Symbol}
	if err := db.Where("symbol = ?", bianjsft.Symbol).FirstOrCreate(&lotteryType, domain.LotteryType{
		Name:            "币安极速飞艇",
		Symbol:          bianjsft.Symbol,
		SecondsPerIssue: 60,
		IssuesPerDay:    1440,
		DrawsAllDay:     true,
	}).Error; err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	collector := collector{db: db, client: bsc.NewClient(cfg.BSCRPCURL), queue: drawQueue, lotteryType: lotteryType}
	log.Printf("币安极速飞艇采集已启动: symbol=%s, block_timezone=UTC, issue_timezone=Asia/Shanghai", bianjsft.Symbol)
	return collector.run(ctx)
}

type collector struct {
	db          *gorm.DB
	client      *bsc.Client
	queue       *queue.LotteryDrawQueueClient
	lotteryType domain.LotteryType
}

func (c collector) run(ctx context.Context) error {
	var completedIssue string

	for {
		now := time.Now().UTC()
		minute := now.Truncate(time.Minute)
		issue := bianjsft.IssueNumber(minute)
		if now.Second() >= 3 && issue != completedIssue {
			if err := c.collect(ctx, minute); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("采集 %s 失败，将自动重试: %v", issue, err)
			} else {
				completedIssue = issue
			}
		}

		// Align polling to natural whole-second boundaries. This avoids carrying
		// the process startup millisecond offset into every subsequent request.
		nextPoll := time.Now().Truncate(collectionPollInterval).Add(collectionPollInterval)
		timer := time.NewTimer(time.Until(nextPoll))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Println("彩票采集任务已停止")
			return nil
		case <-timer.C:
		}
	}
}

func (c collector) collect(ctx context.Context, minute time.Time) error {
	target := bianjsft.WinningBlockTime(minute)
	block, err := c.client.FirstBlockAtOrAfter(ctx, target)
	if err != nil {
		return err
	}
	if !block.Timestamp.Before(minute.Add(time.Minute)) {
		return errors.New("本分钟没有找到符合规则的获胜区块")
	}
	result, err := bianjsft.Result(block.Hash)
	if err != nil {
		return err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	nextMinute := minute.Add(time.Minute)
	record := domain.DrawRecord{
		LotteryTypeID:     c.lotteryType.ID,
		IssueNumber:       bianjsft.IssueNumber(minute),
		DrawResult:        string(resultJSON),
		DrawTimestamp:     block.Timestamp.Unix(),
		NextDrawTimestamp: bianjsft.WinningBlockTime(nextMinute).Unix(),
		NextIssueNumber:   bianjsft.IssueNumber(nextMinute),
	}
	if err := c.db.Where(domain.DrawRecord{LotteryTypeID: record.LotteryTypeID, IssueNumber: record.IssueNumber}).FirstOrCreate(&record).Error; err != nil {
		return err
	}
	published, err := c.queue.Publish(ctx, queue.LotteryDrawMessage{
		RecordID:          record.ID,
		LotteryTypeID:     record.LotteryTypeID,
		LotteryName:       c.lotteryType.Name,
		Symbol:            c.lotteryType.Symbol,
		IssueNumber:       record.IssueNumber,
		Result:            result,
		DrawTimestamp:     record.DrawTimestamp,
		NextIssueNumber:   record.NextIssueNumber,
		NextDrawTimestamp: record.NextDrawTimestamp,
	})
	if err != nil {
		return err
	}
	log.Printf("采集成功: issue=%s block=%d hash=%s result=%s", record.IssueNumber, block.Number, block.Hash, record.DrawResult)
	if published {
		log.Printf("开奖记录已投递至 SafeW 消息队列: issue=%s", record.IssueNumber)
	}
	return nil
}
