# RTDD M6c — `junit-xml`, the Universal Outcome Format

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach RTDD to read test outcomes from JUnit XML — the one report format essentially every runner in every ecosystem emits — and to render a parsed `<testcase>` back into the runner's own selector syntax so a subset run is invocable. After this milestone an adapter declaring `report: junit-xml`, `report_path:` and `id_template:` **runs**: the runner clears the report path, invokes the subset command, parses what the runner wrote, maps the exit code exactly as it does today, and hands `internal/runner` the same `[]report.Outcome` the pytest path produces. No tier, no ranking and no shipped adapter YAML is built here — M6b owns the `TS` tier and M6d owns the adapter set.

**Architecture:** `internal/report` gains a second parser beside `ReadReportLog`, never inside it: `junit.go` (the document model, `ReadJUnitFile`, the four named failure modes), `id.go` (`RenderID`/`ParseID` — the `id_template` round-trip that makes a parsed case invocable), and `reportpath.go` (`ReportPath`: an adapter's `report_path` resolved against the repo root as exactly one file or one directory of `*.xml`, plus `Clear` and `Files`). `internal/runner`'s `execute` gains a single dispatch on `a.Report` where it today calls `report.ReadReportLog` unconditionally, a `{report}` entry in the placeholder map for adapters that declare `report_path`, a clear-before-invocation next to the existing stale-`.coverage` removal, and a skip of the sqlite read under `coverage: none`. `internal/adapter` gains exactly one new rule — `report_path` may not contain a glob metacharacter — and nothing else: the declarative half of the contract landed in M6a. D8 holds throughout: the YAML declares, Go executes.

**Tech Stack:** Go 1.24+, `encoding/xml` (stdlib), stdlib `testing`. No new dependency — `TestGoModRequiresExactlyYAMLAndSQLite` in `internal/contract` fails if one is added.

**Spec:** `docs/specs/2026-09-05-multi-language.md` §4.3 (`report: junit-xml` — the universal outcome format), with one rule from §4.2 (the `report_path` shape). Each task below names the section it discharges on its `**Discharges:**` line, and every task also names the PRD #231 acceptance criteria it closes, so the mapping from plan to PRD is checkable rather than re-derived. **The declarative half of §4.3 already shipped in M6a** (#237, #238, #250) and is out of scope here: `report: junit-xml` validates today, it already requires both `report_path` and `id_template`, `id_template`'s placeholder vocabulary is already `{file}`/`{classname}`/`{name}` and already rejected at load time, `requires` is already declared and already reported by `doctor` and `init`, and `adapters/python.yaml` is already byte-frozen by `internal/contract`. This plan owns only the executing half — the parser, the id renderer, the fixtures and the runner wiring.

## Global Constraints

*(the first five are copied verbatim from `00-interfaces.md`)*

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except `modernc.org/sqlite` and
  `gopkg.in/yaml.v3`. This milestone adds none: JUnit XML is read with `encoding/xml`.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc. `report_path` is declared
  repo-relative and is resolved to an absolute path exactly once, in `NewReportPath`.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

M6c-specific:

- **`internal/report`'s existing `pytest-reportlog` path is untouched.** The two parsers
  sit side by side in the same package and are chosen by the adapter's `report:` field, at
  the one place `internal/runner` reads outcomes. No task lists `internal/report/reportlog.go`
  in its **Files**, no task changes `Outcome`, and no task "generalises" the two parsers
  into one: the JSONL path is phase-accumulating and the XML path is not, and a shared
  abstraction over them would be a rewrite of the only parser that has ever produced a map.
- **`adapters/python.yaml` stays byte-frozen**, as it was for M6a. Nothing in this
  milestone gives the Python adapter a reason to change, so `internal/contract`'s sha256
  digest test needs no edit — and a task that makes it need one has gone out of scope.
- **A JUnit report is never allowed to be silently empty.** Every failure mode in this
  milestone — a missing file, an empty file, malformed XML, a suite-level failure with no
  `<testcase>`, a `{file}` the runner never emitted — is a *named* error. Zero tests parsed
  from a report that should have carried some is the exact shape of a false green: the run
  exits 0, nothing failed, and nothing ran.
- **Exit codes are unchanged** (`00-interfaces.md`): the adapter's `exit_codes` map is read
  before the report is, on the junit path exactly as on the pytest path, so a
  `bad-selector` (4) or `no-tests-collected` (5) is still a `FatalExitError` and never a
  parse failure against a file the runner declined to write.
- **The shipped adapter set is not built here.** `adapters/` gains no file. Every adapter in
  every test below is constructed in the test or written to a `t.TempDir()`.

### The four under-determined decisions, resolved here

PRD #231 and spec §4.3 leave four things open. They are decided here so no implementer has
to guess, and each is pinned to a task that tests it.

**1. The placeholder spelling is `{classname}`, and the PRD's `{class}` means the same key**
(Task 3). PRD #231 AC3 says `id_template` expands `{file}`, `{class}` and `{name}`. The
shipped vocabulary in `internal/adapter` is `{file}`, `{classname}`, `{name}`, and
`validateTemplates` rejects anything else at load time with `id_template %q: unknown
placeholder {class}`. `{classname}` wins for two reasons: it is the *shipped* spelling, so
changing it moves a validated vocabulary, `docs/plans/06-m6a-adapter-contract-v2.md` and
spec §4.3 in one commit for no behavioural gain; and it is the literal name of the JUnit
attribute it renders (`<testcase classname=…>`), so an adapter author reading their runner's
XML types what they see. `{class}` is therefore **not** added as an alias: an alias would
make two spellings valid, and the load-time rejection an author already gets is a better
teacher than a silent second name. Task 3 pins this with a test asserting that
`{classname}` renders and that `{class}` is still rejected by the existing loader.

**2. A `{file}` the runner did not emit is a named error, never a derived value** (Task 3,
pinned to the Maven Surefire fixture in Task 4). `<testcase file=…>` is optional: pytest,
Vitest, RSpec and PHPUnit emit it; Maven Surefire, `go-junit-report` and `jest-junit` (whose
`addFileAttribute` is off by default) do not. Deriving it — from `classname`, from the
`<testsuite name=>`, from `hostname` — requires knowing the host repo's source roots and
naming convention, which RTDD does not know: `com.example.CalculatorTest` is
`src/test/java/com/example/CalculatorTest.java` in one layout and something else in the
next. A derived path that is wrong does not fail loudly; it renders an id the runner does
not recognise, the subset run selects nothing, and RTDD reports a green subset that
executed no tests. So `RenderID` returns `ErrNoFileAttr`, naming the adapter's template, the
suite and the case, and the fix it names is the adapter's: *this runner does not emit
`file=`; use an `id_template` that does not name `{file}`*. The Surefire fixture, which
genuinely lacks the attribute, is the test.

**3. `report_path` names exactly one file, or exactly one directory — never a glob**
(Tasks 5 and 6). §4.3's own table has Surefire and Gradle writing a *directory* of XML
files while every flag-driven runner writes one, so one shape is not enough. A glob is the
third option and is rejected: a glob cannot be cleared without deciding what it may delete,
`paths.MatchGlob` is a classification matcher rather than a filesystem walker, and
`report_path: "target/surefire-reports/*.xml"` would have RTDD removing files by pattern in
the host repo's build output. The declaration is therefore read from the string itself,
never sniffed from disk: **a `report_path` ending in `/` is a directory, anything else is
one file**, and a `report_path` containing `*`, `?` or `[` is rejected at load time (exit 2)
with `report_path %q: globs are not supported; name one file, or a directory ending in "/"`.
"Clear the path before invocation" then means: for a file, remove it (a missing file is not
an error); for a directory, create it if absent and remove every `*.xml` **directly inside
it**, leaving the directory and every non-XML sibling — Surefire's `*.txt` dumps — alone.
Reading a directory reads its `*.xml` children in sorted-name order and concatenates the
cases; sorted rather than readdir order so a merged report is reproducible.

**4. Chunked runs merge in `execute`, per chunk, exactly where the report-log read already
happens** (Task 7). `internal/runner` chunks a subset across several invocations and gives
each chunk its own `{log}` under a temp dir, but `report_path` is a fixed, adapter-declared
path, so chunk *i+1* overwrites chunk *i*'s report. The read therefore stays where it is —
immediately after the invocation, before the next chunk runs — which is the same rule the
`.coverage` read already follows and for the same reason: chunk *i*'s output exists nowhere
else once chunk *i+1* starts. The existing `byTest` de-duplication carries over unchanged,
because the junit path produces `report.Outcome` values whose `Test` is the *rendered* id,
in the same namespace as the ids that were spliced into the command — so "last invocation
wins" keeps meaning what it means today. The clear happens per chunk too, immediately
before each invocation, next to the existing stale-`.coverage` removal: clearing once per
`Run` would let chunk *i*'s cases be re-read as chunk *i+1*'s if chunk *i+1* crashed before
writing.

## What is already true in the tree

Read these before writing code; several tasks are smaller than they look because the
mechanism already exists.

- `internal/adapter/adapter.go` — `report: junit-xml` already validates, and already
  requires both `report_path` and `id_template` (the M6a rules
  `report: junit-xml requires report_path` / `… requires id_template`). `idTemplatePlaceholders`
  is already `{file}`, `{classname}`, `{name}`, enforced by `validateTemplates` at load
  time. Task 5 adds exactly one rule to this file; nothing else in it changes.
- `internal/adapter/expand.go` — `Expand`/`ExpandTests` resolve **whatever keys the vars
  map holds** and reject any placeholder they cannot resolve. The caller's map is the
  vocabulary, so adding `{report}` is a one-line change at the call site, and an adapter
  naming `{report}` without declaring `report_path` fails as an unresolved placeholder
  rather than receiving an empty string.
- `internal/runner/run.go` — `execute` already removes the stale `.coverage` before each
  invocation, already reads `a.ExitCodes` *before* touching the report, already de-duplicates
  outcomes by test id with "last invocation wins", and already calls
  `report.ReadReportLog(logPath)` at exactly one place. That call site is the whole of the
  dispatch this milestone adds.
- `internal/report/reportlog.go` — `Outcome{Test, Status, DurationMS}` and the status
  vocabulary `"pass" | "fail" | "skip" | "error"` are already the map's wire format. The
  JUnit parser produces the same vocabulary; it does not invent a second one.
- `internal/runner/run_test.go` — `stubAdapter` already fakes a runner with a POSIX `sh`
  script (`stubpytest`) driven by `RTDD_STUB_*` environment variables, and already skips on
  Windows and on a temp dir containing whitespace. The junit tests fake their runner the
  same way rather than inventing a second harness.
- `internal/contract/adapter_freeze_test.go` — the sha256 freeze on `adapters/python.yaml`.
  Task 8 asserts it still passes unedited; it does not update the digest.

## File Structure

| File | Single responsibility |
|---|---|
| `internal/report/junit.go` | **NEW** — the JUnit document model, `ReadJUnitFile`, suite flattening, and the four named failure modes |
| `internal/report/junit_test.go` | **NEW** — flattening, the outcome vocabulary, durations, and each named error |
| `internal/report/id.go` | **NEW** — `RenderID` and `ParseID`: the `id_template` round-trip |
| `internal/report/id_test.go` | **NEW** — `{classname}` renders, `{class}` never reaches here, a missing `file=` is named, adjacent placeholders are rejected |
| `internal/report/reportpath.go` | **NEW** — `ReportPath`: one file or one directory, `Files`, `Clear`, `ReadJUnitReport` |
| `internal/report/reportpath_test.go` | **NEW** — both shapes, sorted merge order, the stale-report clear |
| `internal/report/testdata/junit/` | **NEW** — six real captured reports plus `PROVENANCE.md` |
| `internal/report/junit_fixtures_test.go` | **NEW** — the six-fixture table and the parse → render → parse round-trip |
| `internal/adapter/adapter.go` | **MODIFIED** — one new rule: `report_path` may not contain a glob metacharacter |
| `internal/adapter/v2_test.go` | **MODIFIED** — the rejection above |
| `internal/runner/run.go` | **MODIFIED** — `{report}`, clear-before-invocation, the `a.Report` dispatch, and the `coverage: none` skip |
| `internal/runner/junit_test.go` | **NEW** — the junit path end to end against a stub runner, including chunking and exit-code mapping |
| `docs/plans/00-interfaces.md` | **MODIFIED** — record every addition made here |
| `internal/contract/contract_test.go` | **MODIFIED** — the contract records the second parser; the Python adapter is still frozen |

---

## Task 1 — `internal/report`: `ReadJUnitFile` parses one report

**Discharges:** spec §4.3 (one parser covers all ten runners), PRD #231 AC1 (a `junit-xml` parser in `internal/report` beside the untouched report-log one) and AC2 (the outcome vocabulary), and the flattening half of AC5.

**Files:** `internal/report/junit.go`, `internal/report/junit_test.go`

**Interfaces:**

*Consumes:* nothing but `encoding/xml` and `os`. `internal/report`'s existing `Outcome` is
produced by Task 5's reader, not by this one: this task stops at the parsed case, because
an `Outcome.Test` cannot exist before `id_template` has rendered one.

*Produces:*
```go
// JUnitCase is one <testcase> as the runner wrote it, before any id rendering. It keeps
// the attributes id_template may name plus the enclosing suite, which every error message
// in this package uses to say WHICH case it is talking about.
type JUnitCase struct {
	Suite      string // the innermost enclosing <testsuite name=>
	Classname  string // the classname= attribute; "" when absent
	Name       string // the name= attribute
	File       string // the file= attribute; "" when the runner does not emit one
	Status     string // "pass" | "fail" | "skip" | "error" — report.Outcome's vocabulary
	DurationMS int    // from time=, which JUnit writes in SECONDS
}

// ReadJUnitFile parses one JUnit XML file into its cases, in document order.
func ReadJUnitFile(path string) ([]JUnitCase, error)
```

- [ ] **Step 1: Write the failing test**

`internal/report/junit_test.go`:

```go
package report

import (
	"os"
	"path/filepath"
	"testing"
)

// writeXML materialises one report file and returns its path.
func writeXML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "junit.xml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// nestedXML is the shape Maven Surefire, Gradle and jest-junit all produce: a
// <testsuites> root wrapping one or more <testsuite>, and — the case a flat reader gets
// wrong — a <testsuite> nested inside another <testsuite>.
const nestedXML = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="all" tests="4" failures="1" errors="1" skipped="1" time="0.75">
  <testsuite name="outer" tests="2" time="0.5">
    <testcase classname="outer.Alpha" name="passes" file="tests/alpha.ts" time="0.25"/>
    <testsuite name="inner" tests="1" time="0.25">
      <testcase classname="inner.Beta" name="fails" file="tests/beta.ts" time="0.25">
        <failure message="expected 1 to be 2" type="AssertionError">at beta.ts:7</failure>
      </testcase>
    </testsuite>
  </testsuite>
  <testsuite name="second" tests="2" time="0.25">
    <testcase classname="second.Gamma" name="errors" time="0.2">
      <error message="boom" type="RuntimeError">stack</error>
    </testcase>
    <testcase classname="second.Delta" name="skipped">
      <skipped message="not on this platform"/>
    </testcase>
  </testsuite>
</testsuites>
`

func TestReadJUnitFileFlattensNestedSuitesInDocumentOrder(t *testing.T) {
	cases, err := ReadJUnitFile(writeXML(t, nestedXML))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	want := []JUnitCase{
		{Suite: "outer", Classname: "outer.Alpha", Name: "passes", File: "tests/alpha.ts", Status: "pass", DurationMS: 250},
		{Suite: "inner", Classname: "inner.Beta", Name: "fails", File: "tests/beta.ts", Status: "fail", DurationMS: 250},
		{Suite: "second", Classname: "second.Gamma", Name: "errors", Status: "error", DurationMS: 200},
		{Suite: "second", Classname: "second.Delta", Name: "skipped", Status: "skip"},
	}
	if len(cases) != len(want) {
		t.Fatalf("got %d cases, want %d: %+v", len(cases), len(want), cases)
	}
	for i := range want {
		if cases[i] != want[i] {
			t.Errorf("case %d = %+v, want %+v", i, cases[i], want[i])
		}
	}
}

// PRD #231 AC2: an <error> is a failure, not a skip. Both statuses already mean "this test
// did not pass" to internal/runner, which is why they must never collapse into "skip".
func TestReadJUnitFileMapsErrorToErrorAndNotSkip(t *testing.T) {
	cases, err := ReadJUnitFile(writeXML(t, nestedXML))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	for _, c := range cases {
		if c.Name == "errors" && c.Status != "error" {
			t.Errorf("<error> case mapped to %q, want %q", c.Status, "error")
		}
		if c.Name == "skipped" && c.Status != "skip" {
			t.Errorf("<skipped> case mapped to %q, want %q", c.Status, "skip")
		}
	}
}

// A single <testsuite> root with no <testsuites> wrapper is what pytest, PHPUnit and
// RSpec write. It is not a special case in the XML; it must not be one in the reader.
func TestReadJUnitFileAcceptsABareTestsuiteRoot(t *testing.T) {
	const bare = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="only" tests="1" time="0.01">
  <testcase classname="only.One" name="works" time="0.01"/>
</testsuite>
`
	cases, err := ReadJUnitFile(writeXML(t, bare))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Status != "pass" || cases[0].DurationMS != 10 {
		t.Fatalf("got %+v, want one passing case of 10ms", cases)
	}
}

// A testcase with no time= is not a zero-length test that ran; it is a runner that did not
// say. Both render as 0, and the map has always carried 0 for "unknown" — but a MISSING
// attribute must never become a parse error, because three of the six shipped fixtures
// omit it on skipped cases.
func TestReadJUnitFileTreatsAMissingTimeAsZero(t *testing.T) {
	const noTime = `<testsuite name="s"><testcase classname="s.C" name="n"/></testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, noTime))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].DurationMS != 0 {
		t.Fatalf("got %+v, want one case with DurationMS 0", cases)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestReadJUnitFile'
```

Expected: `[build failed]` with `undefined: ReadJUnitFile` and `undefined: JUnitCase`. Not an
assertion failure — neither the type nor the function exists yet.

- [ ] **Step 3: Write minimal implementation**

`internal/report/junit.go`. Decode with `encoding/xml` into a recursive suite struct — a
`<testsuite>` may contain both `<testcase>` and further `<testsuite>` children — then walk
it depth-first so document order is preserved:

```go
// Package-level note: this file is the SECOND parser in internal/report. The
// pytest-reportlog path in reportlog.go is not touched by it and is not refactored into a
// shared abstraction: one accumulates phases per test id, the other reads a tree, and the
// only thing they share is the Outcome they eventually produce.

type junitSuite struct {
	Name   string       `xml:"name,attr"`
	Suites []junitSuite `xml:"testsuite"`
	Cases  []junitCase  `xml:"testcase"`
	// Failure is a suite-level <failure> — a suite that could not run at all. Task 2 uses it.
	Failure *junitDetail `xml:"failure"`
	Error   *junitDetail `xml:"error"`
}

type junitCase struct {
	Classname string       `xml:"classname,attr"`
	Name      string       `xml:"name,attr"`
	File      string       `xml:"file,attr"`
	Time      string       `xml:"time,attr"` // string: an absent attribute must be 0, not an error
	Failure   *junitDetail `xml:"failure"`
	Error     *junitDetail `xml:"error"`
	Skipped   *junitDetail `xml:"skipped"`
}

type junitDetail struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}
```

The root is either `<testsuites>` (a `junitSuites` wrapper) or `<testsuite>`; decide by the
first `xml.StartElement` token rather than by trying one and falling back, so a malformed
document produces Task 2's error and not a mislabelled one. Status precedence is
`error` → `fail` → `skip` → `pass`: a case carrying both `<error>` and `<skipped>` did not
pass, and the vocabulary is the one `reportlog.go` already produces. `time` is seconds and
is converted with `strconv.ParseFloat` + `math.Round(sec*1000)`, exactly as `finalize` does;
an unparseable `time` is 0, not an error, because a duration is telemetry and an outcome is
not.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -run 'TestReadJUnitFile' -v && go test ./internal/report/
```

