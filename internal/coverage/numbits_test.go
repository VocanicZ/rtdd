package coverage

import (
	"reflect"
	"testing"
)

// Every case below was read out of a real .coverage produced on this machine by
// `COVERAGE_CORE=ctrace pytest --cov --cov-context=test` (coverage.py 7.15.4) and
// cross-checked against coverage.numbits.numbits_to_nums. numbits is a PACKED
// BITMAP, not a line list: byte i, bit j set => line i*8+j.
func TestNumbitsRealBlobs(t *testing.T) {
	cases := []struct {
		name string
		blob []byte
		want []int
	}{
		{
			// src/logic.py, empty context: the import line and the three `def` lines.
			// 0x12 = 0b00010010 -> bits 1,4 -> lines 1,4
			// 0x11 = 0b00010001 -> bits 0,4 -> lines 8,12
			name: "logic.py import-time",
			blob: []byte{0x12, 0x11},
			want: []int{1, 4, 8, 12},
		},
		{
			// src/constants.py, empty context.
			// 0xEA = 0b11101010 -> bits 1,3,5,6,7
			name: "constants.py import-time",
			blob: []byte{0xEA},
			want: []int{1, 3, 5, 6, 7},
		},
		{
			// src/logic.py, context "tests/test_a.py::test_add|run".
			// 0x20 = 0b00100000 -> bit 5
			name: "test_add body",
			blob: []byte{0x20},
			want: []int{5},
		},
		{
			// src/logic.py, context "tests/test_b.py::test_mul|run".
			// byte 1 = 0x02 = 0b00000010 -> bit 1 -> line 8+1 = 9
			name: "test_mul body",
			blob: []byte{0x00, 0x02},
			want: []int{9},
		},
		{
			// An empty src/__init__.py: coverage records bit 0, which would decode to
			// "line 0". Line 0 is not a real source line, so Numbits drops it.
			name: "empty __init__.py",
			blob: []byte{0x01},
			want: []int{},
		},
		{name: "nil", blob: nil, want: []int{}},
		{name: "empty", blob: []byte{}, want: []int{}},
		{name: "all zero bytes", blob: []byte{0x00, 0x00, 0x00}, want: []int{}},
		{name: "every bit of byte 0", blob: []byte{0xFF}, want: []int{1, 2, 3, 4, 5, 6, 7}},
		{name: "high bit of byte 3", blob: []byte{0x00, 0x00, 0x00, 0x80}, want: []int{31}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Numbits(tc.blob)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Numbits(% x) = %v, want %v", tc.blob, got, tc.want)
			}
		})
	}
}

func TestNumbitsIsSortedAscending(t *testing.T) {
	got := Numbits([]byte{0xFF, 0xFF, 0xFF})
	// 24 bits are set; bit 0 decodes to line 0, which is dropped.
	if len(got) != 23 {
		t.Fatalf("len = %d, want 23", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("not ascending at %d: %v", i, got[:i+1])
		}
	}
}

func TestNumbitsNeverReturnsNil(t *testing.T) {
	if Numbits(nil) == nil {
		t.Fatal("Numbits(nil) returned a nil slice; callers range over it and append, want empty non-nil")
	}
	if Numbits([]byte{}) == nil {
		t.Fatal("Numbits([]byte{}) returned a nil slice, want empty non-nil")
	}
	if Numbits([]byte{0x01}) == nil {
		t.Fatal("Numbits([]byte{0x01}) dropped line 0 into a nil slice, want empty non-nil")
	}
}
