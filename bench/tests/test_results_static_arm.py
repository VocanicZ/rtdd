"""The published `static` arm: what the backfill left in `bench/results/` (#349).

`bench/results/{flask,httpie}/` were measured before the `TS` static tier had a model.
The arm that models it executes nothing — it is a function of the `changed` set, the
`all_tests` and the committed `importgraph` selection each replayed commit already
carries — so it was backfilled offline with `python -m replay.cli derive` and published
with `report --rebuild`, and no benchmark was re-run. `bench/results/README.md` records
the two commands.

`test_cli.py` covers the verb. This file covers the ARTIFACTS the verb was pointed at,
because the ways a backfill can go wrong are all invisible in a passing code path: an
arm that reached the records and not the summary, a wall-clock figure invented for an
arm that ran nothing, a rewritten `sqlfluff` that `corpus_version: 1` published, or a
ground-truth cache re-keyed to publish a number the cache never measured.
"""

from __future__ import annotations

import hashlib
import json
import pathlib
import subprocess
import textwrap

import pytest

from replay import cli
from replay.corpus import freeze
from replay.records import CommitRecord, StrategyRecord, parse_jsonl_lines

BENCH = pathlib.Path(__file__).resolve().parents[1]
RESULTS = BENCH / "results"

#: The repos whose published results the backfill was pointed at — the corpus that
#: `corpus_version: 2` admits.
BACKFILLED = ("flask", "httpie")

#: `bench/results/sqlfluff/` as `corpus_version: 1` published it. #184 dropped the repo
#: at version 2 and its files are kept for the record, so they are frozen: superseded,
#: never edited in place. Digests rather than a git query, so the guard holds in a
#: checkout, an export or a release tarball alike.
SQLFLUFF_DIGESTS: dict[str, str] = {
    "commits.jsonl": "69036232b527deecd2b1a90884c05c37dae9d4b2cb62d1d7af80af42793d461f",
    "config.json": "2661da61cf87060a791181a7676adb5d1d6ba40a391e200b5895c3b40454d6ab",
    "drift.json": "9b04b770d80f9ed77b86e56bca3a276504fa54487d50ce14c0be743b396ec6de",
    "summary.json": "4007613fc6d4b13e7a44ea8eb4e293d48e921bdc6ebe538081f378f323be1d82",
    "summary.md": "9211f0011a4a95de37ad08afef54ad0df6a7d2f05375b8dfda79f20d2ac1f957",
}


def _tree_digest(root: pathlib.Path) -> dict[str, str]:
    return {
        str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest()
        for p in sorted(root.rglob("*"))
        if p.is_file()
    }


# --- the arm reached the published records and the published summaries ----


@pytest.mark.parametrize("repo_id", BACKFILLED)
def test_the_committed_records_carry_one_derived_static_record_per_commit(repo_id: str) -> None:
    records = parse_jsonl_lines(
        (RESULTS / repo_id / "commits.jsonl").read_text(encoding="utf-8").splitlines()
    )
    commits = [r for r in records if isinstance(r, CommitRecord)]
    static = [r for r in records if isinstance(r, StrategyRecord) and r.strategy == "static"]
    assert commits, f"{repo_id} publishes no commit records at all"
    keys = {(r.commit, r.variant) for r in static}
    assert keys == {(r.commit, r.variant) for r in commits}
    assert len(static) == len(keys), "a commit carries two static records"
    assert all(r.derived for r in static), "a backfilled arm must say it was derived"
    assert all(r.select_ms == 0 for r in static), "nothing was timed; 0 is not a fast run"


@pytest.mark.parametrize("repo_id", BACKFILLED)
def test_the_published_summary_carries_the_static_arm_in_every_variant(repo_id: str) -> None:
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    assert "static" in summary["strategies"]
    for variant, per in summary["by_variant"].items():
        assert "static" in per, f"{repo_id}/{variant} publishes no static arm"


@pytest.mark.parametrize("repo_id", BACKFILLED)
def test_the_static_arm_publishes_no_wallclock_it_never_measured(repo_id: str) -> None:
    """It executed nothing. A row here would be another arm's figure, or one synthesised
    from `durations_ms` — which is a different execution's per-test time."""
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    assert "static" not in (summary["wallclock"].get("rows") or {})
    for line in (RESULTS / repo_id / "commits.jsonl").read_text(encoding="utf-8").splitlines():
        rec = json.loads(line)
        assert not (rec["kind"] == "wallclock" and rec.get("strategy") == "static"), (
            f"{repo_id} committed a wall-clock record for an arm that ran nothing"
        )


