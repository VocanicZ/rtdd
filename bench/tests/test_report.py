"""The emission layer: what a reviewer actually reads, and what a re-run must not churn.

Three properties are load-bearing and each has its own test here:

* **Byte stability.** `commits.jsonl` and `summary.json` are committed, so a re-run
  at an identical config must produce identical bytes — otherwise every benchmark
  run lands a diff nobody can read and the real movements hide in the noise.
* **The verdict line is unconditional.** It is printed whether or not RTDD wins,
  with the pre-registered failure wording used verbatim when it does not.
* **The two variants never share a table.** `probe` seeds the map at the child
  commit, so its numbers are an upper bound for map-based strategies, and the label
  sits on the same line as the numbers rather than in a paragraph above them.
"""

from __future__ import annotations

import json
import subprocess

from replay.config import RunConfig
from replay.hardware import Hardware
from replay.records import CommitRecord, StrategyRecord, UncoveredRecord, WallClockRecord
from replay.replay import ReplayOutput
from replay.report import (
    FAILURE_WORDING,
    build_summary,
    fmt,
    render_aggregate,
    render_markdown,
    verdict_line,
    write_aggregate,
    write_results,
)
from replay.session import DriftCurve, DriftPoint

HW = Hardware("Ryzen 9 7950X", 32, 65536000, "Linux-6.8", "3.12.4", None)
CI_HW = Hardware("Ryzen 9 7950X", 32, 65536000, "Linux-6.8", "3.12.4", "GITHUB_ACTIONS")
CFG = RunConfig(
    "cd" * 32,
    "rtdd 1.2.3 (a3f21e0)",
    (("pytest", "8.3.3"),),
    ("rtdd", "path"),
    ("natural",),
    3,
    0,
    1,
)

ALL = ("a", "b", "c", "d")
DUR = {"a": 100, "b": 100, "c": 100, "d": 700}


def _out(variants=("natural",)):
    o = ReplayOutput()
    for variant in variants:
        o.commits += [
            CommitRecord("synth", "c1", "c0", variant, ALL, DUR, ("a",), (), ("src/a.py",)),
            CommitRecord("synth", "c2", "c1", variant, ALL, DUR, ("a", "b"), (), ("src/b.py",)),
            CommitRecord("synth", "c3", "c2", variant, ALL, DUR, (), (), ("src/c.py",)),
        ]
        o.strategies += [
            StrategyRecord("synth", "c1", variant, "rtdd", ("a",), False, "T0", 2),
            StrategyRecord("synth", "c2", variant, "rtdd", ("a", "b"), False, "T0", 2),
            StrategyRecord("synth", "c3", variant, "rtdd", ("c",), False, "T0", 2),
            StrategyRecord("synth", "c1", variant, "path", ("a",), False, "sib", 1),
            StrategyRecord("synth", "c2", variant, "path", (), False, "none", 1),
            StrategyRecord("synth", "c3", variant, "path", ("c",), False, "sib", 1),
        ]
        o.uncovered += [
            UncoveredRecord("synth", "c1", variant, (("src/a.py", 3),), 1, 1),
            UncoveredRecord("synth", "c2", variant, (), 0, 0),
            UncoveredRecord("synth", "c3", variant, (("src/c.py", 9),), 0, 1),
        ]
    o.wallclocks = [
        WallClockRecord("synth", "c1", "natural", "rtdd", HW.fingerprint(), 4000, 900, 420, False),
        WallClockRecord("synth", "c2", "natural", "rtdd", HW.fingerprint(), 4000, 880, 400, True),
    ]
    return o


def _cheap_path_out():
    """RTDD wins on recall *and* costs no more duration than the path heuristic."""
    o = ReplayOutput()
    o.commits = [
        CommitRecord("synth", "c1", "c0", "natural", ALL, DUR, ("a",), (), ("src/a.py",)),
        CommitRecord("synth", "c2", "c1", "natural", ALL, DUR, ("b",), (), ("src/b.py",)),
    ]
    o.strategies = [
        StrategyRecord("synth", "c1", "natural", "rtdd", ("a",), False, "T0", 2),
        StrategyRecord("synth", "c2", "natural", "rtdd", ("b",), False, "T0", 2),
        StrategyRecord("synth", "c1", "natural", "path", ("d",), False, "sib", 1),
        StrategyRecord("synth", "c2", "natural", "path", ("d",), False, "sib", 1),
    ]
    return o