Expected: four `--- PASS` lines, then `ok` for the whole package with every existing
report-log test unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/report/junit.go internal/report/junit_test.go
git commit -m "report: junit-xml document model and ReadJUnitFile"
```

---

## Task 2 — `internal/report`: the four named failure modes

**Discharges:** spec §4.3 (an unmet prerequisite must never surface as a mid-run parse failure against a report file that was never written — so the failure that *does* surface must name itself), PRD #231 AC5 (a suite-level `<failure>` with no `<testcase>` is surfaced, not dropped) and AC6 (missing, empty and malformed each produce a distinct named error).

**Files:** `internal/report/junit.go`, `internal/report/junit_test.go`

**Interfaces:**

*Consumes:* `ReadJUnitFile` (Task 1).

*Produces:*
```go
// The four ways a JUnit report can fail to be one. Each is a sentinel so a caller can
// distinguish them with errors.Is, and each is WRAPPED with the offending path so a human
// reading exit 1 knows which file to open.
var (
	// ErrNoReport is a report_path that does not exist after the runner ran. It usually
	// means the adapter's `requires` was unmet — the reporter package was never installed —
	// which doctor and init already warn about (spec §4.3, M6a).
	ErrNoReport = errors.New("junit-xml: no report file")
	// ErrEmptyReport is a zero-byte report: the runner created the file and wrote nothing.
	ErrEmptyReport = errors.New("junit-xml: empty report file")
	// ErrMalformedReport is XML that does not parse, or a root element that is neither
	// <testsuites> nor <testsuite>.
	ErrMalformedReport = errors.New("junit-xml: malformed report")
	// ErrSuiteFailure is a <testsuite> carrying a suite-level <failure> or <error> and no
	// <testcase>: a suite that could not run at all. Reporting zero tests from it would be
	// a false green.
	ErrSuiteFailure = errors.New("junit-xml: suite failed before any test ran")
)
```

- [ ] **Step 1: Write the failing test**

Append to `internal/report/junit_test.go`:

```go
package report

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadJUnitFileNamesEachWayAReportCanBeUnreadable(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.xml")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	blank := filepath.Join(dir, "blank.xml")
	if err := os.WriteFile(blank, []byte("   \n\t\n"), 0o644); err != nil {
		t.Fatalf("write blank: %v", err)
	}
	torn := filepath.Join(dir, "torn.xml")
	if err := os.WriteFile(torn, []byte(`<testsuite name="s"><testcase name="a"`), 0o644); err != nil {
		t.Fatalf("write torn: %v", err)
	}
	wrongRoot := filepath.Join(dir, "wrong.xml")
	if err := os.WriteFile(wrongRoot, []byte(`<?xml version="1.0"?><results><ok/></results>`), 0o644); err != nil {
		t.Fatalf("write wrongRoot: %v", err)
	}

	cases := []struct {
		name string
		path string
		want error
	}{
		{"missing file", filepath.Join(dir, "absent.xml"), ErrNoReport},
		{"zero bytes", empty, ErrEmptyReport},
		{"whitespace only", blank, ErrEmptyReport},
		{"truncated xml", torn, ErrMalformedReport},
		{"root is not a junit root", wrongRoot, ErrMalformedReport},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadJUnitFile(tc.path)
			if err == nil {
				t.Fatalf("ReadJUnitFile(%s) = nil error; a report that is not a report must never parse as zero tests", tc.path)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ReadJUnitFile(%s) error = %v, want errors.Is(_, %v)", tc.path, err, tc.want)
			}
			if !strings.Contains(err.Error(), filepath.Base(tc.path)) {
				t.Errorf("error %q does not name the offending file", err)
			}
		})
	}
}

