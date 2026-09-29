# One Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace rtdd's two selection pipelines (Python per-test coverage contexts, static tier for everything else) with one: every test file runs in its own process under the language's stock coverage tool, and the map is built from what those runs executed.

**Architecture:** A new `internal/covfmt` package parses four coverage formats and resolves reported paths to repo-relative ones. `internal/runner` is rewritten to enumerate units (test files) with `gitctx.ListFiles`, run each in isolation with a private `{tmp}`, read its coverage file, and take pass/fail from the exit code — while keeping the `runner.RunResult` / `coverage.Result` shapes `cmd/rtdd` already consumes, so the command layer changes little. The static tier, the report parsers, the sqlite reader and the import scan are then deleted.

**Tech Stack:** Go 1.x (module `github.com/VocanicZ/rtdd`), `gopkg.in/yaml.v3`, `encoding/xml`. Integration fixtures use real toolchains (go, pytest+pytest-cov, node, cargo+cargo-llvm-cov, maven, dotnet, php, ruby) and skip when absent.

**Spec:** [`docs/specs/2026-09-29-one-pipeline.md`](../specs/2026-09-29-one-pipeline.md)

## Global Constraints

- One pipeline for every adapter. No per-language code path in Go; per-language knowledge lives only in adapter YAML (spec §1, §6).
- The unit is the test file; a map row is `{t: test file, f, c, d, s, a}` (spec §3, §4.5).
- Commands are split on whitespace before substitution, never run through a shell (spec §6).
- Placeholders in `unit_cmd`: `{unit}`, `{dir}`, `{name}`, `{names}`, `{tmp}`; `env` values may use `{tmp}` (spec §6).
- Exit 0 = pass, 1 = fail; other codes via `exit_codes`; unlisted = error; 0/1 with no coverage file = error, never pass (spec §4.4).
- Coverage formats: `lcov`, `cobertura`, `gocover`, `jacoco` (spec §5).
- Map format version is 2 (`meta.json` `v: 2`); an older map is treated as unseeded (spec §3).
- Removed adapter fields are rejected at load, naming the field (spec §6).
- `--json` schema becomes 2: no `selection_fidelity`, no `import_time_lines` (spec §10).
- Only `internal/gitctx` may shell out to git (existing contract test `TestOnlyGitctxShellsOutToGit`).

## Review Focus

- A unit whose run exits 0 but writes no coverage file (misconfigured `coverage_file`, tool not installed) — must be status `error`, and `rtdd run` must exit non-zero. Test in Task 4.
- Two units running in parallel writing the same coverage path (e.g. pytest's `.coverage` in CWD) — each unit must see only its own coverage. Test in Task 4 (parallel run, disjoint files asserted) and Task 5 (`COVERAGE_FILE={tmp}/.coverage`).
- A test file with no runnable tests (Go file with only helpers; pytest file collecting nothing) — must be `skip`, not a fatal abort of the whole seed. Tests in Task 4 (`{names}` empty) and Task 5 (pytest exit 5).
- A coverage path that matches two repo files by suffix (two `Bar.java` in different source roots, JaCoCo reports `com/foo/Bar.java`) — must resolve to the one whose full suffix matches, and be dropped if still ambiguous, never attributed to the wrong file. Test in Task 1.
- An existing v1 map (Python test-case ids) after upgrading — must select T2 with a reason telling the user to run `rtdd seed`, not crash and not select stale case ids. Test in Task 6.

---

## File Structure

| Path | Responsibility |
|---|---|
| `internal/covfmt/parse.go` | `Parse(format, r) (Lines, error)` — dispatch to the four parsers |
| `internal/covfmt/lcov.go`, `gocover.go`, `cobertura.go`, `jacoco.go` | one parser each |
| `internal/covfmt/resolve.go` | `Resolve(root, raw, repoFiles) map[string][]int` |
| `internal/gitctx/listfiles.go` | `ListFiles(root) ([]string, error)` — tracked + untracked, not ignored |
| `internal/adapter/adapter.go` | contract v3 fields, validation, removed-field rejection |
| `internal/adapter/unit.go` | `UnitArgv`, `UnitEnv`, `UnitNames`, `CoveragePath` |
| `internal/runner/run.go` | `Units`, `RunUnits`, `Seed`, `Run`, `Outcome`, `RunResult` |
| `adapters/*.yaml` | the ten shipped adapters, contract v3 |
| `internal/runner/testdata/gofix/` | a tiny Go module used by runner tests |
| `cmd/rtdd/*` | switch to the new runner; drop static/record/fidelity/import fallback |

---

### Task 1: `internal/covfmt` — four parsers and the path resolver

**Files:**
- Create: `internal/covfmt/parse.go`, `lcov.go`, `gocover.go`, `cobertura.go`, `jacoco.go`, `resolve.go`
- Test: `internal/covfmt/parse_test.go`, `internal/covfmt/resolve_test.go`

**Interfaces:**
- Produces: `type Lines map[string][]int`; `func Parse(format string, r io.Reader) (Lines, error)`; `func Resolve(root string, raw Lines, repoFiles []string) map[string][]int`; `var Formats = []string{"lcov", "cobertura", "gocover", "jacoco"}`.

- [ ] **Step 1: Write the failing parser tests**

```go
package covfmt

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		format, in string
		want       Lines
	}{
		{"lcov", "TN:\nSF:/r/src/a.py\nDA:1,1\nDA:2,0\nDA:3,4,abc\nend_of_record\nSF:src/b.py\nDA:7,1\nend_of_record\n",
			Lines{"/r/src/a.py": {1, 3}, "src/b.py": {7}}},
		{"gocover", "mode: set\nexample.com/m/calc/calc.go:3.20,5.2 1 1\nexample.com/m/calc/calc.go:7.20,9.2 1 0\nexample.com/m/util.go:1.1,2.2 1 3\n",
			Lines{"example.com/m/calc/calc.go": {3, 4, 5}, "example.com/m/util.go": {1, 2}}},
		{"cobertura", `<?xml version="1.0"?><coverage><sources><source>/r/src</source></sources><packages><package><classes>
<class filename="Calc.cs"><lines><line number="4" hits="2"/><line number="5" hits="0"/></lines></class>
</classes></package></packages></coverage>`,
			Lines{"/r/src/Calc.cs": {4}}},
		{"jacoco", `<?xml version="1.0"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd"><report name="x"><package name="com/foo">
<sourcefile name="Bar.java"><line nr="3" mi="0" ci="2" mb="0" cb="0"/><line nr="4" mi="1" ci="0" mb="0" cb="0"/></sourcefile>
</package></report>`,
			Lines{"com/foo/Bar.java": {3}}},
	}
	for _, c := range cases {
		t.Run(c.format, func(t *testing.T) {
			got, err := Parse(c.format, strings.NewReader(c.in))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%s) = %v, want %v", c.format, got, c.want)
			}
		})
	}
}

func TestParseRejectsUnknownFormatAndGarbage(t *testing.T) {
	if _, err := Parse("sqlite", strings.NewReader("")); err == nil {
		t.Error("unknown format parsed without error")
	}
	if _, err := Parse("cobertura", strings.NewReader("<not-xml")); err == nil {
		t.Error("malformed cobertura parsed without error")
	}
	if _, err := Parse("gocover", strings.NewReader("no mode line\n")); err == nil {
		t.Error("gocover without a mode line parsed without error")
	}
}
```

- [ ] **Step 2: Write the failing resolver test**

```go
package covfmt

import (
	"reflect"
	"testing"
)

func TestResolve(t *testing.T) {
	files := []string{
		"calc/calc.go", "util.go",
		"src/main/java/com/foo/Bar.java",
		"a/com/dup/X.java", "b/com/dup/X.java",
		"src/a.py",
	}
	raw := Lines{
		"/r/src/a.py":                {1, 3},
		"./util.go":                  {2},
		"example.com/m/calc/calc.go": {3, 4},
		"com/foo/Bar.java":           {5},
		"com/dup/X.java":             {6}, // ambiguous: dropped
		"/elsewhere/lib.py":          {1}, // outside the root: dropped
		"src/a.py":                   {3, 9}, // merges with the absolute spelling
	}
	got := Resolve("/r", raw, files)
	want := map[string][]int{
		"src/a.py":                       {1, 3, 9},
		"util.go":                        {2},
		"calc/calc.go":                   {3, 4},
		"src/main/java/com/foo/Bar.java": {5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v\nwant %v", got, want)
	}
}
```

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/covfmt/`
Expected: FAIL — package has no non-test Go files / `undefined: Parse`.

- [ ] **Step 4: Implement**

`internal/covfmt/parse.go`:

```go
// Package covfmt reads the coverage files stock tools write — lcov, cobertura, Go's
// coverprofile and JaCoCo XML — into "path -> lines hit", and resolves the paths those
// tools report to repo-relative ones. It knows nothing about tests: one file is one unit's
// coverage, because every unit runs in its own process (spec §4).
package covfmt

import (
	"fmt"
	"io"
	"sort"
)

// Lines maps a path, as the coverage tool spelled it, to the lines it hit (count > 0).
type Lines map[string][]int

var Formats = []string{"lcov", "cobertura", "gocover", "jacoco"}

func Parse(format string, r io.Reader) (Lines, error) {
	var (
		out Lines
		err error
	)
	switch format {
	case "lcov":
		out, err = parseLcov(r)
	case "gocover":
		out, err = parseGocover(r)
	case "cobertura":
		out, err = parseCobertura(r)
	case "jacoco":
		out, err = parseJacoco(r)
	default:
		return nil, fmt.Errorf("covfmt: unknown format %q (only %v)", format, Formats)
	}
	if err != nil {
		return nil, fmt.Errorf("covfmt: %s: %w", format, err)
	}
	for p, ls := range out {
		out[p] = sortedUnique(ls)
	}
	return out, nil
}

func (l Lines) add(path string, lines ...int) {
	l[path] = append(l[path], lines...)
}

func sortedUnique(ls []int) []int {
	sort.Ints(ls)
	out := ls[:0]
	for i, v := range ls {
		if i == 0 || v != ls[i-1] {
			out = append(out, v)
		}
	}
	return out
}
```

`internal/covfmt/lcov.go`:

```go
package covfmt

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// parseLcov reads SF:/DA: records. DA is "line,count[,checksum]".
func parseLcov(r io.Reader) (Lines, error) {
	out := Lines{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	file := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			file = strings.TrimPrefix(line, "SF:")
		case line == "end_of_record":
			file = ""
		case strings.HasPrefix(line, "DA:") && file != "":
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) < 2 {
				continue
			}
			n, err1 := strconv.Atoi(parts[0])
			c, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err1 == nil && err2 == nil && c > 0 {
				out.add(file, n)
			}
		}
	}
	return out, sc.Err()
}
```

`internal/covfmt/gocover.go`:

```go
package covfmt

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// parseGocover reads `go test -coverprofile` output:
// "mode: set" then "path:startLine.startCol,endLine.endCol numStmts count".
// A block with count > 0 hits every line from startLine to endLine.
func parseGocover(r io.Reader) (Lines, error) {
	out := Lines{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	if !sc.Scan() || !strings.HasPrefix(sc.Text(), "mode:") {
		return nil, errors.New("missing mode line")
	}
	for sc.Scan() {
		line := sc.Text()
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) != 3 {
			continue
		}
		if c, err := strconv.ParseInt(fields[2], 10, 64); err != nil || c == 0 {
			continue
		}
		span := strings.SplitN(fields[0], ",", 2)
		if len(span) != 2 {
			continue
		}
		start, err1 := strconv.Atoi(strings.SplitN(span[0], ".", 2)[0])
		end, err2 := strconv.Atoi(strings.SplitN(span[1], ".", 2)[0])
		if err1 != nil || err2 != nil || end < start {
			continue
		}
		for n := start; n <= end; n++ {
			out.add(line[:colon], n)
		}
	}
	return out, sc.Err()
}
```

`internal/covfmt/cobertura.go`:

```go
package covfmt