def test_fmt():
    assert fmt({"num": 14, "den": 15, "value": 14 / 15}) == "0.933 (14/15)"
    assert fmt({"num": 0, "den": 0, "value": None}) == "n/a (0/0)"
    assert fmt(None) == "n/a"


def test_summary_has_a_row_per_strategy_and_breaks_out_the_single_killer_stratum():
    s = build_summary(_out(), ("rtdd", "path"), HW)
    assert set(s["strategies"]) == {"rtdd", "path"}
    assert s["strategies"]["rtdd"]["strata"]["1"]["change_level_recall"]["value"] == 1.0
    assert s["strategies"]["path"]["strata"]["2-5"]["change_level_recall"]["value"] == 0.0
    assert s["false_signal"]["fired"] == 2
    assert s["false_signal"]["change_false_signal_rate"]["num"] == 1


def test_verdict_line_states_the_pre_registered_criterion():
    s = build_summary(_out(), ("rtdd", "path"), HW)
    line = verdict_line(s)
    assert line.startswith("verdict:")
    assert "path heuristic" in line
    assert "rtdd" in line


def test_verdict_uses_the_failure_wording_verbatim_when_rtdd_costs_more_duration():
    # rtdd has strictly better change-level recall here, but selects more
    # milliseconds than the path heuristic, so the criterion is not met.
    s = build_summary(_out(), ("rtdd", "path"), HW)
    assert s["strategies"]["rtdd"]["change_level_recall"]["value"] == 1.0
    assert s["strategies"]["path"]["change_level_recall"]["value"] == 0.5
    assert FAILURE_WORDING in verdict_line(s)


def test_verdict_says_rtdd_wins_only_when_it_is_no_more_expensive():
    s = build_summary(_cheap_path_out(), ("rtdd", "path"), HW)
    line = verdict_line(s)
    assert "rtdd beats the naive path heuristic" in line
    assert FAILURE_WORDING not in line


def test_verdict_refuses_to_compute_without_both_arms():
    s = build_summary(_out(), ("rtdd",), HW)
    assert verdict_line(s).startswith("verdict: not computable")


def test_every_summary_markdown_carries_a_verdict_line():
    for output, ids in ((_out(), ("rtdd", "path")), (_out(), ("rtdd",))):
        md = render_markdown(build_summary(output, ids, HW), CFG, HW)
        assert [ln for ln in md.splitlines() if ln.startswith("verdict:")]


def test_markdown_embeds_the_config_and_the_hardware():
    s = build_summary(_out(), ("rtdd", "path"), HW)
    md = render_markdown(s, CFG, HW)
    assert "Ryzen 9 7950X" in md
    assert "rtdd 1.2.3 (a3f21e0)" in md
    assert "| rtdd |" in md
    assert "|F_full| == 1" in md


def test_markdown_publishes_every_metric_the_prd_names():
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), HW), CFG, HW)
    for heading in (
        "change recall",
        "test recall (micro)",
        "test recall (macro)",
        "selection ratio",
        "selected duration",
        "escalation",
        "false-signal rate",
        "isolation violations",
        "full uninstrumented",
        "subset instrumented",
        "subset uninstrumented",
    ):
        assert heading in md, heading


def test_the_wallclock_table_discloses_which_column_ran_in_parallel():
    """A reader comparing the `xdist` row to the `full` row has to be told which
    invocation produced each number, or a parallel row and a serial one look the
    same kind of measurement."""
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), HW), CFG, HW)
    section = md.split("## Wall-clock", 1)[1]
    assert "exec_args" in section
    assert "-n auto" in section


def test_isolation_violations_survive_the_ci_wallclock_refusal():
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), CI_HW), CFG, CI_HW)
    assert "Suppressed" in md
    assert "GITHUB_ACTIONS" in md
    assert "full uninstrumented" not in md.split("## Wall-clock", 1)[1]
    assert "isolation violations" in md


