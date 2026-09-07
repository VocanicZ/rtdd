from __future__ import annotations

import dataclasses
import json

import pytest

from replay.records import (
    STRATA,
    CommitRecord,
    StrategyRecord,
    UncoveredRecord,
    WallClockRecord,
    from_dict,
    parse_jsonl_lines,
    stratum_of,
    to_jsonl_lines,
)


def _commit_record() -> CommitRecord:
    return CommitRecord(
        repo_id="synth",
        commit="c1",
        parent="c0",
        variant="natural",
        all_tests=("a::x", "b::y"),
        durations_ms={"b::y": 30, "a::x": 10},
        f_full=("a::x",),
        pre_existing_failures=("b::y",),
        changed=("src/alpha.py",),
    )


def _strategy_record() -> StrategyRecord:
    return StrategyRecord(
        repo_id="synth",
        commit="c1",
        variant="natural",
        strategy="rtdd",
        selected=("a::x",),
        escalated=False,
        reason="T0",
        select_ms=3,
    )


def _wallclock_record() -> WallClockRecord:
    return WallClockRecord(
        repo_id="synth",
        commit="c1",
        variant="natural",
        strategy="rtdd",
        hardware_fingerprint="hw-abc",
        full_uninstrumented_ms=900,
        subset_instrumented_ms=120,
        subset_uninstrumented_ms=None,
        isolation_violation=False,
    )


def _uncovered_record() -> UncoveredRecord:
    return UncoveredRecord(
        repo_id="synth",
        commit="c1",
        variant="natural",
        reported=(("src/alpha.py", 4), ("src/alpha.py", 5)),
        truth_covered_hits=1,
        reported_count=2,
    )


ALL_RECORDS = (
    _commit_record,
    _strategy_record,
    _wallclock_record,
    _uncovered_record,
)


def test_stratum_boundaries():
    assert stratum_of(1) == "1"
    assert stratum_of(2) == "2-5"
    assert stratum_of(5) == "2-5"
    assert stratum_of(6) == "6-20"
    assert stratum_of(20) == "6-20"
    assert stratum_of(21) == "21+"
    assert stratum_of(0) == "0"


def test_strata_are_the_published_order():
    assert STRATA == ("1", "2-5", "6-20", "21+")


def test_commit_record_stratum_and_duration():
    rec = CommitRecord(
        repo_id="synth",
        commit="c1",
        parent="c0",
        variant="natural",
        all_tests=("a::x", "b::y"),
        durations_ms={"a::x": 10, "b::y": 30},
        f_full=("a::x",),
        pre_existing_failures=(),
        changed=("src/alpha.py",),
    )
    assert rec.stratum() == "1"
    assert rec.total_duration_ms() == 40


def test_commit_record_total_duration_ignores_tests_it_never_timed():
    rec = dataclasses.replace(_commit_record(), durations_ms={"a::x": 10})
    assert rec.total_duration_ms() == 10


@pytest.mark.parametrize("factory", ALL_RECORDS, ids=lambda f: f.__name__)
def test_every_record_is_frozen(factory):
    rec = factory()
    with pytest.raises(dataclasses.FrozenInstanceError):
        rec.repo_id = "other"


@pytest.mark.parametrize("factory", ALL_RECORDS, ids=lambda f: f.__name__)
def test_every_record_carries_the_identity_keys(factory):
    d = factory().to_dict()
    assert d["repo_id"] == "synth"
    assert d["commit"] == "c1"
    assert d["variant"] == "natural"
    assert d["kind"] in {"commit", "strategy", "wallclock", "uncovered"}


@pytest.mark.parametrize(
    "factory", (_strategy_record, _wallclock_record), ids=lambda f: f.__name__
)
def test_strategy_scoped_records_carry_the_strategy(factory):
    assert factory().to_dict()["strategy"] == "rtdd"


def test_commit_record_dict_keeps_the_pre_existing_failures_apart_from_f_full():
    d = _commit_record().to_dict()
    assert d["f_full"] == ["a::x"]
    assert d["pre_existing_failures"] == ["b::y"]
    assert d["n_tests"] == 2
    assert d["stratum"] == "1"
    assert d["durations_ms"] == {"a::x": 10, "b::y": 30}


def test_strategy_record_dict_carries_selection_escalation_and_reason():
    d = _strategy_record().to_dict()
    assert d["selected"] == ["a::x"]
    assert d["n_selected"] == 1
    assert d["escalated"] is False
    assert d["reason"] == "T0"
    assert d["select_ms"] == 3


def test_wallclock_record_dict_keeps_missing_timings_null():
    d = _wallclock_record().to_dict()
    assert d["full_uninstrumented_ms"] == 900
    assert d["subset_instrumented_ms"] == 120
    assert d["subset_uninstrumented_ms"] is None
    assert d["isolation_violation"] is False


def test_uncovered_record_dict_uses_pairs():
    d = _uncovered_record().to_dict()
    assert d["reported"] == [["src/alpha.py", 4], ["src/alpha.py", 5]]
    assert d["truth_covered_hits"] == 1
    assert d["reported_count"] == 2


