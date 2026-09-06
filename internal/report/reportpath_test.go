package report

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Decision 3: the shape comes from the declaration. A trailing slash is a directory of
// *.xml; anything else is exactly one file.
func TestNewReportPathReadsTheShapeFromTheDeclarationNotTheDisk(t *testing.T) {
	root := t.TempDir()
	// A directory exists at the file-shaped declaration, and nothing exists at the
	// directory-shaped one: if the shape were sniffed, both answers would be wrong.
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	file, err := NewReportPath(root, "reports")
	if err != nil {
		t.Fatalf("NewReportPath(file-shaped): %v", err)
	}
	if file.IsDir {
		t.Errorf("%q parsed as a directory; only a trailing slash means directory", "reports")
	}
	dir, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath(dir-shaped): %v", err)
	}
	if !dir.IsDir {
		t.Errorf("%q parsed as a file; a trailing slash means directory", "target/surefire-reports/")
	}
	if !strings.HasPrefix(dir.Abs, root) {
		t.Errorf("Abs = %q, want it under the repo root %q", dir.Abs, root)
	}
}

// A report_path that escapes the repo root, or is absolute, is a configuration error: the
// engine clears this path, and clearing outside the repository is not a thing RTDD does.
func TestNewReportPathRefusesToEscapeTheRepoRoot(t *testing.T) {
	root := t.TempDir()
	for _, declared := range []string{"../outside.xml", "/etc/junit.xml", ""} {
		if _, err := NewReportPath(root, declared); err == nil {
			t.Errorf("NewReportPath(%q) = nil error, want a rejection", declared)
		}
	}
}

