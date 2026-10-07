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