import (
	"encoding/xml"
	"io"
	"path"
)

type coberturaDoc struct {
	Sources []string `xml:"sources>source"`
	Classes []struct {
		Filename string `xml:"filename,attr"`
		Lines    []struct {
			Number int   `xml:"number,attr"`
			Hits   int64 `xml:"hits,attr"`
		} `xml:"lines>line"`
	} `xml:"packages>package>classes>class"`
}

// parseCobertura joins each class filename to the report's single <source> when there is
// exactly one; with several, the filename is left for Resolve's suffix match.
func parseCobertura(r io.Reader) (Lines, error) {
	var doc coberturaDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}
	out := Lines{}
	for _, c := range doc.Classes {
		p := c.Filename
		if len(doc.Sources) == 1 && !path.IsAbs(p) {
			p = path.Join(doc.Sources[0], p)
		}
		for _, l := range c.Lines {
			if l.Hits > 0 {
				out.add(p, l.Number)
			}
		}
	}
	return out, nil
}
```

`internal/covfmt/jacoco.go`:

```go
package covfmt

import (
	"encoding/xml"
	"io"
)

type jacocoDoc struct {
	Packages []struct {
		Name  string `xml:"name,attr"`
		Files []struct {
			Name  string `xml:"name,attr"`
			Lines []struct {
				Nr int `xml:"nr,attr"`
				Ci int `xml:"ci,attr"`
			} `xml:"line"`
		} `xml:"sourcefile"`
	} `xml:"package"`
}

// parseJacoco reports package-relative paths ("com/foo/Bar.java"); Resolve maps them onto
// the source root by suffix. A line is hit when it has covered instructions (ci > 0).
func parseJacoco(r io.Reader) (Lines, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false // JaCoCo emits a DOCTYPE pointing at report.dtd
	var doc jacocoDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	out := Lines{}
	for _, p := range doc.Packages {
		for _, f := range p.Files {
			for _, l := range f.Lines {
				if l.Ci > 0 {
					out.add(p.Name+"/"+f.Name, l.Nr)
				}
			}
		}
	}
	return out, nil
}
```

`internal/covfmt/resolve.go`:

```go
package covfmt

import (
	"path"
	"path/filepath"
	"strings"
)

// Resolve maps each reported path onto a repo-relative path from repoFiles, merging lines
// that arrive under two spellings. In order: an absolute path under root; the path itself;
// the path with leading segments dropped (Go import paths); the unique repo file ending in
// "/"+path (JaCoCo package paths). A path that resolves to nothing, or to more than one
// file, is dropped — attributing a line to the wrong file is worse than losing it.
func Resolve(root string, raw Lines, repoFiles []string) map[string][]int {
	set := make(map[string]bool, len(repoFiles))
	byBase := map[string][]string{}
	for _, f := range repoFiles {
		set[f] = true
		byBase[path.Base(f)] = append(byBase[path.Base(f)], f)
	}
	out := map[string][]int{}
	for p, lines := range raw {
		rel, ok := resolveOne(root, p, set, byBase)
		if !ok {
			continue
		}
		out[rel] = sortedUnique(append(out[rel], lines...))
	}
	return out
}

