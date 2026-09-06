# RTDD M6d — The Shipped Adapter Set and Polyglot Detection

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the common languages work out of the box, and stop a polyglot repository being an error. After this milestone `adapters/` holds ten declarations instead of one, `adapter.Detect` returns a *set*, a TypeScript service with a Java module gets a selection from both toolchains, and every map row, selection and subset invocation says which adapter produced it. The contract, the `TS` tier and the `junit-xml` parser are not built here — sibling PRDs #229, #230 and #231 already shipped them; this milestone populates them and removes the one-adapter arity rule that keeps them single-language.

**Architecture:** `adapters/` gains nine `selection: static`, `coverage: none`, `report: junit-xml` declarations. `internal/adapter` loses `Detect`'s arity check (`DetectAll` already exists and already walks once) and gains two declarative splicing keys — `test_flag` and `test_join` — because `{tests}` splices bare argv tokens and four of the nine runners take a repeated flag or a joined string instead. `internal/runner` gains `report_cmd`, one post-run command that converts a runner's captured stdout into the declared `report_path`, because `go test -json` emits JSON on stdout and `go-junit-report` reads stdin, and the engine builds argv and never a shell. `internal/mapstore.Row` gains an `a` key, `.rtdd/meta.json` gains an `adapters` list beside its legacy singular `adapter`, and `cmd/rtdd` loops the detected set for `which`, `run`, `seed` and `doctor`, folding per-adapter exit codes by a stated precedence. D8 holds throughout: the YAML declares, Go executes.

**Tech Stack:** Go 1.24+, gopkg.in/yaml.v3, stdlib testing. No new dependency. Fixture repositories are committed files, not generated ones; the two new JUnit captures come from `scripts/capture-junit-fixtures.sh`, which is not part of the CI gate.

**Spec:** `docs/specs/2026-09-05-multi-language.md` §4.4 (polyglot repositories) and §8 (the shipped adapter set). Each task below names the section it discharges on its `**Discharges:**` line. Two tasks also touch §4.2 and §4.3 — the contract keys this milestone discovers it needs to express its own nine adapters — and say so. Everything else in that spec is a sibling's or a later PRD's and is listed at the end of this document.

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

M6d-specific:

- **`adapters/python.yaml` is byte-frozen, and its digest is not to be edited.** Spec §4.1
  makes "a seeded Python repository's selection is byte-identical to today" a regression
  *requirement*, and the only adapter that has ever produced a map is the cheapest thing to
  hold still. **No `### Task N` below may list that file under **Files:**,** and
  `internal/contract/adapter_freeze_test.go`'s `pythonAdapterSHA256` constant is not to be
  changed by any task here — not to "add the new keys", not to "normalise formatting". Every
  key this plan adds to the contract is optional, so the frozen file stays valid unedited.
  Task 12 adds the guard that fails if a later task forgets.
- **No new engine branch per language.** The nine adapters differ only in YAML. A task that
  finds itself writing `if a.Name == "gradle"` has found a missing declarative key, and the
  key is the fix (decision 8). D8 is the reason: the engine may not learn a language.
- **The `TS` tier, the `junit-xml` parser and the contract validator are consumed, not
  rewritten.** `internal/selector`, `internal/report/junit.go` and `internal/adapter`'s
  `validate()` are opened only to add what the new keys need. No task re-derives ranking,
  re-parses XML or re-validates a v2 field #229 already validates.
- **Exit codes are unchanged** (`00-interfaces.md`): 0 success, 1 a test failed, 2 a
  configuration error, 3 a fatal environment error. Polyglot execution does not add a code;
  it adds a rule for folding several runs into one (decision 5).
- **Detection reads file names, never file contents.** `detect` stays a glob list.
  A `package.json` content probe would make detection a parser, which is the D8 line
  (decision 1).

### The twelve under-determined decisions, resolved here

PRD #232 leaves seven things open, and writing the nine adapters against the shipped
contract turns up five more. Each is decided here so no implementer has to guess, and each
is pinned to a task with a test.

**1. `vitest` and `jest` do NOT list `package.json` as a `detect` marker; they detect on
their own runner config files, and `detect` does not grow** (Task 4, Task 10). Both
ecosystems are announced by `package.json`, so listing it in both makes every JavaScript
repository detect two adapters and makes PRD #232 AC10's fixture — one `package.json` plus
one `pom.xml` — detect three. The discriminating markers are the runner's own config files,
which are ordinary files an ordinary glob already matches:

| adapter | `detect` markers |
|---|---|
| `vitest` | `vitest.config.js`, `vitest.config.ts`, `vitest.config.mjs`, `vitest.config.mts`, `vitest.config.cjs`, `vitest.workspace.ts` |
| `jest` | `jest.config.js`, `jest.config.ts`, `jest.config.mjs`, `jest.config.cjs`, `jest.config.json` |

So `detect` needs no new expressive power, and a `package.json` **content** probe is
refused: it would make detection parse a file rather than name one, which is the D8 line
this design draws in §4.2, and it would give `detect` a second grammar that every host
adapter author then has to learn. The cost is stated rather than hidden: a repository that
configures Vitest inside `vite.config.ts`, or Jest under a `"jest"` key in `package.json`,
detects **nothing** and gets the §5 no-adapter refusal naming JavaScript/TypeScript. That is
the honest failure — RTDD says it cannot serve the repo — and spec §4.5 is the fix the
message points at: six lines of `.rtdd/adapters/jest.yaml` with the marker that repo does
have. Under-detection is recoverable by the user; mis-detection writes a map from the wrong
suite and is not.

**2. PRD #232 AC2's completeness table has ten rows, but only nine of them are checked for
the `junit-xml` keys** (Task 5). The criterion says the table covers "all ten adapters
including python", and read literally that cannot hold: `adapters/python.yaml` declares no
`report_path`, no `id_template` and no `test_for`, and it is **byte-frozen**, so satisfying
the literal reading means breaking the digest spec §4.1 exists to protect. The resolution:
the table has ten rows; the universally-required keys — `name`, `detect`, `subset`, `list`,
`test_globs`, `source_globs`, `opaque`, `full_escalate` — are asserted for all ten; and
`report_path`, `id_template` and at least one `test_for` are asserted for exactly the
adapters that declare `report: junit-xml`, which is the nine. Python is in the table, and
what the table asserts of it is what a coverage adapter can be held to. The freeze is the
stated reason, in the test's own comment, so the next reader does not "fix" the asymmetry.

**3. A map row carries its adapter in a new `"a"` JSON key, omitted when empty; an
**untagged** row is served only to the adapter `.rtdd/meta.json` names, and to nothing else**
(Task 7). `mapstore.Row` gains `A string \`json:"a,omitempty"\``. `omitempty` is not
cosmetic: it keeps every byte of an existing `.rtdd/map.jsonl` valid and unrewritten, so
upgrading RTDD does not produce a whole-file diff in every host repository that has seeded.
An untagged row read from a pre-PRD `map.jsonl` was written by whichever adapter that repo
had when it seeded, and the file that records which one is `.rtdd/meta.json`'s existing
singular `adapter` field — so an untagged row is treated as tagged with that name. If
`meta.json` names none (a hand-made map, a corrupted meta), the row is **ignored for
selection** and preserved on write: serving it to an arbitrary adapter is exactly the
"a row written by one adapter is never served to another" violation AC6 forbids, and
deleting it would throw away a map the user paid a full seed for. Pinned by a test over a
committed pre-PRD `map.jsonl` fixture with no `a` key on any row.

**4. `.rtdd/meta.json` keeps its singular `adapter` and gains a plural `adapters`** (Task
8). Turning `adapter` into a list would make every meta.json written before this PRD
unreadable by the new binary and every meta.json written after it unreadable by the old one,
for no gain: the singular field is already the answer to decision 3's untagged-row question
and deleting it deletes that answer. So `Meta` becomes `{v, adapter, adapters, seeded_at,
cycles}`; `adapters` is the full detected set at seed time, sorted; `adapter` continues to
hold the **coverage** adapter that produced the map, and is empty in a repository whose
adapters are all static. `rtdd doctor` and `rtdd init` read `adapters` and fall back to
`adapter` when `adapters` is absent, which is exactly the pre-PRD file.

**5. Across adapters the exit code is the WORST one any adapter produced**, by this
precedence (Task 9):

| code | meaning | wins over |
|---|---|---|
| 3 | a fatal environment error in any adapter (`.coverage` unreadable, `no-sysmon-context`) | 2, 1, 0 |
| 2 | a configuration error in any adapter (unparseable adapter, mapped `bad-selector` exit) | 1, 0 |
| 1 | a test failed or errored under any adapter | 0 |
| 0 | every adapter's selection ran and nothing failed | — |

The rule is "worst wins", and the ordering above is not arbitrary: 3 and 2 say RTDD's
answer is untrustworthy, 1 says the answer is trustworthy and is *bad news*, and collapsing
either of the first two into 1 would tell an agent a test failed when in fact nothing ran.
The per-adapter detail is printed either way — AC7 requires the failure to be reported per
adapter — so the code is a summary of a report the caller can already read, never a
substitute for it. One adapter exiting 1 never suppresses another adapter's selection: each
runs to completion and the codes are folded at the end.

**6. `rtdd seed` in a mixed repository seeds the coverage adapters, names the static ones,
and exits 0** (Task 11). It does not refuse. `cmd/rtdd/staticadvice.go` already partitions
the detected set into `static` and `coverage` and already exists to print both halves of the
sentence — the existing refusal is for a repository where the partition's coverage half is
**empty**, which is a different question. So: coverage half non-empty → seed exactly those
adapters, print one line naming the static ones and why seeding cannot help them, exit 0;
coverage half empty → the existing exit-2 refusal, unchanged. Refusing the mixed case would
strand the map the Python half of a polyglot repository genuinely needs, which is the
defect, not the safeguard.

**7. Three of the nine render a FILE-granular id; six render a case-granular one** (Task 4,
Task 6). Spec §4.3 names Vitest and RSpec as having no single-token selector for one case;
Jest joins them once its classname is reconfigured (decision 10). A file-granular id is
shared by every case in the file and folds worst-status-wins, which `report.FoldOutcome`
already does — so an id is green only when every case behind it passed.

| adapter | granularity | `id_template` | why | round-trip proven by |
|---|---|---|---|---|
| `vitest` | file | `{classname}` | vitest writes the test file path into `classname=`; its CLI selects files positionally, and `-t` is a filter that cannot be repeated | `TestShippedIDTemplatesRoundTrip/vitest` over `testdata/junit/vitest.xml` |
| `jest` | file | `{classname}` | with `JEST_JUNIT_CLASSNAME={filepath}` the classname is the file; jest positionals are path regexes and repeat | `TestShippedIDTemplatesRoundTrip/jest` over `testdata/junit/jest-filepath.xml` |
| `rspec` | file | `{file}` | rspec emits `file=`, and `rspec a_spec.rb b_spec.rb` repeats; a per-example selector is `path:line`, which no JUnit attribute carries | `TestShippedIDTemplatesRoundTrip/rspec` over `testdata/junit/rspec.xml` |
| `go` | case | `{name}` | `-run` takes a regex over test names; `classname` is the package and is not part of the selector | `TestShippedIDTemplatesRoundTrip/go` over `testdata/junit/go-junit-report.xml` |
| `cargo-nextest` | case | `{name}` | nextest positionals are substring filters over test names and repeat | `TestShippedIDTemplatesRoundTrip/cargo-nextest` over `testdata/junit/nextest.xml` |
| `maven` | case | `{classname}#{name}` | Surefire's `-Dtest=Class#method` is exactly this shape | `TestShippedIDTemplatesRoundTrip/maven` over `testdata/junit/surefire.xml` |
| `gradle` | case | `{classname}.{name}` | Gradle's `--tests` takes a dotted FQCN plus method | `TestShippedIDTemplatesRoundTrip/gradle` over `testdata/junit/surefire.xml` |
| `dotnet` | case | `{classname}.{name}` | `--filter` treats a bare value as `FullyQualifiedName~value` | `TestShippedIDTemplatesRoundTrip/dotnet` over `testdata/junit/dotnet.xml` |
| `phpunit` | case | `{classname}::{name}` | PHPUnit's `--filter 'CalcTest::testAdds'` is exactly this shape; its `file=` is the capture machine's absolute path and is useless as a selector | `TestShippedIDTemplatesRoundTrip/phpunit` over `testdata/junit/phpunit.xml` |

