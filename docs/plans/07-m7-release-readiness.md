# RTDD M7 — Release Readiness: the install path a stranger walks

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the install path a stranger actually walks real, verified and green, so the only things left between this repository and a working `curl | sh` are the irreversible decisions a human has to make. Three halves, in the order a first release meets them. **CI**: `.github/workflows/ci.yml` is turned back on, runs on a commit on `main`, and its verdict — run id, head sha, per-job conclusion, and the fact that the bench steps passed on a runner with a cold cache — is committed as a record a test reads. **Artifacts**: a GoReleaser snapshot build produces the five archives the release matrix promises, a test opens each one and asserts the six shipped paths are inside, and `install.sh` is then driven end to end against those same archives over a local fixture server, so the "fresh machine" path is proven without publishing anything. **Documentation**: `docs/RELEASING.md` writes down the human sequence in order and names the draft-release trap that a first release walks into, and the README's `## Install` block stops promising a `curl | sh` that 404s by documenting the build-from-source route beside it.

**Architecture:** Nothing here is a new subsystem. Every task is a gate over something that already exists, and every gate is a Go test in an existing package, because a check that lives only in a shell script is a check that stops running the moment someone forgets the script. Three of the acceptance criteria are facts about a hosted CI run rather than facts about the tree, and a Go test cannot reach GitHub — `internal/contract` has no network and must not grow one. Those three are therefore split in two: the **action** (run `gh api`, read the run) is a checkbox step performed by the implementer, and the **evidence** (what it returned) is committed as one machine-readable record, `docs/results/m7-ci-green.md`, whose fenced `json` block three separate tests parse and assert over. A record that says `success` where the run said `failure` is a lie an implementer has to type deliberately; an un-asserted claim in prose is one they can make by accident. The artifact half works the same way, one level down: `goreleaser release --snapshot --clean` writes `build/dist/artifacts.json` and `build/dist/metadata.json`, and the tests read those rather than re-deriving the matrix, so an archive the release would actually ship is the archive under test.

**Tech Stack:** Go 1.24+ and stdlib `testing` for every gate (`archive/tar`, `archive/zip`, `encoding/json`, `net/http/httptest` — all stdlib, no new dependency). GoReleaser v2 as a build-time tool, invoked through `go run github.com/goreleaser/goreleaser/v2@<pinned>` so it never enters `go.mod`. `bash` for `scripts/ci-local.sh` and `scripts/release-snapshot.sh`; `sh` for `install.sh`, which stays POSIX. `gh` for the two read-only API queries this plan makes.

**Spec:** `docs/specs/2026-08-26-rtdd-design.md` **§12** (milestone M5 — "Front-ends, release": `PROTOCOL.md` → `SKILL.md`/`AGENTS.md`/`.mdc`, drift check in CI, GoReleaser, README) and **§15** (the open questions that gate publication — the release is not performed until they are answered by a human), plus the README's `## Install` block, which is the contract this milestone makes true. Each task below names its PRD acceptance criterion on its `**Discharges:**` line, and the map after the facts lists all nine.

## Global Constraints

*(the first five are copied verbatim from `00-interfaces.md`)*

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except `modernc.org/sqlite` and
  `gopkg.in/yaml.v3`. This milestone adds none; `TestGoModRequiresExactlyYAMLAndSQLite`
  in `internal/contract` fails if it does. GoReleaser is a *tool*, run through
  `go run <module>@<version>`, and never a `require` line.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

M7-specific. The first block is PRD #368's own global constraint, carried here verbatim,
and it is a constraint on **every** task in this plan rather than a concern of the one task
that would breach it:

- **This PRD prepares the release. It does not perform it.** No task here may:
  - change the repository's **visibility** — that is **#10**, an open human decision.
    Making it public publishes the benchmark results, the raw per-instance records, the
    pre-registration, and the prior-art claims about TDAD, pytest-testmon, Wallaby,
    Infinitest, Ekstazi and SonarQube;
  - push any **`v*`** tag, create a release, or publish a **draft** release;
  - write `signed_at`, `signed_by` or `stratified_recall_floor` into
    `bench/PREREGISTRATION.md` — that is **#8**, an open human decision — or create the
    **`prereg-m4`** tag.
- **`~/.cache/rtdd-bench` is never deleted, re-keyed or invalidated, and no hardware
  fingerprint is added to `cache.key()`** — that is **#218**, an open human decision. It
  holds the collected suites and full-suite timings behind every published number in
  `bench/results/`. Task 3 proves the CI runner needs none of it; that is the opposite of
  touching it.
- **A red check is fixed, never disabled.** `.github/workflows/ci.yml` was switched off
  once already and every commit of M6a–M6e merged with no CI at all. Turning it off again
  to get a green board is the failure mode this milestone exists to close, and Task 9's
  gate is what makes the removal of a check visible.
- **No benchmark is re-run and no published number moves.** `bench/results/` is read-only
  to this milestone. `scripts/ci-prereg.sh` keeps its existing steps; Tasks 3 and 9 add
  steps beside them and remove none.
- **`install.sh` stays POSIX `sh`.** It is piped into `sh` from `curl` on machines whose
  shell is unknown; the `RTDD_BASE_URL` / `RTDD_API_URL` overrides it already has are the
  only test seam, and Task 6 uses them rather than adding a seventh.
- **Nothing in `dist/` is hand-edited.** It regenerates from `protocol/PROTOCOL.md` via
  `rtdd-gen`; the archive tests assert the generated files are *shipped*, never what they
  contain.

## The three facts this plan turns on

Each was established on 2026-09-09 by running something, not by reading code. Every later
task depends on at least one of them, so they are stated here rather than rediscovered.

1. **`scripts/ci-local.sh` exits 0** on `1c1b0e6` and later. Main is healthy; the suite is
   not what is broken.
2. **`.github/workflows/ci.yml` is `state: disabled_manually`.** `gh api
   repos/VocanicZ/rtdd/actions/workflows` reports it, the file's presence in the tree does
   not. Its last run was 2026-08-26 and every run that day failed. **Every commit of
   M6a–M6e merged with no CI at all.** `.github/workflows/release.yml` is `active`; only
   `ci.yml` is off.
3. **`.goreleaser.yaml` sets `release: draft: true`.** There are zero tags and zero
   releases, so `api.github.com/repos/VocanicZ/rtdd/releases/latest` returns 404, and so
   does `raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh` while the repository is
   private. Even after the first `v*` tag is pushed, `releases/latest` keeps returning 404
   until the draft GoReleaser created is **published by hand** — that is the failure mode a
   first release walks into, and Task 7 is where it gets written down.

## The task this plan does not file — `findRepoRoot` (#369)

