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

// Selection and coverage values. `selection` is contract v2's fidelity key: it says
// whether this toolchain's selection is derived from execution or from declaration.
const (
	// SelectionCoverage is the v1 behaviour and the default: tests are chosen from
	// recorded per-test coverage. An adapter that omits `selection` means this, so
	// every adapter written against v1 keeps its meaning without being edited.
	SelectionCoverage = "coverage"
	// SelectionStatic chooses tests from declared correspondence and imports. Spec §4.2.
	SelectionStatic = "static"
	// CoverageNone is the only coverage value permitted under SelectionStatic: an
	// adapter that records nothing.
	CoverageNone = "none"
)

// Importscan is the optional per-language import scanner. It follows the precedent set by
// internal/importscan: the engine runs a script, it never parses the language itself
// (decision D8). An adapter that omits the block skips import ranking; that is a narrower
// static tier, not an error.
type Importscan struct {
	Command string `yaml:"command"` // e.g. "node {script}"
	Script  string `yaml:"script"`  // shipped beside the adapter
}

// Requirement is one binary this adapter cannot work without, and why. Spec §4.3: Go needs
// go-junit-report, Jest needs jest-junit, RSpec needs rspec_junit_formatter and dotnet needs
// JUnitTestLogger before any of them can emit JUnit XML at all. It exists so an unmet
// prerequisite surfaces at doctor/init time instead of as a mid-run parse failure against a
// report file that was never written.
type Requirement struct {
	Bin    string `yaml:"bin"`
	Reason string `yaml:"reason"`
}

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

	// Selection is contract v2 (spec §4.2). It is optional: an omitted key defaults to
	// SelectionCoverage, which is what every v1 adapter already means.
	Selection string `yaml:"selection"`

	// The rest of contract v2: how a static-tier adapter finds and names its tests
	// (spec §4.2), and what its runner needs installed first (spec §4.3). Every one is
	// optional and none is defaulted, so a v1 adapter is a valid v2 adapter unedited.
	ReportPath string        `yaml:"report_path"` // where the runner leaves its outcome file
	IDTemplate string        `yaml:"id_template"` // how a parsed id renders back into a selector
	TestFor    []string      `yaml:"test_for"`    // path-correspondence templates, tried IN ORDER
	Importscan *Importscan   `yaml:"importscan"`
	Requires   []Requirement `yaml:"requires"`

	// Src is the file this adapter was read from — an fs path inside the embedded set
	// ("python.yaml") or an on-disk path for a host-authored one. It is never declared in
	// YAML: it is how doctor and every error message name the file an adapter came from,
	// and a declaration could lie about it. Spec §4.5.
	Src string `yaml:"-"`
}

// The placeholders each template field may use. They are separate vocabularies because the
// two fields answer different questions: test_for maps a changed SOURCE path to a candidate
// test path, so it knows only where that file sits; id_template renders a parsed TEST id
// back into the runner's own selector syntax, so it knows the JUnit <testcase> attributes —
// classname and name — plus the file the case came from. A placeholder outside its field's
// vocabulary can never be substituted, so it would survive into a path or a selector as a
// literal brace; Load rejects it instead (exit 2).
var (
	testForPlaceholders = map[string]bool{
		"{dir}":  true, // the changed source file's directory, repo-relative
		"{name}": true, // its base name without extension
	}
	idTemplatePlaceholders = map[string]bool{
		"{file}":      true, // the file the test case was parsed from
		"{classname}": true, // the JUnit <testcase classname=...> attribute
		"{name}":      true, // the JUnit <testcase name=...> attribute
	}
)

