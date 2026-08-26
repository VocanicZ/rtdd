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

## Task 1 — Bench project skeleton and the config digest

**Files:** `bench/pyproject.toml`, `bench/.gitignore`, `bench/replay/__init__.py`, `bench/replay/config.py`, `bench/tests/test_config.py`

**Interfaces:**

```python
# bench/replay/config.py
@dataclasses.dataclass(frozen=True)
class RunConfig:
    corpus_digest: str
    rtdd_version: str
    tool_versions: tuple[tuple[str, str], ...]   # sorted (name, version) pairs
    strategies: tuple[str, ...]                  # sorted strategy ids
    variants: tuple[str, ...]                    # ("natural", "probe")
    replay_commits: int
    wallclock_sample: int
    random_seed: int
    harness_version: str

    def to_dict(self) -> dict: ...
    def digest(self) -> str: ...                 # sha256 of canonical JSON

def canonical_json(obj: object) -> str: ...      # sorted keys, no spaces, newline-terminated
def tool_versions(env: dict[str, str] | None = None) -> tuple[tuple[str, str], ...]: ...
```

- [ ] **1.1 Failing test for canonical JSON stability.**

```python
# bench/tests/test_config.py
from replay.config import canonical_json


def test_canonical_json_is_key_order_independent_and_newline_terminated():
    a = canonical_json({"b": 1, "a": [3, 2]})
    b = canonical_json({"a": [3, 2], "b": 1})
    assert a == b
    assert a == '{"a":[3,2],"b":1}\n'
```

- [ ] **1.2 Run it and watch it fail.** `cd bench && uv run pytest tests/test_config.py -q` → `ModuleNotFoundError: No module named 'replay'`.

- [ ] **1.3 Create the project.**

```toml
# bench/pyproject.toml
[project]
name = "rtdd-bench"
version = "0.1.0"
description = "RTDD Axis 2 — real-commit replay benchmark harness"
requires-python = ">=3.12"
dependencies = [
    "pyyaml>=6.0.2",
    "pytest>=8.3",
    "pytest-reportlog>=0.4.0",
    "pytest-cov>=5.0.0",
    "pytest-testmon>=2.1.1",
    "pytest-xdist>=3.6.1",
    "coverage>=7.6.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["replay"]

[tool.pytest.ini_options]
testpaths = ["tests"]
addopts = "-p no:randomly"
```

```gitignore
# bench/.gitignore
work/
cache/
.venv/
__pycache__/
```

```python
# bench/replay/__init__.py
__version__ = "0.1.0"
```

- [ ] **1.4 Minimal implementation.**

```python
# bench/replay/config.py
from __future__ import annotations

import dataclasses
import hashlib
import importlib.metadata
import json

import replay

TRACKED_TOOLS = ("pytest", "pytest-testmon", "pytest-xdist", "pytest-cov", "pytest-reportlog", "coverage")


def canonical_json(obj: object) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":")) + "\n"


def tool_versions(env: dict[str, str] | None = None) -> tuple[tuple[str, str], ...]:
    out: list[tuple[str, str]] = []
    for name in TRACKED_TOOLS:
        try:
            out.append((name, importlib.metadata.version(name)))
        except importlib.metadata.PackageNotFoundError:
            out.append((name, "absent"))
    return tuple(sorted(out))


@dataclasses.dataclass(frozen=True)
class RunConfig:
    corpus_digest: str
    rtdd_version: str
    tool_versions: tuple[tuple[str, str], ...]
    strategies: tuple[str, ...]
    variants: tuple[str, ...]
    replay_commits: int
    wallclock_sample: int
    random_seed: int
    harness_version: str = replay.__version__

    def to_dict(self) -> dict:
        return {
            "corpus_digest": self.corpus_digest,
            "rtdd_version": self.rtdd_version,
            "tool_versions": {k: v for k, v in self.tool_versions},
            "strategies": list(self.strategies),
            "variants": list(self.variants),
            "replay_commits": self.replay_commits,
            "wallclock_sample": self.wallclock_sample,
            "random_seed": self.random_seed,
            "harness_version": self.harness_version,
        }

    def digest(self) -> str:
        return hashlib.sha256(canonical_json(self.to_dict()).encode("utf-8")).hexdigest()
```

- [ ] **1.5 Run and pass.** `cd bench && uv sync && uv run pytest tests/test_config.py -q` → 1 passed.

- [ ] **1.6 Failing test for the digest contract.**

```python
# append to bench/tests/test_config.py
from replay.config import RunConfig


def _cfg(**over):
    base = dict(
        corpus_digest="abc",
        rtdd_version="0.3.0",
        tool_versions=(("pytest", "8.3.3"),),
        strategies=("full", "rtdd"),
        variants=("natural",),
        replay_commits=200,
        wallclock_sample=20,
        random_seed=7,
    )
    base.update(over)
    return RunConfig(**base)


def test_digest_changes_when_any_published_knob_changes():
    assert _cfg().digest() == _cfg().digest()
    assert _cfg().digest() != _cfg(replay_commits=201).digest()
    assert _cfg().digest() != _cfg(rtdd_version="0.3.1").digest()
    assert _cfg().digest() != _cfg(strategies=("full",)).digest()
```

- [ ] **1.7 Run and pass** (the implementation above already satisfies it): `cd bench && uv run pytest tests/test_config.py -q` → 2 passed.

- [ ] **1.8 Commit.** `git add bench/pyproject.toml bench/uv.lock bench/.gitignore bench/replay bench/tests && git commit -m "bench: uv project skeleton and RunConfig digest"`

---

## Task 2 — Frozen corpus and the repo whitelist guard

**Files:** `bench/corpus.yaml`, `bench/corpus.lock`, `bench/replay/corpus.py`, `bench/tests/test_corpus.py`

**Interfaces:**

```python
# bench/replay/corpus.py
class CorpusError(RuntimeError): ...
class UnknownRepoError(CorpusError): ...
class CorpusNotFrozenError(CorpusError): ...

@dataclasses.dataclass(frozen=True)
class RepoSpec:
    id: str
    url: str
    pin: str                       # commit sha the corpus was frozen at
    replay_commits: int
    python: str
    install: tuple[str, ...]       # shell-free argv lists, joined by the runner
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]

@dataclasses.dataclass(frozen=True)
class Corpus:
    frozen_at: str
    criteria: tuple[str, ...]
    repos: dict[str, RepoSpec]
    excluded: tuple[dict, ...]
    digest: str

    def require(self, repo_id: str) -> RepoSpec: ...   # raises UnknownRepoError
    def ids(self) -> tuple[str, ...]: ...

def load_corpus(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> Corpus: ...
def freeze(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> str: ...
```

- [ ] **2.1 Failing tests for the two guards.**

```python
# bench/tests/test_corpus.py
import hashlib
import pathlib

import pytest

from replay.corpus import (
    Corpus,
    CorpusNotFrozenError,
    UnknownRepoError,
    load_corpus,
)

YAML = """\
frozen_at: "2026-08-26"
criteria:
  - "pure-Python, pytest-based, no compiled extension in the import path"
  - "suite completes under 10 minutes uninstrumented on the disclosed hardware"
repos:
  - id: demo
    url: https://example.invalid/demo.git
    pin: 1111111111111111111111111111111111111111
    replay_commits: 50
    python: "3.12"
    install: ["uv pip install -e ."]
    source_globs: ["src/**/*.py"]
    test_globs: ["tests/**/*.py"]
excluded:
  - url: https://example.invalid/other.git
    reason: "requires a live PostgreSQL; suite cannot run hermetically"
"""


def _write(tmp_path: pathlib.Path, text: str = YAML, lock: str | None = None):
    y = tmp_path / "corpus.yaml"
    y.write_text(text, encoding="utf-8")
    lk = tmp_path / "corpus.lock"
    digest = hashlib.sha256(y.read_bytes()).hexdigest()
    lk.write_text((lock if lock is not None else digest) + "\n", encoding="utf-8")
    return y, lk


def test_loads_when_lock_matches(tmp_path):
    y, lk = _write(tmp_path)
    c = load_corpus(y, lk)
    assert c.ids() == ("demo",)
    assert c.repos["demo"].replay_commits == 50
    assert c.excluded[0]["reason"].startswith("requires a live PostgreSQL")


def test_refuses_when_lock_does_not_match(tmp_path):
    y, lk = _write(tmp_path, lock="0" * 64)
    with pytest.raises(CorpusNotFrozenError):
        load_corpus(y, lk)


def test_refuses_a_repo_not_in_the_frozen_list(tmp_path):
    y, lk = _write(tmp_path)
    c = load_corpus(y, lk)
    with pytest.raises(UnknownRepoError):
        c.require("some-repo-i-liked-the-look-of")


def test_refuses_an_empty_excluded_table(tmp_path):
    y, lk = _write(tmp_path, text=YAML.split("excluded:")[0] + "excluded: []\n")
    with pytest.raises(CorpusNotFrozenError):
        load_corpus(y, lk)
```

- [ ] **2.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_corpus.py -q` → `ModuleNotFoundError: No module named 'replay.corpus'`.

- [ ] **2.3 Implement.**

```python
# bench/replay/corpus.py
from __future__ import annotations

import dataclasses
import hashlib
import pathlib

import yaml


class CorpusError(RuntimeError):
    pass


class UnknownRepoError(CorpusError):
    pass


class CorpusNotFrozenError(CorpusError):
    pass


@dataclasses.dataclass(frozen=True)
class RepoSpec:
    id: str
    url: str
    pin: str
    replay_commits: int
    python: str
    install: tuple[str, ...]
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]


@dataclasses.dataclass(frozen=True)
class Corpus:
    frozen_at: str
    criteria: tuple[str, ...]
    repos: dict[str, RepoSpec]
    excluded: tuple[dict, ...]
    digest: str

    def require(self, repo_id: str) -> RepoSpec:
        try:
            return self.repos[repo_id]
        except KeyError:
            raise UnknownRepoError(
                f"{repo_id!r} is not in the frozen corpus "
                f"({', '.join(sorted(self.repos))}). Add it to bench/corpus.yaml and "
                f"re-freeze before results exist, never after."
            ) from None

    def ids(self) -> tuple[str, ...]:
        return tuple(sorted(self.repos))


def _digest(yaml_path: pathlib.Path) -> str:
    return hashlib.sha256(yaml_path.read_bytes()).hexdigest()


def freeze(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> str:
    d = _digest(yaml_path)
    lock_path.write_text(d + "\n", encoding="utf-8")
    return d


def load_corpus(yaml_path: pathlib.Path, lock_path: pathlib.Path) -> Corpus:
    if not lock_path.exists():
        raise CorpusNotFrozenError(f"{lock_path} is missing; the corpus must be frozen before use")
    actual = _digest(yaml_path)
    expected = lock_path.read_text(encoding="utf-8").strip()
    if actual != expected:
        raise CorpusNotFrozenError(
            f"{yaml_path} has changed since it was frozen "
            f"(lock={expected[:12]}, actual={actual[:12]}). Re-freezing invalidates every "
            f"published result derived from the old corpus."
        )
    raw = yaml.safe_load(yaml_path.read_text(encoding="utf-8")) or {}
    excluded = tuple(raw.get("excluded") or ())
    if not excluded:
        raise CorpusNotFrozenError(
            "corpus.yaml has an empty `excluded:` table. Every attempted-and-rejected repo "
            "must be listed with a reason; exclusions are where cherry-picking hides."
        )
    repos: dict[str, RepoSpec] = {}
    for r in raw.get("repos") or ():
        spec = RepoSpec(
            id=r["id"],
            url=r["url"],
            pin=r["pin"],
            replay_commits=int(r["replay_commits"]),
            python=str(r["python"]),
            install=tuple(r.get("install") or ()),
            source_globs=tuple(r.get("source_globs") or ("**/*.py",)),
            test_globs=tuple(r.get("test_globs") or ("tests/**/*.py", "**/test_*.py", "**/*_test.py")),
        )
        if spec.id in repos:
            raise CorpusError(f"duplicate repo id {spec.id!r}")
        repos[spec.id] = spec
    if not repos:
        raise CorpusError("corpus.yaml lists no repos")
    return Corpus(
        frozen_at=str(raw.get("frozen_at", "")),
        criteria=tuple(raw.get("criteria") or ()),
        repos=repos,
        excluded=excluded,
        digest=actual,
    )
```

- [ ] **2.4 Run and pass.** `cd bench && uv run pytest tests/test_corpus.py -q` → 4 passed.

- [ ] **2.5 Write the real pre-registered corpus.** Criteria are mechanical so a reader can re-derive the list; `excluded:` records every repo tried and dropped.

```yaml
# bench/corpus.yaml
frozen_at: "2026-08-26"
criteria:
  - "Pure Python; no compiled extension on the import path (so coverage tracing is complete)."
  - "pytest is the repo's own test runner, invoked with no non-default plugins required."
  - "At least 400 collected tests, so selection ratio is not dominated by rounding."
  - "Full suite completes in under 10 minutes uninstrumented on the disclosed hardware."
  - "At least 200 commits in the last 12 months touching a source file, so replay has material."
  - "Hermetic: no network, no database, no container required for a green run."
  - "OSI-licensed and publicly clonable at a fixed commit."
repos:
  - id: httpie
    url: https://github.com/httpie/cli.git
    pin: REPLACE_WITH_SHA_AT_FREEZE_TIME
    replay_commits: 200
    python: "3.12"
    install:
      - "uv pip install -e ."
      - "uv pip install -r requirements-dev.txt"
    source_globs: ["httpie/**/*.py"]
    test_globs: ["tests/**/*.py"]
  - id: flask
    url: https://github.com/pallets/flask.git
    pin: REPLACE_WITH_SHA_AT_FREEZE_TIME
    replay_commits: 200
    python: "3.12"
    install:
      - "uv pip install -e ."
      - "uv pip install -r requirements/tests.txt"
    source_globs: ["src/flask/**/*.py"]
    test_globs: ["tests/**/*.py"]
  - id: sqlfluff
    url: https://github.com/sqlfluff/sqlfluff.git
    pin: REPLACE_WITH_SHA_AT_FREEZE_TIME
    replay_commits: 200
    python: "3.12"
    install:
      - "uv pip install -e ."
      - "uv pip install -r requirements_dev.txt"
    source_globs: ["src/sqlfluff/**/*.py"]
    test_globs: ["test/**/*.py"]
excluded:
  - url: https://github.com/psf/requests.git
    reason: "Test suite requires a live httpbin server; not hermetic."
  - url: https://github.com/numpy/numpy.git
    reason: "Compiled C extensions dominate the import path; coverage cannot attribute them."
  - url: https://github.com/django/django.git
    reason: "Runs under its own runtests.py harness, not plain pytest; adapter is out of v1 scope."
  - url: https://github.com/pandas-dev/pandas.git
    reason: "Full suite exceeds the 10-minute uninstrumented budget by more than an order of magnitude."
  - url: https://github.com/encode/httpx.git
    reason: "Under 400 collected tests at the pin; selection ratio would be rounding-dominated."
  - url: https://github.com/scrapy/scrapy.git
    reason: "Twisted reactor tests are order-dependent; the outcome-independence assumption is known-false here."
```

- [ ] **2.6 Freeze it and add a test that the shipped corpus itself loads.**

```python
# append to bench/tests/test_corpus.py
REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]


def test_shipped_corpus_is_frozen_and_loadable():
    c = load_corpus(REPO_ROOT / "bench" / "corpus.yaml", REPO_ROOT / "bench" / "corpus.lock")
    assert c.ids()
    assert c.excluded
    assert all(len(s.pin) == 40 for s in c.repos.values()), "every pin must be a full sha"
```

Freeze with `cd bench && uv run python -c "import pathlib; from replay.corpus import freeze; print(freeze(pathlib.Path('corpus.yaml'), pathlib.Path('corpus.lock')))"`. The `REPLACE_WITH_SHA_AT_FREEZE_TIME` placeholders must be resolved to real 40-character SHAs (`git ls-remote <url> HEAD`) **before** freezing — the test above fails otherwise, which is the point.

- [ ] **2.7 Run and pass.** `cd bench && uv run pytest tests/test_corpus.py -q` → 5 passed.

- [ ] **2.8 Commit.** `git add bench/corpus.yaml bench/corpus.lock bench/replay/corpus.py bench/tests/test_corpus.py && git commit -m "bench: pre-registered frozen corpus with whitelist guard"`

---

## Task 3 — Hardware probe and the CI wall-clock refusal

**Files:** `bench/replay/hardware.py`, `bench/tests/test_hardware.py`

**Interfaces:**

```python
# bench/replay/hardware.py
CI_ENV_VARS: tuple[str, ...]

class CIWallClockRefused(RuntimeError): ...

@dataclasses.dataclass(frozen=True)
class Hardware:
    cpu_model: str
    cpu_count: int
    mem_total_kb: int
    platform: str
    python_version: str
    ci: str | None            # the env var that gave it away, or None

    def to_dict(self) -> dict: ...
    def fingerprint(self) -> str: ...
    def wallclock_allowed(self) -> bool: ...

def detect_ci(env: Mapping[str, str]) -> str | None: ...
def probe(env: Mapping[str, str] | None = None) -> Hardware: ...
def require_wallclock(hw: Hardware) -> None: ...   # raises CIWallClockRefused
```

- [ ] **3.1 Failing tests.**

```python
# bench/tests/test_hardware.py
import pytest

from replay.hardware import CIWallClockRefused, Hardware, detect_ci, probe, require_wallclock


def test_detects_common_ci_environments():
    assert detect_ci({"GITHUB_ACTIONS": "true"}) == "GITHUB_ACTIONS"
    assert detect_ci({"CI": "1"}) == "CI"
    assert detect_ci({"BUILDKITE": "true"}) == "BUILDKITE"
    assert detect_ci({"CI": "false"}) is None
    assert detect_ci({"HOME": "/home/x"}) is None


def test_probe_records_the_machine_and_flags_ci():
    hw = probe({"GITHUB_ACTIONS": "true"})
    assert hw.ci == "GITHUB_ACTIONS"
    assert hw.cpu_count >= 1
    assert hw.python_version
    assert not hw.wallclock_allowed()


def test_require_wallclock_refuses_on_ci_and_allows_on_a_disclosed_machine():
    ci = probe({"GITHUB_ACTIONS": "true"})
    with pytest.raises(CIWallClockRefused):
        require_wallclock(ci)
    local = probe({})
    assert local.ci is None
    require_wallclock(local)


def test_fingerprint_is_stable_and_excludes_ci_flag():
    a = Hardware("Xeon", 8, 1024, "Linux-6", "3.12.4", None)
    b = Hardware("Xeon", 8, 1024, "Linux-6", "3.12.4", "CI")
    assert a.fingerprint() == b.fingerprint()
```

- [ ] **3.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_hardware.py -q` → `ModuleNotFoundError: No module named 'replay.hardware'`.

- [ ] **3.3 Implement.**

