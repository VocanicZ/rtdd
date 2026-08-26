"""The rtdd binary adapter: schema parsing, failure paths, and the argv contract.

Every test here except the last runs against a *stub* binary written into
``tmp_path``, so the whole module passes on a machine with no ``rtdd`` and no
corpus repo. The last test is the contract test: it runs the real binary and is
the only thing that can catch the schema drifting away from what the parsers
below assume.
"""

from __future__ import annotations

import inspect
import json
import pathlib
import shutil
import subprocess

import pytest

from replay import rtddio
from replay.rtddio import (
    RtddError,
    expand_ranges,
    parse_run,
    parse_which,
    read_cycles,
)

# A trimmed but structurally faithful `rtdd which --base HEAD --json` document,
# schema v1 (docs/plans/00-interfaces.md). Real output carries every changed path
# including untracked noise; the shape is what matters here.
WHICH_PAYLOAD = {
    "schema": 1,
    "command": "which",
    "base": "HEAD",
    "adapter": "python",
    "tier": "T0",
    "reason": "tests whose recorded coverage intersects the changed set",
    "complete": True,
    "warnings": [],
    "changed": [
        {
            "path": "src/alpha.py",
            "status": "modified",
            "instrumentable": True,
            "lines": [{"start": 2, "end": 2}],
        },
        {"path": "docs/notes.md", "status": "added", "instrumentable": False, "lines": []},
    ],
    "selection": {
        "count": 2,
        "direct": ["tests/test_new.py::test_n"],
        "tests": ["tests/test_new.py::test_n", "tests/test_alpha.py::test_add"],
        "import_fallback": {},
    },
    "run": {
        "executed": False,
        "passed": 0,
        "failed": 0,
        "skipped": 0,
        "errored": 0,
        "failures": [],
        "duration_ms": 0,
    },
    "uncovered": {
        "available": False,
        "reason": "the uncovered-change signal requires fresh post-run coverage",
        "summary": {
            "files": 0,
            "covered_lines": 0,
            "uncovered_lines": 0,
            "import_time_lines": 0,
        },
    },
    "unmapped_files": [],
    "exit_code": 0,
}

RUN_PAYLOAD = {
    "schema": 1,
    "command": "run",
    "base": "HEAD",
    "adapter": "python",
    "tier": "T0",
    "reason": "tests whose recorded coverage intersects the changed set",
    "complete": True,
    "warnings": [],
    "changed": [
        {
            "path": "src/alpha.py",
            "status": "modified",
            "instrumentable": True,
            "lines": [{"start": 52, "end": 53}],
        }
    ],
    "selection": {
        "count": 1,
        "direct": [],
        "tests": ["tests/test_alpha.py::test_add"],
        "import_fallback": {},
    },
    "run": {
        "executed": True,
        "passed": 0,
        "failed": 1,
        "skipped": 0,
        "errored": 0,
        "failures": ["tests/test_alpha.py::test_add"],
        "duration_ms": 12,
    },
    "uncovered": {
        "available": True,
        "files": [
            {
                "path": "src/alpha.py",
                "ranges": [
                    {"start": 52, "end": 53, "class": "uncovered"},
                    {"start": 60, "end": 60, "class": "covered"},
                ],
                "uncovered_lines": 2,
            },
            {
                "path": "src/constants.py",
                "ranges": [{"start": 1, "end": 1, "class": "import-time"}],
                "uncovered_lines": 0,
            },
        ],
        "summary": {
            "files": 2,
            "covered_lines": 1,
            "uncovered_lines": 2,
            "import_time_lines": 1,
        },
    },
    "unmapped_files": [],
    "exit_code": 1,
}


def _stub(
    tmp_path: pathlib.Path,
    name: str,
    *,
    stdout: str = "",
    stderr: str = "",
    code: int = 0,
) -> pathlib.Path:
    """Write an executable stub that replays fixed output and logs its argv.

    Payloads go through files rather than the script body so no quoting rule of
    the host shell can corrupt a JSON fixture.
    """
    home = tmp_path / name
    home.mkdir(parents=True, exist_ok=True)
    (home / "stdout").write_text(stdout)
    (home / "stderr").write_text(stderr)
    binary = home / "rtdd"
    binary.write_text(
        "#!/bin/sh\n"
        f'printf "%s\\n" "$@" >> "{home}/argv"\n'
        f'cat "{home}/stdout"\n'
        f'cat "{home}/stderr" >&2\n'
        f"exit {code}\n"
    )
    binary.chmod(0o755)
    return binary


def _argv(binary: pathlib.Path) -> list[str]:
    return (binary.parent / "argv").read_text().splitlines()


