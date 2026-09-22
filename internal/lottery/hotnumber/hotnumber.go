package hotnumber

import (
	"errors"
	"sort"
	"strings"
)

const (
	Size  = 7
	Size6 = 6
)

// FromDrawResult returns the first-place number using 0 to represent car 10.
func FromDrawResult(result []string) (string, error) {
	if len(result) == 0 {
		return "", errors.New("开奖结果为空")
	}
	value := strings.TrimLeft(result[0], "0")
	if value == "" {
		value = "0"
	}
	if value == "10" {
		return "0", nil
	}
	if len(value) != 1 || value[0] < '1' || value[0] > '9' {
		return "", errors.New("开奖号码第一名不在 01-10 范围内")
	}
	return value, nil
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
