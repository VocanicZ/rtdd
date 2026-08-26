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
    Map        *mapstore.Map
    Changes    []gitctx.Change
    Adapter    *adapter.Adapter
    Cfg        Config
    AllTests   []string             // from adapter.List; needed for T2 and for direct-tier discovery
    Distance   func(sha string) int // wraps gitctx.CommitDistance; -1 means unknown
    Cycles     int                  // from meta.json, for DriftGuard
    Merge      bool                 // ADDED: HEAD is a merge commit; escalates to T1 (spec §4)
    ImportOnly func(rel string) []string // static-import fallback; see M2
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
	// Paths already recorded as the source of a rename must not be re-added as plain
	// modifications: git status reports both halves of a rename, and the diff pass has
	// already attached the old path to its Change via OldPath.
	renameSources := map[string]struct{}{}
	for _, c := range changes {
		if c.OldPath != "" {
			renameSources[c.OldPath] = struct{}{}
		}
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
				i++ // consume the origin-path field that accompanies a rename record
			}
		}
		if _, seen := changes[p]; seen {
			continue
		}
		if _, isSource := renameSources[p]; isSource {
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

---

### Task 17: internal/selector — Select, direct tier first and explicit TierEmpty

**Files:**
- Create: `internal/selector/select.go`
- Test: `internal/selector/select_test.go`

**Interfaces:**
- Consumes: `type Inputs`, `type Selection`, `type Config`, `func DefaultConfig() Config`, `func Rank(m *mapstore.Map, tests, changedFiles []string) []string`, `func (m *mapstore.Map) TestsCovering(files []string) []string`, `func (m *mapstore.Map) FanOut() map[string]int`, `func (m *mapstore.Map) Get(t string) (mapstore.Row, bool)`, `func (a *adapter.Adapter) IsTestFile(rel string) bool`, `func (a *adapter.Adapter) IsOpaque(rel string) bool`, `func (a *adapter.Adapter) IsFullEscalate(rel string) bool`.
- Produces: `func Select(in Inputs) Selection`

**Two hard cases this task exists for.**

1. **The direct tier is computed FIRST, before any map lookup.** A test the agent just wrote
   has no map row, so in v1 it was in no tier and step 3 of v1's own loop never executed it.
   `Direct` is built straight from the changed set via `Adapter.IsTestFile`, and is
   **prepended** to `Tests`.
2. **`TierEmpty` is a distinct, explicitly reported outcome.** It is returned with a `Reason`
   and never collapsed into a successful tier. `cmd` prints it as a warning and still exits 0
   — it is a signal, not a failure.

Order of evaluation: direct set → T2 escalations → T1 escalations → T0 → empty.

- [ ] **Step 1: Write the failing test**

Create `internal/selector/select_test.go`:

```go
package selector

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// fixtureAdapter mirrors adapters/python.yaml without touching the filesystem.
func fixtureAdapter() *adapter.Adapter {
	return &adapter.Adapter{
		Name:         "python",
		TestGlobs:    []string{"tests/**/*.py", "**/test_*.py"},
		SourceGlobs:  []string{"src/**/*.py"},
		Opaque:       []string{"**/*.yaml", "**/*.html", "**/fixtures/**"},
		FullEscalate: []string{"requirements.txt", "pyproject.toml", "**/conftest.py"},
	}
}

// fixtureSelectMap is the hand-written map that drives M1a.
func fixtureSelectMap() *mapstore.Map {
	return mapOf(
		mapstore.Row{T: "tests/test_auth.py::test_login", F: []string{"src/auth.py", "src/db.py"}, C: "aaa1111", D: 412, S: "pass"},
		mapstore.Row{T: "tests/test_auth.py::test_logout", F: []string{"src/auth.py"}, C: "aaa1111", D: 90, S: "fail"},
		mapstore.Row{T: "tests/test_db.py::test_query", F: []string{"src/db.py"}, C: "aaa1111", D: 15, S: "pass"},
		mapstore.Row{T: "tests/test_render.py::test_page", F: []string{"src/render.py", "templates/page.html"}, C: "aaa1111", D: 230, S: "pass"},
	)
}

func mod(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Modified, Lines: []gitctx.LineRange{{Start: 1, End: 1}}}
}

func added(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Added, Lines: []gitctx.LineRange{{Start: 1, End: 3}}}
}

func deleted(p string) gitctx.Change {
	return gitctx.Change{Path: p, Status: gitctx.Deleted}
}

// fresh reports every recorded commit as zero commits old.
func fresh(string) int { return 0 }

func baseInputs() Inputs {
	return Inputs{
		Map:      fixtureSelectMap(),
		Adapter:  fixtureAdapter(),
		Cfg:      DefaultConfig(),
		Distance: fresh,
	}
}

// The regression that made v1's contribution #2 unreachable: a test the agent just
// wrote has no map row, so it belonged to no tier and never ran.
func TestSelectPutsANewTestFileInTheDirectTierFirst(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{added("tests/test_brand_new.py")}

	got := Select(in)

	if !reflect.DeepEqual(got.Direct, []string{"tests/test_brand_new.py"}) {
		t.Fatalf("Direct = %#v, want the newly written test file", got.Direct)
	}
	if len(got.Tests) == 0 || got.Tests[0] != "tests/test_brand_new.py" {
		t.Fatalf("Tests = %#v, want the direct test FIRST", got.Tests)
	}
	if got.Tier != TierDirect {
		t.Errorf("Tier = %v, want TierDirect", got.Tier)
	}
	if got.Reason == "" {
		t.Error("Reason is empty; every Selection must explain itself")
	}
}

func TestSelectDirectTestsPrecedeMappedTests(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{
		added("tests/test_brand_new.py"),
		mod("src/auth.py"),
	}

	got := Select(in)

	want := []string{
		"tests/test_brand_new.py",          // direct, first, despite having no map row
		"tests/test_auth.py::test_logout",  // ratio 1.0, last-failed
		"tests/test_auth.py::test_login",   // ratio 0.5
	}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("Tests = %#v, want %#v", got.Tests, want)
	}
	if got.Tier != TierT0 {
		t.Errorf("Tier = %v, want TierT0", got.Tier)
	}
}

func TestSelectAChangedTestFileIsDirectEvenWhenItHasAMapRow(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("tests/test_auth.py")}

	got := Select(in)

	if !reflect.DeepEqual(got.Direct, []string{"tests/test_auth.py"}) {
		t.Errorf("Direct = %#v, want [tests/test_auth.py]", got.Direct)
	}
	if got.Tests[0] != "tests/test_auth.py" {
		t.Errorf("Tests[0] = %q, want the changed test file", got.Tests[0])
	}
}

func TestSelectADeletedTestFileIsNotRunDirectly(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{deleted("tests/test_auth.py")}

	got := Select(in)

	if len(got.Direct) != 0 {
		t.Errorf("Direct = %#v, want empty: a deleted test file cannot be executed", got.Direct)
	}
}

func TestSelectT0(t *testing.T) {
	tests := []struct {
		name      string
		changes   []gitctx.Change
		wantTier  Tier
		wantTests []string
	}{
		{
			name:      "one source file selects its two tests, ranked",
			changes:   []gitctx.Change{mod("src/auth.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_auth.py::test_logout", "tests/test_auth.py::test_login"},
		},
		{
			name:      "two source files union their tests",
			changes:   []gitctx.Change{mod("src/auth.py"), mod("src/db.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_auth.py::test_logout", "tests/test_db.py::test_query", "tests/test_auth.py::test_login"},
		},
		{
			name:      "a deleted source file still selects the tests whose F contains it",
			changes:   []gitctx.Change{deleted("src/render.py")},
			wantTier:  TierT0,
			wantTests: []string{"tests/test_render.py::test_page"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.Changes = tc.changes
			got := Select(in)
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %v (%s), want %v", got.Tier, got.Reason, tc.wantTier)
			}
			if !reflect.DeepEqual(got.Tests, tc.wantTests) {
				t.Errorf("Tests = %#v, want %#v", got.Tests, tc.wantTests)
			}
		})
	}
}

func TestSelectRenameSelectsViaTheOldPathToo(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{{
		Path:    "src/authentication.py",
		OldPath: "src/auth.py",
		Status:  gitctx.Renamed,
		Lines:   []gitctx.LineRange{{Start: 1, End: 10}},
	}}

	got := Select(in)

	want := []string{"tests/test_auth.py::test_logout", "tests/test_auth.py::test_login"}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("Tests = %#v, want %#v (the OLD path must still select its tests)", got.Tests, want)
	}
}

// TierEmpty is a distinct outcome with its own Reason. It must never be reported as a
// tier that ran and passed.
func TestSelectTierEmptyIsExplicit(t *testing.T) {
	tests := []struct {
		name    string
		changes []gitctx.Change
	}{
		{"a source file no test covers", []gitctx.Change{mod("src/orphan.py")}},
		{"nothing changed at all", nil},
		{"a file outside every glob", []gitctx.Change{mod("README.md")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.Changes = tc.changes
			got := Select(in)
			if got.Tier != TierEmpty {
				t.Fatalf("Tier = %v, want TierEmpty", got.Tier)
			}
			if len(got.Tests) != 0 {
				t.Errorf("Tests = %#v, want empty", got.Tests)
			}
			if got.Reason == "" {
				t.Error("TierEmpty with an empty Reason: an empty selection must say why")
			}
		})
	}
}

func TestSelectToleratesNilMapAndNilAdapter(t *testing.T) {
	got := Select(Inputs{Changes: []gitctx.Change{mod("src/auth.py")}, Cfg: DefaultConfig()})
	if got.Tier != TierT2 {
		t.Errorf("Tier = %v, want TierT2 (a nil/empty map is unseeded)", got.Tier)
	}
	if got.Reason == "" {
		t.Error("Reason is empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestSelect -v`
Expected: FAIL — `internal/selector/select_test.go:...: undefined: Select` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/selector/select.go`:

```go
package selector

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Select computes the tier and the ranked test list.
//
// Order of evaluation, and it matters:
//  1. the direct set, BEFORE any map lookup — a test the agent just wrote has no map row,
//     and in v1 it was therefore in no tier and never ran;
//  2. T2 escalations (unseeded map, full-escalate file, drift guard);
//  3. T1 escalations (merge commit, opaque file, import-time-only file, stale row);
//  4. T0;
//  5. TierEmpty, reported explicitly with a Reason.
func Select(in Inputs) Selection {
	m := in.Map
	if m == nil {
		m = mapstore.New()
	}
	cfg := in.Cfg
	if cfg == (Config{}) {
		cfg = DefaultConfig()
	}

	changedFiles := make([]string, 0, len(in.Changes)*2)
	for _, c := range in.Changes {
		changedFiles = append(changedFiles, c.Path)
		if c.OldPath != "" {
			changedFiles = append(changedFiles, c.OldPath)
		}
	}
	sort.Strings(changedFiles)
	changedFiles = dedupe(changedFiles)

	// (1) direct tier, first, with no map lookup at all.
	direct := make([]string, 0, len(in.Changes))
	for _, c := range in.Changes {
		if c.Status == gitctx.Deleted {
			continue // a deleted test file cannot be executed
		}
		if in.Adapter.IsTestFile(c.Path) {
			direct = append(direct, c.Path)
		}
	}
	sort.Strings(direct)
	direct = dedupe(direct)

	// (2) T2.
	if reason, escalate := escalateFull(in, m, cfg); escalate {
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: reason,
		}
	}

	t0 := m.TestsCovering(changedFiles)

	// (3) T1.
	if extra, reason := escalateT1(in, m, cfg, t0); reason != "" {
		ranked := Rank(m, dedupe(append(append([]string{}, t0...), extra...)), changedFiles)
		tests := mergeFirst(direct, ranked)
		if len(tests) == 0 {
			return Selection{
				Tier:   TierEmpty,
				Direct: direct,
				Reason: reason + "; but nothing in the map covers the changed set",
			}
		}
		return Selection{Tier: TierT1, Direct: direct, Tests: tests, Reason: reason}
	}

	// (4) T0, and (5) empty.
	ranked := Rank(m, t0, changedFiles)
	tests := mergeFirst(direct, ranked)
	switch {
	case len(tests) == 0:
		return Selection{
			Tier:   TierEmpty,
			Direct: direct,
			Reason: "no test in the map covers the changed set, and no test file changed",
		}
	case len(ranked) == 0:
		return Selection{
			Tier:   TierDirect,
			Direct: direct,
			Tests:  tests,
			Reason: "changed test files only; no mapped test covers the changed set",
		}
	default:
		return Selection{
			Tier:   TierT0,
			Direct: direct,
			Tests:  tests,
			Reason: "tests whose recorded coverage intersects the changed set",
		}
	}
}

