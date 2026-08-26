# RTDD M1b — Python Adapter, Seed and Run

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the full RTDD loop work against a real Python repo — `rtdd seed` produces a map from one instrumented pytest run, and `rtdd run` executes a selection, refreshes rows by union, and never silently corrupts the map.

**Architecture:** `internal/adapter` loads `adapters/python.yaml`, detects the host repo, classifies files against globs, and expands command templates into argv. `internal/runner` executes those argv in chunks with `COVERAGE_CORE=ctrace` forced, scans the combined output for coverage.py's `no-sysmon-context` warning, maps pytest exit codes 4/5 to fatal errors, and merges per-chunk results. `internal/coverage` reads `.coverage` SQLite directly (decoding the `numbits` packed bitmap, or the `arc` table when the host enables branch coverage) and `internal/report` parses `pytest --report-log` JSONL for the `s`/`d` fields that no coverage report carries; `cmd/rtdd` joins them into `mapstore.Row`s — `Replace` on seed, `Union` on run, never the other way round.

**Tech Stack:** Go 1.24+, modernc.org/sqlite (pure Go, no cgo), gopkg.in/yaml.v3, stdlib testing

## Global Constraints

*(copied verbatim from `00-interfaces.md`)*

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except: `modernc.org/sqlite` (pure-Go, no
  cgo — required so the binary stays static), and `gopkg.in/yaml.v3`. No test framework
  beyond stdlib `testing`.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

### Process exit codes for `rtdd`

| Code | Meaning |
|---|---|
| 0 | Success. **Includes** an empty selection and a non-empty uncovered report — these are signals, not failures |
| 1 | A test failed during `run`/`verify` |
| 2 | Usage or configuration error (bad flag, unparseable adapter, no adapter detected) |
| 3 | Fatal environment error (`no-sysmon-context` warning observed, `.coverage` unreadable, git unavailable) |

RTDD never exits nonzero to express a policy opinion. See spec §2 non-goals.

**M1b binding for the two runner-originated failures:**

| Condition | Runner returns | CLI exits |
|---|---|---|
| `no-sysmon-context` seen in combined stdout+stderr | `runner.ErrSysmonContext` | **3** |
| a chunk exits 4 (`bad-selector`) or 5 (`no-tests-collected`) | `*runner.FatalExitError` | **2** |
| `.coverage` missing / unreadable after a chunk | wrapped `error` | **3** |
| a chunk exits 1 | `RunResult.ExitCode == 1`, no error | **1** |

---

## Measured ground truth

Everything below was **measured on this machine** (Python 3.13.5, pytest 9.0.3, coverage.py
7.15.4, pytest-cov, pytest-reportlog 1.0.0) before this plan was written. Do not "improve"
these constants from memory — re-measure if you doubt them.

### `.coverage` SQLite schema (verbatim from `sqlite_master`)

```sql
CREATE TABLE coverage_schema (version integer);
CREATE TABLE meta (key text, value text, unique (key));
CREATE TABLE file (id integer primary key, path text, unique (path));
CREATE TABLE context (id integer primary key, context text, unique (context));
CREATE TABLE line_bits (
    file_id integer, context_id integer, numbits blob,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id));
CREATE TABLE arc (
    file_id integer, context_id integer, fromno integer, tono integer,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id, fromno, tono));
CREATE TABLE tracer (file_id integer primary key, tracer text,
    foreign key (file_id) references file (id));
```

`meta` holds `('version','7.15.4')` and `('has_arcs','0')`.

`file.path` is **absolute** unless the host sets `[run] relative_files = True`. Measured
absolute: `/tmp/.../m1b/src/logic.py`. Normalise every one through `paths.Normalize`.

### `numbits` is a packed bitmap, not a line list

Byte `i`, bit `j` set ⇒ line `i*8 + j` is covered. Measured pairs, cross-checked against
`coverage.numbits.numbits_to_nums`:

| hex | decoded lines | file/context |
|---|---|---|
| `1211` | `[1, 4, 8, 12]` | `src/logic.py`, empty context |
| `ea` | `[1, 3, 5, 6, 7]` | `src/constants.py`, empty context |
| `20` | `[5]` | `src/logic.py`, `tests/test_a.py::test_add\|run` |
| `0002` | `[9]` | `src/logic.py`, `tests/test_b.py::test_mul\|run` |
| `01` | `[0]` | empty `src/__init__.py` — **line 0 is not a real line, drop it** |

### Context strings

```
''                                            <- import-time; NOT a test
'tests/test_a.py::test_add|run'
'tests/test_a.py::test_param[1-one two]|run'  <- space inside the id
'tests/test_a.py::test_param[2-a-b]|run'      <- '-' inside the id
'tests/test_pipe.py::test_pipe[a|b]|run'      <- '|' inside the id
'tests/test_c.py::test_with_fixture|setup'
'tests/test_c.py::test_with_fixture|teardown'
```

Split on the **last** `|`, and only strip it when the suffix is exactly `run`, `setup`, or
`teardown`. All of the ids above round-trip: passing them back as separate argv elements
re-selects the same tests (verified, exit 0).

### `pytest --report-log` JSONL

Envelope key is `$report_type` ∈ `SessionStart` | `CollectReport` | `TestReport` |
`SessionFinish`. Only `TestReport` matters. Its keys: `nodeid`, `location`, `when`
(`setup`|`call`|`teardown`), `outcome` (`passed`|`failed`|`skipped`), `duration` (float
**seconds**), `start`, `stop`, `longrepr`.

Measured behaviour that the naive "read the call phase" reader gets wrong:

- A **skipped** test emits `setup`(`skipped`) and `teardown`(`passed`) and **no `call` entry at all**.
- A test whose **fixture raises** emits `setup`(`failed`) and `teardown`(`passed`) and no `call`.
- `duration` is seconds; `d` in the map is milliseconds.

### pytest exit codes (measured)

| invocation | exit |
|---|---|
| `pytest tests/test_a.py` (all pass) | 0 |
| `pytest tests/test_b.py` (one failure) | 1 |
| `pytest "tests/test_a.py::test_nonexistent"` | **4** |
| `pytest "tests/test_nope.py::test_x"` | **4** |
| `pytest --badflagxyz` | **4** |
| `pytest -k zzzznomatch` | **5** |

### `COVERAGE_CORE=sysmon` (audit A7 — the silent corruption path)

- Under `sysmon` with `--cov-context=test`, 4 tests ran and **1** non-empty context was recorded. Exit status was unaffected.
- The warning text is emitted **on STDOUT**, inside pytest's warnings summary. **STDERR was empty.** Scanning stderr alone finds nothing. Scan the combined stream.
- Exact captured text:
  ```
  CoverageWarning: Dynamic contexts aren't supported with core=sysmon; context data may be incomplete (no-sysmon-context); see https://coverage.readthedocs.io/en/7.15.4/messages.html#warning-no-sysmon-context
  ```
- `COVERAGE_CORE=ctrace` in the environment **does** override `.coveragerc [run] core = sysmon` (verified: all contexts recorded, no warning). So `Adapter.Env` forcing is sufficient — but the runner still scans, because a future coverage.py could change that.

### Coverage scope: seed and subset must agree

- Bare `--cov` (no value) honours the host's `[run] source` **and** `[run] omit` (verified: `fixt.py` omitted).
- `--cov=` (empty value, what `--cov={src}` produces when `{src}` resolves to nothing) makes pytest **exit 1** and record nothing.
- `--cov=src` also honours the host's `omit`.
- Bare `--cov` with no host config measures every imported file, including test modules — identical for seed and subset, which is the property that matters.

**Therefore the shipped `adapters/python.yaml` uses bare `--cov` and RTDD never guesses a
source list.** This is a deliberate deviation from spec §8's `--cov={src}`; guessing `{src}`
is exactly the "seed and subset disagree on scope" failure the audit names.

### Branch coverage empties `line_bits`

With `[run] branch = True` (common in real repos), measured: `line_bits` has **0 rows** and
`arc` has 60. The contract's `line_bits` query returns nothing and the map comes out empty
with exit 0 — the same silent-corruption shape as sysmon.

Recovery rule, verified to reproduce the `line_bits` answer **exactly**:

> executed lines = `{fromno : fromno > 0} ∪ {tono : tono > 0}`

| context | arcs (fromno,tono) | positive-only union | `line_bits` answer |
|---|---|---|---|
| `src/logic.py` `''` | (-1,1)(1,4)(4,8)(8,12)(12,-1) | `[1,4,8,12]` | `[1,4,8,12]` ✓ |
| `src/constants.py` `''` | (-5,5)(-1,1)(1,3)(3,5)(5,6)(5,7)(6,-1)(6,5)(7,-5) | `[1,3,5,6,7]` | `[1,3,5,6,7]` ✓ |
| `src/logic.py` `…test_add\|run` | (-4,5)(5,-4) | `[5]` | `[5]` ✓ |

### Chunking

`.coverage` is **erased at the start of every pytest run** unless `--cov-append` is passed
(measured: chunk 2 left only chunk 2's contexts). RTDD therefore reads and merges
`.coverage` **after each chunk** in Go rather than relying on `--cov-append`, which would
also silently absorb a stale `.coverage` from an unrelated earlier run.

`COVERAGE_FILE=.coverage` in the environment beats a host `[run] data_file = weird/place.db`
(verified), so the adapter pins the data file and the runner always knows where to look.

8,000 realistic ids measured at 460 KB (avg 59 B/id); spec §8 cites 613 KB for longer ids.
Either way: fits Linux `ARG_MAX` (2,097,152 measured) and is 57–74× over the Windows `CMD`
8,191-char limit. pytest has no argfile option.

---

## File Structure

| File | Single responsibility |
|---|---|
| `adapters/python.yaml` | **NEW** — the Python adapter definition: detection globs, forced env, command templates, exit-code map, file-class globs |
| `adapters/adapters.go` | **NEW** — `package adapters`; a `//go:embed *.yaml` `embed.FS` so the binary ships the adapter definitions and stays a single static file |
| `internal/adapter/adapter.go` | **NEW** — the `Adapter` struct, YAML parsing + validation, `Load`/`LoadFS`/`LoadAll`/`Builtin` |
| `internal/adapter/glob.go` | **NEW** — `**`-aware glob matcher (stdlib `path.Match` has no `**`) |
| `internal/adapter/classify.go` | **NEW** — `IsTestFile`/`IsOpaque`/`IsFullEscalate`/`IsInstrumentable` |
| `internal/adapter/detect.go` | **NEW** — `Detect`: exactly one adapter must match, zero or many is an error |
| `internal/adapter/expand.go` | **NEW** — `Expand` and `ExpandTests`: command template → argv, with test ids spliced as separate argv elements |
| `internal/coverage/numbits.go` | **NEW** — `Numbits`: decode coverage.py's packed line bitmap |
| `internal/coverage/context.go` | **NEW** — `NormalizeContext`: strip the `\|run`/`\|setup`/`\|teardown` phase suffix |
| `internal/coverage/result.go` | **NEW** — `TestCoverage`, `Result`, `Result.Merge` (per-chunk union) |
| `internal/coverage/sqlite.go` | **NEW** — `ReadSQLite`: the `line_bits` path, the `arc` path for `has_arcs=1`, and path normalisation through `internal/paths` |
| `internal/report/reportlog.go` | **NEW** — `ReadReportLog`: pytest-reportlog JSONL → `[]Outcome`, with the setup-skip / setup-error / teardown-error rules |
| `internal/pytestfixture/fixture.go` | **NEW** — materialises a tiny real pytest project + git repo in a temp dir for integration tests |
| `internal/runner/chunk.go` | **NEW** — `Chunk`, `MaxArgvBytes` |
| `internal/runner/errors.go` | **NEW** — `ErrSysmonContext`, `FatalExitError`, `hasSysmonWarning` |
| `internal/runner/run.go` | **NEW** — `Run`, `Seed`, `List`, env merging, per-chunk exec + coverage merge |
| `cmd/rtdd/meta.go` | **NEW** — `.rtdd/meta.json` read/write and cycle counting |
| `cmd/rtdd/rows.go` | **NEW** — join `RunResult.Coverage` + `RunResult.Outcomes` into `[]mapstore.Row`; repo-root discovery; adapter detection; run-error → exit-code mapping |
| `cmd/rtdd/seed.go` | **NEW** — `rtdd seed`: full instrumented run, `mapstore.Replace` (**the only caller of Replace in the tree**) |
| `cmd/rtdd/run.go` | **NEW** — `rtdd run`: select, execute, `mapstore.Union` (**never Replace**), exit codes |
| `cmd/rtdd/main.go` | **MODIFIED** — add the `seed` and `run` cases to the command switch |
| `docs/plans/00-interfaces.md` | **MODIFIED** — record the four contract additions this milestone makes |
| `go.mod` / `go.sum` | **MODIFIED** — add `gopkg.in/yaml.v3` and `modernc.org/sqlite` |

**M1a is done. Consume it, do not reimplement it:** `internal/paths` (`Normalize`),
`internal/mapstore` (`Row`, `Map`, `Load`, `Save`, `Union`, `Replace`, `TestsCovering`),
`internal/gitctx` (`ChangedSet`, `HeadSHA`, `CommitDistance`, `Older`), `internal/selector`
(`Select`, `Rank`, `Inputs`, `Selection`, `DefaultConfig`).

> If M1a left a placeholder `internal/adapter/adapter.go` containing only the `Adapter`
> struct (selector imports it), replace that file wholesale in Task 1 and keep the field
> names byte-identical to `00-interfaces.md`.

---

## Task 1 — `internal/adapter`: the `Adapter` type, YAML loading, and `adapters/python.yaml`

**Files:** `go.mod`, `adapters/python.yaml`, `adapters/adapters.go`, `internal/adapter/adapter.go`, `internal/adapter/adapter_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing from M1a.

*Produces:*
```go
type Adapter struct {
    Name          string            `yaml:"name"`
    Detect        []string          `yaml:"detect"`
    Env           map[string]string `yaml:"env"`
    Seed          string            `yaml:"seed"`
    Subset        string            `yaml:"subset"`
    List          string            `yaml:"list"`
    Coverage      string            `yaml:"coverage"`
    Report        string            `yaml:"report"`
    FailFastFlag  string            `yaml:"failfast_flag"`
    TestGlobs     []string          `yaml:"test_globs"`
    SourceGlobs   []string          `yaml:"source_globs"`
    ExitCodes     map[int]string    `yaml:"exit_codes"`
    Opaque        []string          `yaml:"opaque"`
    FullEscalate  []string          `yaml:"full_escalate"`
}

func Load(path string) (*Adapter, error)
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error)  // CONTRACT ADDITION
func LoadAll(dir string) ([]*Adapter, error)
func Builtin() ([]*Adapter, error)                       // CONTRACT ADDITION
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/adapter_test.go`:

```go
package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

const goodYAML = `name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
env:
  COVERAGE_CORE: ctrace
  COVERAGE_FILE: .coverage
seed: "pytest --cov --cov-context=test --cov-report= --report-log={log}"
subset: "pytest {tests} --cov --cov-context=test --cov-report= --report-log={log}"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
failfast_flag: "-x"
test_globs: ["tests/**/*.py", "**/test_*.py"]
source_globs: ["**/*.py"]
exit_codes:
  4: bad-selector
  5: no-tests-collected
opaque: ["**/*.yaml", "**/fixtures/**"]
full_escalate: ["requirements.txt", "**/conftest.py"]
`

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "python.yaml", goodYAML)

	a, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Name != "python" {
		t.Errorf("Name = %q, want %q", a.Name, "python")
	}
	if got, want := a.Env["COVERAGE_CORE"], "ctrace"; got != want {
		t.Errorf("Env[COVERAGE_CORE] = %q, want %q — sysmon silently drops ~90%% of contexts", got, want)
	}
	if got, want := a.Env["COVERAGE_FILE"], ".coverage"; got != want {
		t.Errorf("Env[COVERAGE_FILE] = %q, want %q", got, want)
	}
	if got, want := a.ExitCodes[4], "bad-selector"; got != want {
		t.Errorf("ExitCodes[4] = %q, want %q", got, want)
	}
	if got, want := a.ExitCodes[5], "no-tests-collected"; got != want {
		t.Errorf("ExitCodes[5] = %q, want %q", got, want)
	}
	if a.FailFastFlag != "-x" {
		t.Errorf("FailFastFlag = %q, want %q", a.FailFastFlag, "-x")
	}
	if len(a.Detect) != 3 {
		t.Errorf("len(Detect) = %d, want 3", len(a.Detect))
	}
}

