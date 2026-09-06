package main

import (
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// meta is .rtdd/meta.json. It is kept OUT of map.jsonl because the JSONL is
// union-merged and these fields are not union-mergeable.
//
// This is an ALIAS, not a second declaration. M1a put the type and its codec in
// internal/mapstore so `status` could read `cycles`; Task 17 asks cmd/rtdd for the
// same shape under a shorter name. Two independent structs with the same JSON tags
// is exactly the drift this alias prevents — a field added to one would silently
// not exist in the other.
type meta = mapstore.Meta

func rtddDir(repoRoot string) string  { return filepath.Join(repoRoot, rtddDirName) }
func metaPath(repoRoot string) string { return filepath.Join(rtddDir(repoRoot), "meta.json") }
func mapPath(repoRoot string) string  { return filepath.Join(rtddDir(repoRoot), "map.jsonl") }

// readMeta returns the zero value and a nil error when meta.json does not exist —
// an unseeded repo is a normal state, not an error. A MALFORMED meta.json is a
// fatal error, never a silent reset: a reset would zero `cycles` and postpone the
// DriftGuard full run forever.
func readMeta(repoRoot string) (meta, error) { return mapstore.LoadMeta(metaPath(repoRoot)) }

func writeMeta(repoRoot string, m meta) error { return mapstore.SaveMeta(metaPath(repoRoot), m) }

// coverageAdapterName is meta.json's singular `adapter` for a detected set: the COVERAGE
// adapter that produced the map (decision 4 of docs/plans/06-m6d-shipped-adapters.md), and
// "" where every detected adapter is static.
//
// It is shared by `seed` and `run` because the field is load-bearing on the READ path:
// mapstore.ForAdapter serves every UNTAGGED row to the adapter this field names and
// withholds it from everyone else. A static adapter's name here hands a legacy map's
// pytest nodeids to `mvn -B test -Dtest=...`, which matches nothing and exits 0 — a false
// pass wearing a real id, and the exact outcome the tag exists to prevent (PRD #232 AC6).
// An all-static repository leaves it empty: no adapter here recorded a row, so there is
// nothing for the field to speak for.
func coverageAdapterName(detected []*adapter.Adapter) string {
	_, coverage := selectionSplit(detected)
	if len(coverage) == 0 {
		return ""
	}
	return coverage[0].Name
}

// metaAfterRun is the meta.json `rtdd run` writes back: the one it loaded, with the fields
// a run may fill in.
//
// Only fields that were EMPTY are filled. The singular `adapter` says whose the untagged
// rows of an existing map are, so a repository that gains a second toolchain must not have
// that answer rewritten underneath it — and a repository that never had one gets the
// coverage adapter, not whichever name detection happened to return first.
func metaAfterRun(mt meta, detected []*adapter.Adapter) meta {
	if mt.Adapter == "" {
		mt.Adapter = coverageAdapterName(detected)
	}
	if mt.V == 0 {
		mt.V = 1
	}
	return mt
}
