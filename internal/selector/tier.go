// Package selector turns (map, changed set, adapter, config) into a ranked selection.
// It is a pure function: no git, no subprocess, no I/O.
package selector

import (
	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

type Tier int

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

func (t Tier) String() string {
	switch t {
	case TierEmpty:
		return "empty"
	case TierDirect:
		return "direct"
	case TierT0:
		return "T0"
	case TierT1:
		return "T1"
	case TierTS:
		return "TS"
	case TierT2:
		return "T2"
	}
	return "unknown"
}

type Config struct {
	StaleCommits int     // default 50
	DriftGuard   int     // default 100
	HubThreshold float64 // default 0.40
}

func DefaultConfig() Config {
	return Config{StaleCommits: 50, DriftGuard: 100, HubThreshold: 0.40}
}

// orDefault treats a zero Config — every field zero, i.e. a caller that never set Cfg —
// as DefaultConfig(). Taken literally, a zero Config would mean "every row is stale",
// "drift-guard on the first cycle" and "every file is a hub", which escalates everything
// to T2 forever. A partially-filled Config is a deliberate choice and is left alone.
func (c Config) orDefault() Config {
	if c == (Config{}) {
		return DefaultConfig()
	}
	return c
}

type Selection struct {
	Tier   Tier
	Tests  []string // final ranked list, direct tests first
	Direct []string // changed/new test files, always run
	Reason string   // human-readable escalation cause
}

type Inputs struct {
	Map      *mapstore.Map
	Changes  []gitctx.Change
	Adapter  *adapter.Adapter
	Cfg      Config
	AllTests []string             // from adapter.List; needed for T2 and for direct-tier discovery
	Distance func(sha string) int // wraps gitctx.CommitDistance; -1 means unknown
	Cycles   int                  // from meta.json, for DriftGuard

	// EscalateDigest is the state of the adapter's `full_escalate` files right now;
	// EscalateDigestAtLastFull is what meta.json recorded when the last full run
	// finished. T2 asks whether a full run has happened SINCE the config reached its
	// current state — not whether the diff mentions a config file. Evaluating the diff
	// alone made the escalation sticky: in an uncommitted session the diff only grows,
	// so one conftest.py edit pinned every later cycle to the full suite (measured at
	// 24 of 25 flask cycles, 16 of 16 httpie cycles). An EMPTY
	// EscalateDigestAtLastFull means no full run is on record — a map seeded by an
	// older rtdd — and escalates exactly as before, so upgrading cannot silently stop
	// a repository escalating.
	EscalateDigest           string
	EscalateDigestAtLastFull string
	Merge                    bool                      // HEAD is a merge commit; escalates to T1
	ImportOnly               func(rel string) []string // static-import fallback; see M2

	// Exists reports whether the repository has this repo-relative path. It resolves
	// test_for templates (spec §4.2) without the selector touching a filesystem.
	// nil means level-1 correspondence is skipped, not that it failed.
	Exists func(rel string) bool

	// ImportDistance maps a changed file to the test files that transitively import it,
	// valued by the shortest number of import hops. It is the level-2 signal of spec
	// §4.1. nil — an adapter declaring no importscan — means level 2 is skipped, not
	// that it failed.
	ImportDistance func(changed string) map[string]int
}