func TestLoadRejectsBadAdapters(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"no name", "detect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "name is required"},
		{"no detect", "name: x\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "detect is required"},
		{"no seed", "name: x\ndetect: [\"a\"]\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n", "seed is required"},
		{"no subset", "name: x\ndetect: [\"a\"]\nseed: s\ncoverage: sqlite\nreport: pytest-reportlog\n", "subset is required"},
		{"bad coverage", "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: lcov\nreport: pytest-reportlog\n", "unsupported coverage"},
		{"bad report", "name: x\ndetect: [\"a\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: junit\n", "unsupported report"},
		{"unknown field", goodYAML + "bogus_key: 1\n", "bogus_key"},
		{"not yaml", "\tname: [[[\n", "adapter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeFile(t, dir, "a.yaml", tc.yaml)
			_, err := Load(p)
			if err == nil {
				t.Fatalf("Load(%s) = nil error, want error containing %q", tc.name, tc.want)
			}
			if !contains(err.Error(), tc.want) {
				t.Fatalf("Load error = %q, want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLoadAllSortedByName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "python.yaml", goodYAML)
	writeFile(t, dir, "zeta.yaml", "name: zeta\ndetect: [\"z.toml\"]\nseed: s\nsubset: \"{tests}\"\ncoverage: sqlite\nreport: pytest-reportlog\n")
	writeFile(t, dir, "ignored.txt", "not yaml at all")

	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(LoadAll) = %d, want 2", len(all))
	}
	if all[0].Name != "python" || all[1].Name != "zeta" {
		t.Fatalf("LoadAll names = [%s %s], want [python zeta]", all[0].Name, all[1].Name)
	}
}

func TestBuiltinShipsPython(t *testing.T) {
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
		t.Fatalf("Builtin() has no adapter named python; got %d adapters", len(all))
	}
	if py.Env["COVERAGE_CORE"] != "ctrace" {
		t.Errorf("builtin python Env[COVERAGE_CORE] = %q, want ctrace (audit A7: sysmon drops contexts and exits 0)",
			py.Env["COVERAGE_CORE"])
	}
	if !contains(py.Subset, "{tests}") {
		t.Errorf("builtin python subset %q must contain {tests}", py.Subset)
	}
	if !contains(py.Seed, "--cov-context=test") || !contains(py.Subset, "--cov-context=test") {
		t.Errorf("both seed and subset must pass --cov-context=test; seed=%q subset=%q", py.Seed, py.Subset)
	}
	// Measured: `--cov=` (empty value) makes pytest exit 1 and record nothing, and a
	// guessed {src} makes seed and subset disagree on scope. Bare --cov defers to the
	// host repo's own coverage source/omit config for BOTH.
	if contains(py.Seed, "--cov=") || contains(py.Subset, "--cov=") {
		t.Errorf("adapter must use bare --cov, never --cov=<value>; seed=%q subset=%q", py.Seed, py.Subset)
	}
	if py.ExitCodes[4] != "bad-selector" || py.ExitCodes[5] != "no-tests-collected" {
		t.Errorf("builtin python exit_codes = %v, want 4:bad-selector 5:no-tests-collected", py.ExitCodes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/adapter/
```

Expected: `internal/adapter/adapter_test.go:34:12: undefined: Load` (and `LoadAll`,
`Builtin`, `Adapter`), i.e. `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

```bash
cd /home/claude/rtdd && go get gopkg.in/yaml.v3@v3.0.1
```

`adapters/python.yaml`:

```yaml
# The Python adapter.
#
# COVERAGE_CORE=ctrace is FORCED. With sysmon (the default on Python 3.14+ where
# supported) coverage.py silently drops dynamic contexts and only emits a
# CoverageWarning (no-sysmon-context) while exiting 0 — measured at 1 context
# recorded out of 4 tests run. See spec §4 / audit A7.
#
# COVERAGE_FILE pins the data file so a host `[run] data_file = ...` cannot hide
# the database from the runner. Measured: the env var wins.
#
# `--cov` is passed BARE, with no value. `--cov=<guess>` would make seed and subset
# disagree on coverage scope, and `--cov=` (empty) makes pytest exit 1 and record
# nothing. Bare --cov defers to the host repo's own [run] source / omit settings
# identically for both commands.
name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
env:
  COVERAGE_CORE: ctrace
  COVERAGE_FILE: .coverage
seed: "pytest --cov --cov-context=test --cov-report= --report-log={log}"
subset: "pytest {tests} --cov --cov-context=test --cov-report= --report-log={log}"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
failfast_flag: "-x"
test_globs: ["tests/**/*.py", "**/test_*.py", "**/*_test.py"]
source_globs: ["**/*.py"]
exit_codes:
  4: bad-selector
  5: no-tests-collected
opaque: ["**/*.yaml", "**/*.yml", "**/*.sql", "**/*.html", "**/*.j2", "**/*.json", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "setup.cfg", "pytest.ini", "tox.ini", "poetry.lock", "uv.lock", "**/conftest.py"]
```

`adapters/adapters.go`:

```go
// Package adapters embeds the built-in adapter definitions so the rtdd binary
// ships them and stays a single static file with no runtime data dependency.
package adapters

import "embed"

//go:embed *.yaml
var FS embed.FS
```

`internal/adapter/adapter.go`:

```go
// Package adapter loads language adapter definitions, detects which one applies
// to a host repository, classifies files against its globs, and expands its
// command templates into argv.
package adapter

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"

	"gopkg.in/yaml.v3"

	rtddadapters "github.com/VocanicZ/rtdd/adapters"
)

// Adapter is a language adapter definition. The YAML declares what genuinely is
// declarative; execution and parsing are implemented per language in the engine
// (spec §8, decision D8).
type Adapter struct {
	Name         string            `yaml:"name"`
	Detect       []string          `yaml:"detect"`
	Env          map[string]string `yaml:"env"`
	Seed         string            `yaml:"seed"`
	Subset       string            `yaml:"subset"`
	List         string            `yaml:"list"`
	Coverage     string            `yaml:"coverage"` // "sqlite"
	Report       string            `yaml:"report"`   // "pytest-reportlog"
	FailFastFlag string            `yaml:"failfast_flag"`
	TestGlobs    []string          `yaml:"test_globs"`
	SourceGlobs  []string          `yaml:"source_globs"`
	ExitCodes    map[int]string    `yaml:"exit_codes"`
	Opaque       []string          `yaml:"opaque"`
	FullEscalate []string          `yaml:"full_escalate"`
}

// Load reads a single adapter definition from a file on disk.
func Load(p string) (*Adapter, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("adapter: reading %s: %w", p, err)
	}
	return parse(b, p)
}

// LoadFS reads every *.yaml under dir in fsys, sorted by adapter name.
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("adapter: reading dir %s: %w", dir, err)
	}
	var out []*Adapter
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".yaml" {
			continue
		}
		full := path.Join(dir, e.Name())
		b, err := fs.ReadFile(fsys, full)
		if err != nil {
			return nil, fmt.Errorf("adapter: reading %s: %w", full, err)
		}
		a, err := parse(b, full)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadAll reads every *.yaml in the directory dir on disk.
func LoadAll(dir string) ([]*Adapter, error) { return LoadFS(os.DirFS(dir), ".") }

// Builtin returns the adapters embedded in the binary.
func Builtin() ([]*Adapter, error) { return LoadFS(rtddadapters.FS, ".") }

func parse(b []byte, src string) (*Adapter, error) {
	var a Adapter
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true) // a typo'd key must be an error, never a silently ignored setting
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter %s: %w", src, err)
	}
	if err := validate(&a, src); err != nil {
		return nil, err
	}
	return &a, nil
}

func validate(a *Adapter, src string) error {
	switch {
	case a.Name == "":
		return fmt.Errorf("adapter %s: name is required", src)
	case len(a.Detect) == 0:
		return fmt.Errorf("adapter %s: detect is required", src)
	case a.Seed == "":
		return fmt.Errorf("adapter %s: seed is required", src)
	case a.Subset == "":
		return fmt.Errorf("adapter %s: subset is required", src)
	case a.Coverage != "sqlite":
		return fmt.Errorf("adapter %s: unsupported coverage %q (only \"sqlite\" in v1)", src, a.Coverage)
	case a.Report != "pytest-reportlog":
		return fmt.Errorf("adapter %s: unsupported report %q (only \"pytest-reportlog\" in v1)", src, a.Report)
	}
	return nil
}
```

Now record the two contract additions. In `docs/plans/00-interfaces.md`, under
`## internal/adapter`, replace the line `func LoadAll(dir string) ([]*Adapter, error)` with:

```go
func LoadAll(dir string) ([]*Adapter, error)

// LoadFS reads every *.yaml under dir in fsys, sorted by adapter name.
// LoadAll is LoadFS(os.DirFS(dir), ".").
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error)

// Builtin returns the adapters embedded in the binary from the root `adapters`
// package (//go:embed *.yaml), so rtdd ships as a single static file.
func Builtin() ([]*Adapter, error)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/
```

Expected: `ok  	github.com/VocanicZ/rtdd/internal/adapter`

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add go.mod go.sum adapters/ internal/adapter/ docs/plans/00-interfaces.md
git commit -m "adapter: Adapter type, YAML loading, embedded adapters/python.yaml

COVERAGE_CORE=ctrace and bare --cov are asserted by test, not left to review.
Adds LoadFS/Builtin to the interface contract."
```

---

## Task 2 — `internal/adapter`: `**`-aware glob matcher and file classification

**Files:** `internal/adapter/glob.go`, `internal/adapter/glob_test.go`, `internal/adapter/classify.go`, `internal/adapter/classify_test.go`

**Interfaces:**

*Consumes:* `adapter.Adapter` (Task 1).

*Produces:*
```go
func (a *Adapter) IsTestFile(rel string) bool
func (a *Adapter) IsOpaque(rel string) bool
func (a *Adapter) IsFullEscalate(rel string) bool
// IsInstrumentable: matches SourceGlobs AND is not a test file AND is not Opaque.
func (a *Adapter) IsInstrumentable(rel string) bool
```
plus the unexported `matchGlob(pattern, name string) bool`.

- [ ] **Step 1: Write the failing test**

`internal/adapter/glob_test.go`:

```go
package adapter

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"**/test_*.py", "tests/test_a.py", true},
		{"**/test_*.py", "test_a.py", true},
		{"**/test_*.py", "a/b/c/test_a.py", true},
		{"**/test_*.py", "tests/helpers.py", false},
		{"tests/**/*.py", "tests/test_a.py", true},
		{"tests/**/*.py", "tests/unit/deep/test_a.py", true},
		{"tests/**/*.py", "src/test_a.py", false},
		{"**/fixtures/**", "tests/fixtures/data.json", true},
		{"**/fixtures/**", "fixtures/a/b/c.sql", true},
		{"**/fixtures/**", "tests/fixture/data.json", false},
		{"**/conftest.py", "conftest.py", true},
		{"**/conftest.py", "tests/unit/conftest.py", true},
		{"requirements.txt", "requirements.txt", true},
		{"requirements.txt", "sub/requirements.txt", false},
		{"**/*.py", "src/logic.py", true},
		{"**/*.py", "src/data.yaml", false},
		{"**/*.yaml", "config/app.yaml", true},
		{"**", "anything/at/all.txt", true},
	}
	for _, tc := range cases {
		if got := matchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}
```

`internal/adapter/classify_test.go`:

```go
package adapter

import "testing"

func pyAdapter() *Adapter {
	return &Adapter{
		Name:         "python",
		TestGlobs:    []string{"tests/**/*.py", "**/test_*.py", "**/*_test.py"},
		SourceGlobs:  []string{"**/*.py"},
		Opaque:       []string{"**/*.yaml", "**/*.sql", "**/fixtures/**"},
		FullEscalate: []string{"requirements.txt", "pyproject.toml", "**/conftest.py"},
	}
}

func TestClassify(t *testing.T) {
	a := pyAdapter()
	cases := []struct {
		rel                                          string
		test, opaque, escalate, instrumentable       bool
	}{
		{"src/logic.py", false, false, false, true},
		{"src/__init__.py", false, false, false, true},
		{"tests/test_a.py", true, false, false, false},
		{"tests/helpers.py", true, false, false, false},
		{"pkg/test_thing.py", true, false, false, false},
		{"pkg/thing_test.py", true, false, false, false},
		{"config/app.yaml", false, true, false, false},
		{"db/schema.sql", false, true, false, false},
		{"tests/fixtures/blob.json", false, true, false, false},
		{"requirements.txt", false, false, true, false},
		{"pyproject.toml", false, false, true, false},
		{"tests/unit/conftest.py", true, false, true, false},
		{"README.md", false, false, false, false},
	}
	for _, tc := range cases {
		if got := a.IsTestFile(tc.rel); got != tc.test {
			t.Errorf("IsTestFile(%q) = %v, want %v", tc.rel, got, tc.test)
		}
		if got := a.IsOpaque(tc.rel); got != tc.opaque {
			t.Errorf("IsOpaque(%q) = %v, want %v", tc.rel, got, tc.opaque)
		}
		if got := a.IsFullEscalate(tc.rel); got != tc.escalate {
			t.Errorf("IsFullEscalate(%q) = %v, want %v", tc.rel, got, tc.escalate)
		}
		if got := a.IsInstrumentable(tc.rel); got != tc.instrumentable {
			t.Errorf("IsInstrumentable(%q) = %v, want %v", tc.rel, got, tc.instrumentable)
		}
	}
}

func TestClassifyNormalisesSeparators(t *testing.T) {
	a := pyAdapter()
	// Everything inside the engine is slash-separated and cleaned, but be tolerant
	// of a caller that has not been through internal/paths yet.
	if !a.IsTestFile("./tests/test_a.py") {
		t.Errorf(`IsTestFile("./tests/test_a.py") = false, want true`)
	}
	if !a.IsInstrumentable("src//logic.py") {
		t.Errorf(`IsInstrumentable("src//logic.py") = false, want true`)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/
```

Expected: `internal/adapter/glob_test.go:25:13: undefined: matchGlob` and
`internal/adapter/classify_test.go:31:17: a.IsTestFile undefined (type *Adapter has no field or method IsTestFile)` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/glob.go`:

```go
package adapter

import (
	"path"
	"strings"
)

// matchGlob reports whether the slash-separated path name matches pattern.
// It understands `**` as "zero or more path segments"; the stdlib's path.Match
// does not, and every adapter glob in the wild uses it.
func matchGlob(pattern, name string) bool {
	name = path.Clean(name)
	name = strings.TrimPrefix(name, "./")
	if pattern == "" || name == "" {
		return false
	}
	return matchSegments(strings.Split(path.Clean(pattern), "/"), strings.Split(name, "/"))
}

func matchSegments(pat, seg []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(seg); i++ {
				if matchSegments(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		}
		if len(seg) == 0 {
			return false
		}
		ok, err := path.Match(pat[0], seg[0])
		if err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}

func matchAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}
```

`internal/adapter/classify.go`:

```go
package adapter

// IsTestFile reports whether rel is a test file the runner can execute directly.
func (a *Adapter) IsTestFile(rel string) bool { return matchAny(a.TestGlobs, rel) }

// IsOpaque reports whether rel is a file coverage cannot see into (templates,
// fixtures, SQL, config data). Changing one escalates to T1.
func (a *Adapter) IsOpaque(rel string) bool { return matchAny(a.Opaque, rel) }

// IsFullEscalate reports whether changing rel forces a full-suite run (T2).
func (a *Adapter) IsFullEscalate(rel string) bool { return matchAny(a.FullEscalate, rel) }

// IsInstrumentable reports whether rel is source code coverage can attribute:
// it matches SourceGlobs, is not a test file, and is not opaque.
func (a *Adapter) IsInstrumentable(rel string) bool {
	return matchAny(a.SourceGlobs, rel) && !a.IsTestFile(rel) && !a.IsOpaque(rel)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run 'TestMatchGlob|TestClassify' -v
```

Expected: `--- PASS: TestMatchGlob`, `--- PASS: TestClassify`,
`--- PASS: TestClassifyNormalisesSeparators`, `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: ** glob matcher and file classification predicates"
```

---

## Task 3 — `internal/adapter`: `Detect`

**Files:** `internal/adapter/detect.go`, `internal/adapter/detect_test.go`

**Interfaces:**

*Consumes:* `paths.Normalize(repoRoot, p string) (rel string, ok bool)` from M1a; `adapter.Adapter`, `matchGlob` (Tasks 1–2).

*Produces:*
```go
// Detect returns the adapter whose Detect globs match a file in repoRoot.
// Exactly one match required; zero or multiple is an error.
func Detect(repoRoot string, adapters []*Adapter) (*Adapter, error)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/detect_test.go`:

```go
package adapter

import (
	"testing"
)

func mkAdapter(name string, detect ...string) *Adapter {
	return &Adapter{
		Name: name, Detect: detect,
		Seed: "x", Subset: "{tests}", Coverage: "sqlite", Report: "pytest-reportlog",
	}
}

func TestDetectExactlyOne(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname='x'\n")
	writeFile(t, dir, "src/logic.py", "x = 1\n")

	py := mkAdapter("python", "pytest.ini", "pyproject.toml", "setup.cfg")
	got, err := Detect(dir, []*Adapter{py})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Name != "python" {
		t.Fatalf("Detect = %q, want python", got.Name)
	}
}

func TestDetectGlobPattern(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/deep/go.mod", "module x\n")
	a := mkAdapter("golang", "**/go.mod")
	got, err := Detect(dir, []*Adapter{a})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Name != "golang" {
		t.Fatalf("Detect = %q, want golang", got.Name)
	}
}

func TestDetectZeroMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "hi\n")
	_, err := Detect(dir, []*Adapter{mkAdapter("python", "pyproject.toml")})
	if err == nil {
		t.Fatal("Detect with no matching marker = nil error, want error")
	}
	if !contains(err.Error(), "no adapter detected") {
		t.Fatalf("Detect error = %q, want it to contain %q", err.Error(), "no adapter detected")
	}
}

func TestDetectMultipleMatchesIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\n")
	writeFile(t, dir, "package.json", "{}\n")
	_, err := Detect(dir, []*Adapter{
		mkAdapter("python", "pyproject.toml"),
		mkAdapter("js", "package.json"),
	})
	if err == nil {
		t.Fatal("Detect with two matching adapters = nil error, want error (polyglot is out of scope in v1)")
	}
	if !contains(err.Error(), "python") || !contains(err.Error(), "js") {
		t.Fatalf("Detect error = %q, want it to name both adapters", err.Error())
	}
}

func TestDetectIgnoresDirectoryNamedLikeAMarker(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml/keep.txt", "a directory, not a marker file\n")
	_, err := Detect(dir, []*Adapter{mkAdapter("python", "pyproject.toml")})
	if err == nil {
		t.Fatal("Detect matched a directory named pyproject.toml, want no match")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run TestDetect
```

Expected: `internal/adapter/detect_test.go:22:14: undefined: Detect` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/detect.go`:

```go
package adapter

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// skipDirs are never walked when looking for detection markers.
var skipDirs = map[string]bool{
	".git": true, ".venv": true, "venv": true, "node_modules": true,
	"__pycache__": true, ".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	".rtdd": true,
}

// Detect returns the adapter whose Detect globs match a file in repoRoot.
// Exactly one match is required; zero or multiple is an error (polyglot repos are
// out of scope in v1 — spec §11).
func Detect(repoRoot string, adapters []*Adapter) (*Adapter, error) {
	var matched []*Adapter
	for _, a := range adapters {
		hit, err := detects(repoRoot, a)
		if err != nil {
			return nil, err
		}
		if hit {
			matched = append(matched, a)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return nil, fmt.Errorf("adapter: no adapter detected in %s", repoRoot)
	default:
		names := make([]string, 0, len(matched))
		for _, a := range matched {
			names = append(names, a.Name)
		}
		return nil, fmt.Errorf("adapter: %d adapters detected in %s (%s); polyglot repos are out of scope in v1",
			len(matched), repoRoot, strings.Join(names, ", "))
	}
}

func detects(repoRoot string, a *Adapter) (bool, error) {
	for _, pattern := range a.Detect {
		hit, err := anyFileMatches(repoRoot, pattern)
		if err != nil {
			return false, err
		}
		if hit {
			return true, nil
		}
	}
	return false, nil
}

func anyFileMatches(repoRoot, pattern string) (bool, error) {
	if !strings.ContainsAny(pattern, "*?[") {
		info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(pattern)))
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("adapter: stat %s: %w", pattern, err)
		}
		return !info.IsDir(), nil
	}
	found := false
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != repoRoot && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, ok := paths.Normalize(repoRoot, p)
		if !ok {
			return nil
		}
		if matchGlob(pattern, rel) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("adapter: walking %s: %w", repoRoot, err)
	}
	return found, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run TestDetect -v
```

Expected: five `--- PASS` lines, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/
git commit -m "adapter: Detect — exactly one adapter must match the host repo"
```

---

## Task 4 — `internal/adapter`: `Expand` and `ExpandTests`

**Files:** `internal/adapter/expand.go`, `internal/adapter/expand_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* `adapter.Adapter` (Task 1).

*Produces:*
```go
// Expand substitutes {src} {out} {log} into a command template and returns argv.
// It is an error for the template to contain {tests}; use ExpandTests.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error)

// ExpandTests is Expand for a template containing {tests}. Each test id becomes
// its own argv element, so ids containing spaces, '[', ']', '-' and '|' survive
// intact. CONTRACT ADDITION.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/expand_test.go`:

```go
package adapter

import (
	"reflect"
	"testing"
)

func TestExpand(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.Expand("pytest --cov --cov-context=test --cov-report= --report-log={log}",
		map[string]string{"log": "/tmp/x/report-0.jsonl"})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := []string{"pytest", "--cov", "--cov-context=test", "--cov-report=", "--report-log=/tmp/x/report-0.jsonl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand =\n  %q\nwant\n  %q", got, want)
	}
}

func TestExpandRejectsTestsPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.Expand("pytest {tests}", map[string]string{})
	if err == nil {
		t.Fatal("Expand on a {tests} template = nil error, want error directing the caller to ExpandTests")
	}
	if !contains(err.Error(), "ExpandTests") {
		t.Fatalf("Expand error = %q, want it to mention ExpandTests", err.Error())
	}
}

func TestExpandUnknownPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.Expand("pytest --junit={junit}", map[string]string{"log": "l"})
	if err == nil {
		t.Fatal("Expand with an unknown placeholder = nil error, want error")
	}
	if !contains(err.Error(), "{junit}") {
		t.Fatalf("Expand error = %q, want it to name {junit}", err.Error())
	}
}

// The ids below are the real ones measured from coverage.py contexts on this
// machine. Each must survive as exactly ONE argv element — pytest accepts them
// back as selectors verbatim (verified, exit 0).
func TestExpandTestsRoundTripsHostileIDs(t *testing.T) {
	a := &Adapter{Name: "python"}
	tests := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
		"tests/test_pipe.py::test_pipe[c d]",
	}
	got, err := a.ExpandTests("pytest {tests} --cov --report-log={log}",
		map[string]string{"log": "/tmp/r.jsonl"}, tests)
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{
		"pytest",
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
		"tests/test_pipe.py::test_pipe[c d]",
		"--cov",
		"--report-log=/tmp/r.jsonl",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandTests =\n  %q\nwant\n  %q", got, want)
	}
}

func TestExpandTestsEmptyList(t *testing.T) {
	a := &Adapter{Name: "python"}
	got, err := a.ExpandTests("pytest {tests} --cov", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"pytest", "--cov"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandTests = %q, want %q", got, want)
	}
}

func TestExpandTestsRequiresPlaceholder(t *testing.T) {
	a := &Adapter{Name: "python"}
	_, err := a.ExpandTests("pytest --cov", map[string]string{}, []string{"t"})
	if err == nil {
		t.Fatal("ExpandTests on a template with no {tests} = nil error, want error; test ids would be silently dropped")
	}
}

func TestExpandEmptyTemplate(t *testing.T) {
	a := &Adapter{Name: "python"}
	if _, err := a.Expand("   ", map[string]string{}); err == nil {
		t.Fatal("Expand on an empty template = nil error, want error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -run TestExpand
```

Expected: `internal/adapter/expand_test.go:12:17: a.Expand undefined (type *Adapter has no field or method Expand)` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/adapter/expand.go`:

```go
package adapter

import (
	"fmt"
	"regexp"
	"strings"
)

var placeholderRe = regexp.MustCompile(`\{[a-z_]+\}`)

// Expand substitutes {src} {out} {log} into a command template and returns argv.
// The template is split on whitespace into argv elements before substitution, so
// a substituted value is never re-split.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error) {
	toks := strings.Fields(tmpl)
	out := make([]string, 0, len(toks))
	for _, tok := range toks {
		if tok == "{tests}" {
			return nil, fmt.Errorf("adapter %s: template %q contains {tests}; use ExpandTests", a.Name, tmpl)
		}
		s, err := a.substitute(tok, vars)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("adapter %s: empty command template", a.Name)
	}
	return out, nil
}

// ExpandTests is Expand for a template containing the {tests} placeholder. Each
// test id is spliced in as its own argv element at that position, so ids
// containing spaces, '[', ']', '-' or '|' round-trip to the runner intact.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error) {
	toks := strings.Fields(tmpl)
	out := make([]string, 0, len(toks)+len(tests))
	sawTests := false
	for _, tok := range toks {
		if tok == "{tests}" {
			sawTests = true
			out = append(out, tests...)
			continue
		}
		s, err := a.substitute(tok, vars)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if !sawTests {
		return nil, fmt.Errorf("adapter %s: template %q has no {tests} placeholder; test ids would be silently dropped", a.Name, tmpl)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("adapter %s: empty command template", a.Name)
	}
	return out, nil
}

func (a *Adapter) substitute(tok string, vars map[string]string) (string, error) {
	var bad string
	s := placeholderRe.ReplaceAllStringFunc(tok, func(m string) string {
		k := m[1 : len(m)-1]
		v, ok := vars[k]
		if !ok {
			if bad == "" {
				bad = m
			}
			return m
		}
		return v
	})
	if bad != "" {
		return "", fmt.Errorf("adapter %s: unknown placeholder %s in %q", a.Name, bad, tok)
	}
	return s, nil
}
```

Record the contract addition. In `docs/plans/00-interfaces.md`, under
`## internal/adapter`, replace the `Expand` line with:

```go
// Expand substitutes {src} {out} {log} into a command template and returns argv.
// The template is split on whitespace BEFORE substitution, so a substituted value
// is never re-split. A template containing {tests} is an error here.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error)

// ExpandTests is Expand for a template containing {tests}. Each test id becomes
// its own argv element, so ids containing spaces, '[', ']', '-' or '|' round-trip
// intact — measured real ids include `tests/test_a.py::test_param[1-one two]`.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/adapter/ -v
```

Expected: every `--- PASS`, then `ok  	github.com/VocanicZ/rtdd/internal/adapter`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter/ docs/plans/00-interfaces.md
git commit -m "adapter: Expand/ExpandTests with test ids as separate argv elements

Parametrised pytest ids contain spaces, brackets, '-' and '|'; splitting the
template before substitution is what keeps them round-trippable.
Adds ExpandTests to the interface contract."
```

---

## Task 5 — `internal/coverage`: `Numbits`

**Files:** `internal/coverage/numbits.go`, `internal/coverage/numbits_test.go`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
// Numbits decodes coverage.py's numbits blob into sorted line numbers.
// Byte i, bit j set => line i*8+j is covered.
func Numbits(b []byte) []int
```

- [ ] **Step 1: Write the failing test**

`internal/coverage/numbits_test.go`:

```go
package coverage

import (
	"reflect"
	"testing"
)

// Every case below was read out of a real .coverage produced on this machine by
// `COVERAGE_CORE=ctrace pytest --cov --cov-context=test` (coverage.py 7.15.4) and
// cross-checked against coverage.numbits.numbits_to_nums. numbits is a PACKED
// BITMAP, not a line list: byte i, bit j set => line i*8+j.
func TestNumbitsRealBlobs(t *testing.T) {
	cases := []struct {
		name string
		blob []byte
		want []int
	}{
		{
			// src/logic.py, empty context: the import line and the three `def` lines.
			// 0x12 = 0b00010010 -> bits 1,4 -> lines 1,4
			// 0x11 = 0b00010001 -> bits 0,4 -> lines 8,12
			name: "logic.py import-time",
			blob: []byte{0x12, 0x11},
			want: []int{1, 4, 8, 12},
		},
		{
			// src/constants.py, empty context.
			// 0xEA = 0b11101010 -> bits 1,3,5,6,7
			name: "constants.py import-time",
			blob: []byte{0xEA},
			want: []int{1, 3, 5, 6, 7},
		},
		{
			// src/logic.py, context "tests/test_a.py::test_add|run".
			// 0x20 = 0b00100000 -> bit 5
			name: "test_add body",
			blob: []byte{0x20},
			want: []int{5},
		},
		{
			// src/logic.py, context "tests/test_b.py::test_mul|run".
			// byte 1 = 0x02 = 0b00000010 -> bit 1 -> line 8+1 = 9
			name: "test_mul body",
			blob: []byte{0x00, 0x02},
			want: []int{9},
		},
		{
			// An empty src/__init__.py: coverage records "line 0", which is not a
			// real source line. Numbits reports it faithfully; callers drop <= 0.
			name: "empty __init__.py",
			blob: []byte{0x01},
			want: []int{0},
		},
		{name: "nil", blob: nil, want: []int{}},
		{name: "empty", blob: []byte{}, want: []int{}},
		{name: "all zero bytes", blob: []byte{0x00, 0x00, 0x00}, want: []int{}},
		{name: "every bit of byte 0", blob: []byte{0xFF}, want: []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{name: "high bit of byte 3", blob: []byte{0x00, 0x00, 0x00, 0x80}, want: []int{31}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Numbits(tc.blob)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Numbits(% x) = %v, want %v", tc.blob, got, tc.want)
			}
		})
	}
}

func TestNumbitsIsSortedAscending(t *testing.T) {
	got := Numbits([]byte{0xFF, 0xFF, 0xFF})
	if len(got) != 24 {
		t.Fatalf("len = %d, want 24", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("not ascending at %d: %v", i, got[:i+1])
		}
	}
}

func TestNumbitsNeverReturnsNil(t *testing.T) {
	if Numbits(nil) == nil {
		t.Fatal("Numbits(nil) returned a nil slice; callers range over it and append, want empty non-nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/coverage/
```

Expected: `internal/coverage/numbits_test.go:63:11: undefined: Numbits` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/coverage/numbits.go`:

```go
// Package coverage reads coverage.py's .coverage SQLite store directly — it *is*
// the bipartite test<->file relation, at a fraction of the size of any exported
// format (spec §4, audit A6).
package coverage

// Numbits decodes coverage.py's numbits blob into sorted line numbers.
//
// numbits is a packed bitmap, not a line list: byte i, bit j set means line
// i*8+j is covered. Verified against coverage.numbits.numbits_to_nums on real
// data — e.g. []byte{0x12, 0x11} decodes to [1 4 8 12].
//
// Line 0 can legitimately appear (an empty __init__.py records it); it is not a
// real source line and callers drop non-positive values.
func Numbits(b []byte) []int {
	out := make([]int, 0, len(b)*2)
	for i, by := range b {
		if by == 0 {
			continue
		}
		for j := 0; j < 8; j++ {
			if by&(1<<uint(j)) != 0 {
				out = append(out, i*8+j)
			}
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestNumbits -v
```

Expected: `--- PASS: TestNumbitsRealBlobs` with all ten sub-tests, plus the two
other `--- PASS` lines, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/coverage/
git commit -m "coverage: decode coverage.py's packed numbits line bitmap

Real blobs from a measured .coverage are the test vectors."
```

---

## Task 6 — `internal/coverage`: `NormalizeContext`

**Files:** `internal/coverage/context.go`, `internal/coverage/context_test.go`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
// NormalizeContext splits "tests/test_a.py::test_x|run" into ("tests/test_a.py::test_x", "run", true).
// An empty context returns ok=false — that is import-time coverage, not a test.
func NormalizeContext(ctx string) (testID, phase string, ok bool)
```

- [ ] **Step 1: Write the failing test**

`internal/coverage/context_test.go`:

```go
package coverage

import "testing"

// Every input below is a verbatim `context.context` value read out of a real
// .coverage on this machine.
func TestNormalizeContext(t *testing.T) {
	cases := []struct {
		name     string
		ctx      string
		wantID   string
		wantPh   string
		wantOK   bool
	}{
		{
			name:   "empty context is import-time, not a test",
			ctx:    "",
			wantID: "", wantPh: "", wantOK: false,
		},
		{
			name:   "plain run phase",
			ctx:    "tests/test_a.py::test_add|run",
			wantID: "tests/test_a.py::test_add", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a space",
			ctx:    "tests/test_a.py::test_param[1-one two]|run",
			wantID: "tests/test_a.py::test_param[1-one two]", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a hyphen",
			ctx:    "tests/test_a.py::test_param[2-a-b]|run",
			wantID: "tests/test_a.py::test_param[2-a-b]", wantPh: "run", wantOK: true,
		},
		{
			name:   "parametrised id containing a pipe: split on the LAST pipe",
			ctx:    "tests/test_pipe.py::test_pipe[a|b]|run",
			wantID: "tests/test_pipe.py::test_pipe[a|b]", wantPh: "run", wantOK: true,
		},
		{
			name:   "setup phase",
			ctx:    "tests/test_c.py::test_with_fixture|setup",
			wantID: "tests/test_c.py::test_with_fixture", wantPh: "setup", wantOK: true,
		},
		{
			name:   "teardown phase",
			ctx:    "tests/test_c.py::test_with_fixture|teardown",
			wantID: "tests/test_c.py::test_with_fixture", wantPh: "teardown", wantOK: true,
		},
		{
			name:   "a static context with no phase suffix is kept whole",
			ctx:    "mystaticcontext",
			wantID: "mystaticcontext", wantPh: "", wantOK: true,
		},
		{
			name:   "a pipe that is not a known phase is part of the id",
			ctx:    "tests/test_x.py::test_y[a|b]",
			wantID: "tests/test_x.py::test_y[a|b]", wantPh: "", wantOK: true,
		},
		{
			name:   "a phase suffix with no id is not a test",
			ctx:    "|teardown",
			wantID: "", wantPh: "", wantOK: false,
		},
		{
			name:   "class-based id",
			ctx:    "tests/test_k.py::TestThing::test_method|run",
			wantID: "tests/test_k.py::TestThing::test_method", wantPh: "run", wantOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ph, ok := NormalizeContext(tc.ctx)
			if id != tc.wantID || ph != tc.wantPh || ok != tc.wantOK {
				t.Fatalf("NormalizeContext(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.ctx, id, ph, ok, tc.wantID, tc.wantPh, tc.wantOK)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestNormalizeContext
```

Expected: `internal/coverage/context_test.go:76:16: undefined: NormalizeContext` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/coverage/context.go`:

```go
package coverage

import "strings"

// phases are the dynamic-context suffixes pytest-cov appends to a nodeid.
// Measured on real data: `|run`, `|setup`, `|teardown`.
var phases = map[string]bool{"run": true, "setup": true, "teardown": true}

// NormalizeContext splits "tests/test_a.py::test_x|run" into
// ("tests/test_a.py::test_x", "run", true).
//
// An empty context returns ok=false — that is import-time coverage, executed
// during collection before any dynamic context is set, and attributed to no test
// (spec §6, audit A1).
//
// The split is on the LAST '|', and only when the suffix is a known phase: a
// parametrised id can itself contain '|' (measured:
// "tests/test_pipe.py::test_pipe[a|b]|run"), and a static context set by the host
// repo has no phase suffix at all.
func NormalizeContext(ctx string) (testID, phase string, ok bool) {
	if ctx == "" {
		return "", "", false
	}
	i := strings.LastIndex(ctx, "|")
	if i < 0 {
		return ctx, "", true
	}
	if !phases[ctx[i+1:]] {
		return ctx, "", true
	}
	id := ctx[:i]
	if id == "" {
		return "", "", false
	}
	return id, ctx[i+1:], true
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestNormalizeContext -v
```

Expected: eleven `--- PASS` sub-tests, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/coverage/
git commit -m "coverage: NormalizeContext strips the |run/|setup/|teardown phase suffix

Splits on the last pipe and only for a known phase, because a parametrised id
can contain a pipe itself."
```

---

## Task 7 — `internal/coverage`: `Result`, `TestCoverage`, `Result.Merge`

**Files:** `internal/coverage/result.go`, `internal/coverage/result_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
type TestCoverage struct {
    Test  string           // normalised id, phase suffix stripped
    Files map[string][]int // repo-relative path -> sorted covered line numbers
}

type Result struct {
    PerTest    []TestCoverage
    ImportTime map[string][]int // empty-context lines: executed, attributed to no test
}

// Merge unions other into r. Used to combine the .coverage read after each argv
// chunk, because pytest ERASES .coverage at the start of every run unless
// --cov-append is passed. CONTRACT ADDITION.
func (r *Result) Merge(other *Result)
```

- [ ] **Step 1: Write the failing test**

`internal/coverage/result_test.go`:

```go
package coverage

import (
	"reflect"
	"testing"
)

func TestResultMergeUnionsChunks(t *testing.T) {
	// Chunk 1 ran tests/test_a.py; chunk 2 ran tests/test_b.py. pytest erased
	// .coverage between them, so each Result holds only its own chunk.
	c1 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		},
		ImportTime: map[string][]int{
			"src/logic.py":     {1, 4, 8, 12},
			"src/constants.py": {1, 3, 5, 6, 7},
		},
	}
	c2 := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
		},
		ImportTime: map[string][]int{
			"src/logic.py": {1, 8},
			"src/other.py": {2},
		},
	}

	c1.Merge(c2)

	wantPerTest := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	if !reflect.DeepEqual(c1.PerTest, wantPerTest) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", c1.PerTest, wantPerTest)
	}
	wantImport := map[string][]int{
		"src/logic.py":     {1, 4, 8, 12},
		"src/constants.py": {1, 3, 5, 6, 7},
		"src/other.py":     {2},
	}
	if !reflect.DeepEqual(c1.ImportTime, wantImport) {
		t.Fatalf("ImportTime =\n  %+v\nwant\n  %+v", c1.ImportTime, wantImport)
	}
}

func TestResultMergeSameTestAcrossChunks(t *testing.T) {
	// The same test id can appear in two Results when its |setup and |run phases
	// land in different reads, or when a repeated id shows up twice.
	a := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_c.py::test_fx", Files: map[string][]int{"src/fixt.py": {2}}},
		},
		ImportTime: map[string][]int{},
	}
	b := &Result{
		PerTest: []TestCoverage{
			{Test: "tests/test_c.py::test_fx", Files: map[string][]int{
				"src/fixt.py":  {6, 10, 2},
				"src/other.py": {3},
			}},
		},
		ImportTime: map[string][]int{},
	}
	a.Merge(b)
	if len(a.PerTest) != 1 {
		t.Fatalf("len(PerTest) = %d, want 1", len(a.PerTest))
	}
	want := map[string][]int{"src/fixt.py": {2, 6, 10}, "src/other.py": {3}}
	if !reflect.DeepEqual(a.PerTest[0].Files, want) {
		t.Fatalf("Files = %+v, want %+v (sorted and deduped)", a.PerTest[0].Files, want)
	}
}

func TestResultMergeDoesNotAliasOther(t *testing.T) {
	a := &Result{ImportTime: map[string][]int{}}
	b := &Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{"g.py": {2}},
	}
	a.Merge(b)
	b.PerTest[0].Files["f.py"] = append(b.PerTest[0].Files["f.py"], 99)
	b.ImportTime["g.py"] = append(b.ImportTime["g.py"], 99)
	if got := a.PerTest[0].Files["f.py"]; !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("Merge aliased other's Files: got %v, want [1]", got)
	}
	if got := a.ImportTime["g.py"]; !reflect.DeepEqual(got, []int{2}) {
		t.Fatalf("Merge aliased other's ImportTime: got %v, want [2]", got)
	}
}

func TestResultMergeNilIsANoop(t *testing.T) {
	a := &Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{},
	}
	a.Merge(nil)
	if len(a.PerTest) != 1 {
		t.Fatalf("len(PerTest) after Merge(nil) = %d, want 1", len(a.PerTest))
	}
}