```python
# bench/replay/hardware.py
from __future__ import annotations

import dataclasses
import hashlib
import os
import pathlib
import platform
import sys
from collections.abc import Mapping

CI_ENV_VARS = (
    "CI",
    "GITHUB_ACTIONS",
    "GITLAB_CI",
    "BUILDKITE",
    "CIRCLECI",
    "JENKINS_URL",
    "TRAVIS",
    "TEAMCITY_VERSION",
    "TF_BUILD",
    "CODEBUILD_BUILD_ID",
    "DRONE",
    "APPVEYOR",
)

_FALSEY = {"", "0", "false", "no", "off"}


class CIWallClockRefused(RuntimeError):
    pass


def detect_ci(env: Mapping[str, str]) -> str | None:
    for name in CI_ENV_VARS:
        val = env.get(name)
        if val is not None and val.strip().lower() not in _FALSEY:
            return name
    return None


def _cpu_model() -> str:
    p = pathlib.Path("/proc/cpuinfo")
    if p.exists():
        for line in p.read_text(encoding="utf-8", errors="replace").splitlines():
            if line.lower().startswith("model name"):
                return line.split(":", 1)[1].strip()
    return platform.processor() or platform.machine() or "unknown"


def _mem_total_kb() -> int:
    p = pathlib.Path("/proc/meminfo")
    if p.exists():
        for line in p.read_text(encoding="utf-8", errors="replace").splitlines():
            if line.startswith("MemTotal:"):
                return int(line.split()[1])
    return 0


@dataclasses.dataclass(frozen=True)
class Hardware:
    cpu_model: str
    cpu_count: int
    mem_total_kb: int
    platform: str
    python_version: str
    ci: str | None

    def to_dict(self) -> dict:
        return {
            "cpu_model": self.cpu_model,
            "cpu_count": self.cpu_count,
            "mem_total_kb": self.mem_total_kb,
            "platform": self.platform,
            "python_version": self.python_version,
            "ci": self.ci,
            "fingerprint": self.fingerprint(),
        }

    def fingerprint(self) -> str:
        raw = f"{self.cpu_model}|{self.cpu_count}|{self.mem_total_kb}|{self.platform}|{self.python_version}"
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()[:16]

    def wallclock_allowed(self) -> bool:
        return self.ci is None


def probe(env: Mapping[str, str] | None = None) -> Hardware:
    e = os.environ if env is None else env
    return Hardware(
        cpu_model=_cpu_model(),
        cpu_count=os.cpu_count() or 1,
        mem_total_kb=_mem_total_kb(),
        platform=platform.platform(),
        python_version=sys.version.split()[0],
        ci=detect_ci(e),
    )


def require_wallclock(hw: Hardware) -> None:
    if not hw.wallclock_allowed():
        raise CIWallClockRefused(
            f"wall-clock measurement refused: {hw.ci} is set. Spec §10 permits wall-clock "
            f"numbers only from disclosed hardware, never from CI runners. Re-run on a "
            f"machine you can name, or pass --no-wallclock to compute the other metrics."
        )
```

