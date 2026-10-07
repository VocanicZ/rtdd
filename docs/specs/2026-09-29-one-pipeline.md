> **Superseded** by [`2026-10-07-node-graph.md`](2026-10-07-node-graph.md) (v0.3.0). Kept as the record of the design it describes; do not implement from it.

# One pipeline for every language

Status: approved design, 2026-09-29. Supersedes the two-tier design of
[`2026-09-05-multi-language.md`](2026-09-05-multi-language.md) (static tier for non-Python,
per-test coverage contexts for Python) and the Python-only conclusion of audit finding A5.

## 1. Goal

Every language and every project gets the same selection quality, from the same pipeline.
Selection is always derived from coverage recorded by a real run of the project's own tests.
There is no static tier, no language-specific fast path, and no second pipeline — Python
included.

Success is measured, not asserted:

- Every shipped adapter seeds a map and reaches the same `rtdd which` / `rtdd run` behaviour.
- On rtdd-bench, every row that ran at `static` runs on the recorded map, and time in tests
  is reported against `/tdd`. A row that is slower is published as slower; it is not fixed by
  adding a per-language fast path.

## 2. Why this is possible now

A5 showed that JS and Go coverage tools carry no per-test dimension. That is true inside one
process. It stops mattering when each unit runs in its own process: the stock coverage tool's
aggregate output *is* that unit's coverage. A5 rejected isolation on cost (27.5× for 30 Jest
files, run serially). This design accepts the cost at seed time, runs units in parallel, and
lets rtdd-bench decide whether the edit loop is still worth it.

## 3. The unit

The unit of selection is the **test file**. A map row is one test file and the source files
its run executed.

Consequences:

- Selection is file-level on both sides: a changed source file selects every test file whose
  run touched it; the whole test file runs.
- Python maps today hold test-case ids (`tests/test_a.py::test_x`). They are not migrated:
  the map format version rises to 2 and an older map is treated as unseeded (T2, with a
  reason telling the user to run `rtdd seed`).

## 4. The pipeline

The same five steps for every adapter. `rtdd seed` runs them over all units; `rtdd run` over
the selected units; a T2 cycle over all units.

1. **Enumerate.** Units are the tracked and untracked files matching the adapter's
   `test_globs`. No runner is invoked to list tests.
2. **Execute in isolation.** Each unit runs as its own process from the repo root, with a
   fresh private temp directory `{tmp}`. Up to `jobs` units run at once (default: CPU count).
3. **Read coverage.** After the process exits, rtdd reads the adapter's `coverage_file`
   (a path under `{tmp}`) in the adapter's `coverage_format`, producing
   `file -> hit lines` for that unit.
