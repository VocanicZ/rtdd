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
	SkillDescription = "Find the tests a code change needs, in rounds: `rtdd which` names the " +
		"changed functions, Round 1 (their tests) and Round 2 (their neighbours' tests), and " +
		"runs nothing. Use in a repository that has a .rtdd/config.yaml, after editing code " +
		"and before running its tests."
	// GlobalSkillDescription is deliberately NOT SkillDescription. The project skill is
	// installed by `rtdd init` into a repository that has already been set up, so it can
	// scope itself to one. The machine-wide skill is read in every repository on the
	// machine, most of which rtdd has never touched — reusing the project trigger would
	// tell the agent to stand down in exactly the repositories this skill exists to set up.
	GlobalSkillDescription = "Find the tests a code change needs, in rounds, on any codebase: " +
		"`rtdd which` names Round 1 (the changed code's tests) and Round 2 (its neighbours' " +
		"tests) and runs nothing. Use in any git repository after editing code: if it has a " +
		".rtdd/config.yaml, run `rtdd which`; if it does not, run `rtdd init` once to set " +
		"rtdd up for that repository first."
	MdcDescription = "Which tests a code change needs, in rounds; rtdd runs no tests."
	// MdcGlobs attaches the rule to every file: rtdd serves any codebase, and a list of
	// extensions would be a list of languages.
	MdcGlobs = "**/*"
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
		Required: []string{"what", "process", "which", "empty", "graphify", "json", "commands", "limits"},
		Desc:     SkillDescription,
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "agents",
		OutPath:  "dist/AGENTS.md",
		MaxBytes: agentsMaxBytes,
		Required: []string{"what", "process", "which", "empty", "graphify"},
		Render:   renderAgents,
		Validate: validateAgents,
	},
	{
		Name:     "mdc",
		OutPath:  "dist/cursor/rules/rtdd.mdc",
		MaxBytes: mdcMaxBytes,
		Required: []string{"what", "process", "which", "empty", "graphify", "limits"},
		Desc:     MdcDescription,
		Render:   renderMDC,
		Validate: validateMDC,
	},
	{
		Name:     "global",
		OutPath:  GlobalSkillPath,
		MaxBytes: skillMaxBytes,
		Required: []string{"setup", "what", "process", "which", "empty", "graphify", "json", "commands", "limits"},
		Desc:     GlobalSkillDescription,
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "global-agents",
		OutPath:  GlobalAgentsPath,
		MaxBytes: agentsMaxBytes,
		Required: []string{"setup", "what", "process", "which", "empty", "graphify"},
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
