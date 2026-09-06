package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// go test -json writes to stdout, go-junit-report reads stdin, and the engine builds argv
// and never a shell. report_cmd is the declared join between them.
func TestReportCmdConvertsCapturedStdoutIntoTheReport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub converter is POSIX sh")
	}
	repo := t.TempDir()
	// The stub "runner" prints one line to stdout and writes no report at all.
	emit := filepath.Join(repo, "emit.sh")
	if err := os.WriteFile(emit, []byte("#!/bin/sh\necho \"case:$1\"\n"), 0o755); err != nil {
		t.Fatalf("write emit: %v", err)
	}
	// The stub "converter" turns that captured line into a JUnit document.
	conv := filepath.Join(repo, "conv.sh")
	script := "#!/bin/sh\nname=$(sed -n 's/^case://p' \"$1\")\n" +
		"printf '<testsuite name=\"s\"><testcase classname=\"s\" name=\"%s\" time=\"0.01\"/></testsuite>' \"$name\" > \"$2\"\n"
	if err := os.WriteFile(conv, []byte(script), 0o755); err != nil {
		t.Fatalf("write conv: %v", err)
	}

	a := &adapter.Adapter{
		Name:       "go",
		Detect:     []string{"go.mod"},
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Subset:     emit + " {tests}",
		ReportCmd:  "sh " + conv + " {log} {report}",
		Report:     "junit-xml",
		ReportPath: ".rtdd/junit.xml",
		IDTemplate: "{name}",
	}

	res, err := Run(a, repo, []string{"TestOne"}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Outcomes) != 1 || res.Outcomes[0].Test != "TestOne" || res.Outcomes[0].Status != "pass" {
		t.Fatalf("Outcomes = %+v, want one passing TestOne read from the converted report", res.Outcomes)
	}
}

// A converter that fails is fatal. Continuing would read a report that was never written
// — or the previous chunk's — and call an unrun suite green.
func TestReportCmdFailureIsFatalAndNamesTheAdapter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub converter is POSIX sh")
	}
	repo := t.TempDir()
	emit := filepath.Join(repo, "emit.sh")
	if err := os.WriteFile(emit, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write emit: %v", err)
	}
	bad := filepath.Join(repo, "bad.sh")
	if err := os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	a := &adapter.Adapter{
		Name: "go", Detect: []string{"go.mod"},
		Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
		Subset: emit + " {tests}", ReportCmd: "sh " + bad + " {log} {report}",
		Report: "junit-xml", ReportPath: ".rtdd/junit.xml", IDTemplate: "{name}",
	}

	_, err := Run(a, repo, []string{"TestOne"}, false)
	if err == nil {
		t.Fatal("Run = nil error, want a fatal error: the report was never produced")
	}
	if !strings.Contains(err.Error(), "go") || !strings.Contains(err.Error(), "report_cmd") {
		t.Errorf("error %q must name the adapter and report_cmd", err)
	}
}

// report_cmd runs once per CHUNK, converting that chunk's own capture. Running it once
// per Run would convert only the last chunk's output, and the per-chunk clear from #265
// would have emptied the report of every chunk before it.
func TestReportCmdRunsOncePerChunkOverThatChunksCapture(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub converter is POSIX sh")
	}
	repo := t.TempDir()
	emit := filepath.Join(repo, "emit.sh")
	if err := os.WriteFile(emit, []byte("#!/bin/sh\necho \"case:$1\"\n"), 0o755); err != nil {
		t.Fatalf("write emit: %v", err)
	}
	tally := filepath.Join(repo, "conversions")
	conv := filepath.Join(repo, "conv.sh")
	script := "#!/bin/sh\necho x >> \"" + tally + "\"\n" +
		"name=$(sed -n 's/^case://p' \"$1\")\n" +
		"printf '<testsuite name=\"s\"><testcase classname=\"s\" name=\"%s\" time=\"0.01\"/></testsuite>' \"$name\" > \"$2\"\n"
	if err := os.WriteFile(conv, []byte(script), 0o755); err != nil {
		t.Fatalf("write conv: %v", err)
	}

	a := &adapter.Adapter{
		Name: "go", Detect: []string{"go.mod"},
		Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
		Subset: emit + " {tests}", ReportCmd: "sh " + conv + " {log} {report}",
		Report: "junit-xml", ReportPath: ".rtdd/junit.xml", IDTemplate: "{name}",
	}

	// One id per chunk: MaxArgvBytes is a byte budget, so a tiny budget forces the split.
	res, err := runWithBudget(a, repo, []string{"TestOne", "TestTwo"}, 1)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	b, err := os.ReadFile(tally)
	if err != nil {
		t.Fatalf("read tally: %v", err)
	}
	if n := len(strings.Fields(string(b))); n != 2 {
		t.Errorf("report_cmd ran %d times for 2 chunks, want once per chunk", n)
	}
	got := map[string]string{}
	for _, o := range res.Outcomes {
		got[o.Test] = o.Status
	}
	if len(got) != 2 || got["TestOne"] != "pass" || got["TestTwo"] != "pass" {
		t.Errorf("Outcomes = %+v, want both chunks' converted cases; chunk 0's is lost if the conversion is not per chunk", res.Outcomes)
	}
}
