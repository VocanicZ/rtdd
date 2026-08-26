# RTDD M1a — Core Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and prove the adapter-free core of RTDD — path normalisation, the JSONL map store, git context, and the tier/rank selector — behind `rtdd status` and `rtdd which`, driven by a hand-written fixture map with no subprocess runner and no coverage reading.

**Architecture:** `cmd/rtdd` parses flags and prints; everything else is a pure library. `internal/gitctx` is the only package that shells out (to `git`), `internal/mapstore` owns `.rtdd/map.jsonl` and `.rtdd/meta.json`, and `internal/selector` is a pure function from (map, changes, adapter, config) to a `Selection`. `internal/adapter` exists in M1a only as a YAML-loaded classifier stub (globs: test / opaque / full-escalate / instrumentable) — detection, command expansion, and subprocess execution are M1b.

**Tech Stack:** Go 1.24+, stdlib testing, gopkg.in/yaml.v3

## Global Constraints

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except: `modernc.org/sqlite` (pure-Go, no cgo — required so the binary stays static), and `gopkg.in/yaml.v3`. No test framework beyond stdlib `testing`.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

**Process exit codes for `rtdd`:**

| Code | Meaning |
|---|---|
| 0 | Success. **Includes** an empty selection and a non-empty uncovered report — these are signals, not failures |
| 1 | A test failed during `run`/`verify` |
| 2 | Usage or configuration error (bad flag, unparseable adapter, no adapter detected) |
| 3 | Fatal environment error (`no-sysmon-context` warning observed, `.coverage` unreadable, git unavailable) |

RTDD never exits nonzero to express a policy opinion. See spec §2 non-goals.

**M1a-specific constraints:**

- M1a implements only `internal/paths`, `internal/mapstore`, `internal/gitctx`, `internal/selector`, the classifier half of `internal/adapter`, and `cmd/rtdd` for `status` + `which`.
- No subprocess test runner, no `.coverage` reading, no line classification. `internal/coverage`, `internal/report`, `internal/runner`, `internal/uncovered`, `internal/doctor` are not created in this milestone.
- Git-dependent tests shell out to a real `git init` in `t.TempDir()`. Git is never mocked.

---

## File Structure

