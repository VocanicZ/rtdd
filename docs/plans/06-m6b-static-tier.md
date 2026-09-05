# RTDD M6b — The `TS` Static Selection Tier

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give a repository RTDD cannot instrument a narrowed, ranked test selection instead of the full suite. `internal/selector` gains a fifth tier, `TS`, resolved between T1 and T2: when the coverage relation cannot answer — the map is unseeded, or the adapter declares `selection: static` — the selector chooses tests by declared `test_for` correspondence and by transitive imports, ranks them three ways, and says so in a reason that never claims coverage it does not have. A seeded Python repository's answer does not move by one byte. Nothing here runs a test: the runner, the `junit-xml` parser, the shipped non-Python adapters and the measured evidence table are sibling PRDs.

**Architecture:** `Select` stays pure. Everything the static tier needs about the world arrives through `Inputs` as injected functions, exactly as `Distance` and `ImportOnly` already do: `Exists(rel string) bool` answers "does the repository have this file" for `test_for` resolution, and `ImportDistance(changed string) map[string]int` answers "which tests reach this file, in how many hops". `internal/adapter` owns template expansion (`TestForCandidate`) because it owns the templates; `internal/selector` owns candidacy and ranking (`staticCandidates`, `rankStatic`); `internal/importscan` gains the impure half — running an adapter's declared `importscan` script over a JSON request on stdin, the shape the embedded Python scanner already uses. `cmd/rtdd` supplies both functions and is, as ever, the only package that touches the filesystem on this path.

**Tech Stack:** Go 1.24+, stdlib testing. No new dependency.

**Spec:** `docs/specs/2026-09-05-multi-language.md` §4.1 (the `TS` tier, its position between T1 and T2, and its three-level ranking) and §2 (the two-axis split: *execution-derived* selection is coverage that was recorded, *static* selection is declared correspondence or imports, and the two must never be confused on any surface). Each task below names the section it discharges on its `**Discharges:**` line. Everything else in that spec — the `junit-xml` parser and the runner (§4.3), polyglot selection (§4.4), the shipped adapter set (§8), the evidence table (§7) and the remaining §6 surfaces — belongs to a sibling or later PRD and is listed at the end of this document.

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

M6b-specific:

- **A seeded repository's selection is byte-identical to before this milestone. This is a
  constraint on every task in this plan, not a concern of the one task that tests it.**
  Spec §4.1 states it as a regression *requirement*: "`TS` never overrides a usable
  coverage relation. In a Python repository with a seeded map, behaviour is byte-identical
  to today." Task 2 pins it with a golden file over the existing `internal/selector`
  fixtures — tier, direct set, ranked list and reason, whole — and that golden is
  generated from pre-change behaviour and never regenerated afterwards. No later task may
  pass by running `-update`. If a task's change moves the golden, the change is wrong; the
  golden is not.
- **`Select` remains pure.** No filesystem access, no subprocess, no git, no printing, in
  `internal/selector` or anything it calls. The two new `Inputs` fields are functions for
  exactly this reason: `test_for` resolution asks "does this file exist" and import
  ranking asks the import graph, and both questions have to be answered *outside* the
  selector and injected. A task that reaches for `os.Stat` inside `internal/selector` has
  taken the wrong route.
- **`adapters/python.yaml` stays byte-frozen.** `internal/contract`'s sha256 guard from
  M6a Task 8 is load-bearing here for the same reason it was there: the byte-identical
  promise above is cheapest to keep by not touching the only adapter that has ever
  produced a map. No task in this plan lists that file. Python therefore declares no
  `test_for` and no `importscan`, and an unseeded Python repo keeps its existing T2 answer
  word for word.
- **Nothing in this milestone runs a test.** `internal/runner` and `internal/report` are
  not opened. `TS` produces a ranked list; who executes it, and how its outcome is parsed,
  is M6c.
- **Exit codes are unchanged** (`00-interfaces.md`) with one addition this plan makes
  deliberately: `rtdd seed` against a `selection: static` adapter exits **2**, a
  configuration error — asking a toolchain that records nothing to record something is a
  misconfiguration, and a silent success that builds no map is the defect PRD #230 AC9b
  names.

### The three under-determined decisions, resolved here

Spec §4.1 leaves three things open. They are decided here so no implementer has to guess,
and each has a task that pins the decision with a test.

**1. Path proximity ORDERS the static selection; it never admits a test to it** (Tasks 4
and 5). Spec §4.1 lists three ranking levels, but only the first two are also *evidence
that a test is related at all*: a `test_for` template resolving to a real file is a
declaration by the adapter author that this test covers that source file, and a transitive
import is a fact about the code. A shared directory prefix is neither — every test in
`src/auth/` shares a prefix with every source file in `src/auth/`, so admitting on
proximity would select the directory, then the package, then, at the root, the suite.

The M6a plan already fixed this from the other side: an adapter with neither a `test_for`
template nor an `importscan` command has fidelity **`none`**, precisely because "all it
could offer is path proximity, which is exactly the `path` baseline spec §7 pre-registers
the static tier against". If proximity could admit on its own, `TS` would *be* the `path`
baseline wearing a tier name, the pre-registration in §7 would compare a thing to itself,
and a fidelity-`none` adapter would return a `TS` selection that its own `rtdd doctor`
line says it cannot produce. So:

| Level | Signal | Admits? | Orders? |
|---|---|---|---|
| 1 | `test_for` correspondence resolving to an existing file | **yes** | first |
| 2 | transitive import distance, shortest first | **yes** | after level 1 |
| 3 | longest shared directory prefix with a changed file | **no** | final tiebreak, before lexicographic |

Task 4 asserts the "no" — a test in the changed file's own directory that neither
corresponds nor imports is *absent* from the candidate set. Task 5 asserts all three
orderings against one fixture.

**2. An adapter with `Fidelity() == none` selects the full suite, as T2, and is told why**
(Task 6). It declares `selection: static`, so the map cannot answer and the `TS` gate
opens — but it has no `test_for` and no `importscan`, so the candidate set is empty, and
by decision 1 proximity may not fill it. The two alternatives are both worse. An empty
`TS` would report a tier that narrowed nothing while `Selection.Tests` was empty, and
`RenderWhich` would print `NOTHING SELECTED` under a tier claiming a static selection —
the exact "empty is not the same as all passed" confusion spec §5 exists to prevent. A
`TierEmpty` would be a lie in the other direction: nothing was ruled out, so running
nothing is unsafe. T2 with the full suite is the honest answer, and it is what the
adapter's `rtdd doctor` line already promises: "RTDD cannot select anything narrower than
the full suite (T2)."

Its reason names the missing declarations rather than the map, and — PRD #230 AC9c —
never says `run rtdd seed`:

```
the elixir adapter declares selection: static but no test_for templates and no
importscan command, so nothing narrower than the full suite can be derived
```

A static adapter that *does* have declarations but finds nothing for this particular
change is also T2, with its own reason (`neither correspondence nor imports reach the
changed set`) — again with no seed advice. Only an unseeded **coverage** adapter keeps
today's `the map is unseeded, so no selection is trustworthy: run rtdd seed`, byte for
byte, because for that adapter seeding is genuinely the fix.

**3. The gate reads `Adapter.Selection` and `Map.Len()`; `Inputs` gains exactly two
function fields** (Tasks 1 and 6). `TS` is attempted when

```go
(in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic) || m.Len() == 0
```

— the declaration, or an unanswerable map. It is **not** read from `Fidelity()`.
`Fidelity` answers *what is the best this adapter could ever produce*, which is the
question `doctor` asks; the gate asks *can the coverage relation answer this call*, which
is a different question with a different answer in the case that matters: a fidelity-`none`
static adapter must still enter the gate, because otherwise it would fall through to the
unseeded-map branch and be told to seed a map it can never build. Fidelity decides what
comes *out* of the gate (decision 2), not who goes in.

PRD #230 AC4's other half follows from the same expression: a **seeded** map that
legitimately selects zero tests has `m.Len() > 0` and a coverage adapter, so the gate is
shut and the answer stays an explicit `TierEmpty` T0 — the static tier never launders an
honest empty into a speculative selection.

`Inputs` needs two new fields, and only two, both functions, because `Select` is pure:

```go
Exists         func(rel string) bool           // does the repository have this path?
ImportDistance func(changed string) map[string]int // test file -> shortest import hops
```

`Exists` rather than a pre-computed file list: a template resolves to one candidate path
per changed file, so the question is a membership test over a set the caller already has
cheaply (`os.Lstat`), whereas materialising every path in the repository to answer three
questions is the expensive way round. `ImportDistance` returns a map per *changed* file
rather than a `func(changed, test string) int` per pair because the scanner answers in
bulk — `internal/importscan.Scan` already returns `map[target][]test` from one subprocess,
and a per-pair signature would turn one process into `len(tests)` calls. Both are `nil`
when the adapter declares nothing: `nil` means *skip this level*, never *fail* (PRD #230
AC7). `AllTests` is not new — it already carries the suite, and it is the universe the
scanner is asked about and the pool level 3 orders.

## What is already true in the tree

Read these before writing code; several tasks are smaller than they look.

- `internal/selector/select.go` — `Select`'s doc comment already documents the resolution
  order as a numbered list, and `escalateFull` already returns `("the map is unseeded, so
  no selection is trustworthy: run rtdd seed", true)` as its **first** check. That branch
  is the one this milestone diverts; the other two (`full_escalate` file, drift guard) are
  untouched and stay ahead of it.
- `internal/selector/tier.go` — `TierEmpty` is `iota`'s zero and `TestTierEmptyIsTheZeroValue`
  enforces it. `TierTS` is inserted between `TierT1` and `TierT2`, which renumbers `TierT2`
  from 4 to 5. Nothing persists a `Tier` as an integer: `--json` and the human renderer
  both go through `Tier.String()`.