@pytest.mark.parametrize("repo_id", BACKFILLED)
def test_the_published_markdown_renders_the_static_wallclock_as_not_measured(repo_id: str) -> None:
    md = (RESULTS / repo_id / "summary.md").read_text(encoding="utf-8")
    assert "## The static arm" in md, f"{repo_id}/summary.md has no §7 comparison section"
    row = [
        ln
        for ln in md.split("## The static arm", 1)[1].splitlines()
        if ln.startswith("| `static`")
    ]
    assert row, f"{repo_id}/summary.md has no `static` row in the comparison table"
    assert row[0].count("not measured") == 4, (
        "the mean and all three percentiles must say not measured, never a blank"
    )


@pytest.mark.parametrize("repo_id", BACKFILLED)
def test_the_run_config_that_stamped_the_numbers_still_never_names_the_derived_arm(
    repo_id: str,
) -> None:
    """`config.json` records what the RUN executed, and its strategy list feeds
    `cfg.digest()` — the key of the ground-truth cache. Adding `static` there to publish
    the arm would orphan every entry the benchmark ever measured (#218)."""
    stamp = json.loads((RESULTS / repo_id / "config.json").read_text(encoding="utf-8"))
    assert "static" not in stamp["config"]["strategies"]


# --- what the backfill was not allowed to touch --------------------------


def test_the_dropped_sqlfluff_result_is_byte_identical_to_what_v1_published() -> None:
    assert _tree_digest(RESULTS / "sqlfluff") == SQLFLUFF_DIGESTS


CORPUS_YAML = textwrap.dedent(
    """
    frozen_at: "2026-01-01"
    corpus_version: 1
    criteria:
      - "synthetic"
    admission:
      min_collected_tests: 400
      max_uninstrumented_full_suite_seconds: 600
      max_compiled_extensions_on_import_path: 0
    repos:
      - id: synth
        url: https://example.invalid/synth.git
        pin: "0123456789012345678901234567890123456789"
        replay_commits: 3
        python: "3.12"
        source_globs: ["src/**/*.py"]
        test_globs: ["tests/**/*.py"]
        measured:
          on_hardware: "synthetic"
          collected_tests: 900
          uninstrumented_full_suite_seconds: 12
          compiled_extensions_on_import_path: 0
          replay_commits_ceiling: 3
    excluded:
      - id: rejected
        reason: "not hermetic"
    """
).lstrip()


@pytest.fixture
def offline_bench(tmp_path, monkeypatch):
    """The CLI's paths, pointed at a throwaway tree with one admitted repo."""
    d = tmp_path / "corpus"
    d.mkdir(parents=True)
    (d / "corpus.yaml").write_text(CORPUS_YAML, encoding="utf-8")
    freeze(d / "corpus.yaml", d / "corpus.lock")
    monkeypatch.setattr(cli, "CORPUS", d / "corpus.yaml")
    monkeypatch.setattr(cli, "LOCK", d / "corpus.lock")
    monkeypatch.setattr(cli, "WORK", tmp_path / "work")
    monkeypatch.setattr(cli, "CACHE", tmp_path / "cache")
    monkeypatch.setattr(cli, "RESULTS", tmp_path / "results")
    return tmp_path


def _synth_results(results: pathlib.Path) -> pathlib.Path:
    from tests.test_cli import _published_with_importgraph

    return _published_with_importgraph(results, "synth")


def test_the_backfill_provisions_nothing_clones_nothing_and_executes_nothing(
    offline_bench, monkeypatch
) -> None:
    """Every route out of "derive offline" and back into "run the benchmark", blocked at
    once. A backfill that reaches any of these is re-measuring, not re-deriving — and it
    would be re-measuring on hardware that is not the one the published numbers name."""

    def forbidden(*a, **kw):
        raise AssertionError("a derivation must not provision, clone or execute anything")

    monkeypatch.setattr(cli, "clone_pinned", forbidden)
    monkeypatch.setattr(cli, "provision", forbidden)
    monkeypatch.setattr(cli, "add_worktree", forbidden)
    monkeypatch.setattr(subprocess, "run", forbidden)
    monkeypatch.setattr(subprocess, "Popen", forbidden)
    _synth_results(pathlib.Path(cli.RESULTS))
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert cli.main(["report", "--rebuild"]) == cli.EXIT_OK


def test_the_backfill_leaves_the_ground_truth_cache_byte_identical(offline_bench) -> None:
    """`RTDD_BENCH_CACHE` holds hundreds of megabytes of collected suites and full-suite
    outcomes this machine may never be able to produce again. It is keyed by a config
    digest a derivation does not change, and nothing in the derivation path reads or
    writes it — deleting or re-keying it is #218, an open human decision."""
    cache = pathlib.Path(cli.CACHE)
    (cache / "entries").mkdir(parents=True)
    (cache / "entries" / "ground-truth.json").write_text('{"collected": ["a"]}', encoding="utf-8")
    before = _tree_digest(cache)
    _synth_results(pathlib.Path(cli.RESULTS))
    assert cli.main(["derive"]) == cli.EXIT_OK
    assert cli.main(["report", "--rebuild"]) == cli.EXIT_OK
    assert _tree_digest(cache) == before
