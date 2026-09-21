package systemconfig

import (
	"reflect"
	"testing"
)

func TestParseChatIDs(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "legacy single value", value: " 10000821381 ", want: []string{"10000821381"}},
		{name: "json list", value: `["1001", "1002"]`, want: []string{"1001", "1002"}},
		{name: "remove blanks and duplicates", value: `["1001", "", "1001", " 1002 "]`, want: []string{"1001", "1002"}},
		{name: "empty", value: "", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseChatIDs(tt.value); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseChatIDs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
