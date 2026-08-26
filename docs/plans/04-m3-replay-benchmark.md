# RTDD M3 — Real-Commit Replay Benchmark

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `bench/replay/`, a Python harness that replays the last N real commits of every pre-registered corpus repo, asks seven selection strategies which tests they would run at each commit, scores them against what the full suite actually did, and emits a per-repo, config-stamped, diffable results table.

**Architecture:** A `Corpus` frozen by digest gates which repos may be touched at all; `gitwork` builds a detached worktree at each commit's *parent* and materialises the child commit's tree into it as uncommitted working-tree changes (so `--base HEAD` sees exactly what an agent mid-cycle sees); `runner` executes pytest in three modes (full uninstrumented, full instrumented, subset) behind a content-addressed `cache` keyed by repo+commit+strategy+config-digest; each of the seven strategies is a module behind one `Strategy` protocol returning a `Selection`; `metrics` scores selections against ground truth without ever pooling across repos, and `report` writes `bench/results/<repo_id>/` as sorted JSONL plus a markdown table that embeds the full config and hardware fingerprint.

**Tech Stack:** Python 3.12+, uv, pytest, pytest-testmon, pytest-xdist, pytest-cov, pytest-reportlog, coverage, PyYAML, plain `git` subprocess (no GitPython — the harness must run against corpus repos with pinned, unusual git layouts and stdlib subprocess is the fewest moving parts).

## Global Constraints

- The harness lives in `bench/`, is a standalone uv project, and is **not** part of the Go module — `bench/` is listed in no `go.mod` and no Go file imports it.
- **Pre-registered corpus.** `bench/corpus.yaml` is frozen: its sha256 is pinned in `bench/corpus.lock`, and every entry point verifies the digest before doing any work.
- **The harness refuses to run on a repo that is not in the frozen list.** `UnknownRepoError`, not a warning.
- `corpus.yaml` carries the **mechanical selection criteria** and an **`excluded:` table with a reason per attempted-and-rejected repo**. An empty `excluded:` list is itself a lock failure — exclusions are where cherry-picking hides.
- **Never pool across repos.** Every aggregate function takes records from exactly one `repo_id` and asserts it; cross-repo numbers exist only as explicitly duration-weighted aggregates, printed in their own table and labelled as such.
- **Shipped defaults only.** The harness never passes tuning flags to `rtdd`; it runs the binary as a user would. The full effective config is printed into every results table.
- **Every cycle is counted, including escalations.** A strategy that escalates to the full suite still produces a record; escalation rate is published as its own number per strategy.
- **Wall-clock only from disclosed hardware.** The harness records the CPU model, core count, RAM and OS it ran on, prints them in the results table, and **refuses to emit wall-clock numbers when it detects a CI environment** (`CIWallClockRefused`); the rest of the metrics still compute.
- **No result is published without the config that produced it.** Every results directory contains `config.json` (corpus digest, tool versions, strategy set, hardware, git SHA of the rtdd binary under test) and every cache key is salted with that config's digest, so a config change invalidates cached results rather than silently mixing them.
- **Pre-registered decision criterion, recorded before results exist:** *if RTDD does not clearly beat the naive `tests/test_<module>.py` path heuristic on change-level recall at equal or better selected-duration fraction, the map is unjustified and the honest outcome is to say so in the README rather than ship it.* The harness prints this comparison as its own line in the summary table.
- Recall is only ever computed over commits where ground truth is non-empty; a change-level detection **requires `F_sel ∩ F_full ≠ ∅`**, so an unrelated flaky failure can never score as a detection.
- Finding A8 of the design audit is binding: **no mutation harness, no `|F_sel| > 0` detection rule, no baseline-free comparison.** If a task in this plan seems to recreate any of the three, stop and re-read the audit.

---

## Replay protocol

Two commit populations. Both come from real history. **They are never pooled** and are printed in separate tables.

**Population `natural` (primary).** For each replay commit `C` with parent `P`:

1. Detached worktree at `P`. Seed the strategy's state there (`rtdd seed`, `pytest --testmon`, a full run to populate `.pytest_cache`), all cached by `(repo, P)`.
2. Materialise `C`'s tree into that worktree **without moving HEAD**: for each path in `git diff --no-renames --name-status P C`, write `git show C:<path>` (or delete it). HEAD stays `P`, so added files are genuinely untracked and renames arrive as delete+add — exactly the mid-cycle state spec §5 defines the changed set over, and exactly the class of change audit A8 says mutation testing cannot produce.
3. Each strategy answers with a `Selection` computed **from this working tree**, `--base HEAD`.
4. Ground truth: run the full suite here. `F_full` = tests failing in this tree **minus** tests already failing in the clean `P` tree. Pre-existing failures are excluded, so a repo with a permanently broken test does not manufacture recall.
5. Commits with `F_full == ∅` are the **green population**: they contribute selection ratio, selected-duration fraction, wall-clock, false-signal and escalation, but not recall. Commits with `F_full ≠ ∅` are the **detecting population** and are the only source of recall.

