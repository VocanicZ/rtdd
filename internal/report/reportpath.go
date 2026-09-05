package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	// ErrReportPathKind is a report_path that exists but is the wrong KIND: a file where
	// the declaration named a directory, or the reverse. The declaration and the disk
	// disagree, and only the adapter's author can say which is right — so the error names
	// the adapter and the path rather than falling back to either reading.
	ErrReportPathKind = errors.New("junit-xml: report_path is the wrong kind")
	// ErrDuplicateTestID is one rendered id produced by two different files of the same
	// directory report. Merging them silently would drop one outcome and report the
	// other's status under both names, which is a false green wearing the right id.
	ErrDuplicateTestID = errors.New("junit-xml: duplicate test id across report files")
)

// ReportPath is an adapter's report_path resolved against the repo root. The shape is read
// from the DECLARATION, never sniffed from disk: a trailing "/" means a directory of
// *.xml, anything else means exactly one file. Sniffing would make the meaning of an
// adapter depend on whether the previous run happened to leave a directory behind.
type ReportPath struct {
	Abs   string // absolute, cleaned
	IsDir bool

	// Declared is the report_path exactly as the adapter wrote it, and Adapter is the
	// adapter that declared it. Both exist so an error can say WHOSE declaration is
	// wrong; both are cosmetic, and an empty Adapter simply drops the prefix.
	Declared string
	Adapter  string
}

// NewReportPath resolves a declared report_path against repoRoot.
func NewReportPath(repoRoot, declared string) (ReportPath, error) {
	return NewReportPathFor("", repoRoot, declared)
}

// NewReportPathFor is NewReportPath with the declaring adapter's name, which every error
// this type produces then carries.
func NewReportPathFor(adapter, repoRoot, declared string) (ReportPath, error) {
	if declared == "" {
		return ReportPath{}, fmt.Errorf("%sreport_path is empty", adapterPrefix(adapter))
	}
	slashed := filepath.ToSlash(declared)
	if filepath.IsAbs(declared) || strings.HasPrefix(slashed, "/") {
		return ReportPath{}, fmt.Errorf("%sreport_path %q: must be relative to the repository root", adapterPrefix(adapter), declared)
	}
	// The shape is read before Clean, which strips the trailing separator that carries it.
	isDir := strings.HasSuffix(slashed, "/")
	clean := filepath.ToSlash(filepath.Clean(slashed))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ReportPath{}, fmt.Errorf("%sreport_path %q: must not leave the repository root", adapterPrefix(adapter), declared)
	}
	return ReportPath{
		Abs:      filepath.Join(repoRoot, filepath.FromSlash(clean)),
		IsDir:    isDir,
		Declared: declared,
		Adapter:  adapter,
	}, nil
}

// adapterPrefix names the declaring adapter when one is known.
func adapterPrefix(adapter string) string {
	if adapter == "" {
		return ""
	}
	return fmt.Sprintf("adapter %q: ", adapter)
}

// prefix is adapterPrefix for a resolved path.
func (p ReportPath) prefix() string { return adapterPrefix(p.Adapter) }

// Files returns the report files to read, sorted: the single file, or every *.xml
// DIRECTLY inside the directory. Sorted rather than readdir order so a merged report is
// reproducible.
func (p ReportPath) Files() ([]string, error) {
	info, err := os.Stat(p.Abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s%w: %s", p.prefix(), ErrNoReport, p.Abs)
	}
	if err != nil {
		return nil, fmt.Errorf("%sreading %s: %w", p.prefix(), p.Abs, err)
	}
	if info.IsDir() != p.IsDir {
		return nil, fmt.Errorf("%s%w: report_path %q names %s, but %s is %s",
			p.prefix(), ErrReportPathKind, p.Declared, kindWord(p.IsDir), p.Abs, kindWord(info.IsDir()))
	}
	if !p.IsDir {
		return []string{p.Abs}, nil
	}
	entries, err := os.ReadDir(p.Abs)
	if err != nil {
		return nil, fmt.Errorf("%sreading %s: %w", p.prefix(), p.Abs, err)
	}
	var files []string
	for _, e := range entries {
		// Not recursive: Surefire and Gradle write their reports flat, and a nested
		// directory in a build output belongs to something other than this run.
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".xml") {
			continue
		}
		files = append(files, filepath.Join(p.Abs, e.Name()))
	}
	if len(files) == 0 {
		// A directory holding no *.xml is not zero tests: it is a runner that wrote
		// nothing, which is the same false green ErrNoReport already names.
		return nil, fmt.Errorf("%s%w: %s holds no *.xml", p.prefix(), ErrNoReport, p.Abs)
	}
	sort.Strings(files)
	return files, nil
}

// kindWord renders a shape for an error message.
func kindWord(isDir bool) string {
	if isDir {
		return "a directory"
	}
	return "a file"
}

// ReadJUnitReport reads every file Files names and renders each case's id through tmpl,
// producing the same []Outcome the pytest-reportlog path produces. It does not
// de-duplicate: internal/runner already de-duplicates across chunks with "last invocation
// wins", and two de-dupers with different rules is how that rule stops being true. Two
// files of ONE directory claiming the same id are a different thing — nothing downstream
// can tell those apart, so they are named here.
func ReadJUnitReport(p ReportPath, tmpl string) ([]Outcome, error) {
	files, err := p.Files()
	if err != nil {
		return nil, err
	}
	var outs []Outcome
	seen := make(map[string]string, 64) // rendered id -> the file that first produced it
	for _, f := range files {
		cases, err := ReadJUnitFile(f)
		if err != nil {
			return nil, err
		}
		for _, c := range cases {
			id, err := RenderID(tmpl, c)
			if err != nil {
				return nil, fmt.Errorf("%s%s: %w", p.prefix(), f, err)
			}
			if first, dup := seen[id]; dup && first != f {
				return nil, fmt.Errorf("%s%w: %q is in both %s and %s",
					p.prefix(), ErrDuplicateTestID, id, filepath.Base(first), filepath.Base(f))
			}
			seen[id] = f
			outs = append(outs, Outcome{Test: id, Status: c.Status, DurationMS: c.DurationMS})
		}
	}
	return outs, nil
}
