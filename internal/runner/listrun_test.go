package runner

import (
	"strings"
	"testing"
)

// Decision 12 of docs/plans/06-m6d-shipped-adapters.md: no runner's enumeration command
// emits ids in the adapter's own id namespace — `vitest list` prints case names while the
// vitest id is a file path, and Maven and Gradle have no enumeration command at all. The
// one place a runner and its adapter agree is report_path, because that is where
// id_template renders. So for a `report: junit-xml` adapter, enumerating IS running, and
// ListRun hands the caller the outcomes that run produced.
func TestListRunReadsIDsAndOutcomesFromTheReportOfAJUnitAdapter(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	a.List = strings.Replace(a.Subset, " {tests}", "", 1)
	xml := writeStubXML(t, repo, "all.xml", `<testsuite name="s" time="0.04">
  <testcase classname="s.A" name="one" time="0.01"/>
  <testcase classname="s.B" name="two" time="0.03"><failure message="nope"/></testcase>
</testsuite>`)
	t.Setenv("RTDD_STUB_XML", xml)

	res, ids, err := ListRun(a, repo)
	if err != nil {
		t.Fatalf("ListRun: %v", err)
	}
	want := []string{"s.A#one", "s.B#two"}
	if len(ids) != len(want) {
		t.Fatalf("ListRun ids = %v, want %v", ids, want)
	}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("ListRun ids[%d] = %q, want %q", i, id, want[i])
		}
	}
	if res == nil {
		t.Fatal("ListRun returned no RunResult: a junit enumeration RAN the suite, and its outcomes are not to be thrown away")
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("ListRun outcomes = %v, want the two cases the report holds", res.Outcomes)
	}
	if res.Outcomes[1].Status != "fail" {
		t.Errorf("ListRun outcome[1].Status = %q, want %q", res.Outcomes[1].Status, "fail")
	}
}

// A coverage adapter enumerates with a COLLECTION — `pytest --collect-only` executes
// nothing — so there is no RunResult to hand back, and List's stdout parsing is
// unchanged. Reusing a nil result is what keeps the subset invocation happening.
func TestListRunReturnsNoResultForACollectOnlyAdapter(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, nil)
	t.Setenv("RTDD_STUB_STDOUT", "tests/test_a.py::test_one\ntests/test_a.py::test_two\n\n2 tests collected in 0.01s")

	res, ids, err := ListRun(a, repo)
	if err != nil {
		t.Fatalf("ListRun: %v", err)
	}
	if res != nil {
		t.Errorf("ListRun result = %+v, want nil: a collection produced no outcomes", res)
	}
	if len(ids) != 2 {
		t.Fatalf("ListRun ids = %v, want the two collected nodeids", ids)
	}
}
