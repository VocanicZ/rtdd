from __future__ import annotations

import dataclasses
import pathlib

import pytest

from replay.gitwork import (
    Change,
    GitError,
    ReplayPoint,
    add_worktree,
    clone_pinned,
    diff_changes,
    git,
    is_test_path,
    materialise_natural,
    materialise_probe,
    probe_candidates,
    remove_worktree,
    replay_points,
    working_changed_paths,
)

TEST_GLOBS = ("tests/**/*.py", "**/test_*.py", "**/conftest.py")


def test_replay_points_are_oldest_first_with_parents(synth):
    pts = replay_points(synth.path, synth.sha(3), 3)
    assert [p.commit for p in pts] == [synth.sha(1), synth.sha(2), synth.sha(3)]
    assert [p.parent for p in pts] == [synth.sha(0), synth.sha(1), synth.sha(2)]


def test_replay_points_skip_the_root_commit(synth):
    pts = replay_points(synth.path, synth.sha(3), 99)
    assert [p.commit for p in pts] == [synth.sha(1), synth.sha(2), synth.sha(3)]


def test_diff_changes_reports_adds_and_modifies(synth):
    ch = diff_changes(synth.path, synth.sha(1), synth.sha(2))
    assert sorted((c.path, c.status) for c in ch) == [
        ("src/alpha.py", "M"),
        ("src/gamma.py", "A"),
        ("tests/test_gamma.py", "A"),
    ]


def test_diff_changes_reports_deletions(synth):
    ch = diff_changes(synth.path, synth.sha(2), synth.sha(1))
    assert sorted((c.path, c.status) for c in ch) == [
        ("src/alpha.py", "M"),
        ("src/gamma.py", "D"),
        ("tests/test_gamma.py", "D"),
    ]


def test_is_test_path(synth):
    assert is_test_path("tests/test_gamma.py", TEST_GLOBS)
    assert is_test_path("conftest.py", TEST_GLOBS)
    assert not is_test_path("src/gamma.py", TEST_GLOBS)


def test_probe_candidates_touch_both_source_and_test_files(synth):
    pts = replay_points(synth.path, synth.sha(3), 3)
    cands = probe_candidates(synth.path, pts, TEST_GLOBS)
    # c1 is source-only, c3 is source-only; only c2 adds gamma source *and* its test.
    assert [p.commit for p in cands] == [synth.sha(2)]


def test_clone_pinned_checks_out_the_pin_and_refuses_a_moved_clone(synth, tmp_path):
    dest = tmp_path / "clone"
    clone_pinned(str(synth.path), synth.sha(1), dest)
    assert git(dest, "rev-parse", "HEAD") == synth.sha(1)

    # An existing clone at the pin is reused, not re-cloned.
    assert clone_pinned(str(synth.path), synth.sha(1), dest) == dest

    with pytest.raises(GitError):
        clone_pinned(str(synth.path), synth.sha(2), dest)


def test_worktrees_are_detached_and_removed_without_disturbing_the_clone(synth, tmp_path):
    head_before = git(synth.path, "rev-parse", "HEAD")
    work = tmp_path / "wt"
    add_worktree(synth.path, synth.sha(1), work)

    assert git(work, "rev-parse", "HEAD") == synth.sha(1)
    assert git(work, "symbolic-ref", "-q", "HEAD", check=False) == ""  # detached
    assert git(synth.path, "rev-parse", "HEAD") == head_before

    remove_worktree(synth.path, work)
    assert not work.exists()
    assert str(work) not in git(synth.path, "worktree", "list")
    assert git(synth.path, "rev-parse", "HEAD") == head_before
    assert git(synth.path, "status", "--porcelain") == ""


def test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked(synth, tmp_path):
    work = tmp_path / "wt"
    pts = replay_points(synth.path, synth.sha(3), 3)
    c2 = [p for p in pts if p.commit == synth.sha(2)][0]
    add_worktree(synth.path, c2.parent, work)
    changed = materialise_natural(work, synth.path, c2)

    assert git(work, "rev-parse", "HEAD") == c2.parent
    assert (work / "src/gamma.py").read_text() == "def sub(a, b):\n    return a - b\n"
    assert (work / "src/alpha.py").read_text() == "def add(a, b):\n    return a + b\n"
    assert sorted(c.path for c in changed) == [
        "src/alpha.py",
        "src/gamma.py",
        "tests/test_gamma.py",
    ]

    seen = {c.path: c.status for c in working_changed_paths(work)}
    assert seen["src/gamma.py"] == "A"
    assert seen["tests/test_gamma.py"] == "A"
    assert seen["src/alpha.py"] == "M"


def test_materialise_natural_applies_deletions(synth, tmp_path):
    work = tmp_path / "wt"
    # A synthetic point whose "child" is the older tree: gamma disappears.
    point = ReplayPoint(commit=synth.sha(1), parent=synth.sha(2))
    add_worktree(synth.path, point.parent, work)
    materialise_natural(work, synth.path, point)

    assert git(work, "rev-parse", "HEAD") == point.parent
    assert not (work / "src/gamma.py").exists()
    assert not (work / "tests/test_gamma.py").exists()
    seen = {c.path: c.status for c in working_changed_paths(work)}
    assert seen["src/gamma.py"] == "D"
    assert seen["tests/test_gamma.py"] == "D"


def test_materialise_probe_keeps_child_tests_over_parent_source(synth, tmp_path):
    work = tmp_path / "wt"
    pts = replay_points(synth.path, synth.sha(3), 3)
    c2 = [p for p in pts if p.commit == synth.sha(2)][0]
    add_worktree(synth.path, c2.commit, work)
    changed = materialise_probe(work, synth.path, c2, TEST_GLOBS)

    assert git(work, "rev-parse", "HEAD") == c2.commit
    # c2's tests survive
    assert (work / "tests/test_gamma.py").exists()
    # c2's source is reverted to c1: gamma.py did not exist, alpha.py is broken
    assert not (work / "src/gamma.py").exists()
    assert (work / "src/alpha.py").read_text() == "def add(a, b):\n    return a + b + 1\n"
    assert sorted(c.path for c in changed) == ["src/alpha.py", "src/gamma.py"]
    assert {c.path: c.status for c in changed} == {"src/alpha.py": "M", "src/gamma.py": "D"}


def test_change_is_a_frozen_value(synth):
    ch = Change(path="src/alpha.py", status="M")
    with pytest.raises(dataclasses.FrozenInstanceError):
        ch.path = "other"  # type: ignore[misc]


def test_working_changed_paths_is_repo_relative_and_slash_separated(synth, tmp_path):
    work = tmp_path / "wt"
    add_worktree(synth.path, synth.sha(0), work)
    nested = work / "src" / "pkg"
    nested.mkdir(parents=True)
    (nested / "deep.py").write_text("x = 1\n", encoding="utf-8")

    paths = [c.path for c in working_changed_paths(work)]
    assert "src/pkg/deep.py" in paths
    assert all(not pathlib.PurePosixPath(p).is_absolute() for p in paths)
