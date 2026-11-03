<h1 align="center">rtdd</h1>

<p align="center"><em>Which tests does this change need — in rounds, on any codebase.</em></p>

**rtdd tells a coding agent which tests a change needs, in rounds.** Running every test after
every edit is slow; running only the test named after the file you touched misses the code
that calls it. rtdd is a skill — a testing process the agent follows — and a small binary
that answers one question fast: which tests does this change need, and in what order. It
runs no tests itself.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/results/figures/how-it-picks-v3-dark.svg">
  <img alt="Two panels side by side, each holding the same graph: four functions in src/calc.py on the left of the panel (add, total, mean and clamp), four tests on the right (test_add, test_total, test_mean and test_clamp), and an edge wherever the graph records a call, including total calling add. The edit to add is highlighted as changed in both. In the left panel, running everything, every edge stays grey and all four tests run, one after another. In the right panel, RTDD, the edge from test_add into add lights and test_add is marked Round 1; then the edge from total into add lights, then the one from test_total into total, and test_total is marked Round 2. Those two run; test_mean and test_clamp are marked Round 3 only (full suite, once at the end). The panel ends on 2 of 4 tests in Rounds 1 and 2, against 4 of 4 on every edit." src="docs/results/figures/how-it-picks-v3-light.svg">
</picture>

## Features

| | Run everything | Path heuristic | rtdd |
| --- | :---: | :---: | :---: |
| Finds the tests of the code you changed | ✓ | ◐ | ✓ |
| Finds the tests of its callers and callees | ✓ | ✗ | ✓ |
| Cheap enough to run after every edit | ✗ | ✓ | ✓ |
| Works on any language, no toolchain or setup run | ✓ | ✓ | ✓ |
| Names changed code that no test reaches | ✗ | ✗ | ✓ |
| Sees calls through reflection, strings or registries | ✓ | ✗ | ✗ |
| Runs the tests itself | ✓ | ✓ | ✗ |
| Beats the path heuristic on our benchmark | — | — | ✗ |

## Install

```
npx github:VocanicZ/rtdd
```

One command on Linux, macOS and Windows. It downloads the release binary for your platform,
checks it against `checksums.txt`, puts it on your PATH and installs the agent skill.
Or build from source:

```
git clone https://github.com/VocanicZ/rtdd && cd rtdd
go build ./cmd/rtdd
```

| | needed for |
| --- | --- |
| Node 18+ | the `npx` route |
| Go 1.24+ | building from source |

The installer also writes a **machine-wide agent skill** for the agents you already have, so
each knows rtdd exists in every repository. Pass `--no-skill` or set `RTDD_NO_SKILL=1` to
install the binary alone; `rtdd skill prompt` prints a copy to hand to any other agent.
Update, uninstall and every path rtdd writes are in the [reference](docs/reference.md).

## Quick start

```
cd your-repo
rtdd init      # .rtdd/config.yaml and .rtdd/.gitignore — nothing outside .rtdd/
rtdd which     # the tests your current changes need, in rounds
```

Take `src/calc.py`, where `total` calls `add`, and `tests/test_calc.py` with `test_add` and
`test_total`. Edit `add`, and:

```
$ rtdd which
graph: scanner, built at aeb07e0, 0 stale files
changed nodes:
  src/calc.py::add  (lines 1-2)
Round 1 — run these first:
  tests/test_calc.py::test_add
Round 2 — then these:
  tests/test_calc.py::test_total
Round 3 — the full suite, once, at the end
untested:
  none
```

- **Round 1** — the tests linked to the code you changed, and every test you changed.
- **Round 2** — the tests linked to its direct neighbours, callers and callees, minus Round 1.
- **Round 3** — the full suite, once, at the end of the task.

The agent works in five steps:

