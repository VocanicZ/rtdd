package graph

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rtdd", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadConfigWithoutAFileIsTheSpecDefaults(t *testing.T) {
	got, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		ScanExclude: []string{"vendor/**", "node_modules/**", "third_party/**", "**/*.min.js",
			"dist/**", "build/**", ".rtdd/**", "graphify-out/**"},
		DefinitionExclude: []string{"**/*.md", "**/*.mdc", "**/*.markdown", "**/*.rst", "**/*.txt", "**/*.adoc"},
		TestFiles: []string{"**/test_*", "**/*_test.*", "**/*.test.*", "**/*.spec.*", "**/*Test.*",
			"**/*Tests.*", "**/tests/**", "**/test/**", "**/spec/**", "**/__tests__/**"},
		TestExclude:    []string{"**/testdata/**", "**/fixtures/**"},
		WholeFileTests: []string{"**/test_*.sh", "**/*_test.sh", "**/*.bats"},
		GraphifyPath:   "graphify-out/graph.json",
		MaxStaleRatio:  0.5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadConfig = %+v\nwant %+v", got, want)
	}
}

// A present key replaces its default wholesale; keys the graph does not own are ignored.
func TestLoadConfigKeyReplacesItsDefaultAndOtherKeysAreIgnored(t *testing.T) {
	root := writeConfig(t, "stale_commits: 50\nscan_exclude: [\"gen/**\"]\n")
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gen/**"}; !reflect.DeepEqual(got.ScanExclude, want) {
		t.Errorf("ScanExclude = %v, want %v", got.ScanExclude, want)
	}
	if got.GraphifyPath != "graphify-out/graph.json" {
		t.Errorf("an absent key lost its default: GraphifyPath = %q", got.GraphifyPath)
	}
}

// #504: prose is read for its file node, never for definitions. A config written before
// definition_exclude existed lists the prose globs in scan_exclude; they move out of it.
func TestLoadConfigMovesProseOutOfAnOlderScanExclude(t *testing.T) {
	root := writeConfig(t, "scan_exclude: [\"vendor/**\", \"**/*.md\", \"**/*.txt\"]\n")
	got, err := LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"vendor/**"}; !reflect.DeepEqual(got.ScanExclude, want) {
		t.Errorf("ScanExclude = %v, want %v", got.ScanExclude, want)
	}
	if want := DefaultConfig().DefinitionExclude; !reflect.DeepEqual(got.DefinitionExclude, want) {
		t.Errorf("DefinitionExclude = %v, want the default %v", got.DefinitionExclude, want)
	}

	// Once the file sets definition_exclude, scan_exclude is exactly what it says.
	root = writeConfig(t, "scan_exclude: [\"**/*.md\"]\ndefinition_exclude: [\"docs/**\"]\n")
	if got, err = LoadConfig(root); err != nil {
		t.Fatal(err)
	}
	if want := []string{"**/*.md"}; !reflect.DeepEqual(got.ScanExclude, want) {
		t.Errorf("ScanExclude = %v, want %v", got.ScanExclude, want)
	}
	if want := []string{"docs/**"}; !reflect.DeepEqual(got.DefinitionExclude, want) {
		t.Errorf("DefinitionExclude = %v, want %v", got.DefinitionExclude, want)
	}
}

func TestLoadConfigRejectsAMalformedGlobAndRatio(t *testing.T) {
	for _, body := range []string{"scan_exclude: [\"[\"]\n", "max_stale_ratio: 0\n", "max_stale_ratio: 1.5\n", "scan_exclude: {\n"} {
		if _, err := LoadConfig(writeConfig(t, body)); err == nil {
			t.Errorf("LoadConfig accepted %q", body)
		}
	}
}
