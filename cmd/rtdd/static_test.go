package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func testAdapterNoScan() *adapter.Adapter { return &adapter.Adapter{Name: "python"} }

func TestRepoExistsAnswersRepoRelativeSlashPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "auth", "token.test.ts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	exists := repoExists(root)

	if !exists("src/auth/token.test.ts") {
		t.Error("exists(src/auth/token.test.ts) = false, want true")
	}
	if exists("src/auth/missing.test.ts") {
		t.Error("exists(src/auth/missing.test.ts) = true, want false")
	}
	// A directory is not a test file; test_for templates name files.
	if exists("src/auth") {
		t.Error("exists(src/auth) = true, want false: a directory is not a candidate")
	}
	// The engine's paths never escape the repository.
	if exists("../outside.ts") {
		t.Error("exists(../outside.ts) = true, want false")
	}
}

// An adapter with no importscan yields a nil resolver, which is how selector.Inputs
// spells "skip level 2" (PRD #230 AC7).
func TestAdapterImportDistanceIsNilWithoutAnImportscan(t *testing.T) {
	f, scanErr := adapterImportDistance(t.TempDir(), testAdapterNoScan(), nil)
	if f != nil {
		t.Error("adapterImportDistance = non-nil, want nil without an importscan")
	}
	if scanErr() != nil {
		t.Errorf("scan error = %v, want nil: an absent scanner is not a failure", scanErr())
	}
	if f, _ := adapterImportDistance(t.TempDir(), nil, nil); f != nil {
		t.Error("adapterImportDistance = non-nil for a nil adapter, want nil")
	}
}

// The declared scanner is handed the repository's OWN test files. `which` never
// enumerates the suite — that costs a collection run it does not pay — so a scanner given
// only the map's rows would be given nothing at all in the repositories the static tier
// exists for: a static adapter records no coverage, so its map is always empty.
func TestStaticTestCandidatesAreTheRepositorysTestFiles(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"src/logic.ts", "src/logic.test.ts", "src/api/gateway.test.ts",
		"node_modules/dep/dep.test.ts", ".git/hooks/hook.test.ts",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ad := &adapter.Adapter{
		Name:        "vitest",
		TestGlobs:   []string{"**/*.test.ts"},
		SourceGlobs: []string{"src/**/*.ts"},
	}

	got := staticTestCandidates(root, ad)

	want := []string{"src/api/gateway.test.ts", "src/logic.test.ts"}
	if len(got) != len(want) {
		t.Fatalf("staticTestCandidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("staticTestCandidates = %v, want %v (sorted, vendored trees excluded)", got, want)
		}
	}
}

// Walking the tree is only worth paying for when something will read the answer. A
// coverage adapter never enters the static tier, so it never enumerates anything.
func TestStaticTestCandidatesIsEmptyForANilAdapter(t *testing.T) {
	if got := staticTestCandidates(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("staticTestCandidates(nil adapter) = %v, want empty", got)
	}
}

// The end-to-end test issue #275 exists for. A `selection: static` adapter, a test_for
// template, a changed source file and a matching test file on disk must reach the user as
// tier TS with that test selected — not as a T2 whose reason merely reads differently.
//
// Every part of this worked in internal/selector for four milestones while `cmd/rtdd`
// supplied neither resolver, so Exists was nil, level 1 was skipped, and every static
// repository fell through to the full suite.
func TestWhichReachesTheStaticTierThroughDeclaredCorrespondence(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b + 1;\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")

	if code != 0 {
		t.Fatalf("rtdd which = %d, want 0 (stderr: %s)", code, stderr)
	}
	out := decodeOutput(t, stdout)
	if out.Tier != "TS" {
		t.Fatalf("tier = %q, want %q (reason: %s)", out.Tier, "TS", out.Reason)
	}
	if !containsString(out.Selection.Tests, "src/logic.test.ts") {
		t.Errorf("selection.tests = %v, want it to contain src/logic.test.ts", out.Selection.Tests)
	}
}

// `run` selects through the same resolvers as `which`. The two commands disagreeing about
// one tree is the defect this repo has already been bitten by once (see the import
// fallback in run.go), so the tier `run` announces is asserted directly.
//
// It is asserted on the tier line, which `run` prints BEFORE it executes anything: this
// fixture's adapter runs vitest, which the test environment has no reason to have.
func TestRunSelectsTheStaticTierThroughTheSameResolvers(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b + 1;\n")
	chdir(t, dir)

	out := captureStdout(t, func() { cmdRun(nil) })

	if !strings.Contains(out, "tier TS") {
		t.Errorf("rtdd run printed %q, want a TS tier: the static resolvers must reach run too", out)
	}
}