- [ ] **3.4 Run and pass.** `cd bench && uv run pytest tests/test_hardware.py -q` → 4 passed. (If the harness itself is being developed inside CI, `test_require_wallclock_...`'s `probe({})` branch is unaffected — it passes an explicit empty env.)

- [ ] **3.5 Commit.** `git add bench/replay/hardware.py bench/tests/test_hardware.py && git commit -m "bench: hardware fingerprint and CI wall-clock refusal"`

---

## Task 4 — Synthetic git repo fixture with hand-computed outcomes

Every later task is verified against this repo, so its history and its test outcomes are
written down here and must not drift.

**Files:** `bench/tests/synthrepo.py`, `bench/tests/conftest.py`, `bench/tests/test_synthrepo.py`

**The history (four commits, oldest first):**

| Commit | Change | Full-suite outcome | `F_full` vs parent |
|---|---|---|---|
| `c0` | root: `src/alpha.py`, `src/beta.py`, `tests/test_alpha.py`, `tests/test_beta.py` | all pass | — |
| `c1` | `src/alpha.py`: `a + b` → `a + b + 1` | `test_add` fails | `{tests/test_alpha.py::test_add}` |
| `c2` | revert alpha; **add** `src/gamma.py` + `tests/test_gamma.py` | all pass | `∅` (green commit) |
| `c3` | `src/beta.py`: `a * b` → `a * b + 1`; harmless edit to `src/gamma.py` | `test_mul` fails | `{tests/test_beta.py::test_mul}` |

**Hand-computed expectations, map/state seeded at the parent:**

| Commit | changed set | RTDD T0 | path heuristic | import graph | `--lf` | change recall (all four) |
|---|---|---|---|---|---|---|
| `c1` | `src/alpha.py` | `{test_add}` | `{test_add}` | `{test_add}` | full suite (no prior failures) | 1/1 |
| `c2` | `src/alpha.py`, `src/gamma.py`, `tests/test_gamma.py` | `{test_add}` + direct `{test_gamma}` | `{test_add, test_gamma}` | `{test_add, test_gamma}` | `{test_add}` (failed at `c1`) | green commit, no recall |
| `c3` | `src/beta.py`, `src/gamma.py` | `{test_mul, test_gamma}` | `{test_mul, test_gamma}` | `{test_mul, test_gamma}` | full suite | 1/1 |

**Interfaces:**

```python
# bench/tests/synthrepo.py
@dataclasses.dataclass(frozen=True)
class SynthRepo:
    path: pathlib.Path
    commits: tuple[str, ...]        # oldest-first, len == 4

    def sha(self, index: int) -> str: ...

def build_synth_repo(root: pathlib.Path) -> SynthRepo: ...
```

- [ ] **4.1 Failing test.**

```python
# bench/tests/test_synthrepo.py
import subprocess

from tests.synthrepo import build_synth_repo


def test_history_shape_and_real_outcomes(tmp_path):
    repo = build_synth_repo(tmp_path / "synth")
    assert len(repo.commits) == 4

    def outcome_at(sha):
        subprocess.run(["git", "checkout", "-q", sha], cwd=repo.path, check=True)
        proc = subprocess.run(
            ["python", "-m", "pytest", "-q", "--no-header", "-p", "no:cacheprovider"],
            cwd=repo.path,
            capture_output=True,
            text=True,
        )
        return proc.returncode

    assert outcome_at(repo.sha(0)) == 0, "c0 is green"
    assert outcome_at(repo.sha(1)) == 1, "c1 breaks test_add"
    assert outcome_at(repo.sha(2)) == 0, "c2 is green"
    assert outcome_at(repo.sha(3)) == 1, "c3 breaks test_mul"
```

- [ ] **4.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_synthrepo.py -q` → `ModuleNotFoundError: No module named 'tests.synthrepo'`.

- [ ] **4.3 Implement.**

```python
# bench/tests/synthrepo.py
from __future__ import annotations

import dataclasses
import pathlib
import subprocess

ENV_OVERRIDES = {
    "GIT_AUTHOR_NAME": "bench",
    "GIT_AUTHOR_EMAIL": "bench@example.invalid",
    "GIT_COMMITTER_NAME": "bench",
    "GIT_COMMITTER_EMAIL": "bench@example.invalid",
    "GIT_AUTHOR_DATE": "2026-01-01T00:00:00+00:00",
    "GIT_COMMITTER_DATE": "2026-01-01T00:00:00+00:00",
}


@dataclasses.dataclass(frozen=True)
class SynthRepo:
    path: pathlib.Path
    commits: tuple[str, ...]

    def sha(self, index: int) -> str:
        return self.commits[index]


def _git(path: pathlib.Path, *args: str) -> str:
    import os

    env = dict(os.environ)
    env.update(ENV_OVERRIDES)
    proc = subprocess.run(
        ["git", *args], cwd=path, env=env, capture_output=True, text=True, check=True
    )
    return proc.stdout.strip()


def _write(path: pathlib.Path, rel: str, text: str) -> None:
    target = path / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


def _commit(path: pathlib.Path, message: str) -> str:
    _git(path, "add", "-A")
    _git(path, "commit", "-q", "-m", message)
    return _git(path, "rev-parse", "HEAD")


def build_synth_repo(root: pathlib.Path) -> SynthRepo:
    root.mkdir(parents=True, exist_ok=True)
    _git(root, "init", "-q", "-b", "main")
    _git(root, "config", "user.name", "bench")
    _git(root, "config", "user.email", "bench@example.invalid")

    shas: list[str] = []

    # --- c0: green root -------------------------------------------------
    _write(root, "pytest.ini", "[pytest]\ntestpaths = tests\n")
    _write(root, "src/__init__.py", "")
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b\n")
    _write(root, "src/beta.py", "def mul(a, b):\n    return a * b\n")
    _write(
        root,
        "tests/test_alpha.py",
        "from src.alpha import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n",
    )
    _write(
        root,
        "tests/test_beta.py",
        "from src.beta import mul\n\n\ndef test_mul():\n    assert mul(2, 3) == 6\n",
    )
    shas.append(_commit(root, "c0: alpha and beta"))

    # --- c1: break alpha ------------------------------------------------
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b + 1\n")
    shas.append(_commit(root, "c1: off-by-one in alpha"))

    # --- c2: fix alpha, add gamma (green) -------------------------------
    _write(root, "src/alpha.py", "def add(a, b):\n    return a + b\n")
    _write(root, "src/gamma.py", "def sub(a, b):\n    return a - b\n")
    _write(
        root,
        "tests/test_gamma.py",
        "from src.gamma import sub\n\n\ndef test_sub():\n    assert sub(5, 2) == 3\n",
    )
    shas.append(_commit(root, "c2: fix alpha, add gamma"))

    # --- c3: break beta, touch gamma harmlessly -------------------------
    _write(root, "src/beta.py", "def mul(a, b):\n    return a * b + 1\n")
    _write(root, "src/gamma.py", "def sub(a, b):\n    # subtraction\n    return a - b\n")
    shas.append(_commit(root, "c3: off-by-one in beta"))

    _git(root, "checkout", "-q", shas[-1])
    return SynthRepo(path=root, commits=tuple(shas))
```

```python
# bench/tests/conftest.py
from __future__ import annotations

import pathlib
import sys

import pytest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

from tests.synthrepo import build_synth_repo  # noqa: E402


@pytest.fixture
def synth(tmp_path):
    return build_synth_repo(tmp_path / "synth")


@pytest.fixture
def cache_root(tmp_path):
    d = tmp_path / "cache"
    d.mkdir()
    return d
```

- [ ] **4.4 Run and pass.** `cd bench && uv run pytest tests/test_synthrepo.py -q` → 1 passed.

- [ ] **4.5 Commit.** `git add bench/tests/synthrepo.py bench/tests/conftest.py bench/tests/test_synthrepo.py && git commit -m "bench: synthetic replay repo with hand-computed outcomes"`

---

## Task 5 — Git worktrees and tree materialisation

**Files:** `bench/replay/gitwork.py`, `bench/tests/test_gitwork.py`

**Interfaces:**

```python
# bench/replay/gitwork.py
@dataclasses.dataclass(frozen=True)
class Change:
    path: str          # repo-relative, slash-separated
    status: str        # "A" | "M" | "D"

@dataclasses.dataclass(frozen=True)
class ReplayPoint:
    commit: str
    parent: str

def git(cwd: pathlib.Path, *args: str, check: bool = True) -> str: ...
def clone_pinned(url: str, pin: str, dest: pathlib.Path) -> pathlib.Path: ...
def replay_points(repo: pathlib.Path, ref: str, n: int) -> list[ReplayPoint]: ...
def diff_changes(repo: pathlib.Path, base: str, head: str) -> list[Change]: ...
def add_worktree(repo: pathlib.Path, sha: str, dest: pathlib.Path) -> pathlib.Path: ...
def remove_worktree(repo: pathlib.Path, dest: pathlib.Path) -> None: ...
def is_test_path(rel: str, test_globs: Sequence[str]) -> bool: ...
def materialise_natural(work: pathlib.Path, repo: pathlib.Path, point: ReplayPoint) -> list[Change]: ...
def materialise_probe(
    work: pathlib.Path, repo: pathlib.Path, point: ReplayPoint, test_globs: Sequence[str]
) -> list[Change]: ...
def working_changed_paths(work: pathlib.Path) -> list[Change]: ...
```

`materialise_natural` leaves HEAD at `point.parent` and writes `point.commit`'s content for
every changed path — additions land as untracked files, deletions as removals, so
`git status --porcelain -uall` in that worktree reproduces spec §5's changed set exactly.
`--no-renames` is mandatory: RTDD must see a rename as delete + add, which is one of the
change classes A8 says mutation testing cannot produce.

- [ ] **5.1 Failing tests.**

```python
# bench/tests/test_gitwork.py
import pathlib

from replay.gitwork import (
    Change,
    add_worktree,
    diff_changes,
    git,
    is_test_path,
    materialise_natural,
    materialise_probe,
    replay_points,
    working_changed_paths,
)

TEST_GLOBS = ("tests/**/*.py", "**/test_*.py", "**/conftest.py")


def test_replay_points_are_oldest_first_with_parents(synth):
    pts = replay_points(synth.path, synth.sha(3), 3)
    assert [p.commit for p in pts] == [synth.sha(1), synth.sha(2), synth.sha(3)]
    assert [p.parent for p in pts] == [synth.sha(0), synth.sha(1), synth.sha(2)]


def test_diff_changes_reports_adds_and_modifies(synth):
    ch = diff_changes(synth.path, synth.sha(1), synth.sha(2))
    assert sorted((c.path, c.status) for c in ch) == [
        ("src/alpha.py", "M"),
        ("src/gamma.py", "A"),
        ("tests/test_gamma.py", "A"),
    ]


def test_is_test_path(synth):
    assert is_test_path("tests/test_gamma.py", TEST_GLOBS)
    assert is_test_path("conftest.py", TEST_GLOBS)
    assert not is_test_path("src/gamma.py", TEST_GLOBS)


def test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked(synth, tmp_path):
    work = tmp_path / "wt"
    pts = replay_points(synth.path, synth.sha(3), 3)
    c2 = [p for p in pts if p.commit == synth.sha(2)][0]
    add_worktree(synth.path, c2.parent, work)
    changed = materialise_natural(work, synth.path, c2)

    assert git(work, "rev-parse", "HEAD") == c2.parent
    assert (work / "src/gamma.py").read_text() == "def sub(a, b):\n    return a - b\n"
    assert (work / "src/alpha.py").read_text() == "def add(a, b):\n    return a + b\n"
    assert sorted(c.path for c in changed) == [
        "src/alpha.py",
        "src/gamma.py",
        "tests/test_gamma.py",
    ]

    seen = {c.path: c.status for c in working_changed_paths(work)}
    assert seen["src/gamma.py"] == "A"
    assert seen["tests/test_gamma.py"] == "A"
    assert seen["src/alpha.py"] == "M"


def test_materialise_probe_keeps_child_tests_over_parent_source(synth, tmp_path):
    work = tmp_path / "wt"
    pts = replay_points(synth.path, synth.sha(3), 3)
    c2 = [p for p in pts if p.commit == synth.sha(2)][0]
    add_worktree(synth.path, c2.commit, work)
    changed = materialise_probe(work, synth.path, c2, TEST_GLOBS)

    assert git(work, "rev-parse", "HEAD") == c2.commit
    # c2's tests survive
    assert (work / "tests/test_gamma.py").exists()
    # c2's source is reverted to c1: gamma.py did not exist, alpha.py is broken
    assert not (work / "src/gamma.py").exists()
    assert (work / "src/alpha.py").read_text() == "def add(a, b):\n    return a + b + 1\n"
    assert sorted(c.path for c in changed) == ["src/alpha.py", "src/gamma.py"]
```

- [ ] **5.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_gitwork.py -q` → `ModuleNotFoundError: No module named 'replay.gitwork'`.

- [ ] **5.3 Implement.**

```python
# bench/replay/gitwork.py
from __future__ import annotations

import dataclasses
import fnmatch
import os
import pathlib
import shutil
import subprocess
from collections.abc import Sequence

_ENV = {
    "GIT_TERMINAL_PROMPT": "0",
    "GIT_CONFIG_NOSYSTEM": "1",
}


class GitError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class Change:
    path: str
    status: str


@dataclasses.dataclass(frozen=True)
class ReplayPoint:
    commit: str
    parent: str


def git(cwd: pathlib.Path, *args: str, check: bool = True) -> str:
    env = dict(os.environ)
    env.update(_ENV)
    proc = subprocess.run(
        ["git", *args], cwd=cwd, env=env, capture_output=True, text=True
    )
    if check and proc.returncode != 0:
        raise GitError(f"git {' '.join(args)} failed in {cwd}: {proc.stderr.strip()}")
    return proc.stdout.strip()


def clone_pinned(url: str, pin: str, dest: pathlib.Path) -> pathlib.Path:
    if dest.exists():
        head = git(dest, "rev-parse", "HEAD")
        if head != pin:
            raise GitError(f"{dest} is at {head}, corpus pin is {pin}; delete it and re-clone")
        return dest
    dest.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "clone", "--quiet", url, str(dest)], check=True)
    git(dest, "checkout", "-q", pin)
    return dest


def replay_points(repo: pathlib.Path, ref: str, n: int) -> list[ReplayPoint]:
    """Last n commits ending at ref, oldest first. Merge commits are skipped: a merge's
    parent is ambiguous, and spec §4 already escalates merges to T1."""
    out = git(repo, "rev-list", "--no-merges", f"--max-count={n}", ref)
    shas = [s for s in out.splitlines() if s]
    shas.reverse()
    points: list[ReplayPoint] = []
    for sha in shas:
        parents = git(repo, "rev-list", "--parents", "-n", "1", sha).split()
        if len(parents) < 2:
            continue  # root commit has no parent to replay from
        points.append(ReplayPoint(commit=sha, parent=parents[1]))
    return points


def diff_changes(repo: pathlib.Path, base: str, head: str) -> list[Change]:
    out = git(repo, "diff", "--no-renames", "--name-status", "-z", base, head)
    fields = [f for f in out.split("\0") if f]
    changes: list[Change] = []
    for i in range(0, len(fields) - 1, 2):
        status = fields[i][0]
        path = fields[i + 1]
        if status not in ("A", "M", "D"):
            status = "M"
        changes.append(Change(path=path, status=status))
    return changes


def add_worktree(repo: pathlib.Path, sha: str, dest: pathlib.Path) -> pathlib.Path:
    dest.parent.mkdir(parents=True, exist_ok=True)
    git(repo, "worktree", "add", "--detach", "--force", str(dest), sha)
    return dest


def remove_worktree(repo: pathlib.Path, dest: pathlib.Path) -> None:
    git(repo, "worktree", "remove", "--force", str(dest), check=False)
    if dest.exists():
        shutil.rmtree(dest, ignore_errors=True)
    git(repo, "worktree", "prune", check=False)


def is_test_path(rel: str, test_globs: Sequence[str]) -> bool:
    return any(fnmatch.fnmatch(rel, g) or fnmatch.fnmatch("/" + rel, "*/" + g) for g in test_globs)


def _write_blob(work: pathlib.Path, repo: pathlib.Path, sha: str, rel: str) -> None:
    blob = subprocess.run(
        ["git", "show", f"{sha}:{rel}"], cwd=repo, capture_output=True, check=True
    ).stdout
    target = work / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_bytes(blob)


def _remove(work: pathlib.Path, rel: str) -> None:
    target = work / rel
    if target.exists():
        target.unlink()


def materialise_natural(
    work: pathlib.Path, repo: pathlib.Path, point: ReplayPoint
) -> list[Change]:
    """Worktree is at point.parent. Write point.commit's content over it without moving HEAD."""
    changes = diff_changes(repo, point.parent, point.commit)
    for ch in changes:
        if ch.status == "D":
            _remove(work, ch.path)
        else:
            _write_blob(work, repo, point.commit, ch.path)
    return changes


def materialise_probe(
    work: pathlib.Path,
    repo: pathlib.Path,
    point: ReplayPoint,
    test_globs: Sequence[str],
) -> list[Change]:
    """Worktree is at point.commit. Revert only the source half of the diff back to parent,
    leaving the commit's tests in place."""
    reverted: list[Change] = []
    for ch in diff_changes(repo, point.parent, point.commit):
        if is_test_path(ch.path, test_globs):
            continue
        if ch.status == "A":
            _remove(work, ch.path)          # did not exist at parent
            reverted.append(Change(ch.path, "D"))
        elif ch.status == "D":
            _write_blob(work, repo, point.parent, ch.path)
            reverted.append(Change(ch.path, "A"))
        else:
            _write_blob(work, repo, point.parent, ch.path)
            reverted.append(Change(ch.path, "M"))
    return reverted


def working_changed_paths(work: pathlib.Path) -> list[Change]:
    """Spec §5's changed set: git status --porcelain -uall over the worktree."""
    out = git(work, "status", "--porcelain=1", "-uall", "-z")
    fields = [f for f in out.split("\0") if f]
    changes: list[Change] = []
    for f in fields:
        code, path = f[:2], f[3:]
        if "D" in code:
            status = "D"
        elif "?" in code or "A" in code:
            status = "A"
        else:
            status = "M"
        changes.append(Change(path=path, status=status))
    return changes
```

- [ ] **5.4 Run and pass.** `cd bench && uv run pytest tests/test_gitwork.py -q` → 5 passed.

- [ ] **5.5 Commit.** `git add bench/replay/gitwork.py bench/tests/test_gitwork.py && git commit -m "bench: worktrees and natural/probe tree materialisation"`

---

## Task 6 — Content-addressed cache

Ground truth is the expensive part: a full suite per commit per variant, plus an
instrumented full suite for the false-signal metric. All of it is cached by
`(repo_id, commit, variant, strategy, config_digest)`, so a re-run after a code change to
the harness is cheap and a re-run after a *config* change is correctly a cache miss.

**Files:** `bench/replay/cache.py`, `bench/tests/test_cache.py`

**Interfaces:**

```python
# bench/replay/cache.py
class Cache:
    def __init__(self, root: pathlib.Path, config_digest: str) -> None: ...
    def key(self, *parts: str) -> str: ...
    def get_json(self, key: str) -> dict | None: ...
    def put_json(self, key: str, obj: dict) -> None: ...
    def json_or_build(self, key: str, build: Callable[[], dict]) -> dict: ...
    def artifact(self, key: str) -> pathlib.Path | None: ...
    def store_artifact(self, key: str, src_dir: pathlib.Path) -> pathlib.Path: ...
    def restore_artifact(self, key: str, dest_dir: pathlib.Path) -> bool: ...
    def stats(self) -> dict: ...
```

- [ ] **6.1 Failing tests.**

```python
# bench/tests/test_cache.py
from replay.cache import Cache


def test_key_includes_config_digest_so_a_config_change_misses(cache_root):
    a = Cache(cache_root, "cfg-a")
    b = Cache(cache_root, "cfg-b")
    assert a.key("repo", "sha", "natural", "rtdd") != b.key("repo", "sha", "natural", "rtdd")
    assert a.key("repo", "sha", "natural", "rtdd") == a.key("repo", "sha", "natural", "rtdd")


def test_json_round_trip_and_build_is_called_once(cache_root):
    c = Cache(cache_root, "cfg")
    calls = []

    def build():
        calls.append(1)
        return {"tests": ["a::b"], "n": 2}

    k = c.key("repo", "sha", "natural", "full")
    assert c.json_or_build(k, build) == {"tests": ["a::b"], "n": 2}
    assert c.json_or_build(k, build) == {"tests": ["a::b"], "n": 2}
    assert calls == [1]
    assert c.stats() == {"hits": 1, "misses": 1}


def test_artifact_round_trip(cache_root, tmp_path):
    c = Cache(cache_root, "cfg")
    src = tmp_path / "state"
    (src / "sub").mkdir(parents=True)
    (src / "map.jsonl").write_text('{"t":"x"}\n', encoding="utf-8")
    (src / "sub" / "meta.json").write_text("{}", encoding="utf-8")

    k = c.key("repo", "sha", "seed", "rtdd")
    assert c.artifact(k) is None
    c.store_artifact(k, src)
    assert c.artifact(k) is not None

    dest = tmp_path / "restored"
    assert c.restore_artifact(k, dest) is True
    assert (dest / "map.jsonl").read_text() == '{"t":"x"}\n'
    assert (dest / "sub" / "meta.json").read_text() == "{}"
```

- [ ] **6.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_cache.py -q` → `ModuleNotFoundError: No module named 'replay.cache'`.

- [ ] **6.3 Implement.**

```python
# bench/replay/cache.py
from __future__ import annotations

import hashlib
import json
import pathlib
import shutil
import tempfile
from collections.abc import Callable

from replay.config import canonical_json


class Cache:
    def __init__(self, root: pathlib.Path, config_digest: str) -> None:
        self.root = pathlib.Path(root)
        self.config_digest = config_digest
        self.root.mkdir(parents=True, exist_ok=True)
        self._hits = 0
        self._misses = 0

    def key(self, *parts: str) -> str:
        raw = "\0".join((self.config_digest, *parts))
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()

    def _json_path(self, key: str) -> pathlib.Path:
        return self.root / "json" / key[:2] / f"{key}.json"

    def _artifact_path(self, key: str) -> pathlib.Path:
        return self.root / "artifact" / key[:2] / key

    def get_json(self, key: str) -> dict | None:
        p = self._json_path(key)
        if not p.exists():
            return None
        return json.loads(p.read_text(encoding="utf-8"))

    def put_json(self, key: str, obj: dict) -> None:
        p = self._json_path(key)
        p.parent.mkdir(parents=True, exist_ok=True)
        tmp = p.with_suffix(".tmp")
        tmp.write_text(canonical_json(obj), encoding="utf-8")
        tmp.replace(p)

    def json_or_build(self, key: str, build: Callable[[], dict]) -> dict:
        got = self.get_json(key)
        if got is not None:
            self._hits += 1
            return got
        self._misses += 1
        obj = build()
        self.put_json(key, obj)
        return obj

    def artifact(self, key: str) -> pathlib.Path | None:
        p = self._artifact_path(key)
        return p if (p / ".complete").exists() else None

    def store_artifact(self, key: str, src_dir: pathlib.Path) -> pathlib.Path:
        p = self._artifact_path(key)
        if p.exists():
            shutil.rmtree(p)
        p.parent.mkdir(parents=True, exist_ok=True)
        staging = pathlib.Path(tempfile.mkdtemp(dir=str(p.parent)))
        shutil.copytree(src_dir, staging / "d")
        (staging / "d" / ".complete").write_text("", encoding="utf-8")
        (staging / "d").replace(p)
        shutil.rmtree(staging, ignore_errors=True)
        return p

    def restore_artifact(self, key: str, dest_dir: pathlib.Path) -> bool:
        p = self.artifact(key)
        if p is None:
            self._misses += 1
            return False
        self._hits += 1
        dest_dir.mkdir(parents=True, exist_ok=True)
        for item in p.iterdir():
            if item.name == ".complete":
                continue
            target = dest_dir / item.name
            if item.is_dir():
                shutil.copytree(item, target, dirs_exist_ok=True)
            else:
                shutil.copy2(item, target)
        return True

    def stats(self) -> dict:
        return {"hits": self._hits, "misses": self._misses}
```

- [ ] **6.4 Run and pass.** `cd bench && uv run pytest tests/test_cache.py -q` → 3 passed.

- [ ] **6.5 Commit.** `git add bench/replay/cache.py bench/tests/test_cache.py && git commit -m "bench: content-addressed cache keyed by repo+commit+strategy+config"`

---

## Task 7 — Pytest runner: full, subset, instrumented

**Files:** `bench/replay/runner.py`, `bench/tests/test_runner.py`

**Interfaces:**

```python
# bench/replay/runner.py
@dataclasses.dataclass(frozen=True)
class Outcome:
    test: str
    status: str          # "pass" | "fail" | "skip" | "error"
    duration_ms: int

@dataclasses.dataclass(frozen=True)
class RunResult:
    outcomes: tuple[Outcome, ...]
    exit_code: int
    wall_ms: int
    collected: tuple[str, ...]

    def failing(self) -> frozenset[str]: ...
    def durations(self) -> dict[str, int]: ...

def collect(work: pathlib.Path, python: str = sys.executable) -> tuple[str, ...]: ...
def run_full(work, python=sys.executable, instrumented=False, source_globs=(), xdist=False) -> RunResult: ...
def run_subset(work, tests, python=sys.executable, instrumented=False, source_globs=()) -> RunResult: ...
def parse_report_log(path: pathlib.Path) -> tuple[Outcome, ...]: ...
def chunk(tests: Sequence[str], max_bytes: int = 100_000) -> list[list[str]]: ...
```

Instrumented runs force `COVERAGE_CORE=ctrace` and `--cov-context=test`, exactly as spec §4
requires, and treat a `no-sysmon-context` warning in stderr as fatal
(`SysmonContextError`) — a benchmark that silently measured a 90%-empty map would be worse
than no benchmark.

- [ ] **7.1 Failing tests.**

```python
# bench/tests/test_runner.py
import subprocess

import pytest

from replay.runner import RunResult, chunk, collect, run_full, run_subset


def _checkout(repo, sha):
    subprocess.run(["git", "checkout", "-q", sha], cwd=repo, check=True)


def test_collect_lists_every_test_id(synth):
    _checkout(synth.path, synth.sha(2))
    ids = collect(synth.path)
    assert set(ids) == {
        "tests/test_alpha.py::test_add",
        "tests/test_beta.py::test_mul",
        "tests/test_gamma.py::test_sub",
    }


def test_run_full_at_green_and_red_commits(synth):
    _checkout(synth.path, synth.sha(2))
    green = run_full(synth.path)
    assert green.exit_code == 0
    assert green.failing() == frozenset()
    assert set(green.durations()) == set(green.collected)
    assert green.wall_ms > 0

    _checkout(synth.path, synth.sha(3))
    red = run_full(synth.path)
    assert red.exit_code == 1
    assert red.failing() == frozenset({"tests/test_beta.py::test_mul"})


def test_run_subset_runs_only_what_it_is_given(synth):
    _checkout(synth.path, synth.sha(3))
    res = run_subset(synth.path, ["tests/test_alpha.py::test_add"])
    assert res.exit_code == 0
    assert [o.test for o in res.outcomes] == ["tests/test_alpha.py::test_add"]


def test_instrumented_run_produces_a_coverage_db(synth):
    _checkout(synth.path, synth.sha(2))
    res = run_full(synth.path, instrumented=True, source_globs=("src",))
    assert res.exit_code == 0
    assert (synth.path / ".coverage").exists()


def test_chunk_splits_on_argv_budget():
    ids = [f"tests/test_x.py::test_{i:04d}" for i in range(500)]
    parts = chunk(ids, max_bytes=1000)
    assert len(parts) > 1
    assert [i for p in parts for i in p] == ids
    assert all(sum(len(i) + 1 for i in p) <= 1000 for p in parts)
```

- [ ] **7.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_runner.py -q` → `ModuleNotFoundError: No module named 'replay.runner'`.

- [ ] **7.3 Implement.**

```python
# bench/replay/runner.py
from __future__ import annotations

import dataclasses
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
from collections.abc import Sequence

MAX_ARGV_BYTES = 100_000


class SysmonContextError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class Outcome:
    test: str
    status: str
    duration_ms: int


@dataclasses.dataclass(frozen=True)
class RunResult:
    outcomes: tuple[Outcome, ...]
    exit_code: int
    wall_ms: int
    collected: tuple[str, ...]

    def failing(self) -> frozenset[str]:
        return frozenset(o.test for o in self.outcomes if o.status in ("fail", "error"))

    def durations(self) -> dict[str, int]:
        return {o.test: o.duration_ms for o in self.outcomes}


def _env(instrumented: bool) -> dict[str, str]:
    env = dict(os.environ)
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    if instrumented:
        env["COVERAGE_CORE"] = "ctrace"
    return env


def parse_report_log(path: pathlib.Path) -> tuple[Outcome, ...]:
    """pytest --report-log JSONL. Only call-phase TestReports produce a pass/fail;
    setup/teardown failures map to 'error' so a broken fixture is never scored as a pass."""
    status: dict[str, str] = {}
    duration: dict[str, float] = {}
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        if not line.strip():
            continue
        rec = json.loads(line)
        if rec.get("$report_type") != "TestReport":
            continue
        nodeid = rec["nodeid"]
        phase = rec.get("when")
        outcome = rec.get("outcome")
        duration[nodeid] = duration.get(nodeid, 0.0) + float(rec.get("duration", 0.0))
        if phase == "call":
            status[nodeid] = {"passed": "pass", "failed": "fail", "skipped": "skip"}.get(outcome, "error")
        elif phase in ("setup", "teardown") and outcome == "failed":
            status[nodeid] = "error"
        elif phase == "setup" and outcome == "skipped":
            status.setdefault(nodeid, "skip")
    return tuple(
        Outcome(test=t, status=status[t], duration_ms=int(round(duration.get(t, 0.0) * 1000)))
        for t in sorted(status)
    )


def chunk(tests: Sequence[str], max_bytes: int = MAX_ARGV_BYTES) -> list[list[str]]:
    parts: list[list[str]] = []
    cur: list[str] = []
    size = 0
    for t in tests:
        cost = len(t) + 1
        if cur and size + cost > max_bytes:
            parts.append(cur)
            cur, size = [], 0
        cur.append(t)
        size += cost
    if cur:
        parts.append(cur)
    return parts


def collect(work: pathlib.Path, python: str = sys.executable) -> tuple[str, ...]:
    proc = subprocess.run(
        [python, "-m", "pytest", "--collect-only", "-q", "--no-header", "-p", "no:cacheprovider"],
        cwd=work,
        capture_output=True,
        text=True,
        env=_env(False),
    )
    ids = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if "::" in line and not line.startswith(("=", "-", "<")):
            ids.append(line)
    return tuple(sorted(set(ids)))


def _pytest_argv(
    instrumented: bool, source_globs: Sequence[str], log: pathlib.Path, xdist: bool
) -> list[str]:
    argv = ["-q", "--no-header", "-p", "no:cacheprovider", f"--report-log={log}"]
    if xdist:
        argv += ["-n", "auto"]
    if instrumented:
        for src in source_globs or (".",):
            argv.append(f"--cov={src}")
        argv += ["--cov-context=test", "--cov-report=", "--cov-branch"]
    return argv


def _invoke(
    work: pathlib.Path,
    python: str,
    tests: Sequence[str],
    instrumented: bool,
    source_globs: Sequence[str],
    xdist: bool,
) -> RunResult:
    outcomes: list[Outcome] = []
    exit_code = 0
    wall_ms = 0
    batches = chunk(list(tests)) if tests else [[]]
    for batch in batches:
        with tempfile.TemporaryDirectory() as td:
            log = pathlib.Path(td) / "report.jsonl"
            argv = [python, "-m", "pytest", *_pytest_argv(instrumented, source_globs, log, xdist), *batch]
            start = time.perf_counter()
            proc = subprocess.run(
                argv, cwd=work, capture_output=True, text=True, env=_env(instrumented)
            )
            wall_ms += int(round((time.perf_counter() - start) * 1000))
            if instrumented and "no-sysmon-context" in (proc.stderr + proc.stdout):
                raise SysmonContextError(
                    "coverage.py emitted no-sysmon-context: dynamic contexts were dropped. "
                    "Any map built from this run is ~90% empty (audit A7)."
                )
            if proc.returncode in (4, 5):
                raise RuntimeError(
                    f"pytest exit {proc.returncode} (bad selector / no tests collected) in {work}: "
                    f"{proc.stdout[-2000:]}"
                )
            if log.exists():
                outcomes.extend(parse_report_log(log))
            exit_code = exit_code or proc.returncode
    uniq = {o.test: o for o in outcomes}
    ordered = tuple(uniq[t] for t in sorted(uniq))
    return RunResult(
        outcomes=ordered,
        exit_code=exit_code,
        wall_ms=wall_ms,
        collected=tuple(o.test for o in ordered),
    )


def run_full(
    work: pathlib.Path,
    python: str = sys.executable,
    instrumented: bool = False,
    source_globs: Sequence[str] = (),
    xdist: bool = False,
) -> RunResult:
    return _invoke(work, python, (), instrumented, source_globs, xdist)


def run_subset(
    work: pathlib.Path,
    tests: Sequence[str],
    python: str = sys.executable,
    instrumented: bool = False,
    source_globs: Sequence[str] = (),
) -> RunResult:
    if not tests:
        return RunResult(outcomes=(), exit_code=0, wall_ms=0, collected=())
    return _invoke(work, python, tests, instrumented, source_globs, False)
```

- [ ] **7.4 Run and pass.** `cd bench && uv run pytest tests/test_runner.py -q` → 5 passed.

- [ ] **7.5 Commit.** `git add bench/replay/runner.py bench/tests/test_runner.py && git commit -m "bench: pytest runner with report-log parsing and ctrace enforcement"`

---

## Task 8 — Coverage reader for the false-signal ground truth

**Files:** `bench/replay/covread.py`, `bench/tests/test_covread.py`

**Interfaces:**

```python
# bench/replay/covread.py
@dataclasses.dataclass(frozen=True)
class CoverageTruth:
    by_test: dict[str, frozenset[tuple[str, int]]]   # test id -> {(path, line)}
    import_time: frozenset[tuple[str, int]]
    covered: frozenset[tuple[str, int]]              # union over by_test — NOT import_time

def numbits(blob: bytes) -> list[int]: ...
def normalize_context(ctx: str) -> tuple[str, bool]: ...   # (test id, is_test)
def read_coverage(db_path: pathlib.Path, work_root: pathlib.Path) -> CoverageTruth: ...
```

`covered` deliberately excludes `import_time`. That is the whole point of the false-signal
metric: a line that only ever ran in the empty context was executed but asserted on by no
test, RTDD reports it as `import-time` rather than `uncovered`, and it must not count as
"in fact adequately tested".

- [ ] **8.1 Failing tests.**

```python
# bench/tests/test_covread.py
import subprocess

from replay.covread import normalize_context, numbits, read_coverage
from replay.runner import run_full


def test_numbits_decodes_the_packed_bitmap():
    assert numbits(b"") == []
    assert numbits(b"\x01") == [0]
    assert numbits(b"\x02") == [1]
    assert numbits(b"\x00\x81") == [8, 15]


def test_normalize_context_strips_the_phase_suffix():
    assert normalize_context("tests/test_a.py::test_x|run") == ("tests/test_a.py::test_x", True)
    assert normalize_context("tests/test_a.py::test_x[1::2]|setup") == ("tests/test_a.py::test_x[1::2]", True)
    assert normalize_context("") == ("", False)


def test_read_coverage_separates_per_test_from_import_time(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    res = run_full(synth.path, instrumented=True, source_globs=("src",))
    assert res.exit_code == 0

    truth = read_coverage(synth.path / ".coverage", synth.path)
    assert "tests/test_alpha.py::test_add" in truth.by_test
    body = ("src/alpha.py", 2)     # `return a + b`
    assert body in truth.by_test["tests/test_alpha.py::test_add"]
    assert body in truth.covered
    # the `def` line executes at import, attributed to no test
    assert ("src/alpha.py", 1) in truth.import_time
    assert truth.covered.isdisjoint(truth.import_time - truth.covered)
```

- [ ] **8.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_covread.py -q` → `ModuleNotFoundError: No module named 'replay.covread'`.

- [ ] **8.3 Implement.**

```python
# bench/replay/covread.py
from __future__ import annotations

import dataclasses
import pathlib
import sqlite3


@dataclasses.dataclass(frozen=True)
class CoverageTruth:
    by_test: dict[str, frozenset[tuple[str, int]]]
    import_time: frozenset[tuple[str, int]]
    covered: frozenset[tuple[str, int]]


def numbits(blob: bytes) -> list[int]:
    lines: list[int] = []
    for i, byte in enumerate(blob):
        for j in range(8):
            if byte & (1 << j):
                lines.append(i * 8 + j)
    return lines


def normalize_context(ctx: str) -> tuple[str, bool]:
    if not ctx:
        return ("", False)
    head, sep, tail = ctx.rpartition("|")
    if sep and tail in ("run", "setup", "teardown"):
        return (head, True)
    return (ctx, True)


def _rel(work_root: pathlib.Path, raw: str) -> str | None:
    p = pathlib.Path(raw)
    try:
        return p.resolve().relative_to(work_root.resolve()).as_posix()
    except ValueError:
        return None


def read_coverage(db_path: pathlib.Path, work_root: pathlib.Path) -> CoverageTruth:
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    try:
        rows = conn.execute(
            "SELECT f.path, c.context, lb.numbits FROM line_bits lb "
            "JOIN file f ON f.id = lb.file_id "
            "JOIN context c ON c.id = lb.context_id"
        ).fetchall()
    finally:
        conn.close()

    by_test: dict[str, set[tuple[str, int]]] = {}
    import_time: set[tuple[str, int]] = set()
    for raw_path, raw_ctx, blob in rows:
        rel = _rel(work_root, raw_path)
        if rel is None:
            continue
        pairs = {(rel, ln) for ln in numbits(blob or b"")}
        test_id, is_test = normalize_context(raw_ctx or "")
        if is_test:
            by_test.setdefault(test_id, set()).update(pairs)
        else:
            import_time.update(pairs)

    covered: set[tuple[str, int]] = set()
    for pairs in by_test.values():
        covered.update(pairs)
    return CoverageTruth(
        by_test={k: frozenset(v) for k, v in by_test.items()},
        import_time=frozenset(import_time),
        covered=frozenset(covered),
    )
```

- [ ] **8.4 Run and pass.** `cd bench && uv run pytest tests/test_covread.py -q` → 3 passed.

- [ ] **8.5 Commit.** `git add bench/replay/covread.py bench/tests/test_covread.py && git commit -m "bench: .coverage reader separating per-test from import-time lines"`

---

## Task 9 — The `rtdd` binary adapter

The **only** module that shells out to `rtdd` or knows its `--json` schema. If M2's actual
schema differs from what is written here, this file is the single place to fix.

**Files:** `bench/replay/rtddio.py`, `bench/tests/test_rtddio.py`

**Consumed schema** (`rtdd which --base HEAD --json`, per `docs/plans/00-interfaces.md`):

```json
{"tier":"T0","reason":"","changed":["src/a.py"],"direct":[],"tests":["tests/test_a.py::test_x"],"cycles":3}
```

**Consumed schema** (`rtdd run --base HEAD --json`):

```json
{"tier":"T0","tests":["tests/test_a.py::test_x"],
 "outcomes":[{"test":"tests/test_a.py::test_x","status":"pass","duration_ms":12}],
 "uncovered":[{"path":"src/a.py","ranges":[{"start":52,"end":58}]}],
 "import_time":[{"path":"src/constants.py","ranges":[{"start":1,"end":12}]}],
 "exit_code":0}
```

**Interfaces:**

```python
# bench/replay/rtddio.py
class RtddError(RuntimeError): ...

@dataclasses.dataclass(frozen=True)
class WhichResult:
    tier: str
    reason: str
    tests: tuple[str, ...]
    direct: tuple[str, ...]
    changed: tuple[str, ...]
    cycles: int
    wall_ms: int

    def escalated(self) -> bool: ...   # tier == "T2"

@dataclasses.dataclass(frozen=True)
class RunOutput:
    tier: str
    tests: tuple[str, ...]
    outcomes: tuple[tuple[str, str, int], ...]
    uncovered: frozenset[tuple[str, int]]
    import_time: frozenset[tuple[str, int]]
    exit_code: int
    wall_ms: int

def rtdd_version(binary: str = "rtdd") -> str: ...
def seed(work: pathlib.Path, binary: str = "rtdd") -> None: ...
def which(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> WhichResult: ...
def run(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> RunOutput: ...
def parse_which(payload: dict, wall_ms: int) -> WhichResult: ...
def parse_run(payload: dict, wall_ms: int) -> RunOutput: ...
def expand_ranges(entries: list[dict]) -> frozenset[tuple[str, int]]: ...
```

Parsing is separated from subprocessing so the schema is tested without the binary; a
contract test then asserts the real binary honours it.

- [ ] **9.1 Failing tests for the parsers.**

```python
# bench/tests/test_rtddio.py
import shutil

import pytest

from replay.rtddio import expand_ranges, parse_run, parse_which


def test_parse_which():
    w = parse_which(
        {
            "tier": "T0",
            "reason": "",
            "changed": ["src/a.py"],
            "direct": ["tests/test_new.py::test_n"],
            "tests": ["tests/test_new.py::test_n", "tests/test_a.py::test_x"],
            "cycles": 3,
        },
        wall_ms=17,
    )
    assert w.tier == "T0"
    assert w.tests == ("tests/test_new.py::test_n", "tests/test_a.py::test_x")
    assert w.direct == ("tests/test_new.py::test_n",)
    assert w.cycles == 3
    assert w.wall_ms == 17
    assert not w.escalated()


def test_parse_which_flags_t2_as_escalation():
    w = parse_which({"tier": "T2", "reason": "pyproject.toml changed", "tests": []}, wall_ms=1)
    assert w.escalated()
    assert w.reason == "pyproject.toml changed"


def test_expand_ranges_is_inclusive():
    got = expand_ranges([{"path": "src/a.py", "ranges": [{"start": 52, "end": 54}]}])
    assert got == frozenset({("src/a.py", 52), ("src/a.py", 53), ("src/a.py", 54)})


def test_parse_run_separates_uncovered_from_import_time():
    r = parse_run(
        {
            "tier": "T0",
            "tests": ["tests/test_a.py::test_x"],
            "outcomes": [{"test": "tests/test_a.py::test_x", "status": "pass", "duration_ms": 12}],
            "uncovered": [{"path": "src/a.py", "ranges": [{"start": 52, "end": 53}]}],
            "import_time": [{"path": "src/c.py", "ranges": [{"start": 1, "end": 1}]}],
            "exit_code": 0,
        },
        wall_ms=900,
    )
    assert r.uncovered == frozenset({("src/a.py", 52), ("src/a.py", 53)})
    assert r.import_time == frozenset({("src/c.py", 1)})
    assert r.outcomes == (("tests/test_a.py::test_x", "pass", 12),)
    assert r.wall_ms == 900


@pytest.mark.skipif(shutil.which("rtdd") is None, reason="rtdd binary not on PATH")
def test_real_binary_honours_the_consumed_schema(synth):
    import subprocess

    from replay.rtddio import seed, which

    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    seed(synth.path)
    (synth.path / "src" / "alpha.py").write_text("def add(a, b):\n    return a + b + 1\n")
    w = which(synth.path)
    assert w.tier in {"direct", "T0", "T1", "T2", "empty"}
    assert "tests/test_alpha.py::test_add" in w.tests
```

The `skipif` is deliberate and narrow: it guards only the *contract* test against a machine
without the binary. The orchestrator (Task 20) has no such guard — a missing `rtdd` there is
a hard failure, because a replay run without the system under test is not a result.

- [ ] **9.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_rtddio.py -q` → `ModuleNotFoundError: No module named 'replay.rtddio'`.

- [ ] **9.3 Implement.**

```python
# bench/replay/rtddio.py
from __future__ import annotations

import dataclasses
import json
import os
import pathlib
import subprocess
import time


class RtddError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class WhichResult:
    tier: str
    reason: str
    tests: tuple[str, ...]
    direct: tuple[str, ...]
    changed: tuple[str, ...]
    cycles: int
    wall_ms: int

    def escalated(self) -> bool:
        return self.tier == "T2"


@dataclasses.dataclass(frozen=True)
class RunOutput:
    tier: str
    tests: tuple[str, ...]
    outcomes: tuple[tuple[str, str, int], ...]
    uncovered: frozenset[tuple[str, int]]
    import_time: frozenset[tuple[str, int]]
    exit_code: int
    wall_ms: int


def expand_ranges(entries: list[dict]) -> frozenset[tuple[str, int]]:
    out: set[tuple[str, int]] = set()
    for e in entries or ():
        path = e["path"]
        for r in e.get("ranges") or ():
            for ln in range(int(r["start"]), int(r["end"]) + 1):
                out.add((path, ln))
    return frozenset(out)


def parse_which(payload: dict, wall_ms: int) -> WhichResult:
    return WhichResult(
        tier=str(payload.get("tier", "empty")),
        reason=str(payload.get("reason", "")),
        tests=tuple(payload.get("tests") or ()),
        direct=tuple(payload.get("direct") or ()),
        changed=tuple(payload.get("changed") or ()),
        cycles=int(payload.get("cycles", 0)),
        wall_ms=wall_ms,
    )


def parse_run(payload: dict, wall_ms: int) -> RunOutput:
    outcomes = tuple(
        (str(o["test"]), str(o["status"]), int(o.get("duration_ms", 0)))
        for o in payload.get("outcomes") or ()
    )
    return RunOutput(
        tier=str(payload.get("tier", "empty")),
        tests=tuple(payload.get("tests") or ()),
        outcomes=outcomes,
        uncovered=expand_ranges(payload.get("uncovered") or []),
        import_time=expand_ranges(payload.get("import_time") or []),
        exit_code=int(payload.get("exit_code", 0)),
        wall_ms=wall_ms,
    )


def _invoke(work: pathlib.Path, argv: list[str]) -> tuple[str, int, int]:
    env = dict(os.environ)
    start = time.perf_counter()
    proc = subprocess.run(argv, cwd=work, capture_output=True, text=True, env=env)
    wall_ms = int(round((time.perf_counter() - start) * 1000))
    if proc.returncode in (2, 3):
        raise RtddError(
            f"{' '.join(argv)} exited {proc.returncode} in {work}: {proc.stderr.strip()[-2000:]}"
        )
    return proc.stdout, proc.returncode, wall_ms


def rtdd_version(binary: str = "rtdd") -> str:
    proc = subprocess.run([binary, "--version"], capture_output=True, text=True)
    if proc.returncode != 0:
        raise RtddError(f"{binary} --version failed: {proc.stderr.strip()}")
    return proc.stdout.strip()


def seed(work: pathlib.Path, binary: str = "rtdd") -> None:
    _invoke(work, [binary, "seed"])


def which(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> WhichResult:
    out, _, wall_ms = _invoke(work, [binary, "which", "--base", base, "--json"])
    try:
        payload = json.loads(out)
    except json.JSONDecodeError as exc:
        raise RtddError(f"rtdd which --json emitted non-JSON: {out[:500]!r}") from exc
    return parse_which(payload, wall_ms)


def run(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> RunOutput:
    out, code, wall_ms = _invoke(work, [binary, "run", "--base", base, "--json"])
    try:
        payload = json.loads(out)
    except json.JSONDecodeError as exc:
        raise RtddError(f"rtdd run --json emitted non-JSON: {out[:500]!r}") from exc
    payload.setdefault("exit_code", code)
    return parse_run(payload, wall_ms)
```

- [ ] **9.4 Run and pass.** `cd bench && uv run pytest tests/test_rtddio.py -q` → 4 passed, 1 passed-or-skipped depending on whether `rtdd` is on PATH.

- [ ] **9.5 Commit.** `git add bench/replay/rtddio.py bench/tests/test_rtddio.py && git commit -m "bench: rtdd binary adapter and --json schema contract"`

---

## Task 10 — Strategy protocol and registry

**Files:** `bench/replay/strategies/__init__.py`, `bench/replay/strategies/base.py`, `bench/tests/test_strategy_base.py`

**Interfaces:**

```python
# bench/replay/strategies/base.py
@dataclasses.dataclass(frozen=True)
class Selection:
    tests: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int = 0
    exec_args: tuple[str, ...] = ()   # extra pytest argv, e.g. ("-n", "auto") for xdist

@dataclasses.dataclass(frozen=True)
class CommitContext:
    repo_id: str
    variant: str                      # "natural" | "probe"
    commit: str
    parent: str
    work: pathlib.Path                # the materialised worktree
    changed: tuple[Change, ...]       # spec §5 changed set, from working_changed_paths
    all_tests: tuple[str, ...]
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]
    python: str
    seed_ms: int = 0
    peer_sizes: dict[str, int] = dataclasses.field(default_factory=dict)

class Strategy(Protocol):
    id: str
    needs_parent_state: bool          # True -> the harness seeds it in the clean parent tree
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...

REGISTRY: dict[str, Strategy]
def register(s: Strategy) -> Strategy: ...
def get(strategy_id: str) -> Strategy: ...
def all_ids() -> tuple[str, ...]: ...
```

`peer_sizes` is how `randomratio` learns RTDD's selection size at the same commit; the
orchestrator fills it in and always runs `rtdd` before `random`.

- [ ] **10.1 Failing tests.**

```python
# bench/tests/test_strategy_base.py
import pathlib

import pytest

from replay.strategies.base import CommitContext, Selection, all_ids, get, register


class _Dummy:
    id = "dummy"
    needs_parent_state = False

    def prepare(self, ctx):
        return None

    def select(self, ctx):
        return Selection(tests=ctx.all_tests[:1], escalated=False, reason="first")


def _ctx():
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=("a::x", "b::y"),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def test_register_and_get():
    register(_Dummy())
    assert "dummy" in all_ids()
    s = get("dummy")
    assert s.select(_ctx()).tests == ("a::x",)


def test_unknown_strategy_is_an_error():
    with pytest.raises(KeyError):
        get("no-such-strategy")


def test_selection_defaults():
    s = Selection(tests=(), escalated=True, reason="unseeded")
    assert s.select_ms == 0
    assert s.exec_args == ()
```

- [ ] **10.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_base.py -q` → `ModuleNotFoundError: No module named 'replay.strategies'`.

- [ ] **10.3 Implement.**

```python
# bench/replay/strategies/__init__.py
from replay.strategies.base import REGISTRY, CommitContext, Selection, all_ids, get, register

__all__ = ["REGISTRY", "CommitContext", "Selection", "all_ids", "get", "register"]
```

```python
# bench/replay/strategies/base.py
from __future__ import annotations

import dataclasses
import pathlib
from typing import Protocol, runtime_checkable

from replay.gitwork import Change


@dataclasses.dataclass(frozen=True)
class Selection:
    tests: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int = 0
    exec_args: tuple[str, ...] = ()


@dataclasses.dataclass(frozen=True)
class CommitContext:
    repo_id: str
    variant: str
    commit: str
    parent: str
    work: pathlib.Path
    changed: tuple[Change, ...]
    all_tests: tuple[str, ...]
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]
    python: str
    seed_ms: int = 0
    peer_sizes: dict[str, int] = dataclasses.field(default_factory=dict)

    def changed_paths(self) -> tuple[str, ...]:
        return tuple(sorted(c.path for c in self.changed))


@runtime_checkable
class Strategy(Protocol):
    id: str
    needs_parent_state: bool

    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...


REGISTRY: dict[str, Strategy] = {}


def register(s: Strategy) -> Strategy:
    REGISTRY[s.id] = s
    return s


def get(strategy_id: str) -> Strategy:
    if strategy_id not in REGISTRY:
        raise KeyError(f"unknown strategy {strategy_id!r}; known: {', '.join(sorted(REGISTRY))}")
    return REGISTRY[strategy_id]


def all_ids() -> tuple[str, ...]:
    return tuple(sorted(REGISTRY))
```

- [ ] **10.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_base.py -q` → 3 passed.

- [ ] **10.5 Commit.** `git add bench/replay/strategies && git commit -m "bench: strategy protocol and registry"`

---

## Task 11 — Baseline 7: full suite (ground truth)

**Files:** `bench/replay/strategies/full.py`, `bench/tests/test_strategy_full.py`

**Interfaces:**

```python
# bench/replay/strategies/full.py
class FullSuite:
    id: str = "full"
    needs_parent_state: bool = False
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...
```

- [ ] **11.1 Failing test.**

```python
# bench/tests/test_strategy_full.py
import pathlib

from replay.strategies.base import CommitContext
from replay.strategies.full import FullSuite


def _ctx(all_tests):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def test_full_selects_everything_and_counts_as_an_escalation():
    s = FullSuite()
    sel = s.select(_ctx(("a::x", "b::y", "c::z")))
    assert sel.tests == ("a::x", "b::y", "c::z")
    assert sel.escalated is True
    assert sel.reason == "full suite"
```

Every cycle is counted including escalations (Global Constraints), and the full suite is
100% escalation by definition — publishing that as `1.00` keeps the escalation column
honest rather than special-casing the control.

- [ ] **11.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_full.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.full'`.

- [ ] **11.3 Implement.**

```python
# bench/replay/strategies/full.py
from __future__ import annotations

from replay.strategies.base import CommitContext, Selection, register


class FullSuite:
    id = "full"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        return Selection(tests=tuple(ctx.all_tests), escalated=True, reason="full suite")


register(FullSuite())
```

- [ ] **11.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_full.py -q` → 1 passed.

- [ ] **11.5 Commit.** `git add bench/replay/strategies/full.py bench/tests/test_strategy_full.py && git commit -m "bench: full-suite baseline"`

---

## Task 12 — Baseline 2: naive path heuristic

The load-bearing baseline. `tests/test_<module>.py` for a changed `src/<module>.py` is what
a developer would write in ten minutes with no map at all. **If RTDD does not clearly beat
this on change-level recall at equal or better selected-duration fraction, the map is not
justified and the project should say so rather than ship.** That sentence is pre-registered
here, before any number exists, and Task 24 prints the comparison as its own summary line.

**Files:** `bench/replay/strategies/pathheuristic.py`, `bench/tests/test_strategy_pathheuristic.py`

**Interfaces:**

```python
# bench/replay/strategies/pathheuristic.py
class PathHeuristic:
    id: str = "path"
    needs_parent_state: bool = False
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...

def candidate_test_files(module_stem: str, all_tests: Sequence[str]) -> set[str]: ...
def tests_in_files(files: set[str], all_tests: Sequence[str]) -> tuple[str, ...]: ...
```

Rule, stated exactly so it is not quietly improved into something less naive:
1. A changed path that is itself a test file contributes every collected test id in it.
2. A changed path `<anything>/<stem>.py` that is not a test file contributes every collected
   test id whose file basename is `test_<stem>.py` or `<stem>_test.py`, anywhere in the tree.
3. Nothing else. No package matching, no directory fallback, no import analysis.
4. A changed path that resolves to no test file contributes nothing — the heuristic under-selects,
   and that is the property being measured.

- [ ] **12.1 Failing tests.**

```python
# bench/tests/test_strategy_pathheuristic.py
import pathlib

from replay.gitwork import Change
from replay.strategies.base import CommitContext
from replay.strategies.pathheuristic import PathHeuristic, candidate_test_files

ALL = (
    "tests/test_alpha.py::test_add",
    "tests/test_beta.py::test_mul",
    "tests/test_gamma.py::test_sub",
)


def _ctx(changed):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=tuple(Change(p, s) for p, s in changed),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )


def test_candidate_matching_is_basename_only():
    assert candidate_test_files("alpha", ALL) == {"tests/test_alpha.py"}
    assert candidate_test_files("nothing", ALL) == set()


def test_source_change_maps_to_its_sibling_test_file():
    sel = PathHeuristic().select(_ctx([("src/alpha.py", "M")]))
    assert sel.tests == ("tests/test_alpha.py::test_add",)
    assert sel.escalated is False


def test_multi_file_change_unions():
    sel = PathHeuristic().select(_ctx([("src/beta.py", "M"), ("src/gamma.py", "M")]))
    assert sel.tests == ("tests/test_beta.py::test_mul", "tests/test_gamma.py::test_sub")


def test_changed_test_file_selects_itself():
    sel = PathHeuristic().select(_ctx([("tests/test_gamma.py", "A")]))
    assert sel.tests == ("tests/test_gamma.py::test_sub",)


def test_unmatched_change_selects_nothing_and_is_not_an_escalation():
    sel = PathHeuristic().select(_ctx([("src/orphan.py", "M")]))
    assert sel.tests == ()
    assert sel.escalated is False
    assert sel.reason == "no sibling test file"
```

- [ ] **12.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_pathheuristic.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.pathheuristic'`.

- [ ] **12.3 Implement.**

```python
# bench/replay/strategies/pathheuristic.py
from __future__ import annotations

import pathlib
import time
from collections.abc import Sequence

from replay.gitwork import is_test_path
from replay.strategies.base import CommitContext, Selection, register


def _file_of(test_id: str) -> str:
    return test_id.split("::", 1)[0]


def candidate_test_files(module_stem: str, all_tests: Sequence[str]) -> set[str]:
    wanted = {f"test_{module_stem}.py", f"{module_stem}_test.py"}
    return {f for f in {_file_of(t) for t in all_tests} if pathlib.PurePosixPath(f).name in wanted}


def tests_in_files(files: set[str], all_tests: Sequence[str]) -> tuple[str, ...]:
    return tuple(sorted(t for t in all_tests if _file_of(t) in files))


class PathHeuristic:
    id = "path"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        files: set[str] = set()
        known = {_file_of(t) for t in ctx.all_tests}
        for ch in ctx.changed:
            if not ch.path.endswith(".py"):
                continue
            if is_test_path(ch.path, ctx.test_globs):
                if ch.path in known:
                    files.add(ch.path)
                continue
            files |= candidate_test_files(pathlib.PurePosixPath(ch.path).stem, ctx.all_tests)
        tests = tests_in_files(files, ctx.all_tests)
        reason = "sibling test files" if tests else "no sibling test file"
        return Selection(
            tests=tests,
            escalated=False,
            reason=reason,
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(PathHeuristic())
```

- [ ] **12.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_pathheuristic.py -q` → 5 passed.

- [ ] **12.5 Commit.** `git add bench/replay/strategies/pathheuristic.py bench/tests/test_strategy_pathheuristic.py && git commit -m "bench: naive path-heuristic baseline"`

---

## Task 13 — Baseline 4: static import graph

The direct stand-in for TDAD's static dependency graph, and therefore the baseline that
carries the project's actual research question (spec §3: does dynamic coverage beat a static
graph?). It must be a *good-faith* static graph — transitive, package-aware — or the
comparison is rigged.

**Files:** `bench/replay/strategies/importgraph.py`, `bench/tests/test_strategy_importgraph.py`

**Interfaces:**

```python
# bench/replay/strategies/importgraph.py
def module_name(rel: str) -> str: ...                       # "src/alpha.py" -> "src.alpha"
def parse_imports(source: str) -> set[str]: ...             # absolute dotted names only
def build_graph(work: pathlib.Path) -> dict[str, set[str]]: ...   # module -> modules it imports
def reverse_reachable(graph: dict[str, set[str]], seeds: set[str]) -> set[str]: ...

class ImportGraph:
    id: str = "importgraph"
    needs_parent_state: bool = False
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...
```

Relative imports (`from . import x`) are resolved against the importing module's package.
A changed non-Python file has no module, so the graph cannot see it; the strategy selects
nothing for it, which is a real and reportable weakness of static analysis, not a bug.

- [ ] **13.1 Failing tests.**

```python
# bench/tests/test_strategy_importgraph.py
import subprocess

from replay.gitwork import Change
from replay.strategies.base import CommitContext
from replay.strategies.importgraph import (
    ImportGraph,
    build_graph,
    module_name,
    parse_imports,
    reverse_reachable,
)


def test_module_name():
    assert module_name("src/alpha.py") == "src.alpha"
    assert module_name("src/__init__.py") == "src"
    assert module_name("tests/test_alpha.py") == "tests.test_alpha"


def test_parse_imports_handles_both_forms():
    src = "import os\nfrom src.alpha import add\nfrom src import beta\n"
    assert parse_imports(src) == {"os", "src.alpha", "src.alpha.add", "src", "src.beta"}


def test_reverse_reachable_is_transitive():
    graph = {"tests.test_a": {"pkg.mid"}, "pkg.mid": {"pkg.low"}, "pkg.low": set()}
    assert reverse_reachable(graph, {"pkg.low"}) == {"pkg.low", "pkg.mid", "tests.test_a"}


def test_graph_over_the_synth_repo_selects_the_right_tests(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    graph = build_graph(synth.path)
    assert "src.alpha" in graph["tests.test_alpha"]

    ctx = CommitContext(
        repo_id="synth",
        variant="natural",
        commit=synth.sha(2),
        parent=synth.sha(1),
        work=synth.path,
        changed=(Change("src/beta.py", "M"), Change("src/gamma.py", "M")),
        all_tests=(
            "tests/test_alpha.py::test_add",
            "tests/test_beta.py::test_mul",
            "tests/test_gamma.py::test_sub",
        ),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )
    sel = ImportGraph().select(ctx)
    assert sel.tests == ("tests/test_beta.py::test_mul", "tests/test_gamma.py::test_sub")


def test_non_python_change_is_invisible_to_the_static_graph(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    ctx = CommitContext(
        repo_id="synth",
        variant="natural",
        commit=synth.sha(2),
        parent=synth.sha(1),
        work=synth.path,
        changed=(Change("data/fixtures.yaml", "M"),),
        all_tests=("tests/test_alpha.py::test_add",),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )
    sel = ImportGraph().select(ctx)
    assert sel.tests == ()
    assert sel.escalated is False
```

- [ ] **13.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_importgraph.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.importgraph'`.

- [ ] **13.3 Implement.**

```python
# bench/replay/strategies/importgraph.py
from __future__ import annotations

import ast
import pathlib
import time

from replay.strategies.base import CommitContext, Selection, register


def module_name(rel: str) -> str:
    p = pathlib.PurePosixPath(rel)
    parts = list(p.parts)
    if parts and parts[-1] == "__init__.py":
        parts = parts[:-1]
    else:
        parts[-1] = p.stem
    return ".".join(parts)


def parse_imports(source: str, package: str = "") -> set[str]:
    try:
        tree = ast.parse(source)
    except SyntaxError:
        return set()
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for a in node.names:
                names.add(a.name)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                base_parts = package.split(".") if package else []
                base_parts = base_parts[: len(base_parts) - (node.level - 1)] or []
                base = ".".join([*base_parts, node.module] if node.module else base_parts)
            else:
                base = node.module or ""
            if base:
                names.add(base)
                for a in node.names:
                    names.add(f"{base}.{a.name}")
    return names


def build_graph(work: pathlib.Path) -> dict[str, set[str]]:
    graph: dict[str, set[str]] = {}
    for path in sorted(work.rglob("*.py")):
        if any(part in (".git", ".venv", "build", "dist", "__pycache__") for part in path.parts):
            continue
        rel = path.relative_to(work).as_posix()
        mod = module_name(rel)
        package = mod.rsplit(".", 1)[0] if "." in mod else ""
        source = path.read_text(encoding="utf-8", errors="replace")
        graph[mod] = parse_imports(source, package)
    return graph


def reverse_reachable(graph: dict[str, set[str]], seeds: set[str]) -> set[str]:
    reverse: dict[str, set[str]] = {}
    for mod, deps in graph.items():
        for dep in deps:
            reverse.setdefault(dep, set()).add(mod)
    seen = set(seeds)
    stack = list(seeds)
    while stack:
        cur = stack.pop()
        for parent in reverse.get(cur, ()):
            if parent not in seen:
                seen.add(parent)
                stack.append(parent)
    return seen


class ImportGraph:
    id = "importgraph"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        graph = build_graph(ctx.work)
        seeds = {module_name(c.path) for c in ctx.changed if c.path.endswith(".py")}
        if not seeds:
            return Selection(
                tests=(),
                escalated=False,
                reason="no Python module in the changed set",
                select_ms=int(round((time.perf_counter() - start) * 1000)),
            )
        impacted = reverse_reachable(graph, seeds)
        tests = tuple(
            sorted(t for t in ctx.all_tests if module_name(t.split("::", 1)[0]) in impacted)
        )
        return Selection(
            tests=tests,
            escalated=False,
            reason="transitive static import closure",
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(ImportGraph())
```

- [ ] **13.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_importgraph.py -q` → 5 passed.

- [ ] **13.5 Commit.** `git add bench/replay/strategies/importgraph.py bench/tests/test_strategy_importgraph.py && git commit -m "bench: static import-graph baseline"`

---

## Task 14 — Baseline 3: `pytest --lf`

**Files:** `bench/replay/strategies/lastfailed.py`, `bench/tests/test_strategy_lastfailed.py`

**Interfaces:**

```python
# bench/replay/strategies/lastfailed.py
class LastFailed:
    id: str = "lf"
    needs_parent_state: bool = True     # a full run at the parent populates .pytest_cache
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...

def read_lastfailed(work: pathlib.Path) -> tuple[str, ...]: ...
```

`--lf` with an empty `lastfailed` cache runs the whole suite. That is real `--lf`
behaviour and it is scored as an **escalation**, not quietly dropped — which is exactly why
`--lf` is in the baseline set: it looks free until you count how often it degenerates.

- [ ] **14.1 Failing tests.**

```python
# bench/tests/test_strategy_lastfailed.py
import json
import pathlib
import subprocess

from replay.strategies.base import CommitContext
from replay.strategies.lastfailed import LastFailed, read_lastfailed


def _ctx(work, all_tests):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=work,
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def _write_lastfailed(work: pathlib.Path, entries: dict):
    d = work / ".pytest_cache" / "v" / "cache"
    d.mkdir(parents=True, exist_ok=True)
    (d / "lastfailed").write_text(json.dumps(entries), encoding="utf-8")


def test_read_lastfailed_returns_node_ids(synth):
    _write_lastfailed(synth.path, {"tests/test_beta.py::test_mul": True, "tests/test_beta.py": True})
    assert read_lastfailed(synth.path) == ("tests/test_beta.py::test_mul",)


def test_selects_only_previously_failing_tests(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(3)], cwd=synth.path, check=True)
    _write_lastfailed(synth.path, {"tests/test_beta.py::test_mul": True})
    sel = LastFailed().select(_ctx(synth.path, ("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul")))
    assert sel.tests == ("tests/test_beta.py::test_mul",)
    assert sel.escalated is False


def test_empty_cache_degenerates_to_the_full_suite_and_is_an_escalation(synth):
    all_tests = ("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul")
    sel = LastFailed().select(_ctx(synth.path, all_tests))
    assert sel.tests == all_tests
    assert sel.escalated is True
    assert "no recorded failures" in sel.reason
```

- [ ] **14.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_lastfailed.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.lastfailed'`.

- [ ] **14.3 Implement.**

```python
# bench/replay/strategies/lastfailed.py
from __future__ import annotations

import json
import pathlib
import time

from replay.runner import run_full
from replay.strategies.base import CommitContext, Selection, register


def read_lastfailed(work: pathlib.Path) -> tuple[str, ...]:
    p = work / ".pytest_cache" / "v" / "cache" / "lastfailed"
    if not p.exists():
        return ()
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return ()
    return tuple(sorted(k for k, v in data.items() if v and "::" in k))


class LastFailed:
    id = "lf"
    needs_parent_state = True

    def prepare(self, ctx: CommitContext) -> None:
        """Populate .pytest_cache by running the full suite in the clean parent tree.
        The harness calls this before materialising the child commit; the orchestrator
        caches the resulting .pytest_cache directory by (repo, parent)."""
        run_full(ctx.work, python=ctx.python)

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        failed = read_lastfailed(ctx.work)
        known = set(ctx.all_tests)
        selected = tuple(t for t in failed if t in known)
        elapsed = int(round((time.perf_counter() - start) * 1000))
        if not selected:
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="--lf with no recorded failures runs the full suite",
                select_ms=elapsed,
            )
        return Selection(tests=selected, escalated=False, reason="--lf", select_ms=elapsed)


register(LastFailed())
```

Note `prepare` runs the full suite *with* the cache provider, so the orchestrator must not
pass `-p no:cacheprovider` for this strategy's prepare step. Task 23 handles that by
calling `prepare` in a worktree where `runner.run_full` is invoked without the flag; add
`cacheprovider: bool = False` to `run_full`/`_invoke`/`_pytest_argv` and set it here:

```python
# bench/replay/runner.py — amend _pytest_argv and the two entry points
def _pytest_argv(instrumented, source_globs, log, xdist, cacheprovider=False):
    argv = ["-q", "--no-header", f"--report-log={log}"]
    if not cacheprovider:
        argv += ["-p", "no:cacheprovider"]
    ...
```

with `cacheprovider` threaded through `_invoke`, `run_full` and `run_subset` as a keyword
argument defaulting to `False`, and `LastFailed.prepare` calling
`run_full(ctx.work, python=ctx.python, cacheprovider=True)`.

- [ ] **14.4 Add the runner amendment and a test for it.**

```python
# append to bench/tests/test_runner.py
def test_cacheprovider_flag_populates_pytest_cache(synth):
    import subprocess

    subprocess.run(["git", "checkout", "-q", synth.sha(3)], cwd=synth.path, check=True)
    run_full(synth.path, cacheprovider=True)
    assert (synth.path / ".pytest_cache" / "v" / "cache" / "lastfailed").exists()
```

- [ ] **14.5 Run and pass.** `cd bench && uv run pytest tests/test_strategy_lastfailed.py tests/test_runner.py -q` → 3 + 6 passed.

- [ ] **14.6 Commit.** `git add bench/replay/strategies/lastfailed.py bench/replay/runner.py bench/tests/test_strategy_lastfailed.py bench/tests/test_runner.py && git commit -m "bench: pytest --lf baseline"`

---

## Task 15 — Baseline 1: pytest-testmon

Method-level AST checksums, finer-grained than RTDD v1's file-level map, shipping since
~2016 (audit A9). This is the first baseline a reviewer asks about and the one RTDD is most
likely to lose to on precision.

**Files:** `bench/replay/strategies/testmon.py`, `bench/tests/test_strategy_testmon.py`

**Interfaces:**

```python
# bench/replay/strategies/testmon.py
class Testmon:
    id: str = "testmon"
    needs_parent_state: bool = True     # .testmondata is built at the parent
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...

def testmon_collect(work: pathlib.Path, python: str) -> tuple[tuple[str, ...], str]: ...
```

Method: `prepare` runs `pytest --testmon` in the clean parent worktree, producing
`.testmondata`. After materialisation, `select` runs
`pytest --testmon --collect-only -q` — testmon deselects at collection, so the collected
list *is* the selection, obtained without executing anything. If `.testmondata` is missing
or testmon reports the database is unusable, testmon runs everything and that is scored as
an escalation.

- [ ] **15.1 Failing tests.**

```python
# bench/tests/test_strategy_testmon.py
import subprocess
import sys

from replay.gitwork import Change
from replay.strategies.base import CommitContext
from replay.strategies.testmon import Testmon

ALL = (
    "tests/test_alpha.py::test_add",
    "tests/test_beta.py::test_mul",
    "tests/test_gamma.py::test_sub",
)


def _ctx(work, changed):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=work,
        changed=tuple(Change(p, s) for p, s in changed),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python=sys.executable,
    )


def test_testmon_selects_only_the_impacted_test_after_a_source_edit(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    s = Testmon()
    s.prepare(_ctx(synth.path, []))
    assert (synth.path / ".testmondata").exists()

    (synth.path / "src" / "beta.py").write_text("def mul(a, b):\n    return a * b + 1\n", encoding="utf-8")
    sel = s.select(_ctx(synth.path, [("src/beta.py", "M")]))
    assert "tests/test_beta.py::test_mul" in sel.tests
    assert "tests/test_alpha.py::test_add" not in sel.tests
    assert sel.escalated is False


def test_missing_database_is_an_escalation(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    db = synth.path / ".testmondata"
    if db.exists():
        db.unlink()
    sel = Testmon().select(_ctx(synth.path, [("src/beta.py", "M")]))
    assert sel.escalated is True
    assert sel.tests == ALL
```

- [ ] **15.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_testmon.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.testmon'`.

- [ ] **15.3 Implement.**

```python
# bench/replay/strategies/testmon.py
from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import time

from replay.strategies.base import CommitContext, Selection, register


def _env() -> dict[str, str]:
    env = dict(os.environ)
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    env["TESTMON_DATAFILE"] = ".testmondata"
    return env


def testmon_collect(work: pathlib.Path, python: str) -> tuple[tuple[str, ...], str]:
    proc = subprocess.run(
        [python, "-m", "pytest", "--testmon", "--collect-only", "-q", "--no-header"],
        cwd=work,
        capture_output=True,
        text=True,
        env=_env(),
    )
    ids: list[str] = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if "::" in line and not line.startswith(("=", "-", "<")):
            ids.append(line)
    return tuple(sorted(set(ids))), proc.stdout + proc.stderr


class Testmon:
    id = "testmon"
    needs_parent_state = True

    def prepare(self, ctx: CommitContext) -> None:
        subprocess.run(
            [ctx.python or sys.executable, "-m", "pytest", "--testmon", "-q", "--no-header"],
            cwd=ctx.work,
            capture_output=True,
            text=True,
            env=_env(),
        )

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        if not (ctx.work / ".testmondata").exists():
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="no .testmondata; testmon runs everything",
                select_ms=int(round((time.perf_counter() - start) * 1000)),
            )
        ids, log = testmon_collect(ctx.work, ctx.python or sys.executable)
        elapsed = int(round((time.perf_counter() - start) * 1000))
        known = set(ctx.all_tests)
        selected = tuple(t for t in ids if t in known)
        if "cannot be read" in log or "new environment" in log.lower():
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="testmon invalidated its database (environment change)",
                select_ms=elapsed,
            )
        return Selection(
            tests=selected,
            escalated=len(selected) == len(known) and len(known) > 0,
            reason="testmon method-level checksums",
            select_ms=elapsed,
        )


register(Testmon())
```

- [ ] **15.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_testmon.py -q` → 2 passed.

- [ ] **15.5 Commit.** `git add bench/replay/strategies/testmon.py bench/tests/test_strategy_testmon.py && git commit -m "bench: pytest-testmon baseline"`

---

## Task 16 — Baseline 5: `pytest -n auto` (xdist)

The intervention a real team actually reaches for. It selects nothing and changes only
execution, so it exists to occupy the wall-clock column: if `-n auto` on the full suite is
faster than RTDD's instrumented subset, that is the finding and the results table must show
it rather than compare RTDD only against a serial full run.

**Files:** `bench/replay/strategies/xdist.py`, `bench/tests/test_strategy_xdist.py`

**Interfaces:**

```python
# bench/replay/strategies/xdist.py
class Xdist:
    id: str = "xdist"
    needs_parent_state: bool = False
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...
```

- [ ] **16.1 Failing test.**

```python
# bench/tests/test_strategy_xdist.py
import pathlib

from replay.strategies.base import CommitContext
from replay.strategies.xdist import Xdist


def _ctx(all_tests):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def test_xdist_selects_everything_but_carries_parallel_exec_args():
    sel = Xdist().select(_ctx(("a::x", "b::y")))
    assert sel.tests == ("a::x", "b::y")
    assert sel.escalated is True
    assert sel.exec_args == ("-n", "auto")
    assert sel.reason == "no selection; parallel execution"
```

- [ ] **16.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_xdist.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.xdist'`.

- [ ] **16.3 Implement.**

```python
# bench/replay/strategies/xdist.py
from __future__ import annotations

from replay.strategies.base import CommitContext, Selection, register


class Xdist:
    id = "xdist"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        return Selection(
            tests=tuple(ctx.all_tests),
            escalated=True,
            reason="no selection; parallel execution",
            exec_args=("-n", "auto"),
        )


register(Xdist())
```

- [ ] **16.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_xdist.py -q` → 1 passed.

- [ ] **16.5 Commit.** `git add bench/replay/strategies/xdist.py bench/tests/test_strategy_xdist.py && git commit -m "bench: pytest -n auto baseline"`

---

## Task 17 — Baseline 6: random selection at equal ratio

The control that shows selection beats chance. It samples exactly as many tests as RTDD
selected at the same commit, from a generator seeded by `(random_seed, repo_id, commit,
variant)` so the whole benchmark is reproducible from the published config.

**Files:** `bench/replay/strategies/randomratio.py`, `bench/tests/test_strategy_randomratio.py`

**Interfaces:**

```python
# bench/replay/strategies/randomratio.py
class RandomRatio:
    id: str = "random"
    needs_parent_state: bool = False
    def __init__(self, seed: int = 0, peer: str = "rtdd") -> None: ...
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...

def seeded_rng(seed: int, *parts: str) -> random.Random: ...
```

If `ctx.peer_sizes` has no entry for `rtdd`, that is a harness ordering bug, not a
degenerate case — raise, do not silently sample zero.

- [ ] **17.1 Failing tests.**

```python
# bench/tests/test_strategy_randomratio.py
import pathlib

import pytest

from replay.strategies.base import CommitContext
from replay.strategies.randomratio import RandomRatio, seeded_rng

ALL = tuple(f"tests/test_m.py::test_{i:02d}" for i in range(20))


def _ctx(peer_sizes, commit="c1"):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit=commit,
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
        peer_sizes=peer_sizes,
    )


def test_matches_the_peer_selection_size():
    sel = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    assert len(sel.tests) == 5
    assert set(sel.tests) <= set(ALL)
    assert sel.escalated is False


def test_is_deterministic_for_a_given_seed_and_commit():
    a = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    b = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    assert a.tests == b.tests
    c = RandomRatio(seed=7).select(_ctx({"rtdd": 5}, commit="c2"))
    assert c.tests != a.tests


def test_missing_peer_size_is_a_harness_error():
    with pytest.raises(RuntimeError, match="peer_sizes"):
        RandomRatio(seed=7).select(_ctx({}))


def test_seeded_rng_is_stable():
    assert seeded_rng(1, "a", "b").random() == seeded_rng(1, "a", "b").random()
    assert seeded_rng(1, "a", "b").random() != seeded_rng(1, "a", "c").random()
```

- [ ] **17.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_randomratio.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.randomratio'`.

- [ ] **17.3 Implement.**

```python
# bench/replay/strategies/randomratio.py
from __future__ import annotations

import hashlib
import random
import time

from replay.strategies.base import CommitContext, Selection, register


def seeded_rng(seed: int, *parts: str) -> random.Random:
    raw = "\0".join((str(seed), *parts)).encode("utf-8")
    return random.Random(int.from_bytes(hashlib.sha256(raw).digest()[:8], "big"))


class RandomRatio:
    id = "random"
    needs_parent_state = False

    def __init__(self, seed: int = 0, peer: str = "rtdd") -> None:
        self.seed = seed
        self.peer = peer

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        if self.peer not in ctx.peer_sizes:
            raise RuntimeError(
                f"peer_sizes has no entry for {self.peer!r}; the orchestrator must run "
                f"{self.peer!r} before 'random' so the control matches its selection size"
            )
        k = min(ctx.peer_sizes[self.peer], len(ctx.all_tests))
        rng = seeded_rng(self.seed, ctx.repo_id, ctx.variant, ctx.commit)
        picked = tuple(sorted(rng.sample(list(ctx.all_tests), k))) if k else ()
        return Selection(
            tests=picked,
            escalated=False,
            reason=f"uniform sample at {self.peer}'s selection size",
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(RandomRatio())
```

The orchestrator replaces the registry entry with `RandomRatio(seed=cfg.random_seed)` so the
published `random_seed` is the one actually used.

- [ ] **17.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_randomratio.py -q` → 4 passed.

- [ ] **17.5 Commit.** `git add bench/replay/strategies/randomratio.py bench/tests/test_strategy_randomratio.py && git commit -m "bench: random-at-equal-ratio control"`

---

## Task 18 — The system under test: RTDD

**Files:** `bench/replay/strategies/rtdd.py`, `bench/tests/test_strategy_rtdd.py`

**Interfaces:**

```python
# bench/replay/strategies/rtdd.py
class Rtdd:
    id: str = "rtdd"
    needs_parent_state: bool = True     # `rtdd seed` runs in the clean parent tree
    def __init__(self, binary: str = "rtdd") -> None: ...
    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...
```

Shipped defaults only: `prepare` is `rtdd seed`, `select` is `rtdd which --base HEAD --json`.
No flags beyond those, ever — a tuned RTDD against untuned baselines is not a result.

- [ ] **18.1 Failing tests** (parser-level, so they run without the binary; the binary-level
contract is Task 9's `skipif` test).

```python
# bench/tests/test_strategy_rtdd.py
import pathlib

from replay.rtddio import WhichResult
from replay.strategies.base import CommitContext
from replay.strategies.rtdd import Rtdd


def _ctx():
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul"),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def test_select_maps_which_output_to_a_selection(monkeypatch):
    s = Rtdd()
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            tier="T0",
            reason="",
            tests=("tests/test_alpha.py::test_add",),
            direct=(),
            changed=("src/alpha.py",),
            cycles=2,
            wall_ms=11,
        ),
    )
    sel = s.select(_ctx())
    assert sel.tests == ("tests/test_alpha.py::test_add",)
    assert sel.escalated is False
    assert sel.reason == "T0"
    assert sel.select_ms == 11