def _work(tmp_path: pathlib.Path, cycles: int | None = None) -> pathlib.Path:
    work = tmp_path / "work"
    work.mkdir(exist_ok=True)
    if cycles is not None:
        (work / ".rtdd").mkdir(exist_ok=True)
        (work / ".rtdd" / "meta.json").write_text(
            json.dumps({"v": 1, "adapter": "python", "seeded_at": "abc1234", "cycles": cycles})
        )
    return work


# --- parsing -----------------------------------------------------------------


def test_parse_which_reads_the_v1_selection_document():
    w = parse_which(WHICH_PAYLOAD, wall_ms=17, cycles=3)
    assert w.tier == "T0"
    assert w.reason == "tests whose recorded coverage intersects the changed set"
    assert w.tests == ("tests/test_new.py::test_n", "tests/test_alpha.py::test_add")
    assert w.direct == ("tests/test_new.py::test_n",)
    assert w.changed == ("src/alpha.py", "docs/notes.md")
    assert w.cycles == 3
    assert w.wall_ms == 17
    assert w.complete is True
    assert w.warnings == ()
    assert not w.escalated()


def test_parse_which_flags_t2_as_escalation():
    payload = dict(WHICH_PAYLOAD, tier="T2", reason="pyproject.toml changed", complete=False)
    w = parse_which(payload, wall_ms=1)
    assert w.escalated()
    assert w.reason == "pyproject.toml changed"


def test_parse_which_carries_complete_and_warnings():
    payload = dict(
        WHICH_PAYLOAD,
        tier="T2",
        complete=False,
        warnings=["T2 means the full suite. rtdd does not enumerate it here."],
        selection={"count": 0, "direct": [], "tests": [], "import_fallback": {}},
    )
    w = parse_which(payload, wall_ms=1)
    assert w.complete is False
    assert w.warnings == ("T2 means the full suite. rtdd does not enumerate it here.",)


def test_parse_which_rejects_an_unknown_schema_version():
    with pytest.raises(RtddError, match="schema"):
        parse_which(dict(WHICH_PAYLOAD, schema=2), wall_ms=1)


def test_parse_which_rejects_a_document_with_no_schema_key():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "schema"}
    with pytest.raises(RtddError, match="schema"):
        parse_which(payload, wall_ms=1)


def test_parse_which_rejects_a_document_with_no_selection():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "selection"}
    with pytest.raises(RtddError, match="selection"):
        parse_which(payload, wall_ms=1)


def test_parse_which_rejects_a_document_with_no_tier():
    payload = {k: v for k, v in WHICH_PAYLOAD.items() if k != "tier"}
    with pytest.raises(RtddError, match="tier"):
        parse_which(payload, wall_ms=1)


def test_expand_ranges_is_inclusive():
    got = expand_ranges([{"path": "src/a.py", "ranges": [{"start": 52, "end": 54}]}])
    assert got == frozenset({("src/a.py", 52), ("src/a.py", 53), ("src/a.py", 54)})


def test_expand_ranges_filters_by_class():
    entries = [
        {
            "path": "src/a.py",
            "ranges": [
                {"start": 1, "end": 1, "class": "uncovered"},
                {"start": 2, "end": 2, "class": "import-time"},
            ],
        }
    ]
    assert expand_ranges(entries, {"uncovered"}) == frozenset({("src/a.py", 1)})
    assert expand_ranges(entries, {"import-time"}) == frozenset({("src/a.py", 2)})


def test_parse_run_separates_uncovered_from_import_time():
    r = parse_run(RUN_PAYLOAD, wall_ms=900)
    assert r.uncovered == frozenset({("src/alpha.py", 52), ("src/alpha.py", 53)})
    assert r.import_time == frozenset({("src/constants.py", 1)})
    assert r.tier == "T0"
    assert r.tests == ("tests/test_alpha.py::test_add",)
    assert r.exit_code == 1
    assert r.wall_ms == 900


def test_parse_run_keeps_covered_lines_out_of_both_sets():
    r = parse_run(RUN_PAYLOAD, wall_ms=1)
    assert ("src/alpha.py", 60) not in r.uncovered
    assert ("src/alpha.py", 60) not in r.import_time


def test_parse_run_derives_outcomes_from_the_named_failures():
    r = parse_run(RUN_PAYLOAD, wall_ms=1)
    assert r.outcomes == (("tests/test_alpha.py::test_add", "fail", 0),)
    assert (r.passed, r.failed, r.skipped, r.errored) == (0, 1, 0, 0)
    assert r.failures == ("tests/test_alpha.py::test_add",)
    assert r.duration_ms == 12