func resolveOne(root, p string, set map[string]bool, byBase map[string][]string) (string, bool) {
	p = filepath.ToSlash(p)
	if path.IsAbs(p) || filepath.IsAbs(p) {
		r, err := filepath.Rel(root, filepath.FromSlash(p))
		if err != nil {
			return "", false
		}
		r = filepath.ToSlash(r)
		if r == ".." || strings.HasPrefix(r, "../") {
			return "", false
		}
		p = r
	}
	p = strings.TrimPrefix(path.Clean(p), "./")
	if set[p] {
		return p, true
	}
	for q := p; ; {
		i := strings.Index(q, "/")
		if i < 0 {
			break
		}
		q = q[i+1:]
		if set[q] {
			return q, true
		}
	}
	match := ""
	for _, f := range byBase[path.Base(p)] {
		if strings.HasSuffix(f, "/"+p) {
			if match != "" {
				return "", false
			}
			match = f
		}
	}
	return match, match != ""
}
```

Note the ambiguous case in the test: `com/dup/X.java` — the drop-leading-segments loop tries `dup/X.java` and `X.java`, neither is in the set, then the suffix match finds two files and drops it.

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./internal/covfmt/ -v`
Expected: PASS (`TestParse/*`, `TestParseRejectsUnknownFormatAndGarbage`, `TestResolve`).

- [ ] **Step 6: Commit**

```bash
git add internal/covfmt
git commit -m "feat(covfmt): parse lcov, cobertura, gocover and jacoco, and resolve reported paths"
```

---

### Task 2: `gitctx.ListFiles`

**Files:**
- Create: `internal/gitctx/listfiles.go`
- Test: `internal/gitctx/listfiles_test.go`

**Interfaces:**
- Produces: `func ListFiles(repoRoot string) ([]string, error)` — sorted, slash-separated, repo-relative; tracked plus untracked-not-ignored; deleted-but-tracked files excluded.

- [ ] **Step 1: Write the failing test** (uses `newRepo`, `write`, `remove`, `commit` from `internal/gitctx/helper_test.go`)

```go
package gitctx

import (
	"reflect"
	"testing"
)

func TestListFilesIsTrackedPlusUntrackedMinusIgnoredAndDeleted(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, ".gitignore", "build/\n")
	write(t, dir, "a.go", "package a\n")
	write(t, dir, "gone.go", "package a\n")
	commit(t, dir, "init")
	write(t, dir, "new_test.go", "package a\n")
	write(t, dir, "build/out.go", "package a\n")
	remove(t, dir, "gone.go")

	got, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".gitignore", "a.go", "new_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFiles = %v, want %v", got, want)
	}
}
```

Check `newRepo` does not already commit files that would appear in the list; if it does, add them to `want`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/gitctx/ -run TestListFiles`
Expected: FAIL — `undefined: ListFiles`.

- [ ] **Step 3: Implement**

```go
package gitctx

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ListFiles is every file git would call part of the working tree: tracked, plus
// untracked files not ignored, minus tracked files deleted from disk. It is how the
// pipeline enumerates units — a test file written a moment ago is a unit before it is
// committed (spec §4.1).
func ListFiles(repoRoot string) ([]string, error) {
	out, err := git(repoRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(p))); err != nil {
			continue
		}
		files = append(files, p)
	}
	sort.Strings(files)
	return files, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/gitctx/ -run TestListFiles -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gitctx/listfiles.go internal/gitctx/listfiles_test.go
git commit -m "feat(gitctx): ListFiles enumerates tracked and untracked, unignored files"
```

---

### Task 3: Adapter contract v3 — new fields and unit expansion

Adds the v3 fields beside the old ones; the old ones are removed in Task 7, once nothing reads them.

**Files:**
- Modify: `internal/adapter/adapter.go` (struct, `validate`)
- Create: `internal/adapter/unit.go`
- Test: `internal/adapter/unit_test.go`

**Interfaces:**
- Produces on `Adapter`: fields `UnitCmd string \`yaml:"unit_cmd"\``, `UnitNames string \`yaml:"unit_names"\``, `CoverageFile string \`yaml:"coverage_file"\``, `CoverageFormat string \`yaml:"coverage_format"\``, `Jobs int \`yaml:"jobs"\``; methods
  - `func (a *Adapter) UnitArgv(unit, tmp, names string) ([]string, error)`
  - `func (a *Adapter) UnitEnv(tmp string) map[string]string`
  - `func (a *Adapter) UnitNamesOf(testFileBody []byte) (names string, ok bool)` — `ok` is false when `unit_names` is declared and matches nothing (the unit has no runnable tests); names is `^(A|B)$`.
  - `func (a *Adapter) CoveragePath(tmp string) string`
  - `func (a *Adapter) IsV3() bool` — `UnitCmd != ""`.

- [ ] **Step 1: Write the failing tests**

```go
package adapter

import (
	"reflect"
	"strings"
	"testing"
)

func v3(t *testing.T, body string) *Adapter {
	t.Helper()
	a, err := parse([]byte(body), "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

const goV3 = `name: go
detect: ["go.mod"]
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
env: { GOFLAGS: "-mod=mod", CACHE: "{tmp}/c" }
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
`

func TestUnitArgvSubstitutesEveryPlaceholder(t *testing.T) {
	a := v3(t, goV3)
	got, err := a.UnitArgv("calc/calc_test.go", "/tmp/u1", "^(TestAdd|TestSub)$")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"go", "test", "-count=1", "-coverpkg=./...", "-coverprofile=/tmp/u1/cover.out",
		"-run", "^(TestAdd|TestSub)$", "./calc"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnitArgv = %q, want %q", got, want)
	}
	if p := a.CoveragePath("/tmp/u1"); p != "/tmp/u1/cover.out" {
		t.Errorf("CoveragePath = %q", p)
	}
	if env := a.UnitEnv("/tmp/u1"); env["CACHE"] != "/tmp/u1/c" || env["GOFLAGS"] != "-mod=mod" {
		t.Errorf("UnitEnv = %v", env)
	}
}

func TestUnitArgvRootDirIsDot(t *testing.T) {
	a := v3(t, goV3)
	got, _ := a.UnitArgv("calc_test.go", "/t", "^(TestA)$")
	if got[len(got)-1] != "./." {
		t.Errorf("root-level unit dir = %q, want ./.", got[len(got)-1])
	}
}

func TestUnitNamesOf(t *testing.T) {
	a := v3(t, goV3)
	names, ok := a.UnitNamesOf([]byte("package c\nfunc TestAdd(t *testing.T) {}\nfunc helper() {}\nfunc TestSub(t *testing.T) {}\n"))
	if !ok || names != "^(TestAdd|TestSub)$" {
		t.Errorf("UnitNamesOf = %q, %v", names, ok)
	}
	if _, ok := a.UnitNamesOf([]byte("package c\nfunc helper() {}\n")); ok {
		t.Error("a file with no Test funcs reported runnable names")
	}
}

func TestV3Validation(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"unknown format", strings.Replace(goV3, "gocover", "sqlite", 1), "coverage_format"},
		{"no unit placeholder", strings.Replace(goV3, "./{dir}", "./...", 1) + "", ""},
		{"coverage file outside tmp", strings.Replace(goV3, `coverage_file: "{tmp}/cover.out"`, `coverage_file: "cover.out"`, 1), "coverage_file"},
		{"unknown placeholder", strings.Replace(goV3, "{names}", "{tests}", 1), "{tests}"},
		{"bad names regex", strings.Replace(goV3, `'^func (Test\w+)\('`, `'^func (Test'`, 1), "unit_names"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := parse([]byte(c.body), "x.yaml")
			if c.want == "" {
				if err != nil {
					t.Fatalf("valid adapter rejected: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
}
```

