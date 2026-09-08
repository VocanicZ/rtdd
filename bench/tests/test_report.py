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

import pytest

from replay.config import RunConfig
from replay.hardware import Hardware
from replay.records import CommitRecord, StrategyRecord, UncoveredRecord, WallClockRecord
from replay.replay import ReplayOutput
from replay.report import (
    COMPARISON_ARMS,
    COMPARISON_WALLCLOCK_COLUMN,
    FAILURE_WORDING,
    NOT_MEASURED,
    ReportError,
    assert_distribution_beside_mean,
    build_summary,
    comparison_table,
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


def test_wallclock_suppressed_when_operator_withholds_it():
    """`--no-wallclock` on a contended box is an operator choice, not a CI refusal —
    the report must say so in words, not just print an empty table."""
    md = render_markdown(
        build_summary(_out(), ("rtdd", "path"), HW, wallclock_enabled=False), CFG, HW
    )
    section = md.split("## Wall-clock", 1)[1].split("## By variant", 1)[0]
    assert "Suppressed" in section
    assert "GITHUB_ACTIONS" not in section
    assert "full uninstrumented" not in section
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


# --- the wall-clock distribution (#214) ----------------------------------


def _spread_out():
    """A bimodal wall-clock population, which is what the real ones are.

    Six cycles: four that selected nothing and cost ~0 ms, one mid, one that
    selected the hub and cost nearly a full run. The mean of that is a number no
    cycle produced, which is the whole reason this issue exists.
    """
    o = _out()
    o.wallclocks = [
        WallClockRecord(
            "synth", f"c{i}", "natural", "rtdd", HW.fingerprint(), 4000, s * 2, s, False
        )
        for i, s in enumerate((0, 0, 0, 0, 600, 3000))
    ]
    return o


def test_wallclock_rows_carry_the_distribution_beside_every_mean():
    wc = build_summary(_spread_out(), ("rtdd", "path"), HW)["wallclock"]
    row = wc["rows"]["rtdd"]
    assert row["n"] == 6
    # nearest-rank over (0, 0, 0, 0, 600, 3000): p50 is the 3rd sample, p90 the 6th.
    assert row["mean_subset_uninstrumented_ms"] == 600
    assert row["p50_subset_uninstrumented_ms"] == 0
    assert row["p90_subset_uninstrumented_ms"] == 3000
    assert row["worst_subset_uninstrumented_ms"] == 3000
    assert row["p50_subset_instrumented_ms"] == 0
    assert row["worst_subset_instrumented_ms"] == 6000
    assert row["p50_full_uninstrumented_ms"] == 4000
    assert row["worst_full_uninstrumented_ms"] == 4000


def test_every_published_mean_has_a_p50_p90_and_worst_beside_it():
    """The criterion (PRD #6, AC 9) is per column, not per table."""
    row = build_summary(_spread_out(), ("rtdd", "path"), HW)["wallclock"]["rows"]["rtdd"]
    for column in (
        "full_uninstrumented_ms",
        "subset_instrumented_ms",
        "subset_uninstrumented_ms",
    ):
        assert f"mean_{column}" in row
        for stat in ("p50", "p90", "worst"):
            assert f"{stat}_{column}" in row, f"{column} publishes a mean with no {stat}"


def test_the_wallclock_markdown_table_publishes_the_distribution():
    md = render_markdown(build_summary(_spread_out(), ("rtdd", "path"), HW), CFG, HW)
    section = md.split("## Wall-clock", 1)[1].split("## By variant", 1)[0]
    header = next(line for line in section.splitlines() if line.startswith("| strategy"))
    for cell in ("mean", "p50", "p90", "worst"):
        assert cell in header, f"the wall-clock table has no {cell} column"
    assert "| 3000 ms |" in section, "the worst cycle is never rounded away into the mean"


def test_a_wallclock_table_that_cites_only_a_mean_is_refused():
    """The guard is a text check on the rendered document, like the Axis 1 one:
    it sees what a reader sees, so it survives whatever the renderer grows into."""
    doc = (
        "## Wall-clock\n\n"
        "| strategy | n | mean full uninstrumented |\n"
        "|---|---|---|\n"
        "| rtdd | 24 | 1283 ms |\n"
    )
    with pytest.raises(ReportError) as exc:
        assert_distribution_beside_mean(doc)
    assert "p50" in str(exc.value)


def test_a_wallclock_table_with_no_mean_at_all_is_still_refused():
    """A bare column of averages that never says the word `mean` is the exact
    table this issue found in `bench/results/*/summary.md`."""
    doc = (
        "## Wall-clock\n\n"
        "| strategy | n | full uninstrumented |\n"
        "|---|---|---|\n"
        "| rtdd | 24 | 1283 ms |\n"
    )
    with pytest.raises(ReportError):
        assert_distribution_beside_mean(doc)


def test_a_mean_outside_the_wallclock_section_needs_its_distribution_too():
    doc = "| strategy | mean latency |\n|---|---|\n| rtdd | 12 ms |\n"
    with pytest.raises(ReportError):
        assert_distribution_beside_mean(doc)


def test_the_distribution_guard_accepts_the_real_rendered_report():
    md = render_markdown(build_summary(_spread_out(), ("rtdd", "path"), HW), CFG, HW)
    assert_distribution_beside_mean(md)


def test_the_suppressed_wallclock_section_has_no_table_to_guard():
    """A refusal is not a mean, and must not be turned into one to pass the guard."""
    md = render_markdown(
        build_summary(_spread_out(), ("rtdd", "path"), HW, wallclock_enabled=False), CFG, HW
    )
    assert_distribution_beside_mean(md)


def test_render_markdown_refuses_to_return_a_document_the_guard_rejects(monkeypatch):
    """`render_markdown` runs the guard on its own output — a future edit that
    drops the distribution columns fails at render time, not at review time."""
    import replay.report as report_mod

    monkeypatch.setattr(
        report_mod,
        "_wallclock_header",
        lambda: ["| strategy | measurement | n | mean |", "|---|---|---|---|"],
    )
    with pytest.raises(ReportError):
        render_markdown(build_summary(_spread_out(), ("rtdd", "path"), HW), CFG, HW)


# --- §7: the static arm against the measured arms (issue #342) -------------------
#
# The renderer half only. Every test below is driven from a synthetic summary dict or
# a synthetic `ReplayOutput` — no benchmark runs, no `~/.cache/rtdd-bench` access, and
# `bench/results/` is not regenerated here.


def _ratio(value, num=0, den=0):
    return {"num": num, "den": den, "value": value}


def _arm(recall, ratio, duration):
    return {
        "change_level_recall": _ratio(recall),
        "selection_ratio": _ratio(ratio),
        "selected_duration_fraction": _ratio(duration),
    }


def _wc_row(n, mean, p50, p90, worst):
    col = COMPARISON_WALLCLOCK_COLUMN
    return {
        "n": n,
        f"mean_{col}": mean,
        f"p50_{col}": p50,
        f"p90_{col}": p90,
        f"worst_{col}": worst,
    }


def _synthetic_summary(*, arms=None, wallclock_rows=None, suppressed=False):
    """A `summary.json`-shaped dict, hand-built rather than measured."""
    arms = (
        arms
        if arms is not None
        else {
            "static": _arm(0.333, 0.100, 0.120),
            "rtdd": _arm(1.000, 0.050, 0.060),
            "path": _arm(0.333, 0.200, 0.220),
            "full": _arm(1.000, 1.000, 1.000),
        }
    )
    return {
        "repo_id": "synth",
        "primary_variant": "natural",
        "strategies": arms,
        "by_variant": {"natural": arms},
        "wallclock": {
            "suppressed": suppressed,
            "reason": "withheld" if suppressed else "",
            "rows": (
                {"rtdd": _wc_row(2, 410, 400, 420, 420)}
                if wallclock_rows is None
                else wallclock_rows
            ),
        },
    }


def _comparison_row(text, arm):
    line = next(ln for ln in text.splitlines() if ln.startswith(f"| `{arm}` |"))
    return [c.strip() for c in line.strip().strip("|").split("|")]


def _static_out():
    """`_out()` plus a derived `static` arm that has no `WallClockRecord` of its own."""
    o = _out()
    for commit, selected in (("c1", ("a",)), ("c2", ("a",)), ("c3", ())):
        o.strategies.append(
            StrategyRecord("synth", commit, "natural", "static", selected, False, "test_for", 0)
        )
        o.strategies.append(
            StrategyRecord("synth", commit, "natural", "full", ALL, False, "everything", 0)
        )
    return o


def test_the_comparison_table_scores_static_against_rtdd_path_and_full():
    """PRD #233 AC3: the §7 evidence table is `static` beside the three arms it has to
    be read against, not a lone row a reader has to diff by eye."""
    text = "\n".join(comparison_table(_synthetic_summary()))
    for arm in COMPARISON_ARMS:
        assert f"| `{arm}` |" in text, arm


def test_the_comparison_table_publishes_recall_selection_ratio_and_selected_duration():
    header = comparison_table(_synthetic_summary())[0].lower()
    for column in ("change recall", "selection ratio", "selected duration"):
        assert column in header, column


def test_an_arm_with_no_wallclock_record_renders_an_explicit_not_measured_cell():
    """The `static` arm is derived offline from committed records, so it executed
    nothing. The absence is stated: never a blank that reads as zero, and never a
    figure borrowed from an arm that really ran."""
    text = "\n".join(comparison_table(_synthetic_summary()))
    static, rtdd = _comparison_row(text, "static"), _comparison_row(text, "rtdd")
    assert len(static) == len(rtdd)
    for cell in static[-4:]:
        assert cell == NOT_MEASURED
        assert cell != ""
    assert "410 ms" in rtdd, "the measured arm must still publish its own mean"
    for cell in static:
        assert "410 ms" not in cell, "a derived arm must not borrow another arm's timing"


def test_the_comparison_table_names_the_arm_whose_wallclock_is_not_measured():
    text = "\n".join(comparison_table(_synthetic_summary()))
    assert "executed nothing" in text
    assert "`static`" in text


def test_the_distribution_guard_covers_the_comparison_table():
    """AC4: any wall-clock figure in a new table carries `p50`, `p90` and `worst`."""
    assert_distribution_beside_mean("\n".join(comparison_table(_synthetic_summary())))


def test_a_comparison_table_carrying_a_bare_mean_is_refused():
    doc = (
        "| arm | change recall | selection ratio | selected duration "
        "| mean subset uninstrumented |\n"
        "|---|---|---|---|---|\n"
        "| `static` | 0.333 | 0.100 | 0.120 | not measured |\n"
        "| `rtdd` | 1.000 | 0.050 | 0.060 | 410 ms |\n"
    )
    with pytest.raises(ReportError) as exc:
        assert_distribution_beside_mean(doc)
    assert "p50" in str(exc.value)


def test_the_guard_is_not_satisfiable_by_omitting_the_wallclock_column():
    """Dropping the column is the other way to hide a spread, and it is the one a
    derived arm invites: no timing to print, so print no column. The guard refuses a
    §7 comparison that publishes no wall-clock at all."""
    doc = (
        "| arm | change recall | selection ratio | selected duration |\n"
        "|---|---|---|---|\n"
        "| `static` | 0.333 | 0.100 | 0.120 |\n"
        "| `rtdd` | 1.000 | 0.050 | 0.060 |\n"
    )
    with pytest.raises(ReportError) as exc:
        assert_distribution_beside_mean(doc)
    assert "mean" in str(exc.value)


def test_a_suppressed_wallclock_run_still_publishes_the_comparison_columns():
    """`--no-wallclock` and a CI runner both leave every arm unmeasured — that is
    still a not-measured cell, not a missing column."""
    lines = comparison_table(_synthetic_summary(wallclock_rows={}, suppressed=True))
    text = "\n".join(lines)
    assert_distribution_beside_mean(text)
    assert _comparison_row(text, "rtdd")[-4:] == [NOT_MEASURED] * 4


def test_the_comparison_table_is_rendered_into_the_published_markdown():
    summary = build_summary(_static_out(), ("rtdd", "path", "full", "static"), HW)
    md = render_markdown(summary, CFG, HW)
    section = md.split("## The static arm", 1)[1].split("## Stratified", 1)[0]
    for arm in COMPARISON_ARMS:
        assert f"| `{arm}` |" in section, arm
    assert NOT_MEASURED in section
    assert_distribution_beside_mean(md)


def test_no_comparison_section_where_the_run_carries_no_static_arm():
    """A "static versus" table without the static arm compares nothing. Existing
    summaries are unchanged until the arm is backfilled."""
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), HW), CFG, HW)
    assert "## The static arm" not in md


