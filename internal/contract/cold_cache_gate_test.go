package contract

import (
	"strings"
	"testing"
)

// scripts/ci-cold-cache.sh reproduces the one state ci.yml's bench steps have never been
// proven in: no RTDD_BENCH_CACHE and no populated ~/.cache/rtdd-bench. Its value is
// entirely in running the SAME commands the workflow runs — a gate that drifts to a
// smaller set still exits 0 and still proves nothing — and in checking the state is
// actually cold BEFORE it runs them, because a silent fallback to the developer's warm
// store also exits 0.
const (
	coldCacheGate = "scripts/ci-cold-cache.sh"
	ciWorkflow    = ".github/workflows/ci.yml"
)

// benchGateCommands are the bench-touching commands ci.yml runs. A literal list rather
// than a parse of the workflow on purpose: if a step's command changes, this test should
// fail and be updated deliberately, not follow the change into vacuity.
var benchGateCommands = []string{
	"scripts/ci-prereg.sh",
	"uv run pytest -q",
	"uv run pytest tests/test_prompts.py -q",
	"uv run pytest tests/test_acceptance.py -q",
	"uv run python run_arm.py --dry-run",
	"uv run python -m replay.cli audit",
}

// Issue #371: every bench command the workflow runs on a bare runner is also run by the
// cold-cache gate. The workflow half of the assertion is what keeps the list honest — a
// command that left ci.yml would otherwise sit in this list forever, checked by nobody.
func TestColdCacheGateRunsEveryBenchCommandTheWorkflowRuns(t *testing.T) {
	workflow := readRepoFile(t, ciWorkflow)
	// The gate runs scripts/ci-prereg.sh, which runs several of these itself, so the
	// pair is searched as one: what matters is that the command executes in the cold
	// state, not which of the two files spells it out.
	gate := readRepoFile(t, coldCacheGate) + "\n" + readRepoFile(t, "scripts/ci-prereg.sh")

	for _, cmd := range benchGateCommands {
		if !strings.Contains(workflow, cmd) {
			t.Errorf("%s no longer runs %q; update benchGateCommands deliberately", ciWorkflow, cmd)
		}
		if !strings.Contains(gate, cmd) {
			t.Errorf("%s does not run %q, so the cold-cache proof does not cover it", coldCacheGate, cmd)
		}
	}
}

// Issue #371 AC1: the coldness is checked before the gates run under it, not after. A
// check that ran afterwards would report a cache the gates had already had the use of.
func TestColdCacheGateProvesColdnessBeforeItRunsAnyBenchCommand(t *testing.T) {
	gate := readRepoFile(t, coldCacheGate)

	check := strings.Index(gate, "python -m replay.coldcache")
	if check < 0 {
		t.Fatalf("%s runs no coldness check; AC1 wants the state proven, not assumed", coldCacheGate)
	}
	for _, cmd := range benchGateCommands {
		at := strings.Index(gate, cmd)
		if at < 0 {
			continue // reported by the test above
		}
		if at < check {
			t.Errorf("%s runs %q before it checks the cache is cold", coldCacheGate, cmd)
		}
	}
}

// Issue #371 AC6 and AC7, as refusals the script itself carries: a cold-cache run
// re-runs no benchmark and leaves the developer's ~/.cache/rtdd-bench alone. #218 is the
// open human decision about that store, and this gate is not allowed to pre-empt it.
func TestColdCacheGateRefusesToPublishOrToDisturbTheWarmStore(t *testing.T) {
	gate := readRepoFile(t, coldCacheGate)

	if !strings.Contains(gate, "status --porcelain -- bench/results") {
		t.Errorf("%s does not check that bench/results/ is untouched", coldCacheGate)
	}
	for _, destructive := range []string{"rm -rf \"$warm\"", "rm -rf $warm", "rm -rf ~/.cache/rtdd-bench"} {
		if strings.Contains(gate, destructive) {
			t.Errorf("%s contains %q; #218 keeps ~/.cache/rtdd-bench untouched", coldCacheGate, destructive)
		}
	}
}
