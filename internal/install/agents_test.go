package install

import (
	"strings"
	"testing"
)

// The agent table's own invariants. Each is a way a new row could quietly write somewhere
// rtdd does not own, or render nothing.
func TestAgentsTableInvariants(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, a := range Agents() {
		if a.ID == "" || a.Name == "" || seen[a.ID] {
			t.Errorf("agent %+v: empty or duplicate id/name", a)
		}
		seen[a.ID] = true
		if len(a.Present) == 0 {
			t.Errorf("%s has no Present path; rtdd could not tell whether it is installed", a.ID)
		}
		if len(a.User) == 0 && len(a.Limits) == 0 {
			t.Errorf("%s has no user surface and states no limitation", a.ID)
		}
		for _, f := range a.User {
			if a.Owned == "" || !strings.HasPrefix(f.Rel, a.Owned) || !strings.HasSuffix(a.Owned, "/rtdd/") {
				t.Errorf("%s writes %s outside an rtdd-owned directory (Owned %q)", a.ID, f.Rel, a.Owned)
			}
		}
		for _, f := range append(append([]GlobalFile{}, a.User...), a.Project...) {
			if f.Source != "" && files[f.Source] == "" {
				t.Errorf("%s: %s renders from %s, which Files() does not hold", a.ID, f.Rel, f.Source)
			}
			if f.Marker == "" {
				t.Errorf("%s: %s has no Marker, so an upgrade could not tell rtdd's render from the user's file", a.ID, f.Rel)
			}
		}
		for _, f := range a.Project {
			if !strings.Contains(f.Rel, "/rtdd/") && !strings.HasSuffix(f.Rel, "/rtdd.mdc") {
				t.Errorf("%s: project file %s is not rtdd's own file", a.ID, f.Rel)
			}
		}
	}
}

// Doctor's per-agent state, from a fake home.
func TestAgentStatusReadsTheHome(t *testing.T) {
	home := homeWith(t, ".claude")
	states, err := AgentStatus(home)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range states {
		got[s.Agent.ID] = s.State
	}
	if got["claude"] != "present" || got["codex"] != "absent" || got["cursor"] != "limited" {
		t.Errorf("AgentStatus = %v, want claude present, codex absent, cursor limited", got)
	}
	steps, _ := PlanGlobal(home, globalFiles(), false)
	if err := Apply(home, steps); err != nil {
		t.Fatal(err)
	}
	states, _ = AgentStatus(home)
	for _, s := range states {
		if s.Agent.ID == "claude" && s.State != "installed" {
			t.Errorf("after install, claude is %q, want installed", s.State)
		}
	}
}
