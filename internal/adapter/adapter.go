// Package adapter loads language adapter definitions, and classifies a host repo's
// files against their globs. The YAML declares what is genuinely declarative about a
// toolchain; execution and parsing stay in Go (spec §8, decision D8).
package adapter

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	rtddadapters "github.com/VocanicZ/rtdd/adapters"
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

// Load reads one adapter declaration from a file on disk.
func Load(p string) (*Adapter, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("adapter: read %s: %w", p, err)
	}
	return parse(b, p)
}

// LoadFS reads every *.yaml directly under dir in fsys, sorted by adapter name. It is
// the one reader: LoadAll and Builtin differ only in the fs.FS they hand it, so a host
// repo's adapters and the embedded ones are validated by identical code.
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("adapter: read dir %s: %w", dir, err)
	}
	var out []*Adapter
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".yaml" {
			continue
		}
		full := path.Join(dir, e.Name())
		b, err := fs.ReadFile(fsys, full)
		if err != nil {
			return nil, fmt.Errorf("adapter: read %s: %w", full, err)
		}
		a, err := parse(b, full)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// LoadAll reads every *.yaml in the on-disk directory dir.
func LoadAll(dir string) ([]*Adapter, error) { return LoadFS(os.DirFS(dir), ".") }

// Builtin returns the adapters embedded in the binary, so rtdd resolves "python"
// without any adapter file on disk.
func Builtin() ([]*Adapter, error) { return LoadFS(rtddadapters.FS, ".") }

// parse decodes one declaration. Unknown fields are rejected so a typo in a host repo's
// adapter is a configuration error (exit 2) rather than a silently ignored glob.
func parse(b []byte, src string) (*Adapter, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var a Adapter
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", src, err)
	}
	if err := a.validate(); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", src, err)
	}
	return &a, nil
}

// validate gives every rejection its own message: an agent that reads exit 2 must be
// told which field is wrong, not merely that the adapter is invalid.
//
// Globs are checked first. A malformed pattern is an unambiguous typo, and reporting it
// ahead of a missing-field message keeps the offending pattern in the error even for a
// partial adapter.
func (a *Adapter) validate() error {
	if err := a.validateGlobs(); err != nil {
		return err
	}
	switch {
	case a.Name == "":
		return fmt.Errorf("name is required")
	case len(a.Detect) == 0:
		return fmt.Errorf("detect is required")
	case a.Seed == "":
		return fmt.Errorf("seed is required")
	case a.Subset == "":
		return fmt.Errorf("subset is required")
	case !strings.Contains(a.Subset, "{tests}"):
		// Without the placeholder the subset command runs the whole suite, so every
		// selection would silently become a full run.
		return fmt.Errorf("subset %q has no {tests} placeholder", a.Subset)
	case a.Coverage != "sqlite":
		return fmt.Errorf("unsupported coverage %q (only \"sqlite\" in v1)", a.Coverage)
	case a.Report != "pytest-reportlog":
		return fmt.Errorf("unsupported report %q (only \"pytest-reportlog\" in v1)", a.Report)
	}
	return nil
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