def test_parse_run_marks_an_empty_selection_report_unavailable():
    payload = dict(
        RUN_PAYLOAD,
        tier="empty",
        selection={"count": 0, "direct": [], "tests": [], "import_fallback": {}},
        run={
            "executed": False,
            "passed": 0,
            "failed": 0,
            "skipped": 0,
            "errored": 0,
            "failures": [],
            "duration_ms": 0,
        },
        uncovered={
            "available": False,
            "reason": "requires fresh post-run coverage",
            "summary": {
                "files": 0,
                "covered_lines": 0,
                "uncovered_lines": 0,
                "import_time_lines": 0,
            },
        },
        exit_code=0,
    )
    r = parse_run(payload, wall_ms=1)
    assert r.executed is False
    assert r.uncovered_available is False
    assert r.unavailable_reason == "requires fresh post-run coverage"
    assert r.uncovered == frozenset()


def test_parse_run_rejects_an_executed_run_with_no_uncovered_report():
    payload = dict(
        RUN_PAYLOAD,
        uncovered={
            "available": False,
            "reason": "requires fresh post-run coverage",
            "summary": {
                "files": 0,
                "covered_lines": 0,
                "uncovered_lines": 0,
                "import_time_lines": 0,
            },
        },
    )
    with pytest.raises(RtddError, match="executed"):
        parse_run(payload, wall_ms=1)


def test_parse_which_rejects_a_malformed_changed_entry():
    payload = dict(WHICH_PAYLOAD, changed=["src/alpha.py"])
    with pytest.raises(RtddError, match="malformed"):
        parse_which(payload, wall_ms=1)


def test_parse_run_rejects_a_range_entry_with_no_path():
    payload = dict(
        RUN_PAYLOAD,
        uncovered={
            "available": True,
            "files": [{"ranges": [{"start": 1, "end": 1, "class": "uncovered"}]}],
            "summary": {
                "files": 1,
                "covered_lines": 0,
                "uncovered_lines": 1,
                "import_time_lines": 0,
            },
        },
    )
    with pytest.raises(RtddError, match="malformed"):
        parse_run(payload, wall_ms=1)


def test_parse_run_rejects_an_unknown_schema_version():
    with pytest.raises(RtddError, match="schema"):
        parse_run(dict(RUN_PAYLOAD, schema=99), wall_ms=1)


# --- cycles ------------------------------------------------------------------


def test_read_cycles_reads_meta_json(tmp_path):
    assert read_cycles(_work(tmp_path, cycles=7)) == 7


def test_read_cycles_is_zero_before_the_first_seed(tmp_path):
    assert read_cycles(_work(tmp_path)) == 0


def test_read_cycles_rejects_a_malformed_meta_json(tmp_path):
    work = _work(tmp_path, cycles=0)
    (work / ".rtdd" / "meta.json").write_text("{not json")
    with pytest.raises(RtddError, match="meta.json"):
        read_cycles(work)


# --- subprocess failure paths ------------------------------------------------


def test_which_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.which(_work(tmp_path), binary=str(tmp_path / "no-such-rtdd"))


def test_which_raises_on_non_json_stdout(tmp_path):
    b = _stub(tmp_path, "b", stdout="rtdd: something human\n")
    with pytest.raises(RtddError, match="non-JSON"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_a_fatal_exit_code(tmp_path):
    b = _stub(tmp_path, "b", stderr="rtdd: git unavailable\n", code=3)
    with pytest.raises(RtddError, match="exited 3"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_which_raises_on_a_failing_exit_code(tmp_path):
    # `which` runs nothing, so it can never legitimately report a test failure.
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD), code=1)
    with pytest.raises(RtddError, match="exited 1"):
        rtddio.which(_work(tmp_path), binary=str(b))


def test_seed_raises_on_a_non_zero_exit(tmp_path):
    b = _stub(tmp_path, "b", stderr="rtdd: pytest exited 2\n", code=3)
    with pytest.raises(RtddError, match="exited 3"):
        rtddio.seed(_work(tmp_path), binary=str(b))


def test_run_accepts_exit_1_as_a_failing_test(tmp_path):
    b = _stub(tmp_path, "b", stdout=json.dumps(RUN_PAYLOAD), code=1)
    r = rtddio.run(_work(tmp_path), binary=str(b))
    assert r.exit_code == 1
    assert r.failures == ("tests/test_alpha.py::test_add",)


def test_run_raises_on_a_fatal_exit_code(tmp_path):
    b = _stub(tmp_path, "b", stderr="rtdd: unreadable coverage\n", code=3)
    with pytest.raises(RtddError, match="exited 3"):
        rtddio.run(_work(tmp_path), binary=str(b))


def test_run_raises_on_non_json_stdout(tmp_path):
    b = _stub(tmp_path, "b", stdout="EMPTY SELECTION - nothing ran.\n")
    with pytest.raises(RtddError, match="non-JSON"):
        rtddio.run(_work(tmp_path), binary=str(b))


