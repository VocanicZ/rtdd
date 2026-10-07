package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/install"
)

// newUnsupportedRepo is a real TypeScript project that no v0.2 adapter matched, so v0.2's
// init refused it. v0.3.0 serves it like any other repository.
func newUnsupportedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// firstParagraph returns the first prose paragraph of a generated front-end: the
// frontmatter, the generated-by comment and the leading headings are not prose, and an
// agent reads past them. Whatever this returns is what an agent reads first.
func firstParagraph(body string) string {
	if strings.HasPrefix(body, "---\n") {
		if i := strings.Index(body[4:], "\n---\n"); i >= 0 {
			body = body[4+i+len("\n---\n"):]
		}
	}
	for _, para := range strings.Split(body, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "<!--") {
			continue
		}
		return p
	}
	return ""
}

// A repo whose only adapter is a host YAML is a served repo. This is §4.5 reaching init.
func TestInitAcceptsAHostOnlyAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\n" +
		"unit_cmd: \"npx vitest run {unit}\"\ncoverage_file: \"{tmp}/lcov.info\"\ncoverage_format: lcov\n" +
		"test_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 with a host adapter (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if strings.Contains(string(b), "no adapter detected") {
		t.Errorf("a served repo must not be given the no-adapter caveat:\n%s", firstParagraph(string(b)))
	}
}

// The gate is about detection, not about --force: a repo an adapter serves installs the
// unmodified front-end, caveat-free, exactly as it did before this change.
func TestInitInADetectableRepoInstallsTheUnmodifiedSkill(t *testing.T) {
	dir := newDetectableRepo(t)

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	files, err := install.Files()
	if err != nil {
		t.Fatalf("install.Files: %v", err)
	}
	if string(b) != files["dist/SKILL.md"] {
		t.Errorf("the installed SKILL.md is not the generated one")
	}
}

// A config that already exists is one someone tuned. The record is never worth rewriting
// it for, so a second init leaves the file byte for byte as it found it.
func TestInitNeverRewritesAnExistingConfigToAddTheRecord(t *testing.T) {
	dir := newDetectableRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tuned := "max_stale_ratio: 0.3\n"
	cfg := filepath.Join(dir, ".rtdd", "config.yaml")
	if err := os.WriteFile(cfg, []byte(tuned), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if string(b) != tuned {
		t.Errorf(".rtdd/config.yaml = %q, want it untouched at %q", string(b), tuned)
	}
}

// PRD #410 AC5 / #452: init no longer detects adapters and no longer refuses: every git
// repository is supported (spec §8).
func TestInitSucceedsInARepositoryNoAdapterWouldHaveMatched(t *testing.T) {
	dir := newUnsupportedRepo(t)
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	for _, banned := range []string{"adapter", "--force"} {
		if strings.Contains(out+errOut, banned) {
			t.Errorf("rtdd init still talks about %q:\n%s%s", banned, out, errOut)
		}
	}
	if strings.Contains(out, ".gitattributes") {
		t.Errorf("rtdd init still writes a merge driver for a map nothing writes:\n%s", out)
	}
}
