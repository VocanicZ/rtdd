# Node Graph Core (N1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** Build the code graph rtdd v0.3.0 selects from — function, method and class nodes, code-relationship edges, tests as nodes — loaded from graphify when the project has it, built by a language-agnostic name-match scanner when it does not, and **never trusting graphify for a file that changed since graphify was built**. `rtdd graph [--json]` builds or refreshes it and reports what it holds. Nothing in the existing coverage pipeline changes: the graph is added beside it.

**Architecture:** Four new packages, one direction of dependency. `internal/graph` is the model (spec §3), the `.rtdd/config.yaml` keys the graph reads, and test classification (§6) — it imports nothing of rtdd's but `internal/paths`. `internal/scan` is the scanner (§4.1–4.3): it reads files it is handed and never touches git, so every scanner test is a plain `t.TempDir()` or a fixture directory. `internal/graphify` reads a graph graphify already built (§3, §5) and only opens files. `internal/graphbuild` is the one place they meet: it asks `internal/gitctx` for the file list, the changed set, blob ids and the diff since graphify's commit, runs the staleness overlay (§5), keeps the `.rtdd/graph.json` cache (§4.5), and returns one `graph.Graph`. `cmd/rtdd/graph.go` prints it. The scanner is a regex table in one file plus brace/indent span rules; every other line of the scanner is language-agnostic.

