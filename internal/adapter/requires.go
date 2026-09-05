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