# --- the static arm's own verdict and the model it publishes (#366) -------
#
# Two documentation-accuracy defects, both of the same shape: an artifact asserts
# something it does not do. `derive.STATIC_TEST_FOR` documents itself as a PUBLISHED
# input that `summary.md` prints, and `summary.md` printed nothing; the plan's Task 6
# promised the pre-registered static verdict "stated in prose, win or lose", and the
# only place it was ever stated is `README.md`. A reader of one repo's `summary.md`
# saw `static` and `path` tie in the by-variant table and was told nothing about it.


def _static_verdict_lines(text: str) -> list[str]:
    return [ln for ln in text.splitlines() if ln.startswith("static verdict")]


def test_the_static_arm_section_discloses_every_template_it_models_with():
    """`STATIC_TEST_FOR` determines the published number — `static`'s selection ratio,
    its selected-duration fraction and therefore the kill-condition verdict all move if
    the templates change. So it is a published input, and the section that prints the
    number prints it beside them rather than leaving it in a source file."""
    from replay.derive import STATIC_TEST_FOR

    summary = build_summary(_static_out(), ("rtdd", "path", "full", "static"), HW)
    md = render_markdown(summary, CFG, HW)
    section = md.split("## The static arm", 1)[1].split("## Stratified", 1)[0]
    for tmpl in STATIC_TEST_FOR:
        assert tmpl in section, tmpl


