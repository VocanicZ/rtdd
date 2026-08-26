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