func escalateFull(in Inputs, m *mapstore.Map, cfg Config) (string, bool) {
	if m.Len() == 0 {
		return "the map is unseeded, so no selection is trustworthy: run rtdd seed", true
	}
	for _, c := range in.Changes {
		if in.Adapter.IsFullEscalate(c.Path) {
			return "full-escalate file changed: " + c.Path, true
		}
	}
	if cfg.DriftGuard > 0 && in.Cycles >= cfg.DriftGuard {
		return fmt.Sprintf("drift guard reached: %d cycles since the last full run (limit %d)",
			in.Cycles, cfg.DriftGuard), true
	}
	return "", false
}

// escalateT1 returns the tests T1 adds to T0, and the reason for the escalation.
// An empty reason means no escalation.
func escalateT1(in Inputs, m *mapstore.Map, cfg Config, t0 []string) (extra []string, reason string) {
	if in.Merge {
		reason = "HEAD is a merge commit: a union-merged map can be stale relative to the merged code"
	}

	for _, c := range in.Changes {
		if !in.Adapter.IsOpaque(c.Path) {
			continue
		}
		extra = append(extra, m.TestsCovering(filesUnder(m, path.Dir(c.Path)))...)
		if reason == "" {
			reason = "opaque file changed (coverage cannot see inside it): " + c.Path
		}
	}

	if in.ImportOnly != nil {
		for _, c := range in.Changes {
			ids := in.ImportOnly(c.Path)
			if len(ids) == 0 {
				continue
			}
			extra = append(extra, ids...)
			if reason == "" {
				reason = "import-time-only file changed: " + c.Path
			}
		}
	}

	if in.Distance != nil && cfg.StaleCommits > 0 {
		for _, id := range t0 {
			r, ok := m.Get(id)
			if !ok {
				continue
			}
			d := in.Distance(r.C)
			if d < 0 {
				if reason == "" {
					reason = fmt.Sprintf("row %s records commit %q, which is unreachable: "+
						"age unknown, treated as stale", id, r.C)
				}
				break
			}
			if d > cfg.StaleCommits {
				if reason == "" {
					reason = fmt.Sprintf("row %s is %d commits stale (limit %d)", id, d, cfg.StaleCommits)
				}
				break
			}
		}
	}

	return dedupe(extra), reason
}

