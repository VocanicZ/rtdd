package main

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/protocol"
)

// PRD #511's feature × agent matrix. A column is one agent; a row is one rtdd feature.
// Every cell runs the real command against a fake home (RTDD_HOME) and, where the feature
// needs one, a fresh git repository — no agent CLI is installed or run. An agent is listed
// in the README's "Supported agents" table only when its whole column is green
// (TestREADMESupportedAgentsAreTheMatrixColumns), so adding an entry here is the claim.

// matrixAgent is one column: what a fake home holds for the agent to count as installed,
// and every path rtdd may write for it. It is written out by hand, never derived from
// install.Agents(), so the table and the test are two statements that have to agree.
type matrixAgent struct {
	id      string   // install.Agent.ID
	name    string   // install.Agent.Name, and the README row
	present []string // home-relative; a trailing slash is a directory
	user    []string // home-relative files `rtdd skill install` writes; empty: no user level
	project []string // repo-relative files `rtdd init --project-skill` writes for it
	shared  []string // home-relative files the agent shares with its user; seeded, never written
	legacy  []string // the shared files an older `rtdd skill install` merged a block into
	limit   string   // a phrase the skip note and doctor's line carry; "" when none
}

var matrixAgents = []matrixAgent{
	{
		id: "claude", name: "Claude Code", present: []string{".claude/"},
		user:    []string{".claude/skills/rtdd/SKILL.md"},
		project: []string{".claude/skills/rtdd/SKILL.md"},
		shared:  []string{".claude/CLAUDE.md"},
	},
	{
		id: "antigravity", name: "Antigravity", present: []string{".gemini/config/"},
		user:    []string{".gemini/config/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".gemini/GEMINI.md"},
	},
	{
		id: "codex", name: "Codex", present: []string{".codex/"},
		user:    []string{".agents/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".codex/AGENTS.md"},
		legacy:  []string{".codex/AGENTS.md"},
	},
	{
		id: "gemini", name: "Gemini CLI", present: []string{".gemini/settings.json"},
		user:    []string{".gemini/extensions/rtdd/GEMINI.md", ".gemini/extensions/rtdd/gemini-extension.json"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".gemini/GEMINI.md"},
		legacy:  []string{".gemini/GEMINI.md"},
	},
	{
		id: "copilot", name: "Copilot CLI", present: []string{".copilot/"},
		user:    []string{".copilot/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".copilot/copilot-instructions.md"},
	},
	{
		id: "cursor", name: "Cursor", present: []string{".cursor/"},
		user:    []string{".cursor/skills/rtdd/SKILL.md"},
		project: []string{".cursor/rules/rtdd.mdc"},
	},
	{
		id: "opencode", name: "OpenCode", present: []string{".config/opencode/"},
		user:    []string{".config/opencode/skills/rtdd/SKILL.md"},
		project: []string{".agents/skills/rtdd/SKILL.md"},
		shared:  []string{".config/opencode/AGENTS.md"},
	},
	{
		id: "zcode", name: "Zcode", present: []string{".zcode/"},
		user: []string{".zcode/skills/rtdd/SKILL.md"},
	},
}

// matrixRows are PRD #511's "Features every agent must pass", one function each.
var matrixRows = []struct {
	name string
	run  func(t *testing.T, a matrixAgent)
}{
	{"user-install", rowUserInstall},
	{"project-opt-in", rowProjectOptIn},
	{"update-refreshes", rowUpdateRefreshes},
	{"uninstall-and-legacy", rowUninstallAndLegacy},
	{"doctor-reports", rowDoctorReports},
	{"same-instructions", rowSameInstructions},
	{"event-log", rowEventLog},
}

func TestAgentMatrix(t *testing.T) {
	for _, a := range matrixAgents {
		for _, row := range matrixRows {
			t.Run(a.id+"/"+row.name, func(t *testing.T) { row.run(t, a) })
		}
	}
}

// The matrix and the table name the same agents, in the same order.
func TestAgentMatrixColumnsAreTheAgentsTable(t *testing.T) {
	var got, want []string
	for _, a := range install.Agents() {
		got = append(got, a.ID+"="+a.Name)
	}
	for _, a := range matrixAgents {
		want = append(want, a.id+"="+a.name)
	}
	if !slices.Equal(got, want) {
		t.Errorf("install.Agents() = %v, matrixAgents = %v; a column is added to both or to neither", got, want)
	}
}

// verifiedOn is each agent's README cell, from docs/agent-capabilities.md: an agent's task
// adds its entry when its column has been checked against that page, and its Agents() row
// must say the same.
var verifiedOn = map[string]string{
	"claude":      "verified on Claude Code 2.1.311, 2026-10-27",
	"antigravity": "verified on agy 1.1.27, 2026-10-27",
	"codex":       "verified on codex-cli 0.154.0, 2026-10-27",
	"gemini":      "verified on Gemini CLI 0.65.0, 2026-10-27",
	"copilot":     "verified on @github/copilot 1.0.104, 2026-10-27",
	"cursor":      "verified on Cursor 2.4, 2026-10-27",
	"opencode":    "verified on opencode-ai 1.19.4, 2026-10-27",
	"zcode":       "verified on ZCode 3.16.3, 2026-10-27",
}

func TestEveryVerifiedColumnSaysSo(t *testing.T) {
	for id, want := range verifiedOn {
		if got := agentRow(t, id).Verified; got != want {
			t.Errorf("%s: Verified = %q, want %q", id, got, want)
		}
	}
}

// The project skill is dist/SKILL.md — the repository-scoped description, which assumes a
// set-up repository — with the opt-in marker, never the machine-wide render.
func TestClaudeCodeProjectSkillIsTheProjectRender(t *testing.T) {
	matrixHome(t, matrixAgents[0])
	dir := newTestRepo(t)
	if code, _, stderr := rtdd(t, dir, "init", "--project-skill"); code != 0 {
		t.Fatalf("rtdd init --project-skill exited %d: %s", code, stderr)
	}
	got := readRepoFileForTest(t, dir, ".claude/skills/rtdd/SKILL.md")
	if !strings.Contains(got, "description: "+protocol.SkillDescription+"\n") {
		t.Errorf("the project skill does not carry protocol.SkillDescription:\n%s", got)
	}
	if !strings.Contains(got, install.OptInMarker) {
		t.Errorf("the project skill carries no opt-in marker:\n%s", got)
	}
}

// Antigravity and Gemini CLI share ~/.gemini/. A machine with only Antigravity gets
// Antigravity's skill and nothing of Gemini CLI's.
func TestSkillInstallForAntigravityAloneWritesNoGeminiExtension(t *testing.T) {
	home := fakeHome(t, ".gemini/config")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "extensions")); !os.IsNotExist(err) {
		t.Errorf("a machine with only Antigravity got ~/.gemini/extensions/ (err %v)", err)
	}
	if got := homeFile(t, home, ".gemini/config/skills/rtdd/SKILL.md"); !strings.Contains(got, "name: rtdd") {
		t.Errorf("Antigravity's skill is not the rtdd skill:\n%s", got)
	}
}

// Gemini CLI reads the extension's context file by the manifest's contextFileName; a
// manifest naming any other file would load an extension with no instructions in it.
func TestGeminiExtensionManifestNamesItsContextFile(t *testing.T) {
	home := fakeHome(t)
	writeFile(t, home, ".gemini/settings.json", "{}\n")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	var m struct{ Name, Version, ContextFileName string }
	if err := json.Unmarshal([]byte(homeFile(t, home, ".gemini/extensions/rtdd/gemini-extension.json")), &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "rtdd" || m.ContextFileName != "GEMINI.md" || m.Version == "" {
		t.Errorf("manifest = %+v, want name rtdd, a version, contextFileName GEMINI.md", m)
	}
	homeFile(t, home, ".gemini/extensions/rtdd/"+m.ContextFileName)
}

// Cursor 2.4 reads ~/.cursor/skills/; installing writes rtdd's skill there and no rule
// file, because User Rules are not files.
func TestCursorGetsAUserLevelSkill(t *testing.T) {
	home := fakeHome(t, ".cursor")
	_, stdout, _ := rtdd(t, t.TempDir(), "skill", "install")
	if got := homeFile(t, home, ".cursor/skills/rtdd/SKILL.md"); !strings.Contains(got, "name: rtdd") {
		t.Errorf("Cursor's user skill is not the rtdd skill:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", "rules")); !os.IsNotExist(err) {
		t.Error("rtdd skill install created ~/.cursor/rules/")
	}
	if strings.Contains(stdout, "user rules live in its settings") {
		t.Errorf("the install still reports Cursor as having no user level:\n%s", stdout)
	}
}

// Copilot's instruction files are the user's; rtdd's way in is the skills directory.
// OpenCode rejects a skill whose frontmatter name differs from its directory.
func TestOpenCodeSkillNameEqualsItsDirectory(t *testing.T) {
	home := fakeHome(t, ".config/opencode")
	rtdd(t, t.TempDir(), "skill", "install")
	rel := ".config/opencode/skills/rtdd/SKILL.md"
	dirName := filepath.Base(filepath.Dir(filepath.FromSlash(rel)))
	if got := homeFile(t, home, rel); !strings.Contains(got, "\nname: "+dirName+"\n") {
		t.Errorf("%s does not declare name: %s:\n%s", rel, dirName, got)
	}
}

// Zcode's project path is undocumented, so --project-skill writes nothing on its behalf.
func TestZcodeHasNoProjectSurface(t *testing.T) {
	if p := agentRow(t, "zcode").Project; len(p) != 0 {
		t.Errorf("zcode has a project surface %v; no Z.ai document names one", p)
	}
	home := fakeHome(t, ".zcode")
	rtdd(t, t.TempDir(), "skill", "install")
	homeFile(t, home, ".zcode/skills/rtdd/SKILL.md")
}

func TestSkillInstallNeverWritesCopilotInstructions(t *testing.T) {
	home := fakeHome(t, ".copilot")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	for _, p := range []string{".copilot/copilot-instructions.md", ".copilot/instructions"} {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("rtdd skill install created %s", p)
		}
	}
	homeFile(t, home, ".copilot/skills/rtdd/SKILL.md")
}

// v0.3.0 wrote ~/.codex/skills/rtdd/. Installing now moves rtdd's render to
// ~/.agents/skills/rtdd/ and removes the old copy, so Codex never loads two — but a file
// the user put there is theirs, and it and its directory stay.
func TestSkillInstallRetiresTheOldCodexSkillDirectory(t *testing.T) {
	home := fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "rtdd")); !os.IsNotExist(err) {
		t.Errorf("the old ~/.codex/skills/rtdd/ is still there (err %v)", err)
	}
	homeFile(t, home, ".agents/skills/rtdd/SKILL.md")

	home = fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	writeFile(t, home, ".codex/skills/rtdd/notes.md", "mine\n")
	_, stdout, _ := rtdd(t, t.TempDir(), "skill", "install")
	if homeFile(t, home, ".codex/skills/rtdd/notes.md") != "mine\n" {
		t.Error("a file the user put in the old directory was not kept")
	}
	if !strings.Contains(stdout, "kept; it holds files rtdd did not write") {
		t.Errorf("the output does not say why the old directory stayed:\n%s", stdout)
	}
}

