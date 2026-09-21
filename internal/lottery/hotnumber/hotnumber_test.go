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

func TestContainsEvaluatesFirstHotNumber(t *testing.T) {
	prediction := []string{"0", "1", "2", "4", "5", "6", "8"}
	if !Contains(prediction, "8") {
		t.Fatal("expected hot number 8 to be correct")
	}
	if Contains(prediction, "3") {
		t.Fatal("expected hot number 3 to be incorrect")
	}
}
