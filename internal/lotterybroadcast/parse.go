package lotterybroadcast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// DecodeJSON decodes with UseNumber so numeric ids keep their text form.
func DecodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

// str mirrors Python str(value or ""): nil / false / "" → "".
func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "True"
		}
		return ""
	default:
		return fmt.Sprint(t)
	}
}

func toInt(v any) (int, error) {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return 0, errors.New("缺少数字字段")
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("数字字段非法：%s", s)
	}
	return int(f), nil
}

func splitAt(game Game, numbers []string) ([]string, []string) {
	at := len(numbers)
	if game.SecondaryCount > 0 {
		at = len(numbers) - game.SecondaryCount
		if at < 0 {
			at = 0
		}
	}
	return append([]string{}, numbers[:at]...), append([]string{}, numbers[at:]...)
}

func firstNonEmpty(values ...any) string {
	for _, v := range values {
		if s := str(v); s != "" {
			return s
		}
	}
	return ""
}

// ParseRealtime mirrors LotteryService.parse_realtime (168 开奖实时接口).
func ParseRealtime(game Game, payload any, detailURL string) (Result, error) {
	m := asMap(payload)
	code := -1
	if v, ok := m["errorCode"]; ok {
		n, err := toInt(v)
		if err != nil {
			return Result{}, err
		}
		code = n
	}
	if code != 0 {
		msg := str(m["message"])
		if msg == "" {
			msg = "接口返回错误"
		}
		return Result{}, errors.New(msg)
	}
	row := asMap(asMap(asMap(m["result"])["data"]))
	if row == nil {
		return Result{}, errors.New("实时接口返回缺少 data")
	}
	numbers := SplitNumbers(str(row["preDrawCode"]))
	if len(numbers) == 0 {
		return Result{}, errors.New("实时接口返回缺少开奖号码")
	}
	issue := str(row["preDrawIssue"])
	if issue == "" {
		return Result{}, errors.New("实时接口返回缺少期号")
	}
	p, s := splitAt(game, numbers)
	r := Result{
		Source: "realtime168", GameCode: game.Code, GameName: game.Name,
		Issue: issue, DrawTime: str(row["preDrawTime"]), Primary: p, Secondary: s,
		DetailURL: detailURL, NextDrawTime: firstNonEmpty(row["drawTime"], row["nextDrawTime"]),
	}
	return normalizeIfStrict(r)
}

// ParseCWL mirrors parse_cwl (one row of 中国福彩网 findDrawNotice).
func ParseCWL(game Game, row map[string]any, baseURL string) (Result, error) {
	if baseURL == "" {
		baseURL = "https://www.cwl.gov.cn/"
	}
	primary := SplitNumbers(str(row["red"]))
	secondary := SplitNumbers(firstNonEmpty(row["blue"], row["blue2"]))
	if len(primary) == 0 {
		return Result{}, errors.New("官方返回缺少开奖号码")
	}
	code, date := str(row["code"]), str(row["date"])
	if code == "" {
		return Result{}, errors.New("官方返回缺少期号")
	}
	detail := str(row["detailsLink"])
	detailURL := baseURL
	if b, err := url.Parse(baseURL); err == nil {
		if ref, err := url.Parse(detail); err == nil {
			detailURL = b.ResolveReference(ref).String()
		}
	}
	r := Result{Source: "cwl", GameCode: game.Code, GameName: game.Name, Issue: code, DrawTime: date,
		Primary: primary, Secondary: secondary, DetailURL: detailURL}
	return normalizeIfStrict(r)
}