// The install that removes the old directory does not also report it as footprint for
// `rtdd uninstall --legacy-files` to remove: by the time the user reads that, it is gone.
func TestSkillInstallDoesNotReportTheDirectoryItRetiresAsLegacy(t *testing.T) {
	home := fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	_, stdout, _ := rtdd(t, t.TempDir(), "skill", "install")
	if !strings.Contains(stdout, "delete         .codex/skills/rtdd/") {
		t.Errorf("the output does not say the old directory is removed:\n%s", stdout)
	}
	if strings.Contains(stdout, "legacy") {
		t.Errorf("the output reports the directory it removes as legacy:\n%s", stdout)
	}
}

// `rtdd update` retires the old directory too, and says it removed it rather than that it
// refreshed it.
func TestRefreshGlobalSkillSaysItRemovedTheOldCodexSkill(t *testing.T) {
	home := fakeHome(t, ".codex")
	writeFile(t, home, ".codex/skills/rtdd/SKILL.md", protocol.Generated+"\nv0.3.0's skill\n")
	var out, errb strings.Builder
	refreshGlobalSkill(&out, &errb)
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "rtdd")); !os.IsNotExist(err) {
		t.Errorf("rtdd update left the old ~/.codex/skills/rtdd/ (err %v)", err)
	}
	if !strings.Contains(out.String(), "removed an older release's agent skill at .codex/skills/rtdd/") {
		t.Errorf("rtdd update's output does not say it removed the old skill:\n%s", out.String())
	}
}

