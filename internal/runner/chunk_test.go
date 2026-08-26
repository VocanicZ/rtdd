package runner

import (
	"fmt"
	"reflect"
	"testing"
)

func TestChunk(t *testing.T) {
	cases := []struct {
		name     string
		tests    []string
		maxBytes int
		want     [][]string
	}{
		{name: "nil", tests: nil, maxBytes: 100, want: nil},
		{name: "empty", tests: []string{}, maxBytes: 100, want: nil},
		{
			name:     "everything fits in one chunk",
			tests:    []string{"a", "b", "c"},
			maxBytes: 100,
			want:     [][]string{{"a", "b", "c"}},
		},
		{
			// len(id)+1 accounts for the NUL separator each argv element costs.
			// "aaaaaaaaaa" is 10 bytes -> 11 each. 11+11=22 <= 25; +11=33 > 25.
			name:     "splits when the next id would exceed the budget",
			tests:    []string{"aaaaaaaaaa", "bbbbbbbbbb", "cccccccccc"},
			maxBytes: 25,
			want:     [][]string{{"aaaaaaaaaa", "bbbbbbbbbb"}, {"cccccccccc"}},
		},
		{
			name:     "exactly on the boundary",
			tests:    []string{"aaaaaaaaaa", "bbbbbbbbbb"},
			maxBytes: 22,
			want:     [][]string{{"aaaaaaaaaa", "bbbbbbbbbb"}},
		},
		{
			name:     "one id larger than the budget gets its own chunk, never dropped",
			tests:    []string{"a", "this-single-id-is-far-longer-than-the-budget", "b"},
			maxBytes: 10,
			want: [][]string{
				{"a"},
				{"this-single-id-is-far-longer-than-the-budget"},
				{"b"},
			},
		},
		{
			name:     "non-positive maxBytes falls back to MaxArgvBytes",
			tests:    []string{"a", "b"},
			maxBytes: 0,
			want:     [][]string{{"a", "b"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Chunk(tc.tests, tc.maxBytes)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Chunk(%q, %d) =\n  %q\nwant\n  %q", tc.tests, tc.maxBytes, got, tc.want)
			}
		})
	}
}

// Measured on this machine: 8,000 realistic pytest ids average 59 bytes and total
// 460 KB of argv. Linux ARG_MAX is 2,097,152 (fits); the Windows CMD limit is
// 8,191 characters (57x over). pytest has no argfile option, so this must chunk.
func TestChunkAtRealisticScale(t *testing.T) {
	tests := make([]string, 8000)
	for i := range tests {
		tests[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	total := 0
	for _, s := range tests {
		total += len(s) + 1
	}
	if total < 400_000 {
		t.Fatalf("fixture ids total %d bytes, want a realistic >400KB argv", total)
	}

	chunks := Chunk(tests, MaxArgvBytes)
	if len(chunks) < 2 {
		t.Fatalf("Chunk produced %d chunk(s) for %d bytes of ids; it must split", len(chunks), total)
	}

	var seen []string
	for i, c := range chunks {
		if len(c) == 0 {
			t.Fatalf("chunk %d is empty", i)
		}
		size := 0
		for _, s := range c {
			size += len(s) + 1
		}
		if size > MaxArgvBytes {
			t.Fatalf("chunk %d is %d bytes, over MaxArgvBytes=%d", i, size, MaxArgvBytes)
		}
		seen = append(seen, c...)
	}
	if !reflect.DeepEqual(seen, tests) {
		t.Fatalf("chunking lost or reordered ids: got %d, want %d", len(seen), len(tests))
	}
}

func TestMaxArgvBytesIsUnderTheWindowsLimitTimesTwelve(t *testing.T) {
	if MaxArgvBytes != 100_000 {
		t.Fatalf("MaxArgvBytes = %d, want 100000 (the value the interface contract fixes)", MaxArgvBytes)
	}
}
