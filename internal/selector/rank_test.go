package selector

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func mapOf(rows ...mapstore.Row) *mapstore.Map {
	m := mapstore.New()
	for _, r := range rows {
		m.Replace(r)
	}
	return m
}

func TestRank(t *testing.T) {
	tests := []struct {
		name    string
		m       *mapstore.Map
		tests   []string
		changed []string
		want    []string
	}{
		{
			name: "descending intersection ratio dominates",
			m: mapOf(
				mapstore.Row{T: "wide", F: []string{"src/a.py", "src/b.py", "src/c.py", "src/d.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "narrow", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"wide", "narrow"},
			changed: []string{"src/a.py"},
			want:    []string{"narrow", "wide"}, // 1.0 before 0.25
		},
		{
			name: "ratio beats last-failed",
			m: mapOf(
				mapstore.Row{T: "failed_wide", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 1, S: "fail"},
				mapstore.Row{T: "passed_narrow", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"failed_wide", "passed_narrow"},
			changed: []string{"src/a.py"},
			want:    []string{"passed_narrow", "failed_wide"}, // 1.0 before 0.5, outcome is only key 2
		},
		{
			name: "last-failed first breaks a ratio tie",
			m: mapOf(
				mapstore.Row{T: "passed", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "failed", F: []string{"src/a.py"}, C: "c1", D: 1, S: "fail"},
			),
			tests:   []string{"passed", "failed"},
			changed: []string{"src/a.py"},
			want:    []string{"failed", "passed"},
		},
		{
			name: "last-failed beats ascending len(F)",
			m: mapOf(
				mapstore.Row{T: "failed_two", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 1, S: "fail"},
				mapstore.Row{T: "passed_one", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"passed_one", "failed_two"},
			changed: []string{"src/a.py", "src/b.py"}, // both are ratio 1.0
			want:    []string{"failed_two", "passed_one"},
		},
		{
			name: "only fail counts as last-failed; error and skip do not",
			m: mapOf(
				mapstore.Row{T: "a_errored", F: []string{"src/a.py"}, C: "c1", D: 1, S: "error"},
				mapstore.Row{T: "b_failed", F: []string{"src/a.py"}, C: "c1", D: 1, S: "fail"},
				mapstore.Row{T: "c_skipped", F: []string{"src/a.py"}, C: "c1", D: 1, S: "skip"},
			),
			tests:   []string{"a_errored", "b_failed", "c_skipped"},
			changed: []string{"src/a.py"},
			want:    []string{"b_failed", "a_errored", "c_skipped"},
		},
		{
			name: "ascending len(F) breaks a ratio and outcome tie",
			m: mapOf(
				mapstore.Row{T: "two_files", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "one_file", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"two_files", "one_file"},
			changed: []string{"src/a.py", "src/b.py"}, // both are ratio 1.0
			want:    []string{"one_file", "two_files"},
		},
		{
			name: "ascending len(F) beats ascending duration",
			m: mapOf(
				mapstore.Row{T: "narrow_slow", F: []string{"src/a.py"}, C: "c1", D: 900, S: "pass"},
				mapstore.Row{T: "wide_fast", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 3, S: "pass"},
			),
			tests:   []string{"wide_fast", "narrow_slow"},
			changed: []string{"src/a.py", "src/b.py"}, // both are ratio 1.0
			want:    []string{"narrow_slow", "wide_fast"},
		},
		{
			name: "ascending duration breaks a len(F) tie",
			m: mapOf(
				mapstore.Row{T: "slow", F: []string{"src/a.py"}, C: "c1", D: 900, S: "pass"},
				mapstore.Row{T: "fast", F: []string{"src/a.py"}, C: "c1", D: 3, S: "pass"},
			),
			tests:   []string{"slow", "fast"},
			changed: []string{"src/a.py"},
			want:    []string{"fast", "slow"},
		},
		{
			name: "test id breaks a total tie, so the order is deterministic",
			m: mapOf(
				mapstore.Row{T: "b_test", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
				mapstore.Row{T: "a_test", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
			),
			tests:   []string{"b_test", "a_test"},
			changed: []string{"src/a.py"},
			want:    []string{"a_test", "b_test"},
		},
		{
			name: "all four keys at once",
			m: mapOf(
				mapstore.Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "c1", D: 412, S: "pass"},
				mapstore.Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "c1", D: 90, S: "fail"},
				mapstore.Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "c1", D: 15, S: "pass"},
			),
			tests: []string{
				"tests/test_auth.py::test_login",
				"tests/test_auth.py::test_logout",
				"tests/test_db.py::test_query",
			},
			changed: []string{"src/auth.py", "src/db.py"},
			// all three are ratio 1.0; the failed one leads, then len(F)=1 (D=15) then len(F)=2
			want: []string{
				"tests/test_auth.py::test_logout",
				"tests/test_db.py::test_query",
				"tests/test_auth.py::test_login",
			},
		},
		{
			name:    "a test with no map row sorts last but is never dropped",
			m:       mapOf(mapstore.Row{T: "mapped", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"}),
			tests:   []string{"unmapped", "mapped"},
			changed: []string{"src/a.py"},
			want:    []string{"mapped", "unmapped"},
		},
		{
			name: "a row whose files none of the changed set touches keeps ratio 0",
			m: mapOf(
				mapstore.Row{T: "untouched", F: []string{"src/z.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "touched", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"untouched", "touched"},
			changed: []string{"src/a.py"},
			want:    []string{"touched", "untouched"}, // 0.5 before 0.0
		},
		{
			name:    "no changed files at all leaves every ratio 0 and falls through to the later keys",
			m:       mapOf(mapstore.Row{T: "slow", F: []string{"src/a.py"}, C: "c1", D: 900, S: "pass"}, mapstore.Row{T: "fast", F: []string{"src/a.py"}, C: "c1", D: 3, S: "pass"}),
			tests:   []string{"slow", "fast"},
			changed: nil,
			want:    []string{"fast", "slow"},
		},
		{
			name:    "duplicates collapse, first occurrence order irrelevant",
			m:       mapOf(mapstore.Row{T: "a", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"}),
			tests:   []string{"a", "a"},
			changed: []string{"src/a.py"},
			want:    []string{"a"},
		},
		{
			name:    "empty input",
			m:       mapOf(),
			tests:   nil,
			changed: []string{"src/a.py"},
			want:    []string{},
		},
		{
			name:    "a nil map never panics and never drops a test",
			m:       nil,
			tests:   []string{"b", "a"},
			changed: []string{"src/a.py"},
			want:    []string{"a", "b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Rank(tc.m, tc.tests, tc.changed)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Rank = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// The ranked list is printed and diffed, so rows that tie on all four spec keys must not
// depend on the caller's input order or on the sort's internal pivot choices.
func TestRankIsStableForIdenticalRows(t *testing.T) {
	m := mapOf(
		mapstore.Row{T: "t_a", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
		mapstore.Row{T: "t_b", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
		mapstore.Row{T: "t_c", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
		mapstore.Row{T: "t_d", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
	)
	want := []string{"t_a", "t_b", "t_c", "t_d"}
	orders := [][]string{
		{"t_a", "t_b", "t_c", "t_d"},
		{"t_d", "t_c", "t_b", "t_a"},
		{"t_c", "t_a", "t_d", "t_b"},
		{"t_b", "t_d", "t_a", "t_c"},
	}
	for _, in := range orders {
		for run := 0; run < 3; run++ {
			if got := Rank(m, in, []string{"src/a.py"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("Rank(%v) run %d = %#v, want %#v", in, run, got, want)
			}
		}
	}
}

// Rank reads the map and the caller's slices; it must not reorder or otherwise mutate them.
func TestRankDoesNotMutateItsArguments(t *testing.T) {
	m := mapOf(
		mapstore.Row{T: "slow", F: []string{"src/a.py"}, C: "c1", D: 900, S: "pass"},
		mapstore.Row{T: "fast", F: []string{"src/a.py"}, C: "c1", D: 3, S: "pass"},
	)
	tests := []string{"slow", "fast"}
	changed := []string{"src/b.py", "src/a.py"}

	Rank(m, tests, changed)

	if !reflect.DeepEqual(tests, []string{"slow", "fast"}) {
		t.Errorf("tests mutated: %#v", tests)
	}
	if !reflect.DeepEqual(changed, []string{"src/b.py", "src/a.py"}) {
		t.Errorf("changedFiles mutated: %#v", changed)
	}
}
