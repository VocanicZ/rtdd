package report

import (
	"errors"
	"path/filepath"
	"strings"
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

// PRD #231 AC3, the load-bearing half: a parsed <testcase> is only an identifier if it can
// be handed back to the runner. So each fixture below carries the id_template an adapter
// for that runner could actually ship — a template whose rendering is ONE argv token that
// runner's own selector syntax accepts — and the ids it renders are written out in full so
// a reviewer can read them against the CLI in the `selector` column rather than trust a
// round-trip that would hold just as well for gibberish.
//
// Two of the six render a FILE-granular id, because Vitest and RSpec have no single-token
// form that names one test: `vitest run <path>` and `rspec <path>` are what those runners
// accept, and `-t` / `-e` are separate flags an id spliced by ExpandTests cannot become.
// A file-granular id is deliberately shared by every case in the file — the runner's own
// de-duplication is what collapses it — so uniqueness is asserted only where the template
// names {name}.
//
// The templates here are NOT the shipped adapter set: PRD #232 owns that, and this table
// may not be read as pinning it.
type idFixture struct {
	runner   string
	file     string
	tmpl     string
	selector string   // the CLI the rendered id is spliced into, one id per argv token
	wantIDs  []string // every case in the fixture, in document order
	perTest  bool     // does the template name one test, or one file?
}

var idFixtures = []idFixture{
	{
		runner:   "vitest",
		file:     "vitest.xml",
		tmpl:     "{classname}",
		selector: "vitest run --reporter=junit --outputFile={report} {tests}",
		wantIDs: []string{
			"test/calc.test.js",
			"test/calc.test.js",
			"test/calc.test.js",
		},
		// Vitest emits no file=; its classname IS the spec file path, which is exactly
		// what `vitest run <path>` filters on.
	},
	{
		runner:   "jest",
		file:     "jest.xml",
		tmpl:     "{name}",
		selector: "jest --reporters=jest-junit -t {tests}",
		wantIDs: []string{
			"calculator adds two numbers",
			"calculator fails on a wrong sum",
			"calculator is not implemented yet",
		},
		perTest: true,
		// jest-junit's name= is the full name including the describe chain, which is what
		// `jest -t` matches. The spaces survive unescaped: the id is one argv token.
	},
	{
		runner:   "go-junit-report",
		file:     "go-junit-report.xml",
		tmpl:     "{name}",
		selector: "go test ./... -run {tests}",
		wantIDs: []string{
			"TestAddsTwoNumbers",
			"TestFailsOnAWrongSum",
			"TestIsNotImplementedYet",
		},
		perTest: true,
		// go-junit-report's classname is the package import path, which `go test -run`
		// does not take; the name alone is the -run pattern.
	},
	{
		runner:   "maven-surefire",
		file:     "surefire.xml",
		tmpl:     "{classname}#{name}",
		selector: "mvn -B test -Dtest={tests}",
		wantIDs: []string{
			"calc.CalcTest#addsTwoNumbers",
			"calc.CalcTest#failsOnAWrongSum",
			"calc.CalcTest#isNotImplementedYet",
		},
		perTest: true,
		// Surefire's own -Dtest= syntax is literally class#method, fully qualified.
	},
	{
		runner:   "rspec",
		file:     "rspec.xml",
		tmpl:     "{file}",
		selector: "rspec --format RspecJunitFormatter --out {report} {tests}",
		wantIDs: []string{
			"./spec/calc_spec.rb",
			"./spec/calc_spec.rb",
			"./spec/calc_spec.rb",
		},
		// RSpec is one of the two runners that DO emit file=, and the path it writes is a
		// selector `rspec` accepts verbatim, leading "./" and all.
	},
	{
		runner:   "phpunit",
		file:     "phpunit.xml",
		tmpl:     "{classname}::{name}",
		selector: "phpunit --log-junit {report} --filter {tests}",
		wantIDs: []string{
			"CalcTest::testAddsTwoNumbers",
			"CalcTest::testFailsOnAWrongSum",
			"CalcTest::testIsNotImplementedYet",
		},
		perTest: true,
		// PHPUnit's --filter takes Class::method, which is the form its own classname and
		// name attributes spell out.
	},
}

// parse -> render -> parse is stable for all six shipped fixtures.
func TestIDRoundTripIsStableForEveryFixture(t *testing.T) {
	for _, f := range idFixtures {
		t.Run(f.runner, func(t *testing.T) {
			cases, err := ReadJUnitFile(filepath.Join("testdata", "junit", f.file))
			if err != nil {
				t.Fatalf("ReadJUnitFile(%s): %v", f.file, err)
			}
			if len(cases) != len(f.wantIDs) {
				t.Fatalf("fixture has %d cases, table names %d ids", len(cases), len(f.wantIDs))
			}

			seen := map[string]bool{}
			for i, c := range cases {
				id, err := RenderID(f.tmpl, c)
				if err != nil {
					t.Fatalf("RenderID(%q, %+v): %v", f.tmpl, c, err)
				}
				if id != f.wantIDs[i] {
					t.Errorf("case %d rendered %q, want %q — the id is spliced into `%s` as one argv token", i, id, f.wantIDs[i], f.selector)
				}
				if f.perTest {
					if seen[id] {
						t.Errorf("id %q is produced by two cases; a colliding id makes a subset run select the wrong test", id)
					}
					seen[id] = true
				}

				back, err := ParseID(f.tmpl, id)
				if err != nil {
					t.Fatalf("ParseID(%q, %q): %v", f.tmpl, id, err)
				}
				again, err := RenderID(f.tmpl, back)
				if err != nil {
					t.Fatalf("RenderID after ParseID (%q): %v", f.tmpl, err)
				}
				if again != id {
					t.Errorf("round trip through %q: %q -> %q", f.tmpl, id, again)
				}
			}
		})
	}
}

// Rendering the same report twice produces the same ids in the same order: the map's test
// ids and the subset command's arguments are joined by string equality, so a rendering
// that varied between runs would silently stop matching.
func TestIDRenderingIsDeterministicAcrossRepeatedReads(t *testing.T) {
	for _, f := range idFixtures {
		t.Run(f.runner, func(t *testing.T) {
			var first []string
			for pass := 0; pass < 5; pass++ {
				cases, err := ReadJUnitFile(filepath.Join("testdata", "junit", f.file))
				if err != nil {
					t.Fatalf("ReadJUnitFile: %v", err)
				}
				var ids []string
				for _, c := range cases {
					id, err := RenderID(f.tmpl, c)
					if err != nil {
						t.Fatalf("RenderID: %v", err)
					}
					ids = append(ids, id)
				}
				if pass == 0 {
					first = ids
					continue
				}
				if len(ids) != len(first) {
					t.Fatalf("pass %d rendered %d ids, pass 0 rendered %d", pass, len(ids), len(first))
				}
				for i := range ids {
					if ids[i] != first[i] {
						t.Fatalf("pass %d id %d = %q, want %q", pass, i, ids[i], first[i])
					}
				}
			}
		})
	}
}

// Decision 2, pinned to the four captures that genuinely lack the attribute: Vitest, Jest,
// go-junit-report and Maven Surefire all emit <testcase> with no file=. A template naming
// {file} against any of them is a NAMED error, never a path derived from classname or the
// suite — a derived path that is wrong renders an id the runner does not recognise, the
// subset selects nothing, and RTDD reports a green run that executed no tests.
func TestATemplateNamingFileAgainstARunnerThatOmitsItIsNamedRatherThanDerived(t *testing.T) {
	for _, name := range []string{"vitest.xml", "jest.xml", "go-junit-report.xml", "surefire.xml"} {
		t.Run(name, func(t *testing.T) {
			cases, err := ReadJUnitFile(filepath.Join("testdata", "junit", name))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v", err)
			}
			if len(cases) == 0 {
				t.Fatalf("fixture parsed to no cases")
			}
			for _, c := range cases {
				if c.File != "" {
					t.Fatalf("case %q has file=%q; this fixture is one of the four that must lack it", c.Name, c.File)
				}
				id, err := RenderID("{file}::{name}", c)
				if !errors.Is(err, ErrNoFileAttr) {
					t.Fatalf("RenderID = (%q, %v), want errors.Is(_, ErrNoFileAttr)", id, err)
				}
				// The message names the case and the placeholder, because the fix is the
				// adapter's id_template rather than anything in the report.
				for _, want := range []string{c.Name, "{file}", "id_template"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not name %q", err, want)
					}
				}
			}
		})
	}
}

// The two runners that DO emit file= are the ones a {file} template is for, and what they
// write is a path, not a class name dressed as one.
func TestTheTwoRunnersThatEmitFileRenderItVerbatim(t *testing.T) {
	for _, tc := range []struct{ file, wantPrefix string }{
		{"rspec.xml", "./spec/"},
		{"phpunit.xml", "/work/tests/"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			cases, err := ReadJUnitFile(filepath.Join("testdata", "junit", tc.file))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v", err)
			}
			for _, c := range cases {
				id, err := RenderID("{file}", c)
				if err != nil {
					t.Fatalf("RenderID(%+v): %v", c, err)
				}
				if id != c.File {
					t.Errorf("RenderID = %q, want the file= attribute verbatim (%q)", id, c.File)
				}
				if !strings.HasPrefix(id, tc.wantPrefix) {
					t.Errorf("rendered %q, want the path this runner actually wrote (prefix %q)", id, tc.wantPrefix)
				}
			}
		})
	}
}