def test_variants_get_separate_tables_and_probe_is_labelled_on_the_number_line():
    s = build_summary(_out(("natural", "probe")), ("rtdd", "path"), HW)
    md = render_markdown(s, CFG, HW)
    assert "### `natural`" in md
    assert "### `probe`" in md
    probe = md.split("### `probe`", 1)[1]
    rtdd_row = [ln for ln in probe.splitlines() if ln.startswith("| rtdd |")]
    path_row = [ln for ln in probe.splitlines() if ln.startswith("| path |")]
    assert rtdd_row and "upper bound" in rtdd_row[0]
    assert path_row and "upper bound" not in path_row[0]
    assert "upper bound" not in md.split("### `natural`", 1)[1].split("### `probe`", 1)[0]


def test_per_repo_markdown_never_names_a_second_repo():
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), HW), CFG, HW)
    assert "aggregate" not in md.lower()


def test_aggregate_is_duration_weighted_and_says_so():
    a = build_summary(_out(), ("rtdd", "path"), HW)
    b = build_summary(_out(), ("rtdd", "path"), HW)
    md = render_aggregate([a, b])
    assert "duration-weighted" in md
    assert "never pooled" in md
    assert "| rtdd |" in md


def test_write_results_is_byte_stable_across_reruns(tmp_path):
    o = _out()
    d = tmp_path / "synth"
    write_results(d, o, CFG, HW, ("rtdd", "path"))
    first = (d / "commits.jsonl").read_bytes()
    first_summary = (d / "summary.json").read_bytes()
    write_results(d, o, CFG, HW, ("rtdd", "path"))
    assert (d / "commits.jsonl").read_bytes() == first
    assert (d / "summary.json").read_bytes() == first_summary
    assert json.loads((d / "config.json").read_text())["hardware"]["cpu_model"] == "Ryzen 9 7950X"
    assert (d / "summary.md").read_text().startswith("# ")


def test_commits_jsonl_is_one_sorted_json_line_per_record(tmp_path):
    o = _out()
    d = tmp_path / "synth"
    write_results(d, o, CFG, HW, ("rtdd", "path"))
    lines = (d / "commits.jsonl").read_text(encoding="utf-8").splitlines()
    assert len(lines) == len(o.commits) + len(o.strategies) + len(o.wallclocks) + len(o.uncovered)
    keys = [
        (d_["repo_id"], d_["commit"], d_["variant"], d_["kind"], d_.get("strategy", ""))
        for d_ in (json.loads(ln) for ln in lines)
    ]
    assert keys == sorted(keys)


def test_summary_json_has_sorted_keys_and_a_trailing_newline(tmp_path):
    d = tmp_path / "synth"
    write_results(d, _out(), CFG, HW, ("rtdd", "path"))
    raw = (d / "summary.json").read_text(encoding="utf-8")
    assert raw.endswith("\n")
    assert raw == json.dumps(json.loads(raw), sort_keys=True, separators=(",", ":")) + "\n"


def test_config_json_carries_everything_needed_to_reproduce_the_numbers(tmp_path):
    d = tmp_path / "synth"
    write_results(d, _out(), CFG, HW, ("rtdd", "path"))
    cj = json.loads((d / "config.json").read_text())
    assert cj["config"]["corpus_digest"] == CFG.corpus_digest
    assert cj["config"]["tool_versions"] == {"pytest": "8.3.3"}
    assert cj["config"]["strategies"] == ["rtdd", "path"]
    # The binary's own identity — `rtdd --version` prints its git SHA.
    assert cj["config"]["rtdd_version"] == "rtdd 1.2.3 (a3f21e0)"
    assert cj["hardware"]["fingerprint"] == HW.fingerprint()
    assert cj["config_digest"] == CFG.digest()


def test_no_result_file_is_written_without_its_config(tmp_path):
    d = tmp_path / "synth"
    write_results(d, _out(), CFG, HW, ("rtdd", "path"))
    assert {p.name for p in d.iterdir()} == {
        "commits.jsonl",
        "summary.json",
        "summary.md",
        "config.json",
    }


