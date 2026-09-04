"""The entry point is where every Global Constraint is enforced, once.

Each guard has a library-level test elsewhere; these are the tests that the CLI
actually *calls* them, in the right order, and turns each refusal into a
meaningful exit code. A guard that lives only in the library is a guard the
caller can forget.
"""

from __future__ import annotations

import json
import os
import pathlib
import sys
import textwrap

import pytest

from replay import cli
from replay.corpus import freeze
from replay.hardware import CI_ENV_VARS
from replay.replay import ReplayOutput

CORPUS_YAML = textwrap.dedent(
    """
    frozen_at: "2026-01-01"
    corpus_version: 1
    criteria:
      - "synthetic"
    admission:
      min_collected_tests: 400
      max_uninstrumented_full_suite_seconds: 600
      max_compiled_extensions_on_import_path: 0
    repos:
      - id: synth
        url: https://example.invalid/synth.git
        pin: "0123456789012345678901234567890123456789"
        replay_commits: 3
        python: "3.12"
        source_globs: ["src/**/*.py"]
        test_globs: ["tests/**/*.py"]
        measured:
          on_hardware: "synthetic"
          collected_tests: 900
          uninstrumented_full_suite_seconds: 12
          compiled_extensions_on_import_path: 0
          replay_commits_ceiling: 3
      - id: synth2
        url: https://example.invalid/synth2.git
        pin: "9876543210987654321098765432109876543210"
        replay_commits: 3
        python: "3.12"
        source_globs: ["src/**/*.py"]
        test_globs: ["tests/**/*.py"]
        measured:
          on_hardware: "synthetic"
          collected_tests: 900
          uninstrumented_full_suite_seconds: 12
          compiled_extensions_on_import_path: 0
          replay_commits_ceiling: 3
    excluded:
      - id: rejected
        reason: "not hermetic"
    """
).lstrip()


def _no_ci(monkeypatch):
    for name in CI_ENV_VARS:
        monkeypatch.delenv(name, raising=False)


def _corpus_at(tmp_path: pathlib.Path, *, frozen: bool = True) -> pathlib.Path:
    d = tmp_path / "corpus"
    d.mkdir(parents=True, exist_ok=True)
    yaml_path = d / "corpus.yaml"
    lock_path = d / "corpus.lock"
    yaml_path.write_text(CORPUS_YAML, encoding="utf-8")
    freeze(yaml_path, lock_path)
    if not frozen:
        # Exactly the drift the digest exists to catch: the corpus edited after
        # the freeze that produced the published results.
        yaml_path.write_text(CORPUS_YAML + "# edited after the freeze\n", encoding="utf-8")
    return d


def _fake_rtdd(tmp_path: pathlib.Path) -> str:
    p = tmp_path / "rtdd-fake"
    p.write_text("#!/bin/sh\necho 'rtdd 9.9.9-test'\n", encoding="utf-8")
    p.chmod(0o755)
    return str(p)


@pytest.fixture
def bench(tmp_path, monkeypatch):
    """Point the CLI's module-level paths at a throwaway tree."""
    d = _corpus_at(tmp_path)
    monkeypatch.setattr(cli, "CORPUS", d / "corpus.yaml")
    monkeypatch.setattr(cli, "LOCK", d / "corpus.lock")
    monkeypatch.setattr(cli, "WORK", tmp_path / "work")
    monkeypatch.setattr(cli, "CACHE", tmp_path / "cache")
    monkeypatch.setattr(cli, "RESULTS", tmp_path / "results")
    return tmp_path


@pytest.fixture
def stub_replay(monkeypatch):
    """Replace the expensive half of `replay` so the guards can be tested alone."""
    seen: dict = {}

    def fake_clone(url, pin, dest):
        seen["cloned"] = (url, pin)
        dest.mkdir(parents=True, exist_ok=True)
        return dest

    def fake_replay_repo(*, repo, spec, cfg, cache, hw, work_root, opts, **kw):
        seen["opts"] = opts
        seen["cfg"] = cfg
        seen["spec_commits"] = spec.replay_commits
        return ReplayOutput()

    def fake_write_results(out_dir, output, cfg, hw, strategy_ids, drift=None, **kw):
        seen["written"] = pathlib.Path(out_dir)
        seen["write_results_kwargs"] = kw
        return {"n_commits": 3}

    monkeypatch.setattr(cli, "clone_pinned", fake_clone)
    monkeypatch.setattr(cli, "replay_repo", fake_replay_repo)
    monkeypatch.setattr(cli, "write_results", fake_write_results)
    return seen


# --- the four subcommands ------------------------------------------------


