package safewbot

import (
	"encoding/json"
	"fmt"
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
	numbers, err := hotNumbersFromRecords(records, "2609061177", hotnumber.Size, hotnumber.PositionChampion)
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

func TestHotNumbersFromRecordsUsesRunnerUpPosition(t *testing.T) {
	// Each draw: [冠军, 亚军]. Hot list for 亚军 should follow 2nd place.
	rows := []struct {
		issue string
		draw  []string
	}{
		{"2609061176", []string{"01", "09"}},
		{"2609061175", []string{"02", "05"}},
		{"2609061174", []string{"03", "07"}},
		{"2609061173", []string{"04", "04"}}, // duplicate 亚军 4 ignored later
		{"2609061172", []string{"05", "01"}},
		{"2609061171", []string{"06", "02"}},
		{"2609061170", []string{"07", "03"}},
		{"2609061169", []string{"08", "06"}},
	}
	records := make([]domain.DrawRecord, 0, len(rows))
	for _, item := range rows {
		result, _ := json.Marshal(item.draw)
		records = append(records, domain.DrawRecord{IssueNumber: item.issue, DrawResult: string(result)})
	}
	numbers, err := hotNumbersFromRecords(records, "2609061177", hotnumber.Size6, hotnumber.PositionRunnerUp)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"9", "5", "7", "4", "1", "2"}
	if !reflect.DeepEqual(numbers, want) {
		t.Fatalf("runner-up hot numbers = %v, want %v", numbers, want)
	}
	// Hit judging must use position 2 of the current draw, not position 1.
	actual, err := hotnumber.FromDrawResultAt([]string{"08", "05"}, hotnumber.PositionRunnerUp)
	if err != nil {
		t.Fatal(err)
	}
	if actual != "5" {
		t.Fatalf("actual runner-up = %q", actual)
	}
	if !hotnumber.Contains(numbers, actual) {
		t.Fatal("expected runner-up hit against position-2 number")
	}
	champ, err := hotnumber.FromDrawResultAt([]string{"08", "05"}, hotnumber.PositionChampion)
	if err != nil {
		t.Fatal(err)
	}
	if hotnumber.Contains(numbers, champ) {
		t.Fatal("champion number must not be used for runner-up hit check in this fixture")
	}
}

func TestRenderPredictionTablePlacesLatestIssueLast(t *testing.T) {
	correct := true
	predictionsNewestFirst := []domain.HotNumberPrediction{
		{IssueNumber: "2609051017", Prediction: "0145678", Correct: nil},
		{IssueNumber: "2609051016", Prediction: "0145678", Correct: &correct},
		{IssueNumber: "2609051015", Prediction: "0145678", Correct: &correct},
	}
	table := renderPredictionTable(predictionsNewestFirst, hotnumber.Size, hotnumber.PositionChampion)
	older := strings.Index(table, "2609051015")
	latest := strings.Index(table, "2609051017")
	if older == -1 || latest == -1 || older >= latest {
		t.Fatalf("latest issue is not last:\n%s", table)
	}
	if !strings.Contains(table[latest:], "等待开奖") {
		t.Fatalf("pending latest issue does not show waiting status:\n%s", table)
	}
	if !strings.Contains(table, "预测热号(冠军)") {
		t.Fatalf("champion table header missing:\n%s", table)
	}
}

