package protocol

const (
	// A Claude Code skill is progressively disclosed: the agent loads it on
	// demand, so length costs little and completeness is worth more.
	skillMaxBytes = 20000
	// AGENTS.md is in context on every turn of every task, most of which have
	// nothing to do with tests. It must stay small enough that its presence is
	// not itself a cost.
	agentsMaxBytes = 1800
	// A Cursor rule is attached when its globs match, so it is between the two.
	mdcMaxBytes = 4000
)

const (
	SkillDescription = "Surface which tests cover the code you changed, and which changed " +
		"lines nothing covers, from recorded coverage rather than a static graph. Use when " +
		"editing a Python repository that has a .rtdd/map.jsonl, before or after changing " +
		"source files, to find the relevant tests and the untested part of a diff."
	MdcDescription = "Which tests cover the code you changed, from recorded coverage."
	MdcGlobs       = "**/*.py"
)

// Targets is the fixed set of generated front-ends. A typo in an OutPath or a
// missing Required section id would silently ship a broken front-end, so both
// are locked down here in one place.
var Targets = []Target{
	{
		Name:     "skill",
		OutPath:  "dist/SKILL.md",
		MaxBytes: skillMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "json", "commands", "limits", "map"},
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "agents",
		OutPath:  "dist/AGENTS.md",
		MaxBytes: agentsMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty"},
		Render:   renderAgents,
		Validate: validateAgents,
	},
	{
		Name:     "mdc",
		OutPath:  "dist/cursor/rules/rtdd.mdc",
		MaxBytes: mdcMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "limits"},
		Render:   renderMDC,
		Validate: validateMDC,
	},
}

func TargetByName(name string) (Target, bool) {
	for _, t := range Targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}
