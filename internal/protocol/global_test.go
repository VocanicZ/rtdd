package protocol

import (
	"strings"
	"testing"
)

// renderReal parses the committed protocol/PROTOCOL.md and renders every target.
func renderReal(t *testing.T) map[string]string {
	t.Helper()
	src, err := readProtocolMD(t)
	if err != nil {
		t.Fatalf("read PROTOCOL.md: %v", err)
	}
	d, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := RenderAll(d)
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	return out
}

// The global front-ends are what an agent reads in a repository rtdd has never touched, so
// their job is the opposite of the project skill's. The project skill assumes a map already
// exists; these have to say what to do when none does. A global front-end that omitted the
// setup section would send an agent to `rtdd which` in a repo with no .rtdd/map.jsonl —
// the exact dead end it exists to prevent.
func TestGlobalFrontEndsTeachInit(t *testing.T) {
	out := renderReal(t)
	for _, path := range []string{GlobalSkillPath, GlobalAgentsPath} {
		body, ok := out[path]
		if !ok {
			t.Fatalf("RenderAll produced no %s", path)
		}
		if !strings.Contains(body, "rtdd init") {
			t.Errorf("%s never mentions `rtdd init`, so an agent in an un-inited repo is told nothing", path)
		}
	}
}

// The project skill's description scopes itself to "a Python repository that has a
// .rtdd/map.jsonl". Copying that verbatim into a machine-wide skill would tell the agent to
// stand down in precisely the repositories the global skill exists to bootstrap, so the two
// descriptions must differ.
func TestGlobalSkillDescriptionDoesNotRequireAMap(t *testing.T) {
	if GlobalSkillDescription == SkillDescription {
		t.Fatal("the global skill reuses the project skill's description; it must not require an existing map")
	}
	if !strings.Contains(GlobalSkillDescription, "rtdd init") {
		t.Error("GlobalSkillDescription does not name `rtdd init`, so the skill will not fire on an un-inited repo")
	}
}

// Both TestGolden and `rtdd-gen render --flat` key files by filepath.Base(OutPath). Two
// targets sharing a basename would silently overwrite each other in both places rather
// than fail, so uniqueness is a property of the target table itself.
func TestTargetBasenamesAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, tgt := range Targets {
		base := tgt.OutPath[strings.LastIndex(tgt.OutPath, "/")+1:]
		if prev, dup := seen[base]; dup {
			t.Errorf("targets %q and %q share basename %q; --flat and TestGolden would overwrite one with the other",
				prev, tgt.OutPath, base)
		}
		seen[base] = tgt.OutPath
	}
}
