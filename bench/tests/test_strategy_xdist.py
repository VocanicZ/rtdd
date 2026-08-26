from __future__ import annotations

import pathlib

from replay.strategies.base import CommitContext
from replay.strategies.xdist import Xdist


def _ctx(all_tests):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=pathlib.Path("/nonexistent"),
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def test_xdist_selects_everything_but_carries_parallel_exec_args():
    sel = Xdist().select(_ctx(("a::x", "b::y")))
    assert sel.tests == ("a::x", "b::y")
    assert sel.escalated is True
    assert sel.exec_args == ("-n", "auto")
    assert sel.reason == "no selection; parallel execution"


def test_the_xdist_baseline_is_registered_and_needs_no_parent_state():
    from replay.strategies.base import get

    s = get("xdist")
    assert s.id == "xdist"
    assert s.needs_parent_state is False


def test_xdist_selects_the_whole_collected_suite_of_the_synthetic_repo(synth):
    import subprocess

    from replay.runner import collect

    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    all_tests = collect(synth.path)
    sel = Xdist().select(_ctx(all_tests))
    assert sel.tests == all_tests