- `internal/adapter/adapter.go` — `TestFor []string` and `Importscan *Importscan` already
  parse and validate (M6a Task 1), and `validateTemplates` already rejects any placeholder
  in a `test_for` entry other than `{dir}` and `{name}` (`testForPlaceholders`,
  `adapter.go:99`). So expansion needs no validation of its own: an unknown placeholder
  cannot reach it.
- `internal/adapter/fidelity.go` — `Fidelity()` is already derived, already returns `none`
  for a static adapter with no `test_for` and no `importscan`, and already carries the
  reason decision 1 restates. Task 6 consumes it; it does not change.
- `internal/importscan/scan.go` — `Scan` already writes a JSON `request` to a child
  process's stdin and reads a JSON answer from stdout, already terminates cycles with a
  visited set, and `Scanner` already memoises across one command invocation and degrades
  (returns `nil`, retains `Err`) rather than failing the command. The adapter-declared
  scanner in Task 7 is a second function in that file's shape, not a second design.
- `cmd/rtdd/which.go` — `selector.Inputs` is built in one place (`which.go:89`) and one
  other (`run.go:92`); `newImportFallback(...).testsImporting` is the precedent for
  wiring an impure resolver into a pure selector.
- `cmd/rtdd/init.go:136` — the next-step line is one unconditional `Fprintln`. That
  unconditionality is the defect PRD #230 AC9a closes.
- `cmd/rtdd/seed.go` — `cmdSeed` calls `detectAdapter` and goes straight to
  `runner.Seed`. There is no fidelity check anywhere on that path; AC9b adds the first.

## File Structure

| File | Single responsibility |
|---|---|
| `internal/selector/tier.go` | **MODIFIED** — `TierTS`, its `String()` case, and the `Exists` / `ImportDistance` fields on `Inputs` |
| `internal/selector/tier_test.go` | **MODIFIED** — `TierTS` naming and ordering, `Inputs` round-trip |
| `internal/selector/golden_test.go` | **NEW** — the byte-identical-selection guard over the seeded fixtures |
| `internal/selector/testdata/selection_golden.txt` | **NEW** — generated once from pre-change behaviour, never regenerated |
| `internal/adapter/testfor.go` | **NEW** — `(*Adapter).TestForCandidate`: `{dir}`/`{name}` expansion, declaration order, first existing file wins |
| `internal/adapter/testfor_test.go` | **NEW** — expansion, ordering, no-match, repo-root, nil-`exists` |
| `internal/selector/static.go` | **NEW** — `staticCandidate`, `staticCandidates`, `rankStatic`, `staticReason`, `unanswerableReason` |
| `internal/selector/static_test.go` | **NEW** — candidacy (levels 1 and 2 only) and the three-level ordering |
| `internal/selector/select.go` | **MODIFIED** — the documented order gains `TS`; the unseeded branch leaves `escalateFull` and becomes the `TS` gate |
| `internal/selector/select_test.go` | **MODIFIED** — `TS` resolution, fidelity-`none` fallback, the `recorded coverage` and `run rtdd seed` substring assertions |
| `internal/importscan/adapterscan.go` | **NEW** — `AdapterScanner`: runs an adapter's declared `importscan` script and returns hop counts |
| `internal/importscan/adapterscan_test.go` | **NEW** — a stub script fixture, a missing-script degradation, memoisation |
| `internal/importscan/testdata/scan-imports-stub.sh` | **NEW** — a fixture scanner that answers from a canned graph |
| `cmd/rtdd/static.go` | **NEW** — `repoExists` and `adapterImportDistance`: the two impure resolvers |
| `cmd/rtdd/which.go` | **MODIFIED** — passes `Exists` and `ImportDistance` |
| `cmd/rtdd/run.go` | **MODIFIED** — passes the same two |
| `cmd/rtdd/init.go` | **MODIFIED** — `RenderNextStep`: the next-step line is fidelity-aware and scoped in a mixed repo |
| `cmd/rtdd/seed.go` | **MODIFIED** — refuses a `selection: static` adapter, exit 2 |
| `cmd/rtdd/init_test.go`, `cmd/rtdd/seed_test.go` | **MODIFIED/NEW** — AC9a–d, including the `run rtdd seed` substring assertion |
| `docs/plans/00-interfaces.md` | **MODIFIED** — record every contract addition made here |

---

### Task 1 — `internal/selector`: `TierTS`, the documented order, and the two static resolvers

**Discharges:** spec §4.1 (the tier and its position). PRD #230 AC1 and AC2.

**Files:**
- Modify: `internal/selector/tier.go`
- Test: `internal/selector/tier_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `TierTS`, ordered `TierT1 < TierTS < TierT2`, `String() == "TS"`
  - `Inputs.Exists func(rel string) bool`
  - `Inputs.ImportDistance func(changed string) map[string]int`

- [ ] **Step 1: Write the failing test**

Append to `internal/selector/tier_test.go`:

```go
func TestTierTSString(t *testing.T) {
	if got := TierTS.String(); got != "TS" {
		t.Errorf("TierTS.String() = %q, want %q", got, "TS")
	}
}

// Spec §4.1 inserts TS BETWEEN T1 and T2, and the constants are that order. TierEmpty
// must survive the insertion as the zero value: a constant added in the wrong place
// renumbers it, and an unfilled Selection would then read as a successful tier.
func TestTierTSIsOrderedBetweenT1AndT2(t *testing.T) {
	if int(TierEmpty) != 0 {
		t.Fatalf("TierEmpty = %d, want 0 (the zero value)", int(TierEmpty))
	}
	if !(TierT1 < TierTS && TierTS < TierT2) {
		t.Errorf("order = T1:%d TS:%d T2:%d, want T1 < TS < T2",
			int(TierT1), int(TierTS), int(TierT2))
	}
}

