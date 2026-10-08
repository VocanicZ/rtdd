package graph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// Config is the node graph's part of .rtdd/config.yaml. Every list is generic — a glob
// over repo-relative paths — never a per-language setting (spec §4.1, §6).
type Config struct {
	ScanExclude   []string // files the scanner never reads (spec §4.1)
	TestFiles     []string // a node in a matching file may be a test (spec §6)
	TestExclude   []string // a file matching these is never a test file, whatever TestFiles says
	GraphifyPath  string   // repo-relative path of graphify's graph.json (spec §5)
	MaxStaleRatio float64  // graphify is ignored when more than this share of its code files is stale
}

// DefaultConfig is spec §4.1, §5 and §6's defaults.
func DefaultConfig() Config {
	return Config{
		ScanExclude: []string{"vendor/**", "node_modules/**", "third_party/**", "**/*.min.js",
			"dist/**", "build/**", ".rtdd/**", "graphify-out/**",
			"**/*.md", "**/*.mdc", "**/*.markdown", "**/*.rst", "**/*.txt", "**/*.adoc"},
		TestFiles: []string{"**/test_*", "**/*_test.*", "**/*.test.*", "**/*.spec.*", "**/*Test.*",
			"**/*Tests.*", "**/tests/**", "**/test/**", "**/spec/**", "**/__tests__/**"},
		TestExclude:   []string{"**/testdata/**", "**/fixtures/**"},
		GraphifyPath:  "graphify-out/graph.json",
		MaxStaleRatio: 0.5,
	}
}

// configFile is the on-disk shape. Pointers tell "absent" from "empty": a key that is
// present REPLACES its default wholesale; an absent key keeps it. Keys rtdd does not
// read here (stale_commits, adapters, ...) are ignored, not rejected.
type configFile struct {
	ScanExclude   *[]string `yaml:"scan_exclude"`
	TestFiles     *[]string `yaml:"test_files"`
	TestExclude   *[]string `yaml:"test_exclude"`
	GraphifyPath  *string   `yaml:"graphify_path"`
	MaxStaleRatio *float64  `yaml:"max_stale_ratio"`
}

// LoadConfig reads <root>/.rtdd/config.yaml over DefaultConfig. A missing file is the
// defaults; a malformed file, a malformed glob or a ratio outside (0, 1] is an error.
func LoadConfig(root string) (Config, error) {
	cfg := DefaultConfig()
	p := filepath.Join(root, ".rtdd", "config.yaml")
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var f configFile
	if err := yaml.Unmarshal(b, &f); err != nil {
		return cfg, fmt.Errorf("%s: %w", p, err)
	}
	for _, l := range []struct {
		key string
		in  *[]string
		out *[]string
	}{{"scan_exclude", f.ScanExclude, &cfg.ScanExclude}, {"test_files", f.TestFiles, &cfg.TestFiles}, {"test_exclude", f.TestExclude, &cfg.TestExclude}} {
		if l.in == nil {
			continue
		}
		for _, g := range *l.in {
			if err := paths.ValidateGlob(g); err != nil {
				return cfg, fmt.Errorf("%s: %s: %w", p, l.key, err)
			}
		}
		*l.out = *l.in
	}
	if f.GraphifyPath != nil {
		cfg.GraphifyPath = *f.GraphifyPath
	}
	if f.MaxStaleRatio != nil {
		if r := *f.MaxStaleRatio; r <= 0 || r > 1 {
			return cfg, fmt.Errorf("%s: max_stale_ratio %v is outside (0, 1]", p, r)
		}
		cfg.MaxStaleRatio = *f.MaxStaleRatio
	}
	return cfg, nil
}