PRD #368's acceptance criteria **1 and 2** — `findRepoRoot` treating a directory as a
repository root only when its `.git` is a *real* repository, and the regression test that
drives it and `rtdd init` from a temp tree with an empty `.git` ancestor and compares
against `git rev-parse --show-toplevel` — are already filed as **issue #369**, a sibling
child of this PRD. **No task below re-files them.** They are named here so an implementer
reading this plan end to end does not conclude the criteria were dropped, and so that a
task ordering that assumed them is impossible: nothing in this plan depends on #369, and
#369 depends on nothing here. The two can land in either order.

## Acceptance-criterion map

| PRD #368 AC | Owned by | In one line |
|---|---|---|
| 1 | **#369**, not this plan | `findRepoRoot` requires a real `.git` |
| 2 | **#369**, not this plan | the empty-`.git`-ancestor regression test |
| 3 | Task 1 | `ci.yml` is `state: active`, verified with `gh api` |
| 4 | Task 2 | the `test` job concludes `success` on a `main` commit, run id and sha cited |
| 5 | Task 3 | the bench steps are green on a runner with no `RTDD_BENCH_CACHE` and no 201 MB cache |
| 6 | Task 4 | `release-preflight.sh` reports four passes and still refuses |
| 7 | Task 5 | the snapshot build's 5 archives each ship all six paths |
| 8 | Task 6 | `install.sh` proven end to end against those archives |
| 9 | Task 7 | `docs/RELEASING.md` states the human sequence and the draft trap |
| 10 | Task 8 | the README's `## Install` promises no path that 404s |
| 11 | Task 9 | `scripts/ci-local.sh` exits 0, with the new gates wired into it |

## File Structure

| File | Single responsibility |
|---|---|
| `docs/results/m7-ci-green.md` | the committed record of the hosted run: workflow state, run id, head sha, per-job conclusion, per-bench-step conclusion, and the cold-cache facts. One fenced `json` block, read by three tests |
| `internal/contract/ci_workflow_test.go` | parses that record and asserts AC3, AC4 and AC5 over it, plus the static facts about `ci.yml` each one needs |
| `docs/results/m7-release-preflight.md` | the committed transcript of one real `scripts/release-preflight.sh` run on this repository |
| `release_preflight_test.go` | grows the assertion that the recorded report is all-green and still refuses |
| `scripts/release-snapshot.sh` | runs GoReleaser in snapshot mode into `build/dist` with a pinned version; the only place this repo invokes GoReleaser |
| `release_snapshot_test.go` | reads `build/dist/artifacts.json`, opens all five archives, asserts the six shipped paths |
| `install_test.go` | grows the end-to-end install over those same archives |
| `docs/RELEASING.md` | the human release sequence, in order, with the draft-release trap named |
| `internal/contract/releasing_doc_test.go` | asserts that document's ordering and its two facts |
| `readme_install_test.go` | asserts the `## Install` block documents a route that works today |
| `scripts/ci-local.sh`, `.github/workflows/ci.yml` | grow the two new gates, named as their own steps |

---

## Task 1 — `.github/workflows/ci.yml` is switched back on, and the switch is verified through the API

**Discharges:** PRD #368 AC3. Spec §12 (M5: "drift check in CI").

**Files:** `docs/results/m7-ci-green.md` (new), `internal/contract/ci_workflow_test.go` (new)

**Interfaces:**

*Consumes:* `readRepoFile` and `fencedBlocks` from `internal/contract` (existing test helpers), `gh api repos/VocanicZ/rtdd/actions/workflows` (read-only).

*Produces:*

```go
// ciGreenRecord is the machine-readable half of docs/results/m7-ci-green.md: what the
// GitHub API said about the workflow and about the one run this milestone stands on.
// Three tests read it — AC3 reads .Workflow, AC4 reads .Run, AC5 reads .Run.Cache and
// .Run.BenchSteps — so the struct is declared once, here.
type ciGreenRecord struct { /* see step 1 */ }

// readCIGreenRecord parses the single fenced json block of docs/results/m7-ci-green.md.
func readCIGreenRecord(t *testing.T) ciGreenRecord
```

- [ ] **Step 1: Write the failing test**

`internal/contract/ci_workflow_test.go` (new file):

```go
// ci_workflow_test.go is the tree-side half of PRD #368's CI criteria. A Go test cannot
// reach the GitHub API and internal/contract must not grow a network dependency, so the
// hosted facts arrive as a committed record — docs/results/m7-ci-green.md — written by the
// implementer from `gh api` output and asserted here. A record that contradicts the run is
// a lie someone has to type; a claim made only in prose is one they can make by accident.
package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

const ciEvidence = "docs/results/m7-ci-green.md"

type ciGreenRecord struct {
	CheckedWith string `json:"checked_with"`
	Workflow    struct {
		Path  string `json:"path"`
		State string `json:"state"`
		ID    int64  `json:"id"`
	} `json:"workflow"`
	Run struct {
		ID         int64             `json:"id"`
		URL        string            `json:"url"`
		HeadSHA    string            `json:"head_sha"`
		HeadBranch string            `json:"head_branch"`
		Event      string            `json:"event"`
		Jobs       map[string]string `json:"jobs"`
		BenchSteps map[string]string `json:"bench_steps"`
		Cache      struct {
			BenchCacheEnv string `json:"RTDD_BENCH_CACHE"`
			RestoredBytes int64  `json:"restored_bytes"`
		} `json:"cache"`
	} `json:"run"`
}

func readCIGreenRecord(t *testing.T) ciGreenRecord {
	t.Helper()
	blocks := fencedBlocks(readRepoFile(t, ciEvidence), "json")
	if len(blocks) == 0 {
		t.Fatalf("%s has no ```json block: the evidence must be machine-readable, not prose", ciEvidence)
	}
	var rec ciGreenRecord
	if err := json.Unmarshal([]byte(blocks[0]), &rec); err != nil {
		t.Fatalf("parse the json block of %s: %v", ciEvidence, err)
	}
	return rec
}

// PRD #368 AC3. The workflow file has been present and switched off since 2026-08-26, so
// its presence proves nothing; only the API's `state` does.
func TestCIWorkflowIsRecordedActive(t *testing.T) {
	rec := readCIGreenRecord(t)
	if rec.Workflow.Path != ".github/workflows/ci.yml" {
		t.Errorf("%s records workflow path %q, want %q", ciEvidence, rec.Workflow.Path, ".github/workflows/ci.yml")
	}
	if rec.Workflow.State != "active" {
		t.Errorf("%s records ci.yml state %q, want %q — the workflow is still disabled", ciEvidence, rec.Workflow.State, "active")
	}
	if !strings.Contains(rec.CheckedWith, "gh api repos/VocanicZ/rtdd/actions/workflows") {
		t.Errorf("%s records checked_with %q; AC3 wants the state read from the API, not assumed from the file", ciEvidence, rec.CheckedWith)
	}
}

