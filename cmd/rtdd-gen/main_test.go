package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

// newProtoRepo builds a minimal repo containing a real protocol/PROTOCOL.md
// (copied from this checkout) and a .git marker so findRepoRoot succeeds.
func newProtoRepo(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))
	if err != nil {
		t.Fatalf("read protocol/PROTOCOL.md: %v", err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "protocol"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "protocol", "PROTOCOL.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	// A real marker, not just an entry named `.git`: since #369 findRepoRoot resolves
	// the `gitdir:` pointer and requires HEAD behind it.
	gitDir := filepath.Join(dir, "gitdir")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: gitdir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// wantRendered parses and renders the same PROTOCOL.md rtdd-gen just used, so
// tests assert against the real renderer rather than a hand-copied fixture.
func wantRendered(t *testing.T, repoDir string) map[string]string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(repoDir, "protocol", "PROTOCOL.md"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := protocol.Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := protocol.RenderAll(d)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// rtddgen runs the CLI with dir as the working directory.
func rtddgen(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestNoArgsExitsUsage(t *testing.T) {
	dir := newProtoRepo(t)
	code, _, stderr := rtddgen(t, dir)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Fatalf("stderr = %q, want usage text", stderr)
	}
}

func TestUnknownSubcommandExitsUsage(t *testing.T) {
	dir := newProtoRepo(t)
	code, _, stderr := rtddgen(t, dir, "bogus")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Fatalf("stderr = %q, want usage text", stderr)
	}
}

func TestRenderWritesEveryTargetUnderRepoRoot(t *testing.T) {
	dir := newProtoRepo(t)
	code, _, stderr := rtddgen(t, dir, "render")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr)
	}
	want := wantRendered(t, dir)
	for _, tgt := range protocol.Targets {
		got, err := os.ReadFile(filepath.Join(dir, tgt.OutPath))
		if err != nil {
			t.Fatalf("read %s: %v", tgt.OutPath, err)
		}
		if string(got) != want[tgt.OutPath] {
			t.Errorf("%s does not match a fresh render", tgt.OutPath)
		}
	}
}

func TestRenderOutFlagWritesUnderGivenDir(t *testing.T) {
	dir := newProtoRepo(t)
	outDir := filepath.Join(dir, "elsewhere")
	code, _, stderr := rtddgen(t, dir, "render", "--out", outDir)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr)
	}
	want := wantRendered(t, dir)
	for _, tgt := range protocol.Targets {
		got, err := os.ReadFile(filepath.Join(outDir, tgt.OutPath))
		if err != nil {
			t.Fatalf("read %s: %v", tgt.OutPath, err)
		}
		if string(got) != want[tgt.OutPath] {
			t.Errorf("%s does not match a fresh render", tgt.OutPath)
		}
	}
}

func TestRenderFlatWritesBasenamesInOneDir(t *testing.T) {
	dir := newProtoRepo(t)
	outDir := filepath.Join(dir, "flat")
	code, _, stderr := rtddgen(t, dir, "render", "--out", outDir, "--flat")
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr)
	}
	want := wantRendered(t, dir)
	for _, tgt := range protocol.Targets {
		base := filepath.Base(tgt.OutPath)
		got, err := os.ReadFile(filepath.Join(outDir, base))
		if err != nil {
			t.Fatalf("read %s: %v", base, err)
		}
		if string(got) != want[tgt.OutPath] {
			t.Errorf("%s does not match a fresh render", base)
		}
	}
	// --flat must not recreate the nested cursor/rules directory.
	if _, err := os.Stat(filepath.Join(outDir, "cursor")); err == nil {
		t.Errorf("--flat created a nested directory under %s", outDir)
	}
}

func TestCheckCleanAfterRender(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	code, stdout, stderr := rtddgen(t, dir, "check")
	if code != 0 {
		t.Fatalf("check: code = %d, stdout = %s, stderr = %s", code, stdout, stderr)
	}
}

func TestCheckCatchesMutatedFile(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	agentsPath := filepath.Join(dir, "dist", "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("mutated by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "check")
	if code == 0 {
		t.Fatalf("check: code = 0, want non-zero on drift")
	}
	if !strings.Contains(stdout, "dist/AGENTS.md") {
		t.Errorf("stdout = %q, want it to name dist/AGENTS.md", stdout)
	}
}

