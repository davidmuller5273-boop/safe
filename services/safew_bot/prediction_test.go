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
	table := renderPredictionTable(predictionsNewestFirst, hotnumber.Size)
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


func TestBuildPredictionMessageSize6TitleAndPrediction(t *testing.T) {
	numbers := []string{"2", "3", "4", "5", "7", "9"}
	pred := hotnumber.Prediction(numbers)
	if len(pred) != 6 {
		t.Fatalf("Prediction length = %d (%q), want 6", len(pred), pred)
	}
	if got, want := pred, "234579"; got != want {
		t.Fatalf("Prediction = %q, want %q", got, want)
	}
	trimmed := hotnumber.TakeFirst(append(numbers, "1"), hotnumber.Size6)
	if len(trimmed) != 6 {
		t.Fatalf("TakeFirst size6 = %v", trimmed)
	}
	body := buildPredictionMessage("币安极速飞艇", hotnumber.Size6, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body, "币安极速飞艇 6码热号预测") {
		t.Fatalf("title missing 6码热号预测:\n%s", body)
	}
	if strings.Contains(body, "7码热号预测") {
		t.Fatalf("size6 body unexpectedly contains 7码:\n%s", body)
	}
}

func TestBuildPredictionMessageSize7Unchanged(t *testing.T) {
	body := buildPredictionMessage("币安极速飞艇", hotnumber.Size, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body, "币安极速飞艇 7码热号预测") {
		t.Fatalf("title missing 7码热号预测:\n%s", body)
	}
}

func TestRenderPredictionTableWidthMatchesSize(t *testing.T) {
	preds := []domain.HotNumberPrediction{
		{IssueNumber: "2609051017", Prediction: "234579", CodeSize: 6},
	}
	table := renderPredictionTable(preds, hotnumber.Size6)
	if !strings.Contains(table, "234579") {
		t.Fatalf("table missing prediction:\n%s", table)
	}
}
