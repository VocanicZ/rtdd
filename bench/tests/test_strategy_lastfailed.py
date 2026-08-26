from __future__ import annotations

import dataclasses
import json
import pathlib
import subprocess
import sys

from replay.strategies.base import CommitContext
from replay.strategies.lastfailed import LastFailed, read_lastfailed


def _ctx(work, all_tests):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=work,
        changed=(),
        all_tests=all_tests,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py",),
        python="python",
    )


def _write_lastfailed(work: pathlib.Path, entries: dict):
    d = work / ".pytest_cache" / "v" / "cache"
    d.mkdir(parents=True, exist_ok=True)
    (d / "lastfailed").write_text(json.dumps(entries), encoding="utf-8")


def test_read_lastfailed_returns_node_ids(synth):
    _write_lastfailed(
        synth.path, {"tests/test_beta.py::test_mul": True, "tests/test_beta.py": True}
    )
    assert read_lastfailed(synth.path) == ("tests/test_beta.py::test_mul",)


def test_selects_only_previously_failing_tests(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(3)], cwd=synth.path, check=True)
    _write_lastfailed(synth.path, {"tests/test_beta.py::test_mul": True})
    sel = LastFailed().select(
        _ctx(synth.path, ("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul"))
    )
    assert sel.tests == ("tests/test_beta.py::test_mul",)
    assert sel.escalated is False


def test_empty_cache_degenerates_to_the_full_suite_and_is_an_escalation(synth):
    all_tests = ("tests/test_alpha.py::test_add", "tests/test_beta.py::test_mul")
    sel = LastFailed().select(_ctx(synth.path, all_tests))
    assert sel.tests == all_tests
    assert sel.escalated is True
    assert "no recorded failures" in sel.reason


def test_the_lf_baseline_is_registered_under_its_plan_id():
    from replay.strategies.base import get

    s = get("lf")
    assert s.id == "lf"
    assert s.needs_parent_state is True


def test_prepare_seeds_the_cache_in_the_parent_tree(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(3)], cwd=synth.path, check=True)
    ctx = dataclasses.replace(
        _ctx(synth.path, ("tests/test_beta.py::test_mul",)), python=sys.executable
    )
    LastFailed().prepare(ctx)
    assert read_lastfailed(synth.path) == ("tests/test_beta.py::test_mul",)