1. Edit code (test first, per TDD).
2. Run `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's own test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.

rtdd runs no tests, and nothing it prints is a pass or a fail.
It is a context provider, not a gate.

## Supported agents

rtdd installs into a directory each agent reads and rtdd owns. An agent that
reads only a shared file — an `AGENTS.md`, a `GEMINI.md`, a rules pane in its settings —
gets only the explicit opt-in `rtdd init --append-to <file>`; rtdd never writes that file itself.

| agent | user level | verified |
| --- | --- | --- |
| Claude Code | `~/.claude/skills/rtdd/` | verified on Claude Code 2.1.311, 2026-10-27 |
| Antigravity | `~/.gemini/config/skills/rtdd/` | verified on agy 1.1.27, 2026-10-27 |
| Codex | `~/.agents/skills/rtdd/` | verified on codex-cli 0.154.0, 2026-10-27 |
| Gemini CLI | `~/.gemini/extensions/rtdd/` | verified on Gemini CLI 0.65.0, 2026-10-27 |
| Copilot CLI | `~/.copilot/skills/rtdd/` | verified on @github/copilot 1.0.104, 2026-10-27 |
| Cursor | `~/.cursor/skills/rtdd/` | verified on Cursor 2.4, 2026-10-27 |
| OpenCode | `~/.config/opencode/skills/rtdd/` | verified on opencode-ai 1.19.4, 2026-10-27 |
| Zcode | `~/.zcode/skills/rtdd/` | verified on ZCode 3.16.3, 2026-10-27 |
| Hermes Agent | `~/.hermes/skills/rtdd/` | verified on Hermes Agent v0.22.5, 2026-10-27 |
| OpenClaw | `~/.openclaw/skills/rtdd/` | verified on openclaw 2026.10.7, 2026-10-27 |

Limitations per agent, project-level paths and `--project-skill` are in the
[reference](docs/reference.md#supported-agents).

## Commands

```
rtdd init                          set up this repository (writes only .rtdd/)
rtdd which [--base REF] [--json]   the tests your changes need, in rounds
rtdd graph [--json]                the graph's source, node, edge and test counts, staleness
rtdd explain <file[:line]|name>    a node's tests, callers and callees
rtdd doctor                        graph source, graphify staleness, test files found
rtdd skill install|prompt|uninstall the machine-wide agent skill
rtdd update [--check]              replace the binary with the latest release
rtdd uninstall [--state]           remove what init wrote
```

Exit codes, `--base`, `--json`, [swarm](https://github.com/VocanicZ/swarm) jobs,
`RTDD_EVENT_LOG` and the optional graphify graph are in the [reference](docs/reference.md#commands-and-exit-codes).

## Does it work?

On the frozen corpus (`httpie`, `flask`), Round 1 cut the time spent in tests to 0.022 and
0.178 of running the full suite every cycle. But on this corpus **rtdd does not clearly beat
the naive path heuristic**, and by the pre-registered criterion the graph is not justified.
Two rows, one run each, nothing re-run. Round 3 is what catches what the rounds miss. The
verdicts as published and the v0.2 record are in the
[reference](docs/reference.md#does-it-work); the run is in
[n4-bench-rounds.md](docs/results/n4-bench-rounds.md).

## Limitations

- Links are by name, without a parser; a name matching more than five definitions links to none.
- A call the text does not show — reflection, a string, a registry — has no edge.
- Depth is one, and a deleted file owns no node: Round 3 is the safety net.

See [Limitations](docs/LIMITATIONS.md) and [What it cannot see](docs/reference.md#what-it-cannot-see).

## Documentation

- [Reference](docs/reference.md) — install, update and uninstall, every supported agent, what rtdd writes, exit codes, `RTDD_EVENT_LOG`, graphify, benchmark verdicts.
- [Limitations](docs/LIMITATIONS.md) — where the rounds can be wrong, and why Round 3 exists.
- [Design](docs/specs/2026-10-07-node-graph.md) — the v0.3.0 specification; earlier specs stay under [`docs/specs/`](docs/specs/).
- [Prior art](docs/PRIOR-ART.md) — pytest-testmon, TDAD and the rest of the field, with what each does better.
- [Results](docs/results/n4-bench-rounds.md) and the [baseline comparison](docs/results/axis2-selection-baselines.md), where rtdd does not win.
- [Agent protocol](protocol/PROTOCOL.md) — the single source for every front-end under `dist/`.
- [rtdd-bench](https://github.com/VocanicZ/rtdd-bench) — the full benchmark, tdd vs rtdd on real Go and Python projects.

## License

MIT. See [LICENSE](LICENSE).

## Star History

<a href="https://www.star-history.com/?repos=vocanicz%2Frtdd&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=vocanicz/rtdd&type=date&legend=top-left" />
 </picture>
</a>
