"""Spec §5's drift axis: selection ratio as a function of cycles-since-commit.

The numbers in :func:`test_drift_accumulates_the_changed_set_across_uncommitted_cycles`
are hand-computed from `tests/synthrepo.py`'s normative history table, replayed
against a worktree pinned at c0 with nothing committed between cycles:

| Cycle | Applied | Working tree vs HEAD (c0)                                  | n |
|-------|---------|------------------------------------------------------------|---|
| 1     | c1      | `src/alpha.py` M                                            | 1 |
| 2     | c2      | `src/gamma.py` A, `tests/test_gamma.py` A                   | 2 |
| 3     | c3      | `src/beta.py` M, `src/gamma.py` A, `tests/test_gamma.py` A  | 3 |

`src/alpha.py` leaves the changed set at cycle 2 because c2 restores it to c0's
exact bytes — the changed set is `git status` against an unmoved HEAD, not a
union of every path a replayed commit ever touched. Task 22 of
`docs/plans/04-m3-replay-benchmark.md` predicted `[1, 3, 4]` on the assumption
that c2 "re-touches alpha"; it reverts it. The measured set is the one spec §5
degrades over, so the table above is normative and the plan's prediction is not.
"""

from __future__ import annotations

import json
import pathlib
import sys

import pytest

from replay.config import canonical_json
from replay.gitwork import ReplayPoint, add_worktree, git, working_changed_paths
from replay.rtddio import WhichResult
from replay.session import DriftCurve, DriftPoint, run_drift


def _which(n_tests: int, tier: str = "T0", cycles: int = 1) -> WhichResult:
    return WhichResult(
        tier=tier,
        reason="",
        tests=tuple(f"t{i}" for i in range(n_tests)),
        direct=(),
        changed=(),
        cycles=cycles,
        wall_ms=1,
    )


def test_ratio_and_serialisation():
    p = DriftPoint(cycle=3, changed_files=5, selected=8, total_tests=20, tier="T0")
    assert p.ratio() == 0.4
    assert p.to_dict() == {
        "cycle": 3,
        "changed_files": 5,
        "selected": 8,
        "total_tests": 20,
        "tier": "T0",
        "selection_ratio": 0.4,
    }


def test_ratio_of_an_empty_suite_is_zero_not_a_zero_division():
    assert DriftPoint(cycle=1, changed_files=0, selected=0, total_tests=0, tier="T0").ratio() == 0.0


def _points(synth) -> list[ReplayPoint]:
    return [
        ReplayPoint(commit=synth.sha(1), parent=synth.sha(0)),
        ReplayPoint(commit=synth.sha(2), parent=synth.sha(1)),
        ReplayPoint(commit=synth.sha(3), parent=synth.sha(2)),
    ]


def test_drift_accumulates_the_changed_set_across_uncommitted_cycles(synth, tmp_path):
    work = tmp_path / "wt"
    points = _points(synth)
    add_worktree(synth.path, points[0].parent, work)

    seen_changed: list[int] = []

    def fake_select(w: pathlib.Path) -> WhichResult:
        n = len(working_changed_paths(w))
        seen_changed.append(n)
        return _which(n, cycles=len(seen_changed))

    curve = run_drift(
        synth.path, "synth", work, points, python=sys.executable, select=fake_select
    )

    assert isinstance(curve, DriftCurve)
    assert curve.repo_id == "synth"
    assert curve.start_commit == points[0].parent
    assert [p.cycle for p in curve.points] == [1, 2, 3]
    assert [p.changed_files for p in curve.points] == [1, 2, 3]
    assert seen_changed == [1, 2, 3]
    assert curve.points[-1].changed_files > curve.points[0].changed_files


def test_drift_denominator_grows_with_the_tests_the_cycles_add(synth, tmp_path):
    """c2 adds `tests/test_gamma.py`, so the suite the ratio divides by is 3 from
    cycle 2 on. A denominator frozen at the parent would silently overstate the
    selection ratio of every later cycle."""
    work = tmp_path / "wt"
    points = _points(synth)
    add_worktree(synth.path, points[0].parent, work)

    curve = run_drift(
        synth.path,
        "synth",
        work,
        points,
        python=sys.executable,
        select=lambda w: _which(1),
    )
    assert [p.total_tests for p in curve.points] == [2, 3, 3]
    assert [round(p.ratio(), 4) for p in curve.points] == [0.5, 0.3333, 0.3333]


def test_drift_never_moves_head_or_commits(synth, tmp_path):
    work = tmp_path / "wt"
    points = _points(synth)
    add_worktree(synth.path, points[0].parent, work)

    run_drift(
        synth.path,
        "synth",
        work,
        points,
        python=sys.executable,
        select=lambda w: _which(1),
    )

    assert git(work, "rev-parse", "HEAD") == points[0].parent
    assert git(work, "rev-list", "--count", "HEAD") == git(
        synth.path, "rev-list", "--count", points[0].parent
    )
    # The accumulated edits are still uncommitted working-tree state.
    assert working_changed_paths(work)


def test_curve_serialises_as_the_report_layer_writes_drift_json(synth, tmp_path):
    work = tmp_path / "wt"
    points = _points(synth)
    add_worktree(synth.path, points[0].parent, work)

    curve = run_drift(
        synth.path,
        "synth",
        work,
        points,
        python=sys.executable,
        select=lambda w: _which(1),
    )
    text = canonical_json(curve.to_dict())
    assert text.endswith("\n")
    doc = json.loads(text)
    assert doc["repo_id"] == "synth"
    assert doc["start_commit"] == points[0].parent
    assert [p["cycle"] for p in doc["points"]] == [1, 2, 3]
    assert doc["points"][0]["selection_ratio"] == 0.5


def test_no_points_is_an_empty_curve_and_never_collects(synth, tmp_path):
    work = tmp_path / "wt"
    add_worktree(synth.path, synth.sha(0), work)

    def explode(w):  # pragma: no cover - must not be reached
        raise AssertionError("select called with no replay points")

    curve = run_drift(
        synth.path, "synth", work, [], python="/nonexistent/python", select=explode
    )
    assert curve.points == ()
    assert curve.start_commit == ""


def test_default_selector_is_the_rtdd_binary_against_base_head(synth, tmp_path, monkeypatch):
    """The drift axis only means anything if the selection really is `--base HEAD`:
    that is the degradation spec §5 names."""
    work = tmp_path / "wt"
    points = _points(synth)[:1]
    add_worktree(synth.path, points[0].parent, work)

    calls: list[dict] = []

    def fake_which(w, binary="rtdd", base="HEAD"):
        calls.append({"work": w, "binary": binary, "base": base})
        return _which(1)

    monkeypatch.setattr("replay.session.rtddio.which", fake_which)
    run_drift(synth.path, "synth", work, points, python=sys.executable, binary="/opt/rtdd")

    assert calls == [{"work": work, "binary": "/opt/rtdd", "base": "HEAD"}]


def test_run_drift_rejects_a_worktree_that_is_not_at_the_first_parent(synth, tmp_path):
    """Seeding happens once, at `points[0].parent`, before the first cycle. A
    worktree checked out anywhere else would measure a different history than the
    curve claims, so it is refused rather than replayed."""
    work = tmp_path / "wt"
    points = _points(synth)
    add_worktree(synth.path, synth.sha(2), work)

    with pytest.raises(ValueError, match="not at"):
        run_drift(
            synth.path,
            "synth",
            work,
            points,
            python=sys.executable,
            select=lambda w: _which(1),
        )