// filesUnder returns every file in the map that lives under dir.
func filesUnder(m *mapstore.Map, dir string) []string {
	out := []string{}
	prefix := dir + "/"
	for f := range m.FanOut() {
		if dir == "." || strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// mergeFirst returns first, then every member of rest not already present.
// This is what keeps the direct tier ahead of everything else.
func mergeFirst(first, rest []string) []string {
	out := make([]string, 0, len(first)+len(rest))
	seen := make(map[string]struct{}, len(first)+len(rest))
	for _, group := range [][]string{first, rest} {
		for _, s := range group {
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -run TestSelect -v`
Expected: PASS, all subtests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/selector/select.go internal/selector/select_test.go
git commit -m "M1a: selector.Select with the direct tier first and an explicit TierEmpty"
```

---

### Task 18: internal/selector — T1 and T2 escalation cases

**Files:**
- Modify: none (proves Task 17's escalation paths)
- Test: `internal/selector/select_test.go` (append)

**Interfaces:**
- Consumes: `func Select(in Inputs) Selection`, `type Inputs`, `type Config`, `func DefaultConfig() Config`.
- Produces: nothing new.

Every escalation must carry a non-empty `Reason`. A `-1` distance escalates exactly like an
over-`StaleCommits` distance: unknown age is never treated as fresh.

- [ ] **Step 1: Write the failing test**

Append to `internal/selector/select_test.go`:

```go
func TestSelectEscalatesToT2(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Inputs)
		reasonHas  string
	}{
		{
			name: "unseeded map",
			mutate: func(in *Inputs) {
				in.Map = mapstore.New()
				in.Changes = []gitctx.Change{mod("src/auth.py")}
			},
			reasonHas: "unseeded",
		},
		{
			name: "dependency manifest changed",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py"), mod("requirements.txt")}
			},
			reasonHas: "requirements.txt",
		},
		{
			name: "test-harness config changed",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("tests/conftest.py")}
			},
			reasonHas: "conftest.py",
		},
		{
			name: "drift guard reached",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Cycles = 100
			},
			reasonHas: "drift guard",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_zebra.py::test_z"}
			tc.mutate(&in)
			got := Select(in)
			if got.Tier != TierT2 {
				t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
			}
			if !contains(got.Reason, tc.reasonHas) {
				t.Errorf("Reason = %q, want it to mention %q", got.Reason, tc.reasonHas)
			}
			if len(got.Tests) != len(in.AllTests) {
				t.Errorf("Tests = %#v, want the full suite %#v", got.Tests, in.AllTests)
			}
		})
	}
}

func TestSelectT2KeepsDirectTestsFirst(t *testing.T) {
	in := baseInputs()
	in.AllTests = []string{"tests/test_auth.py::test_login", "tests/test_db.py::test_query"}
	in.Changes = []gitctx.Change{mod("requirements.txt"), added("tests/test_brand_new.py")}

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want TierT2", got.Tier)
	}
	if got.Tests[0] != "tests/test_brand_new.py" {
		t.Errorf("Tests[0] = %q, want the direct test even at T2", got.Tests[0])
	}
}

func TestSelectEscalatesToT1(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Inputs)
		reasonHas string
		wantHas   string
	}{
		{
			name: "opaque file selects its directory's tests",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("templates/page.html")}
			},
			reasonHas: "opaque",
			wantHas:   "tests/test_render.py::test_page",
		},
		{
			name: "HEAD is a merge commit",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Merge = true
			},
			reasonHas: "merge commit",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "a row is staler than stale_commits",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Distance = func(string) int { return 51 }
			},
			reasonHas: "stale",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "an unreachable commit is unknown, never fresh",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/auth.py")}
				in.Distance = func(string) int { return -1 }
			},
			reasonHas: "unreachable",
			wantHas:   "tests/test_auth.py::test_logout",
		},
		{
			name: "import-time-only fallback contributes tests",
			mutate: func(in *Inputs) {
				in.Changes = []gitctx.Change{mod("src/constants.py")}
				in.ImportOnly = func(rel string) []string {
					if rel == "src/constants.py" {
						return []string{"tests/test_db.py::test_query"}
					}
					return nil
				}
			},
			reasonHas: "import-time-only",
			wantHas:   "tests/test_db.py::test_query",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := baseInputs()
			tc.mutate(&in)
			got := Select(in)
			if got.Tier != TierT1 {
				t.Fatalf("Tier = %v (%s), want TierT1", got.Tier, got.Reason)
			}
			if !contains(got.Reason, tc.reasonHas) {
				t.Errorf("Reason = %q, want it to mention %q", got.Reason, tc.reasonHas)
			}
			found := false
			for _, id := range got.Tests {
				if id == tc.wantHas {
					found = true
				}
			}
			if !found {
				t.Errorf("Tests = %#v, want it to contain %q", got.Tests, tc.wantHas)
			}
		})
	}
}

func TestSelectT1WithNothingToSelectIsStillEmpty(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("config/unmapped.yaml")}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Errorf("Tier = %v (%s), want TierEmpty: an escalation that selects nothing is still nothing",
			got.Tier, got.Reason)
	}
	if got.Reason == "" {
		t.Error("Reason is empty")
	}
}

// A fresh row must NOT escalate, or every selection becomes T1.
func TestSelectFreshRowsDoNotEscalate(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/auth.py")}
	in.Distance = func(string) int { return 50 } // exactly at the limit, not over it

	got := Select(in)

	if got.Tier != TierT0 {
		t.Errorf("Tier = %v (%s), want TierT0 at exactly stale_commits", got.Tier, got.Reason)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && strings.Contains(haystack, needle)
}
```

Add `"strings"` to the imports of `select_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestSelect -v`
Expected: PASS if Task 17 is correct. If any subtest FAILS, the message names the gap, e.g.
`Tier = T0 (tests whose recorded coverage intersects the changed set), want TierT1` for the
merge-commit case. Fix `escalateT1`/`escalateFull`, not the test.

- [ ] **Step 3: Write minimal implementation**

No new production code. If Step 2 failed, the fix belongs in `internal/selector/select.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/... -v`
Expected: PASS across paths, mapstore, gitctx, adapter, and selector.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add internal/selector/select_test.go
git commit -m "M1a: cover every T1/T2 escalation, including unreachable-commit staleness"
```

---

### Task 19: cmd/rtdd — dispatch, loadEnv, and `rtdd status`

**Files:**
- Create: `cmd/rtdd/main.go`, `cmd/rtdd/status.go`, `cmd/rtdd/testdata/map.jsonl`, `cmd/rtdd/testdata/adapter.yaml`
- Test: `cmd/rtdd/main_test.go`

**Interfaces:**
- Consumes:
  - `func gitctx.RepoRoot(start string) (string, error)`
  - `func gitctx.HeadSHA(repoRoot string) (string, error)`
  - `func gitctx.CommitDistance(repoRoot, sha string) (int, error)`
  - `func gitctx.IsMergeCommit(repoRoot, sha string) (bool, error)`
  - `func gitctx.Older(repoRoot string) func(a, b string) string`
  - `func mapstore.LoadWith(path string, older func(a, b string) string) (*mapstore.Map, error)`
  - `func mapstore.LoadMeta(path string) (mapstore.Meta, error)`
  - `func (m *mapstore.Map) Len() int`, `func (m *mapstore.Map) FanOut() map[string]int`
  - `func adapter.Load(path string) (*adapter.Adapter, error)`
  - `func selector.DefaultConfig() selector.Config`
- Produces (internal to `main`):
  - `func run(args []string, stdout, stderr io.Writer) int`
  - `type env struct { root, mapPath, metaPath, adPath string; m *mapstore.Map; meta mapstore.Meta; ad *adapter.Adapter }`
  - `func loadEnv(adapterPath string) (*env, int, error)`
  - `func cmdStatus(args []string, stdout, stderr io.Writer) int`

`cmd/` is the only place that prints. `main` is a one-liner around `run` so every command is
testable with in-memory writers and an asserted exit code.

- [ ] **Step 1: Write the failing test**

Create `cmd/rtdd/testdata/map.jsonl` — the hand-written fixture map that drives M1a. The
token `SEEDSHA` is rewritten to the test repo's real short SHA by the test helper.

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"SEEDSHA","d":412,"s":"pass"}
{"t":"tests/test_auth.py::test_logout","f":["src/auth.py"],"c":"SEEDSHA","d":90,"s":"fail"}
{"t":"tests/test_db.py::test_query","f":["src/db.py"],"c":"SEEDSHA","d":15,"s":"pass"}
{"t":"tests/test_render.py::test_page","f":["src/render.py","templates/page.html"],"c":"SEEDSHA","d":230,"s":"pass"}
```

Create `cmd/rtdd/testdata/adapter.yaml`:

```yaml
name: python
detect: ["pyproject.toml"]
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
opaque: ["**/*.yaml", "**/*.html", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "**/conftest.py"]
```

Create `cmd/rtdd/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// newTestRepo builds a real git repository containing the source tree the fixture map
// describes. Git is never mocked.
func newTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	gitRun(t, dir, "config", "user.email", "rtdd@example.com")
	gitRun(t, dir, "config", "user.name", "rtdd test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")

	writeFile(t, dir, "src/auth.py", "def login():\n    return 1\n")
	writeFile(t, dir, "src/db.py", "def query():\n    return 2\n")
	writeFile(t, dir, "src/render.py", "def page():\n    return 3\n")
	writeFile(t, dir, "templates/page.html", "<p>hi</p>\n")
	writeFile(t, dir, "tests/test_auth.py", "def test_login():\n    pass\n")
	writeFile(t, dir, "tests/test_db.py", "def test_query():\n    pass\n")
	writeFile(t, dir, "tests/test_render.py", "def test_page():\n    pass\n")
	// The test repo ignores .rtdd/ so the fixture map and adapter never show up in the
	// changed set and every assertion is about the code under test. A real host repo
	// commits .rtdd/map.jsonl instead; either way it selects nothing.
	writeFile(t, dir, ".gitignore", ".rtdd/\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "-m", "init")
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

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func headShort(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(gitRun(t, dir, "rev-parse", "--short", "HEAD"))
}

// installRTDD copies the fixture map, meta, and adapter into dir/.rtdd/, substituting
// seedSHA for the SEEDSHA token in the map and the meta.
func installRTDD(t *testing.T, dir, seedSHA string, cycles int) {
	t.Helper()
	raw, err := os.ReadFile("testdata/map.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".rtdd/map.jsonl", strings.ReplaceAll(string(raw), "SEEDSHA", seedSHA))

	ad, err := os.ReadFile("testdata/adapter.yaml")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, ".rtdd/adapter.yaml", string(ad))

	meta := `{"v":1,"adapter":"python","seeded_at":"` + seedSHA + `","cycles":` +
		strconv.Itoa(cycles) + "}\n"
	writeFile(t, dir, ".rtdd/meta.json", meta)
}

// rtdd runs the CLI with dir as the working directory.
func rtdd(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunWithNoArgsIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestRunWithAnUnknownCommandIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir, "frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
	}
}

func TestStatusOnASeededRepo(t *testing.T) {
	dir := newTestRepo(t)
	sha := headShort(t, dir)
	installRTDD(t, dir, sha, 7)

	code, stdout, stderr := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"adapter: python",
		"4 tests",
		"4 files",
		"cycles:  7 / 100",
		sha,
		"0 commits ago",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestStatusOnAnUnseededRepo(t *testing.T) {
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "UNSEEDED") {
		t.Errorf("status must say UNSEEDED when the map is empty:\n%s", stdout)
	}
	if !strings.Contains(stdout, "adapter: none") {
		t.Errorf("status must report a missing adapter explicitly:\n%s", stdout)
	}
}

// An unreachable seed commit is reported as UNREACHABLE, never as fresh.
func TestStatusReportsAnUnreachableSeedCommit(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, "deadbee", 0)

	code, stdout, _ := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "UNREACHABLE") {
		t.Errorf("status must flag an unreachable seed commit:\n%s", stdout)
	}
	if strings.Contains(stdout, "0 commits ago") {
		t.Errorf("status reported an unreachable commit as fresh:\n%s", stdout)
	}
}

func TestStatusOutsideAGitRepoExitsThree(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := rtdd(t, dir, "status")
	if code != 3 {
		t.Errorf("exit code = %d, want 3 (fatal environment error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a fatal environment error must say what went wrong")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/... -v`
Expected: FAIL — `cmd/rtdd/main_test.go:...: undefined: run` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `cmd/rtdd/main.go`:

```go
// Command rtdd reports which tests cover the code that just changed.
// This package is the only place in the engine that prints.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

const usage = `rtdd - relational test-driven development

usage:
  rtdd status [--adapter <path>]
  rtdd which  [--base <ref>] [--json] [--adapter <path>]

exit codes:
  0  success - an empty selection is a signal, not a failure
  1  a test failed
  2  usage or configuration error
  3  fatal environment error (git unavailable, unreadable coverage)
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "status":
		return cmdStatus(args[1:], stdout, stderr)
	case "which":
		return cmdWhich(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "rtdd: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

type env struct {
	root     string
	mapPath  string
	metaPath string
	adPath   string
	m        *mapstore.Map
	meta     mapstore.Meta
	ad       *adapter.Adapter // nil when no adapter file is present
}

// loadEnv resolves the repo root and loads .rtdd/. The returned int is the process exit
// code to use when err is non-nil.
func loadEnv(adapterPath string) (*env, int, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, 3, err
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		return nil, 3, fmt.Errorf("not inside a git work tree, or git is unavailable: %w", err)
	}

	e := &env{
		root:     root,
		mapPath:  filepath.Join(root, ".rtdd", "map.jsonl"),
		metaPath: filepath.Join(root, ".rtdd", "meta.json"),
		adPath:   adapterPath,
	}
	if e.adPath == "" {
		e.adPath = filepath.Join(".rtdd", "adapter.yaml")
	}

	// Duplicate `t` lines left by a union merge are resolved with real commit ages.
	if e.m, err = mapstore.LoadWith(e.mapPath, gitctx.Older(root)); err != nil {
		return nil, 2, err
	}
	if e.meta, err = mapstore.LoadMeta(e.metaPath); err != nil {
		return nil, 2, err
	}

	abs := e.adPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	if _, statErr := os.Stat(abs); statErr == nil {
		if e.ad, err = adapter.Load(abs); err != nil {
			return nil, 2, err
		}
	}
	return e, 0, nil
}
```

Create `cmd/rtdd/status.go`:

```go
package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
)

func cmdStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	adapterPath := fs.String("adapter", "", "path to the adapter YAML (default .rtdd/adapter.yaml)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	e, code, err := loadEnv(*adapterPath)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd status: %v\n", err)
		return code
	}

	cfg := selector.DefaultConfig()

	fmt.Fprintf(stdout, "repo:    %s\n", e.root)
	if e.ad != nil {
		fmt.Fprintf(stdout, "adapter: %s (%s)\n", e.ad.Name, e.adPath)
	} else {
		fmt.Fprintf(stdout, "adapter: none (%s not found) - file classification is disabled\n", e.adPath)
	}
	fmt.Fprintf(stdout, "map:     .rtdd/map.jsonl - %d tests, %d files\n", e.m.Len(), len(e.m.FanOut()))

	switch {
	case e.m.Len() == 0:
		fmt.Fprintf(stdout, "seed:    UNSEEDED - every selection escalates to T2 (full suite)\n")
	case e.meta.SeededAt == "":
		fmt.Fprintf(stdout, "seed:    unknown - no .rtdd/meta.json\n")
	default:
		d, derr := gitctx.CommitDistance(e.root, e.meta.SeededAt)
		switch {
		case derr != nil:
			fmt.Fprintf(stdout, "seed:    %s (age unknown: %v)\n", e.meta.SeededAt, derr)
		case d < 0:
			fmt.Fprintf(stdout, "seed:    %s (UNREACHABLE - rebased, squashed, or shallow clone; "+
				"treated as stale, never as fresh)\n", e.meta.SeededAt)
		default:
			fmt.Fprintf(stdout, "seed:    %s (%d commits ago, stale_commits=%d)\n",
				e.meta.SeededAt, d, cfg.StaleCommits)
		}
	}

	fmt.Fprintf(stdout, "cycles:  %d / %d drift guard\n", e.meta.Cycles, cfg.DriftGuard)

	if head, herr := gitctx.HeadSHA(e.root); herr == nil {
		merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
		if merge {
			fmt.Fprintf(stdout, "head:    %s (merge commit - selections escalate to T1)\n", head)
		} else {
			fmt.Fprintf(stdout, "head:    %s\n", head)
		}
	} else {
		fmt.Fprintf(stdout, "head:    none - the repository has no commits yet\n")
	}
	return 0
}
```

Note: `cmdWhich` does not exist yet, so this will not compile until Task 20. Add a
temporary stub at the bottom of `main.go` so Task 19's tests can run, and delete it in
Task 20:

```go
// TEMPORARY: replaced by cmd/rtdd/which.go in Task 20.
func cmdWhich(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "rtdd which: not implemented yet")
	return 2
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -v`
Expected: PASS, 6 tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd
git commit -m "M1a: rtdd status, command dispatch, and the hand-written fixture map"
```

---

### Task 20: cmd/rtdd — `rtdd which` human output

**Files:**
- Create: `cmd/rtdd/which.go`
- Modify: `cmd/rtdd/main.go` (delete the temporary `cmdWhich` stub)
- Test: `cmd/rtdd/main_test.go` (append)

**Interfaces:**
- Consumes:
  - `func loadEnv(adapterPath string) (*env, int, error)`
  - `func gitctx.ChangedSet(repoRoot, base string) ([]gitctx.Change, error)`
  - `func gitctx.CommitDistance(repoRoot, sha string) (int, error)`
  - `func gitctx.IsMergeCommit(repoRoot, sha string) (bool, error)`
  - `func gitctx.HeadSHA(repoRoot string) (string, error)`
  - `func selector.Select(in selector.Inputs) selector.Selection`
  - `func selector.DefaultConfig() selector.Config`
  - `func (t selector.Tier) String() string`
  - `func (s gitctx.Status) String() string`
- Produces (internal to `main`): `func cmdWhich(args []string, stdout, stderr io.Writer) int`

`rtdd which` runs nothing. It costs one map load and one git diff, and it is the primary
agent integration point. It exits 0 even when the selection is empty — an empty selection is
a signal, and the human output says so in as many words.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/main_test.go`:

```go
// The regression that killed v1: a test file written but never `git add`ed is invisible
// to `git diff`, so it was never selected and never run.
func TestWhichSelectsAJustWrittenUntrackedTestFile(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "tests/test_brand_new.py") {
		t.Fatalf("which did not select the just-written test file:\n%s", stdout)
	}
	if !strings.Contains(stdout, "direct:") {
		t.Errorf("which must report the direct tier:\n%s", stdout)
	}
}

func TestWhichRanksT0(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "tier:     T0") {
		t.Fatalf("want tier T0:\n%s", stdout)
	}
	logout := strings.Index(stdout, "tests/test_auth.py::test_logout")
	login := strings.Index(stdout, "tests/test_auth.py::test_login\n")
	if logout < 0 || login < 0 {
		t.Fatalf("both auth tests must be selected:\n%s", stdout)
	}
	if logout > login {
		t.Errorf("the last-failed, higher-ratio test must be listed first:\n%s", stdout)
	}
}

// An empty selection is exit 0 AND an explicit warning. It must never read as a pass.
func TestWhichEmptySelectionIsExitZeroAndSaysSo(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/orphan_module.py", "def orphan():\n    return 0\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (an empty selection is a signal, not a failure); stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "tier:     empty") {
		t.Fatalf("want tier empty:\n%s", stdout)
	}
	if !strings.Contains(stdout, "not a pass") {
		t.Errorf("an empty selection must be reported in words, not as silence:\n%s", stdout)
	}
	if !strings.Contains(stdout, "selected: 0") {
		t.Errorf("want an explicit zero count:\n%s", stdout)
	}
}

func TestWhichEscalatesOnAFullEscalateFile(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "requirements.txt", "pytest==9.0.3\n")

	code, stdout, _ := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "tier:     T2") {
		t.Errorf("want tier T2:\n%s", stdout)
	}
	if !strings.Contains(stdout, "requirements.txt") {
		t.Errorf("the reason must name the file that forced the escalation:\n%s", stdout)
	}
}

