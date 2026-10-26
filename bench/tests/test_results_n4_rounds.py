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


#: The root README and the outcome file it is copied from (`outcomes_test.go`).
README_DOCS = ("README.md", "docs/outcomes/README.positive.md")


def _verdicts(repo_id: str) -> list[str]:
    text = (N4 / repo_id / "summary.md").read_text(encoding="utf-8")
    return [line for line in text.splitlines() if line.startswith("verdict")]


@pytest.mark.parametrize("rel", README_DOCS)
@pytest.mark.parametrize("repo_id", corpus_ids())
def test_the_readme_states_each_verdict_as_published(rel, repo_id):
    """#506: the v0.3.0 release states the 2026-10-22 verdicts on its front page, verbatim.

    A verdict that says "this must be stated in the README" is stated there in the words
    the run wrote, so a softened or paraphrased verdict fails here.
    """
    readme = (ROOT / rel).read_text(encoding="utf-8")
    verdicts = _verdicts(repo_id)
    assert verdicts, f"bench/results/n4/{repo_id}/summary.md has no verdict line"
    assert "docs/results/n4-bench-rounds.md" in readme, f"{rel} does not link the 2026-10-22 page"
    for verdict in verdicts:
        assert verdict in readme, f"{rel} does not state `{repo_id}`'s verdict as published: {verdict!r}"
