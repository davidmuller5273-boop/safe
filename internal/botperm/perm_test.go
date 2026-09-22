package botperm

import (
	"strings"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/domain"
)

func TestEffectiveCodeModes(t *testing.T) {
	cases := []struct {
		name       string
		row        domain.BotGroupSettings
		want6      bool
		want7      bool
	}{
		{"legacy push only", domain.BotGroupSettings{PushEnabled: true}, false, true},
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
