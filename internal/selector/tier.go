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
	Map        *mapstore.Map
	Changes    []gitctx.Change
	Adapter    *adapter.Adapter
	Cfg        Config
	AllTests   []string                  // from adapter.List; needed for T2 and for direct-tier discovery
	Distance   func(sha string) int      // wraps gitctx.CommitDistance; -1 means unknown
	Cycles     int                       // from meta.json, for DriftGuard
	Merge      bool                      // HEAD is a merge commit; escalates to T1
	ImportOnly func(rel string) []string // static-import fallback; see M2
}
