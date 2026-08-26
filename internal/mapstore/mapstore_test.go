package mapstore

import (
	"reflect"
	"testing"
)

func TestMapAccessors(t *testing.T) {
	m := New()
	if m.Len() != 0 {
		t.Fatalf("New().Len() = %d, want 0", m.Len())
	}
	if _, ok := m.Get("nope"); ok {
		t.Fatalf("Get on an empty map returned ok=true")
	}

	m.Replace(Row{T: "tests/test_b.py::test_x", F: []string{"src/b.py"}, C: "bbb1111", D: 5, S: "pass"})
	m.Replace(Row{T: "tests/test_a.py::test_y", F: []string{"src/a.py"}, C: "aaa2222", D: 9, S: "fail"})

	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}
	got := m.Rows()
	wantOrder := []string{"tests/test_a.py::test_y", "tests/test_b.py::test_x"}
	for i, w := range wantOrder {
		if got[i].T != w {
			t.Errorf("Rows()[%d].T = %q, want %q (Rows must be sorted by T ascending)", i, got[i].T, w)
		}
	}

	r, ok := m.Get("tests/test_a.py::test_y")
	if !ok {
		t.Fatalf("Get missed a row that Replace inserted")
	}
	if r.S != "fail" || r.D != 9 || r.C != "aaa2222" {
		t.Errorf("Get returned %+v, want S=fail D=9 C=aaa2222", r)
	}

	m.Delete("tests/test_a.py::test_y")
	if m.Len() != 1 {
		t.Errorf("after Delete, Len() = %d, want 1", m.Len())
	}
}

func TestReplaceNormalizesF(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		wantF []string
	}{
		{"sorts", []string{"src/z.py", "src/a.py"}, []string{"src/a.py", "src/z.py"}},
		{"dedupes", []string{"src/a.py", "src/a.py", "src/b.py"}, []string{"src/a.py", "src/b.py"}},
		{"nil becomes empty, never null", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Replace(Row{T: "t", F: tc.in, C: "c", D: 1, S: "pass"})
			got, _ := m.Get("t")
			if !reflect.DeepEqual(got.F, tc.wantF) {
				t.Errorf("F = %#v, want %#v", got.F, tc.wantF)
			}
		})
	}
}

func TestReplaceDoesNotAliasCallerSlice(t *testing.T) {
	f := []string{"src/b.py", "src/a.py"}
	m := New()
	m.Replace(Row{T: "t", F: f, C: "c", D: 1, S: "pass"})
	f[0] = "MUTATED"
	got, _ := m.Get("t")
	for _, s := range got.F {
		if s == "MUTATED" {
			t.Fatalf("Replace stored the caller's slice by reference: %#v", got.F)
		}
	}
}
