# RTDD M2 — Uncovered-Change Signal, Doctor, Init

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the line-granular three-way uncovered-change signal (Covered / Uncovered / ImportTime) computed from fresh post-run coverage, plus the static-import fallback that makes import-time-only files selectable, plus `doctor`, `explain`, `init`, and the `--json` contract the agent front-ends bind to.

**Architecture:** `internal/uncovered` owns hunk parsing and line classification — it intersects the changed line ranges from `git diff --unified=0` with the *fresh* `coverage.Result` produced by the run that just finished, never with `map.jsonl` (line data is never persisted there). `internal/importscan` shells out to an embedded Python AST scanner to answer the one question dynamic coverage provably cannot — which test modules transitively import a file that only ever executes at import time — and feeds `selector.Inputs.ImportOnly`. `internal/doctor` and `internal/initrepo` are thin, pure, and fully testable; `cmd/rtdd` remains the only package that prints.

**Tech Stack:** Go 1.24+, stdlib testing

## Global Constraints

Copied verbatim from [`00-interfaces.md`](00-interfaces.md):

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

---

## Verified fixtures (measured on this machine, 2026-08-26)

Everything below is **real observed output**, not illustrative. Python 3.13.5,
coverage.py 7.15.4, pytest 9.0.3, git 2.47.3. These are the fixtures the tasks use.

### F1 — Import-time attribution (spec §6, audit A1) — REPRODUCED

Project:

```
src/constants.py
  1  from dataclasses import dataclass
  2  from enum import Enum
  3
  4  MAX_RETRIES = 3
  5
  6
  7  class Colour(Enum):
  8      RED = "red"
  9      GREEN = "green"
 10
 11
 12  @dataclass
 13  class Limits:
 14      soft: int = 10
 15      hard: int = 20

src/logic.py
  1  from src.constants import MAX_RETRIES
  2
  3
  4  def retries_left(used):
  5      return MAX_RETRIES - used
  6
  7
  8  def unused_helper(x):
  9      return x * 2

tests/test_it.py
  1  from src.constants import MAX_RETRIES, Colour, Limits
  2  from src.logic import retries_left
  3
  4
  5  def test_constants():
  6      assert MAX_RETRIES == 3
  7      assert Colour.RED.value == "red"
  8      assert Limits().soft == 10
  9
 10
 11  def test_logic():
 12      assert retries_left(1) == 2
```

Run: `COVERAGE_CORE=ctrace python3 -m pytest --cov=src --cov-context=test -q` → `2 passed`.

Querying `.coverage` with the spec's own SQL, decoding `numbits`:

```
src/__init__.py      ctx=''                                       lines=[0]                                raw_numbits=01
src/constants.py     ctx=''                                       lines=[1, 2, 4, 7, 8, 9, 12, 13, 14, 15] raw_numbits=96f3
src/logic.py         ctx=''                                       lines=[1, 4, 8]                          raw_numbits=1201
src/logic.py         ctx='tests/test_it.py::test_logic|run'       lines=[5]                                raw_numbits=20
```

Grouped by file, contexts only:

```
src/__init__.py   -> ['<EMPTY>']
src/constants.py  -> ['<EMPTY>']
src/logic.py      -> ['<EMPTY>', 'tests/test_it.py::test_logic|run']
```

**The finding, confirmed.** `src/constants.py` — a module constant, an `Enum`, and a
`@dataclass`, all three imported and asserted on by two *passing* tests — is attributed to
the **empty context and to zero test contexts**. This is audit finding A1 and the entire
reason `ImportTime` exists as a class. Under a rule that treats "no test covers these
changed lines" as Uncovered, this correctly-tested file reports as broken.

**And the function body in the same file IS attributed.** `src/logic.py` line 5
(`return MAX_RETRIES - used`) belongs to `tests/test_it.py::test_logic|run`. Lines 1, 4 and
8 — the import statement and the two `def` statements — execute at import and land in the
empty context. Line 9 (`return x * 2`, the body of the never-called `unused_helper`) is in
**neither** set: genuinely uncovered.

`src/logic.py` is therefore a single file exhibiting all three classes at once, and is used
verbatim as the classification fixture:

| line | in empty ctx | in a test ctx | class |
|---|---|---|---|
| 1 | yes | no | ImportTime |
| 4 | yes | no | ImportTime |
| 5 | no | yes | Covered |
| 8 | yes | no | ImportTime |
| 9 | no | no | **Uncovered** |

### F2 — `git diff --unified=0` hunk headers — all observed forms

