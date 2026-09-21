package bianjsft

import (
	"reflect"
	"testing"
	"time"
)

func TestResultExample(t *testing.T) {
	result, err := Result("0x6ffffff9d1078bb5b5c06fb02b39aff897902e23283b1f1071dec717ada46d7c")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10", "08", "03", "01", "06", "02", "04", "05", "07", "09"}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("Result() = %v, want %v", result, want)
	}
}

func TestIssueNumberUsesBeijingTime(t *testing.T) {
	drawTime := time.Date(2026, 9, 5, 16, 18, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	if got, want := IssueNumber(drawTime), "2609050978"; got != want {
		t.Fatalf("IssueNumber() = %q, want %q", got, want)
	}
}

func TestIssueNumberDayBoundaries(t *testing.T) {
	beijing := time.FixedZone("UTC+8", 8*60*60)
	tests := []struct {
		time time.Time
		want string
	}{
		{time.Date(2026, 9, 5, 0, 1, 0, 0, beijing), "2609050001"},
		{time.Date(2026, 9, 5, 13, 20, 0, 0, beijing), "2609050800"},
		{time.Date(2026, 9, 5, 23, 59, 0, 0, beijing), "2609051439"},
		{time.Date(2026, 9, 6, 0, 0, 0, 0, beijing), "2609051440"},
		{time.Date(2026, 9, 6, 0, 1, 0, 0, beijing), "2609060001"},
	}
	for _, test := range tests {
		if got := IssueNumber(test.time); got != test.want {
			t.Errorf("IssueNumber(%s) = %q, want %q", test.time, got, test.want)
		}
	}
}