4. **Outcome from the exit code.** 0 is pass, 1 is fail. Any other code is looked up in the
   adapter's `exit_codes`: a code labelled `no-tests-collected` (pytest's 5) is a skip, any
   other listed code (pytest's 4, arguments rejected) is fatal, and an unlisted one is an
   error. A unit that exits 0 or 1 but leaves no coverage file is an error — it
   never reads as a pass. A failing unit's captured output (tail) is shown.
5. **Update the map.** One row per unit: `{t: test file, f: files hit, c: HEAD, d: ms,
   s: pass|fail|error, a: adapter}`. Seed replaces rows; run unions them, as today.

Fail-fast (`--fail-fast`) stops scheduling new units after the first failure.

## 5. Coverage formats

Four parsers, each turning a coverage file into `map[path][]line` with hit count > 0:

| format | produced by |
|---|---|
| `lcov` | pytest-cov, jest, vitest, cargo-llvm-cov, simplecov-lcov |
| `cobertura` | coverlet (dotnet), phpunit `--coverage-cobertura` |
| `gocover` | `go test -coverprofile` |
| `jacoco` | JaCoCo XML (maven, gradle) |

Paths are resolved to repo-relative by one resolver shared by all formats, in order:
absolute path under the root → relative to the root; a path that exists relative to the root
→ itself; otherwise the unique file under the root whose path ends with the reported path
(covers Go import paths and JaCoCo package-relative paths). Unresolvable and out-of-repo
paths are dropped. Only files matching `source_globs` or `test_globs` are kept.

## 6. Adapter contract v3

```yaml
name: go
detect: ["go.mod"]
unit_cmd: "go test -count=1 -coverpkg=./... -coverprofile={tmp}/cover.out -run {names} ./{dir}"
unit_names: '^func (Test\w+)\('     # optional; fills {names} as ^(A|B)$
coverage_file: "{tmp}/cover.out"
coverage_format: gocover
env: {}                              # values may use {tmp}
exit_codes: {}
jobs: 0                              # 0 = CPU count; 1 for tools that share a build dir
test_globs: ["**/*_test.go"]
source_globs: ["**/*.go"]
opaque: []
full_escalate: ["go.mod", "go.sum"]
requires: []                         # binaries doctor/init check for
unit_files: {}                       # {relative-name: content}, written into the unit's {tmp}
                                     # before unit_cmd runs; {tmp} is substituted in content
```

Placeholders in `unit_cmd`: `{unit}` (the test file, repo-relative), `{dir}`, `{name}`
(base name without extension), `{names}` (from `unit_names`), `{tmp}`. Commands are split on
whitespace before substitution, never run through a shell — as today.

`unit_names` is the one extra field any language may use when its runner cannot take a file
path (Go). It is declared data, not a code path: the pipeline is identical.

`unit_files` is the other extra field. It lets an adapter ship a small file the tool needs
(Maven's JaCoCo `@argfile`, Gradle's init script, RSpec's SimpleCov loader) without touching
the host repository. Keys are relative, clean names: no leading `/`, no `..`, no backslash,
so a file can only land inside `{tmp}`. `{tmp}` is substituted in the content as it is in
`unit_cmd`.

Removed fields: `seed`, `subset`, `subset_plain`, `list`, `coverage`, `report`,
`report_path`, `report_cmd`, `id_template`, `failfast_flag`, `test_flag`, `test_join`,
`test_selector`, `selection`, `test_for`, `importscan`. An adapter file using any of them is
rejected at load with a message naming the field and pointing at this spec.

## 7. Selection

`rtdd which` stays a pure map lookup. Tiers after this change:

- **Direct** — changed test files. Unchanged.
- **T0** — units whose recorded files intersect the changed set. Unchanged.
- **T1** — merge commit, opaque file, or stale row (distance > `StaleCommits`). The
  import-time fallback (`ImportOnly`) is removed.
- **T2** — `full_escalate` change, drift guard, empty or pre-v2 map. `AllTests` is the
  enumerated units.
- **Empty** — nothing maps. Unchanged, still never green.

Removed: tier `TS`, `selection_fidelity` (every answer is recorded coverage), and
`--record` (every executed unit records; there is nothing else to run).

A changed source file no row covers is reported in `unmapped_files`, as today; its new test
file, if any, is already Direct.

## 8. Uncovered report

Two classes: **covered** (some unit executed the changed line this cycle) and
**uncovered**. The import-time class is removed: in an isolated run, a line executed while
importing is executed by that unit.

## 9. Deleted

- `internal/coverage` sqlite/numbits/context reading, and the `modernc.org/sqlite`
  dependency. The contract tests that pinned it are rewritten to pin `gopkg.in/yaml.v3` only.
- `internal/report` (JUnit and pytest-reportlog parsing, id templates, report paths).
- `internal/importscan` and the embedded `scan.py`.
- `internal/selector/static.go`, tier TS, `adapter` fidelity/testfor/testselector.
- `cmd/rtdd` static/staticadvice/staticwarn/record/fidelity code and the sysmon/ctrace
  handling (`COVERAGE_CORE` is no longer forced: pytest-cov's aggregate data needs no
  dynamic contexts).
- PROTOCOL.md's fidelity section and static caveats; regenerated front-ends.
- Tests and contract tests pinning any of the above, and the plan documents' "python adapter
  frozen" guards.

`bench/` keeps its own `rtdd` and `static` arms as historical records; new runs measure the
new binary.

## 10. Kept

gitctx, mapstore (with meta `v: 2`), tiers other than TS, escalation digest, doctor's
fan-out report, host adapters in `.rtdd/adapters/`, install/init/uninstall/update, the
protocol generator, `--json` (schema 2: `selection_fidelity` removed, `import_time_lines`
removed from the uncovered summary).

## 11. Shipped adapters

All ten are rewritten to contract v3 in this change. Each gets an integration test that
seeds a tiny fixture project and asserts one changed source file selects exactly the test
file that executes it — including one that is not name-correspondent (the case the static
tier missed). The test skips when the toolchain is absent, and CI installs what it can.

| adapter | unit_cmd shape | format | jobs |
|---|---|---|---|
| python | `pytest -q -p no:cacheprovider --cov=. --cov-report=lcov:{tmp}/lcov.info {unit}` with `COVERAGE_FILE={tmp}/.coverage` | lcov | CPU |
| go | as in §6 | gocover | CPU |
| jest | `npx jest --ci --coverage --coverageReporters=lcovonly --coverageDirectory={tmp} {unit}` | lcov | CPU |
| vitest | `npx vitest run --coverage.enabled --coverage.reporter=lcov --coverage.reportsDirectory={tmp} {unit}` | lcov | CPU |
| cargo (adapter `cargo`) | `cargo llvm-cov --lcov --output-path {tmp}/lcov.info --test {name}` | lcov | 1 |
| maven | `mvn -q -Dtest={name} ...` with the JaCoCo agent from plugin goals, then the JaCoCo CLI (`exec:exec`, an `@argfile` shipped via `unit_files`) writes `{tmp}/jacoco.xml` | jacoco | 1 |
| gradle (not verified on a real toolchain in this change) | `./gradlew -q --init-script {tmp}/rtdd.init.gradle test --tests {name}`, the init script shipped via `unit_files` | jacoco | 1 |
| dotnet | `dotnet test -p:CollectCoverage=true -p:CoverletOutputFormat=cobertura -p:CoverletOutput={tmp}/ --filter FullyQualifiedName~{name}` (coverlet.msbuild, referenced by the test project) | cobertura | 1 |
| phpunit (not verified on a real toolchain in this change) | `vendor/bin/phpunit --coverage-cobertura {tmp}/cobertura.xml {unit}` | cobertura | CPU |
| rspec (not verified on a real toolchain in this change) | `bundle exec rspec --require {tmp}/rtdd_simplecov.rb {unit}`, the SimpleCov loader shipped via `unit_files` | lcov | CPU |

Exact commands are settled per adapter in the plan, against a real toolchain.

## 12. Risks

- **Per-unit process cost.** JVM and .NET start slowly and share a build directory
  (`jobs: 1`). The edit loop runs few units, but a T2 cycle or seed on a large suite is slow.
  Measured on rtdd-bench and published either way.
- **Coverage tools must be installed** (pytest-cov, cargo-llvm-cov, JaCoCo via Maven,
  coverlet.msbuild, pcov/xdebug for PHP, simplecov for Ruby). `requires` names them;
  doctor and init report what is missing.
- **Test files that depend on each other** (shared state set up by another file) fail in
  isolation. That is reported as the unit's failure, never hidden.
