# Development

## Toolchain

Go 1.24+ is required. On this machine the toolchain is installed at `/home/claude/goroot`
but is **not** on the default `PATH`. It is symlinked into `~/bin`:

```bash
ln -sf /home/claude/goroot/bin/go   ~/bin/go
ln -sf /home/claude/goroot/bin/gofmt ~/bin/gofmt
go version   # go1.24.4 linux/amd64
```

If `go: command not found`, re-run the symlinks above before doing anything else.
Do not hand-review Go code in place of compiling it.

## Verify the build

`scripts/ci-local.sh` is the authoritative CI gate. Run it before merging anything:

```bash
scripts/ci-local.sh
```

It runs exactly what `.github/workflows/ci.yml` runs:

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1
CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd   # must produce a STATIC binary
file /tmp/rtdd | grep -q 'statically linked' || echo "FAIL: not static"
go test -count=1 -run '^TestReleaseArtifactsAreStaticallyLinked$' .   # all four artifacts
```

The `file` check above only ever sees the linux/amd64 host build. PRD #6 criterion 5 is
about every published artifact, so `release_artifacts_test.go` cross-builds the whole
`.goreleaser.yaml` matrix and inspects each binary in its own format: ELF must be
statically linked (no PT_INTERP, no dynamic section, no imported libraries), Mach-O must
load nothing beyond the base-system dylibs — Go on darwin always links `libSystem`, so
`statically linked` is a string that can never appear there — and PE must import only
OS-provided DLLs. `scripts/release-preflight.sh` reports the same check on its own
`Release binaries:` line.

`CGO_ENABLED=0` is not optional. Spec D4 promises a binary with no runtime dependencies
forced into the host repo; `modernc.org/sqlite` is chosen over `mattn/go-sqlite3`
specifically because it is pure Go. A cgo dependency breaks that promise.

## Dependencies

Only two third-party modules are permitted in the engine:

- `modernc.org/sqlite` — reads `.coverage` directly (spec §4)
- `gopkg.in/yaml.v3` — adapter definitions

Adding a third requires a spec amendment. No test framework beyond stdlib `testing`.

## JUnit XML fixtures

`internal/report/testdata/junit/` holds nine JUnit XML reports captured from nine real
runner configurations — Vitest, Jest, Jest with `classNameTemplate` set to `{filepath}`,
`go-junit-report`, Maven Surefire, RSpec, PHPUnit, cargo-nextest and `dotnet test` with
JunitXml.TestLogger. Six back the parser; the other three back the shipped adapters'
`id_template` round trip in `internal/report/shipped_id_test.go`. They are never
hand-written and never hand-edited: when one contradicts the parser, the parser is
what changes. `scripts/capture-junit-fixtures.sh` regenerates them by running each suite in
a pinned container, so it needs Docker and the network and is deliberately NOT part of the
CI gate:

```bash
scripts/capture-junit-fixtures.sh            # all nine
scripts/capture-junit-fixtures.sh rspec      # just one
```

`internal/report/testdata/junit/README.md` records each fixture's runner version and the
exact command that produced it, plus the checklist of what the six disagree about.

## Benchmark harnesses

`bench/` is Python, managed with `uv`, and is deliberately outside the Go module.
`scripts/ci-local.sh` runs `scripts/ci-prereg.sh` as part of the gate, so `uv` must be on
PATH alongside `go`:

```bash
cd bench/swebench && uv sync && uv run pytest -q
```

### The cold-cache gate

`.github/workflows/ci.yml` runs the bench gates on a hosted runner: no `RTDD_BENCH_CACHE`,
no `~/.cache/rtdd-bench`. Every local run has had that ~201 MB store of collected suites,
full-suite outcomes and repo mirrors behind it, so "the bench gates pass" locally is a
weaker claim than it looks. `scripts/ci-cold-cache.sh` reproduces the bare-runner state
and runs the same commands under it:

```bash
scripts/ci-cold-cache.sh
```

It is NOT part of `scripts/ci-local.sh` — it runs the bench suites twice over and takes
about ten minutes — so run it when anything under `bench/` learns to read a cache, a path
under `$HOME`, or the network.

Getting that state right is the whole difficulty, because getting it wrong is silent.
`RTDD_BENCH_CACHE` moves the replay store (`replay/cli.py`'s `CACHE`) and moves nothing
else; `bench/swebench/run_arm.py` finds its own store through `Path.home()`. Point the
variable at an empty directory and leave `HOME` alone and you get a green run that read
the warm store the entire time. So the script moves `HOME` as well, and
`python -m replay.coldcache` counts **both** stores before a single gate runs and refuses
when either holds an entry. `uv`'s own package cache and managed interpreters stay pointed
at the real home — the runner caches those too, and a cold `HOME` should not turn the gate
into a download test.

Nothing here deletes, re-keys or invalidates `~/.cache/rtdd-bench`: the script fingerprints
it before and after and fails if the file count or byte total moved. Whether that store
should be keyed by hardware is #218, an open human decision, and `cache.key()` carries no
hardware fingerprint until it is made.

`bench/PREREGISTRATION.md` gates every M4 arm. It ships `status: UNSIGNED` with
`stratified_recall_floor:` deliberately empty — that number is a human decision, and
`bench/swebench/preflight.py` refuses to launch (exit 3) until a human writes it, signs the
file, and the signing commit is reachable from the `prereg-m4` tag. The bench gate asserts
that refusal while the file is unsigned, so it is green on an unsigned repo.

`bench/swebench/run_arm.py` is the per-arm driver. It calls `preflight` before it spends a
token, walks the frozen `instances.txt` in file order, writes one JSON record per instance
under `bench/results/swebench/raw/<arm>/`, and is resumable: an instance that already has a
record is skipped, so a crash costs the current instance only. `budget.py` holds the
ceilings — estimated dollars and wall-clock hours — and the run writes
`bench/results/swebench/cost/<arm>/cost.json` however it ends, with the assumed per-token
prices beside the actual token counts. A resume reads that file back and restores the
spend before it charges anything new, so `--max-usd` is a ceiling on the benchmark rather
than on each restart, and the rewritten cost file is the whole arm's spend rather than the
last invocation's.

Every instance's prompt is checked before it reaches the model: all five arms are
re-assembled from that instance's real workspace root and each context arm must still be
its control plus exactly one `<test-context>` block. The workspace path carries no arm name
for that reason — it is interpolated into the prompt's `Repository root:` line, so a
per-arm path would ship the arm's own identity inside every system prompt. A violation
stops the arm with exit 5 rather than being recorded as a failed instance.

```bash
cd bench/swebench
uv run python run_arm.py --dry-run     # all five arms, whole list, no model, no network
uv run python run_arm.py rtdd --max-usd 75
```

`--dry-run` is what CI runs. It assembles every arm's prompt for every instance through the
same code path a real run uses, against the same workspace root a real run would use, and
asserts the composition guarantee on those bytes — each
context arm is its control plus exactly one `<test-context>` block, and the block carries no
imperative — exiting non-zero if any arm violates it. It needs no signed pre-registration
because it spends nothing.

[docs/bench/swebench-harness.md](docs/bench/swebench-harness.md) is the reviewer's page for
that harness: how to reproduce it locally with no model, network or Docker, what should
refuse and with which message, the deliberately failing fixtures that prove the
non-procedural guarantee is load-bearing, and which plan tasks are out of scope because they
are human-in-the-loop. `bench/swebench/tests/test_acceptance.py` is the executable form of
the same page, and CI runs it.

## The Axis 2 replay benchmark (`bench/replay/`)

`bench/` is its own uv project — separate from `bench/swebench/` — and replays real
commits of the frozen corpus in `bench/corpus.yaml`. Its results are committed under
`bench/results/`; `bench/work/` (clones, worktrees, per-repo virtualenvs) and
`bench/cache/` are not.

```bash
cd bench
uv sync
uv run python -m replay.cli doctor          # must print `publishable: yes`
uv run python -m replay.cli replay --repo flask --replay-commits 25 --wallclock-sample 10
uv run python -m replay.cli session --repo flask --cycles 25   # drift.json
uv run python -m replay.cli report                             # aggregate.md
uv run python -m replay.cli report --rebuild                   # summary.{json,md} too
uv run python -m replay.cli derive                             # the derived arms, offline
uv run python -m replay.cli chart                              # docs/results/figures/*.svg
```

`report --rebuild` re-derives each admitted repo's `summary.json` and `summary.md` from
that repo's own committed `commits.jsonl` and `config.json` before writing the aggregate.
Every per-cycle sample the benchmark ever measured is in `commits.jsonl`, so a change to
how they are aggregated — the wall-clock percentiles in #214 were the first — is a
re-render, not a re-measurement on hardware that may no longer exist. It re-derives and
never re-measures: `config.json` is left exactly as the run wrote it, an operator's
`--no-wallclock` refusal stays refused, and `skipped` and `rtdd_run_errors` (which leave
no record behind, by construction) are carried across from the published summary.

`derive` is the other half of that idea, for an *arm* rather than an aggregation. The
`static` arm models RTDD's `TS` tier and is a function of fields every replayed commit
already committed — its `changed` set, the `all_tests` it collected and the `importgraph`
selection recorded beside it — so it is **computed from `commits.jsonl`, never executed**
(`bench/replay/derive.py`). `derive` reads each admitted repo's records, drops any stale
copy of a derived arm, recomputes it and rewrites the file through the same byte-stable
serialiser the run used; `report --rebuild` then publishes it:

```bash
cd bench
uv run python -m replay.cli derive           # appends the static arm to commits.jsonl
uv run python -m replay.cli report --rebuild # re-renders summary.{json,md}
git diff --stat bench/results/               # the review
```

`chart` is the same idea applied to the README's pictures. The three Axis 2 figures are
generated from the committed `summary.json` files and committed as a light/dark SVG pair
under `docs/results/figures/`; `bench/tests/test_chart.py` re-renders them and fails if the
bytes differ, so a figure that disagrees with the numbers beside it is a red build rather
than something a reviewer has to notice. Every mark carries the `data-strategy`,
`data-metric` and `data-value` it was drawn from, and the rules `report.py` follows carry
over: a `null` recall is named under the plot rather than drawn at zero, an arm that
executed nothing gets no wall-clock bar, and the safety figure renders the pre-registered
verdict from `report.py` rather than restating it. Re-run it after anything that moves a
published number:

```bash
cd bench
uv run python -m replay.cli report --rebuild # the tables
uv run python -m replay.cli chart            # the figures that must agree with them
```

Both commands are offline: nothing is cloned, provisioned or executed, no test is run,
and nothing is written to `RTDD_BENCH_CACHE`. `derive` is idempotent — a second run
leaves `commits.jsonl` byte-identical — and it touches only repos the corpus currently
admits, so `results/sqlfluff/` (dropped at `corpus_version: 2`) stays exactly as v1
published it. The derived arm carries no `WallClockRecord`, because it executed nothing
and none is invented; `summary.md` renders those cells as `not measured`.

`doctor` needs the binary under test on `PATH`, which is the same static build the CI
gate produces:

```bash
CGO_ENABLED=0 go build -o ~/bin/rtdd ./cmd/rtdd
```

Three things about the run are worth knowing before reading a table:

- **Each corpus repo gets its own virtualenv**, built by `replay/envsetup.py` from that
  repo's `install:` recipe in `corpus.yaml`. Only `uv pip install ...` commands are
  executed; anything else is an `EnvError`. The harness's own instruments go in first so
  the repo's pins win — flask replays commits from before pytest 9 removed
  `_pytest.monkeypatch.notset`, so its recipe pins `pytest<9` and is believed.
- **The worktree's source goes in front on `PYTHONPATH`.** The repo is installed editable
  from the clone, and a `.pth` entry sorts after everything `PYTHONPATH` contributes; a
  replay whose worktree does not win that lookup scores every commit against the clone's
  code and publishes an empty `F_full`.
- **`--replay-commits` bounds the walk** and the number it used is published in
  `config.json` and in `summary.md`'s header, so a bounded table always says how many
  commits produced it. Omit it to replay the corpus's own count.

## `rtdd update` and the network boundary

`internal/selfupdate` is the only package in this module that can open a connection, and
`TestOnlySelfupdateReachesTheNetwork` (`network_test.go`) fails the build if any other
non-test file imports `net` or `net/http`. Before `rtdd update` existed nothing here made a
network call at all; that was a property of the tool worth keeping true on every other code
path, and a property is only kept by something that fails when it stops being true.

Two consequences worth knowing before you touch this package:

- **It is a second implementation of the installer's protocol.** The installer cannot be Go,
  because it runs before any rtdd binary is on the machine, so the duplication is
  unavoidable. What keeps it honest is `internal/selfupdate/drift_test.go`, which *executes*
  `installer/platform.js` and `installer/release.js` and compares what they return against
  what this package computes. It compares behaviour rather than source text, so a refactor
  that keeps an old literal in a comment while computing something else still fails it.
- **`net/http` costs the darwin artifacts two framework links.** `crypto/x509` verifies
  server certificates against the macOS trust store, which is `CoreFoundation` and
  `Security`. Both are in `baseSystemDylibs` (`release_artifacts_test.go`) for the same
  reason `libresolv` already was: Apple ships them inside macOS and no user installs them.
  `linkage_allowlist_test.go` stops that list widening any further - every entry must sit
  under `/usr/lib/` or `/System/Library/Frameworks/`, and `foreignDylibs` is tested against
  a synthetic Homebrew path so the check can still fail.

Nothing here reads the compiled-in `version`: `selfupdate.Update` takes the running version
as a parameter, so no test is coupled to what ldflags stamped into any particular build.

## `rtdd uninstall`

`install.PlanUninstall` is `install.Plan`'s inverse and covers exactly what `Plan` writes
outside `.rtdd/`. The property under test is a round trip:
`TestMergeThenRemoveReturnsTheFileUnchanged` merges rtdd's block into a host file and
removes it again, and requires the original bytes back. AGENTS.md and CLAUDE.md belong to
the host project, so an ambiguous file - two blocks, half a block - is a conflict that
stops the run, never a guess.

`.rtdd/` and the binary are left alone unless `--state` or `--binary` asks: the recorded map
is the expensive thing to rebuild, and a repository is not where the binary lives.

## Release pre-flight

`scripts/release-preflight.sh` is the last thing an agent runs, at the end of plan Task 24,
before handing off to a human. It runs `rtdd-gen check`, `rtdd-gen verify`, `go test ./...`,
`bench/swebench`'s pytest suite and `preflight.py`, and a placeholder grep, then prints the
`DECISION REQUIRED — repository visibility and release` block with every field filled in from
what it just ran, and stops:

```bash
scripts/release-preflight.sh
```

It **never mutates repository state** — no `gh repo edit`, no `git tag`, no `git push`, no
`gh release`. Going public and pushing a release tag are irreversible and stay a human call
(plan Task 24); this script only informs it.

`Test suite:` covers `go test ./...` and `bench/swebench`'s pytest suite. `preflight.py` is
reported separately, on the `M4 launch gate:` line, because it is not a test: it is the gate
that decides whether an arm may spend a token, and it refuses with exit 3 for as long as
`bench/PREREGISTRATION.md` is unsigned. `scripts/ci-prereg.sh` treats that same refusal as a
PASS. A `refused (pre-registration unsigned …)` launch gate, `Pre-registration tag: missing`
and an `unknown` kill criterion are the correct verdicts until a human writes the kill
criterion, signs, and tags — the script still exits non-zero, because the release is not
ready, but none of the four automated verdicts is failing.

What to do after it stops is written down in [`docs/RELEASING.md`](docs/RELEASING.md): the
three human steps in order — visibility (#10), the `v*` tag, then publishing the GoReleaser
draft — and the draft-release trap that makes `releases/latest` return 404 until the third
one is taken.
