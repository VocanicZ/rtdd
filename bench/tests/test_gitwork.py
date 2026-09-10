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


# --- detecting-commit yield: which commits are worth replaying ---------------
#
# `natural` writes the child's whole diff over the parent, which reproduces the child
# commit — and real projects push green, so it detects nothing, in any repo, ever.
# `probe` is the population that can detect: the child's TESTS against the parent's
# SOURCE, so a test the commit's code change fixed fails.
#
# That only happens when a commit touches both halves. Replaying the most recent N
# commits spends most of them on docs, chores and refactors: flask yielded 3 detecting
# commits out of 23, and httpie and sqlfluff yielded none at all, which is why recall
# has no denominator anywhere in the published corpus.
#
# `paired_points` selects for the property instead of hoping for it. It is corpus
# SELECTION, not fault injection: every commit it returns is a real commit whose real
# tests really failed against its parent's real source. Audit finding A8 rejected a
# mutation harness, and this is not one — nothing is synthesised.

from replay.gitwork import paired_points  # noqa: E402


def _commit(repo, files: dict[str, str], message: str = "c") -> str:
    for rel, body in files.items():
        p = repo / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body)
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", message)
    return git(repo, "rev-parse", "HEAD")


def _history(tmp_path):
    repo = tmp_path / "hist"
    repo.mkdir()
    git(repo, "init", "-q")
    git(repo, "config", "user.email", "t@example.com")
    git(repo, "config", "user.name", "t")
    _commit(repo, {"src/a.py": "def f():\n    return 1\n", "tests/test_a.py": "def test_f():\n    assert True\n"}, "root")
    _commit(repo, {"README.md": "docs\n"}, "docs only")
    _commit(repo, {"src/a.py": "def f():\n    return 2\n"}, "source only")
    _commit(repo, {"tests/test_a.py": "def test_f():\n    assert True  # tidy\n"}, "test only")
    paired = _commit(
        repo,
        {"src/a.py": "def f():\n    return 3\n", "tests/test_a.py": "def test_f():\n    assert True  # fix\n"},
        "fix: both halves",
    )
    return repo, paired


def test_paired_points_keeps_only_commits_touching_source_and_tests(tmp_path):
    repo, paired = _history(tmp_path)
    pts = paired_points(repo, "HEAD", 10, ["tests/**/*.py", "**/test_*.py"])
    assert [p.commit for p in pts] == [paired], "a docs, source-only or test-only commit cannot detect"


def test_paired_points_returns_real_commits_with_real_parents(tmp_path):
    repo, paired = _history(tmp_path)
    pts = paired_points(repo, "HEAD", 10, ["tests/**/*.py", "**/test_*.py"])
    assert pts, "no candidates found"
    for p in pts:
        assert git(repo, "cat-file", "-t", p.commit) == "commit"
        assert git(repo, "cat-file", "-t", p.parent) == "commit"
        assert git(repo, "rev-list", "--parents", "-n", "1", p.commit).split()[1] == p.parent


def test_paired_points_is_capped_and_ordered_oldest_first(tmp_path):
    repo, _ = _history(tmp_path)
    for i in range(3):
        _commit(repo, {"src/a.py": f"def f():\n    return {10 + i}\n", "tests/test_a.py": f"def test_f():\n    assert True  # {i}\n"}, f"fix {i}")
    pts = paired_points(repo, "HEAD", 2, ["tests/**/*.py", "**/test_*.py"])
    assert len(pts) == 2
    order = [git(repo, "rev-list", "--count", f"{p.commit}..HEAD") for p in pts]
    assert order == sorted(order, reverse=True), "points must run oldest first, as replay_points does"


def test_paired_points_skips_merges_like_replay_points(tmp_path):
    repo, _ = _history(tmp_path)
    git(repo, "checkout", "-q", "-b", "side")
    _commit(repo, {"src/b.py": "x = 1\n", "tests/test_b.py": "def test_b():\n    assert True\n"}, "side work")
    git(repo, "checkout", "-q", "-")
    _commit(repo, {"src/c.py": "y = 1\n", "tests/test_c.py": "def test_c():\n    assert True\n"}, "main work")
    git(repo, "merge", "-q", "--no-ff", "side", "-m", "merge")
    pts = paired_points(repo, "HEAD", 20, ["tests/**/*.py", "**/test_*.py"])
    for p in pts:
        parents = git(repo, "rev-list", "--parents", "-n", "1", p.commit).split()
        assert len(parents) == 2, f"{p.commit} is a merge; its parent is ambiguous"
