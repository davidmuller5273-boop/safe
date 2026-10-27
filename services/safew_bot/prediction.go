package safewbot

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/lottery/hotnumber"
	"github.com/davidmuller5273-boop/safe/internal/queue"
)

var predictionSizes = []int{hotnumber.Size6, hotnumber.Size}
var predictionPositions = []int{hotnumber.PositionChampion, hotnumber.PositionRunnerUp}

// predictionTexts maps position -> size -> rendered HTML body.
type predictionTexts map[int]map[int]string

func (t predictionTexts) set(position, size int, body string) {
	if t[position] == nil {
		t[position] = make(map[int]string)
	}
	t[position][size] = body
}

func (t predictionTexts) get(position, size int) (string, bool) {
	bySize, ok := t[position]
	if !ok {
		return "", false
	}
	body, ok := bySize[size]
	return body, ok && body != ""
}

func (t predictionTexts) len() int {
	n := 0
	for _, bySize := range t {
		n += len(bySize)
	}
	return n
}

func (t predictionTexts) keys() []string {
	out := make([]string, 0, t.len())
	for _, position := range predictionPositions {
		for _, size := range predictionSizes {
			if _, ok := t.get(position, size); ok {
				out = append(out, fmt.Sprintf("%s%d", hotnumber.PositionLabel(position), size))
			}
		}
	}
	return out
}

// predictionMessage evaluates the current issue and prepares predictions for
// champion/runner-up × 6/7-code modes when enough history exists.
// ready is true when at least one combination can be rendered.
func (w worker) predictionMessage(message queue.LotteryDrawMessage) (texts predictionTexts, ready bool, err error) {
	texts = make(predictionTexts)
	if message.LotteryTypeID == 0 {
		var record domain.DrawRecord
		if err := w.db.Select("lottery_type_id").First(&record, message.RecordID).Error; err != nil {
			return nil, false, err
		}
		message.LotteryTypeID = record.LotteryTypeID
	}

	for _, position := range predictionPositions {
		actual, err := hotnumber.FromDrawResultAt(message.Result, position)
		if err != nil {
			return nil, false, err
		}
		for _, size := range predictionSizes {
			numbers, err := w.recentHotNumbers(message.LotteryTypeID, message.IssueNumber, size, position)
			if err != nil {
				return nil, false, err
			}
			if len(numbers) < size {
				nextNumbers, err := w.recentHotNumbers(message.LotteryTypeID, message.NextIssueNumber, size, position)
				if err != nil {
					return nil, false, err
				}
				if len(nextNumbers) == size {
					if _, err := w.savePrediction(message.LotteryTypeID, message.NextIssueNumber, nextNumbers, size, position); err != nil {
						return nil, false, err
					}
				}
				continue
			}
			prediction, err := w.savePrediction(message.LotteryTypeID, message.IssueNumber, numbers, size, position)
			if err != nil {
				return nil, false, err
			}
			correct := hotnumber.Contains(numbers, actual)
			now := time.Now()
			if err := w.db.Model(&prediction).Updates(map[string]any{
				"actual_hot_number": actual,
				"correct":           correct,
				"evaluated_at":      &now,
			}).Error; err != nil {
				return nil, false, err
			}

			nextNumbers, err := w.recentHotNumbers(message.LotteryTypeID, message.NextIssueNumber, size, position)
			if err != nil {
				return nil, false, err
			}
			if len(nextNumbers) == size {
				if _, err := w.savePrediction(message.LotteryTypeID, message.NextIssueNumber, nextNumbers, size, position); err != nil {
					return nil, false, err
				}
			}
			text, err := w.renderPredictionHistory(message.LotteryTypeID, message.LotteryName, size, position)
			if err != nil {
				return nil, false, err
			}
			texts.set(position, size, text)
		}
	}
	return texts, texts.len() > 0, nil
}

func (w worker) recentHotNumbers(lotteryTypeID uint, beforeIssue string, size, position int) ([]string, error) {
	var records []domain.DrawRecord
	if err := w.db.Where("lottery_type_id = ? AND issue_number < ?", lotteryTypeID, beforeIssue).Order("issue_number DESC").Limit(100).Find(&records).Error; err != nil {
		return nil, err
	}
	return hotNumbersFromRecords(records, beforeIssue, size, position)
}

