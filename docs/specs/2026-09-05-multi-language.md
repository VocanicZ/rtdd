# RTDD multi-language design — selection fidelity as an adapter property

**Status:** design, not yet implemented.
**Extends:** `docs/specs/2026-08-26-rtdd-design.md` (the v1 spec). Section references below
(`§4`, `§8`, `§11`, `D8`) are to that document.
**Supersedes:** the "JS and Go adapters" bullet of §11 Deferred, for the static tier only.

## 1. The problem this solves

`rtdd init` installs `.claude/skills/rtdd/SKILL.md`, `.cursor/rules/rtdd.mdc` and an
`AGENTS.md` block into **any** repository. It performs no adapter detection
(`cmd/rtdd/init.go` calls `adapter.Detect` nowhere). Only `cmd/rtdd/rows.go:46` detects, so
in a non-Python repository the agent-facing front-ends are installed and every subsequent
answer is:

```
tier: T2  (0 tests selected, ranked)
NOTHING SELECTED — this is not the same as "all passed".
NOTE: no adapter (adapter: no adapter detected in <root>) - file classification is
      disabled: no changed file can be recognised as a test file, so the direct tier is empty
```

T2 means *run the full suite*. So today RTDD is installed as a language-agnostic agent skill
whose permanent answer outside Python is the null baseline its own benchmark measures
against. This is verified behaviour, reproduced on a scratch TypeScript repository, not a
supposition.

## 2. What is and is not achievable

Audit finding A5 is not disputed and is not worked around here. Per-test attribution does
not exist in the JavaScript or Go ecosystems: Istanbul/v8 entries carry aggregate counters
with no test dimension, lcov's `TN:` is emitted empty, and per-test-*file* isolation was
measured at **27.5×** (2.57 s → 77.8 s for 30 files). Go's default `-coverprofile` records
count 0 for every package but the one under test, and per-test attribution needs a prebuilt
`-c` binary driven once per test (7 ms floor, `TestMain` re-run each time) or codegen in the
user's repository.

**A5 establishes that execution-derived, per-test selection is Python-only. It does not
establish that RTDD is Python-only.** Those were conflated. Per-test coverage is required
for the *"derived from real execution"* claim that is RTDD's differentiator; it is not
required to give an agent a useful narrowing of the suite.

The design therefore splits one axis into two:

| | selection fidelity | requires |
|---|---|---|
| **execution-derived** | tests whose *recorded coverage* intersects the changed set | per-test coverage (Python) |
| **static** | tests whose *declared correspondence or imports* reach the changed set | nothing but git and globs |

Both are strictly better than T2. `path` and `importgraph` already exist as baselines in
`bench/` and are measurable: on flask the `path` baseline scored change-level recall 0.333
at a selected-duration fraction of 0.010. Against T2 (selection ratio 1.0) that is 33 % of
regressions caught for 1 % of the test time. Against RTDD's coverage tier it is worse. Both
comparisons must be published; §7 says how.

## 3. Non-goals

- **Not** replicating per-test coverage in any non-Python ecosystem. No Vitest coverage
  provider, no injected `TestMain`, no `ClearCounters()` codegen. §11 keeps those deferred.
- **Not** claiming the static tier is equivalent to the coverage tier. Every surface that
  reports a static selection must say so (§6).
- **Not** method-level checksums or time-budgeted selection. Still deferred.

## 4. Architecture

### 4.1 A new tier, `TS`

`internal/selector` currently resolves, in order: T2 escalations, T1 escalations, T0, empty
(`internal/selector/select.go:18-21`). `TS` is inserted **between T1 and T2**: when the map
cannot answer (unseeded, or the adapter declares `selection: static`), the selector attempts
static correspondence before escalating to the full suite.

`TS` never overrides a usable coverage relation. In a Python repository with a seeded map,
behaviour is byte-identical to today. This is a regression requirement, not a preference.