func TestRenderPredictionTableRunnerUpTitle(t *testing.T) {
	preds := []domain.HotNumberPrediction{
		{IssueNumber: "2609051017", Prediction: "234579", CodeSize: 6, Position: hotnumber.PositionRunnerUp},
	}
	table := renderPredictionTable(preds, hotnumber.Size6, hotnumber.PositionRunnerUp)
	if !strings.Contains(table, "预测热号(亚军)") {
		t.Fatalf("runner-up table header missing:\n%s", table)
	}
	if strings.Contains(table, "预测热号(冠军)") {
		t.Fatalf("runner-up table unexpectedly has champion header:\n%s", table)
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
	numbers, err := hotNumbersFromRecords(records, "2609061177", hotnumber.Size6, hotnumber.PositionChampion)
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
	body := buildPredictionMessage("币安极速飞艇", hotnumber.Size6, hotnumber.PositionChampion, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body, "币安极速飞艇 6码热号预测") {
		t.Fatalf("title missing 6码热号预测:\n%s", body)
	}
	if strings.Contains(body, "7码热号预测") {
		t.Fatalf("size6 body unexpectedly contains 7码:\n%s", body)
	}
	if strings.Contains(body, "亚军") {
		t.Fatalf("champion body must not contain 亚军:\n%s", body)
	}
}

func TestBuildPredictionMessageSize7Unchanged(t *testing.T) {
	body := buildPredictionMessage("币安极速飞艇", hotnumber.Size, hotnumber.PositionChampion, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body, "币安极速飞艇 7码热号预测") {
		t.Fatalf("title missing 7码热号预测:\n%s", body)
	}
	if strings.Contains(body, "亚军") {
		t.Fatalf("champion body must not contain 亚军:\n%s", body)
	}
}

func TestBuildPredictionMessageRunnerUpTitle(t *testing.T) {
	body6 := buildPredictionMessage("币安极速飞艇", hotnumber.Size6, hotnumber.PositionRunnerUp, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body6, "币安极速飞艇 亚军6码热号预测") {
		t.Fatalf("runner-up 6 title missing:\n%s", body6)
	}
	body7 := buildPredictionMessage("币安极速飞艇", hotnumber.Size, hotnumber.PositionRunnerUp, "dummy", predictionStats{}, predictionStats{})
	if !strings.Contains(body7, "币安极速飞艇 亚军7码热号预测") {
		t.Fatalf("runner-up 7 title missing:\n%s", body7)
	}
	// Streak lines must keep the exact champion format.
	for _, body := range []string{body6, body7} {
		if !strings.Contains(body, "近三小时连错❌：") || !strings.Contains(body, "近三小时最大连中✅：") {
			t.Fatalf("streak lines missing:\n%s", body)
		}
	}
}

func TestRenderPredictionTableWidthMatchesSize(t *testing.T) {
	preds := []domain.HotNumberPrediction{
		{IssueNumber: "2609051017", Prediction: "234579", CodeSize: 6},
	}
	table := renderPredictionTable(preds, hotnumber.Size6, hotnumber.PositionChampion)
	if !strings.Contains(table, "234579") {
		t.Fatalf("table missing prediction:\n%s", table)
	}
}

func TestSendKind(t *testing.T) {
	if got := sendKind(hotnumber.PositionChampion, 6); got != "6" {
		t.Fatalf("champion kind = %q", got)
	}
	if got := sendKind(hotnumber.PositionRunnerUp, 7); got != "r7" {
		t.Fatalf("runner-up kind = %q", got)
	}
}

func TestGroupPushJobsIndependent(t *testing.T) {
	// Only runner-up on → still produces jobs (push gate must not skip).
	jobs := groupPushJobs(domain.BotGroupSettings{PushEnabled: true, EnableRunnerUp6Code: true})
	if len(jobs) != 1 || jobs[0].position != hotnumber.PositionRunnerUp || jobs[0].size != 6 {
		t.Fatalf("runner-up only jobs = %+v", jobs)
	}
	// Champion + runner-up both on → both present; exclusive within each series is caller's job.
	jobs = groupPushJobs(domain.BotGroupSettings{
		Enable6Code: true, EnableRunnerUp7Code: true,
	})
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %+v", jobs)
	}
	// All off → empty (sendToChats skips).
	if jobs = groupPushJobs(domain.BotGroupSettings{PushEnabled: true}); len(jobs) != 0 {
		t.Fatalf("expected empty jobs, got %+v", jobs)
	}
}