def test_which_reads_cycles_from_meta_json(tmp_path):
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD))
    w = rtddio.which(_work(tmp_path, cycles=5), binary=str(b))
    assert w.cycles == 5


def test_wall_ms_is_measured_not_reported(tmp_path):
    b = _stub(tmp_path, "b", stdout=json.dumps(WHICH_PAYLOAD))
    w = rtddio.which(_work(tmp_path), binary=str(b))
    assert w.wall_ms >= 0


# --- the argv contract: shipped defaults only --------------------------------


def test_the_harness_passes_no_tuning_flags(tmp_path):
    seed_bin = _stub(tmp_path, "s")
    which_bin = _stub(tmp_path, "w", stdout=json.dumps(WHICH_PAYLOAD))
    run_bin = _stub(tmp_path, "r", stdout=json.dumps(RUN_PAYLOAD), code=1)
    work = _work(tmp_path)

    rtddio.seed(work, binary=str(seed_bin))
    rtddio.which(work, binary=str(which_bin))
    rtddio.run(work, binary=str(run_bin))

    assert _argv(seed_bin) == ["seed"]
    assert _argv(which_bin) == ["which", "--base", "HEAD", "--json"]
    assert _argv(run_bin) == ["run", "--base", "HEAD", "--json"]


def test_no_entry_point_accepts_extra_rtdd_arguments():
    # A knob for extra argv is how "shipped defaults only" quietly stops being true.
    for fn in (rtddio.seed, rtddio.which, rtddio.run):
        params = set(inspect.signature(fn).parameters)
        assert params <= {"work", "binary", "base"}, fn.__name__


# --- version capture for RunConfig -------------------------------------------


def test_rtdd_version_prefers_the_binarys_own_version_output(tmp_path):
    b = _stub(tmp_path, "v", stdout="rtdd 1.2.3 (a3f21e0)\n")
    assert rtddio.rtdd_version(str(b)) == "rtdd 1.2.3 (a3f21e0)"


def test_rtdd_version_falls_back_to_a_content_digest(tmp_path):
    # The shipped CLI has no --version subcommand (it exits 2 on an unknown one),
    # so the binary's own bytes are the reproducible identity RunConfig records.
    b = _stub(tmp_path, "v", stderr="rtdd: unknown command\n", code=2)
    got = rtddio.rtdd_version(str(b))
    assert got.startswith("sha256:")
    assert len(got) == len("sha256:") + 64


def test_rtdd_version_raises_when_the_binary_is_missing(tmp_path):
    with pytest.raises(RtddError, match="not found"):
        rtddio.rtdd_version(str(tmp_path / "no-such-rtdd"))


# --- the contract test: the real binary --------------------------------------


@pytest.mark.skipif(shutil.which("rtdd") is None, reason="rtdd binary not on PATH")
def test_real_binary_honours_the_consumed_schema(synth, monkeypatch):
    # The adapter's `pytest` is the console script, which does not put the repo
    # root on sys.path the way `python -m pytest` does; PYTHONPATH is environment
    # setup the orchestrator owns, not a tuning flag passed to rtdd.
    monkeypatch.setenv("PYTHONPATH", str(synth.path))
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    subprocess.run(["rtdd", "init"], cwd=synth.path, check=True, capture_output=True)

    rtddio.seed(synth.path)
    (synth.path / "src" / "alpha.py").write_text("def add(a, b):\n    return a + b + 1\n")

    w = rtddio.which(synth.path)
    assert w.tier in {"empty", "direct", "T0", "T1", "T2"}
    assert "tests/test_alpha.py::test_add" in w.tests
    assert "src/alpha.py" in w.changed

    r = rtddio.run(synth.path)
    assert r.exit_code == 1
    assert r.failures == ("tests/test_alpha.py::test_add",)
    assert r.uncovered_available is True


def test_seed_accepts_exit_1_because_a_red_test_at_the_base_is_data(tmp_path):
    """`rtdd seed` exits 1 when a test failed while seeding — the map is still built.

    Real history has commits whose suite is red, and `clean_tree_failures` measures
    exactly that so `F_full` can subtract it. Treating the exit code as fatal would
    abort the whole replay on the first such commit, throwing away every commit
    after it because one of them had a failing test.
    """
    b = _stub(tmp_path, "b", stdout="seeded 490 tests at a29f88ce\n", code=1)
    rtddio.seed(_work(tmp_path), binary=str(b))


def test_seed_still_raises_on_a_usage_or_environment_exit(tmp_path):
    for code in (2, 3):
        b = _stub(tmp_path, f"b{code}", stderr="rtdd: no adapter matched\n", code=code)
        with pytest.raises(RtddError, match=f"exited {code}"):
            rtddio.seed(_work(tmp_path), binary=str(b))
