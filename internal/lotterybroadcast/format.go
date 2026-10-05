package lotterybroadcast

import (
	"fmt"
	"strings"
)

// SourceNames mirrors source_names in format_lottery_result.
var SourceNames = map[string]string{
	"cwl":         "中国福彩网",
	"sport":       "中国体彩网",
	"realtime168": "168开奖实时接口",
	"hkjc":        "香港赛马会官方",
	"marksix6":    "第三方 Marksix6（非澳门官方）",
	"macaujc":     "macaujc.com（第三方，非澳门官方）",
	"huiniao":     "慧鸟接口",
}

// SourceLabel returns the display name for a source key.
func SourceLabel(source string) string {
	if v, ok := SourceNames[source]; ok {
		return v
	}
	return source
}

// BroadcastPrefix is prepended to every broadcast except 双色球.
const BroadcastPrefix = "彩票开奖播报\n\n"

func joinMarkSixColumns(cells []string) string {
	return strings.Join(cells[:6], " ") + " + " + cells[6]
}

func pad2(v string) string {
	if isDigits(v) {
		return fmt.Sprintf("%02d", atoi(v))
	}
	return v
}

// FormatMarkSixNumbers mirrors format_mark_six_numbers (3 lines: 号码 / 生肖 / 波色).
func FormatMarkSixNumbers(issue, drawTime string, primary, secondary, zodiac, wave []string) string {
	year := MarkSixYear(issue, drawTime)
	p, s, err := NormalizeMarkSix(primary, secondary)
	if err != nil {
		numbers := make([]string, 0, len(primary)+len(secondary))
		for _, x := range primary {
			numbers = append(numbers, pad2(x))
		}
		for _, x := range secondary {
			numbers = append(numbers, pad2(x))
		}
		if len(numbers) != 7 {
			return strings.Join(numbers, " ")
		}
		z := make([]string, 7)
		c := make([]string, 7)
		for i, n := range numbers {
			if isDigits(n) {
				z[i], c[i] = MarkSixZodiac(n, year), MarkSixColor(n)
			} else {
				z[i], c[i] = "?", "🟢"
			}
		}
		return strings.Join([]string{joinMarkSixColumns(numbers), joinMarkSixColumns(z), joinMarkSixColumns(c)}, "\n")
	}
	numbers := append(append([]string{}, p...), s...)
	var z []string
	if len(zodiac) == 7 {
		for _, item := range zodiac {
			z = append(z, strings.TrimSpace(item))
		}
	} else {
		for _, n := range numbers {
			z = append(z, MarkSixZodiac(n, year))
		}
	}
	var c []string
	if len(wave) == 7 {
		for _, item := range wave {
			raw := strings.TrimSpace(item)
			switch {
			case raw == "🔴" || raw == "🔵" || raw == "🟢":
				c = append(c, raw)
			case markSixWaveEmoji[strings.ToLower(raw)] != "":
				c = append(c, markSixWaveEmoji[strings.ToLower(raw)])
			case isDigits(raw):
				c = append(c, MarkSixColor(raw))
			default:
				c = append(c, "🟢")
			}
		}
	} else {
		for _, n := range numbers {
			c = append(c, MarkSixColor(n))
		}
	}
	return strings.Join([]string{joinMarkSixColumns(numbers), joinMarkSixColumns(z), joinMarkSixColumns(c)}, "\n")
}

// FormatResult mirrors format_lottery_result (plain text, not HTML-escaped).
func FormatResult(r Result) string {
	drawTime := r.DrawTime
	if drawTime == "" {
		drawTime = "数据源未提供具体时间"
	}
	source := SourceLabel(r.Source)
	if r.GameCode == "ssq" {
		red := make([]string, 0, len(r.Primary))
		for _, n := range r.Primary {
			red = append(red, fmt.Sprintf("%02d", atoi(n)))
		}
		blue := make([]string, 0, len(r.Secondary))
		for _, n := range r.Secondary {
			blue = append(blue, fmt.Sprintf("%02d", atoi(n)))
		}
		return fmt.Sprintf("福彩双色球第:%s期开奖结果:\n🔴%s\n🔵%s\n开奖时间：%s\n数据源：%s",
			r.Issue, strings.Join(red, " "), strings.Join(blue, " "), drawTime, source)
	}
	if IsMarkSix(r.GameCode) {
		numberText := FormatMarkSixNumbers(r.Issue, drawTime, r.Primary, r.Secondary, r.Zodiac, r.Wave)
		return fmt.Sprintf("%s 第%s期\n%s\n开奖时间：%s\n数据源：%s", r.GameName, r.Issue, numberText, drawTime, source)
	}
	if IsThreeDigit(r.GameCode) {
		numberText := "开奖号码：" + strings.Join(r.Primary, " ")
		if digits, err := NormalizeThreeDigit(r.Primary); err == nil {
			if kind, err := ClassifyThreeDigit(digits); err == nil {
				numberText = fmt.Sprintf("开奖号码：%s（%s）", strings.Join(digits, " "), kind)
			}
		}
		return fmt.Sprintf("%s 第%s期\n%s\n开奖时间：%s\n数据源：%s", r.GameName, r.Issue, numberText, drawTime, source)
	}
	numbers := strings.Join(r.Primary, " ")
	if len(r.Secondary) > 0 {
		numbers += " + " + strings.Join(r.Secondary, " ")
	}
	return fmt.Sprintf("%s 第%s期\n开奖号码：%s\n开奖时间：%s\n数据源：%s", r.GameName, r.Issue, numbers, drawTime, source)
}

// BroadcastBody is the text queued for subscribed groups (双色球 without prefix).
func BroadcastBody(r Result) string {
	formatted := FormatResult(r)
	if r.GameCode == "ssq" {
		return formatted
	}
	return BroadcastPrefix + formatted
}

// HistoryPageSize mirrors LOTTERY_HISTORY_PAGE_SIZE.
const HistoryPageSize = 10

// HistoryPage mirrors lottery_history_page text part; returns text, page, pageCount.
func HistoryPage(code string, rows []Result, page int) (string, int, int) {
	game := Games[code]
	pageCount := (len(rows) + HistoryPageSize - 1) / HistoryPageSize
	if pageCount < 1 {
		pageCount = 1
	}
	if page < 0 {
		page = 0
	}
	if page > pageCount-1 {
		page = pageCount - 1
	}
	start := page * HistoryPageSize
	end := start + HistoryPageSize
	if end > len(rows) {
		end = len(rows)
	}
	lines := []string{fmt.Sprintf("%s 历史开奖（第 %d/%d 页，共%d期）", game.Name, page+1, pageCount, len(rows))}
	for _, r := range rows[start:end] {
		numbers := strings.Join(r.Primary, " ")
		if len(r.Secondary) > 0 {
			numbers += " + " + strings.Join(r.Secondary, " ")
		}
		drawTime := r.DrawTime
		if drawTime == "" {
			drawTime = "第三方历史页未提供时间"
		}
		if IsMarkSix(code) {
			numbers = FormatMarkSixNumbers(r.Issue, drawTime, r.Primary, r.Secondary, nil, nil)
		}
		lines = append(lines, fmt.Sprintf("第%s期 · %s\n%s", r.Issue, drawTime, numbers))
	}
	if len(rows) == 0 {
		lines = append(lines, "暂无历史开奖缓存。")
	}
	return strings.Join(lines, "\n\n"), page, pageCount
}
