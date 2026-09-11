package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/selfupdate"
)

// TestUpdateFlagsBecomeOptions covers the wiring this command is: flags in, Options out.
func TestUpdateFlagsBecomeOptions(t *testing.T) {
	opts, err := parseUpdateFlags(nil)
	if err != nil {
		t.Fatalf("no flags: %v", err)
	}
	if opts.CheckOnly || opts.Tag != "" {
		t.Errorf("bare `rtdd update` produced %+v, want a plain latest-release update", opts)
	}

	opts, err = parseUpdateFlags([]string{"--check"})
	if err != nil {
		t.Fatalf("--check: %v", err)
	}
	if !opts.CheckOnly {
		t.Error("--check did not set CheckOnly")
	}

	opts, err = parseUpdateFlags([]string{"--version", "v0.1.0"})
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if opts.Tag != "v0.1.0" {
		t.Errorf("Tag = %q, want v0.1.0", opts.Tag)
	}
}

func TestUpdateRejectsUnknownFlags(t *testing.T) {
	if _, err := parseUpdateFlags([]string{"--yolo"}); err == nil {
		t.Fatal("parseUpdateFlags accepted --yolo")
	}
}

// TestRenderUpdateSaysWhichOfTheThreeOutcomesHappened - the command prints one of exactly
// three things, and a user must be able to tell "already current" from "I replaced it"
// without reading the exit code.
func TestRenderUpdateSaysWhichOfTheThreeOutcomesHappened(t *testing.T) {
	replaced := renderUpdate(selfupdate.Result{Current: "0.1.1", Latest: "v0.1.2", Target: "/usr/local/bin/rtdd", Outdated: true, Updated: true})
	if !strings.Contains(replaced, "v0.1.2") || !strings.Contains(replaced, "/usr/local/bin/rtdd") {
		t.Errorf("an applied update does not name the version and the path it wrote: %q", replaced)
	}

	current := renderUpdate(selfupdate.Result{Current: "0.1.2", Latest: "v0.1.2", Target: "/usr/local/bin/rtdd"})
	if !strings.Contains(current, "0.1.2") || strings.Contains(current, "/usr/local/bin/rtdd") {
		t.Errorf("an up-to-date report should name the version and no written path: %q", current)
	}

	available := renderUpdate(selfupdate.Result{Current: "0.1.1", Latest: "v0.1.2", Target: "/usr/local/bin/rtdd", Outdated: true})
	if !strings.Contains(available, "v0.1.2") || !strings.Contains(available, "rtdd update") {
		t.Errorf("--check should name the newer release and how to get it: %q", available)
	}
}

// withVersion pins the version this binary reports for one test. Tests must never read
// whatever ldflags stamped in, or releasing would change their results.
func withVersion(t *testing.T, v string) {
	t.Helper()
	saved := version
	version = v
	t.Cleanup(func() { version = saved })
}

// fakeLatest serves the releases API and nothing else, which is all --check reaches.
func fakeLatest(t *testing.T, tag string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name": %q}`, tag)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestUpdateCheckExitsZeroWhenANewerReleaseExists is the exit-code claim in the usage
// block: rtdd exits non-zero when a test failed and for no other reason. An available
// update is a fact, not a failure.
func TestUpdateCheckExitsZeroWhenANewerReleaseExists(t *testing.T) {
	dir := newTestRepo(t)
	t.Setenv("RTDD_API_URL", fakeLatest(t, "v9999.0.0"))
	withVersion(t, "0.1.1")

	code, stdout, stderr := rtdd(t, dir, "update", "--check")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "v9999.0.0") {
		t.Errorf("stdout = %q, want it to name the newer release", stdout)
	}
}

// TestUpdateFromASourceBuildIsAConfigurationError - exit 2 is "usage or configuration
// error" in the usage block, which is what an unorderable version is.
func TestUpdateFromASourceBuildIsAConfigurationError(t *testing.T) {
	dir := newTestRepo(t)
	t.Setenv("RTDD_API_URL", fakeLatest(t, "v9999.0.0"))
	withVersion(t, "dev")

	code, _, stderr := rtdd(t, dir, "update")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stderr, "--version") {
		t.Errorf("stderr = %q, want it to point at the flag that resolves this", stderr)
	}
}

func TestUsageListsUpdate(t *testing.T) {
	if !strings.Contains(usage, "rtdd update") {
		t.Error("the usage block does not list `rtdd update`")
	}
}
