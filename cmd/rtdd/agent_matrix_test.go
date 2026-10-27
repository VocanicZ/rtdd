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
		id: "codex", name: "Codex", present: []string{".codex/skills/"},
		user:   []string{".codex/skills/rtdd/SKILL.md"},
		shared: []string{".codex/AGENTS.md"}, legacy: []string{".codex/AGENTS.md"},
	},
	{
		id: "gemini", name: "Gemini CLI", present: []string{".gemini/"},
		user:   []string{".gemini/extensions/rtdd/GEMINI.md", ".gemini/extensions/rtdd/gemini-extension.json"},
		shared: []string{".gemini/GEMINI.md"}, legacy: []string{".gemini/GEMINI.md"},
	},
	{
		id: "cursor", name: "Cursor", present: []string{".cursor/"},
		project: []string{".cursor/rules/rtdd.mdc"},
		limit:   "user rules live in its settings",
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
	"claude": "verified on Claude Code 2.1.311, 2026-10-27",
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
