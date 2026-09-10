package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// adapters/python.yaml is byte-frozen for the multi-language PRD. Spec §4.1 makes a
// seeded Python repo's selection byte-identical to today a REQUIREMENT, and the only
// adapter that has ever produced a map is the cheapest thing to hold still. Contract v2
// is additive precisely so this file needs no edit: every new key is optional, and an
// omitted `selection` means `coverage`.
//
// To change it, change this digest in the same commit and say in the message which spec
// section licenses the edit.
//
// Changed once, for `subset_plain`. What §4.1 freezes is a seeded Python repo's
// SELECTION, and the key does not reach selection at all: it is a second RUN command,
// reachable only through `rtdd run --record=auto`, and `--record` defaults to `always`.
// A repository that upgrades and changes no flag runs the identical command it ran
// before, over the identical selection, so the byte-identical-selection requirement the
// freeze protects is untouched.
const pythonAdapterSHA256 = "7ee34da9520db745af2659b2af4c17d39a334fb02b09b693ddf8c0ed2719c37b"

func TestPythonAdapterIsByteFrozen(t *testing.T) {
	sum := sha256.Sum256([]byte(readRepoFile(t, "adapters/python.yaml")))
	got := hex.EncodeToString(sum[:])
	if got != pythonAdapterSHA256 {
		t.Errorf("adapters/python.yaml changed: sha256 = %s, want %s\n"+
			"It is byte-frozen for the multi-language PRD (spec §4.1). If the edit is "+
			"licensed, update pythonAdapterSHA256 in the same commit.", got, pythonAdapterSHA256)
	}
}

// The v1 adapter must not have acquired a v2 key by accident: it earns its v2 meaning by
// defaulting, not by declaring.
func TestPythonAdapterDeclaresNoV2Keys(t *testing.T) {
	src := readRepoFile(t, "adapters/python.yaml")
	for _, key := range []string{"selection:", "report_path:", "id_template:", "test_for:", "importscan:", "requires:", "test_selector:"} {
		if strings.Contains(src, "\n"+key) {
			t.Errorf("adapters/python.yaml declares %q; it is byte-frozen and defaults to v1 behaviour", key)
		}
	}
}