def test_the_parser_exposes_exactly_the_five_documented_subcommands():
    parser = cli.build_parser()
    actions = [a for a in parser._actions if a.dest == "cmd"]
    assert actions, "the parser has no subcommand slot"
    assert set(actions[0].choices) == {"replay", "session", "report", "doctor", "audit"}


# --- the corpus guards ---------------------------------------------------


def test_replay_refuses_a_repo_outside_the_frozen_corpus(bench, stub_replay, capsys):
    rc = cli.main(["replay", "--repo", "not-in-the-corpus"])
    assert rc == cli.EXIT_GUARD
    assert "not in the frozen corpus" in capsys.readouterr().err
    assert "cloned" not in stub_replay


def test_session_refuses_a_repo_outside_the_frozen_corpus(bench, stub_replay, capsys):
    rc = cli.main(["session", "--repo", "not-in-the-corpus"])
    assert rc == cli.EXIT_GUARD
    assert "not in the frozen corpus" in capsys.readouterr().err


def test_replay_refuses_an_unfrozen_corpus_before_doing_any_work(
    tmp_path, monkeypatch, stub_replay, capsys
):
    d = _corpus_at(tmp_path, frozen=False)
    monkeypatch.setattr(cli, "CORPUS", d / "corpus.yaml")
    monkeypatch.setattr(cli, "LOCK", d / "corpus.lock")
    monkeypatch.setattr(cli, "RESULTS", tmp_path / "results")
    rc = cli.main(["replay", "--repo", "synth"])
    assert rc == cli.EXIT_GUARD
    assert "has changed since it was frozen" in capsys.readouterr().err
    assert "cloned" not in stub_replay


# --- the random/peer ordering guard --------------------------------------


def test_replay_refuses_random_without_its_peer_before_any_clone(bench, stub_replay, capsys):
    """`random` is ratio-matched against `rtdd`'s selection size (#189).

    Asking for it alone used to blow up mid-run inside the strategy itself; the
    combination is a usage error and must be rejected before any clone happens.
    """
    rc = cli.main(["replay", "--repo", "synth", "--strategies", "path,random"])
    assert rc == cli.EXIT_GUARD
    err = capsys.readouterr().err
    assert "random" in err
    assert "rtdd" in err
    assert "cloned" not in stub_replay


def test_replay_allows_random_when_its_peer_is_in_the_same_run(bench, stub_replay, monkeypatch):
    _no_ci(monkeypatch)
    rc = cli.main(
        ["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth", "--strategies", "rtdd,random"]
    )
    assert rc == cli.EXIT_OK
    assert "cloned" in stub_replay


# --- the CI wall-clock guard ---------------------------------------------


def test_replay_refuses_wall_clock_on_ci_and_still_computes_the_rest(
    bench, stub_replay, monkeypatch, capsys
):
    _no_ci(monkeypatch)
    monkeypatch.setenv("GITHUB_ACTIONS", "true")
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth"])
    err = capsys.readouterr().err
    assert rc == cli.EXIT_OK
    assert "wall-clock measurement refused" in err
    assert "GITHUB_ACTIONS" in err
    assert stub_replay["opts"].wallclock_enabled is False
    assert "written" in stub_replay, "the other metrics must still be computed and written"


def test_replay_measures_wall_clock_off_ci(bench, stub_replay, monkeypatch, capsys):
    _no_ci(monkeypatch)
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth"])
    assert rc == cli.EXIT_OK
    assert stub_replay["opts"].wallclock_enabled is True
    assert "wall-clock measurement refused" not in capsys.readouterr().err


def test_replay_honours_no_wallclock_without_claiming_ci(
    bench, stub_replay, monkeypatch, capsys
):
    _no_ci(monkeypatch)
    rc = cli.main(
        ["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth", "--no-wallclock"]
    )
    assert rc == cli.EXIT_OK
    assert stub_replay["opts"].wallclock_enabled is False
    assert "wall-clock measurement refused" not in capsys.readouterr().err


# --- doctor --------------------------------------------------------------


def test_doctor_reports_ci_and_the_corpus_digest(bench, monkeypatch, capsys):
    _no_ci(monkeypatch)
    monkeypatch.setenv("GITHUB_ACTIONS", "true")
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "doctor"])
    out = capsys.readouterr().out
    assert rc == cli.EXIT_OK
    assert "wall-clock: SUPPRESSED" in out
    assert "corpus digest" in out
    assert "strategies:" in out