Ranking within `TS`, most to least confident:
1. **direct correspondence** — a `test_for` template resolves to an existing test file;
2. **import distance** — the test file transitively imports the changed file, ranked by
   shortest path;
3. **path proximity** — shared directory prefix, longest prefix first.

### 4.2 Adapter contract v2

Today's adapter YAML (`adapters/python.yaml`) is already mostly language-agnostic: `detect`,
`env`, `seed`, `subset`, `list`, `failfast_flag`, `test_globs`, `source_globs`, `exit_codes`,
`opaque`, `full_escalate`. Only `coverage: sqlite` and `report: pytest-reportlog` dispatch to
Python-specific Go code. v2 adds:

```yaml
selection: static            # "coverage" (default, today's behaviour) | "static"
coverage: none               # new permitted value; required when selection: static
report: junit-xml            # new universal parser (see 4.3)
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"   # how a parsed <testcase> renders into a REPORTED id
                             # (the reporting vocabulary — see test_selector below)
test_for:                    # path-correspondence templates, tried in order
  - "{dir}/{name}.test.ts"     # {dir}    = the changed file's directory
  - "{dir}/__tests__/{name}.test.ts"
  - "tests/{name}.test.ts"
  - "src/test/java/{subdir}/{name}Test.java"  # {subdir} = {dir} or any TRAILING part of
                               # it, longest first. A test tree usually mirrors a suffix
                               # of the source tree — src/main/java/calc/Calc.java
                               # corresponds to src/test/java/calc/CalcTest.java — and how
                               # many leading segments the source root occupies is a
                               # property of the repository, not of the adapter. A
                               # candidate still counts only when the file exists, so the
                               # longest suffix that names something wins.
importscan:
  command: "node {script}"   # optional; omitted means import ranking is skipped
  script: "scan-imports.mjs" # shipped beside the adapter, run like internal/importscan does

test_flag: "--tests"         # emit "<flag> <id>" for each id at the {tests} position
test_join: ","               # OR join every id into ONE argv token, substituted wherever
                             # {tests} appears inside a token (`-Dtest={tests}`)

test_selector: "./{dir}"     # how ONE selected test FILE becomes ONE selector token.
                             # {file} = the test file, repo-relative; {dir} = its
                             # directory; {name} = its base name without extension.
                             # selection: static only; omitted means the identity.
```

`test_flag` and `test_join` are optional and **mutually exclusive** — declaring both is a
load-time error. Declaring neither is today's rule unchanged: one bare argv element per id
at the token that is exactly `{tests}`, which is what pytest, vitest, jest, rspec and
nextest all take. Gradle needs its flag before *each* id; Surefire, PHPUnit, `dotnet test`
and `go test -run` each take one argument holding every id joined by a separator. Under
`test_join`, an id containing the separator is refused by name rather than spliced into a
token that would split back into two selectors.