// ParseCWLHistory mirrors parse_cwl_history (any bad row fails the page, like Python).
func ParseCWLHistory(game Game, payload any, baseURL string) ([]Result, error) {
	var out []Result
	for _, item := range asList(asMap(payload)["result"]) {
		r, err := ParseCWL(game, asMap(item), baseURL)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ParseSport mirrors parse_sport (one row of 中国体彩网 list).
func ParseSport(game Game, row map[string]any) (Result, error) {
	numbers := SplitNumbers(str(row["lotteryDrawResult"]))
	if len(numbers) == 0 {
		return Result{}, errors.New("官方返回缺少开奖号码")
	}
	issue := str(row["lotteryDrawNum"])
	if issue == "" {
		return Result{}, errors.New("官方返回缺少期号")
	}
	p, s := splitAt(game, numbers)
	r := Result{Source: "sport", GameCode: game.Code, GameName: game.Name, Issue: issue,
		DrawTime: str(row["lotteryDrawTime"]), Primary: p, Secondary: s}
	return normalizeIfStrict(r)
}

// ParseSportHistory mirrors parse_sport_history.
func ParseSportHistory(game Game, payload any) ([]Result, error) {
	var out []Result
	for _, item := range asList(asMap(asMap(payload)["value"])["list"]) {
		r, err := ParseSport(game, asMap(item))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// ParsePublicRepoHistory mirrors parse_public_repo_history (lottery-data-repo JSON).
func ParsePublicRepoHistory(game Game, payload any, detailURL string) []Result {
	var out []Result
	for _, item := range asList(asMap(payload)["draws"]) {
		row := asMap(item)
		if row == nil {
			continue
		}
		numbers := SplitNumbers(str(row["number_raw"]))
		if len(numbers) == 0 {
			if groups := asMap(row["numbers"]); groups != nil {
				for _, key := range []string{"red", "blue", "front", "back", "digits", "nums"} {
					for _, v := range asList(groups[key]) {
						s := str(v)
						if isDigits(s) {
							if IsThreeDigit(game.Code) {
								s = strconv.Itoa(atoi(s))
							} else {
								s = fmt.Sprintf("%02d", atoi(s))
							}
						}
						numbers = append(numbers, s)
					}
				}
			}
		}
		if len(numbers) == 0 {
			continue
		}
		issue := str(row["issue"])
		if issue == "" {
			continue
		}
		p, s := splitAt(game, numbers)
		r := Result{Source: game.Source, GameCode: game.Code, GameName: game.Name, Issue: issue,
			DrawTime: firstNonEmpty(row["draw_date"], row["draw_time"]), Primary: p, Secondary: s, DetailURL: detailURL}
		if IsThreeDigit(game.Code) || IsMarkSix(game.Code) {
			nr, err := NormalizeResult(r)
			if err != nil {
				continue
			}
			r = nr
		}
		out = append(out, r)
	}
	return out
}

// ParseHKJCHistory mirrors parse_hkjc_history (香港赛马会 GraphQL).
func ParseHKJCHistory(game Game, payload any) []Result {
	var out []Result
	for _, item := range asList(asMap(asMap(payload)["data"])["lotteryDraws"]) {
		row := asMap(item)
		draw := asMap(row["drawResult"])
		var primary []string
		bad := false
		for _, v := range asList(draw["drawnNo"]) {
			n, err := toInt(v)
			if err != nil {
				bad = true
				break
			}
			primary = append(primary, fmt.Sprintf("%02d", n))
		}
		extra, hasExtra := draw["xDrawnNo"]
		if bad || str(row["status"]) != "Result" || len(primary) != 6 || !hasExtra || extra == nil {
			continue
		}
		x, err := toInt(extra)
		if err != nil {
			continue
		}
		p, s, err := NormalizeMarkSix(primary, []string{fmt.Sprintf("%02d", x)})
		if err != nil {
			continue
		}
		year, err1 := toInt(row["year"])
		no, err2 := toInt(row["no"])
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, Result{Source: "hkjc", GameCode: game.Code, GameName: game.Name,
			Issue: fmt.Sprintf("%04d%03d", year, no), DrawTime: prefix(str(row["drawDate"]), 10),
			Primary: p, Secondary: s, DetailURL: "https://bet.hkjc.com/"})
	}
	return out
}

// ParseMarksix6Latest mirrors parse_marksix6_latest.
func ParseMarksix6Latest(game Game, payload any) (Result, error) {
	m := asMap(payload)
	var numbers []string
	for _, v := range asList(m["numbers"]) {
		numbers = append(numbers, strings.TrimSpace(str(v)))
	}
	if len(numbers) != 7 {
		numbers = SplitNumbers(str(m["openCode"]))
	}
	p, s, err := NormalizeMarkSixNumbers(numbers)
	if err != nil {
		return Result{}, err
	}
	issue := str(m["expect"])
	if issue == "" {
		return Result{}, errors.New("接口返回缺少期号")
	}
	return Result{Source: "marksix6", GameCode: game.Code, GameName: game.Name, Issue: issue,
		DrawTime: str(m["openTime"]), Primary: p, Secondary: s, DetailURL: Marksix6APIURL}, nil
}

func splitComma(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ParseMacaujcPayload mirrors parse_macaujc_payload (list or {"data": [...]}).
func ParseMacaujcPayload(game Game, payload any) []Result {
	rows := asList(payload)
	if rows == nil {
		rows = asList(asMap(payload)["data"])
	}
	var out []Result
	for _, item := range rows {
		row := asMap(item)
		if row == nil || str(row["expect"]) == "" {
			continue
		}
		p, s, err := NormalizeMarkSixNumbers(SplitNumbers(str(row["openCode"])))
		if err != nil {
			continue
		}
		var zodiac, wave []string
		if z := splitComma(str(row["zodiac"])); len(z) == 7 {
			zodiac = z
		}
		if w := splitComma(str(row["wave"])); len(w) == 7 {
			wave = w
		}
		out = append(out, Result{Source: "macaujc", GameCode: game.Code, GameName: game.Name,
			Issue: str(row["expect"]), DrawTime: str(row["openTime"]), Primary: p, Secondary: s,
			DetailURL: "https://macaujc.com/", Zodiac: zodiac, Wave: wave})
	}
	return out
}

var (
	marksixLineRe   = regexp.MustCompile(`<div class="history-line">([\s\S]*?)</div>`)
	marksixPeriodRe = regexp.MustCompile(`<span class="period">\s*([^<期]+)期`)
	marksixBallRe   = regexp.MustCompile(`<span class="ball-sm [^"]+">\s*(\d{1,2})\s*</span>`)
)

// ParseMarksix6HistoryHTML mirrors parse_marksix6_history_html.
func ParseMarksix6HistoryHTML(game Game, page string) []Result {
	sectionRe := regexp.MustCompile(`<section class="card" id="` + regexp.QuoteMeta(game.APICode) + `">([\s\S]*?)</section>`)
	section := sectionRe.FindStringSubmatch(page)
	if section == nil {
		return nil
	}
	var out []Result
	for _, block := range marksixLineRe.FindAllStringSubmatch(section[1], -1) {
		issue := marksixPeriodRe.FindStringSubmatch(block[1])
		var numbers []string
		for _, m := range marksixBallRe.FindAllStringSubmatch(block[1], -1) {
			numbers = append(numbers, m[1])
		}
		if issue == nil || len(numbers) != 7 {
			continue
		}
		p, s, err := NormalizeMarkSixNumbers(numbers)
		if err != nil {
			continue
		}
		out = append(out, Result{Source: "marksix6", GameCode: game.Code, GameName: game.Name,
			Issue: strings.TrimSpace(html.UnescapeString(issue[1])), Primary: p, Secondary: s,
			DetailURL: Marksix6HistoryURL})
	}
	return out
}

var huiniaoKeys = []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
	"eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty"}

// ParseHuiniao mirrors the parsing part of _latest_huiniao.
func ParseHuiniao(game Game, payload any) (Result, error) {
	row := asMap(asMap(asMap(payload)["data"])["last"])
	var raw []string
	for _, key := range huiniaoKeys {
		v := row[key]
		s := strings.TrimSpace(str(v))
		if n, ok := v.(json.Number); ok {
			s = n.String() // keep numeric 0 (Python would drop it as falsy)
		}
		if s != "" {
			raw = append(raw, s)
		}
	}
	var primary, secondary []string
	if IsThreeDigit(game.Code) {
		var digits []string
		for _, part := range raw {
			if !isDigits(part) {
				continue
			}
			digits = append(digits, strconv.Itoa(atoi(part)))
			if len(digits) == 3 {
				break
			}
		}
		if len(digits) != 3 {
			return Result{}, errors.New("慧鸟返回三位彩号码不完整")
		}
		d, err := NormalizeThreeDigit(digits)
		if err != nil {
			return Result{}, err
		}
		primary = d
	} else {
		numbers := make([]string, 0, len(raw))
		for _, part := range raw {
			if isDigits(part) && len(part) < 2 {
				part = "0" + part
			}
			numbers = append(numbers, part)
		}
		if str(row["code"]) == "" || len(numbers) == 0 {
			return Result{}, errors.New("接口返回缺少开奖号码")
		}
		primary, secondary = splitAt(game, numbers)
	}
	if str(row["code"]) == "" {
		return Result{}, errors.New("接口返回缺少开奖号码")
	}
	r := Result{Source: "huiniao", GameCode: game.Code, GameName: game.Name, Issue: str(row["code"]),
		DrawTime: firstNonEmpty(row["open_time"], row["day"]), Primary: primary, Secondary: secondary,
		DetailURL: HuiniaoURL, NextDrawTime: str(row["next_open_time"])}
	return normalizeIfStrict(r)
}