func TestWhichEscalatesWhenTheSeedCommitIsUnreachable(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, "deadbee", 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, _ := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "tier:     T1") {
		t.Errorf("an unreachable row commit is unknown age, which escalates:\n%s", stdout)
	}
}

func TestWhichReportsDeletionsAndRespectsBase(t *testing.T) {
	dir := newTestRepo(t)
	base := headShort(t, dir)
	installRTDD(t, dir, base, 0)

	if err := os.Remove(filepath.Join(dir, "src", "render.py")); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := rtdd(t, dir, "which", "--base", base)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "deleted   src/render.py") {
		t.Errorf("a deletion must be shown:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tests/test_render.py::test_page") {
		t.Errorf("a deleted path must still select the tests whose F contains it:\n%s", stdout)
	}
}

func TestWhichRejectsAnUnknownFlag(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "which", "--nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/... -run TestWhich -v`
Expected: FAIL — every subtest fails with `exit code = 2, want 0` and stderr
`rtdd which: not implemented yet`, from the Task 19 stub.

- [ ] **Step 3: Write minimal implementation**

Delete the temporary `cmdWhich` stub from `cmd/rtdd/main.go`, then create
`cmd/rtdd/which.go`:

```go
package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
)

func cmdWhich(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	base := fs.String("base", "HEAD", "base ref for the changed set")
	asJSON := fs.Bool("json", false, "emit machine-readable JSON")
	adapterPath := fs.String("adapter", "", "path to the adapter YAML (default .rtdd/adapter.yaml)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	e, code, err := loadEnv(*adapterPath)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return code
	}

	changes, err := gitctx.ChangedSet(e.root, *base)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 3
	}

	merge, _ := gitctx.IsMergeCommit(e.root, "HEAD")
	distance := func(sha string) int {
		d, derr := gitctx.CommitDistance(e.root, sha)
		if derr != nil {
			return -1 // unknown, never fresh
		}
		return d
	}

	sel := selector.Select(selector.Inputs{
		Map:      e.m,
		Changes:  changes,
		Adapter:  e.ad,
		Cfg:      selector.DefaultConfig(),
		Cycles:   e.meta.Cycles,
		Merge:    merge,
		Distance: distance,
	})

	if *asJSON {
		return emitWhichJSON(stdout, stderr, e, *base, changes, sel)
	}

	fmt.Fprintf(stdout, "base:     %s\n", *base)
	fmt.Fprintf(stdout, "changed:  %d files\n", len(changes))
	for _, c := range changes {
		if c.OldPath != "" {
			fmt.Fprintf(stdout, "  %-9s %s (from %s)\n", c.Status.String(), c.Path, c.OldPath)
		} else {
			fmt.Fprintf(stdout, "  %-9s %s\n", c.Status.String(), c.Path)
		}
	}
	fmt.Fprintf(stdout, "tier:     %s - %s\n", sel.Tier.String(), sel.Reason)
	fmt.Fprintf(stdout, "direct:   %d\n", len(sel.Direct))
	for _, id := range sel.Direct {
		fmt.Fprintf(stdout, "  %s\n", id)
	}
	fmt.Fprintf(stdout, "selected: %d\n", len(sel.Tests))
	for _, id := range sel.Tests {
		fmt.Fprintf(stdout, "  %s\n", id)
	}
	if sel.Tier == selector.TierEmpty {
		fmt.Fprintf(stdout, "\nNOTE: an empty selection is not a pass. Nothing was checked.\n")
	}
	if sel.Tier == selector.TierT2 && len(sel.Tests) == 0 {
		fmt.Fprintf(stdout, "\nNOTE: T2 means the full suite. rtdd does not enumerate it in M1a "+
			"(adapter.List is M1b), so no test ids are listed.\n")
	}
	return 0
}
```

`emitWhichJSON` is added in Task 21. Until then, add this placeholder at the bottom of
`which.go` so the package compiles, and replace it in Task 21:

```go
// TEMPORARY: replaced by the real implementation in Task 21.
func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change, sel selector.Selection) int {
	fmt.Fprintln(stderr, "rtdd which --json: not implemented yet")
	return 2
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -v`
Expected: PASS, all `status` and `which` tests.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/which.go cmd/rtdd/main.go cmd/rtdd/main_test.go
git commit -m "M1a: rtdd which human output, direct tier first, empty selection reported"
```

---

### Task 21: cmd/rtdd — `rtdd which --json`

**Files:**
- Modify: `cmd/rtdd/which.go` (replace the temporary `emitWhichJSON` placeholder)
- Test: `cmd/rtdd/main_test.go` (append)

**Interfaces:**
- Consumes: `type env`, `type selector.Selection`, `type gitctx.Change`, `func (s gitctx.Status) String() string`, `func (t selector.Tier) String() string`, `func gitctx.HeadSHA(repoRoot string) (string, error)`.
- Produces (internal to `main`):
  - `type whichJSON struct { Base, Head, Tier, Reason string; Direct, Tests []string; Changed []jsonChange; MapTests int }`
  - `type jsonChange struct { Path, OldPath, Status string; Lines []jsonLineRange }`
  - `type jsonLineRange struct { Start, End int }`
  - `func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change, sel selector.Selection) int`

This schema is **provisional in M1a**. Plan M2, task "JSON output", freezes it; the agent
front-ends bind to the frozen version, not this one. Slices are always emitted as arrays,
never `null`, so a consumer never has to special-case an absent key.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/main_test.go`:

