# Bench Rounds (N4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ] `) syntax for tracking.

**Goal:** Measure rtdd v0.3.0 on rtdd-bench's Axis 2 replay. The bench's rtdd strategy stops seeding a coverage map and consuming `rtdd which --json` schema 2 — neither exists in v0.3.0 — and instead builds the node graph once per replayed instance with `rtdd graph`, reads schema 3, and expands its `file::name` test ids to the ids pytest collected. It then reports two arms per instance, **Round 1** alone and **Rounds 1+2**, each with recall against the instance's failing tests and time in tests, beside `/tdd` — the full suite every cycle — and the numbers are published per corpus row under `docs/results/`, whichever way they fall.

**Architecture:** `bench/replay/rtddio.py` stays the only module that shells out to `rtdd` or knows its JSON: it gains `graph()` and a schema-3 `parse_which()`, and loses `seed()`, `run()` and everything that parsed the v0.2 `run` document. A new pure module, `bench/replay/graphids.py`, holds `expand_graph_ids()`, the one place a schema-3 test id is turned into collected pytest ids. `bench/replay/strategies/rtdd.py` registers two strategies over the same `rtdd which` answer: `rtdd` (Round 1; it keeps the id every report, chart and the `random` peer already key on) and `rtdd-r12` (Rounds 1+2; builds no graph of its own). `replay.replay_repo` drops the v0.2 `rtdd run` uncovered step. `replay.report` grows one table — both arms beside `full`, labelled `/tdd` — and `replay.cli` one flag, `--results-label`, so the v0.3.0 run lands in `bench/results/n4/<repo_id>/` and never on a published v0.2 result. The published page, `docs/results/n4-bench-rounds.md`, quotes those summaries and a guard test holds it to them.

**Tech Stack:** Python 3.12 under `uv` (the `bench/` project, `bench/pyproject.toml`), pytest, stdlib only (`dataclasses`, `json`, `re`, `subprocess`); no new dependency. The binary under test is this repository's `rtdd` (Go 1.24), driven as a subprocess and never imported. Tests run against stub binaries written into `tmp_path`, the synthetic replay repo (`bench/tests/synthrepo.py`), and — where a test says so — the real `rtdd` on PATH, skipped unless it is v0.3.0.

**Spec:** [`docs/specs/2026-10-07-node-graph.md`](../specs/2026-10-07-node-graph.md) §1 (success, measured: "on rtdd-bench, time in tests for Rounds 1+2 is reported against `/tdd` … recall is reported for Round 1 alone and for Rounds 1+2. A slower or lower-recall row is published as it is"), §7 (rounds), §9 (`--json` schema 3) and §11 (non-goals) — PRD #412. Each task names the sibling issue it is and the PRD acceptance criteria it discharges; the map after the decisions lists all five.

## Global Constraints

