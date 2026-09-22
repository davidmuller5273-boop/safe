package safewbot

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/lottery/hotnumber"
)

func TestHotNumbersForIssue1177SkipDuplicatesUntilSevenUnique(t *testing.T) {
	firstPlaces := []struct {
		issue string
		first string
	}{
		{"2609061176", "09"},
		{"2609061175", "05"},
		{"2609061174", "07"},
		{"2609061173", "04"},
		{"2609061172", "01"},
		{"2609061171", "01"},
		{"2609061170", "02"},
		{"2609061169", "03"},
		{"2609061168", "03"},
		{"2609061167", "06"},
		{"2609061166", "08"},
	}
	records := make([]domain.DrawRecord, 0, len(firstPlaces))
	for _, item := range firstPlaces {
		result, _ := json.Marshal([]string{item.first})
		records = append(records, domain.DrawRecord{IssueNumber: item.issue, DrawResult: string(result)})
	}
	numbers, err := hotNumbersFromRecords(records, "2609061177", hotnumber.Size)
	if err != nil {
		t.Fatal(err)
	}
	wantNumbers := []string{"9", "5", "7", "4", "1", "2", "3"}
	if !reflect.DeepEqual(numbers, wantNumbers) {
		t.Fatalf("hot numbers = %v, want %v", numbers, wantNumbers)
	}
	if got, want := hotnumber.Prediction(numbers), "1234579"; got != want {
		t.Fatalf("prediction = %q, want %q", got, want)
	}
}

func TestRenderPredictionTablePlacesLatestIssueLast(t *testing.T) {
	correct := true
	predictionsNewestFirst := []domain.HotNumberPrediction{
		{IssueNumber: "2609051017", Prediction: "0145678", Correct: nil},
		{IssueNumber: "2609051016", Prediction: "0145678", Correct: &correct},
		{IssueNumber: "2609051015", Prediction: "0145678", Correct: &correct},
	}
	table := renderPredictionTable(predictionsNewestFirst)
	older := strings.Index(table, "2609051015")
	latest := strings.Index(table, "2609051017")
	if older == -1 || latest == -1 || older >= latest {
		t.Fatalf("latest issue is not last:\n%s", table)
	}
	if !strings.Contains(table[latest:], "等待开奖") {
		t.Fatalf("pending latest issue does not show waiting status:\n%s", table)
	}
}

func TestCalculateStats(t *testing.T) {
	correct, wrong := true, false
	predictionsNewestFirst := []domain.HotNumberPrediction{
		{Correct: &correct},
		{Correct: &correct},
		{Correct: &wrong},
		{Correct: &wrong},
		{Correct: &wrong},
		{Correct: &correct},
	}
	stats := calculateStats(predictionsNewestFirst, 180)
	if stats.WinRate != 50 || stats.MaxWinStreak != 2 || stats.MaxLossStreak != 3 {
		t.Fatalf("calculateStats() = %+v", stats)
	}
}

func TestCalculateStatsIgnoresPendingPrediction(t *testing.T) {
	correct, wrong := true, false
	predictionsNewestFirst := []domain.HotNumberPrediction{
		{Correct: nil},
		{Correct: &correct},
		{Correct: &wrong},
	}
	stats := calculateStats(predictionsNewestFirst, 2)
	if stats.WinRate != 50 || stats.MaxWinStreak != 1 || stats.MaxLossStreak != 1 {
		t.Fatalf("calculateStats() included pending prediction: %+v", stats)
	}
}

func TestHotNumbersSize6(t *testing.T) {
	firstPlaces := []struct {
		issue string
		first string
	}{
		{"2609061176", "09"},
		{"2609061175", "05"},
		{"2609061174", "07"},
		{"2609061173", "04"},
		{"2609061172", "01"},
		{"2609061171", "01"},
		{"2609061170", "02"},
		{"2609061169", "03"},
	}
	records := make([]domain.DrawRecord, 0, len(firstPlaces))
	for _, item := range firstPlaces {
		result, _ := json.Marshal([]string{item.first})
		records = append(records, domain.DrawRecord{IssueNumber: item.issue, DrawResult: string(result)})
	}
	numbers, err := hotNumbersFromRecords(records, "2609061177", hotnumber.Size6)
	if err != nil {
		t.Fatal(err)
	}
	wantNumbers := []string{"9", "5", "7", "4", "1", "2"}
	if !reflect.DeepEqual(numbers, wantNumbers) {
		t.Fatalf("hot numbers size6 = %v, want %v", numbers, wantNumbers)
	}
	if got, want := hotnumber.Prediction(numbers), "124579"; got != want {
		t.Fatalf("prediction = %q, want %q", got, want)
	}
}
