package report

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// goldenOutcomes is the expected parse of testdata/report.jsonl, which is trimmed
// but otherwise verbatim output from a real `pytest --report-log` run. It carries
// one line-for-line example of each of the six measured phase shapes.
var goldenOutcomes = []Outcome{
	// setup=passed, call=passed, teardown=passed.
	{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 412},
	// Sub-millisecond call durations round to 0, they do not become 1.
	{Test: "tests/test_a.py::test_param[1-one two]", Status: "pass", DurationMS: 0},
	// setup=passed, call=failed, teardown=passed.
	{Test: "tests/test_b.py::test_fail", Status: "fail", DurationMS: 1},
	// No `call` entry at all: pytest emits setup(skipped) + teardown(passed).
	// A reader that only looks at the call phase loses this test entirely.
	{Test: "tests/test_b.py::test_skipped", Status: "skip", DurationMS: 1},
	// pytest.skip() inside the body: setup passes, the call phase is skipped.
	{Test: "tests/test_c.py::test_skip_in_body", Status: "skip", DurationMS: 3},
	// A fixture that raises: setup(failed) + teardown(passed), no call phase.
	{Test: "tests/test_d.py::test_errors", Status: "error", DurationMS: 1},
	// A passing test whose teardown blows up is an error, not a pass.
	{Test: "tests/test_e.py::test_teardown_boom", Status: "error", DurationMS: 2},
}

func TestReadReportLog(t *testing.T) {
	got, err := ReadReportLog(filepath.Join("testdata", "report.jsonl"))
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	if !reflect.DeepEqual(got, goldenOutcomes) {
		t.Fatalf("ReadReportLog =\n  %+v\nwant\n  %+v", got, goldenOutcomes)
	}
}

// TestReadReportLogPhaseShapes gives each of the six measured phase shapes its own
// named case, so a regression names the shape it broke instead of dumping a slice.
func TestReadReportLogPhaseShapes(t *testing.T) {
	got, err := ReadReportLog(filepath.Join("testdata", "report.jsonl"))
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	byTest := map[string]Outcome{}
	for _, o := range got {
		byTest[o.Test] = o
	}

	cases := []struct {
		name       string
		test       string
		wantStatus string
		wantMS     int
	}{
		{"setup passed, call passed, teardown passed", "tests/test_a.py::test_add", "pass", 412},
		{"setup passed, call failed, teardown passed", "tests/test_b.py::test_fail", "fail", 1},
		{"setup passed, call skipped, teardown passed", "tests/test_c.py::test_skip_in_body", "skip", 3},
		{"setup skipped, no call entry", "tests/test_b.py::test_skipped", "skip", 1},
		{"setup failed, no call entry", "tests/test_d.py::test_errors", "error", 1},
		{"setup passed, call passed, teardown failed", "tests/test_e.py::test_teardown_boom", "error", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, ok := byTest[tc.test]
			if !ok {
				t.Fatalf("%s is missing from the outcomes entirely", tc.test)
			}
			if o.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", o.Status, tc.wantStatus)
			}
			if o.DurationMS != tc.wantMS {
				t.Errorf("DurationMS = %d, want %d", o.DurationMS, tc.wantMS)
			}
		})
	}
}

// TestReadReportLogSumsAllPhasesWhenThereIsNoCall pins the duration rule for the
// two no-call shapes: setup+teardown, converted from seconds to milliseconds.
func TestReadReportLogSumsAllPhasesWhenThereIsNoCall(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	body := `{"$report_type": "TestReport", "nodeid": "t.py::x", "when": "setup", "outcome": "skipped", "duration": 1.25}
{"$report_type": "TestReport", "nodeid": "t.py::x", "when": "teardown", "outcome": "passed", "duration": 0.5}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadReportLog(p)
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	want := []Outcome{{Test: "t.py::x", Status: "skip", DurationMS: 1750}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadReportLog = %+v, want %+v", got, want)
	}
}

func TestReadReportLogPreservesFirstSeenOrder(t *testing.T) {
	got, err := ReadReportLog(filepath.Join("testdata", "report.jsonl"))
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	wantOrder := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_skipped",
		"tests/test_c.py::test_skip_in_body",
		"tests/test_d.py::test_errors",
		"tests/test_e.py::test_teardown_boom",
	}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d outcomes, want %d: %+v", len(got), len(wantOrder), got)
	}
	for i, w := range wantOrder {
		if got[i].Test != w {
			t.Fatalf("Outcome[%d].Test = %q, want %q", i, got[i].Test, w)
		}
	}
}

// A test id is first seen at its setup phase, so interleaved tests keep the order
// their setup phases appeared in, not the order they finished in.
func TestReadReportLogOrderIsFirstAppearanceNotCompletion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	body := `{"$report_type": "TestReport", "nodeid": "t.py::slow", "when": "setup", "outcome": "passed", "duration": 0.001}
{"$report_type": "TestReport", "nodeid": "t.py::fast", "when": "setup", "outcome": "passed", "duration": 0.001}
{"$report_type": "TestReport", "nodeid": "t.py::fast", "when": "call", "outcome": "passed", "duration": 0.001}
{"$report_type": "TestReport", "nodeid": "t.py::fast", "when": "teardown", "outcome": "passed", "duration": 0.001}
{"$report_type": "TestReport", "nodeid": "t.py::slow", "when": "call", "outcome": "passed", "duration": 0.001}
{"$report_type": "TestReport", "nodeid": "t.py::slow", "when": "teardown", "outcome": "passed", "duration": 0.001}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadReportLog(p)
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	if len(got) != 2 || got[0].Test != "t.py::slow" || got[1].Test != "t.py::fast" {
		t.Fatalf("ReadReportLog = %+v, want slow then fast", got)
	}
}

func TestReadReportLogIgnoresNonTestReportEnvelopes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	body := `{"pytest_version": "9.0.3", "$report_type": "SessionStart"}
{"nodeid": "", "outcome": "passed", "longrepr": null, "result": null, "sections": [], "$report_type": "CollectReport"}
{"exitstatus": 4, "$report_type": "SessionFinish"}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadReportLog(p)
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadReportLog = %+v, want no outcomes", got)
	}
}

// A CollectReport carries a nodeid too — a file path, not a test id. Keying on
// nodeid without checking $report_type would invent a "tests/test_a.py" outcome.
func TestReadReportLogIgnoresCollectReportNodeIDs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	body := `{"nodeid": "tests/test_a.py", "outcome": "passed", "longrepr": null, "result": null, "sections": [], "$report_type": "CollectReport"}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadReportLog(p)
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadReportLog = %+v, want no outcomes", got)
	}
}

func TestReadReportLogMalformedLineIsFatal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	body := `{"pytest_version": "9.0.3", "$report_type": "SessionStart"}
this is not json
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := ReadReportLog(p); err == nil {
		t.Fatal("ReadReportLog on a malformed line = nil error; silently dropping outcomes narrows the map")
	}
}

func TestReadReportLogMissingFileIsAnError(t *testing.T) {
	if _, err := ReadReportLog(filepath.Join(t.TempDir(), "nope.jsonl")); err == nil {
		t.Fatal("ReadReportLog on a missing file = nil error, want error")
	}
}

func TestReadReportLogEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.jsonl")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadReportLog(p)
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadReportLog = %+v, want no outcomes", got)
	}
}
