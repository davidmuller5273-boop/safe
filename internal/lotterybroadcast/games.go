// Package lotterybroadcast ports the Python 「彩票开奖播报」 package
// (福彩 / 体彩 / 港澳六合彩 → subscribed groups) to Go: game catalogue,
// multi-source fetching and parsing, result formatting, draw schedules,
// MySQL storage and the refresh / new-issue detection engine.
package lotterybroadcast

import (
	"regexp"
	"strconv"
	"strings"
)

// Game describes one lottery (mirrors LotteryGame in lottery.py).
type Game struct {
	Code           string
	Name           string
	Source         string
	APICode        string
	SecondaryCount int
}

// Result is one draw (mirrors LotteryResult in lottery.py).
type Result struct {
	Source       string
	GameCode     string
	GameName     string
	Issue        string
	DrawTime     string
	Primary      []string
	Secondary    []string
	DetailURL    string
	NextDrawTime string
	Zodiac       []string
	Wave         []string
}

// GameOrder keeps the Python dict order (used for menus and listings).
var GameOrder = []string{"ssq", "fc3d", "qlc", "kl8", "dlt", "pl3", "pl5", "qxc", "hklhc", "macau_lhc", "new_macau_lhc"}

// Games is LOTTERY_GAMES.
var Games = map[string]Game{
	"ssq":           {"ssq", "双色球", "cwl", "ssq", 1},
	"fc3d":          {"fc3d", "福彩3D", "cwl", "3d", 0},
	"qlc":           {"qlc", "七乐彩", "cwl", "qlc", 1},
	"kl8":           {"kl8", "快乐8", "cwl", "kl8", 0},
	"dlt":           {"dlt", "超级大乐透", "sport", "85", 2},
	"pl3":           {"pl3", "排列3", "sport", "35", 0},
	"pl5":           {"pl5", "排列5", "sport", "37", 0},
	"qxc":           {"qxc", "7星彩", "sport", "04", 0},
	"hklhc":         {"hklhc", "香港六合彩", "hkjc", "hk", 1},
	"macau_lhc":     {"macau_lhc", "澳门六合彩", "marksix6", "macau", 1},
	"new_macau_lhc": {"new_macau_lhc", "新澳六合彩", "macaujc", "newMacau", 1},
}

// Aliases is LOTTERY_ALIASES (keys are lower-cased).
var Aliases = map[string]string{
	"双色球": "ssq", "ssq": "ssq",
	"福彩3d": "fc3d", "3d": "fc3d", "fc3d": "fc3d",
	"七乐彩": "qlc", "qlc": "qlc",
	"快乐8": "kl8", "快乐八": "kl8", "kl8": "kl8",
	"大乐透": "dlt", "超级大乐透": "dlt", "dlt": "dlt",
	"排列3": "pl3", "排列三": "pl3", "pl3": "pl3",
	"排列5": "pl5", "排列五": "pl5", "pl5": "pl5",
	"7星彩": "qxc", "七星彩": "qxc", "qxc": "qxc",
	"香港六合彩": "hklhc", "香港彩": "hklhc", "港彩": "hklhc", "hklhc": "hklhc",
	"澳门六合彩": "macau_lhc", "澳门彩": "macau_lhc", "macau_lhc": "macau_lhc",
	"新澳六合彩": "new_macau_lhc", "新澳门六合彩": "new_macau_lhc",
	"新澳门彩": "new_macau_lhc", "new_macau_lhc": "new_macau_lhc",
	"福彩": "cwl", "体彩": "sport", "六合彩": "marksix",
	"全部": "all", "all": "all",
}

var (
	markSixCodes    = map[string]bool{"hklhc": true, "macau_lhc": true, "new_macau_lhc": true}
	threeDigitCodes = map[string]bool{"fc3d": true, "pl3": true}
)

// IsMarkSix reports whether the game is a 六合彩 variant.
func IsMarkSix(code string) bool { return markSixCodes[code] }

// IsThreeDigit reports whether the game is 福彩3D / 排列3.
func IsThreeDigit(code string) bool { return threeDigitCodes[code] }

// ResolveCode maps user input to a game code or selector ("" when unknown).
func ResolveCode(value string) string {
	return Aliases[strings.ToLower(strings.TrimSpace(value))]
}

// ResolveHistoryKeyword recognises 「<彩种>历史」 keywords.
func ResolveHistoryKeyword(value string) string {
	text := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), " ", "")
	if !strings.HasSuffix(text, "历史") {
		return ""
	}
	code := ResolveCode(strings.TrimSuffix(text, "历史"))
	if _, ok := Games[code]; ok {
		return code
	}
	return ""
}

// CodesForSelector expands all / cwl / sport / marksix / single code.
func CodesForSelector(selector string) []string {
	switch selector {
	case "all":
		return append([]string(nil), GameOrder...)
	case "cwl", "sport":
		var out []string
		for _, code := range GameOrder {
			if Games[code].Source == selector {
				out = append(out, code)
			}
		}
		return out
	case "marksix":
		return []string{"hklhc", "macau_lhc", "new_macau_lhc"}
	}
	if _, ok := Games[selector]; ok {
		return []string{selector}
	}
	return nil
}

// SelectorLabel is lottery_selector_label.
func SelectorLabel(selector string) string {
	switch selector {
	case "all":
		return "全部彩种"
	case "cwl":
		return "全部福彩"
	case "sport":
		return "全部体彩"
	case "marksix":
		return "全部六合彩"
	}
	if g, ok := Games[selector]; ok {
		return g.Name
	}
	return selector
}

// IsSelector reports whether s is a valid subscription selector.
func IsSelector(s string) bool {
	switch s {
	case "all", "cwl", "sport", "marksix":
		return true
	}
	_, ok := Games[s]
	return ok
}

var digitsRe = regexp.MustCompile(`\d+`)

// IssueKey mirrors LotteryService._issue_key: all digit groups as ints.
func IssueKey(issue string) []int64 {
	parts := digitsRe.FindAllString(issue, -1)
	if len(parts) == 0 {
		return []int64{0}
	}
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// CompareIssues compares two issues by IssueKey (Python tuple ordering).
func CompareIssues(a, b string) int {
	ka, kb := IssueKey(a), IssueKey(b)
	for i := 0; i < len(ka) && i < len(kb); i++ {
		if ka[i] != kb[i] {
			if ka[i] < kb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(ka) < len(kb):
		return -1
	case len(ka) > len(kb):
		return 1
	}
	return 0
}

// IsNewerIssue reports whether issue is strictly newer than known ("" known = always newer).
func IsNewerIssue(issue, known string) bool {
	if strings.TrimSpace(known) == "" {
		return true
	}
	return CompareIssues(issue, known) > 0
}