def test_doctor_lists_all_seven_baselines_plus_rtdd(bench, monkeypatch, capsys):
    _no_ci(monkeypatch)
    cli.main(["--rtdd-binary", _fake_rtdd(bench), "doctor"])
    out = capsys.readouterr().out
    for sid in ("rtdd", "testmon", "path", "lf", "importgraph", "xdist", "random", "full"):
        assert sid in out


def test_doctor_reports_the_resolved_config_and_the_tool_versions(bench, monkeypatch, capsys):
    _no_ci(monkeypatch)
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "doctor"])
    out = capsys.readouterr().out
    assert rc == cli.EXIT_OK
    assert "config digest:" in out
    assert "rtdd binary: rtdd 9.9.9-test" in out
    assert "tool: pytest " in out
    assert "publishable: yes" in out


def test_doctor_exits_non_zero_when_the_rtdd_binary_is_absent(bench, monkeypatch, capsys):
    _no_ci(monkeypatch)
    rc = cli.main(["--rtdd-binary", str(bench / "no-such-binary"), "doctor"])
    out = capsys.readouterr().out
    assert rc == cli.EXIT_ENV
    assert "rtdd binary: MISSING" in out
    assert "publishable: no" in out


def test_doctor_exits_non_zero_when_the_corpus_is_not_frozen(tmp_path, monkeypatch, capsys):
    _no_ci(monkeypatch)
    d = _corpus_at(tmp_path, frozen=False)
    monkeypatch.setattr(cli, "CORPUS", d / "corpus.yaml")
    monkeypatch.setattr(cli, "LOCK", d / "corpus.lock")
    rc = cli.main(["--rtdd-binary", _fake_rtdd(tmp_path), "doctor"])
    captured = capsys.readouterr()
    assert rc == cli.EXIT_ENV
    assert "corpus: ERROR" in captured.out
    assert "publishable: no" in captured.out


# --- report --------------------------------------------------------------


def test_report_refuses_when_no_per_repo_summary_has_been_committed(bench, capsys):
    rc = cli.main(["report"])
    assert rc == cli.EXIT_GUARD
    assert "run `replay` first" in capsys.readouterr().err


def test_report_regenerates_the_aggregate_from_committed_summaries(bench, capsys):
    results = pathlib.Path(cli.RESULTS)
    # The aggregate spans the frozen corpus, so the summaries it weighs must be the
    # corpus's own repos — a directory for a repo the corpus does not admit is kept
    # but not weighed (#184).
    for repo_id in ("synth", "synth2"):
        d = results / repo_id
        d.mkdir(parents=True)
        (d / "summary.json").write_text(
            json.dumps(
                {
                    "repo_id": repo_id,
                    "strategies": {
                        "rtdd": {"selected_duration_fraction": {"num": 1.0, "den": 4.0}}
                    },
                }
            ),
            encoding="utf-8",
        )
    rc = cli.main(["report"])
    assert rc == cli.EXIT_OK
    aggregate = (results / "aggregate.md").read_text(encoding="utf-8")
    assert "| rtdd |" in aggregate
    assert "from 2 repos" in capsys.readouterr().out


def test_report_keeps_but_does_not_weigh_a_result_the_corpus_no_longer_admits(bench, capsys):
    """`sqlfluff`'s case (#184): dropped from the corpus, its published result kept.

    Deleting a number that was published is worse than superseding it, so the
    directory stays — but weighing it would make the aggregate span a corpus that no
    longer exists, and `results/aggregate.md` is the one cross-repo claim there is.
    """
    results = pathlib.Path(cli.RESULTS)
    for repo_id in ("synth", "synth2", "dropped"):
        d = results / repo_id
        d.mkdir(parents=True)
        (d / "summary.json").write_text(
            json.dumps(
                {
                    "repo_id": repo_id,
                    "strategies": {
                        "rtdd": {"selected_duration_fraction": {"num": 1.0, "den": 4.0}}
                    },
                }
            ),
            encoding="utf-8",
        )
    assert cli.main(["report"]) == cli.EXIT_OK
    assert "from 2 repos" in capsys.readouterr().out
    assert (results / "dropped" / "summary.json").exists(), "a published result is never deleted"


def test_audit_passes_on_a_corpus_that_meets_its_own_criteria(bench, capsys):
    assert cli.main(["audit"]) == cli.EXIT_OK
    assert "clean" in capsys.readouterr().out


def test_audit_fails_and_names_the_repo_that_breaches(bench, monkeypatch, capsys):
    """A validator that warns is a validator nothing obeys — it must exit non-zero."""
    corpus = pathlib.Path(cli.CORPUS)
    corpus.write_text(
        corpus.read_text(encoding="utf-8").replace(
            "uninstrumented_full_suite_seconds: 12",
            "uninstrumented_full_suite_seconds: 1450",
            1,
        ),
        encoding="utf-8",
    )
    freeze(corpus, pathlib.Path(cli.LOCK))
    assert cli.main(["audit"]) == cli.EXIT_GUARD
    out = capsys.readouterr().out
    assert "BREACH synth: uninstrumented full suite" in out
    assert "1450 s" in out and "600 s" in out