**Tech Stack:** Go 1.24 (module `github.com/VocanicZ/rtdd`), stdlib `regexp`, `encoding/json`, `crypto/sha256`; `gopkg.in/yaml.v3` (already the module's one dependency) for `.rtdd/config.yaml`. Tests use stdlib `testing` and real git repositories built with `internal/gitctx/gittest`. No toolchain of any scanned language is ever invoked.

**Spec:** [`docs/specs/2026-10-07-node-graph.md`](../specs/2026-10-07-node-graph.md) §3 (the graph), §4 (the built-in scanner), §5 (graphify and staleness), §6 (tests), §8 (`rtdd graph` only) and §12 (the defaults this plan must not re-decide) — PRD #409. Each task names the sibling issue it is and the PRD acceptance criteria it discharges; the map after the decisions lists all eleven.

## Global Constraints

- **No per-language code path anywhere except the single regex table of spec §4.2**, which lives in exactly one Go file: `internal/scan/patterns.go`. No `switch` on a file extension, no language name in a condition, no per-language package — in `internal/scan` or anywhere else. A language the scanner misreads is fixed in `patterns.go` or in the language-agnostic span/edge rules of `internal/scan/scan.go`, with a fixture that proves it.
- **Only `internal/gitctx` shells out to git** — the existing contract test `TestOnlyGitctxShellsOutToGit` (`internal/contract/nomock_test.go`) stays green unmodified. Every git read this plan needs is a new exported function in `internal/gitctx` (see "git: what `internal/gitctx` gains"). Tests build repositories with `internal/gitctx/gittest`; git is never mocked.
- **No new runtime dependency, no tree-sitter, no cgo.** `go.mod` keeps exactly `gopkg.in/yaml.v3` (`TestGoModRequiresExactlyYAML`), and the release binary stays `CGO_ENABLED=0` and statically linked.
- **rtdd never invokes graphify.** `internal/graphify` opens files; nothing in the tree executes a `graphify` binary or module, and no output offers to. When graphify is ignored the output *suggests* `graphify --update` to the human (spec §5 step 4, §12).
- **Nothing in the existing pipeline is removed or changed.** `seed`, `run`, `which`, `status`, `explain`, `doctor`, the adapters, `map.jsonl` and `--json` schema 2 stay exactly as they are; `rtdd graph` is added beside them. Rounds, `which` on the graph, schema 3 for `which` and deleting the coverage pipeline are PRD **#410**; skill text and `init` migration are PRD #411.
- **Spec §12 is settled, not open:** `.rtdd/graph.json` is a gitignored cache; graphify is ignored past 50 % stale and never run; all same-named definitions are linked. No task revisits these.
- `scripts/ci-local.sh` exits 0 at the end of every task.

## Review Focus

- A **line inside two nested nodes** — it must belong to the innermost one, and the outer node must get no calls edge from that line. Test in Task 1 (`TestInnermostNodeOwnsALineAndAClassHasMethodEdges`).
- A **graphify edge from an unchanged file into a stale file whose target was renamed or moved** — it must be re-pointed to the scanner node of the same name in the same file, or dropped; never left pointing at a node that no longer exists. Tests in Task 8 (re-point to a differently-qualified ID; drop on rename).
- A **cached file calling a function in a re-scanned file that was renamed** — calls edges are re-linked from the cached call *names* on every build, so the cached caller's edge disappears rather than dangling. Test in Task 7.
- **Exactly 50 % stale** — spec says graphify is ignored when the stale set *exceeds* `max_stale_ratio`; 2 of 4 keeps graphify. Test in Task 9.
- A **README edit after graphify's commit** — not a code file, so not stale, and it must not push a project over the 50 % line. Test in Task 8 (`a non-code file is never stale`).
- The **warm `rtdd graph` on this repository** — under 1 s requires not rewriting an unchanged cache and not re-reading unchanged files. Test in Task 10.

## Node.ID

A node's ID is **`file::` followed by the names of every enclosing node, outermost first, then its own name, joined by `::`** — `src/calc.py::add`, `src/calc.py::Calc::plus`, `src/calc.py::Calc::run::step`. A test node's ID is therefore its runnable id (spec §3: `file::name`), and a test method inside a test class reads `tests/test_calc.py::TestCalc::test_plus`, which is pytest's own spelling.

When two nodes in one file would share an ID (Rust's `struct Calc` and `impl Calc`, Java overloads), the first in line order keeps the plain ID and **every later one gets `@<start>`** appended — `src/calc.rs::Calc@5`. The suffix is the only part of an ID that moves when lines move; it only ever appears on a duplicate.

graphify nodes are given IDs by the same rule. graphify records nesting only through `method` edges, so a graphify method is `file::Class::name` (its class from the `method` edge) and anything else is `file::name`. Where graphify's flat ID and the scanner's nested one differ (a nested function), Task 8's re-pointing by **name within the file** is what reconciles them — IDs are never compared across sources.

`Kind` is decided by nesting, not by the pattern that matched: a callable whose innermost enclosing node is a class is `method`, any other callable is `func`; class-shaped definitions are `class`; `it/test/describe/context('label'` blocks are `test` (and their `Name` is the label).

## Package boundaries

| Package | Owns | Imports (rtdd) | Touches git |
|---|---|---|---|
| `internal/graph` | `Node`, `Edge`, `Kind`, `Relation`, `Graph`, `Sort`; `Config`, `DefaultConfig`, `LoadConfig`; `IsTestFile`, `Classify` | `internal/paths` | no |
| `internal/scan` | `Filter` (§4.1), the pattern table `patterns.go` (§4.2), `ScanFile`, `ScanFiles`, `Link`, `Assemble` (§4.3), `Fingerprint` | `internal/graph`, `internal/paths` | no |
| `internal/graphify` | `Load`, `Graph`, `ErrAbsent`, `DefaultPath` | `internal/graph` | no — reads files only |
| `internal/graphbuild` | `Build`, `Options`, `Result`, the cache (`cache.go`), `StaleSet` and the ignore triggers (`stale.go`), `Overlay` (`overlay.go`) | all three above, `internal/gitctx` | via `internal/gitctx` only |
| `cmd/rtdd` (`graph.go`) | `rtdd graph [--json]` | `internal/graph`, `internal/graphbuild`, `internal/gitctx` | via `internal/gitctx` only |

`internal/scan` takes repo-relative file lists from its caller instead of calling `gitctx.ListFiles` itself; `graphbuild.Build` does the listing (spec §4.1's "`gitctx.ListFiles`") and hands `scan.Filter` the result. That keeps every scanner test free of git and puts every git read in one package.

## git: what `internal/gitctx` gains

| Function | Git | Used by | Task |
|---|---|---|---|
| `BlobIDs(repoRoot string) (map[string]string, error)` | `git ls-tree -r -z --full-tree HEAD` — path → 40-hex blob id of HEAD's tree; an unborn HEAD is an empty map | the cache: re-scan when a file's blob changed (§4.5) | 7 |
| `DiffNamesSince(repoRoot, commit string) ([]string, error)` | `git diff --name-only -z --no-renames <commit> --` — committed since, staged and unstaged, deletions included; sorted | stale set source 1 (§5 step 1) | 8 |
| `CommitKnown(repoRoot, sha string) bool` | `git cat-file -e <sha>^{commit}` — false for empty, unknown, rebased-away or shallow-missing | ignore trigger 2 (§5 step 4) | 9 |

**Untracked files need no new function.** The existing `gitctx.ChangedSet(root, "HEAD")` already returns them (as `Added`, whole file) together with the working-tree changed set, and `gitctx.ListFiles` already lists them for the scanner. `graphbuild.Build` calls `ChangedSet` once and uses it for both "the current changed set" and "untracked" in §5 step 1, and for "in the working-tree changed set" in §4.5. HEAD's short sha (`built_at_commit` of a scanner graph) is the existing `gitctx.HeadSHA`.

## `.rtdd/config.yaml` keys

`graph.LoadConfig(root)` reads them, over `graph.DefaultConfig()`. A key that is present **replaces** its default wholesale (lists are not merged); an absent key keeps it; keys the graph does not own (`stale_commits`, `drift_guard`, `hub_threshold`, `adapters`) are ignored, not rejected. A missing file is the defaults. A malformed file, a glob `paths.ValidateGlob` rejects, or a `max_stale_ratio` outside (0, 1] is an error — `rtdd graph` exits 2 on it.

| Key | Default | Spec | Read by |
|---|---|---|---|
| `scan_exclude` | `vendor/**`, `node_modules/**`, `third_party/**`, `**/*.min.js`, `dist/**`, `build/**`, `.rtdd/**`, `graphify-out/**` | §4.1 | `scan.Filter` (Task 1) |
| `test_files` | `**/test_*`, `**/*_test.*`, `**/*.test.*`, `**/*.spec.*`, `**/*Test.*`, `**/*Tests.*`, `**/tests/**`, `**/test/**`, `**/spec/**`, `**/__tests__/**` | §6 | `graph.Classify` (Task 4) |
| `test_exclude` | `**/testdata/**`, `**/fixtures/**` | §6 ("never test files") | `graph.Classify` (Task 4) |
| `graphify_path` | `graphify-out/graph.json` (its `manifest.json` is read from the same directory) | §5 | `graphify.Load` (Task 5) |
| `max_stale_ratio` | `0.5` | §5 step 4, §12 | `graphbuild` staleness (Task 9) |

`LoadConfig` and every key land in Task 1, so the tasks that read a key later only consume it; each of those tasks owns the test that its override is honoured.

## `.rtdd/graph.json` on disk

graphify's `graph.json` node-link shape — so anything that reads a graphify graph can open it — plus rtdd's own keys, every one prefixed `rtdd_`:

```json
{
  "directed": false,
  "multigraph": false,
  "graph": {},
  "nodes": [
    {"id": "src/calc.py::add", "label": "add()", "file_type": "code", "source_file": "src/calc.py",
     "source_location": "L1", "rtdd_name": "add", "rtdd_kind": "func", "rtdd_start": 1, "rtdd_end": 2},
    {"id": "src/calc.py::total", "label": "total()", "file_type": "code", "source_file": "src/calc.py",
     "source_location": "L5", "rtdd_name": "total", "rtdd_kind": "func", "rtdd_start": 5, "rtdd_end": 6,
     "rtdd_calls": ["add"]}
  ],
  "links": [{"source": "src/calc.py::total", "target": "src/calc.py::add", "relation": "calls"}],
  "built_at_commit": "1d64301",
  "rtdd_cache": 1,
  "rtdd_scanner": "3f9c0a1b2d4e5f60",
  "rtdd_files": {"src/calc.py": "8d3e0b6f…40 hex", "README.md": "…", "tests/test_calc.py": ""}
}
```

- It holds **the scanner's** graph only — never graphify's nodes — for every file the scanner has read: all files on the scanner path, the stale files on the graphify path.
- `rtdd_files` maps each cached file to the HEAD blob id it was scanned from; `""` means it was scanned from working-tree content (it was in the changed set) and is re-scanned next time regardless. A file with no nodes is still listed, so it is not re-read.
- `rtdd_calls` is the node's called **names**, not edges. Calls edges are re-linked from names on every build (Task 7), which is what keeps a cached caller's edge correct when the callee's file is re-scanned. `links` is written for graphify-shaped readers and is not read back except for `method` edges.
- `rtdd_cache` (format version, `1`) and `rtdd_scanner` (`scan.Fingerprint()`: a hash of the pattern table, the keyword set and a `spanRules` version constant) must both match, or the whole cache is a miss. A corrupt, foreign or older cache is rebuilt, never an error.
- The file is written atomically (temp file + rename), and **not rewritten** when a build re-scanned nothing and the cache already covers exactly the files scanned (Task 7; it is what makes Task 10's warm bound).
- It is gitignored (§12). `rtdd init` adding that line is PRD #411; this plan adds `/.rtdd/graph.json` to *this* repository's `.gitignore` in Task 7 so running `rtdd graph` here never dirties the tree.

## Decisions this plan settles

The spec leaves these to "best-effort"; they are settled here so ten lanes do not settle them ten ways. Each is pinned by a test in the task named.

1. **End line, indentation case** (§4.3): the last **non-blank** line before the next non-blank line indented at or below the definition (trailing blank lines are not part of a node). Task 1.
2. **Multi-line headers**: a definition line whose `(` or `[` is still open continues to the line that closes it (at most 50 lines); the brace/indent decision is taken at the end of that header. This is what makes a black-formatted `def run(\n    self,\n):` and a Go signature split over lines span their bodies. Language-agnostic. Task 1.
3. **Brace mode** is entered when braces are still open at the end of the header — not merely when the line contains `{` — so `def f(x={}):` is indentation-ruled and `foo() { return 1; }` is a one-line node. Task 1.
4. **A nested node never outlives its parent**: its end is clamped to the parent's. Task 1.
5. **Calls are attributed by ownership**: a call on a line counts for the innermost node owning that line, so a class gets no calls edge from its methods' bodies. A node's own definition line is excluded (spec §4.3); so a one-line node's calls are not seen — accepted. No self edges. Task 1.
6. **Call names** are whole words before `(`, preceded by a non-word character (`obj.add(` names `add`); control keywords (`if(`, `while(`) and bare `$` (shell's `$(`) are not names. Task 1. Task 3 adds the **command form** (`add 1 2`, `x=$(add 1 2)`, Ruby's `log n`): a word first in a statement, followed by an argument or nothing, that is not an assignment — without it shell and Ruby tests never link to the functions they call.
7. **The closed relation set drops more than §3's list**: graphify edges whose relation is not one of `calls|method|inherits|implements|references` — `imports` and `defines` included — are dropped, as are edges with either end dropped. graphify's **file nodes** (label equal to the file's base name) are dropped (only `contains` and `imports` ever touch them), as are code nodes with no `source_file` (external references). Task 5.
8. **graphify nodes**: name is the label without a leading `.` and a trailing `()`; a label ending `)` is callable (a `method` when it is the target of a `method` edge, else `func`); anything else is `class`; `Start` is `source_location`'s line and **`End = Start`** — graphify records start lines only, which is why every changed file is always the scanner's (spec §5). Task 5.
9. **graphify's `manifest.json`** keys are absolute paths; they are made relative to the root recorded in `graphify-out/.graphify_root` (the repository root when absent), and keys outside it are dropped. A missing `manifest.json` contributes nothing to the stale set rather than marking every file stale. Task 5.
10. **The stale set is code files only**: a file graphify holds code nodes for, or a listed file sharing an extension with one. A `README.md` edit does not make graphify stale. The ratio is `|stale| / |graphify's code files|`; more than `max_stale_ratio` ignores graphify, exactly `max_stale_ratio` keeps it, and a graphify graph with no code files is always ignored as too stale. Tasks 8, 9.
11. **Re-pointing** (§5 step 3) links to **every** scanner node of the target's name in the target's file — the same over-link rule as calls. Task 8.
12. **`rtdd graph --json`** uses spec §9's envelope — `{"schema": 3, "command": "graph", "graph": {…}}` — with the `graph` object `rtdd which` will carry in N2: `source`, `built_at_commit`, `stale_files` (a **count**, as §9's example), `graphify_ignored` (only when ignored: `built_at_commit_missing` | `built_at_commit_unknown` | `too_stale`), plus `nodes`, `edges`, `tests`. `which`'s schema 2 is untouched. Tasks 6, 8, 9.

## Acceptance-criterion map

| PRD #409 AC | What | Task(s) | Issue(s) |
|---|---|---|---|
| AC1 | `internal/graph` model exactly as §3; closed relation set | 1 | #420 |
| AC2 | `internal/scan` §4.1–4.3 | 1 | #420 |
| AC3 | 8-language fixture corpus, exact | 1 (Python, Go), 2 (TS/JS, Java, Rust), 3 (Ruby, Lua, Bash) | #420, #421, #422 |
| AC4 | graphify loader keeps code nodes, drops non-code relations | 5 | #424 |
| AC5 | staleness overlay against a real git repository | 8 | #427 |
| AC6 | graphify ignored, with a reason, on each of three triggers | 9 | #428 |
| AC7 | test classification by generic globs | 4 | #423 |
| AC8 | `.rtdd/graph.json` cache, re-scan only changed blobs and the changed set | 7 | #426 |
| AC9 | `rtdd graph [--json]` source, counts, `built_at_commit`, `stale_files` | 6 (`scanner`), 8 (`graphify+scanner`) | #425, #427 |
| AC10 | 10 000 files cold < 10 s; this repo warm < 1 s | 10 | #429 |
| AC11 | `scripts/ci-local.sh` exits 0 | every task; owned by 10 | #429 |

Order: Task 1 first; Tasks 2, 4 and 5 after it (in parallel); Task 3 after 2 (both edit `patterns.go`); Task 6 after 1 and 4; Tasks 7 and 8 after 6 (8 also after 5) — **7 and 8 may land in either order**, which is why Task 6 declares every `Result` field and the final `Build` is written out in Task 9; Task 9 after 8; Task 10 last.

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `internal/graph/graph.go` | `Node`, `Edge`, `Kind`, `Relation`, `Relations`, `ParseRelation`, `Graph`, `Sort` | 1 |
| `internal/graph/config.go` | `Config`, `DefaultConfig`, `LoadConfig` | 1 |
| `internal/graph/classify.go` | `IsTestFile`, `Classify` | 4 |
| `internal/scan/patterns.go` | **the only language-shaped file**: the §4.2 table, keywords, `matchDefinition`, `Fingerprint` | 1, 2, 3, 7 |
| `internal/scan/scan.go` | `FileResult`, `ScanFile`, `ScanFiles`, `Link`, `Assemble`; spans, ownership, edges | 1, 3 |
| `internal/scan/filter.go` | `Filter`, `MaxFileSize` | 1 |
| `internal/scan/testdata/<lang>/` | one fixture project + `want.json` per language: `python`, `go`, `typescript`, `java`, `rust`, `ruby`, `lua`, `bash` | 1, 2, 3 |
| `internal/graphify/load.go` | `Load`, `Graph`, `ErrAbsent`, `DefaultPath` | 5 |
| `internal/graphify/testdata/graphify-out/` | a graphify-shaped `graph.json`, `manifest.json`, `.graphify_root` | 5 |
| `internal/gitctx/graphgit.go` | `BlobIDs`, `DiffNamesSince`, `CommitKnown` | 7, 8, 9 |
| `internal/graphbuild/build.go` | `Build`, `Options`, `Result`, source and ignore constants | 6–9 |
| `internal/graphbuild/cache.go` | `.rtdd/graph.json` read/write, `scanCached` | 7 |
| `internal/graphbuild/stale.go` | `StaleSet`, `staleness` (ignore triggers) | 8, 9 |
| `internal/graphbuild/overlay.go` | `Overlay` | 8 |
| `cmd/rtdd/graph.go` | `cmdGraph`, `graphJSON`; `main.go` gains the `graph` case and usage line | 6 |
| `internal/contract/plan_n1_test.go` | this document's own contract (issue #419) | — |

---

### Task 1: The graph model and the name-match scanner, exact on Python and Go

**Issue:** #420

**Discharges:** PRD #409 AC1, AC2, AC3 (Python and Go). Spec §3, §4.1–4.4.

**Files:**
- Create: `internal/graph/graph.go`, `internal/graph/config.go`, `internal/scan/patterns.go`, `internal/scan/scan.go`, `internal/scan/filter.go`
- Create: `internal/scan/testdata/python/{calc.py,test_calc.py,want.json}`, `internal/scan/testdata/go/{calc.go,calc_test.go,want.json}`
- Test: `internal/graph/graph_test.go`, `internal/graph/config_test.go`, `internal/scan/filter_test.go`, `internal/scan/rules_test.go`, `internal/scan/fixtures_test.go`

**Interfaces:**
- Produces: `graph.Node{ID, File, Name string; Kind Kind; Start, End int; IsTest bool}`; `graph.Edge{From, To string; Relation Relation}`; `graph.Kind` (`KindFunc|KindMethod|KindClass|KindTest`); `graph.Relation` (`RelCalls|RelMethod|RelInherits|RelImplements|RelReferences`), `graph.Relations`, `graph.ParseRelation(string) (Relation, bool)`; `graph.Graph{Nodes []Node; Edges []Edge}`, `graph.Sort(*Graph)`; `graph.Config{ScanExclude, TestFiles, TestExclude []string; GraphifyPath string; MaxStaleRatio float64}`, `graph.DefaultConfig() Config`, `graph.LoadConfig(root string) (Config, error)`.
- Produces: `scan.MaxFileSize`; `scan.Filter(root string, files, exclude []string) []string`; `scan.FileResult{Path string; Nodes []graph.Node; Edges []graph.Edge; Calls map[string][]string}`; `scan.ScanFile(rel string, src []byte) FileResult`; `scan.ScanFiles(root string, files []string) []FileResult`; `scan.Link(nodes []graph.Node, calls map[string][]string) []graph.Edge`; `scan.Assemble([]FileResult) graph.Graph`.
- Consumes: `paths.ValidateGlob`, `paths.MatchGlob` (`internal/paths/glob.go` — `**` matches zero or more segments).

- [ ] **Step 1: Write the failing tests**

`internal/graph/graph_test.go`:

```go
package graph

import (
	"reflect"
	"testing"
)

// PRD #409 AC1: the model is spec §3 field for field, and the relation set is closed.
func TestNodeAndEdgeAreSpecSection3FieldForField(t *testing.T) {
	fields := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			out = append(out, rt.Field(i).Name)
		}
		return out
	}
	if got, want := fields(Node{}), []string{"ID", "File", "Name", "Kind", "Start", "End", "IsTest"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Node fields = %v, want %v", got, want)
	}
	if got, want := fields(Edge{}), []string{"From", "To", "Relation"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Edge fields = %v, want %v", got, want)
	}
	if got, want := []Kind{KindFunc, KindMethod, KindClass, KindTest}, []Kind{"func", "method", "class", "test"}; !reflect.DeepEqual(got, want) {
		t.Errorf("kinds = %v, want %v", got, want)
	}
}

func TestRelationSetIsClosed(t *testing.T) {
	for _, s := range []string{"calls", "method", "inherits", "implements", "references"} {
		if r, ok := ParseRelation(s); !ok || string(r) != s {
			t.Errorf("ParseRelation(%q) = %q, %v; want it kept", s, r, ok)
		}
	}
	for _, s := range []string{"contains", "conceptually_related_to", "semantically_similar_to",
		"shares_data_with", "cites", "imports", "defines", "", "Calls"} {
		if r, ok := ParseRelation(s); ok {
			t.Errorf("ParseRelation(%q) = %q, true; the relation set is closed", s, r)
		}
	}
	if len(Relations) != 5 {
		t.Errorf("Relations has %d members, want the 5 of spec §3", len(Relations))
	}
}
```

`internal/graph/config_test.go`:

```go
package graph

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadConfigWithoutAFileIsTheSpecDefaults(t *testing.T) {
	got, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		ScanExclude: []string{"vendor/**", "node_modules/**", "third_party/**", "**/*.min.js",
			"dist/**", "build/**", ".rtdd/**", "graphify-out/**"},
		TestFiles: []string{"**/test_*", "**/*_test.*", "**/*.test.*", "**/*.spec.*", "**/*Test.*",
			"**/*Tests.*", "**/tests/**", "**/test/**", "**/spec/**", "**/__tests__/**"},
		TestExclude:   []string{"**/testdata/**", "**/fixtures/**"},
		GraphifyPath:  "graphify-out/graph.json",
		MaxStaleRatio: 0.5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadConfig = %+v\nwant %+v", got, want)
	}
}

// A present key replaces its default wholesale; keys the graph does not own are ignored.
func TestLoadConfigKeyReplacesItsDefaultAndOtherKeysAreIgnored(t *testing.T) {
	root := writeConfig(t, "stale_commits: 50\nscan_exclude: [\"gen/**\"]\n")
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gen/**"}; !reflect.DeepEqual(got.ScanExclude, want) {
		t.Errorf("ScanExclude = %v, want %v", got.ScanExclude, want)
	}
	if got.GraphifyPath != "graphify-out/graph.json" {
		t.Errorf("an absent key lost its default: GraphifyPath = %q", got.GraphifyPath)
	}
}

func TestLoadConfigRejectsAMalformedGlobAndRatio(t *testing.T) {
	for _, body := range []string{"scan_exclude: [\"[\"]\n", "max_stale_ratio: 0\n", "max_stale_ratio: 1.5\n", "scan_exclude: {\n"} {
		if _, err := LoadConfig(writeConfig(t, body)); err == nil {
			t.Errorf("LoadConfig accepted %q", body)
		}
	}
}
```

`internal/scan/filter_test.go`:

```go
package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func write(t *testing.T, root, rel string, b []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Spec §4.1: binary, oversized and scan_exclude'd files are never read.
func TestFilterDropsBinaryOversizedAndEveryDefaultExclude(t *testing.T) {
	root := t.TempDir()
	src := []byte("def f():\n    return 1\n")
	files := []string{
		"app/keep.py",
		"vendor/x.py", "node_modules/x.js", "third_party/x.c", "web/app.min.js",
		"dist/x.js", "build/x.py", ".rtdd/graph.json", "graphify-out/graph.json",
		"app/nul.py", "app/late_nul.py", "app/big.py", "app/exactly_1mib.py",
	}
	for _, f := range files {
		write(t, root, f, src)
	}
	write(t, root, "app/nul.py", append([]byte("def f():\x00"), src...))
	write(t, root, "app/late_nul.py", append([]byte(strings.Repeat("#\n", 4097)), 0)) // NUL past 8 KiB: kept
	write(t, root, "app/big.py", []byte(strings.Repeat("x", MaxFileSize+1)))
	write(t, root, "app/exactly_1mib.py", []byte(strings.Repeat("x", MaxFileSize)))

	got := Filter(root, files, graph.DefaultConfig().ScanExclude)
	want := []string{"app/keep.py", "app/late_nul.py", "app/exactly_1mib.py"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter = %v, want %v", got, want)
	}
}
```

`internal/scan/rules_test.go`:

```go
package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func TestInnermostNodeOwnsALineAndAClassHasMethodEdges(t *testing.T) {
	r := ScanFile("pkg/shapes.py", []byte("class Shape:\n    def area(self):\n        return measure(self)\n\n    def name(self):\n        return label()\n"))
	if got, want := r.Calls["pkg/shapes.py::Shape::area"], []string{"measure"}; !reflect.DeepEqual(got, want) {
		t.Errorf("area calls %v, want %v", got, want)
	}
	if got := r.Calls["pkg/shapes.py::Shape"]; got != nil {
		t.Errorf("the class owns no line its methods own, but calls %v", got)
	}
	want := []graph.Edge{
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::area", Relation: graph.RelMethod},
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::name", Relation: graph.RelMethod},
	}
	if !reflect.DeepEqual(r.Edges, want) {
		t.Errorf("edges = %v\nwant %v", r.Edges, want)
	}
}

func TestEverySameNamedDefinitionIsCalledButNotFromTheDefinitionLine(t *testing.T) {
	var nodes []graph.Node
	calls := map[string][]string{}
	for _, f := range []struct{ path, src string }{
		{"a.py", "def load():\n    return 1\n"},
		{"b.py", "def load():\n    return 2\n"},
		{"c.py", "def start():\n    return load()\n\n\ndef run(cb=load()):\n    return 0\n"},
	} {
		r := ScanFile(f.path, []byte(f.src))
		nodes = append(nodes, r.Nodes...)
		for k, v := range r.Calls {
			calls[k] = v
		}
	}
	want := []graph.Edge{
		{From: "c.py::start", To: "a.py::load", Relation: graph.RelCalls},
		{From: "c.py::start", To: "b.py::load", Relation: graph.RelCalls},
	}
	if got := Link(nodes, calls); !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}
```

`internal/scan/fixtures_test.go` — the corpus test every later fixture task extends:

```go
package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// fixtureLangs is spec §4.4's corpus. Each directory under testdata/ is one small project
// and its want.json; Tasks 2 and 3 of docs/plans/09-node-graph-core.md extend this list.
var fixtureLangs = []string{"python", "go"}

type wantNode struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Kind  graph.Kind `json:"kind"`
	Start int        `json:"start"`
	End   int        `json:"end"`
}

type wantCall struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type want struct {
	Nodes []wantNode `json:"nodes"`
	Calls []wantCall `json:"calls"`
}

// PRD #409 AC3 / spec §4.4: the scanner's nodes and calls edges for each fixture project
// are EXACTLY want.json — nothing missing, nothing extra. No toolchain is invoked.
func TestScannerFixturesExact(t *testing.T) {
	for _, lang := range fixtureLangs {
		t.Run(lang, func(t *testing.T) {
			dir := filepath.Join("testdata", lang)
			b, err := os.ReadFile(filepath.Join(dir, "want.json"))
			if err != nil {
				t.Fatal(err)
			}
			var w want
			if err := json.Unmarshal(b, &w); err != nil {
				t.Fatal(err)
			}

			var files []string
			err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() == "want.json" {
					return err
				}
				rel, _ := filepath.Rel(dir, p)
				files = append(files, filepath.ToSlash(rel))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			g := Assemble(ScanFiles(dir, files))

			var gotNodes []wantNode
			for _, n := range g.Nodes {
				gotNodes = append(gotNodes, wantNode{n.ID, n.Name, n.Kind, n.Start, n.End})
			}
			var gotCalls []wantCall
			for _, e := range g.Edges {
				if e.Relation == graph.RelCalls {
					gotCalls = append(gotCalls, wantCall{e.From, e.To})
				}
			}
			if !reflect.DeepEqual(gotNodes, w.Nodes) {
				t.Errorf("nodes:\n got %+v\nwant %+v", gotNodes, w.Nodes)
			}
			if !reflect.DeepEqual(gotCalls, w.Calls) {
				t.Errorf("calls:\n got %+v\nwant %+v", gotCalls, w.Calls)
			}
		})
	}
}
```

The two fixture projects. `internal/scan/testdata/python/calc.py`:

```python
"""A tiny calculator."""


def add(a, b):
    return a + b


def total(items):
    result = 0
    for item in items:
        result = add(result, item)
    return result


class Calc:
    def __init__(self):
        self.memory = 0

    def plus(self, x):
        self.memory = add(self.memory, x)
        return self.memory

    def run(
        self,
        values,
    ):
        def step(v):
            return self.plus(v)

        return [step(v) for v in values]
```

`internal/scan/testdata/python/test_calc.py`:

```python
from calc import Calc, add, total


def test_add():
    assert add(1, 2) == 3


class TestCalc:
    def test_plus(self):
        c = Calc()
        assert c.plus(2) == 2

    def test_total(self):
        assert total([1, 2]) == 3
```

`internal/scan/testdata/python/want.json` — `run` spans its four-line header and its body; the nested `step` is a `func` (its parent is a method, not a class); the blank line 29 is not part of `step`; the test class gets no calls edge from its methods' bodies:

```json
{
  "nodes": [
    {"id": "calc.py::add", "name": "add", "kind": "func", "start": 4, "end": 5},
    {"id": "calc.py::total", "name": "total", "kind": "func", "start": 8, "end": 12},
    {"id": "calc.py::Calc", "name": "Calc", "kind": "class", "start": 15, "end": 30},
    {"id": "calc.py::Calc::__init__", "name": "__init__", "kind": "method", "start": 16, "end": 17},
    {"id": "calc.py::Calc::plus", "name": "plus", "kind": "method", "start": 19, "end": 21},
    {"id": "calc.py::Calc::run", "name": "run", "kind": "method", "start": 23, "end": 30},
    {"id": "calc.py::Calc::run::step", "name": "step", "kind": "func", "start": 27, "end": 28},
    {"id": "test_calc.py::test_add", "name": "test_add", "kind": "func", "start": 4, "end": 5},
    {"id": "test_calc.py::TestCalc", "name": "TestCalc", "kind": "class", "start": 8, "end": 14},
    {"id": "test_calc.py::TestCalc::test_plus", "name": "test_plus", "kind": "method", "start": 9, "end": 11},
    {"id": "test_calc.py::TestCalc::test_total", "name": "test_total", "kind": "method", "start": 13, "end": 14}
  ],
  "calls": [
    {"from": "calc.py::Calc::plus", "to": "calc.py::add"},
    {"from": "calc.py::Calc::run", "to": "calc.py::Calc::run::step"},
    {"from": "calc.py::Calc::run::step", "to": "calc.py::Calc::plus"},
    {"from": "calc.py::total", "to": "calc.py::add"},
    {"from": "test_calc.py::TestCalc::test_plus", "to": "calc.py::Calc"},
    {"from": "test_calc.py::TestCalc::test_plus", "to": "calc.py::Calc::plus"},
    {"from": "test_calc.py::TestCalc::test_total", "to": "calc.py::total"},
    {"from": "test_calc.py::test_add", "to": "calc.py::add"}
  ]
}
```

`internal/scan/testdata/go/calc.go` (gofmt-clean: `gofmt -l .` walks `testdata/`):

```go
package calc

// Add returns a + b.
func Add(a, b int) int {
	return a + b
}

type Acc struct {
	total int
}

func (a *Acc) Push(n int) {
	a.total = Add(a.total, n)
}

func Sum(ns ...int) int {
	acc := &Acc{}
	for _, n := range ns {
		acc.Push(n)
	}
	return acc.total
}
```

`internal/scan/testdata/go/calc_test.go`:

```go
package calc

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(1, 2); got != 3 {
		t.Errorf("Add(1, 2) = %d", got)
	}
}

func TestSum(t *testing.T) {
	if got := Sum(1, 2, 3); got != 6 {
		t.Fatalf("got %d", got)
	}
}
```

`internal/scan/testdata/go/want.json` — `type Acc struct` is no node (the class row needs the keyword first), a receiver method is a `func` (it is not nested in a class), and `Add(1, 2)` inside the `Errorf` string is the same edge as the real call:

```json
{
  "nodes": [
    {"id": "calc.go::Add", "name": "Add", "kind": "func", "start": 4, "end": 6},
    {"id": "calc.go::Push", "name": "Push", "kind": "func", "start": 12, "end": 14},
    {"id": "calc.go::Sum", "name": "Sum", "kind": "func", "start": 16, "end": 22},
    {"id": "calc_test.go::TestAdd", "name": "TestAdd", "kind": "func", "start": 5, "end": 9},
    {"id": "calc_test.go::TestSum", "name": "TestSum", "kind": "func", "start": 11, "end": 15}
  ],
  "calls": [
    {"from": "calc.go::Push", "to": "calc.go::Add"},
    {"from": "calc.go::Sum", "to": "calc.go::Push"},
    {"from": "calc_test.go::TestAdd", "to": "calc.go::Add"},
    {"from": "calc_test.go::TestSum", "to": "calc.go::Sum"}
  ]
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/graph/ ./internal/scan/
```

Expected: FAIL — `no non-test Go files` in both packages, then `undefined: Node`, `undefined: LoadConfig`, `undefined: ScanFile`, `undefined: Filter`, `undefined: Assemble` once a stub file exists.

- [ ] **Step 3: Implement**

`internal/graph/graph.go`:

```go
// Package graph is the one model rtdd selects from, whatever built it (spec §3): nodes
// are functions, methods, classes and tests; edges are code relationships.
package graph

import (
	"slices"
	"sort"
)

// Kind is what a node is. Only the scanner and the graphify loader assign it.
type Kind string

const (
	KindFunc   Kind = "func"
	KindMethod Kind = "method"
	KindClass  Kind = "class"
	KindTest   Kind = "test"
)

// Relation is an edge's type. The set is closed: anything else is dropped on load.
type Relation string

const (
	RelCalls      Relation = "calls"
	RelMethod     Relation = "method"
	RelInherits   Relation = "inherits"
	RelImplements Relation = "implements"
	RelReferences Relation = "references"
)

// Relations is the closed relation set of spec §3, in spec order.
var Relations = []Relation{RelCalls, RelMethod, RelInherits, RelImplements, RelReferences}

// ParseRelation reports whether s names a relation in the closed set.
func ParseRelation(s string) (Relation, bool) {
	for _, r := range Relations {
		if string(r) == s {
			return r, true
		}
	}
	return "", false
}

// Node is one function, method, class or test. File is repo-relative and slash-separated;
// Start and End are 1-based and inclusive.
type Node struct {
	ID     string
	File   string
	Name   string
	Kind   Kind
	Start  int
	End    int
	IsTest bool
}

// Edge points From one node ID To another.
type Edge struct {
	From     string
	To       string
	Relation Relation
}

// Graph is nodes and edges. Nodes are ordered by File, then Start; Edges by From, To,
// Relation — so two builds of the same tree compare equal.
type Graph struct {
	Nodes []Node
	Edges []Edge
}

// Sort puts g in its canonical order: nodes by File then Start then ID, edges by From,
// To, Relation, with duplicate edges removed.
func Sort(g *Graph) {
	sort.Slice(g.Nodes, func(i, j int) bool {
		a, b := g.Nodes[i], g.Nodes[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.ID < b.ID
	})
	sort.Slice(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Relation < b.Relation
	})
	g.Edges = slices.Compact(g.Edges)
}
```

`internal/graph/config.go`:

```go
package graph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// Config is the node graph's part of .rtdd/config.yaml. Every list is generic — a glob
// over repo-relative paths — never a per-language setting (spec §4.1, §6).
type Config struct {
	ScanExclude   []string // files the scanner never reads (spec §4.1)
	TestFiles     []string // a node in a matching file may be a test (spec §6)
	TestExclude   []string // a file matching these is never a test file, whatever TestFiles says
	GraphifyPath  string   // repo-relative path of graphify's graph.json (spec §5)
	MaxStaleRatio float64  // graphify is ignored when more than this share of its code files is stale
}

// DefaultConfig is spec §4.1, §5 and §6's defaults.
func DefaultConfig() Config {
	return Config{
		ScanExclude: []string{"vendor/**", "node_modules/**", "third_party/**", "**/*.min.js",
			"dist/**", "build/**", ".rtdd/**", "graphify-out/**"},
		TestFiles: []string{"**/test_*", "**/*_test.*", "**/*.test.*", "**/*.spec.*", "**/*Test.*",
			"**/*Tests.*", "**/tests/**", "**/test/**", "**/spec/**", "**/__tests__/**"},
		TestExclude:   []string{"**/testdata/**", "**/fixtures/**"},
		GraphifyPath:  "graphify-out/graph.json",
		MaxStaleRatio: 0.5,
	}
}

// configFile is the on-disk shape. Pointers tell "absent" from "empty": a key that is
// present REPLACES its default wholesale; an absent key keeps it. Keys rtdd does not
// read here (stale_commits, adapters, ...) are ignored, not rejected.
type configFile struct {
	ScanExclude   *[]string `yaml:"scan_exclude"`
	TestFiles     *[]string `yaml:"test_files"`
	TestExclude   *[]string `yaml:"test_exclude"`
	GraphifyPath  *string   `yaml:"graphify_path"`
	MaxStaleRatio *float64  `yaml:"max_stale_ratio"`
}

// LoadConfig reads <root>/.rtdd/config.yaml over DefaultConfig. A missing file is the
// defaults; a malformed file, a malformed glob or a ratio outside (0, 1] is an error.
func LoadConfig(root string) (Config, error) {
	cfg := DefaultConfig()
	p := filepath.Join(root, ".rtdd", "config.yaml")
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var f configFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return cfg, fmt.Errorf("%s: %w", p, err)
	}
	for _, l := range []struct {
		key string
		in  *[]string
		out *[]string
	}{{"scan_exclude", f.ScanExclude, &cfg.ScanExclude}, {"test_files", f.TestFiles, &cfg.TestFiles}, {"test_exclude", f.TestExclude, &cfg.TestExclude}} {
		if l.in == nil {
			continue
		}
		for _, g := range *l.in {
			if err := paths.ValidateGlob(g); err != nil {
				return cfg, fmt.Errorf("%s: %s: %w", p, l.key, err)
			}
		}
		*l.out = *l.in
	}
	if f.GraphifyPath != nil {
		cfg.GraphifyPath = *f.GraphifyPath
	}
	if f.MaxStaleRatio != nil {
		if r := *f.MaxStaleRatio; r <= 0 || r > 1 {
			return cfg, fmt.Errorf("%s: max_stale_ratio %v is outside (0, 1]", p, r)
		}
		cfg.MaxStaleRatio = *f.MaxStaleRatio
	}
	return cfg, nil
}
```

`internal/scan/patterns.go` — the table. Row order matters (first match wins); the method row is last because it is the loosest, and it alone consults the keyword set and the trailing-`;` rule:

```go
package scan

import (
	"regexp"
	"strings"
)

// This file is the ONLY place in rtdd where the shape of a programming language appears
// (spec §4.2; PRD #409 global constraint). Everything else in the scanner — spans,
// ownership, edges — is language-agnostic. A language the table misreads is fixed HERE,
// with a fixture under testdata/ that proves it; never with a branch on a file extension.

// shape is what a definition line declares. Whether a callable is a func or a method is
// not decided here: it is a method when its innermost enclosing node is a class.
type shape int

const (
	shapeCallable shape = iota
	shapeClass
	shapeTest
)

// ident is one identifier segment; a dotted or colon-separated path names its last
// segment (spec §4.2: `function M.load(` names `load`, `func (s *S) Load(` names `Load`).
const ident = `[A-Za-z_$][\w$]*`
const path = ident + `(?:(?:\.|::?)` + ident + `)*`

// modifiers are the words that may precede a definition keyword.
const modifiers = `(?:(?:export|default|pub(?:\([^)]*\))?|async|local|static|unsafe|abstract|final|sealed|data|public|private|protected|internal|override|inline|suspend|open)\s+)*`

type pattern struct {
	shape shape
	re    *regexp.Regexp // group 1 is the name
}

var patterns = []pattern{
	// test: it('label' / test("label" / describe(`label` / context('label'
	{shapeTest, regexp.MustCompile("^\\s*(?:it|test|describe|context)\\s*\\(\\s*['\"`]([^'\"`]*)")},
	// func: def / fn / func / function / fun / sub / proc, optional Go receiver
	{shapeCallable, regexp.MustCompile(`^\s*` + modifiers + `(?:def|fn|func|function|fun|sub|proc)\s+(?:\([^)]*\)\s*)?(` + path + `)`)},
	// class: class / struct / interface / trait / impl / module / object / enum
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `(?:class|struct|interface|trait|module|object|enum)\s+(` + path + `)`)},
	{shapeClass, regexp.MustCompile(`^\s*` + modifiers + `impl(?:<[^>]*>)?\s+(?:` + path + `(?:<[^>]*>)?\s+for\s+)?(` + path + `)`)},
	// JS/TS assigned function: const Name = (async) function / (...) => / x =>
	{shapeCallable, regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let|var)\s+(` + ident + `)\s*=\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*=>|` + ident + `\s*=>)`)},
	// shell: Name() {
	{shapeCallable, regexp.MustCompile(`^\s*(` + ident + `)\s*\(\)\s*\{`)},
	// method: <type tokens> Name(<params>) <modifiers> {? — no trailing ';'
	{shapeCallable, regexp.MustCompile(`^\s*((?:[\w$<>\[\],.?*&:]+\s+)+)[*&]*(` + ident + `)\s*\([^;]*$`)},
}

// controlKeywords never name a definition and never open a method-pattern line: they
// are how a call statement (`return add(x)`, `defer close(ch)`) would otherwise read as
// `<type> Name(`.
var controlKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`if else elif for foreach while do switch case catch try
		finally return throw throws new delete yield await defer go goto raise assert print
		puts echo local not and or in is del lambda when unless until then sizeof typeof
		with import from using package require`) {
		controlKeywords[k] = true
	}
}

// methodTail is what may follow a method's parameter list on its definition line.
var methodTail = regexp.MustCompile(`\)\s*(?:[\w$<>\[\],.?&:]+\s*)*\{?\s*$`)

// matchDefinition reports whether line defines a node, and its shape and name.
func matchDefinition(line string) (string, shape, bool) {
	for i, p := range patterns {
		m := p.re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if i == len(patterns)-1 { // the method row
			for _, tok := range strings.Fields(m[1]) {
				if controlKeywords[tok] {
					return "", 0, false
				}
			}
			if controlKeywords[m[2]] || !methodTail.MatchString(line) {
				return "", 0, false
			}
			return m[2], p.shape, true
		}
		if p.shape == shapeTest {
			return m[1], p.shape, true
		}
		return lastSegment(m[1]), p.shape, true
	}
	return "", 0, false
}

func lastSegment(s string) string {
	if i := strings.LastIndexAny(s, ".:"); i >= 0 {
		return s[i+1:]
	}
	return s
}
```

`internal/scan/scan.go`:

```go
// Package scan is rtdd's built-in, language-agnostic code scanner (spec §4): definitions
// by the regex table in patterns.go, spans by braces or indentation, edges by name.
package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// FileResult is one file's scan: its nodes (IsTest unset), the method edges inside it,
// and the names each node calls. Calls edges are made by Link, over every file at once,
// because a call names a definition that may live anywhere.
type FileResult struct {
	Path  string
	Nodes []graph.Node
	Edges []graph.Edge
	Calls map[string][]string // node ID -> sorted, de-duplicated called names
}

// callSite is `X(` with X a whole word.
var callSite = regexp.MustCompile(`[A-Za-z_$][\w$]*\(`)

type def struct {
	name   string
	shape  shape
	start  int // 0-based
	end    int // 0-based, inclusive
	indent int
	parent int // index into defs, -1 at top level
}

// ScanFile scans one file's source. rel is its repo-relative, slash-separated path.
func ScanFile(rel string, src []byte) FileResult {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}

	var defs []def
	for i, line := range lines {
		name, sh, ok := matchDefinition(line)
		if !ok {
			continue
		}
		defs = append(defs, def{name: name, shape: sh, start: i, end: endLine(lines, i), indent: indentOf(lines[i]), parent: -1})
	}

	// Nesting: a node's parent is the nearest earlier node whose span holds its start;
	// a child never outlives its parent.
	var stack []int
	for i := range defs {
		for len(stack) > 0 && defs[stack[len(stack)-1]].end < defs[i].start {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			p := stack[len(stack)-1]
			defs[i].parent = p
			if defs[i].end > defs[p].end {
				defs[i].end = defs[p].end
			}
		}
		stack = append(stack, i)
	}

	res := FileResult{Path: rel, Calls: map[string][]string{}}
	ids := make([]string, len(defs))
	seen := map[string]bool{}
	for i, d := range defs {
		qual := d.name
		for p := d.parent; p >= 0; p = defs[p].parent {
			qual = defs[p].name + "::" + qual
		}
		id := rel + "::" + qual
		if seen[id] {
			id += "@" + strconv.Itoa(d.start+1)
		}
		seen[id] = true
		ids[i] = id

		kind := graph.KindFunc
		switch {
		case d.shape == shapeClass:
			kind = graph.KindClass
		case d.shape == shapeTest:
			kind = graph.KindTest
		case d.parent >= 0 && defs[d.parent].shape == shapeClass:
			kind = graph.KindMethod
		}
		res.Nodes = append(res.Nodes, graph.Node{ID: id, File: rel, Name: d.name, Kind: kind, Start: d.start + 1, End: d.end + 1})
		if d.parent >= 0 && defs[d.parent].shape == shapeClass {
			res.Edges = append(res.Edges, graph.Edge{From: ids[d.parent], To: id, Relation: graph.RelMethod})
		}
	}

	// Ownership: a line belongs to the innermost node containing it. Parents precede
	// their children in defs, so a later write is always the more inner node.
	owner := make([]int, len(lines))
	for i := range owner {
		owner[i] = -1
	}
	for i, d := range defs {
		for l := d.start; l <= d.end; l++ {
			owner[l] = i
		}
	}
	called := map[int]map[string]bool{}
	for l, line := range lines {
		o := owner[l]
		if o < 0 || defs[o].start == l {
			continue // top-level code, or the node's own definition line
		}
		for _, loc := range callSite.FindAllStringIndex(line, -1) {
			if loc[0] > 0 && isWord(line[loc[0]-1]) {
				continue
			}
			name := line[loc[0] : loc[1]-1]
			if controlKeywords[name] || strings.Trim(name, "$") == "" {
				continue
			}
			if called[o] == nil {
				called[o] = map[string]bool{}
			}
			called[o][name] = true
		}
	}
	for o, names := range called {
		var list []string
		for n := range names {
			list = append(list, n)
		}
		sort.Strings(list)
		res.Calls[ids[o]] = list
	}
	return res
}

// Link makes the calls edges: from each caller to EVERY node bearing a name it calls
// (spec §4.3 — over-linking is accepted; a direct call is never missed). nodes is the
// whole graph's node set, whichever source produced each node. No self edges.
func Link(nodes []graph.Node, calls map[string][]string) []graph.Edge {
	byName := map[string][]string{}
	for _, n := range nodes {
		byName[n.Name] = append(byName[n.Name], n.ID)
	}
	var out []graph.Edge
	for from, names := range calls {
		for _, name := range names {
			for _, to := range byName[name] {
				if to != from {
					out = append(out, graph.Edge{From: from, To: to, Relation: graph.RelCalls})
				}
			}
		}
	}
	g := graph.Graph{Edges: out}
	graph.Sort(&g)
	return g.Edges
}

func isWord(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func blank(line string) bool { return strings.TrimSpace(line) == "" }

// endLine is spec §4.3's end rule for the definition on line i (0-based). The header is
// the definition line plus any lines its open ( or [ carries on to; when the header
// leaves a { open the node ends where braces balance, otherwise at the last non-blank
// line before the next non-blank line indented at or below the definition (or EOF).
func endLine(lines []string, i int) int {
	var t tokenizer
	header := i
	for j := i; j < len(lines); j++ {
		t.line(lines[j])
		header = j
		if t.paren <= 0 || j-i >= 50 {
			break
		}
	}
	if t.brace > 0 {
		for j := header + 1; j < len(lines); j++ {
			t.line(lines[j])
			if t.brace <= 0 {
				return j
			}
		}
		return len(lines) - 1
	}
	in := indentOf(lines[i])
	end := len(lines) - 1
	for j := header + 1; j < len(lines); j++ {
		if !blank(lines[j]) && indentOf(lines[j]) <= in {
			end = j - 1
			break
		}
	}
	for end > header && blank(lines[end]) {
		end--
	}
	return end
}

// tokenizer counts brackets outside string literals and comments, best-effort (spec
// §4.3). String state ends at end of line; a /* block */ comment may span lines.
type tokenizer struct {
	paren, brace int
	block        bool
}

func (t *tokenizer) line(s string) {
	var quote byte
	for k := 0; k < len(s); k++ {
		c := s[k]
		switch {
		case t.block:
			if c == '*' && k+1 < len(s) && s[k+1] == '/' {
				t.block = false
				k++
			}
		case quote != 0:
			if c == '\\' {
				k++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '/' && k+1 < len(s) && s[k+1] == '/':
			return
		case c == '/' && k+1 < len(s) && s[k+1] == '*':
			t.block = true
			k++
		case c == '#' && (k == 0 || s[k-1] == ' ' || s[k-1] == '\t'):
			return
		case c == '(' || c == '[':
			t.paren++
		case c == ')' || c == ']':
			t.paren--
		case c == '{':
			t.brace++
		case c == '}':
			t.brace--
		}
	}
}

// ScanFiles scans each of files (repo-relative, already Filtered) under root. A file that
// cannot be read is skipped.
func ScanFiles(root string, files []string) []FileResult {
	var out []FileResult
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		out = append(out, ScanFile(rel, b))
	}
	return out
}

// Assemble is the scanner's whole graph: every file's nodes and method edges, plus the
// calls edges Link makes across all of them.
func Assemble(results []FileResult) graph.Graph {
	var g graph.Graph
	calls := map[string][]string{}
	for _, r := range results {
		g.Nodes = append(g.Nodes, r.Nodes...)
		g.Edges = append(g.Edges, r.Edges...)
		for id, names := range r.Calls {
			calls[id] = names
		}
	}
	g.Edges = append(g.Edges, Link(g.Nodes, calls)...)
	graph.Sort(&g)
	return g
}
```

`internal/scan/filter.go`:

```go
package scan

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// MaxFileSize is the largest file the scanner reads (spec §4.1).
const MaxFileSize = 1 << 20

// sniffLen is how much of a file is searched for a NUL byte (spec §4.1).
const sniffLen = 8 << 10

// Filter keeps the files the scanner reads: not matching exclude, at most MaxFileSize
// bytes, no NUL byte in the first 8 KiB (spec §4.1). files are repo-relative and
// slash-separated, as gitctx.ListFiles returns them; a file that cannot be read is
// skipped, never an error — it is not a definition the graph can hold.
func Filter(root string, files, exclude []string) []string {
	var out []string
	for _, rel := range files {
		if matchAny(exclude, rel) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileSize {
			continue
		}
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		head := make([]byte, sniffLen)
		n, _ := io.ReadFull(f, head)
		f.Close()
		if bytes.IndexByte(head[:n], 0) >= 0 {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run to verify they pass, then the guards**

```bash
go test ./internal/graph/ ./internal/scan/
go test -count=1 -run 'TestOnlyGitctxShellsOutToGit|TestGoModRequiresExactlyYAML|TestCompiledModulesAreYAMLOnly' ./internal/contract/
gofmt -l . && go vet ./...
```

Expected: PASS; no file listed by `gofmt -l`. The scanner holds no `exec` and no git call; `grep -rn '"\.py"\|"\.go"\|filepath.Ext' internal/scan/*.go` finds nothing.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/graph internal/scan
git commit -m "feat(graph): node graph model and name-match scanner, exact on Python and Go (closes #420)"
```

---

### Task 2: Scanner fixtures exact for TypeScript/JavaScript, Java and Rust

**Issue:** #421

**Discharges:** PRD #409 AC3 (TypeScript/JavaScript, Java, Rust). Spec §4.2–4.4.

**Files:**
- Create: `internal/scan/testdata/typescript/`, `internal/scan/testdata/java/`, `internal/scan/testdata/rust/` — each a small project and a `want.json`
- Modify: `internal/scan/patterns.go` (the TS/JS class-member row), `internal/scan/fixtures_test.go` (`fixtureLangs`)
- Test: `internal/scan/shapes_test.go` (shared helpers), `internal/scan/shapes_t2_test.go`

**Interfaces:**
- Consumes: `scan.ScanFile`, `scan.FileResult`, the `want.json` format of Task 1.
- Produces: `fixtureLangs` grows to `{"python", "go", "typescript", "java", "rust"}`; one new `patterns.go` row. No new exported API.

- [ ] **Step 1: Write the failing tests**

`internal/scan/shapes_test.go` — the helpers both shape tasks use:

```go
package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// span is a node as the shape tests pin it.
type span struct {
	ID         string
	Kind       graph.Kind
	Start, End int
}

func spans(rel, src string) []span {
	var out []span
	for _, n := range ScanFile(rel, []byte(src)).Nodes {
		out = append(out, span{n.ID, n.Kind, n.Start, n.End})
	}
	return out
}

func checkSpans(t *testing.T, rel, src string, want []span) {
	t.Helper()
	if got := spans(rel, src); !reflect.DeepEqual(got, want) {
		t.Errorf("%s nodes:\n got %+v\nwant %+v", rel, got, want)
	}
}
```

`internal/scan/shapes_t2_test.go`:

```go
package scan

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// TS/JS class members carry no type tokens before their name: `plus(x) {` is a method.
// A callback opener (`useEffect(() => {`) and a control statement (`if (x) {`) are not.
func TestTypeScriptClassMembersAreMethods(t *testing.T) {
	checkSpans(t, "src/calc.ts",
		"export class Calc {\n  private memory = 0;\n\n  plus(x: number): number {\n    if (x > 0) {\n      log(x);\n    }\n    return x;\n  }\n\n  async load(path) {\n    return read(path);\n  }\n}\n\nuseEffect(() => {\n  run();\n});\n",
		[]span{
			{"src/calc.ts::Calc", graph.KindClass, 1, 14},
			{"src/calc.ts::Calc::plus", graph.KindMethod, 4, 9},
			{"src/calc.ts::Calc::load", graph.KindMethod, 11, 13},
		})
}

// Java: a generic method is a method; a `;`-terminated declaration and a control
// statement are not nodes at all.
func TestJavaMethodsButNotDeclarationsOrControlStatements(t *testing.T) {
	checkSpans(t, "src/Calc.java",
		"public class Calc implements Op {\n    public <T> List<T> wrap(T x) {\n        return List.of(x);\n    }\n\n    int apply(int a, int b);\n\n    void run() {\n        if (ready()) {\n            while (busy()) {\n                step();\n            }\n        }\n    }\n}\n",
		[]span{
			{"src/Calc.java::Calc", graph.KindClass, 1, 15},
			{"src/Calc.java::Calc::wrap", graph.KindMethod, 2, 4},
			{"src/Calc.java::Calc::run", graph.KindMethod, 8, 14},
		})
}

// Rust: `impl<T> Trait for Type` names Type; struct and impl of one type are two class
// nodes (the second gets the @<start> suffix); `fn new` is a method, not a keyword.
func TestRustImplForNamesTheTypeAndNewIsAMethod(t *testing.T) {
	checkSpans(t, "src/calc.rs",
		"pub struct Calc {\n    total: i32,\n}\n\nimpl<T: Copy> Summer for Calc<T> {\n    fn new() -> Self {\n        Calc { total: 0 }\n    }\n}\n",
		[]span{
			{"src/calc.rs::Calc", graph.KindClass, 1, 3},
			{"src/calc.rs::Calc@5", graph.KindClass, 5, 9},
			{"src/calc.rs::Calc::new", graph.KindMethod, 6, 8},
		})
}
```

And in `internal/scan/fixtures_test.go`:

```go
var fixtureLangs = []string{"python", "go", "typescript", "java", "rust"}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestTypeScript|TestJava|TestRust|TestScannerFixturesExact' ./internal/scan/
```

Expected: FAIL — `TestTypeScriptClassMembersAreMethods` (`plus` and `load` are missing: a TS member has no type tokens, so the method row cannot match it) and `TestScannerFixturesExact/{typescript,java,rust}` (`open testdata/typescript/want.json: no such file or directory`). The Java and Rust shape tests already pass on Task 1's table; they pin it.

- [ ] **Step 3: Make it pass**

Add the member row to `patterns.go`, **directly before the method row** (an indented `Name(<params>)(: Type)? {` with no type tokens; `=>` after the parameters is a callback, not a member, so `useEffect(() => {` does not match), and reject a control keyword as its name the way the method row does:

```go
	// JS/TS class member: indented `Name(<params>)(: Type)? {` with no type tokens before it
	{shapeCallable, regexp.MustCompile(`^\s+(?:(?:static|async|get|set|public|private|protected|readonly|override)\s+)*(` + ident + `)\s*\([^)]*\)\s*(?::\s*[^={;]+)?\{\s*$`)},
```

```go
		if i == len(patterns)-2 && controlKeywords[m[1]] { // the member row
			return "", 0, false
		}
```

Then write the three fixture projects. Each exercises every shape §4.2 claims for that language: **typescript** — `function`, `export const x = async (…) =>`, `const y = function`, a class with members and a `static` member, and `describe('…', () => {` / `it('…', …)` blocks in a `*.test.ts` file; **java** — an interface with a `;` declaration, a class with a constructor, a generic method and an `@Override` method, a test class with `@Test void testX() {` methods, and `if`/`for`/`while` blocks; **rust** — `pub fn`, `struct`, `impl`, `impl Trait for Type`, `enum`, `trait`, and a `#[cfg(test)] mod tests` with `#[test] fn`. Write `want.json` **by hand, line by line from the source**, before running the scanner over it; then run the test and treat every difference as a question about the table or the span rules, never as a reason to copy the scanner's output into `want.json`.

Rust's `mod tests {` is not a class row (`mod` is not in §4.2's keyword list); its `fn`s are top-level `func`s. Leave it that way — adding `mod` is a §4.2 change.

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/scan/
```

Expected: PASS, including `TestScannerFixturesExact/python` and `/go` unchanged.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/scan
git commit -m "feat(scan): TypeScript/JavaScript, Java and Rust fixtures exact (closes #421)"
```

---

### Task 3: Scanner fixtures exact for Ruby, Lua and Bash — the 8-language corpus

**Issue:** #422

**Discharges:** PRD #409 AC3 (Ruby, Lua, Bash — the corpus is complete). Spec §4.2–4.4.

**Files:**
- Create: `internal/scan/testdata/ruby/`, `internal/scan/testdata/lua/`, `internal/scan/testdata/bash/` — each a small project and a `want.json`
- Modify: `internal/scan/patterns.go` (block-form test row, shell/Ruby closers in the keyword set), `internal/scan/scan.go` (command-form call sites), `internal/scan/fixtures_test.go` (`fixtureLangs`)
- Test: `internal/scan/shapes_t3_test.go`

**Interfaces:**
- Consumes: Task 2's `checkSpans` helper.
- Produces: `fixtureLangs` is all eight: `{"python", "go", "typescript", "java", "rust", "ruby", "lua", "bash"}`.

- [ ] **Step 1: Write the failing tests**

`internal/scan/shapes_t3_test.go`:

```go
package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// Lua names the last segment of a dotted or colon path (spec §4.2).
func TestLuaNamesTheLastSegment(t *testing.T) {
	checkSpans(t, "lua/m.lua",
		"local M = {}\n\nfunction M.load(path)\n  return path\nend\n\nfunction M:push(n)\n  self.n = n\nend\n\nreturn M\n",
		[]span{
			{"lua/m.lua::load", graph.KindFunc, 3, 4},
			{"lua/m.lua::push", graph.KindFunc, 7, 8},
		})
}

// Ruby: `it "label" do` is a test node; `end` closes by indentation (the end line itself
// is outside the span, spec §4.3); a paren-less call in command position is a call.
func TestRubyBlockTestsAndCommandCalls(t *testing.T) {
	src := "class Acc\n  def push(n)\n    log n\n  end\n\n  def log(n)\n    puts n\n  end\nend\n\ndescribe Acc do\n  it \"pushes\" do\n    Acc.new.push 1\n  end\nend\n"
	checkSpans(t, "spec/acc_spec.rb", src, []span{
		{"spec/acc_spec.rb::Acc", graph.KindClass, 1, 8},
		{"spec/acc_spec.rb::Acc::push", graph.KindMethod, 2, 3},
		{"spec/acc_spec.rb::Acc::log", graph.KindMethod, 6, 7},
		{"spec/acc_spec.rb::pushes", graph.KindTest, 12, 13},
	})
	r := ScanFile("spec/acc_spec.rb", []byte(src))
	want := []graph.Edge{{From: "spec/acc_spec.rb::Acc::push", To: "spec/acc_spec.rb::Acc::log", Relation: graph.RelCalls}}
	if got := Link(r.Nodes, r.Calls); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %+v, want %+v", got, want)
	}
}

// Shell calls a function by naming it first in a statement: `add 1 2`, `x=$(add 1 2)`.
// An assignment (`x=1`, `x = f()`) is not a call.
func TestShellCommandFormCalls(t *testing.T) {
	src := "add() {\n  echo $(( $1 + $2 ))\n}\n\nfunction total {\n  local x\n  x=$(add 1 2)\n  add 3 4 | cat\n}\n"
	checkSpans(t, "bin/calc.sh", src, []span{
		{"bin/calc.sh::add", graph.KindFunc, 1, 3},
		{"bin/calc.sh::total", graph.KindFunc, 5, 9},
	})
	r := ScanFile("bin/calc.sh", []byte(src))
	want := []graph.Edge{{From: "bin/calc.sh::total", To: "bin/calc.sh::add", Relation: graph.RelCalls}}
	if got := Link(r.Nodes, r.Calls); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %+v, want %+v", got, want)
	}
}
```

And in `internal/scan/fixtures_test.go`:

```go
var fixtureLangs = []string{"python", "go", "typescript", "java", "rust", "ruby", "lua", "bash"}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestLua|TestRuby|TestShell|TestScannerFixturesExact' ./internal/scan/
```

Expected: FAIL — `TestRubyBlockTestsAndCommandCalls` (no `pushes` test node: §4.2's test row needs `(`; no `push -> log` edge: `log n` has no `(`), `TestShellCommandFormCalls` (no `total -> add` edge: shell never writes `add(`), and `TestScannerFixturesExact/{ruby,lua,bash}` (no `want.json`). `TestLuaNamesTheLastSegment` passes on Task 1's table; it pins issue #422's `function M.load(` → `load`.

- [ ] **Step 3: Make it pass**

In `patterns.go`, a block-form test row directly after the `(`-form one — a quoted label followed, on the same line, by `do` and optional block parameters, so a shell `test "$x" = y` never matches:

```go
	// test, block form: it "label" do / describe 'label' do |x|
	{shapeTest, regexp.MustCompile(`^\s*(?:it|test|describe|context)\s+['"]([^'"]*)['"].*\bdo\s*(?:\|[^|]*\|)?\s*$`)},
```

and `end fi done esac` added to `controlKeywords`. In `scan.go`, the command form, collected beside `callSite` for every owned line:

```go
// commandSite is a word in command position — the first word of a statement, followed by
// an argument or nothing — as in `add 1 2` (shell) or `helper x` (Ruby). An assignment
// (`x = 1`, `x := f()`) or a member access is not one.
var commandSite = regexp.MustCompile(`(?:^|;|&&|\|\||\||\$\(|` + "`" + `)\s*([A-Za-z_][\w]*)(?:\s+[^\s=:+\-*/%<>!&|^.,;)\]}]|\s*$)`)
```

```go
		for _, m := range commandSite.FindAllStringSubmatch(line, -1) {
			if !controlKeywords[m[1]] {
				if called[o] == nil {
					called[o] = map[string]bool{}
				}
				called[o][m[1]] = true
			}
		}
```

This is still one rule for every language — it only ever produces a *name*, and `Link` only makes an edge to a name something defines, so Python's `x = 1` adds nothing and the Python and Go `want.json` files do not change. It is the plan's one extension of §4.3's `X(` rule; without it no Bash or Ruby test is linked to what it calls.

Ruby and Lua close with `end`, which the indentation rule leaves **outside** the span (spec §4.3: "the line before the next non-blank line indented at or below the definition") — `want.json` says so; do not special-case `end`. Then write the three fixtures: **ruby** — `module`, `class`, `def self.x`, `def x`, paren-less calls, and an `_spec.rb` with `describe X do` / `it "…" do`; **lua** — `local function`, `function M.load(`, `function M:push(`; **bash** — `name() {`, `function name {`, `function name() {`, calls by command, `$(…)` and after `|`/`&&`, in a `*_test.sh` or `test/*.bats`-style file. Write each `want.json` by hand first, as in Task 2.

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/scan/
```

Expected: PASS — all eight subtests of `TestScannerFixturesExact`.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/scan
git commit -m "feat(scan): Ruby, Lua and Bash fixtures exact; the 8-language corpus is complete (closes #422)"
```

---

### Task 4: Test classification by the generic `test_files` globs

**Issue:** #423

**Discharges:** PRD #409 AC7. Spec §6.

**Files:**
- Create: `internal/graph/classify.go`
- Test: `internal/graph/classify_test.go`

**Interfaces:**
- Produces: `graph.IsTestFile(rel string, cfg Config) bool`; `graph.Classify(nodes []Node, cfg Config)` — sets `IsTest` in place on every node, whichever source made it.
- Consumes: `graph.Config.TestFiles`, `graph.Config.TestExclude` (Task 1), `paths.MatchGlob`.

- [ ] **Step 1: Write the failing tests**

`internal/graph/classify_test.go`:

```go
package graph

import (
	"testing"
)

// PRD #409 AC7: every default test_files glob, alone, makes a func node in a matching
// file a test.
func TestEachDefaultTestFilesGlobMarksAFuncNodeATest(t *testing.T) {
	cases := map[string]string{
		"**/test_*":       "pkg/test_calc.py",
		"**/*_test.*":     "calc_test.go",
		"**/*.test.*":     "src/calc.test.ts",
		"**/*.spec.*":     "src/calc.spec.js",
		"**/*Test.*":      "app/CalcTest.java",
		"**/*Tests.*":     "app/CalcTests.cs",
		"**/tests/**":     "tests/helpers.py",
		"**/test/**":      "test/helpers.rb",
		"**/spec/**":      "spec/calc_helper.rb",
		"**/__tests__/**": "src/__tests__/calc.js",
	}
	defaults := DefaultConfig()
	if len(cases) != len(defaults.TestFiles) {
		t.Fatalf("%d cases for %d default globs", len(cases), len(defaults.TestFiles))
	}
	for _, glob := range defaults.TestFiles {
		file, ok := cases[glob]
		if !ok {
			t.Errorf("default glob %q has no case", glob)
			continue
		}
		cfg := defaults
		cfg.TestFiles = []string{glob}
		nodes := []Node{{ID: file + "::f", File: file, Name: "f", Kind: KindFunc}}
		Classify(nodes, cfg)
		if !nodes[0].IsTest {
			t.Errorf("%s alone does not make a func in %s a test", glob, file)
		}
	}
}

func TestClassesAndNonTestFilesAreNeverTests(t *testing.T) {
	nodes := []Node{
		{ID: "tests/test_a.py::TestA", File: "tests/test_a.py", Name: "TestA", Kind: KindClass},
		{ID: "tests/test_a.py::TestA::test_x", File: "tests/test_a.py", Name: "test_x", Kind: KindMethod},
		{ID: "src/a.test.js::adds", File: "src/a.test.js", Name: "adds", Kind: KindTest},
		{ID: "src/a.py::f", File: "src/a.py", Name: "f", Kind: KindFunc},
		{ID: "tests/testdata/b_test.py::f", File: "tests/testdata/b_test.py", Name: "f", Kind: KindFunc},
		{ID: "tests/fixtures/c.py::f", File: "tests/fixtures/c.py", Name: "f", Kind: KindFunc},
	}
	Classify(nodes, DefaultConfig())
	want := []bool{false, true, true, false, false, false}
	for i, n := range nodes {
		if n.IsTest != want[i] {
			t.Errorf("%s: IsTest = %v, want %v", n.ID, n.IsTest, want[i])
		}
	}
}

func TestTestFilesOverrideReplacesTheDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, "test_files: [\"checks/**\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	nodes := []Node{
		{ID: "checks/a.py::f", File: "checks/a.py", Name: "f", Kind: KindFunc},
		{ID: "tests/test_a.py::f", File: "tests/test_a.py", Name: "f", Kind: KindFunc},
	}
	Classify(nodes, cfg)
	if !nodes[0].IsTest || nodes[1].IsTest {
		t.Errorf("IsTest = %v, %v; want the override alone to decide (true, false)", nodes[0].IsTest, nodes[1].IsTest)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestEachDefaultTestFilesGlob|TestClassesAndNonTestFiles|TestTestFilesOverride' ./internal/graph/
```

Expected: FAIL — `undefined: Classify`.

- [ ] **Step 3: Implement**

`internal/graph/classify.go`:

```go
package graph

import "github.com/VocanicZ/rtdd/internal/paths"

// IsTestFile reports whether rel is a test file: it matches a TestFiles glob and no
// TestExclude glob (spec §6 — testdata/ and fixtures/ are never test files).
func IsTestFile(rel string, cfg Config) bool {
	return matchAny(cfg.TestFiles, rel) && !matchAny(cfg.TestExclude, rel)
}

// Classify sets IsTest on every node, whichever source produced it: a func, method or
// test node in a test file is a test; a class never is (spec §6).
func Classify(nodes []Node, cfg Config) {
	memo := map[string]bool{}
	for i := range nodes {
		n := &nodes[i]
		is, ok := memo[n.File]
		if !ok {
			is = IsTestFile(n.File, cfg)
			memo[n.File] = is
		}
		n.IsTest = is && n.Kind != KindClass
	}
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/graph/
```

Expected: PASS.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/graph
git commit -m "feat(graph): classify test nodes by the generic test_files globs (closes #423)"
```

---

### Task 5: The graphify loader keeps code nodes and drops non-code relations

**Issue:** #424

**Discharges:** PRD #409 AC4. Spec §3, §5.

**Files:**
- Create: `internal/graphify/load.go`
- Create: `internal/graphify/testdata/graphify-out/{graph.json,manifest.json,.graphify_root}`
- Test: `internal/graphify/load_test.go`

**Interfaces:**
- Produces: `graphify.DefaultPath = "graphify-out/graph.json"`; `graphify.ErrAbsent`; `graphify.Graph{Nodes []graph.Node; Edges []graph.Edge; BuiltAtCommit string; CodeFiles []string; Manifest []string}`; `graphify.Load(root, rel string) (*Graph, error)` — `rel` is `cfg.GraphifyPath`; `errors.Is(err, ErrAbsent)` when there is no graph.
- Consumes: `graph.ParseRelation`, `graph.Sort`.

The shape below is graphify's as it writes it (read from a real `graphify-out/` on 2026-10-07): networkx node-link JSON — `nodes` with `id`, `label`, `file_type` (`code|concept|rationale|document`), `source_file` (repo-relative), `source_location` (`L<n>`); `links` with `source`, `target`, `relation`; a top-level `built_at_commit` (full sha); and `manifest.json` beside it keyed by **absolute** path, with `.graphify_root` holding the root those paths are under.

- [ ] **Step 1: Write the failing tests**

`internal/graphify/testdata/graphify-out/graph.json`:

```json
{
  "directed": false,
  "multigraph": false,
  "graph": {},
  "nodes": [
    {"id": "src_calc", "label": "calc.py", "file_type": "code", "source_file": "src/calc.py", "source_location": "L1"},
    {"id": "src_calc_add", "label": "add()", "file_type": "code", "source_file": "src/calc.py", "source_location": "L3"},
    {"id": "src_calc_calc", "label": "Calc", "file_type": "code", "source_file": "src/calc.py", "source_location": "L7"},
    {"id": "src_calc_calc_plus", "label": ".plus()", "file_type": "code", "source_file": "src/calc.py", "source_location": "L8"},
    {"id": "src_base_base", "label": "Base", "file_type": "code", "source_file": "src/base.py", "source_location": "L1"},
    {"id": "src_base_proto", "label": "Proto", "file_type": "code", "source_file": "src/base.py", "source_location": "L5"},
    {"id": "tests_test_calc_test_add", "label": "test_add()", "file_type": "code", "source_file": "tests/test_calc.py", "source_location": "L4"},
    {"id": "external_thing", "label": "Thing", "file_type": "code", "source_file": "", "source_location": ""},
    {"id": "concept_design", "label": "Calculator design", "file_type": "concept", "source_file": "README.md"},
    {"id": "rationale_ints", "label": "Integers only", "file_type": "rationale", "source_file": "README.md"},
    {"id": "document_readme", "label": "README", "file_type": "document", "source_file": "README.md"}
  ],
  "links": [
    {"relation": "contains", "source": "src_calc", "target": "src_calc_add"},
    {"relation": "imports", "source": "src_calc", "target": "src_base_base"},
    {"relation": "method", "source": "src_calc_calc", "target": "src_calc_calc_plus"},
    {"relation": "calls", "source": "src_calc_calc_plus", "target": "src_calc_add"},
    {"relation": "calls", "source": "tests_test_calc_test_add", "target": "src_calc_add"},
    {"relation": "references", "source": "src_calc_calc_plus", "target": "src_calc_calc"},
    {"relation": "inherits", "source": "src_calc_calc", "target": "src_base_base"},
    {"relation": "implements", "source": "src_calc_calc", "target": "src_base_proto"},
    {"relation": "inherits", "source": "src_calc_calc", "target": "external_thing"},
    {"relation": "conceptually_related_to", "source": "concept_design", "target": "src_calc_calc"},
    {"relation": "semantically_similar_to", "source": "src_calc_add", "target": "src_base_base"},
    {"relation": "shares_data_with", "source": "src_calc_calc", "target": "src_base_proto"},
    {"relation": "cites", "source": "document_readme", "target": "src_calc_add"}
  ],
  "hyperedges": [],
  "built_at_commit": "fab6c1ad2d93c5cd157dec646024b8e2a8c080f0"
}
```

`internal/graphify/testdata/graphify-out/manifest.json`:

```json
{
  "/home/dev/proj/src/calc.py": {"mtime": 1780000000.0, "ast_hash": "a", "semantic_hash": "a"},
  "/home/dev/proj/src/base.py": {"mtime": 1780000000.0, "ast_hash": "b", "semantic_hash": "b"},
  "/home/dev/proj/tests/test_calc.py": {"mtime": 1780000000.0, "ast_hash": "c", "semantic_hash": "c"},
  "/home/dev/proj/README.md": {"mtime": 1780000000.0, "ast_hash": "d", "semantic_hash": "d"},
  "/somewhere/else/x.py": {"mtime": 1780000000.0, "ast_hash": "e", "semantic_hash": "e"}
}
```

`internal/graphify/testdata/graphify-out/.graphify_root` holds `/home/dev/proj` (no newline).

`internal/graphify/load_test.go`:

```go
package graphify

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// copyFixture puts testdata/graphify-out at rel/.. under a fresh root.
func copyFixture(t *testing.T, rel string) string {
	t.Helper()
	root := t.TempDir()
	dst := filepath.Join(root, filepath.Dir(filepath.FromSlash(rel)))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(dst, os.DirFS("testdata/graphify-out")); err != nil {
		t.Fatal(err)
	}
	return root
}

// PRD #409 AC4: only code nodes survive, and only the closed relation set's edges
// between surviving nodes.
func TestLoadKeepsCodeNodesAndDropsNonCodeRelations(t *testing.T) {
	root := copyFixture(t, DefaultPath)
	g, err := Load(root, DefaultPath)
	if err != nil {
		t.Fatal(err)
	}
	wantNodes := []graph.Node{
		{ID: "src/base.py::Base", File: "src/base.py", Name: "Base", Kind: graph.KindClass, Start: 1, End: 1},
		{ID: "src/base.py::Proto", File: "src/base.py", Name: "Proto", Kind: graph.KindClass, Start: 5, End: 5},
		{ID: "src/calc.py::add", File: "src/calc.py", Name: "add", Kind: graph.KindFunc, Start: 3, End: 3},
		{ID: "src/calc.py::Calc", File: "src/calc.py", Name: "Calc", Kind: graph.KindClass, Start: 7, End: 7},
		{ID: "src/calc.py::Calc::plus", File: "src/calc.py", Name: "plus", Kind: graph.KindMethod, Start: 8, End: 8},
		{ID: "tests/test_calc.py::test_add", File: "tests/test_calc.py", Name: "test_add", Kind: graph.KindFunc, Start: 4, End: 4},
	}
	if !reflect.DeepEqual(g.Nodes, wantNodes) {
		t.Errorf("nodes:\n got %+v\nwant %+v", g.Nodes, wantNodes)
	}
	wantEdges := []graph.Edge{
		{From: "src/calc.py::Calc", To: "src/base.py::Base", Relation: graph.RelInherits},
		{From: "src/calc.py::Calc", To: "src/base.py::Proto", Relation: graph.RelImplements},
		{From: "src/calc.py::Calc", To: "src/calc.py::Calc::plus", Relation: graph.RelMethod},
		{From: "src/calc.py::Calc::plus", To: "src/calc.py::Calc", Relation: graph.RelReferences},
		{From: "src/calc.py::Calc::plus", To: "src/calc.py::add", Relation: graph.RelCalls},
		{From: "tests/test_calc.py::test_add", To: "src/calc.py::add", Relation: graph.RelCalls},
	}
	if !reflect.DeepEqual(g.Edges, wantEdges) {
		t.Errorf("edges:\n got %+v\nwant %+v", g.Edges, wantEdges)
	}
	for _, e := range g.Edges {
		for _, dropped := range []graph.Relation{"contains", "conceptually_related_to", "semantically_similar_to", "shares_data_with", "cites"} {
			if e.Relation == dropped {
				t.Errorf("a %s edge survived the load: %+v", dropped, e)
			}
		}
	}
	if g.BuiltAtCommit != "fab6c1ad2d93c5cd157dec646024b8e2a8c080f0" {
		t.Errorf("BuiltAtCommit = %q", g.BuiltAtCommit)
	}
	if want := []string{"src/base.py", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(g.CodeFiles, want) {
		t.Errorf("CodeFiles = %v, want %v", g.CodeFiles, want)
	}
	if want := []string{"README.md", "src/base.py", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(g.Manifest, want) {
		t.Errorf("Manifest = %v, want %v (relative to .graphify_root, outside paths dropped)", g.Manifest, want)
	}
}

func TestLoadHonoursGraphifyPath(t *testing.T) {
	root := copyFixture(t, "tools/kg/graph.json")
	if _, err := Load(root, DefaultPath); !errors.Is(err, ErrAbsent) {
		t.Errorf("default path: err = %v, want ErrAbsent", err)
	}
	g, err := Load(root, "tools/kg/graph.json")
	if err != nil || len(g.Nodes) != 6 {
		t.Fatalf("Load(graphify_path) = %v nodes, %v", g, err)
	}
}

func TestLoadAbsentIsErrAbsentAndMalformedIsAnError(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root, DefaultPath); !errors.Is(err, ErrAbsent) {
		t.Errorf("absent: err = %v, want ErrAbsent", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "graphify-out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "graphify-out", "graph.json"), []byte("{\"nodes\": ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, DefaultPath); err == nil || errors.Is(err, ErrAbsent) {
		t.Errorf("malformed: err = %v, want an error that is not ErrAbsent", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/graphify/
```

Expected: FAIL — `no non-test Go files`, then `undefined: Load`, `undefined: DefaultPath`, `undefined: ErrAbsent`.

- [ ] **Step 3: Implement**

`internal/graphify/load.go`:

```go
// Package graphify reads a graph graphify already built. rtdd NEVER runs graphify (spec
// §5, §11): this package opens files and nothing else, and it trusts nothing it reads —
// the staleness overlay in internal/graphbuild decides which of it survives.
package graphify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// DefaultPath is where graphify writes its graph, relative to the repository root.
const DefaultPath = "graphify-out/graph.json"

// ErrAbsent is a missing graph: the project does not use graphify. Not a failure.
var ErrAbsent = errors.New("graphify: no graph")

// Graph is graphify's graph mapped into the rtdd model, plus what staleness needs.
type Graph struct {
	Nodes         []graph.Node // code nodes only; End == Start (graphify records start lines only)
	Edges         []graph.Edge // closed relation set only, both ends kept
	BuiltAtCommit string       // "" when graphify did not record one
	CodeFiles     []string     // sorted repo-relative files holding at least one code node
	Manifest      []string     // sorted repo-relative files in manifest.json; nil when it is absent
}

type nodeLink struct {
	Nodes []struct {
		ID             string `json:"id"`
		Label          string `json:"label"`
		FileType       string `json:"file_type"`
		SourceFile     string `json:"source_file"`
		SourceLocation string `json:"source_location"`
	} `json:"nodes"`
	Links []struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Relation string `json:"relation"`
	} `json:"links"`
	BuiltAtCommit string `json:"built_at_commit"`
}

// Load reads <root>/<rel> (graphify_path) and manifest.json beside it.
func Load(root, rel string) (*Graph, error) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w at %s", ErrAbsent, rel)
	}
	if err != nil {
		return nil, err
	}
	var nl nodeLink
	if err := json.Unmarshal(b, &nl); err != nil {
		return nil, fmt.Errorf("graphify: %s: %w", rel, err)
	}

	// A method edge names its target's class; the class is part of the target's ID.
	parent := map[string]string{}
	label := map[string]string{}
	for _, n := range nl.Nodes {
		label[n.ID] = n.Label
	}
	for _, l := range nl.Links {
		if l.Relation == string(graph.RelMethod) {
			parent[l.Target] = nameOf(label[l.Source])
		}
	}

	g := &Graph{BuiltAtCommit: nl.BuiltAtCommit}
	ids := map[string]string{} // graphify id -> rtdd id
	files := map[string]bool{}
	seen := map[string]bool{}
	for _, n := range nl.Nodes {
		if n.FileType != "code" || n.SourceFile == "" {
			continue
		}
		file := filepath.ToSlash(n.SourceFile)
		files[file] = true
		if n.Label == path.Base(file) {
			continue // the file node: everything it would say, `contains` said, and that is dropped
		}
		name := nameOf(n.Label)
		kind := graph.KindClass
		if strings.HasSuffix(n.Label, ")") {
			kind = graph.KindFunc
			if _, ok := parent[n.ID]; ok {
				kind = graph.KindMethod
			}
		}
		start, _ := strconv.Atoi(strings.TrimPrefix(n.SourceLocation, "L"))
		id := file + "::" + name
		if c, ok := parent[n.ID]; ok {
			id = file + "::" + c + "::" + name
		}
		if seen[id] {
			id += "@" + strconv.Itoa(start)
		}
		seen[id] = true
		ids[n.ID] = id
		g.Nodes = append(g.Nodes, graph.Node{ID: id, File: file, Name: name, Kind: kind, Start: start, End: start})
	}
	for _, l := range nl.Links {
		rel, ok := graph.ParseRelation(l.Relation)
		from, okFrom := ids[l.Source]
		to, okTo := ids[l.Target]
		if ok && okFrom && okTo {
			g.Edges = append(g.Edges, graph.Edge{From: from, To: to, Relation: rel})
		}
	}
	whole := graph.Graph{Nodes: g.Nodes, Edges: g.Edges}
	graph.Sort(&whole)
	g.Nodes, g.Edges = whole.Nodes, whole.Edges
	for f := range files {
		g.CodeFiles = append(g.CodeFiles, f)
	}
	sort.Strings(g.CodeFiles)

	g.Manifest, err = readManifest(root, filepath.Dir(p))
	if err != nil {
		return nil, err
	}
	return g, nil
}

// nameOf turns a graphify label into a name: `.plus()` -> plus, `add()` -> add.
func nameOf(label string) string {
	return strings.TrimSuffix(strings.TrimPrefix(label, "."), "()")
}

// readManifest returns manifest.json's files relative to the root graphify recorded in
// .graphify_root (the repository root when that file is absent); entries outside it are
// dropped. A missing manifest is nil, not an error.
func readManifest(root, dir string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("graphify: manifest.json: %w", err)
	}
	base := root
	if r, err := os.ReadFile(filepath.Join(dir, ".graphify_root")); err == nil {
		base = strings.TrimSpace(string(r))
	}
	out := []string{}
	for k := range m {
		rel := k
		if filepath.IsAbs(k) {
			r, err := filepath.Rel(base, k)
			if err != nil || r == ".." || strings.HasPrefix(r, "../") {
				continue
			}
			rel = r
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out, nil
}
```

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/graphify/
grep -rn 'exec\.' internal/graphify/ || echo "no exec in internal/graphify"
```

Expected: PASS, and `no exec in internal/graphify`.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/graphify
git commit -m "feat(graphify): load graphify's code nodes, drop non-code relations (closes #424)"
```

---

### Task 6: `rtdd graph [--json]` on the scanner path

**Issue:** #425

**Discharges:** PRD #409 AC9 (`source: scanner`). Spec §8 (`rtdd graph` row only).

**Files:**
- Create: `internal/graphbuild/build.go`, `cmd/rtdd/graph.go`
- Modify: `cmd/rtdd/main.go` (the `graph` case and the usage line `  rtdd graph  [--json]`)
- Test: `internal/graphbuild/build_test.go`, `cmd/rtdd/graph_test.go`

**Interfaces:**
- Produces: `graphbuild.Build(root string, cfg graph.Config, opt Options) (*Result, error)`; `graphbuild.Options{CachePath string}`; `graphbuild.Result{Graph graph.Graph; Source, BuiltAtCommit string; StaleFiles []string; GraphifyIgnored, GraphifyCommit string; GraphifyFiles int; Scanned []string}`; constants `SourceScanner`, `SourceGraphifyScanner`, `IgnoredNoCommit`, `IgnoredUnknownCommit`, `IgnoredTooStale`. **Every field and constant is declared here** even though Tasks 7–9 fill them, so 7 and 8 never conflict over the type.
- Produces: `rtdd graph [--json]` — exit 0 success, 2 usage or bad config, 3 not a git repository or an unreadable graph.
- Consumes: `gitctx.ListFiles`, `gitctx.HeadSHA`, `gitctx.RepoRoot`, `scan.Filter`, `scan.ScanFiles`, `scan.Assemble`, `graph.LoadConfig`, `graph.Classify`; the existing `rtdd(t, dir, args...)` test helper in `cmd/rtdd/main_test.go`.

- [ ] **Step 1: Write the failing tests**

`internal/graphbuild/build_test.go` — including the `repo`, `build` and `calcProject` helpers every later graphbuild test uses:

```go
package graphbuild

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// repo commits files into a fresh git repository and returns its root.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := gittest.Init(t)
	for rel, body := range files {
		gittest.Write(t, dir, rel, body)
	}
	gittest.Commit(t, dir, "init")
	return dir
}

// build is Build with the CLI's defaults, failing the test on error.
func build(t *testing.T, root string) *Result {
	t.Helper()
	res, err := Build(root, graph.DefaultConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

var calcProject = map[string]string{
	"src/calc.py":        "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n",
	"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
	"README.md":          "# calc\n",
}

// PRD #409 AC9, scanner half: with no graphify graph the scanner builds everything.
func TestBuildWithoutGraphifyIsTheScannersGraph(t *testing.T) {
	root := repo(t, calcProject)
	res, err := Build(root, graph.DefaultConfig(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceScanner || res.BuiltAtCommit != gittest.HeadShort(t, root) || len(res.StaleFiles) != 0 || res.GraphifyIgnored != "" {
		t.Errorf("Result = {Source:%q BuiltAtCommit:%q StaleFiles:%v GraphifyIgnored:%q}, want scanner at HEAD, nothing stale",
			res.Source, res.BuiltAtCommit, res.StaleFiles, res.GraphifyIgnored)
	}
	wantNodes := []graph.Node{
		{ID: "src/calc.py::add", File: "src/calc.py", Name: "add", Kind: graph.KindFunc, Start: 1, End: 2},
		{ID: "src/calc.py::total", File: "src/calc.py", Name: "total", Kind: graph.KindFunc, Start: 5, End: 6},
		{ID: "tests/test_calc.py::test_add", File: "tests/test_calc.py", Name: "test_add", Kind: graph.KindFunc, Start: 4, End: 5, IsTest: true},
	}
	if !reflect.DeepEqual(res.Graph.Nodes, wantNodes) {
		t.Errorf("nodes:\n got %+v\nwant %+v", res.Graph.Nodes, wantNodes)
	}
	wantEdges := []graph.Edge{
		{From: "src/calc.py::total", To: "src/calc.py::add", Relation: graph.RelCalls},
		{From: "tests/test_calc.py::test_add", To: "src/calc.py::add", Relation: graph.RelCalls},
	}
	if !reflect.DeepEqual(res.Graph.Edges, wantEdges) {
		t.Errorf("edges:\n got %+v\nwant %+v", res.Graph.Edges, wantEdges)
	}
}
```

`cmd/rtdd/graph_test.go`:

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// graphRepo is a two-file project: two functions, one test, two calls edges.
func graphRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n")
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// PRD #409 AC9: `rtdd graph` prints source, counts, built_at_commit and stale_files.
func TestGraphCommandPrintsSourceCountsAndStaleness(t *testing.T) {
	dir := graphRepo(t)
	code, out, errOut := rtdd(t, dir, "graph")
	if code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	want := "source:          scanner\n" +
		"nodes:           3\n" +
		"edges:           2\n" +
		"tests:           1\n" +
		"built_at_commit: " + gittest.HeadShort(t, dir) + "\n" +
		"stale_files:     0\n"
	if out != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out, want)
	}
}

func TestGraphCommandJSONShape(t *testing.T) {
	dir := graphRepo(t)
	code, out, errOut := rtdd(t, dir, "graph", "--json")
	if code != 0 {
		t.Fatalf("rtdd graph --json = %d, stderr %q", code, errOut)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if doc["schema"] != 3.0 || doc["command"] != "graph" {
		t.Errorf("envelope = schema %v, command %v; want 3, graph", doc["schema"], doc["command"])
	}
	g, _ := doc["graph"].(map[string]any)
	want := map[string]any{"source": "scanner", "built_at_commit": gittest.HeadShort(t, dir),
		"stale_files": 0.0, "nodes": 3.0, "edges": 2.0, "tests": 1.0}
	for k, v := range want {
		if g[k] != v {
			t.Errorf("graph.%s = %v, want %v", k, g[k], v)
		}
	}
	if len(g) != len(want) {
		t.Errorf("graph has keys %v, want exactly %v (graphify_ignored only when graphify was ignored)", g, want)
	}
}

func TestGraphCommandExitCodes(t *testing.T) {
	if code, _, _ := rtdd(t, t.TempDir(), "graph"); code != 3 {
		t.Errorf("outside a git repository: exit %d, want 3", code)
	}
	dir := graphRepo(t)
	if code, _, _ := rtdd(t, dir, "graph", "--bogus"); code != 2 {
		t.Errorf("unknown flag: exit %d, want 2", code)
	}
	if _, out, _ := rtdd(t, dir, "--help"); !strings.Contains(out, "rtdd graph") {
		t.Errorf("--help does not list `rtdd graph`:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./internal/graphbuild/
go test -run 'TestGraphCommand' ./cmd/rtdd/
```

Expected: FAIL — `no non-test Go files` in `internal/graphbuild`; in `cmd/rtdd`, `rtdd graph` exits 2 with `unknown command "graph"`.

- [ ] **Step 3: Implement**

`internal/graphbuild/build.go` (scanner path; Tasks 7–9 add the cache and graphify — the final shape is in Task 9):

```go
// Package graphbuild assembles the node graph a command selects from: the scanner's
// graph, cached in .rtdd/graph.json, overlaid on graphify's when the project has one and
// it can be trusted (spec §4.5, §5). It is the only graph package that talks to git, and
// it does so only through internal/gitctx.
package graphbuild

import (
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// Sources of a built graph (spec §5, `graph.source`).
const (
	SourceScanner         = "scanner"
	SourceGraphifyScanner = "graphify+scanner"
)

// Why graphify was ignored entirely (spec §5 step 4). The JSON carries the code; the
// human output explains it.
const (
	IgnoredNoCommit      = "built_at_commit_missing"
	IgnoredUnknownCommit = "built_at_commit_unknown"
	IgnoredTooStale      = "too_stale"
)

// Options tune a build. The zero value is the CLI's behaviour.
type Options struct {
	CachePath string // "" is <root>/.rtdd/graph.json
}

// Result is a built graph and how it was built. Every field is declared here, in Task 6,
// so Tasks 7, 8 and 9 — which may land in either order — only fill them.
type Result struct {
	Graph           graph.Graph // IsTest set on every node
	Source          string      // SourceScanner | SourceGraphifyScanner
	BuiltAtCommit   string      // graphify's when it is used, else HEAD's short sha ("" on an unborn HEAD)
	StaleFiles      []string    // files graphify was not trusted for, sorted; empty for SourceScanner
	GraphifyIgnored string      // an Ignored* code, or "" when graphify was used or absent
	GraphifyCommit  string      // graphify's built_at_commit as it recorded it, when it was read
	GraphifyFiles   int         // graphify's code-file count, when it was read
	Scanned         []string    // files the scanner read on this call, sorted
}

// Build builds the graph for the repository at root.
func Build(root string, cfg graph.Config, opt Options) (*Result, error) {
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		return nil, err
	}
	files := scan.Filter(root, listed, cfg.ScanExclude)
	head, _ := gitctx.HeadSHA(root) // "" on an unborn HEAD

	res := &Result{Source: SourceScanner, BuiltAtCommit: head, Scanned: files}
	res.Graph = scan.Assemble(scan.ScanFiles(root, files))
	graph.Classify(res.Graph.Nodes, cfg)
	return res, nil
}
```

`cmd/rtdd/graph.go`:

```go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
)

// graphJSON is `rtdd graph --json`: the spec §9 envelope (schema 3) with only the
// `graph` object, which is the same object `rtdd which` will carry once N2 lands.
type graphJSON struct {
	Schema  int         `json:"schema"`
	Command string      `json:"command"`
	Graph   graphObject `json:"graph"`
}

type graphObject struct {
	Source          string `json:"source"`
	BuiltAtCommit   string `json:"built_at_commit"`
	StaleFiles      int    `json:"stale_files"`
	GraphifyIgnored string `json:"graphify_ignored,omitempty"`
	Nodes           int    `json:"nodes"`
	Edges           int    `json:"edges"`
	Tests           int    `json:"tests"`
}

// cmdGraph builds or refreshes the node graph and reports what it holds (spec §8). It
// runs no test and changes nothing but .rtdd/graph.json.
func cmdGraph(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON (schema 3)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd graph [--json]")
		return 2
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 3
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: not inside a git work tree, or git is unavailable: %v\n", err)
		return 3
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 2
	}
	res, err := graphbuild.Build(root, cfg, graphbuild.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "rtdd graph: %v\n", err)
		return 3
	}

	obj := graphObject{Source: res.Source, BuiltAtCommit: res.BuiltAtCommit, StaleFiles: len(res.StaleFiles),
		GraphifyIgnored: res.GraphifyIgnored, Nodes: len(res.Graph.Nodes), Edges: len(res.Graph.Edges)}
	for _, n := range res.Graph.Nodes {
		if n.IsTest {
			obj.Tests++
		}
	}
	if *asJSON {
		b, _ := json.MarshalIndent(graphJSON{Schema: 3, Command: "graph", Graph: obj}, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return 0
	}
	fmt.Fprintf(stdout, "source:          %s\n", obj.Source)
	fmt.Fprintf(stdout, "nodes:           %d\n", obj.Nodes)
	fmt.Fprintf(stdout, "edges:           %d\n", obj.Edges)
	fmt.Fprintf(stdout, "tests:           %d\n", obj.Tests)
	fmt.Fprintf(stdout, "built_at_commit: %s\n", obj.BuiltAtCommit)
	fmt.Fprintf(stdout, "stale_files:     %d\n", obj.StaleFiles)
	if res.GraphifyIgnored != "" {
		fmt.Fprintf(stdout, "graphify:        ignored — %s; run `graphify --update` to use it again\n", ignoredWhy(res, cfg))
	}
	return 0
}

// ignoredWhy is the human sentence for a graphbuild.Ignored* code.
func ignoredWhy(res *graphbuild.Result, cfg graph.Config) string {
	switch res.GraphifyIgnored {
	case graphbuild.IgnoredNoCommit:
		return "its graph records no built_at_commit"
	case graphbuild.IgnoredUnknownCommit:
		return fmt.Sprintf("its built_at_commit %s is unknown to git", res.GraphifyCommit)
	case graphbuild.IgnoredTooStale:
		return fmt.Sprintf("%d of its %d code files are stale, more than max_stale_ratio %.2f",
			len(res.StaleFiles), res.GraphifyFiles, cfg.MaxStaleRatio)
	}
	return res.GraphifyIgnored
}
```

In `cmd/rtdd/main.go`, add `  rtdd graph  [--json]` to `usage` after the `explain` line, and:

```go
	case "graph":
		return cmdGraph(args[1:], stdout, stderr)
```

- [ ] **Step 4: Run to verify they pass, and that nothing else moved**

```bash
go test ./internal/graphbuild/ ./cmd/rtdd/
```

Expected: PASS — every existing `cmd/rtdd` test unchanged (`TestPipelineGradle` aside, which fails on a machine with a local `gradle` before and after this plan; hosted CI does not install gradle).

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/graphbuild cmd/rtdd
git commit -m "feat: rtdd graph builds the scanner graph and prints source, counts and staleness (closes #425)"
```

---

### Task 7: `.rtdd/graph.json` caches the scanner graph

**Issue:** #426

**Discharges:** PRD #409 AC8. Spec §4.5, §12.

**Files:**
- Create: `internal/gitctx/graphgit.go` (`BlobIDs`), `internal/graphbuild/cache.go`
- Modify: `internal/scan/patterns.go` (`spanRules`, `Fingerprint`), `internal/graphbuild/build.go`, `.gitignore` (`/.rtdd/graph.json`)
- Test: `internal/gitctx/graphgit_test.go`, `internal/graphbuild/cache_test.go`

**Interfaces:**
- Produces: `gitctx.BlobIDs(repoRoot string) (map[string]string, error)`; `scan.Fingerprint() string`; `Result.Scanned` now lists only re-read files.
- Consumes: `gitctx.ChangedSet(root, "HEAD")` (the working-tree changed set, untracked included); `scan.FileResult.Calls`.

- [ ] **Step 1: Write the failing tests**

`internal/gitctx/graphgit_test.go` (Tasks 8 and 9 append to it):

```go
package gitctx_test

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestBlobIDsChangeOnlyWhenACommitChangesTheFile(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "a.py", "def a():\n    pass\n")
	gittest.Write(t, dir, "lib/b.py", "def b():\n    pass\n")
	gittest.Commit(t, dir, "one")
	first, err := gitctx.BlobIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(first["a.py"]) != 40 || len(first["lib/b.py"]) != 40 {
		t.Fatalf("BlobIDs = %v, want 40-hex ids for a.py and lib/b.py", first)
	}
	gittest.Write(t, dir, "a.py", "def a():\n    return 1\n")
	if again, _ := gitctx.BlobIDs(dir); !reflect.DeepEqual(again, first) {
		t.Errorf("an uncommitted edit changed BlobIDs: %v -> %v (HEAD's tree is the source)", first, again)
	}
	gittest.Commit(t, dir, "two")
	second, _ := gitctx.BlobIDs(dir)
	if second["a.py"] == first["a.py"] || second["lib/b.py"] != first["lib/b.py"] {
		t.Errorf("after committing a.py: %v -> %v; want only a.py's id to change", first, second)
	}
}

func TestBlobIDsOnAnUnbornHeadIsEmpty(t *testing.T) {
	dir := gittest.Init(t)
	got, err := gitctx.BlobIDs(dir)
	if err != nil || len(got) != 0 {
		t.Errorf("BlobIDs(unborn) = %v, %v; want empty, nil", got, err)
	}
}
```

`internal/graphbuild/cache_test.go`:

```go
package graphbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// PRD #409 AC8: a file is re-scanned only when its blob id changed or it is in the
// working-tree changed set.
func TestCacheRescansOnlyChangedBlobsAndTheWorkingTreeChangedSet(t *testing.T) {
	root := repo(t, calcProject)
	if got, want := build(t, root).Scanned, []string{"README.md", "src/calc.py", "tests/test_calc.py"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cold build scanned %v, want %v", got, want)
	}
	if got := build(t, root).Scanned; len(got) != 0 {
		t.Errorf("warm build with no change scanned %v, want nothing", got)
	}
	gittest.Write(t, root, "src/calc.py", calcProject["src/calc.py"]+"\n\ndef sub(a, b):\n    return a - b\n")
	if got, want := build(t, root).Scanned, []string{"src/calc.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after an uncommitted edit scanned %v, want %v", got, want)
	}
	gittest.Commit(t, root, "sub")
	build(t, root) // the cache now holds src/calc.py at its new blob
	if got := build(t, root).Scanned; len(got) != 0 {
		t.Errorf("after committing and rebuilding scanned %v, want nothing", got)
	}
}

func TestCacheRescansACommittedChange(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	gittest.Write(t, root, "tests/test_calc.py", calcProject["tests/test_calc.py"]+"\n\ndef test_more():\n    assert add(2, 2) == 4\n")
	gittest.Commit(t, root, "more") // committed: not in the changed set, but a new blob
	if got, want := build(t, root).Scanned, []string{"tests/test_calc.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("after a committed change scanned %v, want %v", got, want)
	}
}

// The cache is graphify's graph.json shape plus built_at_commit and per-file blob ids.
func TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	b, err := os.ReadFile(filepath.Join(root, ".rtdd", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Directed      *bool             `json:"directed"`
		Nodes         []map[string]any  `json:"nodes"`
		Links         []map[string]any  `json:"links"`
		BuiltAtCommit string            `json:"built_at_commit"`
		Files         map[string]string `json:"rtdd_files"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Directed == nil || len(doc.Nodes) != 3 || len(doc.Links) != 2 {
		t.Errorf("cache has directed=%v, %d nodes, %d links; want a node-link graph of 3 and 2", doc.Directed, len(doc.Nodes), len(doc.Links))
	}
	for _, n := range doc.Nodes {
		for _, k := range []string{"id", "label", "file_type", "source_file", "source_location"} {
			if _, ok := n[k]; !ok {
				t.Errorf("cache node %v has no graphify key %q", n["id"], k)
			}
		}
	}
	if doc.BuiltAtCommit != gittest.HeadShort(t, root) {
		t.Errorf("built_at_commit = %q, want HEAD", doc.BuiltAtCommit)
	}
	blobs, err := gitctx.BlobIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Files["src/calc.py"] != blobs["src/calc.py"] || doc.Files["src/calc.py"] == "" {
		t.Errorf("rtdd_files[src/calc.py] = %q, want HEAD's blob %q", doc.Files["src/calc.py"], blobs["src/calc.py"])
	}
}

func TestCorruptOrForeignCacheIsRebuiltNotAnError(t *testing.T) {
	for name, body := range map[string]string{
		"corrupt":       "{\"nodes\": [",
		"other version": "{\"rtdd_cache\": 99, \"nodes\": []}",
	} {
		t.Run(name, func(t *testing.T) {
			root := repo(t, calcProject)
			build(t, root)
			if err := os.WriteFile(filepath.Join(root, ".rtdd", "graph.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			res := build(t, root)
			if len(res.Scanned) != 3 || len(res.Graph.Nodes) != 3 {
				t.Errorf("scanned %v into %d nodes, want a full rebuild of 3 files / 3 nodes", res.Scanned, len(res.Graph.Nodes))
			}
		})
	}
}

// A cached file's call into a re-scanned file is re-linked, never left dangling.
func TestCachedCallerOfARenamedFunctionIsNotLeftDangling(t *testing.T) {
	root := repo(t, calcProject)
	build(t, root)
	gittest.Write(t, root, "src/calc.py", "def plus(a, b):\n    return a + b\n")
	res := build(t, root)
	ids := map[string]bool{}
	for _, n := range res.Graph.Nodes {
		ids[n.ID] = true
	}
	for _, e := range res.Graph.Edges {
		if !ids[e.From] || !ids[e.To] {
			t.Errorf("dangling edge %+v", e)
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestBlobIDs' ./internal/gitctx/
go test -run 'TestCache|TestCorrupt|TestCachedCaller' ./internal/graphbuild/
```

Expected: FAIL — `undefined: gitctx.BlobIDs`; then `warm build with no change scanned [README.md src/calc.py tests/test_calc.py], want nothing` and `open …/.rtdd/graph.json: no such file or directory`.

- [ ] **Step 3: Implement**

`internal/gitctx/graphgit.go`:

```go
package gitctx

import "strings"

// BlobIDs maps every file in HEAD's tree to its git blob id. A file whose id is unchanged
// since the graph cache recorded it need not be re-scanned (spec §4.5). An unborn HEAD —
// no commits yet — is an empty map, not an error: every file is then in the changed set.
func BlobIDs(repoRoot string) (map[string]string, error) {
	out := map[string]string{}
	if _, err := git(repoRoot, "rev-parse", "--verify", "-q", "HEAD^{commit}"); err != nil {
		return out, nil
	}
	ls, err := git(repoRoot, "ls-tree", "-r", "-z", "--full-tree", "HEAD")
	if err != nil {
		return nil, err
	}
	for _, rec := range strings.Split(ls, "\x00") {
		// <mode> SP <type> SP <object> TAB <path>
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) == 3 && f[1] == "blob" {
			out[p] = f[2]
		}
	}
	return out, nil
}
```

Append to `internal/scan/patterns.go` (and import `crypto/sha256`, `encoding/hex`, `fmt`, `sort`):

```go
// spanRules versions the language-agnostic rules in scan.go. Bump it with any change to
// how spans, ownership or edges are computed, so every cached graph is rebuilt.
const spanRules = "1"

// Fingerprint identifies this scanner: the pattern table, the keyword set and the span
// rules. The graph cache is valid only for the fingerprint that wrote it.
func Fingerprint() string {
	h := sha256.New()
	for _, p := range patterns {
		fmt.Fprintf(h, "%d %s\n", p.shape, p.re)
	}
	keys := make([]string, 0, len(controlKeywords))
	for k := range controlKeywords {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(h, "%s\n%s\n%s\n", methodTail, strings.Join(keys, " "), spanRules)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
```

`internal/graphbuild/cache.go`:

```go
package graphbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// cacheVersion is the rtdd_cache value this build reads and writes. Any other value —
// and any other scanner fingerprint — is a cache miss, never an error.
const cacheVersion = 1

// cacheFile is .rtdd/graph.json: graphify's graph.json node-link shape (so a graphify
// reader can open it), plus rtdd's own keys, all prefixed rtdd_ (spec §4.5).
type cacheFile struct {
	Directed      bool              `json:"directed"`
	Multigraph    bool              `json:"multigraph"`
	Graph         struct{}          `json:"graph"`
	Nodes         []cacheNode       `json:"nodes"`
	Links         []cacheLink       `json:"links"`
	BuiltAtCommit string            `json:"built_at_commit"`
	Cache         int               `json:"rtdd_cache"`
	Scanner       string            `json:"rtdd_scanner"`
	Files         map[string]string `json:"rtdd_files"` // path -> blob id it was scanned from; "" = working-tree content
}

type cacheNode struct {
	ID             string     `json:"id"`
	Label          string     `json:"label"`
	FileType       string     `json:"file_type"`
	SourceFile     string     `json:"source_file"`
	SourceLocation string     `json:"source_location"`
	Name           string     `json:"rtdd_name"`
	Kind           graph.Kind `json:"rtdd_kind"`
	Start          int        `json:"rtdd_start"`
	End            int        `json:"rtdd_end"`
	Calls          []string   `json:"rtdd_calls,omitempty"`
}

type cacheLink struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Relation graph.Relation `json:"relation"`
}

// cache is a read cache: per file, the blob it was scanned from and the scan.
type cache struct {
	blob    map[string]string
	results map[string]scan.FileResult
}

// readCache never fails: a missing, corrupt, other-version or other-scanner cache is empty.
func readCache(p string) *cache {
	c := &cache{blob: map[string]string{}, results: map[string]scan.FileResult{}}
	b, err := os.ReadFile(p)
	if err != nil {
		return c
	}
	var f cacheFile
	if json.Unmarshal(b, &f) != nil || f.Cache != cacheVersion || f.Scanner != scan.Fingerprint() {
		return c
	}
	for _, n := range f.Nodes {
		r := c.results[n.SourceFile]
		if r.Calls == nil {
			r = scan.FileResult{Path: n.SourceFile, Calls: map[string][]string{}}
		}
		r.Nodes = append(r.Nodes, graph.Node{ID: n.ID, File: n.SourceFile, Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End})
		if len(n.Calls) > 0 {
			r.Calls[n.ID] = n.Calls
		}
		c.results[n.SourceFile] = r
	}
	file := map[string]string{}
	for _, n := range f.Nodes {
		file[n.ID] = n.SourceFile
	}
	for _, l := range f.Links {
		if l.Relation == graph.RelMethod {
			r := c.results[file[l.Source]]
			r.Edges = append(r.Edges, graph.Edge{From: l.Source, To: l.Target, Relation: l.Relation})
			c.results[file[l.Source]] = r
		}
	}
	for p, blob := range f.Files {
		c.blob[p] = blob
		if _, ok := c.results[p]; !ok {
			c.results[p] = scan.FileResult{Path: p, Calls: map[string][]string{}} // a file with no nodes
		}
	}
	return c
}

// scanCached returns a scan of every file in toScan, re-reading only a file whose HEAD
// blob differs from the one cached, or that is in the working-tree changed set.
func scanCached(root string, toScan []string, blobs map[string]string, changed map[string]bool, c *cache) ([]scan.FileResult, []string) {
	var out []scan.FileResult
	var fresh []string
	for _, f := range toScan {
		if !changed[f] && blobs[f] != "" && c.blob[f] == blobs[f] {
			out = append(out, c.results[f])
			continue
		}
		fresh = append(fresh, f)
	}
	out = append(out, scan.ScanFiles(root, fresh)...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, fresh
}

// writeCache records results, plus every older entry still valid for a listed file.
func writeCache(p, head string, files []string, blobs map[string]string, changed map[string]bool, old *cache, results []scan.FileResult) error {
	keep := map[string]scan.FileResult{}
	for _, f := range files {
		if r, ok := old.results[f]; ok && !changed[f] && blobs[f] != "" && old.blob[f] == blobs[f] {
			keep[f] = r
		}
	}
	for _, r := range results {
		keep[r.Path] = r
	}
	f := cacheFile{Graph: struct{}{}, BuiltAtCommit: head, Cache: cacheVersion, Scanner: scan.Fingerprint(), Files: map[string]string{}}
	var all []scan.FileResult
	for path, r := range keep {
		blob := blobs[path]
		if changed[path] {
			blob = ""
		}
		f.Files[path] = blob
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	g := scan.Assemble(all)
	calls := map[string][]string{}
	for _, r := range all {
		for id, names := range r.Calls {
			calls[id] = names
		}
	}
	for _, n := range g.Nodes {
		f.Nodes = append(f.Nodes, cacheNode{ID: n.ID, Label: label(n), FileType: "code", SourceFile: n.File,
			SourceLocation: "L" + strconv.Itoa(n.Start), Name: n.Name, Kind: n.Kind, Start: n.Start, End: n.End, Calls: calls[n.ID]})
	}
	for _, e := range g.Edges {
		f.Links = append(f.Links, cacheLink{Source: e.From, Target: e.To, Relation: e.Relation})
	}
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// label is graphify's label convention: `add()`, `.plus()`, `Calc`.
func label(n graph.Node) string {
	switch n.Kind {
	case graph.KindFunc:
		return n.Name + "()"
	case graph.KindMethod:
		return "." + n.Name + "()"
	}
	return n.Name
}

// covers reports whether the cache already holds exactly files — so a call that re-scanned
// nothing need not rewrite it.
func (c *cache) covers(files []string) bool {
	if len(c.blob) != len(files) {
		return false
	}
	for _, f := range files {
		if _, ok := c.blob[f]; !ok {
			return false
		}
	}
	return true
}
```

In `Build`, replace the single scan with the cached one — read the changed set and blob ids, scan through the cache, and write it only when something was re-read or the file set moved:

```go
	changes, err := gitctx.ChangedSet(root, "HEAD")
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, c := range changes {
		changed[c.Path] = true
	}
	blobs, err := gitctx.BlobIDs(root)
	if err != nil {
		return nil, err
	}

	c := readCache(opt.CachePath)
	results, scanned := scanCached(root, toScan, blobs, changed, c)
	res.Scanned = scanned
	if len(scanned) > 0 || !c.covers(toScan) {
		if err := writeCache(opt.CachePath, head, files, blobs, changed, c, results); err != nil {
			return nil, err
		}
	}
```

with `opt.CachePath` defaulting to `filepath.Join(root, ".rtdd", "graph.json")` and `toScan := files` (Task 8 narrows it). Add `/.rtdd/graph.json` to `.gitignore`.

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/gitctx/ ./internal/graphbuild/
go test -count=1 -run 'TestOnlyGitctxShellsOutToGit' ./internal/contract/
```

Expected: PASS.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/gitctx internal/scan internal/graphbuild .gitignore
git commit -m "feat(graphbuild): .rtdd/graph.json caches the scanner graph by blob id (closes #426)"
```

---

### Task 8: The graphify staleness overlay

**Issue:** #427

**Discharges:** PRD #409 AC5, AC9 (`source: graphify+scanner`). Spec §5 steps 1–3.

**Files:**
- Create: `internal/graphbuild/stale.go` (`StaleSet`, a trigger-less `staleness`), `internal/graphbuild/overlay.go`
- Modify: `internal/gitctx/graphgit.go` (`DiffNamesSince`), `internal/graphbuild/build.go` (load graphify, narrow the scan to stale files, overlay)
- Test: `internal/gitctx/graphgit_test.go`, `internal/graphbuild/overlay_test.go`, `cmd/rtdd/graph_test.go`

**Interfaces:**
- Produces: `gitctx.DiffNamesSince(repoRoot, commit string) ([]string, error)`; `graphbuild.StaleSet(root string, gf *graphify.Graph, files []string, changed map[string]bool) ([]string, error)`; `graphbuild.Overlay(gf *graphify.Graph, stale map[string]bool, scanned []scan.FileResult) graph.Graph`; test helpers `gfNode`, `writeGraphify`, `fullHead` (Task 9 uses them).
- Consumes: `graphify.Load` (Task 5), `scan.Link`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/gitctx/graphgit_test.go`:

```go
func TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted(t *testing.T) {
	dir := gittest.Init(t)
	for _, f := range []string{"committed.py", "staged.py", "unstaged.py", "deleted.py", "same.py"} {
		gittest.Write(t, dir, f, "x = 1\n")
	}
	base := gittest.Commit(t, dir, "base")
	gittest.Write(t, dir, "committed.py", "x = 2\n")
	gittest.Commit(t, dir, "later")
	gittest.Write(t, dir, "staged.py", "x = 2\n")
	gittest.Run(t, dir, "add", "staged.py")
	gittest.Write(t, dir, "unstaged.py", "x = 2\n")
	gittest.Run(t, dir, "rm", "-q", "deleted.py")
	gittest.Write(t, dir, "untracked.py", "x = 1\n")

	got, err := gitctx.DiffNamesSince(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"committed.py", "deleted.py", "staged.py", "unstaged.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DiffNamesSince = %v, want %v", got, want)
	}
}
```

`internal/graphbuild/overlay_test.go`:

```go
package graphbuild

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
)

// gfNode is one graphify code node: a function `name()` at line in file.
type gfNode struct {
	id, label, file string
	line            int
}

// writeGraphify writes graphify-out/graph.json and manifest.json (keys absolute under
// root, as graphify writes them) the way graphify would have at builtAt. It is untracked
// and under scan_exclude, as a real graphify-out/ is.
func writeGraphify(t *testing.T, root, builtAt string, nodes []gfNode, calls [][2]string, manifest []string) {
	t.Helper()
	type n struct {
		ID             string `json:"id"`
		Label          string `json:"label"`
		FileType       string `json:"file_type"`
		SourceFile     string `json:"source_file"`
		SourceLocation string `json:"source_location"`
	}
	type l struct {
		Source   string `json:"source"`
		Target   string `json:"target"`
		Relation string `json:"relation"`
	}
	doc := struct {
		Nodes         []n    `json:"nodes"`
		Links         []l    `json:"links"`
		BuiltAtCommit string `json:"built_at_commit,omitempty"`
	}{BuiltAtCommit: builtAt}
	for _, x := range nodes {
		doc.Nodes = append(doc.Nodes, n{x.id, x.label, "code", x.file, "L" + strconv.Itoa(x.line)})
	}
	for _, c := range calls {
		doc.Links = append(doc.Links, l{c[0], c[1], "calls"})
	}
	b, _ := json.Marshal(doc)
	gittest.Write(t, root, "graphify-out/graph.json", string(b))
	m := map[string]any{}
	for _, f := range manifest {
		m[filepath.Join(root, filepath.FromSlash(f))] = map[string]any{"mtime": 0}
	}
	mb, _ := json.Marshal(m)
	gittest.Write(t, root, "graphify-out/manifest.json", string(mb))
}

func fullHead(t *testing.T, root string) string {
	return strings.TrimSpace(gittest.Run(t, root, "rev-parse", "HEAD"))
}

var overlayProject = map[string]string{
	"src/calc.py":        "def add(a, b):\n    return a + b\n\n\ndef run(values):\n    def step(v):\n        return add(v, 1)\n\n    return [step(v) for v in values]\n",
	"src/other.py":       "def helper():\n    return 1\n",
	"tests/test_calc.py": "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n\n\ndef test_step():\n    assert step(1) == 2\n",
}

// overlayGraphify is what graphify would have recorded for overlayProject: start lines
// only, and `step` flat at file level where the scanner nests it under `run`.
func overlayGraphify(t *testing.T, root string) {
	t.Helper()
	all := []string{"src/calc.py", "src/other.py", "tests/test_calc.py"}
	writeGraphify(t, root, fullHead(t, root), []gfNode{
		{"calc_add", "add()", "src/calc.py", 1},
		{"calc_run", "run()", "src/calc.py", 5},
		{"calc_step", "step()", "src/calc.py", 6},
		{"other_helper", "helper()", "src/other.py", 1},
		{"test_add", "test_add()", "tests/test_calc.py", 4},
		{"test_step", "test_step()", "tests/test_calc.py", 8},
	}, [][2]string{{"test_add", "calc_add"}, {"test_step", "calc_step"}, {"calc_run", "calc_step"}, {"calc_step", "calc_add"}}, all)
}

// PRD #409 AC5, against a real repository: commit a change after graphify's commit; the
// changed file's nodes come from the scanner and an unchanged file's edge into it is
// re-pointed by name.
func TestOverlayReplacesAStaleFileAndRepointsEdgesIntoIt(t *testing.T) {
	root := repo(t, overlayProject)
	overlayGraphify(t, root)
	gittest.Write(t, root, "src/calc.py", "# moved down two lines\n\n"+overlayProject["src/calc.py"])
	gittest.Run(t, root, "add", "src/calc.py")
	gittest.Run(t, root, "commit", "-q", "-m", "shift")

	res := build(t, root)
	if res.Source != SourceGraphifyScanner || !reflect.DeepEqual(res.StaleFiles, []string{"src/calc.py"}) {
		t.Fatalf("Source %q, StaleFiles %v; want graphify+scanner with src/calc.py stale", res.Source, res.StaleFiles)
	}
	nodes := map[string]graph.Node{}
	for _, n := range res.Graph.Nodes {
		nodes[n.ID] = n
	}
	if n := nodes["src/calc.py::add"]; n.Start != 3 || n.End != 4 {
		t.Errorf("src/calc.py::add = %+v, want the scanner's span 3-4", n)
	}
	if _, ok := nodes["src/calc.py::step"]; ok {
		t.Error("graphify's src/calc.py::step survived in a stale file")
	}
	if n := nodes["tests/test_calc.py::test_step"]; n.Start != 8 || n.End != 8 {
		t.Errorf("tests/test_calc.py::test_step = %+v, want graphify's start-only node", n)
	}
	has := func(from, to string) bool {
		for _, e := range res.Graph.Edges {
			if e.From == from && e.To == to && e.Relation == graph.RelCalls {
				return true
			}
		}
		return false
	}
	if !has("tests/test_calc.py::test_step", "src/calc.py::run::step") {
		t.Error("graphify's test_step -> step edge was not re-pointed to the scanner's src/calc.py::run::step")
	}
	if !has("tests/test_calc.py::test_add", "src/calc.py::add") || !has("src/calc.py::run::step", "src/calc.py::add") {
		t.Errorf("edges = %+v; want test_add -> add kept and the scanner's step -> add", res.Graph.Edges)
	}
}

func TestOverlayDropsAnEdgeIntoARenamedNode(t *testing.T) {
	root := repo(t, overlayProject)
	overlayGraphify(t, root)
	gittest.Write(t, root, "src/calc.py", strings.ReplaceAll(overlayProject["src/calc.py"], "add", "plus"))
	res := build(t, root)
	for _, e := range res.Graph.Edges {
		if e.From == "tests/test_calc.py::test_add" {
			t.Errorf("an edge into the renamed add survived: %+v", e)
		}
	}
}

// Each source of spec §5 step 1, alone, makes a file stale.
func TestStaleSetSources(t *testing.T) {
	files := map[string]string{"a.py": "def a():\n    pass\n", "b.py": "def b():\n    pass\n",
		"c.py": "def c():\n    pass\n", "README.md": "# r\n"}
	code := []string{"a.py", "b.py", "c.py"}
	cases := []struct {
		name     string
		mutate   func(t *testing.T, root string)
		manifest []string
		changed  map[string]bool
		want     []string
	}{
		{"diff since built_at_commit", func(t *testing.T, root string) {
			gittest.Write(t, root, "a.py", "def a():\n    return 1\n")
			gittest.Commit(t, root, "a")
		}, append(code, "README.md"), nil, []string{"a.py"}},
		{"untracked file", func(t *testing.T, root string) {
			gittest.Write(t, root, "d.py", "def d():\n    pass\n")
		}, append(code, "d.py"), nil, []string{"d.py"}},
		{"working-tree changed set", func(*testing.T, string) {}, code, map[string]bool{"b.py": true}, []string{"b.py"}},
		{"absent from manifest.json", func(*testing.T, string) {}, []string{"a.py", "b.py"}, nil, []string{"c.py"}},
		{"a non-code file is never stale", func(t *testing.T, root string) {
			gittest.Write(t, root, "README.md", "# changed\n")
		}, code, nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repo(t, files)
			gf := &graphify.Graph{BuiltAtCommit: fullHead(t, root), CodeFiles: code, Manifest: c.manifest}
			c.mutate(t, root)
			listed, err := gitctx.ListFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			changed := c.changed
			if changed == nil {
				changed = map[string]bool{}
				cs, err := gitctx.ChangedSet(root, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				for _, ch := range cs {
					changed[ch.Path] = true
				}
			}
			got, err := StaleSet(root, gf, listed, changed)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("StaleSet = %v, want %v", got, c.want)
			}
		})
	}
}
```

Append to `cmd/rtdd/graph_test.go`:

```go
// writeGraphifyFor records graphify's view of graphRepo at builtAt (spec §5).
func writeGraphifyFor(t *testing.T, dir, builtAt string) {
	t.Helper()
	commit := ""
	if builtAt != "" {
		commit = `,"built_at_commit":"` + builtAt + `"`
	}
	gittest.Write(t, dir, "graphify-out/graph.json", `{"nodes":[`+
		`{"id":"add","label":"add()","file_type":"code","source_file":"src/calc.py","source_location":"L1"},`+
		`{"id":"total","label":"total()","file_type":"code","source_file":"src/calc.py","source_location":"L5"},`+
		`{"id":"t","label":"test_add()","file_type":"code","source_file":"tests/test_calc.py","source_location":"L4"}],`+
		`"links":[{"source":"total","target":"add","relation":"calls"},{"source":"t","target":"add","relation":"calls"}]`+commit+`}`)
}

// PRD #409 AC9, graphify half: the source, graphify's commit and the stale count.
func TestGraphCommandReportsGraphifyAndStaleness(t *testing.T) {
	dir := graphRepo(t)
	full := strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	writeGraphifyFor(t, dir, full)
	gittest.Write(t, dir, "tests/test_calc.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(2, 2) == 4\n")

	code, out, errOut := rtdd(t, dir, "graph")
	if code != 0 {
		t.Fatalf("rtdd graph = %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"source:          graphify+scanner\n", "built_at_commit: " + full + "\n", "stale_files:     1\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
	_, out, _ = rtdd(t, dir, "graph", "--json")
	var doc struct {
		Graph map[string]any `json:"graph"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Graph["source"] != "graphify+scanner" || doc.Graph["built_at_commit"] != full || doc.Graph["stale_files"] != 1.0 {
		t.Errorf("graph = %v", doc.Graph)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestDiffNamesSince' ./internal/gitctx/
go test -run 'TestOverlay|TestStaleSet' ./internal/graphbuild/
go test -run 'TestGraphCommandReportsGraphify' ./cmd/rtdd/
```

Expected: FAIL — `undefined: gitctx.DiffNamesSince`, `undefined: StaleSet`; then `Source "scanner", StaleFiles []; want graphify+scanner with src/calc.py stale`, and `stdout lacks "source:          graphify+scanner\n"`.

- [ ] **Step 3: Implement**

Append to `internal/gitctx/graphgit.go` (importing `sort`):

```go
// DiffNamesSince lists every path that differs between commit and the working tree —
// committed since, staged, or unstaged; deletions included; untracked files NOT included
// (ChangedSet has those). Sorted. graphify's staleness reads it (spec §5 step 1).
func DiffNamesSince(repoRoot, commit string) ([]string, error) {
	out, err := git(repoRoot, "diff", "--name-only", "-z", "--no-renames", commit, "--")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			names = append(names, p)
		}
	}
	sort.Strings(names)
	return names, nil
}
```

`internal/graphbuild/stale.go` — `StaleSet`, and a `staleness` that trusts graphify for every file outside it (Task 9 gives `staleness` its triggers):

```go
package graphbuild

import (
	"path"
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graphify"
)

// staleness decides how far graphify can be trusted. Task 8 trusts it for every file
// outside the stale set; Task 9 adds the three triggers that ignore it entirely.
func staleness(root string, gf *graphify.Graph, files []string, changed map[string]bool, maxRatio float64) ([]string, string, error) {
	stale, err := StaleSet(root, gf, files, changed)
	return stale, "", err
}

// StaleSet is spec §5 step 1: files changed since graphify's built_at_commit (committed,
// staged or not), ∪ the working-tree changed set (untracked included), ∪ files absent from
// graphify's manifest.json — restricted to CODE files: one graphify holds code nodes for,
// or a listed file sharing an extension with one. A README edit does not make graphify stale.
func StaleSet(root string, gf *graphify.Graph, files []string, changed map[string]bool) ([]string, error) {
	codeFile := map[string]bool{}
	ext := map[string]bool{}
	for _, f := range gf.CodeFiles {
		codeFile[f] = true
		ext[path.Ext(f)] = true
	}
	isCode := func(f string) bool { return codeFile[f] || ext[path.Ext(f)] && path.Ext(f) != "" }
	listed := map[string]bool{}
	for _, f := range files {
		listed[f] = true
	}

	set := map[string]bool{}
	diff, err := gitctx.DiffNamesSince(root, gf.BuiltAtCommit)
	if err != nil {
		return nil, err
	}
	for _, f := range diff {
		set[f] = true
	}
	for f := range changed {
		set[f] = true
	}
	if gf.Manifest != nil {
		inManifest := map[string]bool{}
		for _, f := range gf.Manifest {
			inManifest[f] = true
		}
		for _, f := range files {
			if !inManifest[f] {
				set[f] = true
			}
		}
	}
	var out []string
	for f := range set {
		if isCode(f) && (listed[f] || codeFile[f]) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out, nil
}
```

`internal/graphbuild/overlay.go`:

```go
package graphbuild

import (
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphify"
	"github.com/VocanicZ/rtdd/internal/scan"
)

// Overlay is spec §5 steps 2-3. graphify's nodes and outgoing edges in a stale file are
// dropped and the scanner's scan of that file (in scanned) stands in. A graphify edge
// from an unchanged file INTO a stale file is re-pointed to the scanner node(s) of the
// same name in that file, or dropped when there is none (renamed, deleted). The
// scanner's calls are then linked against the whole merged graph.
func Overlay(gf *graphify.Graph, stale map[string]bool, scanned []scan.FileResult) graph.Graph {
	var g graph.Graph
	gnode := map[string]graph.Node{}
	for _, n := range gf.Nodes {
		gnode[n.ID] = n
		if !stale[n.File] {
			g.Nodes = append(g.Nodes, n)
		}
	}
	byFileName := map[[2]string][]string{}
	calls := map[string][]string{}
	for _, r := range scanned {
		g.Nodes = append(g.Nodes, r.Nodes...)
		g.Edges = append(g.Edges, r.Edges...)
		for _, n := range r.Nodes {
			k := [2]string{n.File, n.Name}
			byFileName[k] = append(byFileName[k], n.ID)
		}
		for id, names := range r.Calls {
			calls[id] = names
		}
	}
	for _, e := range gf.Edges {
		from, to := gnode[e.From], gnode[e.To]
		switch {
		case stale[from.File]:
			continue
		case !stale[to.File]:
			g.Edges = append(g.Edges, e)
		default:
			for _, id := range byFileName[[2]string{to.File, to.Name}] {
				g.Edges = append(g.Edges, graph.Edge{From: e.From, To: id, Relation: e.Relation})
			}
		}
	}
	g.Edges = append(g.Edges, scan.Link(g.Nodes, calls)...)
	graph.Sort(&g)
	return g
}
```

In `Build`, between computing `head` and reading the cache, load graphify and — when it is present and not ignored — narrow `toScan` to the stale files and report them; after the scan, overlay instead of assembling. The whole of `Build` after Tasks 7, 8 and 9 is written out in Task 9, Step 3; a lane landing second rebases onto it.

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/gitctx/ ./internal/graphbuild/ ./cmd/rtdd/
go test -count=1 -run 'TestOnlyGitctxShellsOutToGit' ./internal/contract/
```

Expected: PASS.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/gitctx internal/graphbuild cmd/rtdd
git commit -m "feat(graphbuild): graphify staleness overlay, stale files from the scanner, edges re-pointed by name (closes #427)"
```

---

### Task 9: graphify is ignored, with a stated reason, when it cannot be trusted at all

**Issue:** #428

**Discharges:** PRD #409 AC6. Spec §5 step 4, §12.

**Files:**
- Modify: `internal/gitctx/graphgit.go` (`CommitKnown`), `internal/graphbuild/stale.go` (`staleness` gains the triggers)
- Test: `internal/gitctx/graphgit_test.go`, `internal/graphbuild/ignore_test.go`, `cmd/rtdd/graph_test.go`

**Interfaces:**
- Produces: `gitctx.CommitKnown(repoRoot, sha string) bool`; `Result.GraphifyIgnored` ∈ {`IgnoredNoCommit`, `IgnoredUnknownCommit`, `IgnoredTooStale`}; `rtdd graph` prints `graphify:        ignored — <why>; run \`graphify --update\` to use it again`, and `--json` carries `graph.graphify_ignored`.
- Consumes: `graph.Config.MaxStaleRatio`; Task 8's `writeGraphify`, `gfNode`, `fullHead`, `writeGraphifyFor`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/gitctx/graphgit_test.go` (importing `strings`):

```go
func TestCommitKnown(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "a.py", "x = 1\n")
	sha := gittest.Commit(t, dir, "one")
	full := strings.TrimSpace(gittest.Run(t, dir, "rev-parse", "HEAD"))
	for _, s := range []string{sha, full} {
		if !gitctx.CommitKnown(dir, s) {
			t.Errorf("CommitKnown(%q) = false for HEAD", s)
		}
	}
	for _, s := range []string{"", "0123456789abcdef0123456789abcdef01234567", "--all", "not a sha"} {
		if gitctx.CommitKnown(dir, s) {
			t.Errorf("CommitKnown(%q) = true", s)
		}
	}
}
```

`internal/graphbuild/ignore_test.go`:

```go
package graphbuild

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

var fourFiles = map[string]string{
	"a.py": "def a():\n    return 1\n", "b.py": "def b():\n    return 2\n",
	"c.py": "def c():\n    return 3\n", "d.py": "def d():\n    return 4\n",
}

// fourGraphify records one function per file of fourFiles at builtAt.
func fourGraphify(t *testing.T, root, builtAt string) {
	t.Helper()
	all := []string{"a.py", "b.py", "c.py", "d.py"}
	var nodes []gfNode
	for i, f := range all {
		nodes = append(nodes, gfNode{f, string(rune('a'+i)) + "()", f, 1})
	}
	writeGraphify(t, root, builtAt, nodes, nil, all)
}

// edit rewrites n of fourFiles' files after graphify was built, uncommitted.
func edit(t *testing.T, root string, files ...string) {
	for _, f := range files {
		gittest.Write(t, root, f, fourFiles[f]+"\n\ndef extra():\n    return 0\n")
	}
}

// PRD #409 AC6: each trigger alone makes graphify ignored, with its own reason.
func TestGraphifyIsIgnoredWithAReason(t *testing.T) {
	cases := []struct {
		name    string
		builtAt func(t *testing.T, root string) string
		edits   []string
		want    string
	}{
		{"built_at_commit missing", func(*testing.T, string) string { return "" }, nil, IgnoredNoCommit},
		{"built_at_commit unknown to git", func(*testing.T, string) string { return "0123456789abcdef0123456789abcdef01234567" }, nil, IgnoredUnknownCommit},
		{"more than half stale", fullHead, []string{"a.py", "b.py", "c.py"}, IgnoredTooStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repo(t, fourFiles)
			fourGraphify(t, root, c.builtAt(t, root))
			edit(t, root, c.edits...)
			res := build(t, root)
			if res.Source != SourceScanner || res.GraphifyIgnored != c.want {
				t.Errorf("Source %q, GraphifyIgnored %q; want scanner, %q", res.Source, res.GraphifyIgnored, c.want)
			}
			if len(res.Graph.Nodes) != 4+len(c.edits) {
				t.Errorf("%d nodes, want the scanner's whole graph (%d)", len(res.Graph.Nodes), 4+len(c.edits))
			}
		})
	}
}

// "Exceeds 50 %" (spec §5): exactly half stale keeps graphify.
func TestExactlyHalfStaleKeepsGraphify(t *testing.T) {
	root := repo(t, fourFiles)
	fourGraphify(t, root, fullHead(t, root))
	edit(t, root, "a.py", "b.py")
	res := build(t, root)
	if res.Source != SourceGraphifyScanner || res.GraphifyIgnored != "" || !reflect.DeepEqual(res.StaleFiles, []string{"a.py", "b.py"}) {
		t.Errorf("Source %q, GraphifyIgnored %q, StaleFiles %v; want graphify+scanner with a.py, b.py stale", res.Source, res.GraphifyIgnored, res.StaleFiles)
	}
}

func TestMaxStaleRatioOverride(t *testing.T) {
	root := repo(t, fourFiles)
	fourGraphify(t, root, fullHead(t, root))
	gittest.Write(t, root, ".rtdd/config.yaml", "max_stale_ratio: 0.25\n")
	edit(t, root, "a.py", "b.py")
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Build(root, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.GraphifyIgnored != IgnoredTooStale {
		t.Errorf("GraphifyIgnored = %q with max_stale_ratio 0.25 and 2 of 4 stale, want %q", res.GraphifyIgnored, IgnoredTooStale)
	}
}
```

Append to `cmd/rtdd/graph_test.go`:

```go
// PRD #409 AC6: an ignored graphify says why and suggests `graphify --update`.
func TestGraphCommandSaysWhyGraphifyWasIgnored(t *testing.T) {
	dir := graphRepo(t)
	writeGraphifyFor(t, dir, "")
	code, out, _ := rtdd(t, dir, "graph")
	if code != 0 || !strings.Contains(out, "source:          scanner\n") ||
		!strings.Contains(out, "no built_at_commit") || !strings.Contains(out, "graphify --update") {
		t.Errorf("exit %d, stdout:\n%s\nwant source scanner, the reason, and `graphify --update`", code, out)
	}
	_, out, _ = rtdd(t, dir, "graph", "--json")
	if !strings.Contains(out, `"graphify_ignored": "built_at_commit_missing"`) {
		t.Errorf("--json does not carry the reason:\n%s", out)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -run 'TestCommitKnown' ./internal/gitctx/
go test -run 'TestGraphifyIsIgnored|TestExactlyHalf|TestMaxStaleRatio' ./internal/graphbuild/
go test -run 'TestGraphCommandSaysWhy' ./cmd/rtdd/
```

Expected: FAIL — `undefined: gitctx.CommitKnown`; then `Source "graphify+scanner", GraphifyIgnored ""; want scanner, "built_at_commit_missing"` (and likewise `built_at_commit_unknown`, `too_stale`) — before the triggers, a graph with no `built_at_commit` diffs against `""` and errors, so the missing-commit case may instead fail with `git diff … : exit status 128`; either is the red this step expects.

- [ ] **Step 3: Implement**

Append to `internal/gitctx/graphgit.go`:

```go
// CommitKnown reports whether sha names a commit this repository has — false for a sha
// rebased away, never fetched (a shallow clone), or malformed (spec §5 step 4).
func CommitKnown(repoRoot, sha string) bool {
	if strings.TrimSpace(sha) == "" || strings.HasPrefix(sha, "-") {
		return false
	}
	_, err := git(repoRoot, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}
```

Replace `staleness` in `internal/graphbuild/stale.go`:

```go
// staleness decides how far graphify can be trusted: the stale set, or why graphify is
// ignored entirely (spec §5 step 4) — no built_at_commit, one git does not know, or more
// than maxRatio of graphify's code files stale.
func staleness(root string, gf *graphify.Graph, files []string, changed map[string]bool, maxRatio float64) ([]string, string, error) {
	if gf.BuiltAtCommit == "" {
		return nil, IgnoredNoCommit, nil
	}
	if !gitctx.CommitKnown(root, gf.BuiltAtCommit) {
		return nil, IgnoredUnknownCommit, nil
	}
	stale, err := StaleSet(root, gf, files, changed)
	if err != nil {
		return nil, "", err
	}
	if len(gf.CodeFiles) == 0 || float64(len(stale)) > maxRatio*float64(len(gf.CodeFiles)) {
		return stale, IgnoredTooStale, nil
	}
	return stale, "", nil
}
```

`Build` after Tasks 7, 8 and 9 — the shape every lane converges on:

```go
// Build builds the graph for the repository at root.
func Build(root string, cfg graph.Config, opt Options) (*Result, error) {
	if opt.CachePath == "" {
		opt.CachePath = filepath.Join(root, ".rtdd", "graph.json")
	}
	listed, err := gitctx.ListFiles(root)
	if err != nil {
		return nil, err
	}
	files := scan.Filter(root, listed, cfg.ScanExclude)
	changes, err := gitctx.ChangedSet(root, "HEAD")
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, c := range changes {
		changed[c.Path] = true
	}
	blobs, err := gitctx.BlobIDs(root)
	if err != nil {
		return nil, err
	}
	head, _ := gitctx.HeadSHA(root) // "" on an unborn HEAD

	res := &Result{Source: SourceScanner, BuiltAtCommit: head}
	gf, err := graphify.Load(root, cfg.GraphifyPath)
	switch {
	case errors.Is(err, graphify.ErrAbsent):
		gf = nil
	case err != nil:
		return nil, err
	}

	toScan := files
	var stale map[string]bool
	if gf != nil {
		res.GraphifyCommit, res.GraphifyFiles = gf.BuiltAtCommit, len(gf.CodeFiles)
		var list []string
		list, res.GraphifyIgnored, err = staleness(root, gf, files, changed, cfg.MaxStaleRatio)
		if err != nil {
			return nil, err
		}
		if res.GraphifyIgnored == "" {
			res.Source, res.BuiltAtCommit, res.StaleFiles = SourceGraphifyScanner, gf.BuiltAtCommit, list
			stale = map[string]bool{}
			for _, f := range list {
				stale[f] = true
			}
			toScan = nil
			for _, f := range files {
				if stale[f] {
					toScan = append(toScan, f)
				}
			}
		} else {
			res.StaleFiles = list
		}
	}

	c := readCache(opt.CachePath)
	results, scanned := scanCached(root, toScan, blobs, changed, c)
	res.Scanned = scanned
	if len(scanned) > 0 || !c.covers(toScan) {
		if err := writeCache(opt.CachePath, head, files, blobs, changed, c, results); err != nil {
			return nil, err
		}
	}

	if stale != nil {
		res.Graph = Overlay(gf, stale, results)
	} else {
		res.Graph = scan.Assemble(results)
	}
	graph.Classify(res.Graph.Nodes, cfg)
	return res, nil
}
```

(with `errors`, `path/filepath` and `internal/graphify` imported.)

- [ ] **Step 4: Run to verify they pass**

```bash
go test ./internal/gitctx/ ./internal/graphbuild/ ./cmd/rtdd/
```

Expected: PASS — including Task 8's tests, which never trip a trigger.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/gitctx internal/graphbuild cmd/rtdd
git commit -m "feat(graphbuild): ignore graphify with a stated reason when its commit is missing, unknown, or over 50 % stale (closes #428)"
```

---

### Task 10: 10 000 files cold in under 10 s; this repository warm in under 1 s

**Issue:** #429

**Discharges:** PRD #409 AC10, AC11. Spec §1 (success, measured), §4.5.

**Files:**
- Test: `internal/graphbuild/perf_test.go`
- Modify (only if a bound fails): `internal/scan/scan.go` (`ScanFiles` over a worker pool), `internal/scan/filter.go`, `internal/graphbuild/cache.go`

**Interfaces:**
- Consumes: `graphbuild.Build` with `Options.CachePath` in `t.TempDir()`, so the warm test never writes into the checkout.
- Produces: nothing new unless a bound fails.

- [ ] **Step 1: Write the failing tests**

`internal/graphbuild/perf_test.go`:

```go
package graphbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

// perfShapes are one small source file per fixture-corpus language. %[1]d makes every
// generated name distinct: a tree where 1 250 files each define `run` measures spec
// §4.3's accepted over-linking (n² calls edges), not the scanner.
var perfShapes = []struct{ ext, src string }{
	{".py", "class C%[1]d:\n    def run%[1]d(self, x):\n        return helper%[1]d(x)\n\n\ndef helper%[1]d(x):\n    if x:\n        return x + 1\n    return 0\n"},
	{".go", "package p\n\nfunc Helper%[1]d(x int) int {\n\tif x > 0 {\n\t\treturn x + 1\n\t}\n\treturn 0\n}\n\nfunc Run%[1]d() int { return Helper%[1]d(1) }\n"},
	{".ts", "export function helper%[1]d(x: number): number {\n  return x + 1;\n}\n\nexport const run%[1]d = (x: number) => {\n  return helper%[1]d(x);\n};\n"},
	{".java", "class C%[1]d {\n    int helper%[1]d(int x) {\n        return x + 1;\n    }\n\n    int run%[1]d() {\n        return helper%[1]d(1);\n    }\n}\n"},
	{".rs", "pub fn helper%[1]d(x: i32) -> i32 {\n    x + 1\n}\n\npub fn run%[1]d() -> i32 {\n    helper%[1]d(1)\n}\n"},
	{".rb", "class C%[1]d\n  def helper%[1]d(x)\n    x + 1\n  end\n\n  def run%[1]d\n    helper%[1]d(1)\n  end\nend\n"},
	{".lua", "local M = {}\n\nfunction M.helper%[1]d(x)\n  return x + 1\nend\n\nfunction M.run%[1]d()\n  return M.helper%[1]d(1)\nend\n\nreturn M\n"},
	{".sh", "helper%[1]d() {\n  echo $(( $1 + 1 ))\n}\n\nrun%[1]d() {\n  helper%[1]d 1\n}\n"},
}

// PRD #409 AC10, first half: a cold scanner build of 10 000 files in under 10 s.
func TestColdBuildOf10000FilesIsUnder10s(t *testing.T) {
	root := gittest.Init(t)
	for i := 0; i < 10000; i++ {
		s := perfShapes[i%len(perfShapes)]
		p := filepath.Join(root, fmt.Sprintf("pkg%03d", i/100), fmt.Sprintf("f%05d%s", i, s.ext))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(fmt.Sprintf(s.src, i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gittest.Commit(t, root, "10k")

	start := time.Now()
	res, err := Build(root, graph.DefaultConfig(), Options{CachePath: filepath.Join(t.TempDir(), "graph.json")})
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("cold build: %d files, %d nodes, %d edges in %v", len(res.Scanned), len(res.Graph.Nodes), len(res.Graph.Edges), took)
	if len(res.Scanned) != 10000 {
		t.Fatalf("scanned %d files, want 10000", len(res.Scanned))
	}
	if took >= 10*time.Second {
		t.Errorf("cold build took %v, want < 10s", took)
	}
}

// PRD #409 AC10, second half: on this repository, a warm build in under 1 s. The cache
// lives in a temp dir so the test never writes into the checkout.
func TestWarmBuildOfThisRepositoryIsUnder1s(t *testing.T) {
	wd, _ := os.Getwd()
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	cfg, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	opt := Options{CachePath: filepath.Join(t.TempDir(), "graph.json")}
	if _, err := Build(root, cfg, opt); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	res, err := Build(root, cfg, opt)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("warm build: %d nodes, %d edges, %d re-scanned, in %v", len(res.Graph.Nodes), len(res.Graph.Edges), len(res.Scanned), took)
	if took >= time.Second {
		t.Errorf("warm build took %v, want < 1s", took)
	}
}
```

- [ ] **Step 2: Run to verify they fail — or record the headroom**

```bash
go test -count=1 -v -run 'TestColdBuildOf10000Files|TestWarmBuildOfThisRepository' ./internal/graphbuild/
```

Expected: FAIL before Tasks 7–9 have landed (`undefined: Options.CachePath`, or a warm build that re-scans every file). On the prototype this plan was written against, with all of Tasks 1–9 in place, the cold build of 10 000 files took **≈2.4 s** single-threaded and the warm build of this repository **≈0.45 s** (≈0.69 s while the cache was still rewritten on every call — the `covers` check in Task 7 is what bought the difference). Both pass there; this step's job is to read the real numbers on this tree and on CI.

- [ ] **Step 3: Make it pass, with headroom**

If either bound is within 2× of its limit on CI hardware, take, in order: (1) `scan.ScanFiles` over `runtime.GOMAXPROCS(0)` workers, results re-sorted by path; (2) `scan.Filter` skipping the 8 KiB NUL sniff for a file whose cached blob id is unchanged (it was sniffed when it was scanned); (3) `graph.Sort` comparing pre-split keys. Do **not** relax a bound, cache across `rtdd` versions, or skip linking. State the measured cold and warm times, and the CI runner's, in the PR.

Names defined in many files (`run`, `get`, `__init__`) make `Link` over-link quadratically — spec §4.3 accepts that, and `rtdd doctor`'s ≥ 8 list (spec §8, PRD #410) is where it is surfaced. The generated tree uses distinct names so the bound measures the scanner, not a hub.

- [ ] **Step 4: Run the whole PRD**

```bash
go test -count=1 ./internal/graph/ ./internal/scan/ ./internal/graphify/ ./internal/graphbuild/ ./internal/gitctx/ ./cmd/rtdd/ ./internal/contract/
```

Expected: PASS — every PRD #409 acceptance criterion, AC1–AC10, has its test green on one tree.

- [ ] **Step 5: Gate and commit**

```bash
scripts/ci-local.sh
git add internal/graphbuild internal/scan
git commit -m "test(graphbuild): the graph builds 10 000 files cold in under 10 s and this repo warm in under 1 s (closes #429)"
```

`scripts/ci-local.sh` exiting 0 on `main` after this merge is AC11, and closes PRD #409.

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #409, and no task above may start it.

- **Rounds, and `rtdd which` on the graph.** `Rounds(graph, changedRanges)`, the synthetic `file::<module>` node for top-level code, depth-1 neighbours, `untested` — spec §7 — and `which`'s schema 3 output are PRD #410. This plan builds the graph `which` will read; it does not change what `which` prints.
- **Deleting the coverage pipeline.** `seed`, `run`, `verify`, `status`, `map compact`, the adapters, `internal/{adapter,covfmt,runner,mapstore,selector,uncovered}`, `.rtdd/map.jsonl` and the merge driver all stay — PRD #410. N1 ships beside them.
- **`rtdd explain` and `rtdd doctor` on the graph**, including `doctor`'s names-defined-≥ 8-times list (spec §8) — PRD #410.
- **`rtdd init` writing `.rtdd/graph.json` into `.gitignore`**, the skill text, and v0.2 → v0.3 migration — PRD #411. Task 7 adds the line to this repository's `.gitignore` only.
- **Running or updating graphify.** No code path executes it, offers to, or writes into `graphify-out/` (spec §5, §11, §12).
- **Spec §12's defaults.** The cache stays gitignored, graphify stays ignored past 50 % and is never run, all same-named definitions stay linked. A lane that finds one of these wrong files an issue against the spec; it does not change it here.
- **Languages and shapes beyond the corpus.** C#, C/C++, PHP, Kotlin, Swift and the rest get whatever the §4.2 table gives them, unproven: no fixture, no claim. Allman-style braces (`{` alone on the next line) are indentation-ruled and end at the definition line; multi-line string literals can unbalance brace counting; a definition inside a docstring or comment can be matched. Each is a fixture-first issue, not a silent extension of this plan.
- **Depth, weights, or pruning of over-linked names.** Linking stays all-same-named (§4.3, §12); hub handling is `doctor`'s report, not a scanner change.
- **Benchmarks.** No rtdd-bench row is re-run; time-in-tests and recall for Rounds 1+2 (spec §1) need Rounds, which are PRD #410.
