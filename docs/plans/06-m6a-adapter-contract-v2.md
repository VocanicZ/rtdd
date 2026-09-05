# RTDD M6a — Adapter Contract v2, Host Adapters, `init` Gating

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the adapter contract able to describe a toolchain RTDD cannot instrument, and stop `rtdd init` installing agent front-ends into a repository where RTDD can answer nothing but "run the full suite". After this milestone a host repo can add `.rtdd/adapters/<language>.yaml`, `rtdd doctor` states in one line what selection fidelity that repository can actually achieve, and `rtdd init` refuses a no-adapter repo without `--force`. No tier, no runner and no parser changes — the `TS` tier (M6b) and the `junit-xml` parser (M6c) consume this contract; they are not built here.

**Architecture:** `internal/adapter` gains seven declarative fields (`selection`, `coverage: none`, `report_path`, `id_template`, `test_for`, `importscan`, `requires`) on the existing `Adapter` struct, two derived predicates (`Fidelity`, `Unmet`), and a second loader source: host `.rtdd/adapters/*.yaml` overlaid by name onto the `go:embed`-ed built-ins by `Available`. Validation stays where it already is — `parse` → `validate`, one message per rejection — so a host typo is exit 2 with the offending field named. `cmd/rtdd/init.go` calls `adapter.DetectAll` before `install.Plan` and refuses on zero matches; `cmd/rtdd/doctor.go` gains a fidelity block above the existing fan-out table and is the ONE lenient reader of host adapters, reporting an invalid one instead of dying on it. D8 holds throughout: the YAML declares, Go executes.

**Tech Stack:** Go 1.24+, gopkg.in/yaml.v3, stdlib testing. No new dependency.

**Spec:** `docs/specs/2026-09-05-multi-language.md` §4.2 (adapter contract v2), §4.5 (user-authorable adapters), §5 (`init` must gate), §6 (the `doctor` honesty surface only). Each task below names the section it discharges on its `**Discharges:**` line. The `requires` key is declared by spec §4.3 — four of the ten runners in its JUnit table need a package the host repo may not have, and cargo-nextest needs a config file rather than a flag — and its `doctor` finding is PRD #229 AC9; they are discharged here under §4.2 (the declaration joins the contract) and §6 (the surface that reports it). Everything else in that spec — the `TS` tier (§4.1), the `junit-xml` parser (§4.3), polyglot selection (§4.4), the evidence table (§7) and the remaining §6 surfaces (`which`/`run`/`--json`/`PROTOCOL.md`) — belongs to a later PRD and is listed at the end of this document.

## Global Constraints

*(the first five are copied verbatim from `00-interfaces.md`)*

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except `modernc.org/sqlite` and
  `gopkg.in/yaml.v3`. This milestone adds none; `TestGoModRequiresExactlyYAMLAndSQLite`
  in `internal/contract` fails if it does.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

M6a-specific:

- **`adapters/python.yaml` is byte-frozen for this PRD.** Spec §4.1 makes "a seeded Python
  repository's selection is byte-identical to today" a regression *requirement*, and the
  cheapest way to keep that promise is to not touch the only adapter that has ever produced
  a map. Every v2 field is optional and every v1 adapter stays valid unchanged. No task in
  this plan lists that file, and Task 7 adds a guard test that fails if a later one does.
- **No behaviour change on any existing path.** `Detect`, `Expand`, the runner, the
  selector and `rtdd run`/`seed`/`which` are untouched. The only user-visible changes are
  `rtdd init`'s new refusal and `rtdd doctor`'s new first block.
- **A v2 field the engine cannot yet act on still parses and validates.** `report: junit-xml`
  is accepted by the contract here and has no parser until M6c; an adapter declaring it
  loads, validates, and fails at *run* time with the existing unsupported-report path. The
  contract is the thing being shipped, not the capability behind it.
- **Exit codes are unchanged** (`00-interfaces.md`): 2 for a configuration error — which is
  what an unparseable host adapter and a no-adapter `init` both are — and 0 for `doctor`,
  always. `doctor` reports; it never expresses a policy opinion through an exit code.

### The three under-determined decisions, resolved here

Spec §4.2/§4.5/§6 leave three things open. They are decided in this plan so no implementer
has to guess, and each has a task that pins the decision with a test.

**1. A host adapter whose `name` collides with a built-in REPLACES the built-in** (Task 4).
Built-ins load first, host adapters load second, and the overlay is by `Name`. The host
wins because §4.5 makes host YAML the actual boundary of support: a user whose Vitest
adapter is subtly wrong for their monorepo must be able to fix it in their own repo without
waiting for an RTDD release, and the alternative rules are both worse — "built-in wins"
makes the shipped set unfixable, and "collision is an error" makes upgrading RTDD break
repos that were working. The override is never silent: `rtdd doctor` prints
`overrides built-in` beside such an adapter (Task 6). Two *host* files declaring the same
name IS an error (exit 2), because there is no principled winner between them; the message
names both paths.

**2. The two illegal key combinations, and their exact wording** (Task 2). `selection`,
`coverage` and `seed` encode one fact between them — whether this toolchain is
instrumented — so PRD #229 AC2 names the two ways of disagreeing. Both are rejected with
the field-and-value shape the rest of `validate` already uses, and each message names
**both** offending keys:

| Declaration | Error text (wrapped by `parse` as `adapter: <src>: <text>`) |
|---|---|
| `selection: coverage` (declared or defaulted) with `coverage: none` | ``coverage: none requires selection: static, got selection "coverage"`` |
| `selection: static` with a non-empty `seed` | ``selection: static forbids seed, got seed "pytest --cov"`` |

Both are exit 2. A third message states the same fact from the remaining direction —
``selection: static requires coverage: none, got coverage "sqlite"`` — so no combination of
the three keys can reach the engine half-declared.

The defaulting rule that makes this coherent: an omitted `selection` is `coverage`, an
omitted `coverage` under `selection: coverage` is `sqlite` — so today's
`adapters/python.yaml`, which declares neither `selection` nor anything but
`coverage: sqlite`, is valid unchanged, and the first row's `got selection "coverage"`
reports the *defaulted* value rather than an absence. `seed` is therefore **required** under
`selection: coverage` and **forbidden** under `selection: static`: there is nothing to
instrument, and a seed command that must never run is a trap for the next reader.

**3. `doctor` prints an explicit, actionable line for fidelity `none`** (Tasks 3 and 6).
An adapter is fidelity `none` when it declares `coverage: none` and offers neither a
`test_for` template nor an `importscan` command. Path proximity alone does not count as
`static`: it is exactly the `path` baseline the spec §7 pre-registration says the static
tier must *beat* to be worth shipping, so an adapter that can only do that is honestly
reported as selecting nothing. `doctor` prints, and exits 0:

```
  elixir   .rtdd/adapters/elixir.yaml   none
      declares selection: static but no test_for templates and no importscan command,
      so RTDD cannot select anything narrower than the full suite (T2).
      Fix: add a test_for template that resolves to a real test file, or an importscan
      command, then re-run rtdd doctor.
```

**4. An unmet `requires` entry is a `doctor` finding and an `init` warning, never a refusal**
(Tasks 7 and 5). PRD #229 AC9 requires an unmet prerequisite to surface at `doctor`/`init`
time rather than as a mid-run parse failure against a report file that was never written.
It does not say `init` must refuse, and this plan decides it must not: the repo *has* an
adapter, so the §5 gate is satisfied, and a missing binary is a machine's state rather than
the repository's — refusing would make `rtdd init` fail on a CI image that installs
toolchains after checkout. `init` prints the unmet entries and exits 0; `doctor` prints
them as named findings and exits 0; the run-time failure they predict is left to the
command that actually runs the suite.

## What is already true in the tree

Read these before writing code; several tasks are smaller than they look because the
mechanism already exists.

- `internal/adapter/adapter.go` — `Adapter` is a flat struct of `yaml:"…"` tags; `parse`
  decodes with `dec.KnownFields(true)`, so **an unknown key is already a hard error**. That
  is why every v2 field must be added to the struct before any host adapter can use it.
- `LoadFS(fsys fs.FS, dir string)` is already "the one reader"; `LoadAll` and `Builtin`
  differ only in the `fs.FS` they hand it. Host loading is a third caller, not a second
  reader.
- `validate()` already gives every rejection its own message and runs `validateGlobs()`
  first. The two new combination checks join that `switch`.
- `internal/adapter/detect.go` — `detectAll` already walks the repo once and returns every
  matching adapter; `Detect` is a thin arity check over it that errors on 0 and on ≥2.
  `init`'s gate needs "≥1", so Task 5 exports `DetectAll` rather than relaxing `Detect`.
  Polyglot *selection* stays out of scope (spec §4.4, M6d).