def test_jsonl_is_sorted_and_newline_terminated():
    recs = [
        StrategyRecord("synth", "c2", "natural", "rtdd", ("a::x",), False, "T0", 3),
        StrategyRecord("synth", "c1", "natural", "path", ("b::y",), False, "sib", 1),
    ]
    lines = to_jsonl_lines(recs)
    assert len(lines) == 2
    assert '"commit":"c1"' in lines[0]
    assert all(line.endswith("\n") for line in lines)


def test_jsonl_orders_strategy_records_of_one_commit_by_strategy():
    recs = [
        StrategyRecord("synth", "c1", "natural", "rtdd", (), False, "", 0),
        StrategyRecord("synth", "c1", "natural", "all", (), True, "", 0),
    ]
    strategies = [json.loads(line)["strategy"] for line in to_jsonl_lines(recs)]
    assert strategies == ["all", "rtdd"]


def test_jsonl_keys_are_sorted_within_each_line():
    line = to_jsonl_lines([_strategy_record()])[0]
    keys = list(json.loads(line))
    assert keys == sorted(keys)


def test_jsonl_is_byte_identical_across_runs_and_input_order():
    recs = [factory() for factory in ALL_RECORDS]
    first = to_jsonl_lines(recs)
    assert to_jsonl_lines(recs) == first
    assert to_jsonl_lines(list(reversed(recs))) == first


def test_jsonl_line_ordering_is_stable_for_a_dict_field_shuffle():
    a = _commit_record()
    b = dataclasses.replace(a, durations_ms={"a::x": 10, "b::y": 30})
    assert to_jsonl_lines([a]) == to_jsonl_lines([b])


@pytest.mark.parametrize("factory", ALL_RECORDS, ids=lambda f: f.__name__)
def test_from_dict_round_trips_every_record_type(factory):
    rec = factory()
    assert from_dict(rec.to_dict()) == rec


@pytest.mark.parametrize("factory", ALL_RECORDS, ids=lambda f: f.__name__)
def test_json_round_trip_is_lossless(factory):
    rec = factory()
    line = to_jsonl_lines([rec])[0]
    assert parse_jsonl_lines([line]) == [rec]


def test_parse_jsonl_lines_round_trips_a_mixed_file_in_written_order():
    recs = [factory() for factory in ALL_RECORDS]
    lines = to_jsonl_lines(recs)
    parsed = parse_jsonl_lines(lines)
    assert to_jsonl_lines(parsed) == lines
    assert sorted(r.to_dict()["kind"] for r in parsed) == [
        "commit",
        "strategy",
        "uncovered",
        "wallclock",
    ]


def test_parse_jsonl_lines_skips_blank_lines():
    lines = to_jsonl_lines([_strategy_record()])
    assert parse_jsonl_lines([*lines, "\n", "  \n"]) == [_strategy_record()]


def test_from_dict_refuses_an_unknown_kind():
    with pytest.raises(ValueError, match="unknown record kind"):
        from_dict({"kind": "nope", "repo_id": "synth"})


def test_a_strategy_record_publishes_the_stale_ids_it_dropped():
    """A rename is visible in the record, not silently absorbed.

    `--lf`, testmon and rtdd answer out of state built at the base tree, so a
    commit that removed a test can be handed an id that no longer collects. The
    id is dropped from `selected` — it is not work — and named here, so a reader
    can see that it happened rather than inferring it from a shrunken count.
    """
    r = StrategyRecord(
        repo_id="synth",
        commit="c1",
        variant="natural",
        strategy="lf",
        selected=("t/f.py::a",),
        escalated=False,
        reason="last failed",
        select_ms=3,
        stale_dropped=("t/f.py::gone",),
    )
    d = r.to_dict()
    assert d["stale_dropped"] == ["t/f.py::gone"]
    assert d["n_stale_dropped"] == 1
    assert StrategyRecord.from_dict(d) == r
    # An older line without the key still reads back.
    older = {k: v for k, v in d.items() if not k.startswith(("stale_", "n_stale"))}
    assert StrategyRecord.from_dict(older).stale_dropped == ()


def test_strategy_record_defaults_to_measured():
    """Every record written before M6e came from a strategy that really ran."""
    rec = StrategyRecord("flask", "c1", "natural", "path", ("a",), False, "sib", 3)
    assert rec.derived is False
    assert rec.to_dict()["derived"] is False


def test_a_derived_record_is_marked_on_the_wire():
    """`select_ms` on a derived record is 0 because nothing was timed, not because it was
    fast. `derived` is the field that says which of those two a 0 means, and it has to
    survive the round-trip or a reader cannot tell them apart at all."""
    rec = StrategyRecord(
        "flask", "c1", "natural", "static", ("a", "b"), False, "derived offline", 0,
        derived=True,
    )
    d = rec.to_dict()
    assert d["derived"] is True
    assert StrategyRecord.from_dict(d) == rec


def test_an_already_committed_line_reads_back_as_measured():
    """bench/results/*/commits.jsonl predates this field. A missing key is `false`, not a
    KeyError: the published records are the input to every derivation and re-writing them
    to add a default would be a schema migration of committed ground truth."""
    old = {
        "kind": "strategy", "repo_id": "flask", "commit": "c1", "variant": "natural",
        "strategy": "path", "selected": ["a"], "n_selected": 1, "escalated": False,
        "reason": "sib", "select_ms": 3, "stale_dropped": [], "n_stale_dropped": 0,
    }
    assert StrategyRecord.from_dict(old).derived is False