// Level 2 is wired independently of level 1: this change has NO test_for match — the
// template names src/core.test.ts, which is not on disk — and is selected purely because
// the adapter's declared importscan reports a transitive import.
//
// The stub scanner answers only when the request carries the repository's own test files,
// so this also asserts that the candidate list reaching the scanner is real.
func TestWhichReachesTheStaticTierThroughDeclaredImportscan(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "importscan:\n  command: \"bash {script}\"\n  script: \"scan-imports.sh\"\n")
	writeHostAdapter(t, dir, "scan-imports.sh", scanImportsStub)
	gittest.Write(t, dir, "src/core.ts", "export const core = () => 1;\n")
	gittest.Write(t, dir, "src/other.test.ts", "it('cores', () => {});\n")
	gittest.Commit(t, dir, "add core")
	gittest.Write(t, dir, "src/core.ts", "export const core = () => 2;\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")

	if code != 0 {
		t.Fatalf("rtdd which = %d, want 0 (stderr: %s)", code, stderr)
	}
	out := decodeOutput(t, stdout)
	if out.Tier != "TS" {
		t.Fatalf("tier = %q, want %q (reason: %s)", out.Tier, "TS", out.Reason)
	}
	if !containsString(out.Selection.Tests, "src/other.test.ts") {
		t.Errorf("selection.tests = %v, want the transitively importing test src/other.test.ts", out.Selection.Tests)
	}
	if containsString(out.Selection.Tests, "src/logic.test.ts") {
		t.Errorf("selection.tests = %v, want no unrelated test: level 2 admits importers, not neighbours", out.Selection.Tests)
	}
}

// A repository whose adapter declares no test_for and no importscan is still a full suite,
// and its reason may not claim either level was evaluated (issue #275, the reason string
// must stop lying).
func TestWhichDoesNotClaimASkippedLevelWasConsulted(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeHostAdapter(t, dir, "bare.yaml", bareStaticAdapterYAML)
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b + 1;\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")

	if code != 0 {
		t.Fatalf("rtdd which = %d, want 0 (stderr: %s)", code, stderr)
	}
	out := decodeOutput(t, stdout)
	if out.Tier != "T2" {
		t.Fatalf("tier = %q, want T2: the adapter declares neither level", out.Tier)
	}
	if strings.Contains(out.Reason, "neither declared correspondence nor imports reach") {
		t.Errorf("reason = %q: neither level was consulted, so neither may be reported as having reached nothing", out.Reason)
	}
	for _, want := range []string{"test_for", "importscan"} {
		if !strings.Contains(out.Reason, want) {
			t.Errorf("reason = %q, want it to name the missing %q declaration", out.Reason, want)
		}
	}
}

// A static adapter with neither declaration: fidelity none, so the reason names what it
// lacks rather than what it failed to find.
const bareStaticAdapterYAML = `name: bare
detect: ["package.json"]
subset: "npx vitest run {tests}"
selection: static
coverage: none
report: junit-xml
report_path: ".rtdd/junit.xml"
id_template: "{file}::{name}"
test_globs: ["**/*.test.ts"]
source_globs: ["src/**/*.ts"]
`

// scanImportsStub answers ONLY when the request carries the repository's own test files,
// so a scanner handed an empty candidate list — which is what an unwired `which` would
// hand it — reports nothing and the test fails.
const scanImportsStub = `#!/usr/bin/env bash
req="$(cat)"
case "$req" in
  *src/other.test.ts*) echo '{"src/core.ts":{"src/other.test.ts":1}}' ;;
  *) echo '{}' ;;
esac
`

// A DECLARED scanner that cannot run narrows the selection to level 1. `which` says so:
// the tier's reason only knows the resolver was supplied, so without this warning the
// output reads as though the imports had been checked and had found nothing.
func TestWhichWarnsWhenTheDeclaredImportscanFails(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "importscan:\n  command: \"bash {script}\"\n  script: \"missing-scan.sh\"\n")
	gittest.Write(t, dir, "src/core.ts", "export const core = () => 1;\n")
	gittest.Commit(t, dir, "add core")
	gittest.Write(t, dir, "src/core.ts", "export const core = () => 2;\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")

	if code != 0 {
		t.Fatalf("rtdd which = %d, want 0: a broken scanner degrades, it never fails the command (stderr: %s)", code, stderr)
	}
	out := decodeOutput(t, stdout)
	if !anyWarningContains(out.Warnings, "importscan failed") {
		t.Errorf("warnings = %v, want one naming the failed importscan", out.Warnings)
	}
	if !anyWarningContains(out.Warnings, "vitest") {
		t.Errorf("warnings = %v, want the failing adapter named", out.Warnings)
	}
}