- `cmd/rtdd/init.go` — `cmdInit` resolves the root, calls `install.Files()`,
  `install.Plan(root, files, force)`, prints, and applies. It calls `adapter.Detect`
  nowhere; that absence is the defect spec §5 closes.
- `internal/install/install.go` — `defaultConfig` is a const string and `.rtdd/config.yaml`
  is **created, never overwritten**. So the detected-adapter record spec §5 asks for can be
  written on a first init and must not silently rewrite an existing config; `doctor`
  derives fidelity live and is the source of truth, the config block is a record.
- `cmd/rtdd/doctor.go` — `RenderDoctor(hubs, total, limit)` is a pure formatter and
  `cmdDoctor` is its only caller. The fidelity block is a second pure formatter above it.

## File Structure

| File | Single responsibility |
|---|---|
| `internal/adapter/adapter.go` | **MODIFIED** — seven v2 fields on `Adapter`, the `Importscan` and `Requirement` structs, defaulting, and the combination checks in `validate` |
| `internal/adapter/fidelity.go` | **NEW** — the `Fidelity` type and `(*Adapter).Fidelity()` |
| `internal/adapter/requires.go` | **NEW** — `(*Adapter).Unmet` and `UnmetFinding`: prerequisites checked at doctor/init time |
| `internal/adapter/fidelity_test.go` | **NEW** — the truth table for the three fidelities |
| `internal/adapter/v2_test.go` | **NEW** — v2 parsing, defaulting, and the two rejected combinations |
| `internal/adapter/host.go` | **NEW** — `LoadHost`, `LoadHostReport`, `Available`: host `.rtdd/adapters/*.yaml` overlaid onto the built-ins by name |
| `internal/adapter/host_test.go` | **NEW** — override precedence, duplicate host names, missing directory |
| `internal/adapter/detect.go` | **MODIFIED** — export `DetectAll` (the existing unexported walk) so `init` can gate on ≥1 without relaxing `Detect` |
| `internal/adapter/markers.go` | **NEW** — `UnsupportedMarkers`: marker file → language name, used only to name what an ungated repo *does* contain |
| `internal/install/install.go` | **MODIFIED** — `ConfigWithAdapters`, and `WithNoAdapterCaveat` for the `--force` path |
| `cmd/rtdd/init.go` | **MODIFIED** — detect before writing; exit 2 with the unsupported-toolchain message; `--force` installs the caveated front-end |
| `cmd/rtdd/doctor.go` | **MODIFIED** — `RenderFidelity` above the fan-out table, `RenderRequirements` under it |
| `internal/contract/adapter_freeze_test.go` | **NEW** — `adapters/python.yaml` is byte-frozen for this PRD |
| `docs/plans/00-interfaces.md` | **MODIFIED** — record every contract addition made here |

---

## Task 1 — `internal/adapter`: the seven v2 fields parse

