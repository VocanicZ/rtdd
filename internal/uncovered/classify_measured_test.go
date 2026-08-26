package uncovered_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/pytestfixture"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// measuredCoverage runs the shipped pytest fixture and decodes the .coverage it
// leaves behind. The unit tests classify against a hand-written copy of fixture F1;
// this one classifies against what coverage.py 7.x actually writes, so the two
// guarantees below are asserted on real data rather than on a transcription of it.
func measuredCoverage(t *testing.T) *coverage.Result {
	t.Helper()
	if !pytestfixture.HavePytest() {
		t.Skip("pytest not on PATH")
	}
	dir := t.TempDir()
	if err := pytestfixture.Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	cmd := exec.Command("pytest", "--cov", "--cov-context=test", "--cov-report=", "-q")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COVERAGE_CORE=ctrace", "COVERAGE_FILE=.coverage")
	out, _ := cmd.CombinedOutput() // the fixture suite exits 1 by design
	cov, err := coverage.ReadSQLite(filepath.Join(dir, ".coverage"), dir)
	if err != nil {
		t.Fatalf("ReadSQLite: %v\npytest output:\n%s", err, out)
	}
	return cov
}

func TestMeasuredImportTimeOnlyFileIsClean(t *testing.T) {
	// Audit A1 on real coverage: src/constants.py is a MAX constant plus a @dataclass,
	// imported and asserted on by two passing tests, attributed to ZERO test contexts.
	// Its import-time lines are 1,3,5,6,7 — 2 and 4 are blank lines coverage never
	// records. Changing the file end to end must report nothing uncovered.
	cov := measuredCoverage(t)

	if len(cov.ImportTime["src/constants.py"]) == 0 {
		t.Fatalf("fixture invariant broken: src/constants.py has no import-time lines: %#v", cov.ImportTime)
	}
	for _, tc := range cov.PerTest {
		if len(tc.Files["src/constants.py"]) != 0 {
			t.Fatalf("fixture invariant broken: %s attributes lines in src/constants.py; "+
				"A1 says import-time code is attributed to zero test contexts", tc.Test)
		}
	}

	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 7}}},
	}
	reports := uncovered.Classify(changes, cov)
	if n := uncovered.Summarize(reports).UncoveredLines; n != 0 {
		t.Fatalf("uncovered_lines = %d, want 0 — a constants/@dataclass module asserted on "+
			"by two passing tests must report clean\nreports: %#v", n, reports)
	}
}

func TestMeasuredClassificationIsLineGranular(t *testing.T) {
	// Audit A2 on real coverage: src/logic.py IS covered (line 5 belongs to test_add),
	// yet lines 12-13 are the never-called `unused`. Line 12 is the `def`, executed at
	// import; line 13 is its body, which nothing executes.
	cov := measuredCoverage(t)

	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 12, End: 13}}},
	}
	reports := uncovered.Classify(changes, cov)
	if len(reports) != 1 {
		t.Fatalf("reports = %#v, want 1 file", reports)
	}
	want := []uncovered.ClassifiedRange{
		{Range: gitctx.LineRange{Start: 12, End: 12}, Class: uncovered.ImportTime},
		{Range: gitctx.LineRange{Start: 13, End: 13}, Class: uncovered.Uncovered},
	}
	if n := reports[0].UncoveredLines(); n != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1 (line 13 only)\nranges: %#v", n, reports[0].Ranges)
	}
	if len(reports[0].Ranges) != len(want) {
		t.Fatalf("ranges = %#v, want %#v", reports[0].Ranges, want)
	}
	for i, r := range reports[0].Ranges {
		if r != want[i] {
			t.Fatalf("ranges = %#v, want %#v", reports[0].Ranges, want)
		}
	}

	// The covered function body in the SAME file is attributed to its test.
	covReports := uncovered.Classify([]gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 5, End: 5}}},
	}, cov)
	if len(covReports) != 1 || len(covReports[0].Ranges) != 1 ||
		covReports[0].Ranges[0].Class != uncovered.Covered {
		t.Fatalf("line 5 must be Covered; got %#v", covReports)
	}
}