```go
type whichJSONForTest struct {
	Base    string `json:"base"`
	Head    string `json:"head"`
	Tier    string `json:"tier"`
	Reason  string `json:"reason"`
	Direct  []string `json:"direct"`
	Tests   []string `json:"tests"`
	Changed []struct {
		Path    string `json:"path"`
		OldPath string `json:"old_path"`
		Status  string `json:"status"`
		Lines   []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"lines"`
	} `json:"changed"`
	MapTests int `json:"map_tests"`
}

func decodeWhichJSON(t *testing.T, s string) whichJSONForTest {
	t.Helper()
	var out whichJSONForTest
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("which --json emitted unparseable JSON: %v\n%s", err, s)
	}
	return out
}

func TestWhichJSON(t *testing.T) {
	dir := newTestRepo(t)
	sha := headShort(t, dir)
	installRTDD(t, dir, sha, 3)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeWhichJSON(t, stdout)

	if got.Tier != "T0" {
		t.Errorf("tier = %q, want T0 (reason: %s)", got.Tier, got.Reason)
	}
	if got.Base != "HEAD" {
		t.Errorf("base = %q, want HEAD", got.Base)
	}
	if got.Head != sha {
		t.Errorf("head = %q, want %q", got.Head, sha)
	}
	if got.MapTests != 4 {
		t.Errorf("map_tests = %d, want 4", got.MapTests)
	}
	if len(got.Direct) != 1 || got.Direct[0] != "tests/test_brand_new.py" {
		t.Errorf("direct = %#v, want [tests/test_brand_new.py]", got.Direct)
	}
	want := []string{
		"tests/test_brand_new.py",
		"tests/test_auth.py::test_logout",
		"tests/test_auth.py::test_login",
	}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("tests = %#v, want %#v", got.Tests, want)
	}
	if got.Reason == "" {
		t.Error("reason is empty; every selection must explain itself")
	}
}