def test_the_static_arm_section_names_the_level_2_source_and_both_construction_facts():
    """A reader comparing the row to `rtdd` has to be told that level 2 is the committed
    `importgraph` selection (so `static ⊇ importgraph` is arithmetic, not a finding) and
    that the adapter modelled here does not ship."""
    from replay.derive import LEVEL2_SOURCE

    summary = build_summary(_static_out(), ("rtdd", "path", "full", "static"), HW)
    md = render_markdown(summary, CFG, HW)
    section = md.split("## The static arm", 1)[1].split("## Stratified", 1)[0]
    for needle in (LEVEL2_SOURCE, "by construction", "does not ship"):
        assert needle in section, needle


def test_the_static_verdict_fires_the_kill_condition_on_a_tie():
    """Spec §7: "if the static tier does not beat the `path` baseline it is not worth
    shipping as a distinct tier". A tie is not a beat — and a tie is the branch the
    published corpus actually produces (flask/probe: 0.333 vs 0.333), so it is the
    branch that has to be right."""
    from replay.report import STATIC_FAILURE_WORDING, static_verdict_line

    tied = _arm(0.333, 0.012, 0.010)
    s = _synthetic_summary(arms={"static": tied, "path": dict(tied)})
    assert STATIC_FAILURE_WORDING in static_verdict_line(s)


