from __future__ import annotations

import dataclasses
import pathlib

import pytest

from replay.gitwork import Change
from replay.strategies import base
from replay.strategies.base import (
    CommitContext,
    Selection,
    Strategy,
    UnknownTestError,
    all_ids,
    get,
    register,
    validate_selection,
)


@pytest.fixture(autouse=True)
def _isolated_registry():
    """The registry is process-global; keep one test's strategies out of another's.

    It is cleared on the way in as well as restored on the way out: importing any
    real strategy module registers it for the whole session, and these tests
    describe the registry's own behaviour, not whichever baselines a sibling test
    module happened to import first.
    """
    saved = dict(base.REGISTRY)
    base.REGISTRY.clear()
    try:
        yield
    finally:
        base.REGISTRY.clear()
        base.REGISTRY.update(saved)


class _Dummy:
    id = "dummy"
    needs_parent_state = False

    def prepare(self, ctx):
        return None

    def select(self, ctx):
        return Selection(tests=ctx.all_tests[:1], escalated=False, reason="first")


class _Seeded(_Dummy):
    id = "seeded"
    needs_parent_state = True


def _ctx(**over):
    kwargs = dict(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=("a::x", "b::y"),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )
    kwargs.update(over)
    return CommitContext(**kwargs)


def test_register_and_get():
    register(_Dummy())
    assert "dummy" in all_ids()
    s = get("dummy")
    assert s.select(_ctx()).tests == ("a::x",)


def test_unknown_strategy_is_an_error():
    with pytest.raises(KeyError):
        get("no-such-strategy")


def test_selection_defaults():
    s = Selection(tests=(), escalated=True, reason="unseeded")
    assert s.select_ms == 0
    assert s.exec_args == ()


def test_selection_is_frozen_and_always_records_escalation():
    s = Selection(tests=("a::x",), escalated=True, reason="escalated to full suite")
    with pytest.raises(dataclasses.FrozenInstanceError):
        s.escalated = False  # type: ignore[misc]
    # `escalated` is required, so no strategy can produce a record without it.
    with pytest.raises(TypeError):
        Selection(tests=(), reason="no escalation flag")  # type: ignore[call-arg]


def test_register_returns_the_strategy_so_it_can_decorate():
    s = _Dummy()
    assert register(s) is s


def test_all_ids_is_sorted_and_the_single_source_of_truth():
    register(_Seeded())
    register(_Dummy())
    assert all_ids() == ("dummy", "seeded")
    assert set(all_ids()) == set(base.REGISTRY)


def test_needs_parent_state_marks_strategies_seeded_in_the_parent_tree():
    register(_Dummy())
    register(_Seeded())
    assert get("dummy").needs_parent_state is False
    assert get("seeded").needs_parent_state is True


def test_strategy_protocol_is_runtime_checkable():
    assert isinstance(_Dummy(), Strategy)
    assert not isinstance(object(), Strategy)


def test_commit_context_carries_the_changed_set_and_the_full_test_list():
    ctx = _ctx(
        changed=(Change("src/b.py", "M"), Change("src/a.py", "A")),
        all_tests=("a::x", "b::y", "c::z"),
        seed_ms=7,
        peer_sizes={"rtdd": 3},
    )
    assert ctx.changed_paths() == ("src/a.py", "src/b.py")
    assert ctx.all_tests == ("a::x", "b::y", "c::z")
    assert ctx.work == pathlib.Path("/nonexistent")
    assert ctx.seed_ms == 7
    assert ctx.peer_sizes == {"rtdd": 3}
    with pytest.raises(dataclasses.FrozenInstanceError):
        ctx.commit = "other"  # type: ignore[misc]


def test_commit_context_peer_sizes_defaults_are_not_shared():
    a, b = _ctx(), _ctx()
    a.peer_sizes["rtdd"] = 1
    assert b.peer_sizes == {}


def test_validate_selection_passes_a_selection_drawn_from_the_collected_tests():
    ctx = _ctx()
    sel = Selection(tests=("b::y",), escalated=False, reason="ok")
    assert validate_selection(sel, ctx) is sel


def test_validate_selection_rejects_a_test_id_the_repo_never_collected():
    ctx = _ctx()
    sel = Selection(tests=("a::x", "ghost::z"), escalated=False, reason="inflated")
    with pytest.raises(UnknownTestError) as exc:
        validate_selection(sel, ctx)
    assert "ghost::z" in str(exc.value)


def test_unknown_test_error_is_a_value_error():
    assert issubclass(UnknownTestError, ValueError)
