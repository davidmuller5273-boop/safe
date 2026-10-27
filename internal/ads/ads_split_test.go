package ads

import (
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/domain"
)

func TestSplitPrefixSuffix(t *testing.T) {
	cases := []struct{ in, p, s string }{
		{"前 | 后", "前", "后"},
		{"前｜后", "前", "后"},
		{"第一行\n第二行\n---\n后面\n两行", "第一行\n第二行", "后面\n两行"},
		{"只有前 |", "只有前", ""},
	}
	for _, c := range cases {
		p, s, ok := SplitPrefixSuffix(c.in)
		if !ok || p != c.p || s != c.s {
			t.Fatalf("%q => %q %q %v", c.in, p, s, ok)
		}
	}
	if _, _, ok := SplitPrefixSuffix("没有分隔"); ok {
		t.Fatal("expected not ok")
	}
}

func TestGroupSettingsNeverFallsBackToGlobal(t *testing.T) {
	// only a group prefix: the suffix stays empty (no global suffix is merged in)
	s := groupSettings(domain.BotGroupAd{PrefixAd: "群前"}, "1")
	if s.Scope != "group" || s.PrefixAd != "群前" || s.SuffixAd != "" {
		t.Fatalf("%+v", s)
	}
	if got := BuildOutboundText(s.PrefixAd, "正文", s.SuffixAd); got != "群前\n\n正文" {
		t.Fatalf("%q", got)
	}
	// a group without any own ad gets just the body
	n := groupSettings(domain.BotGroupAd{PrefixAd: " ", SuffixAd: ""}, "2")
	if n.Scope != "none" || BuildOutboundText(n.PrefixAd, "正文", n.SuffixAd) != "正文" {
		t.Fatalf("%+v", n)
	}
	if got := BuildOutboundText("群前", "正文", "群后"); got != "群前\n\n正文\n\n群后" {
		t.Fatalf("%q", got)
	}
}