func TestWhichJSONReportsChangedLineRanges(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := decodeWhichJSON(t, stdout)

	if len(got.Changed) != 1 {
		t.Fatalf("changed = %#v, want exactly one entry", got.Changed)
	}
	c := got.Changed[0]
	if c.Path != "src/auth.py" || c.Status != "modified" {
		t.Errorf("changed[0] = %+v, want src/auth.py modified", c)
	}
	if len(c.Lines) != 1 || c.Lines[0].Start != 2 || c.Lines[0].End != 2 {
		t.Errorf("lines = %#v, want [{2 2}]", c.Lines)
	}
}

// Every slice is an array, never null: an agent consumer must not have to special-case
// an absent key.
func TestWhichJSONEmptySelectionEmitsArraysNotNull(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/orphan_module.py", "def orphan():\n    return 0\n")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (an empty selection is a signal, not a failure)", code)
	}
	if strings.Contains(stdout, "null") {
		t.Errorf("which --json emitted null:\n%s", stdout)
	}
	got := decodeWhichJSON(t, stdout)
	if got.Tier != "empty" {
		t.Errorf("tier = %q, want empty", got.Tier)
	}
	if len(got.Tests) != 0 {
		t.Errorf("tests = %#v, want empty", got.Tests)
	}
}

func TestWhichJSONReportsARename(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	if err := os.Rename(
		filepath.Join(dir, "src", "render.py"),
		filepath.Join(dir, "src", "renderer.py"),
	); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := decodeWhichJSON(t, stdout)

	found := false
	for _, c := range got.Changed {
		if c.Path == "src/renderer.py" && c.OldPath == "src/render.py" {
			found = true
		}
	}
	if !found {
		t.Errorf("changed = %#v, want a rename carrying old_path", got.Changed)
	}
	hit := false
	for _, id := range got.Tests {
		if id == "tests/test_render.py::test_page" {
			hit = true
		}
	}
	if !hit {
		t.Errorf("tests = %#v, want the old path to still select its test", got.Tests)
	}
}
```

Add `"encoding/json"` and `"reflect"` to the imports of `main_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/... -run TestWhichJSON -v`
Expected: FAIL — every subtest fails with `exit code = 2, want 0` and stderr
`rtdd which --json: not implemented yet`, from the Task 20 placeholder.

- [ ] **Step 3: Write minimal implementation**

Replace the temporary `emitWhichJSON` placeholder at the bottom of `cmd/rtdd/which.go`
with:

```go
type jsonLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type jsonChange struct {
	Path    string          `json:"path"`
	OldPath string          `json:"old_path"`
	Status  string          `json:"status"`
	Lines   []jsonLineRange `json:"lines"`
}

