from __future__ import annotations

import json
import os
import pathlib
import stat
import subprocess

import pytest

from replay.cache import Cache
from replay.runner import (
    MODE_FULL,
    MODE_FULL_INSTRUMENTED,
    MODE_SUBSET,
    Outcome,
    RunResult,
    SysmonContextError,
    cached_run,
    chunk,
    collect,
    parse_report_log,
    result_from_dict,
    result_to_dict,
    run_full,
    run_key,
    run_subset,
)


def _checkout(repo, sha):
    subprocess.run(["git", "checkout", "-q", sha], cwd=repo, check=True)


def _write_report_log(path: pathlib.Path, records: list[dict]) -> pathlib.Path:
    path.write_text("".join(json.dumps(r) + "\n" for r in records), encoding="utf-8")
    return path


def _test_report(nodeid: str, when: str, outcome: str, duration: float = 0.0) -> dict:
    return {
        "$report_type": "TestReport",
        "nodeid": nodeid,
        "when": when,
        "outcome": outcome,
        "duration": duration,
    }


# --- collection -------------------------------------------------------------


def test_collect_lists_every_test_id(synth):
    _checkout(synth.path, synth.sha(2))
    ids = collect(synth.path)
    assert set(ids) == {
        "tests/test_alpha.py::test_add",
        "tests/test_beta.py::test_mul",
        "tests/test_gamma.py::test_sub",
    }


# --- the three run modes ----------------------------------------------------


def test_run_full_at_green_and_red_commits(synth):
    _checkout(synth.path, synth.sha(2))
    green = run_full(synth.path)
    assert green.exit_code == 0
    assert green.failing() == frozenset()
    assert set(green.durations()) == set(green.collected)
    assert green.wall_ms > 0

    _checkout(synth.path, synth.sha(3))
    red = run_full(synth.path)
    assert red.exit_code == 1
    assert red.failing() == frozenset({"tests/test_beta.py::test_mul"})


def test_run_subset_runs_only_what_it_is_given(synth):
    _checkout(synth.path, synth.sha(3))
    res = run_subset(synth.path, ["tests/test_alpha.py::test_add"])
    assert res.exit_code == 0
    assert [o.test for o in res.outcomes] == ["tests/test_alpha.py::test_add"]


def test_run_subset_of_nothing_never_invokes_pytest(synth):
    _checkout(synth.path, synth.sha(3))
    res = run_subset(synth.path, [])
    assert res == RunResult(outcomes=(), exit_code=0, wall_ms=0, collected=())


def test_instrumented_run_produces_a_coverage_db(synth):
    _checkout(synth.path, synth.sha(2))
    res = run_full(synth.path, instrumented=True, source_globs=("src",))
    assert res.exit_code == 0
    assert (synth.path / ".coverage").exists()


def test_every_mode_reports_per_test_durations_and_a_wall_clock(synth):
    """The instrumentation tax is only visible as its own column if all three
    modes carry their own wall-clock and per-test durations."""
    _checkout(synth.path, synth.sha(2))

    plain = run_full(synth.path)
    instrumented = run_full(synth.path, instrumented=True, source_globs=("src",))
    subset = run_subset(synth.path, ["tests/test_alpha.py::test_add"])

    for res in (plain, instrumented, subset):
        assert res.wall_ms > 0
        assert res.durations()
        assert all(d >= 0 for d in res.durations().values())
    assert set(plain.collected) == set(instrumented.collected)
    assert subset.collected == ("tests/test_alpha.py::test_add",)


# --- report-log parsing -----------------------------------------------------


def test_report_log_parsing_yields_pass_fail_skip_and_error(tmp_path):
    log = _write_report_log(
        tmp_path / "report.jsonl",
        [
            _test_report("t.py::passes", "setup", "passed", 0.001),
            _test_report("t.py::passes", "call", "passed", 0.010),
            _test_report("t.py::passes", "teardown", "passed", 0.001),
            _test_report("t.py::fails", "call", "failed", 0.020),
            _test_report("t.py::skips", "setup", "skipped", 0.0),
            _test_report("t.py::broken_fixture", "setup", "failed", 0.030),
        ],
    )
    got = {o.test: o.status for o in parse_report_log(log)}
    assert got == {
        "t.py::passes": "pass",
        "t.py::fails": "fail",
        "t.py::skips": "skip",
        "t.py::broken_fixture": "error",
    }