`gradle` and `dotnet` use `.` as the separator, and `{classname}` itself contains dots, so
`report.ParseID` splits them at the *first* dot rather than the last. That is recorded here
as harmless and asserted as such: only the **rendered** id is ever spliced into `subset`,
and render→parse→render is stable regardless of where the split lands. Nothing downstream
consumes the split halves.

**8. `{tests}` gains two optional, declarative shapes: `test_flag` and `test_join`** (Task
2). `ExpandTests` splices each id as its own bare argv element, which is right for pytest,
vitest, jest, rspec and nextest and wrong for the other four: Gradle needs `--tests` before
*each* id, and Surefire, PHPUnit and `dotnet test` each take **one** argument holding every
id joined by a separator (`,`, `|`, `|`). Four ecosystems is not an exception, so the fix is
declarative rather than a branch per runner:

| key | meaning | declared by |
|---|---|---|
| `test_flag: "--tests"` | emit `<flag> <id>` for each id at the `{tests}` position | `gradle` |
| `test_join: ","` | join every id into ONE argv token, substituted wherever `{tests}` appears inside a token | `maven` (`,`), `dotnet` (`\|`), `phpunit` (`\|`), `go` (`\|`) |

Declaring both is a load-time error (exit 2): they are two answers to one question. When
`test_join` is set, `{tests}` may appear *inside* a token (`-Dtest={tests}`), which is what
lets Surefire's single argument be expressed at all; when it is not, the token must be
exactly `{tests}`, which is today's rule unchanged. Chunking is unaffected — `Chunk` already
splits on the argv byte budget, and a joined chunk is one token whose length it already
measures.

**9. `report_cmd` closes the one pipeline the argv-only engine cannot express** (Task 3).
`go test -json` writes JSON to **stdout** and `go-junit-report` reads **stdin**; the engine
builds argv and never hands a shell a string (`internal/adapter/expand.go`'s header says
why, and the reason — an id containing a space, a `|` and brackets — has not gone away). A
`sh -c` template does not rescue it, because command templates are whitespace-split before
substitution, so the shell string cannot survive as one argv element. The alternatives were
weighed: `gotestsum` is a single process that writes JUnit directly, but PRD #232 AC3 names
`go-junit-report` as the Go adapter's prerequisite, and swapping the tool to dodge a
contract gap hides the gap. So the contract gains one optional key:

```yaml
report_cmd: "go-junit-report -parser gojson -in {log} -out {report}"
```

The runner already captures each chunk's combined output; when `report_cmd` is declared it
writes that capture to `{log}` and runs the command **after** the subset invocation and
**before** reading `report_path`. A non-zero exit from `report_cmd` is a fatal error naming
the adapter, never a silent empty report. `go` is the only shipped adapter that declares it;
it is a key rather than a Go branch for the same reason as decision 8.

**10. `jest`'s classname must be reconfigured, and the committed `jest.xml` fixture is not
evidence for the shipped adapter** (Task 4, Task 6). jest-junit's default `classNameTemplate`
writes the test's own full name into `classname=`, and `name=` is a copy of it — the
captured `internal/report/testdata/junit/jest.xml` shows both attributes holding the identical
string, and neither is a path. Jest's positional arguments are regexes over **file paths**, so
no id derived from the default capture can be spliced back into `subset`: the round trip fails
on the runner, silently, by selecting nothing and reporting green. The adapter therefore
declares `env: {JEST_JUNIT_CLASSNAME: "{filepath}", JEST_JUNIT_OUTPUT_NAME: "junit.xml"}`,
which jest-junit reads as its own template vocabulary — `env` values are passed through
verbatim and are never run through `Expand`, so the braces reach jest-junit unsubstituted.
Task 6 captures a **second** fixture, `jest-filepath.xml`, under exactly those variables and
adds its provenance row; the existing `jest.xml` stays, unedited, as the default-config
capture the parser is tested against. A file path spliced back as a regex over-matches on
`.` — `src/a.test.js` also matches `src/aXtest.js` — which is over-selection, and
over-selection is safe for selection while under-selection is not (`internal/adapter/expand.go`,
`cmd/rtdd/rows.go`).

**11. `requires.bin` is a PATH lookup, so the five declare the executable that must
resolve and name the package in `reason`** (Task 4). `Adapter.Unmet` calls `lookPath`
(`internal/adapter/requires.go`); an entry naming a library that installs no executable —
`jest-junit`, `rspec_junit_formatter`, `JUnitTestLogger` — is **permanently unmet** on every
machine, which turns `rtdd doctor`'s findings into noise and trains the reader to ignore
them. PRD #232 AC3's five still declare a non-empty `requires`; what each declares is the
binary whose absence is checkable, with the package named in the reason a human reads:

| adapter | `requires.bin` | the `reason` names |
|---|---|---|
| `go` | `go-junit-report` | itself — it is a real PATH binary |
| `cargo-nextest` | `cargo-nextest` | itself — it is a real PATH binary |
| `jest` | `npx` | `jest-junit`, which must be a devDependency |
| `rspec` | `rspec` | `rspec_junit_formatter`, which must be in the Gemfile |
| `dotnet` | `dotnet` | `JUnitTestLogger`, which must be a package reference |

**12. For a `report: junit-xml` adapter, `runner.List` reads its ids from `report_path`,
and each of the nine declares `list` as its runner's full-suite invocation** (Task 4, Task
9). No runner's enumeration command emits ids in the adapter's own id namespace: `vitest
list` prints case names while the vitest id is a file path, `jest --listTests` prints
absolute paths while the id is repo-relative, and Maven and Gradle have no enumeration
command at all. The one place every runner already agrees with its adapter is the report,
because that is where `id_template` renders. So `List` dispatches on `Report` exactly as
`readOutcomes` does: `pytest-reportlog` keeps stdout-line parsing unchanged, `junit-xml`
runs the list command and reads `report_path`. The cost is stated, not hidden — for a
static adapter a T2 enumeration is a full suite run — and it is the run T2 was about to
make anyway, so `cmd/rtdd/run.go` must **use the outcomes that run produced** rather than
re-running the same suite as a subset. Task 9 owns that consequence.

## What is already true in the tree

Read these before writing code; several tasks are much smaller than they look because the
mechanism exists and only its caller is single-adapter.

- **`adapter.DetectAll(repoRoot, adapters) ([]*Adapter, error)` already exists** and already
  walks the tree once, skipping `node_modules`, `.venv`, `.git` and friends
  (`internal/adapter/detect.go`). `Detect` is a thin arity check *over* it — one match is
  returned, zero and "two or more" are errors. Task 1 deletes the arity check and its callers'
  assumption, not the walk.
