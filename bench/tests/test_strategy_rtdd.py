"""The system under test as a strategy: `rtdd graph` once, then `rtdd which`.

Parser-level tests monkeypatch `rtddio`, so they run without the binary; the last
test drives the real v0.3.0 binary on the synthetic repo. The strategy passes no
flags of its own: a tuned rtdd against untuned baselines is not a result.
"""
from __future__ import annotations

import pathlib

from replay.gitwork import Change
from replay.rtddio import WhichResult
from replay.strategies.base import CommitContext, get
from replay.strategies.rtdd import Rtdd
from tests.rtddbin import requires_v03_rtdd

ADD = "tests/test_alpha.py::test_add"
MUL = "tests/test_beta.py::test_mul"
ALL = (ADD, MUL)


def _ctx(work: pathlib.Path | str = "/nonexistent", changed: list | None = None) -> CommitContext:
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path(work),
        changed=tuple(Change(p, s) for p, s in (changed or [])),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )


def _which(**over) -> WhichResult:
    fields = dict(round1=(), round2=(), changed=(), untested=(), source="scanner", wall_ms=0)
    fields.update(over)
    return WhichResult(**fields)


def test_prepare_builds_the_graph_with_the_configured_binary(monkeypatch):
    calls = []
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.graph",
        lambda work, binary="rtdd": calls.append((work, binary)),
    )
    Rtdd(binary="/opt/rtdd").prepare(_ctx("/tree"))
    assert calls == [(pathlib.Path("/tree"), "/opt/rtdd")]


def test_select_takes_round_1_expanded_to_collected_ids(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(
            round1=(ADD, "tests/test_alpha.py::helper"), round2=(MUL,), wall_ms=7
        ),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == (ADD,)
    assert sel.escalated is False
    assert sel.select_ms == 7
    assert "round 1" in sel.reason
    assert "1 not collected" in sel.reason


def test_select_passes_only_shipped_defaults(monkeypatch):
    seen = []

    def fake_which(work, binary="rtdd", base="HEAD"):
        seen.append({"work": work, "binary": binary, "base": base})
        return _which()

    monkeypatch.setattr("replay.strategies.rtdd.rtddio.which", fake_which)
    Rtdd(binary="/opt/rtdd").select(_ctx("/tree"))
    assert seen == [{"work": pathlib.Path("/tree"), "binary": "/opt/rtdd", "base": "HEAD"}]


def test_an_empty_round_1_is_an_empty_selection_not_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == ()
    assert sel.escalated is False


def test_the_rtdd_strategy_is_registered_under_its_plan_id():
    s = get("rtdd")
    assert s.id == "rtdd"
    assert s.needs_parent_state is True


@requires_v03_rtdd
def test_verified_against_the_synthetic_repo(synth):
    """End to end on the synth fixture: graph at c2, edit alpha, select test_add."""
    import subprocess

    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    s = Rtdd()
    s.prepare(_ctx(synth.path))
    assert (synth.path / ".rtdd" / "graph.json").is_file()

    (synth.path / "src" / "alpha.py").write_text(
        "def add(a, b):\n    return a + b + 1\n", encoding="utf-8"
    )
    sel = s.select(_ctx(synth.path, [("src/alpha.py", "M")]))
    assert ADD in sel.tests
    assert sel.escalated is False
    assert sel.select_ms >= 0
