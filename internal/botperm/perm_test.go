package botperm

import (
	"strings"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/domain"
)

func TestEffectiveCodeModes(t *testing.T) {
	cases := []struct {
		name  string
		row   domain.BotGroupSettings
		want6 bool
		want7 bool
	}{
		{"legacy push only", domain.BotGroupSettings{PushEnabled: true}, false, false},
		{"6 only", domain.BotGroupSettings{PushEnabled: true, Enable6Code: true}, true, false},
		{"7 only", domain.BotGroupSettings{PushEnabled: true, Enable7Code: true}, false, true},
		{"both", domain.BotGroupSettings{PushEnabled: true, Enable6Code: true, Enable7Code: true}, true, true},
		{"push off with flags", domain.BotGroupSettings{PushEnabled: false, Enable6Code: true, Enable7Code: true}, true, true},
		{"all off", domain.BotGroupSettings{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Effective6Code(tc.row); got != tc.want6 {
				t.Fatalf("Effective6Code = %v, want %v", got, tc.want6)
			}
			if got := Effective7Code(tc.row); got != tc.want7 {
				t.Fatalf("Effective7Code = %v, want %v", got, tc.want7)
			}
		})
	}
}

func TestFormatGroupCodeState(t *testing.T) {
	row := domain.BotGroupSettings{PushEnabled: true, Enable6Code: true}
	got := FormatGroupCodeState(row)
	for _, part := range []string{"push=true", "enable_6=true", "enable_7=false", "6码=true", "7码=false"} {
		if !strings.Contains(got, part) {
			t.Fatalf("FormatGroupCodeState missing %q in %q", part, got)
		}
	}
}

func TestCodeModeUpsertSQL(t *testing.T) {
	cases := []struct {
		size    int
		enabled bool
		want    []string
	}{
		{6, true, []string{"VALUES (?, 1, 1, 0,", "enable_6_code=VALUES(enable_6_code)", "enable_7_code=VALUES(enable_7_code)", "push_enabled=VALUES(push_enabled)"}},
		{7, true, []string{"VALUES (?, 1, 0, 1,", "enable_6_code=VALUES(enable_6_code)", "enable_7_code=VALUES(enable_7_code)"}},
		{6, false, []string{"VALUES (?, 0, 0, 0,", "enable_6_code=0"}},
		{7, false, []string{"VALUES (?, 0, 0, 0,", "enable_7_code=0"}},
	}
	for _, tc := range cases {
		sql, err := codeModeUpsertSQL(tc.size, tc.enabled)
		if err != nil {
			t.Fatalf("size=%d enabled=%v: %v", tc.size, tc.enabled, err)
		}
		if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE") {
			t.Fatalf("missing ON DUPLICATE KEY UPDATE in %q", sql)
		}
		for _, part := range tc.want {
			if !strings.Contains(sql, part) {
				t.Fatalf("size=%d enabled=%v missing %q in %q", tc.size, tc.enabled, part, sql)
			}
		}
	}
	if _, err := codeModeUpsertSQL(5, true); err == nil {
		t.Fatal("expected error for invalid size")
	}
}

func TestCodeModeVerifyOK(t *testing.T) {
	cases := []struct {
		size, e6, e7, push int
		enabled, want      bool
	}{
		{6, 1, 0, 1, true, true},
		{6, 0, 0, 1, true, false},
		{6, 1, 1, 1, true, false},
		{6, 1, 0, 0, true, false},
		{7, 0, 1, 1, true, true},
		{7, 1, 1, 1, true, false},
		{6, 0, 1, 1, false, true},
		{6, 1, 0, 1, false, false},
		{7, 1, 0, 1, false, true},
		{7, 0, 1, 1, false, false},
	}
	for _, tc := range cases {
		got := codeModeVerifyOK(tc.size, tc.enabled, tc.e6, tc.e7, tc.push)
		if got != tc.want {
			t.Fatalf("verify(size=%d en=%v e6=%d e7=%d push=%d)=%v want %v",
				tc.size, tc.enabled, tc.e6, tc.e7, tc.push, got, tc.want)
		}
	}
}
