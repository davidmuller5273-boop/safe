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

// predictionMessage evaluates the current issue and prepares predictions for
// both 6-code and 7-code modes when enough history exists.
// ready is true when at least one size can be rendered.
func (w worker) predictionMessage(message queue.LotteryDrawMessage) (texts map[int]string, ready bool, err error) {
	texts = make(map[int]string)
	if message.LotteryTypeID == 0 {
		var record domain.DrawRecord
		if err := w.db.Select("lottery_type_id").First(&record, message.RecordID).Error; err != nil {
			return nil, false, err
		}
		message.LotteryTypeID = record.LotteryTypeID
	}
	actual, err := hotnumber.FromDrawResult(message.Result)
	if err != nil {
		return nil, false, err
	}

	for _, size := range predictionSizes {
		numbers, err := w.recentHotNumbers(message.LotteryTypeID, message.IssueNumber, size)
		if err != nil {
			return nil, false, err
		}
		if len(numbers) < size {
			nextNumbers, err := w.recentHotNumbers(message.LotteryTypeID, message.NextIssueNumber, size)
			if err != nil {
				return nil, false, err
			}
			if len(nextNumbers) == size {
				if _, err := w.savePrediction(message.LotteryTypeID, message.NextIssueNumber, nextNumbers, size); err != nil {
					return nil, false, err
				}
			}
			continue
		}
		prediction, err := w.savePrediction(message.LotteryTypeID, message.IssueNumber, numbers, size)
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

		nextNumbers, err := w.recentHotNumbers(message.LotteryTypeID, message.NextIssueNumber, size)
		if err != nil {
			return nil, false, err
		}
		if len(nextNumbers) == size {
			if _, err := w.savePrediction(message.LotteryTypeID, message.NextIssueNumber, nextNumbers, size); err != nil {
				return nil, false, err
			}
		}
		text, err := w.renderPredictionHistory(message.LotteryTypeID, message.LotteryName, size)
		if err != nil {
			return nil, false, err
		}
		texts[size] = text
	}
	return texts, len(texts) > 0, nil
}

func (w worker) recentHotNumbers(lotteryTypeID uint, beforeIssue string, size int) ([]string, error) {
	var records []domain.DrawRecord
	if err := w.db.Where("lottery_type_id = ? AND issue_number < ?", lotteryTypeID, beforeIssue).Order("issue_number DESC").Limit(100).Find(&records).Error; err != nil {
		return nil, err
	}
	return hotNumbersFromRecords(records, beforeIssue, size)
}

func hotNumbersFromRecords(records []domain.DrawRecord, beforeIssue string, size int) ([]string, error) {
	if size <= 0 {
		size = hotnumber.Size
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
		actual, err := hotnumber.FromDrawResult(result)
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

func (w worker) savePrediction(lotteryTypeID uint, issue string, numbers []string, size int) (domain.HotNumberPrediction, error) {
	if size <= 0 {
		size = hotnumber.Size
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
		HotNumbers:    string(state),
		Prediction:    hotnumber.Prediction(numbers),
	}
	err = w.db.Where("lottery_type_id = ? AND issue_number = ? AND code_size = ?", lotteryTypeID, issue, size).
		Assign(map[string]any{"hot_numbers": prediction.HotNumbers, "prediction": prediction.Prediction, "code_size": size}).
		FirstOrCreate(&prediction).Error
	return prediction, err
}

func (w worker) renderPredictionHistory(lotteryTypeID uint, lotteryName string, size int) (string, error) {
	if size <= 0 {
		size = hotnumber.Size
	}
	var predictions []domain.HotNumberPrediction
	// Include the next, unevaluated prediction. Fetch one extra row so the
	// statistics can still cover 180 completed issues.
	if err := w.db.Where("lottery_type_id = ? AND code_size = ?", lotteryTypeID, size).Order("issue_number DESC").Limit(181).Find(&predictions).Error; err != nil {
		return "", err
	}
	var records []domain.DrawRecord
	// Fetch enough raw draws to rebuild all displayed/statistical predictions,
	// including extra rows needed when first-place numbers repeat.
	if err := w.db.Where("lottery_type_id = ?", lotteryTypeID).Order("issue_number DESC").Limit(1000).Find(&records).Error; err != nil {
		return "", err
	}
	recordsByIssue := make(map[string]domain.DrawRecord, len(records))
	for _, record := range records {
		recordsByIssue[record.IssueNumber] = record
	}
	for i := range predictions {
		numbers, err := hotNumbersFromRecords(records, predictions[i].IssueNumber, size)
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
		actual, err := hotnumber.FromDrawResult(result)
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
	tablePredictions := predictions
	if len(tablePredictions) > 20 {
		tablePredictions = tablePredictions[:20]
	}
	table := renderPredictionTable(tablePredictions)
	stats30 := calculateStats(predictions, 30)
	stats180 := calculateStats(predictions, 180)
	return fmt.Sprintf(
		"<b>%s %d码热号预测</b>\n<pre>%s</pre>\n\n--------------------\n<b>📊 周期胜率概览</b>\n30期：%.1f%%｜180期：%.1f%%\n近三小时最大连错：%d❌期\n近三小时最大连中：%d✅期",
		html.EscapeString(lotteryName),
		size,
		html.EscapeString(table),
		stats30.WinRate,
		stats180.WinRate,
		stats180.MaxLossStreak,
		stats180.MaxWinStreak,
	), nil
}

type predictionStats struct {
	WinRate       float64
	MaxWinStreak  int
	MaxLossStreak int
}

func calculateStats(predictions []domain.HotNumberPrediction, limit int) predictionStats {
	stats := predictionStats{}
	wins := 0
	winStreak := 0
	lossStreak := 0
	evaluated := 0
	for _, prediction := range predictions {
		if prediction.Correct == nil {
			continue
		}
		if evaluated == limit {
			break
		}
		evaluated++
		if *prediction.Correct {
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
	if evaluated > 0 {
		stats.WinRate = float64(wins) * 100 / float64(evaluated)
	}
	return stats
}

// renderPredictionTable receives predictions queried newest-first and renders
// them oldest-first so the latest issue is always the final row.
func renderPredictionTable(predictions []domain.HotNumberPrediction) string {
	var table strings.Builder
	table.WriteString("期号          预测热号(冠军)  结果\n")
	for i := len(predictions) - 1; i >= 0; i-- {
		status := "等待开奖"
		if predictions[i].Correct != nil && *predictions[i].Correct {
			status = "✅"
		} else if predictions[i].Correct != nil {
			status = "❌"
		}
		fmt.Fprintf(&table, "%-10s    %-7s      %s\n\n", predictions[i].IssueNumber, predictions[i].Prediction, status)
	}
	return strings.TrimRight(table.String(), "\n")
}
