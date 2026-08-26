"""The system under test as a strategy: `rtdd seed` then `rtdd which`.

Most of these are parser-level and monkeypatch `rtddio`, so they run without the
binary — the binary-level contract lives in `test_rtddio.py`. The last test is
the synthetic-repo verification: it drives the real binary end to end and is
skipped when `rtdd` is not on PATH.

The load-bearing property is that the strategy passes **no flags of its own**.
A tuned RTDD scored against untuned baselines is not a result, so
`test_select_passes_only_shipped_defaults` pins the argv the strategy is allowed
to influence to `base="HEAD"` and nothing else.
"""

from __future__ import annotations

import pathlib
import shutil
import subprocess

import pytest

from replay.gitwork import Change
from replay.rtddio import WhichResult
from replay.strategies.base import CommitContext, get
from replay.strategies.rtdd import Rtdd

ALL = ("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul")


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


def _which(**over):
    fields = dict(
        tier="T0", reason="", tests=(), direct=(), changed=(), cycles=0, wall_ms=0
    )
    fields.update(over)
    return WhichResult(**fields)


def test_select_maps_which_output_to_a_selection(monkeypatch):
    s = Rtdd()
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(
            tier="T0",
            tests=("tests/test_alpha.py::test_add",),
            changed=("src/alpha.py",),
            cycles=2,
            wall_ms=11,
        ),
    )
    sel = s.select(_ctx())
    assert sel.tests == ("tests/test_alpha.py::test_add",)
    assert sel.escalated is False
    assert sel.reason == "T0"
    assert sel.select_ms == 11


def test_t2_is_reported_as_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(
            tier="T2",
            reason="pyproject.toml changed",
            tests=ALL,
            changed=("pyproject.toml",),
            cycles=2,
            wall_ms=4,
        ),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == ALL
    assert sel.escalated is True
    assert sel.reason == "T2: pyproject.toml changed"
    assert sel.select_ms == 4


def test_empty_tier_is_reported_and_is_not_an_escalation(monkeypatch):
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.which",
        lambda work, binary="rtdd", base="HEAD": _which(tier="empty", cycles=1, wall_ms=2),
    )
    sel = Rtdd().select(_ctx())
    assert sel.tests == ()
    assert sel.escalated is False
    assert sel.reason == "empty"


def test_prepare_seeds_the_parent_tree_with_the_configured_binary(monkeypatch):
    calls: list[tuple] = []
    monkeypatch.setattr(
        "replay.strategies.rtdd.rtddio.seed",
        lambda work, binary="rtdd": calls.append((work, binary)),
    )
    ctx = _ctx("/parent")
    Rtdd(binary="/opt/rtdd").prepare(ctx)
    assert calls == [(ctx.work, "/opt/rtdd")]


def test_select_passes_only_shipped_defaults(monkeypatch):
    """No tuning flags, ever: the strategy may name only the base revision."""
    seen: list[dict] = []

    def fake_which(work, binary="rtdd", base="HEAD"):
        seen.append({"work": work, "binary": binary, "base": base})
        return _which()

    monkeypatch.setattr("replay.strategies.rtdd.rtddio.which", fake_which)
    Rtdd(binary="/opt/rtdd").select(_ctx("/tree"))
    assert seen == [{"work": pathlib.Path("/tree"), "binary": "/opt/rtdd", "base": "HEAD"}]


def test_the_rtdd_strategy_is_registered_under_its_plan_id():
    s = get("rtdd")
    assert s.id == "rtdd"
    assert s.needs_parent_state is True


@pytest.mark.skipif(shutil.which("rtdd") is None, reason="rtdd binary not on PATH")
def test_verified_against_the_synthetic_repo(synth, monkeypatch):
    """End to end on the synth fixture: seed at c2, edit alpha, select test_add."""
    # PYTHONPATH is environment setup the orchestrator owns, not a tuning flag.
    monkeypatch.setenv("PYTHONPATH", str(synth.path))
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    subprocess.run(["rtdd", "init"], cwd=synth.path, check=True, capture_output=True)

    s = Rtdd()
    ctx = _ctx(synth.path)
    s.prepare(ctx)
    assert (synth.path / ".rtdd" / "map.jsonl").exists()

    (synth.path / "src" / "alpha.py").write_text(
        "def add(a, b):\n    return a + b + 1\n", encoding="utf-8"
    )
    sel = s.select(_ctx(synth.path, [("src/alpha.py", "M")]))
    assert "tests/test_alpha.py::test_add" in sel.tests
    assert sel.escalated is False
    assert sel.reason
    assert sel.select_ms >= 0
