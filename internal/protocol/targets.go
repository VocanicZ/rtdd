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
	// GlobalSkillDescription is deliberately NOT SkillDescription. The project skill is
	// installed by `rtdd init` into a repository that has already been set up, so it can
	// assume a map and scope itself to one. The machine-wide skill is read in every
	// repository on the machine, most of which rtdd has never touched — reusing the
	// project trigger would tell the agent to stand down in exactly the repositories this
	// skill exists to bootstrap.
	GlobalSkillDescription = "Run only the tests that cover the code you changed, from " +
		"recorded coverage rather than a static graph, and surface which changed lines " +
		"nothing covers. Use in any repository before or after editing source files: if it " +
		"has a .rtdd/map.jsonl, run `rtdd which`; if it does not, run `rtdd init` to set " +
		"rtdd up for that repository first."
	MdcDescription = "Which tests cover the code you changed, from recorded coverage."
	MdcGlobs       = "**/*.py"
)

// The machine-wide front-ends. Their basenames differ from the repo-scoped ones on
// purpose: TestGolden and `rtdd-gen render --flat` both key files by
// filepath.Base(OutPath), so a shared basename would overwrite rather than fail.
const (
	GlobalSkillPath  = "dist/GLOBAL-SKILL.md"
	GlobalAgentsPath = "dist/GLOBAL-AGENTS.md"
)

// Targets is the fixed set of generated front-ends. A typo in an OutPath or a
// missing Required section id would silently ship a broken front-end, so both
// are locked down here in one place.
var Targets = []Target{
	{
		Name:     "skill",
		OutPath:  "dist/SKILL.md",
		MaxBytes: skillMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "fidelity", "json", "commands", "limits", "map"},
		Desc:     SkillDescription,
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "agents",
		OutPath:  "dist/AGENTS.md",
		MaxBytes: agentsMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "fidelity"},
		Render:   renderAgents,
		Validate: validateAgents,
	},
	{
		Name:     "mdc",
		OutPath:  "dist/cursor/rules/rtdd.mdc",
		MaxBytes: mdcMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "fidelity", "limits"},
		Desc:     MdcDescription,
		Render:   renderMDC,
		Validate: validateMDC,
	},
	{
		Name:     "global",
		OutPath:  GlobalSkillPath,
		MaxBytes: skillMaxBytes,
		Required: []string{"setup", "what", "which", "run", "uncovered", "empty", "fidelity", "json", "commands", "limits", "map"},
		Desc:     GlobalSkillDescription,
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "global-agents",
		OutPath:  GlobalAgentsPath,
		MaxBytes: agentsMaxBytes,
		Required: []string{"setup", "what", "which", "run", "uncovered", "empty", "fidelity"},
		// Its own sections, but the short `agents` bodies: this block lands in a global
		// AGENTS.md or GEMINI.md, which is in context on every turn of every task.
		BodyKeys: []string{"global-agents", "agents"},
		Render:   renderAgents,
		Validate: validateAgents,
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