def test_t2_is_reported_as_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            tier="T2",
            reason="pyproject.toml changed",
            tests=("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul"),
            direct=(),
            changed=("pyproject.toml",),
            cycles=2,
            wall_ms=4,
        ),
    )
    sel = Rtdd().select(_ctx())
    assert sel.escalated is True
    assert sel.reason == "T2: pyproject.toml changed"


def test_empty_tier_is_reported_and_is_not_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            tier="empty", reason="", tests=(), direct=(), changed=(), cycles=1, wall_ms=2
        ),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == ()
    assert sel.escalated is False
    assert sel.reason == "empty"
```

- [ ] **18.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_strategy_rtdd.py -q` → `ModuleNotFoundError: No module named 'replay.strategies.rtdd'`.

- [ ] **18.3 Implement.**

```python
# bench/replay/strategies/rtdd.py
from __future__ import annotations

from replay import rtddio
from replay.strategies.base import CommitContext, Selection, register


class Rtdd:
    id = "rtdd"
    needs_parent_state = True

    def __init__(self, binary: str = "rtdd") -> None:
        self.binary = binary

    def prepare(self, ctx: CommitContext) -> None:
        rtddio.seed(ctx.work, binary=self.binary)

    def select(self, ctx: CommitContext) -> Selection:
        w = rtddio.which(ctx.work, binary=self.binary, base="HEAD")
        reason = w.tier if not w.reason else f"{w.tier}: {w.reason}"
        return Selection(
            tests=w.tests,
            escalated=w.escalated(),
            reason=reason,
            select_ms=w.wall_ms,
        )


register(Rtdd())
```

