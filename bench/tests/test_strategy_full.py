"""Baseline 7 — the full suite, the ground-truth strategy.

The control arm selects everything, so its recall is 1.00 by construction. What it
is *not* allowed to do is hide that it got there by giving up: every cycle is
counted including escalations (Global Constraints), and the full suite is 100%
escalation by definition. Publishing that as `1.00` keeps the escalation column
honest instead of special-casing the control.
"""

from __future__ import annotations

import pathlib

from replay.gitwork import Change, diff_changes
from replay.strategies.base import REGISTRY, CommitContext, get, validate_selection
from replay.strategies.full import FullSuite


def _ctx(all_tests, changed=()):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=tuple(Change(p, s) for p, s in changed),
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


def test_full_preserves_the_collected_order_and_selects_only_known_ids():
    ctx = _ctx(("b::y", "a::x"))
    sel = FullSuite().select(ctx)
    assert sel.tests == ("b::y", "a::x")
    assert validate_selection(sel, ctx) is sel


def test_empty_suite_still_escalates():
    sel = FullSuite().select(_ctx(()))
    assert sel.tests == ()
    assert sel.escalated is True


def test_the_changed_set_does_not_narrow_the_control_arm():
    sel = FullSuite().select(_ctx(("a::x", "b::y"), changed=[("src/alpha.py", "M")]))
    assert sel.tests == ("a::x", "b::y")


def test_full_passes_no_tuning_flags():
    assert FullSuite().select(_ctx(("a::x",))).exec_args == ()


def test_full_is_registered_under_the_id_the_plan_fixes():
    assert FullSuite.id == "full"
    assert FullSuite.needs_parent_state is False
    assert isinstance(get("full"), FullSuite)
    assert REGISTRY["full"] is get("full")


# --- verified against the synthetic repo, hand-computed per commit -------------
#
# The full suite selects every collected test id at every commit, so the expected
# selection is just the collected list — which grows when c2 adds gamma.
ALPHA = "tests/test_alpha.py::test_add"
BETA = "tests/test_beta.py::test_mul"
GAMMA = "tests/test_gamma.py::test_sub"

SYNTH_EXPECTED = {
    1: ((ALPHA, BETA), (ALPHA, BETA)),
    2: ((ALPHA, BETA, GAMMA), (ALPHA, BETA, GAMMA)),
    3: ((ALPHA, BETA, GAMMA), (ALPHA, BETA, GAMMA)),
}


def test_full_selection_over_the_synth_repo_matches_the_hand_computed_table(synth):
    for index, (collected, expected) in SYNTH_EXPECTED.items():
        ctx = CommitContext(
            repo_id="synth",
            variant="natural",
            commit=synth.sha(index),
            parent=synth.sha(index - 1),
            work=synth.path,
            changed=tuple(diff_changes(synth.path, synth.sha(index - 1), synth.sha(index))),
            all_tests=collected,
            source_globs=("src/**/*.py",),
            test_globs=("tests/**/*.py", "**/test_*.py"),
            python="python",
        )
        sel = FullSuite().select(ctx)
        assert sel.tests == expected, f"c{index}"
        assert sel.escalated is True, f"c{index}"
        validate_selection(sel, ctx)
