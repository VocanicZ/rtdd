"""The orchestrator, verified against the synthetic repo's hand-computed table.

`tests/synthrepo.py` carries the normative history: c1 breaks alpha, c2 reverts it
and adds gamma, c3 breaks beta. Every ground-truth assertion below is read off
that table, never off a previous run of this code.
"""

from __future__ import annotations

import sys

import pytest

from replay.cache import Cache
from replay.config import RunConfig
from replay.corpus import Corpus, RepoSpec, UnknownRepoError
from replay.hardware import probe
from replay.replay import (
    ReplayOptions,
    isolation_violation,
    replay_repo,
    strategy_order,
)

ADD = "tests/test_alpha.py::test_add"
MUL = "tests/test_beta.py::test_mul"
SUB = "tests/test_gamma.py::test_sub"


def _spec(synth, commits: int = 3, pin_index: int = 3) -> RepoSpec:
    return RepoSpec(
        id="synth",
        url="local",
        pin=synth.sha(pin_index),
        replay_commits=commits,
        python="3.12",
        install=(),
        source_globs=("src",),
        test_globs=("tests/**/*.py", "**/test_*.py", "**/conftest.py"),
    )


def _cfg(
    strategies: tuple[str, ...] = ("full", "path", "importgraph"),
    variants: tuple[str, ...] = ("natural",),
    commits: int = 3,
    seed: int = 1,
) -> RunConfig:
    return RunConfig(
        corpus_digest="test",
        rtdd_version="test",
        tool_versions=(("pytest", "test"),),
        strategies=strategies,
        variants=variants,
        replay_commits=commits,
        wallclock_sample=0,
        random_seed=seed,
    )


# --- pure parts -------------------------------------------------------------


def test_rtdd_runs_before_random_and_full_last():
    order = strategy_order(["random", "full", "path", "rtdd"])
    assert order[0] == "rtdd"
    assert order.index("random") > order.index("rtdd")
    assert order[-1] == "full"


def test_a_subset_that_reproduces_the_expected_failures_is_not_a_violation():
    assert not isolation_violation(
        observed_failing=frozenset({MUL}),
        selected=(MUL, SUB),
        f_full=(MUL,),
        pre_existing=frozenset(),
    )


def test_a_subset_that_hides_a_failure_it_selected_is_a_violation():
    assert isolation_violation(
        observed_failing=frozenset(),
        selected=(MUL, SUB),
        f_full=(MUL,),
        pre_existing=frozenset(),
    )


def test_a_pre_existing_failure_inside_the_subset_is_not_an_isolation_violation():
    """`F_full` already excludes the base tree's reds; the observed set must too,
    or every repo with one broken test would report a violation on every cycle."""
    assert not isolation_violation(
        observed_failing=frozenset({MUL, ADD}),
        selected=(MUL, ADD),
        f_full=(MUL,),
        pre_existing=frozenset({ADD}),
    )


# --- end to end -------------------------------------------------------------


def test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth(
    synth, tmp_path, cache_root
):
    cfg = _cfg()
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("path", "importgraph", "full"),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    by_commit = {c.commit: c for c in out.commits}
    assert set(by_commit) == {synth.sha(1), synth.sha(2), synth.sha(3)}

    c1 = by_commit[synth.sha(1)]
    assert c1.f_full == (ADD,)
    assert c1.stratum() == "1"
    assert c1.changed == ("src/alpha.py",)

    c2 = by_commit[synth.sha(2)]
    assert c2.f_full == ()  # green commit
    assert set(c2.changed) == {"src/alpha.py", "src/gamma.py", "tests/test_gamma.py"}

    c3 = by_commit[synth.sha(3)]
    assert c3.f_full == (MUL,)

    path_at_c3 = [
        s for s in out.strategies if s.commit == synth.sha(3) and s.strategy == "path"
    ][0]
    assert set(path_at_c3.selected) == {MUL, SUB}

    full_at_c3 = [
        s for s in out.strategies if s.commit == synth.sha(3) and s.strategy == "full"
    ][0]
    assert set(full_at_c3.selected) == set(c3.all_tests)
    assert full_at_c3.escalated is True


def test_f_full_subtracts_the_failures_already_red_at_the_clean_parent(
    synth, tmp_path, cache_root
):
    """c2 is green, but its parent c1 is red. The replay must not credit c2 with
    inheriting that red, and must still record it."""
    cfg = _cfg(strategies=("full",), commits=1)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth, commits=1, pin_index=2),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("full",),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    assert len(out.commits) == 1
    rec = out.commits[0]
    assert rec.commit == synth.sha(2)
    assert rec.pre_existing_failures == (ADD,)
    assert rec.f_full == ()


