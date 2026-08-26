from __future__ import annotations

import subprocess

import pytest

from replay.metrics import (
    PoolingError,
    assert_single_repo,
    by_stratum,
    change_level_recall,
    detected,
    escalation_rate,
    pair_up,
    selected_duration_fraction,
    selection_ratio,
    summarise,
)
from replay.metrics import test_level_recall_macro as macro_recall
from replay.metrics import test_level_recall_micro as micro_recall
from replay.records import CommitRecord, StrategyRecord
from replay.runner import run_full
from tests.synthrepo import EXPECTED

ALL = ("a", "b", "c", "d")
DUR = {"a": 100, "b": 100, "c": 100, "d": 700}


def _commit(cid, f_full, all_tests=ALL, durations=None, repo_id="synth"):
    return CommitRecord(
        repo_id=repo_id,
        commit=cid,
        parent="p",
        variant="natural",
        all_tests=tuple(all_tests),
        durations_ms=dict(DUR if durations is None else durations),
        f_full=tuple(f_full),
        pre_existing_failures=(),
        changed=("src/x.py",),
    )


def _sel(cid, selected, escalated=False, strategy="rtdd", repo_id="synth"):
    return StrategyRecord(
        repo_id, cid, "natural", strategy, tuple(selected), escalated, "T0", 1
    )


# The hand-computed fixture from docs/plans/04-m3-replay-benchmark.md Task 20.
#
# | commit | f_full  | stratum    | selected | ∩ | |f_full| |
# | c1     | {a}     | 1          | {a, b}   | 1 | 1        |
# | c2     | {a,b,c} | 2-5        | {b, c}   | 2 | 3        |
# | c3     | {d}     | 1          | {a}      | 0 | 1        |
# | c4     | {}      | 0 (green)  | {a}      | — | —        |
COMMITS = [_commit("c1", "a"), _commit("c2", "abc"), _commit("c3", "d"), _commit("c4", "")]
SELS = [
    _sel("c1", "ab"),
    _sel("c2", "bc"),
    _sel("c3", "a"),
    _sel("c4", "a"),
]


# --- the never-pool rule ----------------------------------------------------


def test_pooling_across_repos_is_refused():
    other = CommitRecord("otherrepo", "c9", "p", "natural", ALL, DUR, (), (), ())
    with pytest.raises(PoolingError):
        assert_single_repo([*COMMITS, other])


def test_pair_up_refuses_mixed_repo_records():
    with pytest.raises(PoolingError):
        pair_up(COMMITS, [*SELS, _sel("c1", "a", repo_id="otherrepo")], "rtdd")


def test_assert_single_repo_returns_the_shared_id_and_empty_for_no_records():
    assert assert_single_repo(COMMITS) == "synth"
    assert assert_single_repo([]) == ""


# --- detection --------------------------------------------------------------


def test_detection_needs_a_non_empty_intersection_with_f_full():
    rec = _commit("c1", "a")
    assert detected(rec, _sel("c1", "ab"))
    assert not detected(rec, _sel("c1", "bcd"))


def test_a_selected_pre_existing_failure_is_not_a_detection():
    # the test was already red at the clean base tree, so it is not in f_full
    rec = CommitRecord("synth", "c1", "p", "natural", ALL, DUR, (), ("b",), ())
    assert not detected(rec, _sel("c1", "b"))


# --- recall -----------------------------------------------------------------


def test_change_and_test_level_recall_match_the_hand_computation():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    cl = change_level_recall(pairs)
    assert (cl.num, cl.den) == (2, 3)
    micro = micro_recall(pairs)
    assert (micro.num, micro.den) == (3, 5)
    macro = macro_recall(pairs)
    assert macro.value() == pytest.approx(5 / 9)


def test_green_commits_are_excluded_from_every_recall_denominator():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    assert change_level_recall(pairs).den == 3  # c4 is green, four commits paired
    assert macro_recall(pairs).den == 3


def test_recall_over_an_all_green_population_is_none_not_zero():
    greens = [_commit("g1", ""), _commit("g2", "")]
    pairs = pair_up(greens, [_sel("g1", "a"), _sel("g2", "a")], "rtdd")
    assert change_level_recall(pairs).value() is None
    assert micro_recall(pairs).value() is None


# --- strata -----------------------------------------------------------------


def test_single_killer_stratum_is_broken_out():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    strata = by_stratum(pairs)
    assert set(strata) == {"1", "2-5"}
    one = change_level_recall(strata["1"])
    assert (one.num, one.den) == (1, 2)
    assert micro_recall(strata["1"]).value() == pytest.approx(0.5)
    assert change_level_recall(strata["2-5"]).value() == pytest.approx(1.0)


def test_the_single_killer_stratum_is_reported_first():
    wide = [*COMMITS, _commit("c5", tuple(f"t{i}" for i in range(24)))]
    sels = [*SELS, _sel("c5", "a")]
    strata = by_stratum(pair_up(wide, sels, "rtdd"))
    assert list(strata) == ["1", "2-5", "21+"]


# --- cost -------------------------------------------------------------------