- [ ] **18.4 Run and pass.** `cd bench && uv run pytest tests/test_strategy_rtdd.py -q` → 3 passed.

- [ ] **18.5 Verify all seven baselines are registered.**

```python
# bench/tests/test_registry_complete.py
import replay.strategies.full  # noqa: F401
import replay.strategies.importgraph  # noqa: F401
import replay.strategies.lastfailed  # noqa: F401
import replay.strategies.pathheuristic  # noqa: F401
import replay.strategies.randomratio  # noqa: F401
import replay.strategies.rtdd  # noqa: F401
import replay.strategies.testmon  # noqa: F401
import replay.strategies.xdist  # noqa: F401
from replay.strategies.base import all_ids


def test_every_required_baseline_is_registered():
    assert set(all_ids()) >= {
        "full",
        "importgraph",
        "lf",
        "path",
        "random",
        "rtdd",
        "testmon",
        "xdist",
    }
```

- [ ] **18.6 Run and pass.** `cd bench && uv run pytest tests/test_registry_complete.py -q` → 1 passed.

- [ ] **18.7 Commit.** `git add bench/replay/strategies/rtdd.py bench/tests/test_strategy_rtdd.py bench/tests/test_registry_complete.py && git commit -m "bench: rtdd strategy and complete-registry guard"`