// applyDefaults fills the two keys that encode one fact between them. An omitted
// selection is today's behaviour, and today's behaviour reads sqlite coverage.
func (a *Adapter) applyDefaults() {
	if a.Selection == "" {
		a.Selection = SelectionCoverage
	}
	if a.Coverage == "" && a.Selection == SelectionCoverage {
		a.Coverage = "sqlite"
	}
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
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var a Adapter
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("adapter: %s: %w", src, err)
	}
	a.Src = src
	a.applyDefaults()
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
	case a.Selection != SelectionCoverage && a.Selection != SelectionStatic:
		return fmt.Errorf("unsupported selection %q (only %q and %q)", a.Selection, SelectionCoverage, SelectionStatic)

	// selection, coverage and seed encode one fact between them: whether this toolchain
	// is instrumented. Each message names BOTH offending keys, because either one of the
	// pair could be the typo. The third case closes the remaining direction, so no
	// combination of the three can express a static tier that also reads coverage.
	// All are configuration errors (exit 2). Spec §4.2.
	case a.Coverage == CoverageNone && a.Selection != SelectionStatic:
		return fmt.Errorf("coverage: none requires selection: static, got selection %q", a.Selection)
	case a.Selection == SelectionStatic && a.Seed != "":
		return fmt.Errorf("selection: static forbids seed, got seed %q", a.Seed)
	case a.Selection == SelectionStatic && a.Coverage != CoverageNone:
		return fmt.Errorf("selection: static requires coverage: none, got coverage %q", a.Coverage)

	// A coverage adapter with no seed command can never build a map. Under
	// SelectionStatic there is nothing to seed, so the field is forbidden above rather
	// than required here.
	case a.Selection == SelectionCoverage && a.Seed == "":
		return fmt.Errorf("seed is required")

	case a.Subset == "":
		return fmt.Errorf("subset is required")
	case !strings.Contains(a.Subset, "{tests}"):
		// Without the placeholder the subset command runs the whole suite, so every
		// selection would silently become a full run.
		return fmt.Errorf("subset %q has no {tests} placeholder", a.Subset)
	case a.Selection == SelectionCoverage && a.Coverage != "sqlite":
		return fmt.Errorf("unsupported coverage %q (only \"sqlite\" and \"none\")", a.Coverage)

	// junit-xml is accepted by the contract here and has no parser until M6c: an adapter
	// declaring it loads and validates, and fails at RUN time on the existing unsupported
	// -report path. Both companion keys are required because a parsed <testcase> that
	// cannot round-trip into `subset` is worthless (spec §4.3, audit finding A6), and a
	// report nobody can locate is worse than no report at all.
	case a.Report == "junit-xml" && a.ReportPath == "":
		return fmt.Errorf("report: junit-xml requires report_path")
	case a.Report == "junit-xml" && a.IDTemplate == "":
		return fmt.Errorf("report: junit-xml requires id_template")
	case a.Report != "pytest-reportlog" && a.Report != "junit-xml":
		return fmt.Errorf("unsupported report %q (only \"pytest-reportlog\" and \"junit-xml\")", a.Report)
	}

	if err := a.validateTemplates(); err != nil {
		return err
	}
	if err := a.validateImportscan(); err != nil {
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

// validateTemplates rejects a placeholder the engine could never substitute. The failure
// it prevents is silent: an unsubstituted {folder} survives into a candidate path, that
// path matches no file on disk, and the adapter simply selects nothing — indistinguishable
// from a repository with no corresponding tests. Naming the field and the placeholder makes
// it a one-line fix instead of a debugging session.
func (a *Adapter) validateTemplates() error {
	if a.IDTemplate != "" {
		if bad := unknownPlaceholder(a.IDTemplate, idTemplatePlaceholders); bad != "" {
			return fmt.Errorf("id_template %q: unknown placeholder %s", a.IDTemplate, bad)
		}
	}
	for i, tmpl := range a.TestFor {
		if bad := unknownPlaceholder(tmpl, testForPlaceholders); bad != "" {
			return fmt.Errorf("test_for[%d] %q: unknown placeholder %s", i, tmpl, bad)
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

// validateImportscan rejects a half-declared scanner. command and script are one
// declaration in two halves: a command with no script has nothing to run, and a script with
// no command has nothing to run it. Omitting the whole block is legal and means import
// ranking is skipped (spec §4.2), so only the halves are checked here.
func (a *Adapter) validateImportscan() error {
	if a.Importscan == nil {
		return nil
	}
	switch {
	case a.Importscan.Command == "" && a.Importscan.Script == "":
		return fmt.Errorf("importscan: command and script are required; omit the whole block to skip import ranking")
	case a.Importscan.Script == "":
		return fmt.Errorf("importscan: script is required alongside command")
	case a.Importscan.Command == "":
		return fmt.Errorf("importscan: command is required alongside script")
	}
	return nil
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