def test_selection_ratio_and_duration_fraction_differ_on_a_heavy_tail():
    # every commit selects 2 of 4 tests, but 'd' alone is 70% of the suite's duration
    pairs = pair_up(COMMITS, SELS, "rtdd")
    assert selection_ratio(pairs).value() == pytest.approx((2 + 2 + 1 + 1) / (4 * 4))
    # selected duration: c1 200, c2 200, c3 100, c4 100 out of 1000 each
    assert selected_duration_fraction(pairs).value() == pytest.approx(600 / 4000)


def test_cost_metrics_include_the_green_population():
    pairs = pair_up(COMMITS, SELS, "rtdd")
    assert selection_ratio(pairs).den == 16  # all four commits, green one included
    assert selected_duration_fraction(pairs).den == 4000


# --- escalation -------------------------------------------------------------


def test_escalation_rate_counts_every_cycle():
    sels = [_sel("c1", "abcd", escalated=True), _sel("c2", "b"), _sel("c3", "a"), _sel("c4", "a")]
    assert escalation_rate(sels).value() == pytest.approx(0.25)


def test_escalation_rate_refuses_mixed_repo_records():
    with pytest.raises(PoolingError):
        escalation_rate([_sel("c1", "a"), _sel("c1", "a", repo_id="otherrepo")])


# --- the summary ------------------------------------------------------------


def test_summarise_reports_both_recalls_the_duration_fraction_and_the_strata():
    s = summarise(COMMITS, SELS, "rtdd")
    assert s["strategy"] == "rtdd"
    assert s["repo_id"] == "synth"
    assert (s["cycles"], s["detecting_commits"], s["green_commits"]) == (4, 3, 1)
    assert s["change_level_recall"]["value"] == pytest.approx(2 / 3)
    assert s["test_level_recall_micro"]["value"] == pytest.approx(3 / 5)
    assert s["test_level_recall_macro"]["value"] == pytest.approx(5 / 9)
    assert s["selected_duration_fraction"]["value"] == pytest.approx(600 / 4000)
    assert s["escalation_rate"]["value"] == pytest.approx(0.0)
    assert list(s["strata"]) == ["1", "2-5"]
    assert s["strata"]["1"]["n"] == 2


def test_summarise_scores_only_the_named_strategy():
    sels = [*SELS, _sel("c1", "abcd", strategy="all", escalated=True)]
    assert summarise(COMMITS, sels, "rtdd")["escalation_rate"] == {
        "num": 0,
        "den": 4,
        "value": 0.0,
    }
    assert summarise(COMMITS, sels, "all")["escalation_rate"]["value"] == pytest.approx(1.0)


# --- verified against the synthetic repo ------------------------------------


def _checkout(repo, sha):
    subprocess.run(["git", "checkout", "-q", sha], cwd=repo, check=True)


def _records_from_synth(synth):
    """Replay c1..c3 of the synthetic repo into real `CommitRecord`s."""
    out = []
    for expected in EXPECTED[1:]:
        _checkout(synth.path, synth.sha(expected.index))
        res = run_full(synth.path)
        out.append(
            CommitRecord(
                repo_id="synth",
                commit=synth.sha(expected.index),
                parent=synth.sha(expected.index - 1),
                variant="natural",
                all_tests=tuple(sorted(o.test for o in res.outcomes)),
                durations_ms=res.durations(),
                f_full=tuple(sorted(res.failing())),
                pre_existing_failures=(),
                changed=(),
            )
        )
    return out


def test_metrics_match_hand_computed_numbers_on_the_synthetic_repo(synth):
    commits = _records_from_synth(synth)
    # c1 breaks test_add, c2 is green, c3 breaks test_mul — synthrepo's normative table
    assert [c.f_full for c in commits] == [e.f_full for e in EXPECTED[1:]]
    # c1 collects two tests, c2 and c3 three each
    assert [len(c.all_tests) for c in commits] == [2, 3, 3]

    perfect = [_sel(c.commit, c.f_full or ("tests/test_gamma.py::test_sub",)) for c in commits]
    blind = [_sel(c.commit, ("tests/test_alpha.py::test_add",)) for c in commits]

    p = pair_up(commits, perfect, "rtdd")
    assert (change_level_recall(p).num, change_level_recall(p).den) == (2, 2)
    assert (micro_recall(p).num, micro_recall(p).den) == (2, 2)

    b = pair_up(commits, blind, "rtdd")
    # detects c1's test_add, misses c3's test_mul; green c2 scores neither way
    assert (change_level_recall(b).num, change_level_recall(b).den) == (1, 2)
    assert (micro_recall(b).num, micro_recall(b).den) == (1, 2)
    # but c2 still pays for what it selected: 1 of 2 + 1 of 3 + 1 of 3 tests
    assert (selection_ratio(b).num, selection_ratio(b).den) == (3, 8)
    # both single-failure commits land in the |F_full| == 1 stratum
    assert list(by_stratum(b)) == ["1"]
    assert by_stratum(b)["1"] and len(by_stratum(b)["1"]) == 2
