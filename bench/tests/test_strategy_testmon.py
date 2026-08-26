from __future__ import annotations

import subprocess
import sys

from replay.gitwork import Change
from replay.strategies.base import CommitContext
from replay.strategies.testmon import Testmon

ALL = (
    "tests/test_alpha.py::test_add",
    "tests/test_beta.py::test_mul",
    "tests/test_gamma.py::test_sub",
)


def _ctx(work, changed):
    return CommitContext(
        repo_id="synth",
        variant="natural",
        commit="c",
        parent="p",
        work=work,
        changed=tuple(Change(p, s) for p, s in changed),
        all_tests=ALL,
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python=sys.executable,
    )


def test_testmon_selects_only_the_impacted_test_after_a_source_edit(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    s = Testmon()
    s.prepare(_ctx(synth.path, []))
    assert (synth.path / ".testmondata").exists()

    (synth.path / "src" / "beta.py").write_text(
        "def mul(a, b):\n    return a * b + 1\n", encoding="utf-8"
    )
    sel = s.select(_ctx(synth.path, [("src/beta.py", "M")]))
    assert "tests/test_beta.py::test_mul" in sel.tests
    assert "tests/test_alpha.py::test_add" not in sel.tests
    assert sel.escalated is False


def test_missing_database_is_an_escalation(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    db = synth.path / ".testmondata"
    if db.exists():
        db.unlink()
    sel = Testmon().select(_ctx(synth.path, [("src/beta.py", "M")]))
    assert sel.escalated is True
    assert sel.tests == ALL


def test_the_testmon_baseline_is_registered_under_its_plan_id():
    from replay.strategies.base import get

    s = get("testmon")
    assert s.id == "testmon"
    assert s.needs_parent_state is True