func TestResultMergeIntoZeroValue(t *testing.T) {
	var a Result
	a.Merge(&Result{
		PerTest:    []TestCoverage{{Test: "t", Files: map[string][]int{"f.py": {1}}}},
		ImportTime: map[string][]int{"g.py": {2}},
	})
	if a.ImportTime == nil {
		t.Fatal("Merge into a zero Result left ImportTime nil")
	}
	if len(a.PerTest) != 1 || a.PerTest[0].Test != "t" {
		t.Fatalf("PerTest = %+v, want one entry for t", a.PerTest)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestResult
```

Expected: `internal/coverage/result_test.go:11:8: undefined: Result` and
`undefined: TestCoverage` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/coverage/result.go`:

```go
package coverage

import "sort"

// TestCoverage is one test's file->lines attribution, with the phase suffix
// already stripped from the id.
type TestCoverage struct {
	Test  string           // normalised id, phase suffix stripped
	Files map[string][]int // repo-relative path -> sorted covered line numbers
}

// Result is everything one .coverage store says.
//
// ImportTime holds the empty-context lines: executed during collection, before
// any dynamic context was set, and therefore attributed to no test. It is a
// distinct class and MUST NOT be reported as uncovered (spec §6, audit A1).
type Result struct {
	PerTest    []TestCoverage
	ImportTime map[string][]int
}

// Merge unions other into r.
//
// This exists because pytest ERASES .coverage at the start of every run unless
// --cov-append is passed (measured: chunk 2 left only chunk 2's contexts). RTDD
// reads .coverage after each argv chunk and merges here, rather than relying on
// --cov-append, which would also silently absorb a stale .coverage from an
// unrelated earlier run.
func (r *Result) Merge(other *Result) {
	if other == nil {
		return
	}
	if r.ImportTime == nil {
		r.ImportTime = make(map[string][]int, len(other.ImportTime))
	}
	for f, lines := range other.ImportTime {
		r.ImportTime[f] = mergeLines(r.ImportTime[f], lines)
	}
	idx := make(map[string]int, len(r.PerTest))
	for i, tc := range r.PerTest {
		idx[tc.Test] = i
	}
	for _, tc := range other.PerTest {
		i, ok := idx[tc.Test]
		if !ok {
			cp := TestCoverage{Test: tc.Test, Files: make(map[string][]int, len(tc.Files))}
			for f, lines := range tc.Files {
				cp.Files[f] = mergeLines(nil, lines)
			}
			r.PerTest = append(r.PerTest, cp)
			idx[tc.Test] = len(r.PerTest) - 1
			continue
		}
		if r.PerTest[i].Files == nil {
			r.PerTest[i].Files = map[string][]int{}
		}
		for f, lines := range tc.Files {
			r.PerTest[i].Files[f] = mergeLines(r.PerTest[i].Files[f], lines)
		}
	}
	sort.Slice(r.PerTest, func(i, j int) bool { return r.PerTest[i].Test < r.PerTest[j].Test })
}

// mergeLines returns the sorted, deduped union of a and b. It never aliases b.
func mergeLines(a, b []int) []int {
	seen := make(map[int]bool, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, l := range a {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	for _, l := range b {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Ints(out)
	return out
}
```

Record the contract addition. In `docs/plans/00-interfaces.md`, under
`## internal/coverage`, after the `Result` struct definition, add:

```go
// Merge unions other into r, sorting PerTest by Test. Used to combine the
// .coverage read after each argv chunk: pytest erases .coverage at the start of
// every run unless --cov-append is passed, so RTDD reads and merges per chunk.
func (r *Result) Merge(other *Result)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestResult -v
```

Expected: five `--- PASS` lines, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/coverage/ docs/plans/00-interfaces.md
git commit -m "coverage: Result/TestCoverage plus Merge for per-chunk union

pytest erases .coverage each run without --cov-append, so chunked runs must be
merged in Go. Adds Result.Merge to the interface contract."
```

---

## Task 8 — `internal/coverage`: `ReadSQLite`, the `line_bits` path

**Files:** `go.mod`, `internal/coverage/sqlite.go`, `internal/coverage/sqlite_test.go`

**Interfaces:**

*Consumes:* `paths.Normalize(repoRoot, p string) (rel string, ok bool)` from M1a;
`Numbits`, `NormalizeContext`, `Result`, `TestCoverage`, `mergeLines` (Tasks 5–7).

*Produces:*
```go
// ReadSQLite reads .coverage directly:
//   SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb
//     JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id
// numbits is coverage.py's packed line bitmap; decode with Numbits.
func ReadSQLite(dbPath, repoRoot string) (*Result, error)
```

- [ ] **Step 1: Write the failing test**

`internal/coverage/sqlite_test.go`:

```go
package coverage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// coverageDDL is the schema of a real .coverage file, copied verbatim out of
// sqlite_master on coverage.py 7.15.4.
const coverageDDL = `
CREATE TABLE coverage_schema (version integer);
CREATE TABLE meta (key text, value text, unique (key));
CREATE TABLE file (id integer primary key, path text, unique (path));
CREATE TABLE context (id integer primary key, context text, unique (context));
CREATE TABLE line_bits (
    file_id integer, context_id integer, numbits blob,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id));
CREATE TABLE arc (
    file_id integer, context_id integer, fromno integer, tono integer,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id, fromno, tono));
CREATE TABLE tracer (file_id integer primary key, tracer text,
    foreign key (file_id) references file (id));
`

func newCoverageDB(t *testing.T, dir string, hasArcs string) (string, *sql.DB) {
	t.Helper()
	p := filepath.Join(dir, ".coverage")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(coverageDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO coverage_schema (version) VALUES (7)`); err != nil {
		t.Fatalf("schema row: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('version','7.15.4'), ('has_arcs', ?)`, hasArcs); err != nil {
		t.Fatalf("meta: %v", err)
	}
	return p, db
}

func insFile(t *testing.T, db *sql.DB, id int, path string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO file (id, path) VALUES (?, ?)`, id, path); err != nil {
		t.Fatalf("insert file: %v", err)
	}
}

func insContext(t *testing.T, db *sql.DB, id int, ctx string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO context (id, context) VALUES (?, ?)`, id, ctx); err != nil {
		t.Fatalf("insert context: %v", err)
	}
}

func insLineBits(t *testing.T, db *sql.DB, fileID, ctxID int, numbits []byte) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO line_bits (file_id, context_id, numbits) VALUES (?, ?, ?)`,
		fileID, ctxID, numbits); err != nil {
		t.Fatalf("insert line_bits: %v", err)
	}
}

// The fixture below reproduces, row for row, a .coverage measured on this machine
// from `COVERAGE_CORE=ctrace pytest --cov --cov-context=test`.
func TestReadSQLiteLineBits(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "0")

	abs := func(rel string) string { return filepath.Join(repo, filepath.FromSlash(rel)) }

	// file.path is ABSOLUTE unless the host sets [run] relative_files = True.
	insFile(t, db, 1, abs("src/__init__.py"))
	insFile(t, db, 2, abs("src/logic.py"))
	insFile(t, db, 3, abs("src/constants.py"))
	// A path outside the repo (site-packages) must be dropped, not crash.
	insFile(t, db, 4, "/usr/lib/python3/site-packages/attrs/__init__.py")

	insContext(t, db, 1, "")
	insContext(t, db, 2, "tests/test_a.py::test_add|run")
	insContext(t, db, 3, "tests/test_a.py::test_param[1-one two]|run")
	insContext(t, db, 4, "tests/test_a.py::test_param[2-a-b]|run")
	insContext(t, db, 5, "tests/test_b.py::test_mul|run")
	insContext(t, db, 6, "tests/test_b.py::test_fail|run")
	insContext(t, db, 7, "tests/test_c.py::test_with_fixture|setup")
	insContext(t, db, 8, "tests/test_c.py::test_with_fixture|teardown")

	insLineBits(t, db, 1, 1, []byte{0x01})       // empty __init__.py -> line 0, dropped
	insLineBits(t, db, 3, 1, []byte{0xEA})       // constants.py import-time -> 1,3,5,6,7
	insLineBits(t, db, 2, 1, []byte{0x12, 0x11}) // logic.py import-time -> 1,4,8,12
	insLineBits(t, db, 2, 2, []byte{0x20})       // test_add -> 5
	insLineBits(t, db, 2, 3, []byte{0x20})
	insLineBits(t, db, 2, 4, []byte{0x20})
	insLineBits(t, db, 2, 5, []byte{0x00, 0x02}) // test_mul -> 9
	insLineBits(t, db, 2, 6, []byte{0x00, 0x02}) // test_fail -> 9
	insLineBits(t, db, 2, 7, []byte{0x04})       // test_with_fixture setup -> 2
	insLineBits(t, db, 2, 8, []byte{0x40})       // test_with_fixture teardown -> 6
	insLineBits(t, db, 4, 2, []byte{0xFF})       // site-packages, dropped

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}

	// Import-time: attributed to NO test. src/constants.py is the audit A1 case —
	// a dataclass plus a module constant, imported and asserted by passing tests,
	// with zero test attribution.
	wantImport := map[string][]int{
		"src/__init__.py":  {},
		"src/logic.py":     {1, 4, 8, 12},
		"src/constants.py": {1, 3, 5, 6, 7},
	}
	if !reflect.DeepEqual(res.ImportTime, wantImport) {
		t.Errorf("ImportTime =\n  %+v\nwant\n  %+v", res.ImportTime, wantImport)
	}

	wantPerTest := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_a.py::test_param[1-one two]", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_a.py::test_param[2-a-b]", Files: map[string][]int{"src/logic.py": {5}}},
		// setup and teardown phases collapse into ONE entry for the test.
		{Test: "tests/test_c.py::test_with_fixture", Files: map[string][]int{"src/logic.py": {2, 6}}},
		{Test: "tests/test_b.py::test_fail", Files: map[string][]int{"src/logic.py": {9}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	// PerTest is sorted by Test.
	byID := map[string]map[string][]int{}
	for _, tc := range res.PerTest {
		byID[tc.Test] = tc.Files
	}
	if len(res.PerTest) != len(wantPerTest) {
		t.Fatalf("len(PerTest) = %d, want %d; got %+v", len(res.PerTest), len(wantPerTest), res.PerTest)
	}
	for _, w := range wantPerTest {
		got, ok := byID[w.Test]
		if !ok {
			t.Errorf("PerTest missing %q", w.Test)
			continue
		}
		if !reflect.DeepEqual(got, w.Files) {
			t.Errorf("PerTest[%q].Files = %+v, want %+v", w.Test, got, w.Files)
		}
	}
	for i := 1; i < len(res.PerTest); i++ {
		if res.PerTest[i-1].Test >= res.PerTest[i].Test {
			t.Fatalf("PerTest not sorted by Test at index %d: %q then %q",
				i, res.PerTest[i-1].Test, res.PerTest[i].Test)
		}
	}
}

func TestReadSQLiteRelativeFiles(t *testing.T) {
	// A host with [run] relative_files = True writes repo-relative paths.
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "0")
	insFile(t, db, 1, "src/logic.py")
	insContext(t, db, 1, "tests/test_a.py::test_add|run")
	insLineBits(t, db, 1, 1, []byte{0x20})

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	if len(res.PerTest) != 1 {
		t.Fatalf("len(PerTest) = %d, want 1", len(res.PerTest))
	}
	want := map[string][]int{"src/logic.py": {5}}
	if !reflect.DeepEqual(res.PerTest[0].Files, want) {
		t.Fatalf("Files = %+v, want %+v", res.PerTest[0].Files, want)
	}
}

func TestReadSQLiteMissingFileIsAnError(t *testing.T) {
	repo := t.TempDir()
	_, err := ReadSQLite(filepath.Join(repo, ".coverage"), repo)
	if err == nil {
		t.Fatal("ReadSQLite on a missing .coverage = nil error; a missing store must be fatal (exit 3), never an empty map")
	}
}

func TestReadSQLiteEmptyStore(t *testing.T) {
	repo := t.TempDir()
	dbPath, _ := newCoverageDB(t, repo, "0")
	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite on an empty store: %v", err)
	}
	if len(res.PerTest) != 0 {
		t.Errorf("PerTest = %+v, want empty", res.PerTest)
	}
	if res.ImportTime == nil {
		t.Error("ImportTime is nil, want an empty non-nil map")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go get modernc.org/sqlite@latest && go test ./internal/coverage/ -run TestReadSQLite
```

Expected: `internal/coverage/sqlite_test.go:130:14: undefined: ReadSQLite` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/coverage/sqlite.go`:

```go
package coverage

import (
	"database/sql"
	"fmt"
	"os"
	"sort"

	_ "modernc.org/sqlite" // pure-Go driver, no cgo — the binary must stay static

	"github.com/VocanicZ/rtdd/internal/paths"
)

const lineBitsQuery = `SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb
  JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id`

// ReadSQLite reads coverage.py's .coverage store directly. It *is* the bipartite
// test<->file relation, and is the only export format that carries a test
// identifier at a size that scales (spec §4, audit A6).
//
// Paths in `file` are absolute unless the host set [run] relative_files = True;
// both are normalised through internal/paths against repoRoot, and anything
// outside the repo (site-packages, the stdlib) is dropped.
func ReadSQLite(dbPath, repoRoot string) (*Result, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("coverage: %s is unreadable: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("coverage: opening %s: %w", dbPath, err)
	}
	defer db.Close()

	acc := newAccumulator(repoRoot)

	rows, err := db.Query(lineBitsQuery)
	if err != nil {
		return nil, fmt.Errorf("coverage: querying line_bits in %s: %w", dbPath, err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, ctx string
		var numbits []byte
		if err := rows.Scan(&path, &ctx, &numbits); err != nil {
			return nil, fmt.Errorf("coverage: scanning line_bits in %s: %w", dbPath, err)
		}
		acc.add(path, ctx, Numbits(numbits))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coverage: reading line_bits in %s: %w", dbPath, err)
	}

	return acc.result(), nil
}

// accumulator collects (file, context, lines) triples into a Result, unioning
// the |setup, |run and |teardown phases of one test into a single entry.
type accumulator struct {
	repoRoot   string
	perTest    map[string]map[string]map[int]bool
	importTime map[string]map[int]bool
}

func newAccumulator(repoRoot string) *accumulator {
	return &accumulator{
		repoRoot:   repoRoot,
		perTest:    map[string]map[string]map[int]bool{},
		importTime: map[string]map[int]bool{},
	}
}

func (a *accumulator) add(rawPath, rawCtx string, lines []int) {
	rel, ok := paths.Normalize(a.repoRoot, rawPath)
	if !ok {
		return // outside the repo: site-packages, the stdlib, a sibling checkout
	}
	testID, _, ok := NormalizeContext(rawCtx)
	if !ok {
		set := a.importTime[rel]
		if set == nil {
			set = map[int]bool{}
			a.importTime[rel] = set
		}
		addLines(set, lines)
		return
	}
	files := a.perTest[testID]
	if files == nil {
		files = map[string]map[int]bool{}
		a.perTest[testID] = files
	}
	set := files[rel]
	if set == nil {
		set = map[int]bool{}
		files[rel] = set
	}
	addLines(set, lines)
}

// addLines drops non-positive line numbers: coverage records "line 0" for an
// empty __init__.py, and the arc table uses negative numbers as scope
// entry/exit sentinels. Neither is a real source line.
func addLines(set map[int]bool, lines []int) {
	for _, l := range lines {
		if l > 0 {
			set[l] = true
		}
	}
}

func (a *accumulator) result() *Result {
	res := &Result{ImportTime: make(map[string][]int, len(a.importTime))}
	for rel, set := range a.importTime {
		res.ImportTime[rel] = sortedKeys(set)
	}
	ids := make([]string, 0, len(a.perTest))
	for id := range a.perTest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		files := make(map[string][]int, len(a.perTest[id]))
		for rel, set := range a.perTest[id] {
			files[rel] = sortedKeys(set)
		}
		res.PerTest = append(res.PerTest, TestCoverage{Test: id, Files: files})
	}
	return res
}

func sortedKeys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestReadSQLite -v
```

Expected: `--- PASS: TestReadSQLiteLineBits`, `--- PASS: TestReadSQLiteRelativeFiles`,
`--- PASS: TestReadSQLiteMissingFileIsAnError`, `--- PASS: TestReadSQLiteEmptyStore`, `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add go.mod go.sum internal/coverage/
git commit -m "coverage: ReadSQLite reads .coverage line_bits directly

Absolute and relative_files paths both normalise through internal/paths;
out-of-repo paths are dropped; |setup/|run/|teardown collapse into one entry."
```

---

## Task 9 — `internal/coverage`: `ReadSQLite`, the `arc` path for branch coverage

**Files:** `internal/coverage/sqlite.go`, `internal/coverage/sqlite_arc_test.go`

**Interfaces:**

*Consumes:* everything from Task 8.

*Produces:* no new exported names — `ReadSQLite` gains a second read path selected by
`meta.has_arcs`.

**Why this task exists.** Measured on this machine: with `[run] branch = True` — a very
common host setting — `line_bits` has **0 rows** and `arc` has 60. The Task 8 query returns
nothing, `seed` writes a map with empty `f` for every row, and the process exits 0. That is
the same silent-corruption shape as `COVERAGE_CORE=sysmon`, and it is not in the spec.

The recovery rule, verified to reproduce the `line_bits` answer exactly on every measured
file/context pair: **executed lines = `{fromno : fromno > 0} ∪ {tono : tono > 0}`**.

- [ ] **Step 1: Write the failing test**

`internal/coverage/sqlite_arc_test.go`:

```go
package coverage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func insArc(t *testing.T, db *sql.DB, fileID, ctxID, fromno, tono int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO arc (file_id, context_id, fromno, tono) VALUES (?, ?, ?, ?)`,
		fileID, ctxID, fromno, tono); err != nil {
		t.Fatalf("insert arc: %v", err)
	}
}

// Measured: with `[run] branch = True` coverage.py sets meta.has_arcs='1', writes
// ZERO line_bits rows, and puts everything in `arc`. The line_bits query returns
// nothing and the map comes out empty with exit 0 — a silent corruption path.
//
// The arcs below are verbatim from a real branch-mode .coverage, and the expected
// line sets are the ones the SAME code produced under line mode.
func TestReadSQLiteBranchModeUsesArcTable(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "1")
	abs := func(rel string) string { return filepath.Join(repo, filepath.FromSlash(rel)) }

	insFile(t, db, 1, abs("src/logic.py"))
	insFile(t, db, 2, abs("src/constants.py"))
	insContext(t, db, 1, "")
	insContext(t, db, 2, "tests/test_a.py::test_add|run")
	insContext(t, db, 3, "tests/test_b.py::test_mul|run")

	// src/logic.py, empty context. Negative numbers are scope entry/exit sentinels.
	for _, a := range [][2]int{{-1, 1}, {1, 4}, {4, 8}, {8, 12}, {12, -1}} {
		insArc(t, db, 1, 1, a[0], a[1])
	}
	// src/constants.py, empty context.
	for _, a := range [][2]int{{-5, 5}, {-1, 1}, {1, 3}, {3, 5}, {5, 6}, {5, 7}, {6, -1}, {6, 5}, {7, -5}} {
		insArc(t, db, 2, 1, a[0], a[1])
	}
	// src/logic.py, test_add: enter the function body, run line 5, return.
	insArc(t, db, 1, 2, -4, 5)
	insArc(t, db, 1, 2, 5, -4)
	// src/logic.py, test_mul.
	insArc(t, db, 1, 3, -8, 9)
	insArc(t, db, 1, 3, 9, -8)

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}

	wantImport := map[string][]int{
		"src/logic.py":     {1, 4, 8, 12},     // identical to the line_bits answer
		"src/constants.py": {1, 3, 5, 6, 7},   // identical to the line_bits answer
	}
	if !reflect.DeepEqual(res.ImportTime, wantImport) {
		t.Errorf("ImportTime =\n  %+v\nwant\n  %+v", res.ImportTime, wantImport)
	}

	if len(res.PerTest) != 2 {
		t.Fatalf("len(PerTest) = %d, want 2; got %+v", len(res.PerTest), res.PerTest)
	}
	want := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	if !reflect.DeepEqual(res.PerTest, want) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", res.PerTest, want)
	}
}

func TestReadSQLiteBranchModeWithNoArcsIsNotAnError(t *testing.T) {
	repo := t.TempDir()
	dbPath, _ := newCoverageDB(t, repo, "1")
	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	if len(res.PerTest) != 0 || len(res.ImportTime) != 0 {
		t.Fatalf("res = %+v, want empty", res)
	}
}

func TestReadSQLiteHasArcsAcceptsPythonicTruth(t *testing.T) {
	for _, v := range []string{"1", "True", "true"} {
		repo := t.TempDir()
		dbPath, db := newCoverageDB(t, repo, v)
		insFile(t, db, 1, filepath.Join(repo, "src", "logic.py"))
		insContext(t, db, 1, "tests/test_a.py::test_add|run")
		insArc(t, db, 1, 1, -4, 5)
		res, err := ReadSQLite(dbPath, repo)
		if err != nil {
			t.Fatalf("has_arcs=%q: ReadSQLite: %v", v, err)
		}
		if len(res.PerTest) != 1 {
			t.Fatalf("has_arcs=%q: len(PerTest) = %d, want 1 (the arc path was not taken)", v, len(res.PerTest))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -run TestReadSQLiteBranchMode -v
```

Expected:
```
--- FAIL: TestReadSQLiteBranchModeUsesArcTable
    sqlite_arc_test.go:63: ImportTime =
        map[]
      want
        map[src/constants.py:[1 3 5 6 7] src/logic.py:[1 4 8 12]]
    sqlite_arc_test.go:67: len(PerTest) = 0, want 2; got []
```

- [ ] **Step 3: Write minimal implementation**

In `internal/coverage/sqlite.go`, add the `arc` query constant next to `lineBitsQuery`:

```go
const arcQuery = `SELECT DISTINCT f.path, c.context, a.fromno, a.tono FROM arc a
  JOIN file f ON f.id = a.file_id JOIN context c ON c.id = a.context_id`
```

Replace the body of `ReadSQLite` between `acc := newAccumulator(repoRoot)` and
`return acc.result(), nil` with:

```go
	hasArcs, err := readHasArcs(db)
	if err != nil {
		return nil, fmt.Errorf("coverage: reading meta in %s: %w", dbPath, err)
	}

	if hasArcs {
		// Measured: with [run] branch = True, line_bits has ZERO rows and all data
		// lives in `arc`. Reading line_bits here would produce an empty map on a
		// run that exited 0 — the same silent corruption as COVERAGE_CORE=sysmon.
		//
		// Executed lines = {fromno > 0} union {tono > 0}. Negative values are scope
		// entry/exit sentinels. Verified to reproduce the line-mode answer exactly.
		rows, err := db.Query(arcQuery)
		if err != nil {
			return nil, fmt.Errorf("coverage: querying arc in %s: %w", dbPath, err)
		}
		defer rows.Close()
		for rows.Next() {
			var path, ctx string
			var fromno, tono int
			if err := rows.Scan(&path, &ctx, &fromno, &tono); err != nil {
				return nil, fmt.Errorf("coverage: scanning arc in %s: %w", dbPath, err)
			}
			acc.add(path, ctx, []int{fromno, tono}) // acc.add drops non-positive lines
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("coverage: reading arc in %s: %w", dbPath, err)
		}
		return acc.result(), nil
	}

	rows, err := db.Query(lineBitsQuery)
	if err != nil {
		return nil, fmt.Errorf("coverage: querying line_bits in %s: %w", dbPath, err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, ctx string
		var numbits []byte
		if err := rows.Scan(&path, &ctx, &numbits); err != nil {
			return nil, fmt.Errorf("coverage: scanning line_bits in %s: %w", dbPath, err)
		}
		acc.add(path, ctx, Numbits(numbits))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coverage: reading line_bits in %s: %w", dbPath, err)
	}
```

And add, at the bottom of `internal/coverage/sqlite.go`:

```go
// readHasArcs reports whether the store was recorded with branch coverage on, in
// which case `line_bits` is empty and `arc` holds everything.
func readHasArcs(db *sql.DB) (bool, error) {
	var v string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'has_arcs'`).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v == "1" || v == "True" || v == "true", nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/coverage/ -v
```

Expected: every `--- PASS` including the three branch-mode tests, then
`ok  	github.com/VocanicZ/rtdd/internal/coverage`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/coverage/
git commit -m "coverage: read the arc table when the host enables branch coverage

Measured: [run] branch = True leaves line_bits with zero rows and puts everything
in arc, so the contract query yields an empty map on a run that exits 0. Positive
fromno/tono reproduces the line-mode answer exactly."
```

---

## Task 10 — `internal/report`: `ReadReportLog`

**Files:** `internal/report/reportlog.go`, `internal/report/reportlog_test.go`, `internal/report/testdata/report.jsonl`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
type Outcome struct {
    Test       string
    Status     string // "pass" | "fail" | "skip" | "error"
    DurationMS int
}

// ReadReportLog parses pytest --report-log JSONL.
func ReadReportLog(path string) ([]Outcome, error)
```

**Why this task exists.** `s` and `d` appear in **no coverage report** — coverage.py carries
neither an outcome nor a duration (audit: "`s` and `d` had no data source"). They come from
`pytest --report-log` and nowhere else.

**The measured status rules** (the naive "read the call phase" reader gets all three wrong):

| observed phases | `Status` | `DurationMS` |
|---|---|---|
| setup=passed, call=passed, teardown=passed | `pass` | call duration |
| setup=passed, call=failed, teardown=passed | `fail` | call duration |
| setup=passed, call=skipped, teardown=passed | `skip` | call duration |
| setup=**skipped**, teardown=passed, **no call entry** | `skip` | setup+teardown |
| setup=**failed**, teardown=passed, **no call entry** | `error` | setup+teardown |
| setup=passed, call=passed, teardown=**failed** | `error` | call duration |

`duration` in the JSONL is a float in **seconds**; `d` is milliseconds.

- [ ] **Step 1: Write the failing test**

Create `internal/report/testdata/report.jsonl` with exactly these nine lines (each line is
one JSON object; these are trimmed but otherwise verbatim entries from a real
`pytest --report-log` run on this machine):

```
{"pytest_version": "9.0.3", "$report_type": "SessionStart"}
{"nodeid": "tests/test_a.py", "outcome": "passed", "longrepr": null, "result": null, "sections": [], "$report_type": "CollectReport"}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_add", "location": ["tests/test_a.py", 5, "test_add"], "when": "setup", "outcome": "passed", "duration": 0.006705367937684059, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_add", "location": ["tests/test_a.py", 5, "test_add"], "when": "call", "outcome": "passed", "duration": 0.4120175229886546, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_add", "location": ["tests/test_a.py", 5, "test_add"], "when": "teardown", "outcome": "passed", "duration": 0.0013177299406379461, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_param[1-one two]", "location": ["tests/test_a.py", 9, "test_param[1-one two]"], "when": "setup", "outcome": "passed", "duration": 0.0012868730118498206, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_param[1-one two]", "location": ["tests/test_a.py", 9, "test_param[1-one two]"], "when": "call", "outcome": "passed", "duration": 0.0004828959936276078, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_a.py::test_param[1-one two]", "location": ["tests/test_a.py", 9, "test_param[1-one two]"], "when": "teardown", "outcome": "passed", "duration": 0.0012668049894273281, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_b.py::test_fail", "location": ["tests/test_b.py", 8, "test_fail"], "when": "setup", "outcome": "passed", "duration": 0.00035551097244024277, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_b.py::test_fail", "location": ["tests/test_b.py", 8, "test_fail"], "when": "call", "outcome": "failed", "duration": 0.0013871999690309167, "longrepr": {"reprcrash": {"message": "assert 6 == 7"}}}
{"$report_type": "TestReport", "nodeid": "tests/test_b.py::test_fail", "location": ["tests/test_b.py", 8, "test_fail"], "when": "teardown", "outcome": "passed", "duration": 0.0011728830868378282, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_b.py::test_skipped", "location": ["tests/test_b.py", 12, "test_skipped"], "when": "setup", "outcome": "skipped", "duration": 0.0004462630022317171, "longrepr": ["/repo/tests/test_b.py", 12, "Skipped: nope"]}
{"$report_type": "TestReport", "nodeid": "tests/test_b.py::test_skipped", "location": ["tests/test_b.py", 12, "test_skipped"], "when": "teardown", "outcome": "passed", "duration": 0.00037643895484507084, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_d.py::test_errors", "location": ["tests/test_d.py", 8, "test_errors"], "when": "setup", "outcome": "failed", "duration": 0.000896, "longrepr": {"reprcrash": {"message": "RuntimeError: fixture blew up"}}}
{"$report_type": "TestReport", "nodeid": "tests/test_d.py::test_errors", "location": ["tests/test_d.py", 8, "test_errors"], "when": "teardown", "outcome": "passed", "duration": 0.000308, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_e.py::test_teardown_boom", "location": ["tests/test_e.py", 4, "test_teardown_boom"], "when": "setup", "outcome": "passed", "duration": 0.0004, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_e.py::test_teardown_boom", "location": ["tests/test_e.py", 4, "test_teardown_boom"], "when": "call", "outcome": "passed", "duration": 0.002, "longrepr": null}
{"$report_type": "TestReport", "nodeid": "tests/test_e.py::test_teardown_boom", "location": ["tests/test_e.py", 4, "test_teardown_boom"], "when": "teardown", "outcome": "failed", "duration": 0.0009, "longrepr": {"reprcrash": {"message": "RuntimeError: teardown blew up"}}}
{"exitstatus": 1, "$report_type": "SessionFinish"}
```

`internal/report/reportlog_test.go`:

```go
package report

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadReportLog(t *testing.T) {
	got, err := ReadReportLog(filepath.Join("testdata", "report.jsonl"))
	if err != nil {
		t.Fatalf("ReadReportLog: %v", err)
	}
	want := []Outcome{
		{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 412},
		{Test: "tests/test_a.py::test_param[1-one two]", Status: "pass", DurationMS: 0},
		{Test: "tests/test_b.py::test_fail", Status: "fail", DurationMS: 1},
		// No `call` entry at all: pytest emits setup(skipped) + teardown(passed).
		// A reader that only looks at the call phase loses this test entirely.
		{Test: "tests/test_b.py::test_skipped", Status: "skip", DurationMS: 1},
		// A fixture that raises: setup(failed) + teardown(passed), no call phase.
		{Test: "tests/test_d.py::test_errors", Status: "error", DurationMS: 1},
		// A passing test whose teardown blows up is an error, not a pass.
		{Test: "tests/test_e.py::test_teardown_boom", Status: "error", DurationMS: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadReportLog =\n  %+v\nwant\n  %+v", got, want)
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
		"tests/test_d.py::test_errors",
		"tests/test_e.py::test_teardown_boom",
	}
	for i, w := range wantOrder {
		if got[i].Test != w {
			t.Fatalf("Outcome[%d].Test = %q, want %q", i, got[i].Test, w)
		}
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/report/
```

Expected: `internal/report/reportlog_test.go:12:14: undefined: ReadReportLog` and
`undefined: Outcome` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/report/reportlog.go`:

```go
// Package report parses pytest's --report-log JSONL. It is the ONLY source of the
// map's `s` (outcome) and `d` (duration) fields — no coverage report carries
// either one.
package report

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Outcome is one test's result for a single run.
type Outcome struct {
	Test       string
	Status     string // "pass" | "fail" | "skip" | "error"
	DurationMS int
}

type rawReport struct {
	Type     string  `json:"$report_type"`
	NodeID   string  `json:"nodeid"`
	When     string  `json:"when"`
	Outcome  string  `json:"outcome"`
	Duration float64 `json:"duration"` // SECONDS
}

type phaseAcc struct {
	setupOutcome, callOutcome, teardownOutcome string
	setupDur, callDur, teardownDur             float64
	hasCall                                    bool
}

// ReadReportLog parses pytest --report-log JSONL into one Outcome per test, in
// first-seen order.
//
// The call phase is the primary source, but it is not always present: a skipped
// test emits setup(skipped)+teardown(passed) and NO call entry, and a test whose
// fixture raises emits setup(failed)+teardown(passed) and no call entry. Both are
// measured on pytest 9.0.3. A reader that only inspects the call phase silently
// loses those tests.
//
// A malformed line is a fatal error, never a silent skip — dropping outcomes
// narrows the map.
func ReadReportLog(p string) ([]Outcome, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("report: opening %s: %w", p, err)
	}
	defer f.Close()

	accs := map[string]*phaseAcc{}
	var order []string

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // longrepr tracebacks get long
	for ln := 1; sc.Scan(); ln++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var r rawReport
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("report: %s line %d: %w", p, ln, err)
		}
		if r.Type != "TestReport" || r.NodeID == "" {
			continue
		}
		a := accs[r.NodeID]
		if a == nil {
			a = &phaseAcc{}
			accs[r.NodeID] = a
			order = append(order, r.NodeID)
		}
		switch r.When {
		case "setup":
			a.setupOutcome, a.setupDur = r.Outcome, r.Duration
		case "call":
			a.hasCall = true
			a.callOutcome, a.callDur = r.Outcome, r.Duration
		case "teardown":
			a.teardownOutcome, a.teardownDur = r.Outcome, r.Duration
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("report: reading %s: %w", p, err)
	}

	out := make([]Outcome, 0, len(order))
	for _, id := range order {
		status, ms := finalize(accs[id])
		out = append(out, Outcome{Test: id, Status: status, DurationMS: ms})
	}
	return out, nil
}

func finalize(a *phaseAcc) (string, int) {
	status := "error"
	switch {
	case a.setupOutcome == "failed":
		status = "error"
	case a.setupOutcome == "skipped":
		status = "skip"
	case a.hasCall:
		switch a.callOutcome {
		case "passed":
			status = "pass"
		case "failed":
			status = "fail"
		case "skipped":
			status = "skip"
		}
		if status == "pass" && a.teardownOutcome == "failed" {
			status = "error"
		}
	}
	sec := a.callDur
	if !a.hasCall {
		sec = a.setupDur + a.teardownDur
	}
	return status, int(math.Round(sec * 1000))
}
```

Record the contract clarification. In `docs/plans/00-interfaces.md`, under
`## internal/report`, replace the `ReadReportLog` doc comment with:

```go
// ReadReportLog parses pytest --report-log JSONL into one Outcome per test, in
// first-seen order. Measured phase rules (pytest 9.0.3):
//   setup=failed, no call entry            -> "error"
//   setup=skipped, no call entry           -> "skip"
//   call=passed|failed|skipped             -> "pass"|"fail"|"skip"
//   call=passed but teardown=failed        -> "error"
// DurationMS is the call phase's duration in ms, or setup+teardown when there is
// no call entry. `duration` in the JSONL is a float in seconds.
// A malformed line is a fatal error, never a silent skip.
func ReadReportLog(path string) ([]Outcome, error)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/report/ -v
```

Expected: six `--- PASS` lines, then `ok  	github.com/VocanicZ/rtdd/internal/report`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/report/ docs/plans/00-interfaces.md
git commit -m "report: parse pytest --report-log for the map's s and d fields

Neither is present in any coverage report. Handles the two measured no-call-phase
shapes (skip at setup, fixture error at setup) that a call-phase-only reader drops."
```

---

## Task 11 — `internal/pytestfixture`: a real pytest project for integration tests

**Files:** `internal/pytestfixture/fixture.go`, `internal/pytestfixture/fixture_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
// Materialize writes a minimal, self-contained pytest project into dir.
func Materialize(dir string) error

// InitGit turns dir into a git repository with one commit, so internal/gitctx
// can operate on it.
func InitGit(dir string) error

// HavePytest reports whether `pytest` is on PATH.
func HavePytest() bool
```

**The fixture's measured properties** — every integration test in this plan depends on them:

- `src/constants.py` is attributed to **zero test contexts** (audit A1): it is a dataclass plus a module constant, imported and asserted on by passing tests, and lands entirely in the empty context with lines `[1, 3, 5, 6, 7]`.
- `src/logic.py`'s import-time lines are `[1, 4, 8, 12]`; `test_add` covers line `5`; `test_mul` and `test_fail` cover line `9`; `unused` (line 13) is covered by nothing.
- `tests/test_a.py` contains a parametrised test producing the ids `test_param[1-one two]` (a space) and `test_param[2-a-b]` (a hyphen).
- `tests/test_b.py` contains one deliberate failure and one skip, so a run over it exits 1 and exercises the `fail` and `skip` status paths.

- [ ] **Step 1: Write the failing test**

`internal/pytestfixture/fixture_test.go`:

```go
package pytestfixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeWritesTheProject(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	for _, rel := range []string{
		"pyproject.toml", "src/__init__.py", "src/constants.py", "src/logic.py",
		"tests/__init__.py", "tests/test_a.py", "tests/test_b.py",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

// The line numbers in every coverage assertion in this milestone depend on these
// exact layouts. Pin them.
func TestFixtureLineNumbersArePinned(t *testing.T) {
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	cases := []struct {
		rel  string
		line int
		want string
	}{
		{"src/constants.py", 1, "from dataclasses import dataclass"},
		{"src/constants.py", 3, "MAX = 10"},
		{"src/constants.py", 5, "@dataclass"},
		{"src/constants.py", 6, "class Cfg:"},
		{"src/constants.py", 7, "    a: int = 1"},
		{"src/logic.py", 1, "from src.constants import MAX"},
		{"src/logic.py", 4, "def add(a, b):"},
		{"src/logic.py", 5, "    return a + b"},
		{"src/logic.py", 8, "def mul(a, b):"},
		{"src/logic.py", 9, "    return a * b"},
		{"src/logic.py", 12, "def unused(x):"},
		{"src/logic.py", 13, "    return x - 1"},
	}
	for _, tc := range cases {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tc.rel)))
		if err != nil {
			t.Fatalf("read %s: %v", tc.rel, err)
		}
		lines := strings.Split(string(b), "\n")
		if len(lines) < tc.line {
			t.Fatalf("%s has %d lines, want at least %d", tc.rel, len(lines), tc.line)
		}
		if got := lines[tc.line-1]; got != tc.want {
			t.Errorf("%s:%d = %q, want %q", tc.rel, tc.line, got, tc.want)
		}
	}
}

func TestInitGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if err := InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	if len(strings.TrimSpace(string(out))) < 4 {
		t.Fatalf("HEAD = %q, want a short SHA", out)
	}
}

// This is the audit A1 case reproduced end to end: a correctly tested
// constants.py is attributed to no test at all.
func TestFixtureRunsUnderRealPytest(t *testing.T) {
	if !HavePytest() {
		t.Skip("pytest not on PATH")
	}
	dir := t.TempDir()
	if err := Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	cmd := exec.Command("pytest", "--cov", "--cov-context=test", "--cov-report=", "-q")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "COVERAGE_CORE=ctrace", "COVERAGE_FILE=.coverage")
	out, _ := cmd.CombinedOutput()
	s := string(out)
	if !strings.Contains(s, "1 failed") || !strings.Contains(s, "1 skipped") {
		t.Fatalf("fixture suite summary changed; want 1 failed and 1 skipped. output:\n%s", s)
	}
	if strings.Contains(s, "no-sysmon-context") {
		t.Fatalf("COVERAGE_CORE=ctrace did not take effect:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(dir, ".coverage")); err != nil {
		t.Fatalf(".coverage not written to the repo root: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/pytestfixture/
```

Expected: `internal/pytestfixture/fixture_test.go:14:12: undefined: Materialize` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/pytestfixture/fixture.go`:

```go
// Package pytestfixture materialises a tiny, self-contained pytest project on
// disk so the coverage, report and runner packages can be tested against the
// real toolchain rather than against a mock of it.
//
// The fixture's line numbers and test ids are pinned by fixture_test.go, because
// every coverage assertion in M1b depends on them.
package pytestfixture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PyProject makes the directory a pytest rootdir, which is what makes coverage
// contexts and report-log nodeids repo-root-relative.
const PyProject = "[project]\nname = \"rtddfixture\"\nversion = \"0.1.0\"\n"

// Constants is audit A1 in miniature: imported and asserted on by two passing
// tests, attributed to zero test contexts. Import-time lines: 1, 3, 5, 6, 7.
const Constants = `from dataclasses import dataclass

MAX = 10

@dataclass
class Cfg:
    a: int = 1
`

// Logic has import-time lines 1, 4, 8, 12; add's body is line 5, mul's is line 9,
// and unused's line 13 is covered by nothing.
const Logic = `from src.constants import MAX


def add(a, b):
    return a + b


def mul(a, b):
    return a * b


def unused(x):
    return x - 1
`

// TestA produces the parametrised ids `test_param[1-one two]` (a space) and
// `test_param[2-a-b]` (a hyphen), both of which must round-trip as selectors.
const TestA = `import pytest
from src.logic import add
from src.constants import MAX, Cfg


def test_add():
    assert add(1, 2) == 3


@pytest.mark.parametrize("n,label", [(1, "one two"), (2, "a-b")])
def test_param(n, label):
    assert add(n, 0) == n
    assert isinstance(label, str)


def test_const():
    assert MAX == 10
    assert Cfg().a == 1
`

// TestB carries one deliberate failure and one skip, so a run over it exits 1 and
// exercises the "fail" and "skip" status paths.
const TestB = `import pytest
from src.logic import mul


def test_mul():
    assert mul(2, 3) == 6


def test_fail():
    assert mul(2, 3) == 7


@pytest.mark.skip(reason="nope")
def test_skipped():
    assert False
`

// Files is the whole fixture, keyed by repo-relative slash path.
var Files = map[string]string{
	"pyproject.toml":    PyProject,
	"src/__init__.py":   "",
	"src/constants.py":  Constants,
	"src/logic.py":      Logic,
	"tests/__init__.py": "",
	"tests/test_a.py":   TestA,
	"tests/test_b.py":   TestB,
}

// Materialize writes the fixture project into dir.
func Materialize(dir string) error {
	for rel, body := range Files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("pytestfixture: %w", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return fmt.Errorf("pytestfixture: %w", err)
		}
	}
	return nil
}

// InitGit turns dir into a git repository with one commit containing the whole
// fixture, so internal/gitctx can compute a changed set and a HEAD SHA.
func InitGit(dir string) error {
	steps := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "rtdd@example.invalid"},
		{"config", "user.name", "rtdd fixture"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "fixture"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=rtdd fixture", "GIT_AUTHOR_EMAIL=rtdd@example.invalid",
			"GIT_COMMITTER_NAME=rtdd fixture", "GIT_COMMITTER_EMAIL=rtdd@example.invalid",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("pytestfixture: git %v: %w: %s", args, err, out)
		}
	}
	return nil
}

// HavePytest reports whether `pytest` is on PATH. Integration tests skip without it.
func HavePytest() bool {
	_, err := exec.LookPath("pytest")
	return err == nil
}
```

Record the new package. In `docs/plans/00-interfaces.md`, in the **Package layout** block,
add this line after `internal/report/`:

```
internal/pytestfixture/  test-only: materialises a real pytest project on disk
```

and add a section before `## internal/selector`, containing this heading, this
paragraph, and this Go block:

> ## internal/pytestfixture
>
> Test-only. Materialises a tiny real pytest project so coverage/report/runner can be
> tested against the actual toolchain. Never imported by `cmd/`.
>
> ```go
> func Materialize(dir string) error
> func InitGit(dir string) error
> func HavePytest() bool
> ```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/pytestfixture/ -v
```

Expected: `--- PASS: TestMaterializeWritesTheProject`,
`--- PASS: TestFixtureLineNumbersArePinned`, `--- PASS: TestInitGit`,
`--- PASS: TestFixtureRunsUnderRealPytest`, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/pytestfixture/ docs/plans/00-interfaces.md
git commit -m "pytestfixture: a real pytest project for integration tests

Line numbers and test ids are pinned by test, because every coverage assertion in
M1b depends on them. src/constants.py reproduces audit A1: zero test attribution."
```

---

## Task 12 — `internal/runner`: `Chunk` and `MaxArgvBytes`

**Files:** `internal/runner/chunk.go`, `internal/runner/chunk_test.go`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
const MaxArgvBytes = 100_000 // conservative; Windows CMD is 8191 chars, Linux ARG_MAX is 2MB

func Chunk(tests []string, maxBytes int) [][]string
```

**Why this task exists.** Measured: 8,000 realistic pytest ids occupy 460 KB of argv (spec
§8 cites 613 KB for longer ids). Linux `ARG_MAX` measured at 2,097,152 — it fits. The
Windows `CMD` limit is 8,191 characters — 57–74× over. **pytest has no argfile option**, so
the ids must go on the command line and the run must be split.

- [ ] **Step 1: Write the failing test**

`internal/runner/chunk_test.go`:

```go
package runner

import (
	"fmt"
	"reflect"
	"testing"
)

func TestChunk(t *testing.T) {
	cases := []struct {
		name     string
		tests    []string
		maxBytes int
		want     [][]string
	}{
		{name: "nil", tests: nil, maxBytes: 100, want: nil},
		{name: "empty", tests: []string{}, maxBytes: 100, want: nil},
		{
			name:     "everything fits in one chunk",
			tests:    []string{"a", "b", "c"},
			maxBytes: 100,
			want:     [][]string{{"a", "b", "c"}},
		},
		{
			// len(id)+1 accounts for the NUL separator each argv element costs.
			// "aaaaaaaaaa" is 10 bytes -> 11 each. 11+11=22 <= 25; +11=33 > 25.
			name:     "splits when the next id would exceed the budget",
			tests:    []string{"aaaaaaaaaa", "bbbbbbbbbb", "cccccccccc"},
			maxBytes: 25,
			want:     [][]string{{"aaaaaaaaaa", "bbbbbbbbbb"}, {"cccccccccc"}},
		},
		{
			name:     "exactly on the boundary",
			tests:    []string{"aaaaaaaaaa", "bbbbbbbbbb"},
			maxBytes: 22,
			want:     [][]string{{"aaaaaaaaaa", "bbbbbbbbbb"}},
		},
		{
			name:     "one id larger than the budget gets its own chunk, never dropped",
			tests:    []string{"a", "this-single-id-is-far-longer-than-the-budget", "b"},
			maxBytes: 10,
			want: [][]string{
				{"a"},
				{"this-single-id-is-far-longer-than-the-budget"},
				{"b"},
			},
		},
		{
			name:     "non-positive maxBytes falls back to MaxArgvBytes",
			tests:    []string{"a", "b"},
			maxBytes: 0,
			want:     [][]string{{"a", "b"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Chunk(tc.tests, tc.maxBytes)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Chunk(%q, %d) =\n  %q\nwant\n  %q", tc.tests, tc.maxBytes, got, tc.want)
			}
		})
	}
}

// Measured on this machine: 8,000 realistic pytest ids average 59 bytes and total
// 460 KB of argv. Linux ARG_MAX is 2,097,152 (fits); the Windows CMD limit is
// 8,191 characters (57x over). pytest has no argfile option, so this must chunk.
func TestChunkAtRealisticScale(t *testing.T) {
	tests := make([]string, 8000)
	for i := range tests {
		tests[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	total := 0
	for _, s := range tests {
		total += len(s) + 1
	}
	if total < 400_000 {
		t.Fatalf("fixture ids total %d bytes, want a realistic >400KB argv", total)
	}

	chunks := Chunk(tests, MaxArgvBytes)
	if len(chunks) < 2 {
		t.Fatalf("Chunk produced %d chunk(s) for %d bytes of ids; it must split", len(chunks), total)
	}

	var seen []string
	for i, c := range chunks {
		if len(c) == 0 {
			t.Fatalf("chunk %d is empty", i)
		}
		size := 0
		for _, s := range c {
			size += len(s) + 1
		}
		if size > MaxArgvBytes {
			t.Fatalf("chunk %d is %d bytes, over MaxArgvBytes=%d", i, size, MaxArgvBytes)
		}
		seen = append(seen, c...)
	}
	if !reflect.DeepEqual(seen, tests) {
		t.Fatalf("chunking lost or reordered ids: got %d, want %d", len(seen), len(tests))
	}
}

func TestMaxArgvBytesIsUnderTheWindowsLimitTimesTwelve(t *testing.T) {
	if MaxArgvBytes != 100_000 {
		t.Fatalf("MaxArgvBytes = %d, want 100000 (the value the interface contract fixes)", MaxArgvBytes)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/runner/
```

Expected: `internal/runner/chunk_test.go:59:11: undefined: Chunk` and
`undefined: MaxArgvBytes` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/runner/chunk.go`:

```go
// Package runner executes an adapter's commands as subprocesses: it forces the
// adapter's environment, chunks test ids across invocations, maps fatal exit
// codes, and merges the per-chunk coverage and report results.
package runner

// MaxArgvBytes is the per-invocation argv budget for test ids.
//
// Measured: 8,000 realistic pytest ids are 460 KB (spec §8 cites 613 KB for
// longer ids). Linux ARG_MAX measured at 2,097,152 — it fits. Windows CMD is
// 8,191 characters — 57-74x over. pytest has no argfile option, so the ids must
// go on the command line and the run must be split.
const MaxArgvBytes = 100_000

// Chunk splits tests into groups whose argv footprint stays under maxBytes,
// counting len(id)+1 per id for the separator. Order is preserved, no chunk is
// empty, and a single id longer than the budget gets its own chunk rather than
// being dropped.
func Chunk(tests []string, maxBytes int) [][]string {
	if len(tests) == 0 {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = MaxArgvBytes
	}
	var out [][]string
	var cur []string
	size := 0
	for _, t := range tests {
		n := len(t) + 1
		if len(cur) > 0 && size+n > maxBytes {
			out = append(out, cur)
			cur, size = nil, 0
		}
		cur = append(cur, t)
		size += n
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/runner/ -v
```

Expected: `--- PASS: TestChunk` with all seven sub-tests,
`--- PASS: TestChunkAtRealisticScale`,
`--- PASS: TestMaxArgvBytesIsUnderTheWindowsLimitTimesTwelve`, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/runner/
git commit -m "runner: Chunk test ids under an argv budget

8000 ids is 460KB: fits Linux ARG_MAX, 57x over the Windows CMD limit, and pytest
has no argfile option."
```

---

## Task 13 — `internal/runner`: `ErrSysmonContext`, `hasSysmonWarning`, `FatalExitError`

**Files:** `internal/runner/errors.go`, `internal/runner/errors_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing.

*Produces:*
```go
// ErrSysmonContext is returned when the run emitted coverage.py's
// "no-sysmon-context" warning. Callers MUST exit 3. Never proceed with the map.
var ErrSysmonContext = errors.New("dynamic contexts unavailable: COVERAGE_CORE=sysmon")

// FatalExitError is a chunk that exited with a code the adapter maps
// (4=bad-selector, 5=no-tests-collected). NOT a test failure. CONTRACT ADDITION.
type FatalExitError struct {
    Chunk int
    Code  int
    Label string
}
func (e *FatalExitError) Error() string
```
plus the unexported `hasSysmonWarning(combined []byte) bool`.

**Why this task exists (audit A7).** Measured on this machine: under
`COVERAGE_CORE=sysmon`, 4 tests ran and **1** non-empty context was recorded — a ~90%-empty
map on a process that **exited 0**. The warning is the only signal, and it is printed on
**STDOUT**, inside pytest's warnings summary. **STDERR was empty.** A runner that scans
stderr alone finds nothing and writes the corrupted map.

- [ ] **Step 1: Write the failing test**

`internal/runner/errors_test.go`:

```go
package runner

import (
	"errors"
	"strings"
	"testing"
)

// realWarningStdout is the verbatim stdout of
// `COVERAGE_CORE=sysmon pytest --cov --cov-context=test -q` on this machine
// (coverage.py 7.15.4, pytest 9.0.3). Note where the warning appears: in pytest's
// warnings summary, on STDOUT. Stderr was EMPTY. The process exited 1 only
// because a test failed; with all tests passing it exits 0.
const realWarningStdout = `.....Fs                                                                  [100%]
=============================== warnings summary ===============================
tests/test_a.py::test_add
  /home/claude/.local/lib/python3.13/site-packages/coverage/control.py:799: CoverageWarning: Dynamic contexts aren't supported with core=sysmon; context data may be incomplete (no-sysmon-context); see https://coverage.readthedocs.io/en/7.15.4/messages.html#warning-no-sysmon-context
    self._warn(

-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html
================================ tests coverage ================================
`

const cleanRun = `.....Fs                                                                  [100%]
=========================== short test summary info ============================
FAILED tests/test_b.py::test_fail - assert 6 == 7
==================== 1 failed, 5 passed, 1 skipped in 0.14s ====================
`

func TestHasSysmonWarning(t *testing.T) {
	if !hasSysmonWarning([]byte(realWarningStdout)) {
		t.Fatal("hasSysmonWarning missed the real measured warning; the map would be ~90% empty with exit 0")
	}
	if hasSysmonWarning([]byte(cleanRun)) {
		t.Fatal("hasSysmonWarning fired on a clean run")
	}
	if hasSysmonWarning(nil) {
		t.Fatal("hasSysmonWarning fired on empty output")
	}
}

func TestHasSysmonWarningMatchesTheStableToken(t *testing.T) {
	// coverage.py's message prose and its version-pinned URL both change between
	// releases; the parenthesised warning id does not. Match on that.
	if !hasSysmonWarning([]byte("CoverageWarning: something totally reworded (no-sysmon-context); see https://example/9.9.9/x")) {
		t.Fatal("hasSysmonWarning must key off the no-sysmon-context token, not the surrounding prose")
	}
}

func TestErrSysmonContextMessage(t *testing.T) {
	if ErrSysmonContext == nil {
		t.Fatal("ErrSysmonContext is nil")
	}
	if !strings.Contains(ErrSysmonContext.Error(), "sysmon") {
		t.Fatalf("ErrSysmonContext.Error() = %q, want it to name sysmon", ErrSysmonContext.Error())
	}
	if !errors.Is(ErrSysmonContext, ErrSysmonContext) {
		t.Fatal("errors.Is does not match ErrSysmonContext against itself")
	}
}

func TestFatalExitError(t *testing.T) {
	cases := []struct {
		err  *FatalExitError
		want []string
	}{
		{&FatalExitError{Chunk: 0, Code: 4, Label: "bad-selector"}, []string{"4", "bad-selector"}},
		{&FatalExitError{Chunk: 2, Code: 5, Label: "no-tests-collected"}, []string{"2", "5", "no-tests-collected"}},
	}
	for _, tc := range cases {
		msg := tc.err.Error()
		for _, w := range tc.want {
			if !strings.Contains(msg, w) {
				t.Errorf("FatalExitError.Error() = %q, want it to contain %q", msg, w)
			}
		}
		if !strings.Contains(msg, "not a test failure") {
			t.Errorf("FatalExitError.Error() = %q, want it to say this is not a test failure", msg)
		}
	}
}

func TestFatalExitErrorIsRecoverableWithErrorsAs(t *testing.T) {
	var err error = &FatalExitError{Chunk: 1, Code: 4, Label: "bad-selector"}
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatal("errors.As failed to recover *FatalExitError; the CLI needs it to choose exit 2")
	}
	if fe.Code != 4 {
		t.Fatalf("fe.Code = %d, want 4", fe.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -run 'TestHasSysmon|TestErrSysmon|TestFatalExit'
```

Expected: `internal/runner/errors_test.go:33:6: undefined: hasSysmonWarning`,
`undefined: ErrSysmonContext`, `undefined: FatalExitError` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/runner/errors.go`:

```go
package runner

import (
	"bytes"
	"errors"
	"fmt"
)

// ErrSysmonContext is returned when the run emitted coverage.py's
// "no-sysmon-context" warning. Callers MUST exit 3. Never proceed with the map.
//
// Measured (audit A7): under COVERAGE_CORE=sysmon — the default on Python 3.14+
// where supported — 4 tests ran and 1 non-empty context was recorded, and the
// process exited 0. A warning is the only signal there is.
var ErrSysmonContext = errors.New("dynamic contexts unavailable: COVERAGE_CORE=sysmon")

// sysmonToken is coverage.py's stable warning id. The surrounding prose and the
// version-pinned documentation URL both change between releases; this does not.
const sysmonToken = "no-sysmon-context"

// hasSysmonWarning reports whether the run's COMBINED stdout+stderr carries the
// no-sysmon-context warning.
//
// Measured: pytest prints it in its warnings summary on STDOUT. Stderr was empty.
// Scanning stderr alone finds nothing and the corrupted map gets written.
func hasSysmonWarning(combined []byte) bool {
	return bytes.Contains(combined, []byte(sysmonToken))
}

// FatalExitError is a chunk that exited with a code the adapter maps in its
// exit_codes table — 4 (bad-selector) or 5 (no-tests-collected) for pytest.
//
// These look like test failures to a naive caller and are not: 4 means the
// selector RTDD produced was not accepted, 5 means nothing ran at all. Either
// one silently turns a subset run into "no tests failed".
type FatalExitError struct {
	Chunk int
	Code  int
	Label string
}

func (e *FatalExitError) Error() string {
	return fmt.Sprintf("runner: chunk %d exited %d (%s); this is a fatal error, not a test failure",
		e.Chunk, e.Code, e.Label)
}
```

Record the contract addition. In `docs/plans/00-interfaces.md`, under
`## internal/runner`, after the `ErrSysmonContext` declaration, add:

```go
// FatalExitError is a chunk that exited with a code mapped in Adapter.ExitCodes
// (4=bad-selector, 5=no-tests-collected). It is NOT a test failure; the CLI
// recovers it with errors.As and exits 2.
type FatalExitError struct {
    Chunk int
    Code  int
    Label string
}
func (e *FatalExitError) Error() string
```

and amend the `ErrSysmonContext` comment to read:

```go
// ErrSysmonContext is returned when the run emitted coverage.py's
// "no-sysmon-context" warning. Callers MUST exit 3. Never proceed with the map.
// MEASURED: pytest prints this warning on STDOUT, in its warnings summary, and
// stderr is empty — the runner scans the COMBINED stream.
var ErrSysmonContext = errors.New("dynamic contexts unavailable: COVERAGE_CORE=sysmon")
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -v
```

Expected: `--- PASS: TestHasSysmonWarning`,
`--- PASS: TestHasSysmonWarningMatchesTheStableToken`,
`--- PASS: TestErrSysmonContextMessage`, `--- PASS: TestFatalExitError`,
`--- PASS: TestFatalExitErrorIsRecoverableWithErrorsAs`, plus the Task 12 tests, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/runner/ docs/plans/00-interfaces.md
git commit -m "runner: ErrSysmonContext and FatalExitError

The no-sysmon-context warning is printed on STDOUT, not stderr (measured); the
detector's test vector is the real captured output. Adds FatalExitError to the
interface contract."
```

---

## Task 14 — `internal/runner`: `Run`

**Files:** `internal/runner/run.go`, `internal/runner/run_test.go`

**Interfaces:**

*Consumes:* `adapter.Adapter`, `(*Adapter).ExpandTests`, `(*Adapter).Expand` (Tasks 1, 4);
`coverage.ReadSQLite`, `coverage.Result`, `(*Result).Merge` (Tasks 7–9);
`report.ReadReportLog`, `report.Outcome` (Task 10); `Chunk`, `MaxArgvBytes` (Task 12);
`ErrSysmonContext`, `hasSysmonWarning`, `FatalExitError` (Task 13).

*Produces:*
```go
type RunResult struct {
    Outcomes []report.Outcome
    Coverage *coverage.Result
    Failed   []string
    ExitCode int
}

// Run executes the adapter's subset command. It sets Adapter.Env, chunks test ids
// across multiple invocations when argv would exceed MaxArgvBytes, and merges the
// results. A chunk that exits with a mapped ExitCode (4=bad-selector,
// 5=no-tests-collected) is a fatal error, not a test failure.
func Run(a *adapter.Adapter, repoRoot string, tests []string, failFast bool) (*RunResult, error)
```

- [ ] **Step 1: Write the failing test**

`internal/runner/run_test.go`:

```go
package runner

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// stubScript stands in for pytest so the runner's control flow — chunking, env
// forcing, exit-code mapping, sysmon detection, per-chunk coverage merging — is
// tested deterministically and without a Python interpreter.
const stubScript = `#!/bin/sh
log=""
for a in "$@"; do
  case "$a" in
    --report-log=*) log="${a#--report-log=}" ;;
  esac
done
if [ -n "$RTDD_STUB_ARGS" ]; then
  printf -- '--- invocation ---\n' >> "$RTDD_STUB_ARGS"
  for a in "$@"; do printf '%s\n' "$a" >> "$RTDD_STUB_ARGS"; done
fi
if [ -n "$RTDD_STUB_STDOUT" ]; then
  printf '%s\n' "$RTDD_STUB_STDOUT"
fi
if [ -n "$RTDD_STUB_STDERR" ]; then
  printf '%s\n' "$RTDD_STUB_STDERR" >&2
fi
if [ -n "$log" ] && [ -n "$RTDD_STUB_REPORT" ] && [ -f "$RTDD_STUB_REPORT" ]; then
  cat "$RTDD_STUB_REPORT" > "$log"
fi
if [ -n "$RTDD_STUB_COVERAGE" ] && [ -f "$RTDD_STUB_COVERAGE" ]; then
  cp "$RTDD_STUB_COVERAGE" .coverage
fi
exit ${RTDD_STUB_EXIT:-0}
`

const stubDDL = `
CREATE TABLE meta (key text, value text, unique (key));
CREATE TABLE file (id integer primary key, path text, unique (path));
CREATE TABLE context (id integer primary key, context text, unique (context));
CREATE TABLE line_bits (file_id integer, context_id integer, numbits blob,
    unique (file_id, context_id));
CREATE TABLE arc (file_id integer, context_id integer, fromno integer, tono integer,
    unique (file_id, context_id, fromno, tono));
`

// writeStubCoverage builds a .coverage template holding one test context.
func writeStubCoverage(t *testing.T, path, absSrc, ctx string, numbits []byte) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open stub coverage: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(stubDDL); err != nil {
		t.Fatalf("stub ddl: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('version','7.15.4'),('has_arcs','0')`); err != nil {
		t.Fatalf("stub meta: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO file (id, path) VALUES (1, ?)`, absSrc); err != nil {
		t.Fatalf("stub file: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO context (id, context) VALUES (1, ?)`, ctx); err != nil {
		t.Fatalf("stub context: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO line_bits (file_id, context_id, numbits) VALUES (1, 1, ?)`, numbits); err != nil {
		t.Fatalf("stub line_bits: %v", err)
	}
}

// stubEnv builds an adapter whose subset/seed commands are the stub script.
func stubAdapter(t *testing.T, repo string, env map[string]string) *adapter.Adapter {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub runner script is POSIX sh")
	}
	stub := filepath.Join(repo, "stubpytest")
	if strings.ContainsAny(stub, " \t") {
		t.Skipf("temp dir %q contains whitespace; command templates are whitespace-split", stub)
	}
	if err := os.WriteFile(stub, []byte(stubScript), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if env == nil {
		env = map[string]string{}
	}
	return &adapter.Adapter{
		Name:         "stub",
		Detect:       []string{"pyproject.toml"},
		Env:          env,
		Seed:         stub + " --report-log={log}",
		Subset:       stub + " {tests} --report-log={log}",
		List:         stub + " --collect-only",
		Coverage:     "sqlite",
		Report:       "pytest-reportlog",
		FailFastFlag: "-x",
		ExitCodes:    map[int]string{4: "bad-selector", 5: "no-tests-collected"},
	}
}

func writeStubReport(t *testing.T, path string, entries ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("{\"pytest_version\": \"9.0.3\", \"$report_type\": \"SessionStart\"}\n")
	for _, e := range entries {
		b.WriteString(e)
		b.WriteString("\n")
	}
	b.WriteString("{\"exitstatus\": 0, \"$report_type\": \"SessionFinish\"}\n")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write stub report: %v", err)
	}
}

func testReportEntry(nodeID, when, outcome string, durSec float64) string {
	return fmt.Sprintf(
		`{"$report_type": "TestReport", "nodeid": %q, "when": %q, "outcome": %q, "duration": %v, "longrepr": null}`,
		nodeID, when, outcome, durSec)
}

func invocations(t *testing.T, argsFile string) int {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read args file: %v", err)
	}
	return strings.Count(string(b), "--- invocation ---")
}

func TestRunEmptySelectionExecutesNothing(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_ARGS": argsFile})

	res, err := Run(a, repo, nil, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if len(res.Outcomes) != 0 {
		t.Errorf("Outcomes = %+v, want none", res.Outcomes)
	}
	if res.Coverage == nil || res.Coverage.ImportTime == nil {
		t.Error("Coverage must be a usable empty Result, not nil")
	}
	if n := invocations(t, argsFile); n != 0 {
		t.Errorf("%d subprocess invocations for an empty selection, want 0", n)
	}
}

func TestRunHappyPath(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")

	writeStubReport(t, reportFile,
		testReportEntry("tests/test_a.py::test_add", "setup", "passed", 0.001),
		testReportEntry("tests/test_a.py::test_add", "call", "passed", 0.412),
		testReportEntry("tests/test_a.py::test_add", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_a.py::test_add|run", []byte{0x20})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	res, err := Run(a, repo, []string{"tests/test_a.py::test_add"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "tests/test_a.py::test_add" {
		t.Fatalf("Outcomes = %+v, want one entry for tests/test_a.py::test_add", res.Outcomes)
	}
	if res.Outcomes[0].Status != "pass" || res.Outcomes[0].DurationMS != 412 {
		t.Errorf("Outcome = %+v, want {pass 412}", res.Outcomes[0])
	}
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want one entry", res.Coverage.PerTest)
	}
	if got := res.Coverage.PerTest[0].Files["src/logic.py"]; len(got) != 1 || got[0] != 5 {
		t.Errorf("Coverage for src/logic.py = %v, want [5]", got)
	}
	if len(res.Failed) != 0 {
		t.Errorf("Failed = %v, want none", res.Failed)
	}
}

func TestRunAdapterEnvReachesTheChild(t *testing.T) {
	// This is the same mechanism COVERAGE_CORE=ctrace depends on. If Adapter.Env
	// does not reach the subprocess, sysmon stays on and the map silently empties.
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_STDOUT":   "env-marker-reached-the-child",
	})
	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want 1 — RTDD_STUB_ARGS did not reach the child", n)
	}
}

func TestRunOverridesInheritedEnv(t *testing.T) {
	// A developer with COVERAGE_CORE=sysmon exported in their shell must still get
	// ctrace. The adapter's value wins over the inherited one.
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	t.Setenv("RTDD_STUB_STDOUT", "inherited-value")
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_STDOUT":   "", // adapter clears it
	})
	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The stub prints RTDD_STUB_STDOUT only when non-empty; nothing to assert on
	// stdout, but the run must have succeeded with the adapter's empty override
	// rather than inheriting "inherited-value" and continuing.
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want 1", n)
	}
}

// AUDIT A7 REGRESSION TEST. The warning arrives on STDOUT and the process exits 0.
// Scanning stderr alone, or trusting the exit code, writes a ~90%-empty map.
func TestRunSysmonWarningOnStdoutIsFatal(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_EXIT":     "0",
		"RTDD_STUB_STDOUT": "  /lib/coverage/control.py:799: CoverageWarning: Dynamic contexts aren't supported " +
			"with core=sysmon; context data may be incomplete (no-sysmon-context); see " +
			"https://coverage.readthedocs.io/en/7.15.4/messages.html#warning-no-sysmon-context",
	})

	_, err := Run(a, repo, []string{"tests/test_a.py::test_add"}, false)
	if !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Run = %v, want ErrSysmonContext — the warning was on stdout and the exit code was 0", err)
	}
}

func TestRunSysmonWarningOnStderrIsAlsoFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_EXIT":   "0",
		"RTDD_STUB_STDERR": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	_, err := Run(a, repo, []string{"t"}, false)
	if !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Run = %v, want ErrSysmonContext", err)
	}
}

func TestRunExit4IsFatalNotATestFailure(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})
	_, err := Run(a, repo, []string{"tests/test_a.py::test_gone"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError", err)
	}
	if fe.Code != 4 || fe.Label != "bad-selector" {
		t.Fatalf("FatalExitError = %+v, want code 4 bad-selector", fe)
	}
}

func TestRunExit5IsFatalNotATestFailure(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "5"})
	_, err := Run(a, repo, []string{"t"}, false)
	var fe *FatalExitError
	if !errors.As(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError", err)
	}
	if fe.Code != 5 || fe.Label != "no-tests-collected" {
		t.Fatalf("FatalExitError = %+v, want code 5 no-tests-collected", fe)
	}
}