---

## Task 19 — Records

The four dataclasses that flow orchestrator → metrics → report. They are the diffable line
format in `commits.jsonl`, so their field names are the published schema.

**Files:** `bench/replay/records.py`, `bench/tests/test_records.py`

**Interfaces:**

```python
# bench/replay/records.py
@dataclasses.dataclass(frozen=True)
class CommitRecord:
    repo_id: str
    commit: str
    parent: str
    variant: str
    all_tests: tuple[str, ...]
    durations_ms: dict[str, int]
    f_full: tuple[str, ...]                 # newly failing vs the clean parent/child tree
    pre_existing_failures: tuple[str, ...]
    changed: tuple[str, ...]
    def stratum(self) -> str: ...
    def total_duration_ms(self) -> int: ...
    def to_dict(self) -> dict: ...

@dataclasses.dataclass(frozen=True)
class StrategyRecord:
    repo_id: str
    commit: str
    variant: str
    strategy: str
    selected: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int
    def to_dict(self) -> dict: ...

@dataclasses.dataclass(frozen=True)
class WallClockRecord:
    repo_id: str
    commit: str
    variant: str
    strategy: str
    hardware_fingerprint: str
    full_uninstrumented_ms: int | None
    subset_instrumented_ms: int | None
    subset_uninstrumented_ms: int | None
    isolation_violation: bool
    def to_dict(self) -> dict: ...

@dataclasses.dataclass(frozen=True)
class UncoveredRecord:
    repo_id: str
    commit: str
    variant: str
    reported: tuple[tuple[str, int], ...]
    truth_covered_hits: int
    reported_count: int
    def to_dict(self) -> dict: ...

STRATA: tuple[str, ...] = ("1", "2-5", "6-20", "21+")
def stratum_of(n: int) -> str: ...
def to_jsonl_lines(records: Sequence[object]) -> list[str]: ...
```

- [ ] **19.1 Failing tests.**

```python
# bench/tests/test_records.py
from replay.records import CommitRecord, StrategyRecord, stratum_of, to_jsonl_lines


def test_stratum_boundaries():
    assert stratum_of(1) == "1"
    assert stratum_of(2) == "2-5"
    assert stratum_of(5) == "2-5"
    assert stratum_of(6) == "6-20"
    assert stratum_of(20) == "6-20"
    assert stratum_of(21) == "21+"
    assert stratum_of(0) == "0"


def test_commit_record_stratum_and_duration():
    rec = CommitRecord(
        repo_id="synth",
        commit="c1",
        parent="c0",
        variant="natural",
        all_tests=("a::x", "b::y"),
        durations_ms={"a::x": 10, "b::y": 30},
        f_full=("a::x",),
        pre_existing_failures=(),
        changed=("src/alpha.py",),
    )
    assert rec.stratum() == "1"
    assert rec.total_duration_ms() == 40


def test_jsonl_is_sorted_and_newline_terminated():
    recs = [
        StrategyRecord("synth", "c2", "natural", "rtdd", ("a::x",), False, "T0", 3),
        StrategyRecord("synth", "c1", "natural", "path", ("b::y",), False, "sib", 1),
    ]
    lines = to_jsonl_lines(recs)
    assert len(lines) == 2
    assert '"commit":"c1"' in lines[0]
    assert all(line.endswith("\n") for line in lines)
```

- [ ] **19.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_records.py -q` → `ModuleNotFoundError: No module named 'replay.records'`.

- [ ] **19.3 Implement.**

```python
# bench/replay/records.py
from __future__ import annotations

import dataclasses
import json
from collections.abc import Sequence

STRATA = ("1", "2-5", "6-20", "21+")


def stratum_of(n: int) -> str:
    if n <= 0:
        return "0"
    if n == 1:
        return "1"
    if n <= 5:
        return "2-5"
    if n <= 20:
        return "6-20"
    return "21+"


@dataclasses.dataclass(frozen=True)
class CommitRecord:
    repo_id: str
    commit: str
    parent: str
    variant: str
    all_tests: tuple[str, ...]
    durations_ms: dict[str, int]
    f_full: tuple[str, ...]
    pre_existing_failures: tuple[str, ...]
    changed: tuple[str, ...]

    def stratum(self) -> str:
        return stratum_of(len(self.f_full))

    def total_duration_ms(self) -> int:
        return sum(self.durations_ms.get(t, 0) for t in self.all_tests)

    def to_dict(self) -> dict:
        return {
            "kind": "commit",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "parent": self.parent,
            "variant": self.variant,
            "n_tests": len(self.all_tests),
            "all_tests": list(self.all_tests),
            "durations_ms": dict(sorted(self.durations_ms.items())),
            "f_full": list(self.f_full),
            "pre_existing_failures": list(self.pre_existing_failures),
            "changed": list(self.changed),
            "stratum": self.stratum(),
        }


@dataclasses.dataclass(frozen=True)
class StrategyRecord:
    repo_id: str
    commit: str
    variant: str
    strategy: str
    selected: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int

    def to_dict(self) -> dict:
        return {
            "kind": "strategy",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "strategy": self.strategy,
            "selected": list(self.selected),
            "n_selected": len(self.selected),
            "escalated": self.escalated,
            "reason": self.reason,
            "select_ms": self.select_ms,
        }


@dataclasses.dataclass(frozen=True)
class WallClockRecord:
    repo_id: str
    commit: str
    variant: str
    strategy: str
    hardware_fingerprint: str
    full_uninstrumented_ms: int | None
    subset_instrumented_ms: int | None
    subset_uninstrumented_ms: int | None
    isolation_violation: bool

    def to_dict(self) -> dict:
        return {
            "kind": "wallclock",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "strategy": self.strategy,
            "hardware_fingerprint": self.hardware_fingerprint,
            "full_uninstrumented_ms": self.full_uninstrumented_ms,
            "subset_instrumented_ms": self.subset_instrumented_ms,
            "subset_uninstrumented_ms": self.subset_uninstrumented_ms,
            "isolation_violation": self.isolation_violation,
        }


@dataclasses.dataclass(frozen=True)
class UncoveredRecord:
    repo_id: str
    commit: str
    variant: str
    reported: tuple[tuple[str, int], ...]
    truth_covered_hits: int
    reported_count: int

    def to_dict(self) -> dict:
        return {
            "kind": "uncovered",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "reported": [[p, n] for p, n in self.reported],
            "truth_covered_hits": self.truth_covered_hits,
            "reported_count": self.reported_count,
        }


def to_jsonl_lines(records: Sequence[object]) -> list[str]:
    dicts = [r.to_dict() for r in records]  # type: ignore[attr-defined]
    dicts.sort(key=lambda d: (d["repo_id"], d["commit"], d["variant"], d["kind"], d.get("strategy", "")))
    return [json.dumps(d, sort_keys=True, separators=(",", ":")) + "\n" for d in dicts]
