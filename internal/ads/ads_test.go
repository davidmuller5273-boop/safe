package ads

import "testing"

func TestBuildOutboundText(t *testing.T) {
	cases := []struct {
		prefix, body, suffix, want string
	}{
		{"", "hello", "", "hello"},
		{"PRE", "hello", "", "PRE\n\nhello"},
		{"", "hello", "SUF", "hello\n\nSUF"},
		{"PRE", "hello", "SUF", "PRE\n\nhello\n\nSUF"},
		{"  ", "body", "  ", "body"},
		{"PRE", "", "SUF", "PRE\n\nSUF"},
	}
	for _, c := range cases {
		got := BuildOutboundText(c.prefix, c.body, c.suffix)
		if got != c.want {
			t.Fatalf("BuildOutboundText(%q,%q,%q)=%q want %q", c.prefix, c.body, c.suffix, got, c.want)
		}
	}
}
