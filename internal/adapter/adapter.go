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
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	rtddadapters "github.com/VocanicZ/rtdd/adapters"
	"github.com/VocanicZ/rtdd/internal/paths"
)

// Requirement is one binary this adapter cannot work without, and why (one-pipeline spec
// §12: coverage tools must be installed). It exists so an unmet prerequisite surfaces at
// doctor/init time instead of as a unit that leaves no coverage file.
type Requirement struct {
	Bin    string `yaml:"bin"`
	Reason string `yaml:"reason"`
}

type Adapter struct {
	Name         string            `yaml:"name"`
	Detect       []string          `yaml:"detect"`
	Env          map[string]string `yaml:"env"` // values may use {tmp}
	TestGlobs    []string          `yaml:"test_globs"`
	SourceGlobs  []string          `yaml:"source_globs"`
	ExitCodes    map[int]string    `yaml:"exit_codes"`
	Opaque       []string          `yaml:"opaque"`
	FullEscalate []string          `yaml:"full_escalate"`
	Requires     []Requirement     `yaml:"requires"`

	// Contract v3 (docs/specs/2026-09-29-one-pipeline.md §6). One test file runs per
	// process; UnitCmd writes its coverage to CoverageFile under {tmp}.
	UnitCmd        string `yaml:"unit_cmd"`
	UnitNames      string `yaml:"unit_names"`
	CoverageFile   string `yaml:"coverage_file"`
	CoverageFormat string `yaml:"coverage_format"`
	Jobs           int    `yaml:"jobs"`
	// UnitFiles are written into each unit's {tmp} before unit_cmd runs (relative path ->
	// content; content may use {tmp}). It ships a build-tool init script without touching
	// the host's own build files.
	UnitFiles map[string]string `yaml:"unit_files"`

	// Src is the file this adapter was read from — an fs path inside the embedded set
	// ("python.yaml") or an on-disk path for a host-authored one. It is never declared in
	// YAML: it is how doctor and every error message name the file an adapter came from,
	// and a declaration could lie about it. Spec §4.5.
	Src string `yaml:"-"`
}

// removedFields are the contract v2 keys (spec §6). Each is rejected by name, so a host
// adapter written for an older rtdd says what to change instead of "field not found".
var removedFields = []string{"seed", "subset", "subset_plain", "list", "coverage", "report",
	"report_path", "report_cmd", "id_template", "failfast_flag", "test_flag", "test_join",
	"test_selector", "selection", "test_for", "importscan"}

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
//
// os.DirFS makes every path fs-relative, so LoadFS records Src as a bare file name.
// Rewrite it to the path the caller can actually open: Src exists to be printed back to
// someone who has to go and fix the file.
func LoadAll(dir string) ([]*Adapter, error) {
	out, err := LoadFS(os.DirFS(dir), ".")
	if err != nil {
		return nil, err
	}
	for _, a := range out {
		a.Src = filepath.Join(dir, filepath.FromSlash(a.Src))
	}
	return out, nil
}

// Builtin returns the adapters embedded in the binary, so rtdd resolves "python"
// without any adapter file on disk.
func Builtin() ([]*Adapter, error) { return LoadFS(rtddadapters.FS, ".") }

// parse decodes one declaration. Unknown fields are rejected so a typo in a host repo's
// adapter is a configuration error (exit 2) rather than a silently ignored glob.
func parse(b []byte, src string) (*Adapter, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", src, err)
	}
	for _, key := range removedFields {
		if _, ok := raw[key]; ok {
			return nil, fmt.Errorf("adapter: %s: %s: removed in contract v3 (docs/specs/2026-09-29-one-pipeline.md §6)", src, key)
		}
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var a Adapter
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", src, err)
	}
	a.Src = src
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
	case a.UnitCmd == "":
		return fmt.Errorf("unit_cmd is required")
	case a.CoverageFile == "":
		return fmt.Errorf("coverage_file is required")
	case a.CoverageFormat == "":
		return fmt.Errorf("coverage_format is required")
	}
	if err := a.validateV3(); err != nil {
		return err
	}
	// A prerequisite doctor cannot explain is not worth declaring: the reason is printed
	// verbatim in the finding (spec §4.3, PRD #229 AC9).
	for i, r := range a.Requires {
		switch {
		case r.Bin == "":
			return fmt.Errorf("requires[%d]: bin is required", i)
		case r.Reason == "":
			return fmt.Errorf("requires[%d] (%s): reason is required", i, r.Bin)
		}
	}
	return nil
}

// unknownPlaceholder returns the first {…} run in tmpl that known does not contain, or ""
// when every one of them is recognised. An unterminated brace is not a placeholder — the
// runner's own selector syntax may well contain one — so it is left alone.
func unknownPlaceholder(tmpl string, known map[string]bool) string {
	rest := tmpl
	for {
		open := strings.Index(rest, "{")
		if open < 0 {
			return ""
		}
		shut := strings.Index(rest[open:], "}")
		if shut < 0 {
			return ""
		}
		ph := rest[open : open+shut+1]
		if !known[ph] {
			return ph
		}
		rest = rest[open+shut+1:]
	}
}

// validateGlobs rejects a malformed pattern in any field the engine globs against.
// A typo'd glob would otherwise match nothing, so nothing would be classified as a test
// file and the direct tier — the tier that must never depend on the map — would go empty
// while `rtdd which` reported "no test file changed". That is a configuration error
// (exit 2), and it is caught here, once, at load time.
//
// detect belongs in the table with the four classification fields even though it is not
// globbed by the classifier: Detect matches it against every file in the repo with the
// same matcher, and paths.MatchGlob panics by design on a pattern ValidateGlob rejects.
// Load is the only place allowed to see a malformed glob, so it must see all five.
func (a *Adapter) validateGlobs() error {
	for _, f := range []struct {
		field string
		globs []string
	}{
		{"detect", a.Detect},
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