func TestRunExit1IsATestFailureNotAnError(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_b.py::test_fail", "setup", "passed", 0.001),
		testReportEntry("tests/test_b.py::test_fail", "call", "failed", 0.002),
		testReportEntry("tests/test_b.py::test_fail", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_b.py::test_fail|run", []byte{0x00, 0x02})

	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
		"RTDD_STUB_EXIT":     "1",
	})
	res, err := Run(a, repo, []string{"tests/test_b.py::test_fail"}, false)
	if err != nil {
		t.Fatalf("Run = %v, want nil error (exit 1 is a test failure, not a fatal error)", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
	want := []string{"tests/test_b.py::test_fail"}
	if len(res.Failed) != 1 || res.Failed[0] != want[0] {
		t.Errorf("Failed = %v, want %v", res.Failed, want)
	}
}

func TestRunUnexpectedExitCodeIsAnError(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "3"})
	if _, err := Run(a, repo, []string{"t"}, false); err == nil {
		t.Fatal("Run with an unmapped nonzero exit = nil error, want error")
	}
}

func TestRunFailFastAppendsTheFlag(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	if _, err := Run(a, repo, []string{"t"}, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, _ := os.ReadFile(argsFile)
	if strings.Contains(string(b), "\n-x\n") {
		t.Fatal("-x was passed without --fail-fast; fail-fast is opt-in only (spec §5, decision D6)")
	}

	os.Remove(argsFile)
	if _, err := Run(a, repo, []string{"t"}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, _ = os.ReadFile(argsFile)
	if !strings.Contains(string(b), "\n-x\n") {
		t.Fatal("--fail-fast did not add the adapter's failfast_flag")
	}
}

// The ids below are the real ones measured from coverage.py contexts. Each must
// arrive at the child as exactly one argv element.
func TestRunPassesHostileIDsAsSeparateArgvElements(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	ids := []string{
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_pipe.py::test_pipe[a|b]",
	}
	if _, err := Run(a, repo, ids, false); err != nil {
		t.Fatalf("Run: %v", err)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	lines := strings.Split(string(b), "\n")
	for _, id := range ids {
		found := false
		for _, l := range lines {
			if l == id {
				found = true
			}
		}
		if !found {
			t.Errorf("id %q did not arrive as its own argv element; got lines %q", id, lines)
		}
	}
}

func TestRunChunksAndMergesAtRealisticScale(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_a.py::test_add", "setup", "passed", 0.001),
		testReportEntry("tests/test_a.py::test_add", "call", "passed", 0.412),
		testReportEntry("tests/test_a.py::test_add", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_a.py::test_add|run", []byte{0x20})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	ids := make([]string, 8000)
	for i := range ids {
		ids[i] = fmt.Sprintf("tests/unit/test_module_%04d.py::test_case_name[param-%d]", i, i)
	}
	res, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := invocations(t, argsFile); n < 2 {
		t.Fatalf("%d invocations for 8000 ids, want the run to be chunked", n)
	}
	// Every chunk wrote the same stub report; Run must dedupe by test id rather
	// than emitting one Outcome per chunk.
	if len(res.Outcomes) != 1 {
		t.Fatalf("Outcomes = %+v, want one deduped entry", res.Outcomes)
	}
	// Every chunk erased and rewrote .coverage; the merged result must still hold it.
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want the merged single entry", res.Coverage.PerTest)
	}
	if got := res.Coverage.PerTest[0].Files["src/logic.py"]; len(got) != 1 || got[0] != 5 {
		t.Fatalf("merged coverage for src/logic.py = %v, want [5]", got)
	}
}

func TestRunRemovesAStaleCoverageBeforeExecuting(t *testing.T) {
	// A .coverage left by an earlier, unrelated run must never be mistaken for
	// this run's output.
	repo := t.TempDir()
	stale := filepath.Join(repo, ".coverage")
	if err := os.WriteFile(stale, []byte("not a sqlite file"), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"), "", []byte{0x12})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})
	res, err := Run(a, repo, []string{"t"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Coverage.ImportTime["src/logic.py"]; len(got) != 2 {
		t.Fatalf("ImportTime[src/logic.py] = %v, want [1 4] from the stub's fresh .coverage", got)
	}
}

func TestRunMissingCoverageAfterAChunkIsAnError(t *testing.T) {
	repo := t.TempDir()
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	writeStubReport(t, reportFile)
	// No RTDD_STUB_COVERAGE: the stub writes no .coverage at all.
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_REPORT": reportFile})
	if _, err := Run(a, repo, []string{"t"}, false); err == nil {
		t.Fatal("Run with no .coverage produced = nil error; a missing store must be fatal, never an empty map")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/runner/
```

Expected: `internal/runner/run_test.go:145:14: undefined: Run` and
`undefined: RunResult` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`internal/runner/run.go`:

```go
package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/report"
)

// RunResult is one complete execution: what ran, what it covered, and what failed.
type RunResult struct {
	Outcomes []report.Outcome
	Coverage *coverage.Result
	Failed   []string
	ExitCode int
}

// Run executes the adapter's subset command over tests.
//
// It sets Adapter.Env (which is how COVERAGE_CORE=ctrace is forced), chunks test
// ids across multiple invocations when argv would exceed MaxArgvBytes, and merges
// the per-chunk results. A chunk that exits with a code mapped in
// Adapter.ExitCodes (4=bad-selector, 5=no-tests-collected) is a fatal error, not
// a test failure. A chunk exiting 1 is a test failure and sets ExitCode.
func Run(a *adapter.Adapter, repoRoot string, tests []string, failFast bool) (*RunResult, error) {
	if len(tests) == 0 {
		return &RunResult{Coverage: &coverage.Result{ImportTime: map[string][]int{}}}, nil
	}
	return execute(a, repoRoot, a.Subset, Chunk(tests, MaxArgvBytes), failFast)
}

// execute runs one command template. chunks == nil means a single invocation with
// no {tests} placeholder (the seed and list path).
func execute(a *adapter.Adapter, repoRoot, tmpl string, chunks [][]string, failFast bool) (*RunResult, error) {
	tmpDir, err := os.MkdirTemp("", "rtdd-run-")
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	covPath := filepath.Join(repoRoot, ".coverage")
	res := &RunResult{Coverage: &coverage.Result{ImportTime: map[string][]int{}}}
	byTest := map[string]int{} // test id -> index in res.Outcomes, for dedupe

	n := len(chunks)
	if n == 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		logPath := filepath.Join(tmpDir, fmt.Sprintf("report-%d.jsonl", i))
		vars := map[string]string{"log": logPath, "out": tmpDir, "src": "."}

		var argv []string
		if len(chunks) == 0 {
			argv, err = a.Expand(tmpl, vars)
		} else {
			argv, err = a.ExpandTests(tmpl, vars, chunks[i])
		}
		if err != nil {
			return nil, err
		}
		if failFast && a.FailFastFlag != "" {
			argv = append(argv, a.FailFastFlag)
		}

		// A .coverage left by an earlier, unrelated run must never be mistaken for
		// this chunk's output. pytest erases it anyway unless --cov-append is
		// passed; removing it first makes that guarantee ours, not pytest's.
		if err := os.Remove(covPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("runner: removing stale %s: %w", covPath, err)
		}

		var combined bytes.Buffer
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = repoRoot // .coverage is written to the process CWD, not the rootdir
		cmd.Env = mergeEnv(os.Environ(), a.Env)
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		runErr := cmd.Run()

		code := 0
		if runErr != nil {
			var ee *exec.ExitError
			if !errors.As(runErr, &ee) {
				return nil, fmt.Errorf("runner: executing %s: %w", argv[0], runErr)
			}
			code = ee.ExitCode()
		}

		// Checked BEFORE the exit code, because the warning is emitted on a run
		// that exits 0 with a ~90%-empty map (audit A7). Measured: pytest prints
		// it on STDOUT; stderr is empty. This scans the combined stream.
		if hasSysmonWarning(combined.Bytes()) {
			return nil, ErrSysmonContext
		}

		if label, ok := a.ExitCodes[code]; ok {
			return nil, &FatalExitError{Chunk: i, Code: code, Label: label}
		}
		if code != 0 && code != 1 {
			return nil, fmt.Errorf("runner: chunk %d: %s exited %d\n%s",
				i, argv[0], code, tail(combined.Bytes(), 4000))
		}
		if code == 1 {
			res.ExitCode = 1
		}

		outs, err := report.ReadReportLog(logPath)
		if err != nil {
			return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
		}
		for _, o := range outs {
			if j, seen := byTest[o.Test]; seen {
				res.Outcomes[j] = o // last invocation wins
				continue
			}
			byTest[o.Test] = len(res.Outcomes)
			res.Outcomes = append(res.Outcomes, o)
		}

		cov, err := coverage.ReadSQLite(covPath, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("runner: chunk %d: %w", i, err)
		}
		res.Coverage.Merge(cov)
	}

	for _, o := range res.Outcomes {
		if o.Status == "fail" || o.Status == "error" {
			res.Failed = append(res.Failed, o.Test)
		}
	}
	sort.Strings(res.Failed)
	if len(res.Failed) > 0 {
		res.ExitCode = 1
	}
	return res, nil
}

// mergeEnv returns base with overrides applied, replacing rather than appending
// so an inherited COVERAGE_CORE=sysmon cannot survive.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, hit := overrides[k]; hit {
				continue
			}
		}
		out = append(out, kv)
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+overrides[k])
	}
	return out
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -v
```

Expected: every `--- PASS` including
`TestRunSysmonWarningOnStdoutIsFatal`, `TestRunExit4IsFatalNotATestFailure`,
`TestRunExit5IsFatalNotATestFailure`, `TestRunChunksAndMergesAtRealisticScale`,
then `ok  	github.com/VocanicZ/rtdd/internal/runner`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/runner/
git commit -m "runner: Run — chunked execution, forced env, fatal exit codes

Scans combined stdout+stderr for no-sysmon-context BEFORE looking at the exit
code, because the corrupting run exits 0. Exit 4/5 are fatal, not test failures.
.coverage is removed and re-read per chunk, because pytest erases it each run."
```

---

## Task 15 — `internal/runner`: `Seed` and `List`

**Files:** `internal/runner/run.go`, `internal/runner/seed_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* `execute`, `mergeEnv`, `FatalExitError`, `hasSysmonWarning` (Task 14);
`(*adapter.Adapter).Expand` (Task 4).

*Produces:*
```go
// Seed runs the adapter's full instrumented seed command once.
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error)

// List returns every test id the adapter's list command reports, in collection
// order. Needed for the T2 tier and for direct-tier discovery. CONTRACT ADDITION.
func List(a *adapter.Adapter, repoRoot string) ([]string, error)
```

**Measured `pytest --collect-only -q` output** — one nodeid per line, then a blank line and
a summary line:

```
tests/test_a.py::test_add
tests/test_a.py::test_param[1-one two]
tests/test_a.py::test_param[2-a-b]
tests/test_a.py::test_const
tests/test_b.py::test_mul
tests/test_b.py::test_fail
tests/test_b.py::test_skipped

9 tests collected in 0.03s
```

An empty suite exits **5**. For `List` that is a legitimately empty suite, not a fatal
error — unlike a subset run, where 5 means the ids RTDD produced selected nothing.

- [ ] **Step 1: Write the failing test**

`internal/runner/seed_test.go`:

```go
package runner

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSeedRunsOnceWithNoTestIDs(t *testing.T) {
	repo := t.TempDir()
	argsFile := filepath.Join(repo, "args.txt")
	reportFile := filepath.Join(repo, "stub-report.jsonl")
	covFile := filepath.Join(repo, "stub-coverage.db")
	writeStubReport(t, reportFile,
		testReportEntry("tests/test_a.py::test_add", "setup", "passed", 0.001),
		testReportEntry("tests/test_a.py::test_add", "call", "passed", 0.412),
		testReportEntry("tests/test_a.py::test_add", "teardown", "passed", 0.001),
	)
	writeStubCoverage(t, covFile, filepath.Join(repo, "src", "logic.py"),
		"tests/test_a.py::test_add|run", []byte{0x20})
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_ARGS":     argsFile,
		"RTDD_STUB_REPORT":   reportFile,
		"RTDD_STUB_COVERAGE": covFile,
	})

	res, err := Seed(a, repo)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if n := invocations(t, argsFile); n != 1 {
		t.Fatalf("%d invocations, want exactly 1 — seed is one full instrumented run", n)
	}
	b, _ := os.ReadFile(argsFile)
	if strings.Contains(string(b), "\ntests/") {
		t.Fatalf("seed passed test ids to the runner; it must run the whole suite. argv:\n%s", b)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Status != "pass" {
		t.Fatalf("Outcomes = %+v, want one passing entry", res.Outcomes)
	}
	if len(res.Coverage.PerTest) != 1 {
		t.Fatalf("Coverage.PerTest = %+v, want one entry", res.Coverage.PerTest)
	}
}

func TestSeedSysmonWarningIsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	if _, err := Seed(a, repo); !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("Seed = %v, want ErrSysmonContext — seeding from a 90%%-empty map is the worst case", err)
	}
}

func TestSeedExit5IsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "5"})
	var fe *FatalExitError
	if _, err := Seed(a, repo); !errors.As(err, &fe) {
		t.Fatalf("Seed = %v, want *FatalExitError; seeding an empty suite must not write an empty map", err)
	}
}

func TestListParsesCollectOnlyOutput(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": strings.Join([]string{
			"tests/test_a.py::test_add",
			"tests/test_a.py::test_param[1-one two]",
			"tests/test_a.py::test_param[2-a-b]",
			"tests/test_a.py::test_const",
			"tests/test_b.py::test_mul",
			"tests/test_b.py::test_fail",
			"tests/test_b.py::test_skipped",
			"",
			"9 tests collected in 0.03s",
		}, "\n"),
	})
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_a.py::test_const",
		"tests/test_b.py::test_mul",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_skipped",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List =\n  %q\nwant\n  %q", got, want)
	}
}

func TestListEmptySuiteExit5IsNotFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_EXIT":   "5",
		"RTDD_STUB_STDOUT": "\nno tests ran in 0.01s",
	})
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List on an empty suite = %v, want nil error", err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %q, want empty", got)
	}
}