- **Pre-registration is a human decision.** Nothing in this plan writes `signed_at`, `signed_by` or `stratified_recall_floor` into `bench/PREREGISTRATION.md` — they ship empty and stay empty — and nothing creates a `prereg-*` git tag. Task 3's guard test asserts the three fields are still empty.
- **`~/.cache/rtdd-bench` is never deleted, re-keyed or invalidated.** It is the Axis 1 driver store (`bench/swebench/run_arm.py`'s `CACHE_ROOT`); this plan does not touch `bench/swebench/` at all. The Axis 2 replay cache (`bench/cache/`, `replay.cache.DEFAULT_ROOT`) is salted with the run config's digest, which includes the `rtdd` binary's identity, so a v0.3.0 run writes new keys beside the old ones: no task deletes, renames or re-keys an existing entry, and no task changes `Cache.key`.
- **Shipped defaults only.** The bench invokes `rtdd graph` and `rtdd which --base HEAD --json` exactly — no flag, no config file, no environment variable of its own. `rtddio` keeps exposing no parameter for extra argv (`test_no_entry_point_accepts_extra_rtdd_arguments`).
- **No change to selection to improve a bench number.** Selection — what Round 1 and Round 2 contain — is PRD #410's and is settled (spec §7, §12). The bench consumes `rtdd which` as shipped; no task changes a file outside `bench/` and `docs/results/`. A lane that finds selection wrong files an issue against #410's spec; it does not tune it here, and it does not tune the bench per language or per row either.
- **Never a silent empty selection.** A missing binary, a non-zero exit, malformed JSON, a schema other than 3, or a document missing `rounds` raises `RtddError`; an empty selection is never manufactured from a failure (the existing `rtddio` contract, kept).
- **Never overwrite a published result.** `bench/results/<repo_id>/` holds the published v0.2 tables that the README, `aggregate.md` and `internal/installtest/outcomes_test.go` read. The v0.3.0 run writes to `bench/results/n4/<repo_id>/` (Task 3) — the same rule `results_dir_for` already enforces for `--commit-selection paired`.
- **Publish as measured.** Every row of `bench/corpus.yaml` is replayed at its own `replay_commits`, with the default `--commit-selection recent`, once; no row is dropped, re-run selectively or re-weighted, and a slower or lower-recall row is published as it is (spec §1).
- `uv run pytest -q` in `bench/` and `scripts/ci-local.sh` exit 0 at the end of every task, run with **this branch's** `rtdd` first on PATH, so the real-binary tests run instead of skipping:

  ```bash
  bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
  ```

  An installed v0.2 `rtdd` has no `rtdd graph`; the real-binary tests skip on it rather than fail, which is why the gate is run with the branch binary first.

## Review Focus

- **A schema-3 test id pytest never collected.** The graph classifies every function in a test file as a test (`graph.Classify`), so a helper or a fixture in `tests/` is a Round 1 "test" with no pytest id. It must be dropped and counted in `Selection.reason`, never raised as an invented id (that would abort the cycle) and never kept (that would fail `validate_selection`). Test in Task 1 (`test_a_helper_in_a_test_file_matches_no_collected_id`).
- **Parametrised and class-nested tests.** `tests/t.py::TestA::test_x[1-2]` is the pytest spelling of graph id `tests/t.py::TestA::test_x`; matching on the bare name or on the file alone either drops it or selects the whole file. Tests in Task 1.
- **The duplicate-definition suffix.** The scanner disambiguates a repeated id as `…::name@<line>`; it must still match pytest's id. Test in Task 1.
- **`rtdd graph` run more than once per instance.** The `rtdd-r12` arm answers from the graph the `rtdd` arm's `prepare` built; a second `prepare` doubles the measured cost and is a different experiment. Tests in Task 1 (once per cycle) and Task 2 (`rtdd-r12` builds none).
- **Round 2 double-counted.** Schema 3's Round 2 already excludes Round 1; the union must be de-duplicated anyway, so a test is never charged twice to "time in tests". Test in Task 2.
- **A v0.3.0 run landing on `bench/results/<repo_id>/`.** It would replace the published v0.2 population and break `outcomes_test.go`. Test in Task 3.
- **The results page drifting from the records.** Every number on the page must be the one in `bench/results/n4/<repo_id>/summary.json`, formatted by `report.fmt`. Test in Task 3.

## The consumed document: `rtdd which --json` schema 3

`rtdd which --base HEAD --json` (PRD #410, `cmd/rtdd/which.go`) emits exactly these top-level keys: `schema`, `command`, `base`, `graph`, `changed`, `changed_nodes`, `rounds`, `untested`, `warnings`. `rounds` is `[{round: 1, tests, files}, {round: 2, tests, files}, {round: 3, full_suite: true}]`; each test is `{id, file, name}` with `id` = `<file>::<qualified name>` (`tests/test_calc.py::TestCalc::test_add`). Every array is `[]`, never `null`. Exit codes are 0 / 2 / 3; none reflects a test result.

`rtddio` reads only what it scores and fails loudly on everything else:

```python
SCHEMA_VERSION = 3


@dataclasses.dataclass(frozen=True)
class WhichResult:
    round1: tuple[str, ...]     # schema-3 test ids, document order
    round2: tuple[str, ...]     # schema-3 test ids; excludes round1 by construction (spec §7)
    changed: tuple[str, ...]    # every changed path, deleted ones included
    untested: tuple[str, ...]   # node ids
    source: str                 # graph.source: "scanner" or "graphify"
    wall_ms: int                # measured around the subprocess, never read from the document
    warnings: tuple[str, ...] = ()
```

- `parse_which(payload, wall_ms)` requires `schema == 3` and raises `RtddError` on any other value (`"rtdd which --json is schema 2, this harness consumes schema 3"`), on a missing `schema` key (`"… carries no schema version; refusing to guess"`), and on a document with no `rounds` (`"… has no 'rounds' field"`), no round 1 or no round 2 (`"rtdd which --json has no round 2"`), a test entry with no `id` (`"rtdd which --json: a test in round 1 has no id"`), or a non-list where a list belongs. It does **not** require round 3: the bench never runs it (it is `/tdd`, measured as `full`).
- `graph(work, binary="rtdd")` runs `rtdd graph` and returns `None`; any non-zero exit raises (2 is a configuration error, 3 a graph that cannot be built — neither is a measurement).
- `which(work, binary="rtdd", base="HEAD")` runs `rtdd which --base <base> --json`; only exit 0 is accepted.
- Deleted: `seed`, `run`, `parse_run`, `RunOutput`, `expand_ranges`, `read_cycles`, `CLASS_UNCOVERED`, `CLASS_IMPORT_TIME`, `_RUN_OK_CODES`. v0.3.0 has no `seed`, no `run`, no `meta.json` and no uncovered report (spec §8, §11).
- Kept as is: `RtddError`, `rtdd_version` (v0.3.0 answers `rtdd --version`, so `RunConfig.rtdd_version` records the version string rather than a digest), `_invoke`, `_decode`.

## Test-id expansion: `file::name` to the bench's vocabulary

The bench scores recall and cost per **pytest node id** — `CommitRecord.all_tests` is what `pytest --collect-only` printed (`replay.runner.collect`), and `validate_selection` refuses an id outside it. Schema 3 names graph test nodes. `bench/replay/graphids.py` maps one onto the other:

```python
def expand_graph_ids(
    graph_ids: Sequence[str], all_tests: Sequence[str]
) -> tuple[tuple[str, ...], tuple[str, ...]]:
    """(collected ids the graph ids name, graph ids that name no collected id)."""
```

- A collected id `F::Q[params]` is keyed as `F::Q` — the trailing `[…]` of its **last** segment removed (pytest's parametrisation suffix; a class segment never carries one).
- A graph id `F::Q@N` is keyed as `F::Q` — the scanner's duplicate-definition suffix removed.
- A graph id expands to **every** collected id with the same key, in collection order: one parametrised function is all its cases.
- A graph id matching no key is returned in the second tuple. It is a helper, a fixture, or a test pytest did not collect at this tree — not an error, and not a selected test. `Selection.reason` publishes the count.
- Output is de-duplicated, first occurrence wins, so the union of two rounds never charges a test twice.
- No file-level fallback: a graph id never widens to "every test in its file". That would turn a missed link into a hit and is selection tuning by the back door.

The function is language-neutral by construction — `::` is both schema 3's separator and pytest's — but it is tested per corpus language, and `test_every_corpus_language_has_an_expansion_case` fails the day `bench/corpus.yaml` admits a language with no case (today every row is Python: every `source_globs` entry ends in `.py`).

## The two arms

| Strategy id | Arm | `needs_parent_state` | `prepare` | `select` |
|---|---|---|---|---|
| `rtdd` | **Round 1** | `True` | `rtddio.graph(ctx.work)` — the instance's one graph build, at the parent tree | `rtddio.which`, `round1` expanded |
| `rtdd-r12` | **Rounds 1+2** | `False` | nothing | `rtddio.which`, `round1 + round2` expanded |
| `full` | **`/tdd`** (full suite every cycle) | `False` | nothing | every collected id; unchanged |

- `rtdd` keeps its id, so `random`'s peer, `PARENT_STATE_DIRS["rtdd"] = (".rtdd",)`, `chart.HEAD_TO_HEAD`, `report.MAP_BASED_STRATEGIES` and the pre-registered `verdict:` line (rtdd vs `path`) keep working unchanged. The verdict line now reads Round 1 against `path`.
- `rtdd-r12` reads the `.rtdd/graph.json` the `rtdd` arm's `prepare` built (restored from the parent-state cache on a re-run), the way `random` reads `rtdd`'s selection size: it declares `peer = "rtdd"`, `strategy_order` runs it right after `rtdd`, and `cli replay` refuses a `--strategies` list that has `rtdd-r12` without `rtdd`. It joins `report.MAP_BASED_STRATEGIES` (its state is built at the parent too, so its `probe` rows are an upper bound and are labelled as one).
- Each arm calls `rtdd which` itself and reports its own `select_ms`; `which` builds nothing when the graph cache is warm, so the second call is not a second graph build.
- `escalated` is `False` for both arms on every cycle: v0.3.0 never escalates; Round 3 is the full suite, and that is `full`.
- `/tdd` is the `full` strategy and is not renamed: its id is in every committed record. Tables label it `` `full` — `/tdd` ``.

**Recall** is `metrics.test_level_recall_micro` (Σ|selected ∩ F_full| / Σ|F_full|) and `metrics.change_level_recall`; **time in tests** is `metrics.selected_duration_fraction` — its `num` is the milliseconds the arm's selected tests took in the same commit's ground-truth full run, its `den` the whole suite's, so `full` reads `1.000` and its `num` is `/tdd`'s time. Both already exist and are not changed.

## Decisions this plan settles

The PRD leaves these open; each is settled here so three lanes do not settle it three ways, and each is pinned by a test in the task named.

1. **`SCHEMA_VERSION = 3`** and `WhichResult` as above; schema 2, a missing schema and an unknown schema each raise. Task 1.
2. **`rtdd graph` runs once per instance**, in the `rtdd` arm's `prepare`, at the parent tree; `rtdd seed` is called nowhere and `rtddio.seed` no longer exists. Task 1.
3. **`expand_graph_ids` in `bench/replay/graphids.py`**, keyed as above; unmatched ids are dropped and counted, never widened to a file. Task 1.
4. **The v0.2 `rtdd run` step leaves `replay_repo`** — `_cached_uncovered`, `_cached_coverage_truth` and the `rtdd`-only wall-clock branch go; the `rtdd` arm's instrumented subset is timed like every other arm's. `ReplayOutput.uncovered`, `UncoveredRecord`, `falsesignal` and `report`'s readers stay: `report --rebuild` re-renders the published v0.2 records that carry them. Task 1.
5. **`bench/tests/v02binary.py` is deleted**, with every `@requires_v02_rtdd` — plan 10 made PRD #412 its owner. The tests it fenced are rewritten for v0.3.0 and fenced instead by `requires_v03_rtdd` (skip unless `rtdd --help` lists `rtdd graph`), defined in `bench/tests/rtddbin.py`. Task 1.
6. **The drift session** (`cli session`, `replay.session.run_drift`) builds the graph with `rtddio.graph` where it seeded, and records Round 1's size as `selected`; its `tier` field records `"round1"` (v0.3.0 has no tiers; the key stays so `drift.json` keeps its shape). Task 1.
7. **Arms:** `rtdd` = Round 1, `rtdd-r12` = Rounds 1+2, `full` = `/tdd`; `rtdd-r12` is a registered strategy (so `tests/test_registry_complete.py`'s `REQUIRED` gains it — the PRD and that list move together, as its docstring says). Task 2.
8. **The report table** is `report.rounds_table(summary)` over `ROUNDS_ARMS = ("rtdd", "rtdd-r12", "full")`, rendered under `## Rounds against /tdd` in `summary.md` whenever `rtdd-r12` is in the run; its header starts `strategy`, never `arm` (that cell marks a §7 comparison table, which owes wall-clock columns) and it carries no wall-clock column. Task 2.
9. **`--results-label <label>`** on `cli replay` writes to `bench/results/<label>/<repo_id>/`; `results_dir_for(repo_id, commit_selection, label="")`. The N4 run uses `--results-label n4`. Task 3.
10. **The N4 run** is `--strategies rtdd,rtdd-r12,path,full` with the default variants (`natural,probe`), the corpus's own `replay_commits` and the default wall-clock sampling. `path` is in it so the pre-registered `verdict:` line stays computable; the other baselines are not re-measured. Task 3.
11. **The page** is `docs/results/n4-bench-rounds.md`; `bench/tests/test_results_n4_rounds.py` holds it to the summaries. Task 3.

## Acceptance-criterion map

| PRD #412 AC | What | Task(s) | Issue(s) |
|---|---|---|---|
| AC1 | no seeding; one `rtdd graph` per instance; schema 3 read, any other schema rejected | 1 | #484 |
| AC2 | two arms per instance, Round 1 and Rounds 1+2, each with recall and time in tests; `/tdd` the baseline | 2 | #485 |
| AC3 | `file::name` expanded to the bench's ids, a test per corpus language | 1 | #484 |
| AC4 | a results page under `docs/results/`, per corpus row, both arms against `/tdd`, published as measured | 3 | #486 |
| AC5 | `uv run pytest` in `bench/` and `scripts/ci-local.sh` exit 0 | every task | #484, #485, #486 |

Order: this plan (#483) first. Then Task 1, Task 2, Task 3, strictly in sequence — each consumes the previous one's interfaces.

## File Structure

| Path | Responsibility | Task |
|---|---|---|
| `bench/replay/rtddio.py` | `SCHEMA_VERSION = 3`, `WhichResult`, `parse_which`, `graph`, `which`, `rtdd_version` | 1 |
| `bench/replay/graphids.py` | `expand_graph_ids` | 1 |
| `bench/replay/strategies/rtdd.py` | `Rtdd` (Task 1), `RtddRounds12` (Task 2) | 1, 2 |
| `bench/replay/replay.py` | the v0.2 `rtdd run` step removed (1); `strategy_order`, registration of `rtdd-r12` (2) | 1, 2 |
| `bench/replay/session.py`, `bench/replay/cli.py` | drift session on `rtddio.graph` (1); `rtdd-r12` peer guard and default strategies (2); `--results-label` (3) | 1, 2, 3 |
| `bench/replay/report.py` | `ROUNDS_ARMS`, `rounds_table`, `MAP_BASED_STRATEGIES` | 2 |
| `bench/tests/rtddbin.py` | `rtdd_on_path_is_v03`, `requires_v03_rtdd` | 1 |
| `bench/tests/v02binary.py` | deleted | 1 |
| `bench/tests/test_rtddio.py`, `test_graph_ids.py`, `test_strategy_rtdd.py`, `test_replay.py`, `test_session.py` | Task 1's tests | 1 |
| `bench/tests/test_strategy_rounds.py`, `test_registry_complete.py` | Task 2's tests | 2 |
| `bench/tests/test_results_n4_rounds.py` | the page and the run, held to each other | 3 |
| `bench/results/n4/<repo_id>/` | the v0.3.0 run's records, one directory per corpus row | 3 |
| `docs/results/n4-bench-rounds.md` | the published page | 3 |
| `internal/contract/plan_n4_test.go` | this document's own contract (issue #483) | — |

---

### Task 1: The rtdd strategy builds one graph per instance, reads schema 3 and expands `file::name` ids

**Issue:** #484

**Discharges:** AC1, AC3, AC5

**Files:**
- Modify: `bench/replay/rtddio.py`, `bench/replay/strategies/rtdd.py`, `bench/replay/replay.py`, `bench/replay/session.py`, `bench/replay/cli.py`
- Create: `bench/replay/graphids.py`, `bench/tests/rtddbin.py`, `bench/tests/test_graph_ids.py`
- Replace: `bench/tests/test_rtddio.py`, `bench/tests/test_strategy_rtdd.py`
- Modify: `bench/tests/test_replay.py`, `bench/tests/test_session.py`
- Delete: `bench/tests/v02binary.py`

**Interfaces:**
- Produces: `rtddio.SCHEMA_VERSION = 3`; `rtddio.WhichResult(round1, round2, changed, untested, source, wall_ms, warnings=())`; `rtddio.parse_which(payload: object, wall_ms: int) -> WhichResult`; `rtddio.graph(work: Path, binary: str = "rtdd") -> None`; `rtddio.which(work: Path, binary: str = "rtdd", base: str = "HEAD") -> WhichResult`; `graphids.expand_graph_ids(graph_ids, all_tests) -> tuple[tuple[str, ...], tuple[str, ...]]`; `strategies.rtdd.Rtdd` with `id = "rtdd"`, `needs_parent_state = True`, `ROUNDS = (1,)`, and a `_graph_ids(w: WhichResult) -> tuple[str, ...]` hook Task 2 overrides; `tests.rtddbin.requires_v03_rtdd`.
- Consumes: `rtdd graph` and `rtdd which --base HEAD --json` (PRD #410); `strategies.base.Selection`, `CommitContext`, `register`; `replay.corpus.load_corpus`.

- [ ] **Step 1: Write the failing tests.** Create `bench/tests/rtddbin.py`:

```python
"""The bench tests that drive a real `rtdd` need a v0.3.0 one.

v0.3.0 builds a node graph (`rtdd graph`) and emits `rtdd which --json` schema 3;
v0.2 seeded a coverage map and emitted schema 2. A test that executes the binary
runs only when the `rtdd` on PATH has `rtdd graph`, so a machine with an old
install skips it instead of failing for a reason no branch change can fix.
"""
from __future__ import annotations

import shutil
import subprocess

import pytest


def rtdd_on_path_is_v03() -> bool:
    exe = shutil.which("rtdd")
    if exe is None:
        return False
    out = subprocess.run([exe, "--help"], capture_output=True, text=True, check=False)
    return "rtdd graph" in out.stdout + out.stderr


requires_v03_rtdd = pytest.mark.skipif(
    not rtdd_on_path_is_v03(),
    reason="needs a v0.3.0 rtdd on PATH (rtdd graph, which --json schema 3)",
)
```

Create `bench/tests/test_graph_ids.py`:

```python
"""Schema-3 test ids (`file::name`) expanded to the ids pytest collected.

Recall and cost are scored per collected id, so this is the step that decides
whether a graph test counts at all. Every case is one shape real pytest output
has; `test_every_corpus_language_has_an_expansion_case` fails the day the corpus
admits a language nobody wrote a case for.
"""
from __future__ import annotations

import pathlib

import pytest

from replay.corpus import load_corpus
from replay.graphids import expand_graph_ids

BENCH = pathlib.Path(__file__).resolve().parents[1]

PY_COLLECTED = (
    "tests/test_calc.py::test_add",
    "tests/test_calc.py::TestCalc::test_neg",
    "tests/test_calc.py::TestCalc::TestNested::test_zero",
    "tests/test_calc.py::test_mul[1-1]",
    "tests/test_calc.py::test_mul[2-4]",
    "tests/test_calc.py::test_mul[a::b]",
    "tests/test_other.py::test_add",
)

#: language -> (graph ids, collected ids, expected expansion, expected unmatched)
CASES = {
    "python": (
        (
            "tests/test_calc.py::test_add",
            "tests/test_calc.py::TestCalc::test_neg",
            "tests/test_calc.py::TestCalc::TestNested::test_zero",
            "tests/test_calc.py::test_mul",
            "tests/test_calc.py::helper",
        ),
        PY_COLLECTED,
        (
            "tests/test_calc.py::test_add",
            "tests/test_calc.py::TestCalc::test_neg",
            "tests/test_calc.py::TestCalc::TestNested::test_zero",
            "tests/test_calc.py::test_mul[1-1]",
            "tests/test_calc.py::test_mul[2-4]",
            "tests/test_calc.py::test_mul[a::b]",
        ),
        ("tests/test_calc.py::helper",),
    ),
}

#: A corpus row's language, read from its source globs' extension.
EXTENSIONS = {".py": "python"}


@pytest.mark.parametrize("language", sorted(CASES))
def test_graph_ids_expand_to_collected_ids(language):
    graph_ids, collected, want, unmatched = CASES[language]
    assert expand_graph_ids(graph_ids, collected) == (want, unmatched)


def test_every_corpus_language_has_an_expansion_case():
    corpus = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock")
    languages = set()
    for repo_id in corpus.ids():
        for glob in corpus.require(repo_id).source_globs:
            ext = pathlib.PurePosixPath(glob).suffix
            assert ext in EXTENSIONS, f"{repo_id}: no language known for {glob!r}"
            languages.add(EXTENSIONS[ext])
    assert languages, "the corpus names no source language"
    assert languages <= set(CASES), f"no expansion case for {sorted(languages - set(CASES))}"


def test_the_file_is_part_of_the_key():
    got, _ = expand_graph_ids(("tests/test_other.py::test_add",), PY_COLLECTED)
    assert got == ("tests/test_other.py::test_add",)


def test_the_duplicate_definition_suffix_still_matches():
    got, unmatched = expand_graph_ids(("tests/test_calc.py::test_add@12",), PY_COLLECTED)
    assert got == ("tests/test_calc.py::test_add",)
    assert unmatched == ()


def test_a_helper_in_a_test_file_matches_no_collected_id():
    got, unmatched = expand_graph_ids(("tests/test_calc.py::helper",), PY_COLLECTED)
    assert got == ()
    assert unmatched == ("tests/test_calc.py::helper",)


def test_a_graph_id_never_widens_to_its_whole_file():
    got, _ = expand_graph_ids(("tests/test_calc.py::gone",), PY_COLLECTED)
    assert got == ()


def test_output_is_deduplicated_in_first_seen_order():
    got, _ = expand_graph_ids(
        ("tests/test_calc.py::test_add", "tests/test_calc.py::test_add@9"), PY_COLLECTED
    )
    assert got == ("tests/test_calc.py::test_add",)
```

Replace `bench/tests/test_rtddio.py` with:

```python
"""The rtdd binary adapter: schema 3, failure paths, and the argv contract.

Every test here except the last runs against a *stub* binary written into
``tmp_path``, so the module passes on a machine with no ``rtdd``. The last test
is the contract test: it runs the real v0.3.0 binary and is the only thing that
can catch the schema drifting away from what the parser assumes.
"""
from __future__ import annotations

import copy
import inspect
import json
import pathlib
import subprocess

import pytest

from replay import rtddio
from replay.rtddio import RtddError, parse_which
from tests.rtddbin import requires_v03_rtdd

# A trimmed but structurally faithful `rtdd which --base HEAD --json`, schema 3
# (cmd/rtdd/which.go): the nine top-level keys, three rounds, every array a list.
WHICH_PAYLOAD = {
    "schema": 3,
    "command": "which",
    "base": "HEAD",
    "graph": {
        "source": "scanner",
        "built_at_commit": "abc1234",
        "stale_files": 1,
        "nodes": 9,
        "edges": 7,
        "tests": 4,
    },
    "changed": [
        {"path": "src/alpha.py", "lines": [{"start": 2, "end": 2}]},
        {"path": "src/legacy.py", "lines": []},
    ],
    "changed_nodes": [
        {"id": "src/alpha.py::add", "file": "src/alpha.py", "name": "add", "start": 1, "end": 2}
    ],
    "rounds": [
        {
            "round": 1,
            "tests": [
                {"id": "tests/test_alpha.py::test_add", "file": "tests/test_alpha.py", "name": "test_add"},
                {
                    "id": "tests/test_alpha.py::TestAlpha::test_neg",
                    "file": "tests/test_alpha.py",
                    "name": "test_neg",
                },
            ],
            "files": ["tests/test_alpha.py"],
        },
        {
            "round": 2,
            "tests": [
                {"id": "tests/test_beta.py::test_mul", "file": "tests/test_beta.py", "name": "test_mul"}
            ],
            "files": ["tests/test_beta.py"],
        },
        {"round": 3, "full_suite": True},
    ],
    "untested": ["src/alpha.py::helper"],
    "warnings": ["src/legacy.py was deleted"],
}


def _stub(
    tmp_path: pathlib.Path,
    name: str,
    *,
    stdout: str = "",
    stderr: str = "",
    code: int = 0,
) -> pathlib.Path:
    """Write an executable stub that replays fixed output and logs its argv."""
    home = tmp_path / name
    home.mkdir(parents=True, exist_ok=True)
    (home / "stdout").write_text(stdout)
    (home / "stderr").write_text(stderr)
    binary = home / "rtdd"
    binary.write_text(
        "#!/bin/sh\n"
        f'printf "%s\\n" "$@" >> "{home}/argv"\n'
        f'cat "{home}/stdout"\n'
        f'cat "{home}/stderr" >&2\n'
        f"exit {code}\n"
    )
    binary.chmod(0o755)
    return binary


def _argv(binary: pathlib.Path) -> list[str]:
    return (binary.parent / "argv").read_text().splitlines()


def _work(tmp_path: pathlib.Path) -> pathlib.Path:
    work = tmp_path / "work"
    work.mkdir(exist_ok=True)
    return work


# --- parsing -----------------------------------------------------------------


def test_parse_which_reads_rounds_one_and_two():
    w = parse_which(WHICH_PAYLOAD, wall_ms=17)
    assert w.round1 == ("tests/test_alpha.py::test_add", "tests/test_alpha.py::TestAlpha::test_neg")
    assert w.round2 == ("tests/test_beta.py::test_mul",)
    assert w.changed == ("src/alpha.py", "src/legacy.py")
    assert w.untested == ("src/alpha.py::helper",)
    assert w.source == "scanner"
    assert w.warnings == ("src/legacy.py was deleted",)
    assert w.wall_ms == 17


def test_the_consumed_schema_is_3():
    assert rtddio.SCHEMA_VERSION == 3


@pytest.mark.parametrize("schema", [2, 4, "3", None])
def test_parse_which_rejects_any_schema_but_3(schema):
    payload = {**WHICH_PAYLOAD, "schema": schema}
    with pytest.raises(RtddError, match="schema"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_document_with_no_schema_key():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "schema"}
    with pytest.raises(RtddError, match="no schema"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_schema_2_document():
    v2 = {"schema": 2, "command": "which", "tier": "T0", "selection": {"tests": []}, "changed": []}
    with pytest.raises(RtddError, match="schema 2"):
        parse_which(v2, wall_ms=0)


def test_parse_which_rejects_a_document_with_no_rounds():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "rounds"}
    with pytest.raises(RtddError, match="rounds"):
        parse_which(payload, wall_ms=0)


@pytest.mark.parametrize("missing", [1, 2])
def test_parse_which_rejects_a_missing_round(missing):
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"] = [r for r in payload["rounds"] if r["round"] != missing]
    with pytest.raises(RtddError, match=f"round {missing}"):
        parse_which(payload, wall_ms=0)


def test_parse_which_rejects_a_test_with_no_id():
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"][0]["tests"].append({"file": "tests/x.py", "name": "test_x"})
    with pytest.raises(RtddError, match="id"):
        parse_which(payload, wall_ms=0)


def test_parse_which_does_not_need_round_3():
    payload = copy.deepcopy(WHICH_PAYLOAD)
    payload["rounds"] = payload["rounds"][:2]
    assert parse_which(payload, wall_ms=0).round2 == ("tests/test_beta.py::test_mul",)


def test_v02_entry_points_are_gone():
    for name in ("seed", "run", "parse_run", "RunOutput", "read_cycles", "expand_ranges"):
        assert not hasattr(rtddio, name), f"rtddio.{name} drives a v0.2 binary"


# --- subprocess failure paths ------------------------------------------------


def test_which_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.which(_work(tmp_path), binary=str(tmp_path / "no-such-rtdd"))


def test_which_raises_on_non_json_stdout(tmp_path):
    b = _stub(tmp_path, "b", stdout="rtdd: something human\n")
    with pytest.raises(RtddError, match="non-JSON"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_a_usage_exit_code(tmp_path):
    b = _stub(tmp_path, "b", stderr='rtdd which: gitctx: unknown base "NOPE"\n', code=2)
    with pytest.raises(RtddError, match="exited 2"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_exit_1(tmp_path):
    # v0.3.0 has no exit code 1 at all; one is a broken binary, not a test result.
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD), code=1)
    with pytest.raises(RtddError, match="exited 1"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_graph_raises_on_a_graph_that_cannot_be_built(tmp_path):
    b = _stub(tmp_path, "g", stderr="rtdd graph: not a git repository\n", code=3)
    with pytest.raises(RtddError, match="exited 3"):
        rtddio.graph(_work(tmp_path), binary=str(b))


def test_wall_ms_is_measured_not_reported(tmp_path):
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD))
    assert rtddio.which(_work(tmp_path), binary=str(b)).wall_ms >= 0


# --- the argv contract: shipped defaults only --------------------------------


def test_the_harness_passes_no_tuning_flags(tmp_path):
    graph_bin = _stub(tmp_path, "g", stdout="source: scanner\n")
    which_bin = _stub(tmp_path, "w", stdout=json.dumps(WHICH_PAYLOAD))
    work = _work(tmp_path)

    rtddio.graph(work, binary=str(graph_bin))
    rtddio.which(work, binary=str(which_bin))

    assert _argv(graph_bin) == ["graph"]
    assert _argv(which_bin) == ["which", "--base", "HEAD", "--json"]


def test_no_entry_point_accepts_extra_rtdd_arguments():
    # A knob for extra argv is how "shipped defaults only" quietly stops being true.
    for fn in (rtddio.graph, rtddio.which):
        params = set(inspect.signature(fn).parameters)
        assert params <= {"work", "binary", "base"}, fn.__name__


# --- version capture for RunConfig -------------------------------------------


def test_rtdd_version_prefers_the_binarys_own_version_output(tmp_path):
    b = _stub(tmp_path, "v", stdout="rtdd v0.3.0 (commit a3f21e0)\n")
    assert rtddio.rtdd_version(str(b)) == "rtdd v0.3.0 (commit a3f21e0)"


def test_rtdd_version_falls_back_to_a_content_digest(tmp_path):
    b = _stub(tmp_path, "v", stderr="rtdd: unknown command\n", code=2)
    got = rtddio.rtdd_version(str(b))
    assert got.startswith("sha256:")
    assert len(got) == len("sha256:") + 64


def test_rtdd_version_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.rtdd_version(str(tmp_path / "no-such-rtdd"))


# --- the contract test: the real binary --------------------------------------


@requires_v03_rtdd
def test_real_binary_honours_the_consumed_schema(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    rtddio.graph(synth.path)
    assert (synth.path / ".rtdd" / "graph.json").is_file()

    (synth.path / "src" / "alpha.py").write_text("def add(a, b):\n    return a + b + 1\n")
    w = rtddio.which(synth.path)
    assert "tests/test_alpha.py::test_add" in w.round1
    assert "src/alpha.py" in w.changed
    assert w.source == "scanner"
```

Replace `bench/tests/test_strategy_rtdd.py` with:

```python
"""The system under test as a strategy: `rtdd graph` once, then `rtdd which`.

Parser-level tests monkeypatch `rtddio`, so they run without the binary; the last
test drives the real v0.3.0 binary on the synthetic repo. The strategy passes no
flags of its own: a tuned rtdd against untuned baselines is not a result.
"""
from __future__ import annotations

import pathlib

from replay.gitwork import Change
from replay.rtddio import WhichResult
from replay.strategies.base import CommitContext, get
from replay.strategies.rtdd import Rtdd
from tests.rtddbin import requires_v03_rtdd

ADD = "tests/test_alpha.py::test_add"
MUL = "tests/test_beta.py::test_mul"
ALL = (ADD, MUL)


def _ctx(work: pathlib.Path | str = "/nonexistent", changed: list | None = None) -> CommitContext:
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path(work),
        changed=tuple(Change(p, s) for p, s in (changed or [])),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )


def _which(**over) -> WhichResult:
    fields = dict(round1=(), round2=(), changed=(), untested=(), source="scanner", wall_ms=0)
    fields.update(over)
    return WhichResult(**fields)


def test_prepare_builds_the_graph_with_the_configured_binary(monkeypatch):
    calls = []
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.graph",
        lambda work, binary="rtdd": calls.append((work, binary)),
    )
    Rtdd(binary="/opt/rtdd").prepare(_ctx("/tree"))
    assert calls == [(pathlib.Path("/tree"), "/opt/rtdd")]


def test_select_takes_round_1_expanded_to_collected_ids(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(
            round1=(ADD, "tests/test_alpha.py::helper"), round2=(MUL,), wall_ms=7
        ),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == (ADD,)
    assert sel.escalated is False
    assert sel.select_ms == 7
    assert "round 1" in sel.reason
    assert "1 not collected" in sel.reason


def test_select_passes_only_shipped_defaults(monkeypatch):
    seen = []

    def fake_which(work, binary="rtdd", base="HEAD"):
        seen.append({"work": work, "binary": binary, "base": base})
        return _which()

    monkeypatch.setattr("replay.strategies.rtdd.rtddio.which", fake_which)
    Rtdd(binary="/opt/rtdd").select(_ctx("/tree"))
    assert seen == [{"work": pathlib.Path("/tree"), "binary": "/opt/rtdd", "base": "HEAD"}]


def test_an_empty_round_1_is_an_empty_selection_not_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == ()
    assert sel.escalated is False


def test_the_rtdd_strategy_is_registered_under_its_plan_id():
    s = get("rtdd")
    assert s.id == "rtdd"
    assert s.needs_parent_state is True


@requires_v03_rtdd
def test_verified_against_the_synthetic_repo(synth):
    """End to end on the synth fixture: graph at c2, edit alpha, select test_add."""
    import subprocess

    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    s = Rtdd()
    s.prepare(_ctx(synth.path))
    assert (synth.path / ".rtdd" / "graph.json").is_file()

    (synth.path / "src" / "alpha.py").write_text(
        "def add(a, b):\n    return a + b + 1\n", encoding="utf-8"
    )
    sel = s.select(_ctx(synth.path, [("src/alpha.py", "M")]))
    assert ADD in sel.tests
    assert sel.escalated is False
    assert sel.select_ms >= 0
```

Append to `bench/tests/test_replay.py`, and replace its `test_a_base_tree_rtdd_refuses_to_seed_is_skipped_not_fatal` and `test_a_refused_rtdd_run_is_published_not_fatal` with the first two below (drop the `tests.v02binary` import):

```python
def test_a_base_tree_rtdd_cannot_graph_is_skipped_not_fatal(synth, cache_root, monkeypatch):
    """A parent tree the tool under test cannot build a graph of costs its own cycle."""
    import replay.replay as mod
    from replay.rtddio import RtddError

    built = {"n": 0}

    def refuse_once(work, binary="rtdd"):
        built["n"] += 1
        if built["n"] == 1:
            raise RtddError("rtdd graph exited 3: not a git repository")

    monkeypatch.setattr(mod.rtddio, "graph", refuse_once)
    spec = _spec(synth)
    cfg = _cfg(strategies=("rtdd", "full"))
    out = replay_repo(
        repo=synth.path,
        spec=spec,
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe(),
        work_root=synth.path.parent / "trees-nograph",
        opts=ReplayOptions(
            variants=("natural",), strategy_ids=("rtdd", "full"), wallclock_sample=0
        ),
    )
    assert any(s["reason"] == "parent-state-unavailable" for s in out.skipped), out.skipped
    assert out.skipped[0]["strategy"] == "rtdd"


def test_the_replay_runs_no_rtdd_run(synth, cache_root, monkeypatch):
    """v0.3.0 has no `rtdd run` and no uncovered report; the replay must not ask for one."""
    import replay.replay as mod
    from replay.rtddio import WhichResult

    monkeypatch.setattr(mod.rtddio, "graph", lambda work, binary="rtdd": None)
    monkeypatch.setattr(
        mod.rtddio,
        "which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            round1=(ADD,), round2=(), changed=(), untested=(), source="scanner", wall_ms=1
        ),
    )
    assert not hasattr(mod, "_cached_uncovered")
    spec = _spec(synth)
    cfg = _cfg(strategies=("rtdd", "full"))
    out = replay_repo(
        repo=synth.path,
        spec=spec,
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe(),
        work_root=synth.path.parent / "trees-norun",
        opts=ReplayOptions(
            variants=("natural",), strategy_ids=("rtdd", "full"), wallclock_sample=0
        ),
    )
    assert out.commits
    assert out.uncovered == []
    assert out.rtdd_run_errors == []


def test_the_rtdd_strategy_builds_one_graph_per_instance_and_never_seeds(
    synth, cache_root, monkeypatch
):
    """PRD #412 AC1: one `rtdd graph` per replayed instance, and no `rtdd seed` at all."""
    import replay.replay as mod
    from replay.rtddio import WhichResult

    assert not hasattr(mod.rtddio, "seed"), "v0.3.0 has no `rtdd seed`; nothing may call it"
    built: list = []
    monkeypatch.setattr(mod.rtddio, "graph", lambda work, binary="rtdd": built.append(work))
    monkeypatch.setattr(
        mod.rtddio,
        "which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            round1=(ADD,), round2=(), changed=(), untested=(), source="scanner", wall_ms=1
        ),
    )
    spec = _spec(synth)
    cfg = _cfg(strategies=("rtdd", "full"))
    out = replay_repo(
        repo=synth.path,
        spec=spec,
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe(),
        work_root=synth.path.parent / "trees-graph",
        opts=ReplayOptions(
            variants=("natural",), strategy_ids=("rtdd", "full"), wallclock_sample=0
        ),
    )
    instances = {(s.commit, s.variant) for s in out.strategies if s.strategy == "rtdd"}
    assert instances, "the replay scored no rtdd instance"
    assert len(built) == len(instances)
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd bench && uv run pytest -q tests/test_graph_ids.py tests/test_rtddio.py tests/test_strategy_rtdd.py tests/test_replay.py
```

Expected: FAIL — collection errors `ModuleNotFoundError: No module named 'replay.graphids'` and `ImportError: cannot import name ...` / `TypeError: WhichResult.__init__() got an unexpected keyword argument 'round1'`; in `test_replay.py`, `AttributeError: <module 'replay.rtddio'> has no attribute 'graph'`.

- [ ] **Step 3: `bench/replay/graphids.py`.** Implement `expand_graph_ids` exactly as "Test-id expansion" states: build `key -> [collected ids]` once (`file, _, rest = t.partition("::")`; skip an id with no `::`; strip `re.sub(r"\[.*\]$", "", rest)` from the last segment only), strip `re.sub(r"@\d+$", "", gid)` from each graph id, extend in collection order, de-duplicate with `dict.fromkeys`. Module docstring: why ids are matched by key and never widened to a file.

- [ ] **Step 4: `bench/replay/rtddio.py`.** Rewrite to schema 3 as "The consumed document" states. The module docstring keeps its three load-bearing properties (shipped defaults only; never a silent empty selection; the consumed schema) and names schema 3 and `cmd/rtdd/which.go`. `_require_schema` keeps its messages (`"carries no schema version"`, `"is schema {n!r}, this harness consumes schema 3"`). `graph()` is `_invoke(work, [binary, "graph"])`. Delete what the section lists.

- [ ] **Step 5: `bench/replay/strategies/rtdd.py`.** `Rtdd.prepare` calls `rtddio.graph(ctx.work, binary=self.binary)`. `Rtdd.select` calls `rtddio.which(ctx.work, binary=self.binary, base="HEAD")`, takes `self._graph_ids(w)` (`w.round1` here), expands it with `expand_graph_ids(ids, ctx.all_tests)`, and returns `Selection(tests=…, escalated=False, reason=f"round 1: {len(ids)} graph tests, {len(unmatched)} not collected", select_ms=w.wall_ms)`. Delete `expand_files` (the file-level expansion of schema 2). Update the module docstring: no seed, no tiers.

- [ ] **Step 6: `bench/replay/replay.py`.** Delete `_cached_uncovered`, `_cached_coverage_truth`, the `if "rtdd" in order:` block that called them, and the `if sid == "rtdd" and rtdd_wall_ms is not None` branch (the `rtdd` arm's instrumented subset is timed like every arm's). Remove imports the deletion leaves unused (`build_uncovered`, `covread` helpers) only if nothing else in the module uses them — `uv run ruff check` is not a gate here, so check with `grep`. `ReplayOutput` keeps `uncovered` and `rtdd_run_errors` (published v0.2 records carry them); nothing appends to either any more. Update the `PARENT_STATE_DIRS` docstring: the `rtdd` state is the graph cache.

- [ ] **Step 7: Drift session.** In `bench/replay/cli.py` `cmd_session`, `rtddio.seed(work, …)` becomes `rtddio.graph(work, …)`. In `bench/replay/session.py`, `run_drift` records `selected=len(result.round1)`, `total_tests=0 if total == 0 else max(total, len(result.round1))` and `tier="round1"`; its docstring says the graph is built once before cycle 1. In `bench/tests/test_session.py`, `_which(n_tests, tier=…, cycles=…)` becomes `_which(n_tests)` (every call site drops its `tier`/`cycles` arguments, and the inline `WhichResult(...)` in `test_a_cycle_whose_tree_will_not_collect_publishes_no_ratio` takes the same fields) returning `WhichResult(round1=tuple(f"tests/t.py::test_{i}" for i in range(n_tests)), round2=(), changed=(), untested=(), source="scanner", wall_ms=0)`, and the one assertion on `p.tier` expects `"round1"`.

- [ ] **Step 8: Delete the v0.2 fence.** `git rm bench/tests/v02binary.py`; `grep -rn v02binary bench/` must print nothing.

- [ ] **Step 9: Run to verify they pass, then the gate**

```bash
cd bench && uv run pytest -q
cd .. && bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS, with `test_real_binary_honours_the_consumed_schema` and `test_verified_against_the_synthetic_repo` **run, not skipped** (check with `uv run pytest -q -rs` against the branch binary: neither appears under "SKIPPED").

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(bench): rtdd strategy builds one graph per instance and reads which --json schema 3 (closes #484)"
```

---

### Task 2: The bench reports two arms per instance — Round 1 and Rounds 1+2 — beside `/tdd`

**Issue:** #485

**Discharges:** AC2, AC5

**Files:**
- Modify: `bench/replay/strategies/rtdd.py` (`RtddRounds12`), `bench/replay/replay.py` (`strategy_order`, registration), `bench/replay/cli.py` (`DEFAULT_STRATEGIES`, the `rtdd-r12` peer guard), `bench/replay/report.py` (`ROUNDS_ARMS`, `rounds_table`, `MAP_BASED_STRATEGIES`, `render_markdown`)
- Create: `bench/tests/test_strategy_rounds.py`
- Modify: `bench/tests/test_registry_complete.py` (`REQUIRED` gains `rtdd-r12`)

**Interfaces:**
- Produces: `strategies.rtdd.RtddRounds12` with `id = "rtdd-r12"`, `needs_parent_state = False`, `peer = "rtdd"`, `ROUNDS = (1, 2)`; `report.ROUNDS_ARMS = ("rtdd", "rtdd-r12", "full")`; `report.rounds_table(summary: dict) -> list[str]`; `replay.strategy_order` placing `rtdd-r12` directly after `rtdd`.
- Consumes: Task 1's `Rtdd`, `WhichResult.round1`/`round2`, `expand_graph_ids`; `metrics.summarise` (unchanged); `report.fmt`, `report.assert_distribution_beside_mean`.

- [ ] **Step 1: Write the failing tests.** Create `bench/tests/test_strategy_rounds.py`:

```python
"""PRD #412 AC2: two rtdd arms per instance — Round 1, and Rounds 1+2 — beside /tdd.

Both arms answer from the same `rtdd which`; only `rtdd` builds the graph. Recall
is scored against the instance's failing tests (`F_full`) and time in tests is the
selected tests' share of the same commit's full-suite durations, exactly as every
other strategy is scored — nothing here is new arithmetic.
"""
from __future__ import annotations

import pathlib

import pytest

from replay import metrics
from replay.records import CommitRecord, StrategyRecord
from replay.replay import strategy_order
from replay.report import ROUNDS_ARMS, assert_distribution_beside_mean, rounds_table
from replay.rtddio import WhichResult
from replay.strategies.base import CommitContext, get
from replay.strategies.rtdd import Rtdd, RtddRounds12

R1 = "tests/test_alpha.py::test_add"
R2 = "tests/test_beta.py::test_mul"
OTHER = "tests/test_gamma.py::test_sub"
ALL = (R1, R2, OTHER)


def _ctx() -> CommitContext:
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/tree"),
        changed=(),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


@pytest.fixture
def which(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": WhichResult(
            round1=(R1,), round2=(R2, R1), changed=("src/alpha.py",), untested=(),
            source="scanner", wall_ms=4,
        ),
    )


def test_the_round_1_arm_selects_round_1_only(which):
    assert Rtdd().select(_ctx()).tests == (R1,)


def test_the_rounds_1_2_arm_is_the_union_without_double_counting(which):
    sel = RtddRounds12().select(_ctx())
    assert sel.tests == (R1, R2)
    assert sel.escalated is False
    assert "rounds 1+2" in sel.reason


def test_the_rounds_1_2_arm_builds_no_graph(monkeypatch):
    built = []
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.graph", lambda work, binary="rtdd": built.append(work)
    )
    arm = RtddRounds12()
    arm.prepare(_ctx())
    assert built == []
    assert arm.needs_parent_state is False
    assert arm.peer == "rtdd"


def test_both_arms_are_registered_and_rtdd_answers_first():
    assert get("rtdd-r12").id == "rtdd-r12"
    assert strategy_order(["full", "path", "rtdd-r12", "random", "rtdd"]) == [
        "rtdd", "rtdd-r12", "path", "random", "full",
    ]


def _records():
    commit = CommitRecord(
        repo_id="synth", commit="c", parent="p", variant="natural", all_tests=ALL,
        durations_ms={R1: 10, R2: 30, OTHER: 60}, f_full=(R2,),
        pre_existing_failures=(), changed=("src/alpha.py",),
    )

    def rec(sid, selected, escalated=False):
        return StrategyRecord(
            repo_id="synth", commit="c", variant="natural", strategy=sid,
            selected=selected, escalated=escalated, reason="", select_ms=0,
        )

    return [commit], [rec("rtdd", (R1,)), rec("rtdd-r12", (R1, R2)), rec("full", ALL, True)]


def test_round_2_recovers_a_failure_round_1_missed():
    commits, recs = _records()
    r1 = metrics.summarise(commits, recs, "rtdd")
    r12 = metrics.summarise(commits, recs, "rtdd-r12")
    full = metrics.summarise(commits, recs, "full")
    assert r1["test_level_recall_micro"]["value"] == 0.0
    assert r12["test_level_recall_micro"]["value"] == 1.0
    assert full["test_level_recall_micro"]["value"] == 1.0
    assert r1["selected_duration_fraction"]["num"] == 10
    assert r12["selected_duration_fraction"]["num"] == 40
    assert full["selected_duration_fraction"]["num"] == 100


def test_the_rounds_table_puts_tdd_beside_both_arms():
    commits, recs = _records()
    summary = {"strategies": {sid: metrics.summarise(commits, recs, sid) for sid in ROUNDS_ARMS}}
    lines = rounds_table(summary)
    assert ROUNDS_ARMS == ("rtdd", "rtdd-r12", "full")
    assert lines[0].startswith("| strategy |")
    rows = [line for line in lines if line.startswith("| `")]
    assert [r.split("|")[1].strip() for r in rows] == [
        "`rtdd` — Round 1", "`rtdd-r12` — Rounds 1+2", "`full` — `/tdd`",
    ]
    assert "0.400 (40/100)" in rows[1]
    assert "1.000 (100/100)" in rows[2]
    assert_distribution_beside_mean("\n".join(lines))


def test_an_arm_missing_from_the_run_is_absent_not_imputed():
    commits, recs = _records()
    summary = {"strategies": {sid: metrics.summarise(commits, recs, sid) for sid in ("rtdd", "full")}}
    rows = [line for line in rounds_table(summary) if line.startswith("| `")]
    assert len(rows) == 2
```

Add `"rtdd-r12"` to `REQUIRED` in `bench/tests/test_registry_complete.py` (beside `SYSTEM_UNDER_TEST`, with a one-line comment: the Rounds 1+2 arm of the system under test, PRD #412) and import nothing new — it lives in `replay.strategies.rtdd`, already imported.

- [ ] **Step 2: Run to verify they fail**

```bash
cd bench && uv run pytest -q tests/test_strategy_rounds.py tests/test_registry_complete.py
```

Expected: FAIL — `ImportError: cannot import name 'ROUNDS_ARMS' from 'replay.report'` (and `RtddRounds12` from `replay.strategies.rtdd`); `test_the_registry_holds_exactly_the_required_strategies` fails with `rtdd-r12` missing from the registry.

- [ ] **Step 3: `RtddRounds12`** in `bench/replay/strategies/rtdd.py`: subclass `Rtdd`; `id = "rtdd-r12"`, `needs_parent_state = False`, `peer = "rtdd"`; `prepare` returns `None`; `_graph_ids(w)` returns `w.round1 + w.round2`; the reason reads `rounds 1+2: …`. `register(RtddRounds12())` beside `register(Rtdd())`. In `replay_repo`, register `RtddRounds12(binary=opts.rtdd_binary)` beside `Rtdd(binary=…)`. The docstring states why it builds no graph (the instance's one graph is `rtdd`'s).

- [ ] **Step 4: Order and guard.** `strategy_order` puts `rtdd-r12` immediately after `rtdd` (and is otherwise unchanged; update its docstring). `cli.DEFAULT_STRATEGIES` gains `"rtdd-r12"` after `"rtdd"`. `cmd_replay` refuses a `--strategies` list holding `rtdd-r12` without `rtdd`, exactly as it refuses `random` without its peer (`EXIT_GUARD`, a message naming both), with a test beside the existing `random` guard test in `bench/tests/test_cli.py`.

- [ ] **Step 5: `report.py`.** Add `ROUNDS_ARMS` with a docstring, `ROUNDS_LABELS = {"rtdd": "Round 1", "rtdd-r12": "Rounds 1+2", "full": "`/tdd`"}`, and `rounds_table(summary)`: header `| strategy | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |`; one row per arm present, in `ROUNDS_ARMS` order, `` `<id>` — <label> ``, cells `fmt(change_level_recall)`, `fmt(test_level_recall_micro)`, the `selected_duration_fraction` `num` as an integer, and `fmt(selected_duration_fraction)`. Add `"rtdd-r12"` to `MAP_BASED_STRATEGIES`. In `render_markdown`, when `"rtdd-r12"` is in `summary["strategies"]`, render `## Rounds against /tdd` (one sentence: what each arm runs, and that time in tests is the selected tests' share of the same commit's full-suite durations) followed by `rounds_table(summary)`, after the per-strategy table.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
cd bench && uv run pytest -q
cd .. && bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS. `uv run python -m replay.cli report --rebuild` leaves `git status --porcelain bench/results` empty (no published v0.2 summary has `rtdd-r12`, so none gains the new section).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(bench): Round 1 and Rounds 1+2 arms per instance, reported beside /tdd (closes #485)"
```

---

### Task 3: Publish the v0.3.0 measurement under `docs/results/`

**Issue:** #486

**Discharges:** AC4, AC5

**Files:**
- Modify: `bench/replay/cli.py` (`--results-label`, `results_dir_for`)
- Create: `bench/tests/test_results_n4_rounds.py`
- Create (by running the bench): `bench/results/n4/<repo_id>/{commits.jsonl,config.json,summary.json,summary.md}` for every row of `bench/corpus.yaml`
- Create: `docs/results/n4-bench-rounds.md`

**Interfaces:**
- Produces: `cli.results_dir_for(repo_id: str, commit_selection: str, label: str = "") -> Path`; `cli replay --results-label <label>`; the published page.
- Consumes: Tasks 1 and 2 (the arms, `rounds_table`); `replay.corpus.load_corpus`; `report.fmt`.

- [ ] **Step 1: Write the failing tests.** Create `bench/tests/test_results_n4_rounds.py`:

```python
"""PRD #412 AC4: the v0.3.0 measurement, published per corpus row, held to its records.

The page is prose a reader trusts; `bench/results/n4/` is the record it was written
from. Every number the page prints for an arm must be the one `report.fmt` renders
from that row's `summary.json`, so a hand-edited, rounded or re-run-until-better
figure fails here. Nothing asserts which way a number falls.
"""
from __future__ import annotations

import json
import pathlib
import re

import pytest

from replay.cli import results_dir_for
from replay.corpus import load_corpus
from replay.report import fmt

BENCH = pathlib.Path(__file__).resolve().parents[1]
ROOT = BENCH.parent
PAGE = ROOT / "docs" / "results" / "n4-bench-rounds.md"
N4 = BENCH / "results" / "n4"
ARMS = ("rtdd", "rtdd-r12", "full")


def corpus_ids() -> tuple[str, ...]:
    return load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock").ids()


def _summary(repo_id: str) -> dict:
    return json.loads((N4 / repo_id / "summary.json").read_text(encoding="utf-8"))


def _row(page: str, repo_id: str, arm: str) -> str:
    # `{arm}` — : the closing backtick and the dash keep `rtdd` from matching `rtdd-r12`.
    rows = [
        line
        for line in page.splitlines()
        if line.startswith(f"| `{repo_id}` | `{arm}` —")
    ]
    assert len(rows) == 1, f"want one `{repo_id}` / `{arm}` row on the page, got {len(rows)}"
    return rows[0]


def test_a_labelled_run_never_lands_on_the_published_directory():
    assert results_dir_for("flask", "recent", label="n4") == BENCH / "results" / "n4" / "flask"
    assert results_dir_for("flask", "recent") == BENCH / "results" / "flask"
    assert results_dir_for("flask", "paired") == BENCH / "results" / "paired" / "flask"


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_every_corpus_row_was_replayed_with_both_arms_and_tdd(repo_id):
    for name in ("commits.jsonl", "config.json", "summary.json", "summary.md"):
        assert (N4 / repo_id / name).is_file(), f"bench/results/n4/{repo_id}/{name} is missing"
    summary = _summary(repo_id)
    assert summary["repo_id"] == repo_id
    for arm in ARMS:
        assert arm in summary["strategies"], f"{repo_id}: no `{arm}` arm in the run"
    assert "## Rounds against /tdd" in (N4 / repo_id / "summary.md").read_text(encoding="utf-8")


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_the_page_quotes_each_row_as_its_summary_has_it(repo_id):
    page = PAGE.read_text(encoding="utf-8")
    strategies = _summary(repo_id)["strategies"]
    for arm in ARMS:
        row = _row(page, repo_id, arm)
        for key in ("change_level_recall", "test_level_recall_micro", "selected_duration_fraction"):
            assert fmt(strategies[arm][key]) in row, f"{repo_id}/{arm}: {key} differs from summary.json"


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_the_page_states_the_binary_corpus_and_hardware(repo_id):
    page = PAGE.read_text(encoding="utf-8")
    cfg = json.loads((N4 / repo_id / "config.json").read_text(encoding="utf-8"))
    assert cfg["config"]["rtdd_version"] in page
    assert cfg["config"]["corpus_digest"][:12] in page
    assert cfg["hardware"]["fingerprint"] in page


def test_the_page_says_how_to_reproduce_it():
    page = PAGE.read_text(encoding="utf-8")
    assert "## Reproduce" in page
    assert "uv run python -m replay.cli replay" in page
    assert "--results-label n4" in page
    assert "--strategies rtdd,rtdd-r12,path,full" in page


def test_the_preregistration_is_left_unsigned():
    text = (BENCH / "PREREGISTRATION.md").read_text(encoding="utf-8")
    for field in ("signed_at", "signed_by", "stratified_recall_floor"):
        m = re.search(rf"^{field}:(.*)$", text, re.MULTILINE)
        assert m is not None, f"PREREGISTRATION.md lost its {field} field"
        assert m.group(1).strip() == "", f"{field} was written; it is a human decision"
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd bench && uv run pytest -q tests/test_results_n4_rounds.py
```

Expected: FAIL — `TypeError: results_dir_for() got an unexpected keyword argument 'label'`; every parametrised row fails on the missing `bench/results/n4/<repo_id>/` files and the missing page (`FileNotFoundError: …/docs/results/n4-bench-rounds.md`). `test_the_preregistration_is_left_unsigned` passes from the start and must stay green.

- [ ] **Step 3: `--results-label`.** `results_dir_for(repo_id, commit_selection, label="")` returns `RESULTS / label / repo_id` when `label` is set (a label wins over a non-default commit selection: `RESULTS / label / commit_selection / repo_id` when both are set), else today's behaviour. `cmd_replay` passes `args.results_label`; the flag's help says it is how a run that must not replace a published result is kept beside it. A label is a single path segment: reject one containing `/` or `..`, or equal to a corpus id, with `EXIT_GUARD` (test it in `bench/tests/test_cli.py`). `cmd_rebuild`, `cmd_aggregate` and the corpus-coverage tests read `RESULTS / <repo_id>` only, so a labelled tree is never folded into the published aggregate — check with `uv run python -m replay.cli report --rebuild` leaving `git status --porcelain bench/results` empty.

- [ ] **Step 4: Run the bench.** Build this branch's binary and put it first on PATH; confirm `rtdd --version` reports v0.3.0 (or the branch's `dev` build, which `config.json` then records verbatim). For each row of `bench/corpus.yaml`, in corpus order:

```bash
cd bench && uv run python -m replay.cli replay --repo <repo_id> --strategies rtdd,rtdd-r12,path,full --results-label n4
```

No `--replay-commits`, no `--commit-selection`, no `--variants`: the corpus's own counts and the default `natural,probe`. Each row is run **once**. A row that errors is fixed at the cause (harness bug, environment) and re-run in full, and the page says so; a row is never re-run because its numbers disappointed. Skipped cycles (`parent-state-unavailable`, `collect-error-or-empty`, `no-tests-collected`) are published in the row's `summary.json` as they fall. Do not delete or clear `bench/cache/` or `~/.cache/rtdd-bench` before, during or after.

- [ ] **Step 5: Write `docs/results/n4-bench-rounds.md`**, in the shape of `docs/results/one-pipeline-first-measure.md` (title with the date, a paragraph saying what was measured and what was not, `## Method`, `## Results`, `## Reading`, `## Reproduce`):
  - the rtdd version string, the corpus digest (first 12 hex), the hardware (CPU, cores, memory, platform, Python, fingerprint) — read from each row's `config.json`;
  - `## Results`: one table per variant, `natural` first and `probe` second, labelled upper bound (graph built at the child commit) and never pooled with `natural`; columns `| row | arm | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |`; three rows per corpus row — `` | `<repo_id>` | `rtdd` — Round 1 | ``, `` | `<repo_id>` | `rtdd-r12` — Rounds 1+2 | ``, `` | `<repo_id>` | `full` — `/tdd` | `` in the `natural` table; the `probe` table's row cell reads `` `<repo_id>` (probe, upper bound) `` so the guard's row lookup finds exactly one `natural` row per arm — each cell copied from that row's `summary.json` as `report.fmt` renders it (`summary.md`'s `## Rounds against /tdd` table has them, for the primary variant; `by_variant` holds `probe`);
  - each row's pre-registered `verdict:` line, quoted from its `summary.md` as it reads;
  - `## Reading`: per corpus row, whether Rounds 1+2 cut time in tests against `/tdd` and what it missed, in plain words; a slower or lower-recall row says so; no per-language explanation is offered as a reason to discount a row;
  - `## Reproduce`: the build and the `replay` command of Step 4, and that a re-run at the same config reproduces `bench/results/n4/` byte for byte from `bench/cache/`.

- [ ] **Step 6: Run to verify they pass, then the gate**

```bash
cd bench && uv run pytest -q
cd .. && bin="$(mktemp -d)" && go build -o "$bin/rtdd" ./cmd/rtdd && PATH="$bin:$PATH" scripts/ci-local.sh
```

Expected: PASS. `git diff --stat bench/results/ ':!bench/results/n4'` is empty — no published v0.2 result moved. `git status --porcelain bench/PREREGISTRATION.md` is empty and `git tag -l 'prereg-*'` lists exactly what it listed before the run.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "docs(results): rtdd v0.3.0 on rtdd-bench — Round 1 and Rounds 1+2 against /tdd, per corpus row (closes #486)"
```

---

## What this plan deliberately leaves undone

Everything below is real work; none of it belongs to PRD #412, and no task above may start it.

- **Changing selection to improve a bench number.** What Round 1 and Round 2 contain — depth, direction, linking of same-named functions, test classification — is PRD #410's, settled in spec §7 and §12. A disappointing row is published, and if it shows selection is wrong, an issue is filed against #410's spec; nothing in `cmd/`, `internal/` or the graph changes under this PRD, and no per-language or per-row tuning enters the bench.
- **Signing the pre-registration.** `signed_at`, `signed_by` and `stratified_recall_floor` in `bench/PREREGISTRATION.md` and any `prereg-*` tag are human decisions; Task 3 only asserts they are untouched.
- **The Axis 1 (SWE-bench) driver and its cache.** `bench/swebench/` and `~/.cache/rtdd-bench` are not touched — no deletion, no re-keying, no invalidation. Whether the agent A/B is re-run on v0.3.0 is a separate decision.
- **Round 3.** The full suite is `/tdd`, measured once as `full`; no arm runs "Rounds 1+2, then the full suite" — the cost of that is `rtdd-r12` plus `full`, readable from the table.
- **Re-measuring the other baselines** (`testmon`, `lf`, `importgraph`, `xdist`, `random`) on v0.3.0. Their published v0.2-era numbers do not depend on the rtdd binary; `path` is re-run only so the pre-registered verdict line stays computable.
- **The false-signal (uncovered) axis.** v0.3.0 reports no uncovered lines (spec §11); the replay stops asking. The published v0.2 false-signal figures stay as history, and `falsesignal.py` stays so `report --rebuild` can re-render them.
- **Replacing the published v0.2 results.** `bench/results/<repo_id>/`, `aggregate.md`, the README's tables and `internal/installtest/outcomes_test.go` keep reading the v0.2 population; whether the README cites the N4 page is PRD #411's (the front-ends) or a later PRD's call.
- **Admitting new corpus rows or languages.** `bench/corpus.yaml` is frozen; `test_every_corpus_language_has_an_expansion_case` makes a future admission add its expansion case first.
- **Tagging and releasing v0.3.0** — a human action (#418).
