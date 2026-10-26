"""Issue #508: the before/after page is held to the two records it was written from.

Every figure docs/results/i508-scoped-resolution.md prints for `flask` must be the one
`report.fmt` renders from bench/results/n4/flask (before) or bench/results/i508/flask
(after). Nothing asserts which way a number falls.
"""
from __future__ import annotations

import json
import pathlib

import pytest

from replay.report import fmt

BENCH = pathlib.Path(__file__).resolve().parents[1]
PAGE = BENCH.parent / "docs" / "results" / "i508-scoped-resolution.md"
RUNS = {"before": BENCH / "results" / "n4" / "flask", "after": BENCH / "results" / "i508" / "flask"}
ARMS = ("rtdd", "rtdd-r12", "path")
KEYS = ("change_level_recall", "test_level_recall_micro", "selection_ratio", "selected_duration_fraction")


def _section(page: str, variant: str) -> str:
    start = page.index(f"### `{variant}`")
    end = page.find("\n### ", start + 1)
    return page[start:] if end < 0 else page[start:end]


@pytest.mark.parametrize("variant", ("natural", "probe"))
@pytest.mark.parametrize("run", tuple(RUNS))
def test_each_row_quotes_its_summary(variant, run):
    section = _section(PAGE.read_text(encoding="utf-8"), variant)
    strategies = json.loads((RUNS[run] / "summary.json").read_text(encoding="utf-8"))["by_variant"][variant]
    for arm in ARMS:
        rows = [l for l in section.splitlines() if l.startswith(f"| `{arm}` —") and f"| {run} |" in l]
        assert len(rows) == 1, f"{variant}/{arm}/{run}: want one row, got {len(rows)}"
        for key in KEYS:
            assert fmt(strategies[arm][key]) in rows[0], f"{variant}/{arm}/{run}: {key} differs from summary.json"


def test_the_page_states_both_binaries_and_hardware():
    page = PAGE.read_text(encoding="utf-8")
    for d in RUNS.values():
        cfg = json.loads((d / "config.json").read_text(encoding="utf-8"))
        assert cfg["config"]["rtdd_version"] in page
        assert cfg["config"]["corpus_digest"][:12] in page
        assert cfg["hardware"]["fingerprint"] in page