func TestListExit4IsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{"RTDD_STUB_EXIT": "4"})
	var fe *FatalExitError
	if _, err := List(a, repo); !errors.As(err, &fe) {
		t.Fatalf("List = %v, want *FatalExitError for exit 4", err)
	}
}

func TestListSysmonWarningIsFatal(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, map[string]string{
		"RTDD_STUB_STDOUT": "CoverageWarning: ... (no-sysmon-context); see ...",
	})
	if _, err := List(a, repo); !errors.Is(err, ErrSysmonContext) {
		t.Fatalf("List = %v, want ErrSysmonContext", err)
	}
}

func TestListWithoutAListCommandIsAnError(t *testing.T) {
	repo := t.TempDir()
	a := stubAdapter(t, repo, nil)
	a.List = ""
	if _, err := List(a, repo); err == nil {
		t.Fatal("List with no list command = nil error, want error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/runner/ -run 'TestSeed|TestList'
```

Expected: `internal/runner/seed_test.go:31:14: undefined: Seed` and
`undefined: List` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/runner/run.go`:

```go
// Seed runs the adapter's full instrumented seed command once, over the whole
// suite. It is the only operation that may shrink a map row, so a partial or
// corrupted seed is worse than no seed: every fatal condition Run treats as fatal
// is fatal here too.
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error) {
	return execute(a, repoRoot, a.Seed, nil, false)
}

// List returns every test id the adapter's list command reports, in collection
// order. The T2 tier and direct-tier discovery both need the full id set.
//
// Measured `pytest --collect-only -q` output is one nodeid per line, terminated
// by a blank line and a summary. An empty suite exits 5, which for List is a
// legitimately empty suite rather than the fatal "the ids I produced selected
// nothing" it means for a subset run.
func List(a *adapter.Adapter, repoRoot string) ([]string, error) {
	if a.List == "" {
		return nil, fmt.Errorf("runner: adapter %s has no list command", a.Name)
	}
	argv, err := a.Expand(a.List, map[string]string{"src": ".", "out": "", "log": ""})
	if err != nil {
		return nil, err
	}

	var combined bytes.Buffer
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repoRoot
	cmd.Env = mergeEnv(os.Environ(), a.Env)
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	runErr := cmd.Run()

	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			return nil, fmt.Errorf("runner: executing %s: %w", argv[0], runErr)
		}
		code = ee.ExitCode()
	}
	if hasSysmonWarning(combined.Bytes()) {
		return nil, ErrSysmonContext
	}
	if label, ok := a.ExitCodes[code]; ok && label != "no-tests-collected" {
		return nil, &FatalExitError{Chunk: 0, Code: code, Label: label}
	}
	if code != 0 {
		if label, ok := a.ExitCodes[code]; !ok || label != "no-tests-collected" {
			return nil, fmt.Errorf("runner: %s --collect-only exited %d\n%s",
				argv[0], code, tail(combined.Bytes(), 4000))
		}
		return nil, nil // an empty suite is empty, not an error
	}

	var ids []string
	for _, line := range strings.Split(combined.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			break // the blank line before pytest's summary terminates the id list
		}
		if !strings.Contains(line, "::") {
			continue
		}
		ids = append(ids, line)
	}
	return ids, nil
}
```

Record the contract addition. In `docs/plans/00-interfaces.md`, under
`## internal/runner`, after the `Seed` declaration, add:

```go
// List returns every test id the adapter's list command reports, in collection
// order. Needed for the T2 tier and for direct-tier discovery. An exit code
// mapped to "no-tests-collected" yields an empty list and a nil error — an empty
// suite is empty, not fatal; any other mapped code is a *FatalExitError.
func List(a *adapter.Adapter, repoRoot string) ([]string, error)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -run 'TestSeed|TestList' -v
```

Expected: `--- PASS: TestSeedRunsOnceWithNoTestIDs`,
`--- PASS: TestSeedSysmonWarningIsFatal`, `--- PASS: TestSeedExit5IsFatal`,
`--- PASS: TestListParsesCollectOnlyOutput`,
`--- PASS: TestListEmptySuiteExit5IsNotFatal`, `--- PASS: TestListExit4IsFatal`,
`--- PASS: TestListSysmonWarningIsFatal`,
`--- PASS: TestListWithoutAListCommandIsAnError`, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/runner/ docs/plans/00-interfaces.md
git commit -m "runner: Seed and List

Seed is one full instrumented run with no test ids. List parses collect-only
output and treats exit 5 as an empty suite, not a fatal error. Adds List to the
interface contract."
```

---

## Task 16 — `internal/runner`: integration test against real pytest

**Files:** `internal/runner/integration_test.go`

**Interfaces:**

*Consumes:* `adapter.Builtin`, `adapter.Detect` (Tasks 1, 3); `Run`, `Seed`, `List`
(Tasks 14–15); `pytestfixture.Materialize`, `pytestfixture.HavePytest` (Task 11).

*Produces:* no new names. This task adds no implementation — if it fails, the bug is in
Tasks 1–15 and is fixed there.

- [ ] **Step 1: Write the failing test**

`internal/runner/integration_test.go`:

```go
package runner

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/pytestfixture"
)

func realPythonRepo(t *testing.T) (string, *adapter.Adapter) {
	t.Helper()
	if !pytestfixture.HavePytest() {
		t.Skip("pytest not on PATH")
	}
	repo := t.TempDir()
	if err := pytestfixture.Materialize(repo); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	a, err := adapter.Detect(repo, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if a.Name != "python" {
		t.Fatalf("Detect = %q, want python", a.Name)
	}
	return repo, a
}

func TestIntegrationSeedAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)

	res, err := Seed(a, repo)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	// The fixture has one deliberate failure.
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1 (tests/test_b.py::test_fail fails on purpose)", res.ExitCode)
	}
	if want := []string{"tests/test_b.py::test_fail"}; !reflect.DeepEqual(res.Failed, want) {
		t.Errorf("Failed = %v, want %v", res.Failed, want)
	}

	byTest := map[string]string{}
	for _, o := range res.Outcomes {
		byTest[o.Test] = o.Status
	}
	wantStatus := map[string]string{
		"tests/test_a.py::test_add":               "pass",
		"tests/test_a.py::test_param[1-one two]":  "pass",
		"tests/test_a.py::test_param[2-a-b]":      "pass",
		"tests/test_a.py::test_const":             "pass",
		"tests/test_b.py::test_mul":               "pass",
		"tests/test_b.py::test_fail":              "fail",
		"tests/test_b.py::test_skipped":           "skip",
	}
	for id, want := range wantStatus {
		got, ok := byTest[id]
		if !ok {
			t.Errorf("no Outcome for %q; got %v", id, byTest)
			continue
		}
		if got != want {
			t.Errorf("Outcome[%q] = %q, want %q", id, got, want)
		}
	}

	// AUDIT A1: src/constants.py is imported and asserted on by two passing tests
	// and is attributed to ZERO test contexts. It must land in ImportTime.
	gotImport, ok := res.Coverage.ImportTime["src/constants.py"]
	if !ok {
		t.Fatalf("src/constants.py missing from ImportTime; got keys %v", keysOf(res.Coverage.ImportTime))
	}
	if want := []int{1, 3, 5, 6, 7}; !reflect.DeepEqual(gotImport, want) {
		t.Errorf("ImportTime[src/constants.py] = %v, want %v", gotImport, want)
	}
	for _, tc := range res.Coverage.PerTest {
		if _, hit := tc.Files["src/constants.py"]; hit {
			t.Errorf("src/constants.py is attributed to test %q; it must be import-time only", tc.Test)
		}
	}

	// Per-test attribution must be real, not empty.
	covByTest := map[string]map[string][]int{}
	for _, tc := range res.Coverage.PerTest {
		covByTest[tc.Test] = tc.Files
	}
	if got := covByTest["tests/test_a.py::test_add"]["src/logic.py"]; !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("test_add covers src/logic.py %v, want [5]", got)
	}
	if got := covByTest["tests/test_b.py::test_mul"]["src/logic.py"]; !reflect.DeepEqual(got, []int{9}) {
		t.Errorf("test_mul covers src/logic.py %v, want [9]", got)
	}
	// Parametrised ids must survive the context round-trip verbatim.
	if _, ok := covByTest["tests/test_a.py::test_param[1-one two]"]; !ok {
		t.Errorf("no coverage entry for the space-containing parametrised id; got %v", keysOfStr(covByTest))
	}
	if _, ok := covByTest["tests/test_a.py::test_param[2-a-b]"]; !ok {
		t.Errorf("no coverage entry for the hyphen-containing parametrised id; got %v", keysOfStr(covByTest))
	}
}

func TestIntegrationRunSubsetAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)

	ids := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
	}
	res, err := Run(a, repo, ids, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	var got []string
	for _, o := range res.Outcomes {
		got = append(got, o.Test)
	}
	sort.Strings(got)
	want := append([]string(nil), ids...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Outcomes = %q, want exactly the selected ids %q — the ids did not round-trip", got, want)
	}
	if _, err := os.Stat(filepath.Join(repo, ".coverage")); err != nil {
		t.Errorf(".coverage was not written to the repo root: %v", err)
	}
}

func TestIntegrationRunWithABadSelectorIsFatal(t *testing.T) {
	repo, a := realPythonRepo(t)
	_, err := Run(a, repo, []string{"tests/test_a.py::test_does_not_exist"}, false)
	var fe *FatalExitError
	if !errorsAs(err, &fe) {
		t.Fatalf("Run = %v, want *FatalExitError; pytest exits 4 on a bad selector and that must not look like a pass", err)
	}
	if fe.Code != 4 {
		t.Fatalf("FatalExitError.Code = %d, want 4", fe.Code)
	}
}

func TestIntegrationListAgainstRealPytest(t *testing.T) {
	repo, a := realPythonRepo(t)
	got, err := List(a, repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_a.py::test_const",
		"tests/test_b.py::test_mul",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_skipped",
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List =\n  %q\nwant\n  %q", got, want)
	}
}

