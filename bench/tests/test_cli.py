"""The entry point is where every Global Constraint is enforced, once.

Each guard has a library-level test elsewhere; these are the tests that the CLI
actually *calls* them, in the right order, and turns each refusal into a
meaningful exit code. A guard that lives only in the library is a guard the
caller can forget.
"""

from __future__ import annotations

import json
import pathlib
import textwrap

import pytest

from replay import cli
from replay.corpus import freeze
from replay.hardware import CI_ENV_VARS
from replay.replay import ReplayOutput

CORPUS_YAML = textwrap.dedent(
    """
    frozen_at: "2026-01-01"
    criteria:
      - "synthetic"
    repos:
      - id: synth
        url: https://example.invalid/synth.git
        pin: "0123456789012345678901234567890123456789"
        replay_commits: 3
        python: "3.12"
        source_globs: ["src/**/*.py"]
        test_globs: ["tests/**/*.py"]
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
        return ReplayOutput()

    def fake_write_results(out_dir, output, cfg, hw, strategy_ids, drift=None):
        seen["written"] = pathlib.Path(out_dir)
        return {"n_commits": 3}

    monkeypatch.setattr(cli, "clone_pinned", fake_clone)
    monkeypatch.setattr(cli, "replay_repo", fake_replay_repo)
    monkeypatch.setattr(cli, "write_results", fake_write_results)
    return seen


# --- the four subcommands ------------------------------------------------


def test_the_parser_exposes_exactly_the_four_documented_subcommands():
    parser = cli.build_parser()
    actions = [a for a in parser._actions if a.dest == "cmd"]
    assert actions, "the parser has no subcommand slot"
    assert set(actions[0].choices) == {"replay", "session", "report", "doctor"}


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
    for repo_id in ("alpha", "beta"):
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


# --- exit codes ----------------------------------------------------------


def test_the_exit_codes_are_distinct_and_meaningful():
    assert cli.EXIT_OK == 0
    assert len({cli.EXIT_OK, cli.EXIT_GUARD, cli.EXIT_ENV}) == 3