// PRD #511 AC3: every agent present at once, and still only rtdd-owned paths appear.
func TestSkillInstallWithEveryAgentPresentTouchesNoUserFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("RTDD_HOME", home)
	var want []string
	for _, a := range matrixAgents {
		seedAgent(t, home, a)
		want = append(want, a.user...)
	}
	before := snapshot(t, home)
	if code, _, stderr := rtdd(t, t.TempDir(), "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	assertOnlyCreated(t, home, before, want)
}

// seedAgent makes the agent present in home and gives each of its shared files the user's
// own text.
func seedAgent(t *testing.T, home string, a matrixAgent) {
	t.Helper()
	for _, p := range a.present {
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(filepath.Join(home, filepath.FromSlash(p)), 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, home, p, "{}\n")
		}
	}
	for _, s := range a.shared {
		writeFile(t, home, s, userText)
	}
}

const userText = "# my own rules\n\nkeep me byte for byte\n"

// matrixHome is a fresh fake home holding exactly one agent, and its contents before rtdd
// ran.
func matrixHome(t *testing.T, a matrixAgent) (string, map[string]string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("RTDD_HOME", home)
	seedAgent(t, home, a)
	return home, snapshot(t, home)
}

// snapshot reads every file under dir, keyed by its slash-separated relative path.
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// assertOnlyCreated fails unless every file in before is byte-identical in dir now and the
// files that appeared are exactly want.
func assertOnlyCreated(t *testing.T, dir string, before map[string]string, want []string) {
	t.Helper()
	after := snapshot(t, dir)
	for p, b := range before {
		if after[p] != b {
			t.Errorf("%s changed; a file rtdd does not own must stay byte-identical", p)
		}
	}
	var created []string
	for p := range after {
		if _, ok := before[p]; !ok {
			created = append(created, p)
		}
	}
	slices.Sort(created)
	want = slices.Compact(slices.Sorted(slices.Values(want)))
	if !slices.Equal(created, want) {
		t.Errorf("created %v, want exactly %v", created, want)
	}
}