func hotNumbersFromRecords(records []domain.DrawRecord, beforeIssue string, size, position int) ([]string, error) {
	if size <= 0 {
		size = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	numbers := make([]string, 0, size)
	for _, record := range records {
		if record.IssueNumber >= beforeIssue {
			continue
		}
		var result []string
		if err := json.Unmarshal([]byte(record.DrawResult), &result); err != nil {
			return nil, fmt.Errorf("解析期号 %s 开奖结果失败: %w", record.IssueNumber, err)
		}
		actual, err := hotnumber.FromDrawResultAt(result, position)
		if err != nil {
			return nil, err
		}
		if !hotnumber.Contains(numbers, actual) {
			numbers = append(numbers, actual)
		}
		if len(numbers) == size {
			break
		}
	}
	return numbers, nil
}

func (w worker) savePrediction(lotteryTypeID uint, issue string, numbers []string, size, position int) (domain.HotNumberPrediction, error) {
	if size <= 0 {
		size = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	numbers = hotnumber.TakeFirst(numbers, size)
	state, err := json.Marshal(numbers)
	if err != nil {
		return domain.HotNumberPrediction{}, err
	}
	prediction := domain.HotNumberPrediction{
		LotteryTypeID: lotteryTypeID,
		IssueNumber:   issue,
		CodeSize:      size,
		Position:      position,
		HotNumbers:    string(state),
		Prediction:    hotnumber.Prediction(numbers),
	}
	err = w.db.Where("lottery_type_id = ? AND issue_number = ? AND code_size = ? AND position = ?", lotteryTypeID, issue, size, position).
		Assign(map[string]any{
			"hot_numbers": prediction.HotNumbers,
			"prediction":  prediction.Prediction,
			"code_size":   size,
			"position":    position,
		}).
		FirstOrCreate(&prediction).Error
	return prediction, err
}

func (w worker) renderPredictionHistory(lotteryTypeID uint, lotteryName string, size, position int) (string, error) {
	if size <= 0 {
		size = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	var predictions []domain.HotNumberPrediction
	// Only the rows shown in the table (incl. the next, unevaluated prediction)
	// are read from the predictions table. The 30/180-issue statistics are NOT
	// taken from stored predictions or running counters; they are recomputed
	// from the raw draw records below on every render.
	if err := w.db.Where("lottery_type_id = ? AND code_size = ? AND position = ?", lotteryTypeID, size, position).
		Order("issue_number DESC").Limit(predictionTableRows).Find(&predictions).Error; err != nil {
		return "", err
	}
	var records []domain.DrawRecord
	// Fetch enough raw draws to rebuild all displayed/statistical predictions
	// (statsWindow issues plus the look-back each one needs, including extra
	// rows needed when finish-place numbers repeat).
	if err := w.db.Where("lottery_type_id = ?", lotteryTypeID).Order("issue_number DESC").Limit(drawRecordFetchLimit).Find(&records).Error; err != nil {
		return "", err
	}
	recordsByIssue := make(map[string]domain.DrawRecord, len(records))
	for _, record := range records {
		recordsByIssue[record.IssueNumber] = record
	}
	for i := range predictions {
		numbers, err := hotNumbersFromRecords(records, predictions[i].IssueNumber, size, position)
		if err != nil {
			return "", err
		}
		if len(numbers) == size {
			state, err := json.Marshal(numbers)
			if err != nil {
				return "", err
			}
			value := hotnumber.Prediction(numbers)
			if predictions[i].HotNumbers != string(state) || predictions[i].Prediction != value {
				if err := w.db.Model(&predictions[i]).Updates(map[string]any{"hot_numbers": string(state), "prediction": value}).Error; err != nil {
					return "", err
				}
				predictions[i].HotNumbers = string(state)
				predictions[i].Prediction = value
			}
		}
		record, exists := recordsByIssue[predictions[i].IssueNumber]
		if !exists {
			continue
		}
		var result []string
		if err := json.Unmarshal([]byte(record.DrawResult), &result); err != nil {
			return "", err
		}
		actual, err := hotnumber.FromDrawResultAt(result, position)
		if err != nil {
			return "", err
		}
		correct := hotnumber.Contains(numbers, actual)
		if predictions[i].Correct == nil || *predictions[i].Correct != correct {
			if err := w.db.Model(&predictions[i]).Update("correct", correct).Error; err != nil {
				return "", err
			}
			predictions[i].Correct = &correct
		}
	}
	table := renderPredictionTable(predictions, size, position)
	outcomes := windowOutcomes(records, size, position, statsWindow)
	stats30 := statsFromOutcomes(outcomes, 30)
	stats180 := statsFromOutcomes(outcomes, statsWindow)
	return buildPredictionMessage(lotteryName, size, position, table, stats30, stats180), nil
}

// buildPredictionMessage formats the outbound prediction body for a given code size and position.
// Champion keeps the historical title 「N码热号预测」; runner-up uses 「亚军N码热号预测」.
func buildPredictionMessage(lotteryName string, size, position int, table string, stats30, stats180 predictionStats) string {
	if size <= 0 {
		size = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	title := fmt.Sprintf("%d码热号预测", size)
	if position == hotnumber.PositionRunnerUp {
		title = fmt.Sprintf("亚军%d码热号预测", size)
	}
	return fmt.Sprintf(
		"<b>%s %s</b>\n<pre>%s</pre>\n\n--------------------\n<b>📊 周期胜率概览</b>\n30期：%.1f%%｜180期：%.1f%%\n近三小时连错❌：%d期\n近三小时最大连中✅：%d期",
		html.EscapeString(lotteryName),
		title,
		html.EscapeString(table),
		stats30.WinRate,
		stats180.WinRate,
		stats180.MaxLossStreak,
		stats180.MaxWinStreak,
	)
}

const (
	// statsWindow is the number of most recent draw issues (1 issue/minute,
	// so 180 issues = 3 hours) used for the win-rate and streak statistics.
	statsWindow = 180
	// predictionTableRows is the number of rows shown in the history table.
	predictionTableRows = 20
	// drawRecordFetchLimit bounds how many raw draws are loaded per render.
	drawRecordFetchLimit = 1000
)

type predictionStats struct {
	WinRate       float64
	MaxWinStreak  int
	MaxLossStreak int
}

// windowOutcomes re-evaluates the last `window` draw issues from raw draw
// records only (records must be newest-first). For every issue the hot numbers
// are rebuilt from the draws strictly before it, exactly like the live
// prediction, and compared with that issue's actual number at `position`.
// The result is newest-first: true = hit (✅), false = miss (❌).
// Issues without enough earlier history (or with an unreadable result) are
// skipped, so fewer than `window` outcomes may be returned.
func windowOutcomes(records []domain.DrawRecord, size, position, window int) []bool {
	if size <= 0 {
		size = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	outcomes := make([]bool, 0, window)
	for i := 0; i < len(records) && len(outcomes) < window; i++ {
		record := records[i]
		var result []string
		if err := json.Unmarshal([]byte(record.DrawResult), &result); err != nil {
			continue
		}
		actual, err := hotnumber.FromDrawResultAt(result, position)
		if err != nil {
			continue
		}
		numbers, err := hotNumbersFromRecords(records[i+1:], record.IssueNumber, size, position)
		if err != nil {
			continue
		}
		if len(numbers) < size {
			// Not enough earlier draws; older issues will not have any either.
			break
		}
		outcomes = append(outcomes, hotnumber.Contains(numbers, actual))
	}
	return outcomes
}

// statsFromOutcomes computes win rate and the longest win/loss streaks over the
// newest `limit` outcomes (newest-first). Fewer outcomes are handled gracefully.
func statsFromOutcomes(outcomes []bool, limit int) predictionStats {
	stats := predictionStats{}
	if limit > 0 && len(outcomes) > limit {
		outcomes = outcomes[:limit]
	}
	wins, winStreak, lossStreak := 0, 0, 0
	for _, hit := range outcomes {
		if hit {
			wins++
			winStreak++
			lossStreak = 0
			if winStreak > stats.MaxWinStreak {
				stats.MaxWinStreak = winStreak
			}
		} else {
			lossStreak++
			winStreak = 0
			if lossStreak > stats.MaxLossStreak {
				stats.MaxLossStreak = lossStreak
			}
		}
	}
	if len(outcomes) > 0 {
		stats.WinRate = float64(wins) * 100 / float64(len(outcomes))
	}
	return stats
}

// calculateStats derives stats from stored predictions (newest first, pending
// ones skipped). Kept for callers/tests; the broadcast message no longer uses it.
func calculateStats(predictions []domain.HotNumberPrediction, limit int) predictionStats {
	outcomes := make([]bool, 0, len(predictions))
	for _, prediction := range predictions {
		if prediction.Correct == nil {
			continue
		}
		if len(outcomes) == limit {
			break
		}
		outcomes = append(outcomes, *prediction.Correct)
	}
	return statsFromOutcomes(outcomes, limit)
}

// renderPredictionTable receives predictions queried newest-first and renders
// them oldest-first so the latest issue is always the final row.
// predWidth should match code size (6 or 7) so the prediction column aligns.
func renderPredictionTable(predictions []domain.HotNumberPrediction, predWidth, position int) string {
	if predWidth <= 0 {
		predWidth = hotnumber.Size
	}
	if position <= 0 {
		position = hotnumber.PositionChampion
	}
	var table strings.Builder
	fmt.Fprintf(&table, "期号          预测热号(%s)  结果\n", hotnumber.PositionLabel(position))
	for i := len(predictions) - 1; i >= 0; i-- {
		status := "等待开奖"
		if predictions[i].Correct != nil && *predictions[i].Correct {
			status = "✅"
		} else if predictions[i].Correct != nil {
			status = "❌"
		}
		fmt.Fprintf(&table, "%-10s    %-*s      %s\n\n", predictions[i].IssueNumber, predWidth, predictions[i].Prediction, status)
	}
	return strings.TrimRight(table.String(), "\n")
}

// sendKind builds the Redis dedup key suffix for a position×size push.
// Champion keeps historical "6"/"7"; runner-up uses "r6"/"r7".
func sendKind(position, size int) string {
	if position == hotnumber.PositionRunnerUp {
		return fmt.Sprintf("r%d", size)
	}
	return fmt.Sprintf("%d", size)
}