```
@@ -3 +3 @@ l2                              1-line modification; BOTH counts omitted
@@ -8,0 +9,2 @@ l8                          2-line insertion after old line 8 -> new 9..10
@@ -11,2 +12,0 @@ l10                       pure deletion; new count 0 -> contributes NO new lines
@@ -3,0 +4 @@ l3                            1-line insertion; new count omitted -> 1
@@ -0,0 +1,3 @@                             whole new file, 3 lines; no context suffix at all
@@ -5 +5 @@ def foo():  # @@ marker @@      the context suffix may itself contain "@@"
```

Parsing rule, derived from those six: strip the leading `"@@ "`, cut at the **first**
subsequent `" @@"` (never the last — the final form above proves the suffix can contain
`@@`), split the remainder on spaces, take the field beginning with `+`, and read
`+start[,count]`. A missing count means **1**. A count of **0** means the hunk adds no new
lines and contributes no range.

Full diff bodies for the file-header cases:

```
diff --git a/added.txt b/added.txt
new file mode 100644
index 0000000..de98044
--- /dev/null
+++ b/added.txt
@@ -0,0 +1,3 @@
+a
+b
+c
```

```
diff --git a/f.txt b/g.txt
similarity index 94%
rename from f.txt
rename to g.txt
index 8afd661..cfeb442 100644
--- a/f.txt
+++ b/g.txt
@@ -2 +2 @@ l1
-l2
+B
```

An **empty** new file produces a `diff --git` stanza with **no `@@` header at all**:

```
diff --git a/empty.txt b/empty.txt
new file mode 100644
index 0000000..e69de29
```

And `git status --porcelain -uall` for the same tree:

```
R  f.txt -> g.txt
?? untracked.txt
```

### F3 — Static import scan over a cycle — verified

Fixture tree (`src/a.py` and `src/b.py` import each other):

```
src/constants.py     MAX = 3
src/a.py             from src.constants import MAX
                     import src.b
src/b.py             import src.a
tests/test_direct.py from src.constants import MAX
tests/test_trans.py  from src import a
tests/test_unrelated.py  (imports nothing)
```

Observed scanner output:

```
$ echo '{"root":"...","targets":["src/constants.py"],"tests":["tests/test_direct.py","tests/test_trans.py","tests/test_unrelated.py"]}' | python3 scan.py
{"src/constants.py": ["tests/test_direct.py", "tests/test_trans.py"]}

$ echo '{"root":"...","targets":["src/b.py","src/nonexistent.py"],"tests":[...]}' | python3 scan.py
{"src/b.py": ["tests/test_trans.py"], "src/nonexistent.py": []}
exit=0
```

Direct import found, transitive import through `src/a.py` found, the `a ↔ b` cycle
terminates (exit 0, no hang), an unknown target yields an empty list, and the unrelated
test is excluded.

---

## File Structure

New files this milestone creates:

```
internal/uncovered/
  hunk.go              ParseHunks, WithLines
  hunk_test.go
  classify.go          Class, ClassifiedRange, FileReport, Classify, UncoveredLines, Summary, Summarize
  classify_test.go
internal/importscan/
  scan.py              embedded Python AST import scanner
  scan.go              Scan, Scanner, TestsImporting
  scan_test.go
internal/doctor/
  doctor.go            Hub, Hubs, Caveat
  doctor_test.go
internal/initrepo/
  assets/agents-block.md   managed front-end block (embedded)
  initrepo.go          Action, MergeManagedBlock, EnsureGitAttributes, EnsureConfig, EnsureFrontEnd, Run
  initrepo_test.go
cmd/rtdd/
  jsonout.go           Output schema types + BuildOutput
  jsonout_test.go
  exit.go              ExitCodeFor
  exit_test.go
  uncoveredtext.go     RenderUncovered
  uncoveredtext_test.go
  doctor.go            RenderDoctor + cmdDoctor
  doctor_test.go
  explain.go           RenderExplain + cmdExplain
  explain_test.go
  init.go              RenderInit + cmdInit
  init_test.go
  acceptance_test.go   the four M2 guarantees
```

Files this milestone edits:

```
internal/gitctx/git.go        + RawDiff
cmd/rtdd/main.go              + dispatch entries for doctor / explain / init
cmd/rtdd/run.go               + post-run classification wiring
cmd/rtdd/which.go             + unmapped-files reporting
docs/plans/00-interfaces.md   contract additions (one edit step per task, same commit)
```

---
