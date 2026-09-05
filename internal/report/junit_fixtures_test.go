package report

import (
	"path/filepath"
	"testing"
)

// The six fixtures under testdata/junit are CAPTURED output — each one is the file a real
// runner wrote while executing a three-test suite (one pass, one fail, one skip), copied
// out unedited by scripts/capture-junit-fixtures.sh. That script also records how to
// regenerate each of them, and testdata/junit/README.md records the versions the committed
// files came from.
//
// They exist because "essentially every runner emits JUnit XML" is true only as far as the
// element names go. What the six actually disagree about, and what every future ecosystem
// added here must be checked against:
//
//	runner            root          nesting                     file=      skip spelling
//	----------------  ------------  --------------------------  ---------  -------------------------
//	Vitest            <testsuites>  one <testsuite> per file    absent     <skipped/>, empty
//	jest-junit        <testsuites>  one <testsuite> per DESCRIBE absent    <skipped/>, empty
//	go-junit-report   <testsuites>  one <testsuite> per package absent     <skipped message="Skipped">
//	Maven Surefire    <testsuite>   bare root, no wrapper       absent     <skipped message="not implemented yet"/>
//	RSpec             <testsuite>   bare root, no wrapper       PRESENT    <skipped/>, empty
//	PHPUnit           <testsuites>  <testsuite> INSIDE          PRESENT    <skipped/>, empty
//	                                <testsuite>
//
// Three further disagreements the table below pins but the columns above cannot show:
//   - classname is the file path (Vitest), the package (Go), the class (Surefire, PHPUnit),
//     the spec file in dotted form (RSpec) — and, in jest-junit, a copy of the test's own
//     full name, so classname and name are identical.
//   - jest-junit writes <failure> with NO message= attribute at all, and PHPUnit writes one
//     with type= but no message=. A reader that keys "did this fail" off message= reads
//     both ecosystems as passing.
//   - A suite name is the file (Vitest), the describe block (jest-junit), the package (Go),
//     the class (Surefire, PHPUnit) or the literal string "rspec". Nothing may depend on it
//     being any of those.
//
// The fixtures are ground truth: when a runner's real output contradicts the parser, the
// parser is what changes. A fixture is never hand-edited to make a test pass, and the
// durations below are the exact values in the committed files — a re-capture is expected to
// move them, and updating them is how a re-capture gets reviewed.
func TestReadJUnitFileReadsRealCapturedOutputFromSixRunners(t *testing.T) {
	fixtures := []struct {
		runner string // named in the subtest so a failure says which ecosystem broke
		file   string
		want   []JUnitCase
	}{
		{
			runner: "vitest",
			file:   "vitest.xml",
			want: []JUnitCase{
				{Suite: "test/calc.test.js", Classname: "test/calc.test.js", Name: "calculator > adds two numbers", Status: "pass", DurationMS: 3},
				{Suite: "test/calc.test.js", Classname: "test/calc.test.js", Name: "calculator > fails on a wrong sum", Status: "fail", DurationMS: 14},
				{Suite: "test/calc.test.js", Classname: "test/calc.test.js", Name: "calculator > is not implemented yet", Status: "skip", DurationMS: 0},
			},
		},
		{
			runner: "jest",
			file:   "jest.xml",
			want: []JUnitCase{
				{Suite: "calculator", Classname: "calculator adds two numbers", Name: "calculator adds two numbers", Status: "pass", DurationMS: 4},
				{Suite: "calculator", Classname: "calculator fails on a wrong sum", Name: "calculator fails on a wrong sum", Status: "fail", DurationMS: 4},
				{Suite: "calculator", Classname: "calculator is not implemented yet", Name: "calculator is not implemented yet", Status: "skip", DurationMS: 0},
			},
		},
		{
			runner: "go-junit-report",
			file:   "go-junit-report.xml",
			want: []JUnitCase{
				{Suite: "example.com/calc/calc", Classname: "example.com/calc/calc", Name: "TestAddsTwoNumbers", Status: "pass", DurationMS: 20},
				{Suite: "example.com/calc/calc", Classname: "example.com/calc/calc", Name: "TestFailsOnAWrongSum", Status: "fail", DurationMS: 0},
				{Suite: "example.com/calc/calc", Classname: "example.com/calc/calc", Name: "TestIsNotImplementedYet", Status: "skip", DurationMS: 0},
			},
		},
		{
			runner: "maven-surefire",
			file:   "surefire.xml",
			want: []JUnitCase{
				{Suite: "calc.CalcTest", Classname: "calc.CalcTest", Name: "addsTwoNumbers", Status: "pass", DurationMS: 30},
				{Suite: "calc.CalcTest", Classname: "calc.CalcTest", Name: "failsOnAWrongSum", Status: "fail", DurationMS: 10},
				{Suite: "calc.CalcTest", Classname: "calc.CalcTest", Name: "isNotImplementedYet", Status: "skip", DurationMS: 0},
			},
		},
		{
			runner: "rspec",
			file:   "rspec.xml",
			want: []JUnitCase{
				{Suite: "rspec", Classname: "spec.calc_spec", Name: "Calc adds two numbers", File: "./spec/calc_spec.rb", Status: "pass", DurationMS: 1},
				{Suite: "rspec", Classname: "spec.calc_spec", Name: "Calc fails on a wrong sum", File: "./spec/calc_spec.rb", Status: "fail", DurationMS: 9},
				{Suite: "rspec", Classname: "spec.calc_spec", Name: "Calc is not implemented yet", File: "./spec/calc_spec.rb", Status: "skip", DurationMS: 0},
			},
		},
		{
			// PHPUnit's inner <testsuite> is the class; the outer one is named after the
			// command line ("CLI Arguments"). The innermost enclosing suite is the one a
			// human recognises, so it is the one JUnitCase.Suite must carry.
			runner: "phpunit",
			file:   "phpunit.xml",
			want: []JUnitCase{
				{Suite: "CalcTest", Classname: "CalcTest", Name: "testAddsTwoNumbers", File: "/work/tests/CalcTest.php", Status: "pass", DurationMS: 0},
				{Suite: "CalcTest", Classname: "CalcTest", Name: "testFailsOnAWrongSum", File: "/work/tests/CalcTest.php", Status: "fail", DurationMS: 1},
				{Suite: "CalcTest", Classname: "CalcTest", Name: "testIsNotImplementedYet", File: "/work/tests/CalcTest.php", Status: "skip", DurationMS: 0},
			},
		},
	}

	for _, f := range fixtures {
		t.Run(f.runner, func(t *testing.T) {
			got, err := ReadJUnitFile(filepath.Join("testdata", "junit", f.file))
			if err != nil {
				t.Fatalf("ReadJUnitFile(%s): %v", f.file, err)
			}
			if len(got) != len(f.want) {
				t.Fatalf("got %d cases, want %d: %+v", len(got), len(f.want), got)
			}
			for i := range f.want {
				if got[i] != f.want[i] {
					t.Errorf("case %d = %+v, want %+v", i, got[i], f.want[i])
				}
			}

			// Every fixture exercises the whole outcome vocabulary the format has for a
			// test that ran: a pass, a fail and a skip. A capture that lost one of them
			// would still compare equal to a table someone updated to match it, so the
			// requirement is asserted here rather than left to the table.
			seen := map[string]int{}
			for _, c := range got {
				seen[c.Status]++
			}
			for _, status := range []string{"pass", "fail", "skip"} {
				if seen[status] == 0 {
					t.Errorf("fixture has no %q case; every captured suite must carry one pass, one fail and one skip", status)
				}
			}

			// Durations are telemetry, so a missing one is 0 rather than an error — but a
			// reader that returned 0 for EVERY case would pass the table above only until
			// someone regenerated it. Each of the six runners does emit a time= a human
			// would call non-zero on at least one case.
			nonZero := false
			for _, c := range got {
				if c.DurationMS > 0 {
					nonZero = true
				}
			}
			if !nonZero {
				t.Errorf("every case parsed as 0ms; %s emits time= in seconds and the reader must convert it", f.runner)
			}
		})
	}
}