| File | Single responsibility |
|---|---|
| `go.mod` | Module `github.com/VocanicZ/rtdd`, Go 1.24, one dependency: `gopkg.in/yaml.v3` |
| `.gitattributes` | Declares `.rtdd/map.jsonl merge=union` for this repo's own dogfooding |
| `.gitignore` | Ignores build output (`/rtdd`, `/dist/bin`) |
| `docs/plans/00-interfaces.md` | **Modified once** (Task 1) to append the M1a contract additions |
| `internal/paths/paths.go` | `Normalize`, `StripModulePrefix` — the only place OS paths become repo-relative slash paths |
| `internal/paths/paths_test.go` | Table tests for `Normalize` and `StripModulePrefix` |
| `internal/paths/glob.go` | `MatchGlob` — `**`-aware glob matching over slash paths |
| `internal/paths/glob_test.go` | Table tests for `MatchGlob` |
| `internal/mapstore/mapstore.go` | `Row`, `Map`, `New`, `Get`, `Len`, `Rows`, `Replace`, `Delete`, `Union`, `TestsCovering`, `FanOut` |
| `internal/mapstore/mapstore_test.go` | Table tests for the in-memory map, including `Union` older-commit resolution |
| `internal/mapstore/io.go` | `Load`, `LoadWith`, `Save` — JSONL parse/emit, duplicate resolution, fatal malformed lines |
| `internal/mapstore/io_test.go` | Duplicate-`t` union, fatal malformed line, trailing-newline emission, compaction round-trip |
| `internal/mapstore/meta.go` | `Meta`, `LoadMeta`, `SaveMeta` — `.rtdd/meta.json` |
| `internal/mapstore/meta_test.go` | Missing-file and round-trip tests for `Meta` |
| `internal/mapstore/testdata/dup.jsonl` | Hand-written map with two lines for the same `t` (post-union-merge shape) |
| `internal/mapstore/testdata/malformed.jsonl` | Hand-written map with one unparseable line |
| `internal/gitctx/gitctx.go` | `Status`, `LineRange`, `Change`, `RepoRoot`, `HeadSHA`, the `git` exec helper |
| `internal/gitctx/changedset.go` | `ChangedSet` — diff + porcelain union, untracked/deleted/renamed handling, hunk parsing |
| `internal/gitctx/history.go` | `CommitDistance`, `IsMergeCommit`, `Older` |
| `internal/gitctx/helper_test.go` | `newRepo`, `run`, `write`, `commit` — real `git init` in `t.TempDir()` |
| `internal/gitctx/gitctx_test.go` | `RepoRoot` / `HeadSHA` tests |
| `internal/gitctx/changedset_test.go` | Untracked, deleted, renamed, line-range tests against a real repo |
| `internal/gitctx/history_test.go` | Distance, unreachable-SHA `-1`, merge detection, `Older` tests |
| `internal/adapter/adapter.go` | `Adapter` struct, `Load`, `IsTestFile`, `IsOpaque`, `IsFullEscalate`, `IsInstrumentable` |
| `internal/adapter/adapter_test.go` | YAML load + classification table tests |
| `internal/adapter/testdata/python.yaml` | Fixture adapter used by adapter, selector, and CLI tests |
| `adapters/python.yaml` | The shipped Python adapter declaration (M1b fills in what it can't yet use) |
| `internal/selector/tier.go` | `Tier`, `Tier.String`, `Config`, `DefaultConfig`, `Selection`, `Inputs` |
| `internal/selector/tier_test.go` | `Tier.String` and `DefaultConfig` tests |
| `internal/selector/rank.go` | `Rank` — the four-key ordering plus a deterministic final tiebreak |
| `internal/selector/rank_test.go` | Table tests for each ranking key in isolation |
| `internal/selector/select.go` | `Select` — direct tier first, then T2 escalation, then T1 escalation, then T0, then empty |
| `internal/selector/select_test.go` | Tier tests including new-test-file direct tier and explicit `TierEmpty` |
| `cmd/rtdd/main.go` | Command dispatch, usage text, exit codes, `loadEnv` |
| `cmd/rtdd/status.go` | `rtdd status` output |
| `cmd/rtdd/which.go` | `rtdd which` human and `--json` output |
| `cmd/rtdd/main_test.go` | End-to-end tests against a real git repo seeded with the fixture map |
| `cmd/rtdd/testdata/map.jsonl` | The hand-written fixture map that drives M1a |
| `cmd/rtdd/testdata/adapter.yaml` | The fixture adapter copied into the test repo's `.rtdd/` |

---

### Task 1: Bootstrap the module and amend the interface contract

**Files:**
- Create: `go.mod`, `.gitattributes`, `.gitignore`
- Modify: `docs/plans/00-interfaces.md`

**Interfaces:**
- Consumes: nothing.
- Produces: module path `github.com/VocanicZ/rtdd`; the M1a contract additions that every later task binds to.

- [ ] **Step 1: Create the module**

Run:
```bash
cd /home/claude/rtdd
go mod init github.com/VocanicZ/rtdd
go mod edit -go=1.24
go get gopkg.in/yaml.v3@v3.0.1
```

- [ ] **Step 2: Create `.gitattributes`**

```
.rtdd/map.jsonl merge=union
```

- [ ] **Step 3: Create `.gitignore`**

```
/rtdd
/dist/bin/
```

- [ ] **Step 4: Append the M1a additions to `docs/plans/00-interfaces.md`**

Append this section verbatim to the end of the file. It is the only contract change M1a makes; every later task binds to these names exactly.

````markdown
---

## M1a amendments

Added by [`01-m1a-core.md`](01-m1a-core.md). These are part of the contract.

### internal/paths

```go
// MatchGlob reports whether rel matches a slash-separated glob pattern.
// "*" and "?" match within one path segment; "**" matches zero or more whole segments.
func MatchGlob(pattern, rel string) bool
```

### internal/mapstore

```go
// LoadWith is Load with an explicit commit-age comparator, used to resolve duplicate
// `t` lines left by a union merge. Load(path) is LoadWith(path, nil); with a nil
// comparator the first line's C wins — deterministic, but age-blind.
func LoadWith(path string, older func(a, b string) string) (*Map, error)

// Meta is .rtdd/meta.json.
type Meta struct {
    V        int    `json:"v"`
    Adapter  string `json:"adapter"`
    SeededAt string `json:"seeded_at"`
    Cycles   int    `json:"cycles"`
}

func LoadMeta(path string) (Meta, error) // missing file returns the zero Meta and a nil error
func SaveMeta(path string, m Meta) error
```

Compaction (`rtdd map compact`) is defined as `LoadWith` followed by `Save`: `LoadWith`
resolves duplicates and `Save` writes exactly one line per `t`. There is no separate
entry point.

### internal/gitctx

```go
// RepoRoot returns the absolute, cleaned top level of the git work tree containing start.
func RepoRoot(start string) (string, error)

// String returns "added" | "modified" | "deleted" | "renamed" | "untracked".
func (s Status) String() string
```

`Untracked` is reserved for callers that need the distinction. Per the `ChangedSet` doc
comment, `ChangedSet` itself reports an untracked file as `Added` with the whole file as
one `LineRange`.

### internal/selector

```go
type Inputs struct {
    // ...existing fields...
    Merge bool // HEAD is a merge commit; escalates to T1 (spec §4: "Merge commits therefore escalate")
}
```

A zero `Config` (every field zero) is treated as `DefaultConfig()`.

### CLI surface

```
rtdd status [--adapter <path>]
rtdd which  [--base <ref>] [--json] [--adapter <path>]
```

`--adapter` defaults to `.rtdd/adapter.yaml`. It exists because `adapter.Detect` and
`adapter.LoadAll` are M1b; detection replaces the default in M1b and the flag stays as an
override. `rtdd which --json` output is **provisional in M1a**; plan M2 task "JSON output"
freezes the schema.
````

- [ ] **Step 5: Verify the module builds**

Run: `go build ./... && go vet ./...`
Expected: no output (no packages yet, no errors).

- [ ] **Step 6: Commit**

```bash
cd /home/claude/rtdd
git add go.mod go.sum .gitattributes .gitignore docs/plans/00-interfaces.md
git commit -m "M1a: bootstrap module and amend the interface contract"
```

---

### Task 2: internal/paths — Normalize and StripModulePrefix

**Files:**
- Create: `internal/paths/paths.go`
- Test: `internal/paths/paths_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func Normalize(repoRoot, p string) (rel string, ok bool)`
  - `func StripModulePrefix(modulePath, p string) string`

- [ ] **Step 1: Write the failing test**

Create `internal/paths/paths_test.go`:

```go
package paths

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		root    string
		in      string
		wantRel string
		wantOK  bool
	}{
		{"absolute inside repo", "/repo", "/repo/src/auth.py", "src/auth.py", true},
		{"already relative", "/repo", "src/auth.py", "src/auth.py", true},
		{"needs cleaning", "/repo", "./src/../src/auth.py", "src/auth.py", true},
		{"nested", "/repo", "/repo/tests/unit/test_a.py", "tests/unit/test_a.py", true},
		{"trailing slash on root", "/repo/", "/repo/src/auth.py", "src/auth.py", true},
		{"repo root itself is not a file", "/repo", "/repo", "", false},
		{"escapes via absolute path", "/repo", "/etc/passwd", "", false},
		{"escapes via dotdot", "/repo", "../outside.py", "", false},
		{"sibling directory prefix is not inside", "/repo", "/repo-other/a.py", "", false},
		{"empty path", "/repo", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRel, gotOK := Normalize(tc.root, tc.in)
			if gotRel != tc.wantRel || gotOK != tc.wantOK {
				t.Errorf("Normalize(%q, %q) = (%q, %v), want (%q, %v)",
					tc.root, tc.in, gotRel, gotOK, tc.wantRel, tc.wantOK)
			}
		})
	}
}

func TestStripModulePrefix(t *testing.T) {
	tests := []struct {
		name string
		mod  string
		in   string
		want string
	}{
		{"strips module prefix", "github.com/VocanicZ/rtdd", "github.com/VocanicZ/rtdd/internal/paths/paths.go", "internal/paths/paths.go"},
		{"module with trailing slash", "github.com/VocanicZ/rtdd/", "github.com/VocanicZ/rtdd/main.go", "main.go"},
		{"leaves unrelated path alone", "github.com/VocanicZ/rtdd", "vendor/x/y.go", "vendor/x/y.go"},
		{"empty module is identity", "", "internal/paths/paths.go", "internal/paths/paths.go"},
		{"prefix match must be on a segment boundary", "github.com/VocanicZ/rtdd", "github.com/VocanicZ/rtdd-other/a.go", "github.com/VocanicZ/rtdd-other/a.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StripModulePrefix(tc.mod, tc.in); got != tc.want {
				t.Errorf("StripModulePrefix(%q, %q) = %q, want %q", tc.mod, tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/... -run 'TestNormalize|TestStripModulePrefix' -v`
Expected: FAIL — `internal/paths/paths_test.go:12:23: undefined: Normalize` and `undefined: StripModulePrefix` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/paths/paths.go`:

```go
// Package paths is the single boundary where operating-system paths become
// repo-relative, slash-separated, cleaned paths. Nothing else in the engine may
// convert a path ad hoc.
package paths

import (
	"path/filepath"
	"strings"
)

// Normalize converts an absolute or runner-relative path to a cleaned,
// slash-separated path relative to repoRoot. Returns ok=false if p escapes repoRoot.
func Normalize(repoRoot, p string) (rel string, ok bool) {
	if p == "" {
		return "", false
	}
	root := filepath.Clean(repoRoot)
	q := filepath.FromSlash(p)
	if !filepath.IsAbs(q) {
		q = filepath.Join(root, q)
	}
	r, err := filepath.Rel(root, filepath.Clean(q))
	if err != nil {
		return "", false
	}
	r = filepath.ToSlash(r)
	if r == "." || r == ".." || strings.HasPrefix(r, "../") {
		return "", false
	}
	return r, true
}

// StripModulePrefix removes a Go module path prefix. Unused in M1; present so the
// Go adapter (deferred) has a defined home.
func StripModulePrefix(modulePath, p string) string {
	if modulePath == "" {
		return p
	}
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	if strings.HasPrefix(p, prefix) {
		return strings.TrimPrefix(p, prefix)
	}
	return p
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/paths/... -run 'TestNormalize|TestStripModulePrefix' -v`
Expected: PASS, 15 subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/paths/paths.go internal/paths/paths_test.go
git commit -m "M1a: paths.Normalize and paths.StripModulePrefix"
```

---

### Task 3: internal/paths — MatchGlob

**Files:**
- Create: `internal/paths/glob.go`
- Test: `internal/paths/glob_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func MatchGlob(pattern, rel string) bool` (added to `00-interfaces.md` in Task 1).

Why this exists: `path.Match` does not understand `**`, and every adapter glob in the spec
(`tests/**/*.py`, `**/test_*.py`, `**/fixtures/**`) needs it.

- [ ] **Step 1: Write the failing test**

Create `internal/paths/glob_test.go`:

```go
package paths

import "testing"

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		rel     string
		want    bool
	}{
		{"exact literal", "pyproject.toml", "pyproject.toml", true},
		{"literal does not match nested", "pyproject.toml", "sub/pyproject.toml", false},
		{"star within a segment", "src/*.py", "src/auth.py", true},
		{"star does not cross a separator", "src/*.py", "src/pkg/auth.py", false},
		{"question mark within a segment", "src/a?.py", "src/ab.py", true},
		{"doublestar spans many segments", "tests/**/*.py", "tests/unit/api/test_a.py", true},
		{"doublestar spans zero segments", "tests/**/*.py", "tests/test_a.py", true},
		{"leading doublestar", "**/test_*.py", "a/b/test_x.py", true},
		{"leading doublestar at root", "**/test_*.py", "test_x.py", true},
		{"leading doublestar wrong basename", "**/test_*.py", "a/b/helper.py", false},
		{"doublestar suffix matches subtree", "**/fixtures/**", "tests/fixtures/data/a.json", true},
		{"doublestar suffix wrong directory", "**/fixtures/**", "tests/unit/a.json", false},
		{"extension glob at root", "**/*.yaml", "config.yaml", true},
		{"extension glob nested", "**/*.yaml", "deploy/k8s/config.yaml", true},
		{"extension glob wrong extension", "**/*.yaml", "deploy/k8s/config.json", false},
		{"conftest anywhere", "**/conftest.py", "tests/unit/conftest.py", true},
		{"pattern longer than path", "a/b/c", "a/b", false},
		{"path longer than pattern", "a/b", "a/b/c", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchGlob(tc.pattern, tc.rel); got != tc.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.rel, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/... -run TestMatchGlob -v`
Expected: FAIL — `internal/paths/glob_test.go:26:15: undefined: MatchGlob` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/paths/glob.go`:

```go
package paths

import (
	"path"
	"strings"
)

// MatchGlob reports whether rel matches a slash-separated glob pattern.
// "*" and "?" match within one path segment; "**" matches zero or more whole segments.
func MatchGlob(pattern, rel string) bool {
	if pattern == "" || rel == "" {
		return false
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/paths/... -v`
Expected: PASS, all subtests including the 15 from Task 2.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/paths/glob.go internal/paths/glob_test.go
git commit -m "M1a: paths.MatchGlob with doublestar support"
```

---

### Task 4: internal/mapstore — Row, Map, and the simple accessors

**Files:**
- Create: `internal/mapstore/mapstore.go`
- Test: `internal/mapstore/mapstore_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Row struct { T string; F []string; C string; D int; S string }`
  - `type Map struct { /* unexported */ }`
  - `func New() *Map`
  - `func (m *Map) Get(t string) (Row, bool)`
  - `func (m *Map) Len() int`
  - `func (m *Map) Rows() []Row`
  - `func (m *Map) Replace(r Row)`
  - `func (m *Map) Delete(t string)`

- [ ] **Step 1: Write the failing test**

Create `internal/mapstore/mapstore_test.go`:

```go
package mapstore

import (
	"reflect"
	"testing"
)

func TestMapAccessors(t *testing.T) {
	m := New()
	if m.Len() != 0 {
		t.Fatalf("New().Len() = %d, want 0", m.Len())
	}
	if _, ok := m.Get("nope"); ok {
		t.Fatalf("Get on an empty map returned ok=true")
	}

	m.Replace(Row{T: "tests/test_b.py::test_x", F: []string{"src/b.py"}, C: "bbb1111", D: 5, S: "pass"})
	m.Replace(Row{T: "tests/test_a.py::test_y", F: []string{"src/a.py"}, C: "aaa2222", D: 9, S: "fail"})

	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", m.Len())
	}
	got := m.Rows()
	wantOrder := []string{"tests/test_a.py::test_y", "tests/test_b.py::test_x"}
	for i, w := range wantOrder {
		if got[i].T != w {
			t.Errorf("Rows()[%d].T = %q, want %q (Rows must be sorted by T ascending)", i, got[i].T, w)
		}
	}

	r, ok := m.Get("tests/test_a.py::test_y")
	if !ok {
		t.Fatalf("Get missed a row that Replace inserted")
	}
	if r.S != "fail" || r.D != 9 || r.C != "aaa2222" {
		t.Errorf("Get returned %+v, want S=fail D=9 C=aaa2222", r)
	}

	m.Delete("tests/test_a.py::test_y")
	if m.Len() != 1 {
		t.Errorf("after Delete, Len() = %d, want 1", m.Len())
	}
}

func TestReplaceNormalizesF(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		wantF []string
	}{
		{"sorts", []string{"src/z.py", "src/a.py"}, []string{"src/a.py", "src/z.py"}},
		{"dedupes", []string{"src/a.py", "src/a.py", "src/b.py"}, []string{"src/a.py", "src/b.py"}},
		{"nil becomes empty, never null", nil, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Replace(Row{T: "t", F: tc.in, C: "c", D: 1, S: "pass"})
			got, _ := m.Get("t")
			if !reflect.DeepEqual(got.F, tc.wantF) {
				t.Errorf("F = %#v, want %#v", got.F, tc.wantF)
			}
		})
	}
}

func TestReplaceDoesNotAliasCallerSlice(t *testing.T) {
	f := []string{"src/b.py", "src/a.py"}
	m := New()
	m.Replace(Row{T: "t", F: f, C: "c", D: 1, S: "pass"})
	f[0] = "MUTATED"
	got, _ := m.Get("t")
	for _, s := range got.F {
		if s == "MUTATED" {
			t.Fatalf("Replace stored the caller's slice by reference: %#v", got.F)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run 'TestMapAccessors|TestReplace' -v`
Expected: FAIL — `internal/mapstore/mapstore_test.go:9:7: undefined: New` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/mapstore/mapstore.go`:

```go
// Package mapstore owns .rtdd/map.jsonl and .rtdd/meta.json: the committed,
// union-merged, file-level test-to-source relation.
package mapstore

import "sort"

// Row is one line of map.jsonl: one test, and the source files it executed.
type Row struct {
	T string   `json:"t"` // test id, exactly as the runner accepts it as a selector
	F []string `json:"f"` // repo-relative source files, sorted, deduped
	C string   `json:"c"` // short SHA of HEAD when recorded
	D int      `json:"d"` // last duration, ms
	S string   `json:"s"` // last outcome: "pass" | "fail" | "skip" | "error"
}

// Map is an in-memory map.jsonl keyed by test id.
type Map struct {
	rows map[string]Row
}

func New() *Map { return &Map{rows: make(map[string]Row)} }

func (m *Map) Get(t string) (Row, bool) {
	r, ok := m.rows[t]
	return r, ok
}

func (m *Map) Len() int { return len(m.rows) }

// Rows returns every row sorted by T ascending.
func (m *Map) Rows() []Row {
	out := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// Replace overwrites the row wholesale. ONLY rtdd seed may call this.
func (m *Map) Replace(r Row) { m.rows[r.T] = normalizeRow(r) }

func (m *Map) Delete(t string) { delete(m.rows, t) }

// normalizeRow copies F, sorts it, dedupes it, and guarantees a non-nil slice so the
// JSONL never emits `"f":null`.
func normalizeRow(r Row) Row {
	f := append([]string(nil), r.F...)
	sort.Strings(f)
	out := make([]string, 0, len(f))
	for i, s := range f {
		if i > 0 && s == f[i-1] {
			continue
		}
		out = append(out, s)
	}
	r.F = out
	return r
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -run 'TestMapAccessors|TestReplace' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/mapstore.go internal/mapstore/mapstore_test.go
git commit -m "M1a: mapstore Row/Map and accessors"
```

---

### Task 5: internal/mapstore — Union with older-commit resolution

**Files:**
- Modify: `internal/mapstore/mapstore.go`
- Test: `internal/mapstore/mapstore_test.go` (append)

**Interfaces:**
- Consumes: `type Row`, `type Map`, `func New() *Map`, `func (m *Map) Get(t string) (Row, bool)`, `normalizeRow`.
- Produces: `func (m *Map) Union(r Row, older func(a, b string) string)`

Why this is load-bearing: spec D11 — `f` unions, never replaces, outside `seed`. A subset run
legitimately records *less* coverage than the seed run, and a failing test records a truncated
prefix of its real path. Replacing on those runs silently narrowed rows in v1 with no merge
involved.

- [ ] **Step 1: Write the failing test**

Append to `internal/mapstore/mapstore_test.go`:

```go
// olderLexical is a deterministic stand-in for gitctx.Older in unit tests:
// the lexically smaller sha is treated as the older one.
func olderLexical(a, b string) string {
	if a <= b {
		return a
	}
	return b
}

func TestUnion(t *testing.T) {
	tests := []struct {
		name  string
		seed  []Row
		apply Row
		wantF []string
		wantC string
		wantD int
		wantS string
	}{
		{
			name:  "insert into an empty map",
			seed:  nil,
			apply: Row{T: "t1", F: []string{"src/b.py", "src/a.py"}, C: "ccc", D: 10, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py"},
			wantC: "ccc", wantD: 10, wantS: "pass",
		},
		{
			name:  "F is the set union, never a replacement",
			seed:  []Row{{T: "t1", F: []string{"src/a.py", "src/db.py"}, C: "bbb", D: 10, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "ccc", D: 3, S: "pass"},
			wantF: []string{"src/a.py", "src/db.py"},
			wantC: "bbb", wantD: 3, wantS: "pass",
		},
		{
			name:  "D and S take the new row's values",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "bbb", D: 999, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "ccc", D: 7, S: "fail"},
			wantF: []string{"src/a.py"},
			wantC: "bbb", wantD: 7, wantS: "fail",
		},
		{
			name:  "C takes the older commit even when the new row is older",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "zzz", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/b.py"}, C: "aaa", D: 2, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py"},
			wantC: "aaa", wantD: 2, wantS: "pass",
		},
		{
			name:  "an empty incoming C keeps the existing one",
			seed:  []Row{{T: "t1", F: []string{"src/a.py"}, C: "bbb", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/a.py"}, C: "", D: 2, S: "pass"},
			wantF: []string{"src/a.py"},
			wantC: "bbb", wantD: 2, wantS: "pass",
		},
		{
			name:  "duplicate files across both sides collapse",
			seed:  []Row{{T: "t1", F: []string{"src/a.py", "src/b.py"}, C: "bbb", D: 1, S: "pass"}},
			apply: Row{T: "t1", F: []string{"src/b.py", "src/c.py"}, C: "bbb", D: 2, S: "pass"},
			wantF: []string{"src/a.py", "src/b.py", "src/c.py"},
			wantC: "bbb", wantD: 2, wantS: "pass",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			for _, r := range tc.seed {
				m.Replace(r)
			}
			m.Union(tc.apply, olderLexical)
			got, ok := m.Get("t1")
			if !ok {
				t.Fatalf("Union did not insert the row")
			}
			if !reflect.DeepEqual(got.F, tc.wantF) {
				t.Errorf("F = %#v, want %#v", got.F, tc.wantF)
			}
			if got.C != tc.wantC {
				t.Errorf("C = %q, want %q", got.C, tc.wantC)
			}
			if got.D != tc.wantD {
				t.Errorf("D = %d, want %d", got.D, tc.wantD)
			}
			if got.S != tc.wantS {
				t.Errorf("S = %q, want %q", got.S, tc.wantS)
			}
		})
	}
}

func TestUnionNilComparatorKeepsExistingC(t *testing.T) {
	m := New()
	m.Replace(Row{T: "t1", F: []string{"src/a.py"}, C: "first", D: 1, S: "pass"})
	m.Union(Row{T: "t1", F: []string{"src/b.py"}, C: "second", D: 2, S: "pass"}, nil)
	got, _ := m.Get("t1")
	if got.C != "first" {
		t.Errorf("C = %q, want %q (a nil comparator must be deterministic: first wins)", got.C, "first")
	}
	if !reflect.DeepEqual(got.F, []string{"src/a.py", "src/b.py"}) {
		t.Errorf("F = %#v, want the union", got.F)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run TestUnion -v`
Expected: FAIL — `m.Union undefined (type *Map has no field or method Union)` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Append to `internal/mapstore/mapstore.go`:

```go
// Union merges r into the map: F becomes the set-union, D and S take r's values,
// and C takes the OLDER of the two commits (a row's F is only as trustworthy as its
// stalest component). Used by every path except seed.
func (m *Map) Union(r Row, older func(a, b string) string) {
	r = normalizeRow(r)
	prev, ok := m.rows[r.T]
	if !ok {
		m.rows[r.T] = r
		return
	}
	m.rows[r.T] = Row{
		T: r.T,
		F: unionStrings(prev.F, r.F),
		C: pickOlder(prev.C, r.C, older),
		D: r.D,
		S: r.S,
	}
}

// pickOlder resolves C. With a nil comparator the existing value wins, which is
// deterministic but age-blind; callers that have git available pass gitctx.Older.
func pickOlder(existing, incoming string, older func(a, b string) string) string {
	switch {
	case existing == "":
		return incoming
	case incoming == "":
		return existing
	case existing == incoming:
		return existing
	case older == nil:
		return existing
	}
	return older(existing, incoming)
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, group := range [][]string{a, b} {
		for _, s := range group {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -run TestUnion -v`
Expected: PASS, 7 subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/mapstore.go internal/mapstore/mapstore_test.go
git commit -m "M1a: mapstore.Union — set-union F, older commit wins for C"
```

---

### Task 6: internal/mapstore — Save with a mandatory trailing newline

**Files:**
- Create: `internal/mapstore/io.go`
- Test: `internal/mapstore/io_test.go`

**Interfaces:**
- Consumes: `type Row`, `type Map`, `func New() *Map`, `func (m *Map) Rows() []Row`, `func (m *Map) Replace(r Row)`.
- Produces: `func (m *Map) Save(path string) error`

Why the trailing newline is load-bearing: `.rtdd/map.jsonl` is `merge=union`. If the last line
has no `\n`, the union driver concatenates the last row of one side with the first row of the
other, producing a single unparseable line — and `Load` treats a malformed line as fatal, so
the whole map dies.

- [ ] **Step 1: Write the failing test**

Create `internal/mapstore/io_test.go`:

```go
package mapstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveEmitsSortedLinesWithTrailingNewline(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_b.py::test_x", F: []string{"src/b.py"}, C: "bbb1111", D: 5, S: "pass"})
	m.Replace(Row{T: "tests/test_a.py::test_y", F: []string{"src/db.py", "src/a.py"}, C: "aaa2222", D: 9, S: "fail"})

	path := filepath.Join(t.TempDir(), "nested", "map.jsonl")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(b)

	want := `{"t":"tests/test_a.py::test_y","f":["src/a.py","src/db.py"],"c":"aaa2222","d":9,"s":"fail"}
{"t":"tests/test_b.py::test_x","f":["src/b.py"],"c":"bbb1111","d":5,"s":"pass"}
`
	if got != want {
		t.Errorf("Save wrote:\n%q\nwant:\n%q", got, want)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Save MUST end with a trailing newline: a missing one joins two rows on the next union merge")
	}
}

func TestSaveEmptyMapWritesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "map.jsonl")
	if err := New().Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(b) != 0 {
		t.Errorf("empty map wrote %q, want an empty file", string(b))
	}
}

func TestSaveEmitsEmptyArrayNotNullForF(t *testing.T) {
	m := New()
	m.Replace(Row{T: "t1", F: nil, C: "aaa", D: 1, S: "pass"})
	path := filepath.Join(t.TempDir(), "map.jsonl")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), `"f":null`) {
		t.Errorf("Save emitted %q; F must serialise as [] so the row stays machine-readable", string(b))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run TestSave -v`
Expected: FAIL — `m.Save undefined (type *Map has no field or method Save)` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/mapstore/io.go`:

```go
package mapstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save writes the map as JSONL sorted by T ascending. Every line, including the last,
// is terminated with "\n": map.jsonl is merged with the union driver, and a missing
// final newline joins the last row of one side to the first row of the other, producing
// a line no parser can read.
func (m *Map) Save(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mapstore: mkdir %s: %w", dir, err)
		}
	}
	var buf bytes.Buffer
	for _, r := range m.Rows() {
		b, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("mapstore: marshal %q: %w", r.T, err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("mapstore: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("mapstore: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -run TestSave -v`
Expected: PASS, 3 tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/io.go internal/mapstore/io_test.go
git commit -m "M1a: mapstore.Save with a mandatory trailing newline"
```

---

### Task 7: internal/mapstore — Load and LoadWith, duplicate-tolerant and fatal on malformed

**Files:**
- Modify: `internal/mapstore/io.go`
- Create: `internal/mapstore/testdata/dup.jsonl`, `internal/mapstore/testdata/malformed.jsonl`
- Test: `internal/mapstore/io_test.go` (append)

**Interfaces:**
- Consumes: `type Row`, `type Map`, `func New() *Map`, `func (m *Map) Union(r Row, older func(a, b string) string)`, `func (m *Map) Get(t string) (Row, bool)`.
- Produces:
  - `func Load(path string) (*Map, error)` — missing file returns an empty Map and nil error
  - `func LoadWith(path string, older func(a, b string) string) (*Map, error)`

Why this is load-bearing: `merge=union` leaves **two lines with the same `t`** whenever two
agents touched the same test. `Load` must resolve them exactly as `Union` does — set-union of
`F`, older commit for `C`. And a malformed line is a **fatal error, never a silent skip**:
silently dropping a row narrows selection, which is the one failure mode selection must not
have.

- [ ] **Step 1: Write the failing test**

Create `internal/mapstore/testdata/dup.jsonl` (note the deliberate duplicate `t`, and that
the *newer* commit `ccc3333` appears on the second line):

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"aaa1111","d":412,"s":"pass"}
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/session.py"],"c":"ccc3333","d":88,"s":"fail"}
{"t":"tests/test_db.py::test_query","f":["src/db.py"],"c":"bbb2222","d":15,"s":"pass"}
```

Create `internal/mapstore/testdata/malformed.jsonl` (line 2 is what a missing trailing
newline produces after a union merge — two rows joined into one line):

```
{"t":"tests/test_a.py::test_x","f":["src/a.py"],"c":"aaa1111","d":1,"s":"pass"}
{"t":"tests/test_b.py::test_y","f":["src/b.py"],"c":"bbb2222","d":2,"s":"pass"}{"t":"tests/test_c.py::test_z","f":["src/c.py"],"c":"ccc3333","d":3,"s":"pass"}
```

Append to `internal/mapstore/io_test.go`:

```go
func TestLoadMissingFileIsEmptyAndNotAnError(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("Load of a missing file returned %v, want nil", err)
	}
	if m == nil || m.Len() != 0 {
		t.Fatalf("Load of a missing file must return an empty Map, got %+v", m)
	}
}

func TestLoadResolvesDuplicateTLinesLikeUnion(t *testing.T) {
	m, err := LoadWith("testdata/dup.jsonl", olderLexical)
	if err != nil {
		t.Fatalf("LoadWith: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (the two lines for the same t must collapse)", m.Len())
	}
	got, ok := m.Get("tests/test_auth.py::test_login")
	if !ok {
		t.Fatalf("the duplicated test id is missing from the map")
	}
	wantF := []string{"src/auth.py", "src/db.py", "src/session.py"}
	if !reflect.DeepEqual(got.F, wantF) {
		t.Errorf("F = %#v, want %#v (union of both lines; a union merge may never narrow)", got.F, wantF)
	}
	if got.C != "aaa1111" {
		t.Errorf("C = %q, want %q (the OLDER commit wins)", got.C, "aaa1111")
	}
	if got.D != 88 || got.S != "fail" {
		t.Errorf("D/S = %d/%q, want 88/fail (the last line's values win)", got.D, got.S)
	}
}

func TestLoadIsFatalOnAMalformedLine(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantIn  string
	}{
		{"not json at all", "this is not json\n", "line 1"},
		{"row with no t", "{\"f\":[\"src/a.py\"],\"c\":\"aaa\",\"d\":1,\"s\":\"pass\"}\n", "empty"},
		{"truncated json", "{\"t\":\"t1\",\"f\":[\"src/a.py\"\n", "line 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "map.jsonl")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			m, err := Load(p)
			if err == nil {
				t.Fatalf("Load returned nil error and %d rows; a malformed line MUST be fatal, never a silent skip", m.Len())
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.wantIn)
			}
		})
	}
}

func TestLoadIsFatalOnTwoRowsJoinedByAMissingNewline(t *testing.T) {
	m, err := Load("testdata/malformed.jsonl")
	if err == nil {
		t.Fatalf("Load returned nil error and %d rows; two rows joined on one line MUST be fatal", m.Len())
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q does not point at line 2", err.Error())
	}
}

func TestLoadSkipsBlankLines(t *testing.T) {
	p := filepath.Join(t.TempDir(), "map.jsonl")
	content := "{\"t\":\"t1\",\"f\":[\"src/a.py\"],\"c\":\"aaa\",\"d\":1,\"s\":\"pass\"}\n\n   \n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Len() != 1 {
		t.Errorf("Len() = %d, want 1", m.Len())
	}
}
```

Add `"reflect"` to the imports of `io_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run TestLoad -v`
Expected: FAIL — `internal/mapstore/io_test.go:...: undefined: Load` and `undefined: LoadWith` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Append to `internal/mapstore/io.go`:

```go
// Load reads map.jsonl. A missing file returns an empty Map and a nil error.
// Duplicate `t` lines are resolved with a nil comparator (first line's C wins).
func Load(path string) (*Map, error) { return LoadWith(path, nil) }

// LoadWith reads map.jsonl, resolving duplicate `t` lines exactly as Union does:
// F is set-unioned and C takes the older commit per the comparator. Duplicates are
// expected — the union merge driver leaves both lines whenever two agents touched the
// same test. A malformed line is a FATAL error, never a silent skip: dropping a row
// narrows selection, and selection may never silently narrow.
func LoadWith(path string, older func(a, b string) string) (*Map, error) {
	m := New()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("mapstore: open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		var r Row
		if err := dec.Decode(&r); err != nil {
			return nil, fmt.Errorf("mapstore: %s line %d: malformed row: %w", path, lineNo, err)
		}
		if dec.More() {
			return nil, fmt.Errorf("mapstore: %s line %d: trailing content after the row "+
				"(two rows joined by a missing trailing newline?)", path, lineNo)
		}
		if r.T == "" {
			return nil, fmt.Errorf("mapstore: %s line %d: row has an empty \"t\"", path, lineNo)
		}
		m.Union(r, older)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mapstore: read %s: %w", path, err)
	}
	return m, nil
}
```

Add `"bufio"` and `"strings"` to the imports of `io.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -v`
Expected: PASS, all mapstore tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/io.go internal/mapstore/io_test.go internal/mapstore/testdata
git commit -m "M1a: mapstore.Load tolerates duplicate t, dies on malformed lines"
```

---

### Task 8: internal/mapstore — compaction round-trip

**Files:**
- Modify: none (proves existing behaviour)
- Test: `internal/mapstore/io_test.go` (append)

**Interfaces:**
- Consumes: `func LoadWith(path string, older func(a, b string) string) (*Map, error)`, `func (m *Map) Save(path string) error`.
- Produces: nothing new. This task discharges the spec §12 M1a requirement that *compaction* be provable, and pins `rtdd map compact` to `LoadWith` + `Save`.

- [ ] **Step 1: Write the failing test**

Append to `internal/mapstore/io_test.go`:

```go
// Compaction (rtdd map compact) is LoadWith followed by Save: LoadWith resolves the
// duplicate lines a union merge leaves behind, Save writes exactly one line per t.
func TestCompactionRoundTrip(t *testing.T) {
	src, err := os.ReadFile("testdata/dup.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := os.WriteFile(p, src, 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadWith(p, olderLexical)
	if err != nil {
		t.Fatalf("LoadWith: %v", err)
	}
	if err := m.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py","src/session.py"],"c":"aaa1111","d":88,"s":"fail"}
{"t":"tests/test_db.py::test_query","f":["src/db.py"],"c":"bbb2222","d":15,"s":"pass"}
`
	if string(b) != want {
		t.Errorf("compaction produced:\n%s\nwant:\n%s", string(b), want)
	}

	// Compaction is idempotent: a second round trip is a no-op.
	m2, err := LoadWith(p, olderLexical)
	if err != nil {
		t.Fatalf("second LoadWith: %v", err)
	}
	if err := m2.Save(p); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	b2, _ := os.ReadFile(p)
	if string(b2) != string(b) {
		t.Errorf("compaction is not idempotent:\nfirst:\n%s\nsecond:\n%s", string(b), string(b2))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run TestCompactionRoundTrip -v`
Expected: PASS if Tasks 5-7 are correct. If it FAILS, the failure is real — the most likely
messages are `compaction produced: ...` with a duplicated `t` line (Load did not resolve
duplicates) or a missing final `\n` (Save regression). Fix the implementation, not the test.

- [ ] **Step 3: Write minimal implementation**

No production code. If Step 2 failed, the fix belongs in `Load`/`Union`/`Save` from Tasks 5-7.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/io_test.go
git commit -m "M1a: prove compaction is LoadWith+Save and is idempotent"
```

---

### Task 9: internal/mapstore — TestsCovering and FanOut

**Files:**
- Modify: `internal/mapstore/mapstore.go`
- Test: `internal/mapstore/mapstore_test.go` (append)

**Interfaces:**
- Consumes: `type Row`, `type Map`, `func New() *Map`, `func (m *Map) Replace(r Row)`.
- Produces:
  - `func (m *Map) TestsCovering(files []string) []string` — unsorted
  - `func (m *Map) FanOut() map[string]int`

- [ ] **Step 1: Write the failing test**

Append to `internal/mapstore/mapstore_test.go`:

```go
func fixtureMap() *Map {
	m := New()
	m.Replace(Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "aaa1111", D: 412, S: "pass"})
	m.Replace(Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "aaa1111", D: 90, S: "fail"})
	m.Replace(Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "aaa1111", D: 15, S: "pass"})
	m.Replace(Row{T: "tests/test_render.py::test_page", F: []string{"src/render.py", "templates/page.html"}, C: "aaa1111", D: 230, S: "pass"})
	return m
}

func TestTestsCovering(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{"single file, two tests", []string{"src/auth.py"},
			[]string{"tests/test_auth.py::test_login", "tests/test_auth.py::test_logout"}},
		{"single file, one test", []string{"src/render.py"},
			[]string{"tests/test_render.py::test_page"}},
		{"two files union without duplicating a test", []string{"src/auth.py", "src/db.py"},
			[]string{"tests/test_auth.py::test_login", "tests/test_auth.py::test_logout", "tests/test_db.py::test_query"}},
		{"unknown file selects nothing", []string{"src/brand_new.py"}, []string{}},
		{"no files selects nothing", nil, []string{}},
		{"a deleted path still selects its tests", []string{"templates/page.html"},
			[]string{"tests/test_render.py::test_page"}},
	}
	m := fixtureMap()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.TestsCovering(tc.files)
			sort.Strings(got)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TestsCovering(%v) = %#v, want %#v", tc.files, got, tc.want)
			}
		})
	}
}

func TestFanOut(t *testing.T) {
	got := fixtureMap().FanOut()
	want := map[string]int{
		"src/auth.py":         2,
		"src/db.py":           2,
		"src/render.py":       1,
		"templates/page.html": 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FanOut() = %#v, want %#v", got, want)
	}
}
```

Add `"sort"` to the imports of `mapstore_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run 'TestTestsCovering|TestFanOut' -v`
Expected: FAIL — `m.TestsCovering undefined (type *Map has no field or method TestsCovering)` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Append to `internal/mapstore/mapstore.go`:

```go
// TestsCovering returns every test id whose F intersects any of files. Unsorted.
func (m *Map) TestsCovering(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(files))
	for _, f := range files {
		want[f] = struct{}{}
	}
	out := make([]string, 0, len(m.rows))
	for t, r := range m.rows {
		for _, f := range r.F {
			if _, hit := want[f]; hit {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

// FanOut returns file -> number of tests whose F contains it.
func (m *Map) FanOut() map[string]int {
	out := make(map[string]int)
	for _, r := range m.rows {
		for _, f := range r.F {
			out[f]++
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/mapstore.go internal/mapstore/mapstore_test.go
git commit -m "M1a: mapstore.TestsCovering and mapstore.FanOut"
```

---

### Task 10: internal/mapstore — Meta

**Files:**
- Create: `internal/mapstore/meta.go`
- Test: `internal/mapstore/meta_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (added to `00-interfaces.md` in Task 1):
  - `type Meta struct { V int; Adapter string; SeededAt string; Cycles int }`
  - `func LoadMeta(path string) (Meta, error)`
  - `func SaveMeta(path string, m Meta) error`

- [ ] **Step 1: Write the failing test**

Create `internal/mapstore/meta_test.go`:

```go
package mapstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMetaMissingFileIsZeroAndNotAnError(t *testing.T) {
	got, err := LoadMeta(filepath.Join(t.TempDir(), "meta.json"))
	if err != nil {
		t.Fatalf("LoadMeta of a missing file returned %v, want nil", err)
	}
	if got != (Meta{}) {
		t.Errorf("LoadMeta = %+v, want the zero Meta", got)
	}
}

func TestMetaRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "meta.json")
	want := Meta{V: 1, Adapter: "python", SeededAt: "a3f21e0", Cycles: 7}
	if err := SaveMeta(p, want); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(b) != `{"v":1,"adapter":"python","seeded_at":"a3f21e0","cycles":7}`+"\n" {
		t.Errorf("SaveMeta wrote %q", string(b))
	}
	got, err := LoadMeta(p)
	if err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if got != want {
		t.Errorf("LoadMeta = %+v, want %+v", got, want)
	}
}

func TestLoadMetaIsFatalOnMalformedJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "meta.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMeta(p); err == nil {
		t.Fatal("LoadMeta returned nil error for malformed JSON")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mapstore/... -run 'TestLoadMeta|TestMetaRoundTrip' -v`
Expected: FAIL — `internal/mapstore/meta_test.go:10:14: undefined: LoadMeta` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/mapstore/meta.go`:

```go
package mapstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Meta is .rtdd/meta.json. It is kept out of map.jsonl because the JSONL is union-merged
// and these fields must not be duplicated by a merge.
type Meta struct {
	V        int    `json:"v"`
	Adapter  string `json:"adapter"`
	SeededAt string `json:"seeded_at"`
	Cycles   int    `json:"cycles"`
}

// LoadMeta reads .rtdd/meta.json. A missing file returns the zero Meta and a nil error.
func LoadMeta(path string) (Meta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Meta{}, nil
		}
		return Meta{}, fmt.Errorf("mapstore: read %s: %w", path, err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("mapstore: %s: malformed meta: %w", path, err)
	}
	return m, nil
}

func SaveMeta(path string, m Meta) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mapstore: mkdir %s: %w", dir, err)
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("mapstore: marshal meta: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("mapstore: write %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mapstore/... -v`
Expected: PASS, every mapstore test.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/mapstore/meta.go internal/mapstore/meta_test.go
git commit -m "M1a: mapstore Meta load/save for .rtdd/meta.json"
```

---

### Task 11: internal/gitctx — real-git test harness, RepoRoot, HeadSHA

**Files:**
- Create: `internal/gitctx/gitctx.go`
- Test: `internal/gitctx/helper_test.go`, `internal/gitctx/gitctx_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Status int` with `Added, Modified, Deleted, Renamed, Untracked`
  - `func (s Status) String() string`
  - `type LineRange struct{ Start, End int }`
  - `type Change struct { Path, OldPath string; Status Status; Lines []LineRange }`
  - `func RepoRoot(start string) (string, error)`
  - `func HeadSHA(repoRoot string) (string, error)`
  - unexported `func git(repoRoot string, args ...string) (string, error)`

Git is **never mocked**. Every test in this package builds a real repository with real
commits in `t.TempDir()`.

- [ ] **Step 1: Write the failing test**

Create `internal/gitctx/helper_test.go`:

```go
package gitctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a real git repository in t.TempDir() with a deterministic identity.
// Git is never mocked in this package: the bugs this code exists to prevent live in
// git's actual output.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "rtdd@example.com")
	gitRun(t, dir, "config", "user.name", "rtdd test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func remove(t *testing.T, dir, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

// commit stages everything and commits, returning the short SHA.
func commit(t *testing.T, dir, msg string) string {
	t.Helper()
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", msg)
	return strings.TrimSpace(gitRun(t, dir, "rev-parse", "--short", "HEAD"))
}
```

Create `internal/gitctx/gitctx_test.go`:

```go
package gitctx

import (
	"path/filepath"
	"testing"
)

func TestRepoRoot(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "x = 1\n")
	commit(t, dir, "init")

	tests := []struct {
		name  string
		start string
	}{
		{"from the root", dir},
		{"from a subdirectory", filepath.Join(dir, "src")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepoRoot(tc.start)
			if err != nil {
				t.Fatalf("RepoRoot(%q): %v", tc.start, err)
			}
			wantBase := filepath.Base(dir)
			if filepath.Base(got) != wantBase {
				t.Errorf("RepoRoot = %q, want a path ending in %q", got, wantBase)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("RepoRoot = %q, want an absolute path", got)
			}
		})
	}
}

func TestRepoRootOutsideARepoIsAnError(t *testing.T) {
	if _, err := RepoRoot(t.TempDir()); err == nil {
		t.Fatal("RepoRoot outside a work tree returned nil error")
	}
}

func TestHeadSHA(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "x = 1\n")
	want := commit(t, dir, "init")

	got, err := HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if got != want {
		t.Errorf("HeadSHA = %q, want %q", got, want)
	}
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{Added, "added"},
		{Modified, "modified"},
		{Deleted, "deleted"},
		{Renamed, "renamed"},
		{Untracked, "untracked"},
	}
	for _, tc := range tests {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Status(%d).String() = %q, want %q", int(tc.s), got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitctx/... -run 'TestRepoRoot|TestHeadSHA|TestStatusString' -v`
Expected: FAIL — `internal/gitctx/gitctx_test.go:...: undefined: RepoRoot`, `undefined: HeadSHA`, `undefined: Added` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/gitctx/gitctx.go`:

```go
// Package gitctx is the only package in the engine that shells out to git.
package gitctx

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Status int

const (
	Added Status = iota
	Modified
	Deleted
	Renamed
	Untracked
)

func (s Status) String() string {
	switch s {
	case Added:
		return "added"
	case Modified:
		return "modified"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	case Untracked:
		return "untracked"
	}
	return "unknown"
}

// LineRange is 1-indexed and inclusive, in the NEW file.
type LineRange struct{ Start, End int }

type Change struct {
	Path    string
	OldPath string      // set only when Status == Renamed
	Status  Status
	Lines   []LineRange // empty for Deleted
}

// git runs a git subcommand in repoRoot and returns its stdout.
func git(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return string(out), fmt.Errorf("gitctx: git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// RepoRoot returns the absolute, cleaned top level of the git work tree containing start.
func RepoRoot(start string) (string, error) {
	out, err := git(start, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", fmt.Errorf("gitctx: git rev-parse --show-toplevel returned nothing for %q", start)
	}
	return filepath.Clean(root), nil
}

func HeadSHA(repoRoot string) (string, error) {
	out, err := git(repoRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitctx/... -run 'TestRepoRoot|TestHeadSHA|TestStatusString' -v`
Expected: PASS, 6 subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/gitctx/gitctx.go internal/gitctx/gitctx_test.go internal/gitctx/helper_test.go
git commit -m "M1a: gitctx types, RepoRoot, HeadSHA, and a real-git test harness"
```

---

### Task 12: internal/gitctx — ChangedSet including untracked files

**Files:**
- Create: `internal/gitctx/changedset.go`
- Test: `internal/gitctx/changedset_test.go`

**Interfaces:**
- Consumes: `type Status`, `type LineRange`, `type Change`, unexported `git`.
- Produces: `func ChangedSet(repoRoot, base string) ([]Change, error)`

**This is v1's flagship bug.** `git diff` does not list a just-written, never-added file, and
a just-written file is the single most common input in a TDD cycle. The changed set is the
**union** of `git diff <base>` and `git status --porcelain -uall`. Deletions are retained (a
deleted path still selects the tests whose `F` contains it), and renames carry `OldPath` so
the old path also participates in selection.

- [ ] **Step 1: Write the failing test**

Create `internal/gitctx/changedset_test.go`:

```go
package gitctx

import (
	"reflect"
	"sort"
	"testing"
)

// byPath indexes a ChangedSet result for assertions.
func byPath(cs []Change) map[string]Change {
	out := make(map[string]Change, len(cs))
	for _, c := range cs {
		out[c.Path] = c
	}
	return out
}

func paths(cs []Change) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Path)
	}
	sort.Strings(out)
	return out
}

// The regression that killed v1: a test file the agent just wrote and never added is
// invisible to `git diff`, so it was in no tier and never ran.
func TestChangedSetIncludesUntrackedFiles(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "def login():\n    return 1\n")
	commit(t, dir, "init")

	write(t, dir, "tests/test_auth.py", "def test_login():\n    assert login()\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	got := byPath(cs)
	c, ok := got["tests/test_auth.py"]
	if !ok {
		t.Fatalf("ChangedSet omitted the untracked file; got %v", paths(cs))
	}
	if c.Status != Added {
		t.Errorf("Status = %v, want added (ChangedSet reports untracked files as Added)", c.Status)
	}
	if !reflect.DeepEqual(c.Lines, []LineRange{{Start: 1, End: 2}}) {
		t.Errorf("Lines = %#v, want the whole file as one range {1,2}", c.Lines)
	}
}

func TestChangedSetRetainsDeletions(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/legacy.py", "x = 1\n")
	write(t, dir, "src/keep.py", "y = 1\n")
	commit(t, dir, "init")

	remove(t, dir, "src/legacy.py")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c, ok := byPath(cs)["src/legacy.py"]
	if !ok {
		t.Fatalf("ChangedSet dropped the deleted path; got %v", paths(cs))
	}
	if c.Status != Deleted {
		t.Errorf("Status = %v, want deleted", c.Status)
	}
	if len(c.Lines) != 0 {
		t.Errorf("Lines = %#v, want empty for a deletion", c.Lines)
	}
}

func TestChangedSetHandlesRenames(t *testing.T) {
	dir := newRepo(t)
	body := "def a():\n    return 1\n\ndef b():\n    return 2\n\ndef c():\n    return 3\n"
	write(t, dir, "src/old.py", body)
	commit(t, dir, "init")

	remove(t, dir, "src/old.py")
	write(t, dir, "src/new.py", body)
	gitRun(t, dir, "add", "-A")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c, ok := byPath(cs)["src/new.py"]
	if !ok {
		t.Fatalf("ChangedSet omitted the rename destination; got %v", paths(cs))
	}
	if c.Status != Renamed {
		t.Errorf("Status = %v, want renamed", c.Status)
	}
	if c.OldPath != "src/old.py" {
		t.Errorf("OldPath = %q, want %q (the old path must still select its tests)", c.OldPath, "src/old.py")
	}
}

func TestChangedSetLineRanges(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n")
	commit(t, dir, "init")

	// Edit line 2 and lines 8-9; the two hunks must stay separate at --unified=0.
	write(t, dir, "src/auth.py", "l1\nEDITED2\nl3\nl4\nl5\nl6\nl7\nEDITED8\nEDITED9\nl10\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c := byPath(cs)["src/auth.py"]
	want := []LineRange{{Start: 2, End: 2}, {Start: 8, End: 9}}
	if !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("Lines = %#v, want %#v", c.Lines, want)
	}
	if c.Status != Modified {
		t.Errorf("Status = %v, want modified", c.Status)
	}
}

func TestChangedSetUnionsStagedWorktreeAndUntracked(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	write(t, dir, "src/b.py", "b = 1\n")
	commit(t, dir, "init")

	write(t, dir, "src/a.py", "a = 2\n")
	gitRun(t, dir, "add", "src/a.py") // staged
	write(t, dir, "src/b.py", "b = 2\n") // unstaged
	write(t, dir, "src/c.py", "c = 1\n") // untracked

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	want := []string{"src/a.py", "src/b.py", "src/c.py"}
	if !reflect.DeepEqual(paths(cs), want) {
		t.Errorf("paths = %v, want %v", paths(cs), want)
	}
}

func TestChangedSetAgainstAnEarlierBase(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	base := commit(t, dir, "one")
	write(t, dir, "src/b.py", "b = 1\n")
	commit(t, dir, "two")
	write(t, dir, "src/c.py", "c = 1\n")

	cs, err := ChangedSet(dir, base)
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	want := []string{"src/b.py", "src/c.py"}
	if !reflect.DeepEqual(paths(cs), want) {
		t.Errorf("paths = %v, want %v", paths(cs), want)
	}
}

func TestChangedSetIsSortedAndEmptyOnACleanTree(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	commit(t, dir, "init")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("ChangedSet on a clean tree = %v, want empty", paths(cs))
	}
}

func TestChangedSetOnARepoWithNoCommits(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "tests/test_a.py", "def test_a():\n    pass\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet on a commitless repo returned %v, want nil", err)
	}
	if !reflect.DeepEqual(paths(cs), []string{"tests/test_a.py"}) {
		t.Errorf("paths = %v, want [tests/test_a.py]", paths(cs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitctx/... -run TestChangedSet -v`
Expected: FAIL — `internal/gitctx/changedset_test.go:...: undefined: ChangedSet` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/gitctx/changedset.go`:

```go
package gitctx

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ChangedSet returns the union of `git diff --name-only --unified=0 <base>` and
// `git status --porcelain -uall`. Untracked files are included (Status=Added with the
// whole file as one LineRange). Deletions are retained.
//
// The union is not an optimisation. `git diff` does not list a file that was written but
// never added, and a just-written file is the most common input in a TDD cycle; omitting
// it is what made v1's flagship case invisible.
func ChangedSet(repoRoot, base string) ([]Change, error) {
	if strings.TrimSpace(base) == "" {
		base = "HEAD"
	}

	hasBase := true
	if _, err := git(repoRoot, "rev-parse", "--verify", "-q", base+"^{commit}"); err != nil {
		if base != "HEAD" {
			return nil, fmt.Errorf("gitctx: unknown base %q", base)
		}
		hasBase = false // a repository with no commits yet
	}

	changes := map[string]*Change{}

	if hasBase {
		nameStatus, err := git(repoRoot, "diff", "--name-status", "-M", "-z", base)
		if err != nil {
			return nil, err
		}
		fields := splitZ(nameStatus)
		for i := 0; i < len(fields); {
			code := fields[i]
			i++
			if code == "" {
				continue
			}
			switch code[0] {
			case 'R', 'C':
				if i+1 >= len(fields) {
					return nil, fmt.Errorf("gitctx: truncated rename record from git diff --name-status")
				}
				oldPath, newPath := fields[i], fields[i+1]
				i += 2
				changes[newPath] = &Change{Path: newPath, OldPath: oldPath, Status: Renamed}
			default:
				if i >= len(fields) {
					return nil, fmt.Errorf("gitctx: truncated record from git diff --name-status")
				}
				p := fields[i]
				i++
				changes[p] = &Change{Path: p, Status: statusFromDiffCode(code[0])}
			}
		}

		diff, err := git(repoRoot, "diff", "--unified=0", "--no-color", "-M", base)
		if err != nil {
			return nil, err
		}
		for p, ranges := range parseHunks(diff) {
			if c, ok := changes[p]; ok {
				c.Lines = append(c.Lines, ranges...)
			}
		}
	}

	porcelain, err := git(repoRoot, "status", "--porcelain", "-uall", "-z")
	if err != nil {
		return nil, err
	}
	pf := splitZ(porcelain)
	for i := 0; i < len(pf); {
		rec := pf[i]
		i++
		if len(rec) < 4 {
			continue
		}
		x, y := rec[0], rec[1]
		p := rec[3:]
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			if i < len(pf) {
				i++ // consume the origin-path field that follows a rename record
			}
		}
		if _, seen := changes[p]; seen {
			continue
		}
		if x == '?' && y == '?' {
			changes[p] = &Change{Path: p, Status: Added, Lines: wholeFile(repoRoot, p)}
			continue
		}
		changes[p] = &Change{Path: p, Status: statusFromPorcelain(x, y)}
	}

	out := make([]Change, 0, len(changes))
	for _, c := range changes {
		if c.Status == Deleted {
			c.Lines = nil
		} else {
			c.Lines = mergeRanges(c.Lines)
		}
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func statusFromDiffCode(c byte) Status {
	switch c {
	case 'A':
		return Added
	case 'D':
		return Deleted
	case 'R', 'C':
		return Renamed
	default:
		return Modified
	}
}

func statusFromPorcelain(x, y byte) Status {
	for _, c := range []byte{x, y} {
		switch c {
		case 'A':
			return Added
		case 'D':
			return Deleted
		case 'R', 'C':
			return Renamed
		case 'M', 'T', 'U':
			return Modified
		}
	}
	return Modified
}

// splitZ splits NUL-separated git output, dropping empty trailing fields.
func splitZ(s string) []string {
	parts := strings.Split(s, "\x00")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// wholeFile returns the whole of an untracked file as one LineRange.
func wholeFile(repoRoot, rel string) []LineRange {
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil || len(b) == 0 {
		return nil
	}
	n := bytes.Count(b, []byte("\n"))
	if b[len(b)-1] != '\n' {
		n++
	}
	if n == 0 {
		return nil
	}
	return []LineRange{{Start: 1, End: n}}
}

// parseHunks extracts NEW-file line ranges from a `git diff --unified=0` body,
// keyed by the repo-relative path in the `+++ b/<path>` header.
func parseHunks(diff string) map[string][]LineRange {
	out := map[string][]LineRange{}
	cur := ""
	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if i := strings.IndexByte(p, '\t'); i >= 0 {
				p = p[:i]
			}
			if p == "/dev/null" {
				cur = ""
				continue
			}
			cur = strings.TrimPrefix(p, "b/")
		case strings.HasPrefix(line, "@@") && cur != "":
			if r, ok := parseHunkHeader(line); ok {
				out[cur] = append(out[cur], r)
			}
		}
	}
	return out
}

// parseHunkHeader parses "@@ -a,b +c,d @@ ..." and returns the NEW-side range.
func parseHunkHeader(line string) (LineRange, bool) {
	i := strings.IndexByte(line, '+')
	if i < 0 {
		return LineRange{}, false
	}
	rest := line[i+1:]
	j := strings.IndexAny(rest, " @")
	if j < 0 {
		return LineRange{}, false
	}
	spec := rest[:j]
	startStr, countStr := spec, "1"
	if k := strings.IndexByte(spec, ','); k >= 0 {
		startStr, countStr = spec[:k], spec[k+1:]
	}
	start, err1 := strconv.Atoi(startStr)
	count, err2 := strconv.Atoi(countStr)
	if err1 != nil || err2 != nil {
		return LineRange{}, false
	}
	if count == 0 {
		// A pure deletion: attribute it to the surviving line it was removed after.
		if start == 0 {
			start = 1
		}
		return LineRange{Start: start, End: start}, true
	}
	return LineRange{Start: start, End: start + count - 1}, true
}

// mergeRanges sorts and coalesces adjacent or overlapping ranges.
func mergeRanges(rs []LineRange) []LineRange {
	if len(rs) == 0 {
		return nil
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Start != rs[j].Start {
			return rs[i].Start < rs[j].Start
		}
		return rs[i].End < rs[j].End
	})
	out := []LineRange{rs[0]}
	for _, r := range rs[1:] {
		last := &out[len(out)-1]
		if r.Start <= last.End+1 {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		out = append(out, r)
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitctx/... -run TestChangedSet -v`
Expected: PASS, 8 tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/gitctx/changedset.go internal/gitctx/changedset_test.go
git commit -m "M1a: gitctx.ChangedSet unions diff and porcelain, keeps untracked and deleted"
```

---

### Task 13: internal/gitctx — CommitDistance, IsMergeCommit, Older

**Files:**
- Create: `internal/gitctx/history.go`
- Test: `internal/gitctx/history_test.go`

**Interfaces:**
- Consumes: unexported `git`.
- Produces:
  - `func CommitDistance(repoRoot, sha string) (int, error)` — `(-1, nil)` when `sha` is unreachable
  - `func IsMergeCommit(repoRoot, sha string) (bool, error)`
  - `func Older(repoRoot string) func(a, b string) string`

`-1` means **unknown**, never **fresh**. A rebase, a squash, or a shallow clone makes a
recorded `c` unreachable; treating that as distance 0 would report a map that may be
arbitrarily stale as current, and staleness escalation would never fire.

- [ ] **Step 1: Write the failing test**

Create `internal/gitctx/history_test.go`:

```go
package gitctx

import "testing"

func TestCommitDistance(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	first := commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	second := commit(t, dir, "two")
	write(t, dir, "c.txt", "1\n")
	third := commit(t, dir, "three")

	tests := []struct {
		name string
		sha  string
		want int
	}{
		{"HEAD is zero away from itself", third, 0},
		{"one commit back", second, 1},
		{"two commits back", first, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommitDistance(dir, tc.sha)
			if err != nil {
				t.Fatalf("CommitDistance: %v", err)
			}
			if got != tc.want {
				t.Errorf("CommitDistance(%q) = %d, want %d", tc.sha, got, tc.want)
			}
		})
	}
}

// A rebase, squash, or shallow clone leaves a recorded `c` unreachable. -1 means
// UNKNOWN. A caller that reads it as 0 would report a possibly-ancient map as fresh.
func TestCommitDistanceUnreachableIsMinusOne(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")

	tests := []struct {
		name string
		sha  string
	}{
		{"a sha that never existed", "deadbee"},
		{"an empty sha", ""},
		{"a full-length sha that never existed", "0123456789abcdef0123456789abcdef01234567"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommitDistance(dir, tc.sha)
			if err != nil {
				t.Fatalf("CommitDistance returned an error for an unreachable sha: %v", err)
			}
			if got != -1 {
				t.Errorf("CommitDistance(%q) = %d, want -1 (unknown, never fresh)", tc.sha, got)
			}
		})
	}
}

func TestCommitDistanceAfterAnAmendMakesTheOldShaUnreachable(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	stale := commit(t, dir, "two")

	write(t, dir, "b.txt", "2\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "--amend", "-m", "two-amended")

	got, err := CommitDistance(dir, stale)
	if err != nil {
		t.Fatalf("CommitDistance: %v", err)
	}
	// The amended-away commit is still in the reflog, so git can resolve it; what must
	// never happen is a report of 0, which would mean "fresh".
	if got == 0 {
		t.Errorf("CommitDistance(%q) = 0 after an amend; a rewritten commit is never fresh", stale)
	}
}

func TestIsMergeCommit(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")

	if merge, err := IsMergeCommit(dir, "HEAD"); err != nil || merge {
		t.Errorf("IsMergeCommit on an ordinary commit = (%v, %v), want (false, nil)", merge, err)
	}

	gitRun(t, dir, "checkout", "-q", "-b", "side")
	write(t, dir, "side.txt", "1\n")
	commit(t, dir, "side")
	gitRun(t, dir, "checkout", "-q", "main")
	write(t, dir, "main.txt", "1\n")
	commit(t, dir, "main")
	gitRun(t, dir, "merge", "-q", "--no-ff", "-m", "merge side", "side")

	merge, err := IsMergeCommit(dir, "HEAD")
	if err != nil {
		t.Fatalf("IsMergeCommit: %v", err)
	}
	if !merge {
		t.Error("IsMergeCommit on a merge commit = false, want true")
	}
}

func TestOlder(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	first := commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	second := commit(t, dir, "two")

	older := Older(dir)

	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"ancestor wins, a first", first, second, first},
		{"ancestor wins, b first", second, first, first},
		{"identical shas", first, first, first},
		{"unreachable a is treated as older", "deadbee", second, "deadbee"},
		{"unreachable b is treated as older", first, "deadbee", "deadbee"},
		{"empty a yields b", "", second, second},
		{"empty b yields a", first, "", first},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := older(tc.a, tc.b); got != tc.want {
				t.Errorf("Older()(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitctx/... -run 'TestCommitDistance|TestIsMergeCommit|TestOlder' -v`
Expected: FAIL — `internal/gitctx/history_test.go:...: undefined: CommitDistance`, `undefined: IsMergeCommit`, `undefined: Older` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/gitctx/history.go`:

```go
package gitctx

import (
	"os/exec"
	"strconv"
	"strings"
)

// CommitDistance returns the number of commits from sha to HEAD.
// Returns (-1, nil) when sha is unreachable — after a rebase, squash, or shallow clone.
// Callers MUST treat -1 as "unknown", never as "fresh".
// An error is returned only when git itself is unusable, which is exit code 3 territory.
func CommitDistance(repoRoot, sha string) (int, error) {
	if strings.TrimSpace(sha) == "" {
		return -1, nil
	}
	if _, err := git(repoRoot, "rev-parse", "--git-dir"); err != nil {
		return -1, err
	}
	if !reachable(repoRoot, sha) {
		return -1, nil
	}
	out, err := git(repoRoot, "rev-list", "--count", sha+"..HEAD")
	if err != nil {
		return -1, nil
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil {
		return -1, nil
	}
	return n, nil
}

func IsMergeCommit(repoRoot, sha string) (bool, error) {
	if strings.TrimSpace(sha) == "" {
		sha = "HEAD"
	}
	out, err := git(repoRoot, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return false, err
	}
	// "<sha> <parent1> [<parent2> ...]"
	return len(strings.Fields(strings.TrimSpace(out))) > 2, nil
}

// Older returns whichever of a or b is the earlier ancestor. If either is unreachable,
// it returns that one (unknown age is treated as older, i.e. less trustworthy).
func Older(repoRoot string) func(a, b string) string {
	return func(a, b string) string {
		switch {
		case a == "":
			return b
		case b == "":
			return a
		case a == b:
			return a
		}
		aOK, bOK := reachable(repoRoot, a), reachable(repoRoot, b)
		switch {
		case !aOK:
			return a
		case !bOK:
			return b
		case isAncestor(repoRoot, a, b):
			return a
		case isAncestor(repoRoot, b, a):
			return b
		}
		// Unrelated histories: the one further from HEAD is the older one.
		da, _ := CommitDistance(repoRoot, a)
		db, _ := CommitDistance(repoRoot, b)
		if db > da {
			return b
		}
		return a
	}
}

func reachable(repoRoot, sha string) bool {
	_, err := git(repoRoot, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

func isAncestor(repoRoot, a, b string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", a, b)
	cmd.Dir = repoRoot
	return cmd.Run() == nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitctx/... -v`
Expected: PASS, every gitctx test.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/gitctx/history.go internal/gitctx/history_test.go
git commit -m "M1a: gitctx CommitDistance (-1 = unknown), IsMergeCommit, Older"
```

---

### Task 14: internal/adapter — the M1a classifier stub

**Files:**
- Create: `internal/adapter/adapter.go`, `internal/adapter/testdata/python.yaml`, `adapters/python.yaml`
- Test: `internal/adapter/adapter_test.go`

**Interfaces:**
- Consumes: `func paths.MatchGlob(pattern, rel string) bool`
- Produces:
  - `type Adapter struct { Name, Detect, Env, Seed, Subset, List, Coverage, Report, FailFastFlag, TestGlobs, SourceGlobs, ExitCodes, Opaque, FullEscalate }` exactly as in `00-interfaces.md`
  - `func Load(path string) (*Adapter, error)`
  - `func (a *Adapter) IsTestFile(rel string) bool`
  - `func (a *Adapter) IsOpaque(rel string) bool`
  - `func (a *Adapter) IsFullEscalate(rel string) bool`
  - `func (a *Adapter) IsInstrumentable(rel string) bool`

**M1a scope:** `LoadAll`, `Detect`, and `Expand` are **not** implemented here — they belong to
M1b, which is where a real repository gets probed and a real command gets built. M1a loads one
adapter from an explicit path and uses it only to classify files.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/testdata/python.yaml`:

```yaml
name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
env: { COVERAGE_CORE: ctrace }
seed: "pytest --cov={src} --cov-context=test --report-log={log}"
subset: "pytest {tests} --cov={src} --cov-context=test --report-log={log}"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
failfast_flag: "-x"
test_globs: ["tests/**/*.py", "**/test_*.py"]
source_globs: ["src/**/*.py"]
exit_codes: { 4: bad-selector, 5: no-tests-collected }
opaque: ["**/*.yaml", "**/*.yml", "**/*.sql", "**/*.html", "**/*.j2", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
```

Create `adapters/python.yaml` with the identical content (this is the shipped declaration;
M1b consumes the `seed`/`subset`/`list` fields that M1a only parses).

Create `internal/adapter/adapter_test.go`:

```go
package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T) *Adapter {
	t.Helper()
	a, err := Load("testdata/python.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return a
}

func TestLoadParsesEveryField(t *testing.T) {
	a := loadFixture(t)
	if a.Name != "python" {
		t.Errorf("Name = %q, want python", a.Name)
	}
	if a.Env["COVERAGE_CORE"] != "ctrace" {
		t.Errorf("Env[COVERAGE_CORE] = %q, want ctrace", a.Env["COVERAGE_CORE"])
	}
	if a.Coverage != "sqlite" || a.Report != "pytest-reportlog" {
		t.Errorf("Coverage/Report = %q/%q, want sqlite/pytest-reportlog", a.Coverage, a.Report)
	}
	if a.FailFastFlag != "-x" {
		t.Errorf("FailFastFlag = %q, want -x", a.FailFastFlag)
	}
	if a.ExitCodes[4] != "bad-selector" || a.ExitCodes[5] != "no-tests-collected" {
		t.Errorf("ExitCodes = %#v, want 4=bad-selector 5=no-tests-collected", a.ExitCodes)
	}
	if len(a.Detect) != 3 || len(a.TestGlobs) != 2 || len(a.FullEscalate) != 3 {
		t.Errorf("Detect/TestGlobs/FullEscalate lengths = %d/%d/%d, want 3/2/3",
			len(a.Detect), len(a.TestGlobs), len(a.FullEscalate))
	}
}

func TestLoadRejectsBadYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"not yaml", "\tthis: is: not: yaml\n"},
		{"unknown field", "name: python\nnot_a_field: 1\n"},
		{"missing name", "detect: [\"pyproject.toml\"]\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.yaml")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(p); err == nil {
				t.Errorf("Load accepted %q; a bad adapter is a configuration error (exit 2)", tc.content)
			}
		})
	}
}

func TestLoadMissingFileIsAnError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("Load of a missing adapter returned nil error")
	}
}

func TestClassification(t *testing.T) {
	a := loadFixture(t)
	tests := []struct {
		name            string
		rel             string
		test            bool
		opaque          bool
		fullEscalate    bool
		instrumentable  bool
	}{
		{"test under tests/", "tests/test_auth.py", true, false, false, false},
		{"nested test under tests/", "tests/unit/api/test_auth.py", true, false, false, false},
		{"test_ prefixed anywhere", "src/pkg/test_helpers.py", true, false, false, false},
		{"plain source", "src/auth.py", false, false, false, true},
		{"nested source", "src/pkg/deep/auth.py", false, false, false, true},
		{"source outside source_globs", "scripts/tool.py", false, false, false, false},
		{"template is opaque", "templates/page.html", false, true, false, false},
		{"yaml is opaque", "config/settings.yaml", false, true, false, false},
		{"fixtures directory is opaque", "tests/fixtures/data/users.json", false, true, false, false},
		{"conftest escalates to full", "tests/conftest.py", false, false, true, false},
		{"pyproject escalates to full", "pyproject.toml", false, false, true, false},
		{"requirements escalates to full", "requirements.txt", false, false, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := a.IsTestFile(tc.rel); got != tc.test {
				t.Errorf("IsTestFile(%q) = %v, want %v", tc.rel, got, tc.test)
			}
			if got := a.IsOpaque(tc.rel); got != tc.opaque {
				t.Errorf("IsOpaque(%q) = %v, want %v", tc.rel, got, tc.opaque)
			}
			if got := a.IsFullEscalate(tc.rel); got != tc.fullEscalate {
				t.Errorf("IsFullEscalate(%q) = %v, want %v", tc.rel, got, tc.fullEscalate)
			}
			if got := a.IsInstrumentable(tc.rel); got != tc.instrumentable {
				t.Errorf("IsInstrumentable(%q) = %v, want %v", tc.rel, got, tc.instrumentable)
			}
		})
	}
}

func TestNilAdapterClassifiesNothing(t *testing.T) {
	var a *Adapter
	if a.IsTestFile("tests/test_a.py") || a.IsOpaque("a.yaml") || a.IsFullEscalate("pyproject.toml") || a.IsInstrumentable("src/a.py") {
		t.Error("a nil *Adapter must classify nothing rather than panic")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/... -v`
Expected: FAIL — `internal/adapter/adapter_test.go:11:12: undefined: Load` and `undefined: Adapter` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/adapter/adapter.go`:

```go
// Package adapter declares what is genuinely declarative about a language toolchain.
// Execution and parsing are implemented per language in M1b; M1a uses only the
// glob-classification half.
package adapter

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/VocanicZ/rtdd/internal/paths"
)

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

// Load reads one adapter declaration. Unknown fields are rejected so a typo in a host
// repo's adapter is a configuration error (exit 2) rather than a silently ignored glob.
func Load(path string) (*Adapter, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("adapter: read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var a Adapter
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", path, err)
	}
	if a.Name == "" {
		return nil, fmt.Errorf("adapter: %s: missing required field \"name\"", path)
	}
	return &a, nil
}

func (a *Adapter) IsTestFile(rel string) bool {
	return a != nil && anyGlob(a.TestGlobs, rel)
}

func (a *Adapter) IsOpaque(rel string) bool {
	return a != nil && anyGlob(a.Opaque, rel)
}

func (a *Adapter) IsFullEscalate(rel string) bool {
	return a != nil && anyGlob(a.FullEscalate, rel)
}

// IsInstrumentable: matches SourceGlobs AND is not a test file AND is not Opaque.
func (a *Adapter) IsInstrumentable(rel string) bool {
	if a == nil {
		return false
	}
	return anyGlob(a.SourceGlobs, rel) && !a.IsTestFile(rel) && !a.IsOpaque(rel)
}

func anyGlob(patterns []string, rel string) bool {
	for _, p := range patterns {
		if paths.MatchGlob(p, rel) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/... -v`
Expected: PASS, all subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/adapter adapters/python.yaml
git commit -m "M1a: adapter YAML load and glob classification (stub scope)"
```

---

### Task 15: internal/selector — Tier, Config, Selection, Inputs

**Files:**
- Create: `internal/selector/tier.go`
- Test: `internal/selector/tier_test.go`

**Interfaces:**
- Consumes: `*mapstore.Map`, `[]gitctx.Change`, `*adapter.Adapter`.
- Produces:
  - `type Tier int` with `TierEmpty, TierDirect, TierT0, TierT1, TierT2`
  - `func (t Tier) String() string`
  - `type Config struct { StaleCommits int; DriftGuard int; HubThreshold float64 }`
  - `func DefaultConfig() Config`
  - `type Selection struct { Tier Tier; Tests []string; Direct []string; Reason string }`
  - `type Inputs struct { Map; Changes; Adapter; Cfg; AllTests; Distance; Cycles; Merge; ImportOnly }`

`TierEmpty` is deliberately the zero value **and** an explicitly reported outcome. Every
under-selection path terminates there, so it is never silently green.

- [ ] **Step 1: Write the failing test**

Create `internal/selector/tier_test.go`:

```go
package selector

import "testing"

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierEmpty, "empty"},
		{TierDirect, "direct"},
		{TierT0, "T0"},
		{TierT1, "T1"},
		{TierT2, "T2"},
	}
	for _, tc := range tests {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("Tier(%d).String() = %q, want %q", int(tc.tier), got, tc.want)
		}
	}
}

// TierEmpty must be the zero value so a Selection nobody filled in reads as "nothing
// selected" rather than as a successful tier.
func TestTierEmptyIsTheZeroValue(t *testing.T) {
	var s Selection
	if s.Tier != TierEmpty {
		t.Errorf("zero Selection.Tier = %v, want TierEmpty", s.Tier)
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.StaleCommits != 50 {
		t.Errorf("StaleCommits = %d, want 50", c.StaleCommits)
	}
	if c.DriftGuard != 100 {
		t.Errorf("DriftGuard = %d, want 100", c.DriftGuard)
	}
	if c.HubThreshold != 0.40 {
		t.Errorf("HubThreshold = %v, want 0.40", c.HubThreshold)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run 'TestTier|TestDefaultConfig' -v`
Expected: FAIL — `internal/selector/tier_test.go:8:9: undefined: Tier` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/selector/tier.go`:

```go
// Package selector turns (map, changed set, adapter, config) into a ranked selection.
// It is a pure function: no git, no subprocess, no I/O.
package selector

import (
	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

type Tier int

const (
	// TierEmpty means nothing was selected. It is reported explicitly and is never
	// collapsed into success: every under-selection path terminates here.
	TierEmpty Tier = iota
	TierDirect
	TierT0
	TierT1
	TierT2
)

func (t Tier) String() string {
	switch t {
	case TierEmpty:
		return "empty"
	case TierDirect:
		return "direct"
	case TierT0:
		return "T0"
	case TierT1:
		return "T1"
	case TierT2:
		return "T2"
	}
	return "unknown"
}

type Config struct {
	StaleCommits int     // default 50
	DriftGuard   int     // default 100
	HubThreshold float64 // default 0.40
}

func DefaultConfig() Config {
	return Config{StaleCommits: 50, DriftGuard: 100, HubThreshold: 0.40}
}

type Selection struct {
	Tier   Tier
	Tests  []string // final ranked list, direct tests first
	Direct []string // changed/new test files, always run
	Reason string   // human-readable escalation cause
}

type Inputs struct {
	Map        *mapstore.Map
	Changes    []gitctx.Change
	Adapter    *adapter.Adapter
	Cfg        Config
	AllTests   []string             // from adapter.List; needed for T2 and for direct-tier discovery
	Distance   func(sha string) int // wraps gitctx.CommitDistance; -1 means unknown
	Cycles     int                  // from meta.json, for DriftGuard
	Merge      bool                 // HEAD is a merge commit; escalates to T1
	ImportOnly func(rel string) []string // static-import fallback; see M2
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -run 'TestTier|TestDefaultConfig' -v`
Expected: PASS, 3 tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/selector/tier.go internal/selector/tier_test.go
git commit -m "M1a: selector Tier, Config, Selection, Inputs"
```

---

### Task 16: internal/selector — Rank

**Files:**
- Create: `internal/selector/rank.go`
- Test: `internal/selector/rank_test.go`

**Interfaces:**
- Consumes: `func (m *mapstore.Map) Get(t string) (mapstore.Row, bool)`.
- Produces: `func Rank(m *mapstore.Map, tests, changedFiles []string) []string`

Ordering, in order: descending `|F ∩ changed| / |F|`, then `S=="fail"` first, then ascending
`len(F)`, then ascending `D`, then ascending test id (a deterministic final tiebreak — the
list is printed and diffed).

- [ ] **Step 1: Write the failing test**

Create `internal/selector/rank_test.go`:

```go
package selector

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func mapOf(rows ...mapstore.Row) *mapstore.Map {
	m := mapstore.New()
	for _, r := range rows {
		m.Replace(r)
	}
	return m
}

func TestRank(t *testing.T) {
	tests := []struct {
		name    string
		m       *mapstore.Map
		tests   []string
		changed []string
		want    []string
	}{
		{
			name: "descending intersection ratio dominates",
			m: mapOf(
				mapstore.Row{T: "wide", F: []string{"src/a.py", "src/b.py", "src/c.py", "src/d.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "narrow", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"wide", "narrow"},
			changed: []string{"src/a.py"},
			want:    []string{"narrow", "wide"}, // 1.0 before 0.25
		},
		{
			name: "last-failed first breaks a ratio tie",
			m: mapOf(
				mapstore.Row{T: "passed", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "failed", F: []string{"src/a.py"}, C: "c1", D: 1, S: "fail"},
			),
			tests:   []string{"passed", "failed"},
			changed: []string{"src/a.py"},
			want:    []string{"failed", "passed"},
		},
		{
			name: "ascending len(F) breaks a ratio and outcome tie",
			m: mapOf(
				mapstore.Row{T: "two_files", F: []string{"src/a.py", "src/b.py"}, C: "c1", D: 1, S: "pass"},
				mapstore.Row{T: "one_file", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"},
			),
			tests:   []string{"two_files", "one_file"},
			changed: []string{"src/a.py", "src/b.py"}, // both are ratio 1.0
			want:    []string{"one_file", "two_files"},
		},
		{
			name: "ascending duration breaks a len(F) tie",
			m: mapOf(
				mapstore.Row{T: "slow", F: []string{"src/a.py"}, C: "c1", D: 900, S: "pass"},
				mapstore.Row{T: "fast", F: []string{"src/a.py"}, C: "c1", D: 3, S: "pass"},
			),
			tests:   []string{"slow", "fast"},
			changed: []string{"src/a.py"},
			want:    []string{"fast", "slow"},
		},
		{
			name: "test id breaks a total tie, so the order is deterministic",
			m: mapOf(
				mapstore.Row{T: "b_test", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
				mapstore.Row{T: "a_test", F: []string{"src/a.py"}, C: "c1", D: 5, S: "pass"},
			),
			tests:   []string{"b_test", "a_test"},
			changed: []string{"src/a.py"},
			want:    []string{"a_test", "b_test"},
		},
		{
			name: "all four keys at once",
			m: mapOf(
				mapstore.Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "c1", D: 412, S: "pass"},
				mapstore.Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "c1", D: 90, S: "fail"},
				mapstore.Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "c1", D: 15, S: "pass"},
			),
			tests: []string{
				"tests/test_auth.py::test_login",
				"tests/test_auth.py::test_logout",
				"tests/test_db.py::test_query",
			},
			changed: []string{"src/auth.py", "src/db.py"},
			// all three are ratio 1.0; the failed one leads, then len(F)=1 (D=15) then len(F)=2
			want: []string{
				"tests/test_auth.py::test_logout",
				"tests/test_db.py::test_query",
				"tests/test_auth.py::test_login",
			},
		},
		{
			name:    "a test with no map row sorts last but is never dropped",
			m:       mapOf(mapstore.Row{T: "mapped", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"}),
			tests:   []string{"unmapped", "mapped"},
			changed: []string{"src/a.py"},
			want:    []string{"mapped", "unmapped"},
		},
		{
			name:    "duplicates collapse, first occurrence order irrelevant",
			m:       mapOf(mapstore.Row{T: "a", F: []string{"src/a.py"}, C: "c1", D: 1, S: "pass"}),
			tests:   []string{"a", "a"},
			changed: []string{"src/a.py"},
			want:    []string{"a"},
		},
		{
			name:    "empty input",
			m:       mapOf(),
			tests:   nil,
			changed: []string{"src/a.py"},
			want:    []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Rank(tc.m, tc.tests, tc.changed)
			if len(got) == 0 {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Rank = %#v, want %#v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestRank -v`
Expected: FAIL — `internal/selector/rank_test.go:...: undefined: Rank` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/selector/rank.go`:

```go
package selector

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Rank orders tests by: descending |F ∩ changed| / |F|, then S=="fail" first,
// then ascending len(F), then ascending D.
//
// The final tiebreak is the test id, so the printed list is stable across runs.
// A test with no map row keeps ratio 0, len(F) 0 and D 0; it is ordered, never dropped.
func Rank(m *mapstore.Map, tests, changedFiles []string) []string {
	changed := make(map[string]struct{}, len(changedFiles))
	for _, f := range changedFiles {
		changed[f] = struct{}{}
	}

	type sortKey struct {
		id     string
		ratio  float64
		failed bool
		nFiles int
		durMS  int
	}

	keys := make([]sortKey, 0, len(tests))
	seen := make(map[string]struct{}, len(tests))
	for _, id := range tests {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		k := sortKey{id: id}
		if r, ok := m.Get(id); ok {
			hits := 0
			for _, f := range r.F {
				if _, in := changed[f]; in {
					hits++
				}
			}
			if len(r.F) > 0 {
				k.ratio = float64(hits) / float64(len(r.F))
			}
			k.failed = r.S == "fail"
			k.nFiles = len(r.F)
			k.durMS = r.D
		}
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.ratio != b.ratio {
			return a.ratio > b.ratio
		}
		if a.failed != b.failed {
			return a.failed
		}
		if a.nFiles != b.nFiles {
			return a.nFiles < b.nFiles
		}
		if a.durMS != b.durMS {
			return a.durMS < b.durMS
		}
		return a.id < b.id
	})

	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.id
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -run TestRank -v`
Expected: PASS, 9 subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/selector/rank.go internal/selector/rank_test.go
git commit -m "M1a: selector.Rank with the four spec keys plus a deterministic tiebreak"
```