def test_the_static_verdict_reports_a_win_when_there_is_one():
    from replay.report import STATIC_SUCCESS_WORDING, static_verdict_line

    s = _synthetic_summary(
        arms={"static": _arm(0.800, 0.100, 0.100), "path": _arm(0.400, 0.100, 0.120)}
    )
    line = static_verdict_line(s)
    assert STATIC_SUCCESS_WORDING in line


def test_static_recall_bought_with_more_time_is_not_a_win():
    """"at comparable or better selected-duration fraction" is half the criterion. An arm
    that buys recall by selecting more of the suite is on its way to being `full`."""
    from replay.report import STATIC_FAILURE_WORDING, STATIC_SUCCESS_WORDING, static_verdict_line

    s = _synthetic_summary(
        arms={"static": _arm(0.800, 0.900, 0.900), "path": _arm(0.400, 0.100, 0.100)}
    )
    line = static_verdict_line(s)
    assert STATIC_FAILURE_WORDING in line
    assert STATIC_SUCCESS_WORDING not in line


def test_a_static_population_with_no_ground_truth_is_not_computable_not_a_loss():
    """Neither published repo's `natural` population has a detecting commit. Scoring that
    as a failure would publish a verdict about a comparison nothing was measured for."""
    from replay.report import STATIC_FAILURE_WORDING, static_verdict_line

    s = _synthetic_summary(
        arms={"static": _arm(None, 0.05, 0.05), "path": _arm(None, 0.05, 0.05)}
    )
    assert "not computable" in static_verdict_line(s)
    assert STATIC_FAILURE_WORDING not in static_verdict_line(s)


