package runner

import (
	"sort"
	"strings"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
)

// Outcome is one unit's result.
type Outcome struct {
	Test       string
	Status     string // pass | fail | error | skip
	DurationMS int
}

// RunResult is one complete execution: what ran, what it covered, and what failed.
type RunResult struct {
	Outcomes []Outcome
	Coverage *coverage.Result
	Failed   []string
	ExitCode int
	// Output is the tail of each failed or errored unit's combined output.
	Output map[string]string
}

// Run executes the selected units, each in its own process (spec §4).
func Run(a *adapter.Adapter, repoRoot string, tests []string, failFast bool) (*RunResult, error) {
	return RunUnits(a, repoRoot, tests, failFast)
}

// Seed runs every unit the adapter claims. It is the only operation that may shrink a
// map row, so every condition Run treats as fatal is fatal here too.
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error) {
	units, err := Units(a, repoRoot)
	if err != nil {
		return nil, err
	}
	return RunUnits(a, repoRoot, units, false)
}

// mergeEnv returns base with overrides applied, replacing rather than appending so an
// inherited value of an overridden key cannot survive.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, hit := overrides[k]; hit {
				continue
			}
		}
		out = append(out, kv)
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+overrides[k])
	}
	return out
}

// tail keeps a failing unit's diagnostics readable: the end of the stream is where the
// traceback and the summary line are.
func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}
