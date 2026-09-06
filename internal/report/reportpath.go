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
	// ErrReportPathSymlink is a report_path ANY element of which is a SYMLINK. It is
	// refused rather than followed: Clear empties the path before every invocation, and
	// a link resolves to a directory the repository does not own, so following one turns
	// "clear the previous run's report" into deleting files outside the repo root.
	// Refusing beats resolving-and-checking-containment — a link that stays inside the
	// root today is one `ln -sf` from not doing so tomorrow, and only the adapter's
	// author can say which real path was meant.
	ErrReportPathSymlink = errors.New("junit-xml: report_path is a symlink")
	// ErrClearReportPath is a report_path the engine could not empty before invocation.
	// It is raised BEFORE the runner starts, because the alternative — continue and read
	// whatever is there — parses the previous run's report as this run's result.
	ErrClearReportPath = errors.New("junit-xml: cannot clear report_path")
)

// ReportPath is an adapter's report_path resolved against the repo root. The shape is read
// from the DECLARATION, never sniffed from disk: a trailing "/" means a directory of
// *.xml, anything else means exactly one file. Sniffing would make the meaning of an
// adapter depend on whether the previous run happened to leave a directory behind.
type ReportPath struct {
	Abs   string // absolute, cleaned
	IsDir bool

	// Root is the repository root Abs was resolved against, cleaned. The clear and the
	// read walk from it down to Abs to refuse a symlink at ANY element the declaration
	// names, so they have to know where the declaration's own elements begin: a link
	// ABOVE the root — a checkout under /tmp reached through /tmp -> /private/tmp — is
	// not the declaration's doing and is not its author's to fix.
	Root string

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
		Root:     filepath.Clean(repoRoot),
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

// Clear removes the previous run's report so a runner that writes nothing produces
// ErrNoReport rather than last run's outcomes. For a file: remove it; a missing file is
// not an error. For a directory: create it if absent, then remove every *.xml DIRECTLY
// inside it — never the directory itself and never a non-XML sibling, because
// target/surefire-reports also holds the *.txt dumps a developer may be reading.
//
// Nothing outside p.Abs is ever touched, and three rules together are what make that true:
// NewReportPathFor has already refused an absolute path and one that climbs out of the repo
// root; EVERY element the declaration names, from the repository root down to p.Abs, is
// refused here rather than followed if it is a SYMLINK, because a link at any of them —
// "build" as readily as "build/reports" — resolves to a directory the repo root does not
// contain; and the removals are then p.Abs itself or its direct children — each removed
// with os.Remove, which unlinks a symlinked child without touching what it points at. The
// declared shape decides which, so a file-shaped declaration pointing at a directory
// removes nothing and is named instead — a directory the adapter did not name is not this
// function's to delete.
func (p ReportPath) Clear() error {
	// The symlink walk comes first, before anything that would resolve one: MkdirAll and
	// Remove both follow the parent chain, so an intermediate link reaches the target
	// even when p.Abs itself does not exist yet.
	if err := p.symlinkChainErr(); err != nil {
		return err
	}
	// Then the kind check, before MkdirAll on a path where a file sits or Remove on a
	// directory that happens to be empty — each would answer the disagreement itself. It
	// is an Lstat, not a Stat, for the same reason the walk uses one.
	if info, err := os.Lstat(p.Abs); err == nil {
		if info.IsDir() != p.IsDir {
			return p.kindErr(info.IsDir())
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return p.clearErr(p.Abs, err)
	}

	// A runner told to write into a directory that does not exist writes nothing, and the
	// run then fails on a report it could have had.
	dir := p.Abs
	if !p.IsDir {
		dir = filepath.Dir(p.Abs)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return p.clearErr(dir, err)
	}
	if !p.IsDir {
		if err := os.Remove(p.Abs); err != nil && !errors.Is(err, os.ErrNotExist) {
			return p.clearErr(p.Abs, err)
		}
		return nil
	}
	entries, err := os.ReadDir(p.Abs)
	if err != nil {
		return p.clearErr(p.Abs, err)
	}
	for _, e := range entries {
		// Exactly the set Files reads. Clearing more than the read would delete a file
		// no RTDD run was ever going to look at.
		if !isReportEntry(e) {
			continue
		}
		f := filepath.Join(p.Abs, e.Name())
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			return p.clearErr(f, err)
		}
	}
	return nil
}

// kindErr is the declaration and the disk disagreeing about the shape. Clear raises it
// before the run and Files after it, and both say the same thing.
func (p ReportPath) kindErr(onDiskIsDir bool) error {
	return fmt.Errorf("%s%w: report_path %q names %s, but %s is %s",
		p.prefix(), ErrReportPathKind, p.Declared, kindWord(p.IsDir), p.Abs, kindWord(onDiskIsDir))
}

// symlinkErr is a report_path one of whose elements is a link. It names the adapter, the
// declaration and the OFFENDING element, because the fix is in the declaration and the
// element says which part of it to change: only the adapter's author can say which real
// path was meant, and RTDD guessing — by resolving the link and checking that the target
// is still inside the root — would bless a link that is repointed tomorrow.
func (p ReportPath) symlinkErr(elem string) error {
	return fmt.Errorf("%s%w: report_path %q: %s is a symlink, which RTDD clears before every invocation and will not follow; declare the path it points at",
		p.prefix(), ErrReportPathSymlink, p.Declared, elem)
}

// symlinkChainErr walks the elements the declaration names — from p.Root down to p.Abs,
// inclusive — and refuses the path if any one of them is a symlink. Checking p.Abs alone
// would miss the ordinary case: "build", "target" and "reports" are exactly the
// directories a monorepo links into a shared out-of-tree output area, and an Lstat of
// build/reports/ says nothing about build. The walk starts AT the root and not above it
// because a link on the way to the root — a checkout under /tmp on macOS, reached through
// /tmp -> /private/tmp — belongs to the checkout rather than to the declaration, and
// refusing it would refuse every report_path in that repository.
func (p ReportPath) symlinkChainErr() error {
	if p.Root == "" {
		// A ReportPath built by hand rather than by the constructor has no root to walk
		// from, and no way to tell the declaration's own elements from the ones above
		// them. The final element is still checked, which is what #286 guaranteed.
		return p.elementSymlinkErr(p.Abs)
	}
	rel, err := filepath.Rel(p.Root, p.Abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// p.Abs is not under p.Root — which the constructor's own checks already rule
		// out — so there is no element chain to walk, only the path itself.
		return p.elementSymlinkErr(p.Abs)
	}
	at := p.Root
	if rel != "." {
		for _, e := range strings.Split(rel, string(filepath.Separator)) {
			at = filepath.Join(at, e)
			if err := p.elementSymlinkErr(at); err != nil {
				return err
			}
		}
	}
	return nil
}

// elementSymlinkErr is the refusal for ONE element, and nil for everything else: an
// element that does not exist yet is not a link, and one Lstat cannot answer for is left
// to the clear or the read, which report the OS error with the context it belongs to.
func (p ReportPath) elementSymlinkErr(elem string) error {
	info, err := os.Lstat(elem)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return p.symlinkErr(elem)
}

// clearErr names the path the clear stumbled on and keeps the OS error inspectable, so a
// caller can still tell a permission problem from a busy file.
func (p ReportPath) clearErr(path string, err error) error {
	return fmt.Errorf("%s%w: %s: %w", p.prefix(), ErrClearReportPath, path, err)
}

// Files returns the report files to read, sorted: the single file, or every *.xml
// DIRECTLY inside the directory. Sorted rather than readdir order so a merged report is
// reproducible.
func (p ReportPath) Files() ([]string, error) {
	// The same symlink walk Clear does, and refused here too — every element of it: a
	// path Clear will not empty is not one this run may read either, and the two
	// disagreeing is how a report gets read out of a directory nothing cleared.
	if err := p.symlinkChainErr(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p.Abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s%w: %s", p.prefix(), ErrNoReport, p.Abs)
	}
	if err != nil {
		return nil, fmt.Errorf("%sreading %s: %w", p.prefix(), p.Abs, err)
	}
	if info.IsDir() != p.IsDir {
		return nil, p.kindErr(info.IsDir())
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
		if !isReportEntry(e) {
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

// isReportEntry is a directory entry that belongs to a directory report: a *.xml file
// DIRECTLY inside it. Not recursive — Surefire and Gradle write their reports flat, and a
// nested directory in a build output belongs to something other than this run. One
// predicate for both the read and the clear, because a file cleared but never read is a
// deletion RTDD had no reason to make, and one read but never cleared goes stale.
func isReportEntry(e os.DirEntry) bool {
	return !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".xml")
}

// kindWord renders a shape for an error message.
func kindWord(isDir bool) string {
	if isDir {
		return "a directory"
	}
	return "a file"
}

// ReadJUnitReport reads every file Files names and renders each case's id through tmpl,
// producing the same []Outcome the pytest-reportlog path produces.
//
// Two cases of ONE file that render the same id are folded into one outcome,
// worst-status-wins: error > fail > skip > pass. A file-granular id_template — Vitest's
// {classname}, RSpec's {file} — is deliberately shared by every case in the file, and a
// per-test template collides too whenever a runner permits two tests of one file to carry
// the same name. Emitting both outcomes handed the collapse to internal/runner's
// "last invocation wins", which would let a passing case erase the failing one ahead of
// it and report a red run green. Folding here is not a second de-duplicator disagreeing
// with that rule: the runner's rule is about the SAME id seen in two invocations, where
// the later invocation is genuinely the newer result, and it still decides that. This one
// is about one invocation's own report, where neither case supersedes the other and the
// id is only green when every case behind it is.
//
// Two files of ONE directory claiming the same id are a different thing — nothing
// downstream can tell those apart, so they are named here rather than folded.
func ReadJUnitReport(p ReportPath, tmpl string) ([]Outcome, error) {
	files, err := p.Files()
	if err != nil {
		return nil, err
	}
	var outs []Outcome
	seen := make(map[string]string, 64) // rendered id -> the file that first produced it
	at := make(map[string]int, 64)      // rendered id -> its index in outs
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
			if i, folded := at[id]; folded {
				// The id keeps the position of its first case and accumulates the time
				// every case behind it took: a file-granular id stands for the whole
				// file, so its duration is the file's rather than one case's.
				outs[i].Status = worseStatus(outs[i].Status, c.Status)
				outs[i].DurationMS += c.DurationMS
				continue
			}
			seen[id] = f
			at[id] = len(outs)
			outs = append(outs, Outcome{Test: id, Status: c.Status, DurationMS: c.DurationMS})
		}
	}
	return outs, nil
}

// statusRank orders the Outcome vocabulary by how much it matters that a reader sees it:
// an error outranks a failure, either outranks a skip, and anything outranks a pass. An
// unrecognised status ranks above pass, because the one thing a status this reader does
// not know must never do is silently become green.
func statusRank(s string) int {
	switch s {
	case "error":
		return 4
	case "fail":
		return 3
	case "skip":
		return 2
	case "pass":
		return 0
	default:
		return 1
	}
}

// worseStatus is the survivor when two cases share one id.
func worseStatus(a, b string) string {
	if statusRank(b) > statusRank(a) {
		return b
	}
	return a
}