- **Contract v2 parses and validates already** (#229): `selection`, `coverage: none`,
  `report_path`, `id_template`, `test_for`, `importscan` and `requires` all load, and
  `validate()` rejects every illegal combination of them with its own message. A shipped
  adapter that declares them needs no validator change. Out of scope here.
- **The `TS` tier exists** (#230): `selector.TierTS` sits between T1 and T2,
  `Adapter.TestForCandidate` resolves `test_for` against the filesystem, and ranking is
  correspondence → import distance → path proximity. `internal/selector` is opened by no task
  in this plan. Out of scope here.
- **One `junit-xml` parser exists** (#231): `report.ReadJUnitReport(rp, idTemplate)`,
  `report.RenderID`/`ParseID`, `report.FoldOutcome`'s worst-status-wins fold, the
  `report_path` clear-before-every-chunk discipline, and the runner's `report:` dispatch in
  `internal/runner/run.go`'s `readOutcomes`. Six real captures live in
  `internal/report/testdata/junit/` with their provenance. Out of scope here except the two
  new captures Task 6 adds.
- `internal/runner/run.go` already resolves `{report}` from `report_path` once per Run and
  clears it per chunk; `{log}` is already a per-chunk temp path in the same `vars` map.
- `cmd/rtdd/staticadvice.go` already partitions a detected **set** into static and coverage
  halves and already renders both halves into a sentence. Decision 6 is a caller change.
- `internal/adapter/markers.go`'s `UnsupportedMarkers` names `package.json`, `go.mod`,
  `Cargo.toml`, `pom.xml`, `build.gradle`, `Gemfile`, `composer.json` and `*.csproj` for the
  no-adapter refusal message. Those entries stay: they are the message a repo gets when
  decision 1's under-detection bites.

## File Structure

```
adapters/
  python.yaml                    # BYTE-FROZEN, untouched by every task here
  vitest.yaml       jest.yaml       go.yaml         cargo-nextest.yaml
  maven.yaml        gradle.yaml     rspec.yaml      dotnet.yaml      phpunit.yaml
internal/adapter/
  adapter.go                     # + TestFlag, TestJoin, ReportCmd; validate() rules
  detect.go                      # - the arity check and the v1 polyglot error string
  expand.go                      # ExpandTests learns the two splicing shapes
  splice_test.go                 # NEW
internal/runner/
  run.go                         # report_cmd runs between the subset and the read
  reportcmd_test.go              # NEW
internal/report/
  shipped_id_test.go             # NEW - the round trip for all nine id_templates
  testdata/junit/nextest.xml     # NEW capture
  testdata/junit/dotnet.xml      # NEW capture
  testdata/junit/jest-filepath.xml  # NEW capture
internal/mapstore/
  mapstore.go                    # Row.A
  meta.go                        # Meta.Adapters
  testdata/pre-prd-map.jsonl     # NEW - rows with no `a` key
cmd/rtdd/
  polyglot.go                    # NEW - the per-adapter loop and the exit-code fold
  testdata/fixtures/<nine>/      # NEW - one minimal repo per adapter
  testdata/fixtures/polyglot/    # NEW - package.json + vitest.config.ts + pom.xml
internal/contract/
  shipped_adapters_test.go       # NEW - the ten-row completeness table
scripts/
  capture-junit-fixtures.sh      # + nextest, dotnet, jest-filepath runners
```

## Task 1 — `internal/adapter`: `Detect` returns a set, and the v1 arity rule is deleted

**Discharges:** spec §4.4 (polyglot repositories). PRD #232 AC4 and AC5.

**Files:** `internal/adapter/detect.go`, `internal/adapter/detect_test.go`, `cmd/rtdd/rows.go`, `cmd/rtdd/detect_test.go`

**Interfaces:**

*Consumes:* `adapter.DetectAll` (existing, unchanged), `adapter.AvailableReport` (existing).

*Produces:*
```go
// Detect returns EVERY adapter whose detect globs match a file in repoRoot, in the order
// the adapters were given. Zero matches is still an error: RTDD has no toolchain to run.
// Two or more is no longer an error — spec §4.4 makes a polyglot repository ordinary, and
// each adapter's rows, selections and invocations carry its name (PRD #232 AC4).
func Detect(repoRoot string, adapters []*Adapter) ([]*Adapter, error)

// detectAdapters replaces cmd/rtdd's single-adapter detectAdapter. The warn writer still
// names every host adapter that failed to load.
func detectAdapters(repoRoot string, warn io.Writer) ([]*adapter.Adapter, error)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/detect_test.go` (append):

```go
package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec §4.4: a TypeScript service with a Java module is ordinary, not an error. The v1
// rule returned a message instead of a selection, so the repo got the null baseline from
// BOTH toolchains rather than a narrowed suite from each.
func TestDetectReturnsEveryMatchingAdapter(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"vitest.config.ts", "pom.xml"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}
	as := []*Adapter{
		{Name: "vitest", Detect: []string{"vitest.config.ts"}},
		{Name: "maven", Detect: []string{"pom.xml"}},
		{Name: "rspec", Detect: []string{".rspec"}},
	}

	got, err := Detect(root, as)
	if err != nil {
		t.Fatalf("Detect: %v, want no error for a polyglot repo", err)
	}
	if len(got) != 2 || got[0].Name != "vitest" || got[1].Name != "maven" {
		t.Fatalf("Detect = %v, want [vitest maven]", names(got))
	}
}

// Zero matches stays an error: it is the §5 gate rtdd init refuses on.
func TestDetectStillErrorsOnZeroMatches(t *testing.T) {
	root := t.TempDir()
	_, err := Detect(root, []*Adapter{{Name: "maven", Detect: []string{"pom.xml"}}})
	if err == nil {
		t.Fatal("Detect = nil error, want an error naming the repo with no adapter")
	}
	if !strings.Contains(err.Error(), root) {
		t.Errorf("Detect error %q does not name the repo root", err)
	}
}

// PRD #232 AC5: the v1 refusal string is gone from the tree, not merely unreachable. A
// dead string is one revert away from being live again, and it contradicts the spec.
func TestV1PolyglotErrorStringIsDeletedFromTheTree(t *testing.T) {
	const banned = "polyglot repos are out of scope in v1"
	root := repoRootForTest(t)
	var hits []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == "dist") {
				return filepath.SkipDir
			}
			return err
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), banned) {
			hits = append(hits, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("the v1 string %q survives in %v; spec §4.4 makes polyglot repos supported", banned, hits)
	}
}

func names(as []*Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/adapter/ -run 'TestDetectReturnsEveryMatchingAdapter|TestDetectStillErrorsOnZeroMatches|TestV1PolyglotErrorStringIsDeletedFromTheTree' -count=1
```

Expected: a compile failure first — `Detect` returns `(*Adapter, error)`, so `len(got)` and
`got[0].Name` do not type-check. After the signature changes,
`TestV1PolyglotErrorStringIsDeletedFromTheTree` fails naming
`internal/adapter/detect.go` as the file that still contains
`polyglot repos are out of scope in v1`.

- [ ] **Step 3: Make it pass.** Delete the `switch len(matched)` arity block; return
  `matched` and keep only the zero-match error. Delete the v1 message and the now-unused
  `strings` import. Update `cmd/rtdd/rows.go`'s `detectAdapter` to `detectAdapters`
  returning the slice, and every call site to loop it. The single-adapter call sites that
  Task 8 will rewrite properly may take `[0]` for now **only if** a `// TODO(#232 Task 8)`
  comment says so; nothing else may.
- [ ] **Step 4: Refactor.** `Detect` is now a two-line wrapper over `DetectAll`. Keep both:
  `DetectAll` is the walk, `Detect` is the walk plus the zero-match policy, and `init`
  (spec §5) wants the policy while `doctor` wants the walk.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh` — every existing polyglot-is-an-error
  assertion in `internal/adapter/detect_test.go` and `cmd/rtdd/detect_test.go` must be
  **rewritten to the new rule**, never deleted. Their subject is still a two-adapter repo;
  the expected outcome is what changed.

## Task 2 — `internal/adapter`: `test_flag` and `test_join`, the two id-splicing shapes

**Discharges:** spec §8 (the shipped adapter set cannot be expressed without them) and §4.2 (the contract key that expresses them). Unblocks PRD #232 AC1 for `gradle`, `maven`, `dotnet`, `phpunit` and `go`.

**Files:** `internal/adapter/adapter.go`, `internal/adapter/expand.go`, `internal/adapter/splice_test.go`

**Interfaces:**

*Consumes:* `Adapter.ExpandTests` (existing), `Adapter.validate` (existing).

*Produces:*
```go
// Two optional keys, mutually exclusive, that say how this runner accepts more than one
// selector. Bare splicing (both empty) is today's rule and stays the default.
//	TestFlag string `yaml:"test_flag"` // emit "<flag> <id>" for each id
//	TestJoin string `yaml:"test_join"` // join every id into ONE argv token

// ExpandTests splices ids in the shape the adapter declares:
//   neither: one bare argv element per id, at the token that is exactly "{tests}"
//   TestFlag: TestFlag, id, TestFlag, id, ... at that token
//   TestJoin: the ids joined by TestJoin, substituted wherever {tests} appears in a token
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/splice_test.go`:

```go
package adapter

import (
	"reflect"
	"strings"
	"testing"
)

// Gradle needs the flag before EACH id: `gradle test --tests A B` makes B a task name,
// and gradle then fails with "Task 'B' not found", which reads as a broken repo rather
// than a broken adapter.
func TestExpandTestsRepeatsTestFlagBeforeEachID(t *testing.T) {
	a := &Adapter{Name: "gradle", Subset: "gradle test {tests}", TestFlag: "--tests"}
	got, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest.adds", "calc.CalcTest.subs"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"gradle", "test", "--tests", "calc.CalcTest.adds", "--tests", "calc.CalcTest.subs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// Surefire takes ONE -Dtest argument holding every selector, so {tests} has to be
// substitutable INSIDE a token. Splicing bare ids here would hand mvn a list of goals.
func TestExpandTestsJoinsIDsInsideOneTokenWhenTestJoinIsDeclared(t *testing.T) {
	a := &Adapter{Name: "maven", Subset: "mvn -B test -Dtest={tests}", TestJoin: ","}
	got, err := a.ExpandTests(a.Subset, nil, []string{"calc.CalcTest#adds", "calc.CalcTest#subs"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"mvn", "-B", "test", "-Dtest=calc.CalcTest#adds,calc.CalcTest#subs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// The default is unchanged: pytest, vitest, jest, rspec and nextest all take repeated
// bare positionals, and adapters/python.yaml declares neither key.
func TestExpandTestsStillSplicesBareIDsWhenNeitherKeyIsDeclared(t *testing.T) {
	a := &Adapter{Name: "python", Subset: "pytest {tests} --cov"}
	got, err := a.ExpandTests(a.Subset, nil, []string{"tests/test_a.py::test_one", "tests/test_b.py::test_two"})
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}
	want := []string{"pytest", "tests/test_a.py::test_one", "tests/test_b.py::test_two", "--cov"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExpandTests = %q, want %q", got, want)
	}
}

// Two answers to one question is a configuration error, not a precedence puzzle.
func TestBothSplicingKeysIsALoadTimeError(t *testing.T) {
	_, err := parse([]byte(`name: bad
detect: ["x.toml"]
selection: static
coverage: none
subset: "run {tests}"
list: "run --list"
report: junit-xml
report_path: "junit.xml"
id_template: "{classname}#{name}"
test_flag: "--tests"
test_join: ","
`), "bad.yaml")
	if err == nil {
		t.Fatal("parse = nil error, want a rejection naming both test_flag and test_join")
	}
	for _, want := range []string{"test_flag", "test_join"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/adapter/ -run 'TestExpandTests|TestBothSplicingKeys' -count=1
```

Expected: compile failure — `Adapter` has no field `TestFlag` or `TestJoin`. Once the
fields exist, `TestExpandTestsRepeatsTestFlagBeforeEachID` fails with
`ExpandTests = ["gradle" "test" "calc.CalcTest.adds" "calc.CalcTest.subs"]`, and
`TestExpandTestsJoinsIDsInsideOneTokenWhenTestJoinIsDeclared` fails with
`adapter maven: template "mvn -B test -Dtest={tests}" has no {tests} placeholder` — the
existing code only recognises a token that is exactly `{tests}`.

- [ ] **Step 3: Make it pass.** Add both `yaml` fields. In `ExpandTests`, branch on which is
  set: bare (today's `tok == "{tests}"`), flag-repeated, or joined via
  `strings.ReplaceAll(tok, "{tests}", strings.Join(tests, a.TestJoin))` before the normal
  `substitute` pass. Add the mutual-exclusion rejection to `validate()` with both key names
  in the message, beside the other combination rules.
- [ ] **Step 4: Refactor.** Keep the "no `{tests}` anywhere in the template" error: under
  `TestJoin` it must now look for the substring rather than the whole token, and losing it
  would let a typo run the whole suite silently.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. `adapters/python.yaml` declares
  neither key, so its behaviour and its digest are untouched.

## Task 3 — `internal/runner`: `report_cmd`, the post-run report producer

**Discharges:** spec §8 (the `go` adapter cannot otherwise ship) and §4.3 (the report contract it extends).

**Files:** `internal/adapter/adapter.go`, `internal/runner/run.go`, `internal/runner/reportcmd_test.go`

**Interfaces:**

*Consumes:* `runner.execute`'s per-chunk capture buffer and `vars` map (existing), `report.ReportPath` (existing).

*Produces:*
```go
// ReportCmd is an optional command run AFTER the subset invocation and BEFORE report_path
// is read. It exists for exactly one shape the argv-only engine cannot express: a runner
// that writes its machine-readable output to stdout and a converter that reads stdin.
// {log} is the chunk's captured combined output, {report} is the resolved report_path.
//	ReportCmd string `yaml:"report_cmd"`

// A non-zero exit is fatal and names the adapter and the command. It is never treated as
// "no report": an empty report parses as a run in which nothing failed.
```

- [ ] **Step 1: Write the failing test**

`internal/runner/reportcmd_test.go`:

```go
package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// go test -json writes to stdout, go-junit-report reads stdin, and the engine builds argv
// and never a shell. report_cmd is the declared join between them.
func TestReportCmdConvertsCapturedStdoutIntoTheReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub converter is POSIX sh")
	}
	repo := t.TempDir()
	// The stub "runner" prints one line to stdout and writes no report at all.
	emit := filepath.Join(repo, "emit.sh")
	if err := os.WriteFile(emit, []byte("#!/bin/sh\necho \"case:$1\"\n"), 0o755); err != nil {
		t.Fatalf("write emit: %v", err)
	}
	// The stub "converter" turns that captured line into a JUnit document.
	conv := filepath.Join(repo, "conv.sh")
	script := "#!/bin/sh\nname=$(sed -n 's/^case://p' \"$1\")\n" +
		"printf '<testsuite name=\"s\"><testcase classname=\"s\" name=\"%s\" time=\"0.01\"/></testsuite>' \"$name\" > \"$2\"\n"
	if err := os.WriteFile(conv, []byte(script), 0o755); err != nil {
		t.Fatalf("write conv: %v", err)
	}

	a := &adapter.Adapter{
		Name:       "go",
		Detect:     []string{"go.mod"},
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Subset:     emit + " {tests}",
		ReportCmd:  "sh " + conv + " {log} {report}",
		Report:     "junit-xml",
		ReportPath: ".rtdd/junit.xml",
		IDTemplate: "{name}",
	}

	res, err := Run(a, repo, []string{"TestOne"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "TestOne" || res.Outcomes[0].Status != "pass" {
		t.Fatalf("Outcomes = %+v, want one passing TestOne read from the converted report", res.Outcomes)
	}
}

// A converter that fails is fatal. Continuing would read a report that was never written
// — or the previous chunk's — and call an unrun suite green.
func TestReportCmdFailureIsFatalAndNamesTheAdapter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub converter is POSIX sh")
	}
	repo := t.TempDir()
	emit := filepath.Join(repo, "emit.sh")
	if err := os.WriteFile(emit, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write emit: %v", err)
	}
	bad := filepath.Join(repo, "bad.sh")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	a := &adapter.Adapter{
		Name: "go", Detect: []string{"go.mod"},
		Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
		Subset: emit + " {tests}", ReportCmd: "sh " + bad + " {log} {report}",
		Report: "junit-xml", ReportPath: ".rtdd/junit.xml", IDTemplate: "{name}",
	}

	_, err := Run(a, repo, []string{"TestOne"}, false)
	if err == nil {
		t.Fatal("Run = nil error, want a fatal error: the report was never produced")
	}
	if !strings.Contains(err.Error(), "go") || !strings.Contains(err.Error(), "report_cmd") {
		t.Errorf("error %q must name the adapter and report_cmd", err)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/runner/ -run 'TestReportCmd' -count=1
```

Expected: compile failure — `adapter.Adapter` has no field `ReportCmd`. With the field
added and nothing wired, `TestReportCmdConvertsCapturedStdoutIntoTheReport` fails with
`junit-xml: cannot read report_path` (the stub runner writes no report), and
`TestReportCmdFailureIsFatalAndNamesTheAdapter` fails with `Run = nil error`.

- [ ] **Step 3: Make it pass.** Add the `report_cmd` field and validate it: declaring it
  without `report: junit-xml` is exit 2, and it must name `{report}`. In `execute`, after
  `cmd.Run()` and before `readOutcomes`, write `combined.Bytes()` to the chunk's `{log}`
  path, `Expand` the `report_cmd` template with the same `vars`, run it with the same
  `cmd.Dir` and `cmd.Env`, and wrap a non-zero exit as
  `runner: adapter %s: report_cmd %q: %w`.
- [ ] **Step 4: Refactor.** `{log}` already exists in `vars` as the pytest report-log path.
  Reuse it rather than adding a third name: one adapter uses it as the file its runner
  writes, the other as the file the engine writes, and both are "this chunk's scratch file".
  Say so in the comment.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. Python declares no `report_cmd`,
  so the pytest path does not execute a single new line.

## Task 4 — `adapters/`: the nine shipped declarations

**Discharges:** spec §8 (the shipped adapter set). PRD #232 AC1, AC3, AC8.

**Files:** `adapters/vitest.yaml`, `adapters/jest.yaml`, `adapters/go.yaml`, `adapters/cargo-nextest.yaml`, `adapters/maven.yaml`, `adapters/gradle.yaml`, `adapters/rspec.yaml`, `adapters/dotnet.yaml`, `adapters/phpunit.yaml`, `internal/adapter/shipped_test.go`

**Interfaces:**

*Consumes:* `adapter.Builtin()` (existing — `adapters/*.yaml` is already `go:embed`-ed), the contract v2 validator (#229), `TestFlag`/`TestJoin` (Task 2), `ReportCmd` (Task 3).

*Produces:* nine files. They are reproduced here in full so the implementer writes YAML rather than inventing it.

```yaml
name: vitest
detect: ["vitest.config.js", "vitest.config.ts", "vitest.config.mjs", "vitest.config.mts", "vitest.config.cjs", "vitest.workspace.ts"]
selection: static
coverage: none
subset: "npx vitest run {tests} --reporter=junit --outputFile={report}"
list: "npx vitest run --reporter=junit --outputFile={report}"
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{classname}"
failfast_flag: "--bail=1"
test_for: ["{dir}/{name}.test.ts", "{dir}/{name}.test.tsx", "{dir}/{name}.spec.ts", "{dir}/__tests__/{name}.test.ts", "test/{name}.test.ts", "tests/{name}.test.ts"]
test_globs: ["**/*.test.ts", "**/*.test.tsx", "**/*.test.js", "**/*.spec.ts", "**/*.spec.js", "test/**/*.ts", "tests/**/*.ts"]
source_globs: ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx", "**/*.mjs"]
opaque: ["**/*.json", "**/*.css", "**/*.html", "**/*.snap", "**/fixtures/**"]
full_escalate: ["package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "vitest.config.js", "vitest.config.ts", "vitest.config.mjs", "vitest.config.mts", "vitest.config.cjs", "vitest.workspace.ts", "tsconfig.json", "**/setupTests.ts"]
```

```yaml
name: jest
detect: ["jest.config.js", "jest.config.ts", "jest.config.mjs", "jest.config.cjs", "jest.config.json"]
selection: static
coverage: none
env:
  JEST_JUNIT_CLASSNAME: "{filepath}"
  JEST_JUNIT_OUTPUT_DIR: ".rtdd"
  JEST_JUNIT_OUTPUT_NAME: "junit.xml"
subset: "npx jest --reporters=jest-junit {tests}"
list: "npx jest --reporters=jest-junit"
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{classname}"
failfast_flag: "--bail"
requires:
  - bin: npx
    reason: "jest and its JUnit reporter run through npx; jest-junit must be a devDependency or no JUnit XML is written"
test_for: ["{dir}/{name}.test.ts", "{dir}/{name}.test.js", "{dir}/__tests__/{name}.test.js", "test/{name}.test.js", "tests/{name}.test.js"]
test_globs: ["**/*.test.ts", "**/*.test.tsx", "**/*.test.js", "**/*.spec.js", "**/__tests__/**/*.js", "**/__tests__/**/*.ts"]
source_globs: ["**/*.ts", "**/*.tsx", "**/*.js", "**/*.jsx", "**/*.mjs"]
opaque: ["**/*.json", "**/*.css", "**/*.html", "**/*.snap", "**/__snapshots__/**", "**/fixtures/**"]
full_escalate: ["package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "jest.config.js", "jest.config.ts", "jest.config.mjs", "jest.config.cjs", "jest.config.json", "jest.setup.js", "babel.config.js", "tsconfig.json"]
```

```yaml
name: go
detect: ["go.mod"]
selection: static
coverage: none
subset: "go test -json -run {tests} ./..."
test_join: "|"
report_cmd: "go-junit-report -parser gojson -in {log} -out {report}"
list: "go test -json ./..."
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{name}"
failfast_flag: "-failfast"
requires:
  - bin: go-junit-report
    reason: "go emits no JUnit XML natively; go test -json is converted by go-junit-report"
test_for: ["{dir}/{name}_test.go"]
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
opaque: ["**/testdata/**", "**/*.golden", "**/*.json", "**/*.yaml", "**/*.tmpl"]
full_escalate: ["go.mod", "go.sum", "vendor/modules.txt", "**/tools.go"]
```

```yaml
name: cargo-nextest
detect: [".config/nextest.toml"]
selection: static
coverage: none
subset: "cargo nextest run --profile rtdd {tests}"
list: "cargo nextest run --profile rtdd"
report: junit-xml
report_path: "target/nextest/rtdd/junit.xml"
id_template: "{name}"
failfast_flag: "--fail-fast"
requires:
  - bin: cargo-nextest
    reason: "cargo test emits no JUnit XML; nextest writes it from the [profile.rtdd.junit] block in .config/nextest.toml, which is a config file and not a CLI flag"
test_for: ["tests/{name}.rs", "{dir}/{name}/tests.rs"]
test_globs: ["tests/**/*.rs", "**/tests.rs"]
source_globs: ["src/**/*.rs", "tests/**/*.rs", "benches/**/*.rs"]
opaque: ["**/*.toml", "**/*.json", "**/testdata/**", "**/snapshots/**"]
full_escalate: ["Cargo.toml", "Cargo.lock", ".config/nextest.toml", "rust-toolchain.toml", "build.rs"]
```

```yaml
name: maven
detect: ["pom.xml"]
selection: static
coverage: none
subset: "mvn -B test -Dtest={tests} -DfailIfNoSpecifiedTests=false"
test_join: ","
list: "mvn -B test"
report: junit-xml
report_path: "target/surefire-reports/"
id_template: "{classname}#{name}"
failfast_flag: "-ff"
test_for: ["src/test/java/{dir}/{name}Test.java", "src/test/java/{dir}/{name}Tests.java"]
test_globs: ["src/test/java/**/*.java", "src/test/kotlin/**/*.kt"]
source_globs: ["src/main/java/**/*.java", "src/main/kotlin/**/*.kt", "src/test/java/**/*.java"]
opaque: ["src/main/resources/**", "src/test/resources/**", "**/*.xml", "**/*.properties"]
full_escalate: ["pom.xml", "**/pom.xml", ".mvn/wrapper/maven-wrapper.properties", "src/test/resources/**"]
```

```yaml
name: gradle
detect: ["build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"]
selection: static
coverage: none
subset: "./gradlew test {tests}"
test_flag: "--tests"
list: "./gradlew test"
report: junit-xml
report_path: "build/test-results/test/"
id_template: "{classname}.{name}"
failfast_flag: "--fail-fast"
test_for: ["src/test/java/{dir}/{name}Test.java", "src/test/kotlin/{dir}/{name}Test.kt"]
test_globs: ["src/test/java/**/*.java", "src/test/kotlin/**/*.kt"]
source_globs: ["src/main/java/**/*.java", "src/main/kotlin/**/*.kt", "src/test/java/**/*.java", "src/test/kotlin/**/*.kt"]
opaque: ["src/main/resources/**", "src/test/resources/**", "**/*.properties", "**/*.xml"]
full_escalate: ["build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "gradle.properties", "gradle/libs.versions.toml", "gradle/wrapper/gradle-wrapper.properties"]
```

```yaml
name: rspec
detect: [".rspec", "spec/spec_helper.rb"]
selection: static
coverage: none
subset: "bundle exec rspec --format RspecJunitFormatter --out {report} {tests}"
list: "bundle exec rspec --format RspecJunitFormatter --out {report}"
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}"
failfast_flag: "--fail-fast"
requires:
  - bin: rspec
    reason: "rspec emits no JUnit XML without the rspec_junit_formatter gem; add it to the Gemfile"
test_for: ["spec/{dir}/{name}_spec.rb", "spec/{name}_spec.rb", "{dir}/{name}_spec.rb"]
test_globs: ["spec/**/*_spec.rb"]
source_globs: ["lib/**/*.rb", "app/**/*.rb", "spec/**/*.rb"]
opaque: ["spec/fixtures/**", "**/*.yml", "**/*.erb", "**/*.json"]
full_escalate: ["Gemfile", "Gemfile.lock", ".rspec", "spec/spec_helper.rb", "spec/rails_helper.rb", "*.gemspec"]
```

```yaml
name: dotnet
detect: ["*.sln", "*.csproj", "*.fsproj"]
selection: static
coverage: none
subset: "dotnet test --logger junit --results-directory {report} --filter {tests}"
test_join: "|"
list: "dotnet test --logger junit --results-directory {report}"
report: junit-xml
report_path: "TestResults/"
id_template: "{classname}.{name}"
requires:
  - bin: dotnet
    reason: "only trx is built in; the JUnitTestLogger package reference is what makes --logger junit resolve"
test_for: ["tests/{name}.Tests/{name}Tests.cs", "{dir}.Tests/{name}Tests.cs", "test/{name}Tests.cs"]
test_globs: ["**/*Tests.cs", "**/*Test.cs", "tests/**/*.cs", "test/**/*.cs"]
source_globs: ["**/*.cs", "**/*.fs"]
opaque: ["**/*.json", "**/*.resx", "**/*.xml", "**/TestData/**"]
full_escalate: ["*.sln", "**/*.csproj", "**/*.fsproj", "Directory.Build.props", "Directory.Packages.props", "nuget.config", "global.json"]
```

```yaml
name: phpunit
detect: ["phpunit.xml", "phpunit.xml.dist", "phpunit.dist.xml"]
selection: static
coverage: none
subset: "vendor/bin/phpunit --log-junit {report} --filter {tests}"
test_join: "|"
list: "vendor/bin/phpunit --log-junit {report}"
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{classname}::{name}"
failfast_flag: "--stop-on-failure"
test_for: ["tests/{name}Test.php", "tests/{dir}/{name}Test.php", "{dir}/{name}Test.php"]
test_globs: ["tests/**/*Test.php"]
source_globs: ["src/**/*.php", "lib/**/*.php", "tests/**/*.php"]
opaque: ["**/*.json", "**/*.xml", "**/*.twig", "tests/fixtures/**"]
full_escalate: ["composer.json", "composer.lock", "phpunit.xml", "phpunit.xml.dist", "phpunit.dist.xml", "tests/bootstrap.php"]
```

- [ ] **Step 1: Write the failing test**

`internal/adapter/shipped_test.go`:

```go
package adapter

import (
	"strings"
	"testing"
)

// Every shipped adapter must LOAD. The embedded set is read at startup by every command,
// so a typo in one of the nine is not a bad adapter, it is a binary that cannot run
// anywhere (PRD #232 AC1).
func TestBuiltinShipsTenAdaptersAndAllOfThemLoad(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := []string{
		"cargo-nextest", "dotnet", "go", "gradle", "jest",
		"maven", "phpunit", "python", "rspec", "vitest",
	}
	if len(all) != len(want) {
		t.Fatalf("Builtin returned %d adapters (%v), want %d", len(all), adapterNamesFor(all), len(want))
	}
	for i, name := range want {
		if all[i].Name != name {
			t.Errorf("Builtin()[%d].Name = %q, want %q (LoadFS sorts by name)", i, all[i].Name, name)
		}
	}
}

// Decision 1: package.json is NOT a marker. Listing it in both JS adapters makes every
// JavaScript repo detect two adapters and AC10's fixture detect three.
func TestNoShippedAdapterDetectsOnPackageJSON(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, a := range all {
		for _, m := range a.Detect {
			if m == "package.json" {
				t.Errorf("adapter %s detects on package.json; both JS adapters would match every JS repo (decision 1)", a.Name)
			}
		}
	}
}

// PRD #232 AC3, and decision 11: the five declare a requires whose bin is a PATH binary,
// and whose reason names the package a human has to install.
func TestTheFiveRequiresAdaptersNameACheckableBinAndTheirPackage(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := map[string]struct{ bin, pkg string }{
		"go":            {"go-junit-report", "go-junit-report"},
		"cargo-nextest": {"cargo-nextest", "nextest"},
		"jest":          {"npx", "jest-junit"},
		"rspec":         {"rspec", "rspec_junit_formatter"},
		"dotnet":        {"dotnet", "JUnitTestLogger"},
	}
	for _, a := range all {
		w, needs := want[a.Name]
		if !needs {
			if len(a.Requires) != 0 {
				t.Errorf("adapter %s declares requires; its runner emits JUnit XML unaided (AC3)", a.Name)
			}
			continue
		}
		if len(a.Requires) == 0 {
			t.Errorf("adapter %s declares no requires (AC3)", a.Name)
			continue
		}
		if a.Requires[0].Bin != w.bin {
			t.Errorf("adapter %s requires[0].Bin = %q, want the PATH-checkable %q (decision 11)", a.Name, a.Requires[0].Bin, w.bin)
		}
		if !strings.Contains(a.Requires[0].Reason, w.pkg) {
			t.Errorf("adapter %s requires[0].Reason %q does not name the package %q", a.Name, a.Requires[0].Reason, w.pkg)
		}
	}
}

// PRD #232 AC8: full_escalate names the ecosystem's REAL lockfile and runner config. A
// dependency bump that escalates nothing selects a stale suite against new dependencies.
func TestFullEscalateNamesTheLockfileAndRunnerConfig(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	want := map[string][]string{
		"vitest":        {"package-lock.json", "pnpm-lock.yaml", "vitest.config.ts"},
		"jest":          {"package-lock.json", "jest.config.js"},
		"go":            {"go.sum"},
		"cargo-nextest": {"Cargo.lock", ".config/nextest.toml"},
		"maven":         {"pom.xml"},
		"gradle":        {"gradle/wrapper/gradle-wrapper.properties", "build.gradle"},
		"rspec":         {"Gemfile.lock", ".rspec"},
		"dotnet":        {"Directory.Packages.props"},
		"phpunit":       {"composer.lock", "phpunit.xml"},
	}
	byName := map[string]*Adapter{}
	for _, a := range all {
		byName[a.Name] = a
	}
	for name, needles := range want {
		a := byName[name]
		if a == nil {
			t.Errorf("no adapter named %s", name)
			continue
		}
		joined := strings.Join(a.FullEscalate, " ")
		for _, n := range needles {
			if !strings.Contains(joined, n) {
				t.Errorf("adapter %s full_escalate does not name %q: %v", name, n, a.FullEscalate)
			}
		}
	}
}

func adapterNamesFor(as []*Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/adapter/ -run 'TestBuiltinShipsTenAdapters|TestNoShippedAdapterDetectsOnPackageJSON|TestTheFiveRequiresAdapters|TestFullEscalateNames' -count=1
```

Expected: `Builtin returned 1 adapters ([python]), want 10`, and the other three failing on
`no adapter named vitest` and friends.

- [ ] **Step 3: Make it pass.** Write the nine files exactly as above. `adapters/adapters.go`
  already embeds `*.yaml`, so nothing there changes.
- [ ] **Step 4: Refactor.** Each file opens with a comment block in the voice of
  `adapters/python.yaml`: what the runner emits, why the `id_template` is the shape it is,
  and — for `jest`, `go`, `maven`, `gradle`, `dotnet` and `phpunit` — which of decisions 8,
  9 and 10 the file is an instance of. An adapter that is surprising and unexplained is one
  the next reader will "simplify".
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. `adapters/python.yaml` is not
  opened; the digest test must still pass untouched.

## Task 5 — `internal/contract`: the ten-row completeness table

**Discharges:** spec §8 (a half-specified adapter must not be able to ship). PRD #232 AC2.

**Files:** `internal/contract/shipped_adapters_test.go`

**Interfaces:**

*Consumes:* `adapter.Builtin()`, `adapter.Adapter`.

*Produces:* no production code. One table-driven test that is the gate every future
adapter — shipped or host-authored — is measured against.

- [ ] **Step 1: Write the failing test**

`internal/contract/shipped_adapters_test.go`:

```go
package contract

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// PRD #232 AC2: a future adapter cannot ship half-specified.
//
// Decision 2: the table has TEN rows and the universally-required keys are asserted for
// all ten, but report_path / id_template / test_for are asserted only for the adapters
// that declare `report: junit-xml`. adapters/python.yaml declares none of the three and is
// BYTE-FROZEN (internal/contract/adapter_freeze_test.go): satisfying a literal "all ten
// including python" reading would mean editing a file spec §4.1 exists to hold still. The
// asymmetry is the freeze, not an oversight, and it is not to be "fixed".
func TestEveryShippedAdapterIsFullySpecified(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(all) != 10 {
		t.Fatalf("Builtin returned %d adapters, want 10", len(all))
	}

	for _, a := range all {
		t.Run(a.Name, func(t *testing.T) {
			nonEmptyString(t, "name", a.Name)
			nonEmptyString(t, "subset", a.Subset)
			nonEmptyString(t, "list", a.List)
			nonEmptySlice(t, "detect", a.Detect)
			nonEmptySlice(t, "test_globs", a.TestGlobs)
			nonEmptySlice(t, "source_globs", a.SourceGlobs)
			nonEmptySlice(t, "opaque", a.Opaque)
			nonEmptySlice(t, "full_escalate", a.FullEscalate)

			if a.Report != "junit-xml" {
				return // python: see the comment above.
			}
			nonEmptyString(t, "report_path", a.ReportPath)
			nonEmptyString(t, "id_template", a.IDTemplate)
			nonEmptySlice(t, "test_for", a.TestFor)
			if a.Selection != adapter.SelectionStatic {
				t.Errorf("selection = %q, want %q", a.Selection, adapter.SelectionStatic)
			}
			if a.Coverage != adapter.CoverageNone {
				t.Errorf("coverage = %q, want %q", a.Coverage, adapter.CoverageNone)
			}
			if a.Fidelity() != adapter.FidelityStatic {
				t.Errorf("Fidelity() = %q, want %q: an adapter that can select nothing is not worth shipping", a.Fidelity(), adapter.FidelityStatic)
			}
		})
	}
}

func nonEmptyString(t *testing.T, key, got string) {
	t.Helper()
	if got == "" {
		t.Errorf("%s is empty; PRD #232 AC2 requires it non-empty", key)
	}
}

func nonEmptySlice(t *testing.T, key string, got []string) {
	t.Helper()
	if len(got) == 0 {
		t.Errorf("%s is empty; PRD #232 AC2 requires it non-empty", key)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestEveryShippedAdapterIsFullySpecified' -count=1
```

Expected: before Task 4 lands, `Builtin returned 1 adapters, want 10`. Run it again after
Task 4 and every subtest must be green; a red subtest names the adapter and the key.

- [ ] **Step 3: Make it pass.** Nothing to implement — Task 4's YAML is the implementation.
  If a key is missing, the fix is the YAML file, never a weakened assertion.
- [ ] **Step 4: Refactor.** Nothing. Resist adding a `skip` list: a skip list is how the
  next half-specified adapter ships.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

## Task 6 — `internal/report`: every shipped `id_template` round-trips, over real captures

**Discharges:** spec §4.3 (the id round trip, per adapter) in service of §8. PRD #232 AC1.

**Files:** `internal/report/shipped_id_test.go`, `internal/report/testdata/junit/nextest.xml`, `internal/report/testdata/junit/dotnet.xml`, `internal/report/testdata/junit/jest-filepath.xml`, `internal/report/testdata/junit/README.md`, `scripts/capture-junit-fixtures.sh`

**Interfaces:**

*Consumes:* `report.ReadJUnitReport(p, tmpl)`, `report.RenderID(tmpl, c)`, `report.ParseID(tmpl, id)`, `report.NewReportPathFor(adapter, root, declared)` (all existing, #231).

*Produces:* three new captures and one test that pins decision 7's table to them.

- [ ] **Step 1: Write the failing test**

`internal/report/shipped_id_test.go`:

```go
package report

import (
	"path/filepath"
	"strings"
	"testing"
)

// Decision 7's table, asserted against the runners' OWN output. A hand-written fixture
// records what its author believes the runner emits; these record what six — now nine —
// ecosystems actually emit, which is what keeps disagreeing.
//
// The property is: every case in the capture renders a NON-EMPTY id under the shipped
// adapter's id_template, and re-rendering the parsed id reproduces it exactly. An id that
// renders empty folds the whole suite into one row (ErrEmptyRenderedID); an id that cannot
// be read back cannot be checked against the ids the runner was handed.
func TestShippedIDTemplatesRoundTrip(t *testing.T) {
	cases := []struct {
		adapter  string
		fixture  string
		template string
		wantID   string // the id the FIRST testcase in the capture must render
	}{
		{"vitest", "vitest.xml", "{classname}", "test/calc.test.js"},
		{"jest", "jest-filepath.xml", "{classname}", "test/calc.test.js"},
		{"rspec", "rspec.xml", "{file}", "./spec/calc_spec.rb"},
		{"go", "go-junit-report.xml", "{name}", "TestAddsTwoNumbers"},
		{"cargo-nextest", "nextest.xml", "{name}", "tests::adds_two_numbers"},
		{"maven", "surefire.xml", "{classname}#{name}", "calc.CalcTest#addsTwoNumbers"},
		{"gradle", "surefire.xml", "{classname}.{name}", "calc.CalcTest.addsTwoNumbers"},
		{"dotnet", "dotnet.xml", "{classname}.{name}", "Calc.Tests.CalcTest.AddsTwoNumbers"},
		{"phpunit", "phpunit.xml", "{classname}::{name}", "CalcTest::testAddsTwoNumbers"},
	}
	for _, tc := range cases {
		t.Run(tc.adapter, func(t *testing.T) {
			rp, err := NewReportPathFor(tc.adapter, "testdata/junit", filepath.Base(tc.fixture))
			if err != nil {
				t.Fatalf("NewReportPathFor: %v", err)
			}
			outcomes, err := ReadJUnitReport(rp, tc.template)
			if err != nil {
				t.Fatalf("ReadJUnitReport(%s, %q): %v", tc.fixture, tc.template, err)
			}
			if len(outcomes) == 0 {
				t.Fatalf("%s produced no outcomes", tc.fixture)
			}
			if outcomes[0].Test != tc.wantID {
				t.Errorf("first id = %q, want %q", outcomes[0].Test, tc.wantID)
			}
			for _, o := range outcomes {
				if strings.TrimSpace(o.Test) == "" {
					t.Fatalf("%s rendered an empty id under %q", tc.fixture, tc.template)
				}
				parsed, err := ParseID(tc.template, o.Test)
				if err != nil {
					t.Fatalf("ParseID(%q, %q): %v", tc.template, o.Test, err)
				}
				again, err := RenderID(tc.template, parsed)
				if err != nil {
					t.Fatalf("RenderID: %v", err)
				}
				if again != o.Test {
					t.Errorf("round trip: %q -> %q", o.Test, again)
				}
			}
		})
	}
}

// Decision 7: three of the nine share one id across every case in a file, and the fold is
// worst-status-wins. A file whose only failing case folded to `pass` is a false green.
func TestFileGranularAdaptersFoldAFileToOneOutcome(t *testing.T) {
	for _, tc := range []struct{ adapter, fixture, template string }{
		{"vitest", "vitest.xml", "{classname}"},
		{"jest", "jest-filepath.xml", "{classname}"},
		{"rspec", "rspec.xml", "{file}"},
	} {
		t.Run(tc.adapter, func(t *testing.T) {
			rp, err := NewReportPathFor(tc.adapter, "testdata/junit", tc.fixture)
			if err != nil {
				t.Fatalf("NewReportPathFor: %v", err)
			}
			outcomes, err := ReadJUnitReport(rp, tc.template)
			if err != nil {
				t.Fatalf("ReadJUnitReport: %v", err)
			}
			// Each capture is one file holding one pass, one fail and one skip.
			if len(outcomes) != 1 {
				t.Fatalf("outcomes = %d, want 1: every case in the file shares the id", len(outcomes))
			}
			if outcomes[0].Status != "fail" {
				t.Errorf("folded status = %q, want %q (worst-status-wins)", outcomes[0].Status, "fail")
			}
		})
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/report/ -run 'TestShippedIDTemplatesRoundTrip|TestFileGranularAdaptersFold' -count=1
```

Expected: `ReadJUnitReport(nextest.xml, "{name}")` fails with
`no such file or directory` for the three captures that do not exist yet, and the `jest`
subtest fails on `jest-filepath.xml` for the same reason. `gradle` and `maven` read the same
`surefire.xml` and must pass as soon as the file is read at all.

- [ ] **Step 3: Make it pass.** Add three capture functions to
  `scripts/capture-junit-fixtures.sh` — `nextest` (image `rust:1-slim`, `cargo nextest run
  --profile ci` with a `.config/nextest.toml` declaring `[profile.ci.junit] path =
  "junit.xml"`), `dotnet` (image `mcr.microsoft.com/dotnet/sdk:8.0`, a package reference to
  `JunitXml.TestLogger` and `dotnet test --logger junit`), and `jest-filepath` (the existing
  jest project re-run with `JEST_JUNIT_CLASSNAME='{filepath}'`). Run the script for those
  three runners, commit the captures **unedited**, and add their provenance rows to
  `internal/report/testdata/junit/README.md` — image, version, exact command, capture date.
- [ ] **Step 4: Refactor.** Extend the README's "what the nine disagree about" table with the
  three new rows: nextest's `classname` is the binary id, `JunitXml.TestLogger` writes a
  `<testsuites>` root, and `jest-filepath.xml` differs from `jest.xml` **only** in
  `classname=`, which is the whole point of decision 10. Leave `jest.xml` untouched: it is
  the default-config capture the parser is tested against.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. The script is not part of the
  gate — it needs Docker and the network — so the gate sees only the committed captures.

## Task 7 — `internal/mapstore`: a row carries its adapter, and what an untagged row means

**Discharges:** spec §4.4 ("rows would need an adapter tag"). PRD #232 AC6.

**Files:** `internal/mapstore/mapstore.go`, `internal/mapstore/io.go`, `internal/mapstore/mapstore_test.go`, `internal/mapstore/testdata/pre-prd-map.jsonl`

**Interfaces:**

*Consumes:* `mapstore.Row`, `Map.Union`, `Map.Replace`, `Map.TestsCovering`, `mapstore.Load`, `Map.Save` (existing).

*Produces:*
```go
// Row gains the adapter that produced it. omitempty keeps every byte of an existing
// .rtdd/map.jsonl valid and unrewritten: upgrading rtdd must not produce a whole-file
// diff in every host repo that has ever seeded.
//	A string `json:"a,omitempty"`

// TestsCoveringFor is TestsCovering restricted to one adapter's rows. An untagged row —
// written before this PRD — belongs to the adapter .rtdd/meta.json names, and to no other
// (decision 3).
func (m *Map) TestsCoveringFor(adapterName, legacyAdapter string, files []string) []string
```

- [ ] **Step 1: Write the failing test**

`internal/mapstore/mapstore_test.go` (append):

```go
package mapstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PRD #232 AC6: a row written by one adapter is never served to another. Serving a pytest
// row to the vitest adapter hands `npx vitest run` a pytest nodeid, which selects nothing
// and reports green — a false pass wearing a real id.
func TestTestsCoveringForNeverServesAnotherAdaptersRow(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_a.py::test_one", F: []string{"src/calc.py"}, A: "python"})
	m.Replace(Row{T: "test/calc.test.js", F: []string{"src/calc.py"}, A: "vitest"})

	got := m.TestsCoveringFor("python", "", []string{"src/calc.py"})
	if len(got) != 1 || got[0] != "tests/test_a.py::test_one" {
		t.Errorf("TestsCoveringFor(python) = %v, want only the python row", got)
	}
	got = m.TestsCoveringFor("vitest", "", []string{"src/calc.py"})
	if len(got) != 1 || got[0] != "test/calc.test.js" {
		t.Errorf("TestsCoveringFor(vitest) = %v, want only the vitest row", got)
	}
}

// Decision 3: an untagged row is the whole map of every repo that seeded before this PRD.
// It belongs to the adapter meta.json names, and to nothing else.
func TestUntaggedRowsBelongToTheAdapterMetaNames(t *testing.T) {
	b, err := os.ReadFile("testdata/pre-prd-map.jsonl")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if strings.Contains(string(b), `"a"`) {
		t.Fatal("the pre-PRD fixture must contain no adapter tag; that is what makes it the fixture")
	}
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write map: %v", err)
	}
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := m.TestsCoveringFor("python", "python", []string{"src/calc.py"}); len(got) == 0 {
		t.Error("an untagged row was withheld from the adapter meta.json names; a seeded repo lost its map on upgrade")
	}
	if got := m.TestsCoveringFor("vitest", "python", []string{"src/calc.py"}); len(got) != 0 {
		t.Errorf("TestsCoveringFor(vitest) = %v, want none: an untagged row is python's, not everyone's", got)
	}
	// No meta.json to say whose it is: ignored for selection, never guessed at.
	if got := m.TestsCoveringFor("python", "", []string{"src/calc.py"}); len(got) != 0 {
		t.Errorf("TestsCoveringFor with no legacy adapter = %v, want none", got)
	}
}

// omitempty: an existing map.jsonl must round-trip byte-identically through read+write.
func TestWritingAnUntaggedRowEmitsNoAdapterKey(t *testing.T) {
	m := New()
	m.Replace(Row{T: "tests/test_a.py::test_one", F: []string{"src/calc.py"}, C: "abc1234", D: 12, S: "pass"})
	p := filepath.Join(t.TempDir(), "map.jsonl")
	if err := m.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if strings.Contains(string(out), `"a"`) {
		t.Errorf("Save emitted an adapter key for an untagged row: %s", out)
	}
}
```

`internal/mapstore/testdata/pre-prd-map.jsonl`:

```
{"t":"tests/test_a.py::test_one","f":["src/calc.py"],"c":"abc1234","d":12,"s":"pass"}
{"t":"tests/test_b.py::test_two","f":["src/calc.py","src/util.py"],"c":"abc1234","d":8,"s":"pass"}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/mapstore/ -run 'TestTestsCoveringForNeverServes|TestUntaggedRowsBelongTo|TestWritingAnUntaggedRow' -count=1
```

Expected: compile failure — `Row` has no field `A`, and `Map` has no method
`TestsCoveringFor`.

- [ ] **Step 3: Make it pass.** Add `A string \`json:"a,omitempty"\`` to `Row` and
  `TestsCoveringFor` beside `TestsCovering`. Keep `TestsCovering` — `rtdd explain` and the
  drift guard ask "which tests cover this file" without an adapter in hand, and answering
  that with the whole map is right.
- [ ] **Step 4: Refactor.** `normalizeRow` and `Union` must carry `A` through unchanged. A
  `Union` between two rows with the same `T` and **different** `A` is a duplicate id across
  adapters: keep both by keying the map on `A + "\x00" + T` internally rather than on `T`
  alone, and say in the comment that two ecosystems naming one test the same string is a
  collision the tag exists to survive.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

## Task 8 — `cmd/rtdd`: per-adapter selection, and `.rtdd/meta.json`'s `adapters`

**Discharges:** spec §4.4 (selections carry the adapter that produced them). PRD #232 AC6.

**Files:** `cmd/rtdd/polyglot.go`, `cmd/rtdd/which.go`, `cmd/rtdd/rows.go`, `cmd/rtdd/meta.go`, `internal/mapstore/meta.go`, `cmd/rtdd/polyglot_test.go`, `internal/mapstore/meta_test.go`

**Interfaces:**

*Consumes:* `adapter.Detect` (Task 1), `selector.Select` (existing, #230), `Map.TestsCoveringFor` (Task 7).

*Produces:*
```go
// Meta keeps the singular adapter AND gains the plural set (decision 4). Turning the
// singular into a list would make every pre-PRD meta.json unreadable, and the singular is
// the answer to "whose is this untagged row?".
type Meta struct {
	V        int      `json:"v"`
	Adapter  string   `json:"adapter"`            // the COVERAGE adapter that produced the map; "" when all are static
	Adapters []string `json:"adapters,omitempty"` // the full detected set at seed time, sorted
	SeededAt string   `json:"seeded_at"`
	Cycles   int      `json:"cycles"`
}

// DetectedAdapters is the set Meta names, newest key first: adapters when present,
// otherwise the singular adapter, otherwise nothing.
func (m Meta) DetectedAdapters() []string

// AdapterSelection is one adapter's answer. cmd/rtdd renders one block per element and
// never merges two adapters' ids into one list.
type AdapterSelection struct {
	Adapter   string
	Selection selector.Selection
}

func selectPerAdapter(root string, ads []*adapter.Adapter, m *mapstore.Map, meta mapstore.Meta) ([]AdapterSelection, error)
```

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/polyglot_test.go`:

```go
package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// Spec §4.4: each adapter answers for itself. Merging the two lists would hand one
// runner the other's selectors, which is PRD #232 AC6's forbidden case with extra steps.
func TestSelectPerAdapterKeepsEachAdaptersIDsInItsOwnBlock(t *testing.T) {
	root := t.TempDir()
	ads := []*adapter.Adapter{
		{Name: "python", Detect: []string{"pyproject.toml"}, Selection: adapter.SelectionCoverage, Coverage: "sqlite", TestGlobs: []string{"tests/**/*.py"}, SourceGlobs: []string{"**/*.py"}},
		{Name: "vitest", Detect: []string{"vitest.config.ts"}, Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone, TestGlobs: []string{"**/*.test.ts"}, SourceGlobs: []string{"**/*.ts"}},
	}
	m := mapstore.New()
	m.Replace(mapstore.Row{T: "tests/test_calc.py::test_add", F: []string{"src/calc.py"}, A: "python"})
	m.Replace(mapstore.Row{T: "src/calc.test.ts", F: []string{"src/calc.ts"}, A: "vitest"})

	got, err := selectPerAdapter(root, ads, m, mapstore.Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d blocks, want one per detected adapter", len(got))
	}
	for _, blk := range got {
		for _, id := range blk.Selection.Tests {
			if blk.Adapter == "python" && id == "src/calc.test.ts" {
				t.Errorf("python's block contains vitest's id %q", id)
			}
			if blk.Adapter == "vitest" && id == "tests/test_calc.py::test_add" {
				t.Errorf("vitest's block contains python's id %q", id)
			}
		}
	}
}

// Decision 4: the plural key is added, the singular is kept, and a pre-PRD meta.json is
// still read correctly by the new binary.
func TestMetaReadsThePluralKeyAndFallsBackToTheSingular(t *testing.T) {
	newer := mapstore.Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}}
	if got := newer.DetectedAdapters(); len(got) != 2 || got[0] != "python" || got[1] != "vitest" {
		t.Errorf("DetectedAdapters = %v, want [python vitest]", got)
	}
	prePRD := mapstore.Meta{V: 1, Adapter: "python"}
	if got := prePRD.DetectedAdapters(); len(got) != 1 || got[0] != "python" {
		t.Errorf("DetectedAdapters on a pre-PRD meta = %v, want [python]", got)
	}
	empty := mapstore.Meta{V: 1}
	if got := empty.DetectedAdapters(); len(got) != 0 {
		t.Errorf("DetectedAdapters on an empty meta = %v, want none", got)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./cmd/rtdd/ ./internal/mapstore/ -run 'TestSelectPerAdapter|TestMetaReadsThePluralKey' -count=1
```

Expected: compile failure — `undefined: selectPerAdapter` and
`meta.Adapters undefined (type mapstore.Meta has no field or method Adapters)`.

- [ ] **Step 3: Make it pass.** Add `Adapters` and `DetectedAdapters` to `mapstore.Meta`.
  Add `cmd/rtdd/polyglot.go` with `AdapterSelection` and `selectPerAdapter`, looping the
  detected set and calling the existing `selector.Select` once per adapter with that
  adapter's rows (`TestsCoveringFor`) and that adapter's classification. `rtdd which` prints
  one block per adapter, each headed by the adapter name and its tier.
- [ ] **Step 4: Refactor.** A one-adapter repository must print **exactly** what it printed
  before: the per-adapter heading appears only when more than one adapter was detected.
  Spec §4.1's byte-identical promise is about selection, but a gratuitous output change in
  every Python repo is the kind of churn that makes a golden test worthless.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`, including
  `internal/selector/testdata/selection_golden.txt`, which must not move.

## Task 9 — `cmd/rtdd`: one adapter's failure does not void another's, and the exit-code fold

**Discharges:** spec §4.4 ("one adapter's failure does not void another's selection"). PRD #232 AC7.

**Files:** `cmd/rtdd/polyglot.go`, `cmd/rtdd/run.go`, `cmd/rtdd/exit.go`, `cmd/rtdd/polyglot_run_test.go`

**Interfaces:**

*Consumes:* `runner.Run`, `runner.List`, `ExitCodeFor` (existing).

*Produces:*
```go
// AdapterRun is one adapter's run, kept whole. A failure is data on this struct, never a
// short circuit: the next adapter's selection is still worth running and still worth
// reporting (spec §4.4).
type AdapterRun struct {
	Adapter string
	Result  *runner.RunResult
	Err     error
	Code    int
}

// FoldExitCodes returns the worst code any adapter produced, by decision 5's precedence:
// 3 > 2 > 1 > 0. "Worst" and not "last": a fatal environment error in the first adapter
// must not be reported as success because the second one happened to pass.
func FoldExitCodes(runs []AdapterRun) int
```

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/polyglot_run_test.go`:

```go
package main

import (
	"errors"
	"testing"

	"github.com/VocanicZ/rtdd/internal/runner"
)

// Decision 5's table, asserted. Collapsing 3 or 2 into 1 tells an agent a test failed
// when in fact nothing ran, which is the most expensive possible way to be wrong.
func TestFoldExitCodesTakesTheWorstAcrossAdapters(t *testing.T) {
	cases := []struct {
		name  string
		codes []int
		want  int
	}{
		{"all green", []int{0, 0}, 0},
		{"one test failed", []int{0, 1}, 1},
		{"a config error outranks a test failure", []int{1, 2}, 2},
		{"a fatal environment error outranks everything", []int{1, 2, 3}, 3},
		{"order does not matter", []int{3, 0}, 3},
		{"nothing ran", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := make([]AdapterRun, 0, len(tc.codes))
			for i, c := range tc.codes {
				runs = append(runs, AdapterRun{Adapter: string(rune('a' + i)), Code: c})
			}
			if got := FoldExitCodes(runs); got != tc.want {
				t.Errorf("FoldExitCodes(%v) = %d, want %d", tc.codes, got, tc.want)
			}
		})
	}
}

// PRD #232 AC7: the failure is reported PER ADAPTER, and the other adapter still ran.
func TestOneAdaptersFailureDoesNotVoidAnothersRun(t *testing.T) {
	runs := []AdapterRun{
		{Adapter: "maven", Err: errors.New("mvn: command not found"), Code: 3},
		{Adapter: "vitest", Result: &runner.RunResult{ExitCode: 0}, Code: 0},
	}
	out := renderAdapterRuns(runs)
	for _, want := range []string{"maven", "mvn: command not found", "vitest"} {
		if !contains(out, want) {
			t.Errorf("rendered runs %q do not name %q; AC7 requires the failure reported per adapter", out, want)
		}
	}
	if got := FoldExitCodes(runs); got != 3 {
		t.Errorf("FoldExitCodes = %d, want 3: one adapter's environment is broken", got)
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (hay == needle || len(needle) == 0 || indexOf(hay, needle) >= 0)
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./cmd/rtdd/ -run 'TestFoldExitCodes|TestOneAdaptersFailureDoesNotVoid' -count=1
```

Expected: compile failure — `undefined: AdapterRun`, `undefined: FoldExitCodes`,
`undefined: renderAdapterRuns`.

- [ ] **Step 3: Make it pass.** Add `AdapterRun`, `FoldExitCodes` and `renderAdapterRuns`.
  `rtdd run` loops the detected set, runs each adapter's selection to completion,
  **recovers** rather than returns on a per-adapter error, prints one block per adapter and
  exits `FoldExitCodes`. `ExitCodeFor` stays the single-adapter rule and is what each block
  contributes.
- [ ] **Step 4: Refactor.** Decision 12's consequence: for a `report: junit-xml` adapter a
  T2 escalation already ran the suite to enumerate it, so `rtdd run` must use **those**
  outcomes instead of re-running the same suite as a subset. Guard it with a test that
  counts invocations of a stub runner and asserts one, not two.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. `cmd/rtdd/exit_test.go`'s
  existing single-adapter assertions must still hold unchanged.

## Task 10 — `cmd/rtdd`: ten fixture repositories and the `rtdd which` integration test

**Discharges:** spec §8 (each shipped adapter is proved against a real repository layout) and §4.4 (the polyglot fixture). PRD #232 AC9 and AC10.

**Files:** `cmd/rtdd/testdata/fixtures/vitest/`, `cmd/rtdd/testdata/fixtures/jest/`, `cmd/rtdd/testdata/fixtures/go/`, `cmd/rtdd/testdata/fixtures/cargo-nextest/`, `cmd/rtdd/testdata/fixtures/maven/`, `cmd/rtdd/testdata/fixtures/gradle/`, `cmd/rtdd/testdata/fixtures/rspec/`, `cmd/rtdd/testdata/fixtures/dotnet/`, `cmd/rtdd/testdata/fixtures/phpunit/`, `cmd/rtdd/testdata/fixtures/polyglot/`, `cmd/rtdd/fixtures_test.go`

**Interfaces:**

*Consumes:* `adapter.Detect` (Task 1), `selectPerAdapter` (Task 8), `selector.TierTS` (existing).

*Produces:* ten committed minimal repositories. Each is the smallest tree that (a) detects
its adapter, (b) has one source file and one corresponding test file the adapter's first
`test_for` template resolves to, and (c) nothing else. **No runner is executed:** these
fixtures prove detection and *selection*, which is what `rtdd which` answers. Proving a
runner accepts the id needs the runner installed, and spec §7's evidence PRD (#233) owns it.

| fixture | markers committed | changed source | test file `rtdd which` must name |
|---|---|---|---|
| `testdata/fixtures/vitest/` | `vitest.config.ts`, `package.json` | `src/calc.ts` | `src/calc.test.ts` |
| `testdata/fixtures/jest/` | `jest.config.js`, `package.json` | `src/calc.js` | `src/calc.test.js` |
| `testdata/fixtures/go/` | `go.mod` | `calc/calc.go` | `calc/calc_test.go` |
| `testdata/fixtures/cargo-nextest/` | `.config/nextest.toml`, `Cargo.toml` | `src/calc.rs` | `tests/calc.rs` |
| `testdata/fixtures/maven/` | `pom.xml` | `src/main/java/calc/Calc.java` | `src/test/java/calc/CalcTest.java` |
| `testdata/fixtures/gradle/` | `build.gradle.kts` | `src/main/java/calc/Calc.java` | `src/test/java/calc/CalcTest.java` |
| `testdata/fixtures/rspec/` | `.rspec`, `Gemfile` | `lib/calc.rb` | `spec/calc_spec.rb` |
| `testdata/fixtures/dotnet/` | `Calc.sln`, `Calc.csproj` | `Calc/Calc.cs` | `tests/Calc.Tests/CalcTests.cs` |
| `testdata/fixtures/phpunit/` | `phpunit.xml`, `composer.json` | `src/Calc.php` | `tests/CalcTest.php` |
| `testdata/fixtures/polyglot/` | `package.json`, `vitest.config.ts`, `pom.xml` | `src/calc.ts` **and** `src/main/java/calc/Calc.java` | `src/calc.test.ts` (vitest) **and** `src/test/java/calc/CalcTest.java` (maven) |

The polyglot fixture carries a `vitest.config.ts` **as well as** its `package.json`, because
decision 1 makes `package.json` alone detect nothing. That is the fixture being honest about
the rule, not a workaround: PRD #232 AC10 asks for a repository holding both a
`package.json` and a `pom.xml` that detects **exactly two adapters**, and this one does —
`vitest` and `maven`, never `jest`, never three.

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/fixtures_test.go`:

```go
package main

import (
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// PRD #232 AC9: one committed minimal repository per shipped adapter, and a non-empty TS
// selection naming the expected test file. A shipped adapter nobody ever pointed at a real
// tree is a YAML file that compiles, not a supported language.
func TestEachFixtureRepoDetectsItsAdapterAndSelectsItsTest(t *testing.T) {
	cases := []struct{ fixture, wantAdapter, changed, wantTest string }{
		{"vitest", "vitest", "src/calc.ts", "src/calc.test.ts"},
		{"jest", "jest", "src/calc.js", "src/calc.test.js"},
		{"go", "go", "calc/calc.go", "calc/calc_test.go"},
		{"cargo-nextest", "cargo-nextest", "src/calc.rs", "tests/calc.rs"},
		{"maven", "maven", "src/main/java/calc/Calc.java", "src/test/java/calc/CalcTest.java"},
		{"gradle", "gradle", "src/main/java/calc/Calc.java", "src/test/java/calc/CalcTest.java"},
		{"rspec", "rspec", "lib/calc.rb", "spec/calc_spec.rb"},
		{"dotnet", "dotnet", "Calc/Calc.cs", "tests/Calc.Tests/CalcTests.cs"},
		{"phpunit", "phpunit", "src/Calc.php", "tests/CalcTest.php"},
	}
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			root := filepath.Join("testdata", "fixtures", tc.fixture)
			got, err := adapter.Detect(root, all)
			if err != nil {
				t.Fatalf("Detect(%s): %v", root, err)
			}
			if len(got) != 1 || got[0].Name != tc.wantAdapter {
				t.Fatalf("Detect(%s) = %v, want exactly [%s]", root, adapterNames(got), tc.wantAdapter)
			}
			tests := whichTestsForFixture(t, root, got, tc.changed)
			if len(tests) == 0 {
				t.Fatalf("rtdd which selected nothing for %s; a shipped adapter must narrow its own fixture", tc.changed)
			}
			if !containsString(tests, tc.wantTest) {
				t.Errorf("selection %v does not name %q", tests, tc.wantTest)
			}
		})
	}
}

// PRD #232 AC10: a package.json plus a pom.xml detects EXACTLY two adapters, and both
// contribute a selection.
func TestPolyglotFixtureDetectsExactlyTwoAdaptersAndSelectsFromBoth(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	root := filepath.Join("testdata", "fixtures", "polyglot")
	got, err := adapter.Detect(root, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Detect = %v, want exactly two adapters", adapterNames(got))
	}
	if !containsString(adapterNames(got), "vitest") || !containsString(adapterNames(got), "maven") {
		t.Fatalf("Detect = %v, want vitest and maven", adapterNames(got))
	}
	ts := whichTestsForFixture(t, root, got, "src/calc.ts")
	if !containsString(ts, "src/calc.test.ts") {
		t.Errorf("vitest half selected %v, want src/calc.test.ts", ts)
	}
	jv := whichTestsForFixture(t, root, got, "src/main/java/calc/Calc.java")
	if !containsString(jv, "src/test/java/calc/CalcTest.java") {
		t.Errorf("maven half selected %v, want src/test/java/calc/CalcTest.java", jv)
	}
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./cmd/rtdd/ -run 'TestEachFixtureRepoDetects|TestPolyglotFixtureDetects' -count=1
```

Expected: compile failure — `undefined: whichTestsForFixture`, `undefined: adapterNames`.
Once those exist, every subtest fails with
`Detect(testdata/fixtures/vitest): adapter: no adapter detected` because the fixtures are
not committed yet.

- [ ] **Step 3: Make it pass.** Commit the ten trees exactly as the table specifies. Add
  `whichTestsForFixture`, a thin helper over `selectPerAdapter` that returns the flat id
  list for the adapter whose classification owns the changed file, and `adapterNames`.
  Fixture files may be empty or near-empty — detection reads names, and `TestForCandidate`
  asks only whether the path exists.
- [ ] **Step 4: Refactor.** Add a `README.md` per fixture directory naming the adapter it
  proves and the one changed-file → test-file correspondence it demonstrates. Under
  `testdata/`, the Go tool ignores the nested `go.mod` and `Cargo.toml`, so the fixtures
  cannot affect the build; say so in the `go` fixture's README, because a nested `go.mod`
  reads as a mistake to anyone who has not met that rule.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. Confirm `go build ./...` and
  `go vet ./...` still see one module.

## Task 11 — `cmd/rtdd`: `rtdd seed` in a polyglot repository

**Discharges:** spec §4.4 (a mixed repository is ordinary, and every command has to say what it did for each half).

**Files:** `cmd/rtdd/seed.go`, `cmd/rtdd/staticadvice.go`, `cmd/rtdd/seed_polyglot_test.go`

**Interfaces:**

*Consumes:* `selectionSplit`, `staticNothingRecorded`, `seedScope` (all existing in `cmd/rtdd/staticadvice.go`).

*Produces:* no new type. One changed policy (decision 6) and the sentence that reports it.

- [ ] **Step 1: Write the failing test**

`cmd/rtdd/seed_polyglot_test.go`:

```go
package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// Decision 6: seeding a mixed repo seeds the coverage half and NAMES the static half. The
// existing refusal is for a repo whose coverage half is empty; refusing here would strand
// the map the Python half genuinely needs.
func TestSeedInAMixedRepoSeedsTheCoverageAdaptersAndNamesTheStaticOnes(t *testing.T) {
	detected := []*adapter.Adapter{
		{Name: "python", Selection: adapter.SelectionCoverage, Coverage: "sqlite", Seed: "pytest --cov"},
		{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
	}
	static, coverage := selectionSplit(detected)
	if len(static) != 1 || len(coverage) != 1 {
		t.Fatalf("selectionSplit = static %v, coverage %v, want one each", adapterNames(static), adapterNames(coverage))
	}

	plan, msg := seedPlan(detected)
	if len(plan) != 1 || plan[0].Name != "python" {
		t.Fatalf("seedPlan = %v, want only the coverage adapter", adapterNames(plan))
	}
	for _, want := range []string{"vitest", "no map is ever built", "python"} {
		if !strings.Contains(msg, want) {
			t.Errorf("seed message %q does not name %q", msg, want)
		}
	}
}

// The all-static repo keeps the existing exit-2 refusal: there is genuinely nothing to do.
func TestSeedInAnAllStaticRepoStillRefuses(t *testing.T) {
	detected := []*adapter.Adapter{
		{Name: "vitest", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
		{Name: "maven", Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone},
	}
	plan, msg := seedPlan(detected)
	if len(plan) != 0 {
		t.Fatalf("seedPlan = %v, want nothing to seed", adapterNames(plan))
	}
	if !strings.Contains(msg, "vitest") || !strings.Contains(msg, "maven") {
		t.Errorf("refusal %q must name both static adapters", msg)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./cmd/rtdd/ -run 'TestSeedInAMixedRepo|TestSeedInAnAllStaticRepo' -count=1
```

Expected: compile failure — `undefined: seedPlan`. `selectionSplit` already exists and
already returns the partition, so only the policy on top of it is missing.

- [ ] **Step 3: Make it pass.** Add `seedPlan(detected) ([]*adapter.Adapter, string)` beside
  the existing partition helpers, and call it from `rtdd seed`: a non-empty coverage half
  seeds exactly those adapters, prints the static-half sentence and exits 0; an empty
  coverage half keeps today's exit-2 refusal word for word.
- [ ] **Step 4: Refactor.** `.rtdd/meta.json`'s `Adapter` records the coverage adapter that
  produced the map and `Adapters` records the whole detected set (decision 4). Write both in
  `seed`, so decision 3's untagged-row rule has the answer it needs in every repo seeded
  from here on.
- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`, including
  `cmd/rtdd/seed_static_test.go` and `cmd/rtdd/staticadvice_test.go`, whose all-static
  assertions must be unchanged.

## Task 12 — `internal/contract`: the freeze guard, the plan guard, and the CI gate

**Discharges:** spec §8 (the shipped set is a contract, and so is what it may not touch). PRD #232 AC11.

**Files:** `internal/contract/adapter_freeze_test.go`, `internal/contract/shipped_adapters_test.go`

**Interfaces:**

*Consumes:* `readRepoFile` (existing), `pythonAdapterSHA256` (existing, unchanged).

*Produces:* two guards over the whole milestone.

- [ ] **Step 1: Write the failing test**

`internal/contract/shipped_adapters_test.go` (append):

```go
package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The freeze is a global constraint of the M6d plan, and a global constraint nothing
// checks is a paragraph. If a later task "adds the new keys" to the Python adapter, the
// digest test fires — and this test says WHY, so the fix is reverting the edit rather than
// updating the constant.
func TestNoShippedAdapterEditIsAllowedToTouchThePythonAdapter(t *testing.T) {
	src := readRepoFile(t, "internal/contract/adapter_freeze_test.go")
	const want = `pythonAdapterSHA256 = "a20c009fed0db285f4cfe04d0bbb1752fb6e0beca9a469736cb5f0eedf3df123"`
	if !strings.Contains(src, want) {
		t.Errorf("the python adapter digest changed; PRD #232 licenses no edit to adapters/python.yaml (M6d global constraints)")
	}
	yaml := readRepoFile(t, "adapters/python.yaml")
	for _, key := range []string{"test_flag:", "test_join:", "report_cmd:"} {
		if strings.Contains(yaml, "\n"+key) {
			t.Errorf("adapters/python.yaml declares %q; every M6d key is optional and python declares none", key)
		}
	}
}

// PRD #232 AC5, stated once more where a reviewer will look for it: the v1 refusal is not
// in the tree, and no adapter YAML re-introduces the one-adapter assumption by claiming a
// marker that belongs to another ecosystem.
func TestNoTwoShippedAdaptersShareADetectMarker(t *testing.T) {
	src := readRepoFile(t, "docs/plans/06-m6d-shipped-adapters.md")
	if !regexp.MustCompile(`(?m)^\| ` + "`vitest`" + ` \|`).MatchString(src) {
		t.Fatal("the plan's decision-1 marker table is missing; the shipped set's detection rule has no written source")
	}
	// The real assertion is over the loaded adapters, not the plan: two adapters sharing a
	// marker makes every repo of that ecosystem detect both, which is decision 1's defect.
	seen := map[string]string{}
	all := builtinForTest(t)
	for _, a := range all {
		for _, m := range a.Detect {
			if prev, dup := seen[m]; dup {
				t.Errorf("adapters %s and %s both detect on %q; every repo of that ecosystem would match both (decision 1)", prev, a.Name, m)
			}
			seen[m] = a.Name
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestNoShippedAdapterEditIsAllowed|TestNoTwoShippedAdaptersShareADetectMarker' -count=1
```

Expected: compile failure — `undefined: builtinForTest`. After the helper exists, the
second test fails on `docs/plans/06-m6d-shipped-adapters.md` only if the marker table was
edited away; with the nine adapters as specified it passes.

- [ ] **Step 3: Make it pass.** Add `builtinForTest`, a two-line wrapper over
  `adapter.Builtin` that fails the test on error. Fix any shared marker by narrowing the
  adapter's `detect` list, never by deleting the assertion.
- [ ] **Step 4: Refactor.** Nothing. These two tests exist to be inconvenient.
- [ ] **Step 5: Run the full CI gate.** PRD #232 AC11:

```bash
scripts/ci-local.sh
```

It must exit **0**: `go build ./...`, `go vet ./...`, `gofmt -l .` empty, `go test ./...`
green, the `CGO_ENABLED=0` static binary check, and the cross-built release-artifact check.
Its exit code is the verdict; `gh pr checks` is not a second opinion.

## Definition of Done

- [ ] `adapters/` holds ten declarations; `adapters/python.yaml` is byte-identical and its
      digest constant is unchanged.
- [ ] `adapter.Detect` returns `([]*Adapter, error)`; zero matches is still an error; the
      string `polyglot repos are out of scope in v1` appears nowhere in the tree.
- [ ] `internal/contract`'s ten-row table passes, and a half-specified adapter fails it.
- [ ] Every shipped `id_template` round-trips over a real capture, and the three
      file-granular adapters fold a file to one worst-status-wins outcome.
- [ ] A map row carries its adapter; an untagged row is served only to the adapter
      `.rtdd/meta.json` names; a pre-PRD `map.jsonl` round-trips with no `"a"` key added.
- [ ] `rtdd which` prints one block per detected adapter and never mixes two adapters' ids.
- [ ] `rtdd run` runs every adapter to completion, reports each failure under its adapter's
      name, and exits the worst code any adapter produced.
- [ ] Ten fixture repositories are committed; nine detect exactly their own adapter and get
      a non-empty `TS` selection naming the expected test file; the polyglot one detects
      exactly two and selects from both.
- [ ] `rtdd seed` in a mixed repo seeds the coverage half, names the static half, exits 0.
- [ ] `scripts/ci-local.sh` exits 0.

## What this plan deliberately leaves undone

Everything below is real work the multi-language design wants; none of it belongs to
PRD #232, and no task above may start it. Each line names the milestone from spec §8 and
the sibling or later PRD that owns it — a scope this plan may not silently absorb.

- **The measured evidence — spec §7, milestone M6e, sibling PRD #233.** The static tier's
  change-level recall, selection ratio and selected-duration fraction against the
  coverage-derived, `path` and T2 baselines over the flask and httpie replay corpus, with
  `p50`/`p90`/`worst` and never a bare mean, is not measured here and **nothing in this plan
  or the code it produces may claim the static tier beats the `path` baseline.** Nine
  adapters existing is not evidence that they select well; §7 pre-registers what would be.
- **The front-end fidelity statements — spec §6, milestone M6e, sibling PRD #233.**
  `selection_fidelity` on `--json`, the `tier: TS (static)` suffix on `rtdd which`/`rtdd
  run` human output, the static-tier caveat in the `warnings` array, the `PROTOCOL.md` →
  `SKILL.md`/`AGENTS.md`/`.mdc` regeneration, and README's "It is Python only" are #233's.
  This plan adds a per-adapter *block* to `which` and `run` because a polyglot repo has more
  than one answer; it does not add the fidelity vocabulary those blocks will eventually
  carry.
- **Per-test coverage for any of these ecosystems — spec §11, audit finding A5.** It stands
  and it stays deferred: no Vitest coverage provider, no injected `TestMain`, no
  `ClearCounters()` codegen, no per-test-file isolation (measured at 27.5×). Every adapter
  shipped here is `selection: static` **by design**, and `coverage: none` is the honest
  declaration of that, not a placeholder for a later value.
- **Proving a real runner accepts a rendered id.** Task 6 proves render→parse→render
  stability over each runner's own captured report; it does not install Vitest, Maven or
  `dotnet` and run them. That needs nine toolchains in CI, which is an infrastructure
  decision this PRD is not the place to make, and #233's measured evidence is where a real
  invocation first has to happen.
- **Languages outside the shipped set.** Elixir, Swift, Scala, Haskell and everything else
  are supported the way spec §4.5 says: a YAML file in `.rtdd/adapters/`, which PRD #229
  already delivered. Adding a tenth built-in is a separate decision, and the shipped set
  being a convenience rather than the boundary of support is the design's own claim.
- **`importscan` for any shipped adapter.** None of the nine declares one, so the `TS`
  tier's level 2 is skipped for all of them and their selections rest on `test_for`
  correspondence alone. Shipping a per-language import scanner — including resolving a
  `script` from the embedded built-in FS, which plan 06-m6b deferred to this milestone —
  is real work with its own measurement, and it is deferred again rather than smuggled in:
  a narrower static tier is honest, and a scanner nobody measured is not.
