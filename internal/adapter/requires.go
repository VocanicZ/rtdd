package adapter

// Unmet returns the requirements whose bin does not resolve, in declaration order.
//
// lookPath is a parameter rather than a direct exec.LookPath call so the engine stays
// testable against a fixed PATH: what is installed on the machine running the tests must
// never decide whether this function is correct.
func (a *Adapter) Unmet(lookPath func(string) (string, error)) []Requirement {
	if a == nil {
		return nil
	}
	var out []Requirement
	for _, r := range a.Requires {
		if _, err := lookPath(r.Bin); err != nil {
			out = append(out, r)
		}
	}
	return out
}

// UnmetFinding pairs one unmet requirement with the adapter that declared it. Spec §4.3:
// an unmet prerequisite surfaces at doctor/init time, never as a mid-run parse failure
// against a report file that was never written.
type UnmetFinding struct {
	Adapter string
	Req     Requirement
}

// UnmetFindings collects every declared prerequisite that does not resolve on this
// machine, adapter by adapter, each adapter's own in declaration order.
//
// Callers pass the DETECTED adapters: a missing binary only some other toolchain's
// adapter wants is not a finding about this repository, and naming it sends an agent to
// install something nothing here runs.
//
// It lives here rather than beside one command's renderer because spec §4.3 requires the
// same finding at BOTH `doctor` and `init` time; a copy in each would be two chances to
// drift apart.
func UnmetFindings(adapters []*Adapter, lookPath func(string) (string, error)) []UnmetFinding {
	var out []UnmetFinding
	for _, a := range adapters {
		for _, r := range a.Unmet(lookPath) {
			out = append(out, UnmetFinding{Adapter: a.Name, Req: r})
		}
	}
	return out
}