func TestCheckCatchesMissingFile(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	if err := os.Remove(filepath.Join(dir, "dist", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "check")
	if code == 0 {
		t.Fatalf("check: code = 0, want non-zero on missing file")
	}
	if !strings.Contains(stdout, "dist/SKILL.md") {
		t.Errorf("stdout = %q, want it to name dist/SKILL.md", stdout)
	}
}

func TestVerifyCleanAfterRender(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	code, stdout, stderr := rtddgen(t, dir, "verify")
	if code != 0 {
		t.Fatalf("verify: code = %d, stdout = %s, stderr = %s", code, stdout, stderr)
	}
}

func TestVerifyCatchesStrippedFrontmatter(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	mdcPath := filepath.Join(dir, "dist", "cursor", "rules", "rtdd.mdc")
	raw, err := os.ReadFile(mdcPath)
	if err != nil {
		t.Fatal(err)
	}
	// Strip the frontmatter block, keeping the body — a drift check (byte
	// compare) would not distinguish this from a well-formed file with
	// different content; verify's job is to catch a well-formed-looking file
	// that is wrong for its target.
	parts := strings.SplitN(string(raw), "\n---\n\n", 2)
	if len(parts) != 2 {
		t.Fatalf("could not split frontmatter out of %s", mdcPath)
	}
	if err := os.WriteFile(mdcPath, []byte(parts[1]), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero on stripped frontmatter")
	}
	if !strings.Contains(stdout, "mdc") {
		t.Errorf("stdout = %q, want it to name the mdc target", stdout)
	}
}

func TestVerifyCatchesMissingFile(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	if err := os.Remove(filepath.Join(dir, "dist", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero on missing file")
	}
	if !strings.Contains(stdout, "dist/AGENTS.md") {
		t.Errorf("stdout = %q, want it to name dist/AGENTS.md", stdout)
	}
}

// TestVerifyCatchesEmptyAgentsFile reproduces, at the CLI, the first failure
// the review found: a dist/AGENTS.md holding nothing but "hello" — no markers,
// no content — exited 0. verify exists precisely to fail this.
func TestVerifyCatchesEmptyAgentsFile(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "AGENTS.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero on an AGENTS.md with no rtdd block")
	}
	if !strings.Contains(stdout, "agents") {
		t.Errorf("stdout = %q, want it to name the agents target", stdout)
	}
}

// TestVerifyCatchesOverBudgetMDC reproduces the second: the whole SKILL body
// with .mdc frontmatter grafted on, far over the .mdc budget, exited 0 because
// no validator read the budget of a file on disk.
func TestVerifyCatchesOverBudgetMDC(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	rendered := wantRendered(t, dir)
	skillBody := rendered["dist/SKILL.md"]
	frontmatter := "---\ndescription: " + protocol.MdcDescription +
		"\nglobs: " + protocol.MdcGlobs + "\nalwaysApply: false\n---\n\n"
	grafted := frontmatter + strings.SplitN(skillBody, "\n---\n\n", 2)[1]
	mdcPath := filepath.Join(dir, "dist", "cursor", "rules", "rtdd.mdc")
	if err := os.WriteFile(mdcPath, []byte(grafted), 0o644); err != nil {
		t.Fatal(err)
	}
	mdc, ok := protocol.TargetByName("mdc")
	if !ok {
		t.Fatal("mdc target not registered")
	}
	if len(grafted) <= mdc.MaxBytes {
		t.Fatalf("fixture is %d bytes, not over the %d-byte budget it must breach", len(grafted), mdc.MaxBytes)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero on an over-budget .mdc")
	}
	if !strings.Contains(stdout, "mdc") {
		t.Errorf("stdout = %q, want it to name the mdc target", stdout)
	}
}

// TestVerifyCatchesTruncatedAgentsBlock: markers intact, content gone. A byte
// comparison would call this stale; verify must call it invalid on its own.
func TestVerifyCatchesTruncatedAgentsBlock(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	truncated := protocol.BeginMarker + "\n## rtdd\n\n" + protocol.EndMarker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "dist", "AGENTS.md"), []byte(truncated), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero on a truncated AGENTS.md block")
	}
	if !strings.Contains(stdout, "agents") {
		t.Errorf("stdout = %q, want it to name the agents target", stdout)
	}
}