// The static tier's two questions — "does this file exist" and "which tests import this
// file, how far away" — arrive as injected functions so Select stays pure.
func TestInputsCarriesTheStaticResolvers(t *testing.T) {
	in := Inputs{
		Exists:         func(rel string) bool { return rel == "src/a.test.ts" },
		ImportDistance: func(string) map[string]int { return map[string]int{"src/a.test.ts": 2} },
	}
	if !in.Exists("src/a.test.ts") || in.Exists("nope.ts") {
		t.Error("Inputs.Exists did not round-trip")
	}
	if got := in.ImportDistance("src/a.ts")["src/a.test.ts"]; got != 2 {
		t.Errorf("Inputs.ImportDistance(...)[src/a.test.ts] = %d, want 2", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run 'TestTierTS|TestInputsCarriesTheStaticResolvers' -v`
Expected: FAIL — `internal/selector/tier_test.go:…: undefined: TierTS` and `unknown field Exists in struct literal of type Inputs` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

In `internal/selector/tier.go`, insert the constant and its `String()` case, and add the
two fields to `Inputs`:

```go
const (
	// TierEmpty means nothing was selected. It is reported explicitly and is never
	// collapsed into success: every under-selection path terminates here.
	TierEmpty Tier = iota
	TierDirect
	TierT0
	TierT1
	// TierTS is the static tier (spec §4.1): tests chosen from declared test_for
	// correspondence and transitive imports, used when the coverage relation cannot
	// answer. It sits between T1 and T2 in confidence — narrower than the full suite,
	// and never a substitute for a usable map.
	TierTS
	TierT2
)
```

```go
	case TierTS:
		return "TS"
```

```go
type Inputs struct {
	// … existing fields …

	// Exists reports whether the repository has this repo-relative path. It resolves
	// test_for templates (spec §4.2) without the selector touching a filesystem.
	// nil means level-1 correspondence is skipped, not that it failed.
	Exists func(rel string) bool

	// ImportDistance maps a changed file to the test files that transitively import it,
	// valued by the shortest number of import hops. It is the level-2 signal of spec
	// §4.1. nil — an adapter declaring no importscan — means level 2 is skipped, not
	// that it failed (PRD #230 AC7).
	ImportDistance func(changed string) map[string]int
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -count=1`
Expected: PASS, including every pre-existing selector test unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/selector/tier.go internal/selector/tier_test.go
git commit -m "M6b: TierTS and the two static resolvers on Inputs"
```

---

### Task 2 — `internal/selector`: the byte-identical-selection guard

**Discharges:** spec §4.1's regression requirement ("in a Python repository with a seeded
map, behaviour is byte-identical to today"). PRD #230 AC3, which calls this guard
load-bearing and rules out a spot check.

**Files:**
- Create: `internal/selector/golden_test.go`, `internal/selector/testdata/selection_golden.txt`

**Interfaces:**
- Consumes: `Select`, and the existing `baseInputs`, `fixtureSelectMap`, `fixtureAdapter`,
  `mod`, `added`, `deleted`, `fresh` helpers in `select_test.go` / `rank_test.go`.
- Produces: a golden file that later tasks must not move.

This task comes second, before a single line of static-tier behaviour exists, so the
golden captures pre-change behaviour by construction. Task 1 added a constant and two
nil-valued fields and cannot have moved it.

- [ ] **Step 1: Write the failing test**

Create `internal/selector/golden_test.go`:

```go
package selector

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// -update regenerates the golden. It is run ONCE, in this task, to capture behaviour as
// it stands before the static tier exists. A later task that finds the golden moved has
// found a regression, not a stale file: spec §4.1 makes a seeded repository's selection
// byte-identical a requirement, so the fix is the code, never this file.
var update = flag.Bool("update", false, "rewrite testdata/selection_golden.txt")

type goldenCase struct {
	name   string
	mutate func(*Inputs)
}

// seededCases covers every shape the existing fixtures reach with a SEEDED map: the
// direct tier, T0, each T1 escalation, each T2 escalation, and the explicit empty.
func seededCases() []goldenCase {
	return []goldenCase{
		{"direct only, brand new test file", func(in *Inputs) {
			in.Changes = []gitctx.Change{added("tests/test_brand_new.py")}
		}},
		{"direct before mapped", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py"), mod("tests/test_db.py")}
		}},
		{"deleted test file is not run", func(in *Inputs) {
			in.Changes = []gitctx.Change{deleted("tests/test_auth.py")}
		}},
		{"T0 one source file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
		}},
		{"T0 two source files", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py"), mod("src/db.py")}
		}},
		{"T0 through a rename's old path", func(in *Inputs) {
			in.Changes = []gitctx.Change{{
				Path: "src/auth2.py", OldPath: "src/auth.py", Status: gitctx.Renamed,
			}}
		}},
		{"empty, nothing covers the change", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/untouched.py")}
		}},
		{"T1 merge commit", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Merge = true
		}},
		{"T1 opaque file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("templates/page.html")}
		}},
		{"T1 stale row", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Distance = func(string) int { return 999 }
		}},
		{"T2 full-escalate file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("requirements.txt")}
		}},
		{"T2 drift guard", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Cycles = 100
		}},
	}
}

func renderSelection(name string, s Selection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "== %s\n", name)
	fmt.Fprintf(&b, "tier:   %s\n", s.Tier)
	fmt.Fprintf(&b, "direct: %s\n", strings.Join(s.Direct, " "))
	fmt.Fprintf(&b, "tests:  %s\n", strings.Join(s.Tests, " "))
	fmt.Fprintf(&b, "reason: %s\n\n", s.Reason)
	return b.String()
}

// The whole Selection is compared, not a field of it: a regression that kept the tier and
// reordered the ranked list would pass any spot check and would still change what an
// agent runs first.
func TestSeededSelectionIsByteIdentical(t *testing.T) {
	var b strings.Builder
	for _, tc := range seededCases() {
		in := baseInputs()
		in.AllTests = []string{
			"tests/test_auth.py::test_login",
			"tests/test_auth.py::test_logout",
			"tests/test_db.py::test_query",
			"tests/test_render.py::test_page",
		}
		tc.mutate(&in)
		b.WriteString(renderSelection(tc.name, Select(in)))
	}
	got := b.String()

	golden := filepath.Join("testdata", "selection_golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden: %v (generate it once with -update)", err)
	}
	if got != string(want) {
		t.Errorf("a seeded repository's selection moved; spec §4.1 forbids it.\n"+
			"--- got ---\n%s--- want ---\n%s", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestSeededSelectionIsByteIdentical -v`
Expected: FAIL — `reading the golden: open testdata/selection_golden.txt: no such file or directory (generate it once with -update)`.

- [ ] **Step 3: Generate the golden and read it**

Run: `go test ./internal/selector/... -run TestSeededSelectionIsByteIdentical -update`

Then **read `internal/selector/testdata/selection_golden.txt` before committing it**. It
is the definition of "unchanged" for the rest of this plan, so a wrong line here would be
frozen as correct. Check that the T0 cases name the mapped tests, that the T2 cases carry
the full `AllTests` list, that `empty, nothing covers the change` has an empty `tests:`
line and a reason, and that the unseeded reason does not appear anywhere — no case in the
table has an empty map.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/selector/golden_test.go internal/selector/testdata/selection_golden.txt
git commit -m "M6b: golden guard — a seeded repo's selection is byte-identical"
```

---

### Task 3 — `internal/adapter`: `test_for` resolution

**Discharges:** spec §4.2 (`test_for` — "path-correspondence templates, tried in order")
and §4.1 level 1. PRD #230 AC6.

**Files:**
- Create: `internal/adapter/testfor.go`
- Test: `internal/adapter/testfor_test.go`

**Interfaces:**
- Consumes: `Adapter.TestFor`, and an injected `exists func(string) bool`.
- Produces: `func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool)`

Expansion lives in `internal/adapter` because the adapter owns the templates and already
owns the placeholder vocabulary (`testForPlaceholders`) and the rejection of anything
outside it. `internal/selector` calls it and stays a package about tiers.

- [ ] **Step 1: Write the failing test**

Create `internal/adapter/testfor_test.go`:

```go
package adapter

import "testing"

func testForAdapter() *Adapter {
	return &Adapter{
		Name:      "typescript",
		Selection: SelectionStatic,
		Coverage:  CoverageNone,
		TestFor: []string{
			"{dir}/{name}.test.ts",
			"{dir}/__tests__/{name}.test.ts",
			"tests/{name}.test.ts",
		},
	}
}

func existsIn(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

func TestTestForCandidateExpandsDirAndName(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate(
		"src/auth/token.ts", existsIn("src/auth/token.test.ts"))
	if !ok {
		t.Fatal("TestForCandidate found nothing, want src/auth/token.test.ts")
	}
	if got != "src/auth/token.test.ts" {
		t.Errorf("TestForCandidate = %q, want %q", got, "src/auth/token.test.ts")
	}
}

// Declaration order is the adapter author's confidence order, so the first template that
// resolves wins even when a later one also does.
func TestTestForCandidateTriesTemplatesInDeclarationOrder(t *testing.T) {
	exists := existsIn("src/auth/__tests__/token.test.ts", "tests/token.test.ts")
	got, ok := testForAdapter().TestForCandidate("src/auth/token.ts", exists)
	if !ok {
		t.Fatal("TestForCandidate found nothing")
	}
	if got != "src/auth/__tests__/token.test.ts" {
		t.Errorf("TestForCandidate = %q, want the earlier template's %q",
			got, "src/auth/__tests__/token.test.ts")
	}
}

// "The first template resolving to an EXISTING file wins" — a template that expands
// cleanly but names nothing is not a candidate. Guessing here would select a path the
// runner then rejects.
func TestTestForCandidateReportsNoMatchRatherThanAGuess(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate("src/auth/token.ts", existsIn())
	if ok {
		t.Errorf("TestForCandidate = %q, true; want no match when no file exists", got)
	}
}

// At the repository root path.Dir is ".", and "./index.test.ts" matches no repo-relative
// path in the engine, which keeps every path cleaned.
func TestTestForCandidateAtTheRepoRootEmitsNoDotSegment(t *testing.T) {
	got, ok := testForAdapter().TestForCandidate("index.ts", existsIn("index.test.ts"))
	if !ok || got != "index.test.ts" {
		t.Errorf("TestForCandidate = %q, %v; want %q, true", got, ok, "index.test.ts")
	}
}

// A nil exists — a caller with no filesystem to ask — skips level 1. It is not fatal
// and it is not an assumption that the file is there.
func TestTestForCandidateWithoutAnExistsFunctionSkips(t *testing.T) {
	if _, ok := testForAdapter().TestForCandidate("src/a.ts", nil); ok {
		t.Error("TestForCandidate resolved with a nil exists; want no match")
	}
}

// An adapter declaring no templates is the ordinary Python case, not an error.
func TestTestForCandidateWithNoTemplatesSkips(t *testing.T) {
	a := &Adapter{Name: "python"}
	if _, ok := a.TestForCandidate("src/a.py", existsIn("src/a.py")); ok {
		t.Error("TestForCandidate resolved without templates; want no match")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/... -run TestTestForCandidate -v`
Expected: FAIL — `internal/adapter/testfor_test.go:…: a.TestForCandidate undefined (type *Adapter has no field or method TestForCandidate)` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/adapter/testfor.go`:

```go
package adapter

import (
	"path"
	"strings"
)

// TestForCandidate resolves this adapter's test_for templates against one changed source
// file and returns the FIRST template naming a file the repository actually has.
//
// Templates are tried in declaration order because the order is the adapter author's
// confidence ranking: "{dir}/{name}.test.ts" before "tests/{name}.test.ts" says
// co-located tests are the convention here and the central directory is the fallback.
// Resolving to an existing file, rather than to a plausible path, is what makes level 1
// of spec §4.1 evidence rather than a guess — a template always expands, so expansion
// alone would nominate a test for every changed file in the repository.
//
// exists is injected rather than called through os.Stat here because every caller up to
// selector.Select is pure; see that package's contract.
//
// No placeholder validation happens here: validateTemplates already rejects any name
// outside {dir} and {name} at load time, so an unknown one cannot reach this function.
func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool) {
	if a == nil || exists == nil || len(a.TestFor) == 0 {
		return "", false
	}
	r := strings.NewReplacer(
		"{dir}", path.Dir(rel),
		"{name}", strings.TrimSuffix(path.Base(rel), path.Ext(rel)),
	)
	for _, tmpl := range a.TestFor {
		// Clean collapses the "./" a root-level {dir} would otherwise produce; engine
		// paths are always cleaned and slash-separated.
		cand := path.Clean(r.Replace(tmpl))
		if cand == "" || cand == "." {
			continue
		}
		if exists(cand) {
			return cand, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/... -count=1`
Expected: PASS, 7 new tests, every existing adapter test unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/testfor.go internal/adapter/testfor_test.go
git commit -m "M6b: adapter.TestForCandidate — test_for resolution in declaration order"
```

---

### Task 4 — `internal/selector`: the static candidate set

**Discharges:** spec §4.1 levels 1 and 2, and §2 (a static selection is *declared
correspondence or imports*, and nothing else). PRD #230 AC5 (membership half), AC7,
and decision 1's "no".

**Files:**
- Create: `internal/selector/static.go`
- Test: `internal/selector/static_test.go`

**Interfaces:**
- Consumes: `Inputs.Adapter`, `Inputs.Changes`, `Inputs.Exists`, `Inputs.ImportDistance`.
- Produces:
  - `type staticCandidate struct { Test string; Level int; Distance int }`
  - `func staticCandidates(in Inputs) []staticCandidate`

- [ ] **Step 1: Write the failing test**

Create `internal/selector/static_test.go`:

```go
package selector

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

func staticFixtureAdapter() *adapter.Adapter {
	return &adapter.Adapter{
		Name:      "typescript",
		Selection: adapter.SelectionStatic,
		Coverage:  adapter.CoverageNone,
		TestGlobs: []string{"**/*.test.ts"},
		TestFor:   []string{"{dir}/{name}.test.ts"},
	}
}

func existsIn(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

// The fixture repository:
//
//	src/auth/token.ts        changed
//	src/auth/token.test.ts   corresponds to it            -> level 1
//	src/auth/session.test.ts imports token.ts, 1 hop      -> level 2
//	src/api/gateway.test.ts  imports token.ts, 3 hops     -> level 2
//	src/auth/unrelated.test.ts  same directory, no import -> NOT a candidate
func staticInputs() Inputs {
	return Inputs{
		Adapter: staticFixtureAdapter(),
		Cfg:     DefaultConfig(),
		Changes: []gitctx.Change{mod("src/auth/token.ts")},
		AllTests: []string{
			"src/api/gateway.test.ts",
			"src/auth/session.test.ts",
			"src/auth/token.test.ts",
			"src/auth/unrelated.test.ts",
		},
		Exists: existsIn(
			"src/api/gateway.test.ts",
			"src/auth/session.test.ts",
			"src/auth/token.test.ts",
			"src/auth/unrelated.test.ts",
		),
		ImportDistance: func(changed string) map[string]int {
			if changed != "src/auth/token.ts" {
				return nil
			}
			return map[string]int{
				"src/auth/session.test.ts": 1,
				"src/api/gateway.test.ts":  3,
			}
		},
	}
}

func levelOf(cands []staticCandidate, test string) (staticCandidate, bool) {
	for _, c := range cands {
		if c.Test == test {
			return c, true
		}
	}
	return staticCandidate{}, false
}

func TestStaticCandidatesFindsCorrespondenceAtLevelOne(t *testing.T) {
	got, ok := levelOf(staticCandidates(staticInputs()), "src/auth/token.test.ts")
	if !ok {
		t.Fatal("the corresponding test is not a candidate")
	}
	if got.Level != 1 {
		t.Errorf("Level = %d, want 1 (test_for correspondence)", got.Level)
	}
}

func TestStaticCandidatesFindsImportersAtLevelTwoWithTheirDistance(t *testing.T) {
	cands := staticCandidates(staticInputs())
	for _, tc := range []struct {
		test string
		dist int
	}{
		{"src/auth/session.test.ts", 1},
		{"src/api/gateway.test.ts", 3},
	} {
		got, ok := levelOf(cands, tc.test)
		if !ok {
			t.Errorf("%s is not a candidate", tc.test)
			continue
		}
		if got.Level != 2 {
			t.Errorf("%s Level = %d, want 2 (import distance)", tc.test, got.Level)
		}
		if got.Distance != tc.dist {
			t.Errorf("%s Distance = %d, want %d", tc.test, got.Distance, tc.dist)
		}
	}
}

// Decision 1. src/auth/unrelated.test.ts shares the changed file's directory — the
// longest possible shared prefix — and neither corresponds nor imports. Path proximity
// ORDERS a selection; it never admits to one, because the level-3 signal alone is the
// `path` baseline spec §7 pre-registers this tier against.
func TestStaticCandidatesNeverAdmitsOnPathProximityAlone(t *testing.T) {
	if got, ok := levelOf(staticCandidates(staticInputs()), "src/auth/unrelated.test.ts"); ok {
		t.Errorf("path proximity admitted %q at level %d; only levels 1 and 2 admit",
			got.Test, got.Level)
	}
}

// PRD #230 AC7: no importscan is a skipped level, not a failure. Level 1 still answers.
func TestStaticCandidatesWithoutImportDistanceStillFindsCorrespondence(t *testing.T) {
	in := staticInputs()
	in.ImportDistance = nil
	cands := staticCandidates(in)
	if len(cands) != 1 || cands[0].Test != "src/auth/token.test.ts" {
		t.Fatalf("candidates = %#v, want only the corresponding test", cands)
	}
}

// The mirror case: an adapter with an importscan and no test_for templates.
func TestStaticCandidatesWithoutTestForStillFindsImporters(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad
	cands := staticCandidates(in)
	if len(cands) != 2 {
		t.Fatalf("candidates = %#v, want the two importers", cands)
	}
	for _, c := range cands {
		if c.Level != 2 {
			t.Errorf("%s Level = %d, want 2", c.Test, c.Level)
		}
	}
}

// A test reachable both ways keeps the more confident level and the import distance it
// was also found at, so the level-2 tiebreak still has a value to use.
func TestStaticCandidatesKeepsTheMoreConfidentLevel(t *testing.T) {
	in := staticInputs()
	in.ImportDistance = func(string) map[string]int {
		return map[string]int{"src/auth/token.test.ts": 4}
	}
	got, ok := levelOf(staticCandidates(in), "src/auth/token.test.ts")
	if !ok {
		t.Fatal("the corresponding test is not a candidate")
	}
	if got.Level != 1 {
		t.Errorf("Level = %d, want 1: correspondence outranks an import path", got.Level)
	}
	if got.Distance != 4 {
		t.Errorf("Distance = %d, want the import distance 4 to be retained", got.Distance)
	}
}

// A deleted file cannot correspond to anything runnable, and a changed TEST file is the
// direct tier's business, not the static tier's.
func TestStaticCandidatesIgnoresDeletedAndTestFileChanges(t *testing.T) {
	in := staticInputs()
	in.Changes = []gitctx.Change{
		deleted("src/auth/token.ts"),
		mod("src/auth/token.test.ts"),
	}
	if cands := staticCandidates(in); len(cands) != 0 {
		t.Errorf("candidates = %#v, want none", cands)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestStaticCandidates -v`
Expected: FAIL — `internal/selector/static_test.go:…: undefined: staticCandidates` and `undefined: staticCandidate` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/selector/static.go`:

```go
package selector

import (
	"sort"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// staticCandidate is one test the static tier found, with the evidence that found it.
// Level is spec §4.1's ranking level: 1 is declared test_for correspondence, 2 is a
// transitive import. There is no level 3 here on purpose — path proximity orders a
// candidate set, it never contributes one (see the plan's decision 1).
type staticCandidate struct {
	Test     string
	Level    int
	Distance int // import hops; 0 when the test was never reached through imports
}

// staticCandidates is the membership half of the static tier: which tests are related to
// the changed set at all, by evidence rather than by neighbourhood.
//
// A missing resolver is a skipped level, never an error. An adapter with no test_for
// templates, or one whose importscan is absent or failed, still gets the level it can
// answer (PRD #230 AC7); an adapter that can answer neither returns nothing, and Select
// turns that into an honest T2 rather than an empty TS.
func staticCandidates(in Inputs) []staticCandidate {
	byTest := map[string]staticCandidate{}

	add := func(test string, level, distance int) {
		prev, seen := byTest[test]
		if !seen {
			byTest[test] = staticCandidate{Test: test, Level: level, Distance: distance}
			return
		}
		// The most confident level wins; the import distance is kept either way so the
		// level-2 tiebreak has a value even for a test found both ways.
		if level < prev.Level {
			prev.Level = level
		}
		if distance > 0 && (prev.Distance == 0 || distance < prev.Distance) {
			prev.Distance = distance
		}
		byTest[test] = prev
	}

	for _, c := range in.Changes {
		if c.Status == gitctx.Deleted {
			continue // nothing corresponds to a file that is gone
		}
		if in.Adapter != nil && in.Adapter.IsTestFile(c.Path) {
			continue // a changed test file is the direct tier's, and it already has it
		}

		if in.Adapter != nil && in.Exists != nil {
			if test, ok := in.Adapter.TestForCandidate(c.Path, in.Exists); ok {
				add(test, 1, 0)
			}
		}
		if in.ImportDistance != nil {
			for test, hops := range in.ImportDistance(c.Path) {
				add(test, 2, hops)
			}
		}
	}

	out := make([]staticCandidate, 0, len(byTest))
	for _, c := range byTest {
		out = append(out, c)
	}
	// Deterministic before ranking: map iteration order must never reach a selection.
	sort.Slice(out, func(i, j int) bool { return out[i].Test < out[j].Test })
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -count=1`
Expected: PASS, 7 new tests; `TestSeededSelectionIsByteIdentical` still passes — nothing calls `staticCandidates` yet.

- [ ] **Step 5: Commit**

```bash
git add internal/selector/static.go internal/selector/static_test.go
git commit -m "M6b: the static candidate set — correspondence and imports admit, proximity does not"
```

---

### Task 5 — `internal/selector`: `rankStatic`, the three-level ordering

**Discharges:** spec §4.1's ranking ("most to least confident: direct correspondence,
import distance, path proximity"). PRD #230 AC5 (ordering half).

**Files:**
- Modify: `internal/selector/static.go`
- Test: `internal/selector/static_test.go`

**Interfaces:**
- Produces: `func rankStatic(cands []staticCandidate, changedFiles []string) []string`

- [ ] **Step 1: Write the failing test**

Append to `internal/selector/static_test.go`:

```go
// PRD #230 AC5: all three levels order correctly against ONE fixture, so the test fails
// if any level is applied out of turn rather than only if a level is missing.
//
//	src/auth/token.test.ts    level 1                       -> first
//	src/auth/session.test.ts  level 2, 1 hop                -> second
//	src/api/gateway.test.ts   level 2, 3 hops               -> third
//	src/auth/far.test.ts      level 2, 3 hops, longer shared
//	                          prefix with src/auth/token.ts -> before gateway
func TestRankStaticOrdersByLevelThenDistanceThenProximity(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/api/gateway.test.ts", Level: 2, Distance: 3},
		{Test: "src/auth/far.test.ts", Level: 2, Distance: 3},
		{Test: "src/auth/session.test.ts", Level: 2, Distance: 1},
		{Test: "src/auth/token.test.ts", Level: 1},
	}
	want := []string{
		"src/auth/token.test.ts",
		"src/auth/session.test.ts",
		"src/auth/far.test.ts",
		"src/api/gateway.test.ts",
	}
	got := rankStatic(cands, []string{"src/auth/token.ts"})
	if len(got) != len(want) {
		t.Fatalf("rankStatic = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rankStatic = %#v, want %#v", got, want)
		}
	}
}

// Two candidates alike on every key still have to come out in one order, every run:
// a selection an agent cannot reproduce is a selection it cannot bisect.
func TestRankStaticIsDeterministicOnAFullTie(t *testing.T) {
	cands := []staticCandidate{
		{Test: "src/auth/b.test.ts", Level: 2, Distance: 1},
		{Test: "src/auth/a.test.ts", Level: 2, Distance: 1},
	}
	for i := 0; i < 5; i++ {
		got := rankStatic(cands, []string{"src/auth/token.ts"})
		if got[0] != "src/auth/a.test.ts" || got[1] != "src/auth/b.test.ts" {
			t.Fatalf("rankStatic = %#v, want lexicographic on a full tie", got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run TestRankStatic -v`
Expected: FAIL — `internal/selector/static_test.go:…: undefined: rankStatic` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Append to `internal/selector/static.go`:

```go
// rankStatic orders the candidate set by spec §4.1's three levels, most confident first:
//
//  1. declared test_for correspondence;
//  2. import distance, shortest transitive path first;
//  3. path proximity, longest shared directory prefix with any changed file first.
//
// Level 3 appears here and NOT in staticCandidates: it is a tiebreak over tests some
// other level already vouched for. Lexicographic order is the final key so a full tie is
// reproducible.
func rankStatic(cands []staticCandidate, changedFiles []string) []string {
	prox := make(map[string]int, len(cands))
	for _, c := range cands {
		prox[c.Test] = maxSharedPrefix(c.Test, changedFiles)
	}
	ranked := append([]staticCandidate(nil), cands...)
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		if a.Level == 2 && a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		if prox[a.Test] != prox[b.Test] {
			return prox[a.Test] > prox[b.Test]
		}
		return a.Test < b.Test
	})
	out := make([]string, 0, len(ranked))
	for _, c := range ranked {
		out = append(out, c.Test)
	}
	return out
}

// maxSharedPrefix is the length, in leading path SEGMENTS, of the longest directory
// prefix this test shares with any changed file. Segments rather than bytes: "src/authz"
// and "src/auth" share seven bytes and no directory, and ranking on the byte count would
// call two unrelated packages neighbours.
func maxSharedPrefix(test string, changedFiles []string) int {
	best := 0
	tp := strings.Split(path.Dir(test), "/")
	for _, f := range changedFiles {
		fp := strings.Split(path.Dir(f), "/")
		n := 0
		for n < len(tp) && n < len(fp) && tp[n] == fp[n] {
			n++
		}
		if n > best {
			best = n
		}
	}
	return best
}
```

Add `"path"` and `"strings"` to the file's imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... -count=1`
Expected: PASS, 2 new tests; the golden is untouched.

- [ ] **Step 5: Commit**

```bash
git add internal/selector/static.go internal/selector/static_test.go
git commit -m "M6b: rankStatic — correspondence, then import distance, then path proximity"
```

---

### Task 6 — `internal/selector`: `Select` resolves `TS`

**Discharges:** spec §4.1 (the tier's position in the resolution order and the gate that
opens it) and §2 (the reason names correspondence or imports, never coverage). PRD #230
AC1 (the documented order), AC4, AC8, AC9c's selector half, and decision 2.

**Files:**
- Modify: `internal/selector/select.go`
- Test: `internal/selector/select_test.go`

**Interfaces:**
- Consumes: `staticCandidates`, `rankStatic`, `adapter.SelectionStatic`, `(*Adapter).Fidelity`.
- Produces: no new exported name. `Select` gains the `TS` outcome; `escalateFull` loses
  its unseeded branch to two new unexported helpers, `mapCannotAnswer` and
  `unanswerableReason`.

- [ ] **Step 1: Write the failing test**

Append to `internal/selector/select_test.go`:

```go
func TestSelectResolvesTS(t *testing.T) {
	in := staticInputs()
	got := Select(in)

	if got.Tier != TierTS {
		t.Fatalf("Tier = %v (%s), want TierTS", got.Tier, got.Reason)
	}
	if got.Tests[0] != "src/auth/token.test.ts" {
		t.Errorf("Tests[0] = %q, want the corresponding test first", got.Tests[0])
	}
	if len(got.Tests) != 3 {
		t.Errorf("Tests = %#v, want the three related tests and not the fourth", got.Tests)
	}
}

// PRD #230 AC8, and the whole point of spec §2's two-axis split: a static selection must
// never describe itself with the words an execution-derived one uses.
func TestSelectTSReasonNamesItsEvidenceAndNotCoverage(t *testing.T) {
	got := Select(staticInputs())
	if contains(got.Reason, "recorded coverage") {
		t.Errorf("Reason = %q, must not contain %q", got.Reason, "recorded coverage")
	}
	if !contains(got.Reason, "correspondence") && !contains(got.Reason, "import") {
		t.Errorf("Reason = %q, want it to name correspondence or imports", got.Reason)
	}
}

// PRD #230 AC9c: a static adapter is never sent to a command that cannot do anything.
func TestSelectStaticReasonsNeverAdviseSeeding(t *testing.T) {
	ad := staticFixtureAdapter()
	none := staticFixtureAdapter()
	none.TestFor = nil // fidelity none: no test_for, no importscan

	for _, tc := range []struct {
		name string
		in   Inputs
	}{
		{"a TS selection", staticInputs()},
		{"a static adapter that found nothing", func() Inputs {
			in := staticInputs()
			in.Adapter = ad
			in.Exists = existsIn()
			in.ImportDistance = nil
			return in
		}()},
		{"a static adapter that can find nothing", func() Inputs {
			in := staticInputs()
			in.Adapter = none
			in.ImportDistance = nil
			return in
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if r := Select(tc.in).Reason; contains(r, "run rtdd seed") {
				t.Errorf("Reason = %q, must not contain %q", r, "run rtdd seed")
			}
		})
	}
}

// Decision 2: fidelity none declares selection: static, so the gate opens, but it can
// produce no candidate and path proximity may not fill the gap. The honest answer is the
// full suite at T2, with a reason naming what the adapter is missing — the same fact its
// rtdd doctor line reports.
func TestSelectFidelityNoneIsAFullSuiteThatSaysWhy(t *testing.T) {
	in := staticInputs()
	ad := staticFixtureAdapter()
	ad.TestFor = nil
	in.Adapter = ad
	in.ImportDistance = nil

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v (%s), want TierT2", got.Tier, got.Reason)
	}
	if len(got.Tests) != len(in.AllTests) {
		t.Errorf("Tests = %#v, want the full suite", got.Tests)
	}
	for _, want := range []string{"typescript", "test_for", "importscan"} {
		if !contains(got.Reason, want) {
			t.Errorf("Reason = %q, want it to mention %q", got.Reason, want)
		}
	}
}

// PRD #230 AC4's other half. A seeded map that covers nothing in the changed set is an
// honest empty; turning it into a speculative static selection would replace a true
// "nothing is related" with a guess.
func TestSelectASeededMapSelectingNothingStaysEmpty(t *testing.T) {
	in := baseInputs()
	in.Changes = []gitctx.Change{mod("src/untouched.py")}
	in.Exists = existsIn("tests/test_auth.py")
	in.ImportDistance = func(string) map[string]int {
		return map[string]int{"tests/test_auth.py::test_login": 1}
	}

	got := Select(in)

	if got.Tier != TierEmpty {
		t.Fatalf("Tier = %v (%s), want TierEmpty: a seeded map answered", got.Tier, got.Reason)
	}
}

// An unseeded COVERAGE adapter keeps today's answer word for word — seeding really is
// the fix for it, and adapters/python.yaml declares no static capability.
func TestSelectAnUnseededCoverageRepoIsUnchanged(t *testing.T) {
	in := baseInputs()
	in.Map = mapstore.New()
	in.Changes = []gitctx.Change{mod("src/auth.py")}
	in.AllTests = []string{"tests/test_auth.py::test_login"}

	got := Select(in)

	if got.Tier != TierT2 {
		t.Fatalf("Tier = %v, want TierT2", got.Tier)
	}
	const want = "the map is unseeded, so no selection is trustworthy: run rtdd seed"
	if got.Reason != want {
		t.Errorf("Reason = %q, want %q", got.Reason, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/selector/... -run 'TestSelectResolvesTS|TestSelectTSReason|TestSelectStaticReasons|TestSelectFidelityNone' -v`
Expected: FAIL — `TestSelectResolvesTS: Tier = T2 (the map is unseeded, so no selection is trustworthy: run rtdd seed), want TierTS`, and `TestSelectStaticReasonsNeverAdviseSeeding/a_TS_selection` failing on the same string.

- [ ] **Step 3: Write minimal implementation**

In `internal/selector/select.go`, restate the documented order in `Select`'s doc comment —
PRD #230 AC1 requires the package's documented sequence to read T2, T1, T0, TS, empty:

```go
// Order of evaluation, and it matters:
//  1. the direct set, BEFORE any map lookup — a test the agent just wrote has no map row,
//     and in v1 it was therefore in no tier and never ran;
//  2. T2 escalations (full-escalate file, drift guard);
//  3. T1 escalations (merge commit, opaque file, import-time-only file, stale row);
//  4. T0;
//  5. TS — static correspondence and imports, reached only when the coverage relation
//     cannot answer: an unseeded map, or an adapter declaring selection: static. TS
//     never overrides a usable map (spec §4.1), so steps 3 and 4 are skipped on this
//     path and a seeded repository never reaches it;
//  6. TierEmpty, reported explicitly with a Reason — or, when the static tier had nothing
//     to offer either, the full suite at T2 with a reason naming what was missing.
```

Remove the unseeded branch from `escalateFull`, leaving its other two checks in place and
in order:

```go
func escalateFull(in Inputs, m *mapstore.Map, cfg Config) (string, bool) {
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
```

In `Select`, gate steps 3–5 on whether the map can answer:

```go
	// (2) T2 escalations that do not depend on the map.
	if reason, escalate := escalateFull(in, m, cfg); escalate {
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: reason,
		}
	}

	// (5) TS. The map cannot answer, so T1 and T0 have nothing to consult and are
	// skipped entirely; static evidence is the only evidence there is.
	if mapCannotAnswer(in, m) {
		cands := staticCandidates(in)
		if ranked := rankStatic(cands, changedFiles); len(ranked) > 0 {
			return Selection{
				Tier:   TierTS,
				Direct: direct,
				Tests:  mergeFirst(direct, ranked),
				Reason: staticReason(cands),
			}
		}
		return Selection{
			Tier:   TierT2,
			Direct: direct,
			Tests:  mergeFirst(direct, in.AllTests),
			Reason: unanswerableReason(in),
		}
	}

	t0 := m.TestsCovering(changedFiles)

	// (3) T1.
	…unchanged…
```

Append the three helpers to `internal/selector/static.go`:

```go
// mapCannotAnswer is the TS gate (spec §4.1: "when the map cannot answer — unseeded, or
// the adapter declares selection: static").
//
// It reads the DECLARATION and the map, never Fidelity(). Fidelity answers "what is the
// best this adapter could ever produce", which is the question rtdd doctor asks; the gate
// asks "can the coverage relation answer this call", and the two differ exactly where it
// matters: a fidelity-none static adapter must still enter the gate, or it falls through
// to the unseeded-map branch and is told to seed a map it can never build.
func mapCannotAnswer(in Inputs, m *mapstore.Map) bool {
	if in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic {
		return true
	}
	return m.Len() == 0
}

// staticReason names the evidence the selection actually rests on. It may never contain
// "recorded coverage": spec §2 splits fidelity into two axes precisely so a static
// selection cannot borrow an execution-derived one's authority, and PRD #230 AC8 makes
// the substring's absence a test.
func staticReason(cands []staticCandidate) string {
	corresponds, imports := false, false
	for _, c := range cands {
		if c.Level == 1 {
			corresponds = true
		} else {
			imports = true
		}
	}
	switch {
	case corresponds && imports:
		return "static selection: tests whose declared test_for correspondence, " +
			"or whose transitive imports, reach the changed set"
	case corresponds:
		return "static selection: tests whose declared test_for correspondence names the changed set"
	default:
		return "static selection: tests that transitively import the changed set, shortest import path first"
	}
}

// unanswerableReason explains a full suite that neither the map nor the static tier could
// narrow. A static adapter is never advised to seed (PRD #230 AC9c): coverage: none means
// no map is ever built, so `rtdd seed` is a command that cannot help it.
func unanswerableReason(in Inputs) string {
	if in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic {
		if in.Adapter.Fidelity() == adapter.FidelityNone {
			return "the " + in.Adapter.Name + " adapter declares selection: static but no " +
				"test_for templates and no importscan command, so nothing narrower than " +
				"the full suite can be derived"
		}
		return "the " + in.Adapter.Name + " adapter selects statically, and neither " +
			"declared correspondence nor imports reach the changed set"
	}
	return "the map is unseeded, so no selection is trustworthy: run rtdd seed"
}
```

Add `"github.com/VocanicZ/rtdd/internal/adapter"` and
`"github.com/VocanicZ/rtdd/internal/mapstore"` to `static.go`'s imports.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/selector/... ./cmd/rtdd/... -count=1`
Expected: PASS. `TestSeededSelectionIsByteIdentical` in particular must pass **unchanged**
— do not run it with `-update`. If it fails, the gate is letting a seeded repository into
the static path and the gate is what to fix.

- [ ] **Step 5: Commit**

```bash
git add internal/selector/select.go internal/selector/static.go internal/selector/select_test.go
git commit -m "M6b: Select resolves TS between T1 and T2, and says what it rests on"
```

---

### Task 7 — `internal/importscan`: run an adapter's declared scanner

**Discharges:** spec §4.1 level 2 and §4.2's `importscan` key ("optional; omitted means
import ranking is skipped"). PRD #230 AC7's impure half.

**Files:**
- Create: `internal/importscan/adapterscan.go`, `internal/importscan/adapterscan_test.go`,
  `internal/importscan/testdata/scan-imports-stub.sh`

**Interfaces:**
- Consumes: `adapter.Importscan{Command, Script}`, `(*Adapter).Expand`.
- Produces:
  - `type AdapterScanner struct{ … }`
  - `func NewAdapterScanner(repoRoot string, a *adapter.Adapter, tests []string) *AdapterScanner`
  - `func (s *AdapterScanner) Distances(changed string) map[string]int`
  - `func (s *AdapterScanner) Err() error`

The wire contract, and the reason for it: the embedded Python scanner already writes a
JSON request to the child's stdin and reads JSON from its stdout, so a declared scanner
uses the same shape with hop counts instead of bare lists — one process per command
invocation, memoised, and a superset of what `Scan` already returns.

```
stdin :  {"root": "/abs/repo", "targets": ["src/a.ts"], "tests": ["src/a.test.ts"]}
stdout:  {"src/a.ts": {"src/a.test.ts": 1}}
```

`script` resolves against `.rtdd/adapters/`, beside the host YAML that declared it. M6b
ships no built-in adapter with an `importscan` (spec §8 puts those in M6d), so resolving
an embedded script is that milestone's problem and is listed under "leaves undone".
A script that is missing, exits non-zero, or writes unparseable JSON degrades selection to
levels 1 and 3 and records the error in `Err`; it never fails the command, exactly as
`Scanner` already does.

- [ ] **Step 1: Write the failing test**

Create `internal/importscan/testdata/scan-imports-stub.sh`:

```bash
#!/usr/bin/env bash
# A fixture scanner: answers from a canned graph, ignoring the request's targets so the
# test asserts the wire contract rather than a scanning algorithm.
cat >/dev/null
echo '{"src/auth/token.ts":{"src/auth/session.test.ts":1,"src/api/gateway.test.ts":3}}'
```

Create `internal/importscan/adapterscan_test.go`:

```go
package importscan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

func scannerRepo(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".rtdd", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		src, err := os.ReadFile(filepath.Join("testdata", script))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, script), src, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scanAdapter(script string) *adapter.Adapter {
	return &adapter.Adapter{
		Name:       "typescript",
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Importscan: &adapter.Importscan{Command: "bash {script}", Script: script},
	}
}

func TestAdapterScannerReturnsHopCounts(t *testing.T) {
	root := scannerRepo(t, "scan-imports-stub.sh")
	s := NewAdapterScanner(root, scanAdapter("scan-imports-stub.sh"),
		[]string{"src/auth/session.test.ts", "src/api/gateway.test.ts"})

	got := s.Distances("src/auth/token.ts")

	if s.Err() != nil {
		t.Fatalf("Err() = %v, want nil", s.Err())
	}
	if got["src/auth/session.test.ts"] != 1 || got["src/api/gateway.test.ts"] != 3 {
		t.Errorf("Distances = %#v, want session:1 gateway:3", got)
	}
}

// A missing script degrades selection to levels 1 and 3. It must not fail the command:
// spec §4.2 makes importscan optional, and a broken one is no worse than an absent one.
func TestAdapterScannerDegradesWhenTheScriptIsMissing(t *testing.T) {
	root := scannerRepo(t, "")
	s := NewAdapterScanner(root, scanAdapter("scan-imports-stub.sh"), []string{"a.test.ts"})

	if got := s.Distances("src/auth/token.ts"); got != nil {
		t.Errorf("Distances = %#v, want nil", got)
	}
	if s.Err() == nil {
		t.Error("Err() = nil, want the failure recorded for the caller to report")
	}
}

// An adapter that declares no scanner is the ordinary case, not an error.
func TestAdapterScannerWithNoImportscanIsInert(t *testing.T) {
	s := NewAdapterScanner(t.TempDir(), &adapter.Adapter{Name: "python"}, nil)
	if got := s.Distances("src/a.py"); got != nil {
		t.Errorf("Distances = %#v, want nil", got)
	}
	if s.Err() != nil {
		t.Errorf("Err() = %v, want nil: an absent scanner is not a failure", s.Err())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/importscan/... -run TestAdapterScanner -v`
Expected: FAIL — `internal/importscan/adapterscan_test.go:…: undefined: NewAdapterScanner` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `internal/importscan/adapterscan.go` with `NewAdapterScanner`, `Distances` and
`Err`. It follows `Scan`'s existing shape: build the `request`, marshal it to the child's
stdin, run `exec.Command` with argv from `a.Expand(a.Importscan.Command, map[string]string{"script": <abs path>})`,
unmarshal `map[string]map[string]int` from stdout. One invocation per scanner, memoised in
a `map[string]map[string]int` keyed by target with a `done` flag, guarded exactly as
`Scanner` guards its own memo. On any error: record it in `s.err` if `s.err == nil`,
return `nil`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/importscan/... -count=1`
Expected: PASS, 3 new tests, existing scanner tests unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/importscan/adapterscan.go internal/importscan/adapterscan_test.go internal/importscan/testdata/scan-imports-stub.sh
git commit -m "M6b: run an adapter's declared importscan, degrade rather than fail"
```

---

### Task 8 — `cmd/rtdd`: wire the two resolvers into `which` and `run`

**Discharges:** spec §4.1 (the tier reaches a user). PRD #230 AC5 and AC7 end to end.

**Files:**
- Create: `cmd/rtdd/static.go`
- Modify: `cmd/rtdd/which.go`, `cmd/rtdd/run.go`
- Test: `cmd/rtdd/static_test.go`

**Interfaces:**
- Produces:
  - `func repoExists(root string) func(rel string) bool`
  - `func adapterImportDistance(root string, ad *adapter.Adapter, tests []string) func(string) map[string]int`

- [ ] **Step 1: Write the failing test**

Create `cmd/rtdd/static_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoExistsAnswersRepoRelativeSlashPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "auth", "token.test.ts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	exists := repoExists(root)

	if !exists("src/auth/token.test.ts") {
		t.Error("exists(src/auth/token.test.ts) = false, want true")
	}
	if exists("src/auth/missing.test.ts") {
		t.Error("exists(src/auth/missing.test.ts) = true, want false")
	}
	// A directory is not a test file; test_for templates name files.
	if exists("src/auth") {
		t.Error("exists(src/auth) = true, want false: a directory is not a candidate")
	}
	// The engine's paths never escape the repository.
	if exists("../outside.ts") {
		t.Error("exists(../outside.ts) = true, want false")
	}
}

// An adapter with no importscan yields a nil resolver, which is how selector.Inputs
// spells "skip level 2" (PRD #230 AC7).
func TestAdapterImportDistanceIsNilWithoutAnImportscan(t *testing.T) {
	if f := adapterImportDistance(t.TempDir(), testAdapterNoScan(), nil); f != nil {
		t.Error("adapterImportDistance = non-nil, want nil without an importscan")
	}
}
```

Add whatever minimal `testAdapterNoScan` helper the file needs (`&adapter.Adapter{Name: "python"}`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/... -run 'TestRepoExists|TestAdapterImportDistance' -v`
Expected: FAIL — `cmd/rtdd/static_test.go:…: undefined: repoExists` (build failure, `[build failed]`).

- [ ] **Step 3: Write minimal implementation**

Create `cmd/rtdd/static.go`. `repoExists` joins the cleaned repo-relative path onto the
root with `filepath.FromSlash`, rejects any path that escapes the root, `os.Lstat`s it and
requires a regular file. `adapterImportDistance` returns `nil` when
`ad == nil || ad.Importscan == nil`, and otherwise an `importscan.NewAdapterScanner`'s
`Distances` method bound to the root, adapter and suite — so one scanner, memoised, per
command invocation.

Then pass both at the two `selector.Inputs` sites, `which.go:89` and `run.go:92`:

```go
		Exists:         repoExists(e.root),
		ImportDistance: adapterImportDistance(e.root, e.ad, allTests),
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -count=1`
Expected: PASS. Every existing `cmd/rtdd` test is unchanged: `adapters/python.yaml`
declares no `test_for` and no `importscan`, so both resolvers are inert on the Python
path — `Exists` is supplied but nothing asks it anything.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/static.go cmd/rtdd/static_test.go cmd/rtdd/which.go cmd/rtdd/run.go
git commit -m "M6b: which and run supply the static tier's two resolvers"
```

---

### Task 9 — `cmd/rtdd`: stop sending static repositories to `rtdd seed`

**Discharges:** spec §5 (`init`'s installed guidance must match what the repository can
do) and §6 (every surface reports fidelity honestly). PRD #230 AC9a, AC9b, AC9c's
command-surface half, and AC9d.

**Files:**
- Modify: `cmd/rtdd/init.go`, `cmd/rtdd/seed.go`
- Test: `cmd/rtdd/init_test.go`, `cmd/rtdd/seed_test.go`

**Interfaces:**
- Produces: `func RenderNextStep(detected []*adapter.Adapter) string` — a pure formatter,
  like every other `Render*` in this package, so the next-step line can be tested without
  running an install.

This is the defect observed on a real repository after PRD #229 closed: `rtdd init`
printed ``Next: run `rtdd seed` once to build .rtdd/map.jsonl`` in a repo whose only
adapter declares `coverage: none`, and `rtdd seed` then ran, found nothing to instrument,
and did not say so.

- [ ] **Step 1: Write the failing test**

Append to `cmd/rtdd/init_test.go`:

```go
func staticTestAdapter(name string) *adapter.Adapter {
	return &adapter.Adapter{
		Name:      name,
		Selection: adapter.SelectionStatic,
		Coverage:  adapter.CoverageNone,
		TestFor:   []string{"{dir}/{name}.test.ts"},
	}
}

func coverageTestAdapter(name string) *adapter.Adapter {
	return &adapter.Adapter{Name: name, Coverage: "sqlite", Seed: "pytest --cov"}
}

func TestRenderNextStepIsFidelityAware(t *testing.T) {
	tests := []struct {
		name     string
		detected []*adapter.Adapter
		wantHas  []string
		wantNot  []string
	}{
		{
			name:     "every adapter is static",
			detected: []*adapter.Adapter{staticTestAdapter("typescript")},
			wantHas:  []string{"rtdd which", "typescript"},
			wantNot:  []string{"run rtdd seed", "map.jsonl"},
		},
		{
			name:     "a coverage adapter still seeds",
			detected: []*adapter.Adapter{coverageTestAdapter("python")},
			wantHas:  []string{"rtdd seed", "map.jsonl"},
		},
		{
			// AC9d: the guidance survives, scoped to the adapters it applies to.
			name: "a mixed repository",
			detected: []*adapter.Adapter{
				coverageTestAdapter("python"),
				staticTestAdapter("typescript"),
			},
			wantHas: []string{"rtdd seed", "python", "typescript"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderNextStep(tc.detected)
			for _, want := range tc.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("RenderNextStep = %q, want it to mention %q", got, want)
				}
			}
			for _, never := range tc.wantNot {
				if strings.Contains(got, never) {
					t.Errorf("RenderNextStep = %q, must not contain %q", got, never)
				}
			}
		})
	}
}
```

Append to `cmd/rtdd/seed_test.go` (create the file if it does not exist):

```go
// PRD #230 AC9b. `coverage: none` means no map is ever built, so a seed run that
// "succeeded" having written nothing is the worst of the three possible answers.
func TestSeedRefusesAStaticAdapter(t *testing.T) {
	root := seedRepoWithHostAdapter(t, staticAdapterYAML)

	code, _, stderr := runInDir(t, root, "seed")

	if code == 0 {
		t.Fatalf("exit = 0, want non-zero: a static adapter records nothing")
	}
	for _, want := range []string{"typescript", "records nothing"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to mention %q", stderr, want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".rtdd", "map.jsonl")); err == nil {
		t.Error(".rtdd/map.jsonl was written by a refused seed")
	}
}
```

Reuse this package's existing repo-fixture and `runInDir`-shaped helpers rather than
inventing new ones; `seedRepoWithHostAdapter` writes `staticAdapterYAML` to
`.rtdd/adapters/typescript.yaml` in a real git repo, the way the M6a host-adapter tests
already do.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/rtdd/... -run 'TestRenderNextStep|TestSeedRefusesAStaticAdapter' -v`
Expected: FAIL — `undefined: RenderNextStep` (build failure), and once that compiles,
`TestSeedRefusesAStaticAdapter: exit = 0, want non-zero: a static adapter records nothing`.

- [ ] **Step 3: Write minimal implementation**

In `cmd/rtdd/init.go`, replace the unconditional line at `init.go:136` with
`fmt.Fprint(stdout, RenderNextStep(detected))`, and add the formatter:

```go
// RenderNextStep is the line `rtdd init` closes with. It is fidelity-aware because the
// old unconditional version sent every repository to `rtdd seed`, and a repository whose
// adapters all declare coverage: none can never build a map — the command it was sent to
// cannot do anything (PRD #230 AC9a). In a mixed repository the seed guidance survives,
// scoped by name to the adapters it applies to (AC9d): one static adapter must not
// silence advice the Python half of the repo still needs.
func RenderNextStep(detected []*adapter.Adapter) string {
	var static, coverage []string
	for _, a := range detected {
		if a.Selection == adapter.SelectionStatic {
			static = append(static, a.Name)
		} else {
			coverage = append(coverage, a.Name)
		}
	}
	switch {
	case len(coverage) == 0 && len(static) > 0:
		return fmt.Sprintf("\nNext: run `rtdd which` to see what a change selects. "+
			"The %s adapter selects statically, so there is no map to build and "+
			"`rtdd seed` does not apply.\n", strings.Join(static, ", "))
	case len(static) > 0:
		return fmt.Sprintf("\nNext: run `rtdd seed` once to build .rtdd/map.jsonl for the "+
			"%s adapter, then commit it. The %s adapter selects statically and records "+
			"nothing, so seeding does not cover it.\n",
			strings.Join(coverage, ", "), strings.Join(static, ", "))
	default:
		return "\nNext: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it.\n"
	}
}
```

The `default` branch is byte-identical to the old line, so every existing `init` test on a
Python repository passes unchanged.

In `cmd/rtdd/seed.go`, refuse immediately after `detectAdapter`:

```go
	if ad.Selection == adapter.SelectionStatic {
		fmt.Fprintf(os.Stderr, "rtdd seed: the %s adapter declares selection: static and "+
			"records nothing, so there is no map to build; run `rtdd which` to see what a "+
			"change selects\n", ad.Name)
		return 2
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/rtdd/... -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/rtdd/init.go cmd/rtdd/seed.go cmd/rtdd/init_test.go cmd/rtdd/seed_test.go
git commit -m "M6b: init's next step is fidelity-aware and seed refuses a static adapter"
```

---

### Task 10 — record the contract additions

**Discharges:** spec §4.1 (the tier's names, as the sibling PRDs will bind to them) under
the repo's own rule (`00-interfaces.md` → "Rule for future additions"): every name a later
milestone depends on is recorded before it is depended on.

**Files:**
- Create: `internal/contract/interfaces_m6b_test.go`
- Modify: `docs/plans/00-interfaces.md`

**Interfaces:**
- Consumes: `readRepoFile` from `internal/contract`.
- Produces: no engine name. It records the ones Tasks 1–9 produced, so M6c and M6d bind to
  a written contract rather than to whatever the implementation happened to call them.

- [ ] **Step 1: Write the failing test**

Create `internal/contract/interfaces_m6b_test.go`:

```go
package contract

import (
	"strings"
	"testing"
)

// M6c binds to these names, and a name that lives only in the implementation is a name
// the next milestone has to reverse-engineer. The rule at the end of 00-interfaces.md
// makes recording them part of the milestone, not a follow-up.
func TestInterfacesRecordsTheM6bAmendments(t *testing.T) {
	src := readRepoFile(t, "docs/plans/00-interfaces.md")

	const heading = "## M6b amendments"
	if !strings.Contains(src, heading) {
		t.Fatalf("docs/plans/00-interfaces.md has no %q section", heading)
	}
	tail := src[strings.Index(src, heading):]
	for _, want := range []string{
		"TierTS",
		"Exists func(rel string) bool",
		"ImportDistance func(changed string) map[string]int",
		"TestForCandidate",
		"NewAdapterScanner",
		"repoExists",
		"adapterImportDistance",
		"RenderNextStep",
	} {
		if !strings.Contains(tail, want) {
			t.Errorf("the M6b amendments do not record %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contract/... -run TestInterfacesRecordsTheM6bAmendments -v`
Expected: FAIL — `docs/plans/00-interfaces.md has no "## M6b amendments" section`.

- [ ] **Step 3: Append the M6b amendments**

Append to `docs/plans/00-interfaces.md`:

````markdown
---

## M6b amendments — the `TS` static selection tier

Added by [`06-m6b-static-tier.md`](06-m6b-static-tier.md). These are part of the contract.

### internal/selector

```go
// TierTS is the static tier (spec §4.1), ordered between TierT1 and TierT2. Its String()
// is "TS". The documented resolution order in select.go is now: T2 escalations, T1
// escalations, T0, TS, empty.
//
// TS is attempted when, and only when, the coverage relation cannot answer:
//
//     (in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic) || map.Len() == 0
//
// A SEEDED map that selects nothing stays an explicit TierEmpty at T0. A static adapter
// that can produce no candidate returns the full suite at T2 with a reason naming what it
// lacks — never an empty TS, and never advice to run `rtdd seed`.

type Inputs struct {
    // … M1a fields …

    // Exists reports whether the repository has this repo-relative path; it resolves
    // test_for templates without the selector touching a filesystem. nil skips level 1.
    Exists func(rel string) bool

    // ImportDistance maps a changed file to the tests that transitively import it,
    // valued by the shortest number of hops. nil skips level 2 (PRD #230 AC7).
    ImportDistance func(changed string) map[string]int
}
```

Ranking inside `TS`, most to least confident: declared `test_for` correspondence; import
distance, shortest first; longest shared directory prefix. **The third level orders a
selection and never contributes to one** — proximity alone is the `path` baseline spec §7
pre-registers this tier against.

### internal/adapter

```go
// TestForCandidate resolves this adapter's test_for templates against one changed source
// file and returns the first template naming a file the repository actually has.
// Templates are tried in declaration order; exists is injected so every caller up to
// selector.Select stays pure. {dir} and {name} are the only placeholders, already
// enforced by validateTemplates at load time.
func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool)
```

### internal/importscan

```go
// AdapterScanner runs an adapter's declared importscan script and returns hop counts.
// The wire contract matches the embedded Python scanner's: a JSON request on the child's
// stdin, a JSON answer on its stdout, with distances instead of bare lists.
//
//     stdin :  {"root": "...", "targets": ["src/a.ts"], "tests": ["src/a.test.ts"]}
//     stdout:  {"src/a.ts": {"src/a.test.ts": 1}}
//
// `script` resolves against .rtdd/adapters/. A missing, failing or unparseable scanner
// degrades selection to levels 1 and 3 and is reported through Err; it never fails the
// command.
func NewAdapterScanner(repoRoot string, a *adapter.Adapter, tests []string) *AdapterScanner
func (s *AdapterScanner) Distances(changed string) map[string]int
func (s *AdapterScanner) Err() error
```

### cmd/rtdd — internal to `main`

```go
// repoExists and adapterImportDistance are the impure halves of the static tier; they are
// built in cmd/ and injected into selector.Inputs by both `which` and `run`.
func repoExists(root string) func(rel string) bool
func adapterImportDistance(root string, ad *adapter.Adapter, tests []string) func(string) map[string]int

// RenderNextStep is `rtdd init`'s closing line, fidelity-aware: a repository whose
// detected adapters all declare selection: static is pointed at `rtdd which`, a mixed
// repository keeps the seed guidance scoped by adapter name, and a coverage-only
// repository gets the pre-M6b line byte for byte.
func RenderNextStep(detected []*adapter.Adapter) string
```

`rtdd seed` against a `selection: static` adapter exits **2** naming the adapter and
saying it records nothing. It is a configuration error, not a run failure: a seed that
silently succeeds having built no map is what PRD #230 AC9b closes.
````

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/contract/... -count=1`
Expected: PASS.

- [ ] **Step 5: Run the gate and commit**

Run: `scripts/ci-local.sh`
Expected: exit 0.

```bash
git add docs/plans/00-interfaces.md internal/contract/interfaces_m6b_test.go
git commit -m "M6b: record the TS tier's contract additions"
```

---

## Definition of Done

**Behaviour**

- [ ] `internal/selector` has `TierTS`, ordered `TierT1 < TierTS < TierT2`, `String()` is
      `"TS"`, and `TierEmpty` is still the zero value.
- [ ] `select.go`'s documented order reads: T2 escalations, T1 escalations, T0, TS, empty.
- [ ] `TS` is attempted when the adapter declares `selection: static` or the map is
      unseeded, and never otherwise.
- [ ] A seeded map that selects zero tests is an explicit `TierEmpty`, not a `TS`.
- [ ] Ranking is three-level — correspondence, then shortest import distance, then longest
      shared directory prefix — and one fixture asserts all three order correctly.
- [ ] Path proximity never admits a test to the selection; a same-directory test with no
      correspondence and no import is absent, and a test asserts it.
- [ ] `test_for` templates expand `{dir}` and `{name}`, are tried in declaration order,
      and the first resolving to an **existing** file wins.
- [ ] An adapter with no `importscan` still gets levels 1 and 3; a scanner that is
      missing, fails or writes unparseable JSON degrades rather than failing the command.
- [ ] A `TS` reason names correspondence or imports and does not contain the substring
      `recorded coverage`; a test asserts the absence.
- [ ] No reason produced for a `selection: static` adapter contains the substring
      `run rtdd seed`; a test asserts the absence.
- [ ] A `Fidelity() == none` adapter returns the full suite at T2 with a reason naming the
      missing `test_for` and `importscan`.
- [ ] `rtdd init` points a static-only repository at `rtdd which`, keeps the seed line
      byte-identical for a coverage-only repository, and scopes it by adapter name in a
      mixed one.
- [ ] `rtdd seed` against a `selection: static` adapter exits 2, names the adapter, says
      it records nothing, and writes no map.

**The regressions this milestone had to avoid**

- [ ] `TestSeededSelectionIsByteIdentical` passes against the golden generated in Task 2,
      which was never regenerated afterwards.
- [ ] `rtdd which`, `rtdd run` and `rtdd status` produce byte-identical output to before
      this milestone in a seeded Python repository.
- [ ] An unseeded Python repository still reports T2 with
      `the map is unseeded, so no selection is trustworthy: run rtdd seed`, word for word.
- [ ] `adapters/python.yaml` is still byte-frozen; `internal/contract`'s guard passes.
- [ ] No existing test was deleted, skipped or weakened.
- [ ] `internal/selector` still makes no filesystem, subprocess or git call.

**Contract**

- [ ] `docs/plans/00-interfaces.md` records every addition; no name in the implementation
      diverges from it.

**Hygiene**

- [ ] `scripts/ci-local.sh` exits 0.
- [ ] `go.mod` still lists exactly `gopkg.in/yaml.v3` and `modernc.org/sqlite`.
- [ ] Every task's commit is separate and its test was seen to fail before its
      implementation was written.

## What this plan deliberately leaves undone

Everything below is real work the multi-language design wants; none of it belongs to this
PRD (#230), and no task above may start it. Each line names the milestone from spec §8 and
the sibling or later PRD that owns it — a scope this plan may not silently absorb.

- **The runner and `report: junit-xml` — spec §4.3, milestone M6c, a SIBLING PRD.** `TS`
  produces a ranked list of test files and stops there. Nothing in this plan executes a
  static suite, expands a `{report}` placeholder, parses a JUnit XML document, round-trips
  an id through `id_template`, or maps a non-Python runner's exit codes. An adapter
  declaring `junit-xml` still fails at run time on the existing unsupported-report path,
  and that is the intended state at the end of this PRD. `internal/runner` and
  `internal/report` are not opened by any task here.
- **The shipped non-Python adapters and polyglot selection — spec §8 and §4.4, milestone
  M6d, a LATER PRD.** `adapters/` gains no file: no Vitest, Jest, Go, Rust, Java/Kotlin,
  Ruby, C# or PHP declaration. Every static adapter exercised above is a test fixture or a
  host `.rtdd/adapters/*.yaml`. `Detect` still errors on ≥2 adapters, so a repository
  reaches the static tier with exactly one adapter; the adapter tag on rows, selections and
  subset invocations is that PRD's. Resolving an `importscan` `script` shipped *inside* the
  embedded built-in FS is deferred with them — Task 7 resolves against `.rtdd/adapters/`
  only, because until M6d no built-in declares one.
- **The measured evidence table — spec §7, milestone M6e, a later PRD.** The
  static-vs-coverage-vs-`path`-vs-T2 comparison over the flask and httpie replay corpus,
  with `p50`/`p90`/`worst`, is not run, not published and not cited here. **Nothing in this
  plan or the code it produces may claim the static tier beats the `path` baseline.** That
  is a measurement, spec §7 pre-registers it, and it has not been made. Decision 1 above
  is an argument about what counts as evidence, not a claim about how well the tier scores.
- **The remaining §6 honesty surfaces — milestone M6e, a later PRD.** `rtdd which` and
  `rtdd run` gain no `tier: TS (static)` suffix, `--json` gains no `selection_fidelity`
  field, the `warnings` array gains no static-tier caveat, and `PROTOCOL.md` →
  `SKILL.md`/`AGENTS.md`/`.mdc` regeneration is untouched. `TS` reaches those surfaces here
  only through `Tier.String()` and the `Reason` string, which is enough for a human to read
  and not yet the structured fidelity contract §6 specifies. `rtdd doctor`'s fidelity block
  already landed in M6a and is not edited.
- **Per-test coverage outside Python.** Audit A5 stands and spec §3 keeps it a non-goal in
  this PRD and every other in the chain: no Vitest coverage provider, no injected
  `TestMain`, no `ClearCounters()` codegen. The static tier exists *because* that route is
  closed, and it never claims to be equivalent to the one it replaces.