(`./...` without `{unit}`/`{dir}` is valid: a runner may run a unit by `{names}` alone.)

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/adapter/ -run 'TestUnit|TestV3'`
Expected: FAIL — unknown fields / `undefined: UnitArgv`.

- [ ] **Step 3: Add the fields** to the `Adapter` struct in `internal/adapter/adapter.go`, directly under `FullEscalate`:

```go
	// Contract v3 (docs/specs/2026-09-29-one-pipeline.md §6). One test file runs per
	// process; UnitCmd writes its coverage to CoverageFile under {tmp}.
	UnitCmd        string `yaml:"unit_cmd"`
	UnitNames      string `yaml:"unit_names"`
	CoverageFile   string `yaml:"coverage_file"`
	CoverageFormat string `yaml:"coverage_format"`
	Jobs           int    `yaml:"jobs"`
```

- [ ] **Step 4: Validate v3.** At the top of `validate()`, after `validateGlobs`, when `a.UnitCmd != ""` return `a.validateV3()` and skip the v2 switch (v2 checks require `subset`/`seed`, which v3 adapters do not have):

```go
	if a.UnitCmd != "" {
		if a.Name == "" {
			return fmt.Errorf("name is required")
		}
		if len(a.Detect) == 0 {
			return fmt.Errorf("detect is required")
		}
		return a.validateV3()
	}
```

In `internal/adapter/unit.go`:

```go
package adapter

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/VocanicZ/rtdd/internal/covfmt"
)

var unitPlaceholders = map[string]bool{"{unit}": true, "{dir}": true, "{name}": true, "{names}": true, "{tmp}": true}

func (a *Adapter) IsV3() bool { return a != nil && a.UnitCmd != "" }

func (a *Adapter) validateV3() error {
	if bad := unknownPlaceholder(a.UnitCmd, unitPlaceholders); bad != "" {
		return fmt.Errorf("unit_cmd %q: unknown placeholder %s (only {unit}, {dir}, {name}, {names}, {tmp})", a.UnitCmd, bad)
	}
	if !slices.Contains(covfmt.Formats, a.CoverageFormat) {
		return fmt.Errorf("coverage_format %q: unsupported (only %v)", a.CoverageFormat, covfmt.Formats)
	}
	// Every unit gets a private {tmp}; a coverage file anywhere else is shared between
	// parallel units and one unit would read another's coverage.
	if !strings.HasPrefix(a.CoverageFile, "{tmp}/") {
		return fmt.Errorf("coverage_file %q: must start with {tmp}/ so parallel units never share it", a.CoverageFile)
	}
	if a.UnitNames != "" {
		re, err := regexp.Compile("(?m)" + a.UnitNames)
		if err != nil {
			return fmt.Errorf("unit_names %q: %v", a.UnitNames, err)
		}
		if re.NumSubexp() < 1 {
			return fmt.Errorf("unit_names %q: needs one capture group naming the test", a.UnitNames)
		}
	}
	if a.Jobs < 0 {
		return fmt.Errorf("jobs %d: must be 0 (CPU count) or positive", a.Jobs)
	}
	return nil
}

func (a *Adapter) unitVars(unit, tmp, names string) map[string]string {
	dir := path.Dir(unit)
	base := path.Base(unit)
	return map[string]string{
		"unit":  unit,
		"dir":   dir,
		"name":  strings.TrimSuffix(base, path.Ext(base)),
		"names": names,
		"tmp":   tmp,
	}
}

// UnitArgv renders unit_cmd for one test file. Split on whitespace BEFORE substitution,
// as every command template is: a path with a space stays one argv element.
func (a *Adapter) UnitArgv(unit, tmp, names string) ([]string, error) {
	return a.Expand(a.UnitCmd, a.unitVars(unit, tmp, names))
}

func (a *Adapter) CoveragePath(tmp string) string {
	return strings.ReplaceAll(a.CoverageFile, "{tmp}", tmp)
}

func (a *Adapter) UnitEnv(tmp string) map[string]string {
	out := make(map[string]string, len(a.Env))
	for k, v := range a.Env {
		out[k] = strings.ReplaceAll(v, "{tmp}", tmp)
	}
	return out
}

// UnitNamesOf fills {names} from the test file's source: every first capture of
// unit_names, as ^(A|B)$. ok is false when unit_names is declared and matches nothing —
// the file holds no runnable test and the unit is skipped, not run.
func (a *Adapter) UnitNamesOf(body []byte) (string, bool) {
	if a.UnitNames == "" {
		return "", true
	}
	re := regexp.MustCompile("(?m)" + a.UnitNames)
	var names []string
	for _, m := range re.FindAllSubmatch(body, -1) {
		names = append(names, regexp.QuoteMeta(string(m[1])))
	}
	if len(names) == 0 {
		return "", false
	}
	return "^(" + strings.Join(names, "|") + ")$", true
}
```

`Expand` (`expand.go`) rejects any template containing `{tests}` and resolves each `{key}` by looking up `vars[key]` without braces, so the map keys are `unit`, `dir`, `name`, `names`, `tmp`; an unknown placeholder is an error naming it.

- [ ] **Step 5: Run to verify they pass**

Run: `go test ./internal/adapter/ -run 'TestUnit|TestV3' -v && go test ./internal/adapter/`
Expected: PASS, and the existing adapter tests still pass (v2 adapters untouched).

- [ ] **Step 6: Commit**

```bash
git add internal/adapter
git commit -m "feat(adapter): contract v3 fields — unit_cmd, unit_names, coverage_file/format, jobs"
```

---

### Task 4: The unit runner

`runner.Seed` / `runner.Run` keep their signatures and the `RunResult` shape; for a v3 adapter they go through `RunUnits`. v2 adapters keep the old path until Task 7 deletes it.

**Files:**
- Create: `internal/runner/units.go`, `internal/runner/testdata/gofix/{go.mod,calc/calc.go,calc/calc_test.go,calc/helpers_test.go,store/store.go,api/api.go,api/api_test.go}`
- Modify: `internal/runner/run.go` (`Seed`, `Run` dispatch; `Outcome` type)
- Test: `internal/runner/units_test.go`

**Interfaces:**
- Consumes: `covfmt.Parse`, `covfmt.Resolve` (Task 1); `gitctx.ListFiles` (Task 2); `Adapter.UnitArgv/UnitEnv/UnitNamesOf/CoveragePath/IsV3`, `Adapter.IsTestFile/IsInstrumentable` (Task 3).
- Produces:
  - `func Units(a *adapter.Adapter, repoRoot string) ([]string, error)`
  - `func RunUnits(a *adapter.Adapter, repoRoot string, units []string, failFast bool) (*RunResult, error)`
  - `RunResult` gains `Output map[string]string` — the tail (last 4000 bytes) of each failed or errored unit's combined output.
  - Unit statuses: `pass`, `fail`, `skip`, `error`.

- [ ] **Step 1: Create the fixture module.** `store.go` is reached only through `api.go`, so `api_test.go` — not name-correspondent to `store.go` — must be the unit that covers it.

`internal/runner/testdata/gofix/go.mod`:
```
module example.com/gofix

go 1.22
```
`calc/calc.go`:
```go
package calc

func Add(a, b int) int { return a + b }

func Sub(a, b int) int { return a - b }
```
`calc/calc_test.go`:
```go
package calc

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("add")
	}
}
```
`calc/helpers_test.go`:
```go
package calc

func helper() int { return 1 }
```
`store/store.go`:
```go
package store

func Get(k string) string { return "v:" + k }
```
`api/api.go`:
```go
package api

