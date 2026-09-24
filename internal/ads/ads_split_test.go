package ads

import "testing"

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

func TestMergeSides(t *testing.T) {
	g := Settings{PrefixAd: "G前", SuffixAd: "G后"}
	s := mergeSides("群前", "", g, "1")
	if s.PrefixAd != "群前" || s.SuffixAd != "G后" {
		t.Fatalf("%+v", s)
	}
	if got := BuildOutboundText("群前", "正文", "群后"); got != "群前\n\n正文\n\n群后" {
		t.Fatalf("%q", got)
	}
}
