package protocol

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate internal/protocol/testdata/golden from protocol/PROTOCOL.md")

// TestGolden renders the real protocol/PROTOCOL.md and compares each target's
// output byte-for-byte against internal/protocol/testdata/golden. Regenerate
// with: go test ./internal/protocol -run TestGolden -update
func TestGolden(t *testing.T) {
	src, err := readProtocolMD(t)
	if err != nil {
		t.Fatalf("read PROTOCOL.md: %v", err)
	}
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := RenderAll(d)
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}

	goldenDir := filepath.Join("testdata", "golden")
	for _, tgt := range Targets {
		got, ok := out[tgt.OutPath]
		if !ok {
			t.Fatalf("RenderAll output missing %q", tgt.OutPath)
		}
		goldenPath := filepath.Join(goldenDir, filepath.Base(tgt.OutPath))

		if *updateGolden {
			if err := os.MkdirAll(goldenDir, 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", goldenDir, err)
			}
			if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
				t.Fatalf("WriteFile %s: %v", goldenPath, err)
			}
			continue
		}

		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden %s: %v (run `go test ./internal/protocol -run TestGolden -update` to generate it)", goldenPath, err)
		}
		if string(want) != got {
			t.Errorf("%s does not match golden %s\n--- got ---\n%s\n--- want ---\n%s", tgt.OutPath, goldenPath, got, string(want))
		}
	}
}