// An active workflow that no event starts is a workflow that is still off in practice.
func TestCIWorkflowStillTriggersOnPushToMainAndOnPullRequests(t *testing.T) {
	src := readRepoFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{"pull_request:", "branches: [main]"} {
		if !strings.Contains(src, want) {
			t.Errorf(".github/workflows/ci.yml no longer declares %q", want)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestCIWorkflowIsRecordedActive|TestCIWorkflowStillTriggersOnPushToMainAndOnPullRequests' -count=1
```

Expected: `TestCIWorkflowIsRecordedActive` fails in `readCIGreenRecord` with
`read docs/results/m7-ci-green.md: open .../docs/results/m7-ci-green.md: no such file or
directory`. `TestCIWorkflowStillTriggersOnPushToMainAndOnPullRequests` passes from the
first run — it is a regression guard on a fact that already holds, and it is in this task
because the fact only starts mattering once the workflow is on.

- [ ] **Step 3: Make it pass.** Enable the workflow, then read the state back:

```bash
gh workflow enable ci.yml -R VocanicZ/rtdd
gh api repos/VocanicZ/rtdd/actions/workflows --jq '.workflows[] | select(.path==".github/workflows/ci.yml")'
```

Write `docs/results/m7-ci-green.md` with the prose header (what was wrong, when it was
switched off, what the last runs did) followed by the single fenced `json` block. Fill
`workflow` from the query above. Leave `run` at its zero values for now — Task 2 fills it
and Task 3 fills `cache`/`bench_steps`; those tests are red until then, which is correct.

- [ ] **Step 4: Refactor.** None expected. If the record grows a second `json` block for
  any reason, `readCIGreenRecord` must be changed to name the block it wants rather than
  taking `blocks[0]` — silently reading the first of several is how the evidence and the
  assertion drift apart.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

---

## Task 2 — the `test` job concludes `success` on a commit on `main`, and the record cites it

**Discharges:** PRD #368 AC4. Spec §12 (M5).

**Files:** `docs/results/m7-ci-green.md`, `internal/contract/ci_workflow_test.go`

**Interfaces:**

*Consumes:* `readCIGreenRecord` from Task 1.

*Produces:* no new symbol — one test over the `run` half of the record.

- [ ] **Step 1: Write the failing test**

`internal/contract/ci_workflow_test.go` (append):

```go
package contract

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// fullSHA is a complete 40-character commit sha. An abbreviated sha is ambiguous across
// the lifetime of a repository, and this record is meant to be readable years later.
var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// PRD #368 AC4: the test job CONCLUDED success on a commit on main. "The workflow is on"
// (AC3) and "the workflow passes" are different claims, and only the second one means the
// tree is actually green under CI. A red run is fixed, not disabled again.
func TestCITestJobConcludedSuccessOnAMainCommit(t *testing.T) {
	rec := readCIGreenRecord(t)

	if got := rec.Run.Jobs["test"]; got != "success" {
		t.Errorf("%s records the `test` job as %q, want %q — a red run is not a pass", ciEvidence, got, "success")
	}
	if rec.Run.ID == 0 {
		t.Errorf("%s cites no run id; AC4 wants the run identified, not asserted", ciEvidence)
	}
	if !fullSHA.MatchString(rec.Run.HeadSHA) {
		t.Errorf("%s records head_sha %q, want a full 40-character sha", ciEvidence, rec.Run.HeadSHA)
	}
	if rec.Run.HeadBranch != "main" {
		t.Errorf("%s records head_branch %q, want %q — AC4 is about a commit on the default branch", ciEvidence, rec.Run.HeadBranch, "main")
	}
	if rec.Run.Event != "push" {
		t.Errorf("%s records event %q, want %q — a pull_request run does not prove main is green", ciEvidence, rec.Run.Event, "push")
	}
	if !strings.Contains(rec.Run.URL, strconv.FormatInt(rec.Run.ID, 10)) {
		t.Errorf("%s records url %q, which does not contain run id %d", ciEvidence, rec.Run.URL, rec.Run.ID)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestCITestJobConcludedSuccessOnAMainCommit' -count=1
```

Expected: once Task 1 has created the record with a zeroed `run`, four failures in one
test — `records the `test` job as "" , want "success"`, `cites no run id`,
`records head_sha "", want a full 40-character sha`, and `records head_branch "", want
"main"`. Before Task 1 lands it instead fails on the missing file, with the same message
Task 1's step 2 shows.

- [ ] **Step 3: Make it pass.** Let a real run happen — push the branch, merge it, or use
  `gh workflow run` if the workflow gains a `workflow_dispatch` trigger — then read the
  run and fill `run` in:

```bash
gh run list -R VocanicZ/rtdd --workflow ci.yml --branch main --limit 5 \
  --json databaseId,headSha,event,conclusion,url
gh run view <run-id> -R VocanicZ/rtdd --json jobs \
  --jq '.jobs[] | {name, conclusion}'
```

  **If the run is red, fix the cause and re-run.** Disabling the workflow again is
  forbidden by this plan's global constraints, and recording a `failure` conclusion as
  `success` is the one way to make this whole milestone worthless.

- [ ] **Step 4: Refactor.** None expected. Resist adding a `conclusion: "success"` field
  at the run level beside the per-job map: two places to state the same thing is two
  places for them to disagree.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. Note the closing comment on
  the issue must cite the run id and the head sha — the same two values the record now
  carries.

---

## Task 3 — the bench steps are green on a runner with no `RTDD_BENCH_CACHE` and no 201 MB cache

**Discharges:** PRD #368 AC5. Spec §12 (M5).

**Files:** `docs/results/m7-ci-green.md`, `internal/contract/ci_workflow_test.go`

**Interfaces:**

*Consumes:* `readCIGreenRecord` from Task 1, `readRepoFile` from `internal/contract`.

*Produces:* no new symbol — one test over `run.bench_steps` and `run.cache`, plus a static
assertion that the workflow primes no cache for the bench steps.

- [ ] **Step 1: Write the failing test**

`internal/contract/ci_workflow_test.go` (append):

```go
package contract

import (
	"strings"
	"testing"
)

// m7BenchSteps are the steps of ci.yml that exercise bench/ — the ones a developer's
// primed ~/.cache/rtdd-bench would carry locally and a fresh runner cannot.
var m7BenchSteps = []string{"prereg gate", "corpus admission gate", "bench replay suite"}

// PRD #368 AC5: those steps passed on a runner with NO RTDD_BENCH_CACHE and no restored
// 201 MB cache. That is what makes them a real gate rather than a replay of a local
// developer's state — and proving it does not touch ~/.cache/rtdd-bench, which #218 keeps
// off limits.
func TestCIBenchStepsRanWithoutAPrimedCache(t *testing.T) {
	rec := readCIGreenRecord(t)

	for _, step := range m7BenchSteps {
		got, ok := rec.Run.BenchSteps[step]
		if !ok {
			t.Errorf("%s records no conclusion for the %q step of ci.yml", ciEvidence, step)
			continue
		}
		if got != "success" {
			t.Errorf("%s records the %q step as %q, want %q", ciEvidence, step, got, "success")
		}
	}
	if rec.Run.Cache.BenchCacheEnv != "unset" {
		t.Errorf("%s records RTDD_BENCH_CACHE as %q on the runner, want %q", ciEvidence, rec.Run.Cache.BenchCacheEnv, "unset")
	}
	if rec.Run.Cache.RestoredBytes != 0 {
		t.Errorf("%s records %d bytes of bench cache restored on the runner, want 0", ciEvidence, rec.Run.Cache.RestoredBytes)
	}
}

// The record above says the runner had no primed cache. This says the workflow cannot
// quietly acquire one later: neither the variable nor a cache action pointed at the bench
// cache directory may appear in ci.yml. #218 keeps ~/.cache/rtdd-bench untouched, and a
// workflow that restores it would make AC5 unfalsifiable rather than satisfied.
func TestCIWorkflowPrimesNoBenchCache(t *testing.T) {
	src := readRepoFile(t, ".github/workflows/ci.yml")
	for _, banned := range []string{"RTDD_BENCH_CACHE", "rtdd-bench"} {
		if strings.Contains(src, banned) {
			t.Errorf(".github/workflows/ci.yml mentions %q; AC5 wants the bench steps green on a cold runner", banned)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestCIBenchStepsRanWithoutAPrimedCache|TestCIWorkflowPrimesNoBenchCache' -count=1
```

Expected: `TestCIBenchStepsRanWithoutAPrimedCache` fails five times — `records no
conclusion for the "prereg gate" step`, the same for `"corpus admission gate"` and
`"bench replay suite"`, then `records RTDD_BENCH_CACHE as "" on the runner, want "unset"`.
`TestCIWorkflowPrimesNoBenchCache` passes from the first run: today's `ci.yml` mentions
neither string, and the test exists to keep it that way.

- [ ] **Step 3: Make it pass.** From the same run Task 2 recorded, read the step
  conclusions of the `prereg` job and fill `bench_steps`, then set
  `cache.RTDD_BENCH_CACHE` to `"unset"` and `cache.restored_bytes` to `0`:

```bash
gh run view <run-id> -R VocanicZ/rtdd --json jobs \
  --jq '.jobs[] | select(.name=="prereg") | .steps[] | {name, conclusion}'
```

  If a bench step is red on a cold runner, that is a real finding about the harness's
  dependence on a primed cache — fix the harness or the workflow. Do **not** add a cache
  restore step to make it green: that breaks this task's second test and #218's constraint
  in the same commit.

- [ ] **Step 4: Refactor.** None expected. `m7BenchSteps` is a literal list rather than a
  parse of `ci.yml` on purpose: if a step is renamed, this test should fail and be updated
  deliberately, not follow the rename into vacuity.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

---

## Task 4 — `scripts/release-preflight.sh` reports four passes, and still refuses

**Discharges:** PRD #368 AC6. Spec §15 (the open questions the release waits on).

**Files:** `docs/results/m7-release-preflight.md` (new), `release_preflight_test.go`

**Interfaces:**

*Consumes:* `findRepoRootForTest` from `release_preflight_test.go` (existing), `scripts/release-preflight.sh` (existing, unchanged).

*Produces:* no new symbol — one test over a committed transcript of one real run.

The existing tests in this file drive the script against a fixture repo with stubbed
`go`/`uv`/`gh`, because the script runs `go test ./...` as one of its own steps and a real
`go` there would make the test invoke itself recursively. That stays true. AC6 is about the
script's verdict on **this** repository, which no stubbed run can produce, so the real run
is performed once by the implementer and its output committed — the same evidence pattern
Tasks 1–3 use, for the same reason.

- [ ] **Step 1: Write the failing test**

`release_preflight_test.go` (append):

```go
package installtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// preflightEvidence is the committed transcript of one real `scripts/release-preflight.sh`
// run on this repository. The stubbed tests above prove the script's branches; this proves
// its verdict here, which is what PRD #368 AC6 asks for.
const preflightEvidence = "docs/results/m7-release-preflight.md"

func readPreflightEvidence(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRootForTest(t), preflightEvidence))
	if err != nil {
		t.Fatalf("read %s: %v", preflightEvidence, err)
	}
	return string(raw)
}

// PRD #368 AC6, first half: the four automated verdicts are all green on this repository.
func TestReleasePreflightOnThisRepoIsRecordedAllGreen(t *testing.T) {
	out := readPreflightEvidence(t)
	for _, want := range []struct{ label, value string }{
		{"Front-end checks:", "pass"},
		{"Test suite:", "pass"},
		{"Release binaries:", "pass"},
		{"Placeholders:", "none"},
	} {
		re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(want.label) + `\s+(.*)$`)
		m := re.FindStringSubmatch(out)
		if m == nil {
			t.Errorf("%s has no %q line from the DECISION REQUIRED block", preflightEvidence, want.label)
			continue
		}
		if got := strings.TrimSpace(m[1]); got != want.value {
			t.Errorf("%s records %s %q, want %q", preflightEvidence, want.label, got, want.value)
		}
	}
	if !strings.Contains(out, "exit status: 0") {
		t.Errorf("%s does not record `exit status: 0` for the run", preflightEvidence)
	}
}

// PRD #368 AC6, second half: the script still REFUSES, and still names both irreversible
// actions. That is correct behaviour, not a defect — a preflight that offered to flip
// visibility or push a tag would be the bug.
func TestReleasePreflightStillRefusesAndNamesVisibilityAndTheTag(t *testing.T) {
	out := readPreflightEvidence(t)
	for _, want := range []string{
		"DECISION REQUIRED",
		"Make the repository public",
		"Push a v* tag",
		"I will not do either of these.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s does not contain %q; the refusal block is the point of the script", preflightEvidence, want)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test . -run 'TestReleasePreflightOnThisRepoIsRecordedAllGreen|TestReleasePreflightStillRefusesAndNamesVisibilityAndTheTag' -count=1
```

Expected: both tests fail in `readPreflightEvidence` with
`read docs/results/m7-release-preflight.md: open .../docs/results/m7-release-preflight.md:
no such file or directory`.

- [ ] **Step 3: Make it pass.** Run the real script and commit what it printed:

```bash
scripts/release-preflight.sh 2>&1 | tee /tmp/preflight.txt; echo "exit status: $?"
```

  Write `docs/results/m7-release-preflight.md` as a short prose header (when it was run,
  against which sha) plus the transcript, and append the `exit status: 0` line. Two fields
  will **not** be `pass` and must not be forced: `Pre-registration tag: prereg-m4 ->
  missing` (that tag is #8's, and creating it is forbidden here) and `Current visibility`,
  which reports whatever the repository is today. Neither sets the script's exit code. If
  one of the four green fields is not `pass`, fix the underlying check — the script is
  reporting a real defect, and editing the transcript would be fraud.

- [ ] **Step 4: Refactor.** None expected. Do not make the test tolerant of a `Kill
  criterion:` value: that line reports the M4 run, which this milestone does not perform.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

---

## Task 5 — the GoReleaser snapshot's five archives each ship all six release paths

**Discharges:** PRD #368 AC7. Spec §12 (M5: GoReleaser).

**Files:** `scripts/release-snapshot.sh` (new), `release_snapshot_test.go` (new), `.gitignore` (already ignores `/build/`)

**Interfaces:**

*Consumes:* `.goreleaser.yaml` (existing, unchanged), `findRepoRootForTest` from `release_preflight_test.go`.

*Produces:*

```bash
# scripts/release-snapshot.sh — the ONLY place this repo invokes GoReleaser. Builds a
# snapshot into build/dist with a pinned GoReleaser, creating nothing on GitHub: no tag,
# no release, no draft. Idempotent; --clean wipes build/dist first.
scripts/release-snapshot.sh
```

```go
// snapshotArchives returns the Archive-typed entries of build/dist/artifacts.json.
func snapshotArchives(t *testing.T) []goreleaserArtifact

// archiveMembers lists the paths inside a .tar.gz or .zip archive, slash-separated.
func archiveMembers(t *testing.T, archivePath string) []string
```

- [ ] **Step 1: Write the failing test**

`release_snapshot_test.go` (new file):

```go
// release_snapshot_test.go is PRD #368 AC7: the archives a release would actually publish,
// opened and inspected. release_artifacts_test.go already proves every BINARY in the
// matrix is self-contained; this proves every ARCHIVE carries the five files beside it,
// because a statically linked binary shipped without dist/SKILL.md installs a tool whose
// front-ends `rtdd init` cannot write.
package installtest

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshotDist is .goreleaser.yaml's `dist:` — build/dist, deliberately not /dist, which
// is the tracked generated front-end tree GoReleaser would otherwise wipe.
const snapshotDist = "build/dist"

// releaseTargets is the matrix .goreleaser.yaml declares: linux/darwin/windows x
// amd64/arm64, minus windows/arm64. Five archives, listed rather than re-derived, so
// dropping a platform from the release config fails here instead of passing quietly.
var releaseTargets = []string{
	"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64", "windows_amd64",
}

type goreleaserArtifact struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Goos   string `json:"goos"`
	Goarch string `json:"goarch"`
	Type   string `json:"type"`
}

func snapshotArchives(t *testing.T) []goreleaserArtifact {
	t.Helper()
	p := filepath.Join(findRepoRootForTest(t), snapshotDist, "artifacts.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v — run scripts/release-snapshot.sh first", p, err)
	}
	var all []goreleaserArtifact
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	var out []goreleaserArtifact
	for _, a := range all {
		if a.Type == "Archive" {
			out = append(out, a)
		}
	}
	return out
}

func archiveMembers(t *testing.T, archivePath string) []string {
	t.Helper()
	var names []string
	if strings.HasSuffix(archivePath, ".zip") {
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			t.Fatalf("open %s: %v", archivePath, err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			names = append(names, filepath.ToSlash(f.Name))
		}
		sort.Strings(names)
		return names
	}
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open %s: %v", archivePath, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gunzip %s: %v", archivePath, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read %s: %v", archivePath, err)
		}
		names = append(names, filepath.ToSlash(h.Name))
	}
	sort.Strings(names)
	return names
}

// PRD #368 AC7, first half: five archives, exactly the declared matrix.
func TestSnapshotBuildProducesTheFiveReleaseArchives(t *testing.T) {
	var got []string
	for _, a := range snapshotArchives(t) {
		got = append(got, a.Goos+"_"+a.Goarch)
	}
	sort.Strings(got)
	want := append([]string(nil), releaseTargets...)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("snapshot archives = %v, want %v", got, want)
	}
}

// PRD #368 AC7, second half: each archive carries all six shipped paths. The binary is
// named rtdd.exe on windows; the other five are identical everywhere.
func TestEverySnapshotArchiveShipsAllSixReleasePaths(t *testing.T) {
	root := findRepoRootForTest(t)
	for _, a := range snapshotArchives(t) {
		a := a
		t.Run(a.Goos+"_"+a.Goarch, func(t *testing.T) {
			binary := "rtdd"
			if a.Goos == "windows" {
				binary = "rtdd.exe"
			}
			want := []string{binary, "README.md", "LICENSE", "dist/SKILL.md", "dist/AGENTS.md", "dist/cursor/rules/rtdd.mdc"}
			members := archiveMembers(t, filepath.Join(root, a.Path))
			have := map[string]bool{}
			for _, m := range members {
				have[m] = true
			}
			for _, w := range want {
				if !have[w] {
					t.Errorf("%s does not ship %q; it holds %v", a.Name, w, members)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test . -run 'TestSnapshotBuildProducesTheFiveReleaseArchives|TestEverySnapshotArchiveShipsAllSixReleasePaths' -count=1
```

Expected: both tests fail in `snapshotArchives` with `read .../build/dist/artifacts.json:
no such file or directory — run scripts/release-snapshot.sh first`. `build/` is gitignored
and no snapshot has ever been built in this tree.

- [ ] **Step 3: Make it pass.** Write `scripts/release-snapshot.sh`:

```bash
#!/usr/bin/env bash
# Build the release archives locally, publishing nothing. GoReleaser is pinned and run
# through `go run` so it never enters go.mod, and --snapshot means no tag is required and
# no GitHub state is touched: no tag, no release, no draft (PRD #368's global constraint).
set -euo pipefail
cd "$(dirname "$0")/.."
# Pin the exact version — resolve it once with
#   go list -m -versions github.com/goreleaser/goreleaser/v2
# and write the answer in, rather than tracking @latest.
GORELEASER="${GORELEASER:-github.com/goreleaser/goreleaser/v2@v2.12.7}"
go run "$GORELEASER" release --snapshot --clean --skip=publish
```

  Run it, then re-run the tests. If an archive is missing a file, fix `.goreleaser.yaml`'s
  `archives[].files` — never the test's `want` list, which is the acceptance criterion.

- [ ] **Step 4: Refactor.** `releaseTargets` duplicates the matrix that
  `release_artifacts_test.go` derives from `.goreleaser.yaml`. Leave the duplication: that
  file asks "is every artifact the config declares self-contained", this one asks "does the
  config still declare the five platforms the release promises", and a shared derivation
  would make the second question unanswerable.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`. Task 9 wires the snapshot
  build in front of these tests so a clean checkout runs them for real.

---

## Task 6 — `install.sh`, proven end to end against those snapshot archives

**Discharges:** PRD #368 AC8. Spec §12 (M5: release), README `## Install`.

**Files:** `install_test.go`

**Interfaces:**

*Consumes:* `snapshotArchives` and `snapshotDist` from Task 5, `isolatedPATH` and `fixtureOSArch` from `install_test.go` (existing), `install.sh`'s existing `RTDD_BASE_URL` / `RTDD_API_URL` overrides.

*Produces:*

```go
// runInstallWithEnv runs install.sh with an explicit environment — no RTDD_VERSION
// default, so the latest-release resolution path is exercised the way a stranger's
// `curl | sh` exercises it.
func runInstallWithEnv(t *testing.T, env ...string) (code int, output, installDir string)
```

- [ ] **Step 1: Write the failing test**

`install_test.go` (append):

```go
package installtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// PRD #368 AC8: the fresh-machine path, over the archives a release would really publish,
// without publishing anything. The fixture server stands in for GitHub through the two
// overrides install.sh already has; the archives are Task 5's snapshot output, not a
// hand-built tarball, so this is the first test that exercises the real thing end to end.
func snapshotVersion(t *testing.T) string {
	t.Helper()
	p := filepath.Join(findRepoRootForTest(t), snapshotDist, "metadata.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v — run scripts/release-snapshot.sh first", p, err)
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("parse %s: %v", p, err)
	}
	if meta.Version == "" {
		t.Fatalf("%s carries no version", p)
	}
	return meta.Version
}

func runInstallWithEnv(t *testing.T, env ...string) (int, string, string) {
	t.Helper()
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH")
	}
	installDir := filepath.Join(t.TempDir(), "bin")
	full := append([]string{
		"PATH=" + isolatedPATH(t),
		"HOME=" + t.TempDir(),
		"TMPDIR=" + t.TempDir(),
		"RTDD_INSTALL_DIR=" + installDir,
	}, env...)
	cmd := exec.Command(shPath, filepath.Join(findRepoRootForTest(t), "install.sh"))
	cmd.Env = full
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return ee.ExitCode(), string(out), installDir
		}
		t.Fatalf("running install.sh: %v", runErr)
	}
	return 0, string(out), installDir
}

func TestInstallFromTheGoreleaserSnapshotArchives(t *testing.T) {
	fixtureOSArch(t)
	version := snapshotVersion(t)
	dist := filepath.Join(findRepoRootForTest(t), snapshotDist)

	// Serve build/dist exactly where install.sh looks: <base>/<version>/<file>.
	mux := http.NewServeMux()
	mux.Handle("/"+version+"/", http.StripPrefix("/"+version+"/", http.FileServer(http.Dir(dist))))
	mux.HandleFunc("/api/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, version)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	code, output, installDir := runInstallWithEnv(t,
		"RTDD_BASE_URL="+srv.URL,
		"RTDD_API_URL="+srv.URL+"/api/latest",
	)
	if code != 0 {
		t.Fatalf("install.sh exited %d against the snapshot archives:\n%s", code, output)
	}

	installed := filepath.Join(installDir, "rtdd")
	got, err := exec.Command(installed, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version failed: %v\n%s", installed, err, got)
	}
	if !strings.Contains(string(got), version) {
		t.Errorf("%s --version = %q, want it to name the snapshot version %q", installed, got, version)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test . -run 'TestInstallFromTheGoreleaserSnapshotArchives' -count=1
```

Expected: a failure in `snapshotVersion` — `read .../build/dist/metadata.json: no such
file or directory — run scripts/release-snapshot.sh first`. After Task 5's script has run
once, the test goes green with no production change: `install.sh` is already correct, and
this is the first evidence of it against real release archives rather than a fixture
tarball built by the test.

- [ ] **Step 3: Make it pass.** Run `scripts/release-snapshot.sh`, then re-run. If it is
  still red, the bug is real and belongs to `install.sh` or `.goreleaser.yaml`'s
  `name_template` — the two must agree on `rtdd_<version>_<os>_<arch>.tar.gz`. Fix the
  mismatch; do not special-case the test.

- [ ] **Step 4: Refactor.** The four existing install tests keep their hand-built fixture:
  they cover the checksum-corruption and unresolvable-latest branches, which a real
  snapshot cannot produce on demand. Do not collapse them into this one.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

---

## Task 7 — `docs/RELEASING.md`: the human sequence, and the draft-release trap

**Discharges:** PRD #368 AC9. Spec §15 (the decisions a human owns), §12 (M5).

**Files:** `docs/RELEASING.md` (new), `internal/contract/releasing_doc_test.go` (new)

**Interfaces:**

*Consumes:* `readRepoFile` from `internal/contract`, `.goreleaser.yaml` (read, never edited).

*Produces:* a document with three ordered steps and two stated facts; no code.

- [ ] **Step 1: Write the failing test**

`internal/contract/releasing_doc_test.go` (new file):

```go
// releasing_doc_test.go asserts docs/RELEASING.md is a SEQUENCE, not a list of topics.
// The order is the whole content: publishing the draft before the tag exists is
// impossible, and flipping visibility after the release is published leaks nothing but
// also helps nobody. A document that names the three steps in the wrong order is worse
// than none, because it will be followed.
package contract

import (
	"strings"
	"testing"
)

const releasingDoc = "docs/RELEASING.md"

// PRD #368 AC9: the exact human sequence, in order.
func TestReleasingDocStatesTheHumanSequenceInOrder(t *testing.T) {
	src := readRepoFile(t, releasingDoc)

	steps := []struct{ what, token string }{
		{"flip the repository visibility (#10)", "visibility"},
		{"push the v* tag", "v*"},
		{"publish the GoReleaser draft", "publish"},
	}
	prev := -1
	for _, s := range steps {
		i := strings.Index(src, s.token)
		if i < 0 {
			t.Errorf("%s never mentions %s (no %q)", releasingDoc, s.what, s.token)
			continue
		}
		if i < prev {
			t.Errorf("%s puts %s before the step that must precede it", releasingDoc, s.what)
		}
		prev = i
	}
	if !strings.Contains(src, "#10") {
		t.Errorf("%s does not attribute the visibility flip to issue #10", releasingDoc)
	}
}

// PRD #368 AC9, second half: the failure mode a first release walks into is named, with
// the config line that causes it and the symptom it produces.
func TestReleasingDocNamesTheDraftReleaseTrap(t *testing.T) {
	src := readRepoFile(t, releasingDoc)
	for _, want := range []struct{ what, token string }{
		{"the config that drafts the release", "draft: true"},
		{"the file that sets it", ".goreleaser.yaml"},
		{"the endpoint that stays 404", "releases/latest"},
		{"the status code a stranger sees", "404"},
		{"the script that cannot work until the draft is published", "install.sh"},
	} {
		if !strings.Contains(src, want.token) {
			t.Errorf("%s does not name %s (no %q)", releasingDoc, want.what, want.token)
		}
	}
	// The claim has to stay true of the config, not merely be written down once.
	if !strings.Contains(readRepoFile(t, ".goreleaser.yaml"), "draft: true") {
		t.Errorf(".goreleaser.yaml no longer sets `draft: true`, so %s is now wrong", releasingDoc)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestReleasingDocStatesTheHumanSequenceInOrder|TestReleasingDocNamesTheDraftReleaseTrap' -count=1
```

Expected: both fail in `readRepoFile` with `read docs/RELEASING.md: open
.../docs/RELEASING.md: no such file or directory`.

- [ ] **Step 3: Make it pass.** Write `docs/RELEASING.md`. It has one job: tell a human
  what to do, in order, and warn them about the one step whose omission looks like a bug in
  `install.sh`. Required content, in this order:
  1. **Flip the repository's visibility to public (#10).** What that publishes — the
     benchmark results, the raw per-instance records, the pre-registration, and the
     prior-art claims. `install.sh` cannot be fetched from `raw.githubusercontent.com`
     while the repository is private, so this genuinely comes first.
  2. **Push the `v*` tag.** It triggers `.github/workflows/release.yml`, which is already
     `active`, which runs GoReleaser for real.
  3. **Publish the GoReleaser draft.** `.goreleaser.yaml` sets `release: draft: true`, so
     the release GoReleaser creates is a **draft**: `api.github.com/repos/VocanicZ/rtdd/
     releases/latest` keeps returning **404** and `install.sh`'s latest-version resolution
     keeps failing with `could not resolve the latest release version` until a human
     publishes it. This is the failure mode a first release walks into.

  The document states plainly that **this plan performs none of the three** and that each
  is a human decision. It does not recommend removing `draft: true` — a draft is a
  deliberate safety catch, and the fix is to know about it.

- [ ] **Step 4: Refactor.** Cross-link it from `DEVELOPMENT.md` beside the existing
  `release-preflight.sh` entry, so the preflight's refusal block points at the document
  that says what to do next.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`.

---

## Task 8 — the README's `## Install` block promises no path that 404s

**Discharges:** PRD #368 AC10. README `## Install`, spec §12 (M5: README).

**Files:** `README.md`, `docs/outcomes/README.positive.md`, `docs/outcomes/README.negative.md`, `readme_install_test.go` (new)

**Interfaces:**

*Consumes:* `findRepoRootForTest` (existing), `outcomes_test.go`'s byte-identity rule (existing, unchanged).

*Produces:* no new symbol — one test over the three README files.

`docs/outcomes/SELECTED` is `positive`, and `outcomes_test.go` requires `README.md` to be
**byte-identical** to `docs/outcomes/README.positive.md`. So the edit lands in all three
files in one commit: identically in the root README and the positive outcome, and as the
same change in the negative outcome's own prose. Editing only the root README turns
`TestRootREADMEMatchesSelectedOutcome` red.

- [ ] **Step 1: Write the failing test**

`readme_install_test.go` (new file):

```go
// readme_install_test.go is PRD #368 AC10. Today the ## Install block offers exactly one
// route — `curl | sh` against raw.githubusercontent.com — and both that URL and the
// releases/latest endpoint it depends on return 404: the repository is private and there
// are zero tags and zero releases. A README whose only documented route 404s is a broken
// promise to the first stranger who tries it, so the build-from-source route is documented
// beside it until a release exists.
package installtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installBlock returns the ## Install section of the given README.
func installBlock(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(findRepoRootForTest(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	src := string(raw)
	i := strings.Index(src, "\n## Install\n")
	if i < 0 {
		t.Fatalf("%s has no `## Install` section", rel)
	}
	rest := src[i+len("\n## Install\n"):]
	if j := strings.Index(rest, "\n## "); j > 0 {
		rest = rest[:j]
	}
	return rest
}

func TestREADMEInstallDocumentsTheBuildFromSourceRoute(t *testing.T) {
	for _, rel := range []string{"README.md", "docs/outcomes/README.positive.md", "docs/outcomes/README.negative.md"} {
		block := installBlock(t, rel)
		for _, want := range []struct{ what, token string }{
			{"the build-from-source command", "go build ./cmd/rtdd"},
			{"the Go version it needs", "1.24"},
		} {
			if !strings.Contains(block, want.token) {
				t.Errorf("%s's ## Install block does not document %s (no %q)", rel, want.what, want.token)
			}
		}
	}
}

// The curl route stays documented — it is the route that works the moment the first
// release is published — but it may not stand alone and unqualified while it 404s.
func TestREADMEInstallSaysTheCurlRouteNeedsAPublishedRelease(t *testing.T) {
	block := installBlock(t, "README.md")
	if !strings.Contains(block, "install.sh") {
		t.Fatalf("README.md's ## Install block no longer documents install.sh at all")
	}
	if !strings.Contains(block, "release") {
		t.Errorf("README.md's ## Install block does not say the curl route needs a published release")
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test . -run 'TestREADMEInstallDocumentsTheBuildFromSourceRoute|TestREADMEInstallSaysTheCurlRouteNeedsAPublishedRelease' -count=1
```

Expected: six failures from the first test — `README.md's ## Install block does not
document the build-from-source command (no "go build ./cmd/rtdd")` and the matching
`(no "1.24")`, repeated for `docs/outcomes/README.positive.md` and
`docs/outcomes/README.negative.md` — plus one from the second, `does not say the curl
route needs a published release`.

- [ ] **Step 3: Make it pass.** Edit the `## Install` block: keep the `curl | sh` lines,
  add one sentence saying it works once a release is published, and add the
  build-from-source route (`go build ./cmd/rtdd` with Go 1.24+) beneath it. Apply the
  identical bytes to `docs/outcomes/README.positive.md` and the same change, in that
  file's own voice, to `docs/outcomes/README.negative.md`. **One commit**, so
  `TestRootREADMEMatchesSelectedOutcome` never sees a divergent tree.

- [ ] **Step 4: Refactor.** None. Do not remove the `curl | sh` route: Task 7's document
  makes it correct, and deleting it would make `install.sh` — which Task 6 just proved
  works — undocumented.

- [ ] **Step 5: Run the full suite.** `scripts/ci-local.sh`, which runs
  `TestRootREADMEMatchesSelectedOutcome` as part of `go test ./...`.

---

## Task 9 — `scripts/ci-local.sh` exits 0, with both new gates wired into it and into CI

**Discharges:** PRD #368 AC11. Spec §12 (M5: drift check in CI).

**Files:** `scripts/ci-local.sh`, `.github/workflows/ci.yml`, `internal/contract/ci_workflow_test.go`

**Interfaces:**

*Consumes:* `scripts/release-snapshot.sh` from Task 5, `readRepoFile` from `internal/contract`.

*Produces:* two named steps in both gates — the snapshot build, and the tests that read its
output — following this repo's existing `-list` idiom, because `go test -run` on a pattern
that matches nothing exits 0 and a deleted gate would otherwise pass silently.

- [ ] **Step 1: Write the failing test**

`internal/contract/ci_workflow_test.go` (append):

```go
package contract

import (
	"strings"
	"testing"
)

// Tasks 5 and 6 produce tests that are meaningless without build/dist, and build/dist is
// gitignored — so on a clean checkout they fail unless something builds the snapshot
// first. This asserts both gates do, and that the tests are named as their own steps
// rather than left to be swept up by `go test ./...`, which is how this repo points a red
// build at the thing that broke.
func TestBothGatesRunTheReleaseSnapshotAndItsTests(t *testing.T) {
	for _, gate := range []string{"scripts/ci-local.sh", ".github/workflows/ci.yml"} {
		src := readRepoFile(t, gate)
		for _, want := range []struct{ what, token string }{
			{"the snapshot build", "scripts/release-snapshot.sh"},
			{"the archive-contents gate", "TestEverySnapshotArchiveShipsAllSixReleasePaths"},
			{"the end-to-end install gate", "TestInstallFromTheGoreleaserSnapshotArchives"},
		} {
			if !strings.Contains(src, want.token) {
				t.Errorf("%s does not run %s (no %q)", gate, want.what, want.token)
			}
		}
		if !strings.Contains(src, "go test -list") && !strings.Contains(src, "go test -list '^TestEverySnapshotArchiveShipsAllSixReleasePaths$'") {
			t.Errorf("%s does not guard the new gate with a `go test -list` check; a renamed test would pass silently", gate)
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
go test ./internal/contract/ -run 'TestBothGatesRunTheReleaseSnapshotAndItsTests' -count=1
```

Expected: six failures — `scripts/ci-local.sh does not run the snapshot build (no
"scripts/release-snapshot.sh")` and the two matching gate names, then the same three for
`.github/workflows/ci.yml`, plus the two `-list` guard failures.

- [ ] **Step 3: Make it pass.** Add to `scripts/ci-local.sh`, after the existing static
  binary and artifact steps:

```bash
# PRD #368 AC7 and AC8: the archives a release would publish, and install.sh driven
# against them. build/dist is gitignored, so the snapshot has to be built here or the two
# tests below have nothing to read. Nothing is published: --snapshot creates no tag, no
# release and no draft.
echo "==> release snapshot archives (#368 AC7)"
scripts/release-snapshot.sh
listed="$(go test -list '^TestEverySnapshotArchiveShipsAllSixReleasePaths$' .)"
case "$listed" in
  *TestEverySnapshotArchiveShipsAllSixReleasePaths*) ;;
  *) echo "the release-archive contents gate is gone from the root package"; exit 1 ;;
esac
go test -count=1 -run '^TestSnapshotBuildProducesTheFiveReleaseArchives$|^TestEverySnapshotArchiveShipsAllSixReleasePaths$' .

echo "==> install.sh end to end against the snapshot archives (#368 AC8)"
listed="$(go test -list '^TestInstallFromTheGoreleaserSnapshotArchives$' .)"
case "$listed" in
  *TestInstallFromTheGoreleaserSnapshotArchives*) ;;
  *) echo "the end-to-end install gate is gone from the root package"; exit 1 ;;
esac
go test -count=1 -run '^TestInstallFromTheGoreleaserSnapshotArchives$' .
```

  Mirror both as named steps in the `test` job of `.github/workflows/ci.yml`. Order
  matters: the snapshot build comes before both, and after `go test ./...` so a broken
  suite fails faster than a five-platform cross-build.

- [ ] **Step 4: Refactor.** `go test ./...` will now run the snapshot tests too and they
  will fail before `scripts/release-snapshot.sh` has run in a fresh checkout. That is
  correct and deliberate — the failure message says `run scripts/release-snapshot.sh
  first` — but it means the snapshot step must sit **before** the `go test ./...` step in
  both gates, or the plain suite goes red first with a less useful message. Move it and
  re-run.

- [ ] **Step 5: Run the full suite.**

```bash
scripts/ci-local.sh; echo "exit status: $?"
```

  This is the acceptance criterion itself: `exit status: 0`. Any red here is fixed, never
  skipped and never disabled. The bar is no new failures against the baseline the branch
  started from, plus every test this plan added passing.

---

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #368, and no task above may start
it.

- **The three irreversible actions themselves.** The repository's visibility (#10), the
  `v*` tag, and the release or draft publish. Task 7 writes down the sequence; performing
  it is a human's call, and this plan's global constraints forbid every task from taking
  it. A green preflight is not permission.
- **The pre-registration signature and the `prereg-m4` tag — #8, an OPEN HUMAN DECISION.**
  `signed_at`, `signed_by` and `stratified_recall_floor` stay unwritten in
  `bench/PREREGISTRATION.md`, and the tag stays absent. Task 4's transcript will record
  `prereg-m4 -> missing`; that is the honest state, and forcing it green would be the one
  edit this milestone must never make.
- **A hardware fingerprint in `cache.key()` — #218, an OPEN HUMAN DECISION.**
  `~/.cache/rtdd-bench` is not deleted, re-keyed or invalidated by anything here. Task 3
  proves the hosted runner needs none of it, which is the opposite of touching it.
- **Acceptance criteria 1 and 2 — `findRepoRoot` and its empty-`.git` regression test.**
  Filed as **#369**, a sibling child of this PRD. Named in its own section above so nobody
  concludes it was dropped, and depended on by nothing here so the two can land in either
  order.
- **Re-running any benchmark.** Every number in `bench/results/` stays as committed. No
  new corpus repo, no re-measured wall-clock, no second static arm. `bench/` is read-only
  to this milestone apart from the CI steps that already execute its test suite.
- **The M4 SWE-bench run.** `bench/swebench/` is untouched. Task 4's transcript will
  report `Kill criterion: unknown (no bench/results/swebench/tables.md ...)` and that is
  the correct reading of a run that has not happened.
- **Making the hosted runner fast.** The snapshot build cross-compiles five platforms and
  the bench steps run the full replay suite; both are slow. Caching them is exactly the
  kind of change AC5 exists to forbid measuring against, and a faster gate that proves less
  is not an improvement. If CI wall-clock becomes the binding constraint, that is a new
  issue with its own evidence.
- **Removing `release: draft: true`.** A draft is a deliberate safety catch on an
  irreversible publish. Task 7 documents it; nothing here deletes it, and a future issue
  that wants to would need to argue the catch is unnecessary rather than inconvenient.
- **Any change to what RTDD claims to be.** That is #234, a `wayfinder` decision. This
  milestone makes the documented install path true; it does not restate the project's
  argument, retitle it, or revisit its positioning.
