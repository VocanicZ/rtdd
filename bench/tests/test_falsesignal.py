from __future__ import annotations

import subprocess

import pytest

from replay.covread import read_coverage
from replay.falsesignal import (
    build_record,
    change_false_signal_rate,
    fire_rate,
    line_false_signal_rate,
    summarise,
)
from replay.metrics import PoolingError
from replay.runner import run_full


def _rec(cid, reported, truth, repo_id="synth"):
    return build_record(repo_id, cid, "natural", frozenset(reported), frozenset(truth))


# --- the record -------------------------------------------------------------


def test_record_counts_hits_against_truth():
    r = _rec("c1", {("src/a.py", 1), ("src/a.py", 2)}, {("src/a.py", 1)})
    assert r.reported_count == 2
    assert r.truth_covered_hits == 1


def test_reported_pairs_are_sorted_so_the_record_line_is_byte_stable():
    r = _rec("c1", [("src/b.py", 2), ("src/a.py", 9), ("src/a.py", 1)], set())
    assert r.reported == (("src/a.py", 1), ("src/a.py", 9), ("src/b.py", 2))


# --- the two rates ----------------------------------------------------------


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


def test_a_rate_with_no_fired_commit_has_no_value_rather_than_a_zero():
    silent = _rec("c1", set(), {("src/a.py", 1)})
    assert change_false_signal_rate([silent]).value() is None
    assert line_false_signal_rate([silent]).value() is None


# --- the summary ------------------------------------------------------------


def test_summarise_reports_fired_silent_and_all_three_rates():
    silent = _rec("c1", set(), set())
    wholly_wrong = _rec("c2", {("src/a.py", 1)}, {("src/a.py", 1)})
    partly_right = _rec("c3", {("src/a.py", 1), ("src/a.py", 2)}, {("src/a.py", 1)})
    out = summarise([silent, wholly_wrong, partly_right], total_cycles=4)

    assert out["repo_id"] == "synth"
    assert (out["cycles"], out["fired"], out["silent"]) == (4, 2, 1)
    assert out["fire_rate"]["value"] == pytest.approx(0.5)
    assert out["change_false_signal_rate"] == {"num": 1, "den": 2, "value": 0.5}
    assert out["line_false_signal_rate"] == {"num": 2, "den": 3, "value": pytest.approx(2 / 3)}


# --- the never-pool rule ----------------------------------------------------


def test_every_published_rate_refuses_to_pool_across_repos():
    a = _rec("c1", {("src/a.py", 1)}, set(), repo_id="alpha")
    b = _rec("c1", {("src/a.py", 1)}, set(), repo_id="beta")
    for call in (
        lambda: change_false_signal_rate([a, b]),
        lambda: line_false_signal_rate([a, b]),
        lambda: fire_rate([a, b], total_cycles=2),
        lambda: summarise([a, b], total_cycles=2),
    ):
        with pytest.raises(PoolingError):
            call()


# --- against the synthetic repo, hand-computed ------------------------------


def test_false_signal_rate_against_the_synthetic_repo(synth):
    """Hand-computed at `c2`, whose tree is green and fully covered.

    `src/alpha.py` is two lines: line 1 `def add(a, b):` runs only at import, line
    2 `return a + b` runs under `test_add`. `src/gamma.py` line 2 runs under
    `test_sub`. Line 99 of `alpha.py` does not exist and is in nothing.

    Report those four lines as Uncovered and exactly two of them — the two real
    per-test lines — are false signals. The import-time line is **not**: spec §6
    says a line that only ran at import was asserted on by nobody, so calling it
    "in fact adequately tested" would manufacture the very signal being measured.
    """
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    res = run_full(synth.path, instrumented=True, source_globs=("src",))
    assert res.exit_code == 0
    truth = read_coverage(synth.path / ".coverage", synth.path)

    import_time_only = ("src/alpha.py", 1)
    per_test = ("src/alpha.py", 2)
    other_per_test = ("src/gamma.py", 2)
    absent = ("src/alpha.py", 99)
    assert import_time_only in truth.import_time and import_time_only not in truth.covered

    rec = build_record(
        "synth",
        synth.sha(2),
        "natural",
        {import_time_only, per_test, other_per_test, absent},
        truth.covered,
    )

    assert (rec.reported_count, rec.truth_covered_hits) == (4, 2)
    assert line_false_signal_rate([rec]).value() == pytest.approx(0.5)
    # fired, but not wholly wrong: two of the four reported lines really are untested
    assert change_false_signal_rate([rec]).value() == pytest.approx(0.0)
    assert fire_rate([rec], total_cycles=4).value() == pytest.approx(0.25)


def test_truth_is_only_the_instrumented_run_never_the_reported_set(synth):
    """A report that fires on nothing but import-time lines is wholly *right*."""
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    assert run_full(synth.path, instrumented=True, source_globs=("src",)).exit_code == 0
    truth = read_coverage(synth.path / ".coverage", synth.path)

    rec = build_record("synth", synth.sha(2), "natural", truth.import_time, truth.covered)

    assert rec.reported_count == len(truth.import_time) > 0
    assert rec.truth_covered_hits == 0
    assert line_false_signal_rate([rec]).value() == pytest.approx(0.0)