**Discharges:** spec §4.2 (adapter contract v2 — the declarative surface; the `requires` key is spec §4.3's, per PRD #229 AC1).

**Files:** `internal/adapter/adapter.go`, `internal/adapter/v2_test.go`

**Interfaces:**

*Consumes:* `adapter.Adapter`, `parse`, `Load` (existing).

*Produces:*
```go
// Importscan is the optional per-language import scanner. It follows the precedent of
// internal/importscan: the engine runs a script, it does not parse the language itself.
type Importscan struct {
	Command string `yaml:"command"` // e.g. "node {script}"
	Script  string `yaml:"script"`  // shipped beside the adapter
}

// Requirement is one binary this adapter cannot work without, and why. Spec §4.3: Go
// needs go-junit-report, Jest needs jest-junit, RSpec needs rspec_junit_formatter and
// dotnet needs JUnitTestLogger before any of them can emit JUnit XML at all. It exists so
// an unmet prerequisite surfaces at doctor/init time instead of as a mid-run parse failure
// against a report file that was never written (PRD #229 AC9).
type Requirement struct {
	Bin    string `yaml:"bin"`
	Reason string `yaml:"reason"`
}

// New Adapter fields, all optional; a v1 adapter stays valid unchanged.
//	Selection  string        `yaml:"selection"`    // "coverage" (default) | "static"
//	ReportPath string        `yaml:"report_path"`
//	IDTemplate string        `yaml:"id_template"`
//	TestFor    []string      `yaml:"test_for"`
//	Importscan *Importscan   `yaml:"importscan"`
//	Requires   []Requirement `yaml:"requires"`
//	Src        string        `yaml:"-"`            // the file this adapter was loaded from

const (
	SelectionCoverage = "coverage"
	SelectionStatic   = "static"
	CoverageNone      = "none"
)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/v2_test.go`:

```go
package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

// staticYAML is the shape spec §4.2 shows: a toolchain RTDD cannot instrument, described
// entirely declaratively. Every key here is new in v2 except name/detect/subset/list and
// the glob fields.
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
importscan:
  command: "node {script}"
  script: "scan-imports.mjs"
requires:
  - bin: node
    reason: "the importscan script and the vitest runner both run under node"
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
opaque: ["**/*.json"]
full_escalate: ["package.json"]
`

// writeAdapter materialises one adapter file and returns its path.
func writeAdapter(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestLoadParsesTheV2Fields(t *testing.T) {
	a, err := Load(writeAdapter(t, "vitest.yaml", staticYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Selection != SelectionStatic {
		t.Errorf("Selection = %q, want %q", a.Selection, SelectionStatic)
	}
	if a.Coverage != CoverageNone {
		t.Errorf("Coverage = %q, want %q", a.Coverage, CoverageNone)
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
	want := []string{"{dir}/{name}.test.ts", "{dir}/__tests__/{name}.test.ts"}
	if len(a.TestFor) != len(want) {
		t.Fatalf("TestFor = %v, want %v", a.TestFor, want)
	}
	for i := range want {
		if a.TestFor[i] != want[i] {
			t.Errorf("TestFor[%d] = %q, want %q", i, a.TestFor[i], want[i])
		}
	}
	if a.Importscan == nil {
		t.Fatalf("Importscan = nil, want the declared scanner")
	}
	if a.Importscan.Command != "node {script}" || a.Importscan.Script != "scan-imports.mjs" {
		t.Errorf("Importscan = %+v, want {node {script} scan-imports.mjs}", *a.Importscan)
	}
	if len(a.Requires) != 1 {
		t.Fatalf("Requires = %v, want exactly one entry", a.Requires)
	}
	if a.Requires[0].Bin != "node" || a.Requires[0].Reason == "" {
		t.Errorf("Requires[0] = %+v, want bin node with a non-empty reason", a.Requires[0])
	}
}

// A v1 adapter declares neither key. It must keep loading, and it must default to the
// behaviour it has today — spec §4.1 makes byte-identical Python behaviour a requirement.
func TestSelectionAndCoverageDefaultToTodaysBehaviour(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	var py *Adapter
	for _, a := range all {
		if a.Name == "python" {
			py = a
		}
	}
	if py == nil {
		t.Fatalf("Builtin has no python adapter")
	}
	if py.Selection != SelectionCoverage {
		t.Errorf("python Selection = %q, want the default %q", py.Selection, SelectionCoverage)
	}
	if py.Coverage != "sqlite" {
		t.Errorf("python Coverage = %q, want %q", py.Coverage, "sqlite")
	}
	if py.Importscan != nil || len(py.TestFor) != 0 || len(py.Requires) != 0 || py.ReportPath != "" || py.IDTemplate != "" {
		t.Errorf("python adapter picked up v2 fields it does not declare: %+v", py)
	}
}

// Src is how doctor and every error message name the file a bad adapter came from.
func TestLoadRecordsTheSourceFile(t *testing.T) {
	p := writeAdapter(t, "vitest.yaml", staticYAML)
	a, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Src != p {
		t.Errorf("Src = %q, want %q", a.Src, p)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestLoadParsesTheV2Fields|TestSelectionAndCoverage|TestLoadRecordsTheSourceFile'
```

Expected: `[build failed]` with `a.Selection undefined (type *Adapter has no field or method Selection)`, `undefined: SelectionStatic`, `undefined: CoverageNone`, `a.Requires undefined`, `a.Src undefined`. Not a plain assertion failure — the fields do not exist yet, and `KnownFields(true)` would additionally reject `staticYAML` at decode time with `field selection not found in type adapter.Adapter`.

- [ ] **Step 3: Write minimal implementation**

In `internal/adapter/adapter.go`, extend the struct and add the constants and defaulting.
`Src` is set by every loader, so `parse` takes the source name it already receives:

```go
const (
	// SelectionCoverage is the v1 behaviour and the default: tests are chosen from
	// recorded per-test coverage.
	SelectionCoverage = "coverage"
	// SelectionStatic chooses tests from declared correspondence and imports. Spec §4.2.
	SelectionStatic = "static"
	// CoverageNone is the only coverage value permitted under SelectionStatic.
	CoverageNone = "none"
)

// Importscan is the optional per-language import scanner. Per D8 the engine runs a
// script and never parses the language itself — the precedent internal/importscan set.
type Importscan struct {
	Command string `yaml:"command"`
	Script  string `yaml:"script"`
}

// Requirement is one binary this adapter cannot work without, and why.
type Requirement struct {
	Bin    string `yaml:"bin"`
	Reason string `yaml:"reason"`
}

type Adapter struct {
	// ... every v1 field unchanged ...

	// v2 (spec §4.2, plus `requires` from PRD #229 AC1). All optional: a v1 adapter is
	// a valid v2 adapter.
	Selection  string        `yaml:"selection"`
	ReportPath string        `yaml:"report_path"`
	IDTemplate string        `yaml:"id_template"`
	TestFor    []string      `yaml:"test_for"`
	Importscan *Importscan   `yaml:"importscan"`
	Requires   []Requirement `yaml:"requires"`

	// Src is the file this adapter was read from. Never declared in YAML; it is how
	// doctor and every validation message name the offending file.
	Src string `yaml:"-"`
}

// applyDefaults fills the two keys that encode one fact between them. An omitted
// selection is today's behaviour, and today's behaviour reads sqlite coverage.
func (a *Adapter) applyDefaults() {
	if a.Selection == "" {
		a.Selection = SelectionCoverage
	}
	if a.Coverage == "" && a.Selection == SelectionCoverage {
		a.Coverage = "sqlite"
	}
}
```

and in `parse`, between `Decode` and `validate`:

```go
	a.Src = src
	a.applyDefaults()
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestLoadParsesTheV2Fields|TestSelectionAndCoverage|TestLoadRecordsTheSourceFile' -v
```

Expected: `--- PASS: TestLoadParsesTheV2Fields`, `--- PASS: TestSelectionAndCoverageDefaultToTodaysBehaviour`, `--- PASS: TestLoadRecordsTheSourceFile`, `ok`. Then `go test ./internal/adapter/` in full, which must stay green: no v1 test may change.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: v2 declarative fields (selection, report_path, id_template, test_for, importscan, requires)"
```

---

## Task 2 — `internal/adapter`: validation of the two illegal key combinations

**Discharges:** spec §4.2 (`coverage: none` is *required* when `selection: static`; `seed` stops applying under static), hardened by PRD #229 AC2.

**Files:** `internal/adapter/adapter.go`, `internal/adapter/v2_test.go`

**Interfaces:**

*Consumes:* `(*Adapter).validate`, `(*Adapter).applyDefaults` (Task 1).

*Produces:* no new exported symbol. `validate` gains six rejections, in this order after
`validateGlobs`: the two combination errors PRD #229 AC2 names, the third
`static`/`coverage` message that closes the remaining direction, `seed` required under
`SelectionCoverage`, `requires` entries needing both `bin` and `reason`, and
`report: junit-xml` requiring both `report_path` and `id_template` (spec §4.3's id
round-trip, audit A6 — the contract carries it even though the parser lands in a sibling
PRD).

- [ ] **Step 1: Write the failing test**

Append to `internal/adapter/v2_test.go`:

```go
package adapter

import (
	"strings"
	"testing"
)

// The two illegal combinations PRD #229 AC2 names, with the exact wording this plan pins.
// Each message names BOTH offending keys, so an agent reading exit 2 knows which one to
// change and what the other must become.
func TestValidateRejectsTheTwoIllegalKeyCombinations(t *testing.T) {
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
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
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
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
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
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
`,
			want: `selection: static requires coverage: none, got coverage "sqlite"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(writeAdapter(t, "a.yaml", tc.yaml))
			if err == nil {
				t.Fatalf("Load accepted an illegal selection/coverage/seed combination")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

// seed instruments a suite. There is nothing to instrument under selection: static, and a
// coverage adapter without one cannot build a map at all.
func TestSeedIsForbiddenUnderStaticAndRequiredUnderCoverage(t *testing.T) {
	if _, err := Load(writeAdapter(t, "vitest.yaml", staticYAML)); err != nil {
		t.Errorf("static adapter without seed: Load = %v, want nil", err)
	}

	noSeed := `name: python2
detect: ["pyproject.toml"]
subset: "pytest {tests} --cov"
coverage: sqlite
report: pytest-reportlog
`
	_, err := Load(writeAdapter(t, "b.yaml", noSeed))
	if err == nil {
		t.Fatalf("Load accepted a coverage adapter with no seed command")
	}
	if !strings.Contains(err.Error(), "seed is required") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "seed is required")
	}
}

// A requires entry with no reason is a prerequisite nobody can act on: doctor prints the
// reason verbatim, so an empty one produces a finding that names a binary and no cause.
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
		{base + "requires:\n  - reason: \"runs vitest\"\n", `requires[0]: bin is required`},
		{base + "requires:\n  - bin: node\n", `requires[0] (node): reason is required`},
	} {
		_, err := Load(writeAdapter(t, "d.yaml", tc.yaml))
		if err == nil {
			t.Fatalf("Load accepted a requires entry that %q rejects", tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
		}
	}
}

// A JUnit testcase is a (classname, name) pair. Without id_template there is nothing to
// render it back into the runner's own selector syntax, and without report_path there is
// nothing to read — audit A6 is the reason this is a contract rule and not an assumption.
func TestJUnitReportRequiresPathAndIDTemplate(t *testing.T) {
	base := `name: vitest
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
`
	for _, tc := range []struct{ yaml, want string }{
		{base + "id_template: \"{file}::{name}\"\n", `report: junit-xml requires report_path`},
		{base + "report_path: \".rtdd/junit.xml\"\n", `report: junit-xml requires id_template`},
	} {
		_, err := Load(writeAdapter(t, "c.yaml", tc.yaml))
		if err == nil {
			t.Fatalf("Load accepted junit-xml without the field %q names", tc.want)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("error = %q, want it to contain %q", err.Error(), tc.want)
		}
	}
}

// The shipped adapter must survive every new rule unchanged. adapters/python.yaml is
// byte-frozen for this PRD (see Task 7); this is the behavioural half of that freeze.
func TestBuiltinAdaptersStillValidate(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("Builtin returned no adapters")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestValidateRejectsTheTwoIllegal|TestSeedIsForbidden|TestRequiresEntries|TestJUnitReportRequires'
```

Expected: `--- FAIL: TestValidateRejectsTheTwoIllegalKeyCombinations/coverage_none_with_defaulted_selection` with `Load accepted an illegal selection/coverage/seed combination`, and the same for the other two subtests; `--- FAIL: TestSeedIsForbiddenUnderStaticAndRequiredUnderCoverage` with `static adapter without seed: Load = adapter: …: seed is required, want nil`; `--- FAIL: TestRequiresEntriesNeedABinAndAReason` with `Load accepted a requires entry that "requires[0]: bin is required" rejects`; `--- FAIL: TestJUnitReportRequiresPathAndIDTemplate` with `Load accepted junit-xml without the field "report: junit-xml requires report_path" names`.

- [ ] **Step 3: Write minimal implementation**

Replace the tail of `validate()`'s switch in `internal/adapter/adapter.go`:

```go
	switch {
	case a.Name == "":
		return fmt.Errorf("name is required")
	case len(a.Detect) == 0:
		return fmt.Errorf("detect is required")

	// selection, coverage and seed encode one fact between them: whether this toolchain
	// is instrumented. PRD #229 AC2 names the first two rejections; the third closes the
	// remaining direction. All are configuration errors (exit 2). Spec §4.2.
	case a.Coverage == CoverageNone && a.Selection != SelectionStatic:
		return fmt.Errorf("coverage: none requires selection: static, got selection %q", a.Selection)
	case a.Selection == SelectionStatic && a.Seed != "":
		return fmt.Errorf("selection: static forbids seed, got seed %q", a.Seed)
	case a.Selection == SelectionStatic && a.Coverage != CoverageNone:
		return fmt.Errorf("selection: static requires coverage: none, got coverage %q", a.Coverage)

	// A coverage adapter with no seed command can never build a map.
	case a.Selection == SelectionCoverage && a.Seed == "":
		return fmt.Errorf("seed is required")

	case a.Subset == "":
		return fmt.Errorf("subset is required")
	case !strings.Contains(a.Subset, "{tests}"):
		return fmt.Errorf("subset %q has no {tests} placeholder", a.Subset)

	case a.Selection == SelectionCoverage && a.Coverage != "sqlite":
		return fmt.Errorf("unsupported coverage %q (only \"sqlite\" and \"none\")", a.Coverage)

	case a.Report == "junit-xml" && a.ReportPath == "":
		return fmt.Errorf("report: junit-xml requires report_path")
	case a.Report == "junit-xml" && a.IDTemplate == "":
		return fmt.Errorf("report: junit-xml requires id_template")
	case a.Report != "pytest-reportlog" && a.Report != "junit-xml":
		return fmt.Errorf("unsupported report %q (only \"pytest-reportlog\" and \"junit-xml\")", a.Report)
	}

	// A prerequisite doctor cannot explain is not worth declaring: the reason is printed
	// verbatim in the finding (spec §4.3, PRD #229 AC9).
	for i, r := range a.Requires {
		switch {
		case r.Bin == "":
			return fmt.Errorf("requires[%d]: bin is required", i)
		case r.Reason == "":
			return fmt.Errorf("requires[%d] (%s): reason is required", i, r.Bin)
		}
	}
	return nil
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -count=1
```

Expected: `--- PASS` for the three new tests and `ok github.com/VocanicZ/rtdd/internal/adapter`. Every pre-existing adapter test passes unchanged; if one of them asserted the old `unsupported report` wording, fix the assertion **in that test only** and never the rule.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: reject the illegal selection/coverage/seed combinations and incomplete requires entries"
```

---

## Task 3 — `internal/adapter`: `Fidelity`

**Discharges:** spec §4.2 and §6 (`selection_fidelity` is `"execution-derived" | "static" | "none"`, never null).

**Files:** `internal/adapter/fidelity.go`, `internal/adapter/fidelity_test.go`

**Interfaces:**

*Consumes:* the v2 fields (Task 1), the validation rules (Task 2).

*Produces:*
```go
// Fidelity is how a selection was derived. It is the value spec §6 puts on every
// surface; the string values are the wire format and must not be reworded.
type Fidelity string

const (
	FidelityExecution Fidelity = "execution-derived"
	FidelityStatic    Fidelity = "static"
	FidelityNone      Fidelity = "none"
)

// Fidelity reports the best selection this adapter can ever produce. It is a property of
// the declaration alone — not of whether a map has been seeded, which is a tier question.
func (a *Adapter) Fidelity() Fidelity
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/fidelity_test.go`:

```go
package adapter

import "testing"

// The truth table. `none` is the decision this plan pins: path proximity alone is the
// `path` baseline the static tier must beat (spec §7), so an adapter offering only that
// selects nothing and says so.
func TestFidelity(t *testing.T) {
	cases := []struct {
		name string
		a    *Adapter
		want Fidelity
	}{
		{
			name: "coverage adapter",
			a:    &Adapter{Selection: SelectionCoverage, Coverage: "sqlite"},
			want: FidelityExecution,
		},
		{
			name: "static with correspondence templates",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone, TestFor: []string{"{dir}/{name}.test.ts"}},
			want: FidelityStatic,
		},
		{
			name: "static with an import scanner only",
			a: &Adapter{Selection: SelectionStatic, Coverage: CoverageNone,
				Importscan: &Importscan{Command: "node {script}", Script: "scan-imports.mjs"}},
			want: FidelityStatic,
		},
		{
			name: "static with neither",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone},
			want: FidelityNone,
		},
		{
			name: "static with an empty test_for list",
			a:    &Adapter{Selection: SelectionStatic, Coverage: CoverageNone, TestFor: []string{}},
			want: FidelityNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Fidelity(); got != tc.want {
				t.Errorf("Fidelity() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The wire values are a contract with --json consumers (spec §6). Renaming one is a
// breaking change to the agent front-ends, so it fails here first.
func TestFidelityWireValues(t *testing.T) {
	for got, want := range map[Fidelity]string{
		FidelityExecution: "execution-derived",
		FidelityStatic:    "static",
		FidelityNone:      "none",
	} {
		if string(got) != want {
			t.Errorf("fidelity constant = %q, want %q", string(got), want)
		}
	}
}

// A nil adapter is the no-adapter repo, and spec §6 says the field is never null.
func TestNilAdapterIsFidelityNone(t *testing.T) {
	var a *Adapter
	if got := a.Fidelity(); got != FidelityNone {
		t.Errorf("(*Adapter)(nil).Fidelity() = %q, want %q", got, FidelityNone)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestFidelity|TestNilAdapterIsFidelityNone'
```

Expected: `[build failed]` — `undefined: Fidelity`, `undefined: FidelityExecution`, `tc.a.Fidelity undefined (type *Adapter has no field or method Fidelity)`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/fidelity.go`:

```go
package adapter

// Fidelity is how a selection was derived (spec §6). The string values are the wire
// format for --json and must not be reworded.
type Fidelity string

const (
	FidelityExecution Fidelity = "execution-derived"
	FidelityStatic    Fidelity = "static"
	FidelityNone      Fidelity = "none"
)

// Fidelity reports the best selection this adapter can ever produce.
//
// It is a property of the declaration, not of the repository's state: an unseeded Python
// repo is still execution-derived — it just has no map yet, which is a tier question the
// selector answers.
//
// A static adapter with neither a test_for template nor an importscan command is `none`,
// not `static`. All it could offer is path proximity, which is exactly the `path`
// baseline spec §7 pre-registers the static tier against; reporting that as `static`
// would tell an agent it had a narrowed suite when it has the full one.
func (a *Adapter) Fidelity() Fidelity {
	if a == nil {
		return FidelityNone
	}
	if a.Coverage != CoverageNone && a.Selection != SelectionStatic {
		return FidelityExecution
	}
	if len(a.TestFor) > 0 || a.Importscan != nil {
		return FidelityStatic
	}
	return FidelityNone
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestFidelity|TestNilAdapterIsFidelityNone' -v
```

Expected: `--- PASS: TestFidelity` with all five subtests, `--- PASS: TestFidelityWireValues`, `--- PASS: TestNilAdapterIsFidelityNone`, `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: Fidelity — execution-derived, static, or an honest none"
```

---

## Task 4 — `internal/adapter`: host `.rtdd/adapters/*.yaml` and built-in precedence

**Discharges:** spec §4.5 (user-authorable adapters — "the load-bearing part of the design").

**Files:** `internal/adapter/host.go`, `internal/adapter/host_test.go`

**Interfaces:**

*Consumes:* `LoadFS`, `Builtin` (existing); `Adapter.Src` (Task 1).

*Produces:*
```go
// HostAdapterDir is where a host repo declares its own adapters.
const HostAdapterDir = ".rtdd/adapters"

// Invalid is one host adapter file that failed to load, kept rather than returned as a
// fatal error so `rtdd doctor` can name it. Nothing else is lenient.
type Invalid struct {
	Path string
	Err  error
}

// LoadHost reads every .rtdd/adapters/*.yaml under repoRoot. A missing directory is not
// an error; an unparseable file is.
func LoadHost(repoRoot string) ([]*Adapter, error)

// LoadHostReport is LoadHost for doctor: it returns the adapters that did load and a
// report of the ones that did not.
func LoadHostReport(repoRoot string) ([]*Adapter, []Invalid, error)

// Available returns the built-ins overlaid with the host's adapters. A host adapter whose
// name equals a built-in's REPLACES it; the result is sorted by name.
func Available(repoRoot string) ([]*Adapter, error)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/host_test.go`:

```go
package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHostAdapter drops one YAML file into <root>/.rtdd/adapters/.
func writeHostAdapter(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".rtdd", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// The whole point of §4.5: a language the engine has never heard of, supported by YAML.
func TestAvailableIncludesHostAdapters(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	names := map[string]*Adapter{}
	for _, a := range all {
		names[a.Name] = a
	}
	if names["python"] == nil {
		t.Errorf("Available lost the built-in python adapter")
	}
	if names["vitest"] == nil {
		t.Fatalf("Available did not load .rtdd/adapters/vitest.yaml; got %v", names)
	}
	if !strings.HasSuffix(filepath.ToSlash(names["vitest"].Src), ".rtdd/adapters/vitest.yaml") {
		t.Errorf("host adapter Src = %q, want the host path", names["vitest"].Src)
	}
}

// The precedence rule this plan pins: the host wins, so a user can fix a shipped adapter
// for their own repo without waiting for a release.
func TestHostAdapterOverridesTheBuiltinOfTheSameName(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "python.yaml", `name: python
detect: ["pyproject.toml"]
seed: "pytest --cov --cov-context=test"
subset: "pytest {tests} --cov --cov-context=test -p no:randomly"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
test_globs: ["tests/**/*.py"]
source_globs: ["**/*.py"]
`)

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	n := 0
	var py *Adapter
	for _, a := range all {
		if a.Name == "python" {
			n++
			py = a
		}
	}
	if n != 1 {
		t.Fatalf("Available returned %d adapters named python, want exactly 1", n)
	}
	if !strings.Contains(py.Subset, "-p no:randomly") {
		t.Errorf("built-in python won over the host adapter: Subset = %q", py.Subset)
	}
}

// Two host files claiming one name have no principled winner, so it is exit 2 and the
// message names both paths.
func TestDuplicateHostAdapterNamesAreAnError(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "a-vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "z-vitest.yaml", staticYAML)

	_, err := Available(root)
	if err == nil {
		t.Fatalf("Available accepted two host adapters named vitest")
	}
	for _, want := range []string{"a-vitest.yaml", "z-vitest.yaml", "vitest"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err.Error(), want)
		}
	}
}

// Every repo that exists today has no .rtdd/adapters/. That is the normal case, not a
// failure, and it must return exactly the built-ins.
func TestAvailableWithNoHostDirectoryReturnsTheBuiltins(t *testing.T) {
	got, err := Available(t.TempDir())
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	want, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Available = %d adapters, want the %d built-ins", len(got), len(want))
	}
}

// doctor must be able to report a broken host adapter instead of dying on it, and it must
// still see the ones that are fine.
func TestLoadHostReportNamesTheInvalidFileAndKeepsTheRest(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "broken.yaml", "name: broken\ndetect: [\"go.mod\"]\nsubset: \"go test ./...\"\ncoverage: sqlite\nreport: pytest-reportlog\nseed: \"go test ./...\"\n")

	ok, bad, err := LoadHostReport(root)
	if err != nil {
		t.Fatalf("LoadHostReport: %v", err)
	}
	if len(ok) != 1 || ok[0].Name != "vitest" {
		t.Errorf("valid adapters = %v, want just vitest", ok)
	}
	if len(bad) != 1 {
		t.Fatalf("invalid adapters = %v, want exactly one", bad)
	}
	if !strings.HasSuffix(filepath.ToSlash(bad[0].Path), ".rtdd/adapters/broken.yaml") {
		t.Errorf("invalid path = %q, want .rtdd/adapters/broken.yaml", bad[0].Path)
	}
	if !strings.Contains(bad[0].Err.Error(), "{tests}") {
		t.Errorf("invalid err = %q, want it to name the failing field", bad[0].Err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestAvailable|TestHostAdapterOverrides|TestDuplicateHostAdapter|TestLoadHostReport'
```

Expected: `[build failed]` — `undefined: Available`, `undefined: LoadHostReport`, `undefined: Invalid`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/host.go`:

```go
package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// HostAdapterDir is where a host repo declares adapters for languages RTDD does not ship
// (spec §4.5). The shipped set is a convenience, not the boundary of support.
const HostAdapterDir = ".rtdd/adapters"

// Invalid is one host adapter file that failed to load. It exists so `rtdd doctor` can
// name a broken file; every other caller treats a broken adapter as exit 2.
type Invalid struct {
	Path string
	Err  error
}

// LoadHost reads every *.yaml directly under <repoRoot>/.rtdd/adapters. A missing
// directory returns no adapters and no error — that is every repo that exists today.
func LoadHost(repoRoot string) ([]*Adapter, error) {
	dir := filepath.Join(repoRoot, filepath.FromSlash(HostAdapterDir))
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", dir, err)
	}
	return LoadAll(dir)
}

// LoadHostReport is LoadHost for doctor: one file's failure does not hide the others.
func LoadHostReport(repoRoot string) ([]*Adapter, []Invalid, error) {
	dir := filepath.Join(repoRoot, filepath.FromSlash(HostAdapterDir))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("adapter: %s: %w", dir, err)
	}
	var ok []*Adapter
	var bad []Invalid
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		a, err := Load(p)
		if err != nil {
			bad = append(bad, Invalid{Path: p, Err: err})
			continue
		}
		ok = append(ok, a)
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].Name < ok[j].Name })
	return ok, bad, nil
}

// Available returns the built-in adapters overlaid with the host's.
//
// Precedence: the HOST wins. §4.5 makes host YAML the real boundary of support, so a user
// whose shipped adapter is wrong for their monorepo must be able to correct it without an
// RTDD release. "Built-in wins" would make the shipped set unfixable and "collision is an
// error" would break working repos on upgrade. The override is never silent — rtdd doctor
// prints `overrides built-in` beside it.
//
// Two HOST files claiming one name is an error: there is no principled winner.
func Available(repoRoot string) ([]*Adapter, error) {
	builtin, err := Builtin()
	if err != nil {
		return nil, err
	}
	host, err := LoadHost(repoRoot)
	if err != nil {
		return nil, err
	}

	byName := map[string]*Adapter{}
	for _, a := range builtin {
		byName[a.Name] = a
	}
	seen := map[string]string{}
	for _, a := range host {
		if prev, dup := seen[a.Name]; dup {
			return nil, fmt.Errorf("adapter: two host adapters are both named %q: %s and %s", a.Name, prev, a.Src)
		}
		seen[a.Name] = a.Src
		byName[a.Name] = a
	}

	out := make([]*Adapter, 0, len(byName))
	for _, a := range byName {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
```

`LoadAll` must set `Src` to the on-disk path for this to work: it reads through
`LoadFS(os.DirFS(dir), ".")`, whose `Src` is the fs-relative name. Give `LoadFS` the
directory prefix it should record — `LoadAll` passes `dir`, `Builtin` passes `""` — or have
`LoadAll` rewrite `Src` after the call. Either is fine; the test only asserts the suffix.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -count=1 -v -run 'TestAvailable|TestHostAdapterOverrides|TestDuplicateHostAdapter|TestLoadHostReport'
```

Expected: `--- PASS` for all five, then `go test ./... -count=1` still green.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: load host .rtdd/adapters/*.yaml, host overrides the built-in of the same name"
```

---

## Task 5 — `cmd/rtdd`: `init` gates on detection

**Discharges:** spec §5 (`init` must gate).

**Files:** `internal/adapter/detect.go`, `internal/adapter/markers.go`, `internal/install/install.go`, `cmd/rtdd/init.go`, `cmd/rtdd/init_gate_test.go`

**Interfaces:**

*Consumes:* `adapter.Available` (Task 4), `adapter.Fidelity` (Task 3), `install.Plan`,
`findRepoRoot`.

*Produces:*
```go
// DetectAll reports every adapter with a matching marker in repoRoot, sorted by name.
// Detect stays the one-adapter arity check over it; init needs "at least one".
func adapter.DetectAll(repoRoot string, adapters []*Adapter) ([]*Adapter, error)

// UnsupportedMarkers maps a well-known toolchain marker to its language name. It is used
// ONLY to tell a user what their repo does contain when no adapter matched.
var adapter.UnsupportedMarkers map[string]string

// ConfigWithAdapters renders .rtdd/config.yaml with a record of what was detected.
func install.ConfigWithAdapters(recs []install.AdapterRecord) string

// WithNoAdapterCaveat prefixes the no-adapter caveat into the FIRST paragraph of the
// generated SKILL.md, for the --force path only.
func install.WithNoAdapterCaveat(files map[string]string) map[string]string
```

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/init_gate_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// newUnsupportedRepo is the repo spec §1 reproduced on: a real TypeScript project, no
// adapter, and — today — a full RTDD install whose every answer is "run the full suite".
func newUnsupportedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Commit(t, dir, "init")
	return dir
}

func TestInitRefusesARepoWithNoAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 2 {
		t.Fatalf("rtdd init = %d, want 2 (stdout: %s stderr: %s)", code, stdout, stderr)
	}
	for _, want := range []string{"no adapter detected", "package.json", "JavaScript/TypeScript", ".rtdd/adapters/", "--force"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not mention %q:\n%s", want, stderr)
		}
	}
	for _, rel := range []string{"AGENTS.md", ".cursor/rules/rtdd.mdc", ".claude/skills/rtdd/SKILL.md", ".rtdd/config.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s was installed into a repo with no adapter", rel)
		}
	}
}

// --force overrides, and then the front-end must say what it cannot do — in its first
// paragraph, where an agent reads it before it reads a selection.
func TestInitForceInstallsTheNoAdapterCaveat(t *testing.T) {
	dir := newUnsupportedRepo(t)

	if code, _, stderr := rtdd(t, dir, "init", "--force"); code != 0 {
		t.Fatalf("rtdd init --force = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	body := string(b)
	first := body
	if i := strings.Index(strings.TrimLeft(body, "-\n "), "\n\n"); i > 0 {
		first = body[:i+len(body)-len(strings.TrimLeft(body, "-\n "))]
	}
	if !strings.Contains(first, "no adapter") {
		t.Errorf("SKILL.md first paragraph carries no no-adapter caveat:\n%s", first)
	}
}

// A repo RTDD can serve installs exactly as it does today, and records what it found.
func TestInitProceedsAndRecordsTheDetectedAdapter(t *testing.T) {
	dir := newDetectableRepo(t)

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".rtdd", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	cfg := string(b)
	for _, want := range []string{"adapters:", "name: python", "selection: coverage", "fidelity: execution-derived"} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".rtdd/config.yaml does not record %q:\n%s", want, cfg)
		}
	}
	// The v1 defaults must survive.
	if !strings.Contains(cfg, "stale_commits: 50") {
		t.Errorf(".rtdd/config.yaml lost its defaults:\n%s", cfg)
	}
}

// A repo whose only adapter is a host YAML is a served repo. This is §4.5 reaching init.
func TestInitAcceptsAHostOnlyAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\nsubset: \"npx vitest run {tests}\"\nselection: static\ncoverage: none\nreport: junit-xml\nreport_path: \".rtdd/junit.xml\"\nid_template: \"{file}::{name}\"\ntest_for: [\"{dir}/{name}.test.ts\"]\ntest_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 with a host adapter (stderr: %s)", code, stderr)
	}
}

// --dry-run must gate too: printing a plan it would refuse to execute is a lie.
func TestInitDryRunAlsoRefusesARepoWithNoAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	if code, _, _ := rtdd(t, dir, "init", "--dry-run"); code != 2 {
		t.Errorf("rtdd init --dry-run = %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./cmd/rtdd/ -run 'TestInit(Refuses|Force|Proceeds|Accepts|DryRun)' -count=1
```

Expected: `--- FAIL: TestInitRefusesARepoWithNoAdapter` with `rtdd init = 0, want 2` and four `… was installed into a repo with no adapter` errors — today `cmdInit` never calls `adapter.Detect`, which is the defect. Also `--- FAIL: TestInitProceedsAndRecordsTheDetectedAdapter` with `.rtdd/config.yaml does not record "adapters:"`.

- [ ] **Step 3: Write minimal implementation**

Export the existing walk in `internal/adapter/detect.go` (rename `detectAll` →
`DetectAll`, keep `Detect` as its arity check, sort the result by name), then add
`internal/adapter/markers.go`:

```go
package adapter

// UnsupportedMarkers names what a repository plainly contains when no adapter matched it.
// It drives one message and nothing else: it is never used for selection, and an entry
// here is not a claim of support. Spec §5 requires init to name the languages it found.
var UnsupportedMarkers = map[string]string{
	"package.json":     "JavaScript/TypeScript",
	"go.mod":           "Go",
	"Cargo.toml":       "Rust",
	"pom.xml":          "Java (Maven)",
	"build.gradle":     "Java/Kotlin (Gradle)",
	"build.gradle.kts": "Java/Kotlin (Gradle)",
	"Gemfile":          "Ruby",
	"composer.json":    "PHP",
	"mix.exs":          "Elixir",
	"*.csproj":         "C#",
}
```

and gate in `cmd/rtdd/init.go`, immediately after the root is resolved and **before**
`install.Plan` — so `--dry-run` gates too:

```go
	all, err := adapter.Available(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 2
	}
	detected, err := adapter.DetectAll(root, all)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd init: %v\n", err)
		return 3
	}
	if len(detected) == 0 && !*force {
		fmt.Fprintf(stderr, "rtdd init: no adapter detected in %s\n", root)
		if found := unsupportedToolchains(root); len(found) > 0 {
			fmt.Fprintf(stderr, "  found, but not supported by any adapter: %s\n", strings.Join(found, ", "))
		}
		fmt.Fprintln(stderr, "  Installing agent instructions here would promise a selection RTDD cannot make:")
		fmt.Fprintln(stderr, "  every answer would be T2, \"run the full suite\".")
		fmt.Fprintf(stderr, "  Write an adapter in %s/<language>.yaml (see docs/specs/2026-09-05-multi-language.md §4.5),\n", adapter.HostAdapterDir)
		fmt.Fprintln(stderr, "  or re-run with --force to install anyway.")
		return 2
	}

	files, err := install.Files()
	if err != nil { /* unchanged */ }
	if len(detected) == 0 {
		files = install.WithNoAdapterCaveat(files)
	}
```

`unsupportedToolchains(root)` reads `root` non-recursively and returns
`"package.json (JavaScript/TypeScript)"`-shaped strings for each `UnsupportedMarkers` hit,
sorted. The config record is passed into `install.Plan` as
`install.ConfigWithAdapters(recs)` where each `AdapterRecord` is
`{Name, Selection, Fidelity}` taken from `detected`; `.rtdd/config.yaml` is still created
and never overwritten, so an existing config is left exactly as it is — `rtdd doctor`
derives fidelity live and is the source of truth, the config block is a human-readable
record of the last first-install.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./cmd/rtdd/ -count=1
```

Expected: the five new tests PASS and every existing `cmd/rtdd` test stays green —
in particular `TestWhichDetectsTheAdapterOnAStockPostInitRepo` and the init tests that
assert the five installed paths, all of which run in a detectable Python repo.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/ internal/install/ cmd/rtdd/
git commit -m "init: detect before installing; refuse a no-adapter repo without --force"
```

---

## Task 6 — `cmd/rtdd`: `doctor` reports selection fidelity

**Discharges:** spec §6 (the `doctor` honesty surface) and §4.5 (`rtdd doctor` validates host adapters and names the failing field).

**Files:** `cmd/rtdd/doctor.go`, `cmd/rtdd/doctor_fidelity_test.go`

**Interfaces:**

*Consumes:* `adapter.Available`, `adapter.LoadHostReport`, `adapter.DetectAll`,
`adapter.Fidelity`, `doctor.Hubs`, `RenderDoctor`.

*Produces:*
```go
// FidelityRow is one detected adapter as doctor reports it.
type FidelityRow struct {
	Name     string
	Src      string // "built-in" or the host path
	Override bool   // a host adapter that replaced a built-in
	Fidelity adapter.Fidelity
	Why      string // one clause: why this fidelity, and for none what to do about it
}

// RenderFidelity formats the fidelity block printed above the fan-out table. Pure.
func RenderFidelity(rows []FidelityRow, invalid []adapter.Invalid) string
```

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/doctor_fidelity_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

func TestRenderFidelityExecutionDerived(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: "built-in", Fidelity: adapter.FidelityExecution,
		Why: "per-test coverage recorded from a real run",
	}}, nil)

	for _, want := range []string{"selection fidelity", "python", "built-in", "execution-derived"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// The decision this plan pins: fidelity none is stated plainly, with the fix, and it is
// not dressed up as a static selection.
func TestRenderFidelityNoneSaysWhatIsMissingAndHowToFixIt(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "elixir", Src: ".rtdd/adapters/elixir.yaml", Fidelity: adapter.FidelityNone,
		Why: "declares selection: static but no test_for templates and no importscan command",
	}}, nil)

	for _, want := range []string{
		"elixir",
		".rtdd/adapters/elixir.yaml",
		"none",
		"no test_for templates and no importscan command",
		"full suite",
		"Fix:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "execution-derived") {
		t.Errorf("a fidelity: none row must not mention execution-derived:\n%s", got)
	}
}

func TestRenderFidelityMarksAHostOverride(t *testing.T) {
	got := RenderFidelity([]FidelityRow{{
		Name: "python", Src: ".rtdd/adapters/python.yaml", Override: true,
		Fidelity: adapter.FidelityExecution, Why: "per-test coverage recorded from a real run",
	}}, nil)
	if !strings.Contains(got, "overrides built-in") {
		t.Errorf("a host override must be named, never silent:\n%s", got)
	}
}

// §4.5: doctor validates host adapters and names the failing field. It is the ONE lenient
// reader — a broken file is reported, not fatal, because doctor is the command you run to
// find out what is wrong.
func TestRenderFidelityNamesAnInvalidHostAdapter(t *testing.T) {
	got := RenderFidelity(nil, []adapter.Invalid{{
		Path: ".rtdd/adapters/broken.yaml",
		Err:  errString(`adapter: .rtdd/adapters/broken.yaml: subset "go test ./..." has no {tests} placeholder`),
	}})

	for _, want := range []string{"broken.yaml", "{tests}", "not loaded"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// A repo with no adapter at all is the state --force leaves behind. It is still `none`,
// and spec §6 says the field is never null.
func TestRenderFidelityWithNoAdapters(t *testing.T) {
	got := RenderFidelity(nil, nil)
	if !strings.Contains(got, "none") {
		t.Errorf("no-adapter output must still state a fidelity:\n%s", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./cmd/rtdd/ -run TestRenderFidelity -count=1
```

Expected: `[build failed]` — `undefined: RenderFidelity` and `undefined: FidelityRow`.

- [ ] **Step 3: Write minimal implementation**

Add to `cmd/rtdd/doctor.go` a `FidelityRow`, a pure `RenderFidelity`, and the wiring in
`cmdDoctor` that builds the rows from `adapter.Available` + `adapter.DetectAll` +
`adapter.LoadHostReport` and prints the block **above** `RenderDoctor`. Shape:

```
selection fidelity

  python   built-in                     execution-derived
      per-test coverage recorded from a real run

  elixir   .rtdd/adapters/elixir.yaml   none
      declares selection: static but no test_for templates and no importscan command,
      so RTDD cannot select anything narrower than the full suite (T2).
      Fix: add a test_for template that resolves to a real test file, or an importscan
      command, then re-run rtdd doctor.

  not loaded: .rtdd/adapters/broken.yaml
      adapter: .rtdd/adapters/broken.yaml: subset "go test ./..." has no {tests} placeholder
```

Rules the formatter must hold to:

- The `Why` clause is always printed; a fidelity with no reason is not an honesty surface.
- A `none` row always ends with the `Fix:` line and always contains the words
  `full suite`, because that is the operational consequence for the agent reading it.
- An `Override` row prints `overrides built-in` after the source path.
- `cmdDoctor` still returns **0** in every one of these cases, including an invalid host
  adapter. `doctor` reports; the exit code is not an opinion (`00-interfaces.md`).

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./cmd/rtdd/ -run TestRenderFidelity -count=1 -v
```

Expected: `--- PASS` for all five, then `go test ./cmd/rtdd/ -count=1` green, including the
existing `RenderDoctor` tests — the fan-out table's own output must be unchanged.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/
git commit -m "doctor: report per-adapter selection fidelity, host overrides and unloadable host adapters"
```

---

## Task 7 — `internal/adapter` + `cmd/rtdd`: unmet `requires` is a named finding

**Discharges:** spec §6 (the `doctor` honesty surface — a repository is told what it cannot do) and §4.2 (the declaration the finding reads, added by spec §4.3), per PRD #229 AC9.

**Files:** `internal/adapter/requires.go`, `internal/adapter/requires_test.go`, `cmd/rtdd/doctor.go`, `cmd/rtdd/init.go`, `cmd/rtdd/doctor_requires_test.go`

**Interfaces:**

*Consumes:* `Adapter.Requires` (Task 1), `RenderFidelity` (Task 6), the `init` gate (Task 5).

*Produces:*
```go
// Unmet returns the requirements whose bin is not resolvable, in declaration order.
// lookPath is injected so the test never depends on what is installed on the machine
// running it; cmd/rtdd passes exec.LookPath.
func (a *Adapter) Unmet(lookPath func(string) (string, error)) []Requirement

// UnmetFinding pairs one unmet requirement with the adapter that declared it.
type UnmetFinding struct {
	Adapter string
	Req     Requirement
}

// RenderRequirements formats the prerequisite findings. Pure; empty input renders "".
func RenderRequirements(findings []adapter.UnmetFinding) string
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/requires_test.go`:

```go
package adapter

import (
	"errors"
	"testing"
)

// lookPath is injected: a test that asked the machine whether `node` exists would pass on
// a developer's laptop and fail on a minimal CI image, which is the opposite of a guard.
func fakeLookPath(present ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, p := range present {
		set[p] = true
	}
	return func(bin string) (string, error) {
		if set[bin] {
			return "/usr/bin/" + bin, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
}

func TestUnmetReportsMissingBinariesInDeclarationOrder(t *testing.T) {
	a := &Adapter{Name: "vitest", Requires: []Requirement{
		{Bin: "node", Reason: "runs vitest and the importscan script"},
		{Bin: "npx", Reason: "resolves the vitest binary from the lockfile"},
	}}

	got := a.Unmet(fakeLookPath("node"))
	if len(got) != 1 {
		t.Fatalf("Unmet = %v, want exactly the npx entry", got)
	}
	if got[0].Bin != "npx" || got[0].Reason == "" {
		t.Errorf("Unmet[0] = %+v, want the npx requirement with its reason", got[0])
	}
}

func TestUnmetIsEmptyWhenEverythingIsPresent(t *testing.T) {
	a := &Adapter{Name: "vitest", Requires: []Requirement{{Bin: "node", Reason: "runs vitest"}}}
	if got := a.Unmet(fakeLookPath("node")); len(got) != 0 {
		t.Errorf("Unmet = %v, want none", got)
	}
}

// An adapter declaring nothing has nothing to be missing, and a nil adapter is the
// no-adapter repo.
func TestUnmetWithNoRequirements(t *testing.T) {
	if got := (&Adapter{Name: "python"}).Unmet(fakeLookPath()); len(got) != 0 {
		t.Errorf("Unmet = %v, want none", got)
	}
	var nilAdapter *Adapter
	if got := nilAdapter.Unmet(fakeLookPath()); len(got) != 0 {
		t.Errorf("nil adapter Unmet = %v, want none", got)
	}
}
```

`cmd/rtdd/doctor_requires_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Spec §4.3 / PRD #229 AC9: the finding names the missing binary, the adapter needing it,
// and the reason — at doctor time, not as a mid-run parse failure against a report file
// that was never written.
func TestRenderRequirementsNamesBinAdapterAndReason(t *testing.T) {
	got := RenderRequirements([]adapter.UnmetFinding{{
		Adapter: "vitest",
		Req:     adapter.Requirement{Bin: "npx", Reason: "resolves the vitest binary from the lockfile"},
	}})

	for _, want := range []string{"npx", "vitest", "resolves the vitest binary from the lockfile", "not on PATH"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not contain %q:\n%s", want, got)
		}
	}
}

// No findings must print nothing at all: a heading over an empty list reads as a problem.
func TestRenderRequirementsIsSilentWhenNothingIsMissing(t *testing.T) {
	if got := RenderRequirements(nil); got != "" {
		t.Errorf("RenderRequirements(nil) = %q, want the empty string", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ ./cmd/rtdd/ -run 'TestUnmet|TestRenderRequirements' -count=1
```

Expected: `[build failed]` in both packages — `a.Unmet undefined (type *Adapter has no field or method Unmet)` and `undefined: RenderRequirements`, `undefined: adapter.UnmetFinding`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/requires.go`:

```go
package adapter

// Unmet returns the requirements whose bin does not resolve, in declaration order.
//
// lookPath is a parameter rather than a direct exec.LookPath call so the engine stays
// testable against a fixed PATH: what is installed on the machine running the tests must
// never decide whether this function is correct.
func (a *Adapter) Unmet(lookPath func(string) (string, error)) []Requirement {
	if a == nil {
		return nil
	}
	var out []Requirement
	for _, r := range a.Requires {
		if _, err := lookPath(r.Bin); err != nil {
			out = append(out, r)
		}
	}
	return out
}

// UnmetFinding pairs one unmet requirement with the adapter that declared it.
type UnmetFinding struct {
	Adapter string
	Req     Requirement
}
```

`cmd/rtdd/doctor.go` gains `RenderRequirements`, printed directly under the fidelity block:

```
prerequisites
  npx is not on PATH — needed by adapter vitest: resolves the vitest binary from the lockfile
```

`cmdDoctor` builds the findings with `exec.LookPath` over the detected adapters and still
returns **0**. `cmdInit` prints the same block after a successful install and also returns
0 — decision 4 above: the repo has an adapter, so §5's gate is satisfied, and a binary
missing from this machine is not a reason to refuse to install.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ ./cmd/rtdd/ -run 'TestUnmet|TestRenderRequirements' -count=1 -v
```

Expected: `--- PASS` for all five, then `go test ./... -count=1` green.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/ cmd/rtdd/
git commit -m "doctor: report unmet adapter prerequisites by bin, adapter and reason"
```

---

## Task 8 — `internal/contract`: freeze `adapters/python.yaml` and record the contract

**Discharges:** spec §4.2 (the contract as recorded in `00-interfaces.md`) and §4.1's byte-identical-Python regression requirement, which is what the freeze protects.

**Files:** `internal/contract/adapter_freeze_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* `repoRoot`, `readRepoFile` (existing helpers in `internal/contract`).

*Produces:* no engine symbol. A guard test, plus the `00-interfaces.md` entries for
`Adapter.Selection`, `Adapter.ReportPath`, `Adapter.IDTemplate`, `Adapter.TestFor`,
`Adapter.Importscan`, `Adapter.Requires`, `Adapter.Src`, `adapter.Requirement`,
`adapter.Fidelity`, `adapter.Unmet`, `adapter.UnmetFinding`, `adapter.LoadHost`,
`adapter.LoadHostReport`, `adapter.Available`, `adapter.Invalid`, `adapter.DetectAll`,
`adapter.UnsupportedMarkers`, `install.ConfigWithAdapters`,
`install.WithNoAdapterCaveat`, `RenderFidelity` and `RenderRequirements`.

- [ ] **Step 1: Write the failing test**

`internal/contract/adapter_freeze_test.go`:

```go
package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// adapters/python.yaml is byte-frozen for the multi-language PRD. Spec §4.1 makes a
// seeded Python repo's selection byte-identical to today a REQUIREMENT, and the only
// adapter that has ever produced a map is the cheapest thing to hold still. Every v2
// field is optional precisely so this file needs no edit.
//
// To change it, change this digest in the same commit and say in the message which spec
// section licenses the edit.
const pythonAdapterSHA256 = "<paste the digest printed by step 2 here>"

func TestPythonAdapterIsByteFrozen(t *testing.T) {
	sum := sha256.Sum256([]byte(readRepoFile(t, "adapters/python.yaml")))
	got := hex.EncodeToString(sum[:])
	if got != pythonAdapterSHA256 {
		t.Errorf("adapters/python.yaml changed: sha256 = %s, want %s\n"+
			"It is byte-frozen for the multi-language PRD (spec §4.1). If the edit is "+
			"licensed, update pythonAdapterSHA256 in the same commit.", got, pythonAdapterSHA256)
	}
}

// The v1 adapter must not have acquired a v2 key by accident.
func TestPythonAdapterDeclaresNoV2Keys(t *testing.T) {
	src := readRepoFile(t, "adapters/python.yaml")
	for _, key := range []string{"selection:", "report_path:", "id_template:", "test_for:", "importscan:", "requires:"} {
		if strings.Contains(src, "\n"+key) {
			t.Errorf("adapters/python.yaml declares %q; it is byte-frozen and defaults to v1 behaviour", key)
		}
	}
}

// Every contract addition this milestone makes is recorded where the plans bind.
func TestInterfacesRecordsTheV2ContractAdditions(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")
	for _, name := range []string{
		"Fidelity", "Requirement", "Unmet", "UnmetFinding", "LoadHost", "LoadHostReport",
		"Available", "DetectAll", "UnsupportedMarkers", "ConfigWithAdapters",
		"WithNoAdapterCaveat", "RenderFidelity", "RenderRequirements",
	} {
		if !strings.Contains(src, name) {
			t.Errorf("00-interfaces.md does not record %q", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/contract/ -run 'TestPythonAdapter|TestInterfacesRecords' -count=1
```

Expected: `--- FAIL: TestPythonAdapterIsByteFrozen` with
`adapters/python.yaml changed: sha256 = <the real digest>, want <paste the digest printed by step 2 here>`
— the failure prints the digest to paste — and `--- FAIL: TestInterfacesRecordsTheV2ContractAdditions`
with one line per unrecorded name.

- [ ] **Step 3: Write minimal implementation**

Paste the digest the failure printed into `pythonAdapterSHA256`, and add the contract
additions to `docs/plans/00-interfaces.md` under the existing adapter section, each with
its one-line meaning and the spec section that licenses it. Nothing else changes:
`adapters/python.yaml` is not edited by this or any task in this plan.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/contract/ -count=1 -v -run 'TestPythonAdapter|TestInterfacesRecords'
```

Expected: `--- PASS: TestPythonAdapterIsByteFrozen`, `--- PASS: TestPythonAdapterDeclaresNoV2Keys`,
`--- PASS: TestInterfacesRecordsTheV2ContractAdditions`, `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/contract/ docs/plans/00-interfaces.md
git commit -m "contract: byte-freeze adapters/python.yaml for the multi-language PRD; record the v2 additions"
```

---

## Definition of Done

**Behaviour**

- [ ] An adapter declaring `selection: static`, `coverage: none`, `report: junit-xml`,
      `report_path`, `id_template`, `test_for`, `importscan` and `requires` loads and
      validates.
- [ ] `coverage: none` with `selection: coverage`, and a non-empty `seed` under
      `selection: static`, are both exit-2 errors naming both keys, with the wording tabled
      above; so is `selection: static` with any `coverage` but `none`.
- [ ] `seed` is required under `selection: coverage` and forbidden under
      `selection: static`.
- [ ] A `requires` entry missing `bin` or `reason` is an exit-2 error naming its index.
- [ ] `.rtdd/adapters/*.yaml` loads; a host adapter replaces the built-in of the same name;
      two host adapters of one name is an error naming both files.
- [ ] `rtdd init` exits 2 in a repo with no detected adapter, writes nothing, names the
      unsupported toolchains it found, points at `.rtdd/adapters/`, and mentions `--force`.
      `--dry-run` gates identically.
- [ ] `rtdd init --force` installs, and the installed `SKILL.md` carries the no-adapter
      caveat in its first paragraph.
- [ ] `rtdd init` on a served repo records `name`, `selection` and `fidelity` per detected
      adapter in a newly created `.rtdd/config.yaml`, and never rewrites an existing one.
- [ ] `rtdd doctor` prints a fidelity block naming each detected adapter, its source,
      whether it overrides a built-in, its fidelity, and — for `none` — what is missing and
      how to fix it. It exits 0 even with an unloadable host adapter, which it names along
      with the failing field.
- [ ] `rtdd doctor` reports every unmet `requires` entry by missing binary, the adapter
      that needs it, and its `reason`; `rtdd init` prints the same block after installing.
      Neither exits non-zero because of one.
- [ ] A test reproduces the original defect: `init` in a fixture repo containing only
      `package.json` and a `.ts` file does not create `.claude/skills/rtdd/SKILL.md`.

**The regressions this milestone had to avoid**

- [ ] `adapters/python.yaml` is byte-identical to its state at `a830349`, asserted by a
      sha256 guard test, and declares no v2 key.
- [ ] Every existing `internal/adapter`, `cmd/rtdd`, `internal/install` and
      `internal/contract` test passes unchanged in behaviour; no test was deleted, skipped
      or weakened to accommodate a new rule.
- [ ] `Detect` still errors on 0 and on ≥2 adapters. Polyglot selection is untouched.
- [ ] `rtdd which`, `rtdd run`, `rtdd seed` and `rtdd status` produce byte-identical output
      to before this milestone in a seeded Python repo.

**Contract**

- [ ] `docs/plans/00-interfaces.md` records every addition; no name in the implementation
      diverges from it.
- [ ] The `Fidelity` wire strings are exactly `execution-derived`, `static`, `none`.

**Hygiene**

- [ ] `scripts/ci-local.sh` exits 0.
- [ ] `go.mod` still lists exactly `gopkg.in/yaml.v3` and `modernc.org/sqlite`.
- [ ] Every task's commit is separate and its test was seen to fail before its
      implementation was written.

## What this plan deliberately leaves undone

Everything below is real work the multi-language design wants; none of it belongs to this
PRD (#229), and no task above may start it. Each line names the milestone from the spec §8
table and the sibling or later PRD that owns it — a scope this plan may not silently
absorb.

- **The `TS` tier itself — spec §4.1, milestone M6b, the NEXT PRD in the chain.** This
  plan makes an adapter able to *declare* `selection: static` and reports the fidelity that
  declaration implies. It does not insert
  a tier between T1 and T2, does not resolve `test_for` templates against the filesystem,
  does not run `importscan`, and does not rank by correspondence, import distance or path
  proximity. `internal/selector` is not opened by any task here. Until that PRD lands, an
  adapter declaring `selection: static` detects, validates and reports — and still selects
  nothing, which is exactly what its `doctor` line says.
- **The `junit-xml` parser and the static runner — spec §4.3, milestone M6c, a SIBLING
  PRD.** `report: junit-xml` is accepted by the contract here so host adapters can be
  written against it, `report_path` and `id_template` are validated as its required
  companions (audit A6), and `requires` is parsed and reported so an adapter can declare the
  packages §4.3's table names. No parser exists in `internal/report`, nothing runs a static
  suite, no `{report}` placeholder is expanded, and the id round-trip through
  `id_template` is not implemented. An adapter declaring `junit-xml` fails at run time on
  the existing unsupported-report path, and that is the intended state at the end of this
  PRD.
- **The shipped non-Python adapter YAML set — spec §8, milestone M6d, a LATER PRD.** No
  adapter file for TypeScript/JavaScript (Vitest, Jest), Go, Rust, Java/Kotlin, Ruby, C# or
  PHP is added by
  this plan; `adapters/` gains no file. The `staticYAML` fixture in the tests above is a
  test fixture, not a shipped adapter definitions set. Polyglot detection — `Detect`
  returning a set, and the adapter tag on rows, selections and subset invocations
  (spec §4.4) — is that later PRD's as well; `DetectAll` is exported here only so `init`
  can gate on "at least one".
- **The remaining §6 surfaces — milestone M6e, a later PRD.** `rtdd which` / `rtdd run`
  human output, the `selection_fidelity` field on `--json`, the static-tier entry in the
  `warnings` array,
  and the `PROTOCOL.md` → `SKILL.md`/`AGENTS.md`/`.mdc` regeneration all still describe a
  coverage-only world. Only `doctor`'s block and `init`'s refusal move here.
- **The evidence table — spec §7, milestone M6e, a later PRD.** The
  static-vs-coverage-vs-`path`-vs-T2 comparison over the flask and httpie replay corpus,
  with `p50`/`p90`/`worst`, is not run, not
  published, and not cited by this plan. Nothing here may claim the static tier beats the
  `path` baseline; that is a measurement, and it has not been made.
- **Per-test coverage outside Python.** Audit A5 stands and spec §3 keeps it a non-goal:
  no Vitest coverage provider, no injected `TestMain`, no `ClearCounters()` codegen, in
  this PRD or any other in the chain.
