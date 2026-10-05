package lotterybroadcast

import (
	"strings"
	"time"
)

// Beijing is UTC+8 (fixed zone, no tzdata dependency).
var Beijing = time.FixedZone("CST", 8*3600)

type drawSchedule struct {
	weekdays     map[int]bool // Python weekday: 0=Monday … 6=Sunday
	hour, minute int
}

func days(ds ...int) map[int]bool {
	m := map[int]bool{}
	for _, d := range ds {
		m[d] = true
	}
	return m
}

var everyDay = days(0, 1, 2, 3, 4, 5, 6)

// DrawSchedules mirrors LOTTERY_DRAW_SCHEDULES (北京时间).
var DrawSchedules = map[string]drawSchedule{
	"ssq":           {days(1, 3, 6), 21, 15},
	"fc3d":          {everyDay, 21, 15},
	"qlc":           {days(0, 2, 4), 21, 15},
	"kl8":           {everyDay, 21, 30},
	"dlt":           {days(0, 2, 5), 21, 25},
	"pl3":           {everyDay, 21, 25},
	"pl5":           {everyDay, 21, 25},
	"qxc":           {days(1, 4, 6), 21, 25},
	"hklhc":         {days(1, 3, 5), 21, 30},
	"macau_lhc":     {everyDay, 21, 32},
	"new_macau_lhc": {everyDay, 21, 32},
}

// FastWindowBefore / FastWindowAfter bound the fast-poll window around a draw.
const (
	FastWindowBefore = 2 * time.Minute
	FastWindowAfter  = 45 * time.Minute
)

func pyWeekday(t time.Time) int { return (int(t.Weekday()) + 6) % 7 }

func atClock(t time.Time, hour, minute int) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), hour, minute, 0, 0, t.Location())
}

// ParseDrawTime mirrors parse_lottery_draw_time (Beijing time).
func ParseDrawTime(value string) (time.Time, bool) {
	cleaned := strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(value), "T", " "), "Z")
	if cleaned == "" {
		return time.Time{}, false
	}
	if len(cleaned) > 19 {
		cleaned = cleaned[:19]
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, cleaned, Beijing); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// NextDraw mirrors next_lottery_draw.
func NextDraw(code string, after time.Time) time.Time {
	sc := DrawSchedules[code]
	after = after.In(Beijing)
	for offset := 0; offset < 9; offset++ {
		c := atClock(after.AddDate(0, 0, offset), sc.hour, sc.minute)
		if sc.weekdays[pyWeekday(c)] && c.After(after) {
			return c
		}
	}
	return after.AddDate(0, 0, 1)
}

// PreviousDraw mirrors previous_lottery_draw.
func PreviousDraw(code string, at time.Time) (time.Time, bool) {
	sc := DrawSchedules[code]
	at = at.In(Beijing)
	for offset := 0; offset < 8; offset++ {
		c := atClock(at.AddDate(0, 0, -offset), sc.hour, sc.minute)
		if sc.weekdays[pyWeekday(c)] && !c.After(at) {
			return c, true
		}
	}
	return time.Time{}, false
}

// PollWindow mirrors lottery_poll_window: 2 minutes before a draw through 45 minutes after.
func PollWindow(code string, at time.Time) (time.Time, bool) {
	if _, ok := DrawSchedules[code]; !ok {
		return time.Time{}, false
	}
	at = at.In(Beijing)
	upcoming := NextDraw(code, at)
	if upcoming.After(at) && !upcoming.After(at.Add(FastWindowBefore)) {
		return upcoming, true
	}
	if expected, ok := PreviousDraw(code, at); ok && !at.Before(expected) && !at.After(expected.Add(FastWindowAfter)) {
		return expected, true
	}
	return time.Time{}, false
}
