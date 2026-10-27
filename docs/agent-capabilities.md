# Agent capabilities

Where each agent CLI in PRD #511's scope loads skills, rules and extensions. rtdd writes its
agent instructions only into a directory it owns (#509), so the one question this page
answers for every agent is: **is there such a directory, and where?** Every cell cites its
source — an official document, or the exact local source on a machine where the tool was
installed — and the version that was checked. "Unverified" means the source does not say,
and the plan ([`docs/plans/2026-10-24-every-agent.md`](plans/2026-10-24-every-agent.md))
treats it as unknown rather than guessing.

Checked 2026-10-27. A tool's layout changes between releases; when one does, this page, the
plan's surface table and `internal/install/agents.go` change together, and the agent's
matrix column says which version it was verified on.

## The table

| agent | user level | project level | format | discovery | self-contained dir | version checked |
| --- | --- | --- | --- | --- | --- | --- |
| Claude Code | `~/.claude/skills/<name>/SKILL.md` [cc-skills] | `.claude/skills/<name>/SKILL.md`, from the start directory up to the repository root [cc-skills] | `SKILL.md`, YAML frontmatter; every field optional, `name` defaults to the directory name, `description` recommended [cc-skills] | descriptions in context at startup, body loaded on use; live reload, `/reload-skills` for a skills directory created mid-session [cc-skills] | yes — `~/.claude/skills/rtdd/` [cc-skills] | 2.1.311 [cc-local] |
| Google Antigravity (`agy`) | `~/.gemini/config/skills/<name>/SKILL.md`; global root `~/.gemini/config/` [agy-custom] | `.agents/skills/<name>/SKILL.md` (also `.agent/`, `_agents/`, `_agent/`), walking up to the repository root; rules in `GEMINI.md`, `AGENTS.md`, `.agents/rules/*.md` [agy-custom] | `SKILL.md` whose YAML frontmatter has `name` and `description` [agy-skills] | scans its roots automatically; only names and descriptions are injected; workspace beats declared beats global beats built-in [agy-custom] | yes — `~/.gemini/config/skills/rtdd/` [agy-custom] | agy 1.1.27 [agy-local] |
| OpenAI Codex CLI | `$HOME/.agents/skills/<name>/SKILL.md` (documented) [codex-skills]; the binary's own skill installer still writes `$CODEX_HOME/skills/` = `~/.codex/skills/` [codex-local] | `.agents/skills/<name>/SKILL.md`, in the current directory and each parent up to the repository root [codex-skills] | `SKILL.md` that must include `name` and `description`; optional `agents/openai.yaml` [codex-skills] | progressive disclosure: name, description and path listed (about 2 % of the context window), body read on use; changes detected automatically; `[[skills.config]] enabled=false` in `~/.codex/config.toml` disables one [codex-skills] | yes — `~/.agents/skills/rtdd/` [codex-skills] | codex-cli 0.154.0 [codex-local] |
| Gemini CLI | extensions `~/.gemini/extensions/<name>/gemini-extension.json` with a context file (`contextFileName`, default `GEMINI.md`) [gemini-ext]; skills `~/.gemini/skills/<name>/SKILL.md` and `~/.agents/skills/<name>/SKILL.md`, on by default [gemini-skills] [gemini-pkg] | skills `.gemini/skills/` and `.agents/skills/`, loaded only in a trusted folder; no workspace extensions directory [gemini-skills] [gemini-pkg] | extension: JSON manifest `name`, `version`, `description`, `contextFileName` [gemini-ext]; skill: `SKILL.md` (or `*/SKILL.md`) whose frontmatter has string `name` and `description`, else it fails to load [gemini-pkg] | extensions: every subfolder read at startup, enabled by default, no `gemini extensions install` needed, restart to pick up [gemini-ext] [gemini-pkg]; skills: scanned at session start, name and description in the system prompt, body on `activate_skill` after a confirmation [gemini-skills] | yes — `~/.gemini/extensions/rtdd/` (or `~/.gemini/skills/rtdd/`) [gemini-ext] [gemini-skills] | @google/gemini-cli 0.65.0 [gemini-pkg] |
| GitHub Copilot CLI | skills `~/.copilot/skills/<name>/SKILL.md` or `~/.agents/skills/<name>/SKILL.md` [copilot-skills]; instructions `~/.copilot/copilot-instructions.md` (a shared file) and `~/.copilot/instructions/**/*.instructions.md`; `COPILOT_HOME` replaces `~/.copilot` [copilot-instr] | skills `.github/skills/`, `.claude/skills/`, `.agents/skills/` [copilot-skills]; instructions `.github/copilot-instructions.md`, `.github/instructions/**/*.instructions.md`, `AGENTS.md`, `CLAUDE.md`, `GEMINI.md` [copilot-instr] | `SKILL.md`; `name` (lowercase, hyphens) and `description` required; optional `license`, `allowed-tools` [copilot-skills] | chosen by description, body added when picked or invoked as `/skill-name`; `/skills reload`, `copilot skill add\|enable\|disable` [copilot-skills] | yes — `~/.copilot/skills/rtdd/` [copilot-skills] | @github/copilot 1.0.104 (docs read 2026-10-27) [copilot-npm] |
| Cursor (editor and `cursor-agent`) | skills `~/.cursor/skills/` and `~/.agents/skills/`, plus `~/.claude/skills/` and `~/.codex/skills/` for compatibility [cursor-skills]; User Rules live in the app's settings, not in a file [cursor-rules] | rules `.cursor/rules/**/*.mdc`; skills `.cursor/skills/`, `.agents/skills/`, `.claude/skills/`, `.codex/skills/`; `AGENTS.md` [cursor-rules] [cursor-skills]; the CLI "supports the same rules system as the editor" [cursor-cli] | skill: `SKILL.md`, `name` (equal to its folder) and `description` required [cursor-skills]; rule: `.mdc` with `description`, `globs`, `alwaysApply` frontmatter; a plain `.md` in `.cursor/rules/` is ignored [cursor-rules] | skills found at startup, chosen by description, `/` invokes one [cursor-skills]; whether the CLI scans the user-level skill folders is unverified [cursor-cli] | yes — `~/.cursor/skills/rtdd/` for the editor; unverified for the CLI [cursor-skills] | Cursor 2.4 (docs read 2026-10-27) [cursor-skills] |
| OpenCode | `~/.config/opencode/skills/<name>/SKILL.md`; also reads `~/.claude/skills/` and `~/.agents/skills/` [opencode-skills] | `.opencode/skills/`, `.claude/skills/`, `.agents/skills/`, every directory from the working directory up to the git worktree root [opencode-skills] | `SKILL.md`; frontmatter `name` (`^[a-z0-9]+(-[a-z0-9]+)*$`, 1–64 characters, equal to the directory name) and `description` (1–1024 characters) required [opencode-skills] | the `skill` tool's description lists every name and description; the body loads on `skill({name})`; `permission.skill` in `opencode.json` gates it; no enable step [opencode-skills] | yes — `~/.config/opencode/skills/rtdd/` [opencode-skills] | opencode-ai 1.19.4 [opencode-npm] |
| Zcode (Z.ai ZCode, desktop app) | `~/.zcode/skills/<name>/SKILL.md`; or a plugin holding `.zcode-plugin/plugin.json` and `skills/<name>/SKILL.md` [zcode-skill] [zcode-plugin] | unverified: the official page says only that an import can target the current project [zcode-skill] | `SKILL.md`; `name` and `description` required, a skill missing either is ignored; a `description` over 1024 characters drops the skill [zcode-skill] | descriptions injected as summaries (≤ 250 characters), body loaded on invocation; per-skill toggle and Refresh under Settings → Skills [zcode-skill] | yes — `~/.zcode/skills/rtdd/` [zcode-skill] | ZCode desktop 3.16.3 [zcode-site] |
| Hermes Agent (Nous Research) | `~/.hermes/skills/<name>/SKILL.md`, "the primary directory and source of truth"; `skills.external_dirs` in `~/.hermes/config.yaml` adds more [hermes-skills] | `<git root>/.hermes/skills/` and `<git root>/.agents/skills/`, loaded only after `hermes skills trust` [hermes-skills] | `SKILL.md`; `name` and `description` are the core fields (the page does not say "required"); optional `version`, `platforms`, `metadata.hermes.*` [hermes-skills] | three-level progressive disclosure: `skills_list()` in the system prompt, `skill_view(name)`, `skill_view(name, path)`; a skill added mid-session is announced next turn [hermes-skills] | yes — `~/.hermes/skills/rtdd/` [hermes-skills] | Hermes Agent v0.22.5 (2026-10-23) [hermes-release] |
| OpenClaw | `~/.openclaw/skills/<name>/SKILL.md`; also `~/.agents/skills/` with the default state directory; `skills.load.extraDirs` adds more [openclaw-skills] | `<workspace>/skills/` and `<workspace>/.agents/skills/`; the workspace defaults to `~/.openclaw/workspace`, not the repository [openclaw-workspace] | `SKILL.md`; `name` and `description` required (`name` falls back to the directory name); optional `metadata.openclaw.requires` [openclaw-skills] | each root scanned up to six levels deep; snapshotted at session start and re-scanned on change; `skills.entries.<name>.enabled: false` disables one [openclaw-skills] | yes — `~/.openclaw/skills/rtdd/` [openclaw-skills] | openclaw 2026.10.7 [openclaw-npm] |

## Notes per agent

**Claude Code.** Unchanged from v0.3.0: `~/.claude/skills/rtdd/` and the opt-in
`.claude/skills/rtdd/` already are rtdd-owned directories.

**Google Antigravity.** Antigravity keeps its configuration under `~/.gemini/` — the same
directory as Gemini CLI. Its global root is `~/.gemini/config/`; the binary's changelog
records a fix for a CLI that displayed `~/.gemini/antigravity-cli/` instead [agy-local], and
the codelab's `~/.gemini/antigravity/skills/` is the older IDE path that agy 1.1.27's
documentation no longer names [agy-codelab]. Neither of those is a target. Because both
tools share `~/.gemini/`, its presence says nothing about which one is installed; the plan
gates Antigravity on `~/.gemini/config/` and Gemini CLI on its own files.

**OpenAI Codex CLI.** The documentation names `~/.agents/skills/` and does not mention
`~/.codex/skills/`, neither as supported nor as deprecated [codex-skills]; codex-cli
0.154.0's bundled skill installer still writes `$CODEX_HOME/skills/<name>` [codex-local].
rtdd follows the documentation. `~/.agents/skills/` is a cross-agent directory — OpenCode
and OpenClaw read it too — so a skill there is read by every one of them; the plan makes
that copy byte-identical to the others, so whichever one an agent loads says the same
thing.

**Gemini CLI.** v0.3.0 already installs the extension `~/.gemini/extensions/rtdd/`; a hand-placed
extension folder loads without `gemini extensions install` [gemini-pkg]. Gemini CLI now also
reads Agent Skills, including the shared `~/.agents/skills/` [gemini-skills], so on a machine
with Codex it finds rtdd's skill there as well as its extension. Its config directory
`~/.gemini/` is shared with Antigravity (above), so `~/.gemini/` existing does not mean Gemini
CLI is installed. The 0.65.0 package settles which of its files say it is: Gemini CLI writes
`~/.gemini/settings.json` (`saveSettings` at `Storage.getGlobalSettingsPath()`) and
`~/.gemini/oauth_creds.json` (`cacheCredentials` at `Storage.getOAuthCredsPath()`), and agy
1.1.27 writes neither — its settings are `~/.gemini/antigravity-cli/settings.json` and its
binary names no `oauth_creds.json` [gemini-pkg] [agy-local]. Gemini CLI also writes
`~/.gemini/installation_id` and `~/.gemini/google_accounts.json`, but agy's binary names
`installation_id` too, and both files sit in the `~/.gemini/` of a machine with both tools,
so neither gates Gemini CLI.

**GitHub Copilot CLI.** `~/.copilot/copilot-instructions.md` is a shared file and rtdd never
writes it; the skills directory is rtdd's way in. The npm package is a loader for a native
binary that is compressed, so every fact here is from docs.github.com [copilot-skills]
[copilot-instr].

**Cursor.** v0.3.0's README said Cursor has no file-based user level; Cursor 2.4 reads Agent
Skills from `~/.cursor/skills/` [cursor-skills], which gives it one. User Rules are still in
the app's settings [cursor-rules]. A forum report says `~/.agents/skills/` is not loaded,
contradicting the docs, so that path is not relied on. The CLI's documentation names its binary
`agent`; PRD #511 calls it `cursor-agent` [cursor-cli].

**OpenCode.** It reads Claude Code's and the shared `~/.agents/skills/` directories as well
as its own, so on a machine with Claude Code it can find rtdd twice. Each copy is the same
render; how OpenCode resolves two skills of one name is unverified [opencode-skills].

**Zcode.** The official tool is Z.ai's ZCode, a desktop application with no official CLI
[zcode-site]; the npm package `zcode` and the GitHub projects of that name are unrelated
third parties. Its project-level path is unverified, so the plan gives Zcode no
`--project-skill` target until a release documents one.

**Hermes Agent.** A project-level skill loads only after the user runs `hermes skills
trust`; rtdd never runs it [hermes-skills].

**OpenClaw.** Its "project" is a workspace, `~/.openclaw/workspace` by default, not the git
repository an agent works in [openclaw-workspace]; a `--project-skill` file in the repository
is read only when the repository is the workspace. `.agents/skills/` is the one project path
that is.

## Sources

[cc-skills]: https://code.claude.com/docs/en/skills
[cc-local]: local — `claude --version` printed `2.1.311 (Claude Code)`
[agy-custom]: local — agy 1.1.27's bundled `~/.gemini/antigravity-cli/builtin/skills/agy-customizations/SKILL.md`, "Workspace Customizations", "Global Configuration — Path: `~/.gemini/config/`", "Loading Priority and Precedence"
[agy-skills]: local — agy 1.1.27's bundled `agy-customizations/docs/skills.md`: a `SKILL.md` "must start with a YAML frontmatter block containing the `name` and `description` fields"
[agy-local]: local — `agy --version` printed `1.1.27`; its changelog strings record the `~/.gemini/antigravity-cli/` → `~/.gemini/config/` display fix
[agy-codelab]: https://codelabs.developers.google.com/getting-started-with-antigravity-skills
[codex-skills]: https://developers.openai.com/codex/skills
[codex-local]: local — `codex --version` printed `codex-cli 0.154.0`; its bundled skill-installer reads "Installs into `$CODEX_HOME/skills/<skill-name>` (defaults to `~/.codex/skills`)"
[gemini-ext]: https://geminicli.com/docs/extensions/reference/
[gemini-skills]: https://geminicli.com/docs/cli/skills/
[gemini-pkg]: local — the `@google/gemini-cli` 0.65.0 npm tarball, `bundle/chunk-DKZARV6U.js` (`getUserSkillsDir`, `getUserAgentSkillsDir`, `loadSkillsFromDir`) and `bundle/chunk-34OW3ELT.js` (`loadExtensions`, `_buildExtension`, `isEnabled`), `bundle/chunk-AZOL6UFL.js` (`saveSettings`) and `bundle/chunk-DKZARV6U.js` (`getGlobalSettingsPath`, `getOAuthCredsPath`, `cacheCredentials`), inspected 2026-10-27
[copilot-skills]: https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/create-skills
[copilot-instr]: https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-custom-instructions
[copilot-npm]: https://www.npmjs.com/package/@github/copilot
[cursor-skills]: https://cursor.com/docs/skills
[cursor-rules]: https://cursor.com/docs/context/rules
[cursor-cli]: https://cursor.com/docs/cli/using
[opencode-skills]: https://opencode.ai/docs/skills/
[opencode-npm]: https://www.npmjs.com/package/opencode-ai
[zcode-site]: https://zcode.z.ai
[zcode-skill]: https://zcode.z.ai/cn/docs/skill
[zcode-plugin]: https://zcode.z.ai/cn/docs/plugin
[hermes-skills]: https://hermes-agent.nousresearch.com/docs/user-guide/features/skills
[hermes-release]: https://github.com/NousResearch/hermes-agent/releases
[openclaw-skills]: https://docs.openclaw.ai/tools/skills
[openclaw-workspace]: https://docs.openclaw.ai/concepts/agent-workspace
[openclaw-npm]: https://www.npmjs.com/package/openclaw
