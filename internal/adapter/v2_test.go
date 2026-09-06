package adapter

import (
	"strings"
	"testing"
)

// staticYAML is the shape spec §4.2 describes: a toolchain RTDD cannot instrument,
// described entirely declaratively. Every key here is new in contract v2 except
// name/detect/subset/list and the glob fields.
const staticYAML = `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests} --reporter=junit --outputFile={report}"
list: "npx vitest list"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
test_for:
  - "{dir}/{name}.test.ts"
  - "{dir}/__tests__/{name}.test.ts"
  - "tests/{name}.test.ts"
importscan:
  command: "node {script}"
  script: "scan-imports.mjs"
requires:
  - bin: node
    reason: "the importscan script and the vitest runner both run under node"
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
`

// An adapter that declares no selection keeps the meaning it has today. This is what
// makes contract v2 additive rather than a migration: spec §4.1 requires a seeded Python
// repo's selection to stay byte-identical, and no shipped adapter is edited to get it.
func TestSelectionDefaultsToCoverage(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "demo.yaml", validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Selection != SelectionCoverage {
		t.Errorf("Selection = %q, want the default %q", a.Selection, SelectionCoverage)
	}
}

// The shipped Python adapter declares no selection key and must not need one.
func TestBuiltinPythonReportsCoverageSelection(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatalf("Builtin has no python adapter")
	}
	if py.Selection != SelectionCoverage {
		t.Errorf("python Selection = %q, want the default %q", py.Selection, SelectionCoverage)
	}
	if py.Coverage != "sqlite" {
		t.Errorf("python Coverage = %q, want %q", py.Coverage, "sqlite")
	}
}

// selection: static with coverage: none and no seed is the whole point of the pair: an
// adapter that says outright it cannot derive selection from execution.
func TestLoadParsesStaticSelection(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Selection != SelectionStatic {
		t.Errorf("Selection = %q, want %q", a.Selection, SelectionStatic)
	}
	if a.Coverage != CoverageNone {
		t.Errorf("Coverage = %q, want %q", a.Coverage, CoverageNone)
	}
}

