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
