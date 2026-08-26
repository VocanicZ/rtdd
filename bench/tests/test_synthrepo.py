from __future__ import annotations

import pathlib
import subprocess
import sys

from tests.synthrepo import EXPECTED, build_synth_repo


def _full_suite(repo: pathlib.Path, sha: str) -> tuple[int, frozenset[str]]:
    """Run the whole suite at `sha` and return (returncode, failing node ids)."""
    subprocess.run(["git", "checkout", "-q", sha], cwd=repo, check=True)
    proc = subprocess.run(
        [
            sys.executable,
            "-m",
            "pytest",
            "-q",
            "--no-header",
            "-p",
            "no:cacheprovider",
            "--tb=no",
            "-rf",
        ],
        cwd=repo,
        capture_output=True,
        text=True,
    )
    failed = frozenset(
        line[len("FAILED ") :].split(" - ")[0].strip()
        for line in proc.stdout.splitlines()
        if line.startswith("FAILED ")
    )
    return proc.returncode, failed


def test_history_shape_and_real_outcomes(tmp_path):
    repo = build_synth_repo(tmp_path / "synth")
    assert len(repo.commits) == 4

    assert _full_suite(repo.path, repo.sha(0))[0] == 0, "c0 is green"
    assert _full_suite(repo.path, repo.sha(1))[0] == 1, "c1 breaks test_add"
    assert _full_suite(repo.path, repo.sha(2))[0] == 0, "c2 is green"
    assert _full_suite(repo.path, repo.sha(3))[0] == 1, "c3 breaks test_mul"


def test_full_suite_outcomes_match_the_hand_computed_table(tmp_path):
    repo = build_synth_repo(tmp_path / "synth")
    assert len(EXPECTED) == len(repo.commits)

    for expected in EXPECTED:
        code, failed = _full_suite(repo.path, repo.sha(expected.index))
        assert code == expected.returncode, expected.message
        assert failed == set(expected.failures), expected.message


def test_f_full_against_the_parent_matches_the_hand_computed_table(tmp_path):
    repo = build_synth_repo(tmp_path / "synth")

    observed: dict[int, frozenset[str]] = {
        expected.index: _full_suite(repo.path, repo.sha(expected.index))[1]
        for expected in EXPECTED
    }

    for expected in EXPECTED[1:]:
        f_full = observed[expected.index] - observed[expected.index - 1]
        assert f_full == set(expected.f_full), expected.message


def test_a_failure_already_present_in_the_parent_is_not_counted_as_f_full(tmp_path):
    """c2's parent c1 is red; c2 is green, so its F_full is empty, not a fix."""
    repo = build_synth_repo(tmp_path / "synth")

    _, at_c1 = _full_suite(repo.path, repo.sha(1))
    _, at_c2 = _full_suite(repo.path, repo.sha(2))

    assert at_c1 == {"tests/test_alpha.py::test_add"}
    assert at_c2 == frozenset()
    assert at_c2 - at_c1 == frozenset()
    assert EXPECTED[2].f_full == ()


def test_f_full_subtracts_every_pre_existing_failure_not_just_the_first():
    """Multi-failure parents: F_full is a set difference, so two pre-existing
    failures are both excluded and only the newly broken test survives."""
    parent = frozenset({"tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul"})
    child = parent | {"tests/test_gamma.py::test_sub"}

    assert child - parent == {"tests/test_gamma.py::test_sub"}


def test_rebuilding_the_fixture_is_deterministic(tmp_path):
    first = build_synth_repo(tmp_path / "one")
    second = build_synth_repo(tmp_path / "two")

    assert first.commits == second.commits
    for expected in EXPECTED:
        assert _full_suite(first.path, first.sha(expected.index)) == _full_suite(
            second.path, second.sha(expected.index)
        )


def test_synth_fixture_and_cache_root_fixture_are_usable(synth, cache_root):
    assert len(synth.commits) == 4
    assert (synth.path / ".git").is_dir()
    assert not cache_root.exists() or cache_root.is_dir()
