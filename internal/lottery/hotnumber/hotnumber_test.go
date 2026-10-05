package hotnumber

import "testing"

func TestPredictionSortsSevenNumbers(t *testing.T) {
	numbers := []string{"2", "4", "0", "5", "1", "6", "8"}
	if got := Prediction(numbers); got != "1245680" {
		t.Fatalf("Prediction() = %q", got)
	}
}

func TestCarTenMapsToZero(t *testing.T) {
	got, err := FromDrawResult([]string{"10", "08"})
	if err != nil || got != "0" {
		t.Fatalf("FromDrawResult() = %q, %v", got, err)
	}
}

func TestFromDrawResultAtRunnerUp(t *testing.T) {
	got, err := FromDrawResultAt([]string{"03", "10", "01"}, PositionRunnerUp)
	if err != nil || got != "0" {
		t.Fatalf("FromDrawResultAt runner-up = %q, %v", got, err)
	}
	got, err = FromDrawResultAt([]string{"03", "08", "01"}, PositionRunnerUp)
	if err != nil || got != "8" {
		t.Fatalf("FromDrawResultAt runner-up = %q, %v", got, err)
	}
	if _, err := FromDrawResultAt([]string{"03"}, PositionRunnerUp); err == nil {
		t.Fatal("expected error when result shorter than position")
	}
}

func TestFromDrawResultDefaultsToChampion(t *testing.T) {
	got, err := FromDrawResult([]string{"05", "08"})
	if err != nil || got != "5" {
		t.Fatalf("FromDrawResult() = %q, %v", got, err)
	}
}

func TestContainsEvaluatesFirstHotNumber(t *testing.T) {
	prediction := []string{"0", "1", "2", "4", "5", "6", "8"}
	if !Contains(prediction, "8") {
		t.Fatal("expected hot number 8 to be correct")
	}
	if Contains(prediction, "3") {
		t.Fatal("expected hot number 3 to be incorrect")
	}
}

func TestTakeFirst(t *testing.T) {
	nums := []string{"1", "2", "3", "4", "5", "6", "7"}
	got := TakeFirst(nums, Size6)
	if len(got) != 6 || got[5] != "6" {
		t.Fatalf("TakeFirst size6 = %v", got)
	}
	got = TakeFirst(nums[:3], 6)
	if len(got) != 3 {
		t.Fatalf("TakeFirst shorter = %v", got)
	}
	got = TakeFirst(nums, 0)
	if len(got) != 0 {
		t.Fatalf("TakeFirst zero = %v", got)
	}
}

func TestPositionLabel(t *testing.T) {
	if PositionLabel(PositionChampion) != "冠军" {
		t.Fatal("champion label")
	}
	if PositionLabel(PositionRunnerUp) != "亚军" {
		t.Fatal("runner-up label")
	}
}