// TestVerifyCatchesAgentsSwallowingANonAgentsSection covers the widened
// containment check against a file on disk: a section shared by skill and mdc
// is no more welcome in AGENTS.md than a skill-only one.
func TestVerifyCatchesAgentsSwallowingANonAgentsSection(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	src, err := os.ReadFile(filepath.Join(dir, "protocol", "PROTOCOL.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := protocol.Parse(string(src))
	if err != nil {
		t.Fatal(err)
	}
	var body string
	for _, s := range doc.Sections {
		if !s.HasTarget("agents") && len(s.Targets) > 1 {
			body = s.BodyFor(s.Targets[0])
			break
		}
	}
	if body == "" {
		t.Skip("PROTOCOL.md has no multi-target non-agents section to swallow")
	}
	agentsPath := filepath.Join(dir, "dist", "AGENTS.md")
	raw, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentsPath, append(raw, []byte(body+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := rtddgen(t, dir, "verify")
	if code == 0 {
		t.Fatalf("verify: code = 0, want non-zero when AGENTS.md swallows a non-agents section")
	}
	if !strings.Contains(stdout, "agents") {
		t.Errorf("stdout = %q, want it to name the agents target", stdout)
	}
}

// TestCheckCatchesAnUnrenderedProtocolEdit is the drift direction that actually
// happens: PROTOCOL.md is edited and `rtdd-gen render` is never run, so dist/
// still carries the previous text and the shipped skill is stale. The mutated-file
// test above proves check notices a hand-edited dist/ file; this one proves it
// notices the source moving out from under an untouched one, which is what makes
// the CI step a drift gate rather than a tamper gate.
func TestCheckCatchesAnUnrenderedProtocolEdit(t *testing.T) {
	dir := newProtoRepo(t)
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render: code = %d, stderr = %s", code, stderr)
	}
	if code, stdout, stderr := rtddgen(t, dir, "check"); code != 0 {
		t.Fatalf("check before the edit: code = %d, stdout = %s, stderr = %s", code, stdout, stderr)
	}

	protoPath := filepath.Join(dir, "protocol", "PROTOCOL.md")
	src, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatal(err)
	}
	// Append a section every target carries, so all three go stale at once. An
	// edit inside an existing section would only reach the targets that do not
	// override it with a variant.
	edited := string(src) + `
<!-- rtdd:section id=drifttest title="Drift test" targets=skill,agents,mdc order=95 -->
a sentence no generated file has seen yet
<!-- rtdd:endsection -->
`
	if err := os.WriteFile(protoPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := rtddgen(t, dir, "check")
	if code == 0 {
		t.Fatalf("check: code = 0, want non-zero when PROTOCOL.md changed and dist/ was not regenerated")
	}
	for _, tgt := range protocol.Targets {
		if !strings.Contains(stdout, tgt.OutPath) {
			t.Errorf("stdout does not report %s as stale:\n%s", tgt.OutPath, stdout)
		}
	}
	if !strings.Contains(stdout, "rtdd-gen render") {
		t.Errorf("stdout does not say how to fix the drift:\n%s", stdout)
	}

	// Rendering is the fix, and check must go quiet again afterwards — otherwise
	// the gate is unpassable and gets deleted rather than obeyed.
	if code, _, stderr := rtddgen(t, dir, "render"); code != 0 {
		t.Fatalf("render after the edit: code = %d, stderr = %s", code, stderr)
	}
	if code, stdout, _ := rtddgen(t, dir, "check"); code != 0 {
		t.Fatalf("check after re-render: code = %d, stdout = %s", code, stdout)
	}
}

// Issue #369: rtdd-gen carries its own copy of findRepoRoot and had the same defect —
// any entry named `.git` was accepted as a repository root. These pin the same rule
// here so the two copies cannot drift apart again.
func TestFindRepoRootRejectsAnEmptyGitDirectory(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	inner := filepath.Join(outer, "a", "b")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	if got, err := findRepoRoot(inner); err == nil {
		t.Fatalf("findRepoRoot under an empty .git ancestor = %q, want an error", got)
	}
}

func TestFindRepoRootRejectsAGitDirectoryWithoutHEAD(t *testing.T) {
	outer := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outer, ".git", "info"), 0o755); err != nil {
		t.Fatalf("mkdir .git/info: %v", err)
	}
	if got, err := findRepoRoot(outer); err == nil {
		t.Fatalf("findRepoRoot under a HEAD-less .git ancestor = %q, want an error", got)
	}
}

func TestFindRepoRootRejectsAWorktreeFileWithADeadGitdir(t *testing.T) {
	outer := t.TempDir()
	if err := os.WriteFile(filepath.Join(outer, ".git"), []byte("gitdir: /nonexistent-rtdd-369\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	if got, err := findRepoRoot(outer); err == nil {
		t.Fatalf("findRepoRoot with a dead gitdir pointer = %q, want an error", got)
	}
}

// The live shape: `.harness/` runs the fleet out of git worktrees, where `.git` is a
// FILE pointing at the real git directory. It must still resolve.
func TestFindRepoRootAcceptsAWorktreeFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "gitdirs", "wt")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+target+"\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got, err := findRepoRoot(wt)
	if err != nil {
		t.Fatalf("findRepoRoot in a worktree: %v", err)
	}
	gotEval, _ := filepath.EvalSymlinks(got)
	wantEval, _ := filepath.EvalSymlinks(wt)
	if gotEval != wantEval {
		t.Fatalf("findRepoRoot = %q, want %q", gotEval, wantEval)
	}
}
