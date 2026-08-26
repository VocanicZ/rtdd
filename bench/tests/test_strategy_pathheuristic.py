"""Baseline 2 — the naive path heuristic, and the misses that keep it naive.

This is the load-bearing baseline: `tests/test_<module>.py` for a changed
`src/<module>.py` is what a developer writes in ten minutes with no map at all.
PRD #4's pre-registered decision criterion is measured against it, so the rule is
pinned here exactly as the plan states it — *including* the cases where it selects
nothing. A heuristic quietly improved with package matching or a directory
fallback would flatter RTDD by making its opponent stronger than the one the
pre-registration named.
"""

from __future__ import annotations

import pathlib

from replay.gitwork import Change, diff_changes
from replay.strategies.base import REGISTRY, CommitContext, get, validate_selection
from replay.strategies.pathheuristic import (
    PathHeuristic,
    candidate_test_files,
)
from replay.strategies.pathheuristic import tests_in_files as _tests_in_files

ALL = (
    "tests/test_alpha.py::test_add",
    "tests/test_beta.py::test_mul",
    "tests/test_gamma.py::test_sub",
)


def _ctx(changed, all_tests=ALL):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=tuple(Change(p, s) for p, s in changed),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )


def test_candidate_matching_is_basename_only():
    assert candidate_test_files("alpha", ALL) == {"tests/test_alpha.py"}
    assert candidate_test_files("nothing", ALL) == set()


def test_candidate_matching_accepts_the_suffix_form_too():
    all_tests = ("suite/alpha_test.py::test_add",)
    assert candidate_test_files("alpha", all_tests) == {"suite/alpha_test.py"}


def test_candidate_matching_ignores_the_directory_a_test_file_lives_in():
    all_tests = ("deep/nested/tests/test_alpha.py::test_add",)
    assert candidate_test_files("alpha", all_tests) == {"deep/nested/tests/test_alpha.py"}


def test_tests_in_files_returns_every_id_in_the_matched_files_sorted():
    all_tests = ("tests/test_alpha.py::test_b", "tests/test_alpha.py::test_a", "tests/test_beta.py::t")
    assert _tests_in_files({"tests/test_alpha.py"}, all_tests) == (
        "tests/test_alpha.py::test_a",
        "tests/test_alpha.py::test_b",
    )


def test_source_change_maps_to_its_sibling_test_file():
    sel = PathHeuristic().select(_ctx([("src/alpha.py", "M")]))
    assert sel.tests == ("tests/test_alpha.py::test_add",)
    assert sel.escalated is False
    assert sel.reason == "sibling test files"


def test_multi_file_change_unions():
    sel = PathHeuristic().select(_ctx([("src/beta.py", "M"), ("src/gamma.py", "M")]))
    assert sel.tests == ("tests/test_beta.py::test_mul", "tests/test_gamma.py::test_sub")


def test_changed_test_file_selects_itself():
    sel = PathHeuristic().select(_ctx([("tests/test_gamma.py", "A")]))
    assert sel.tests == ("tests/test_gamma.py::test_sub",)


def test_changed_test_file_the_repo_never_collected_contributes_nothing():
    sel = PathHeuristic().select(_ctx([("tests/test_deleted.py", "D")]))
    assert sel.tests == ()


def test_unmatched_change_selects_nothing_and_is_not_an_escalation():
    sel = PathHeuristic().select(_ctx([("src/orphan.py", "M")]))
    assert sel.tests == ()
    assert sel.escalated is False
    assert sel.reason == "no sibling test file"


def test_the_heuristic_does_no_package_matching():
    """`src/pkg/thing.py` does not reach `tests/test_pkg.py`. That miss is the point."""
    all_tests = ("tests/test_pkg.py::test_it",)
    sel = PathHeuristic().select(_ctx([("src/pkg/thing.py", "M")], all_tests=all_tests))
    assert sel.tests == ()


def test_a_changed_package_init_reaches_nothing():
    """The stem of `__init__.py` is `__init__`; no `test___init__.py` exists. Also the point."""
    all_tests = ("tests/test_pkg.py::test_it",)
    sel = PathHeuristic().select(_ctx([("src/pkg/__init__.py", "M")], all_tests=all_tests))
    assert sel.tests == ()


def test_non_python_changes_are_ignored():
    sel = PathHeuristic().select(_ctx([("data/alpha.yaml", "M")]))
    assert sel.tests == ()


def test_selection_only_ever_names_collected_test_ids():
    ctx = _ctx([("src/alpha.py", "M"), ("tests/test_beta.py", "M")])
    sel = PathHeuristic().select(ctx)
    assert validate_selection(sel, ctx) is sel


def test_the_heuristic_passes_no_tuning_flags():
    assert PathHeuristic().select(_ctx([("src/alpha.py", "M")])).exec_args == ()


def test_path_is_registered_under_the_id_the_plan_fixes():
    assert PathHeuristic.id == "path"
    assert PathHeuristic.needs_parent_state is False
    assert isinstance(get("path"), PathHeuristic)
    assert REGISTRY["path"] is get("path")


# --- verified against the synthetic repo, hand-computed per commit -------------
#
# | Commit | Changed set                                        | Path heuristic selects        |
# |--------|----------------------------------------------------|-------------------------------|
# | c1     | src/alpha.py                                       | test_alpha                    |
# | c2     | src/alpha.py, src/gamma.py, tests/test_gamma.py    | test_alpha, test_gamma        |
# | c3     | src/beta.py, src/gamma.py                          | test_beta, test_gamma         |
ALPHA = "tests/test_alpha.py::test_add"
BETA = "tests/test_beta.py::test_mul"
GAMMA = "tests/test_gamma.py::test_sub"

SYNTH_EXPECTED = {
    1: ((ALPHA, BETA), (ALPHA,)),
    2: ((ALPHA, BETA, GAMMA), (ALPHA, GAMMA)),
    3: ((ALPHA, BETA, GAMMA), (BETA, GAMMA)),
}


def test_path_selection_over_the_synth_repo_matches_the_hand_computed_table(synth):
    for index, (collected, expected) in SYNTH_EXPECTED.items():
        ctx = CommitContext(
            repo_id="synth",
            variant="natural",
            commit=synth.sha(index),
            parent=synth.sha(index - 1),
            work=synth.path,
            changed=tuple(diff_changes(synth.path, synth.sha(index - 1), synth.sha(index))),
            all_tests=collected,
            source_globs=("src/**/*.py",),
            test_globs=("tests/**/*.py", "**/test_*.py"),
            python="python",
        )
        sel = PathHeuristic().select(ctx)
        assert sel.tests == expected, f"c{index}"
        assert sel.escalated is False, f"c{index}"
        validate_selection(sel, ctx)