def test_report_log_parsing_sums_every_phase_into_one_duration(tmp_path):
    log = _write_report_log(
        tmp_path / "report.jsonl",
        [
            _test_report("t.py::x", "setup", "passed", 0.001),
            _test_report("t.py::x", "call", "passed", 0.010),
            _test_report("t.py::x", "teardown", "passed", 0.004),
        ],
    )
    assert parse_report_log(log) == (Outcome(test="t.py::x", status="pass", duration_ms=15),)


def test_a_teardown_failure_after_a_passing_call_is_an_error(tmp_path):
    log = _write_report_log(
        tmp_path / "report.jsonl",
        [
            _test_report("t.py::x", "call", "passed", 0.01),
            _test_report("t.py::x", "teardown", "failed", 0.01),
        ],
    )
    assert [o.status for o in parse_report_log(log)] == ["error"]


def test_a_collection_error_is_a_recorded_outcome(tmp_path):
    log = _write_report_log(
        tmp_path / "report.jsonl",
        [
            {
                "$report_type": "CollectReport",
                "nodeid": "tests/test_broken.py",
                "outcome": "failed",
                "longrepr": "ModuleNotFoundError: No module named 'zzz'",
            },
            {"$report_type": "CollectReport", "nodeid": "tests", "outcome": "passed"},
            _test_report("tests/test_ok.py::test_ok", "call", "passed", 0.01),
        ],
    )
    got = {o.test: o.status for o in parse_report_log(log)}
    assert got == {"tests/test_broken.py": "error", "tests/test_ok.py::test_ok": "pass"}


def test_a_collect_report_never_overwrites_a_real_test_outcome(tmp_path):
    """A module-level CollectReport shares a nodeid prefix with its tests; the
    per-test verdict is the authoritative one and must survive."""
    log = _write_report_log(
        tmp_path / "report.jsonl",
        [
            _test_report("tests/test_a.py::test_x", "call", "passed", 0.01),
            {
                "$report_type": "CollectReport",
                "nodeid": "tests/test_a.py::test_x",
                "outcome": "failed",
            },
        ],
    )
    assert [o.status for o in parse_report_log(log)] == ["pass"]


def test_a_collection_error_in_a_real_repo_is_recorded_not_a_crash(synth):
    """One unimportable test module must not cost the whole commit: the rest of
    the suite still runs and the broken module is one `error` outcome."""
    _checkout(synth.path, synth.sha(2))
    (synth.path / "tests" / "test_broken.py").write_text(
        "import zzz_module_that_does_not_exist\n\n\ndef test_bad():\n    assert True\n",
        encoding="utf-8",
    )

    res = run_full(synth.path)

    statuses = {o.test: o.status for o in res.outcomes}
    assert statuses.get("tests/test_broken.py") == "error"
    assert statuses.get("tests/test_alpha.py::test_add") == "pass"
    assert res.exit_code != 0
    assert "tests/test_broken.py" in res.failing()


def test_parse_report_log_ignores_blank_lines_and_other_record_types(tmp_path):
    log = tmp_path / "report.jsonl"
    log.write_text(
        json.dumps({"$report_type": "SessionStart"})
        + "\n\n"
        + json.dumps(_test_report("t.py::x", "call", "passed", 0.01))
        + "\n",
        encoding="utf-8",
    )
    assert [o.test for o in parse_report_log(log)] == ["t.py::x"]


# --- argv budgeting ---------------------------------------------------------


def test_chunk_splits_on_argv_budget():
    ids = [f"tests/test_x.py::test_{i:04d}" for i in range(500)]
    parts = chunk(ids, max_bytes=1000)
    assert len(parts) > 1
    assert [i for p in parts for i in p] == ids
    assert all(sum(len(i) + 1 for i in p) <= 1000 for p in parts)


def test_chunk_never_drops_a_test_longer_than_the_budget():
    long_id = "tests/" + "a" * 200 + ".py::test_x"
    parts = chunk(["short::a", long_id], max_bytes=10)
    assert [i for p in parts for i in p] == ["short::a", long_id]