// Surefire and Gradle write a directory of files; the merged read is every *.xml directly
// inside it, in sorted order, and nothing else.
func TestFilesReadsEveryXMLInADirectoryInSortedOrder(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "target", "surefire-reports")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range map[string]string{
		"TEST-b.xml":        `<testsuite name="b"><testcase classname="b" name="two"/></testsuite>`,
		"TEST-a.xml":        `<testsuite name="a"><testcase classname="a" name="one"/></testsuite>`,
		"a.txt":             "not xml",
		"nested/TEST-c.xml": `<testsuite name="c"><testcase classname="c" name="three"/></testsuite>`,
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	p, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	files, err := p.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	var got []string
	for _, f := range files {
		got = append(got, filepath.Base(f))
	}
	want := []string{"TEST-a.xml", "TEST-b.xml"}
	if len(got) != len(want) {
		t.Fatalf("Files = %v, want %v (sorted, *.xml only, not recursive)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Files[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The directory read is one report: the cases of every file, in file order, rendered
// through one id_template into the Outcome vocabulary the map already speaks.
func TestReadJUnitReportMergesADirectoryIntoOneOutcomeList(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("TEST-a.xml", `<testsuite name="a" time="0.02">
  <testcase classname="a.A" name="one" time="0.02"/>
</testsuite>`)
	write("TEST-b.xml", `<testsuite name="b" time="0.03">
  <testcase classname="b.B" name="two" time="0.03"><failure message="nope"/></testcase>
</testsuite>`)

	p, err := NewReportPath(root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	outs, err := ReadJUnitReport(p, "{classname}#{name}")
	if err != nil {
		t.Fatalf("ReadJUnitReport: %v", err)
	}
	want := []Outcome{
		{Test: "a.A#one", Status: "pass", DurationMS: 20},
		{Test: "b.B#two", Status: "fail", DurationMS: 30},
	}
	if len(outs) != len(want) {
		t.Fatalf("got %+v, want %+v", outs, want)
	}
	for i := range want {
		if outs[i] != want[i] {
			t.Errorf("outcome %d = %+v, want %+v", i, outs[i], want[i])
		}
	}
}

// One file is the other shape, and it is read through exactly the same call.
func TestReadJUnitReportReadsASingleFileReportPath(t *testing.T) {
	root := t.TempDir()
	body := `<testsuite name="only" time="0.01"><testcase classname="only.One" name="works" time="0.01"/></testsuite>`
	if err := os.WriteFile(filepath.Join(root, "junit.xml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	p, err := NewReportPath(root, "junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	outs, err := ReadJUnitReport(p, "{classname}#{name}")
	if err != nil {
		t.Fatalf("ReadJUnitReport: %v", err)
	}
	want := []Outcome{{Test: "only.One#works", Status: "pass", DurationMS: 10}}
	if len(outs) != 1 || outs[0] != want[0] {
		t.Fatalf("got %+v, want %+v", outs, want)
	}
}

// A directory that exists and holds no *.xml is not zero tests: it is a runner that wrote
// nothing, which is ErrNoReport by another route.
func TestReadJUnitReportTreatsAnEmptyDirectoryAsNoReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p, err := NewReportPath(root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if _, err := ReadJUnitReport(p, "{classname}#{name}"); err == nil {
		t.Fatalf("ReadJUnitReport = nil error for a directory holding no report")
	}
}

// Two files in one directory that render the SAME id are not a merge: one of them is
// about to be dropped, and a silent last-wins would report the loser's outcome as the
// winner's. Name it instead, with both files and the adapter that declared the template.
func TestReadJUnitReportRejectsADuplicateTestIDAcrossTwoFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dup := `<testsuite name="s" time="0.01"><testcase classname="a.A" name="one" time="0.01"/></testsuite>`
	for _, name := range []string{"TEST-a.xml", "TEST-b.xml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(dup), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	p, err := NewReportPathFor("maven", root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	_, err = ReadJUnitReport(p, "{classname}#{name}")
	if !errors.Is(err, ErrDuplicateTestID) {
		t.Fatalf("ReadJUnitReport error = %v, want errors.Is(_, ErrDuplicateTestID)", err)
	}
	for _, want := range []string{"maven", "a.A#one", "TEST-a.xml", "TEST-b.xml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// A report_path that exists but is the wrong KIND is an adapter bug, not a missing
// report: the declaration and the disk disagree, and only a human can decide which is
// right. Both directions are named, and both name the adapter and the path.
func TestFilesNamesAReportPathOfTheWrongKind(t *testing.T) {
	root := t.TempDir()
	// Declared as a directory, but a regular file sits there.
	if err := os.WriteFile(filepath.Join(root, "reports"), []byte("<testsuite/>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Declared as a file, but a directory sits there.
	if err := os.MkdirAll(filepath.Join(root, "junit.xml"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	dirDecl, err := NewReportPathFor("gradle", root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor(dir): %v", err)
	}
	if _, err := dirDecl.Files(); !errors.Is(err, ErrReportPathKind) {
		t.Fatalf("Files(dir declared, file on disk) error = %v, want errors.Is(_, ErrReportPathKind)", err)
	} else {
		for _, want := range []string{"gradle", "reports"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	}

	fileDecl, err := NewReportPathFor("vitest", root, "junit.xml")
	if err != nil {
		t.Fatalf("NewReportPathFor(file): %v", err)
	}
	if _, err := fileDecl.Files(); !errors.Is(err, ErrReportPathKind) {
		t.Fatalf("Files(file declared, directory on disk) error = %v, want errors.Is(_, ErrReportPathKind)", err)
	} else {
		for _, want := range []string{"vitest", "junit.xml"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	}
}

// PRD #231 AC7: the previous run's report must not survive into this one. The failure it
// prevents is the worst kind — a subset run whose runner never started, reporting the
// outcomes of a run that is not this one.
func TestClearRemovesAStaleSingleFileReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stale := filepath.Join(root, ".rtdd", "junit.xml")
	if err := os.WriteFile(stale, []byte(`<testsuite name="old"><testcase classname="old" name="passed"/></testsuite>`), 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	p, err := NewReportPath(root, ".rtdd/junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale report still present after Clear (stat err = %v)", err)
	}
	if _, err := ReadJUnitReport(p, "{classname}#{name}"); !errors.Is(err, ErrNoReport) {
		t.Fatalf("after Clear, read error = %v, want errors.Is(_, ErrNoReport)", err)
	}
	// Clearing a path that is already absent is not an error: the first run of a repo.
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear on an absent report: %v", err)
	}
}

// For a directory the clear is surgical: this run's *.xml go, the directory stays, and
// everything that is not XML stays — Surefire's own *.txt dumps included.
func TestClearRemovesOnlyTheXMLInADirectoryReport(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "target", "surefire-reports")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range map[string]string{
		"TEST-old.xml":         `<testsuite name="old"/>`,
		"com.example.Test.txt": "a human-readable dump",
		"nested/TEST-deep.xml": `<testsuite name="deep"/>`,
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	p, err := NewReportPath(root, "target/surefire-reports/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "TEST-old.xml")); !os.IsNotExist(err) {
		t.Errorf("stale TEST-old.xml survived Clear (stat err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "com.example.Test.txt")); err != nil {
		t.Errorf("Clear deleted a non-XML sibling: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nested", "TEST-deep.xml")); err != nil {
		t.Errorf("Clear descended into a nested directory Files never reads: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("Clear removed the report directory itself: %v", err)
	}
}

// A directory report_path whose directory does not exist yet must be created: a runner
// invoked with --outputFile in a directory that is not there writes nothing and the run
// fails on a report it could have had.
func TestClearCreatesAMissingReportDirectory(t *testing.T) {
	root := t.TempDir()
	p, err := NewReportPath(root, "build/test-results/test/")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	st, err := os.Stat(filepath.Join(root, "build", "test-results", "test"))
	if err != nil || !st.IsDir() {
		t.Fatalf("report directory not created by Clear: %v", err)
	}
}

// The single-file form needs its parent to exist for the same reason.
func TestClearCreatesTheParentOfASingleFileReport(t *testing.T) {
	root := t.TempDir()
	p, err := NewReportPath(root, ".rtdd/junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	st, err := os.Stat(filepath.Join(root, ".rtdd"))
	if err != nil || !st.IsDir() {
		t.Fatalf("parent directory not created by Clear: %v", err)
	}
}

// The whole point, end to end at this layer: a stale report names a test that no longer
// exists, the clear runs, the "runner" writes a fresh report, and the parsed outcome holds
// only what THIS run produced. Without the clear the stale id merges in as a pass.
func TestClearKeepsAStaleTestIDOutOfTheFreshOutcome(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		staleAt  string
		freshAt  string
	}{
		{"single file", ".rtdd/junit.xml", ".rtdd/junit.xml", ".rtdd/junit.xml"},
		// The directory case is the one a merge hides: the fresh run writes a
		// DIFFERENTLY named file, so nothing overwrites the stale one.
		{"directory", "target/surefire-reports/", "target/surefire-reports/TEST-old.xml", "target/surefire-reports/TEST-new.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p, err := NewReportPath(root, tc.declared)
			if err != nil {
				t.Fatalf("NewReportPath: %v", err)
			}
			write := func(rel, body string) {
				abs := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
					t.Fatalf("write %s: %v", rel, err)
				}
			}
			write(tc.staleAt, `<testsuite name="stale" time="0.01">
  <testcase classname="gone.Deleted" name="test_removed_last_week" time="0.01"/>
</testsuite>`)

			// The clear is what the runner does immediately before invocation.
			if err := p.Clear(); err != nil {
				t.Fatalf("Clear: %v", err)
			}
			// The subset run writes its own report.
			write(tc.freshAt, `<testsuite name="fresh" time="0.02">
  <testcase classname="live.Kept" name="test_still_here" time="0.02"/>
</testsuite>`)

			outs, err := ReadJUnitReport(p, "{classname}#{name}")
			if err != nil {
				t.Fatalf("ReadJUnitReport: %v", err)
			}
			want := []Outcome{{Test: "live.Kept#test_still_here", Status: "pass", DurationMS: 20}}
			if len(outs) != len(want) || outs[0] != want[0] {
				t.Fatalf("outcomes = %+v, want %+v (the stale id must be gone)", outs, want)
			}
			for _, o := range outs {
				if o.Test == "gone.Deleted#test_removed_last_week" {
					t.Fatalf("stale test id %q survived the clear into the parsed outcome", o.Test)
				}
			}
		})
	}
}

// A clear that cannot remove the stale report is a named error BEFORE the run starts. The
// silent continue is the dangerous branch: the run proceeds and then parses the very
// report the clear failed to delete.
func TestClearNamesAReportPathItCannotClear(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory permissions this test relies on")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "TEST-old.xml"), []byte(`<testsuite name="old"/>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Read and traverse, but no write: the entry can be listed and not unlinked.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	p, err := NewReportPathFor("maven", root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	err = p.Clear()
	if !errors.Is(err, ErrClearReportPath) {
		t.Fatalf("Clear error = %v, want errors.Is(_, ErrClearReportPath)", err)
	}
	for _, want := range []string{"maven", "TEST-old.xml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// The declaration and the disk disagreeing is named here too, before the run rather than
// after it — and in particular the clear never removes a directory a file-shaped
// declaration happens to point at.
func TestClearNamesAReportPathOfTheWrongKind(t *testing.T) {
	root := t.TempDir()
	// Declared as a file, but a directory sits there.
	if err := os.MkdirAll(filepath.Join(root, "junit.xml"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Declared as a directory, but a regular file sits there.
	if err := os.WriteFile(filepath.Join(root, "reports"), []byte("<testsuite/>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	fileDecl, err := NewReportPathFor("vitest", root, "junit.xml")
	if err != nil {
		t.Fatalf("NewReportPathFor(file): %v", err)
	}
	if err := fileDecl.Clear(); !errors.Is(err, ErrReportPathKind) {
		t.Fatalf("Clear(file declared, directory on disk) error = %v, want errors.Is(_, ErrReportPathKind)", err)
	} else if !strings.Contains(err.Error(), "vitest") {
		t.Errorf("error %q does not name the adapter", err)
	}
	if st, err := os.Stat(filepath.Join(root, "junit.xml")); err != nil || !st.IsDir() {
		t.Errorf("Clear removed a directory the adapter declared as a file: %v", err)
	}

	dirDecl, err := NewReportPathFor("gradle", root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor(dir): %v", err)
	}
	if err := dirDecl.Clear(); !errors.Is(err, ErrReportPathKind) {
		t.Fatalf("Clear(dir declared, file on disk) error = %v, want errors.Is(_, ErrReportPathKind)", err)
	} else if !strings.Contains(err.Error(), "gradle") {
		t.Errorf("error %q does not name the adapter", err)
	}
	if _, err := os.Stat(filepath.Join(root, "reports")); err != nil {
		t.Errorf("Clear removed the file sitting at a directory-shaped report_path: %v", err)
	}
}

// A report_path whose final element is a SYMLINK is refused, not followed. Clear removes
// the previous run's report, and os.Stat answers the kind question through the link, so a
// repo-relative report_path pointing at a symlinked directory would clear the link's
// TARGET — files outside the repository root that no RTDD run was ever going to read.
// Symlinked build outputs are ordinary (target -> /var/build/..., a pnpm or bazel output
// link), so the declaring adapter does not have to be malicious for this to delete a
// developer's files.
func TestClearRefusesASymlinkedDirectoryReportPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep.xml")
	if err := os.WriteFile(keep, []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write keep.xml: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("surefire", root, "link/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	err = p.Clear()
	if !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Clear(symlinked directory) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
	// The adapter's declaration is what has to change, so the error names both.
	for _, want := range []string{"surefire", "link/"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("Clear followed the symlink and deleted a file outside the repository root: %v", err)
	}
}

// The single-file form has the same hole: os.Remove would unlink the link only, but the
// os.Stat kind-check ahead of it has already been answered by the target, so a file-shaped
// declaration pointing at a symlinked DIRECTORY passes the check it was meant to fail.
func TestClearRefusesASymlinkedSingleFileReportPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "keep.xml")
	if err := os.WriteFile(target, []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write keep.xml: %v", err)
	}
	link := filepath.Join(root, "link.xml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("vitest", root, "link.xml")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	err = p.Clear()
	if !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Clear(symlinked file) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("Clear unlinked a report_path it refused: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("Clear followed the symlink and deleted its target outside the root: %v", err)
	}
}

// A symlink pointing INSIDE the repository root is refused too. Resolving and then
// checking containment would accept this one, but containment today is not containment
// tomorrow: the link is a file in the repo, and repointing it at /var or a home directory
// is one `ln -sf` away from a change no RTDD test would notice.
func TestClearRefusesASymlinkPointingInsideTheRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "build", "test-results")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(inside, filepath.Join(root, "reports")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("gradle", root, "reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	if err := p.Clear(); !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Clear(symlink inside the root) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
}

// Files refuses the same declaration, and for the same reason: what Clear will not empty
// is not a path this run may read either, and one of the two accepting it is how a report
// gets read from a directory nothing cleared.
func TestFilesRefusesASymlinkedReportPath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "TEST-other.xml"), []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("surefire", root, "link/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	if _, err := p.Files(); !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Files(symlinked directory) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
}

// An INTERMEDIATE element of a report_path is a symlink just as often as the final one:
// "build", "target" and "reports" are exactly the directories a monorepo links into a
// shared out-of-tree output area, and `report_path: "target/surefire-reports/"` is the
// declaration Surefire wants. An Lstat of the final element says nothing about its
// parents, so the walk has to cover every element the declaration names.
func TestClearRefusesASymlinkedIntermediateElement(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	precious := filepath.Join(outside, "reports", "precious.xml")
	if err := os.MkdirAll(filepath.Dir(precious), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(precious, []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write precious.xml: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "build")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("surefire", root, "build/reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	err = p.Clear()
	if !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Clear(symlinked intermediate element) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
	// The fix is in the declaration, so the error names the adapter, the declaration and
	// the element that is actually the link — "build/reports/" alone would send its
	// reader to look at the wrong one.
	for _, want := range []string{"surefire", "build/reports/", filepath.Join(root, "build")} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("Clear cleared through an intermediate symlink and deleted a file outside the repository root: %v", err)
	}
}

// The single-file shape has the same hole, and reaches it by a different route: the final
// element does not exist at all, so the Lstat that guards it says ErrNotExist and Clear
// walks on to MkdirAll the parent — through the link — and remove the file behind it.
func TestClearRefusesASymlinkedIntermediateElementForASingleFile(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	precious := filepath.Join(outside, "reports", "results.xml")
	if err := os.MkdirAll(filepath.Dir(precious), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(precious, []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write results.xml: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "build")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	p, err := NewReportPathFor("vitest", root, "build/reports/results.xml")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	if err := p.Clear(); !errors.Is(err, ErrReportPathSymlink) {
		t.Fatalf("Clear(symlinked intermediate element, file shape) error = %v, want errors.Is(_, ErrReportPathSymlink)", err)
	}
	if _, err := os.Stat(precious); err != nil {
		t.Fatalf("Clear removed a file outside the repository root through an intermediate symlink: %v", err)
	}
}

// Files refuses the intermediate link for the reason it already refuses the final one: a
// path Clear will not empty is not one this run may read either, and the two disagreeing
// is how a report gets read out of a directory nothing cleared.
func TestFilesRefusesASymlinkedIntermediateElement(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "reports", "TEST-other.xml"), []byte(`<testsuite name="not-ours"/>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "build")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	for _, tc := range []struct {
		name     string
		declared string
	}{
		{"directory", "build/reports/"},
		{"single file", "build/reports/TEST-other.xml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewReportPathFor("surefire", root, tc.declared)
			if err != nil {
				t.Fatalf("NewReportPathFor: %v", err)
			}
			if _, err := p.Files(); !errors.Is(err, ErrReportPathSymlink) {
				t.Fatalf("Files(%q) error = %v, want errors.Is(_, ErrReportPathSymlink)", tc.declared, err)
			}
		})
	}
}

// A link BELOW the repository root's own path is not the declaration's doing: a repo
// checked out under /tmp on macOS already sits behind /tmp -> /private/tmp, and walking
// past the root would refuse every report_path in it. The walk starts AT the root and
// covers only the elements the declaration itself names.
func TestClearAcceptsAReportPathWhoseRootIsReachedThroughASymlink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "build", "reports"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	root := filepath.Join(base, "repo")
	if err := os.Symlink(real, root); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	stale := filepath.Join(root, "build", "reports", "TEST-old.xml")
	if err := os.WriteFile(stale, []byte(`<testsuite name="stale"/>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	p, err := NewReportPathFor("surefire", root, "build/reports/")
	if err != nil {
		t.Fatalf("NewReportPathFor: %v", err)
	}
	if err := p.Clear(); err != nil {
		t.Fatalf("Clear(report_path under a symlinked repo root) = %v, want nil", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Clear left the previous run's report in place: %v", err)
	}
}

// Two <testcase> elements of ONE file that render the same id are not two outcomes: the
// runner's cross-chunk de-duplication would collapse them last-wins, and a fail followed
// by a pass would report green. A file-granular id_template like Vitest's {classname} or
// RSpec's {file} is deliberately shared by every case in the file, so this is the ordinary
// case rather than a malformed report — the fold is worst-status-wins, error > fail >
// skip > pass, so the id is only green when every case behind it is.
func TestReadJUnitReportFoldsCasesSharingAnIDWorstStatusWins(t *testing.T) {
	body := func(cases string) string {
		return `<testsuite name="s">` + cases + `</testsuite>`
	}
	pass := `<testcase classname="test/calc.test.js" name="%s" time="0.01"/>`
	fail := `<testcase classname="test/calc.test.js" name="%s" time="0.01"><failure message="boom"/></testcase>`
	erroR := `<testcase classname="test/calc.test.js" name="%s" time="0.01"><error message="blew up"/></testcase>`
	skip := `<testcase classname="test/calc.test.js" name="%s" time="0.01"><skipped/></testcase>`

	for _, tc := range []struct {
		name  string
		cases []string
		want  string
	}{
		{"fail then pass", []string{fail, pass}, "fail"},
		{"pass then fail", []string{pass, fail}, "fail"},
		{"error beats fail", []string{fail, erroR}, "error"},
		{"fail beats error is false", []string{erroR, fail}, "error"},
		{"fail beats skip", []string{skip, fail}, "fail"},
		{"skip beats pass", []string{pass, skip}, "skip"},
		{"all pass stays pass", []string{pass, pass}, "pass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			var xml string
			for i, c := range tc.cases {
				xml += "\n  " + strings.Replace(c, "%s", string(rune('a'+i)), 1)
			}
			if err := os.WriteFile(filepath.Join(root, "junit.xml"), []byte(body(xml)), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
			p, err := NewReportPath(root, "junit.xml")
			if err != nil {
				t.Fatalf("NewReportPath: %v", err)
			}
			outs, err := ReadJUnitReport(p, "{classname}")
			if err != nil {
				t.Fatalf("ReadJUnitReport: %v", err)
			}
			if len(outs) != 1 {
				t.Fatalf("outcomes = %+v, want the shared id folded into exactly one", outs)
			}
			if outs[0].Test != "test/calc.test.js" || outs[0].Status != tc.want {
				t.Errorf("outcome = %+v, want test/calc.test.js %s", outs[0], tc.want)
			}
		})
	}
}

// Folding two cases into one id keeps the time both of them took: a file-granular id
// stands for the whole file, so its duration is the file's, not one case's.
func TestReadJUnitReportSumsTheDurationOfFoldedCases(t *testing.T) {
	root := t.TempDir()
	body := `<testsuite name="s">
  <testcase classname="test/calc.test.js" name="a" time="0.01"><failure message="boom"/></testcase>
  <testcase classname="test/calc.test.js" name="b" time="0.03"/>
</testsuite>`
	if err := os.WriteFile(filepath.Join(root, "junit.xml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	p, err := NewReportPath(root, "junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	outs, err := ReadJUnitReport(p, "{classname}")
	if err != nil {
		t.Fatalf("ReadJUnitReport: %v", err)
	}
	want := []Outcome{{Test: "test/calc.test.js", Status: "fail", DurationMS: 40}}
	if len(outs) != 1 || outs[0] != want[0] {
		t.Fatalf("outcomes = %+v, want %+v", outs, want)
	}
}

// The fold does not reorder the report: the folded id stays where its FIRST case was, so
// a merged directory report still reads in document order.
func TestReadJUnitReportKeepsTheFirstPositionOfAFoldedID(t *testing.T) {
	root := t.TempDir()
	body := `<testsuite name="s">
  <testcase classname="test/a.test.js" name="one" time="0.01"/>
  <testcase classname="test/b.test.js" name="two" time="0.01"/>
  <testcase classname="test/a.test.js" name="three" time="0.01"><failure message="boom"/></testcase>
</testsuite>`
	if err := os.WriteFile(filepath.Join(root, "junit.xml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	p, err := NewReportPath(root, "junit.xml")
	if err != nil {
		t.Fatalf("NewReportPath: %v", err)
	}
	outs, err := ReadJUnitReport(p, "{classname}")
	if err != nil {
		t.Fatalf("ReadJUnitReport: %v", err)
	}
	want := []Outcome{
		{Test: "test/a.test.js", Status: "fail", DurationMS: 20},
		{Test: "test/b.test.js", Status: "pass", DurationMS: 10},
	}
	if len(outs) != len(want) {
		t.Fatalf("outcomes = %+v, want %+v", outs, want)
	}
	for i := range want {
		if outs[i] != want[i] {
			t.Errorf("outcome %d = %+v, want %+v", i, outs[i], want[i])
		}
	}
}
