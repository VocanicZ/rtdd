from __future__ import annotations

import pathlib

import pytest

from replay.strategies.base import CommitContext
from replay.strategies.randomratio import RandomRatio, seeded_rng

ALL = tuple(f"tests/test_m.py::test_{i:02d}" for i in range(20))


def _ctx(peer_sizes, commit="c1", all_tests=ALL, variant="natural"):
    return CommitContext(
        repo_id="synth",
        variant=variant,
        commit=commit,
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
        peer_sizes=peer_sizes,
    )


def test_matches_the_peer_selection_size():
    sel = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    assert len(sel.tests) == 5
    assert set(sel.tests) <= set(ALL)
    assert sel.escalated is False


def test_is_deterministic_for_a_given_seed_and_commit():
    a = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    b = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    assert a.tests == b.tests
    c = RandomRatio(seed=7).select(_ctx({"rtdd": 5}, commit="c2"))
    assert c.tests != a.tests


def test_a_different_seed_gives_a_different_sample_at_the_same_commit():
    a = RandomRatio(seed=7).select(_ctx({"rtdd": 5}))
    b = RandomRatio(seed=8).select(_ctx({"rtdd": 5}))
    assert a.tests != b.tests


def test_the_two_variants_of_one_commit_are_sampled_independently():
    a = RandomRatio(seed=7).select(_ctx({"rtdd": 5}, variant="natural"))
    b = RandomRatio(seed=7).select(_ctx({"rtdd": 5}, variant="tdd"))
    assert a.tests != b.tests


def test_missing_peer_size_is_a_harness_error():
    with pytest.raises(RuntimeError, match="peer_sizes"):
        RandomRatio(seed=7).select(_ctx({}))


def test_a_peer_that_selected_more_than_the_suite_holds_is_clamped():
    sel = RandomRatio(seed=7).select(_ctx({"rtdd": 999}))
    assert sel.tests == tuple(sorted(ALL))


def test_a_peer_that_selected_nothing_samples_nothing():
    sel = RandomRatio(seed=7).select(_ctx({"rtdd": 0}))
    assert sel.tests == ()
    assert sel.escalated is False


def test_seeded_rng_is_stable():
    assert seeded_rng(1, "a", "b").random() == seeded_rng(1, "a", "b").random()
    assert seeded_rng(1, "a", "b").random() != seeded_rng(1, "a", "c").random()


def test_the_random_baseline_is_registered_and_needs_no_parent_state():
    from replay.strategies.base import get

    s = get("random")
    assert s.id == "random"
    assert s.needs_parent_state is False


def test_it_samples_real_ids_from_the_synthetic_repo(synth):
    import subprocess

    from replay.runner import collect
    from replay.strategies.base import validate_selection

    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    all_tests = collect(synth.path)
    ctx = _ctx({"rtdd": 2}, all_tests=all_tests)
    sel = RandomRatio(seed=7).select(ctx)
    assert len(sel.tests) == 2
    assert validate_selection(sel, ctx) is sel
