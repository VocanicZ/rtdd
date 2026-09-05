package adapter

// Fidelity is how a selection was derived (spec §6). The string values are the wire
// format for --json and must not be reworded.
type Fidelity string

const (
	FidelityExecution Fidelity = "execution-derived"
	FidelityStatic    Fidelity = "static"
	FidelityNone      Fidelity = "none"
)

// Fidelity reports the best selection this adapter can ever produce.
//
// It is DERIVED from selection and coverage rather than declared beside them, so a repo
// can never be told it has execution-derived selection by an adapter that says it records
// nothing. It is a property of the declaration, not of the repository's state: an unseeded
// Python repo is still execution-derived — it just has no map yet, which is a tier question
// the selector answers.
//
// A static adapter with neither a test_for template nor an importscan command is `none`,
// not `static`. All it could offer is path proximity, which is exactly the `path` baseline
// spec §7 pre-registers the static tier against; reporting that as `static` would tell an
// agent it had a narrowed suite when it has the full one.
func (a *Adapter) Fidelity() Fidelity {
	if a == nil {
		return FidelityNone
	}
	if a.Coverage != CoverageNone && a.Selection != SelectionStatic {
		return FidelityExecution
	}
	if len(a.TestFor) > 0 || a.Importscan != nil {
		return FidelityStatic
	}
	return FidelityNone
}