```

- [ ] **19.4 Run and pass.** `cd bench && uv run pytest tests/test_records.py -q` → 3 passed.

- [ ] **19.5 Commit.** `git add bench/replay/records.py bench/tests/test_records.py && git commit -m "bench: record schema for the diffable results format"`

---

## Task 20 — Metrics

Every function here takes records from **one repo** and asserts it. Cross-repo aggregation
lives in Task 24 and is duration-weighted and labelled.

**Files:** `bench/replay/metrics.py`, `bench/tests/test_metrics.py`

**Definitions, fixed here:**

- `detected(rec, sr)` ⟺ `set(sr.selected) & set(rec.f_full) ≠ ∅` — an unrelated flaky
  failure inside the selection can never satisfy this, because `f_full` already excludes
  pre-existing failures and the intersection must be non-empty.
- `change_level_recall` = detecting commits with `detected` / detecting commits.
- `test_level_recall` (micro) = `Σ|selected ∩ f_full| / Σ|f_full|`; macro = mean of the
  per-commit ratio. **Both are published**; audit A8 killed publishing only the flattering one.
- `selection_ratio` = `|selected| / |all_tests|`.
- `selected_duration_fraction` = `Σ d[t] for t in selected` / `Σ d[t] for all tests`.
- `escalation_rate` = escalated records / all records, over **every** cycle including
  escalations.
- Strata: `1`, `2-5`, `6-20`, `21+` over `|f_full|`, with `1` always reported first and
  never merged into a pooled figure.

**Interfaces:**

```python
# bench/replay/metrics.py
class PoolingError(RuntimeError): ...

@dataclasses.dataclass(frozen=True)
class Ratio:
    num: float
    den: float
    def value(self) -> float | None: ...      # None when den == 0
    def to_dict(self) -> dict: ...

def assert_single_repo(records: Sequence) -> str: ...
def detected(rec: CommitRecord, sr: StrategyRecord) -> bool: ...
def pair_up(commits, strategy_records, strategy) -> list[tuple[CommitRecord, StrategyRecord]]: ...
def change_level_recall(pairs) -> Ratio: ...
def test_level_recall_micro(pairs) -> Ratio: ...
def test_level_recall_macro(pairs) -> Ratio: ...
def by_stratum(pairs) -> dict[str, list[tuple[CommitRecord, StrategyRecord]]]: ...
def selection_ratio(pairs) -> Ratio: ...
def selected_duration_fraction(pairs) -> Ratio: ...
def escalation_rate(strategy_records) -> Ratio: ...
def summarise(commits, strategy_records, strategy) -> dict: ...
```

- [ ] **20.1 Failing tests with a hand-computed answer.**

The fixture below is small enough to check by hand and is the regression anchor for every
later change to the metric code.

| commit | `f_full` | stratum | rtdd selected | ∩ | `|f_full|` |
|---|---|---|---|---|---|
| `c1` | `{a}` | 1 | `{a, b}` | 1 | 1 |
| `c2` | `{a, b, c}` | 2-5 | `{b, c}` | 2 | 3 |
| `c3` | `{d}` | 1 | `{a}` | 0 | 1 |
| `c4` | `{}` | 0 (green) | `{a}` | — | — |

Change-level recall over detecting commits: `c1` hit, `c2` hit, `c3` miss → **2/3**.
Test-level micro: `(1 + 2 + 0) / (1 + 3 + 1)` = **3/5**.
Test-level macro: `(1/1 + 2/3 + 0/1) / 3` = **5/9**.
Stratum `1`: change-level `1/2`, micro `1/2`.
Stratum `2-5`: change-level `1/1`, micro `2/3`.

```python
# bench/tests/test_metrics.py
import pytest

from replay.metrics import (
    PoolingError,
    assert_single_repo,
    by_stratum,
    change_level_recall,
    escalation_rate,
    pair_up,
    selected_duration_fraction,
    selection_ratio,
    test_level_recall_macro,
    test_level_recall_micro,
)
from replay.records import CommitRecord, StrategyRecord

ALL = ("a", "b", "c", "d")
DUR = {"a": 100, "b": 100, "c": 100, "d": 700}


def _commit(cid, f_full):
    return CommitRecord(
        repo_id="synth",
        commit=cid,
        parent="p",
        variant="natural",
        all_tests=ALL,
        durations_ms=DUR,
        f_full=tuple(f_full),
        pre_existing_failures=(),
        changed=("src/x.py",),
    )


def _sel(cid, selected, escalated=False):
    return StrategyRecord("synth", cid, "natural", "rtdd", tuple(selected), escalated, "T0", 1)


COMMITS = [_commit("c1", "a"), _commit("c2", "abc"), _commit("c3", "d"), _commit("c4", "")]
SELS = [
    _sel("c1", "ab"),
    _sel("c2", "bc"),
    _sel("c3", "a"),
    _sel("c4", "a"),
]


def test_pooling_across_repos_is_refused():
    other = CommitRecord("otherrepo", "c9", "p", "natural", ALL, DUR, (), (), ())
    with pytest.raises(PoolingError):
        assert_single_repo([*COMMITS, other])


def test_change_and_test_level_recall_match_the_hand_computation():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    cl = change_level_recall(pairs)
    assert (cl.num, cl.den) == (2, 3)
    micro = test_level_recall_micro(pairs)
    assert (micro.num, micro.den) == (3, 5)
    macro = test_level_recall_macro(pairs)
    assert macro.value() == pytest.approx(5 / 9)


def test_single_killer_stratum_is_broken_out():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    strata = by_stratum(pairs)
    assert set(strata) == {"1", "2-5"}
    one = change_level_recall(strata["1"])
    assert (one.num, one.den) == (1, 2)
    assert test_level_recall_micro(strata["1"]).value() == pytest.approx(0.5)
    assert change_level_recall(strata["2-5"]).value() == pytest.approx(1.0)


def test_selection_ratio_and_duration_fraction_differ_on_a_heavy_tail():
    # every commit selects 2 of 4 tests, but 'd' alone is 70% of the suite's duration
    pairs = pair_up(COMMITS, SELS, "rtdd")
    assert selection_ratio(pairs).value() == pytest.approx((2 + 2 + 1 + 1) / (4 * 4))
    # selected duration: c1 200, c2 200, c3 100, c4 100 out of 1000 each
    assert selected_duration_fraction(pairs).value() == pytest.approx(600 / 4000)


def test_escalation_rate_counts_every_cycle():
    sels = [_sel("c1", "abcd", escalated=True), _sel("c2", "b"), _sel("c3", "a"), _sel("c4", "a")]
    assert escalation_rate(sels).value() == pytest.approx(0.25)
```

- [ ] **20.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_metrics.py -q` → `ModuleNotFoundError: No module named 'replay.metrics'`.

- [ ] **20.3 Implement.**

```python
# bench/replay/metrics.py
from __future__ import annotations

import dataclasses
from collections.abc import Sequence

from replay.records import STRATA, CommitRecord, StrategyRecord


class PoolingError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class Ratio:
    num: float
    den: float

    def value(self) -> float | None:
        return None if self.den == 0 else self.num / self.den

    def to_dict(self) -> dict:
        return {"num": self.num, "den": self.den, "value": self.value()}


def assert_single_repo(records: Sequence) -> str:
    ids = {r.repo_id for r in records}
    if len(ids) > 1:
        raise PoolingError(
            f"refusing to pool across repos: {sorted(ids)}. Spec §10 forbids pooled "
            f"recall; use the duration-weighted aggregate instead."
        )
    return next(iter(ids)) if ids else ""


def detected(rec: CommitRecord, sr: StrategyRecord) -> bool:
    return bool(set(sr.selected) & set(rec.f_full))


def pair_up(
    commits: Sequence[CommitRecord],
    strategy_records: Sequence[StrategyRecord],
    strategy: str,
) -> list[tuple[CommitRecord, StrategyRecord]]:
    assert_single_repo([*commits, *strategy_records])
    index = {
        (s.commit, s.variant): s for s in strategy_records if s.strategy == strategy
    }
    pairs = []
    for c in commits:
        s = index.get((c.commit, c.variant))
        if s is not None:
            pairs.append((c, s))
    return pairs


def _detecting(pairs):
    return [(c, s) for c, s in pairs if c.f_full]


def change_level_recall(pairs) -> Ratio:
    det = _detecting(pairs)
    return Ratio(num=sum(1 for c, s in det if detected(c, s)), den=len(det))


def test_level_recall_micro(pairs) -> Ratio:
    det = _detecting(pairs)
    num = sum(len(set(s.selected) & set(c.f_full)) for c, s in det)
    den = sum(len(c.f_full) for c, s in det)
    return Ratio(num=num, den=den)


def test_level_recall_macro(pairs) -> Ratio:
    det = _detecting(pairs)
    total = sum(len(set(s.selected) & set(c.f_full)) / len(c.f_full) for c, s in det)
    return Ratio(num=total, den=len(det))


def by_stratum(pairs) -> dict[str, list]:
    out: dict[str, list] = {}
    for c, s in _detecting(pairs):
        out.setdefault(c.stratum(), []).append((c, s))
    return {k: out[k] for k in STRATA if k in out}


def selection_ratio(pairs) -> Ratio:
    num = sum(len(s.selected) for c, s in pairs)
    den = sum(len(c.all_tests) for c, s in pairs)
    return Ratio(num=num, den=den)


def selected_duration_fraction(pairs) -> Ratio:
    num = sum(sum(c.durations_ms.get(t, 0) for t in s.selected) for c, s in pairs)
    den = sum(c.total_duration_ms() for c, s in pairs)
    return Ratio(num=num, den=den)


def escalation_rate(strategy_records: Sequence[StrategyRecord]) -> Ratio:
    return Ratio(num=sum(1 for s in strategy_records if s.escalated), den=len(strategy_records))


def summarise(
    commits: Sequence[CommitRecord],
    strategy_records: Sequence[StrategyRecord],
    strategy: str,
) -> dict:
    pairs = pair_up(commits, strategy_records, strategy)
    mine = [s for s in strategy_records if s.strategy == strategy]
    strata = by_stratum(pairs)
    return {
        "strategy": strategy,
        "cycles": len(pairs),
        "detecting_commits": len(_detecting(pairs)),
        "green_commits": len(pairs) - len(_detecting(pairs)),
        "change_level_recall": change_level_recall(pairs).to_dict(),
        "test_level_recall_micro": test_level_recall_micro(pairs).to_dict(),
        "test_level_recall_macro": test_level_recall_macro(pairs).to_dict(),
        "selection_ratio": selection_ratio(pairs).to_dict(),
        "selected_duration_fraction": selected_duration_fraction(pairs).to_dict(),
        "escalation_rate": escalation_rate(mine).to_dict(),
        "strata": {
            k: {
                "n": len(v),
                "change_level_recall": change_level_recall(v).to_dict(),
                "test_level_recall_micro": test_level_recall_micro(v).to_dict(),
            }
            for k, v in strata.items()
        },
    }
```

- [ ] **20.4 Run and pass.** `cd bench && uv run pytest tests/test_metrics.py -q` → 5 passed.

- [ ] **20.5 Commit.** `git add bench/replay/metrics.py bench/tests/test_metrics.py && git commit -m "bench: recall, strata, ratio and escalation metrics"`

---

## Task 21 — False-signal rate for the uncovered report

Spec §10: *how often the uncovered report fires on a change that is in fact adequately
tested.* Given §6's import-time class this is the number that decides whether anyone leaves
the feature on, and v1 never measured it.

**Operational definition, fixed here:**
- `reported` = the `(path, line)` pairs `rtdd run --json` classed **Uncovered** (never
  `import_time`, which RTDD reports separately by design).
- `truth` = the `(path, line)` pairs covered by **at least one test context** in the full
  instrumented run at the same tree — `CoverageTruth.covered`, which excludes the empty
  context, so a line that only ran at import is not counted as tested.
- A **line-level false signal** is a reported pair that is in `truth`.
- A **change-level false signal** is a commit where `reported ≠ ∅` and **every** reported
  pair is in `truth` — the report fired and was wholly wrong.
- The rate denominators are commits where `reported ≠ ∅`; commits with an empty report are
  counted separately as `silent`.

**Files:** `bench/replay/falsesignal.py`, `bench/tests/test_falsesignal.py`

**Interfaces:**

```python
# bench/replay/falsesignal.py
def build_record(repo_id, commit, variant, reported, truth) -> UncoveredRecord: ...
def change_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio: ...
def line_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio: ...
def fire_rate(records: Sequence[UncoveredRecord], total_cycles: int) -> Ratio: ...
def summarise(records: Sequence[UncoveredRecord], total_cycles: int) -> dict: ...
```

- [ ] **21.1 Failing tests.**

```python
# bench/tests/test_falsesignal.py
import pytest

from replay.falsesignal import (
    build_record,
    change_false_signal_rate,
    fire_rate,
    line_false_signal_rate,
)


def _rec(cid, reported, truth):
    return build_record("synth", cid, "natural", frozenset(reported), frozenset(truth))


def test_record_counts_hits_against_truth():
    r = _rec("c1", {("src/a.py", 1), ("src/a.py", 2)}, {("src/a.py", 1)})
    assert r.reported_count == 2
    assert r.truth_covered_hits == 1


def test_change_level_false_signal_requires_every_reported_line_to_be_tested():
    wholly_wrong = _rec("c1", {("src/a.py", 1)}, {("src/a.py", 1)})
    partly_right = _rec("c2", {("src/a.py", 1), ("src/a.py", 2)}, {("src/a.py", 1)})
    right = _rec("c3", {("src/a.py", 9)}, {("src/a.py", 1)})
    rate = change_false_signal_rate([wholly_wrong, partly_right, right])
    assert (rate.num, rate.den) == (1, 3)


def test_line_level_false_signal_rate():
    a = _rec("c1", {("src/a.py", 1), ("src/a.py", 2)}, {("src/a.py", 1)})
    b = _rec("c2", {("src/b.py", 5)}, set())
    rate = line_false_signal_rate([a, b])
    assert (rate.num, rate.den) == (1, 3)


def test_silent_commits_are_excluded_from_the_rate_but_counted_in_fire_rate():
    silent = _rec("c1", set(), {("src/a.py", 1)})
    fired = _rec("c2", {("src/a.py", 3)}, set())
    assert change_false_signal_rate([silent, fired]).den == 1
    assert fire_rate([silent, fired], total_cycles=10).value() == pytest.approx(0.1)
```

- [ ] **21.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_falsesignal.py -q` → `ModuleNotFoundError: No module named 'replay.falsesignal'`.

- [ ] **21.3 Implement.**

```python
# bench/replay/falsesignal.py
from __future__ import annotations

from collections.abc import Iterable, Sequence

from replay.metrics import Ratio
from replay.records import UncoveredRecord


def build_record(
    repo_id: str,
    commit: str,
    variant: str,
    reported: Iterable[tuple[str, int]],
    truth: Iterable[tuple[str, int]],
) -> UncoveredRecord:
    rep = tuple(sorted(reported))
    truth_set = set(truth)
    return UncoveredRecord(
        repo_id=repo_id,
        commit=commit,
        variant=variant,
        reported=rep,
        truth_covered_hits=sum(1 for p in rep if p in truth_set),
        reported_count=len(rep),
    )


def _fired(records: Sequence[UncoveredRecord]) -> list[UncoveredRecord]:
    return [r for r in records if r.reported_count > 0]


def change_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio:
    fired = _fired(records)
    wholly_wrong = sum(1 for r in fired if r.truth_covered_hits == r.reported_count)
    return Ratio(num=wholly_wrong, den=len(fired))


def line_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio:
    fired = _fired(records)
    return Ratio(
        num=sum(r.truth_covered_hits for r in fired),
        den=sum(r.reported_count for r in fired),
    )


def fire_rate(records: Sequence[UncoveredRecord], total_cycles: int) -> Ratio:
    return Ratio(num=len(_fired(records)), den=total_cycles)


def summarise(records: Sequence[UncoveredRecord], total_cycles: int) -> dict:
    return {
        "cycles": total_cycles,
        "fired": len(_fired(records)),
        "silent": len(records) - len(_fired(records)),
        "fire_rate": fire_rate(records, total_cycles).to_dict(),
        "change_false_signal_rate": change_false_signal_rate(records).to_dict(),
        "line_false_signal_rate": line_false_signal_rate(records).to_dict(),
    }
```

- [ ] **21.4 Run and pass.** `cd bench && uv run pytest tests/test_falsesignal.py -q` → 4 passed.

- [ ] **21.5 Commit.** `git add bench/replay/falsesignal.py bench/tests/test_falsesignal.py && git commit -m "bench: uncovered-report false-signal rate"`

---

## Task 22 — Selection ratio as a function of cycles-since-commit

Spec §5 states the known degradation plainly: with `--base HEAD` the changed set grows
monotonically across a long uncommitted session, so selection ratio degrades toward 1.0 the
longer an agent runs without committing. Spec §10 requires it measured as its own axis.

**Method.** Take a run of consecutive commits `C1..Ck` from real history. Create a worktree
at `C1`'s parent, seed RTDD once, then apply `C1`, `C2`, … one after another **without
committing anything**. After each application, record `rtdd which`'s selection size. Cycle
`i` therefore has the accumulated changed set of `i` real commits, which is exactly the
uncommitted-session shape.

**Files:** `bench/replay/session.py`, `bench/tests/test_session.py`

**Interfaces:**

```python
# bench/replay/session.py
@dataclasses.dataclass(frozen=True)
class DriftPoint:
    cycle: int
    changed_files: int
    selected: int
    total_tests: int
    tier: str
    def ratio(self) -> float: ...

@dataclasses.dataclass(frozen=True)
class DriftCurve:
    repo_id: str
    start_commit: str
    points: tuple[DriftPoint, ...]
    def to_dict(self) -> dict: ...

def run_drift(
    repo: pathlib.Path,
    repo_id: str,
    work: pathlib.Path,
    points: Sequence[ReplayPoint],
    python: str,
    binary: str = "rtdd",
    select: Callable[[pathlib.Path], WhichResult] | None = None,
) -> DriftCurve: ...
```

`select` is injectable so the curve logic is testable without the binary; the CLI passes
`None` and gets `rtddio.which`.

- [ ] **22.1 Failing tests.**

```python
# bench/tests/test_session.py
import sys

from replay.gitwork import ReplayPoint, add_worktree
from replay.rtddio import WhichResult
from replay.session import DriftPoint, run_drift


def test_ratio_and_serialisation():
    p = DriftPoint(cycle=3, changed_files=5, selected=8, total_tests=20, tier="T0")
    assert p.ratio() == 0.4