func agentRow(t *testing.T, id string) install.Agent {
	t.Helper()
	for _, a := range install.Agents() {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("install.Agents() has no %q", id)
	return install.Agent{}
}

func rowUserInstall(t *testing.T, a matrixAgent) {
	home, before := matrixHome(t, a)
	code, stdout, stderr := rtdd(t, t.TempDir(), "skill", "install")
	if code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	assertOnlyCreated(t, home, before, a.user)
	if a.limit != "" && !strings.Contains(stdout, a.limit) {
		t.Errorf("the output does not state %s's limitation %q:\n%s", a.name, a.limit, stdout)
	}
}

func rowProjectOptIn(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	before := map[string]string{}
	for _, f := range worktreeFiles(t, dir) {
		before[f] = readRepoFileForTest(t, dir, f)
	}
	if code, _, stderr := rtdd(t, dir, "init", "--project-skill"); code != 0 {
		t.Fatalf("rtdd init --project-skill exited %d: %s", code, stderr)
	}
	created := map[string]bool{}
	for _, f := range worktreeFiles(t, dir) {
		if _, ok := before[f]; !ok {
			created[f] = true
		}
	}
	for _, p := range a.project {
		if !created[p] {
			t.Errorf("--project-skill did not write %s for %s", p, a.name)
		}
	}
	// A path several agents share is written once whichever row asked for it, so the
	// files on disk cannot show that this agent's row names its own project surface.
	var rels []string
	for _, f := range agentRow(t, a.id).Project {
		rels = append(rels, f.Rel)
	}
	if !slices.Equal(rels, a.project) {
		t.Errorf("install.Agents() %s Project = %v, want %v", a.id, rels, a.project)
	}
	for p := range created {
		if _, ours := install.ProjectSkillFiles[p]; !ours && !strings.HasPrefix(p, ".rtdd/") {
			t.Errorf("--project-skill wrote %s, which is no agent's project surface", p)
		}
	}
}

func rowUpdateRefreshes(t *testing.T, a matrixAgent) {
	home, _ := matrixHome(t, a)
	files, err := install.Files()
	if err != nil {
		t.Fatal(err)
	}
	ag := agentRow(t, a.id)
	for _, f := range ag.User {
		writeFile(t, home, f.Rel, f.Marker+"\nan older release's text\n")
	}
	before := snapshot(t, home)
	var out, errb strings.Builder
	refreshGlobalSkill(&out, &errb)
	var want []string
	for _, f := range ag.User {
		if got := homeFile(t, home, f.Rel); got != f.Content(files) {
			t.Errorf("rtdd update left %s as an older release's render", f.Rel)
		}
		want = append(want, f.Rel)
	}
	for p, b := range before {
		if !slices.Contains(want, p) && snapshot(t, home)[p] != b {
			t.Errorf("rtdd update changed %s, which is not an rtdd file", p)
		}
	}
}

func rowUninstallAndLegacy(t *testing.T, a matrixAgent) {
	home, before := matrixHome(t, a)
	dir := t.TempDir()
	if code, _, stderr := rtdd(t, dir, "skill", "install"); code != 0 {
		t.Fatalf("rtdd skill install exited %d: %s", code, stderr)
	}
	if code, _, stderr := rtdd(t, dir, "skill", "uninstall"); code != 0 {
		t.Fatalf("rtdd skill uninstall exited %d: %s", code, stderr)
	}
	if after := snapshot(t, home); !reflect.DeepEqual(after, before) {
		t.Errorf("install then uninstall left %v, want the home exactly as it was %v", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	for _, l := range a.legacy {
		writeFile(t, home, l, "# my own rules\n\n"+protocol.BeginMarker+"\nold block\n"+protocol.EndMarker+"\n\nkeep me\n")
	}
	repo := newTestRepo(t)
	if code, _, stderr := rtdd(t, repo, "uninstall", "--legacy-files"); code != 0 {
		t.Fatalf("rtdd uninstall --legacy-files exited %d: %s", code, stderr)
	}
	for _, l := range a.legacy {
		got := homeFile(t, home, l)
		if strings.Contains(got, protocol.BeginMarker) || !strings.Contains(got, "# my own rules") || !strings.Contains(got, "keep me") {
			t.Errorf("--legacy-files left %s as %q; want the block gone and the user's text kept", l, got)
		}
	}
	for _, s := range a.shared {
		if !slices.Contains(a.legacy, s) && homeFile(t, home, s) != userText {
			t.Errorf("--legacy-files changed %s, which no rtdd ever wrote", s)
		}
	}
}

// doctorLine is the line of doctor's agents: section that names the agent.
func doctorLine(t *testing.T, out, name string) string {
	t.Helper()
	_, section, ok := strings.Cut(out, "\nagents:\n")
	if !ok {
		t.Fatalf("rtdd doctor printed no agents: section:\n%s", out)
	}
	for _, l := range strings.Split(section, "\n") {
		if strings.HasPrefix(l, "  "+name+"  ") {
			return l
		}
	}
	t.Fatalf("rtdd doctor's agents: section has no line for %s:\n%s", name, section)
	return ""
}

func rowDoctorReports(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	_, out, _ := rtdd(t, dir, "doctor")
	first := doctorLine(t, out, a.name)
	if len(a.user) == 0 {
		if !strings.Contains(first, a.limit) {
			t.Errorf("doctor's line for %s does not state its limitation %q: %q", a.name, a.limit, first)
		}
		return
	}
	if !strings.Contains(first, "present, not installed") {
		t.Errorf("before skill install, doctor's line for %s = %q, want \"present, not installed\"", a.name, first)
	}
	rtdd(t, dir, "skill", "install")
	_, out, _ = rtdd(t, dir, "doctor")
	if got := doctorLine(t, out, a.name); !strings.Contains(got, "installed  ~/"+agentRow(t, a.id).Owned) {
		t.Errorf("after skill install, doctor's line for %s = %q, want \"installed  ~/%s\"", a.name, got, agentRow(t, a.id).Owned)
	}
}

// protocolSource is read before any test changes directory.
var protocolSource, protocolErr = os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))

// processSteps are the five numbered lines of PROTOCOL.md's process section, whitespace
// collapsed: the rounds process every agent must be told, word for word.
func processSteps(t *testing.T) []string {
	t.Helper()
	if protocolErr != nil {
		t.Fatal(protocolErr)
	}
	_, body, _ := strings.Cut(string(protocolSource), "id=process ")
	body, _, _ = strings.Cut(body, "<!-- rtdd:endsection -->")
	steps := regexp.MustCompile(`(?m)^[1-5]\. .*$`).FindAllString(body, -1)
	if len(steps) != 5 {
		t.Fatalf("PROTOCOL.md's process section has %d numbered steps, want 5", len(steps))
	}
	return steps
}

func rowSameInstructions(t *testing.T, a matrixAgent) {
	home, _ := matrixHome(t, a)
	dir := newTestRepo(t)
	rtdd(t, dir, "skill", "install")
	rtdd(t, dir, "init", "--project-skill")
	flat := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	check := func(where, body string) {
		for _, step := range processSteps(t) {
			if !strings.Contains(flat(body), flat(step)) {
				t.Errorf("%s does not carry the process step %q", where, step)
			}
		}
	}
	for _, p := range a.user {
		if strings.HasSuffix(p, ".json") {
			var m map[string]any
			if err := json.Unmarshal([]byte(homeFile(t, home, p)), &m); err != nil || m["name"] != "rtdd" {
				t.Errorf("~/%s is not rtdd's manifest", p)
			}
			continue
		}
		check("~/"+p, homeFile(t, home, p))
	}
	for _, p := range a.project {
		check(p, readRepoFileForTest(t, dir, p))
	}
}

func rowEventLog(t *testing.T, a matrixAgent) {
	matrixHome(t, a)
	dir := newTestRepo(t)
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv(eventLogVar, log)
	t.Setenv("HARNESS_ISSUE", "511")
	t.Setenv("HARNESS_AGENT", a.id)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 9\n")
	if code, _, stderr := rtdd(t, dir, "which"); code != 0 {
		t.Fatalf("rtdd which exited %d: %s", code, stderr)
	}
	ev := eventLines(t, log)
	if len(ev) != 1 {
		t.Fatalf("%d events, want 1", len(ev))
	}
	// ctx holds every HARNESS_* variable, and a fleet host sets more than these two; the
	// row asserts the two it set, as event_test.go does.
	ctx, _ := ev[0]["ctx"].(map[string]any)
	if ctx["HARNESS_ISSUE"] != "511" || ctx["HARNESS_AGENT"] != a.id {
		t.Errorf("event ctx under %s = %v, want HARNESS_ISSUE=511 and HARNESS_AGENT=%s", a.name, ctx, a.id)
	}
	if ev[0]["cmd"] != "which" || ev[0]["round1"] == nil {
		t.Errorf("event under %s = %v, want a which event with round1", a.name, ev[0])
	}
}