// whichJSON is the machine-readable form of a selection. PROVISIONAL in M1a:
// plan M2, task "JSON output", freezes this schema, and the agent front-ends bind to
// the frozen version.
type whichJSON struct {
	Base     string       `json:"base"`
	Head     string       `json:"head"`
	Tier     string       `json:"tier"`
	Reason   string       `json:"reason"`
	Direct   []string     `json:"direct"`
	Tests    []string     `json:"tests"`
	Changed  []jsonChange `json:"changed"`
	MapTests int          `json:"map_tests"`
}

func emitWhichJSON(stdout, stderr io.Writer, e *env, base string, changes []gitctx.Change, sel selector.Selection) int {
	head, _ := gitctx.HeadSHA(e.root)
	out := whichJSON{
		Base:     base,
		Head:     head,
		Tier:     sel.Tier.String(),
		Reason:   sel.Reason,
		Direct:   nonNilStrings(sel.Direct),
		Tests:    nonNilStrings(sel.Tests),
		Changed:  make([]jsonChange, 0, len(changes)),
		MapTests: e.m.Len(),
	}
	for _, c := range changes {
		jc := jsonChange{
			Path:    c.Path,
			OldPath: c.OldPath,
			Status:  c.Status.String(),
			Lines:   make([]jsonLineRange, 0, len(c.Lines)),
		}
		for _, r := range c.Lines {
			jc.Lines = append(jc.Lines, jsonLineRange{Start: r.Start, End: r.End})
		}
		out.Changed = append(out.Changed, jc)
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "rtdd which: %v\n", err)
		return 2
	}
	return 0
}