def test_drift_accumulates_the_changed_set_across_uncommitted_cycles(synth, tmp_path):
    work = tmp_path / "wt"
    points = [
        ReplayPoint(commit=synth.sha(1), parent=synth.sha(0)),
        ReplayPoint(commit=synth.sha(2), parent=synth.sha(1)),
        ReplayPoint(commit=synth.sha(3), parent=synth.sha(2)),
    ]
    add_worktree(synth.path, points[0].parent, work)

    seen_changed: list[int] = []

    def fake_select(w):
        from replay.gitwork import working_changed_paths

        n = len(working_changed_paths(w))
        seen_changed.append(n)
        return WhichResult(
            tier="T0", reason="", tests=tuple(f"t{i}" for i in range(n)),
            direct=(), changed=(), cycles=len(seen_changed), wall_ms=1,
        )

    curve = run_drift(
        synth.path, "synth", work, points, python=sys.executable, select=fake_select
    )
    assert [p.cycle for p in curve.points] == [1, 2, 3]
    # c1 touches alpha; c2 adds gamma + its test and re-touches alpha; c3 touches beta+gamma
    assert [p.changed_files for p in curve.points] == [1, 3, 4]
    assert seen_changed == [1, 3, 4]
    assert curve.points[-1].changed_files > curve.points[0].changed_files
    assert curve.to_dict()["points"][0]["cycle"] == 1
```

The accumulation is the assertion: cycle 3's changed set is `src/alpha.py`, `src/gamma.py`,
`tests/test_gamma.py`, `src/beta.py` — four files from three commits, none of them committed,
which is the monotone growth spec §5 describes.

- [ ] **22.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_session.py -q` → `ModuleNotFoundError: No module named 'replay.session'`.

- [ ] **22.3 Implement.**

```python
# bench/replay/session.py
from __future__ import annotations

import dataclasses
import pathlib
from collections.abc import Callable, Sequence

from replay import rtddio
from replay.gitwork import ReplayPoint, materialise_natural, working_changed_paths
from replay.runner import collect
from replay.rtddio import WhichResult


@dataclasses.dataclass(frozen=True)
class DriftPoint:
    cycle: int
    changed_files: int
    selected: int
    total_tests: int
    tier: str

    def ratio(self) -> float:
        return 0.0 if self.total_tests == 0 else self.selected / self.total_tests

    def to_dict(self) -> dict:
        return {
            "cycle": self.cycle,
            "changed_files": self.changed_files,
            "selected": self.selected,
            "total_tests": self.total_tests,
            "tier": self.tier,
            "selection_ratio": self.ratio(),
        }


@dataclasses.dataclass(frozen=True)
class DriftCurve:
    repo_id: str
    start_commit: str
    points: tuple[DriftPoint, ...]

    def to_dict(self) -> dict:
        return {
            "repo_id": self.repo_id,
            "start_commit": self.start_commit,
            "points": [p.to_dict() for p in self.points],
        }


def run_drift(
    repo: pathlib.Path,
    repo_id: str,
    work: pathlib.Path,
    points: Sequence[ReplayPoint],
    python: str,
    binary: str = "rtdd",
    select: Callable[[pathlib.Path], WhichResult] | None = None,
) -> DriftCurve:
    """The worktree must already be checked out at points[0].parent with RTDD seeded there.
    Nothing is committed between cycles: --base HEAD therefore sees the accumulated set."""
    picker = select or (lambda w: rtddio.which(w, binary=binary, base="HEAD"))
    total = len(collect(work, python=python))
    out: list[DriftPoint] = []
    for i, point in enumerate(points, start=1):
        materialise_natural(work, repo, point)
        changed = working_changed_paths(work)
        w = picker(work)
        out.append(
            DriftPoint(
                cycle=i,
                changed_files=len(changed),
                selected=len(w.tests),
                total_tests=max(total, len(w.tests)),
                tier=w.tier,
            )
        )
    return DriftCurve(
        repo_id=repo_id,
        start_commit=points[0].parent if points else "",
        points=tuple(out),
    )
```

- [ ] **22.4 Run and pass.** `cd bench && uv run pytest tests/test_session.py -q` → 2 passed.

- [ ] **22.5 Commit.** `git add bench/replay/session.py bench/tests/test_session.py && git commit -m "bench: selection-ratio drift across uncommitted cycles"`

---

## Task 23 — Orchestrator

Walks the replay points, builds the two variants, drives every strategy, caches everything,
and produces the four record types. It is where the guards actually bite.

**Files:** `bench/replay/replay.py`, `bench/tests/test_replay.py`

**Per-commit sequence, `natural` variant:**

1. `add_worktree(repo, parent, work)`.
2. Restore or build the clean-parent state: cached full run (outcomes → `pre_existing_failures`),
   plus `prepare()` for every `needs_parent_state` strategy, each stored as a cached artifact
   directory keyed by `(repo_id, parent, strategy)`.
3. `materialise_natural(work, repo, point)`.
4. `collect(work)` → `all_tests`; a collection error at this commit records
   `skipped: collect-error` and moves on (broken historical commits are real and must not
   abort a 200-commit replay).
5. Cached full uninstrumented run → outcomes, durations, wall-clock column 1.
   `f_full = failing − pre_existing_failures`.
6. Cached full instrumented run → `CoverageTruth` for the false-signal metric.
7. For each strategy **in order, `rtdd` first**: `select(ctx)` → `StrategyRecord`; feed
   `peer_sizes["rtdd"]` before `random` runs.
8. `rtdd run --json` → `UncoveredRecord` and wall-clock column 2.
9. On the wall-clock sample only: real `run_subset` per strategy → wall-clock column 3, and
   the isolation check.
10. `remove_worktree`.

The `probe` variant is identical except step 1 checks out `point.commit`, step 3 is
`materialise_probe`, and `pre_existing_failures` comes from the clean **child** tree.

**Interfaces:**

```python
# bench/replay/replay.py
@dataclasses.dataclass
class ReplayOutput:
    commits: list[CommitRecord]
    strategies: list[StrategyRecord]
    wallclocks: list[WallClockRecord]
    uncovered: list[UncoveredRecord]
    skipped: list[dict]

@dataclasses.dataclass(frozen=True)
class ReplayOptions:
    variants: tuple[str, ...] = ("natural", "probe")
    strategy_ids: tuple[str, ...] = ("rtdd", "testmon", "path", "lf", "importgraph", "xdist", "random", "full")
    wallclock_sample: int = 20
    wallclock_enabled: bool = True
    rtdd_binary: str = "rtdd"

def strategy_order(ids: Sequence[str]) -> list[str]: ...      # rtdd first, random last
def clean_tree_failures(work, python, cache, key) -> tuple[frozenset[str], dict[str, int]]: ...
def replay_repo(
    repo: pathlib.Path, spec: RepoSpec, cfg: RunConfig, cache: Cache,
    hw: Hardware, work_root: pathlib.Path, opts: ReplayOptions,
) -> ReplayOutput: ...
```

- [ ] **23.1 Failing tests for the pure parts and one end-to-end pass over the synth repo.**

```python
# bench/tests/test_replay.py
import sys

import pytest

from replay.cache import Cache
from replay.config import RunConfig
from replay.corpus import RepoSpec
from replay.hardware import probe
from replay.replay import ReplayOptions, replay_repo, strategy_order


def test_rtdd_runs_before_random_and_full_last():
    order = strategy_order(["random", "full", "path", "rtdd"])
    assert order[0] == "rtdd"
    assert order.index("random") > order.index("rtdd")
    assert order[-1] == "full"


def test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth(
    synth, tmp_path, cache_root
):
    spec = RepoSpec(
        id="synth",
        url="local",
        pin=synth.sha(3),
        replay_commits=3,
        python="3.12",
        install=(),
        source_globs=("src",),
        test_globs=("tests/**/*.py", "**/test_*.py", "**/conftest.py"),
    )
    cfg = RunConfig(
        corpus_digest="test",
        rtdd_version="test",
        tool_versions=(("pytest", "test"),),
        strategies=("full", "path", "importgraph"),
        variants=("natural",),
        replay_commits=3,
        wallclock_sample=0,
        random_seed=1,
    )
    out = replay_repo(
        repo=synth.path,
        spec=spec,
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("path", "importgraph", "full"),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    by_commit = {c.commit: c for c in out.commits}
    assert set(by_commit) == {synth.sha(1), synth.sha(2), synth.sha(3)}

    c1 = by_commit[synth.sha(1)]
    assert c1.f_full == ("tests/test_alpha.py::test_add",)
    assert c1.stratum() == "1"
    assert c1.changed == ("src/alpha.py",)

    c2 = by_commit[synth.sha(2)]
    assert c2.f_full == ()          # green commit
    assert set(c2.changed) == {"src/alpha.py", "src/gamma.py", "tests/test_gamma.py"}

    c3 = by_commit[synth.sha(3)]
    assert c3.f_full == ("tests/test_beta.py::test_mul",)

    path_at_c3 = [
        s for s in out.strategies if s.commit == synth.sha(3) and s.strategy == "path"
    ][0]
    assert set(path_at_c3.selected) == {
        "tests/test_beta.py::test_mul",
        "tests/test_gamma.py::test_sub",
    }

    full_at_c3 = [
        s for s in out.strategies if s.commit == synth.sha(3) and s.strategy == "full"
    ][0]
    assert set(full_at_c3.selected) == set(c3.all_tests)
    assert full_at_c3.escalated is True


def test_a_second_replay_is_served_from_cache(synth, tmp_path, cache_root):
    spec = RepoSpec("synth", "local", synth.sha(3), 2, "3.12", (), ("src",), ("tests/**/*.py",))
    cfg = RunConfig("test", "test", (("pytest", "t"),), ("full",), ("natural",), 2, 0, 1)
    cache = Cache(cache_root, cfg.digest())
    opts = ReplayOptions(
        variants=("natural",), strategy_ids=("full",), wallclock_sample=0, wallclock_enabled=False
    )
    kwargs = dict(repo=synth.path, spec=spec, cfg=cfg, cache=cache, hw=probe({}), opts=opts)
    replay_repo(work_root=tmp_path / "w1", **kwargs)
    first = cache.stats()["misses"]
    replay_repo(work_root=tmp_path / "w2", **kwargs)
    assert cache.stats()["misses"] == first, "the second pass must be fully cached"
```

- [ ] **23.2 Run and watch it fail.** `cd bench && uv run pytest tests/test_replay.py -q` → `ModuleNotFoundError: No module named 'replay.replay'`.

- [ ] **23.3 Implement.**

```python
# bench/replay/replay.py
from __future__ import annotations

import dataclasses
import pathlib
import shutil
import sys
from collections.abc import Sequence

from replay import rtddio
from replay.cache import Cache
from replay.config import RunConfig
from replay.corpus import RepoSpec
from replay.covread import read_coverage
from replay.falsesignal import build_record as build_uncovered
from replay.gitwork import (
    ReplayPoint,
    add_worktree,
    materialise_natural,
    materialise_probe,
    remove_worktree,
    replay_points,
    working_changed_paths,
)
from replay.hardware import Hardware
from replay.records import CommitRecord, StrategyRecord, UncoveredRecord, WallClockRecord
from replay.runner import collect, run_full, run_subset
from replay.strategies import base as sbase
from replay.strategies.randomratio import RandomRatio

# import for side-effect registration
from replay.strategies import (  # noqa: F401
    full as _full,
    importgraph as _ig,
    lastfailed as _lf,
    pathheuristic as _ph,
    rtdd as _rtdd,
    testmon as _tm,
    xdist as _xd,
)

PARENT_STATE_DIRS = {
    "rtdd": (".rtdd",),
    "testmon": (".testmondata",),
    "lf": (".pytest_cache",),
}


@dataclasses.dataclass
class ReplayOutput:
    commits: list[CommitRecord] = dataclasses.field(default_factory=list)
    strategies: list[StrategyRecord] = dataclasses.field(default_factory=list)
    wallclocks: list[WallClockRecord] = dataclasses.field(default_factory=list)
    uncovered: list[UncoveredRecord] = dataclasses.field(default_factory=list)
    skipped: list[dict] = dataclasses.field(default_factory=list)


@dataclasses.dataclass(frozen=True)
class ReplayOptions:
    variants: tuple[str, ...] = ("natural", "probe")
    strategy_ids: tuple[str, ...] = (
        "rtdd", "testmon", "path", "lf", "importgraph", "xdist", "random", "full",
    )
    wallclock_sample: int = 20
    wallclock_enabled: bool = True
    rtdd_binary: str = "rtdd"


def strategy_order(ids: Sequence[str]) -> list[str]:
    rest = [i for i in ids if i not in ("rtdd", "random", "full")]
    out = [i for i in ids if i == "rtdd"] + sorted(rest)
    out += [i for i in ids if i == "random"]
    out += [i for i in ids if i == "full"]
    return out


def _snapshot_state(work: pathlib.Path, names: Sequence[str], dest: pathlib.Path) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    for name in names:
        src = work / name
        if not src.exists():
            continue
        target = dest / name
        if src.is_dir():
            shutil.copytree(src, target, dirs_exist_ok=True)
        else:
            shutil.copy2(src, target)


def _prepare_state(
    strategy_id: str, ctx: sbase.CommitContext, cache: Cache, repo_id: str, parent: str
) -> None:
    names = PARENT_STATE_DIRS.get(strategy_id, ())
    if not names:
        sbase.get(strategy_id).prepare(ctx)
        return
    key = cache.key(repo_id, parent, "state", strategy_id)
    if cache.restore_artifact(key, ctx.work):
        return
    sbase.get(strategy_id).prepare(ctx)
    staging = ctx.work.parent / f".state-{strategy_id}"
    if staging.exists():
        shutil.rmtree(staging)
    _snapshot_state(ctx.work, names, staging)
    cache.store_artifact(key, staging)
    shutil.rmtree(staging, ignore_errors=True)


def _run_full_cached(
    work: pathlib.Path, python: str, cache: Cache, key: str, instrumented: bool, srcs: Sequence[str]
) -> dict:
    def build() -> dict:
        res = run_full(work, python=python, instrumented=instrumented, source_globs=srcs)
        return {
            "outcomes": [[o.test, o.status, o.duration_ms] for o in res.outcomes],
            "exit_code": res.exit_code,
            "wall_ms": res.wall_ms,
        }

    return cache.json_or_build(key, build)


def _failing(payload: dict) -> frozenset[str]:
    return frozenset(t for t, s, _ in payload["outcomes"] if s in ("fail", "error"))


def _durations(payload: dict) -> dict[str, int]:
    return {t: d for t, _, d in payload["outcomes"]}


def replay_repo(
    repo: pathlib.Path,
    spec: RepoSpec,
    cfg: RunConfig,
    cache: Cache,
    hw: Hardware,
    work_root: pathlib.Path,
    opts: ReplayOptions,
) -> ReplayOutput:
    sbase.register(RandomRatio(seed=cfg.random_seed))
    sbase.register(_rtdd.Rtdd(binary=opts.rtdd_binary))

    out = ReplayOutput()
    python = sys.executable
    points = replay_points(repo, spec.pin, spec.replay_commits)
    order = strategy_order(opts.strategy_ids)
    wall_every = (
        max(1, len(points) // opts.wallclock_sample)
        if opts.wallclock_enabled and opts.wallclock_sample
        else 0
    )

    for index, point in enumerate(points):
        for variant in opts.variants:
            work = work_root / f"{spec.id}-{variant}-{point.commit[:8]}"
            base_sha = point.parent if variant == "natural" else point.commit
            try:
                add_worktree(repo, base_sha, work)
                clean_key = cache.key(spec.id, base_sha, "clean", "full")
                clean = _run_full_cached(work, python, cache, clean_key, False, spec.source_globs)
                pre_existing = _failing(clean)

                seed_ctx = sbase.CommitContext(
                    repo_id=spec.id, variant=variant, commit=point.commit, parent=point.parent,
                    work=work, changed=(), all_tests=(), source_globs=spec.source_globs,
                    test_globs=spec.test_globs, python=python,
                )
                for sid in order:
                    if getattr(sbase.get(sid), "needs_parent_state", False):
                        _prepare_state(sid, seed_ctx, cache, spec.id, base_sha)

                if variant == "natural":
                    materialise_natural(work, repo, point)
                else:
                    materialise_probe(work, repo, point, spec.test_globs)

                all_tests = collect(work, python=python)
                if not all_tests:
                    out.skipped.append(
                        {"repo_id": spec.id, "commit": point.commit, "variant": variant,
                         "reason": "collect-error-or-empty"}
                    )
                    continue

                gt_key = cache.key(spec.id, point.commit, variant, "groundtruth")
                gt = _run_full_cached(work, python, cache, gt_key, False, spec.source_globs)
                durations = _durations(gt)
                f_full = tuple(sorted(_failing(gt) - pre_existing))
                changed = tuple(sorted(c.path for c in working_changed_paths(work)))

                crec = CommitRecord(
                    repo_id=spec.id, commit=point.commit, parent=point.parent, variant=variant,
                    all_tests=all_tests, durations_ms=durations, f_full=f_full,
                    pre_existing_failures=tuple(sorted(pre_existing)), changed=changed,
                )
                out.commits.append(crec)

                ctx = dataclasses.replace(
                    seed_ctx,
                    changed=tuple(working_changed_paths(work)),
                    all_tests=all_tests,
                )
                peer_sizes: dict[str, int] = {}
                selections: dict[str, sbase.Selection] = {}
                for sid in order:
                    ctx = dataclasses.replace(ctx, peer_sizes=peer_sizes)
                    sel = sbase.get(sid).select(ctx)
                    selections[sid] = sel
                    peer_sizes[sid] = len(sel.tests)
                    out.strategies.append(
                        StrategyRecord(
                            repo_id=spec.id, commit=point.commit, variant=variant, strategy=sid,
                            selected=sel.tests, escalated=sel.escalated, reason=sel.reason,
                            select_ms=sel.select_ms,
                        )
                    )

                if "rtdd" in order:
                    ro = rtddio.run(work, binary=opts.rtdd_binary, base="HEAD")
                    inst_key = cache.key(spec.id, point.commit, variant, "instrumented")
                    _run_full_cached(work, python, cache, inst_key, True, spec.source_globs)
                    truth = read_coverage(work / ".coverage", work)
                    out.uncovered.append(
                        build_uncovered(spec.id, point.commit, variant, ro.uncovered, truth.covered)
                    )
                    if wall_every and index % wall_every == 0 and hw.wallclock_allowed():
                        for sid in order:
                            sub = run_subset(
                                work, selections[sid].tests, python=python,
                                source_globs=spec.source_globs,
                            )
                            observed = sub.failing()
                            expected = set(f_full) & set(selections[sid].tests)
                            out.wallclocks.append(
                                WallClockRecord(
                                    repo_id=spec.id, commit=point.commit, variant=variant,
                                    strategy=sid,
                                    hardware_fingerprint=hw.fingerprint(),
                                    full_uninstrumented_ms=gt["wall_ms"],
                                    subset_instrumented_ms=ro.wall_ms if sid == "rtdd" else None,
                                    subset_uninstrumented_ms=sub.wall_ms,
                                    isolation_violation=observed != expected,
                                )
                            )
            finally:
                remove_worktree(repo, work)
    return out
```

- [ ] **23.4 Run and pass.** `cd bench && uv run pytest tests/test_replay.py -q` → 3 passed. (`rtdd` is excluded from `strategy_ids` in these tests, so the orchestrator runs without the binary; the real run in Task 26 includes it.)

- [ ] **23.5 Commit.** `git add bench/replay/replay.py bench/tests/test_replay.py && git commit -m "bench: replay orchestrator for the natural and probe variants"`

---