def test_the_two_populations_are_tagged_and_never_pooled(synth, tmp_path, cache_root):
    cfg = _cfg(strategies=("full",), variants=("natural", "probe"), commits=1)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth, commits=1, pin_index=3),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural", "probe"),
            strategy_ids=("full",),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    by_variant = {c.variant: c for c in out.commits}
    assert set(by_variant) == {"natural", "probe"}

    natural = by_variant["natural"]
    assert natural.f_full == (MUL,)
    assert natural.pre_existing_failures == ()

    # The probe reverts c3's source half back to c2, so the tree goes green while
    # the clean child tree it was built from is red.
    probe_rec = by_variant["probe"]
    assert probe_rec.pre_existing_failures == (MUL,)
    assert probe_rec.f_full == ()

    assert {s.variant for s in out.strategies} == {"natural", "probe"}


def test_every_cycle_produces_a_strategy_record_for_every_strategy(
    synth, tmp_path, cache_root
):
    cfg = _cfg(strategies=("full", "path"), commits=3)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("path", "full"),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    assert len(out.commits) == 3
    assert len(out.strategies) == 6
    assert {(s.commit, s.strategy) for s in out.strategies} == {
        (c.commit, sid) for c in out.commits for sid in ("path", "full")
    }
    # The escalating baseline is recorded as a cycle, not dropped for escalating.
    assert all(s.escalated for s in out.strategies if s.strategy == "full")


def test_a_commit_that_collects_nothing_is_skipped_and_the_replay_continues(
    synth, tmp_path, cache_root, monkeypatch
):
    from replay import replay as replay_mod

    real_collect = replay_mod.collect

    def collect_nothing_at_c2(work, python=sys.executable):
        if work.name.endswith(synth.sha(2)[:8]):
            return ()  # what pytest gives back when the tree will not import
        return real_collect(work, python=python)

    monkeypatch.setattr(replay_mod, "collect", collect_nothing_at_c2)

    cfg = _cfg(strategies=("full",), commits=3)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("full",),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    assert [s["commit"] for s in out.skipped] == [synth.sha(2)]
    assert out.skipped[0]["reason"] == "collect-error-or-empty"
    assert {c.commit for c in out.commits} == {synth.sha(1), synth.sha(3)}


# --- wall clock -------------------------------------------------------------


def test_wall_clock_captures_all_three_forms_and_the_isolation_check(
    synth, tmp_path, cache_root
):
    cfg = _cfg(strategies=("full", "path"), commits=1)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth, commits=1, pin_index=3),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("path", "full"),
            wallclock_sample=1,
            wallclock_enabled=True,
        ),
    )

    by_strategy = {w.strategy: w for w in out.wallclocks}
    assert set(by_strategy) == {"path", "full"}
    for rec in by_strategy.values():
        assert rec.full_uninstrumented_ms is not None and rec.full_uninstrumented_ms > 0
        assert rec.subset_uninstrumented_ms is not None
        assert rec.subset_instrumented_ms is not None
        assert rec.isolation_violation is False
        assert rec.hardware_fingerprint == probe({}).fingerprint()


def test_wall_clock_is_suppressed_on_ci(synth, tmp_path, cache_root):
    cfg = _cfg(strategies=("full",), commits=1)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth, commits=1, pin_index=3),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({"GITHUB_ACTIONS": "true"}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("full",),
            wallclock_sample=1,
            wallclock_enabled=True,
        ),
    )

    assert out.wallclocks == []
    assert len(out.commits) == 1


# --- guards and caching -----------------------------------------------------


def test_a_repo_outside_the_frozen_corpus_is_refused(synth, tmp_path, cache_root):
    spec = _spec(synth)
    corpus = Corpus(
        frozen_at="2026-01-01",
        criteria=(),
        repos={"something-else": spec},
        excluded=(),
        digest="test",
    )
    cfg = _cfg(strategies=("full",))
    with pytest.raises(UnknownRepoError):
        replay_repo(
            repo=synth.path,
            spec=spec,
            cfg=cfg,
            cache=Cache(cache_root, cfg.digest()),
            hw=probe({}),
            work_root=tmp_path / "work",
            opts=ReplayOptions(
                variants=("natural",),
                strategy_ids=("full",),
                wallclock_sample=0,
                wallclock_enabled=False,
            ),
            corpus=corpus,
        )
    assert not (tmp_path / "work").exists(), "refused before any worktree was added"


