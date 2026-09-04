"""The committed results must cover the frozen corpus, not one repo of it.

`bench/results/` is committed on purpose (see `bench/results/README.md`): a number
that lives only on the machine that produced it cannot be reviewed. The same
argument applies to a *missing* number — a corpus that freezes three repos and a
results tree that holds one is a publication gate that never gated, and nothing in
the harness notices, because every module in it is scoped to a single repo.

This is the check that notices. It reads the frozen corpus and asserts that every
repo in it has a published, self-describing result beside the others, that the sole
cross-repo file spans the whole corpus, and that the `full` control reads perfect
change-level recall wherever a population had anything to detect — the plan's
ground-truth sanity gate. It asserts nothing about *which way* a verdict fell: the
criterion is pre-registered and the harness prints it win or lose.
"""

from __future__ import annotations

import json
import pathlib
import re

import pytest

from replay.corpus import load_corpus

BENCH = pathlib.Path(__file__).resolve().parents[1]
RESULTS = BENCH / "results"

#: Everything one repo's directory owes a reader. `drift.json` is in the list
#: because Task 26.5 runs a drift session per repo, not per corpus.
REQUIRED_FILES = ("commits.jsonl", "config.json", "drift.json", "summary.json", "summary.md")


def corpus_ids() -> tuple[str, ...]:
    return load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock").ids()


@pytest.mark.parametrize("repo_id", corpus_ids())
@pytest.mark.parametrize("name", REQUIRED_FILES)
def test_every_corpus_repo_has_every_published_file(repo_id: str, name: str) -> None:
    path = RESULTS / repo_id / name
    assert path.is_file(), f"{path.relative_to(BENCH)} is missing: the corpus freezes {repo_id}"
    assert path.stat().st_size > 0, f"{path.relative_to(BENCH)} is empty"


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_every_summary_carries_a_verdict_line(repo_id: str) -> None:
    text = (RESULTS / repo_id / "summary.md").read_text(encoding="utf-8")
    assert re.search(r"^verdict: .+$", text, re.MULTILINE), (
        f"{repo_id}/summary.md has no `verdict:` line; the criterion is unconditional"
    )


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_summary_is_for_the_repo_it_sits_under(repo_id: str) -> None:
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    assert summary["repo_id"] == repo_id


@pytest.mark.parametrize("repo_id", corpus_ids())
def test_full_control_recalls_everything_it_could_detect(repo_id: str) -> None:
    """26.3's ground-truth gate: running the whole suite cannot miss a failure.

    A population with no detecting commits has no denominator, and `n/a` is the
    honest reading of it — that case is asserted elsewhere, not softened here.
    """
    summary = json.loads((RESULTS / repo_id / "summary.json").read_text(encoding="utf-8"))
    for variant, per in summary["by_variant"].items():
        recall = per["full"]["change_level_recall"]
        if recall["den"] == 0:
            continue
        assert recall["value"] == pytest.approx(1.0), (
            f"{repo_id}/{variant}: the `full` control missed a failure the full suite "
            f"produced — {recall}"
        )


def test_aggregate_spans_the_whole_corpus() -> None:
    """The duration-weighted aggregate is the only cross-repo artifact; it must be one."""
    expected = len(corpus_ids())
    text = (RESULTS / "aggregate.md").read_text(encoding="utf-8")
    counts = {int(m) for m in re.findall(r"^\|\s*\S+\s*\|[^|]*\|\s*(\d+)\s*\|$", text, re.M)}
    assert counts, "aggregate.md has no strategy rows"
    assert counts == {expected}, (
        f"aggregate.md weights {sorted(counts)} repo(s); the frozen corpus has {expected}"
    )
