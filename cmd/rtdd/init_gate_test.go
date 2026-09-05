package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/install"
)

// newUnsupportedRepo is the repo the defect was reported on: a real TypeScript project,
// no adapter that matches it, and — before this gate — a full RTDD install whose every
// answer would have been "run the full suite".
func newUnsupportedRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// snapshotTree records every file under dir (git's own bookkeeping excluded, since git
// rewrites it on its own schedule) with a hash of its content. Two equal snapshots mean
// the command wrote nothing at all — a stronger claim than "the skill file is absent".
func snapshotTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out = append(out, filepath.ToSlash(rel)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

func assertTreeUnchanged(t *testing.T, before, after []string) {
	t.Helper()
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Errorf("the filesystem changed:\nbefore:\n%s\nafter:\n%s",
			strings.Join(before, "\n"), strings.Join(after, "\n"))
	}
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

func TestInitRefusesARepoWithNoAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	before := snapshotTree(t, dir)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 2 {
		t.Fatalf("rtdd init = %d, want 2 (stdout: %s stderr: %s)", code, stdout, stderr)
	}
	for _, want := range []string{"no adapter detected", "package.json", "JavaScript/TypeScript", ".rtdd/adapters/", "--force"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not mention %q:\n%s", want, stderr)
		}
	}
	assertTreeUnchanged(t, before, snapshotTree(t, dir))
}

// The regression test for the reported defect, stated in its own terms: the fixture repo
// is a package.json and a .ts file, and init must leave no Claude Code skill behind.
func TestInitWritesNoSkillFileIntoATypeScriptRepo(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Commit(t, dir, "init")

	if code, _, _ := rtdd(t, dir, "init"); code != 2 {
		t.Fatalf("rtdd init = %d, want 2", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf(".claude/skills/rtdd/SKILL.md exists after init in a repo no adapter serves (err: %v)", err)
	}
}

// --dry-run gates too: printing a plan the command would refuse to execute is a lie.
func TestInitDryRunAlsoRefusesARepoWithNoAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	before := snapshotTree(t, dir)

	code, _, stderr := rtdd(t, dir, "init", "--dry-run")
	if code != 2 {
		t.Errorf("rtdd init --dry-run = %d, want 2 (stderr: %s)", code, stderr)
	}
	assertTreeUnchanged(t, before, snapshotTree(t, dir))
}

// --force overrides the gate, and then the front-end must say what it cannot do — in its
// first paragraph, where an agent reads it before it reads a selection.
func TestInitForceInstallsTheNoAdapterCaveat(t *testing.T) {
	dir := newUnsupportedRepo(t)

	if code, _, stderr := rtdd(t, dir, "init", "--force"); code != 0 {
		t.Fatalf("rtdd init --force = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "rtdd", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	first := firstParagraph(string(b))
	for _, want := range []string{"no adapter detected", ".rtdd/adapters/"} {
		if !strings.Contains(first, want) {
			t.Errorf("SKILL.md first paragraph does not carry %q:\n%s", want, first)
		}
	}
}

// A repo whose only adapter is a host YAML is a served repo. This is §4.5 reaching init.
func TestInitAcceptsAHostOnlyAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: vitest\ndetect: [\"package.json\"]\nsubset: \"npx vitest run {tests}\"\n" +
		"selection: static\ncoverage: none\nreport: junit-xml\nreport_path: \".rtdd/junit.xml\"\n" +
		"id_template: \"{file}::{name}\"\ntest_for: [\"{dir}/{name}.test.ts\"]\n" +
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

// PRD #229 AC5 at init time: a malformed host adapter names the failing FILE and the
// failing FIELD. The refusal that omits it tells the user to write the adapter they have
// already written — and both facts were computed and then thrown away.
func TestInitRefusalNamesTheMalformedHostAdapter(t *testing.T) {
	dir := newUnsupportedRepo(t)
	adir := filepath.Join(dir, ".rtdd", "adapters")
	if err := os.MkdirAll(adir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A real contract violation naming a field, not a YAML syntax error: subset has no
	// {tests} placeholder, so the "subset" would run the whole suite.
	yaml := "name: vitest\ndetect: [\"package.json\"]\nsubset: \"npx vitest run\"\n" +
		"selection: static\ncoverage: none\nreport: junit-xml\nreport_path: \".rtdd/junit.xml\"\n" +
		"id_template: \"{file}::{name}\"\ntest_for: [\"{dir}/{name}.test.ts\"]\n" +
		"test_globs: [\"**/*.test.ts\"]\nsource_globs: [\"src/**/*.ts\"]\n"
	if err := os.WriteFile(filepath.Join(adir, "vitest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	before := snapshotTree(t, dir)

	code, _, stderr := rtdd(t, dir, "init")
	if code != 2 {
		t.Fatalf("rtdd init = %d, want 2 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{".rtdd/adapters/vitest.yaml", "subset", "{tests}"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not name %q:\n%s", want, stderr)
		}
	}
	// The existing remedy text survives beside it — the file is broken, but writing an
	// adapter is still what fixes this repository.
	for _, want := range []string{"no adapter detected", ".rtdd/adapters/", "--force"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lost the remedy text %q:\n%s", want, stderr)
		}
	}
	assertTreeUnchanged(t, before, snapshotTree(t, dir))
}

// Spec §5, the ≥1-adapter branch: proceed, and record the detected adapters and their
// selection in .rtdd/config.yaml. `rtdd doctor` still derives fidelity live; this is the
// record of what the install saw.
func TestInitProceedsAndRecordsTheDetectedAdapter(t *testing.T) {
	dir := newDetectableRepo(t)

	if code, _, stderr := rtdd(t, dir, "init"); code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".rtdd", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	cfg := string(b)
	for _, want := range []string{"adapters:", "name: python", "selection: coverage", "fidelity: execution-derived"} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".rtdd/config.yaml does not record %q:\n%s", want, cfg)
		}
	}
	for _, want := range []string{"stale_commits: 50", "drift_guard: 100", "hub_threshold: 0.40"} {
		if !strings.Contains(cfg, want) {
			t.Errorf(".rtdd/config.yaml lost the default %q:\n%s", want, cfg)
		}
	}
}

// A config that already exists is one someone tuned. The record is never worth rewriting
// it for, so a second init leaves the file byte for byte as it found it.
func TestInitNeverRewritesAnExistingConfigToAddTheRecord(t *testing.T) {
	dir := newDetectableRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, ".rtdd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tuned := "stale_commits: 999\n"
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

// --force into a repo nothing matched has nothing to record, and must not claim it looked
// and found an empty set: the config it writes is the unchanged three-key default.
func TestInitForceRecordsNoAdaptersInTheConfig(t *testing.T) {
	dir := newUnsupportedRepo(t)

	if code, _, stderr := rtdd(t, dir, "init", "--force"); code != 0 {
		t.Fatalf("rtdd init --force = %d, want 0 (stderr: %s)", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".rtdd", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if strings.Contains(string(b), "adapters:") {
		t.Errorf(".rtdd/config.yaml claims a detected set in a repo nothing matched:\n%s", string(b))
	}
}
