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
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: nowhere\n"), 0o644); err != nil {
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