`test_selector` is the SELECTION vocabulary, and it is deliberately not `id_template`. The
`TS` tier's output is a test **file path** — that is what `test_for` correspondence resolves
to — while `id_template` renders a `<testcase>` from a report that, on the `TS` path, has
not been written yet. Six of the nine shipped runners select by test **name**, so without a
declared translation a file path is spliced into `-run`, `-Dtest=`, `--tests` or `--filter`,
the runner matches nothing, and the run reports a pass over zero executed tests (#334). The
key is permitted only under `selection: static`, because a coverage adapter's ids come from
the map and are already selectors; omitting it is the identity, which is every v1 adapter
unchanged. Rendering happens in `runner.Run` before chunking, so the argv byte budget
measures what is actually spliced and two files that render one selector collapse to one.

`seed` becomes optional when `selection: static` — there is nothing to seed.

**D8 is preserved, not weakened.** The YAML still declares only what is genuinely
declarative; execution and parsing stay in Go. `importscan` follows the precedent already set
by `internal/importscan`, whose header states the engine "must not parse Python itself, so
the scan runs as an embedded Python script". Per-language scanners are scripts, not engine
branches.

### 4.3 `report: junit-xml` — the universal outcome format

The static tier still has to know which tests ran and which failed. JUnit XML is the closest
thing to a universal outcome format, but "essentially every runner emits it" is too loose to
build on: what each runner needs differs, and an adapter that assumes a flag exists fails on
a clean machine. The three cases are distinct and an adapter must declare which it is in:

| runner | JUnit XML via | needs installing |
|---|---|---|
| Maven Surefire | `target/surefire-reports/*.xml`, written by default | no |
| Gradle | `build/test-results/test/*.xml`, written by default | no |
| PHPUnit | `--log-junit <path>` | no |
| pytest | `--junitxml=<path>` | no |
| Vitest | `--reporter=junit --outputFile=<path>` | no |
| cargo-nextest | `[profile.<p>.junit] path` in `.config/nextest.toml` — **not** a CLI flag | nextest itself |
| Jest | `--reporters=jest-junit` | `jest-junit` |
| RSpec | `--format RspecJunitFormatter` | `rspec_junit_formatter` |
| Go | piping `go test -json` through `go-junit-report` | `go-junit-report` |
| dotnet | `--logger junit` | `JUnitTestLogger`; only `trx` is built in |

Four of the ten need a package the host repo may not have, and cargo-nextest needs a config
file rather than a flag. So the contract gains one more key:

```yaml
requires:                    # optional; prerequisites the host repo must already have
  - bin: go-junit-report     # a binary that must resolve on PATH
    reason: "go emits no JUnit XML natively"
```

`rtdd doctor` reports an unmet `requires` as a named, actionable finding — the prerequisite,
which adapter needs it, and why. An unmet prerequisite must surface at `doctor` and `init`
time, **never** as a mid-run parse failure against a report file that was never written.

One parser in `internal/report` covers all ten. The hazard is **id round-tripping**: a
JUnit `<testcase classname= name=>` pair must render back into something the runner's own
selector syntax accepts. The valid placeholders are exactly **`{file}`, `{classname}` and
`{name}`** — `{classname}` matches the JUnit attribute's own spelling, and `{class}` is
deliberately *not* an alias for it, because two spellings for one attribute is a trap for
adapter authors. A template naming `{file}` against a runner that emits only `classname=`
fails at run time with a message saying so, and adjacent placeholders with no literal
separator (`"{classname}{name}"`) are rejected at load time: they render but cannot be read
back, so the round-trip is not stable.

A template need not name one test. Vitest and RSpec have no single-token selector for one
case, so their adapters render a **file-granular** id that every case in the file shares.
Cases of one report file that render the same id therefore fold into one outcome, and the
fold is **worst-status-wins** — `error` > `fail` > `skip` > `pass` — so an id is green only
when every case behind it passed. (The same id in two files of one *directory* report is
not this: nothing downstream can tell those apart, so it is an error naming both files.)

That is what `id_template` is for, and it is per-adapter because
only the adapter knows its runner's syntax. Audit finding A6 ("three of five parse formats
structurally cannot carry a test identifier") is the reason this is specified explicitly
rather than assumed.

### 4.4 Polyglot repositories

`adapter.Detect` (`internal/adapter/detect.go:44-51`) errors on zero matches and errors on
two or more ("polyglot repos are out of scope in v1"). A TypeScript service with a Python
tooling directory is ordinary, so v2 permits multiple active adapters. §11 already named the
requirement: "rows would need an adapter tag." Map rows, selections and subset invocations
each carry the adapter that produced them, and one adapter's failure does not void another's
selection.

### 4.5 User-authorable adapters

**This is what makes "every language Claude can write" true, and it is the load-bearing part
of the design.** Adapters load from `.rtdd/adapters/*.yaml` in the host repository in
addition to the `go:embed`-ed built-ins. A language RTDD has never heard of is supported by
writing YAML — no engine change, no release. `rtdd doctor` validates host adapters and names
the failing field.

The shipped set is a convenience, not the boundary of support.

## 5. `init` must gate

`rtdd init` runs `adapter.Detect` before writing anything.

- **≥1 adapter:** proceed; record the detected adapters and their `selection` in
  `.rtdd/config.yaml`; the installed front-ends state the fidelity (§6).
- **0 adapters:** do **not** silently install. Exit 2 with a message naming the languages
  detected-but-unsupported and pointing at `.rtdd/adapters/`. `--force` overrides, and then
  the installed `SKILL.md` must carry the no-adapter caveat in its first paragraph.

Installing agent instructions for a tool that cannot function is the defect this closes.

## 6. Honesty surfaces

An agent cannot calibrate on a number it is not given. Every surface reports fidelity:

- `rtdd which` / `rtdd run` human output: the tier line reads `tier: TS (static)` and the
  reason names correspondence or imports, never "recorded coverage".
- `--json`: a new top-level `selection_fidelity` field, `"execution-derived" | "static" |
  "none"`. Never null.
- The existing `warnings` array gains a static-tier caveat: a static selection can miss a
  test that execution-derived selection would have caught, and passing it is weaker evidence.
- `rtdd doctor` reports, per detected adapter, which fidelity this repository can achieve and
  why — the single command a human or agent runs to find out where they stand.
- `PROTOCOL.md` → `SKILL.md` / `AGENTS.md` / `.mdc` regeneration carries the fidelity
  statement, so the agent reads it before it reads a selection.

## 7. Evidence

The static tier ships with its cost measured, not asserted. The measurement reuses the
existing corpus and the durable `~/.cache/rtdd-bench` ground truth — **no new benchmark
infrastructure and no re-earned cache.**

Run the static tier as an additional strategy over the *same* flask and httpie replay corpus
where the coverage-derived answer is already known, and publish, per repo:

- change-level recall, static vs coverage-derived vs `path` vs T2;
- selection ratio and selected-duration fraction for each;
- the wall-clock distribution as `p50` / `p90` / `worst`, never a bare mean (PRD criterion
  from issue #214 applies to any new table).

If the static tier does not beat the `path` baseline it is not worth shipping as a
distinct tier, and the README says so. That is the same pre-registration discipline §12
applies to M3 and M4.

## 8. Milestones

| # | Slice | Done when |
|---|---|---|
| **M6a** | Adapter contract v2 + `init` gating | `selection`/`coverage: none`/`report_path`/`id_template`/`test_for`/`importscan`/`requires` parse and validate; host `.rtdd/adapters/*.yaml` load; `init` refuses a no-adapter repo without `--force`; `doctor` reports fidelity and any unmet prerequisite |
| **M6b** | The `TS` tier | Tier inserted between T1 and T2; three-level ranking; a seeded Python repo's selection is byte-identical to before |
| **M6c** | `report: junit-xml` + runner | One parser, id round-trip through `id_template`, subset invocation, exit-code mapping |
| **M6d** | Shipped adapters + polyglot | Adapters for the languages below; `Detect` returns a set; rows and selections carry an adapter tag |
| **M6e** | Evidence + front-end honesty | The §7 table published; `selection_fidelity` on every surface; README's "It is Python only" replaced by the two-tier statement |

**Shipped adapter set for M6d** — chosen because each has a standard runner with JUnit XML
output and a conventional test layout, which is what the contract needs:
TypeScript/JavaScript (Vitest and Jest), Go, Rust (`cargo nextest`), Java/Kotlin (Gradle and
Maven Surefire), Ruby (RSpec), C# (`dotnet test`), PHP (PHPUnit). Python keeps its existing
coverage adapter unchanged.

This list is the convenience set. §4.5 is the actual answer to "every language Claude can
write"; a language outside this list is supported by a YAML file in the host repo.

## 9. What this design deliberately leaves undone

- Per-test coverage anywhere but Python. A5 stands; §11 stays deferred.
- Method-level checksums, time-budgeted selection.
- Any claim that a static selection is as trustworthy as an execution-derived one. It is
  not, it is measured in §7, and §6 makes every surface say so.