def test_the_static_verdict_refuses_to_compute_without_both_arms():
    from replay.report import static_verdict_line

    s = _synthetic_summary(arms={"rtdd": _arm(1.0, 0.05, 0.06)})
    assert static_verdict_line(s).startswith("static verdict: not computable")


def test_the_static_secondary_verdict_labels_probe_as_an_upper_bound():
    """`probe` is the only population with ground truth in the published corpus, so it is
    the population the verdict actually turns on — and it is still an upper bound, so it
    is labelled as one on its own line rather than promoted to the primary verdict."""
    from replay.report import static_secondary_verdict_lines

    arms = {"static": _arm(0.333, 0.012, 0.010), "path": _arm(0.333, 0.012, 0.010)}
    s = _synthetic_summary(arms=arms)
    s["by_variant"] = {"natural": arms, "probe": arms}
    lines = static_secondary_verdict_lines(s)
    assert len(lines) == 1
    assert lines[0].startswith("static verdict (probe")
    assert "upper bound" in lines[0]


def test_the_markdown_states_the_static_verdict_beside_the_comparison_table():
    """The per-repo artifact states its own result instead of relying on the reader
    reaching `README.md`."""
    summary = build_summary(_static_out(), ("rtdd", "path", "full", "static"), HW)
    md = render_markdown(summary, CFG, HW)
    section = md.split("## The static arm", 1)[1].split("## Stratified", 1)[0]
    assert _static_verdict_lines(section), "the static arm section states no verdict"


def test_a_run_with_no_static_arm_states_no_static_verdict():
    md = render_markdown(build_summary(_out(), ("rtdd", "path"), HW), CFG, HW)
    assert _static_verdict_lines(md) == []


def test_the_rtdd_verdict_is_never_mistaken_for_the_static_one():
    """Both verdicts share one body, so `^verdict: ` must still select exactly the
    rtdd-vs-path line and never the static one."""
    summary = build_summary(_static_out(), ("rtdd", "path", "full", "static"), HW)
    md = render_markdown(summary, CFG, HW)
    primary = [ln for ln in md.splitlines() if ln.startswith("verdict:")]
    assert len(primary) == 1
    assert "rtdd=" in primary[0]