// PRD #231 AC5: a suite-level <failure> with no <testcase> is the "the whole file blew up
// at import time" report. Zero cases plus exit 0 would be a false green.
func TestReadJUnitFileSurfacesASuiteLevelFailureWithNoTestcase(t *testing.T) {
	const suiteFailed = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="tests/broken.ts" tests="0" failures="1">
    <failure message="Cannot find module &#39;./missing&#39;" type="Error">at broken.ts:1</failure>
  </testsuite>
</testsuites>
`
	_, err := ReadJUnitFile(writeXML(t, suiteFailed))
	if err == nil {
		t.Fatalf("ReadJUnitFile = nil error for a suite that failed before any test ran")
	}
	if !errors.Is(err, ErrSuiteFailure) {
		t.Fatalf("error = %v, want errors.Is(_, ErrSuiteFailure)", err)
	}
	for _, want := range []string{"tests/broken.ts", "Cannot find module"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q — the suite name and the runner's own message are the only diagnosis available", err, want)
		}
	}
}

// A suite-level <failure> ALONGSIDE testcases is a summary, not a catastrophe: Surefire
// writes one when a test failed, and the cases carry the detail. It must not error.
func TestReadJUnitFileKeepsASuiteFailureThatAlsoHasTestcases(t *testing.T) {
	const both = `<testsuite name="s" tests="1" failures="1">
  <failure message="1 test failed"/>
  <testcase classname="s.C" name="n" time="0.01"><failure message="nope"/></testcase>
</testsuite>`
	cases, err := ReadJUnitFile(writeXML(t, both))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) != 1 || cases[0].Status != "fail" {
		t.Fatalf("got %+v, want one failing case", cases)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestReadJUnitFileNamesEachWay|TestReadJUnitFileSurfaces|TestReadJUnitFileKeeps'
```

Expected: `[build failed]` with `undefined: ErrNoReport`, `undefined: ErrEmptyReport`,
`undefined: ErrMalformedReport`, `undefined: ErrSuiteFailure`. After the sentinels exist but
before the checks do, the remaining red is
`ReadJUnitFile(...) = nil error; a report that is not a report must never parse as zero tests`
for the empty and whitespace-only cases, and `error = <nil>` for the suite-level failure —
`encoding/xml` accepts an empty document and a `<results>` root by returning nothing.

- [ ] **Step 3: Write minimal implementation**

In `internal/report/junit.go`: `os.ReadFile`, mapping `os.IsNotExist` to `ErrNoReport`;
`bytes.TrimSpace(b)` of length 0 to `ErrEmptyReport`; a decode error, or a first
`StartElement` whose name is neither `testsuites` nor `testsuite`, to `ErrMalformedReport`.
Each is wrapped as `fmt.Errorf("report: %w: %s", ErrX, path)` so `errors.Is` matches and the
path is printed. During the depth-first walk, a suite with `len(Cases) == 0 &&
len(Suites) == 0` and a non-nil `Failure` or `Error` returns
`fmt.Errorf("report: %w: %s: %s: %s", ErrSuiteFailure, path, suite.Name, detail.Message)`.
A suite that is merely empty — no cases, no failure — is not an error: a runner may write
an empty `<testsuite>` for a file it collected and skipped entirely.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -run 'TestReadJUnitFile' -v && go test ./internal/report/
```

Expected: every `TestReadJUnitFile*` subtest `--- PASS`, and the package green — the
report-log tests are untouched.

- [ ] **Step 5: Commit**

```bash
git add internal/report/junit.go internal/report/junit_test.go
git commit -m "report: junit-xml names its four failure modes instead of parsing zero tests"
```

---

## Task 3 — `internal/report`: `RenderID` and `ParseID`, the `id_template` round-trip

**Discharges:** spec §4.3 (id round-tripping — "a JUnit `<testcase classname= name=>` pair must render back into something the runner's own selector syntax accepts", audit finding A6) and PRD #231 AC3. Pins under-determined decisions 1 (`{classname}` is the spelling) and 2 (a missing `file=` is a named error).

**Files:** `internal/report/id.go`, `internal/report/id_test.go`

**Interfaces:**

*Consumes:* `JUnitCase` (Task 1). Deliberately **not** `internal/adapter`: the template
arrives as a string, so `internal/report` keeps its single dependency direction and the
adapter package stays free of parsing.

*Produces:*
```go
// RenderID renders one parsed case into the runner's own selector syntax, expanding
// {file}, {classname} and {name} — the vocabulary internal/adapter's validateTemplates
// already enforces at load time. The result is ONE argv token: ExpandTests splices each id
// as its own argument, so a template that renders a space produces one argument containing
// a space, which is what the runner then receives.
func RenderID(tmpl string, c JUnitCase) (string, error)

// ParseID is RenderID's inverse: it reads a rendered id back into the fields the template
// names, by splitting on the template's literal segments. It is what makes "parse → render
// → parse is stable" an assertion rather than a hope (PRD #231 AC3).
func ParseID(tmpl, id string) (JUnitCase, error)

var (
	// ErrNoFileAttr is a template naming {file} against a runner that does not emit the
	// attribute — Maven Surefire, go-junit-report and jest-junit by default. Deriving the
	// path would produce an id the runner does not recognise, so the subset would select
	// nothing and report green (decision 2).
	ErrNoFileAttr = errors.New("junit-xml: testcase has no file attribute")
	// ErrAmbiguousTemplate is an id_template whose placeholders are adjacent, e.g.
	// "{classname}{name}": it renders, but nothing can read it back, so the round-trip
	// PRD #231 AC3 requires cannot hold.
	ErrAmbiguousTemplate = errors.New("junit-xml: id_template placeholders need a literal separator")
)
```

- [ ] **Step 1: Write the failing test**

`internal/report/id_test.go`:

```go
package report

import (
	"errors"
	"strings"
	"testing"
)

// Decision 1: {classname} is the spelling. The PRD's prose says {class}; the shipped
// vocabulary in internal/adapter is {classname}, validateTemplates rejects everything
// else at load time, and this package renders exactly what that loader admits. {class} is
// NOT an alias — two spellings for one key is worse than one rejection with a message.
func TestRenderIDExpandsTheShippedPlaceholderVocabulary(t *testing.T) {
	c := JUnitCase{
		Suite:     "tests/math.test.ts",
		Classname: "math > add",
		Name:      "adds two numbers",
		File:      "tests/math.test.ts",
	}
	cases := []struct {
		tmpl string
		want string
	}{
		{"{file}::{name}", "tests/math.test.ts::adds two numbers"},
		{"{classname}#{name}", "math > add#adds two numbers"},
		{"{file}", "tests/math.test.ts"},
		{"{name}", "adds two numbers"},
	}
	for _, tc := range cases {
		got, err := RenderID(tc.tmpl, c)
		if err != nil {
			t.Fatalf("RenderID(%q): %v", tc.tmpl, err)
		}
		if got != tc.want {
			t.Errorf("RenderID(%q) = %q, want %q", tc.tmpl, got, tc.want)
		}
	}
}

// Decision 2: a runner that does not emit file= gets a named error, never a derived path.
// A derived path that is wrong renders an id the runner does not recognise, the subset
// selects nothing, and the run reports green having executed no tests.
func TestRenderIDRefusesToInventAMissingFileAttribute(t *testing.T) {
	surefire := JUnitCase{
		Suite:     "com.example.CalculatorTest",
		Classname: "com.example.CalculatorTest",
		Name:      "addsTwoNumbers",
	}
	_, err := RenderID("{file}::{name}", surefire)
	if err == nil {
		t.Fatalf("RenderID = nil error; a {file} the runner never wrote must not be derived")
	}
	if !errors.Is(err, ErrNoFileAttr) {
		t.Fatalf("error = %v, want errors.Is(_, ErrNoFileAttr)", err)
	}
	for _, want := range []string{"com.example.CalculatorTest", "addsTwoNumbers", "{file}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q — the fix is the adapter's id_template, so the message must name the case and the placeholder", err, want)
		}
	}
	// The same case renders fine under a template that does not name {file}: this is an
	// adapter-authoring error, not an unreadable report.
	got, err := RenderID("{classname}#{name}", surefire)
	if err != nil {
		t.Fatalf("RenderID without {file}: %v", err)
	}
	if got != "com.example.CalculatorTest#addsTwoNumbers" {
		t.Errorf("RenderID = %q, want %q", got, "com.example.CalculatorTest#addsTwoNumbers")
	}
}

