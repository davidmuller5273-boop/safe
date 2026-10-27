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
