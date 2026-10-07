package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(1, 2); got != 3 {
		t.Errorf("Add(1, 2) = %d", got)
	}
}

func TestSum(t *testing.T) {
	if got := Sum(1, 2, 3); got != 6 {
		t.Fatalf("got %d", got)
	}
}
