package safewbot

import (
	"strings"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/botperm"
)

func TestAITronCommandParsing(t *testing.T) {
	cases := []struct {
		text        string
		enabled, ok bool
	}{
		{"/开启AI波场单双一分", true, true},
		{"/关闭AI波场单双一分", false, true},
		{"/开启ai波场单双一分", true, true},
		{"/开启 AI波场单双一分", true, true},
		{"/关闭 AI 波场单双一分", false, true},
		{"/开启6码", false, false},
		{"/开启 别的", false, false},
	}
	for _, c := range cases {
		// mirror handleCommand's tokenisation
		parts := strings.Fields(c.text)
		cmd := strings.ToLower(parts[0])
		args := ""
		if len(parts) > 1 {
			args = strings.TrimSpace(strings.TrimPrefix(c.text, parts[0]))
		}
		enabled, ok := aiTronCommand(cmd, args)
		if ok != c.ok || (ok && enabled != c.enabled) {
			t.Errorf("%q: got enabled=%v ok=%v", c.text, enabled, ok)
		}
	}
	// the lower-cased constants must be what handleCommand produces for the exact commands
	if got := strings.ToLower("/开启AI波场单双一分"); got != aiTronEnableCmd {
		t.Errorf("enable cmd constant %q != %q", aiTronEnableCmd, got)
	}
	if got := strings.ToLower("/关闭AI波场单双一分"); got != aiTronDisableCmd {
		t.Errorf("disable cmd constant %q != %q", aiTronDisableCmd, got)
	}
}

func TestHelpListsAITronOnlyForPrivilegedRoles(t *testing.T) {
	for _, role := range []string{botperm.RoleDeveloper, botperm.RoleAdmin} {
		h := helpForRole(role)
		if !strings.Contains(h, "/开启AI波场单双一分") || !strings.Contains(h, "/关闭AI波场单双一分") {
			t.Errorf("role %q help misses AI波场单双一分", role)
		}
	}
	for _, role := range []string{botperm.RoleGroupAdmin, botperm.RoleNone} {
		if strings.Contains(helpForRole(role), "AI波场单双一分") {
			t.Errorf("role %q must not see AI波场单双一分", role)
		}
	}
}

func tokenise(text string) (cmd, args string) {
	parts := strings.Fields(text)
	cmd = strings.ToLower(parts[0])
	if len(parts) > 1 {
		args = strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	}
	return
}

func TestAutoTronCommandParsing(t *testing.T) {
	cases := []struct {
		text        string
		enabled, ok bool
	}{
		{"/开启自动最优波场单双一分", true, true},
		{"/关闭自动最优波场单双一分", false, true},
		{"/开启 自动最优波场单双一分", true, true},
		{"/关闭 自动最优 波场单双一分", false, true},
		{"/开启AI波场单双一分", false, false},
		{"/开启6码", false, false},
		{"/开启 自动最优", false, false},
	}
	for _, c := range cases {
		cmd, args := tokenise(c.text)
		enabled, ok := autoTronCommand(cmd, args)
		if ok != c.ok || (ok && enabled != c.enabled) {
			t.Errorf("%q: got enabled=%v ok=%v", c.text, enabled, ok)
		}
	}
	if got := strings.ToLower("/开启自动最优波场单双一分"); got != autoTronEnableCmd {
		t.Errorf("enable cmd constant %q != %q", autoTronEnableCmd, got)
	}
	if got := strings.ToLower("/关闭自动最优波场单双一分"); got != autoTronDisableCmd {
		t.Errorf("disable cmd constant %q != %q", autoTronDisableCmd, got)
	}
}

// The two broadcasts have separate switches: an AI command never toggles auto and vice versa.
func TestAITronAndAutoTronCommandsAreDisjoint(t *testing.T) {
	for _, text := range []string{
		"/开启AI波场单双一分", "/关闭AI波场单双一分", "/开启 AI波场单双一分",
		"/开启自动最优波场单双一分", "/关闭自动最优波场单双一分", "/关闭 自动最优波场单双一分",
	} {
		cmd, args := tokenise(text)
		_, ai := aiTronCommand(cmd, args)
		_, auto := autoTronCommand(cmd, args)
		if ai == auto {
			t.Errorf("%q: ai=%v auto=%v (exactly one must match)", text, ai, auto)
		}
	}
}

func TestHelpListsAutoTronOnlyForPrivilegedRoles(t *testing.T) {
	for _, role := range []string{botperm.RoleDeveloper, botperm.RoleAdmin} {
		h := helpForRole(role)
		if !strings.Contains(h, "/开启自动最优波场单双一分") || !strings.Contains(h, "/关闭自动最优波场单双一分") {
			t.Errorf("role %q help misses 自动最优波场单双一分", role)
		}
	}
	for _, role := range []string{botperm.RoleGroupAdmin, botperm.RoleNone} {
		if strings.Contains(helpForRole(role), "自动最优波场单双一分") {
			t.Errorf("role %q must not see 自动最优波场单双一分", role)
		}
	}
}

func TestTronEnvSwitchesAreIndependent(t *testing.T) {
	t.Setenv("AI_TRON_ENABLED", "0")
	t.Setenv("AUTO_TRON_ENABLED", "")
	if !tronEnvOff("AI_TRON_ENABLED") || tronEnvOff("AUTO_TRON_ENABLED") {
		t.Fatal("AI_TRON_ENABLED=0 must not switch auto off")
	}
	t.Setenv("AUTO_TRON_ENABLED", "off")
	if !tronEnvOff("AUTO_TRON_ENABLED") {
		t.Fatal("AUTO_TRON_ENABLED=off must switch auto off")
	}
}