// makeDraws builds newest-first draw records whose champion number is firsts[i]
// (firsts[0] is the newest issue).
func makeDraws(firsts []string) []domain.DrawRecord {
	records := make([]domain.DrawRecord, 0, len(firsts))
	for i, first := range firsts {
		result, _ := json.Marshal([]string{first, "02"})
		records = append(records, domain.DrawRecord{
			IssueNumber: fmt.Sprintf("%010d", 2609060000+len(firsts)-i),
			DrawResult:  string(result),
		})
	}
	return records
}

func TestWindowOutcomesFromRawDraws(t *testing.T) {
	// Oldest -> newest champions: 01 02 03 04 05 06 07 08 09 01.
	// Hot numbers (size 6) of an issue = the 6 most recent distinct champions
	// before it. Issues 01/09/08/07 each miss; older ones lack 6 earlier numbers.
	newestFirst := []string{"01", "09", "08", "07", "06", "05", "04", "03", "02", "01"}
	got := windowOutcomes(makeDraws(newestFirst), hotnumber.Size6, hotnumber.PositionChampion, 180)
	want := []bool{false, false, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windowOutcomes = %v, want %v", got, want)
	}
}

func TestWindowOutcomesHitAndStreaks(t *testing.T) {
	// Oldest -> newest champions: 1 2 3 4 5 6 6 6 1 7 7.
	// Newest first: 7 hit (prev 7,1,6,5,4,3), 7 miss (prev 1,6,5,4,3,2), 1 hit,
	// 6 hit, 6 hit; the remaining 6 has only 5 earlier distinct numbers -> stop.
	newestFirst := []string{"7", "7", "1", "6", "6", "6", "5", "4", "3", "2", "1"}
	records := makeDraws(newestFirst)
	outcomes := windowOutcomes(records, hotnumber.Size6, hotnumber.PositionChampion, 180)
	want := []bool{true, false, true, true, true}
	if !reflect.DeepEqual(outcomes, want) {
		t.Fatalf("windowOutcomes = %v, want %v", outcomes, want)
	}
	stats := statsFromOutcomes(outcomes, 180)
	if stats.MaxWinStreak != 3 || stats.MaxLossStreak != 1 || stats.WinRate != 80 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestWindowOutcomesCapsAtWindowAndHandlesShortHistory(t *testing.T) {
	if got := windowOutcomes(nil, hotnumber.Size6, hotnumber.PositionChampion, 180); len(got) != 0 {
		t.Fatalf("empty history outcomes = %v", got)
	}
	if stats := statsFromOutcomes(nil, 180); stats != (predictionStats{}) {
		t.Fatalf("empty stats = %+v", stats)
	}
	firsts := make([]string, 400)
	for i := range firsts {
		firsts[i] = fmt.Sprintf("%d", i%9+1)
	}
	records := makeDraws(firsts)
	got := windowOutcomes(records, hotnumber.Size6, hotnumber.PositionChampion, 180)
	if len(got) != 180 {
		t.Fatalf("window not capped at 180: %d", len(got))
	}
	if short := windowOutcomes(records[:50], hotnumber.Size6, hotnumber.PositionChampion, 180); len(short) != 50-6 {
		t.Fatalf("short history outcomes = %d, want %d", len(short), 50-6)
	}
}

func TestWindowOutcomesRunnerUpUsesSecondPlace(t *testing.T) {
	records := make([]domain.DrawRecord, 0, 12)
	// Champion is always 01 (never 6 distinct numbers); runner-up cycles 1..9.
	for i := 0; i < 12; i++ {
		result, _ := json.Marshal([]string{"01", fmt.Sprintf("%02d", i%9+1)})
		records = append(records, domain.DrawRecord{IssueNumber: fmt.Sprintf("%010d", 2609060100-i), DrawResult: string(result)})
	}
	champion := windowOutcomes(records, hotnumber.Size6, hotnumber.PositionChampion, 180)
	runner := windowOutcomes(records, hotnumber.Size6, hotnumber.PositionRunnerUp, 180)
	if len(champion) != 0 {
		t.Fatalf("champion outcomes = %v, want none (only one distinct number)", champion)
	}
	if len(runner) == 0 {
		t.Fatal("runner-up outcomes missing")
	}
}
