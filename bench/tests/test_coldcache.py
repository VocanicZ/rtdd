"""Is the bench cache actually cold?

`.github/workflows/ci.yml` runs the bench gates on a hosted runner with no
`RTDD_BENCH_CACHE` and no populated `~/.cache/rtdd-bench` — the ~201 MB store of
collected suites and full-suite timings every local run has had. A "cold-cache" run
that silently fell back to the developer's warm store proves nothing, and it fails
*open*: nothing crashes, every gate is green, and the claim is worthless.

So the coldness is checked, before the gates run, by :mod:`replay.coldcache` — and the
check itself is checked here. Two stores have to be cold, not one:

* the **replay** store, `RTDD_BENCH_CACHE` or `bench/cache/`, which `replay/cli.py` reads;
* the **driver** store, `~/.cache/rtdd-bench`, which `bench/swebench/run_arm.py` reaches
  through `Path.home()` and which `RTDD_BENCH_CACHE` does not move at all.

Pointing the variable at an empty temp directory therefore leaves the second one warm.
That is the exact silent fallback this module exists to catch.
"""

from __future__ import annotations

import os
import pathlib

from replay import cli, coldcache


def _entry(root: pathlib.Path, name: str = "json/ab/deadbeef.json") -> None:
    p = root / name
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text('{"collected": ["a"]}', encoding="utf-8")


# --- the guard resolves the paths the harness itself would use --------------------


def test_the_replay_root_is_the_variable_when_it_is_set(tmp_path) -> None:
    p = tmp_path / "cold"
    assert coldcache.replay_root({"RTDD_BENCH_CACHE": str(p)}) == p


def test_an_unset_variable_falls_back_to_bench_cache_like_the_cli_does() -> None:
    assert coldcache.replay_root({}) == coldcache.BENCH / "cache"


def test_the_replay_root_expands_a_tilde_because_the_cli_does(tmp_path) -> None:
    got = coldcache.replay_root({"RTDD_BENCH_CACHE": "~/cold-store"})
    assert "~" not in str(got)
    assert got == pathlib.Path("~/cold-store").expanduser()


def test_the_replay_root_agrees_with_the_cli_under_this_processs_environment() -> None:
    """The drift guard. `replay/cli.py` computes `CACHE` once at import from
    `os.environ`; if this module ever computed it differently, a cold run would be
    checking a directory the harness does not use."""
    assert coldcache.replay_root(os.environ) == cli.CACHE


def test_the_driver_root_follows_HOME_and_not_the_variable(tmp_path) -> None:
    """`bench/swebench/run_arm.py` never reads `RTDD_BENCH_CACHE`. Only `HOME` moves it."""
    env = {"HOME": str(tmp_path / "home"), "RTDD_BENCH_CACHE": str(tmp_path / "cold")}
    assert coldcache.driver_root(env) == tmp_path / "home" / ".cache" / "rtdd-bench"


def test_the_driver_root_matches_the_literal_run_arm_actually_uses() -> None:
    """`bench/swebench` is a separate uv project, so this module cannot import the
    driver to ask. It mirrors the driver's literal instead — and reads the driver's
    source to prove the literal is still that one."""
    src = (coldcache.BENCH / "swebench" / "run_arm.py").read_text(encoding="utf-8")
    assert 'CACHE_ROOT = Path.home() / ".cache" / "rtdd-bench"' in src


# --- cold and warm ----------------------------------------------------------------


def test_an_absent_store_is_cold_and_the_guard_does_not_create_it(tmp_path) -> None:
    absent = tmp_path / "never-made"
    state = coldcache.probe({"HOME": str(tmp_path / "home"), "RTDD_BENCH_CACHE": str(absent)})

    assert coldcache.findings(state) == []
    assert not absent.exists(), "the coldness guard must not create the store it reports on"
    assert not (tmp_path / "home" / ".cache").exists()


def test_an_empty_store_is_cold(tmp_path) -> None:
    (tmp_path / "cold").mkdir()
    state = coldcache.probe({"HOME": str(tmp_path / "home"), "RTDD_BENCH_CACHE": str(tmp_path / "cold")})
    assert coldcache.findings(state) == []


def test_a_store_with_entries_is_warm_and_the_finding_names_the_path_and_the_count(tmp_path) -> None:
    warm = tmp_path / "warm"
    _entry(warm)
    _entry(warm, "artifact/cd/feedface/suite.txt")
    state = coldcache.probe({"HOME": str(tmp_path / "home"), "RTDD_BENCH_CACHE": str(warm)})

    found = coldcache.findings(state)
    assert len(found) == 1
    assert str(warm) in found[0]
    assert "2" in found[0]


def test_a_warm_HOME_is_a_finding_even_when_the_variable_points_somewhere_cold(tmp_path) -> None:
    """The silent fallback, exactly. AC1 of #371: a run that set the variable and
    stopped there is still reading the developer's 201 MB store through `Path.home()`."""
    home = tmp_path / "home"
    _entry(home / ".cache" / "rtdd-bench", "mirrors/psf__requests/HEAD")
    (tmp_path / "cold").mkdir()

    state = coldcache.probe({"HOME": str(home), "RTDD_BENCH_CACHE": str(tmp_path / "cold")})
    found = coldcache.findings(state)

    assert len(found) == 1
    assert str(home / ".cache" / "rtdd-bench") in found[0]


def test_both_stores_warm_reports_both(tmp_path) -> None:
    home = tmp_path / "home"
    _entry(home / ".cache" / "rtdd-bench")
    _entry(tmp_path / "warm")
    state = coldcache.probe({"HOME": str(home), "RTDD_BENCH_CACHE": str(tmp_path / "warm")})
    assert len(coldcache.findings(state)) == 2


def test_the_state_records_the_variable_as_unset_rather_than_empty(tmp_path) -> None:
    """`docs/` and the CI evidence both want the word "unset"; `""` and "not set at all"
    are different states and only one of them is what a bare runner has."""
    state = coldcache.probe({"HOME": str(tmp_path / "home")})
    assert state.env == "unset"
    assert coldcache.probe({"HOME": str(tmp_path / "h"), "RTDD_BENCH_CACHE": ""}).env == "unset"


# --- the entry point the shell gate calls -----------------------------------------


def test_main_exits_zero_and_names_every_store_it_checked(tmp_path, capsys) -> None:
    code = coldcache.main(
        ["--home", str(tmp_path / "home"), "--replay-cache", str(tmp_path / "cold")]
    )
    out = capsys.readouterr().out

    assert code == 0
    assert str(tmp_path / "cold") in out
    assert str(tmp_path / "home" / ".cache" / "rtdd-bench") in out
    assert "cold" in out


def test_main_exits_nonzero_on_a_warm_store_and_says_which(tmp_path, capsys) -> None:
    _entry(tmp_path / "warm")
    code = coldcache.main(
        ["--home", str(tmp_path / "home"), "--replay-cache", str(tmp_path / "warm")]
    )
    captured = capsys.readouterr()

    assert code == 1
    assert str(tmp_path / "warm") in captured.out + captured.err