# --- ctrace enforcement -----------------------------------------------------


def _stub_python(tmp_path: pathlib.Path, stderr_text: str) -> str:
    shim = tmp_path / "stub-python"
    shim.write_text(f'#!/bin/sh\necho "{stderr_text}" >&2\nexit 0\n', encoding="utf-8")
    shim.chmod(shim.stat().st_mode | stat.S_IEXEC)
    return str(shim)


def test_an_instrumented_run_that_lost_its_contexts_is_fatal(synth, tmp_path):
    python = _stub_python(tmp_path, "Coverage.py warning: no-sysmon-context")
    with pytest.raises(SysmonContextError):
        run_full(synth.path, python=python, instrumented=True, source_globs=("src",))


def test_the_same_warning_on_an_uninstrumented_run_is_not_fatal(synth, tmp_path):
    python = _stub_python(tmp_path, "Coverage.py warning: no-sysmon-context")
    assert run_full(synth.path, python=python).outcomes == ()


def test_an_instrumented_run_forces_ctrace(synth, tmp_path):
    dumped = tmp_path / "env.json"
    shim = tmp_path / "dump-env"
    shim.write_text(
        "#!/bin/sh\n"
        f'python3 -c "import json,os,sys; json.dump(dict(os.environ), open(sys.argv[1], \'w\'))" {dumped}\n'
        "exit 0\n",
        encoding="utf-8",
    )
    shim.chmod(shim.stat().st_mode | stat.S_IEXEC)

    run_full(synth.path, python=str(shim), instrumented=True, source_globs=("src",))

    assert json.loads(dumped.read_text(encoding="utf-8"))["COVERAGE_CORE"] == "ctrace"


# --- the cache from #121 ----------------------------------------------------


def test_a_run_result_round_trips_through_the_cache_payload():
    res = RunResult(
        outcomes=(Outcome("t.py::x", "fail", 12), Outcome("t.py::y", "skip", 0)),
        exit_code=1,
        wall_ms=345,
        collected=("t.py::x", "t.py::y"),
    )
    assert result_from_dict(result_to_dict(res)) == res


def test_a_cached_run_is_computed_once_and_replayed(cache_root):
    cache = Cache(cache_root, "cfg-digest")
    calls = []

    def build():
        calls.append(1)
        return RunResult(
            outcomes=(Outcome("t.py::x", "pass", 4),),
            exit_code=0,
            wall_ms=99,
            collected=("t.py::x",),
        )

    key = run_key(cache, MODE_FULL, "repo", "sha", "natural", "full")
    first = cached_run(cache, key, build)
    second = cached_run(cache, key, build)

    assert calls == [1]
    assert first.cached is False
    assert second.cached is True
    assert second.outcomes == first.outcomes
    assert second.exit_code == first.exit_code == 0
    assert second.wall_ms == first.wall_ms == 99


def test_a_config_change_makes_the_run_a_miss_not_a_stale_hit(cache_root):
    def build(wall):
        return lambda: RunResult(outcomes=(), exit_code=0, wall_ms=wall, collected=())

    old = Cache(cache_root, "cfg-a")
    cached_run(old, run_key(old, MODE_FULL, "repo", "sha", "natural", "full"), build(10))

    new = Cache(cache_root, "cfg-b")
    got = cached_run(new, run_key(new, MODE_FULL, "repo", "sha", "natural", "full"), build(20))

    assert got.wall_ms == 20
    assert got.cached is False


def test_run_keys_separate_the_three_modes_at_the_same_commit(cache_root):
    cache = Cache(cache_root, "cfg")
    coords = ("repo", "sha", "natural", "rtdd")
    full = run_key(cache, MODE_FULL, *coords)
    instrumented = run_key(cache, MODE_FULL_INSTRUMENTED, *coords)
    subset = run_key(cache, MODE_SUBSET, *coords, tests=("t.py::x",))

    assert len({full, instrumented, subset}) == 3


