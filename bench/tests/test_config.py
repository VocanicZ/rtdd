from replay.config import canonical_json


def test_canonical_json_is_key_order_independent_and_newline_terminated():
    a = canonical_json({"b": 1, "a": [3, 2]})
    b = canonical_json({"a": [3, 2], "b": 1})
    assert a == b
    assert a == '{"a":[3,2],"b":1}\n'


from replay.config import RunConfig


def _cfg(**over):
    base = dict(
        corpus_digest="abc",
        rtdd_version="0.3.0",
        tool_versions=(("pytest", "8.3.3"),),
        strategies=("full", "rtdd"),
        variants=("natural",),
        replay_commits=200,
        wallclock_sample=20,
        random_seed=7,
    )
    base.update(over)
    return RunConfig(**base)


def test_digest_changes_when_any_published_knob_changes():
    assert _cfg().digest() == _cfg().digest()
    assert _cfg().digest() != _cfg(replay_commits=201).digest()
    assert _cfg().digest() != _cfg(rtdd_version="0.3.1").digest()
    assert _cfg().digest() != _cfg(strategies=("full",)).digest()


import dataclasses

import pytest

from replay.config import TRACKED_TOOLS, tool_versions


def test_run_config_is_frozen():
    cfg = _cfg()
    with pytest.raises(dataclasses.FrozenInstanceError):
        cfg.rtdd_version = "0.4.0"


def test_to_dict_carries_every_published_knob():
    d = _cfg().to_dict()
    assert d["corpus_digest"] == "abc"
    assert d["rtdd_version"] == "0.3.0"
    assert d["tool_versions"] == {"pytest": "8.3.3"}
    assert d["strategies"] == ["full", "rtdd"]
    assert d["variants"] == ["natural"]
    assert d["random_seed"] == 7
    assert d["harness_version"]


def test_tool_versions_are_sorted_and_cover_every_tracked_tool():
    tv = tool_versions()
    assert tv == tuple(sorted(tv))
    assert {name for name, _ in tv} == set(TRACKED_TOOLS)
    assert all(version for _, version in tv)
