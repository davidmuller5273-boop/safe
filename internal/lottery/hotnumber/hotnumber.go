package hotnumber

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	Size  = 7
	Size6 = 6

	// PositionChampion is 冠军 (1st place / index 0).
	PositionChampion = 1
	// PositionRunnerUp is 亚军 (2nd place / index 1).
	PositionRunnerUp = 2
)

// FromDrawResult returns the first-place (冠军) number using 0 to represent car 10.
func FromDrawResult(result []string) (string, error) {
	return FromDrawResultAt(result, PositionChampion)
}

// FromDrawResultAt returns the number at the given 1-based finish position
// (1=冠军, 2=亚军), using 0 to represent car 10.
func FromDrawResultAt(result []string, position int) (string, error) {
	if position < 1 {
		return "", fmt.Errorf("名次必须从 1 开始，收到 %d", position)
	}
	idx := position - 1
	if len(result) <= idx {
		return "", fmt.Errorf("开奖结果不足 %d 名", position)
	}
	value := strings.TrimLeft(result[idx], "0")
	if value == "" {
		value = "0"
	}
	if value == "10" {
		return "0", nil
	}
	if len(value) != 1 || value[0] < '1' || value[0] > '9' {
		return "", errors.New("开奖号码不在 01-10 范围内")
	}
	return value, nil
}

// PositionLabel returns the Chinese label for a finish position.
func PositionLabel(position int) string {
	switch position {
	case PositionRunnerUp:
		return "亚军"
	default:
		return "冠军"
	}
}

func Prediction(numbers []string) string {
	values := append([]string(nil), numbers...)
	sort.Slice(values, func(i, j int) bool {
		if values[i] == "0" {
			return false
		}
		if values[j] == "0" {
			return true
		}
		return values[i] < values[j]
	})
	return strings.Join(values, "")
}

func Contains(numbers []string, actual string) bool {
	for _, number := range numbers {
		if number == actual {
			return true
		}
	}
	return false
}

// TakeFirst returns the first n elements of numbers, or all of them if shorter.
func TakeFirst(numbers []string, n int) []string {
	if n <= 0 {
		return []string{}
	}
	if len(numbers) <= n {
		return append([]string(nil), numbers...)
	}
	return append([]string(nil), numbers[:n]...)
}