# --- exit codes ----------------------------------------------------------


def test_the_exit_codes_are_distinct_and_meaningful():
    assert cli.EXIT_OK == 0
    assert len({cli.EXIT_OK, cli.EXIT_GUARD, cli.EXIT_ENV}) == 3


# --- the per-repo environment --------------------------------------------


@pytest.fixture
def stub_provision(monkeypatch, tmp_path):
    """Record what the CLI asks for, without building a real venv."""
    from replay.envsetup import RepoEnv

    seen: dict = {}

    def fake_provision(spec, repo, root, **kw):
        seen["spec"] = spec
        seen["root"] = pathlib.Path(root)
        venv = tmp_path / "envs" / spec.id
        (venv / "bin").mkdir(parents=True, exist_ok=True)
        python = venv / "bin" / "python"
        if not python.exists():
            python.symlink_to(sys.executable)
        return RepoEnv(spec.id, venv, python, "d" * 64)

    monkeypatch.setattr(cli, "provision", fake_provision)
    return seen


def test_replay_runs_the_corpus_repo_in_its_own_provisioned_interpreter(
    bench, stub_replay, stub_provision, monkeypatch
):
    _no_ci(monkeypatch)
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth"])
    assert rc == cli.EXIT_OK
    env_python = stub_provision["spec"].id
    assert stub_replay["opts"].python.endswith(f"envs/{env_python}/bin/python")
    # `adapters/python.yaml` runs a bare `pytest`, so the venv has to win the
    # PATH lookup the rtdd binary itself performs.
    assert pathlib.Path(stub_replay["opts"].python).parent == pathlib.Path(
        os.environ["PATH"].split(os.pathsep)[0]
    )


def test_replay_commits_can_be_bounded_and_the_bound_is_published(
    bench, stub_replay, stub_provision, monkeypatch
):
    _no_ci(monkeypatch)
    rc = cli.main(
        ["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth", "--replay-commits", "2"]
    )
    assert rc == cli.EXIT_OK
    # The published config has to say how many commits produced the numbers.
    assert stub_replay["cfg"].replay_commits == 2
    assert stub_replay["spec_commits"] == 2


def test_replay_defaults_to_the_frozen_commit_count(
    bench, stub_replay, stub_provision, monkeypatch
):
    _no_ci(monkeypatch)
    cli.main(["--rtdd-binary", _fake_rtdd(bench), "replay", "--repo", "synth"])
    assert stub_replay["cfg"].replay_commits == 3
    assert stub_replay["spec_commits"] == 3


def test_session_runs_the_drift_worktree_with_its_own_source_in_front(
    bench, stub_provision, monkeypatch, tmp_path
):
    """The drift session needs the same import fix the replay has.

    It seeds `rtdd` in a worktree at an older commit while the repo is installed
    editable from the clone at the pin; without the worktree's source in front of
    `PYTHONPATH` the suite imports the pin's flask against the older tree's tests,
    which does not even collect.
    """
    _no_ci(monkeypatch)
    monkeypatch.setattr(cli, "RESULTS", tmp_path / "results")
    seen: dict = {}

    def fake_clone(url, pin, dest):
        dest.mkdir(parents=True, exist_ok=True)
        return dest

    class _Point:
        parent = "p"

    monkeypatch.setattr(cli, "clone_pinned", fake_clone)
    monkeypatch.setattr(cli, "replay_points", lambda repo, pin, n: [_Point()])
    monkeypatch.setattr(cli, "add_worktree", lambda repo, sha, work: None)
    monkeypatch.setattr(cli, "remove_worktree", lambda repo, work: None)

    def record_seed(work, binary="rtdd"):
        seen["pythonpath"] = os.environ.get("PYTHONPATH", "")
        seen["work"] = pathlib.Path(work)

    monkeypatch.setattr(cli.rtddio, "seed", record_seed)

    from replay.session import DriftCurve

    monkeypatch.setattr(cli, "run_drift", lambda *a, **kw: DriftCurve("synth", "p", ()))
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "session", "--repo", "synth", "--cycles", "2"])
    assert rc == cli.EXIT_OK
    assert seen["pythonpath"].split(os.pathsep)[0].startswith(str(seen["work"]))