// selection takes exactly two values. Anything else is a typo, and exit 2 must name both
// the field and what was written so an agent can fix it without reading the source.
func TestLoadRejectsAnUnknownSelectionValue(t *testing.T) {
	p := writeAdapter(t, t.TempDir(), "a.yaml", strings.Replace(staticYAML, "selection: static", "selection: guesswork", 1))
	_, err := Load(p)
	if err == nil {
		t.Fatalf("Load accepted selection: guesswork; an unknown selection is a configuration error (exit 2)")
	}
	for _, want := range []string{"selection", `"guesswork"`, p} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// The two combinations that cannot mean anything, with the wording the M6a plan pins.
// Each message names BOTH offending keys, so an agent reading exit 2 knows which one to
// change and what the other must become. The third case closes the remaining direction,
// so no combination of the three keys can express a static tier that also reads coverage.
func TestValidateRejectsTheIllegalKeyCombinations(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "coverage none with defaulted selection",
			yaml: `name: vitest
detect: ["package.json"]
seed: "npx vitest run"
subset: "npx vitest run {tests}"
coverage: none
report: pytest-reportlog
`,
			want: `coverage: none requires selection: static, got selection "coverage"`,
		},
		{
			name: "static with a seed command",
			yaml: `name: vitest
detect: ["package.json"]
seed: "pytest --cov"
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: pytest-reportlog
`,
			want: `selection: static forbids seed, got seed "pytest --cov"`,
		},
		{
			name: "static with coverage sqlite",
			yaml: `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: sqlite
report: pytest-reportlog
`,
			want: `selection: static requires coverage: none, got coverage "sqlite"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted an illegal selection/coverage/seed combination")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
			// Every adapter rejection names the file it came from, so a bad adapter in a
			// directory of adapters is identifiable.
			if !strings.Contains(err.Error(), p) {
				t.Errorf("error = %q, want it to name the source file %q", err.Error(), p)
			}
		})
	}
}

// seed instruments a suite. There is nothing to instrument under selection: static, and a
// coverage adapter without one cannot build a map at all.
func TestSeedIsForbiddenUnderStaticAndRequiredUnderCoverage(t *testing.T) {
	if _, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML)); err != nil {
		t.Errorf("static adapter without seed: Load = %v, want nil", err)
	}

	noSeed := `name: python2
detect: ["pyproject.toml"]
subset: "pytest {tests} --cov"
coverage: sqlite
report: pytest-reportlog
`
	_, err := Load(writeAdapter(t, t.TempDir(), "b.yaml", noSeed))
	if err == nil {
		t.Fatalf("Load accepted a coverage adapter with no seed command")
	}
	if !strings.Contains(err.Error(), "seed is required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "seed is required")
	}
}

// The shipped adapter must survive every new rule unchanged. adapters/python.yaml is
// byte-frozen for this PRD; this is the behavioural half of that freeze.
func TestBuiltinAdaptersStillValidate(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("Builtin returned no adapters")
	}
}

// The four keys that describe how a static-tier adapter finds and names its tests
// (spec §4.2/§4.3), plus the `requires` prerequisites. Nothing consumes them yet — the
// TS tier and the junit-xml parser are later PRDs — so the contract carrying them is the
// whole of what is asserted here.
func TestLoadParsesTheStaticTierFields(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Report != "junit-xml" {
		t.Errorf("Report = %q, want %q", a.Report, "junit-xml")
	}
	if a.ReportPath != ".rtdd/junit.xml" {
		t.Errorf("ReportPath = %q, want %q", a.ReportPath, ".rtdd/junit.xml")
	}
	if a.IDTemplate != "{file}::{name}" {
		t.Errorf("IDTemplate = %q, want %q", a.IDTemplate, "{file}::{name}")
	}
	if a.Importscan == nil {
		t.Fatalf("Importscan = nil, want the declared scanner")
	}
	if a.Importscan.Command != "node {script}" || a.Importscan.Script != "scan-imports.mjs" {
		t.Errorf("Importscan = %+v, want {Command: \"node {script}\", Script: \"scan-imports.mjs\"}", *a.Importscan)
	}
	if len(a.Requires) != 1 {
		t.Fatalf("Requires = %+v, want exactly one entry", a.Requires)
	}
	if a.Requires[0].Bin != "node" || a.Requires[0].Reason == "" {
		t.Errorf("Requires[0] = %+v, want bin node with a non-empty reason", a.Requires[0])
	}
}

// test_for is tried in order, so its order is meaning rather than presentation: the first
// template that resolves to a real test file wins (spec §4.1 ranking). A map or a sorted
// slice would silently reorder the adapter author's intent.
func TestTestForPreservesDocumentOrder(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", staticYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"{dir}/{name}.test.ts", "{dir}/__tests__/{name}.test.ts", "tests/{name}.test.ts"}
	if len(a.TestFor) != len(want) {
		t.Fatalf("TestFor = %v, want %v", a.TestFor, want)
	}
	for i := range want {
		if a.TestFor[i] != want[i] {
			t.Errorf("TestFor[%d] = %q, want %q", i, a.TestFor[i], want[i])
		}
	}
}

// An omitted importscan means import ranking is skipped for this adapter. It is not an
// error and nothing is defaulted into place: a defaulted command would be a scanner the
// adapter author never declared, run against a language nobody said it could read.
func TestOmittedImportscanIsValidAndNotDefaulted(t *testing.T) {
	yaml := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
test_for: ["{dir}/{name}.test.ts"]
`
	a, err := Load(writeAdapter(t, t.TempDir(), "vitest.yaml", yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Importscan != nil {
		t.Errorf("Importscan = %+v, want nil for an adapter that declares none", *a.Importscan)
	}
}

// command and script are one declaration in two halves: a command with no script has
// nothing to run, and a script with no command has nothing to run it. Either alone is a
// configuration error (exit 2) that names the half that is missing.
func TestImportscanRequiresBothCommandAndScript(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
`
	cases := []struct{ name, yaml, want string }{
		{
			name: "command without script",
			yaml: base + "importscan:\n  command: \"node {script}\"\n",
			want: "importscan: script is required alongside command",
		},
		{
			name: "script without command",
			yaml: base + "importscan:\n  script: \"scan-imports.mjs\"\n",
			want: "importscan: command is required alongside script",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted a half-declared importscan block")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
			if !strings.Contains(err.Error(), p) {
				t.Errorf("error = %q, want it to name the source file %q", err.Error(), p)
			}
		})
	}
}

// A template placeholder the engine cannot substitute is a typo that would otherwise
// survive into a path or a selector as a literal brace. The message names the file, the
// field and the placeholder, so an agent reading exit 2 can fix it without reading Go.
func TestUnknownTemplatePlaceholdersAreRejected(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
`
	cases := []struct {
		name, yaml string
		want       []string
	}{
		{
			name: "id_template",
			yaml: base + "id_template: \"{file}::{testcase}\"\n",
			want: []string{"id_template", "{testcase}"},
		},
		{
			name: "test_for entry",
			yaml: base + "id_template: \"{file}::{name}\"\ntest_for:\n  - \"{dir}/{name}.test.ts\"\n  - \"{folder}/{name}.test.ts\"\n",
			want: []string{"test_for[1]", "{folder}"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := writeAdapter(t, t.TempDir(), "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load accepted an unrecognised template placeholder")
			}
			for _, want := range append(tc.want, p, "unknown placeholder") {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

// The placeholders the contract does recognise must all be accepted, or an adapter author
// discovers the vocabulary one exit 2 at a time.
func TestRecognisedTemplatePlaceholdersAreAccepted(t *testing.T) {
	yaml := `name: gradle
detect: ["build.gradle"]
subset: "gradle test {tests}"
selection: static
coverage: none
report: junit-xml
report_path: "build/test-results/test/"
id_template: "{classname}.{name}"
test_for:
  - "{dir}/{name}Test.java"
`
	if _, err := Load(writeAdapter(t, t.TempDir(), "gradle.yaml", yaml)); err != nil {
		t.Errorf("Load = %v, want nil for an adapter using only recognised placeholders", err)
	}
}

// A JUnit testcase is a (classname, name) pair. Without id_template there is nothing to
// render it back into the runner's own selector syntax, and without report_path there is
// nothing to read — audit A6 is why this is a contract rule and not an assumption.
func TestJUnitReportRequiresPathAndIDTemplate(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
`
	for _, tc := range []struct{ yaml, want string }{
		{base + "id_template: \"{file}::{name}\"\n", "report: junit-xml requires report_path"},
		{base + "report_path: \".rtdd/junit.xml\"\n", "report: junit-xml requires id_template"},
	} {
		_, err := Load(writeAdapter(t, t.TempDir(), "c.yaml", tc.yaml))
		if err == nil {
			t.Fatalf("Load accepted junit-xml without the field %q names", tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
		}
	}
}

// A prerequisite doctor cannot explain is not worth declaring: the reason is printed
// verbatim in the finding (spec §4.3, PRD #229 AC9).
func TestRequiresEntriesNeedABinAndAReason(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
test_for: ["{dir}/{name}.test.ts"]
`
	for _, tc := range []struct{ yaml, want string }{
		{base + "requires:\n  - reason: \"runs vitest\"\n", "requires[0]: bin is required"},
		{base + "requires:\n  - bin: node\n", "requires[0] (node): reason is required"},
	} {
		_, err := Load(writeAdapter(t, t.TempDir(), "d.yaml", tc.yaml))
		if err == nil {
			t.Fatalf("Load accepted a requires entry that %q rejects", tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
		}
	}
}

// Every v2 key is optional. A v1 adapter declaring none of them still loads, and picks up
// none of them by default — spec §4.1 makes byte-identical Python behaviour a regression
// requirement, and a defaulted report_path or test_for would quietly break that.
func TestTheStaticTierKeysAreAllOptional(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "demo.yaml", validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.ReportPath != "" || a.IDTemplate != "" || len(a.TestFor) != 0 || a.Importscan != nil || len(a.Requires) != 0 {
		t.Errorf("a v1 adapter picked up v2 fields it does not declare: %+v", a)
	}

	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatalf("Builtin has no python adapter")
	}
	if py.ReportPath != "" || py.IDTemplate != "" || len(py.TestFor) != 0 || py.Importscan != nil || len(py.Requires) != 0 {
		t.Errorf("adapters/python.yaml picked up v2 fields it does not declare: %+v", py)
	}
}

// Decision 3: a glob in report_path is rejected at LOAD time. The engine clears this path
// before every invocation, and "clear everything matching this pattern" in a host repo's
// build output is not a thing an adapter may ask for.
func TestValidateRejectsAGlobInReportPath(t *testing.T) {
	const globbed = `name: maven
detect: ["pom.xml"]
subset: "mvn -B test -Dtest={tests}"
selection: static
coverage: none
report: junit-xml
report_path: "target/surefire-reports/*.xml"
id_template: "{classname}#{name}"
`
	dir := t.TempDir()
	_, err := Load(writeAdapter(t, dir, "maven.yaml", globbed))
	if err == nil {
		t.Fatalf("Load = nil error for a globbed report_path")
	}
	for _, want := range []string{"report_path", "globs are not supported", `ending in "/"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
	// The directory form of the same declaration is legal.
	if _, err := Load(writeAdapter(t, dir, "maven-dir.yaml", strings.Replace(globbed,
		`report_path: "target/surefire-reports/*.xml"`,
		`report_path: "target/surefire-reports/"`, 1))); err != nil {
		t.Fatalf("Load(directory report_path): %v", err)
	}
	// So is the single-file form, with a ? and a [ nowhere in it.
	if _, err := Load(writeAdapter(t, dir, "maven-file.yaml", strings.Replace(globbed,
		`report_path: "target/surefire-reports/*.xml"`,
		`report_path: "target/surefire-reports/TEST-all.xml"`, 1))); err != nil {
		t.Fatalf("Load(single-file report_path): %v", err)
	}
}

// Decision 9: report_cmd is the one post-run command the argv-only engine needs, and it
// exists to produce the declared report_path. Both refusals are load-time (exit 2): a
// report_cmd under a report the runner writes itself has nothing to convert, and one that
// never names {report} writes its output where nothing will read it — which is exactly
// the empty report #294 forbids being read as "nothing failed".
func TestReportCmdRequiresJUnitXMLAndNamesTheReport(t *testing.T) {
	const base = `name: go
detect: ["go.mod"]
subset: "go test -json -run {tests} ./..."
list: "go test -list . ./..."
selection: static
coverage: none
`
	junit := base + `report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{name}"
`
	for _, tc := range []struct {
		name, yaml string
		want       []string
	}{
		{
			name: "without junit-xml",
			yaml: "name: demo\ndetect: [\"setup.py\"]\nseed: \"pytest\"\nsubset: \"pytest {tests}\"\nreport: pytest-reportlog\nreport_cmd: \"conv {log} {report}\"\n",
			want: []string{"report_cmd", "junit-xml"},
		},
		{
			name: "naming no {report}",
			yaml: junit + "report_cmd: \"go-junit-report -in {log}\"\n",
			want: []string{"report_cmd", "{report}"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeAdapter(t, t.TempDir(), "go.yaml", tc.yaml))
			if err == nil {
				t.Fatalf("Load accepted report_cmd %s", tc.name)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not carry %q", err, want)
				}
			}
		})
	}

	a, err := Load(writeAdapter(t, t.TempDir(), "go.yaml", junit+"report_cmd: \"go-junit-report -parser gojson -in {log} -out {report}\"\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.ReportCmd != "go-junit-report -parser gojson -in {log} -out {report}" {
		t.Errorf("ReportCmd = %q, want the declared command", a.ReportCmd)
	}
}

// report_cmd is optional: an adapter that declares none is a valid adapter, and the
// pytest path must not acquire a post-run command it never asked for.
func TestReportCmdIsOptional(t *testing.T) {
	a, err := Load(writeAdapter(t, t.TempDir(), "demo.yaml", validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.ReportCmd != "" {
		t.Errorf("ReportCmd = %q, want empty for an adapter that declares none", a.ReportCmd)
	}
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if py := byName(all, "python"); py == nil || py.ReportCmd != "" {
		t.Errorf("adapters/python.yaml picked up a report_cmd it does not declare: %+v", py)
	}
}
