// ci_workflow_test.go is the tree-side half of PRD #368's CI criteria. A Go test cannot
// reach the GitHub API and internal/contract must not grow a network dependency, so the
// hosted facts arrive as a committed record — docs/results/m7-ci-green.md — written by the
// implementer from `gh api` output and asserted here. A record that contradicts the run is
// a lie someone has to type; a claim made only in prose is one they can make by accident.
package contract

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// snapshotGateGates are the two tests that read `build/dist`, which is gitignored. On a
// clean checkout nothing builds it, so both are meaningless unless a gate runs
// scripts/release-snapshot.sh first — #380, the plan's Task 9, PRD #368 AC11. The names
// are the real ones in the tree, not the provisional ones the plan quoted; the test below
// checks each against a `func Test…` that actually exists, because a gate step naming a
// test that is gone passes silently: `go test -run` on a pattern matching nothing exits 0.
var snapshotGateGates = []struct{ what, name string }{
	{"the archive-contents gate", "TestGoreleaserSnapshotShipsEveryArchiveWithEveryShippedPath"},
	{"the end-to-end install gate", "TestInstallFromRealSnapshotArchivesPinnedToTheBuildsOwnVersion"},
}

// ciGates are the two gates that must stay in step. ci-local.sh is the authoritative one;
// a check that lives in only one of them is a check the other merges without.
var ciGates = []string{"scripts/ci-local.sh", ".github/workflows/ci.yml"}

// TestBothGatesRunTheReleaseSnapshotAndItsTests is PRD #368 AC11. Both gates must build
// the release snapshot and then name each of the two tests that read its output as its own
// step, guarded by this repo's `-list` idiom — not swept up by a bare `go test ./...`,
// because a red build must name the thing that broke.
func TestBothGatesRunTheReleaseSnapshotAndItsTests(t *testing.T) {
	// A name wired into a gate that no longer exists in the tree is the exact failure the
	// `-list` guard catches at run time; catching it here catches it at author time.
	for _, gate := range snapshotGateGates {
		if !repoDeclaresTestFunc(t, gate.name) {
			t.Errorf("no `func %s(` exists in the tree, so wiring it into a gate would pass silently", gate.name)
		}
	}

	for _, gate := range ciGates {
		src := readRepoFile(t, gate)

		if !strings.Contains(src, snapshotBuildScript) {
			t.Errorf("%s does not run the snapshot build (no %q); build/dist is gitignored, so both gates below read nothing", gate, snapshotBuildScript)
		}
		for _, g := range snapshotGateGates {
			if !strings.Contains(src, g.name) {
				t.Errorf("%s does not run %s (no %q)", gate, g.what, g.name)
			}
			listGuard := "go test -list '^" + g.name + "$'"
			if !strings.Contains(src, listGuard) {
				t.Errorf("%s does not guard %s with %s; `go test -run` on a pattern matching nothing exits 0, so a renamed or deleted test would pass silently", gate, g.what, listGuard)
			}
		}
	}
}

// snapshotBuildScript is the only place this repo invokes GoReleaser, and the only thing
// that fills the gitignored build/dist the two gates above read.
const snapshotBuildScript = "scripts/release-snapshot.sh"

// repoDeclaresTestFunc reports whether the tree declares `func <name>(` in some _test.go
// file. It scans rather than trusting the plan: the plan's Task 9 quoted provisional test
// names and the implementations chose different ones.
func repoDeclaresTestFunc(t *testing.T, name string) bool {
	t.Helper()
	root := repoRoot(t)
	want := "func " + name + "("
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "build" {
				return fs.SkipDir
			}
			return nil
		}
		if found || !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), want) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s looking for %s: %v", root, want, err)
	}
	return found
}
