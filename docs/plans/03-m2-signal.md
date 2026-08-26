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

## Task 1 — Hunk parsing (`uncovered.ParseHunks`)

**Files:** `internal/uncovered/hunk.go`, `internal/uncovered/hunk_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes* (M1a, unchanged):
```go
type LineRange struct{ Start, End int } // 1-indexed, inclusive, in the NEW file
```

*Produces* (contract addition, added to `00-interfaces.md` in this same commit):
```go
// ParseHunks parses `git diff --unified=0` output and returns, per NEW-file path,
// the line ranges that exist in the new file. Hunks whose new-side count is 0
// (pure deletions) contribute nothing. Files whose new side is /dev/null are omitted.
func ParseHunks(diff string) map[string][]gitctx.LineRange
```

### Steps

- [ ] **1.1 — Write the failing test.** Create `internal/uncovered/hunk_test.go`:

```go
package uncovered

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

func TestParseHunks(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want map[string][]gitctx.LineRange
	}{
		{
			name: "single line modification, both counts omitted",
			diff: "diff --git a/f.txt b/f.txt\n" +
				"index 8afd661..858e521 100644\n" +
				"--- a/f.txt\n" +
				"+++ b/f.txt\n" +
				"@@ -3 +3 @@ l2\n" +
				"-l3\n" +
				"+L3CHANGED\n",
			want: map[string][]gitctx.LineRange{"f.txt": {{Start: 3, End: 3}}},
		},
		{
			name: "multi line insertion",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -8,0 +9,2 @@ l8\n+NEW8a\n+NEW8b\n",
			want: map[string][]gitctx.LineRange{"f.txt": {{Start: 9, End: 10}}},
		},
		{
			name: "single line insertion, new count omitted",
			diff: "--- a/f.py\n+++ b/f.py\n@@ -3,0 +4 @@ l3\n+INSERTED\n",
			want: map[string][]gitctx.LineRange{"f.py": {{Start: 4, End: 4}}},
		},
		{
			name: "pure deletion contributes no new lines",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -11,2 +12,0 @@ l10\n-l11\n-l12\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "whole new file",
			diff: "diff --git a/added.txt b/added.txt\n" +
				"new file mode 100644\n" +
				"index 0000000..de98044\n" +
				"--- /dev/null\n" +
				"+++ b/added.txt\n" +
				"@@ -0,0 +1,3 @@\n+a\n+b\n+c\n",
			want: map[string][]gitctx.LineRange{"added.txt": {{Start: 1, End: 3}}},
		},
		{
			name: "empty new file has no hunk header",
			diff: "diff --git a/empty.txt b/empty.txt\n" +
				"new file mode 100644\n" +
				"index 0000000..e69de29\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "context suffix may itself contain @@",
			diff: "--- a/h.py\n+++ b/h.py\n" +
				"@@ -5 +5 @@ def foo():  # @@ marker @@\n" +
				"-    z = 3\n+    z = 99\n",
			want: map[string][]gitctx.LineRange{"h.py": {{Start: 5, End: 5}}},
		},
		{
			name: "rename uses the new path",
			diff: "diff --git a/f.txt b/g.txt\n" +
				"similarity index 94%\nrename from f.txt\nrename to g.txt\n" +
				"index 8afd661..cfeb442 100644\n" +
				"--- a/f.txt\n+++ b/g.txt\n@@ -2 +2 @@ l1\n-l2\n+B\n",
			want: map[string][]gitctx.LineRange{"g.txt": {{Start: 2, End: 2}}},
		},
		{
			name: "deleted file has /dev/null on the new side",
			diff: "--- a/gone.txt\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n",
			want: map[string][]gitctx.LineRange{},
		},
		{
			name: "three files in one diff",
			diff: "--- a/f.txt\n+++ b/f.txt\n@@ -3 +3 @@ l2\n-l3\n+L3CHANGED\n" +
				"@@ -8,0 +9,2 @@ l8\n+NEW8a\n+NEW8b\n" +
				"--- a/g.py\n+++ b/g.py\n@@ -1,0 +2 @@ x\n+y\n" +
				"--- a/h.py\n+++ /dev/null\n@@ -1 +0,0 @@\n-z\n",
			want: map[string][]gitctx.LineRange{
				"f.txt": {{Start: 3, End: 3}, {Start: 9, End: 10}},
				"g.py":  {{Start: 2, End: 2}},
			},
		},
		{
			name: "empty diff",
			diff: "",
			want: map[string][]gitctx.LineRange{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseHunks(tc.diff)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseHunks()\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func TestParseHunksQuotedPath(t *testing.T) {
	diff := "--- \"a/we ird\\ttab.py\"\n+++ \"b/we ird\\ttab.py\"\n@@ -1 +1 @@ ctx\n-a\n+b\n"
	got := ParseHunks(diff)
	want := map[string][]gitctx.LineRange{"we ird\ttab.py": {{Start: 1, End: 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseHunks()\n got: %#v\nwant: %#v", got, want)
	}
}
```

- [ ] **1.2 — Run it and see it fail.**

```bash
go test ./internal/uncovered/
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/uncovered [github.com/VocanicZ/rtdd/internal/uncovered.test]
internal/uncovered/hunk_test.go:57:11: undefined: ParseHunks
FAIL	github.com/VocanicZ/rtdd/internal/uncovered [build failed]
```

- [ ] **1.3 — Minimal implementation.** Create `internal/uncovered/hunk.go`:

```go
// Package uncovered computes the three-way classification of changed lines
// (Covered / Uncovered / ImportTime) against fresh post-run coverage.
package uncovered

import (
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// ParseHunks parses `git diff --unified=0` output and returns, per NEW-file path,
// the line ranges that exist in the new file. Hunks whose new-side count is 0
// (pure deletions) contribute nothing. Files whose new side is /dev/null are omitted.
func ParseHunks(diff string) map[string][]gitctx.LineRange {
	out := map[string][]gitctx.LineRange{}
	cur := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ ") {
			cur = newSidePath(strings.TrimSpace(line[4:]))
			continue
		}
		if cur == "" || !strings.HasPrefix(line, "@@ ") {
			continue
		}
		r, ok := parseHunkHeader(line)
		if !ok {
			continue
		}
		out[cur] = append(out[cur], r)
	}
	return out
}

// newSidePath turns the `+++` operand into a repo-relative path, or "" for /dev/null.
func newSidePath(p string) string {
	if strings.HasPrefix(p, "\"") {
		if uq, err := strconv.Unquote(p); err == nil {
			p = uq
		}
	}
	if p == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(p, "b/") {
		p = p[2:]
	}
	return p
}

// parseHunkHeader reads the new-side range from "@@ -a,b +c,d @@ optional context".
// The context suffix may itself contain "@@", so the range region is cut at the FIRST
// following " @@", never the last. A missing count means 1; a count of 0 means the hunk
// adds no new lines and ok is false.
func parseHunkHeader(line string) (gitctx.LineRange, bool) {
	body := line[3:] // strip "@@ "
	if i := strings.Index(body, " @@"); i >= 0 {
		body = body[:i]
	}
	for _, f := range strings.Fields(body) {
		if !strings.HasPrefix(f, "+") {
			continue
		}
		spec := f[1:]
		startStr, countStr := spec, "1"
		if i := strings.IndexByte(spec, ','); i >= 0 {
			startStr, countStr = spec[:i], spec[i+1:]
		}
		start, err := strconv.Atoi(startStr)
		if err != nil {
			return gitctx.LineRange{}, false
		}
		count, err := strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			return gitctx.LineRange{}, false
		}
		return gitctx.LineRange{Start: start, End: start + count - 1}, true
	}
	return gitctx.LineRange{}, false
}
```

- [ ] **1.4 — Run it and see it pass.**

```bash
go test ./internal/uncovered/
```

Expected:

```
ok  	github.com/VocanicZ/rtdd/internal/uncovered	0.004s
```

- [ ] **1.5 — Add the contract entry.** In `docs/plans/00-interfaces.md`, under
`## internal/uncovered`, insert this block immediately **above** the existing
`type Class int` declaration:

```go
// ParseHunks parses `git diff --unified=0` output and returns, per NEW-file path,
// the line ranges that exist in the new file. Hunks whose new-side count is 0
// (pure deletions) contribute nothing. Files whose new side is /dev/null are omitted.
// The hunk header's context suffix may itself contain "@@"; the range region is cut
// at the FIRST following " @@". A missing count means 1.
func ParseHunks(diff string) map[string][]gitctx.LineRange
```

- [ ] **1.6 — Commit.**

```bash
git add internal/uncovered/hunk.go internal/uncovered/hunk_test.go docs/plans/00-interfaces.md
git commit -m "uncovered: parse git diff --unified=0 hunk headers

All six observed header forms are covered, including the single-line form with
the count omitted, the count-0 pure deletion, and a context suffix containing @@."
```

---

## Task 2 — Changed line ranges (`gitctx.RawDiff`, `uncovered.WithLines`)

**Files:** `internal/gitctx/git.go`, `internal/uncovered/hunk.go`, `internal/uncovered/hunk_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes* (M1a, unchanged):
```go
type Status int
const (
    Added Status = iota
    Modified
    Deleted
    Renamed
    Untracked
)
type Change struct {
    Path     string
    OldPath  string
    Status   Status
    Lines    []LineRange
}
func ChangedSet(repoRoot, base string) ([]Change, error)
```

*Produces* (contract additions, added to `00-interfaces.md` in this same commit):
```go
// internal/gitctx
// RawDiff returns the full text of `git diff --unified=0 <base>` for the working tree.
func RawDiff(repoRoot, base string) (string, error)

// internal/uncovered
// WithLines returns changes with Lines populated from rawDiff. It is authoritative:
// it OVERWRITES any Lines already present, so there is exactly one source of truth.
// Deleted changes get nil Lines. A change absent from rawDiff (an untracked file git
// diff never lists) gets the whole file as one range, counted from disk; a missing or
// empty file gets nil Lines.
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error)
```

### Steps

- [ ] **2.1 — Write the failing test.** Append to `internal/uncovered/hunk_test.go`:

```go
func TestWithLines(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "untracked.py", "a = 1\nb = 2\nc = 3\n")
	writeFile(t, root, "noeol.py", "x = 1\ny = 2")
	writeFile(t, root, "empty.py", "")
	writeFile(t, root, "mod.py", "l1\nl2\nl3\nl4\n")

	diff := "--- a/mod.py\n+++ b/mod.py\n@@ -3 +3 @@ l2\n-old\n+new\n" +
		"--- a/stale.py\n+++ b/stale.py\n@@ -0,0 +1,2 @@\n+p\n+q\n"

	in := []gitctx.Change{
		{Path: "mod.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 99, End: 99}}},
		{Path: "stale.py", Status: gitctx.Added},
		{Path: "untracked.py", Status: gitctx.Untracked},
		{Path: "noeol.py", Status: gitctx.Untracked},
		{Path: "empty.py", Status: gitctx.Untracked},
		{Path: "gone.py", Status: gitctx.Deleted, Lines: []gitctx.LineRange{{Start: 1, End: 5}}},
		{Path: "vanished.py", Status: gitctx.Untracked},
	}

	got, err := WithLines(root, in, diff)
	if err != nil {
		t.Fatalf("WithLines() error = %v", err)
	}

	want := []gitctx.Change{
		{Path: "mod.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 3, End: 3}}},
		{Path: "stale.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 2}}},
		{Path: "untracked.py", Status: gitctx.Untracked, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
		{Path: "noeol.py", Status: gitctx.Untracked, Lines: []gitctx.LineRange{{Start: 1, End: 2}}},
		{Path: "empty.py", Status: gitctx.Untracked, Lines: nil},
		{Path: "gone.py", Status: gitctx.Deleted, Lines: nil},
		{Path: "vanished.py", Status: gitctx.Untracked, Lines: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithLines()\n got: %#v\nwant: %#v", got, want)
	}
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

and extend the import block at the top of `internal/uncovered/hunk_test.go` to:

```go
import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)
```

- [ ] **2.2 — Run it and see it fail.**

```bash
go test ./internal/uncovered/ -run TestWithLines
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/uncovered [github.com/VocanicZ/rtdd/internal/uncovered.test]
internal/uncovered/hunk_test.go:...: undefined: WithLines
FAIL	github.com/VocanicZ/rtdd/internal/uncovered [build failed]
```

- [ ] **2.3 — Minimal implementation.** Append to `internal/uncovered/hunk.go`:

```go
// WithLines returns changes with Lines populated from rawDiff. It is authoritative:
// it OVERWRITES any Lines already present, so there is exactly one source of truth.
// Deleted changes get nil Lines. A change absent from rawDiff (an untracked file git
// diff never lists) gets the whole file as one range, counted from disk; a missing or
// empty file gets nil Lines.
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error) {
	hunks := ParseHunks(rawDiff)
	out := make([]gitctx.Change, 0, len(changes))
	for _, c := range changes {
		c.Lines = nil
		if c.Status == gitctx.Deleted {
			out = append(out, c)
			continue
		}
		if rs, ok := hunks[c.Path]; ok {
			c.Lines = rs
			out = append(out, c)
			continue
		}
		n, err := countLines(repoRoot, c.Path)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			c.Lines = []gitctx.LineRange{{Start: 1, End: n}}
		}
		out = append(out, c)
	}
	return out, nil
}

// countLines returns the number of lines in repoRoot/rel. A missing file is 0, not an
// error: a path can be listed as changed and then removed before the report runs.
func countLines(repoRoot, rel string) (int, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if len(b) == 0 {
		return 0, nil
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n, nil
}
```

and extend the import block at the top of `internal/uncovered/hunk.go` to:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)
```

- [ ] **2.4 — Run it and see it pass.**

```bash
go test ./internal/uncovered/
```

Expected:

```
ok  	github.com/VocanicZ/rtdd/internal/uncovered	0.006s
```

- [ ] **2.5 — Add `RawDiff` to gitctx.** Append to `internal/gitctx/git.go`:

```go
// RawDiff returns the full text of `git diff --unified=0 <base>` for the working tree.
// Renames are detected so the new-side path is authoritative.
func RawDiff(repoRoot, base string) (string, error) {
	cmd := exec.Command("git", "diff", "--unified=0", "-M", "--no-color", base)
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff --unified=0 %s: %w: %s", base, err, stderr.String())
	}
	return stdout.String(), nil
}
```

If `internal/gitctx/git.go` does not already import `bytes`, `fmt` and `os/exec`, add them
to its import block.

- [ ] **2.6 — Write the RawDiff test.** Create `internal/gitctx/rawdiff_test.go`:

```go
package gitctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRawDiff(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q", ".")
	run("config", "user.email", "a@b.c")
	run("config", "user.name", "t")

	p := filepath.Join(root, "f.txt")
	if err := os.WriteFile(p, []byte("l1\nl2\nl3\nl4\nl5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "init")

	if err := os.WriteFile(p, []byte("l1\nl2\nCHANGED\nl4\nl5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := RawDiff(root, "HEAD")
	if err != nil {
		t.Fatalf("RawDiff() error = %v", err)
	}
	if !strings.Contains(got, "@@ -3 +3 @@") {
		t.Fatalf("RawDiff() missing single-line hunk header, got:\n%s", got)
	}
	if !strings.Contains(got, "+++ b/f.txt") {
		t.Fatalf("RawDiff() missing new-side path, got:\n%s", got)
	}
}
```

- [ ] **2.7 — Run the gitctx test and see it pass.**

```bash
go test ./internal/gitctx/ -run TestRawDiff -v
```

Expected:

```
=== RUN   TestRawDiff
--- PASS: TestRawDiff (0.04s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/gitctx	0.045s
```

- [ ] **2.8 — Add the contract entries.** In `docs/plans/00-interfaces.md`, under
`## internal/gitctx`, insert after the `ChangedSet` declaration:

```go
// RawDiff returns the full text of `git diff --unified=0 -M <base>` for the working tree.
func RawDiff(repoRoot, base string) (string, error)
```

and under `## internal/uncovered`, insert after the `ParseHunks` declaration:

```go
// WithLines returns changes with Lines populated from rawDiff. It is authoritative:
// it OVERWRITES any Lines already present, so there is exactly one source of truth.
// Deleted changes get nil Lines. A change absent from rawDiff (an untracked file git
// diff never lists) gets the whole file as one range, counted from disk; a missing or
// empty file gets nil Lines.
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error)
```

- [ ] **2.9 — Commit.**

```bash
git add internal/gitctx/git.go internal/gitctx/rawdiff_test.go internal/uncovered/hunk.go internal/uncovered/hunk_test.go docs/plans/00-interfaces.md
git commit -m "gitctx,uncovered: populate changed line ranges from --unified=0

WithLines is authoritative and overwrites Lines, so hunk parsing is the single
source of truth. Untracked files, which git diff never lists, fall back to the
whole file counted from disk."
```

---

## Task 3 — Three-way line classification (`uncovered.Classify`)

This is the centrepiece. Fixture F1 is used verbatim.

**Files:** `internal/uncovered/classify.go`, `internal/uncovered/classify_test.go`

**Interfaces:**

*Consumes* (M1b, unchanged):
```go
type TestCoverage struct {
    Test  string
    Files map[string][]int // repo-relative path -> sorted covered line numbers
}
type Result struct {
    PerTest    []TestCoverage
    ImportTime map[string][]int // empty-context lines: executed, attributed to no test
}
```

*Produces* (already in `00-interfaces.md`, implemented here):
```go
type Class int
const (
    Covered Class = iota
    Uncovered
    ImportTime
)

type ClassifiedRange struct {
    Range gitctx.LineRange
    Class Class
}

type FileReport struct {
    Path   string
    Ranges []ClassifiedRange
}

func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport
func (r FileReport) UncoveredLines() int
```

*Produces* (contract addition, `Class.String`, added in this same commit):
```go
func (c Class) String() string // "covered" | "uncovered" | "import-time"
```

**Rules, and why:**

1. A line covered by **any** entry in `cov.PerTest` is `Covered`. Test attribution wins over
   import-time attribution, because a line can appear in both (a `def` executed at import
   and re-entered during a test would be in both sets; the test signal is the stronger one).
2. A line present **only** in `cov.ImportTime` is `ImportTime` and **MUST NOT** be reported
   as `Uncovered`. Spec §6, audit A1.
3. Everything else is `Uncovered`. Line-granular, never file-granular. Audit A2.
4. `Deleted` changes and changes with no `Lines` produce no `FileReport`.
5. Classification consumes only the fresh `*coverage.Result` handed in by the caller. It
   never reads `map.jsonl`, which carries no line data at all.
6. **Caller responsibility:** only instrumentable changes may be passed. A test file or a
   `.yaml` is not in `cov` and would classify wholly `Uncovered`. `cmd/rtdd` filters with
   `adapter.IsInstrumentable` before calling. Task 10 tests that filter.

Adjacent lines of the same class are coalesced into one `ClassifiedRange`; the output is
sorted by `Path`, and each file's ranges are sorted ascending by `Range.Start`.

### Steps

- [ ] **3.1 — Write the failing test.** Create `internal/uncovered/classify_test.go`:

```go
package uncovered

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// f1Coverage is fixture F1, measured on 2026-08-26 with coverage.py 7.15.4 /
// pytest 9.0.3 / Python 3.13.5 on a project whose two tests both pass:
//
//	src/__init__.py   ctx=''                                  lines=[0]
//	src/constants.py  ctx=''                                  lines=[1,2,4,7,8,9,12,13,14,15]
//	src/logic.py      ctx=''                                  lines=[1,4,8]
//	src/logic.py      ctx='tests/test_it.py::test_logic|run'  lines=[5]
//
// src/constants.py holds a module constant, an Enum and a @dataclass, is imported and
// asserted on by BOTH passing tests, and is attributed to ZERO test contexts.
func f1Coverage() *coverage.Result {
	return &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{
				Test:  "tests/test_it.py::test_logic",
				Files: map[string][]int{"src/logic.py": {5}},
			},
			{
				Test:  "tests/test_it.py::test_constants",
				Files: map[string][]int{},
			},
		},
		ImportTime: map[string][]int{
			"src/__init__.py":  {0},
			"src/constants.py": {1, 2, 4, 7, 8, 9, 12, 13, 14, 15},
			"src/logic.py":     {1, 4, 8},
		},
	}
}

func TestClassifyImportTimeOnlyFileIsClean(t *testing.T) {
	// The whole point of ImportTime: a dataclass/enum/constants module changed
	// end to end, asserted on by two passing tests, must produce ZERO uncovered lines.
	changes := []gitctx.Change{
		{
			Path:   "src/constants.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 1, End: 15}},
		},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 1 {
		t.Fatalf("Classify() len = %d, want 1", len(got))
	}
	if n := got[0].UncoveredLines(); n != 0 {
		t.Fatalf("UncoveredLines() = %d, want 0 — import-time lines must NEVER be Uncovered\n got: %#v", n, got[0].Ranges)
	}
	for _, r := range got[0].Ranges {
		if r.Class == Uncovered {
			t.Fatalf("range %d-%d classified Uncovered; import-time lines must never be", r.Range.Start, r.Range.End)
		}
	}
}

func TestClassifyThreeWayInOneFile(t *testing.T) {
	// src/logic.py exhibits all three classes at once:
	//   1 import-time, 4 import-time, 5 covered, 8 import-time, 9 uncovered.
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 1, End: 9}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 1}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 2, End: 3}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 4, End: 4}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 6, End: 7}, Class: Uncovered},
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 9, End: 9}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyIsLineGranularNotFileGranular(t *testing.T) {
	// Audit A2: adding a function to an ALREADY-COVERED file must report the new
	// function's lines as Uncovered. v1 was file-granular and reported green here.
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines:  []gitctx.LineRange{{Start: 8, End: 9}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 8, End: 8}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 9, End: 9}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
	if n := got[0].UncoveredLines(); n != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1", n)
	}
}

func TestClassifyBrandNewFileIsAllUncovered(t *testing.T) {
	changes := []gitctx.Change{
		{
			Path:   "src/brand_new.py",
			Status: gitctx.Added,
			Lines:  []gitctx.LineRange{{Start: 1, End: 4}},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/brand_new.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 1, End: 4}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifyTestAttributionBeatsImportTime(t *testing.T) {
	cov := &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_x.py::test_a", Files: map[string][]int{"src/dual.py": {4}}},
		},
		ImportTime: map[string][]int{"src/dual.py": {4}},
	}
	changes := []gitctx.Change{
		{Path: "src/dual.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 4, End: 4}}},
	}
	got := Classify(changes, cov)
	want := []FileReport{
		{
			Path:   "src/dual.py",
			Ranges: []ClassifiedRange{{Range: gitctx.LineRange{Start: 4, End: 4}, Class: Covered}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifySkipsDeletedAndEmpty(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/gone.py", Status: gitctx.Deleted},
		{Path: "src/empty.py", Status: gitctx.Added, Lines: nil},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 0 {
		t.Fatalf("Classify() = %#v, want empty", got)
	}
}

func TestClassifyOverlappingRangesAreDeduped(t *testing.T) {
	changes := []gitctx.Change{
		{
			Path:   "src/logic.py",
			Status: gitctx.Modified,
			Lines: []gitctx.LineRange{
				{Start: 4, End: 5},
				{Start: 5, End: 5},
				{Start: 5, End: 6},
			},
		},
	}
	got := Classify(changes, f1Coverage())
	want := []FileReport{
		{
			Path: "src/logic.py",
			Ranges: []ClassifiedRange{
				{Range: gitctx.LineRange{Start: 4, End: 4}, Class: ImportTime},
				{Range: gitctx.LineRange{Start: 5, End: 5}, Class: Covered},
				{Range: gitctx.LineRange{Start: 6, End: 6}, Class: Uncovered},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Classify()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestClassifySortsByPath(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/z.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 1}}},
		{Path: "src/a.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 1}}},
	}
	got := Classify(changes, f1Coverage())
	if len(got) != 2 || got[0].Path != "src/a.py" || got[1].Path != "src/z.py" {
		t.Fatalf("Classify() paths not sorted: %#v", got)
	}
}

func TestClassString(t *testing.T) {
	tests := []struct {
		c    Class
		want string
	}{
		{Covered, "covered"},
		{Uncovered, "uncovered"},
		{ImportTime, "import-time"},
		{Class(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.c.String(); got != tc.want {
			t.Fatalf("Class(%d).String() = %q, want %q", int(tc.c), got, tc.want)
		}
	}
}
```

- [ ] **3.2 — Run it and see it fail.**

```bash
go test ./internal/uncovered/ -run TestClassify
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/uncovered [github.com/VocanicZ/rtdd/internal/uncovered.test]
internal/uncovered/classify_test.go:...: undefined: Classify
internal/uncovered/classify_test.go:...: undefined: FileReport
internal/uncovered/classify_test.go:...: undefined: ClassifiedRange
internal/uncovered/classify_test.go:...: undefined: ImportTime
FAIL	github.com/VocanicZ/rtdd/internal/uncovered [build failed]
```

- [ ] **3.3 — Minimal implementation.** Create `internal/uncovered/classify.go`:

```go
package uncovered

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// Class is the three-way classification of a changed line.
type Class int

const (
	// Covered — an executing test touched this line.
	Covered Class = iota
	// Uncovered — no test executed it. The real signal.
	Uncovered
	// ImportTime — executed during collection/import, attributed to no test.
	// NEVER counted as Uncovered. See spec §6 and audit finding A1.
	ImportTime
)

func (c Class) String() string {
	switch c {
	case Covered:
		return "covered"
	case Uncovered:
		return "uncovered"
	case ImportTime:
		return "import-time"
	default:
		return "unknown"
	}
}

// ClassifiedRange is a maximal run of consecutive changed lines sharing one Class.
type ClassifiedRange struct {
	Range gitctx.LineRange
	Class Class
}

// FileReport is the classification of one changed file's changed lines.
type FileReport struct {
	Path   string
	Ranges []ClassifiedRange
}

// UncoveredLines counts the changed lines classified Uncovered.
func (r FileReport) UncoveredLines() int {
	n := 0
	for _, cr := range r.Ranges {
		if cr.Class == Uncovered {
			n += cr.Range.End - cr.Range.Start + 1
		}
	}
	return n
}

// Summary aggregates a set of FileReports for the --json output and the text report.
type Summary struct {
	Files           int
	CoveredLines    int
	UncoveredLines  int
	ImportTimeLines int
}

// Summarize totals the reports.
func Summarize(reports []FileReport) Summary {
	s := Summary{Files: len(reports)}
	for _, r := range reports {
		for _, cr := range r.Ranges {
			n := cr.Range.End - cr.Range.Start + 1
			switch cr.Class {
			case Covered:
				s.CoveredLines += n
			case Uncovered:
				s.UncoveredLines += n
			case ImportTime:
				s.ImportTimeLines += n
			}
		}
	}
	return s
}

// Classify intersects each Change's line ranges with FRESH post-run coverage.
//
// A line covered by any test is Covered. A line present only in Result.ImportTime is
// ImportTime and MUST NOT be reported as Uncovered. Everything else is Uncovered.
//
// cov must be the coverage produced by the run that just finished. Line data is never
// persisted in map.jsonl, so there is no line-drift problem: both sides are current.
//
// Callers must pass only instrumentable changes; a file absent from cov entirely
// classifies wholly Uncovered, which is correct for a new source file and wrong for a
// test file or an opaque asset.
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport {
	covered := map[string]map[int]bool{}
	importTime := map[string]map[int]bool{}

	if cov != nil {
		for _, tc := range cov.PerTest {
			for path, lines := range tc.Files {
				set := covered[path]
				if set == nil {
					set = map[int]bool{}
					covered[path] = set
				}
				for _, ln := range lines {
					set[ln] = true
				}
			}
		}
		for path, lines := range cov.ImportTime {
			set := importTime[path]
			if set == nil {
				set = map[int]bool{}
				importTime[path] = set
			}
			for _, ln := range lines {
				set[ln] = true
			}
		}
	}

	var out []FileReport
	for _, ch := range changes {
		if ch.Status == gitctx.Deleted || len(ch.Lines) == 0 {
			continue
		}
		lines := expandLines(ch.Lines)
		if len(lines) == 0 {
			continue
		}
		cSet, iSet := covered[ch.Path], importTime[ch.Path]
		ranges := coalesce(lines, func(ln int) Class {
			switch {
			case cSet[ln]:
				return Covered
			case iSet[ln]:
				return ImportTime
			default:
				return Uncovered
			}
		})
		out = append(out, FileReport{Path: ch.Path, Ranges: ranges})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// expandLines flattens ranges into a sorted, deduplicated line list.
func expandLines(rs []gitctx.LineRange) []int {
	seen := map[int]bool{}
	var out []int
	for _, r := range rs {
		for ln := r.Start; ln <= r.End; ln++ {
			if ln < 1 || seen[ln] {
				continue
			}
			seen[ln] = true
			out = append(out, ln)
		}
	}
	sort.Ints(out)
	return out
}

// coalesce merges consecutive lines of the same Class into maximal ranges.
func coalesce(lines []int, classOf func(int) Class) []ClassifiedRange {
	var out []ClassifiedRange
	for _, ln := range lines {
		c := classOf(ln)
		if n := len(out); n > 0 && out[n-1].Class == c && out[n-1].Range.End == ln-1 {
			out[n-1].Range.End = ln
			continue
		}
		out = append(out, ClassifiedRange{Range: gitctx.LineRange{Start: ln, End: ln}, Class: c})
	}
	return out
}
```

- [ ] **3.4 — Run it and see it pass.**

```bash
go test ./internal/uncovered/ -v -run TestClassify
```

Expected:

```
=== RUN   TestClassifyImportTimeOnlyFileIsClean
--- PASS: TestClassifyImportTimeOnlyFileIsClean (0.00s)
=== RUN   TestClassifyThreeWayInOneFile
--- PASS: TestClassifyThreeWayInOneFile (0.00s)
=== RUN   TestClassifyIsLineGranularNotFileGranular
--- PASS: TestClassifyIsLineGranularNotFileGranular (0.00s)
=== RUN   TestClassifyBrandNewFileIsAllUncovered
--- PASS: TestClassifyBrandNewFileIsAllUncovered (0.00s)
=== RUN   TestClassifyTestAttributionBeatsImportTime
--- PASS: TestClassifyTestAttributionBeatsImportTime (0.00s)
=== RUN   TestClassifySkipsDeletedAndEmpty
--- PASS: TestClassifySkipsDeletedAndEmpty (0.00s)
=== RUN   TestClassifyOverlappingRangesAreDeduped
--- PASS: TestClassifyOverlappingRangesAreDeduped (0.00s)
=== RUN   TestClassifySortsByPath
--- PASS: TestClassifySortsByPath (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/uncovered	0.005s
```

- [ ] **3.5 — Add the contract entries.** In `docs/plans/00-interfaces.md`, under
`## internal/uncovered`, add after the `Class` const block:

```go
func (c Class) String() string // "covered" | "uncovered" | "import-time"
```

and after the existing `func (r FileReport) UncoveredLines() int`:

```go
type Summary struct {
    Files           int
    CoveredLines    int
    UncoveredLines  int
    ImportTimeLines int
}

// Summarize totals a set of FileReports.
func Summarize(reports []FileReport) Summary
```

- [ ] **3.6 — Commit.**

```bash
git add internal/uncovered/classify.go internal/uncovered/classify_test.go docs/plans/00-interfaces.md
git commit -m "uncovered: three-way line classification against fresh coverage

Covered / Uncovered / ImportTime, line-granular. Import-time lines are never
reported as Uncovered (audit A1); adding a function to an already-covered file
reports the new body as Uncovered (audit A2). Fixtures are real coverage.py
7.15.4 output measured on 2026-08-26."
```

---

## Task 4 — Static import scan (`internal/importscan`, embedded Python)

Spec §6 / D14: a file appearing **only** in the empty context cannot be reached through the
coverage relation, so selection falls back to a Python AST import scan. The engine is Go and
must not parse Python itself, so the scan is a small embedded Python script shelled out to.

**Files:** `internal/importscan/scan.py`, `internal/importscan/scan.go`, `internal/importscan/scan_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing from other RTDD packages.

*Produces* (contract addition — new package, added to `00-interfaces.md` in this same commit):
```go
// Scan returns, for each target, the test files whose module transitively imports it.
// Import cycles terminate via a visited set. A target no module resolves to maps to an
// empty slice, never a missing key.
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)
```

### Steps

- [ ] **4.1 — Write the failing test.** Create `internal/importscan/scan_test.go`:

```go
package importscan

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("no python interpreter on PATH")
		}
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cycleFixture is fixture F3, verified on 2026-08-26: src/a.py and src/b.py import
// each other, so any traversal without a visited set hangs.
func cycleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "src/__init__.py", "")
	write(t, root, "tests/__init__.py", "")
	write(t, root, "src/constants.py", "MAX = 3\n")
	write(t, root, "src/a.py", "from src.constants import MAX\nimport src.b\n")
	write(t, root, "src/b.py", "import src.a\n")
	write(t, root, "tests/test_direct.py", "from src.constants import MAX\ndef test_d(): assert MAX == 3\n")
	write(t, root, "tests/test_trans.py", "from src import a\ndef test_t(): assert a.MAX == 3\n")
	write(t, root, "tests/test_unrelated.py", "def test_u(): assert True\n")
	return root
}

func allTests() []string {
	return []string{"tests/test_direct.py", "tests/test_trans.py", "tests/test_unrelated.py"}
}

func TestScanDirectAndTransitive(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	got, err := Scan(root, []string{"src/constants.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/constants.py": {"tests/test_direct.py", "tests/test_trans.py"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanTerminatesOnImportCycle(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	// src/b.py is reachable only by walking into the a <-> b cycle.
	got, err := Scan(root, []string{"src/b.py", "src/nonexistent.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/b.py":           {"tests/test_trans.py"},
		"src/nonexistent.py": {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanRelativeImports(t *testing.T) {
	requirePython(t)
	root := t.TempDir()
	write(t, root, "pkg/__init__.py", "")
	write(t, root, "pkg/models.py", "NAME = 'x'\n")
	write(t, root, "pkg/service.py", "from .models import NAME\n")
	write(t, root, "tests/__init__.py", "")
	write(t, root, "tests/test_svc.py", "from pkg.service import NAME\ndef test_s(): assert NAME == 'x'\n")
	got, err := Scan(root, []string{"pkg/models.py"}, []string{"tests/test_svc.py"})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{"pkg/models.py": {"tests/test_svc.py"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanSyntaxErrorIsNotFatal(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	write(t, root, "src/broken.py", "def (((\n")
	got, err := Scan(root, []string{"src/constants.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/constants.py": {"tests/test_direct.py", "tests/test_trans.py"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanNoTargetsIsEmpty(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	got, err := Scan(root, nil, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Scan() = %#v, want empty", got)
	}
}
```

- [ ] **4.2 — Run it and see it fail.**

```bash
go test ./internal/importscan/
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/importscan [github.com/VocanicZ/rtdd/internal/importscan.test]
internal/importscan/scan_test.go:...: undefined: Scan
FAIL	github.com/VocanicZ/rtdd/internal/importscan [build failed]
```

- [ ] **4.3 — Write the Python scanner.** Create `internal/importscan/scan.py`. This is the
exact script whose output is fixture F3:

```python
"""RTDD static import scan.

Reads a JSON request on stdin:
    {"root": "<abs repo root>", "targets": ["rel/path.py", ...], "tests": ["rel/test.py", ...]}
Writes a JSON object on stdout:
    {"<target>": ["<test rel path>", ...], ...}   sorted, one key per target.

A test is selected for a target when the test module transitively imports the target
module. Import cycles terminate via a visited set. Unparseable files contribute no edges
rather than aborting the scan.
"""
import ast
import json
import os
import sys

SKIP_DIRS = {".git", ".venv", "venv", "__pycache__", ".tox", "node_modules", ".rtdd",
             ".mypy_cache", ".pytest_cache", "build", "dist", ".eggs"}


def mod_name(rel):
    parts = rel[:-3].split("/")
    if parts[-1] == "__init__":
        parts = parts[:-1]
    return ".".join(parts)


def py_files(root):
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            if fn.endswith(".py"):
                rel = os.path.relpath(os.path.join(dirpath, fn), root)
                out.append(rel.replace(os.sep, "/"))
    return sorted(out)


def imports_of(root, rel, self_mod):
    path = os.path.join(root, rel)
    try:
        with open(path, "rb") as fh:
            tree = ast.parse(fh.read(), filename=rel)
    except (SyntaxError, OSError, ValueError):
        return set()
    if rel == "__init__.py" or rel.endswith("/__init__.py"):
        pkg = self_mod
    else:
        pkg = self_mod.rsplit(".", 1)[0] if "." in self_mod else ""
    out = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for al in node.names:
                out.add(al.name)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                base = pkg.split(".") if pkg else []
                if node.level > 1:
                    base = base[: len(base) - (node.level - 1)]
                prefix = ".".join([p for p in base if p])
                mod = prefix + ("." + node.module if node.module else "")
            else:
                mod = node.module or ""
            if mod:
                out.add(mod)
            for al in node.names:
                out.add((mod + "." + al.name) if mod else al.name)
    return out


def main():
    req = json.load(sys.stdin)
    root = req["root"]
    targets = req.get("targets") or []
    tests = req.get("tests") or []

    files = py_files(root)
    mod2rel = {}
    for rel in files:
        mod2rel.setdefault(mod_name(rel), rel)

    edges = {}
    for rel in files:
        raw = imports_of(root, rel, mod_name(rel))
        edges[rel] = sorted({mod2rel[x] for x in raw if x in mod2rel})

    result = {}
    for tgt in targets:
        hits = []
        for t in tests:
            if t not in edges:
                continue
            seen = {t}
            stack = [t]
            found = False
            while stack:
                cur = stack.pop()
                if cur == tgt:
                    found = True
                    break
                for nxt in edges.get(cur, ()):
                    if nxt not in seen:
                        seen.add(nxt)
                        stack.append(nxt)
            if found:
                hits.append(t)
        result[tgt] = sorted(hits)

    json.dump(result, sys.stdout, sort_keys=True)
    sys.stdout.write("\n")


main()
```

- [ ] **4.4 — Write the Go wrapper.** Create `internal/importscan/scan.go`:

```go
// Package importscan is RTDD's single use of static analysis (spec §6, D14).
//
// A file that appears only in coverage's empty context cannot be reached through the
// coverage relation, so selection falls back to a Python AST import scan that selects
// tests whose module transitively imports it. The engine is Go and must not parse Python
// itself, so the scan runs as an embedded Python script.
package importscan

import (
	_ "embed"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed scan.py
var scanScript []byte

type request struct {
	Root    string   `json:"root"`
	Targets []string `json:"targets"`
	Tests   []string `json:"tests"`
}

// Scan returns, for each target, the test files whose module transitively imports it.
// Import cycles terminate via a visited set. A target no module resolves to maps to an
// empty slice, never a missing key.
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error) {
	if len(targets) == 0 {
		return map[string][]string{}, nil
	}
	bin, err := pythonBin()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rtdd-importscan-")
	if err != nil {
		return nil, fmt.Errorf("importscan: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	script := filepath.Join(dir, "scan.py")
	if err := os.WriteFile(script, scanScript, 0o600); err != nil {
		return nil, fmt.Errorf("importscan: write script: %w", err)
	}

	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("importscan: abs %q: %w", repoRoot, err)
	}
	payload, err := json.Marshal(request{Root: abs, Targets: targets, Tests: tests})
	if err != nil {
		return nil, fmt.Errorf("importscan: marshal request: %w", err)
	}

	cmd := exec.Command(bin, script)
	cmd.Dir = abs
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("importscan: %s scan.py: %w: %s", bin, err, strings.TrimSpace(stderr.String()))
	}

	out := map[string][]string{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("importscan: decode result: %w", err)
	}
	for _, t := range targets {
		if out[t] == nil {
			out[t] = []string{}
		}
	}
	return out, nil
}

func pythonBin() (string, error) {
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("importscan: no python3 or python on PATH")
}
```

- [ ] **4.5 — Run it and see it pass.**

```bash
go test ./internal/importscan/ -v
```

Expected:

```
=== RUN   TestScanDirectAndTransitive
--- PASS: TestScanDirectAndTransitive (0.06s)
=== RUN   TestScanTerminatesOnImportCycle
--- PASS: TestScanTerminatesOnImportCycle (0.05s)
=== RUN   TestScanRelativeImports
--- PASS: TestScanRelativeImports (0.05s)
=== RUN   TestScanSyntaxErrorIsNotFatal
--- PASS: TestScanSyntaxErrorIsNotFatal (0.05s)
=== RUN   TestScanNoTargetsIsEmpty
--- PASS: TestScanNoTargetsIsEmpty (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/importscan	0.216s
```

- [ ] **4.6 — Add the contract entry.** In `docs/plans/00-interfaces.md`, add
`internal/importscan/  static import fallback for import-time-only files` to the
**Package layout** block (immediately after the `internal/selector/` line), and add a new
section immediately **before** `## internal/uncovered`:

````markdown
## internal/importscan

RTDD's single use of static analysis (spec §6, D14). Shells out to an embedded Python AST
script; the Go engine never parses Python itself.

```go
// Scan returns, for each target, the test files whose module transitively imports it.
// Import cycles terminate via a visited set. A target no module resolves to maps to an
// empty slice, never a missing key.
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)
```
````

- [ ] **4.7 — Commit.**

```bash
git add internal/importscan docs/plans/00-interfaces.md
git commit -m "importscan: Python AST import fallback for import-time-only files

Embedded scan.py resolves module names to repo-relative paths, walks import
edges transitively, and terminates on cycles via a visited set. Verified against
a src/a.py <-> src/b.py cycle."
```

---

## Task 5 — Wire the fallback into selection (`importscan.Scanner`, `selector.Inputs.ImportOnly`)

`selector.Inputs.ImportOnly func(rel string) []string` was declared in M1a and left nil.
This task supplies it. The trigger is exactly the condition spec §6 names: a changed
instrumentable file that **no map row covers**. Because import-time lines are attributed to
no test, they never enter any row's `f`, so "zero tests cover this file in the map" is
precisely the import-time-only case (or a genuinely untested file, where selecting importers
is still the best available guess).

**Files:** `internal/importscan/scan.go`, `internal/importscan/scan_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:*
```go
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)
type Inputs struct { /* ... */ ImportOnly func(rel string) []string }
```

*Produces* (contract addition, added in this same commit):
```go
// Scanner memoises Scan across repeated lookups within one command invocation.
// It is the value passed as selector.Inputs.ImportOnly.
type Scanner struct { /* unexported */ }

func NewScanner(repoRoot string, tests []string) *Scanner

// TestsImporting returns the test files whose module transitively imports rel.
// On scanner error it returns nil; the error is retained and reported by Err.
// A failed scan degrades selection, it never fails the command.
func (s *Scanner) TestsImporting(rel string) []string

// Err returns the first error any TestsImporting call encountered, or nil.
func (s *Scanner) Err() error
```

### Steps

- [ ] **5.1 — Write the failing test.** Append to `internal/importscan/scan_test.go`:

```go
func TestScannerMemoises(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	s := NewScanner(root, allTests())

	first := s.TestsImporting("src/constants.py")
	want := []string{"tests/test_direct.py", "tests/test_trans.py"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("TestsImporting()\n got: %#v\nwant: %#v", first, want)
	}
	second := s.TestsImporting("src/constants.py")
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("memoised TestsImporting()\n got: %#v\nwant: %#v", second, want)
	}
	if got := s.TestsImporting("src/nonexistent.py"); len(got) != 0 {
		t.Fatalf("TestsImporting(nonexistent) = %#v, want empty", got)
	}
	if err := s.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestScannerErrorDegradesRatherThanPanics(t *testing.T) {
	s := NewScanner(string([]byte{0x00}), allTests())
	got := s.TestsImporting("src/constants.py")
	if got != nil {
		t.Fatalf("TestsImporting() = %#v, want nil on scanner failure", got)
	}
	if s.Err() == nil {
		t.Fatal("Err() = nil, want the retained scan error")
	}
}

func TestScannerSatisfiesSelectorImportOnly(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	s := NewScanner(root, allTests())
	var importOnly func(rel string) []string = s.TestsImporting
	if got := importOnly("src/constants.py"); len(got) != 2 {
		t.Fatalf("importOnly() = %#v, want 2 tests", got)
	}
}
```

- [ ] **5.2 — Run it and see it fail.**

```bash
go test ./internal/importscan/ -run TestScanner
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/importscan [github.com/VocanicZ/rtdd/internal/importscan.test]
internal/importscan/scan_test.go:...: undefined: NewScanner
FAIL	github.com/VocanicZ/rtdd/internal/importscan [build failed]
```

- [ ] **5.3 — Minimal implementation.** Append to `internal/importscan/scan.go`:

```go
// Scanner memoises Scan across repeated lookups within one command invocation.
// It is the value passed as selector.Inputs.ImportOnly.
type Scanner struct {
	repoRoot string
	tests    []string
	cache    map[string][]string
	err      error
}

// NewScanner returns a Scanner over the given repo and candidate test files.
func NewScanner(repoRoot string, tests []string) *Scanner {
	return &Scanner{repoRoot: repoRoot, tests: tests, cache: map[string][]string{}}
}

// TestsImporting returns the test files whose module transitively imports rel.
// On scanner error it returns nil; the error is retained and reported by Err.
// A failed scan degrades selection, it never fails the command.
func (s *Scanner) TestsImporting(rel string) []string {
	if v, ok := s.cache[rel]; ok {
		return v
	}
	res, err := Scan(s.repoRoot, []string{rel}, s.tests)
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		s.cache[rel] = nil
		return nil
	}
	v := res[rel]
	if v == nil {
		v = []string{}
	}
	s.cache[rel] = v
	return v
}

// Err returns the first error any TestsImporting call encountered, or nil.
func (s *Scanner) Err() error { return s.err }
```

- [ ] **5.4 — Run it and see it pass.**

```bash
go test ./internal/importscan/ -v -run TestScanner
```

Expected:

```
=== RUN   TestScannerMemoises
--- PASS: TestScannerMemoises (0.06s)
=== RUN   TestScannerErrorDegradesRatherThanPanics
--- PASS: TestScannerErrorDegradesRatherThanPanics (0.05s)
=== RUN   TestScannerSatisfiesSelectorImportOnly
--- PASS: TestScannerSatisfiesSelectorImportOnly (0.05s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/importscan	0.171s
```

- [ ] **5.5 — Add the contract entry.** In `docs/plans/00-interfaces.md`, in the
`## internal/importscan` section added by Task 4, append inside the same code fence:

```go
// Scanner memoises Scan across repeated lookups within one command invocation.
// It is the value passed as selector.Inputs.ImportOnly.
type Scanner struct { /* unexported */ }

func NewScanner(repoRoot string, tests []string) *Scanner

// TestsImporting returns the test files whose module transitively imports rel.
// On scanner error it returns nil; the error is retained and reported by Err.
// A failed scan degrades selection, it never fails the command.
func (s *Scanner) TestsImporting(rel string) []string

// Err returns the first error any TestsImporting call encountered, or nil.
func (s *Scanner) Err() error
```

and, under `## internal/selector`, replace the `ImportOnly` field comment so it reads:

```go
    ImportOnly func(rel string) []string // importscan.Scanner.TestsImporting; static-import fallback
```

- [ ] **5.6 — Commit.**

```bash
git add internal/importscan docs/plans/00-interfaces.md
git commit -m "importscan: memoising Scanner satisfying selector.Inputs.ImportOnly

A failed scan degrades selection to whatever the coverage relation offers; it
never fails the command."
```

---

## Task 6 — `internal/doctor`: fan-out ranking and the mandatory caveat

Spec §9. The caveat must appear **in the tool's own output**, not only in documentation:
anything executed once per process gets a fan-out of 1, so the most-coupled file in the repo
can appear as its cleanest.

**Files:** `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes* (M1a, unchanged):
```go
func (m *Map) FanOut() map[string]int
func (m *Map) Len() int
func (m *Map) Union(r Row, older func(a, b string) string)
```

*Produces* (already in `00-interfaces.md`, implemented here):
```go
type Hub struct {
    Path      string
    TestCount int
    Fraction  float64 // TestCount / total tests in the map
}
func Hubs(m *mapstore.Map) []Hub
```

*Produces* (contract addition, added in this same commit):
```go
// Caveat is the limitation rtdd doctor MUST print alongside its table (spec §9).
const Caveat = "..."
```

### Steps

- [ ] **6.1 — Write the failing test.** Create `internal/doctor/doctor_test.go`:

```go
package doctor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func older(a, b string) string { return a }

func fixtureMap() *mapstore.Map {
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_a.py::t1", F: []string{"src/hub.py", "src/a.py"}, C: "aaa", D: 10, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_b.py::t2", F: []string{"src/hub.py", "src/b.py"}, C: "aaa", D: 20, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_c.py::t3", F: []string{"src/hub.py"}, C: "aaa", D: 30, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_d.py::t4", F: []string{"src/b.py"}, C: "aaa", D: 40, S: "pass"}, older)
	return m
}

func TestHubsRanksByDescendingTestCount(t *testing.T) {
	got := Hubs(fixtureMap())
	want := []Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
		{Path: "src/a.py", TestCount: 1, Fraction: 0.25},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Hubs()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestHubsTiesBreakByPathForDeterminism(t *testing.T) {
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/t.py::t1", F: []string{"src/z.py", "src/a.py", "src/m.py"}, C: "aaa", D: 1, S: "pass"}, older)
	got := Hubs(m)
	want := []Hub{
		{Path: "src/a.py", TestCount: 1, Fraction: 1},
		{Path: "src/m.py", TestCount: 1, Fraction: 1},
		{Path: "src/z.py", TestCount: 1, Fraction: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Hubs()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestHubsEmptyMap(t *testing.T) {
	got := Hubs(mapstore.New())
	if len(got) != 0 {
		t.Fatalf("Hubs() = %#v, want empty", got)
	}
}

func TestCaveatNamesEveryOncePerProcessMechanism(t *testing.T) {
	for _, needle := range []string{
		"lru_cache",
		"module singleton",
		"DI container",
		"session-scoped fixture",
		"fan-out of 1",
		"most coupled",
		"cleanest",
	} {
		if !strings.Contains(Caveat, needle) {
			t.Fatalf("Caveat is missing %q; spec §9 requires the limitation be stated in the tool's own output.\nCaveat = %q", needle, Caveat)
		}
	}
}
```

- [ ] **6.2 — Run it and see it fail.**

```bash
go test ./internal/doctor/
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/doctor [github.com/VocanicZ/rtdd/internal/doctor.test]
internal/doctor/doctor_test.go:...: undefined: Hubs
internal/doctor/doctor_test.go:...: undefined: Hub
internal/doctor/doctor_test.go:...: undefined: Caveat
FAIL	github.com/VocanicZ/rtdd/internal/doctor [build failed]
```

- [ ] **6.3 — Minimal implementation.** Create `internal/doctor/doctor.go`:

```go
// Package doctor ranks source files by fan-out — how many tests cover them — as a
// coupling diagnostic. Fan-out is never used as an automatic escalation trigger; see
// spec §9 and Caveat.
package doctor

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Caveat is the limitation rtdd doctor MUST print alongside its table (spec §9).
// Anything executed once per process is attributed to whichever test happened to run
// first, so the repo's most coupled file can be reported as its cleanest.
const Caveat = "CAVEAT: anything executed once per process — @lru_cache results, " +
	"module singletons, DI container wiring, session-scoped fixtures — runs during " +
	"whichever test happened to go first and therefore gets a fan-out of 1. The most " +
	"coupled file in the repo can appear here as the cleanest. Fan-out is a diagnostic " +
	"only; RTDD never escalates selection on it."

// Hub is one file's fan-out.
type Hub struct {
	Path      string
	TestCount int
	Fraction  float64 // TestCount / total tests in the map
}

// Hubs returns files sorted by descending TestCount, then ascending Path so the output
// is deterministic.
func Hubs(m *mapstore.Map) []Hub {
	fan := m.FanOut()
	total := m.Len()
	out := make([]Hub, 0, len(fan))
	for path, n := range fan {
		frac := 0.0
		if total > 0 {
			frac = float64(n) / float64(total)
		}
		out = append(out, Hub{Path: path, TestCount: n, Fraction: frac})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TestCount != out[j].TestCount {
			return out[i].TestCount > out[j].TestCount
		}
		return out[i].Path < out[j].Path
	})
	return out
}
```

- [ ] **6.4 — Run it and see it pass.**

```bash
go test ./internal/doctor/ -v
```

Expected:

```
=== RUN   TestHubsRanksByDescendingTestCount
--- PASS: TestHubsRanksByDescendingTestCount (0.00s)
=== RUN   TestHubsTiesBreakByPathForDeterminism
--- PASS: TestHubsTiesBreakByPathForDeterminism (0.00s)
=== RUN   TestHubsEmptyMap
--- PASS: TestHubsEmptyMap (0.00s)
=== RUN   TestCaveatNamesEveryOncePerProcessMechanism
--- PASS: TestCaveatNamesEveryOncePerProcessMechanism (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/doctor	0.004s
```

- [ ] **6.5 — Add the contract entry.** In `docs/plans/00-interfaces.md`, under
`## internal/doctor`, insert above the `type Hub struct` declaration:

```go
// Caveat is the limitation rtdd doctor MUST print alongside its table (spec §9).
const Caveat = "CAVEAT: anything executed once per process — @lru_cache results, " +
	"module singletons, DI container wiring, session-scoped fixtures — runs during " +
	"whichever test happened to go first and therefore gets a fan-out of 1. The most " +
	"coupled file in the repo can appear here as the cleanest. Fan-out is a diagnostic " +
	"only; RTDD never escalates selection on it."
```

and append below `func Hubs`:

```go
// Hubs sorts by descending TestCount, then ascending Path for determinism.
```

- [ ] **6.6 — Commit.**

```bash
git add internal/doctor docs/plans/00-interfaces.md
git commit -m "doctor: fan-out ranking with the once-per-process caveat

The caveat is a package constant so it cannot be dropped from the CLI output;
a test asserts it names lru_cache, module singletons, DI containers and
session-scoped fixtures."
```

---

## Task 7 — The `--json` output schema

This is the contract the agent front-ends bind to, so it is defined in full here and copied
into `00-interfaces.md`.

**Files:** `cmd/rtdd/jsonout.go`, `cmd/rtdd/jsonout_test.go`, `docs/plans/00-interfaces.md`

### The schema, version 1

```json
{
  "schema": 1,
  "command": "run",
  "base": "HEAD",
  "adapter": "python",
  "tier": "T0",
  "reason": "3 map rows intersect the changed set",
  "changed": [
    {"path": "src/logic.py", "status": "modified", "instrumentable": true,
     "lines": [{"start": 8, "end": 9}]},
    {"path": "src/constants.py", "status": "modified", "instrumentable": true,
     "lines": [{"start": 1, "end": 15}]}
  ],
  "selection": {
    "count": 2,
    "direct": ["tests/test_new.py"],
    "tests": ["tests/test_new.py", "tests/test_it.py::test_logic"],
    "import_fallback": {"src/constants.py": ["tests/test_it.py"]}
  },
  "run": {
    "executed": true,
    "passed": 2, "failed": 0, "skipped": 0, "errored": 0,
    "failures": [],
    "duration_ms": 1400
  },
  "uncovered": {
    "available": true,
    "files": [
      {"path": "src/constants.py",
       "ranges": [{"start": 1, "end": 15, "class": "import-time"}],
       "uncovered_lines": 0},
      {"path": "src/logic.py",
       "ranges": [{"start": 8, "end": 8, "class": "import-time"},
                  {"start": 9, "end": 9, "class": "uncovered"}],
       "uncovered_lines": 1}
    ],
    "summary": {"files": 2, "covered_lines": 0, "uncovered_lines": 1,
                "import_time_lines": 16}
  },
  "unmapped_files": ["src/constants.py"],
  "exit_code": 0
}
```

**Field contract.**

| Field | Type | Meaning |
|---|---|---|
| `schema` | int | Always `1` for this version. Consumers MUST reject an unknown value rather than guess. |
| `command` | string | `"run"` or `"which"`. |
| `base` | string | The `--base` ref actually used. |
| `adapter` | string | Detected adapter name. |
| `tier` | string | `"empty"`, `"direct"`, `"T0"`, `"T1"`, `"T2"` — `selector.Tier.String()`. |
| `reason` | string | Human-readable escalation cause; `""` when none. |
| `changed[].path` | string | Repo-relative, slash-separated. |
| `changed[].status` | string | `"added"`, `"modified"`, `"deleted"`, `"renamed"`, `"untracked"`. |
| `changed[].instrumentable` | bool | Whether the adapter would instrument it. Only instrumentable files can appear in `uncovered.files`. |
| `changed[].lines` | array | New-file line ranges, 1-indexed inclusive. Empty for `deleted`. |
| `selection.count` | int | `len(selection.tests)`. |
| `selection.direct` | array | Changed/new test files, always run first. Never null. |
| `selection.tests` | array | Final ranked list, direct first. Never null. Empty is a legitimate outcome and is reported as `tier: "empty"`. |
| `selection.import_fallback` | object | file → tests chosen by the static import scan. `{}` when the fallback did not fire. |
| `run.executed` | bool | `false` for `which`, which runs nothing. |
| `run.passed`/`failed`/`skipped`/`errored` | int | Outcome counts; all `0` when `executed` is `false`. |
| `run.failures` | array of string | Failing test ids. Never null. |
| `run.duration_ms` | int | Sum of executed test durations. |
| `uncovered.available` | bool | `true` only when fresh post-run coverage exists. `which` always emits `false`. |
| `uncovered.reason` | string | Present only when `available` is `false`; explains why. |
| `uncovered.files` | array | Omitted when `available` is `false`. Sorted by `path`. |
| `uncovered.files[].ranges[].class` | string | `"covered"`, `"uncovered"`, `"import-time"`. **`"import-time"` is never `"uncovered"`.** |
| `uncovered.summary` | object | Totals over `files`. |
| `unmapped_files` | array | Changed instrumentable files no map row covers — the import-fallback trigger set. Never null. |
| `exit_code` | int | The process exit code. **`0` even when `uncovered.summary.uncovered_lines > 0`.** |

**Invariant, and it is tested:** `exit_code` is `1` if and only if `run.failed + run.errored
> 0`. A non-empty uncovered report never changes it.

### Steps

- [ ] **7.1 — Write the failing test.** Create `cmd/rtdd/jsonout_test.go`:

```go
package main

import (
	"encoding/json"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func TestBuildOutputRunWithUncoveredReport(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 8, End: 8}, Class: uncovered.ImportTime},
			{Range: gitctx.LineRange{Start: 9, End: 9}, Class: uncovered.Uncovered},
		}},
	}
	in := OutputInput{
		Command: "run",
		Base:    "HEAD",
		Adapter: "python",
		Sel: selector.Selection{
			Tier:   selector.TierT0,
			Tests:  []string{"tests/test_new.py", "tests/test_it.py::test_logic"},
			Direct: []string{"tests/test_new.py"},
			Reason: "3 map rows intersect the changed set",
		},
		Changes:        changes,
		Instrumentable: map[string]bool{"src/constants.py": true, "src/logic.py": true},
		Executed:       true,
		Outcomes: []report.Outcome{
			{Test: "tests/test_new.py::test_x", Status: "pass", DurationMS: 400},
			{Test: "tests/test_it.py::test_logic", Status: "pass", DurationMS: 1000},
		},
		Reports:        reports,
		UncoveredOK:    true,
		UnmappedFiles:  []string{"src/constants.py"},
		ImportFallback: map[string][]string{"src/constants.py": {"tests/test_it.py"}},
	}

	out := BuildOutput(in)

	if out.Schema != 1 {
		t.Fatalf("schema = %d, want 1", out.Schema)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0 — an uncovered report is a signal, never a verdict", out.ExitCode)
	}
	if out.Uncovered.Summary.UncoveredLines != 1 {
		t.Fatalf("uncovered_lines = %d, want 1", out.Uncovered.Summary.UncoveredLines)
	}
	if out.Uncovered.Summary.ImportTimeLines != 16 {
		t.Fatalf("import_time_lines = %d, want 16", out.Uncovered.Summary.ImportTimeLines)
	}
	if out.Run.DurationMS != 1400 {
		t.Fatalf("duration_ms = %d, want 1400", out.Run.DurationMS)
	}
	if out.Tier != "T0" {
		t.Fatalf("tier = %q, want \"T0\"", out.Tier)
	}
	if len(out.Uncovered.Files) != 2 || out.Uncovered.Files[0].Path != "src/constants.py" {
		t.Fatalf("uncovered.files = %#v", out.Uncovered.Files)
	}
	if out.Uncovered.Files[0].Ranges[0].Class != "import-time" {
		t.Fatalf("class = %q, want \"import-time\"", out.Uncovered.Files[0].Ranges[0].Class)
	}
	if out.Uncovered.Files[0].UncoveredLines != 0 {
		t.Fatalf("constants.py uncovered_lines = %d, want 0", out.Uncovered.Files[0].UncoveredLines)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	for _, k := range []string{"schema", "command", "base", "adapter", "tier", "reason",
		"changed", "selection", "run", "uncovered", "unmapped_files", "exit_code"} {
		if _, ok := round[k]; !ok {
			t.Fatalf("marshalled object is missing required key %q: %s", k, b)
		}
	}
}

func TestBuildOutputNeverNullsSlices(t *testing.T) {
	out := BuildOutput(OutputInput{Command: "which", Base: "HEAD", Adapter: "python"})
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	for _, needle := range []string{
		`"changed":[]`,
		`"direct":[]`,
		`"tests":[]`,
		`"import_fallback":{}`,
		`"failures":[]`,
		`"unmapped_files":[]`,
	} {
		if !contains(s, needle) {
			t.Fatalf("output must never emit null for a collection; missing %s in %s", needle, s)
		}
	}
}

func TestBuildOutputWhichHasNoUncoveredReport(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel:           selector.Selection{Tier: selector.TierT0, Tests: []string{"tests/a.py::t"}},
		UnmappedFiles: []string{"src/constants.py"},
	})
	if out.Run.Executed {
		t.Fatal("which must report run.executed = false")
	}
	if out.Uncovered.Available {
		t.Fatal("which must report uncovered.available = false; it runs nothing, so there is no fresh coverage")
	}
	if out.Uncovered.Reason == "" {
		t.Fatal("uncovered.reason must explain why the report is unavailable")
	}
	if out.Uncovered.Files != nil {
		t.Fatalf("uncovered.files must be omitted when unavailable, got %#v", out.Uncovered.Files)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", out.ExitCode)
	}
}

func TestBuildOutputFailingTestExitsOne(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Sel:      selector.Selection{Tier: selector.TierT0, Tests: []string{"tests/a.py::t"}},
		Executed: true,
		Outcomes: []report.Outcome{
			{Test: "tests/a.py::t", Status: "fail", DurationMS: 5},
			{Test: "tests/b.py::t", Status: "error", DurationMS: 3},
			{Test: "tests/c.py::t", Status: "skip", DurationMS: 1},
		},
		UncoveredOK: true,
	})
	if out.ExitCode != 1 {
		t.Fatalf("exit_code = %d, want 1", out.ExitCode)
	}
	if out.Run.Failed != 1 || out.Run.Errored != 1 || out.Run.Skipped != 1 {
		t.Fatalf("counts = %+v", out.Run)
	}
	want := []string{"tests/a.py::t", "tests/b.py::t"}
	if len(out.Run.Failures) != 2 || out.Run.Failures[0] != want[0] || out.Run.Failures[1] != want[1] {
		t.Fatalf("failures = %#v, want %#v", out.Run.Failures, want)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) <= len(haystack) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
```

- [ ] **7.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestBuildOutput
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/jsonout_test.go:...: undefined: OutputInput
cmd/rtdd/jsonout_test.go:...: undefined: BuildOutput
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **7.3 — Minimal implementation.** Create `cmd/rtdd/jsonout.go`:

```go
package main

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/selector"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// SchemaVersion is the --json contract version. Consumers MUST reject an unknown value.
const SchemaVersion = 1

// JSONLineRange is a 1-indexed inclusive range in the NEW file.
type JSONLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// JSONChange is one entry of the changed set.
type JSONChange struct {
	Path           string          `json:"path"`
	Status         string          `json:"status"`
	Instrumentable bool            `json:"instrumentable"`
	Lines          []JSONLineRange `json:"lines"`
}

// JSONSelection is the ranked selection.
type JSONSelection struct {
	Count          int                 `json:"count"`
	Direct         []string            `json:"direct"`
	Tests          []string            `json:"tests"`
	ImportFallback map[string][]string `json:"import_fallback"`
}

// JSONRun is the outcome of the executed subset. All zero when Executed is false.
type JSONRun struct {
	Executed   bool     `json:"executed"`
	Passed     int      `json:"passed"`
	Failed     int      `json:"failed"`
	Skipped    int      `json:"skipped"`
	Errored    int      `json:"errored"`
	Failures   []string `json:"failures"`
	DurationMS int      `json:"duration_ms"`
}

// JSONClassifiedRange is one maximal run of changed lines sharing a class.
type JSONClassifiedRange struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Class string `json:"class"` // "covered" | "uncovered" | "import-time"
}

// JSONFileReport is one file's classification.
type JSONFileReport struct {
	Path           string                `json:"path"`
	Ranges         []JSONClassifiedRange `json:"ranges"`
	UncoveredLines int                   `json:"uncovered_lines"`
}

// JSONSummary totals the file reports.
type JSONSummary struct {
	Files           int `json:"files"`
	CoveredLines    int `json:"covered_lines"`
	UncoveredLines  int `json:"uncovered_lines"`
	ImportTimeLines int `json:"import_time_lines"`
}

// JSONUncovered is the uncovered-change signal. Available is false for `which`,
// which runs nothing and therefore has no fresh coverage.
type JSONUncovered struct {
	Available bool             `json:"available"`
	Reason    string           `json:"reason,omitempty"`
	Files     []JSONFileReport `json:"files,omitempty"`
	Summary   JSONSummary      `json:"summary"`
}

// Output is the top-level --json document.
type Output struct {
	Schema        int            `json:"schema"`
	Command       string         `json:"command"`
	Base          string         `json:"base"`
	Adapter       string         `json:"adapter"`
	Tier          string         `json:"tier"`
	Reason        string         `json:"reason"`
	Changed       []JSONChange   `json:"changed"`
	Selection     JSONSelection  `json:"selection"`
	Run           JSONRun        `json:"run"`
	Uncovered     JSONUncovered  `json:"uncovered"`
	UnmappedFiles []string       `json:"unmapped_files"`
	ExitCode      int            `json:"exit_code"`
}

// OutputInput is everything BuildOutput needs. It is a plain struct so the schema can be
// tested without a repo, a runner, or coverage.
type OutputInput struct {
	Command        string
	Base           string
	Adapter        string
	Sel            selector.Selection
	Changes        []gitctx.Change
	Instrumentable map[string]bool
	Executed       bool
	Outcomes       []report.Outcome
	Reports        []uncovered.FileReport
	UncoveredOK    bool
	UnmappedFiles  []string
	ImportFallback map[string][]string
}

var statusNames = map[gitctx.Status]string{
	gitctx.Added:     "added",
	gitctx.Modified:  "modified",
	gitctx.Deleted:   "deleted",
	gitctx.Renamed:   "renamed",
	gitctx.Untracked: "untracked",
}

// BuildOutput assembles the --json document.
//
// ExitCode is 1 if and only if a test failed or errored. A non-empty uncovered report
// NEVER changes it: RTDD reports, it does not gate (spec §2, §6).
func BuildOutput(in OutputInput) Output {
	out := Output{
		Schema:        SchemaVersion,
		Command:       in.Command,
		Base:          in.Base,
		Adapter:       in.Adapter,
		Tier:          in.Sel.Tier.String(),
		Reason:        in.Sel.Reason,
		Changed:       []JSONChange{},
		UnmappedFiles: []string{},
	}

	for _, c := range in.Changes {
		lines := []JSONLineRange{}
		for _, r := range c.Lines {
			lines = append(lines, JSONLineRange{Start: r.Start, End: r.End})
		}
		out.Changed = append(out.Changed, JSONChange{
			Path:           c.Path,
			Status:         statusNames[c.Status],
			Instrumentable: in.Instrumentable[c.Path],
			Lines:          lines,
		})
	}

	out.Selection = JSONSelection{
		Count:          len(in.Sel.Tests),
		Direct:         nonNil(in.Sel.Direct),
		Tests:          nonNil(in.Sel.Tests),
		ImportFallback: map[string][]string{},
	}
	for k, v := range in.ImportFallback {
		out.Selection.ImportFallback[k] = nonNil(v)
	}

	out.Run = JSONRun{Executed: in.Executed, Failures: []string{}}
	if in.Executed {
		for _, o := range in.Outcomes {
			out.Run.DurationMS += o.DurationMS
			switch o.Status {
			case "pass":
				out.Run.Passed++
			case "fail":
				out.Run.Failed++
				out.Run.Failures = append(out.Run.Failures, o.Test)
			case "error":
				out.Run.Errored++
				out.Run.Failures = append(out.Run.Failures, o.Test)
			case "skip":
				out.Run.Skipped++
			}
		}
		sort.Strings(out.Run.Failures)
	}

	if in.UncoveredOK {
		out.Uncovered.Available = true
		out.Uncovered.Files = []JSONFileReport{}
		for _, r := range in.Reports {
			fr := JSONFileReport{
				Path:           r.Path,
				Ranges:         []JSONClassifiedRange{},
				UncoveredLines: r.UncoveredLines(),
			}
			for _, cr := range r.Ranges {
				fr.Ranges = append(fr.Ranges, JSONClassifiedRange{
					Start: cr.Range.Start,
					End:   cr.Range.End,
					Class: cr.Class.String(),
				})
			}
			out.Uncovered.Files = append(out.Uncovered.Files, fr)
		}
		s := uncovered.Summarize(in.Reports)
		out.Uncovered.Summary = JSONSummary{
			Files:           s.Files,
			CoveredLines:    s.CoveredLines,
			UncoveredLines:  s.UncoveredLines,
			ImportTimeLines: s.ImportTimeLines,
		}
	} else {
		out.Uncovered.Reason = "the uncovered-change signal requires fresh post-run coverage; run `rtdd run`"
	}

	out.UnmappedFiles = nonNil(in.UnmappedFiles)
	sort.Strings(out.UnmappedFiles)

	if out.Run.Failed+out.Run.Errored > 0 {
		out.ExitCode = 1
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
```

- [ ] **7.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run TestBuildOutput
```

Expected:

```
=== RUN   TestBuildOutputRunWithUncoveredReport
--- PASS: TestBuildOutputRunWithUncoveredReport (0.00s)
=== RUN   TestBuildOutputNeverNullsSlices
--- PASS: TestBuildOutputNeverNullsSlices (0.00s)
=== RUN   TestBuildOutputWhichHasNoUncoveredReport
--- PASS: TestBuildOutputWhichHasNoUncoveredReport (0.00s)
=== RUN   TestBuildOutputFailingTestExitsOne
--- PASS: TestBuildOutputFailingTestExitsOne (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.008s
```

- [ ] **7.5 — Add the schema to the contract.** In `docs/plans/00-interfaces.md`, replace
the two closing lines

```
`--json` emits a machine-readable object for agent consumption. Its schema is defined in
plan M2, task "JSON output", and is the interface the agent front-ends depend on.
```

with the full schema section — the example document, the field-contract table, and the
invariant — copied verbatim from **Task 7 of `03-m2-signal.md`** (the "The schema, version 1"
subsection in its entirety, including the JSON example, the field table, and the sentence
beginning "**Invariant, and it is tested:**").

- [ ] **7.6 — Commit.**

```bash
git add cmd/rtdd/jsonout.go cmd/rtdd/jsonout_test.go docs/plans/00-interfaces.md
git commit -m "cmd: --json schema v1, the agent front-end contract

Collections are never null. exit_code is 1 iff a test failed or errored; a
non-empty uncovered report never changes it."
```

---

## Task 8 — Text report and the exit-code rule

Spec §6's worked example is the golden output. The exit-code rule gets its own pure
function so it can be asserted directly, without a subprocess.

**Files:** `cmd/rtdd/uncoveredtext.go`, `cmd/rtdd/uncoveredtext_test.go`, `cmd/rtdd/exit.go`, `cmd/rtdd/exit_test.go`

**Interfaces:**

*Consumes:*
```go
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport
func Summarize(reports []FileReport) Summary
func (r FileReport) UncoveredLines() int
type Outcome struct { Test string; Status string; DurationMS int }
```

*Produces* (cmd-local, not contract surface):
```go
// RenderUncovered formats the post-run uncovered report exactly as spec §6 shows.
// Returns "" when there is nothing to say.
func RenderUncovered(reports []uncovered.FileReport) string

// ExitCodeFor returns the process exit code for a completed run.
// It is 1 if and only if a test failed or errored. An uncovered report is a signal,
// never a verdict (spec §2 non-goals, §6, D3).
func ExitCodeFor(outcomes []report.Outcome, reports []uncovered.FileReport) int
```

### Steps

- [ ] **8.1 — Write the failing exit-code test.** Create `cmd/rtdd/exit_test.go`:

```go
package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func uncoveredReports() []uncovered.FileReport {
	return []uncovered.FileReport{
		{Path: "src/auth.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 52, End: 58}, Class: uncovered.Uncovered},
		}},
	}
}

func TestExitCodeZeroWithNonEmptyUncoveredReport(t *testing.T) {
	// The load-bearing guarantee: `rtdd run` exits 1 ONLY when a test fails.
	// Uncovered is a signal, not a verdict.
	outcomes := []report.Outcome{
		{Test: "tests/test_auth.py::test_login", Status: "pass", DurationMS: 12},
		{Test: "tests/test_auth.py::test_logout", Status: "pass", DurationMS: 9},
	}
	reports := uncoveredReports()
	if n := uncovered.Summarize(reports).UncoveredLines; n != 7 {
		t.Fatalf("fixture is wrong: uncovered_lines = %d, want 7", n)
	}
	if got := ExitCodeFor(outcomes, reports); got != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0 with 7 uncovered lines and no failing test", got)
	}
}

func TestExitCodeZeroWithImportTimeOnlyReport(t *testing.T) {
	outcomes := []report.Outcome{{Test: "tests/t.py::a", Status: "pass", DurationMS: 1}}
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
	}
	if got := ExitCodeFor(outcomes, reports); got != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0", got)
	}
}

func TestExitCodeOneOnFailure(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []report.Outcome
		want     int
	}{
		{"all pass", []report.Outcome{{Test: "a", Status: "pass"}}, 0},
		{"one fail", []report.Outcome{{Test: "a", Status: "pass"}, {Test: "b", Status: "fail"}}, 1},
		{"one error", []report.Outcome{{Test: "a", Status: "error"}}, 1},
		{"skips are not failures", []report.Outcome{{Test: "a", Status: "skip"}}, 0},
		{"empty selection", nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeFor(tc.outcomes, uncoveredReports()); got != tc.want {
				t.Fatalf("ExitCodeFor() = %d, want %d", got, tc.want)
			}
		})
	}
}
```

- [ ] **8.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestExitCode
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/exit_test.go:...: undefined: ExitCodeFor
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **8.3 — Minimal implementation.** Create `cmd/rtdd/exit.go`:

```go
package main

import (
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// ExitCodeFor returns the process exit code for a completed run.
//
// It is 1 if and only if a test failed or errored. The uncovered report is accepted as a
// parameter precisely so that this function's tests can assert it is IGNORED: RTDD never
// exits nonzero to express a policy opinion (spec §2 non-goals, §6, decision D3).
func ExitCodeFor(outcomes []report.Outcome, _ []uncovered.FileReport) int {
	for _, o := range outcomes {
		if o.Status == "fail" || o.Status == "error" {
			return 1
		}
	}
	return 0
}
```

- [ ] **8.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run TestExitCode
```

Expected:

```
=== RUN   TestExitCodeZeroWithNonEmptyUncoveredReport
--- PASS: TestExitCodeZeroWithNonEmptyUncoveredReport (0.00s)
=== RUN   TestExitCodeZeroWithImportTimeOnlyReport
--- PASS: TestExitCodeZeroWithImportTimeOnlyReport (0.00s)
=== RUN   TestExitCodeOneOnFailure
=== RUN   TestExitCodeOneOnFailure/all_pass
=== RUN   TestExitCodeOneOnFailure/one_fail
=== RUN   TestExitCodeOneOnFailure/one_error
=== RUN   TestExitCodeOneOnFailure/skips_are_not_failures
=== RUN   TestExitCodeOneOnFailure/empty_selection
--- PASS: TestExitCodeOneOnFailure (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.007s
```

- [ ] **8.5 — Write the failing renderer test.** Create `cmd/rtdd/uncoveredtext_test.go`:

```go
package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

func TestRenderUncoveredMatchesSpecExample(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/auth.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 40, End: 51}, Class: uncovered.Covered},
			{Range: gitctx.LineRange{Start: 52, End: 58}, Class: uncovered.Uncovered},
		}},
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 12}, Class: uncovered.ImportTime},
		}},
	}
	got := RenderUncovered(reports)
	want := "" +
		"  UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)\n" +
		"  import-time: src/constants.py:1-12  (executed during collection, not attributed)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUncoveredCleanWhenOnlyImportTimeAndCovered(t *testing.T) {
	// A file whose changed lines are ALL import-time is correctly tested and must
	// produce no UNCOVERED line at all.
	reports := []uncovered.FileReport{
		{Path: "src/constants.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 1, End: 15}, Class: uncovered.ImportTime},
		}},
	}
	got := RenderUncovered(reports)
	want := "  import-time: src/constants.py:1-15  (executed during collection, not attributed)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
	if contains(got, "UNCOVERED") {
		t.Fatal("an all-import-time file must never render an UNCOVERED line")
	}
}

func TestRenderUncoveredSingleLineRange(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 9, End: 9}, Class: uncovered.Uncovered},
		}},
	}
	got := RenderUncovered(reports)
	want := "  UNCOVERED: src/logic.py:9  (1 changed line, no executing test)\n"
	if got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderUncoveredAllCoveredIsEmpty(t *testing.T) {
	reports := []uncovered.FileReport{
		{Path: "src/logic.py", Ranges: []uncovered.ClassifiedRange{
			{Range: gitctx.LineRange{Start: 5, End: 5}, Class: uncovered.Covered},
		}},
	}
	if got := RenderUncovered(reports); got != "" {
		t.Fatalf("RenderUncovered() = %q, want empty", got)
	}
}

func TestRenderUncoveredNoReports(t *testing.T) {
	if got := RenderUncovered(nil); got != "" {
		t.Fatalf("RenderUncovered(nil) = %q, want empty", got)
	}
}
```

- [ ] **8.6 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestRenderUncovered
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/uncoveredtext_test.go:...: undefined: RenderUncovered
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **8.7 — Minimal implementation.** Create `cmd/rtdd/uncoveredtext.go`:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// RenderUncovered formats the post-run uncovered report exactly as spec §6 shows:
//
//	UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)
//	import-time: src/constants.py:1-12  (executed during collection, not attributed)
//
// Import-time ranges are reported on their own line and are NEVER rendered as UNCOVERED.
// Returns "" when every changed line is Covered.
func RenderUncovered(reports []uncovered.FileReport) string {
	var b strings.Builder
	for _, r := range reports {
		for _, cr := range r.Ranges {
			if cr.Class != uncovered.Uncovered {
				continue
			}
			n := cr.Range.End - cr.Range.Start + 1
			fmt.Fprintf(&b, "  UNCOVERED: %s:%s  (%d changed %s, no executing test)\n",
				r.Path, spanText(cr.Range), n, plural(n, "line", "lines"))
		}
	}
	for _, r := range reports {
		for _, cr := range r.Ranges {
			if cr.Class != uncovered.ImportTime {
				continue
			}
			fmt.Fprintf(&b, "  import-time: %s:%s  (executed during collection, not attributed)\n",
				r.Path, spanText(cr.Range))
		}
	}
	return b.String()
}

func spanText(r gitctx.LineRange) string {
	if r.Start == r.End {
		return fmt.Sprintf("%d", r.Start)
	}
	return fmt.Sprintf("%d-%d", r.Start, r.End)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
```

- [ ] **8.8 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run 'TestRenderUncovered|TestExitCode'
```

Expected:

```
=== RUN   TestRenderUncoveredMatchesSpecExample
--- PASS: TestRenderUncoveredMatchesSpecExample (0.00s)
=== RUN   TestRenderUncoveredCleanWhenOnlyImportTimeAndCovered
--- PASS: TestRenderUncoveredCleanWhenOnlyImportTimeAndCovered (0.00s)
=== RUN   TestRenderUncoveredSingleLineRange
--- PASS: TestRenderUncoveredSingleLineRange (0.00s)
=== RUN   TestRenderUncoveredAllCoveredIsEmpty
--- PASS: TestRenderUncoveredAllCoveredIsEmpty (0.00s)
=== RUN   TestRenderUncoveredNoReports
--- PASS: TestRenderUncoveredNoReports (0.00s)
=== RUN   TestExitCodeZeroWithNonEmptyUncoveredReport
--- PASS: TestExitCodeZeroWithNonEmptyUncoveredReport (0.00s)
=== RUN   TestExitCodeZeroWithImportTimeOnlyReport
--- PASS: TestExitCodeZeroWithImportTimeOnlyReport (0.00s)
=== RUN   TestExitCodeOneOnFailure
--- PASS: TestExitCodeOneOnFailure (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.008s
```

- [ ] **8.9 — Commit.**

```bash
git add cmd/rtdd/exit.go cmd/rtdd/exit_test.go cmd/rtdd/uncoveredtext.go cmd/rtdd/uncoveredtext_test.go
git commit -m "cmd: uncovered text report and the exit-code rule

ExitCodeFor takes the uncovered report as a parameter solely so its tests can
assert it is ignored. An all-import-time file renders no UNCOVERED line."
```

---

## Task 9 — Post-run wiring: classify on FRESH coverage inside `rtdd run`

The classification input is the `*coverage.Result` the run just produced — never
`map.jsonl`, which holds no line data at all. This task adds the pure assembly function so
it is testable, then calls it from `cmd/rtdd/run.go`.

**Files:** `cmd/rtdd/uncoveredtext.go`, `cmd/rtdd/uncoveredtext_test.go`, `cmd/rtdd/run.go`

**Interfaces:**

*Consumes:*
```go
type RunResult struct {
    Outcomes []report.Outcome
    Coverage *coverage.Result
    Failed   []string
    ExitCode int
}
func (a *Adapter) IsInstrumentable(rel string) bool
func RawDiff(repoRoot, base string) (string, error)
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error)
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport
func (m *Map) TestsCovering(files []string) []string
```

*Produces* (cmd-local):
```go
// SignalInput is everything the post-run signal needs.
type SignalInput struct {
    Changes []gitctx.Change
    Cov     *coverage.Result
    IsInstrumentable func(rel string) bool
    Map     *mapstore.Map
}

// SignalOutput carries the classification and the import-fallback trigger set.
type SignalOutput struct {
    Reports        []uncovered.FileReport
    Instrumentable map[string]bool
    UnmappedFiles  []string
}

// BuildSignal filters the changed set to instrumentable files and classifies them
// against FRESH post-run coverage.
func BuildSignal(in SignalInput) SignalOutput
```

### Steps

- [ ] **9.1 — Write the failing test.** Append to `cmd/rtdd/uncoveredtext_test.go`:

```go
func TestBuildSignalFiltersToInstrumentableFiles(t *testing.T) {
	// A changed test file and a changed YAML asset are not in coverage at all.
	// Passing them to Classify would report them wholly Uncovered, which is wrong.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
		{Path: "tests/test_it.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 12}}},
		{Path: "config/app.yaml", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	cov := &coverage.Result{
		PerTest: []coverage.TestCoverage{
			{Test: "tests/test_it.py::test_logic", Files: map[string][]int{"src/logic.py": {5}}},
		},
		ImportTime: map[string][]int{"src/logic.py": {1, 4, 8}},
	}
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_it.py::test_logic", F: []string{"src/logic.py"}, C: "aaa", D: 3, S: "pass"},
		func(a, b string) string { return a })

	got := BuildSignal(SignalInput{
		Changes: changes,
		Cov:     cov,
		Map:     m,
		IsInstrumentable: func(rel string) bool {
			return rel == "src/logic.py" || rel == "src/constants.py"
		},
	})

	if len(got.Reports) != 1 || got.Reports[0].Path != "src/logic.py" {
		t.Fatalf("Reports = %#v, want only src/logic.py", got.Reports)
	}
	if got.Reports[0].UncoveredLines() != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1 (line 9 only; line 8 is a def, import-time)",
			got.Reports[0].UncoveredLines())
	}
	if !got.Instrumentable["src/logic.py"] || got.Instrumentable["tests/test_it.py"] {
		t.Fatalf("Instrumentable = %#v", got.Instrumentable)
	}
	if len(got.UnmappedFiles) != 0 {
		t.Fatalf("UnmappedFiles = %#v, want empty; src/logic.py IS covered by a map row", got.UnmappedFiles)
	}
}

func TestBuildSignalReportsUnmappedInstrumentableFiles(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	cov := &coverage.Result{
		PerTest:    []coverage.TestCoverage{},
		ImportTime: map[string][]int{"src/constants.py": {1, 2, 4, 7, 8, 9, 12, 13, 14, 15}},
	}
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_it.py::test_logic", F: []string{"src/logic.py"}, C: "aaa", D: 3, S: "pass"},
		func(a, b string) string { return a })

	got := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              cov,
		Map:              m,
		IsInstrumentable: func(rel string) bool { return true },
	})

	want := []string{"src/constants.py"}
	if len(got.UnmappedFiles) != 1 || got.UnmappedFiles[0] != want[0] {
		t.Fatalf("UnmappedFiles = %#v, want %#v — an import-time-only file is in no row's f",
			got.UnmappedFiles, want)
	}
	if got.Reports[0].UncoveredLines() != 0 {
		t.Fatalf("UncoveredLines() = %d, want 0 — every changed line is import-time",
			got.Reports[0].UncoveredLines())
	}
}

func TestBuildSignalWithNoCoverageIsAllUncovered(t *testing.T) {
	changes := []gitctx.Change{
		{Path: "src/new.py", Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}},
	}
	got := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              &coverage.Result{ImportTime: map[string][]int{}},
		Map:              mapstore.New(),
		IsInstrumentable: func(rel string) bool { return true },
	})
	if len(got.Reports) != 1 || got.Reports[0].UncoveredLines() != 3 {
		t.Fatalf("Reports = %#v, want 3 uncovered lines", got.Reports)
	}
}
```

and extend the import block at the top of `cmd/rtdd/uncoveredtext_test.go` to:

```go
import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)
```

- [ ] **9.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestBuildSignal
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/uncoveredtext_test.go:...: undefined: BuildSignal
cmd/rtdd/uncoveredtext_test.go:...: undefined: SignalInput
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **9.3 — Minimal implementation.** Append to `cmd/rtdd/uncoveredtext.go`:

```go
// SignalInput is everything the post-run uncovered signal needs.
//
// Cov MUST be the coverage produced by the run that just finished. Line data is never
// persisted in map.jsonl, so both the changed line ranges and the coverage are current by
// construction and there is no line-drift problem (spec §4, §6).
type SignalInput struct {
	Changes          []gitctx.Change
	Cov              *coverage.Result
	IsInstrumentable func(rel string) bool
	Map              *mapstore.Map
}

// SignalOutput carries the classification and the import-fallback trigger set.
type SignalOutput struct {
	Reports        []uncovered.FileReport
	Instrumentable map[string]bool
	UnmappedFiles  []string
}

// BuildSignal filters the changed set to instrumentable files and classifies them
// against fresh post-run coverage.
//
// UnmappedFiles is the set of changed instrumentable files that NO map row covers.
// Because import-time lines are attributed to no test, they never enter any row's f, so
// this set is exactly the static-import fallback's trigger set (spec §6, D14).
func BuildSignal(in SignalInput) SignalOutput {
	out := SignalOutput{
		Instrumentable: map[string]bool{},
		UnmappedFiles:  []string{},
	}
	var keep []gitctx.Change
	for _, c := range in.Changes {
		ok := in.IsInstrumentable != nil && in.IsInstrumentable(c.Path)
		out.Instrumentable[c.Path] = ok
		if !ok || c.Status == gitctx.Deleted {
			continue
		}
		keep = append(keep, c)
		if in.Map != nil && len(in.Map.TestsCovering([]string{c.Path})) == 0 {
			out.UnmappedFiles = append(out.UnmappedFiles, c.Path)
		}
	}
	sort.Strings(out.UnmappedFiles)
	out.Reports = uncovered.Classify(keep, in.Cov)
	return out
}
```

and extend the import block at the top of `cmd/rtdd/uncoveredtext.go` to:

```go
import (
	"fmt"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)
```

- [ ] **9.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run TestBuildSignal
```

Expected:

```
=== RUN   TestBuildSignalFiltersToInstrumentableFiles
--- PASS: TestBuildSignalFiltersToInstrumentableFiles (0.00s)
=== RUN   TestBuildSignalReportsUnmappedInstrumentableFiles
--- PASS: TestBuildSignalReportsUnmappedInstrumentableFiles (0.00s)
=== RUN   TestBuildSignalWithNoCoverageIsAllUncovered
--- PASS: TestBuildSignalWithNoCoverageIsAllUncovered (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.009s
```

- [ ] **9.5 — Wire it into `rtdd run`.** In `cmd/rtdd/run.go`, after the runner returns and
the map rows are refreshed, and **before** the process exits, insert:

```go
	// Populate the changed line ranges from git diff --unified=0. WithLines is
	// authoritative and overwrites Lines, so hunk parsing is the single source of truth.
	rawDiff, err := gitctx.RawDiff(repoRoot, base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}
	changes, err = uncovered.WithLines(repoRoot, changes, rawDiff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}

	// Classify against the coverage this run just produced — never against map.jsonl,
	// which carries no line data.
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              res.Coverage,
		IsInstrumentable: ad.IsInstrumentable,
		Map:              m,
	})

	code := ExitCodeFor(res.Outcomes, sig.Reports)

	if jsonOut {
		out := BuildOutput(OutputInput{
			Command:        "run",
			Base:           base,
			Adapter:        ad.Name,
			Sel:            sel,
			Changes:        changes,
			Instrumentable: sig.Instrumentable,
			Executed:       true,
			Outcomes:       res.Outcomes,
			Reports:        sig.Reports,
			UncoveredOK:    true,
			UnmappedFiles:  sig.UnmappedFiles,
			ImportFallback: importFallback,
		})
		out.ExitCode = code
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
			return 2
		}
		return code
	}

	if s := RenderUncovered(sig.Reports); s != "" {
		fmt.Fprint(os.Stdout, "\n"+s)
	}
	return code
```

Identifiers assumed present in `run.go` from M1b: `repoRoot`, `base`, `changes`, `ad`
(the `*adapter.Adapter`), `m` (the `*mapstore.Map`), `sel` (the `selector.Selection`),
`res` (the `*runner.RunResult`), `jsonOut` (the `--json` flag), and `importFallback`
(the `map[string][]string` recorded when `selector.Inputs.ImportOnly` fired; declare it as
`importFallback := map[string][]string{}` if M1b did not). Ensure `run.go` imports
`encoding/json`, `fmt`, `os`, `internal/gitctx` and `internal/uncovered`.

- [ ] **9.6 — Build and run the full suite.**

```bash
go build ./... && go test ./...
```

Expected: `ok` for every package, no `FAIL` lines.

- [ ] **9.7 — Commit.**

```bash
git add cmd/rtdd/uncoveredtext.go cmd/rtdd/uncoveredtext_test.go cmd/rtdd/run.go
git commit -m "cmd: classify changed lines on fresh post-run coverage in rtdd run

Only instrumentable changes are classified; test files and opaque assets are
excluded so they cannot be reported wholly uncovered. UnmappedFiles is the
static-import fallback's trigger set."
```

---

## Task 10 — `rtdd which`: selection plus the honest "no fresh coverage" answer

`rtdd which` runs nothing, so it has no fresh coverage and therefore no uncovered report.
Reporting a stale one would reintroduce exactly the line-drift problem spec §4 eliminates.
`which` instead emits `uncovered.available: false` with a reason, plus `unmapped_files` —
the file-level signal it *can* honestly compute from the map.

**Files:** `cmd/rtdd/whichtext.go`, `cmd/rtdd/whichtext_test.go`, `cmd/rtdd/which.go`

**Interfaces:**

*Consumes:*
```go
func BuildOutput(in OutputInput) Output
func BuildSignal(in SignalInput) SignalOutput
type Selection struct { Tier Tier; Tests []string; Direct []string; Reason string }
```

*Produces* (cmd-local):
```go
// RenderWhich formats the ranked selection and the unmapped-file notice.
func RenderWhich(sel selector.Selection, unmapped []string) string
```

### Steps

- [ ] **10.1 — Write the failing test.** Create `cmd/rtdd/whichtext_test.go`:

```go
package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/selector"
)

func TestRenderWhichRankedSelection(t *testing.T) {
	sel := selector.Selection{
		Tier:   selector.TierT0,
		Direct: []string{"tests/test_new.py"},
		Tests:  []string{"tests/test_new.py", "tests/test_it.py::test_logic"},
		Reason: "",
	}
	got := RenderWhich(sel, nil)
	want := "" +
		"  tier: T0  (2 tests selected, ranked)\n" +
		"  direct: tests/test_new.py\n" +
		"    tests/test_new.py\n" +
		"    tests/test_it.py::test_logic\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhichEmptySelectionIsExplicit(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierEmpty, Reason: "no map row intersects the changed set"}
	got := RenderWhich(sel, nil)
	want := "" +
		"  tier: empty  (0 tests selected, ranked)\n" +
		"  reason: no map row intersects the changed set\n" +
		"  NOTHING SELECTED — this is not the same as \"all passed\".\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhichNamesUnmappedFiles(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_it.py"},
		Reason: "import-time-only change"}
	got := RenderWhich(sel, []string{"src/constants.py"})
	want := "" +
		"  tier: T1  (1 test selected, ranked)\n" +
		"  reason: import-time-only change\n" +
		"    tests/test_it.py\n" +
		"  no map row covers: src/constants.py  (import-time-only or untested; " +
		"tests selected by static import scan)\n"
	if got != want {
		t.Fatalf("RenderWhich()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWhichJSONHasNoUncoveredReport(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which", Base: "HEAD", Adapter: "python",
		Sel:           selector.Selection{Tier: selector.TierT1, Tests: []string{"tests/test_it.py"}},
		UnmappedFiles: []string{"src/constants.py"},
	})
	if out.Uncovered.Available {
		t.Fatal("which must never claim a fresh uncovered report")
	}
	if len(out.UnmappedFiles) != 1 {
		t.Fatalf("unmapped_files = %#v", out.UnmappedFiles)
	}
	if out.ExitCode != 0 {
		t.Fatalf("exit_code = %d, want 0", out.ExitCode)
	}
}
```

- [ ] **10.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run 'TestRenderWhich|TestWhichJSON'
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/whichtext_test.go:...: undefined: RenderWhich
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **10.3 — Minimal implementation.** Create `cmd/rtdd/whichtext.go`:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/selector"
)

// RenderWhich formats the ranked selection and the unmapped-file notice.
//
// An empty selection is stated explicitly and is never allowed to read as "all passed"
// (spec §5, tier "empty").
func RenderWhich(sel selector.Selection, unmapped []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  tier: %s  (%d %s selected, ranked)\n",
		sel.Tier.String(), len(sel.Tests), plural(len(sel.Tests), "test", "tests"))
	if len(sel.Direct) > 0 {
		fmt.Fprintf(&b, "  direct: %s\n", strings.Join(sel.Direct, ", "))
	}
	if sel.Reason != "" {
		fmt.Fprintf(&b, "  reason: %s\n", sel.Reason)
	}
	for _, t := range sel.Tests {
		fmt.Fprintf(&b, "    %s\n", t)
	}
	if len(sel.Tests) == 0 {
		b.WriteString("  NOTHING SELECTED — this is not the same as \"all passed\".\n")
	}
	for _, f := range unmapped {
		fmt.Fprintf(&b, "  no map row covers: %s  (import-time-only or untested; "+
			"tests selected by static import scan)\n", f)
	}
	return b.String()
}
```

- [ ] **10.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run 'TestRenderWhich|TestWhichJSON'
```

Expected:

```
=== RUN   TestRenderWhichRankedSelection
--- PASS: TestRenderWhichRankedSelection (0.00s)
=== RUN   TestRenderWhichEmptySelectionIsExplicit
--- PASS: TestRenderWhichEmptySelectionIsExplicit (0.00s)
=== RUN   TestRenderWhichNamesUnmappedFiles
--- PASS: TestRenderWhichNamesUnmappedFiles (0.00s)
=== RUN   TestWhichJSONHasNoUncoveredReport
--- PASS: TestWhichJSONHasNoUncoveredReport (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.009s
```

- [ ] **10.5 — Wire it into `rtdd which`.** In `cmd/rtdd/which.go`, replace the selection
printing with:

```go
	rawDiff, err := gitctx.RawDiff(repoRoot, base)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}
	changes, err = uncovered.WithLines(repoRoot, changes, rawDiff)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}

	// which runs nothing, so there is no fresh coverage and therefore no line-level
	// signal. Only the file-level "no map row covers this" answer is honest here.
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              nil,
		IsInstrumentable: ad.IsInstrumentable,
		Map:              m,
	})

	if jsonOut {
		out := BuildOutput(OutputInput{
			Command:        "which",
			Base:           base,
			Adapter:        ad.Name,
			Sel:            sel,
			Changes:        changes,
			Instrumentable: sig.Instrumentable,
			Executed:       false,
			UncoveredOK:    false,
			UnmappedFiles:  sig.UnmappedFiles,
			ImportFallback: importFallback,
		})
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
			return 2
		}
		return 0
	}

	fmt.Fprint(os.Stdout, RenderWhich(sel, sig.UnmappedFiles))
	return 0
```

Ensure `which.go` imports `encoding/json`, `fmt`, `os`, `internal/gitctx` and
`internal/uncovered`.

- [ ] **10.6 — Build and run the full suite.**

```bash
go build ./... && go test ./...
```

Expected: `ok` for every package, no `FAIL` lines.

- [ ] **10.7 — Commit.**

```bash
git add cmd/rtdd/whichtext.go cmd/rtdd/whichtext_test.go cmd/rtdd/which.go
git commit -m "cmd: rtdd which reports selection and unmapped files, never a stale signal

which runs nothing, so uncovered.available is false with a reason. Reporting a
stale line-level report would reintroduce the drift problem the design removes."
```

---

## Task 11 — `rtdd explain <file>`

**Files:** `cmd/rtdd/explain.go`, `cmd/rtdd/explain_test.go`, `cmd/rtdd/main.go`

**Interfaces:**

*Consumes:*
```go
func (m *Map) TestsCovering(files []string) []string
func (m *Map) Get(t string) (Row, bool)
func (m *Map) Len() int
```

*Produces* (cmd-local):
```go
// RenderExplain lists the tests covering path, ascending by duration then id.
func RenderExplain(m *mapstore.Map, path string) string
```

### Steps

- [ ] **11.1 — Write the failing test.** Create `cmd/rtdd/explain_test.go`:

```go
package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func explainFixture() *mapstore.Map {
	keep := func(a, b string) string { return a }
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_b.py::t2", F: []string{"src/hub.py"}, C: "aaa", D: 90, S: "pass"}, keep)
	m.Union(mapstore.Row{T: "tests/test_a.py::t1", F: []string{"src/hub.py", "src/a.py"}, C: "aaa", D: 12, S: "fail"}, keep)
	m.Union(mapstore.Row{T: "tests/test_c.py::t3", F: []string{"src/other.py"}, C: "aaa", D: 5, S: "pass"}, keep)
	return m
}

func TestRenderExplainListsCoveringTests(t *testing.T) {
	got := RenderExplain(explainFixture(), "src/hub.py")
	want := "" +
		"src/hub.py is covered by 2 tests:\n" +
		"    tests/test_a.py::t1        12ms  fail\n" +
		"    tests/test_b.py::t2        90ms  pass\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderExplainNoCoveringTests(t *testing.T) {
	got := RenderExplain(explainFixture(), "src/constants.py")
	want := "" +
		"src/constants.py is covered by 0 tests.\n" +
		"  No map row lists this file. Either nothing exercises it, or it only ever\n" +
		"  executes at import time, where coverage attributes it to no test at all\n" +
		"  (spec §6). Selection falls back to a static import scan for this file.\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderExplainEmptyMap(t *testing.T) {
	got := RenderExplain(mapstore.New(), "src/hub.py")
	want := "" +
		"src/hub.py is covered by 0 tests.\n" +
		"  The map is empty. Run `rtdd seed` first.\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}
```

- [ ] **11.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestRenderExplain
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/explain_test.go:...: undefined: RenderExplain
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **11.3 — Minimal implementation.** Create `cmd/rtdd/explain.go`:

```go
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// RenderExplain lists the tests covering path, ascending by duration then id.
//
// A file with zero covering tests is the import-time-only case as often as it is the
// untested case, and the output says so rather than implying the file is untested.
func RenderExplain(m *mapstore.Map, path string) string {
	ids := m.TestsCovering([]string{path})
	var b strings.Builder

	if len(ids) == 0 {
		fmt.Fprintf(&b, "%s is covered by 0 tests.\n", path)
		if m.Len() == 0 {
			b.WriteString("  The map is empty. Run `rtdd seed` first.\n")
			return b.String()
		}
		b.WriteString("  No map row lists this file. Either nothing exercises it, or it only ever\n")
		b.WriteString("  executes at import time, where coverage attributes it to no test at all\n")
		b.WriteString("  (spec §6). Selection falls back to a static import scan for this file.\n")
		return b.String()
	}

	rows := make([]mapstore.Row, 0, len(ids))
	for _, id := range ids {
		if r, ok := m.Get(id); ok {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].D != rows[j].D {
			return rows[i].D < rows[j].D
		}
		return rows[i].T < rows[j].T
	})

	fmt.Fprintf(&b, "%s is covered by %d %s:\n", path, len(rows), plural(len(rows), "test", "tests"))
	for _, r := range rows {
		fmt.Fprintf(&b, "    %-25s%4dms  %s\n", r.T, r.D, r.S)
	}
	return b.String()
}
```

- [ ] **11.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run TestRenderExplain
```

Expected:

```
=== RUN   TestRenderExplainListsCoveringTests
--- PASS: TestRenderExplainListsCoveringTests (0.00s)
=== RUN   TestRenderExplainNoCoveringTests
--- PASS: TestRenderExplainNoCoveringTests (0.00s)
=== RUN   TestRenderExplainEmptyMap
--- PASS: TestRenderExplainEmptyMap (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.009s
```

- [ ] **11.5 — Add the command.** Append to `cmd/rtdd/explain.go`:

```go
// cmdExplain implements `rtdd explain <file>`.
func cmdExplain(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: rtdd explain <file>")
		return 2
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}
	rel, ok := paths.Normalize(repoRoot, args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "rtdd: %q is outside the repository\n", args[0])
		return 2
	}
	m, err := mapstore.Load(filepath.Join(repoRoot, ".rtdd", "map.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 2
	}
	fmt.Fprint(os.Stdout, RenderExplain(m, rel))
	return 0
}
```

and extend the import block at the top of `cmd/rtdd/explain.go` to:

```go
import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/paths"
)
```

In `cmd/rtdd/main.go`, add to the subcommand dispatch switch:

```go
	case "explain":
		return cmdExplain(args)
```

- [ ] **11.6 — Build and check by hand.**

```bash
go build ./... && go test ./cmd/rtdd/
```

Expected: `ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.010s`

- [ ] **11.7 — Commit.**

```bash
git add cmd/rtdd/explain.go cmd/rtdd/explain_test.go cmd/rtdd/main.go
git commit -m "cmd: rtdd explain <file>

Zero covering tests is reported as import-time-only OR untested, never as
untested alone."
```

---

## Task 12 — `rtdd doctor`

**Files:** `cmd/rtdd/doctor.go`, `cmd/rtdd/doctor_test.go`, `cmd/rtdd/main.go`

**Interfaces:**

*Consumes:*
```go
type Hub struct { Path string; TestCount int; Fraction float64 }
func Hubs(m *mapstore.Map) []Hub
const Caveat = "..."
```

*Produces* (cmd-local):
```go
// RenderDoctor formats the fan-out table. The spec §9 caveat is always included.
func RenderDoctor(hubs []doctor.Hub, total int, limit int) string
```

### Steps

- [ ] **12.1 — Write the failing test.** Create `cmd/rtdd/doctor_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/doctor"
)

func TestRenderDoctorTable(t *testing.T) {
	hubs := []doctor.Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
		{Path: "src/a.py", TestCount: 1, Fraction: 0.25},
	}
	got := RenderDoctor(hubs, 4, 2)
	want := "" +
		"fan-out over 4 tests (top 2 of 3 files)\n" +
		"\n" +
		"  tests  share  file\n" +
		"      3    75%  src/hub.py\n" +
		"      2    50%  src/b.py\n" +
		"\n" +
		doctor.Caveat + "\n"
	if got != want {
		t.Fatalf("RenderDoctor()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderDoctorAlwaysPrintsTheCaveat(t *testing.T) {
	// Spec §9: the limitation must be stated in the tool's own output, alongside the
	// table, because the repo's most coupled file can appear as its cleanest.
	for _, tc := range []struct {
		name string
		hubs []doctor.Hub
		tot  int
	}{
		{"populated", []doctor.Hub{{Path: "src/a.py", TestCount: 1, Fraction: 1}}, 1},
		{"empty map", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderDoctor(tc.hubs, tc.tot, 20)
			if !strings.Contains(got, doctor.Caveat) {
				t.Fatalf("doctor output is missing the §9 caveat:\n%s", got)
			}
			for _, needle := range []string{"lru_cache", "session-scoped fixture", "fan-out of 1", "cleanest"} {
				if !strings.Contains(got, needle) {
					t.Fatalf("doctor output is missing %q:\n%s", needle, got)
				}
			}
		})
	}
}

func TestRenderDoctorEmptyMap(t *testing.T) {
	got := RenderDoctor(nil, 0, 20)
	if !strings.Contains(got, "map is empty") {
		t.Fatalf("RenderDoctor() on an empty map should say so:\n%s", got)
	}
	if !strings.Contains(got, "rtdd seed") {
		t.Fatalf("RenderDoctor() should point at `rtdd seed`:\n%s", got)
	}
}
```

- [ ] **12.2 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run TestRenderDoctor
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/doctor_test.go:...: undefined: RenderDoctor
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **12.3 — Minimal implementation.** Create `cmd/rtdd/doctor.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// RenderDoctor formats the fan-out table. The spec §9 caveat is ALWAYS included: a
// fan-out number without it actively misleads, because anything executed once per
// process is attributed to whichever test ran first.
func RenderDoctor(hubs []doctor.Hub, total, limit int) string {
	var b strings.Builder
	if len(hubs) == 0 {
		b.WriteString("fan-out: the map is empty. Run `rtdd seed` first.\n")
		b.WriteString("\n")
		b.WriteString(doctor.Caveat + "\n")
		return b.String()
	}
	shown := hubs
	if limit > 0 && limit < len(hubs) {
		shown = hubs[:limit]
	}
	fmt.Fprintf(&b, "fan-out over %d %s (top %d of %d files)\n",
		total, plural(total, "test", "tests"), len(shown), len(hubs))
	b.WriteString("\n")
	b.WriteString("  tests  share  file\n")
	for _, h := range shown {
		fmt.Fprintf(&b, "  %5d  %4.0f%%  %s\n", h.TestCount, h.Fraction*100, h.Path)
	}
	b.WriteString("\n")
	b.WriteString(doctor.Caveat + "\n")
	return b.String()
}

// cmdDoctor implements `rtdd doctor`.
func cmdDoctor(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: rtdd doctor")
		return 2
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}
	m, err := mapstore.Load(filepath.Join(repoRoot, ".rtdd", "map.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 2
	}
	fmt.Fprint(os.Stdout, RenderDoctor(doctor.Hubs(m), m.Len(), 20))
	return 0
}
```

In `cmd/rtdd/main.go`, add to the subcommand dispatch switch:

```go
	case "doctor":
		return cmdDoctor(args)
```

- [ ] **12.4 — Run it and see it pass.**

```bash
go test ./cmd/rtdd/ -v -run TestRenderDoctor
```

Expected:

```
=== RUN   TestRenderDoctorTable
--- PASS: TestRenderDoctorTable (0.00s)
=== RUN   TestRenderDoctorAlwaysPrintsTheCaveat
=== RUN   TestRenderDoctorAlwaysPrintsTheCaveat/populated
=== RUN   TestRenderDoctorAlwaysPrintsTheCaveat/empty_map
--- PASS: TestRenderDoctorAlwaysPrintsTheCaveat (0.00s)
=== RUN   TestRenderDoctorEmptyMap
--- PASS: TestRenderDoctorEmptyMap (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.010s
```

- [ ] **12.5 — Commit.**

```bash
git add cmd/rtdd/doctor.go cmd/rtdd/doctor_test.go cmd/rtdd/main.go
git commit -m "cmd: rtdd doctor prints fan-out with the mandatory §9 caveat

The caveat is emitted even for an empty map, and a test asserts it names
lru_cache, session-scoped fixtures, fan-out of 1, and 'cleanest'."
```

---

## Task 13 — `internal/initrepo` and `rtdd init`

`rtdd init` installs `.gitattributes`, config, and the agent front-ends into a host repo —
**including a merge strategy for a repo that already has an `AGENTS.md` or `CLAUDE.md`**
(spec §13). Nothing is ever clobbered.

**Merge rules:**

| Existing file | Action |
|---|---|
| absent | create it containing only the managed block |
| present, contains `<!-- BEGIN RTDD -->` … `<!-- END RTDD -->` | replace **only** the text between the markers; everything outside is byte-identical |
| present, no markers | append a blank line and the managed block; the original content is byte-identical and comes first |
| present, block already identical | report `unchanged`, write nothing |

**Files:** `internal/initrepo/assets/agents-block.md`, `internal/initrepo/initrepo.go`, `internal/initrepo/initrepo_test.go`, `cmd/rtdd/init.go`, `cmd/rtdd/init_test.go`, `cmd/rtdd/main.go`, `docs/plans/00-interfaces.md`

**Interfaces:**

*Consumes:* nothing from other RTDD packages.

*Produces* (contract addition — new package, added to `00-interfaces.md` in this same commit):
```go
type Action struct {
    Path string // repo-relative
    Kind string // "created" | "updated" | "unchanged"
}

const (
    BeginMarker = "<!-- BEGIN RTDD -->"
    EndMarker   = "<!-- END RTDD -->"
)

// MergeManagedBlock returns existing with block installed between the markers.
// Never clobbers: content outside the markers is preserved byte for byte.
func MergeManagedBlock(existing, block string) string

func EnsureGitAttributes(repoRoot string) (Action, error)
func EnsureConfig(repoRoot string) (Action, error)
func EnsureFrontEnd(repoRoot, rel, block string) (Action, error)

// Block returns the managed agent front-end text, marker lines included.
func Block() string

// Run installs .gitattributes, .rtdd/config.yaml, AGENTS.md, CLAUDE.md and
// .cursor/rules/rtdd.mdc. Actions are returned in installation order.
func Run(repoRoot string) ([]Action, error)
```

### Steps

- [ ] **13.1 — Write the failing test.** Create `internal/initrepo/initrepo_test.go`:

```go
package initrepo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func put(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeManagedBlockIntoAbsentFile(t *testing.T) {
	got := MergeManagedBlock("", "BLOCK")
	if got != "BLOCK\n" {
		t.Fatalf("MergeManagedBlock() = %q, want %q", got, "BLOCK\n")
	}
}

func TestMergeManagedBlockAppendsWithoutClobbering(t *testing.T) {
	existing := "# My project\n\nRules the team wrote.\n"
	got := MergeManagedBlock(existing, BeginMarker+"\nrtdd\n"+EndMarker)
	if !strings.HasPrefix(got, existing) {
		t.Fatalf("existing content must be preserved byte for byte and come first:\n%q", got)
	}
	want := existing + "\n" + BeginMarker + "\nrtdd\n" + EndMarker + "\n"
	if got != want {
		t.Fatalf("MergeManagedBlock()\n got: %q\nwant: %q", got, want)
	}
}

func TestMergeManagedBlockReplacesBetweenMarkersOnly(t *testing.T) {
	existing := "HEADER\n" + BeginMarker + "\nold rtdd text\n" + EndMarker + "\nFOOTER\n"
	block := BeginMarker + "\nnew rtdd text\n" + EndMarker
	got := MergeManagedBlock(existing, block)
	want := "HEADER\n" + BeginMarker + "\nnew rtdd text\n" + EndMarker + "\nFOOTER\n"
	if got != want {
		t.Fatalf("MergeManagedBlock()\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "HEADER") || !strings.Contains(got, "FOOTER") {
		t.Fatal("content outside the markers must survive")
	}
}

func TestEnsureFrontEndOnExistingAgentsMD(t *testing.T) {
	root := t.TempDir()
	original := "# AGENTS\n\nDo not run destructive commands.\n"
	put(t, root, "AGENTS.md", original)

	act, err := EnsureFrontEnd(root, "AGENTS.md", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act.Kind != "updated" {
		t.Fatalf("Kind = %q, want \"updated\"", act.Kind)
	}
	got := read(t, root, "AGENTS.md")
	if !strings.HasPrefix(got, original) {
		t.Fatalf("the host repo's AGENTS.md was clobbered:\n%s", got)
	}
	if !strings.Contains(got, BeginMarker) || !strings.Contains(got, EndMarker) {
		t.Fatalf("managed block missing:\n%s", got)
	}

	// Second run is idempotent.
	act2, err := EnsureFrontEnd(root, "AGENTS.md", Block())
	if err != nil {
		t.Fatalf("EnsureFrontEnd() error = %v", err)
	}
	if act2.Kind != "unchanged" {
		t.Fatalf("second Kind = %q, want \"unchanged\"", act2.Kind)
	}
	if read(t, root, "AGENTS.md") != got {
		t.Fatal("second run must be byte-identical")
	}
}

func TestEnsureGitAttributes(t *testing.T) {
	root := t.TempDir()
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "created" {
		t.Fatalf("Kind = %q, want \"created\"", act.Kind)
	}
	if got := read(t, root, ".gitattributes"); got != ".rtdd/map.jsonl merge=union\n" {
		t.Fatalf(".gitattributes = %q", got)
	}
}

func TestEnsureGitAttributesAppendsToExisting(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitattributes", "*.png binary\n")
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "updated" {
		t.Fatalf("Kind = %q, want \"updated\"", act.Kind)
	}
	want := "*.png binary\n.rtdd/map.jsonl merge=union\n"
	if got := read(t, root, ".gitattributes"); got != want {
		t.Fatalf(".gitattributes = %q, want %q", got, want)
	}
}

func TestEnsureGitAttributesIdempotent(t *testing.T) {
	root := t.TempDir()
	put(t, root, ".gitattributes", "*.png binary\n.rtdd/map.jsonl merge=union\n")
	act, err := EnsureGitAttributes(root)
	if err != nil {
		t.Fatalf("EnsureGitAttributes() error = %v", err)
	}
	if act.Kind != "unchanged" {
		t.Fatalf("Kind = %q, want \"unchanged\"", act.Kind)
	}
}

func TestEnsureConfigNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	act, err := EnsureConfig(root)
	if err != nil {
		t.Fatalf("EnsureConfig() error = %v", err)
	}
	if act.Kind != "created" {
		t.Fatalf("Kind = %q, want \"created\"", act.Kind)
	}
	got := read(t, root, ".rtdd/config.yaml")
	for _, needle := range []string{"stale_commits: 50", "drift_guard: 100", "hub_threshold: 0.40"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("config.yaml missing %q:\n%s", needle, got)
		}
	}

	put(t, root, ".rtdd/config.yaml", "stale_commits: 7\n")
	act2, err := EnsureConfig(root)
	if err != nil {
		t.Fatalf("EnsureConfig() error = %v", err)
	}
	if act2.Kind != "unchanged" {
		t.Fatalf("Kind = %q, want \"unchanged\"", act2.Kind)
	}
	if read(t, root, ".rtdd/config.yaml") != "stale_commits: 7\n" {
		t.Fatal("an existing config must never be overwritten")
	}
}

func TestRunInstallsEverything(t *testing.T) {
	root := t.TempDir()
	put(t, root, "CLAUDE.md", "# House rules\n")

	acts, err := Run(root)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	want := []string{".gitattributes", ".rtdd/config.yaml", "AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"}
	if len(acts) != len(want) {
		t.Fatalf("Run() returned %d actions, want %d: %#v", len(acts), len(want), acts)
	}
	for i, w := range want {
		if acts[i].Path != w {
			t.Fatalf("acts[%d].Path = %q, want %q", i, acts[i].Path, w)
		}
	}
	if !strings.HasPrefix(read(t, root, "CLAUDE.md"), "# House rules\n") {
		t.Fatal("an existing CLAUDE.md must not be clobbered")
	}
	if !strings.Contains(read(t, root, "CLAUDE.md"), BeginMarker) {
		t.Fatal("CLAUDE.md must gain the managed block")
	}
	if !strings.Contains(read(t, root, "AGENTS.md"), BeginMarker) {
		t.Fatal("AGENTS.md must be created with the managed block")
	}
	if !strings.Contains(read(t, root, ".cursor/rules/rtdd.mdc"), BeginMarker) {
		t.Fatal(".cursor/rules/rtdd.mdc must be created with the managed block")
	}
	if read(t, root, ".gitattributes") != ".rtdd/map.jsonl merge=union\n" {
		t.Fatal(".gitattributes must carry the union merge driver")
	}
}
```

- [ ] **13.2 — Run it and see it fail.**

```bash
go test ./internal/initrepo/
```

Expected:

```
# github.com/VocanicZ/rtdd/internal/initrepo [github.com/VocanicZ/rtdd/internal/initrepo.test]
internal/initrepo/initrepo_test.go:...: undefined: MergeManagedBlock
internal/initrepo/initrepo_test.go:...: undefined: BeginMarker
internal/initrepo/initrepo_test.go:...: undefined: Block
FAIL	github.com/VocanicZ/rtdd/internal/initrepo [build failed]
```

- [ ] **13.3 — Write the managed block asset.** Create
`internal/initrepo/assets/agents-block.md` with exactly this content (marker lines
included — `Block()` embeds the file verbatim):

````markdown
<!-- BEGIN RTDD -->
## RTDD — which tests cover what you just changed

RTDD reports; you decide. It never blocks and never gates.

Before you verify a change, ask it what to run:

```
rtdd which --json
```

`selection.tests` is the ranked list of tests that can observe your change.
`tier: "empty"` means nothing was selected — that is NOT the same as "all passed".

To run that selection and get the uncovered-change signal:

```
rtdd run --json
```

Read `uncovered` in the output:

- `class: "uncovered"` — you changed these lines and no test executed them. This is the
  signal worth acting on.
- `class: "import-time"` — these lines DID execute, during import, where coverage
  attributes them to no test. Dataclasses, enums, config modules, Pydantic and Django
  models, and `__init__.py` re-exports land here. **This is not a gap.**
- `class: "covered"` — a test executed them.

`exit_code` is `1` only when a test failed. A non-empty uncovered report never changes it.

Other commands:

```
rtdd explain <file>   which tests cover this file
rtdd doctor           fan-out / coupling report (read its caveat)
rtdd verify           full suite
rtdd seed             one full instrumented run; rebuilds the map
```

Do not edit between the RTDD markers; `rtdd init` rewrites this block.
<!-- END RTDD -->
````

- [ ] **13.4 — Minimal implementation.** Create `internal/initrepo/initrepo.go`:

```go
// Package initrepo installs RTDD into a host repository: the union merge driver, the
// config, and the agent front-ends. Nothing is ever clobbered.
package initrepo

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/agents-block.md
var agentsBlock string

// Markers delimiting the RTDD-managed region of a host repo's agent front-end file.
const (
	BeginMarker = "<!-- BEGIN RTDD -->"
	EndMarker   = "<!-- END RTDD -->"
)

const gitAttributesLine = ".rtdd/map.jsonl merge=union"

const defaultConfig = "# rtdd configuration — see docs/specs for meaning\n" +
	"stale_commits: 50\n" +
	"drift_guard: 100\n" +
	"hub_threshold: 0.40\n"

// Action records what init did to one path.
type Action struct {
	Path string // repo-relative
	Kind string // "created" | "updated" | "unchanged"
}

// Block returns the managed agent front-end text, marker lines included.
func Block() string { return strings.TrimRight(agentsBlock, "\n") }

// MergeManagedBlock returns existing with block installed between the markers.
//
// Never clobbers. If the markers are present, only the text between them changes and
// everything outside is preserved byte for byte. If they are absent, block is appended
// after a blank line and the original content comes first.
func MergeManagedBlock(existing, block string) string {
	block = strings.TrimRight(block, "\n")
	if existing == "" {
		return block + "\n"
	}
	b := strings.Index(existing, BeginMarker)
	e := strings.Index(existing, EndMarker)
	if b >= 0 && e > b {
		return existing[:b] + block + existing[e+len(EndMarker):]
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return existing + "\n" + block + "\n"
}

// EnsureGitAttributes adds the union merge driver for .rtdd/map.jsonl.
func EnsureGitAttributes(repoRoot string) (Action, error) {
	rel := ".gitattributes"
	cur, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	for _, line := range strings.Split(cur, "\n") {
		if strings.TrimSpace(line) == gitAttributesLine {
			return Action{Path: rel, Kind: "unchanged"}, nil
		}
	}
	next := cur
	if next != "" && !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	next += gitAttributesLine + "\n"
	if err := writeFile(repoRoot, rel, next); err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "updated"}, nil
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// EnsureConfig writes .rtdd/config.yaml only when it does not already exist.
// A host repo's tuned config is never overwritten.
func EnsureConfig(repoRoot string) (Action, error) {
	rel := ".rtdd/config.yaml"
	_, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "unchanged"}, nil
	}
	if err := writeFile(repoRoot, rel, defaultConfig); err != nil {
		return Action{}, err
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// EnsureFrontEnd installs block into repoRoot/rel using MergeManagedBlock.
func EnsureFrontEnd(repoRoot, rel, block string) (Action, error) {
	cur, existed, err := readIfExists(repoRoot, rel)
	if err != nil {
		return Action{}, err
	}
	next := MergeManagedBlock(cur, block)
	if next == cur {
		return Action{Path: rel, Kind: "unchanged"}, nil
	}
	if err := writeFile(repoRoot, rel, next); err != nil {
		return Action{}, err
	}
	if existed {
		return Action{Path: rel, Kind: "updated"}, nil
	}
	return Action{Path: rel, Kind: "created"}, nil
}

// Run installs .gitattributes, .rtdd/config.yaml, AGENTS.md, CLAUDE.md and
// .cursor/rules/rtdd.mdc. Actions are returned in installation order.
func Run(repoRoot string) ([]Action, error) {
	var acts []Action

	a, err := EnsureGitAttributes(repoRoot)
	if err != nil {
		return nil, err
	}
	acts = append(acts, a)

	a, err = EnsureConfig(repoRoot)
	if err != nil {
		return nil, err
	}
	acts = append(acts, a)

	for _, rel := range []string{"AGENTS.md", "CLAUDE.md", ".cursor/rules/rtdd.mdc"} {
		a, err = EnsureFrontEnd(repoRoot, rel, Block())
		if err != nil {
			return nil, err
		}
		acts = append(acts, a)
	}
	return acts, nil
}

func readIfExists(repoRoot, rel string) (string, bool, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(b), true, nil
}

func writeFile(repoRoot, rel, content string) error {
	p := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}
```

- [ ] **13.5 — Run it and see it pass.**

```bash
go test ./internal/initrepo/ -v
```

Expected:

```
=== RUN   TestMergeManagedBlockIntoAbsentFile
--- PASS: TestMergeManagedBlockIntoAbsentFile (0.00s)
=== RUN   TestMergeManagedBlockAppendsWithoutClobbering
--- PASS: TestMergeManagedBlockAppendsWithoutClobbering (0.00s)
=== RUN   TestMergeManagedBlockReplacesBetweenMarkersOnly
--- PASS: TestMergeManagedBlockReplacesBetweenMarkersOnly (0.00s)
=== RUN   TestEnsureFrontEndOnExistingAgentsMD
--- PASS: TestEnsureFrontEndOnExistingAgentsMD (0.00s)
=== RUN   TestEnsureGitAttributes
--- PASS: TestEnsureGitAttributes (0.00s)
=== RUN   TestEnsureGitAttributesAppendsToExisting
--- PASS: TestEnsureGitAttributesAppendsToExisting (0.00s)
=== RUN   TestEnsureGitAttributesIdempotent
--- PASS: TestEnsureGitAttributesIdempotent (0.00s)
=== RUN   TestEnsureConfigNeverOverwrites
--- PASS: TestEnsureConfigNeverOverwrites (0.00s)
=== RUN   TestRunInstallsEverything
--- PASS: TestRunInstallsEverything (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/internal/initrepo	0.006s
```

- [ ] **13.6 — Write the CLI test.** Create `cmd/rtdd/init_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/initrepo"
)

func TestRenderInit(t *testing.T) {
	acts := []initrepo.Action{
		{Path: ".gitattributes", Kind: "created"},
		{Path: ".rtdd/config.yaml", Kind: "created"},
		{Path: "AGENTS.md", Kind: "created"},
		{Path: "CLAUDE.md", Kind: "updated"},
		{Path: ".cursor/rules/rtdd.mdc", Kind: "unchanged"},
	}
	got := RenderInit(acts)
	want := "" +
		"rtdd init\n" +
		"  created    .gitattributes\n" +
		"  created    .rtdd/config.yaml\n" +
		"  created    AGENTS.md\n" +
		"  updated    CLAUDE.md\n" +
		"  unchanged  .cursor/rules/rtdd.mdc\n" +
		"\n" +
		"Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.\n"
	if got != want {
		t.Fatalf("RenderInit()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestInitBlockDocumentsTheThreeClasses(t *testing.T) {
	b := initrepo.Block()
	for _, needle := range []string{"import-time", "uncovered", "covered", "This is not a gap"} {
		if !strings.Contains(b, needle) {
			t.Fatalf("agent front-end block is missing %q:\n%s", needle, b)
		}
	}
}
```

- [ ] **13.7 — Run it and see it fail.**

```bash
go test ./cmd/rtdd/ -run 'TestRenderInit|TestInitBlock'
```

Expected:

```
# github.com/VocanicZ/rtdd/cmd/rtdd [github.com/VocanicZ/rtdd/cmd/rtdd.test]
cmd/rtdd/init_test.go:...: undefined: RenderInit
FAIL	github.com/VocanicZ/rtdd/cmd/rtdd [build failed]
```

- [ ] **13.8 — Implement the command.** Create `cmd/rtdd/init.go`:

```go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/VocanicZ/rtdd/internal/initrepo"
)

// RenderInit formats what `rtdd init` installed.
func RenderInit(acts []initrepo.Action) string {
	var b strings.Builder
	b.WriteString("rtdd init\n")
	for _, a := range acts {
		fmt.Fprintf(&b, "  %-9s  %s\n", a.Kind, a.Path)
	}
	b.WriteString("\n")
	b.WriteString("Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.\n")
	return b.String()
}

// cmdInit implements `rtdd init`.
func cmdInit(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: rtdd init")
		return 2
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 3
	}
	acts, err := initrepo.Run(repoRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rtdd: %v\n", err)
		return 2
	}
	fmt.Fprint(os.Stdout, RenderInit(acts))
	return 0
}
```

In `cmd/rtdd/main.go`, add to the subcommand dispatch switch:

```go
	case "init":
		return cmdInit(args)
```

- [ ] **13.9 — Run it and see it pass.**

```bash
go build ./... && go test ./cmd/rtdd/ -v -run 'TestRenderInit|TestInitBlock'
```

Expected:

```
=== RUN   TestRenderInit
--- PASS: TestRenderInit (0.00s)
=== RUN   TestInitBlockDocumentsTheThreeClasses
--- PASS: TestInitBlockDocumentsTheThreeClasses (0.00s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	0.011s
```

- [ ] **13.10 — Add the contract entry.** In `docs/plans/00-interfaces.md`, add
`internal/initrepo/   rtdd init: gitattributes, config, agent front-ends` to the
**Package layout** block (after the `internal/doctor/` line), and add a new section
immediately **after** `## internal/doctor`:

````markdown
## internal/initrepo

```go
type Action struct {
    Path string // repo-relative
    Kind string // "created" | "updated" | "unchanged"
}

const (
    BeginMarker = "<!-- BEGIN RTDD -->"
    EndMarker   = "<!-- END RTDD -->"
)

// MergeManagedBlock returns existing with block installed between the markers.
// Never clobbers: content outside the markers is preserved byte for byte. With no
// markers present, block is appended after a blank line and the original comes first.
func MergeManagedBlock(existing, block string) string

func EnsureGitAttributes(repoRoot string) (Action, error) // adds ".rtdd/map.jsonl merge=union"
func EnsureConfig(repoRoot string) (Action, error)        // never overwrites an existing config
func EnsureFrontEnd(repoRoot, rel, block string) (Action, error)

// Block returns the managed agent front-end text, marker lines included.
func Block() string

// Run installs .gitattributes, .rtdd/config.yaml, AGENTS.md, CLAUDE.md and
// .cursor/rules/rtdd.mdc. Actions are returned in installation order.
func Run(repoRoot string) ([]Action, error)
```
````

- [ ] **13.11 — Commit.**

```bash
git add internal/initrepo cmd/rtdd/init.go cmd/rtdd/init_test.go cmd/rtdd/main.go docs/plans/00-interfaces.md
git commit -m "initrepo: rtdd init installs merge driver, config and agent front-ends

An existing AGENTS.md or CLAUDE.md is merged, never clobbered: content outside
the RTDD markers is preserved byte for byte, and re-running is idempotent."
```

---

## Task 14 — Acceptance: the four M2 guarantees, end to end on real coverage

This task proves the milestone against a real pytest run, not against hand-written
coverage structs. It builds the fixture from §F1, runs pytest with
`COVERAGE_CORE=ctrace --cov-context=test`, reads the resulting `.coverage` through
`coverage.ReadSQLite`, and asserts each guarantee.

**Files:** `cmd/rtdd/acceptance_test.go`

**Interfaces:**

*Consumes* (M1b, unchanged):
```go
func ReadSQLite(dbPath, repoRoot string) (*Result, error)
```

### Steps

- [ ] **14.1 — Write the failing test.** Create `cmd/rtdd/acceptance_test.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/report"
	"github.com/VocanicZ/rtdd/internal/uncovered"
)

// buildPyFixture materialises the §F1 project and runs pytest under coverage with
// dynamic contexts. It returns the repo root and the parsed coverage result.
//
// Measured on 2026-08-26 (Python 3.13.5, coverage.py 7.15.4, pytest 9.0.3) the run
// produces exactly:
//
//	src/__init__.py   ctx=''                                  lines=[0]
//	src/constants.py  ctx=''                                  lines=[1,2,4,7,8,9,12,13,14,15]
//	src/logic.py      ctx=''                                  lines=[1,4,8]
//	src/logic.py      ctx='tests/test_it.py::test_logic|run'  lines=[5]
func buildPyFixture(t *testing.T) (string, *coverage.Result) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	if err := exec.Command(py, "-c", "import pytest, coverage, pytest_cov").Run(); err != nil {
		t.Skip("pytest / coverage / pytest-cov not importable")
	}

	root := t.TempDir()
	files := map[string]string{
		"src/__init__.py":   "",
		"tests/__init__.py": "",
		"pyproject.toml":    "[tool.pytest.ini_options]\npythonpath = [\".\"]\n",
		"src/constants.py": "from dataclasses import dataclass\nfrom enum import Enum\n\n" +
			"MAX_RETRIES = 3\n\n\n" +
			"class Colour(Enum):\n    RED = \"red\"\n    GREEN = \"green\"\n\n\n" +
			"@dataclass\nclass Limits:\n    soft: int = 10\n    hard: int = 20\n",
		"src/logic.py": "from src.constants import MAX_RETRIES\n\n\n" +
			"def retries_left(used):\n    return MAX_RETRIES - used\n\n\n" +
			"def unused_helper(x):\n    return x * 2\n",
		"tests/test_it.py": "from src.constants import MAX_RETRIES, Colour, Limits\n" +
			"from src.logic import retries_left\n\n\n" +
			"def test_constants():\n    assert MAX_RETRIES == 3\n" +
			"    assert Colour.RED.value == \"red\"\n    assert Limits().soft == 10\n\n\n" +
			"def test_logic():\n    assert retries_left(1) == 2\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command(py, "-m", "pytest", "--cov=src", "--cov-context=test", "-q")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "COVERAGE_CORE=ctrace")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pytest failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "2 passed") {
		t.Fatalf("expected 2 passed, got:\n%s", out)
	}

	// Fatal, never skipped: a sysmon run silently drops ~90% of contexts (audit A7).
	if strings.Contains(string(out), "no-sysmon-context") {
		t.Fatalf("dynamic contexts were dropped; COVERAGE_CORE=ctrace was not honoured:\n%s", out)
	}

	cov, err := coverage.ReadSQLite(filepath.Join(root, ".coverage"), root)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	return root, cov
}

func TestAcceptanceImportTimeOnlyFileIsNeverUncovered(t *testing.T) {
	// GUARANTEE 1: a file whose changed lines are all import-time is correctly tested
	// and must produce a clean report. This is audit finding A1 and the reason RTDD
	// is usable on any repo with dataclasses, enums, config modules, Pydantic/Django
	// models, or __init__.py re-exports.
	_, cov := buildPyFixture(t)

	if lines, ok := cov.ImportTime["src/constants.py"]; !ok || len(lines) == 0 {
		t.Fatalf("fixture invariant broken: src/constants.py has no import-time lines: %#v", cov.ImportTime)
	}
	for _, tc := range cov.PerTest {
		if len(tc.Files["src/constants.py"]) != 0 {
			t.Fatalf("fixture invariant broken: %s attributes lines in src/constants.py; "+
				"finding A1 says import-time code is attributed to ZERO test contexts", tc.Test)
		}
	}

	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
	}
	reports := uncovered.Classify(changes, cov)
	if n := uncovered.Summarize(reports).UncoveredLines; n != 0 {
		t.Fatalf("uncovered_lines = %d, want 0 — a dataclass/enum/constants module asserted "+
			"on by two passing tests must produce a clean report\nreports: %#v", n, reports)
	}
	if s := RenderUncovered(reports); strings.Contains(s, "UNCOVERED") {
		t.Fatalf("text report must contain no UNCOVERED line:\n%s", s)
	}
}

func TestAcceptanceClassificationIsLineGranular(t *testing.T) {
	// GUARANTEE 2: adding a function to an already-covered file must report the new
	// function's lines as Uncovered. v1 was file-granular and reported green here.
	_, cov := buildPyFixture(t)

	// src/logic.py IS covered (line 5 belongs to tests/test_it.py::test_logic), yet
	// lines 8-9 are the never-called unused_helper. Line 8 is the `def` (import-time);
	// line 9 is its body, which nothing executes.
	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := uncovered.Classify(changes, cov)
	if len(reports) != 1 {
		t.Fatalf("reports = %#v, want 1 file", reports)
	}
	if n := reports[0].UncoveredLines(); n != 1 {
		t.Fatalf("UncoveredLines() = %d, want 1 (line 9 only)\nranges: %#v", n, reports[0].Ranges)
	}
	want := "  UNCOVERED: src/logic.py:9  (1 changed line, no executing test)\n" +
		"  import-time: src/logic.py:8  (executed during collection, not attributed)\n"
	if got := RenderUncovered(reports); got != want {
		t.Fatalf("RenderUncovered()\n got:\n%s\nwant:\n%s", got, want)
	}

	// And the covered function body in the SAME file is attributed to its test.
	covChanges := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 5, End: 5}}},
	}
	covReports := uncovered.Classify(covChanges, cov)
	if len(covReports) != 1 || len(covReports[0].Ranges) != 1 ||
		covReports[0].Ranges[0].Class != uncovered.Covered {
		t.Fatalf("line 5 must be Covered; got %#v", covReports)
	}
}

func TestAcceptanceUncoveredReportNeverChangesTheExitCode(t *testing.T) {
	// GUARANTEE 3: `rtdd run` exits 1 only when a test fails. Uncovered is a signal.
	_, cov := buildPyFixture(t)

	changes := []gitctx.Change{
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	reports := uncovered.Classify(changes, cov)
	if uncovered.Summarize(reports).UncoveredLines == 0 {
		t.Fatal("fixture invariant broken: the report must be non-empty for this test to mean anything")
	}

	outcomes := []report.Outcome{
		{Test: "tests/test_it.py::test_constants", Status: "pass", DurationMS: 2},
		{Test: "tests/test_it.py::test_logic", Status: "pass", DurationMS: 1},
	}
	if code := ExitCodeFor(outcomes, reports); code != 0 {
		t.Fatalf("ExitCodeFor() = %d, want 0 with a non-empty uncovered report and no failing test", code)
	}

	out := BuildOutput(OutputInput{
		Command: "run", Base: "HEAD", Adapter: "python",
		Executed: true, Outcomes: outcomes,
		Reports: reports, UncoveredOK: true,
		Instrumentable: map[string]bool{"src/logic.py": true},
		Changes:        changes,
	})
	if out.ExitCode != 0 {
		t.Fatalf("json exit_code = %d, want 0", out.ExitCode)
	}
	if out.Uncovered.Summary.UncoveredLines != 1 {
		t.Fatalf("json uncovered_lines = %d, want 1", out.Uncovered.Summary.UncoveredLines)
	}
}

func TestAcceptanceImportOnlyFileIsInNoMapRow(t *testing.T) {
	// GUARANTEE 4: an import-time-only file is in NO map row's f, which is exactly the
	// static-import fallback's trigger condition (spec §6, D14).
	_, cov := buildPyFixture(t)

	keep := func(a, b string) string { return a }
	m := mapstore.New()
	for _, tc := range cov.PerTest {
		var fs []string
		for path := range tc.Files {
			fs = append(fs, path)
		}
		m.Union(mapstore.Row{T: tc.Test, F: fs, C: "aaaaaaa", D: 1, S: "pass"}, keep)
	}

	if got := m.TestsCovering([]string{"src/constants.py"}); len(got) != 0 {
		t.Fatalf("TestsCovering(src/constants.py) = %#v, want empty — import-time lines "+
			"are attributed to no test and therefore enter no row's f", got)
	}

	changes := []gitctx.Change{
		{Path: "src/constants.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 15}}},
		{Path: "src/logic.py", Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 8, End: 9}}},
	}
	sig := BuildSignal(SignalInput{
		Changes:          changes,
		Cov:              cov,
		Map:              m,
		IsInstrumentable: func(rel string) bool { return strings.HasPrefix(rel, "src/") },
	})
	if len(sig.UnmappedFiles) != 1 || sig.UnmappedFiles[0] != "src/constants.py" {
		t.Fatalf("UnmappedFiles = %#v, want [src/constants.py]", sig.UnmappedFiles)
	}
}
```

- [ ] **14.2 — Run it and see it fail (or skip) before the milestone is complete.**

```bash
go test ./cmd/rtdd/ -run TestAcceptance -v
```

If any earlier task is incomplete this fails at compile time with `undefined: BuildSignal`
or `undefined: ExitCodeFor`. With Tasks 1–13 done it must pass:

```
=== RUN   TestAcceptanceImportTimeOnlyFileIsNeverUncovered
--- PASS: TestAcceptanceImportTimeOnlyFileIsNeverUncovered (1.42s)
=== RUN   TestAcceptanceClassificationIsLineGranular
--- PASS: TestAcceptanceClassificationIsLineGranular (1.38s)
=== RUN   TestAcceptanceUncoveredReportNeverChangesTheExitCode
--- PASS: TestAcceptanceUncoveredReportNeverChangesTheExitCode (1.40s)
=== RUN   TestAcceptanceImportOnlyFileIsInNoMapRow
--- PASS: TestAcceptanceImportOnlyFileIsInNoMapRow (1.39s)
PASS
ok  	github.com/VocanicZ/rtdd/cmd/rtdd	5.601s
```

- [ ] **14.3 — Run the whole suite and vet.**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: `ok` for every package; `go vet` silent.

- [ ] **14.4 — Commit.**

```bash
git add cmd/rtdd/acceptance_test.go
git commit -m "cmd: end-to-end acceptance for the four M2 guarantees

Runs real pytest under COVERAGE_CORE=ctrace --cov-context=test, reads the real
.coverage, and asserts: import-time-only files never report uncovered;
classification is line-granular on an already-covered file; a non-empty
uncovered report never changes the exit code; an import-time-only file is in no
map row and therefore triggers the static-import fallback."
```

---

## Definition of Done

- [ ] `go build ./...`, `go vet ./...` and `go test ./...` all pass with zero failures.
- [ ] **Zero non-stdlib dependencies added.** `internal/importscan` shells out to Python; it
      does not link a Python or an AST library.
- [ ] `uncovered.Classify` returns the three classes and **never** labels an import-time line
      `Uncovered` — asserted on real coverage.py output in `TestAcceptanceImportTimeOnlyFileIsNeverUncovered`.
- [ ] A file whose changed lines are all import-time produces an **empty** UNCOVERED section
      in both the text report and `--json`.
- [ ] Classification is **line-granular**: adding a function to an already-covered file
      reports the new body as `Uncovered` — `TestAcceptanceClassificationIsLineGranular`.
- [ ] Classification consumes **only** the fresh post-run `*coverage.Result`. No code path
      reads line numbers from `map.jsonl`, which never contains them.
- [ ] `rtdd run` exits **0** with a non-empty uncovered report — asserted directly in
      `TestExitCodeZeroWithNonEmptyUncoveredReport` and `TestAcceptanceUncoveredReportNeverChangesTheExitCode`.
      `ExitCodeFor` returns 1 if and only if a test failed or errored.
- [ ] All six observed `git diff --unified=0` hunk-header forms parse correctly, including
      the count-omitted single-line form, the count-0 pure deletion, and a context suffix
      containing `@@`.
- [ ] `internal/importscan` selects tests by **transitive** Python import, terminates on
      import cycles, tolerates unparseable files, and satisfies
      `selector.Inputs.ImportOnly func(rel string) []string`.
- [ ] A failed import scan degrades selection; it never fails the command.
- [ ] `rtdd doctor` prints the spec §9 fan-out caveat **in its own output**, naming
      `@lru_cache`, module singletons, DI containers and session-scoped fixtures, and stating
      that the most coupled file can appear as the cleanest — asserted in
      `TestRenderDoctorAlwaysPrintsTheCaveat`, including for an empty map.
- [ ] `rtdd explain <file>` reports zero covering tests as *import-time-only or untested*,
      never as untested alone.
- [ ] `rtdd which` emits `uncovered.available: false` with a reason. It never reports a
      stale line-level signal.
- [ ] `rtdd init` installs `.gitattributes` containing `.rtdd/map.jsonl merge=union`,
      `.rtdd/config.yaml`, `AGENTS.md`, `CLAUDE.md` and `.cursor/rules/rtdd.mdc`.
- [ ] `rtdd init` **merges** into an existing `AGENTS.md`/`CLAUDE.md`: content outside the
      RTDD markers is preserved byte for byte, an existing config is never overwritten, and
      re-running reports `unchanged` and writes nothing.
- [ ] The `--json` schema v1 is implemented, emits no `null` for any collection, and is
      documented in full in `docs/plans/00-interfaces.md`.
- [ ] Every contract addition made by this milestone is present in
      `docs/plans/00-interfaces.md`: `uncovered.ParseHunks`, `uncovered.WithLines`,
      `uncovered.Summary`/`Summarize`, `Class.String`, `gitctx.RawDiff`,
      `internal/importscan` (`Scan`, `Scanner`, `NewScanner`, `TestsImporting`, `Err`),
      `doctor.Caveat`, `internal/initrepo`, and the `--json` schema.
- [ ] Milestone M2's spec §12 row is satisfied: *line-level post-run report with the three
      classes; import-time fallback selection.*
