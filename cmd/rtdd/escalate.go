package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// escalateDigest names the state of the `full_escalate` files that the working diff
// currently touches — the config the next full run would be covering.
//
// T2 asks whether a full run has happened since the config reached its current state.
// Answering it needs a name for that state, and this is the cheapest honest one: the
// changed full-escalate paths and the content each currently holds. It needs no repo
// walk, and it has the two properties the tier rule depends on.
//
//   - Editing conftest.py again changes the digest, so the next cycle escalates again.
//     The full run that already happened covered different bytes.
//   - Reverting the edit takes the path out of the diff, so the digest returns to the
//     empty-set value and nothing escalates. The tree matches what the map was built
//     from, which is the state that never needed a full run.
//
// The empty string is reserved for "nothing full-escalate is changed" and is never a
// digest, so it can never compare equal to a recorded one and accidentally suppress an
// escalation.
func escalateDigest(root string, ads []*adapter.Adapter, changes []gitctx.Change) string {
	var names []string
	seen := map[string]bool{}
	for _, c := range changes {
		if seen[c.Path] || !anyFullEscalate(ads, c.Path) {
			continue
		}
		seen[c.Path] = true
		names = append(names, c.Path)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte{0})
		// A file the diff names but the tree does not have is a deletion, and a
		// deletion is a config state like any other — it must be distinguishable
		// from the same path holding empty bytes, or deleting conftest.py and
		// truncating it would share a digest.
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			h.Write([]byte("\x01absent"))
		} else {
			sum := sha256.Sum256(b)
			h.Write(sum[:])
		}
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// anyFullEscalate reports whether any detected adapter treats rel as forcing a full run.
// The digest spans every adapter for the same reason meta.json carries one `cycles`:
// the full run it records is the repository's, not one adapter's.
func anyFullEscalate(ads []*adapter.Adapter, rel string) bool {
	for _, ad := range ads {
		if ad.IsFullEscalate(rel) {
			return true
		}
	}
	return false
}
