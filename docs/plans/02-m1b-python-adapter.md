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