def test_drift_is_written_only_when_a_curve_was_measured(tmp_path):
    d = tmp_path / "synth"
    curve = DriftCurve("synth", "c0", (DriftPoint(1, 1, 2, 4, "T0"),))
    write_results(d, _out(), CFG, HW, ("rtdd", "path"), drift=curve)
    assert json.loads((d / "drift.json").read_text())["points"][0]["cycle"] == 1


def test_write_aggregate_is_the_only_cross_repo_file(tmp_path):
    s = build_summary(_out(), ("rtdd", "path"), HW)
    path = write_aggregate(tmp_path, [s])
    assert path.name == "aggregate.md"
    assert path.parent == tmp_path
    assert "duration-weighted" in path.read_text(encoding="utf-8")


def test_results_directory_is_committed_and_not_gitignored():
    proc = subprocess.run(
        ["git", "check-ignore", "-q", "bench/results"],
        cwd=str(__import__("pathlib").Path(__file__).resolve().parents[2]),
        capture_output=True,
    )
    assert proc.returncode == 1, "bench/results/ must be committed, not gitignored"


# --- the secondary verdict ------------------------------------------------


def _no_natural_detections():
    """A `natural` population that is entirely green, plus a detecting `probe`.

    This is the ordinary case on a real repo: commits are pushed green, so the
    natural detecting population can be empty while `probe` — the same commits'
    source halves reverted — detects.
    """
    o = ReplayOutput()
    o.commits = [
        CommitRecord("synth", "c1", "c0", "natural", ALL, DUR, (), (), ("src/a.py",)),
        CommitRecord("synth", "c1", "c0", "probe", ALL, DUR, ("a",), (), ("src/a.py",)),
    ]
    o.strategies = [
        StrategyRecord("synth", "c1", "natural", "rtdd", ("a",), False, "T0", 2),
        StrategyRecord("synth", "c1", "natural", "path", (), False, "none", 1),
        StrategyRecord("synth", "c1", "probe", "rtdd", ("a",), False, "T0", 2),
        StrategyRecord("synth", "c1", "probe", "path", (), False, "none", 1),
    ]
    return o


def test_a_green_natural_population_still_publishes_the_probe_comparison():
    """`verdict: not computable` must not be the last word when `probe` detected.

    The pre-registered criterion is measured on `natural`, and that stays true:
    the probe line is labelled an upper bound and never replaces the verdict. But
    a summary whose only comparison is "not computable" gates nothing, and the
    probe population is exactly what the plan added for this case.
    """
    from replay.report import secondary_verdict_lines

    s = build_summary(_no_natural_detections(), ("rtdd", "path"), HW)
    assert verdict_line(s).startswith("verdict: not computable")
    extra = secondary_verdict_lines(s)
    assert extra, "a detecting probe population must be reported"
    line = extra[0]
    assert line.startswith("verdict (probe")
    assert "upper bound" in line
    assert "rtdd=1.000" in line and "path heuristic=0.000" in line


def test_the_probe_verdict_is_absent_when_probe_was_not_run():
    from replay.report import secondary_verdict_lines

    s = build_summary(_out(), ("rtdd", "path"), HW)
    assert secondary_verdict_lines(s) == []


def test_the_markdown_carries_the_probe_verdict_beside_the_primary_one():
    md = render_markdown(build_summary(_no_natural_detections(), ("rtdd", "path"), HW), CFG, HW)
    assert [ln for ln in md.splitlines() if ln.startswith("verdict:")]
    assert [ln for ln in md.splitlines() if ln.startswith("verdict (probe")]


def test_the_summary_publishes_the_cycles_where_rtdd_run_refused():
    """A refusal is a number about the system under test, so it is in the table.

    `rtdd run` exits 2 rather than run a map that names a deleted test. Leaving
    that out of the summary would present the uncovered-signal rate as if it had
    been measured on every cycle.
    """
    o = _out()
    o.rtdd_run_errors = [
        {"repo_id": "synth", "commit": "c2", "variant": "natural", "reason": "rtdd-run-refused"}
    ]
    s = build_summary(o, ("rtdd", "path"), HW)
    assert s["rtdd_run_errors"] == 1
    md = render_markdown(s, CFG, HW)
    assert "`rtdd run` refused on 1 of" in md