// nonNilStrings guarantees a JSON array rather than null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
```

Add `"encoding/json"` to the imports of `which.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS across every package.

- [ ] **Step 5: Commit**

```bash
cd /home/claude/rtdd
git add cmd/rtdd/which.go cmd/rtdd/main_test.go
git commit -m "M1a: rtdd which --json (provisional schema, arrays never null)"
```

---

## Definition of Done

The milestone is complete when every box below is checked.

**Build and hygiene**

- [ ] `go build ./...` succeeds.
- [ ] `go vet ./...` is clean.
- [ ] `gofmt -l .` prints nothing.
- [ ] `go test ./... -count=1` passes.
- [ ] `go test ./... -race -count=1` passes.
- [ ] `go list -m all` lists exactly two modules: `github.com/VocanicZ/rtdd` and `gopkg.in/yaml.v3`.
- [ ] `go.mod` requires exactly one non-stdlib module: `gopkg.in/yaml.v3`.
- [ ] No package under `internal/` prints to stdout or stderr: `grep -rn 'fmt.Print\|os.Stdout\|os.Stderr' internal/ --include='*.go' | grep -v _test.go` is empty.

**Contract**

- [ ] Every exported name in `internal/paths`, `internal/mapstore`, `internal/gitctx`, `internal/selector`, and the M1a half of `internal/adapter` matches `docs/plans/00-interfaces.md` exactly.
- [ ] The only edit to `00-interfaces.md` is the "M1a amendments" section added in Task 1.
- [ ] `internal/coverage`, `internal/report`, `internal/runner`, `internal/uncovered`, `internal/doctor` do not exist yet — M1a is adapter-free by design.

**The hard cases from `docs/audits/2026-08-26-design-audit.md`**

- [ ] `mapstore.Load` resolves duplicate `t` lines by set-union of `F` with the **older** commit winning for `C` (`TestLoadResolvesDuplicateTLinesLikeUnion`).
- [ ] A malformed line is a **fatal error**, never a silent skip (`TestLoadIsFatalOnAMalformedLine`).
- [ ] Two rows joined by a missing newline are detected and fatal (`TestLoadIsFatalOnTwoRowsJoinedByAMissingNewline`).
- [ ] `Save` always emits a trailing newline (`TestSaveEmitsSortedLinesWithTrailingNewline`).
- [ ] Compaction is `LoadWith` + `Save`, and it is idempotent (`TestCompactionRoundTrip`).
- [ ] `F` unions and never replaces outside `Replace` (`TestUnion`, case "F is the set union, never a replacement").
- [ ] `gitctx.ChangedSet` includes untracked files (`TestChangedSetIncludesUntrackedFiles`) — v1's flagship bug.
- [ ] `ChangedSet` retains deletions (`TestChangedSetRetainsDeletions`) and carries `OldPath` for renames (`TestChangedSetHandlesRenames`).
- [ ] `ChangedSet` returns per-hunk line ranges at `--unified=0` (`TestChangedSetLineRanges`).
- [ ] `gitctx.CommitDistance` returns `-1` for an unreachable SHA (`TestCommitDistanceUnreachableIsMinusOne`) and callers escalate on it rather than treating it as fresh (`TestSelectEscalatesToT1`, case "an unreachable commit is unknown, never fresh"; `TestStatusReportsAnUnreachableSeedCommit`).
- [ ] `selector.Select` puts changed and new test files in `Direct` **before any map lookup**, and `Direct` leads `Tests` (`TestSelectPutsANewTestFileInTheDirectTierFirst`, `TestSelectDirectTestsPrecedeMappedTests`, `TestSelectT2KeepsDirectTestsFirst`).
- [ ] `TierEmpty` is a distinct outcome with a non-empty `Reason`, never collapsed into success (`TestSelectTierEmptyIsExplicit`, `TestWhichEmptySelectionIsExitZeroAndSaysSo`).
- [ ] `Rank` orders by descending `|F ∩ changed|/|F|`, then `S=="fail"` first, then ascending `len(F)`, then ascending `D` — each key covered in isolation (`TestRank`).
- [ ] Every git-dependent test uses a real `git init` in `t.TempDir()`; no git mock exists anywhere in the tree.

**Behaviour**

- [ ] `rtdd status` reports adapter, map size, seed freshness (including UNREACHABLE and UNSEEDED), cycles against the drift guard, and HEAD, and exits 0.
- [ ] `rtdd status` outside a git work tree exits **3**.
- [ ] `rtdd which` prints the changed set, the tier with its reason, the direct list, and the ranked selection, and exits **0** for every tier including `empty`.
- [ ] `rtdd which --base <ref>` honours an explicit base.
- [ ] `rtdd which --json` emits arrays rather than `null` for `direct`, `tests`, `changed`, and `lines`.
- [ ] An unknown command or an unknown flag exits **2**.
- [ ] Nothing in M1a ever exits **1** — no test is executed in this milestone.

**Ready for M1b**

- [ ] `adapters/python.yaml` parses through `adapter.Load` with `KnownFields(true)`, including `seed`, `subset`, `list`, and `exit_codes`, even though M1a executes none of them.
- [ ] `selector.Inputs.AllTests` and `selector.Inputs.ImportOnly` are wired through `Select` and covered by tests, so M1b (`adapter.List`) and M2 (import-time fallback) only have to supply values.