def test_session_does_not_overwrite_the_replay_config_that_produced_the_table(
    bench, stub_provision, monkeypatch, tmp_path
):
    """`drift.json` lands beside `summary.md`, and must not replace its `config.json`.

    A drift session runs a different config — its own cycle count, one strategy —
    so writing it over the replay's `config.json` would leave the published table
    stamped with a config that did not produce it, which is the one thing the
    results layout exists to prevent.
    """
    _no_ci(monkeypatch)
    results = tmp_path / "results"
    (results / "synth").mkdir(parents=True)
    existing = '{"config":{"replay_commits":25},"hardware":{}}\n'
    (results / "synth" / "config.json").write_text(existing, encoding="utf-8")
    monkeypatch.setattr(cli, "RESULTS", results)

    def fake_clone(url, pin, dest):
        dest.mkdir(parents=True, exist_ok=True)
        return dest

    monkeypatch.setattr(cli, "clone_pinned", fake_clone)
    monkeypatch.setattr(cli, "replay_points", lambda repo, pin, n: [object()])
    monkeypatch.setattr(cli, "add_worktree", lambda repo, sha, work: None)
    monkeypatch.setattr(cli, "remove_worktree", lambda repo, work: None)

    class _Point:
        parent = "p"

    monkeypatch.setattr(cli, "replay_points", lambda repo, pin, n: [_Point()])
    monkeypatch.setattr(cli.rtddio, "seed", lambda work, binary="rtdd": None)

    from replay.session import DriftCurve

    monkeypatch.setattr(cli, "run_drift", lambda *a, **kw: DriftCurve("synth", "p", ()))
    rc = cli.main(["--rtdd-binary", _fake_rtdd(bench), "session", "--repo", "synth", "--cycles", "4"])
    assert rc == cli.EXIT_OK
    assert (results / "synth" / "config.json").read_text(encoding="utf-8") == existing
    drift = json.loads((results / "synth" / "drift.json").read_text(encoding="utf-8"))
    # The curve still carries the config that produced it — just not by
    # overwriting somebody else's.
    assert drift["config"]["replay_commits"] == 4
    assert drift["hardware"]


# --- --corpus-version on session and report (#189) -----------------------


def test_session_threads_corpus_version_through_to_the_loader(
    bench, stub_provision, monkeypatch
):
    """`session --repo x --corpus-version N` used to ignore N entirely (#189)."""
    _no_ci(monkeypatch)
    calls: list[int | None] = []
    original = cli._corpus

    def spy(version=None):
        calls.append(version)
        return original(version)

    monkeypatch.setattr(cli, "_corpus", spy)

    def fake_clone(url, pin, dest):
        dest.mkdir(parents=True, exist_ok=True)
        return dest

    class _Point:
        parent = "p"

    monkeypatch.setattr(cli, "clone_pinned", fake_clone)
    monkeypatch.setattr(cli, "replay_points", lambda repo, pin, n: [_Point()])
    monkeypatch.setattr(cli, "add_worktree", lambda repo, sha, work: None)
    monkeypatch.setattr(cli, "remove_worktree", lambda repo, work: None)
    monkeypatch.setattr(cli.rtddio, "seed", lambda work, binary="rtdd": None)

    from replay.session import DriftCurve

    monkeypatch.setattr(cli, "run_drift", lambda *a, **kw: DriftCurve("synth", "p", ()))
    rc = cli.main(
        ["--rtdd-binary", _fake_rtdd(bench), "session", "--repo", "synth", "--cycles", "1",
         "--corpus-version", "1"]
    )
    assert rc == cli.EXIT_OK
    assert calls == [1]


def test_report_threads_corpus_version_through_to_the_loader(bench, capsys, monkeypatch):
    """`report --corpus-version N` used to ignore N entirely (#189)."""
    calls: list[int | None] = []
    original = cli._corpus

    def spy(version=None):
        calls.append(version)
        return original(version)

    monkeypatch.setattr(cli, "_corpus", spy)
    cli.main(["report", "--corpus-version", "1"])
    assert calls == [1]


def test_the_cache_can_outlive_the_checkout(monkeypatch, tmp_path):
    """The fleet reaps its worktree on every claim; a cache inside it dies with it."""
    import importlib

    monkeypatch.setenv("RTDD_BENCH_CACHE", str(tmp_path / "durable"))
    reloaded = importlib.reload(cli)
    try:
        assert reloaded.CACHE == tmp_path / "durable"
        monkeypatch.delenv("RTDD_BENCH_CACHE")
        assert importlib.reload(cli).CACHE == reloaded.BENCH / "cache"
    finally:
        importlib.reload(cli)