**Population `probe` (secondary, labelled, never pooled with `natural`).** Real commits are usually committed green, so the natural detecting population is small. For each `C` that touches at least one source file *and* at least one test file: worktree at `C`, then revert **only the source-file half** of `C`'s diff back to `P`. The tree is `C`'s tests over `P`'s source; HEAD is `C`; the changed set is the source files with their hunks reversed. `F_full` = tests failing here that pass in the clean `C` tree. The edit is a real developer's real multi-file edit, merely applied in reverse — it is not a mutant.

`probe`'s map is seeded at `C`, so it is **an upper bound on selection quality** for every map-based strategy. The results table says this on the same line as the numbers. It is still a fair *comparative* measurement: every strategy gets the same advantage, and the path heuristic, import graph, `--lf` and random get no benefit from it at all.

**Outcome independence.** `F_sel` is computed analytically as `F_full ∩ selected` rather than by running each strategy's subset at every commit, which would multiply the corpus cost by seven. That assumes a test's outcome does not depend on which other tests ran with it. The harness **validates the assumption instead of asserting it**: on the wall-clock sample it really runs the subset, compares the observed failing set against `F_full ∩ selected`, and publishes `isolation_violations` as its own number. A non-zero count is a finding, not a crash.

---

## File Structure

| File | Single responsibility |
|---|---|
| `bench/pyproject.toml` | uv project definition; pins pytest, pytest-testmon, pytest-xdist, pytest-cov, pytest-reportlog, coverage, PyYAML |
| `bench/uv.lock` | committed lockfile — the harness's own dependency versions are part of the published config |
| `bench/.gitignore` | ignores `work/` (worktrees, clones) and `cache/`; `results/` is **not** ignored |
| `bench/corpus.yaml` | the frozen pre-registered corpus: criteria, repo list with pins, and the attempted-and-excluded table |
| `bench/corpus.lock` | one line: sha256 of `corpus.yaml`'s bytes |
| `bench/replay/__init__.py` | package marker; exports `__version__` used in the config digest |
| `bench/replay/config.py` | `RunConfig` — every knob, tool version and digest that must appear beside a published number |
| `bench/replay/corpus.py` | load + digest-verify `corpus.yaml`; `Corpus.require(repo_id)` is the whitelist guard |
| `bench/replay/hardware.py` | CPU/RAM/OS probe, CI detection, and the wall-clock refusal |
| `bench/replay/gitwork.py` | clone at pin, worktrees, commit enumeration, diff parsing, tree materialisation for both variants |
| `bench/replay/cache.py` | content-addressed JSON and directory cache, keyed by repo+commit+strategy+config digest |
| `bench/replay/runner.py` | pytest invocation (full / subset / instrumented), report-log parsing, wall-clock capture |
| `bench/replay/covread.py` | reads a corpus repo's `.coverage` SQLite into per-test and import-time line sets (ground truth for false-signal) |
| `bench/replay/rtddio.py` | the only place that shells out to the `rtdd` binary and knows its `--json` schema |
| `bench/replay/strategies/base.py` | `Selection`, `CommitContext`, the `Strategy` protocol, and the registry |
| `bench/replay/strategies/full.py` | baseline 7 — the full suite / ground truth |
| `bench/replay/strategies/pathheuristic.py` | baseline 2 — `tests/test_<module>.py` for `src/<module>.py` |
| `bench/replay/strategies/importgraph.py` | baseline 4 — AST static import graph, reverse reachability |
| `bench/replay/strategies/lastfailed.py` | baseline 3 — `pytest --lf` |
| `bench/replay/strategies/testmon.py` | baseline 1 — pytest-testmon method-level checksums |
| `bench/replay/strategies/xdist.py` | baseline 5 — `pytest -n auto`; selects everything, differs only in execution |
| `bench/replay/strategies/randomratio.py` | baseline 6 — uniform sample at RTDD's selection size, seeded per commit |
| `bench/replay/strategies/rtdd.py` | the system under test, via `rtddio` |
| `bench/replay/records.py` | the four record dataclasses that flow from orchestrator to metrics to report |
| `bench/replay/metrics.py` | change-level and test-level recall, strata, selection ratio, duration fraction, escalation rate |
| `bench/replay/falsesignal.py` | uncovered-report false-signal rate, line-level and change-level |
| `bench/replay/session.py` | selection ratio as a function of cycles-since-commit (spec §5 drift) |
| `bench/replay/replay.py` | the orchestrator — walks commits, drives strategies, produces records |
| `bench/replay/report.py` | writes `bench/results/<repo_id>/{commits.jsonl,summary.json,summary.md,config.json}` |
| `bench/replay/cli.py` | `replay`, `session`, `report`, `doctor` subcommands; enforces the global guards at the entry point |
| `bench/tests/synthrepo.py` | builds a synthetic git repo with a known commit history and hand-computed test outcomes |
| `bench/tests/conftest.py` | pytest fixtures wrapping `synthrepo` and a temp cache |
| `bench/tests/test_*.py` | one test module per `replay/` module |
| `bench/results/<repo_id>/commits.jsonl` | one sorted line per `(commit, variant, strategy)` — diffable |
| `bench/results/<repo_id>/summary.json` | metrics, sorted keys, newline-terminated — diffable |
| `bench/results/<repo_id>/summary.md` | the published per-repo table |
| `bench/results/<repo_id>/config.json` | the config and hardware that produced the two files above |

---