def test_a_second_replay_is_served_from_cache(synth, tmp_path, cache_root):
    spec = RepoSpec("synth", "local", synth.sha(3), 2, "3.12", (), ("src",), ("tests/**/*.py",))
    cfg = RunConfig("test", "test", (("pytest", "t"),), ("full",), ("natural",), 2, 0, 1)
    cache = Cache(cache_root, cfg.digest())
    opts = ReplayOptions(
        variants=("natural",), strategy_ids=("full",), wallclock_sample=0, wallclock_enabled=False
    )
    kwargs = dict(repo=synth.path, spec=spec, cfg=cfg, cache=cache, hw=probe({}), opts=opts)
    replay_repo(work_root=tmp_path / "w1", **kwargs)
    first = cache.stats()["misses"]
    replay_repo(work_root=tmp_path / "w2", **kwargs)
    assert cache.stats()["misses"] == first, "the second pass must be fully cached"


def test_a_config_change_is_a_cache_miss_not_a_silent_mix(synth, tmp_path, cache_root):
    spec = _spec(synth, commits=1, pin_index=3)
    opts = ReplayOptions(
        variants=("natural",), strategy_ids=("full",), wallclock_sample=0, wallclock_enabled=False
    )
    first_cfg = _cfg(strategies=("full",), commits=1, seed=1)
    replay_repo(
        repo=synth.path, spec=spec, cfg=first_cfg,
        cache=Cache(cache_root, first_cfg.digest()), hw=probe({}),
        work_root=tmp_path / "w1", opts=opts,
    )

    second_cfg = _cfg(strategies=("full",), commits=1, seed=99)
    assert second_cfg.digest() != first_cfg.digest()
    second_cache = Cache(cache_root, second_cfg.digest())
    replay_repo(
        repo=synth.path, spec=spec, cfg=second_cfg, cache=second_cache, hw=probe({}),
        work_root=tmp_path / "w2", opts=opts,
    )
    assert second_cache.stats()["hits"] == 0, "a config change must never read old entries"
    assert second_cache.stats()["misses"] > 0


# --- a base tree that stays red ---------------------------------------------


def _repo_whose_red_survives_the_child(root):
    """c0 ships one already-failing test; c1 breaks a second one.

    The synth repo never has a red that outlives its commit, so it cannot tell a
    subtracted `F_full` from an unsubtracted one. This history can.
    """
    from tests.synthrepo import _commit, _git, _write

    root.mkdir(parents=True, exist_ok=True)
    _git(root, "init", "-q", "-b", "main")
    _git(root, "config", "user.name", "bench")
    _git(root, "config", "user.email", "bench@example.invalid")

    _write(root, "pytest.ini", "[pytest]\ntestpaths = tests\n")
    _write(root, "src/__init__.py", "")
    _write(root, "src/a.py", "def f():\n    return 1\n")
    _write(root, "tests/test_a.py", "from src.a import f\n\n\ndef test_f():\n    assert f() == 1\n")
    _write(root, "tests/test_broken.py", "def test_broken():\n    assert False\n")
    c0 = _commit(root, "c0: one test already red")

    _write(root, "src/a.py", "def f():\n    return 2\n")
    c1 = _commit(root, "c1: break the second test too")
    return c0, c1


def test_f_full_excludes_a_red_that_outlives_the_parent(tmp_path, cache_root):
    root = tmp_path / "reds"
    _c0, c1 = _repo_whose_red_survives_the_child(root)
    spec = RepoSpec("reds", "local", c1, 1, "3.12", (), ("src",), ("tests/**/*.py",))
    cfg = _cfg(strategies=("full",), commits=1)
    out = replay_repo(
        repo=root,
        spec=spec,
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("full",),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    rec = out.commits[0]
    assert rec.pre_existing_failures == ("tests/test_broken.py::test_broken",)
    assert rec.f_full == ("tests/test_a.py::test_f",)


def test_state_a_strategy_built_in_the_base_tree_is_not_reported_as_a_change(
    synth, tmp_path, cache_root
):
    """testmon's prepare() leaves `.testmondata` in the worktree, and nothing in an
    arbitrary repo gitignores it. Feeding it to the strategies as a changed path
    would invent work no commit asked for."""
    cfg = _cfg(strategies=("testmon", "full"), commits=1)
    out = replay_repo(
        repo=synth.path,
        spec=_spec(synth, commits=1, pin_index=1),
        cfg=cfg,
        cache=Cache(cache_root, cfg.digest()),
        hw=probe({}),
        work_root=tmp_path / "work",
        opts=ReplayOptions(
            variants=("natural",),
            strategy_ids=("testmon", "full"),
            wallclock_sample=0,
            wallclock_enabled=False,
        ),
    )

    assert out.commits[0].changed == ("src/alpha.py",)