// parse → render → parse must be stable, which requires the template to be invertible.
func TestParseIDInvertsRenderID(t *testing.T) {
	c := JUnitCase{Classname: "com.example.CalculatorTest", Name: "addsTwoNumbers", File: "src/calc_test.go"}
	for _, tmpl := range []string{"{file}::{name}", "{classname}#{name}", "{file}::{classname}::{name}"} {
		id, err := RenderID(tmpl, c)
		if err != nil {
			t.Fatalf("RenderID(%q): %v", tmpl, err)
		}
		back, err := ParseID(tmpl, id)
		if err != nil {
			t.Fatalf("ParseID(%q, %q): %v", tmpl, id, err)
		}
		again, err := RenderID(tmpl, back)
		if err != nil {
			t.Fatalf("RenderID after ParseID (%q): %v", tmpl, err)
		}
		if again != id {
			t.Errorf("round trip through %q: %q -> %q", tmpl, id, again)
		}
	}
}

// A template whose placeholders touch renders something no reader can split again, so the
// round-trip AC3 asks for cannot hold. It is rejected rather than silently one-way.
func TestParseIDRejectsAdjacentPlaceholders(t *testing.T) {
	_, err := ParseID("{classname}{name}", "com.example.CalculatorTestaddsTwoNumbers")
	if !errors.Is(err, ErrAmbiguousTemplate) {
		t.Fatalf("ParseID error = %v, want errors.Is(_, ErrAmbiguousTemplate)", err)
	}
	if _, err := RenderID("{classname}{name}", JUnitCase{Classname: "a", Name: "b"}); !errors.Is(err, ErrAmbiguousTemplate) {
		t.Fatalf("RenderID error = %v, want errors.Is(_, ErrAmbiguousTemplate); a template that cannot round-trip must fail on the way out too", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestRenderID|TestParseID'
```

Expected: `[build failed]` with `undefined: RenderID`, `undefined: ParseID`,
`undefined: ErrNoFileAttr`, `undefined: ErrAmbiguousTemplate`.

- [ ] **Step 3: Write minimal implementation**

`internal/report/id.go`. Split the template once into an alternating list of literals and
placeholders (`{file}`, `{classname}`, `{name}` only — anything else is
`ErrAmbiguousTemplate`'s sibling `fmt.Errorf("unknown placeholder %s", ph)`, which
`internal/adapter` already prevents reaching here). Two adjacent placeholders, or a
placeholder at the very start immediately followed by another, is `ErrAmbiguousTemplate`;
check it in the shared splitter so `RenderID` and `ParseID` reject the same templates.
`RenderID` substitutes, returning `ErrNoFileAttr` — wrapped with the suite, the case name
and the literal `{file}` — when the template names `{file}` and `c.File == ""`. A missing
`classname` is *not* an error: plenty of runners omit it on a top-level test, and an empty
string is a truthful rendering of what the report said. `ParseID` walks the literal segments
left to right with `strings.Index`, assigning each gap to its placeholder's field; a literal
that does not appear, or appears out of order, is
`fmt.Errorf("report: id %q does not match id_template %q", id, tmpl)`.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -run 'TestRenderID|TestParseID' -v && go test ./internal/report/ ./internal/adapter/
```

Expected: four `--- PASS` lines, then both packages `ok`. `internal/adapter` is run because
this task depends on its `idTemplatePlaceholders` vocabulary and must not diverge from it —
if it fails here, the vocabulary moved and decision 1 has been broken.

- [ ] **Step 5: Commit**

```bash
git add internal/report/id.go internal/report/id_test.go
git commit -m "report: id_template rendering and its inverse, with {classname} as the one spelling"
```

---

## Task 4 — `internal/report/testdata/junit`: six real captured reports

**Discharges:** spec §4.3 ("one parser in `internal/report` covers all ten" — a claim only real output can support) and PRD #231 AC3 (the round-trip asserted over all six) and AC4 (real captured output from six named runners; hand-written XML is not acceptable for these six).

**Files:** `internal/report/testdata/junit/vitest.xml`, `internal/report/testdata/junit/jest.xml`, `internal/report/testdata/junit/gojunit.xml`, `internal/report/testdata/junit/surefire.xml`, `internal/report/testdata/junit/rspec.xml`, `internal/report/testdata/junit/phpunit.xml`, `internal/report/testdata/junit/PROVENANCE.md`, `internal/report/junit_fixtures_test.go`

**Interfaces:**

*Consumes:* `ReadJUnitFile` (Tasks 1–2), `RenderID`/`ParseID` (Task 3).

*Produces:* no new Go symbol. It produces the evidence — six reports and the record of how
each was made.

**How each fixture is captured.** Every one is produced from a throwaway project containing
one passing test, one failing test and one skipped test, run in a scratch directory outside
this repository, with the resulting XML copied in verbatim (only absolute paths in
`hostname`/`timestamp`-style attributes may be shortened, and `PROVENANCE.md` records that
it was done). The exact commands:

| Fixture | Runner | Command that produced it |
|---|---|---|
| `vitest.xml` | Vitest | `npx --yes vitest@3 run --reporter=junit --outputFile=vitest.xml` |
| `jest.xml` | Jest | `npm i -D jest jest-junit && JEST_JUNIT_OUTPUT_FILE=jest.xml npx jest --reporters=jest-junit` |
| `gojunit.xml` | `go-junit-report` | `go install github.com/jstemmer/go-junit-report/v2@latest && go test -v ./... 2>&1 \| go-junit-report -set-exit-code > gojunit.xml` |
| `surefire.xml` | Maven Surefire | `mvn -B test`, then copy `target/surefire-reports/TEST-com.example.CalculatorTest.xml` |
| `rspec.xml` | RSpec | `gem install rspec rspec_junit_formatter && rspec --format RspecJunitFormatter --out rspec.xml` |
| `phpunit.xml` | PHPUnit | `composer require --dev phpunit/phpunit && vendor/bin/phpunit --log-junit phpunit.xml` |

`PROVENANCE.md` records, per fixture: the command above verbatim, the runner version the
capture came from (`npx vitest --version`, `mvn -v`, and so on), the date, and **the
attribute set the capture actually has** — in particular whether `<testcase file=…>` is
present. This plan predicts that Vitest, RSpec and PHPUnit emit `file=` and that Surefire,
`go-junit-report` and jest-junit do not. **If a capture disagrees, the capture wins**: fix
the table in `PROVENANCE.md` and the expectations in the test below, and say so in the
commit message. A fixture edited to match a prediction is a hand-written fixture wearing a
runner's name, which is what AC4 forbids.

- [ ] **Step 1: Write the failing test**

`internal/report/junit_fixtures_test.go`:

```go
package report

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// fixture is one real captured report and what this milestone claims about it. The
// id_template on each row is the one a Task-3 render can actually satisfy for that
// runner's attribute set — NOT the template the shipped adapter will use, which PRD #232
// owns.
type fixture struct {
	file       string
	minCases   int
	hasFile    bool   // does this runner emit <testcase file=…>?
	idTemplate string // a template that renders every case in this fixture
	wantStatus map[string]bool
}

var junitFixtures = []fixture{
	{file: "vitest.xml", minCases: 3, hasFile: true, idTemplate: "{file}::{name}"},
	{file: "jest.xml", minCases: 3, hasFile: false, idTemplate: "{classname}#{name}"},
	{file: "gojunit.xml", minCases: 3, hasFile: false, idTemplate: "{classname}#{name}"},
	{file: "surefire.xml", minCases: 3, hasFile: false, idTemplate: "{classname}#{name}"},
	{file: "rspec.xml", minCases: 3, hasFile: true, idTemplate: "{file}::{name}"},
	{file: "phpunit.xml", minCases: 3, hasFile: true, idTemplate: "{file}::{name}"},
}

func fixturePath(name string) string { return filepath.Join("testdata", "junit", name) }

// Every fixture is a real capture, so the first thing asserted is that it exists and is
// not a stub someone wrote to make the table pass.
func TestEveryFixtureIsPresentAndNonTrivial(t *testing.T) {
	for _, f := range junitFixtures {
		st, err := os.Stat(fixturePath(f.file))
		if err != nil {
			t.Errorf("%s: %v — AC4 requires real captured output, committed", f.file, err)
			continue
		}
		if st.Size() < 200 {
			t.Errorf("%s is %d bytes; a real capture of three tests is not that small", f.file, st.Size())
		}
	}
	if _, err := os.Stat(filepath.Join("testdata", "junit", "PROVENANCE.md")); err != nil {
		t.Errorf("PROVENANCE.md: %v — a fixture nobody can regenerate is not evidence", err)
	}
}

func TestEveryFixtureParsesIntoTheOutcomeVocabulary(t *testing.T) {
	valid := map[string]bool{"pass": true, "fail": true, "skip": true, "error": true}
	for _, f := range junitFixtures {
		t.Run(f.file, func(t *testing.T) {
			cases, err := ReadJUnitFile(fixturePath(f.file))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v", err)
			}
			if len(cases) < f.minCases {
				t.Fatalf("got %d cases, want at least %d", len(cases), f.minCases)
			}
			sawFail := false
			for _, c := range cases {
				if !valid[c.Status] {
					t.Errorf("case %q status %q is outside the Outcome vocabulary", c.Name, c.Status)
				}
				if c.Name == "" {
					t.Errorf("case with empty name: %+v", c)
				}
				if c.Status == "fail" || c.Status == "error" {
					sawFail = true
				}
				if got := c.File != ""; got != f.hasFile {
					t.Errorf("case %q has file=%q; this fixture was recorded as hasFile=%v", c.Name, c.File, f.hasFile)
				}
			}
			if !sawFail {
				t.Errorf("%s carries no failing case; a fixture that only passes never exercises <failure>", f.file)
			}
		})
	}
}

// PRD #231 AC3: parse → render → parse is stable for all six fixtures.
func TestIDRoundTripIsStableForEveryFixture(t *testing.T) {
	for _, f := range junitFixtures {
		t.Run(f.file, func(t *testing.T) {
			cases, err := ReadJUnitFile(fixturePath(f.file))
			if err != nil {
				t.Fatalf("ReadJUnitFile: %v", err)
			}
			seen := map[string]bool{}
			for _, c := range cases {
				id, err := RenderID(f.idTemplate, c)
				if err != nil {
					t.Fatalf("RenderID(%q, %+v): %v", f.idTemplate, c, err)
				}
				if seen[id] {
					t.Errorf("id %q is produced by two cases; a colliding id makes a subset run select the wrong test", id)
				}
				seen[id] = true
				back, err := ParseID(f.idTemplate, id)
				if err != nil {
					t.Fatalf("ParseID(%q, %q): %v", f.idTemplate, id, err)
				}
				again, err := RenderID(f.idTemplate, back)
				if err != nil {
					t.Fatalf("RenderID after ParseID: %v", err)
				}
				if again != id {
					t.Errorf("round trip: %q -> %q", id, again)
				}
			}
		})
	}
}

// Decision 2, pinned to the fixture that actually lacks the attribute.
func TestSurefireFixtureHasNoFileAttributeAndIsNamedRatherThanDerived(t *testing.T) {
	cases, err := ReadJUnitFile(fixturePath("surefire.xml"))
	if err != nil {
		t.Fatalf("ReadJUnitFile: %v", err)
	}
	if len(cases) == 0 {
		t.Fatalf("surefire fixture parsed to no cases")
	}
	for _, c := range cases {
		if c.File != "" {
			t.Fatalf("surefire case %q has file=%q; this fixture is the one that must lack it", c.Name, c.File)
		}
	}
	if _, err := RenderID("{file}::{name}", cases[0]); !errors.Is(err, ErrNoFileAttr) {
		t.Fatalf("RenderID error = %v, want errors.Is(_, ErrNoFileAttr)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestEveryFixture|TestIDRoundTrip|TestSurefireFixture'
```

Expected: `--- FAIL: TestEveryFixtureIsPresentAndNonTrivial` with six
`open testdata/junit/<name>.xml: no such file or directory` lines and the `PROVENANCE.md`
line, and `TestEveryFixtureParsesIntoTheOutcomeVocabulary` failing every subtest on the same
missing files. No build failure — every symbol exists after Task 3.

- [ ] **Step 3: Write minimal implementation**

Capture the six reports with the commands in the table above and commit them verbatim, plus
`PROVENANCE.md`. There is no Go code in this task: the "implementation" is the evidence. If
a runner cannot be installed on the machine doing the work, install it in a container
(`docker run --rm -v "$PWD":/w -w /w maven:3-eclipse-temurin-21 mvn -B test` and its
equivalents) rather than hand-writing the file — a synthesised fixture makes every later
claim about "real captured output" false. Update `junitFixtures`' `hasFile` and
`idTemplate` columns to what the captures actually contain, and record any divergence from
this plan's prediction in `PROVENANCE.md`.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -run 'TestEveryFixture|TestIDRoundTrip|TestSurefireFixture' -v
```

Expected: every subtest `--- PASS`, six fixture subtests named in the output. A fixture
whose real attribute set differs from the table fails here with the exact attribute in the
message, which is the signal to fix the table rather than the fixture.

- [ ] **Step 5: Commit**

```bash
git add internal/report/testdata/junit internal/report/junit_fixtures_test.go
git commit -m "report: six real captured JUnit reports and the id round-trip over them"
```

---

## Task 5 — `internal/report` + `internal/adapter`: `report_path` is one file or one directory

**Discharges:** spec §4.3 (Surefire and Gradle write a *directory* of XML while every flag-driven runner writes one file) and spec §4.2 (the contract rule that makes the shape unambiguous), PRD #231 AC1 and the reading half of AC7. Pins under-determined decision 3.

**Files:** `internal/report/reportpath.go`, `internal/report/reportpath_test.go`, `internal/adapter/adapter.go`, `internal/adapter/v2_test.go`

**Interfaces:**

*Consumes:* `ReadJUnitFile` (Tasks 1–2), `RenderID` (Task 3), `Outcome` (existing).

*Produces:*
```go
// ReportPath is an adapter's report_path resolved against the repo root. The shape is read
// from the DECLARATION, never sniffed from disk: a trailing "/" means a directory of
// *.xml, anything else means exactly one file. Sniffing would make the meaning of an
// adapter depend on whether the previous run happened to leave a directory behind.
type ReportPath struct {
	Abs   string // absolute, cleaned
	IsDir bool
}

// NewReportPath resolves a declared report_path against repoRoot.
func NewReportPath(repoRoot, declared string) (ReportPath, error)

// Files returns the report files to read, sorted: the single file, or every *.xml
// DIRECTLY inside the directory. Sorted rather than readdir order so a merged report is
// reproducible.
func (p ReportPath) Files() ([]string, error)

// ReadJUnitReport reads every file Files names and renders each case's id through tmpl,
// producing the same []Outcome the pytest-reportlog path produces.
func ReadJUnitReport(p ReportPath, tmpl string) ([]Outcome, error)
```

and, in `internal/adapter`, one new `validate` rejection:
`report_path "target/surefire-reports/*.xml": globs are not supported; name one file, or a directory ending in "/"`.

- [ ] **Step 1: Write the failing test**

`internal/report/reportpath_test.go`:

```go
package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Decision 3: the shape comes from the declaration. A trailing slash is a directory of
// *.xml; anything else is exactly one file.
func TestNewReportPathReadsTheShapeFromTheDeclarationNotTheDisk(t *testing.T) {
	root := t.TempDir()
	// A directory exists at the file-shaped declaration, and nothing exists at the
	// directory-shaped one: if the shape were sniffed, both answers would be wrong.
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	file, err := NewReportPath(root, "reports")
	if err != nil {
		t.Fatalf("NewReportPath(file-shaped): %v", err)
	}
	if file.IsDir {
		t.Errorf("%q parsed as a directory; only a trailing slash means directory", "reports")
	}
	dir, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath(dir-shaped): %v", err)
	}
	if !dir.IsDir {
		t.Errorf("%q parsed as a file; a trailing slash means directory", "target/surefire-reports/")
	}
	if !strings.HasPrefix(dir.Abs, root) {
		t.Errorf("Abs = %q, want it under the repo root %q", dir.Abs, root)
	}
}

// A report_path that escapes the repo root, or is absolute, is a configuration error: the
// engine clears this path, and clearing outside the repository is not a thing RTDD does.
func TestNewReportPathRefusesToEscapeTheRepoRoot(t *testing.T) {
	root := t.TempDir()
	for _, declared := range []string{"../outside.xml", "/etc/junit.xml", ""} {
		if _, err := NewReportPath(root, declared); err == nil {
			t.Errorf("NewReportPath(%q) = nil error, want a rejection", declared)
		}
	}
}

// Surefire and Gradle write a directory of files; the merged read is every *.xml directly
// inside it, in sorted order, and nothing else.
func TestFilesReadsEveryXMLInADirectoryInSortedOrder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "target", "surefire-reports")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range map[string]string{
		"TEST-b.xml":         `<testsuite name="b"><testcase classname="b" name="two"/></testsuite>`,
		"TEST-a.xml":         `<testsuite name="a"><testcase classname="a" name="one"/></testsuite>`,
		"a.txt":              "not xml",
		"nested/TEST-c.xml":  `<testsuite name="c"><testcase classname="c" name="three"/></testsuite>`,
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	p, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	files, err := p.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	var got []string
	for _, f := range files {
		got = append(got, filepath.Base(f))
	}
	want := []string{"TEST-a.xml", "TEST-b.xml"}
	if len(got) != len(want) {
		t.Fatalf("Files = %v, want %v (sorted, *.xml only, not recursive)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Files[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The directory read is one report: the cases of every file, in file order, rendered
// through one id_template into the Outcome vocabulary the map already speaks.
func TestReadJUnitReportMergesADirectoryIntoOneOutcomeList(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("TEST-a.xml", `<testsuite name="a" time="0.02">
  <testcase classname="a.A" name="one" time="0.02"/>
</testsuite>`)
	write("TEST-b.xml", `<testsuite name="b" time="0.03">
  <testcase classname="b.B" name="two" time="0.03"><failure message="nope"/></testcase>
</testsuite>`)

	p, err := NewReportPath(root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	outs, err := ReadJUnitReport(p, "{classname}#{name}")
	if err != nil {
		t.Fatalf("ReadJUnitReport: %v", err)
	}
	want := []Outcome{
		{Test: "a.A#one", Status: "pass", DurationMS: 20},
		{Test: "b.B#two", Status: "fail", DurationMS: 30},
	}
	if len(outs) != len(want) {
		t.Fatalf("got %+v, want %+v", outs, want)
	}
	for i := range want {
		if outs[i] != want[i] {
			t.Errorf("outcome %d = %+v, want %+v", i, outs[i], want[i])
		}
	}
}

// A directory that exists and holds no *.xml is not zero tests: it is a runner that wrote
// nothing, which is ErrNoReport by another route.
func TestReadJUnitReportTreatsAnEmptyDirectoryAsNoReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p, err := NewReportPath(root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if _, err := ReadJUnitReport(p, "{classname}#{name}"); err == nil {
		t.Fatalf("ReadJUnitReport = nil error for a directory holding no report")
	}
}
```

and, in `internal/adapter/v2_test.go`:

```go
package adapter

import (
	"strings"
	"testing"
)

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
	_, err := Load(writeAdapter(t, "maven.yaml", globbed))
	if err == nil {
		t.Fatalf("Load = nil error for a globbed report_path")
	}
	for _, want := range []string{"report_path", "globs are not supported", `ending in "/"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
	// The directory form of the same declaration is legal.
	if _, err := Load(writeAdapter(t, "maven-dir.yaml", strings.Replace(globbed,
		`report_path: "target/surefire-reports/*.xml"`,
		`report_path: "target/surefire-reports/"`, 1))); err != nil {
		t.Fatalf("Load(directory report_path): %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestNewReportPath|TestFiles|TestReadJUnitReport' && go test ./internal/adapter/ -run 'TestValidateRejectsAGlobInReportPath'
```

Expected: `internal/report` fails to build with `undefined: NewReportPath`,
`undefined: ReadJUnitReport` and `p.Files undefined`; `internal/adapter` compiles and fails
with `Load = nil error for a globbed report_path` — `*` is a legal character in a YAML
string and nothing rejects it yet.

- [ ] **Step 3: Write minimal implementation**

`internal/report/reportpath.go`: `NewReportPath` rejects an empty declaration, an absolute
path (`filepath.IsAbs` or a leading `/`) and any path that leaves the root after
`filepath.Clean`; records `IsDir` from a trailing `/` **before** cleaning strips it.
`Files` returns `[]string{p.Abs}` for a file, or `os.ReadDir` filtered to non-directory
entries with `strings.EqualFold(filepath.Ext(name), ".xml")`, sorted by name, for a
directory — a missing directory is `ErrNoReport`, and so is a directory with no `*.xml`.
`ReadJUnitReport` calls `ReadJUnitFile` per file, renders each case with `RenderID`, and
appends `Outcome{Test: id, Status: c.Status, DurationMS: c.DurationMS}` — no
de-duplication here, because `internal/runner` already de-duplicates across chunks and two
de-dupers with different rules is how "last invocation wins" stops being true.

In `internal/adapter/adapter.go`, one new case in `validate`'s switch, beside the existing
`report: junit-xml requires report_path`:

```go
	case a.Report == "junit-xml" && strings.ContainsAny(a.ReportPath, "*?["):
		return fmt.Errorf("report_path %q: globs are not supported; name one file, or a directory ending in %q", a.ReportPath, "/")
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -v -run 'TestNewReportPath|TestFiles|TestReadJUnitReport' && go test ./internal/report/ ./internal/adapter/ ./internal/contract/
```

Expected: the five subtests `--- PASS`, then all three packages `ok` — `internal/contract`
included, because it is the package that would notice if the Python adapter had been
touched to satisfy the new rule.

- [ ] **Step 5: Commit**

```bash
git add internal/report/reportpath.go internal/report/reportpath_test.go internal/adapter/adapter.go internal/adapter/v2_test.go
git commit -m "report: report_path is one file or one directory of *.xml, never a glob"
```

---

## Task 6 — `internal/report`: clearing the report path before invocation

**Discharges:** spec §4.3 (a report that was never written must not be read as a result) and PRD #231 AC7 (a stale report from a previous run cannot be mistaken for the current one).

**Files:** `internal/report/reportpath.go`, `internal/report/reportpath_test.go`

**Interfaces:**

*Consumes:* `ReportPath` (Task 5).

*Produces:*
```go
// Clear removes the previous run's report so a runner that writes nothing produces
// ErrNoReport rather than last run's outcomes. For a file: remove it; a missing file is
// not an error. For a directory: create it if absent, then remove every *.xml DIRECTLY
// inside it — never the directory itself and never a non-XML sibling, because
// target/surefire-reports also holds the *.txt dumps a developer may be reading.
func (p ReportPath) Clear() error
```

- [ ] **Step 1: Write the failing test**

Append to `internal/report/reportpath_test.go`:

```go
package report

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// PRD #231 AC7: the previous run's report must not survive into this one. The failure it
// prevents is the worst kind — a subset run whose runner never started, reporting the
// outcomes of a run that is not this one.
func TestClearRemovesAStaleSingleFileReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(root, ".rtdd", "junit.xml")
	if err := os.WriteFile(stale, []byte(`<testsuite name="old"><testcase classname="old" name="passed"/></testsuite>`), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	p, err := NewReportPath(root, ".rtdd/junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale report still present after Clear (stat err = %v)", err)
	}
	if _, err := ReadJUnitReport(p, "{classname}#{name}"); !errors.Is(err, ErrNoReport) {
		t.Fatalf("after Clear, read error = %v, want errors.Is(_, ErrNoReport)", err)
	}
	// Clearing a path that is already absent is not an error: the first run of a repo.
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear on an absent report: %v", err)
	}
}

// For a directory the clear is surgical: this run's *.xml go, the directory stays, and
// everything that is not XML stays — Surefire's own *.txt dumps included.
func TestClearRemovesOnlyTheXMLInADirectoryReport(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "target", "surefire-reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range map[string]string{
		"TEST-old.xml":         `<testsuite name="old"/>`,
		"com.example.Test.txt": "a human-readable dump",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	p, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "TEST-old.xml")); !os.IsNotExist(err) {
		t.Errorf("stale TEST-old.xml survived Clear (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "com.example.Test.txt")); err != nil {
		t.Errorf("Clear deleted a non-XML sibling: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Clear removed the report directory itself: %v", err)
	}
}

// A directory report_path whose directory does not exist yet must be created: a runner
// invoked with --outputFile in a directory that is not there writes nothing and the run
// fails on a report it could have had.
func TestClearCreatesAMissingReportDirectory(t *testing.T) {
	root := t.TempDir()
	p, err := NewReportPath(root, "build/test-results/test/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	st, err := os.Stat(filepath.Join(root, "build", "test-results", "test"))
	if err != nil || !st.IsDir() {
		t.Fatalf("report directory not created by Clear: %v", err)
	}
}

// The single-file form needs its parent to exist for the same reason.
func TestClearCreatesTheParentOfASingleFileReport(t *testing.T) {
	root := t.TempDir()
	p, err := NewReportPath(root, ".rtdd/junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	st, err := os.Stat(filepath.Join(root, ".rtdd"))
	if err != nil || !st.IsDir() {
		t.Fatalf("parent directory not created by Clear: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/ -run 'TestClear'
```

Expected: `[build failed]` with `p.Clear undefined (type ReportPath has no field or method Clear)`.

- [ ] **Step 3: Write minimal implementation**

`Clear` on `ReportPath`: `os.MkdirAll` of the directory (`p.Abs` when `IsDir`, else
`filepath.Dir(p.Abs)`), then either `os.Remove(p.Abs)` ignoring `os.IsNotExist`, or a
`os.ReadDir` loop removing each non-directory `*.xml` child. Every removal error other than
"not exist" is returned wrapped with the path — a report path RTDD cannot clear is a report
path it must not then read.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/report/ -run 'TestClear' -v && go test ./internal/report/
```

Expected: the four `--- PASS` lines and a green package.

- [ ] **Step 5: Commit**

```bash
git add internal/report/reportpath.go internal/report/reportpath_test.go
git commit -m "report: clear the report path before invocation so a stale report cannot be read as this run"
```

---

## Task 7 — `internal/runner`: the `report:` dispatch, `{report}`, and per-chunk merge

**Discharges:** spec §4.3 (the runner half — "one parser in `internal/report` covers all ten" is only true once something calls it) and PRD #231 AC1, AC7 (the runner writes to and reads from `report_path`, clearing it before invocation) and AC8 (`exit_codes` mapping applies exactly as it does today). Pins under-determined decision 4.

**Files:** `internal/runner/run.go`, `internal/runner/junit_test.go`

**Interfaces:**

*Consumes:* `report.NewReportPath`, `report.ReadJUnitReport`, `(ReportPath).Clear` (Tasks 5–6); `adapter.Adapter` (existing); `execute` (existing).

*Produces:* no new exported symbol in `internal/runner`. `execute` gains:

```go
// {report} joins {out} and {log} in the placeholder map, and ONLY when the adapter
// declares report_path. Expand's vocabulary is the caller's map, so an adapter naming
// {report} without report_path still fails as an unresolved placeholder rather than
// receiving an empty string and writing its report to "".
	if a.ReportPath != "" {
		vars["report"] = rp.Abs
	}

// readOutcomes is the ONE dispatch this milestone adds. It sits exactly where
// report.ReadReportLog was called unconditionally.
func readOutcomes(a *adapter.Adapter, logPath string, rp report.ReportPath) ([]report.Outcome, error)
```

- [ ] **Step 1: Write the failing test**

`internal/runner/junit_test.go`:

```go
package runner

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/report"
)

// stubJUnitScript stands in for a JUnit-emitting runner. It writes the XML it is told to
// write to the path it is given, records its argv, and exits with the code it is told to,
// so the runner's control flow — clearing, chunking, exit-code mapping, per-chunk merging
// — is tested without node, maven or ruby on PATH.
const stubJUnitScript = `#!/bin/sh
out=""
for a in "$@"; do
  case "$a" in
    --outputFile=*) out="${a#--outputFile=}" ;;
  esac
done
if [ -n "$RTDD_STUB_ARGS" ]; then
  printf -- '--- invocation ---\n' >> "$RTDD_STUB_ARGS"
  for a in "$@"; do printf '%s\n' "$a" >> "$RTDD_STUB_ARGS"; done
fi
if [ -n "$out" ] && [ -n "$RTDD_STUB_XML" ] && [ -f "$RTDD_STUB_XML" ]; then
  cat "$RTDD_STUB_XML" > "$out"
fi
exit ${RTDD_STUB_EXIT:-0}
`

// junitStubAdapter builds a static adapter whose subset command is the stub script.
func junitStubAdapter(t *testing.T, repo string, env map[string]string) *adapter.Adapter {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runner script is POSIX sh")
	}
	stub := filepath.Join(repo, "stubjunit")
	if strings.ContainsAny(stub, " \t") {
		t.Skipf("temp dir %q contains whitespace; command templates are whitespace-split", stub)
	}
	if err := os.WriteFile(stub, []byte(stubJUnitScript), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if env == nil {
		env = map[string]string{}
	}
	return &adapter.Adapter{
		Name:       "stubjunit",
		Detect:     []string{"package.json"},
		Env:        env,
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Subset:     stub + " {tests} --outputFile={report}",
		Report:     "junit-xml",
		ReportPath: ".rtdd/junit.xml",
		IDTemplate: "{classname}#{name}",
		ExitCodes:  map[int]string{4: "bad-selector", 5: "no-tests-collected"},
	}
}

func writeStubXML(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// The junit path end to end: {report} expands, the stub writes there, the parser reads it,
// and the ids come back rendered through id_template — in the same namespace as the ids
// that were spliced into the command.
func TestRunReadsOutcomesFromTheAdaptersReportPath(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	xml := writeStubXML(t, repo, "src.xml", `<testsuite name="s" time="0.04">
  <testcase classname="s.A" name="one" time="0.01"/>
  <testcase classname="s.B" name="two" time="0.03"><failure message="nope"/></testcase>
</testsuite>`)
	a.Env["RTDD_STUB_XML"] = xml

	res, err := Run(a, repo, []string{"s.A#one", "s.B#two"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 2 {
		t.Fatalf("Outcomes = %+v, want two", res.Outcomes)
	}
	if res.Outcomes[0].Test != "s.A#one" || res.Outcomes[0].Status != "pass" || res.Outcomes[0].DurationMS != 10 {
		t.Errorf("Outcomes[0] = %+v, want s.A#one pass 10ms", res.Outcomes[0])
	}
	if len(res.Failed) != 1 || res.Failed[0] != "s.B#two" {
		t.Errorf("Failed = %v, want [s.B#two]", res.Failed)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 for a run with a failing test", res.ExitCode)
	}
	// coverage: none means nothing reads .coverage, and the empty Result is not nil.
	if res.Coverage == nil || len(res.Coverage.ImportTime) != 0 {
		t.Errorf("Coverage = %+v, want the empty result under coverage: none", res.Coverage)
	}
}

// PRD #231 AC7, in the runner: a report left by a previous run must be cleared before the
// invocation, so a runner that writes nothing fails loudly instead of replaying history.
func TestRunClearsAStaleReportBeforeInvoking(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	if err := os.MkdirAll(filepath.Join(repo, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(repo, ".rtdd", "junit.xml")
	if err := os.WriteFile(stale, []byte(`<testsuite name="old"><testcase classname="old.X" name="ghost"/></testsuite>`), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	// No RTDD_STUB_XML: the stub runs and writes nothing at all.
	_, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = nil error; a runner that wrote no report must not return the previous run's outcomes")
	}
	if !errors.Is(err, report.ErrNoReport) {
		t.Fatalf("error = %v, want errors.Is(_, report.ErrNoReport)", err)
	}
	if strings.Contains(err.Error(), "ghost") {
		t.Errorf("the stale report reached the caller: %v", err)
	}
}

// Decision 4: report_path is fixed, so chunk i+1 overwrites chunk i's report. The read
// therefore happens per chunk, and the existing last-invocation-wins de-duplication
// carries over unchanged.
func TestRunMergesEveryChunksReportBeforeTheNextOverwritesIt(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, nil)
	// One id per chunk: MaxArgvBytes is a byte budget, so a tiny budget forces the split.
	first := writeStubXML(t, repo, "first.xml", `<testsuite name="s"><testcase classname="s.A" name="one" time="0.01"/></testsuite>`)
	a.Env["RTDD_STUB_XML"] = first

	res, err := runWithBudget(a, repo, []string{"s.A#one", "s.B#two"}, 12)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both chunks wrote the same single-case report, so the merged result is that one
	// outcome recorded once — not two, and not the second chunk's report alone.
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "s.A#one" {
		t.Fatalf("Outcomes = %+v, want exactly one s.A#one after the per-chunk merge", res.Outcomes)
	}
}

// PRD #231 AC8: the adapter's exit_codes mapping applies on this path exactly as it does
// on the pytest one — and it is read BEFORE the report, so a bad-selector exit is a named
// fatal error and never a parse failure against a file the runner declined to write.
func TestExitCodeMappingAppliesOnTheJUnitPathBeforeTheReportIsRead(t *testing.T) {
	repo := t.TempDir()
	a := junitStubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})

	_, err := Run(a, repo, []string{"s.A#one"}, false)
	if err == nil {
		t.Fatalf("Run = nil error for an exit mapped to bad-selector")
	}
	var fatal *FatalExitError
	if !errors.As(err, &fatal) {
		t.Fatalf("error = %v (%T), want *FatalExitError", err, err)
	}
	if fatal.Code != 4 || fatal.Label != "bad-selector" {
		t.Errorf("FatalExitError = %+v, want code 4 bad-selector", fatal)
	}
	if errors.Is(err, report.ErrNoReport) {
		t.Errorf("the missing report shadowed the exit-code mapping: %v", err)
	}
}
```

`runWithBudget` is a one-line test helper beside `Chunk`'s existing tests: it calls the same
`execute` with `Chunk(tests, budget)` so the chunking path is exercised without a
100,000-byte argv.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/runner/ -run 'TestRunReadsOutcomesFrom|TestRunClears|TestRunMerges|TestExitCodeMappingApplies'
```

Expected: `[build failed]` with `undefined: runWithBudget`. With that helper added and the
runner still unchanged, the red becomes
`Run: adapter stubjunit: unknown placeholder {report} in "--outputFile={report}"` for the
first three, and — once `{report}` resolves but the dispatch does not exist —
`Run: chunk 0: report: opening /tmp/rtdd-run-*/report-0.jsonl: no such file or directory`,
because `execute` still reads a pytest report log for every adapter.

- [ ] **Step 3: Write minimal implementation**

In `internal/runner/run.go`:

1. Resolve the report path once, before the chunk loop:
   `rp, err := report.NewReportPath(repoRoot, a.ReportPath)` when `a.ReportPath != ""`.
2. Add `vars["report"] = rp.Abs` inside the loop, guarded on `a.ReportPath != ""`.
3. Call `rp.Clear()` immediately before `cmd.Run()`, next to the existing stale-`.coverage`
   removal — per chunk, for the reason decision 4 gives.
4. Replace the single `report.ReadReportLog(logPath)` call with `readOutcomes(a, logPath, rp)`,
   which switches on `a.Report`: `"pytest-reportlog"` → `report.ReadReportLog(logPath)`,
   `"junit-xml"` → `report.ReadJUnitReport(rp, a.IDTemplate)`, default →
   `fmt.Errorf("runner: adapter %s: unsupported report %q", a.Name, a.Report)`.
5. Guard the coverage read with `if a.Coverage != adapter.CoverageNone` — a static adapter
   has no `.coverage` to read, and `ReadSQLite` against a file that does not exist is an
   error, not an empty result.

Nothing else moves. The exit-code mapping, the sysmon check and the `byTest` de-duplication
are untouched, which is what makes AC8 a test rather than a change.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/runner/ -run 'TestRunReadsOutcomesFrom|TestRunClears|TestRunMerges|TestExitCodeMappingApplies' -v && go test ./internal/runner/ ./internal/report/ ./internal/adapter/
```

Expected: four `--- PASS` lines, then all three packages `ok` — every existing pytest-path
runner test included, unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/run.go internal/runner/junit_test.go
git commit -m "runner: dispatch on report:, expand {report}, clear and read report_path per chunk"
```

---

## Task 8 — `internal/contract` + `00-interfaces.md`: record the second parser and hold the freeze

**Discharges:** spec §4.3 (the contract as recorded in `00-interfaces.md`, so no later milestone re-derives it) and PRD #231 AC9 (`scripts/ci-local.sh` exits 0 — the whole gate, on the whole tree).

**Files:** `docs/plans/00-interfaces.md`, `internal/contract/contract_test.go`

**Interfaces:**

*Consumes:* every symbol added by Tasks 1–7.

*Produces:* no runtime symbol. It produces the record: `## internal/report` in
`00-interfaces.md` gains `JUnitCase`, `ReadJUnitFile`, `RenderID`, `ParseID`, `ReportPath`,
`NewReportPath`, `Files`, `Clear`, `ReadJUnitReport` and the four sentinels, with the two
decisions that are not visible from a signature written beside them: a missing `file=` is
named rather than derived, and `report_path`'s shape comes from a trailing slash.

- [ ] **Step 1: Write the failing test**

Append to `internal/contract/contract_test.go`:

```go
package contract

import (
	"strings"
	"testing"
)

// The interface contract is where a later milestone reads what M6c shipped. A symbol that
// exists in the tree and not in the contract is one a sibling PRD re-invents differently.
func TestInterfaceContractRecordsTheJUnitParser(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")
	for _, want := range []string{
		"ReadJUnitFile", "JUnitCase", "RenderID", "ParseID",
		"ReportPath", "NewReportPath", "ReadJUnitReport",
		"ErrNoReport", "ErrEmptyReport", "ErrMalformedReport", "ErrSuiteFailure", "ErrNoFileAttr",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md does not record %s", want)
		}
	}
}

// The two decisions a signature cannot carry. Both are the difference between a correct
// run and a silently empty one, so both are written down where the next reader looks.
func TestInterfaceContractRecordsTheTwoJUnitDecisions(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")
	if !strings.Contains(src, "{classname}") || strings.Contains(src, "id_template expands {class}") {
		t.Errorf("00-interfaces.md must record {classname} as the id_template spelling")
	}
	for _, want := range []string{"trailing", "file attribute"} {
		if !strings.Contains(src, want) {
			t.Errorf("00-interfaces.md does not record the %q decision", want)
		}
	}
}

// The two parsers sit side by side. A single "read the report" entry point that hides the
// choice is how the pytest path acquires a caller it was never tested under.
func TestBothReportParsersExistAndTheReportLogOneIsUnchanged(t *testing.T) {
	junit := readRepoFile(t, "internal/report/junit.go")
	if !strings.Contains(junit, "func ReadJUnitFile") {
		t.Errorf("internal/report/junit.go does not define ReadJUnitFile")
	}
	log := readRepoFile(t, "internal/report/reportlog.go")
	if !strings.Contains(log, "func ReadReportLog") {
		t.Errorf("internal/report/reportlog.go no longer defines ReadReportLog")
	}
	if strings.Contains(log, "junit") || strings.Contains(log, "JUnit") {
		t.Errorf("the pytest-reportlog parser mentions junit; this PRD does not touch that file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/contract/ -run 'TestInterfaceContractRecordsTheJUnit|TestInterfaceContractRecordsTheTwoJUnit|TestBothReportParsers'
```

Expected: `--- FAIL: TestInterfaceContractRecordsTheJUnitParser` with one
`00-interfaces.md does not record <symbol>` line per symbol, and the decisions test failing
on `trailing` / `file attribute`. `TestBothReportParsersExistAndTheReportLogOneIsUnchanged`
passes as soon as Task 1 landed — it is a regression guard, not new ground.

- [ ] **Step 3: Write minimal implementation**

Extend `## internal/report` in `docs/plans/00-interfaces.md` with the signatures and the two
decision notes, in the style the rest of that file uses (a fenced `go` block of signatures
with the non-obvious rule stated in a comment above each). Add the M6c amendment section at
the end of the file, as M6a did, naming what changed and what it deliberately did not:
`reportlog.go` untouched, `adapters/python.yaml` still byte-frozen, `internal/selector` and
every tier untouched.

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/contract/ && scripts/ci-local.sh
```

Expected: `ok github.com/VocanicZ/rtdd/internal/contract`, then the full local gate — build,
vet, gofmt, the whole test suite, `rtdd-gen check`, `rtdd-gen verify`, the embedded protocol
diff, the prereg gate and the static binary — exiting 0. That exit code is PRD #231 AC9.

- [ ] **Step 5: Commit**

```bash
git add docs/plans/00-interfaces.md internal/contract/contract_test.go
git commit -m "contract: record the junit-xml parser, its two decisions, and the untouched reportlog path"
```

---

## Definition of Done

**The capability**

- [ ] An adapter declaring `report: junit-xml`, `report_path` and `id_template` runs: the
      subset command is invoked, the report is parsed, and `RunResult.Outcomes` carries one
      `Outcome` per `<testcase>` with `Test` rendered through `id_template` (AC1, AC3).
- [ ] `<failure>` is `fail`, `<error>` is `error`, `<skipped>` is `skip`, and a bare
      `<testcase>` is `pass`. An `<error>` is never a skip (AC2).
- [ ] Nested `<testsuite>` elements are flattened in document order, and a suite-level
      `<failure>` with no `<testcase>` is `ErrSuiteFailure`, naming the suite and the
      runner's own message (AC5).
- [ ] A missing report, an empty report and malformed XML are three distinct sentinels,
      each wrapped with the offending path. No input produces a silent zero-test success
      (AC6).
- [ ] The report path is cleared before every invocation, and a runner that writes nothing
      produces `ErrNoReport` rather than the previous run's outcomes (AC7).
- [ ] `exit_codes` maps on the junit path exactly as on the pytest path, and is read before
      the report (AC8).

**The evidence**

- [ ] Six real captured reports are committed under `internal/report/testdata/junit/`, from
      Vitest, Jest, `go-junit-report`, Maven Surefire, RSpec and PHPUnit, with
      `PROVENANCE.md` recording the command, the runner version and the attribute set of
      each (AC4).
- [ ] `parse → render → parse` is asserted stable over every case of all six fixtures, and
      no two cases in one fixture render the same id (AC3).

**The regressions this milestone had to avoid**

- [ ] `internal/report/reportlog.go` is unchanged; no task listed it, and no existing
      report-log test changed in behaviour.
- [ ] `adapters/python.yaml` is byte-frozen and `internal/contract`'s digest test needed no
      edit.
- [ ] `rtdd which`, `rtdd run`, `rtdd seed` and `rtdd status` produce byte-identical output
      to before this milestone in a seeded Python repo.
- [ ] No test was deleted, skipped or weakened to accommodate a new rule.

**Contract**

- [ ] `docs/plans/00-interfaces.md` records every addition; no name in the implementation
      diverges from it.
- [ ] The `Outcome.Status` vocabulary is still exactly `pass`, `fail`, `skip`, `error`.

**Hygiene**

- [ ] `scripts/ci-local.sh` exits 0 (AC9).
- [ ] `go.mod` still lists exactly `gopkg.in/yaml.v3` and `modernc.org/sqlite`.
- [ ] Every task's commit is separate and its test was seen to fail before its
      implementation was written.

## What this plan deliberately leaves undone

Everything below is real work the multi-language design wants; none of it belongs to
PRD #231, and no task above may start it. Each line names the milestone from spec §8 and
the sibling or later PRD that owns it — a scope this plan may not silently absorb.

- **The `TS` tier's selection logic — spec §4.1, milestone M6b, sibling PRD #230.** This
  plan makes a static adapter's *outcomes* readable. It does not insert a tier between T1
  and T2, does not resolve `test_for` templates against the filesystem, does not run
  `importscan`, and does not rank by correspondence, import distance or path proximity.
  `internal/selector` is not opened by any task here. Until #230 lands, an adapter declaring
  `selection: static` can run a subset it is handed and still has nothing handing it one.
- **The shipped adapter set — spec §8, milestone M6d, sibling PRD #232.** `adapters/` gains
  no file: no Vitest, Jest, Go, Rust, Java/Kotlin, Ruby, C# or PHP adapter. Every adapter in
  every test above is built in the test. That PRD also owns the question this plan can only
  answer for the parser — which `id_template` each runner's own selector syntax actually
  accepts, and whether a rendered id survives `ExpandTests`' one-token-per-id splicing. The
  round-trip asserted here is `RenderID`/`ParseID` stability, **not** a claim that any
  runner accepts the result; only a real invocation against a real runner proves that, and
  #232 is where it is proved. Polyglot detection — `Detect` returning a set, and the adapter
  tag on rows, selections and subset invocations (spec §4.4) — is #232's as well.
- **Per-test coverage in any non-Python ecosystem — spec §11, audit finding A5.** It stands
  and it stays deferred: no Vitest coverage provider, no injected `TestMain`, no
  `ClearCounters()` codegen, in this PRD or any other in the chain. `junit-xml` carries
  outcomes, which is the *`s`* and *`d`* half of a map row; the coverage half does not exist
  outside Python and this milestone does not pretend otherwise.
- **The remaining §6 honesty surfaces — milestone M6e, a later PRD.** `rtdd which` and
  `rtdd run` human output, `selection_fidelity` on `--json`, the static-tier entry in the
  `warnings` array, and the `PROTOCOL.md` → `SKILL.md`/`AGENTS.md`/`.mdc` regeneration are
  untouched. A junit-driven run is readable by the engine here; making every front-end say
  what fidelity produced it is that PRD's.
- **The evidence table — spec §7, milestone M6e, a later PRD.** No measurement is made here
  and nothing in this plan may claim the static tier beats the `path` baseline.
- **Reading anything but JUnit XML.** `trx`, TAP, `go test -json` and Istanbul JSON are not
  parsed. Spec §4.3 chose one universal outcome format precisely so there would be one
  parser; a second format is a contract change and belongs in a spec amendment, not in an
  implementation task.