func keysOf(m map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysOfStr(m map[string]map[string][]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
```

Add this one-line helper at the bottom of the same file so the test compiles without a
second `errors` import name clash:

```go
func errorsAs(err error, target **FatalExitError) bool {
	for err != nil {
		if fe, ok := err.(*FatalExitError); ok {
			*target = fe
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -run TestIntegration -v
```

Expected, before Tasks 1–15 are all correct: a real assertion failure, e.g.
`integration_test.go:96: src/constants.py missing from ImportTime; got keys []`.
If pytest is not installed the test SKIPs, which is not a pass — install it first:

```bash
python3 -m pip install --user pytest pytest-cov pytest-reportlog
```

- [ ] **Step 3: Write minimal implementation**

None. This task is pure verification of Tasks 1–15 against the real toolchain. If it fails,
fix the responsible package and re-run — do not weaken the assertions.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go test ./internal/runner/ -run TestIntegration -v
```

Expected: `--- PASS: TestIntegrationSeedAgainstRealPytest`,
`--- PASS: TestIntegrationRunSubsetAgainstRealPytest`,
`--- PASS: TestIntegrationRunWithABadSelectorIsFatal`,
`--- PASS: TestIntegrationListAgainstRealPytest`, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/runner/
git commit -m "runner: integration tests against real pytest

Proves audit A1 (constants.py has zero test attribution), parametrised id
round-tripping, and that a bad selector exits 4 and is fatal rather than a pass."
```

---

## Task 17 — `cmd/rtdd`: shared plumbing (repo root, adapter detection, meta, rows, error mapping)

**Files:** `cmd/rtdd/meta.go`, `cmd/rtdd/rows.go`, `cmd/rtdd/rows_test.go`

**Interfaces:**

*Consumes:* `mapstore.Row` (M1a); `adapter.Builtin`, `adapter.Detect` (Tasks 1, 3);
`runner.RunResult`, `runner.ErrSysmonContext`, `runner.FatalExitError` (Tasks 13–14);
`coverage.Result`, `coverage.TestCoverage` (Task 7); `report.Outcome` (Task 10).

*Produces* (all unexported, `package main`):
```go
type meta struct {
    V        int    `json:"v"`
    Adapter  string `json:"adapter"`
    SeededAt string `json:"seeded_at"`
    Cycles   int    `json:"cycles"`
}
func metaPath(repoRoot string) string
func mapPath(repoRoot string) string
func readMeta(repoRoot string) (meta, error)   // missing file: zero value, nil error
func writeMeta(repoRoot string, m meta) error
func findRepoRoot(start string) (string, error)
func detectAdapter(repoRoot string) (*adapter.Adapter, error)
func rowsFrom(res *runner.RunResult, sha string) []mapstore.Row
func reportRunErr(err error) int
```

> **If M1a already created `cmd/rtdd/meta.go`** (its `status` command needs `cycles`), keep
> the existing file and add only what is missing. Do not create a second `meta` type.

**Design decision recorded here.** `rowsFrom` keeps **every repo-relative path** coverage
reported for a test, including the test's own module and any helper under `tests/`. It does
**not** filter through `adapter.IsInstrumentable`. Filtering would drop `tests/helpers.py`
from every `f`, so editing a shared test helper would select nothing. The audit's finding
stands: *over-selection is safe for selection*; under-selection is not. Paths outside the
repo (site-packages, the stdlib) are already dropped by `coverage.ReadSQLite`.

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/rows_test.go`:

```go
package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/runner"
)

func TestRowsFrom(t *testing.T) {
	res := &runner.RunResult{
		Outcomes: []report.Outcome{
			{Test: "tests/test_a.py::test_add", Status: "pass", DurationMS: 412},
			{Test: "tests/test_b.py::test_fail", Status: "fail", DurationMS: 2},
			// A test that ran but recorded no coverage rows at all: it executed
			// nothing measurable outside already-recorded import-time lines.
			{Test: "tests/test_b.py::test_skipped", Status: "skip", DurationMS: 1},
		},
		Coverage: &coverage.Result{
			PerTest: []coverage.TestCoverage{
				{Test: "tests/test_a.py::test_add", Files: map[string][]int{
					"src/logic.py":    {5},
					"tests/test_a.py": {6, 7},
				}},
				{Test: "tests/test_b.py::test_fail", Files: map[string][]int{
					"src/logic.py": {9},
				}},
			},
			ImportTime: map[string][]int{"src/constants.py": {1, 3, 5, 6, 7}},
		},
	}

	got := rowsFrom(res, "a3f21e0")
	want := []mapRow{
		{T: "tests/test_a.py::test_add", F: []string{"src/logic.py", "tests/test_a.py"}, C: "a3f21e0", D: 412, S: "pass"},
		{T: "tests/test_b.py::test_fail", F: []string{"src/logic.py"}, C: "a3f21e0", D: 2, S: "fail"},
		{T: "tests/test_b.py::test_skipped", F: nil, C: "a3f21e0", D: 1, S: "skip"},
	}
	if len(got) != len(want) {
		t.Fatalf("len(rowsFrom) = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].T != want[i].T || got[i].C != want[i].C || got[i].D != want[i].D || got[i].S != want[i].S {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
		if len(got[i].F) == 0 && len(want[i].F) == 0 {
			continue
		}
		if !reflect.DeepEqual(got[i].F, want[i].F) {
			t.Errorf("row %d F = %v, want %v (sorted)", i, got[i].F, want[i].F)
		}
	}
	// Import-time files belong to no test and must not leak into any f.
	for _, r := range got {
		for _, f := range r.F {
			if f == "src/constants.py" {
				t.Errorf("row %q has import-time file src/constants.py in f", r.T)
			}
		}
	}
}

// mapRow mirrors mapstore.Row's field set so the assertions above read clearly.
type mapRow struct {
	T string
	F []string
	C string
	D int
	S string
}

func TestMetaRoundTrip(t *testing.T) {
	repo := t.TempDir()

	got, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta on a fresh repo: %v", err)
	}
	if (got != meta{}) {
		t.Fatalf("readMeta on a fresh repo = %+v, want the zero value", got)
	}

	want := meta{V: 1, Adapter: "python", SeededAt: "a3f21e0", Cycles: 7}
	if err := writeMeta(repo, want); err != nil {
		t.Fatalf("writeMeta: %v", err)
	}
	got, err = readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if got != want {
		t.Fatalf("readMeta = %+v, want %+v", got, want)
	}

	b, err := os.ReadFile(metaPath(repo))
	if err != nil {
		t.Fatalf("read meta file: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("meta.json is not valid JSON: %v", err)
	}
	for _, k := range []string{"v", "adapter", "seeded_at", "cycles"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("meta.json has no %q key; got %v", k, raw)
		}
	}
}

func TestMetaMalformedIsAnError(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(metaPath(repo)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(metaPath(repo), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readMeta(repo); err == nil {
		t.Fatal("readMeta on malformed meta.json = nil error, want error")
	}
}

func TestFindRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}
	got, err := findRepoRoot(deep)
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	rootEval, _ := filepath.EvalSymlinks(root)
	if gotEval != rootEval {
		t.Fatalf("findRepoRoot = %q, want %q", gotEval, rootEval)
	}
}

func TestFindRepoRootOutsideAnyRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := findRepoRoot(dir); err == nil {
		t.Fatal("findRepoRoot outside a git repo = nil error, want error")
	}
}

func TestReportRunErrExitCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"sysmon is a fatal environment error", runner.ErrSysmonContext, 3},
		{"wrapped sysmon", errors.New("x: " + runner.ErrSysmonContext.Error()), 3},
		{"bad selector is a configuration error", &runner.FatalExitError{Chunk: 0, Code: 4, Label: "bad-selector"}, 2},
		{"no tests collected is a configuration error", &runner.FatalExitError{Chunk: 1, Code: 5, Label: "no-tests-collected"}, 2},
		{"anything else is a fatal environment error", errors.New("boom"), 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err
			if tc.name == "wrapped sysmon" {
				err = wrapErr(runner.ErrSysmonContext)
			}
			if got := reportRunErr(err); got != tc.want {
				t.Fatalf("reportRunErr(%v) = %d, want %d", err, got, tc.want)
			}
		})
	}
}

func wrapErr(e error) error { return errWrap{e} }

type errWrap struct{ e error }

func (w errWrap) Error() string { return "wrapped: " + w.e.Error() }
func (w errWrap) Unwrap() error { return w.e }

// STRUCTURAL GUARD (spec §4, decision D11, audit A4): mapstore.Replace may be
// called from exactly one place in the tree — cmd/rtdd/seed.go. Every other path
// must union, because a subset run legitimately records LESS coverage than the
// seed and a failing test records a truncated prefix.
func TestOnlySeedCallsMapstoreReplace(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	var offenders []string
	err := filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "testdata", "bench":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(p)
		if strings.HasSuffix(rel, "cmd/rtdd/seed.go") || strings.Contains(rel, "internal/mapstore/") {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), ".Replace(") {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(offenders) > 0 {
		t.Fatalf("mapstore.Replace is called outside cmd/rtdd/seed.go: %v\n"+
			"Only rtdd seed may shrink a row (spec §4, D11). Everything else must Union.", offenders)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/rtdd/
```

Expected: `cmd/rtdd/rows_test.go:44:9: undefined: rowsFrom`, `undefined: readMeta`,
`undefined: findRepoRoot`, `undefined: reportRunErr` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`cmd/rtdd/meta.go`:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// meta is .rtdd/meta.json. It is kept OUT of map.jsonl because the JSONL is
// union-merged and this is not union-mergeable.
type meta struct {
	V        int    `json:"v"`
	Adapter  string `json:"adapter"`
	SeededAt string `json:"seeded_at"`
	Cycles   int    `json:"cycles"`
}

func rtddDir(repoRoot string) string  { return filepath.Join(repoRoot, ".rtdd") }
func metaPath(repoRoot string) string { return filepath.Join(rtddDir(repoRoot), "meta.json") }
func mapPath(repoRoot string) string  { return filepath.Join(rtddDir(repoRoot), "map.jsonl") }

// readMeta returns the zero value and a nil error when meta.json does not exist.
// A malformed meta.json is a fatal error, never a silent reset — a reset would
// zero `cycles` and postpone the DriftGuard full run forever.
func readMeta(repoRoot string) (meta, error) {
	b, err := os.ReadFile(metaPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return meta{}, nil
		}
		return meta{}, fmt.Errorf("reading %s: %w", metaPath(repoRoot), err)
	}
	var m meta
	if err := json.Unmarshal(b, &m); err != nil {
		return meta{}, fmt.Errorf("parsing %s: %w", metaPath(repoRoot), err)
	}
	return m, nil
}

func writeMeta(repoRoot string, m meta) error {
	if err := os.MkdirAll(rtddDir(repoRoot), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", rtddDir(repoRoot), err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("encoding meta: %w", err)
	}
	if err := os.WriteFile(metaPath(repoRoot), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", metaPath(repoRoot), err)
	}
	return nil
}
```

`cmd/rtdd/rows.go`:

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// findRepoRoot walks up from start looking for a .git entry.
func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", start)
		}
		dir = parent
	}
}

// detectAdapter picks the one adapter that matches repoRoot, from the set
// embedded in the binary.
func detectAdapter(repoRoot string) (*adapter.Adapter, error) {
	all, err := adapter.Builtin()
	if err != nil {
		return nil, err
	}
	return adapter.Detect(repoRoot, all)
}

// rowsFrom joins a run's coverage and its test report into map rows.
//
// `s` and `d` come from the test report; no coverage report carries either one.
// `f` keeps EVERY repo-relative path coverage reported for the test, including
// the test's own module and helpers under tests/ — filtering to "instrumentable"
// files would drop tests/helpers.py from every row, so editing a shared helper
// would select nothing. Over-selection is safe for selection; under-selection is
// not. Out-of-repo paths were already dropped by coverage.ReadSQLite.
func rowsFrom(res *runner.RunResult, sha string) []mapstore.Row {
	byTest := map[string][]string{}
	if res.Coverage != nil {
		for _, tc := range res.Coverage.PerTest {
			files := make([]string, 0, len(tc.Files))
			for f := range tc.Files {
				files = append(files, f)
			}
			sort.Strings(files)
			byTest[tc.Test] = files
		}
	}
	rows := make([]mapstore.Row, 0, len(res.Outcomes))
	for _, o := range res.Outcomes {
		rows = append(rows, mapstore.Row{
			T: o.Test,
			F: byTest[o.Test],
			C: sha,
			D: o.DurationMS,
			S: o.Status,
		})
	}
	return rows
}

// reportRunErr prints a runner error and returns the process exit code for it.
func reportRunErr(err error) int {
	if errors.Is(err, runner.ErrSysmonContext) {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		fmt.Fprintln(os.Stderr, "rtdd: coverage.py dropped dynamic contexts; the map would be ~90% empty on a run that exits 0.")
		fmt.Fprintln(os.Stderr, "rtdd: the python adapter forces COVERAGE_CORE=ctrace — check for a wrapper script or CI setting that overrides it.")
		return 3
	}
	var fe *runner.FatalExitError
	if errors.As(err, &fe) {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		if fe.Code == 4 {
			fmt.Fprintln(os.Stderr, "rtdd: the test ids rtdd produced were rejected by the runner; the map may be stale. Try: rtdd seed")
		}
		return 2
	}
	fmt.Fprintln(os.Stderr, "rtdd:", err)
	return 3
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./cmd/rtdd/ -v
```

Expected: `--- PASS: TestRowsFrom`, `--- PASS: TestMetaRoundTrip`,
`--- PASS: TestMetaMalformedIsAnError`, `--- PASS: TestFindRepoRoot`,
`--- PASS: TestFindRepoRootOutsideAnyRepo`, `--- PASS: TestReportRunErrExitCodes`,
`--- PASS: TestOnlySeedCallsMapstoreReplace`, then `ok`.

> `TestOnlySeedCallsMapstoreReplace` passes trivially right now because
> `cmd/rtdd/seed.go` does not exist yet. It becomes load-bearing in Task 18 and
> stays load-bearing forever after.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/
git commit -m "cmd/rtdd: repo root, adapter detection, meta.json, row join, exit mapping

Includes the structural guard that mapstore.Replace may only be called from
cmd/rtdd/seed.go (spec D11, audit A4)."
```

---

## Task 18 — `cmd/rtdd`: the `seed` command

**Files:** `cmd/rtdd/seed.go`, `cmd/rtdd/seed_test.go`, `cmd/rtdd/main.go`

**Interfaces:**

*Consumes:* `mapstore.New`, `(*Map).Replace`, `(*Map).Save`, `(*Map).Len` (M1a);
`gitctx.HeadSHA` (M1a); `runner.Seed` (Task 15); `findRepoRoot`, `detectAdapter`,
`rowsFrom`, `reportRunErr`, `writeMeta`, `mapPath` (Task 17).

*Produces:*
```go
func cmdSeed(args []string) int
```

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/seed_test.go`:

```go
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/pytestfixture"
)

// chdir moves into dir for the duration of the test. cmdSeed and cmdRun resolve
// the repo root from the working directory, exactly as the CLI does.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func realRepo(t *testing.T) string {
	t.Helper()
	if !pytestfixture.HavePytest() {
		t.Skip("pytest not on PATH")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := pytestfixture.Materialize(dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if err := pytestfixture.InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	return dir
}

type jsonlRow struct {
	T string   `json:"t"`
	F []string `json:"f"`
	C string   `json:"c"`
	D int      `json:"d"`
	S string   `json:"s"`
}

func readMapJSONL(t *testing.T, repo string) map[string]jsonlRow {
	t.Helper()
	f, err := os.Open(mapPath(repo))
	if err != nil {
		t.Fatalf("open map.jsonl: %v", err)
	}
	defer f.Close()
	out := map[string]jsonlRow{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r jsonlRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("map.jsonl line %q: %v", line, err)
		}
		out[r.T] = r
	}
	return out
}

func TestCmdSeedWritesTheMap(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)

	// The fixture has one deliberate failure, so seed exits 1.
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1 (tests/test_b.py::test_fail fails on purpose)", code)
	}

	rows := readMapJSONL(t, repo)
	wantTests := []string{
		"tests/test_a.py::test_add",
		"tests/test_a.py::test_const",
		"tests/test_a.py::test_param[1-one two]",
		"tests/test_a.py::test_param[2-a-b]",
		"tests/test_b.py::test_fail",
		"tests/test_b.py::test_mul",
		"tests/test_b.py::test_skipped",
	}
	var got []string
	for k := range rows {
		got = append(got, k)
	}
	sort.Strings(got)
	if len(got) != len(wantTests) {
		t.Fatalf("map has %d rows (%v), want %d (%v)", len(got), got, len(wantTests), wantTests)
	}
	for i := range wantTests {
		if got[i] != wantTests[i] {
			t.Fatalf("map row %d = %q, want %q", i, got[i], wantTests[i])
		}
	}

	add := rows["tests/test_a.py::test_add"]
	if add.S != "pass" {
		t.Errorf("test_add s = %q, want pass", add.S)
	}
	if add.C == "" {
		t.Error("test_add c is empty, want the short HEAD SHA")
	}
	if !containsStr(add.F, "src/logic.py") {
		t.Errorf("test_add f = %v, want it to contain src/logic.py", add.F)
	}
	if !sort.StringsAreSorted(add.F) {
		t.Errorf("test_add f = %v, want it sorted", add.F)
	}
	if rows["tests/test_b.py::test_fail"].S != "fail" {
		t.Errorf("test_fail s = %q, want fail", rows["tests/test_b.py::test_fail"].S)
	}
	if rows["tests/test_b.py::test_skipped"].S != "skip" {
		t.Errorf("test_skipped s = %q, want skip", rows["tests/test_b.py::test_skipped"].S)
	}

	// src/constants.py is import-time only (audit A1): no test may claim it.
	for id, r := range rows {
		if containsStr(r.F, "src/constants.py") {
			t.Errorf("row %q claims import-time file src/constants.py", id)
		}
	}
}

func TestCmdSeedWritesMeta(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	m, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.V != 1 {
		t.Errorf("meta.V = %d, want 1", m.V)
	}
	if m.Adapter != "python" {
		t.Errorf("meta.Adapter = %q, want python", m.Adapter)
	}
	if m.SeededAt == "" {
		t.Error("meta.SeededAt is empty, want the short HEAD SHA")
	}
	if m.Cycles != 0 {
		t.Errorf("meta.Cycles = %d, want 0 — seeding resets the cycle counter", m.Cycles)
	}
}

// seed is the ONLY operation that may shrink a row. Re-seeding after a source
// file is deleted must drop it, not keep it forever.
func TestCmdSeedMayShrinkARow(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("first cmdSeed = %d, want 1", code)
	}
	before := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if !containsStr(before.F, "src/logic.py") {
		t.Fatalf("precondition: test_add f = %v, want src/logic.py", before.F)
	}

	// Hand-widen the row, then re-seed: seed replaces, so the phantom must go.
	widened := before
	widened.F = append(append([]string{}, before.F...), "src/phantom.py")
	sort.Strings(widened.F)
	b, err := json.Marshal(widened)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := appendLine(mapPath(repo), string(b)); err != nil {
		t.Fatalf("append: %v", err)
	}

	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("second cmdSeed = %d, want 1", code)
	}
	after := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if containsStr(after.F, "src/phantom.py") {
		t.Fatalf("after re-seed f = %v, still contains src/phantom.py; seed must Replace, not Union", after.F)
	}
}

func TestCmdSeedOutsideARepoIsAUsageError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if code := cmdSeed(nil); code != 2 {
		t.Fatalf("cmdSeed outside a git repo = %d, want 2", code)
	}
}

func TestCmdSeedWithNoAdapterIsAUsageError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := pytestfixture.InitGit(dir); err != nil {
		t.Fatalf("InitGit: %v", err)
	}
	chdir(t, dir)
	if code := cmdSeed(nil); code != 2 {
		t.Fatalf("cmdSeed with no detectable adapter = %d, want 2", code)
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line + "\n")
	return err
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/claude/rtdd && go test ./cmd/rtdd/ -run TestCmdSeed
```

Expected: `cmd/rtdd/seed_test.go:106:14: undefined: cmdSeed` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`cmd/rtdd/seed.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
)

// cmdSeed runs the whole suite once, instrumented, and writes a fresh map.
//
// This is the ONLY operation permitted to shrink a row, and therefore the only
// caller of mapstore.Replace in the tree (spec §4, decision D11, audit A4). A
// subset run legitimately records LESS coverage than a seed — import-time and
// first-caller-wins lines migrate to whichever test ran first, and a failing test
// records a truncated prefix of its real path — so every other command unions.
func cmdSeed(args []string) int {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "rtdd: seed takes no arguments, got %v\n", fs.Args())
		return 2
	}

	root, err := findRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	ad, err := detectAdapter(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	fmt.Printf("seeding with the %s adapter (one full instrumented run)\n", ad.Name)
	res, err := runner.Seed(ad, root)
	if err != nil {
		return reportRunErr(err)
	}

	m := mapstore.New()
	for _, row := range rowsFrom(res, sha) {
		m.Replace(row) // seed only; see the doc comment above
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := writeMeta(root, meta{V: 1, Adapter: ad.Name, SeededAt: sha, Cycles: 0}); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	fmt.Printf("seeded %d tests at %s\n", m.Len(), sha)
	if len(res.Failed) > 0 {
		fmt.Printf("%d failed during seeding: %v\n", len(res.Failed), res.Failed)
	}
	return res.ExitCode
}
```

In `cmd/rtdd/main.go`, add this case to the command switch:

```go
	case "seed":
		os.Exit(cmdSeed(os.Args[2:]))
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go build ./... && go test ./cmd/rtdd/ -run 'TestCmdSeed|TestOnlySeed' -v
```

Expected: `--- PASS: TestCmdSeedWritesTheMap`, `--- PASS: TestCmdSeedWritesMeta`,
`--- PASS: TestCmdSeedMayShrinkARow`, `--- PASS: TestCmdSeedOutsideARepoIsAUsageError`,
`--- PASS: TestCmdSeedWithNoAdapterIsAUsageError`,
`--- PASS: TestOnlySeedCallsMapstoreReplace`, then `ok`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/
git commit -m "cmd/rtdd: seed — one full instrumented run, the only caller of Replace"
```

---

## Task 19 — `cmd/rtdd`: the `run` command

**Files:** `cmd/rtdd/run.go`, `cmd/rtdd/run_test.go`, `cmd/rtdd/main.go`

**Interfaces:**

*Consumes:* `mapstore.Load`, `(*Map).Union`, `(*Map).Save`, `(*Map).Get`, `(*Map).Len`
(M1a); `gitctx.ChangedSet`, `gitctx.HeadSHA`, `gitctx.CommitDistance`, `gitctx.Older`
(M1a); `selector.Select`, `selector.Inputs`, `selector.DefaultConfig`, `selector.Selection`,
`selector.TierEmpty` (M1a); `runner.Run`, `runner.List` (Tasks 14–15); `findRepoRoot`,
`detectAdapter`, `rowsFrom`, `reportRunErr`, `readMeta`, `writeMeta`, `mapPath` (Task 17).

*Produces:*
```go
func cmdRun(args []string) int
```

**The one rule this task exists to enforce (spec §4, decision D11, audit A4):**
`run` calls `mapstore.Union` and **never** `mapstore.Replace`. A subset run legitimately
records *less* coverage than the seed — import-time and first-caller-wins lines migrate to
whichever test ran first in that subset, and a failing test records a truncated prefix of
its real path. v1 replaced on those runs and silently narrowed rows, on a single branch,
with no merge involved.

`cycles` increments on every completed run, pass or fail, so a failing streak still buys an
eventual `DriftGuard` full run.

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/run_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestCmdRunUnionsAndNeverNarrows(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)

	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}

	// Widen test_add's row with a file this run cannot possibly re-record. If run
	// replaced instead of unioning, this would vanish — that is audit A4 exactly.
	rows := readMapJSONL(t, repo)
	add := rows["tests/test_a.py::test_add"]
	if len(add.F) == 0 {
		t.Fatalf("precondition: test_add has empty f")
	}
	widened := add
	widened.F = append(append([]string{}, add.F...), "src/legacy_import_time_only.py")
	sort.Strings(widened.F)
	b, err := json.Marshal(widened)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := appendLine(mapPath(repo), string(b)); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Touch a source file so the selection is non-empty, then run.
	logic := filepath.Join(repo, "src", "logic.py")
	src, err := os.ReadFile(logic)
	if err != nil {
		t.Fatalf("read logic.py: %v", err)
	}
	if err := os.WriteFile(logic, append(src, []byte("\n\ndef added():\n    return 42\n")...), 0o644); err != nil {
		t.Fatalf("write logic.py: %v", err)
	}

	code := cmdRun(nil)
	if code != 0 && code != 1 {
		t.Fatalf("cmdRun = %d, want 0 or 1", code)
	}

	after := readMapJSONL(t, repo)["tests/test_a.py::test_add"]
	if !containsStr(after.F, "src/legacy_import_time_only.py") {
		t.Fatalf("after run, test_add f = %v; the hand-added file was dropped.\n"+
			"run MUST Union, never Replace (spec §4, D11, audit A4)", after.F)
	}
	if !containsStr(after.F, "src/logic.py") {
		t.Fatalf("after run, test_add f = %v, want it to still contain src/logic.py", after.F)
	}
	if !sort.StringsAreSorted(after.F) {
		t.Errorf("after run, test_add f = %v, want it sorted", after.F)
	}
	// The union merge driver leaves duplicate `t` lines; Load resolves them, and
	// Save must emit exactly one line per test.
	seen := map[string]int{}
	for id := range readMapJSONL(t, repo) {
		seen[id]++
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("map has %d rows for %q after run, want 1", n, id)
		}
	}
}

func TestCmdRunBumpsCyclesOnPassAndOnFail(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	if m, _ := readMeta(repo); m.Cycles != 0 {
		t.Fatalf("cycles after seed = %d, want 0", m.Cycles)
	}

	logic := filepath.Join(repo, "src", "logic.py")
	src, _ := os.ReadFile(logic)
	if err := os.WriteFile(logic, append(src, []byte("\n\ndef added():\n    return 42\n")...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	for i := 1; i <= 2; i++ {
		if code := cmdRun(nil); code != 0 && code != 1 {
			t.Fatalf("cmdRun = %d, want 0 or 1", code)
		}
		m, err := readMeta(repo)
		if err != nil {
			t.Fatalf("readMeta: %v", err)
		}
		if m.Cycles != i {
			t.Fatalf("cycles after %d runs = %d, want %d — a failing streak must still buy an eventual DriftGuard run", i, m.Cycles, i)
		}
	}
}

func TestCmdRunEmptySelectionExitsZeroAndSaysSo(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	// Nothing changed since the seed commit and nothing is untracked apart from
	// .rtdd, so the selection is empty.
	code := cmdRun(nil)
	if code != 0 {
		t.Fatalf("cmdRun on an empty selection = %d, want 0 — an empty selection is a signal, not a failure", code)
	}
	m, err := readMeta(repo)
	if err != nil {
		t.Fatalf("readMeta: %v", err)
	}
	if m.Cycles != 1 {
		t.Errorf("cycles after an empty-selection run = %d, want 1", m.Cycles)
	}
}

func TestCmdRunFailingTestExitsOne(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	// Change the file test_fail covers, so test_fail is selected.
	logic := filepath.Join(repo, "src", "logic.py")
	src, _ := os.ReadFile(logic)
	if err := os.WriteFile(logic, append(src, []byte("\n\ndef added():\n    return 42\n")...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if code := cmdRun(nil); code != 1 {
		t.Fatalf("cmdRun with a failing selected test = %d, want 1", code)
	}
}

func TestCmdRunBadBaseIsAUsageError(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdSeed(nil); code != 1 {
		t.Fatalf("cmdSeed = %d, want 1", code)
	}
	if code := cmdRun([]string{"--base", "no-such-ref-anywhere"}); code == 0 || code == 1 {
		t.Fatalf("cmdRun with an unresolvable --base = %d, want a nonzero non-test-failure code", code)
	}
}

func TestCmdRunRejectsUnknownFlag(t *testing.T) {
	repo := realRepo(t)
	chdir(t, repo)
	if code := cmdRun([]string{"--nonsense"}); code != 2 {
		t.Fatalf("cmdRun with an unknown flag = %d, want 2", code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./cmd/rtdd/ -run TestCmdRun
```

Expected: `cmd/rtdd/run_test.go:52:10: undefined: cmdRun` — `[build failed]`.

- [ ] **Step 3: Write minimal implementation**

`cmd/rtdd/run.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/runner"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// cmdRun selects, executes, refreshes the map, and reports.
//
// It exits nonzero ONLY when a test failed. An empty selection is exit 0 and is
// reported explicitly, so it can never read as "all passed" (spec §5).
//
// `f` is UNIONED, never replaced (spec §4, decision D11, audit A4): a subset run
// legitimately records less coverage than the seed, because import-time and
// first-caller-wins lines migrate to whichever test ran first in that subset, and
// a failing test records a truncated prefix of its real path. Only rtdd seed may
// shrink a row.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	base := fs.String("base", "HEAD", "diff base ref for the changed set")
	failFast := fs.Bool("fail-fast", false, "stop at the first failure (opt-in only)")
	asJSON := fs.Bool("json", false, "machine-readable output (schema lands in M2)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "rtdd: run takes no positional arguments, got %v\n", fs.Args())
		return 2
	}
	_ = asJSON // M2 defines the schema; the flag is accepted now so front-ends can bind to it.

	root, err := findRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	ad, err := detectAdapter(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}

	m, err := mapstore.Load(mapPath(root))
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	mt, err := readMeta(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	changes, err := gitctx.ChangedSet(root, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 2
	}
	all, err := runner.List(ad, root)
	if err != nil {
		return reportRunErr(err)
	}

	sel := selector.Select(selector.Inputs{
		Map:      m,
		Changes:  changes,
		Adapter:  ad,
		Cfg:      selector.DefaultConfig(),
		AllTests: all,
		Distance: func(sha string) int {
			d, err := gitctx.CommitDistance(root, sha)
			if err != nil {
				return -1 // unknown, never "fresh"
			}
			return d
		},
		Cycles:     mt.Cycles,
		ImportOnly: func(string) []string { return nil }, // static import fallback lands in M2
	})

	fmt.Printf("tier %s: %d selected", sel.Tier, len(sel.Tests))
	if sel.Reason != "" {
		fmt.Printf(" (%s)", sel.Reason)
	}
	fmt.Println()

	if sel.Tier == selector.TierEmpty || len(sel.Tests) == 0 {
		fmt.Println("EMPTY SELECTION — nothing ran. This is not a pass.")
		mt.Cycles++
		if err := writeMeta(root, mt); err != nil {
			fmt.Fprintln(os.Stderr, "rtdd:", err)
			return 3
		}
		return 0
	}

	res, err := runner.Run(ad, root, sel.Tests, *failFast)
	if err != nil {
		return reportRunErr(err)
	}

	sha, err := gitctx.HeadSHA(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	older := gitctx.Older(root)
	for _, row := range rowsFrom(res, sha) {
		// UNION, NEVER REPLACE. See the doc comment on cmdRun.
		m.Union(row, older)
	}
	if err := os.MkdirAll(rtddDir(root), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}
	if err := m.Save(mapPath(root)); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	mt.Cycles++ // every completed run, pass or fail
	if mt.Adapter == "" {
		mt.Adapter = ad.Name
	}
	if mt.V == 0 {
		mt.V = 1
	}
	if err := writeMeta(root, mt); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd:", err)
		return 3
	}

	fmt.Printf("%d ran, %d failed, %d rows in the map\n", len(res.Outcomes), len(res.Failed), m.Len())
	for _, id := range res.Failed {
		fmt.Printf("FAILED %s\n", id)
	}
	return res.ExitCode
}
```

In `cmd/rtdd/main.go`, add this case to the command switch:

```go
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/claude/rtdd && go build ./... && go test ./cmd/rtdd/ -v
```

Expected: every `--- PASS` including `TestCmdRunUnionsAndNeverNarrows`,
`TestCmdRunBumpsCyclesOnPassAndOnFail`, `TestCmdRunEmptySelectionExitsZeroAndSaysSo`,
`TestCmdRunFailingTestExitsOne`, and `TestOnlySeedCallsMapstoreReplace`, then
`ok  	github.com/VocanicZ/rtdd/cmd/rtdd`.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/
git commit -m "cmd/rtdd: run — select, execute, union the map, report

f is unioned and never replaced (spec D11, audit A4). An empty selection exits 0
and says so explicitly. cycles increments on pass and on fail."
```

---

## Definition of Done

**Behaviour**

- [ ] `rtdd seed` in a real pytest repo writes `.rtdd/map.jsonl` with one row per collected test, `f` sorted and deduped, `c` the short HEAD SHA, and `s`/`d` taken from the test report.
- [ ] `rtdd run` selects, executes, unions the map, prints the selection tier and the failures, and exits 1 only when a test failed.
- [ ] `rtdd run` on an empty selection prints an explicit EMPTY SELECTION line and exits **0**, distinguishably from "all passed".
- [ ] `rtdd seed` and `rtdd run` both exit **3** when `no-sysmon-context` is observed, and **2** when a chunk exits 4 or 5.
- [ ] `.rtdd/meta.json` carries `v`, `adapter`, `seeded_at`, `cycles`; `cycles` increments on every completed run, pass or fail, and resets to 0 on seed.

**The measured failures this milestone had to avoid**

- [ ] `adapters/python.yaml` sets `COVERAGE_CORE: ctrace`, asserted by a test — not left to review.
- [ ] The runner scans **combined stdout+stderr** for `no-sysmon-context`, with the real captured warning text as the test vector, and checks it **before** the exit code. A stderr-only scan is a failing test.
- [ ] `s` and `d` come from `--report-log`, and the parser handles both measured no-call-phase shapes (skip at setup, fixture error at setup) plus a teardown failure after a passing call.
- [ ] `NormalizeContext` splits on the **last** `|` and only for a known phase; the round-trip test covers ids containing a space, a hyphen, and a pipe.
- [ ] Every coverage path goes through `paths.Normalize`; absolute paths, `relative_files = True` paths, and out-of-repo paths are all covered by test.
- [ ] The adapter passes **bare `--cov`** for both seed and subset, so both defer to the host repo's own `[run] source`/`omit`. A test forbids `--cov=`.
- [ ] `Chunk` splits 8,000 realistic ids into multiple invocations, preserves order, drops nothing, and `Run` merges the per-chunk report and coverage. A chunk exiting 4 or 5 is a `*FatalExitError`.
- [ ] `mapstore.Replace` is called from exactly one file — `cmd/rtdd/seed.go` — enforced by `TestOnlySeedCallsMapstoreReplace`. `run` unions, and an integration test proves a hand-widened row survives a subset run.
- [ ] `ReadSQLite` reads the `arc` table when `meta.has_arcs = 1`, because `[run] branch = True` leaves `line_bits` empty.

**Contract**

- [ ] `docs/plans/00-interfaces.md` records every addition made here: `adapter.LoadFS`, `adapter.Builtin`, `adapter.ExpandTests`, `coverage.Result.Merge`, `runner.FatalExitError`, `runner.List`, the `internal/pytestfixture` package, and the clarified `report.ReadReportLog` phase rules.
- [ ] No name in the implementation diverges from `00-interfaces.md`.

**Hygiene**

- [ ] `go build ./...` and `go vet ./...` are clean.
- [ ] `go test ./...` passes with pytest and git installed; no integration test SKIPs in that environment.
- [ ] `go.mod` lists exactly `gopkg.in/yaml.v3` and `modernc.org/sqlite` as direct dependencies.
- [ ] `go test ./... -race` passes.
- [ ] Every task's commit is separate and its test was seen to fail before its implementation was written.
