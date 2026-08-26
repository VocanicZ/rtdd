// Package adapter declares what is genuinely declarative about a language toolchain.
// Execution and parsing are implemented per language in M1b; M1a uses only the
// glob-classification half.
package adapter

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/VocanicZ/rtdd/internal/paths"
)

type Adapter struct {
	Name         string            `yaml:"name"`
	Detect       []string          `yaml:"detect"`
	Env          map[string]string `yaml:"env"`
	Seed         string            `yaml:"seed"`
	Subset       string            `yaml:"subset"`
	List         string            `yaml:"list"`
	Coverage     string            `yaml:"coverage"` // "sqlite"
	Report       string            `yaml:"report"`   // "pytest-reportlog"
	FailFastFlag string            `yaml:"failfast_flag"`
	TestGlobs    []string          `yaml:"test_globs"`
	SourceGlobs  []string          `yaml:"source_globs"`
	ExitCodes    map[int]string    `yaml:"exit_codes"`
	Opaque       []string          `yaml:"opaque"`
	FullEscalate []string          `yaml:"full_escalate"`
}

// Load reads one adapter declaration. Unknown fields are rejected so a typo in a host
// repo's adapter is a configuration error (exit 2) rather than a silently ignored glob.
func Load(path string) (*Adapter, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("adapter: read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var a Adapter
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", path, err)
	}
	if a.Name == "" {
		return nil, fmt.Errorf("adapter: %s: missing required field %q", path, "name")
	}
	if err := a.validateGlobs(); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", path, err)
	}
	return &a, nil
}

// validateGlobs rejects a malformed pattern in any field the classifier globs against.
// A typo'd glob would otherwise match nothing, so nothing would be classified as a test
// file and the direct tier — the tier that must never depend on the map — would go empty
// while `rtdd which` reported "no test file changed". That is a configuration error
// (exit 2), and it is caught here, once, at load time.
func (a *Adapter) validateGlobs() error {
	for _, f := range []struct {
		field string
		globs []string
	}{
		{"test_globs", a.TestGlobs},
		{"source_globs", a.SourceGlobs},
		{"opaque", a.Opaque},
		{"full_escalate", a.FullEscalate},
	} {
		for _, g := range f.globs {
			if err := paths.ValidateGlob(g); err != nil {
				return fmt.Errorf("%s: %w", f.field, err)
			}
		}
	}
	return nil
}

// IsTestFile reports whether rel is a file the runner may name as a test selector.
// FullEscalate wins over TestGlobs: a fixture module such as tests/conftest.py matches
// a broad test glob like tests/**/*.py, but collects no tests. Naming it as a selector
// makes the runner exit 5 (no-tests-collected), which is fatal. A change to it escalates
// to a full run through IsFullEscalate instead.
func (a *Adapter) IsTestFile(rel string) bool {
	return a != nil && anyGlob(a.TestGlobs, rel) && !anyGlob(a.FullEscalate, rel)
}

func (a *Adapter) IsOpaque(rel string) bool {
	return a != nil && anyGlob(a.Opaque, rel)
}

func (a *Adapter) IsFullEscalate(rel string) bool {
	return a != nil && anyGlob(a.FullEscalate, rel)
}

// IsInstrumentable: matches SourceGlobs AND is not a test file AND is not Opaque.
func (a *Adapter) IsInstrumentable(rel string) bool {
	if a == nil {
		return false
	}
	return anyGlob(a.SourceGlobs, rel) && !a.IsTestFile(rel) && !a.IsOpaque(rel)
}

func anyGlob(patterns []string, rel string) bool {
	for _, p := range patterns {
		if paths.MatchGlob(p, rel) {
			return true
		}
	}
	return false
}
