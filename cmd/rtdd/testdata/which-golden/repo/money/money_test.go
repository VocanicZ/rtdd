package money

import "testing"

func TestParse(t *testing.T) {
	if Parse(" 1 ") != "1" {
		t.Fatal("Parse")
	}
}

func TestNormalize(t *testing.T) {
	if Normalize(" 1") != "1" {
		t.Fatal("Normalize")
	}
}
