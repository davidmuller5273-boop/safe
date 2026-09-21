// Package bianjsft implements the draw rules for 币安极速飞艇.
package bianjsft

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const Symbol = "bianjsft"

var beijingLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// Result derives the ten finishing car numbers from a BSC block hash.
func Result(blockHash string) ([]string, error) {
	hash := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(blockHash)), "0x")
	seen := make(map[byte]bool, 16)
	type car struct {
		number int
		value  byte
	}
	cars := make([]car, 0, 10)
	for i := len(hash) - 1; i >= 0 && len(cars) < 10; i-- {
		value, ok := hexValue(hash[i])
		if !ok {
			return nil, fmt.Errorf("区块哈希包含非法字符 %q", hash[i])
		}
		if seen[hash[i]] {
			continue
		}
		seen[hash[i]] = true
		cars = append(cars, car{number: len(cars) + 1, value: value})
	}
	if len(cars) != 10 {
		return nil, errors.New("区块哈希中不足 10 个不重复字符")
	}
	sort.Slice(cars, func(i, j int) bool { return cars[i].value > cars[j].value })
	result := make([]string, len(cars))
	for i, item := range cars {
		result[i] = fmt.Sprintf("%02d", item.number)
	}
	return result, nil
}

// IssueNumber uses Beijing time (UTC+8) and formats an issue as YYMMDD plus a four-digit daily
// sequence. A draw day starts at 00:01 with issue 0001 and ends at 00:00 on
// the following calendar day with issue 1440.
func IssueNumber(drawMinute time.Time) string {
	drawMinute = drawMinute.In(beijingLocation).Truncate(time.Minute)
	drawDate := drawMinute
	issue := drawMinute.Hour()*60 + drawMinute.Minute()
	if issue == 0 {
		drawDate = drawDate.AddDate(0, 0, -1)
		issue = 1440
	}
	return fmt.Sprintf("%s%04d", drawDate.Format("060102"), issue)
}

// WinningBlockTime is the earliest eligible block time in a UTC minute.
func WinningBlockTime(drawMinute time.Time) time.Time {
	return drawMinute.UTC().Truncate(time.Minute).Add(3 * time.Second)
}

func hexValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	default:
		return 0, false
	}
}
