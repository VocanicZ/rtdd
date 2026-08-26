package main

import (
	"path/filepath"

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

func rtddDir(repoRoot string) string  { return filepath.Join(repoRoot, ".rtdd") }
func metaPath(repoRoot string) string { return filepath.Join(rtddDir(repoRoot), "meta.json") }
func mapPath(repoRoot string) string  { return filepath.Join(rtddDir(repoRoot), "map.jsonl") }

// readMeta returns the zero value and a nil error when meta.json does not exist —
// an unseeded repo is a normal state, not an error. A MALFORMED meta.json is a
// fatal error, never a silent reset: a reset would zero `cycles` and postpone the
// DriftGuard full run forever.
func readMeta(repoRoot string) (meta, error) { return mapstore.LoadMeta(metaPath(repoRoot)) }

func writeMeta(repoRoot string, m meta) error { return mapstore.SaveMeta(metaPath(repoRoot), m) }
