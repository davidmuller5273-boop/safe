package queue

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	LotteryDrawQueue      = "safe:queue:lottery_draw"
	LotteryDrawProcessing = "safe:queue:lottery_draw:processing"
)

type LotteryDrawMessage struct {
	RecordID          uint     `json:"record_id"`
	LotteryTypeID     uint     `json:"lottery_type_id"`
	LotteryName       string   `json:"lottery_name"`
	Symbol            string   `json:"symbol"`
	IssueNumber       string   `json:"issue_number"`
	Result            []string `json:"result"`
	DrawTimestamp     int64    `json:"draw_timestamp"`
	NextIssueNumber   string   `json:"next_issue_number"`
	NextDrawTimestamp int64    `json:"next_draw_timestamp"`
}

type LotteryDrawQueueClient struct{ client *redis.Client }

func NewLotteryDrawQueue(addr, password string, db int) *LotteryDrawQueueClient {
	return &LotteryDrawQueueClient{client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})}
}

func (q *LotteryDrawQueueClient) Close() error                   { return q.client.Close() }
func (q *LotteryDrawQueueClient) Ping(ctx context.Context) error { return q.client.Ping(ctx).Err() }

var publishScript = redis.NewScript(`
if redis.call("SET", KEYS[1], "1", "NX", "EX", ARGV[1]) then
  redis.call("LPUSH", KEYS[2], ARGV[2])
  return 1
end
return 0
`)

func (q *LotteryDrawQueueClient) Publish(ctx context.Context, message LotteryDrawMessage) (bool, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return false, err
	}
	dedupeKey := fmt.Sprintf("safe:dedupe:lottery_draw:%d", message.RecordID)
	result, err := publishScript.Run(ctx, q.client, []string{dedupeKey, LotteryDrawQueue}, int((7 * 24 * time.Hour).Seconds()), payload).Int()
	return result == 1, err
}

func (q *LotteryDrawQueueClient) Recover(ctx context.Context) error {
	for {
		_, err := q.client.RPopLPush(ctx, LotteryDrawProcessing, LotteryDrawQueue).Result()
		if err == redis.Nil {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func (q *LotteryDrawQueueClient) Receive(ctx context.Context) (string, LotteryDrawMessage, error) {
	payload, err := q.client.BRPopLPush(ctx, LotteryDrawQueue, LotteryDrawProcessing, 0).Result()
	if err != nil {
		return "", LotteryDrawMessage{}, err
	}
	var message LotteryDrawMessage
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return payload, LotteryDrawMessage{}, err
	}
	return payload, message, nil
}

func (q *LotteryDrawQueueClient) Ack(ctx context.Context, payload string) error {
	return q.client.LRem(ctx, LotteryDrawProcessing, 1, payload).Err()
}

func (q *LotteryDrawQueueClient) WasSent(ctx context.Context, recordID uint, chatID string) (bool, error) {
	count, err := q.client.Exists(ctx, sentKey(recordID, chatID)).Result()
	return count > 0, err
}

func (q *LotteryDrawQueueClient) MarkSent(ctx context.Context, recordID uint, chatID string) error {
	return q.client.Set(ctx, sentKey(recordID, chatID), "1", 30*24*time.Hour).Err()
}

func sentKey(recordID uint, chatID string) string {
	hash := sha256.Sum256([]byte(chatID))
	return fmt.Sprintf("safe:sent:lottery_draw:%d:%x", recordID, hash[:8])
}