import "example.com/gofix/store"

func Handle(k string) string { return store.Get(k) }
```
`api/api_test.go`:
```go
package api

import "testing"

func TestHandle(t *testing.T) {
	if Handle("a") != "v:a" {
		t.Fatal("handle")
	}
}
```

- [ ] **Step 2: Write the failing tests**

```go
package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

const goAdapter = `name: go
detect: ["go.mod"]
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
`

// gofixRepo copies testdata/gofix into a fresh git repo, because units are enumerated
// through git.
func gofixRepo(t *testing.T) (string, *adapter.Adapter) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/gofix")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	p := filepath.Join(t.TempDir(), "go.yaml")
	if err := os.WriteFile(p, []byte(goAdapter), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := adapter.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return dir, a
}

func TestUnitsAreTheTestFiles(t *testing.T) {
	dir, a := gofixRepo(t)
	got, err := Units(a, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api/api_test.go", "calc/calc_test.go", "calc/helpers_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Units = %v, want %v", got, want)
	}
}

func TestSeedRecordsWhatEachUnitExecuted(t *testing.T) {
	dir, a := gofixRepo(t)
	res, err := Seed(a, dir)
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, o := range res.Outcomes {
		status[o.Test] = o.Status
	}
	wantStatus := map[string]string{"api/api_test.go": "pass", "calc/calc_test.go": "pass", "calc/helpers_test.go": "skip"}
	if !reflect.DeepEqual(status, wantStatus) {
		t.Errorf("statuses = %v, want %v", status, wantStatus)
	}
	files := map[string][]string{}
	for _, pt := range res.Coverage.PerTest {
		for f := range pt.Files {
			files[pt.Test] = append(files[pt.Test], f)
		}
	}
	// The case the static tier missed: store.go is covered by api_test.go.
	if !contains(files["api/api_test.go"], "store/store.go") {
		t.Errorf("api_test.go files = %v, want store/store.go among them", files["api/api_test.go"])
	}
	// Isolation: calc's unit never saw store.go.
	if contains(files["calc/calc_test.go"], "store/store.go") {
		t.Errorf("calc_test.go files = %v leaked another unit's coverage", files["calc/calc_test.go"])
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestAFailingUnitFailsTheRunAndKeepsItsOutput(t *testing.T) {
	dir, a := gofixRepo(t)
	p := filepath.Join(dir, "calc/calc_test.go")
	body, _ := os.ReadFile(p)
	os.WriteFile(p, []byte(strings.Replace(string(body), "!= 3", "!= 4", 1)), 0o644)
	res, err := Run(a, dir, []string{"calc/calc_test.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 1 || len(res.Failed) != 1 || res.Failed[0] != "calc/calc_test.go" {
		t.Errorf("ExitCode=%d Failed=%v, want 1 [calc/calc_test.go]", res.ExitCode, res.Failed)
	}
	if !strings.Contains(res.Output["calc/calc_test.go"], "add") {
		t.Errorf("failure output not kept: %q", res.Output["calc/calc_test.go"])
	}
}

func TestAPassWithNoCoverageFileIsAnError(t *testing.T) {
	dir, a := gofixRepo(t)
	a.CoverageFile = "{tmp}/never-written.out"
	res, err := Run(a, dir, []string{"calc/calc_test.go"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcomes[0].Status != "error" || res.ExitCode == 0 {
		t.Errorf("status=%s exit=%d, want error and a non-zero exit", res.Outcomes[0].Status, res.ExitCode)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run to verify they fail**

Run: `go test ./internal/runner/ -run 'TestUnits|TestSeedRecords|TestAFailing|TestAPass'`
Expected: FAIL — `undefined: Units` (and `res.Output` undefined).

- [ ] **Step 4: Implement `internal/runner/units.go`**

```go
package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/covfmt"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// Units is every test file the adapter claims, tracked or not (spec §4.1).
func Units(a *adapter.Adapter, repoRoot string) ([]string, error) {
	files, err := gitctx.ListFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("runner: enumerate units: %w", err)
	}
	var out []string
	for _, f := range files {
		if a.IsTestFile(f) {
			out = append(out, f)
		}
	}
	return out, nil
}

type unitResult struct {
	outcome Outcome
	files   map[string][]int
	output  string
	fatal   error
}

// RunUnits runs each unit in its own process, up to Jobs at once (spec §4.2–4.5).
func RunUnits(a *adapter.Adapter, repoRoot string, units []string, failFast bool) (*RunResult, error) {
	repoFiles, err := gitctx.ListFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	jobs := a.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	results := make([]unitResult, len(units))
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		stopped bool
	)
	sem := make(chan struct{}, jobs)
	for i, u := range units {
		mu.Lock()
		stop := stopped
		mu.Unlock()
		if stop {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			defer func() { <-sem }()
			r := runUnit(a, repoRoot, u, repoFiles)
			results[i] = r
			if failFast && (r.outcome.Status == "fail" || r.outcome.Status == "error") {
				mu.Lock()
				stopped = true
				mu.Unlock()
			}
		}(i, u)
	}
	wg.Wait()

	res := &RunResult{Coverage: &coverage.Result{}, Output: map[string]string{}}
	for _, r := range results {
		if r.outcome.Test == "" {
			continue // never scheduled (fail-fast)
		}
		if r.fatal != nil {
			return nil, r.fatal
		}
		res.Outcomes = append(res.Outcomes, r.outcome)
		if r.files != nil {
			res.Coverage.PerTest = append(res.Coverage.PerTest, coverage.TestCoverage{Test: r.outcome.Test, Files: r.files})
		}
		switch r.outcome.Status {
		case "fail", "error":
			res.Failed = append(res.Failed, r.outcome.Test)
			res.Output[r.outcome.Test] = r.output
			res.ExitCode = 1
		}
	}
	sort.Slice(res.Coverage.PerTest, func(i, j int) bool { return res.Coverage.PerTest[i].Test < res.Coverage.PerTest[j].Test })
	return res, nil
}

func runUnit(a *adapter.Adapter, repoRoot, unit string, repoFiles []string) unitResult {
	r := unitResult{outcome: Outcome{Test: unit}}
	body, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(unit)))
	if err != nil {
		r.outcome.Status, r.output = "error", err.Error()
		return r
	}
	names, ok := a.UnitNamesOf(body)
	if !ok {
		r.outcome.Status = "skip" // no runnable test in this file
		return r
	}
	tmp, err := os.MkdirTemp("", "rtdd-unit-")
	if err != nil {
		r.fatal = fmt.Errorf("runner: %w", err)
		return r
	}
	defer os.RemoveAll(tmp)
	argv, err := a.UnitArgv(unit, tmp, names)
	if err != nil {
		r.fatal = err
		return r
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repoRoot
	cmd.Env = mergeEnv(os.Environ(), a.UnitEnv(tmp))
	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	r.outcome.DurationMS = int(time.Since(start).Milliseconds())
	r.output = tail(out, 4000)

	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			r.fatal = fmt.Errorf("runner: executing %s for %s: %w", argv[0], unit, runErr)
			return r
		}
		code = ee.ExitCode()
	}
	switch label, mapped := a.ExitCodes[code]; {
	case code == 0:
		r.outcome.Status = "pass"
	case code == 1:
		r.outcome.Status = "fail"
	case mapped && label == "no-tests-collected":
		r.outcome.Status = "skip"
		return r
	case mapped:
		r.fatal = &FatalExitError{Unit: unit, Code: code, Label: label}
		return r
	default:
		r.outcome.Status = "error"
		return r
	}

	f, err := os.Open(a.CoveragePath(tmp))
	if err != nil {
		// A run that says it passed but recorded nothing is not a pass (spec §4.4).
		r.outcome.Status = "error"
		r.output = fmt.Sprintf("no coverage file at %s: %v\n%s", a.CoverageFile, err, r.output)
		return r
	}
	defer f.Close()
	raw, err := covfmt.Parse(a.CoverageFormat, f)
	if err != nil {
		r.outcome.Status = "error"
		r.output = err.Error() + "\n" + r.output
		return r
	}
	kept := map[string][]int{}
	for p, ls := range covfmt.Resolve(repoRoot, raw, repoFiles) {
		if a.IsInstrumentable(p) || a.IsTestFile(p) {
			kept[p] = ls
		}
	}
	r.files = kept
	return r
}
```

- [ ] **Step 5: Wire it in `internal/runner/run.go`.**
  - Add the `Outcome` type (moved here from `report`; `report.Outcome` stays as an alias until Task 7): `type Outcome = report.Outcome`.
  - Add `Output map[string]string` to `RunResult`.
  - In `Seed`: `if a.IsV3() { units, err := Units(a, repoRoot); if err != nil { return nil, err }; return RunUnits(a, repoRoot, units, false) }` before the existing body.
  - In `Run`: `if a.IsV3() { return RunUnits(a, repoRoot, tests, failFast) }` before the existing body.
  - In `internal/runner/errors.go`: add `Unit string` to `FatalExitError` and make `Error()` name the unit when set: `if e.Unit != "" { return fmt.Sprintf("runner: %s exited %d (%s); this is a fatal error, not a test failure", e.Unit, e.Code, e.Label) }`.

- [ ] **Step 6: Run to verify they pass**

Run: `go test ./internal/runner/ -v -run 'TestUnits|TestSeedRecords|TestAFailing|TestAPass' && go test ./internal/runner/`
Expected: PASS, and existing runner tests still pass.

- [ ] **Step 7: Commit**

```bash
git add internal/runner
git commit -m "feat(runner): run each test file in its own process and record its coverage"
```

---

### Task 5: Python and Go adapters on contract v3

**Files:**
- Modify: `adapters/python.yaml`, `adapters/go.yaml`
- Modify: `internal/adapter/testdata/python.yaml` (the copy tests load; keep it identical to `adapters/python.yaml`)
- Delete test: `internal/contract/adapter_freeze_test.go` (it byte-freezes the old python.yaml; the spec supersedes the freeze)
- Test: `cmd/rtdd/pipeline_python_test.go`, `cmd/rtdd/pipeline_go_test.go`

**Interfaces:**
- Consumes: Tasks 3–4.
- Produces: the two shipped YAMLs below, used by Tasks 6–7 tests.

- [ ] **Step 1: Rewrite `adapters/python.yaml`** (keep a short header comment saying what each line is for):

```yaml
name: python
detect: ["pytest.ini", "pyproject.toml", "setup.cfg"]
# One test file per process. COVERAGE_FILE keeps parallel units off each other's
# .coverage; --cov-report writes the lcov rtdd reads.
unit_cmd: "pytest -q -p no:cacheprovider --cov=. --cov-report=lcov:{tmp}/lcov.info {unit}"
env: { COVERAGE_FILE: "{tmp}/.coverage" }
coverage_file: "{tmp}/lcov.info"
coverage_format: lcov
exit_codes: { 4: bad-selector, 5: no-tests-collected }
requires:
  - bin: pytest
    reason: "the python adapter runs pytest with the pytest-cov plugin"
test_globs: ["**/test_*.py", "**/*_test.py"]
source_globs: ["**/*.py"]
opaque: ["**/*.yaml", "**/*.yml", "**/*.sql", "**/*.html", "**/*.j2", "**/*.json", "**/fixtures/**"]
full_escalate: ["requirements.txt", "pyproject.toml", "setup.cfg", "pytest.ini", "tox.ini", "poetry.lock", "uv.lock", "**/conftest.py"]
```

`test_globs` drops `tests/**/*.py`: a unit must be a file pytest can run on its own, and `tests/__init__.py` or a helper module is not.

- [ ] **Step 2: Rewrite `adapters/go.yaml`**:

```yaml
name: go
detect: ["go.mod"]
# go test cannot take a file, so a unit is its package filtered to the file's own Test
# functions. -coverpkg=./... records every package the test reaches, not only its own.
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
requires:
  - bin: go
    reason: "the go adapter runs go test"
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
opaque: ["**/testdata/**", "**/*.golden", "**/*.json", "**/*.yaml", "**/*.tmpl"]
full_escalate: ["go.mod", "go.sum", "vendor/modules.txt", "**/tools.go"]
```

- [ ] **Step 3: Write the end-to-end tests** — seed a fixture through the real CLI entry point, change a source file reached only through another package, and assert `rtdd which --json` selects the non-name-correspondent test file. Use the existing CLI test harness in `cmd/rtdd` (find it with `grep -n "func runCLI\|func runRTDD\|run(\[\]string" cmd/rtdd/*_test.go`); the Go test reuses `internal/runner/testdata/gofix` (copy it into a temp git repo as in Task 4). The Python test builds this fixture inline:

```
pytest.ini            [pytest]
app/__init__.py       (empty)
app/store.py          def get(k): return "v:" + k
app/api.py            from app.store import get
                      def handle(k): return get(k)
tests/test_api.py     from app.api import handle
                      def test_handle(): assert handle("a") == "v:a"
tests/test_other.py   def test_other(): assert 1 + 1 == 2
```

Assertions, for both languages:
1. `rtdd seed` exits 0 and `.rtdd/meta.json` has `"v": 2`.
2. Append a line to the store source (`app/store.py` / `store/store.go`), run `rtdd which --json`: `tier` is `T0`, `selection.tests` is exactly `["tests/test_api.py"]` / `["api/api_test.go"]`.
3. `rtdd run --json` exits 0 and `uncovered.summary.uncovered_lines` counts the appended line.

The Python test skips unless `pytest` and pytest-cov are importable (`python3 -c "import pytest_cov"` from the `pytest` on PATH); the Go test skips without `go`.

- [ ] **Step 4: Run to verify they fail**

Run: `go test ./cmd/rtdd/ -run 'TestPipelinePython|TestPipelineGo' -v`
Expected: FAIL — `rtdd which` still routes the Go adapter through the static tier / seed refuses (Task 6 fixes the command layer). If they already pass, that is fine: the runner switch in Task 4 may be enough for seed; Task 6 still owns the selector changes.

- [ ] **Step 5: Commit** (tests may be red until Task 6; commit them with the YAML so Task 6 starts from a failing test)

```bash
git rm internal/contract/adapter_freeze_test.go
git add adapters/python.yaml adapters/go.yaml internal/adapter/testdata/python.yaml cmd/rtdd/pipeline_python_test.go cmd/rtdd/pipeline_go_test.go
git commit -m "feat(adapters): python and go on the one pipeline"
```

---

### Task 6: Command layer on the one pipeline

**Files:**
- Modify: `internal/selector/select.go`, `internal/selector/tier.go`
- Modify: `cmd/rtdd/polyglot.go` (`selectFor`, `runSelection`), `cmd/rtdd/seed.go`, `cmd/rtdd/run.go`, `cmd/rtdd/which.go`, `cmd/rtdd/meta.go`, `cmd/rtdd/rows.go`, `cmd/rtdd/uncoveredgate.go`
- Test: `internal/selector/select_test.go` (new cases), `cmd/rtdd/mapversion_test.go`

**Interfaces:**
- Consumes: `runner.Units`, `runner.Run`, `runner.Seed`, `RunResult.Output` (Task 4).
- Produces: `const mapstore.MapVersion = 2`; selector reason string `"map is unseeded or from an older rtdd; run `rtdd seed`"`.

- [ ] **Step 1: Failing selector tests** in `internal/selector/select_test.go`:

```go
func TestEmptyMapIsT2WithTheSeedReason(t *testing.T) {
	a := testAdapter(t) // the file's existing adapter helper
	sel := Select(Inputs{
		Map:      mapstore.New(),
		Changes:  []gitctx.Change{{Path: "src/a.py", Status: gitctx.Modified}},
		Adapter:  a,
		AllTests: []string{"tests/test_a.py", "tests/test_b.py"},
	})
	if sel.Tier != TierT2 || !strings.Contains(sel.Reason, "rtdd seed") {
		t.Fatalf("tier=%v reason=%q, want T2 naming rtdd seed", sel.Tier, sel.Reason)
	}
	if !reflect.DeepEqual(sel.Tests, []string{"tests/test_a.py", "tests/test_b.py"}) {
		t.Errorf("tests = %v, want every unit", sel.Tests)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/selector/ -run TestEmptyMapIsT2WithTheSeedReason`
Expected: FAIL — the current code routes an empty map to TS.

- [ ] **Step 3: Selector.** In `Select`, replace the `(5) TS` block with:

```go
	if m.Len() == 0 {
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: "map is unseeded or from an older rtdd; run `rtdd seed`",
		}
	}
```

Delete the `ImportOnly` block from `escalateT1` and the `ImportOnly`, `Exists`, `ImportDistance` fields from `Inputs`. Rewrite the `Select` header comment's numbered list to: 1 direct, 2 T2 (full-escalate, drift guard, empty map), 3 T1 (merge, opaque, stale row), 4 T0, 5 empty — `TestDocumentedResolutionOrderMatchesTheSelector` in `internal/contract/contract_test.go` reads this comment; update that test's expected steps to match.

- [ ] **Step 4: `cmd/rtdd/polyglot.go` `selectFor`.** Compute units once and pass them as `AllTests`; remove `newImportFallback`, `staticResolvers`, `ctx.Enumerate`, `SuiteEnumerated`, `SuiteResult`, `ImportFallback`, `FallbackErr`, `ScanErr`:

```go
	units, err := runner.Units(ad, root)
	if err != nil {
		return blk, err
	}
	blk.Selection = selector.Select(selector.Inputs{
		Map:                      sub,
		Changes:                  ctx.Changes,
		Adapter:                  ad,
		Cfg:                      ctx.Cfg,
		AllTests:                 units,
		Cycles:                   ctx.Cycles,
		Merge:                    ctx.Merge,
		EscalateDigest:           ctx.EscalateDigest,
		EscalateDigestAtLastFull: ctx.EscalateDigestAtLastFull,
		Distance:                 ctx.Distance,
	})
	return blk, nil
```

`runSelection` becomes `return runner.Run(blk.Ad, root, blk.Selection.Tests, failFast)` (no plain/record branches).

- [ ] **Step 5: Map version.** In `internal/mapstore/meta.go` add `const MapVersion = 2`. In `cmd/rtdd/seed.go` write `V: mapstore.MapVersion`. Where `cmdRun` and `cmdWhich` load the map (`mapstore.LoadWith`), if `mt.V < mapstore.MapVersion` replace the loaded map with `mapstore.New()` so the selector takes the empty-map T2 branch. Test in `cmd/rtdd/mapversion_test.go`: write a meta with `"v":1` and a map row `{"t":"tests/test_a.py::test_x","f":["src/a.py"]}`, run `rtdd which --json` in a repo with the python adapter, assert `tier == "T2"` and the reason contains `rtdd seed`.

- [ ] **Step 6: Seed.** `cmdSeed` seeds every detected adapter (drop `seedPlan`/`selectionSplit`/`staticSeedRefusalFor`); each goes through `runner.Seed`.

- [ ] **Step 7: Run.** In `cmdRun`: remove the `--record` flag, `shouldRecord`, and the not-recorded branches — every executed unit's rows refresh (`rowsFrom` + `UnionFor`) and its coverage feeds `BuildSignal`. In the text renderer, after the summary line, print each failed unit's output: for each `u` in `res.Failed`, print `--- u ---` then `res.Output[u]`. `recordsCoverage(ad)` in `uncoveredgate.go` becomes `return ad != nil` (every adapter records).

- [ ] **Step 8: Run the command-layer tests**

Run: `go test ./internal/selector/ ./cmd/rtdd/ 2>&1 | tail -40`
Expected: the Task 5 pipeline tests and `TestEmptyMapIsT2WithTheSeedReason` pass. Tests pinning the static tier, `--record`, `selection_fidelity` or the import fallback now fail — list them; Task 7 deletes them with the code. Do not delete them here.

- [ ] **Step 9: Commit**

```bash
git add -A internal/selector internal/mapstore cmd/rtdd
git commit -m "feat: which/run/seed use the unit pipeline; empty or v1 map is T2"
```

---

### Task 7: Delete the second pipeline

**Files (delete):**
- `internal/coverage/sqlite.go`, `numbits.go`, `context.go` and their tests (keep `result.go`, drop `ImportTime` from `Result` and `Merge`)
- `internal/report/` (all files; move `Outcome{Test, Status string; DurationMS int}` into `internal/runner/run.go` as a real struct, replacing the alias)
- `internal/importscan/` (all files, including `scan.py`)
- `internal/selector/static.go`, `TierTS` and its `String` case, and their tests
- `internal/adapter/fidelity.go`, `testfor.go`, `testselector.go`, `classify.go:CanRunPlain`, `expand.go:ExpandTests/expandJoined`, and their tests
- `cmd/rtdd/static.go`, `staticadvice.go`, `staticwarn.go`, `record.go`, `fidelity.go`, `distance.go` if only the static tier used it (check), and their tests
- `internal/runner/chunk.go`, the v2 branches of `run.go` (`execute`, `runReportCmd`, `readOutcomes`, `ListRun`, `parseTestIDs`, `RunPlain`), `errors.go`'s sysmon code, and their tests
- `cmd/rtdd/testdata/which-python-golden.txt` and `internal/selector/testdata/selection_golden.txt` if their tests are deleted; otherwise regenerate them

**Files (modify):**
- `internal/adapter/adapter.go`: remove every v2 field. Reject them at load: decode first into `map[string]any` and, for each key in `removedFields`, return `fmt.Errorf("%s: removed in contract v3 (docs/specs/2026-09-29-one-pipeline.md §6)", key)`:

```go
var removedFields = []string{"seed", "subset", "subset_plain", "list", "coverage", "report",
	"report_path", "report_cmd", "id_template", "failfast_flag", "test_flag", "test_join",
	"test_selector", "selection", "test_for", "importscan"}
```

  `unit_cmd`, `coverage_file` and `coverage_format` become required.
- `internal/uncovered/classify.go`: drop the `ImportTime` class and `Summary.ImportTimeLines`.
- `cmd/rtdd/jsonout.go`: `SchemaVersion = 2`; remove `selection_fidelity` (top-level and per adapter), `import_fallback`, `import_time_lines`.
- `cmd/rtdd/doctor.go`: fidelity block becomes one row per adapter with its source and unmet `requires`; remove static caveats.
- `go.mod` / `go.sum`: `go mod tidy` removes `modernc.org/sqlite`.
- `internal/contract/contract_test.go`: `TestGoModRequiresExactlyYAMLAndSQLite` → `TestGoModRequiresExactlyYAML`; `TestCompiledModulesAreYAMLAndSQLiteOnly` → YAML only; delete `TestSQLiteDriverIsPureGo`, the TS/importscan/fidelity interface tests, and update `TestJSONSchemaV1KeySetMatchesTheInterfaceContract` plus `docs/plans/00-interfaces.md` to schema 2.
- `internal/contract/shipped_adapters_test.go`, `plan_m6*_test.go`, `plan_m7_test.go`: delete assertions pinning the python-adapter freeze, the reportlog path, static/junit pairing; keep assertions about files that still exist.
- `internal/pytestfixture`: keep only if a remaining test uses it (the Task 5 Python test may); otherwise delete.

- [ ] **Step 1: Delete and edit as listed.** Work package by package; after each, run `go build ./... && go vet ./...` and fix what the compiler names.

- [ ] **Step 2: Removed-field test** in `internal/adapter/unit_test.go`:

```go
func TestRemovedV2FieldIsRejectedByName(t *testing.T) {
	_, err := parse([]byte(goV3+"subset: \"go test {tests}\"\n"), "x.yaml")
	if err == nil || !strings.Contains(err.Error(), "subset") || !strings.Contains(err.Error(), "one-pipeline") {
		t.Fatalf("err = %v, want it to name subset and the spec", err)
	}
}
```

- [ ] **Step 3: Full suite**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -v '^ok\|no test files'`
Expected: no output except adapters not yet converted (jest, vitest, cargo, maven, gradle, dotnet, phpunit, rspec) failing to load — Tasks 8–12 convert them. If shipped-adapter load failures break unrelated tests, convert those YAMLs to a minimal valid v3 shape now (`unit_cmd` with `{unit}`, their format from spec §11) and let Tasks 8–12 verify them against real toolchains.

- [ ] **Step 4: `deadcode` is clean**

Run: `go run golang.org/x/tools/cmd/deadcode@latest ./cmd/...`
Expected: at most `ParseID`-class test oracles; anything in the deleted areas means a leftover.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: delete the static tier, report parsers, sqlite coverage and import scan"
```

---

### Tasks 8–12: The remaining eight adapters

Each task has the same shape. For each adapter:

1. Build a fixture in `cmd/rtdd/testdata/fixtures/<adapter>/` with the same three roles as the Go fixture: a `store` source reached only through an `api` source, an `api` test (not name-correspondent to `store`), and a `calc` source with its own test. Reuse an existing fixture dir if one is already there (`ls cmd/rtdd/testdata/fixtures`).
2. Add the fixture to the table-driven `cmd/rtdd/pipeline_adapters_test.go` (one `t.Run` per adapter, the same three assertions as Task 5), watch it fail, then write the YAML from spec §11 and iterate until it passes. Run the unit command by hand in the fixture first when a failure is unclear.
3. The test skips when the adapter's `requires` binaries are missing (`exec.LookPath`).
4. Commit per task.

| Task | Adapters | Local toolchain | Notes to settle in the YAML |
|---|---|---|---|
| 8 | jest, vitest | node 24 | fixture `package.json` devDependencies: `jest`; `vitest` + `@vitest/coverage-v8`; install with `npm ci` in the test (skip on no network). `detect` stays file-based (`jest.config.*`, `vitest.config.*`); a project configuring either inside `package.json` or `vite.config.ts` needs a host adapter — say so in the YAML header. |
| 9 | cargo-nextest → rename to `cargo` | cargo 1.95; needs `cargo install cargo-llvm-cov` | unit = integration test file `tests/{name}.rs` → `--test {name}`; unit tests inside `src/` are one unit per crate: `test_globs: ["tests/*.rs"]` plus `src/**/*.rs` files containing `#[cfg(test)]` is not expressible as a glob — ship `tests/*.rs` only and say so in the YAML header. `jobs: 1` (shared `target/`). |
| 10 | maven, gradle | maven 3.9, java 25; gradle absent (test skips) | maven: `mvn -q -Dtest={name} -Djacoco.destFile={tmp}/jacoco.exec org.jacoco:jacoco-maven-plugin:0.8.13:prepare-agent test org.jacoco:jacoco-maven-plugin:0.8.13:report -Djacoco.dataFile={tmp}/jacoco.exec -Djacoco.outputDirectory={tmp}/site` with `coverage_file: "{tmp}/site/jacoco.xml"`. gradle: ship `adapters/gradle-jacoco.init.gradle` embedded beside the YAML and pass `--init-script`; the adapter loader must resolve a relative init-script path against the adapter's own directory — add a `{adapter_dir}` placeholder only if no simpler path works, and add it to Task 3's placeholder set and tests if so. `jobs: 1`. |
| 11 | dotnet | dotnet 8 | `dotnet test --collect "XPlat Code Coverage" --results-directory {tmp} --filter FullyQualifiedName~{name}`; coverlet writes `{tmp}/<guid>/coverage.cobertura.xml` — a GUID directory. `coverage_file` must support one `*` glob segment for this: extend `CoveragePath` to glob when the path contains `*` and take the single match (error on 0 or >1), with a unit test in Task 3's file. `jobs: 1`. |
| 12 | phpunit, rspec | php 8.3 (needs phpunit + pcov); ruby absent (test skips) | phpunit: `vendor/bin/phpunit --coverage-cobertura {tmp}/cobertura.xml {unit}`, `requires` pcov or xdebug. rspec: `bundle exec rspec {unit}` with `RUBYOPT: "-r{adapter_dir}/simplecov_start.rb"` only if `{adapter_dir}` was added in Task 10; otherwise require the project to have simplecov + simplecov-lcov and say so in `requires`. |

---

### Task 13: Protocol, front-ends and docs

**Files:**
- Modify: `protocol/PROTOCOL.md` (remove the `fidelity` section and static caveats; `what` section: selection is from coverage recorded by running each test file in isolation, for every language; `json` example: schema 2; `limits`: file-level on both sides, isolation cost, test files that depend on each other)
- Modify: `internal/protocol/targets.go` (remove `"fidelity"` from every `Required` list), `internal/protocol/fidelity_test.go` (delete)
- Regenerate: `go run ./cmd/rtdd-gen render && cp protocol/PROTOCOL.md internal/install/protocol.md`, then copy `dist/*.md` goldens into `internal/protocol/testdata/golden/`
- Modify: `README.md` + `docs/outcomes/README.positive.md` (identical; `outcomes_test.go` asserts it), `docs/LIMITATIONS.md` (first bullet replaced: every language uses the same pipeline; the cost is one process per test file)
- Modify: `outcomes_test.go`: delete `TestREADMEStatesTheTwoTiersRatherThanPythonOnly` and `TestREADMEReportsThePreRegisteredStaticVerdict`

- [ ] **Step 1:** Edit PROTOCOL.md and targets.go; regenerate; `go test ./internal/protocol/ ./cmd/rtdd-gen/ ./internal/install/ .`
- [ ] **Step 2:** Edit README/outcome/LIMITATIONS; `go test -run 'Outcome|README' .`
- [ ] **Step 3:** `scripts/ci-local.sh` passes end to end.
- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "docs: one pipeline in the protocol, front-ends, README and limitations"
```

---

### Task 14: Measure on rtdd-bench

Not code. Build the branch binary, re-run the four existing rows (`cron-parser/{go,python}`, `ledger/{go,python}`) in the rtdd arm following `rtdd-bench/README.md`: fresh clones of the `*-rtdd` repos, new binary on PATH, `rtdd init` + `rtdd seed`, `rtdd doctor` shows no static row. Collect with `--label rtdd-new`, render against the existing `tdd.json` and `rtdd.json`. Publish the numbers whichever way they go (spec §1).
