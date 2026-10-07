package install

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// PRD #411 AC5: the config `rtdd init` writes is the graph's own defaults written out, so
// the graph code loads it back to exactly graph.DefaultConfig() — one list, two spellings
// that cannot drift.
func TestDefaultConfigLoadsAsTheGraphDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte(DefaultConfig()), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := graph.LoadConfig(root)
	if err != nil {
		t.Fatalf("graph.LoadConfig over DefaultConfig(): %v\n%s", err, DefaultConfig())
	}
	if want := graph.DefaultConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("DefaultConfig() loads as\n%+v\nwant graph.DefaultConfig()\n%+v", got, want)
	}
}

// PRD #411 AC5: every key is written out, so a user edits a value rather than learning a
// key name; no v0.2 key survives.
func TestDefaultConfigNamesEveryKeyAndNoV02Key(t *testing.T) {
	cfg := DefaultConfig()
	for _, key := range []string{"scan_exclude:", "test_files:", "test_exclude:", "graphify_path:", "max_stale_ratio:"} {
		if !strings.Contains(cfg, "\n"+key) {
			t.Errorf("DefaultConfig() has no %s line:\n%s", key, cfg)
		}
	}
	for _, gone := range []string{"adapters", "stale_commits", "drift_guard", "hub_threshold"} {
		if strings.Contains(cfg, gone) {
			t.Errorf("DefaultConfig() still says %q:\n%s", gone, cfg)
		}
	}
}
