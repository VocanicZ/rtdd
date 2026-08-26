"""The kill switch.

A benchmark that can run away is a benchmark that will, so the ceilings are
tested for the property that matters at 3am: ``add`` records the tokens it was
given *before* it refuses, so the cost file written on the way out names the
spend that actually happened rather than the spend up to the previous instance.
"""

from __future__ import annotations

import pytest

import budget as budget_mod
from budget import Budget, BudgetExceeded


def test_usd_is_the_two_per_million_prices_applied_to_the_two_counts():
    b = Budget(usd_per_m_prompt=0.10, usd_per_m_completion=0.40)
    b.start()
    b.add(2_000_000, 1_000_000)
    assert b.usd == pytest.approx(0.2 + 0.4)


def test_add_accumulates_across_calls():
    b = Budget()
    b.start()
    b.add(10, 20)
    b.add(5, 7)
    assert (b.prompt_tokens, b.completion_tokens) == (15, 27)


def test_add_raises_when_the_spend_cap_is_crossed():
    b = Budget(max_usd=0.05, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    b.start()
    b.add(10_000, 10_000)  # $0.02 — under
    with pytest.raises(BudgetExceeded, match=r"exceeded cap \$0.05"):
        b.add(40_000, 0)  # $0.06 — over


def test_the_tokens_that_crossed_the_cap_are_still_counted():
    # The cost file is written after this raise. An overrun that is not recorded
    # is an overrun that is not published.
    b = Budget(max_usd=0.01, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    b.start()
    with pytest.raises(BudgetExceeded):
        b.add(90_000, 10_000)
    assert (b.prompt_tokens, b.completion_tokens) == (90_000, 10_000)
    assert b.snapshot()["usd_estimated"] == pytest.approx(0.1)


def test_add_raises_when_the_wall_clock_cap_is_crossed(monkeypatch):
    clock = [1000.0]
    monkeypatch.setattr(budget_mod.time, "time", lambda: clock[0])
    b = Budget(max_usd=1000.0, max_wall_hours=2.0)
    b.start()
    clock[0] += 3600.0
    b.add(1, 1)
    clock[0] += 3600.0 * 1.5
    with pytest.raises(BudgetExceeded, match="wall clock"):
        b.add(1, 1)


def test_snapshot_records_the_assumed_prices_beside_the_actual_counts():
    b = Budget(usd_per_m_prompt=0.10, usd_per_m_completion=0.40)
    b.start()
    b.add(1_000_000, 500_000)
    snap = b.snapshot()
    assert snap["prompt_tokens"] == 1_000_000
    assert snap["completion_tokens"] == 500_000
    assert snap["usd_per_m_prompt"] == 0.10
    assert snap["usd_per_m_completion"] == 0.40
    assert snap["usd_estimated"] == pytest.approx(0.30)
    assert snap["wall_hours"] >= 0.0


def test_snapshot_is_json_serialisable():
    import json

    b = Budget()
    b.start()
    b.add(3, 4)
    assert json.loads(json.dumps(b.snapshot()))["prompt_tokens"] == 3


# --- resuming an arm ------------------------------------------------------


def test_restore_adopts_a_previous_runs_spend_so_a_resume_tops_it_up():
    # Otherwise --max-usd is a per-invocation ceiling: a run stopped at the cap
    # and resumed would be allowed to spend the whole cap again.
    b = Budget()
    b.restore({"prompt_tokens": 3_000, "completion_tokens": 300, "wall_hours": 1.5})
    b.start()
    b.add(1_000, 100)
    assert (b.prompt_tokens, b.completion_tokens) == (4_000, 400)


def test_restore_carries_the_previous_runs_wall_clock(monkeypatch):
    clock = [1000.0]
    monkeypatch.setattr(budget_mod.time, "time", lambda: clock[0])
    b = Budget(max_wall_hours=2.0)
    b.restore({"prompt_tokens": 0, "completion_tokens": 0, "wall_hours": 1.5})
    b.start()
    clock[0] += 3600.0
    assert b.wall_hours == pytest.approx(2.5)
    with pytest.raises(BudgetExceeded, match="wall clock"):
        b.add(1, 1)


def test_a_restored_budget_snapshots_the_whole_benchmarks_spend():
    b = Budget()
    b.restore({"prompt_tokens": 3_000, "completion_tokens": 300, "wall_hours": 0.25})
    b.start()
    b.add(1_000, 100)
    snap = b.snapshot()
    assert snap["prompt_tokens"] == 4_000
    assert snap["completion_tokens"] == 400
    assert snap["wall_hours"] >= 0.25


def test_restore_of_an_empty_snapshot_changes_nothing():
    b = Budget()
    b.restore({})
    assert (b.prompt_tokens, b.completion_tokens, b.wall_hours) == (0, 0, 0.0)


def test_check_refuses_a_resume_that_is_already_over_the_cap():
    # The refusal has to come before the next instance, not after it.
    b = Budget(max_usd=0.01, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    b.restore({"prompt_tokens": 90_000, "completion_tokens": 10_000})
    b.start()
    with pytest.raises(BudgetExceeded, match=r"exceeded cap"):
        b.check()


def test_check_is_silent_while_both_ceilings_hold():
    b = Budget()
    b.start()
    b.add(10, 10)
    assert b.check() is None
