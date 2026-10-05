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

func TestEffectiveRunnerUpCodeModes(t *testing.T) {
	cases := []struct {
		name  string
		row   domain.BotGroupSettings
		want6 bool
		want7 bool
	}{
		{"off", domain.BotGroupSettings{PushEnabled: true, Enable6Code: true}, false, false},
		{"r6 only", domain.BotGroupSettings{EnableRunnerUp6Code: true}, true, false},
		{"r7 only", domain.BotGroupSettings{EnableRunnerUp7Code: true}, false, true},
		{"both", domain.BotGroupSettings{EnableRunnerUp6Code: true, EnableRunnerUp7Code: true}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveRunnerUp6Code(tc.row); got != tc.want6 {
				t.Fatalf("EffectiveRunnerUp6Code = %v, want %v", got, tc.want6)
			}
			if got := EffectiveRunnerUp7Code(tc.row); got != tc.want7 {
				t.Fatalf("EffectiveRunnerUp7Code = %v, want %v", got, tc.want7)
			}
		})
	}
}

func TestHasAnyCodeMode(t *testing.T) {
	if HasAnyCodeMode(domain.BotGroupSettings{}) {
		t.Fatal("empty should be false")
	}
	if !HasAnyCodeMode(domain.BotGroupSettings{EnableRunnerUp6Code: true}) {
		t.Fatal("runner-up only should count")
	}
	if !HasAnyCodeMode(domain.BotGroupSettings{Enable6Code: true}) {
		t.Fatal("champion only should count")
	}
}

func TestFormatGroupCodeState(t *testing.T) {
	row := domain.BotGroupSettings{PushEnabled: true, Enable6Code: true, EnableRunnerUp7Code: true}
	got := FormatGroupCodeState(row)
	for _, part := range []string{
		"push=true", "enable_6=true", "enable_7=false",
		"enable_亚军6=false", "enable_亚军7=true",
		"冠军6码=true", "冠军7码=false", "亚军6码=false", "亚军7码=true",
	} {
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
		unwant  []string
	}{
		{6, true, []string{"VALUES (?, 1, 1, 0,", "enable_6_code=VALUES(enable_6_code)", "enable_7_code=VALUES(enable_7_code)", "push_enabled=VALUES(push_enabled)"}, []string{"enable_runner_up_6_code=VALUES", "enable_runner_up_7_code=VALUES"}},
		{7, true, []string{"VALUES (?, 1, 0, 1,", "enable_6_code=VALUES(enable_6_code)", "enable_7_code=VALUES(enable_7_code)"}, []string{"enable_runner_up_6_code=1"}},
		{6, false, []string{"VALUES (?, 0, 0, 0,", "enable_6_code=0"}, []string{"enable_runner_up_6_code=0,"}},
		{7, false, []string{"VALUES (?, 0, 0, 0,", "enable_7_code=0"}, nil},
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
		for _, part := range tc.unwant {
			// On UPDATE path champion SQL must not overwrite runner-up flags.
			updatePart := sql[strings.Index(sql, "ON DUPLICATE KEY UPDATE"):]
			if strings.Contains(updatePart, part) {
				t.Fatalf("size=%d enabled=%v champion UPDATE must not touch runner-up via %q in %q", tc.size, tc.enabled, part, updatePart)
			}
		}
	}
	if _, err := codeModeUpsertSQL(5, true); err == nil {
		t.Fatal("expected error for invalid size")
	}
}

func TestRunnerUpCodeModeUpsertSQL(t *testing.T) {
	cases := []struct {
		size    int
		enabled bool
		want    []string
	}{
		{6, true, []string{"enable_runner_up_6_code=1", "enable_runner_up_7_code=0", "push_enabled=1"}},
		{7, true, []string{"enable_runner_up_6_code=0", "enable_runner_up_7_code=1", "push_enabled=1"}},
		{6, false, []string{"enable_runner_up_6_code=0"}},
		{7, false, []string{"enable_runner_up_7_code=0"}},
	}
	for _, tc := range cases {
		sql, err := runnerUpCodeModeUpsertSQL(tc.size, tc.enabled)
		if err != nil {
			t.Fatalf("size=%d enabled=%v: %v", tc.size, tc.enabled, err)
		}
		if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE") {
			t.Fatalf("missing ON DUPLICATE KEY UPDATE")
		}
		updatePart := sql[strings.Index(sql, "ON DUPLICATE KEY UPDATE"):]
		for _, part := range tc.want {
			if !strings.Contains(updatePart, part) {
				t.Fatalf("size=%d enabled=%v missing %q in UPDATE %q", tc.size, tc.enabled, part, updatePart)
			}
		}
		// Must not flip champion flags on UPDATE.
		for _, bad := range []string{"enable_6_code=", "enable_7_code="} {
			if strings.Contains(updatePart, bad) {
				t.Fatalf("runner-up UPDATE must not touch champion flag %q: %q", bad, updatePart)
			}
		}
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

func TestRunnerUpCodeModeVerifyOK(t *testing.T) {
	cases := []struct {
		size, r6, r7, push int
		enabled, want      bool
	}{
		{6, 1, 0, 1, true, true},
		{6, 1, 1, 1, true, false},
		{7, 0, 1, 1, true, true},
		{7, 1, 0, 1, true, false},
		{6, 0, 1, 0, false, true},
		{6, 1, 0, 1, false, false},
	}
	for _, tc := range cases {
		got := runnerUpCodeModeVerifyOK(tc.size, tc.enabled, tc.r6, tc.r7, tc.push)
		if got != tc.want {
			t.Fatalf("runner verify(size=%d en=%v r6=%d r7=%d push=%d)=%v want %v",
				tc.size, tc.enabled, tc.r6, tc.r7, tc.push, got, tc.want)
		}
	}
}
