package lotterybroadcast

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	markSixRed       = map[int]bool{1: true, 2: true, 7: true, 8: true, 12: true, 13: true, 18: true, 19: true, 23: true, 24: true, 29: true, 30: true, 34: true, 35: true, 40: true, 45: true, 46: true}
	markSixBlue      = map[int]bool{3: true, 4: true, 9: true, 10: true, 14: true, 15: true, 20: true, 25: true, 26: true, 31: true, 36: true, 37: true, 41: true, 42: true, 47: true, 48: true}
	markSixWaveEmoji = map[string]string{"red": "🔴", "blue": "🔵", "green": "🟢"}
	chineseZodiac    = []string{"鼠", "牛", "虎", "兔", "龍", "蛇", "馬", "羊", "猴", "雞", "狗", "豬"}
)

// SplitNumbers mirrors split_numbers: "," "+" "|" and whitespace separate numbers.
func SplitNumbers(value string) []string {
	r := strings.NewReplacer(",", " ", "+", " ", "|", " ")
	return strings.Fields(r.Replace(value))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// MarkSixColor mirrors mark_six_color.
func MarkSixColor(number string) string {
	v := atoi(number)
	if markSixRed[v] {
		return "🔴"
	}
	if markSixBlue[v] {
		return "🔵"
	}
	return "🟢"
}

// MarkSixZodiac mirrors mark_six_zodiac.
func MarkSixZodiac(number string, year int) string {
	current := mod(year-2020, 12)
	return chineseZodiac[mod(current-(atoi(number)-1), 12)]
}

func mod(a, m int) int {
	return ((a % m) + m) % m
}

// MarkSixYear mirrors mark_six_year.
func MarkSixYear(issue, drawTime string) int {
	for _, v := range []string{prefix(issue, 4), prefix(drawTime, 4)} {
		if isDigits(v) {
			if n := atoi(v); n >= 2000 && n <= 2200 {
				return n
			}
		}
	}
	return time.Now().UTC().Year()
}

func prefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// NormalizeMarkSix mirrors normalize_mark_six_numbers(primary, secondary).
func NormalizeMarkSix(primary, secondary []string) ([]string, []string, error) {
	p, s := cleanList(primary), cleanList(secondary)
	if len(p) != 6 || len(s) != 1 {
		return nil, nil, errors.New("六合彩须为6个正码+1个特码")
	}
	seen := map[int]bool{}
	normalized := make([]string, 0, 7)
	for _, raw := range append(append([]string{}, p...), s...) {
		if !isDigits(raw) {
			return nil, nil, fmt.Errorf("六合彩号码非法：%s", raw)
		}
		v := atoi(raw)
		if v < 1 || v > 49 {
			return nil, nil, fmt.Errorf("六合彩号码超出1-49：%s", raw)
		}
		if seen[v] {
			return nil, nil, fmt.Errorf("六合彩号码重复：%02d", v)
		}
		seen[v] = true
		normalized = append(normalized, fmt.Sprintf("%02d", v))
	}
	return normalized[:6], normalized[6:], nil
}

// NormalizeMarkSixNumbers mirrors normalize_mark_six_numbers(numbers=...).
func NormalizeMarkSixNumbers(numbers []string) ([]string, []string, error) {
	values := cleanList(numbers)
	if len(values) != 7 {
		return nil, nil, errors.New("六合彩须为6个正码+1个特码")
	}
	return NormalizeMarkSix(values[:6], values[6:])
}

// IsValidMarkSix mirrors is_valid_mark_six_result.
func IsValidMarkSix(primary, secondary []string) bool {
	_, _, err := NormalizeMarkSix(primary, secondary)
	return err == nil
}

// ClassifyThreeDigit mirrors classify_three_digit (豹子/顺子/组三/组六).
func ClassifyThreeDigit(primary []string) (string, error) {
	if len(primary) != 3 {
		return "", errors.New("三位彩须为3个号码")
	}
	d := make([]int, 3)
	for i, v := range primary {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return "", fmt.Errorf("三位彩号码非法：%s", v)
		}
		d[i] = n
	}
	a, b, c := d[0], d[1], d[2]
	if a == b && b == c {
		return "豹子", nil
	}
	unique := map[int]bool{a: true, b: true, c: true}
	if len(unique) == 3 {
		o := []int{a, b, c}
		for i := 0; i < 3; i++ {
			for j := i + 1; j < 3; j++ {
				if o[j] < o[i] {
					o[i], o[j] = o[j], o[i]
				}
			}
		}
		linear := o[0]+1 == o[1] && o[1]+1 == o[2]
		circular := (unique[0] && unique[8] && unique[9]) || (unique[0] && unique[1] && unique[9])
		if linear || circular {
			return "顺子", nil
		}
		return "组六", nil
	}
	if len(unique) == 2 {
		return "组三", nil
	}
	return "组六", nil
}

// NormalizeThreeDigit mirrors normalize_three_digit_numbers.
func NormalizeThreeDigit(primary []string) ([]string, error) {
	values := cleanList(primary)
	if len(values) != 3 {
		return nil, errors.New("福彩3D/排列3须为恰好3个号码")
	}
	out := make([]string, 0, 3)
	for _, raw := range values {
		if !isDigits(raw) {
			return nil, fmt.Errorf("三位彩号码非法：%s", raw)
		}
		v := atoi(raw)
		if v < 0 || v > 9 {
			return nil, fmt.Errorf("三位彩号码须为0-9：%s", raw)
		}
		out = append(out, strconv.Itoa(v))
	}
	return out, nil
}

// IsValidThreeDigit mirrors is_valid_three_digit_result.
func IsValidThreeDigit(primary []string) bool {
	_, err := NormalizeThreeDigit(primary)
	return err == nil
}

// ValidateNumbers mirrors validate_lottery_numbers.
func ValidateNumbers(code string, primary, secondary []string) error {
	if IsThreeDigit(code) {
		if _, err := NormalizeThreeDigit(primary); err != nil {
			return err
		}
		if len(secondary) > 0 {
			return errors.New("福彩3D/排列3不应包含特别号")
		}
		return nil
	}
	if IsMarkSix(code) {
		_, _, err := NormalizeMarkSix(primary, secondary)
		return err
	}
	return nil
}

// IsValidResult mirrors is_valid_lottery_result.
func IsValidResult(r Result) bool {
	return ValidateNumbers(r.GameCode, r.Primary, r.Secondary) == nil
}

// NormalizeResult mirrors normalize_lottery_result.
func NormalizeResult(r Result) (Result, error) {
	if IsThreeDigit(r.GameCode) {
		p, err := NormalizeThreeDigit(r.Primary)
		if err != nil {
			return r, err
		}
		if len(r.Secondary) > 0 {
			return r, errors.New("福彩3D/排列3不应包含特别号")
		}
		r.Primary, r.Secondary = p, nil
		return r, nil
	}
	if IsMarkSix(r.GameCode) {
		p, s, err := NormalizeMarkSix(r.Primary, r.Secondary)
		if err != nil {
			return r, err
		}
		r.Primary, r.Secondary = p, s
		return r, nil
	}
	return r, nil
}

func normalizeIfStrict(r Result) (Result, error) {
	if IsThreeDigit(r.GameCode) || IsMarkSix(r.GameCode) {
		return NormalizeResult(r)
	}
	return r, nil
}