def test_a_subset_run_key_depends_on_the_selected_tests_not_their_order(cache_root):
    cache = Cache(cache_root, "cfg")
    coords = ("repo", "sha", "natural", "rtdd")
    a = run_key(cache, MODE_SUBSET, *coords, tests=("t.py::x", "t.py::y"))
    b = run_key(cache, MODE_SUBSET, *coords, tests=("t.py::y", "t.py::x"))
    c = run_key(cache, MODE_SUBSET, *coords, tests=("t.py::x",))

    assert a == b
    assert a != c


def test_a_cached_run_of_the_real_suite_skips_the_second_pytest(synth, cache_root):
    _checkout(synth.path, synth.sha(2))
    cache = Cache(cache_root, "cfg")
    key = run_key(cache, MODE_FULL, "synth", synth.sha(2), "natural", "full")

    first = cached_run(cache, key, lambda: run_full(synth.path))
    os.rename(synth.path / "tests", synth.path / "tests-hidden")
    second = cached_run(cache, key, lambda: run_full(synth.path))
    os.rename(synth.path / "tests-hidden", synth.path / "tests")

    assert first.failing() == frozenset()
    assert second.collected == first.collected
    assert second.durations() == first.durations()
    assert second.cached is True


def test_cacheprovider_flag_populates_pytest_cache(synth):
    _checkout(synth.path, synth.sha(3))
    run_full(synth.path, cacheprovider=True)
    assert (synth.path / ".pytest_cache" / "v" / "cache" / "lastfailed").exists()


def test_the_cacheprovider_is_disabled_by_default(synth):
    _checkout(synth.path, synth.sha(3))
    run_full(synth.path)
    assert not (synth.path / ".pytest_cache").exists()


# --- exec_args: the parallel baseline's execution mode ----------------------


def _argv_dump_python(tmp_path: pathlib.Path, dumped: pathlib.Path) -> str:
    """A `python` that records the argv it was handed and exits 0.

    The parallel baseline is defined entirely by the flags it adds, so the only
    honest assertion is on the argv pytest actually receives.
    """
    recorder = tmp_path / "record_argv.py"
    recorder.write_text(
        "import json, sys\n" f"json.dump(sys.argv[1:], open({str(dumped)!r}, 'w'))\n",
        encoding="utf-8",
    )
    shim = tmp_path / "dump-argv"
    shim.write_text(f'#!/bin/sh\nexec python3 {recorder} "$@"\n', encoding="utf-8")
    shim.chmod(shim.stat().st_mode | stat.S_IEXEC)
    return str(shim)


def test_a_subsets_exec_args_reach_the_pytest_invocation(synth, tmp_path):
    dumped = tmp_path / "argv.json"
    run_subset(
        synth.path,
        ["tests/test_alpha.py::test_add"],
        python=_argv_dump_python(tmp_path, dumped),
        exec_args=("-n", "auto"),
    )
    argv = json.loads(dumped.read_text(encoding="utf-8"))

    assert argv[:2] == ["-m", "pytest"]
    assert "-n" in argv and argv[argv.index("-n") + 1] == "auto"


def test_a_subset_without_exec_args_stays_serial(synth, tmp_path):
    dumped = tmp_path / "argv.json"
    run_subset(
        synth.path,
        ["tests/test_alpha.py::test_add"],
        python=_argv_dump_python(tmp_path, dumped),
    )

    assert "-n" not in json.loads(dumped.read_text(encoding="utf-8"))


def test_a_full_runs_exec_args_reach_the_pytest_invocation(synth, tmp_path):
    dumped = tmp_path / "argv.json"
    run_full(synth.path, python=_argv_dump_python(tmp_path, dumped), exec_args=("-n", "auto"))
    argv = json.loads(dumped.read_text(encoding="utf-8"))

    assert "-n" in argv and argv[argv.index("-n") + 1] == "auto"


def test_a_parallel_run_key_is_not_the_serial_one(cache_root):
    cache = Cache(cache_root, "cfg")
    coords = ("repo", "sha", "natural", "xdist")
    tests = ("t.py::x", "t.py::y")
    serial = run_key(cache, MODE_SUBSET, *coords, tests=tests)
    parallel = run_key(cache, MODE_SUBSET, *coords, tests=tests, exec_args=("-n", "auto"))

    assert serial != parallel
