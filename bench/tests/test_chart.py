"""The figures: what a reader sees before they read a table, and what may not be drawn.

A chart is a claim with the arithmetic hidden, so the properties tested here are the
ones that stop a picture from saying something the committed records do not.

* **Every mark is read from `summary.json`.** Each plotted element carries
  `data-strategy` / `data-metric` / `data-value`, and the test asserts the geometry
  is that value's position on the axis — a figure cannot carry a hand-typed number.
* **A missing measurement is never drawn as zero.** `change_level_recall` is `null`
  wherever a population detected nothing, and a scatter point at y=0 would read as
  "this strategy caught none of them" when what happened is that there was nothing
  to catch. Those strategies are named in the figure instead, never plotted.
* **The verdict travels with the figure.** The pre-registered rtdd-vs-path result is
  rendered into the safety chart from `report.verdict_line`, so a reader who looks
  only at the picture still sees that the criterion was not met.
* **Light and dark differ only in palette.** The two files carry byte-identical data
  marks, so a theme swap cannot also swap the numbers.
"""

from __future__ import annotations

import copy
import json
import pathlib
import re

import pytest

from replay import chart
from replay.report import FAILURE_WORDING

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]
RESULTS = REPO_ROOT / "bench" / "results"
FIGURE_DIR = REPO_ROOT / "docs" / "results" / "figures"

MARK = re.compile(
    r'data-strategy="(?P<strategy>[^"]+)"\s+data-metric="(?P<metric>[^"]+)"\s+data-value="(?P<value>[^"]+)"'
)


def marks(svg: str) -> dict[tuple[str, str], str]:
    return {(m["strategy"], m["metric"]): m["value"] for m in MARK.finditer(svg)}


@pytest.fixture
def flask() -> dict:
    return json.loads((RESULTS / "flask" / "summary.json").read_text())


@pytest.fixture
def httpie() -> dict:
    return json.loads((RESULTS / "httpie" / "summary.json").read_text())


# --- every mark comes from the summary -------------------------------------------


def test_savings_plots_every_strategy_the_summary_carries(flask, httpie):
    svg = chart.savings_svg([flask, httpie], chart.LIGHT)
    plotted = {s for s, _ in marks(svg)}
    assert plotted == set(flask["strategies"]) | set(httpie["strategies"])


def test_savings_publishes_both_the_test_count_and_the_duration(flask, httpie):
    svg = chart.savings_svg([flask, httpie], chart.LIGHT)
    got = marks(svg)
    assert got[("rtdd", "flask/selection_ratio")] == "0.235"
    assert got[("rtdd", "flask/selected_duration_fraction")] == "0.282"
    assert got[("rtdd", "httpie/selection_ratio")] == "0.186"
    assert got[("rtdd", "httpie/selected_duration_fraction")] == "0.188"


def test_savings_bar_length_is_the_committed_fraction_not_a_literal(flask, httpie):
    """Halve a fraction in the summary and the bar must halve with it."""
    doctored = copy.deepcopy(flask)
    doctored["strategies"]["rtdd"]["selected_duration_fraction"]["value"] = 0.5

    before = _bar_width(chart.savings_svg([flask, httpie], chart.LIGHT), "rtdd", "flask/selected_duration_fraction")
    after = _bar_width(chart.savings_svg([doctored, httpie], chart.LIGHT), "rtdd", "flask/selected_duration_fraction")

    assert after == pytest.approx(chart.PLOT_WIDTH * 0.5, abs=0.51)
    assert before == pytest.approx(chart.PLOT_WIDTH * 0.282, abs=0.51)


def test_the_full_suite_is_drawn_at_the_full_width(flask, httpie):
    svg = chart.savings_svg([flask, httpie], chart.LIGHT)
    assert _bar_width(svg, "full", "flask/selected_duration_fraction") == pytest.approx(
        chart.PLOT_WIDTH, abs=0.51
    )


def _bar_width(svg: str, strategy: str, metric: str) -> float:
    pattern = re.compile(
        r'<rect[^>]*\swidth="(?P<w>[0-9.]+)"[^>]*data-strategy="%s"\s+data-metric="%s"'
        % (re.escape(strategy), re.escape(metric))
    )
    m = pattern.search(svg)
    assert m is not None, f"no bar for {strategy}/{metric}"
    return float(m["w"])


# --- a missing measurement is never drawn as zero ---------------------------------


def test_safety_never_plots_a_null_recall_as_zero(flask):
    """`natural` detected nothing in any repo, so no point may sit on the floor."""
    svg = chart.safety_svg(flask, "natural", chart.LIGHT)
    assert marks(svg) == {}
    assert chart.NOT_COMPUTABLE in svg


def test_safety_names_the_strategies_it_could_not_plot(flask):
    svg = chart.safety_svg(flask, "natural", chart.LIGHT)
    for strategy in flask["by_variant"]["natural"]:
        assert strategy in svg


def test_safety_plots_the_population_that_does_have_ground_truth(flask):
    svg = chart.safety_svg(flask, "probe", chart.LIGHT)
    got = marks(svg)
    assert got[("rtdd", "change_level_recall")] == "1.000"
    assert got[("testmon", "change_level_recall")] == "1.000"
    assert got[("path", "change_level_recall")] == "0.333"
    assert got[("importgraph", "change_level_recall")] == "0.000"


def test_a_real_zero_recall_is_plotted_and_a_null_one_is_not(flask):
    """importgraph really did catch 0 of 3; static really has a denominator here."""
    svg = chart.safety_svg(flask, "probe", chart.LIGHT)
    plotted = {s for s, m in marks(svg) if m == "change_level_recall"}
    computable = {
        name
        for name, row in flask["by_variant"]["probe"].items()
        if row["change_level_recall"]["value"] is not None
    }
    assert plotted == computable


def test_safety_marks_the_upper_bound_when_the_variant_is_probe(flask):
    assert "upper bound" in chart.safety_svg(flask, "probe", chart.LIGHT)
    assert "upper bound" not in chart.safety_svg(flask, "natural", chart.LIGHT)


# --- the verdict travels with the figure ------------------------------------------


def test_safety_figure_carries_the_pre_registered_verdict(flask):
    """A reader who only looks at the picture still learns the criterion failed."""
    svg = chart.safety_svg(flask, "probe", chart.LIGHT)
    assert FAILURE_WORDING.split(" — ")[0] in svg or "does NOT clearly beat" in svg


def test_safety_verdict_is_derived_not_typed(flask):
    """Flip the comparison in the summary and the figure must stop saying it failed."""
    doctored = copy.deepcopy(flask)
    doctored["by_variant"]["probe"]["path"]["selected_duration_fraction"]["value"] = 0.9
    svg = chart.safety_svg(doctored, "probe", chart.LIGHT)
    assert "does NOT clearly beat" not in svg


# --- wall-clock -------------------------------------------------------------------


def test_wallclock_publishes_the_distribution_never_a_bare_mean(flask, httpie):
    svg = chart.wallclock_svg([flask, httpie], chart.LIGHT)
    got = marks(svg)
    assert got[("rtdd", "flask/p50_subset_uninstrumented_ms")] == "0"
    assert got[("rtdd", "flask/p90_subset_uninstrumented_ms")] == "3109"
    assert got[("rtdd", "flask/worst_subset_uninstrumented_ms")] == "4045"
    assert ("rtdd", "flask/mean_subset_uninstrumented_ms") not in got


def test_wallclock_draws_the_full_suite_reference(flask, httpie):
    svg = chart.wallclock_svg([flask, httpie], chart.LIGHT)
    assert marks(svg)[("full", "flask/reference_full_uninstrumented_ms")] == "3127"


def test_wallclock_omits_a_strategy_with_no_recorded_run(flask, httpie):
    """`static` executed nothing, so it has no wall-clock row and must not get a bar."""
    svg = chart.wallclock_svg([flask, httpie], chart.LIGHT)
    assert ("static", "flask/p50_subset_uninstrumented_ms") not in marks(svg)
    assert chart.NOT_MEASURED in svg


# --- palette and determinism ------------------------------------------------------


@pytest.mark.parametrize("figure", chart.FIGURES)
def test_light_and_dark_carry_identical_data(figure, flask, httpie):
    light = chart.render_figure(figure, [flask, httpie], chart.LIGHT)
    dark = chart.render_figure(figure, [flask, httpie], chart.DARK)
    assert marks(light) == marks(dark)


@pytest.mark.parametrize("figure", chart.FIGURES)
def test_render_is_byte_stable(figure, flask, httpie):
    once = chart.render_figure(figure, [flask, httpie], chart.LIGHT)
    twice = chart.render_figure(figure, [flask, httpie], chart.LIGHT)
    assert once == twice


@pytest.mark.parametrize("figure", chart.FIGURES)
def test_every_figure_declares_its_source(figure, flask, httpie):
    """Every figure names the committed file it was drawn from, and that file exists.

    Was a substring check for `bench/results`, which a figure drawn from a record kept
    anywhere else would fail for the wrong reason — and which a figure citing a path
    that had since been deleted or renamed would pass. Reading the cited paths off the
    source line and stat-ing them checks the thing the line is actually promising.
    """
    svg = chart.render_figure(figure, [flask, httpie], chart.LIGHT)
    line = next((l for l in svg.splitlines() if ">source:" in l), None)
    assert line is not None, f"{figure} declares no source line"
    cited = re.findall(r"[\w./&;-]*/[\w./&;-]*\.(?:jsonl|json|md)", line)
    assert cited, f"{figure} source line names no committed file: {line}"
    for rel in cited:
        # `bench/results/&lt;repo&gt;/summary.json` stands for one file per repo, so the
        # figure cannot name a single path. The directory above the placeholder is still
        # a real one, and checking it catches the rename this guard exists to catch.
        concrete = rel.split("&lt;")[0].rstrip("/") if "&lt;" in rel else rel
        assert (REPO_ROOT / concrete).exists(), (
            f"{figure} cites {rel}, but {concrete} does not exist"
        )


def test_dark_palette_actually_changes_the_ink(flask, httpie):
    light = chart.savings_svg([flask, httpie], chart.LIGHT)
    dark = chart.savings_svg([flask, httpie], chart.DARK)
    assert light != dark
    assert chart.DARK.bg in dark and chart.DARK.bg not in light


# --- the committed figures may not drift from the committed records ---------------


def test_committed_figures_match_the_committed_summaries(flask, httpie):
    """The guard `outcomes_test.go` is to the README, this is to the pictures."""
    expected = chart.render_all([flask, httpie])
    missing = [name for name in expected if not (FIGURE_DIR / name).exists()]
    assert not missing, f"figures not committed: {missing} — run `uv run python -m replay.cli chart`"
    stale = [
        name for name, svg in expected.items() if (FIGURE_DIR / name).read_text() != svg
    ]
    assert not stale, f"committed figures are stale: {stale} — run `uv run python -m replay.cli chart`"


def test_render_all_covers_both_palettes_of_every_figure():
    names = chart.render_all.__doc__ or ""
    assert names  # documented
    for figure in chart.FIGURES:
        for palette in (chart.LIGHT, chart.DARK):
            assert f"{figure}-{palette.name}.svg" in chart.figure_names()


# --- the head-to-head: RTDD against running everything, and nothing else -----------
#
# This figure answers the project's own question rather than the pre-registered one, and
# it is the figure a reader sees first. Its whole value is that there is no third bar to
# compare against: the moment a cheaper baseline appears beside RTDD and the full suite,
# the picture stops answering "is this better than running everything" and starts
# answering "is the map worth building", which the axis2 figures already answer in full.
# These tests are what stops that drift.


@pytest.fixture
def paired() -> dict:
    return json.loads((RESULTS / "paired" / "flask" / "summary.json").read_text())


@pytest.fixture
def session() -> dict:
    return json.loads((RESULTS / "agent-session" / "flask.json").read_text())


def test_head_to_head_draws_only_rtdd_and_the_full_suite(paired, session):
    svg = chart.head_to_head_svg(paired, session, chart.LIGHT)
    drawn = {strategy for strategy, _ in marks(svg)}
    assert drawn == {"rtdd", "full"}, (
        f"the head-to-head figure drew {sorted(drawn)}; it may only ever draw "
        f"{sorted(chart.HEAD_TO_HEAD)} — every other selector belongs in the axis2 figures"
    )


def test_head_to_head_never_plots_a_baseline_even_when_the_summary_carries_one(paired, session):
    # The summary really does carry testmon, path, lf, importgraph and random. A figure
    # that iterated `summary["strategies"]` would quietly pick them all up on the next
    # re-render, which is exactly the regression this asserts against.
    assert len(paired["strategies"]) > len(chart.HEAD_TO_HEAD)
    svg = chart.head_to_head_svg(paired, session, chart.LIGHT)
    for baseline in set(paired["strategies"]) - set(chart.HEAD_TO_HEAD):
        assert f'data-strategy="{baseline}"' not in svg


def test_head_to_head_bars_are_the_committed_values_not_literals(paired, session):
    svg = chart.head_to_head_svg(paired, session, chart.LIGHT)
    got = marks(svg)
    for name in chart.HEAD_TO_HEAD:
        row = paired["strategies"][name]
        assert got[(name, "change_level_recall")] == f"{row['change_level_recall']['value']:.3f}"
        assert got[(name, "selected_duration_fraction")] == (
            f"{row['selected_duration_fraction']['value']:.3f}"
        )
    for label, key in (("full suite", "full"), ("rtdd run", "always"), ("--record=auto", "auto")):
        strategy = "full" if key == "full" else "rtdd"
        assert got[(strategy, f"session_ms/{label}")] == f"{session['session_ms'][key]} ms"


def test_head_to_head_carries_its_own_limits(paired, session):
    # A figure this favourable travels without its table — screenshotted into an issue,
    # pasted into a slide — so the limits have to be inside the image.
    svg = chart.head_to_head_svg(paired, session, chart.LIGHT)
    assert "upper bound" in svg
    assert "Parity is the ceiling" in svg
    detecting = paired["strategies"]["full"]["change_level_recall"]["den"]
    assert f"{detecting} detecting commits" in svg


def test_head_to_head_refuses_a_summary_missing_an_arm(paired, session):
    stripped = copy.deepcopy(paired)
    del stripped["strategies"]["full"]
    with pytest.raises(chart.ChartError, match="full"):
        chart.head_to_head_svg(stripped, session, chart.LIGHT)


# --- the worked example: a diagram that cannot disagree with the tool ---------------
#
# This figure explains a mechanism rather than plotting a measurement, which is exactly
# the kind of picture that drifts: prose and diagrams describing selection are written
# from memory, and the tool changes underneath them. It is drawn from the map `rtdd seed`
# really built and the selection `rtdd which` really returned for the same change, so a
# selector that stopped behaving this way fails here instead of shipping a lie on the
# front page.


@pytest.fixture
def worked():
    return chart.load_worked_example(REPO_ROOT)


def test_the_diagram_draws_the_tests_the_map_actually_holds(worked):
    rows, selection = worked
    svg = chart.worked_example_svg(rows, selection, chart.LIGHT)
    for row in rows:
        short = row["t"].split("::", 1)[1]
        assert short in svg, f"{row['t']} is in the map but not in the figure"


def test_every_test_the_diagram_runs_is_one_the_selection_returned(worked):
    rows, selection = worked
    selected = set(selection["selection"]["tests"])
    assert selected, "the committed selection is empty; the figure would show nothing running"
    assert selected < {r["t"] for r in rows}, "the point is that some test is skipped"
    svg = chart.worked_example_svg(rows, selection, chart.LIGHT)
    skipped = [r["t"] for r in rows if r["t"] not in selected]
    for t in skipped:
        assert "skipped" in svg
    # the claim the whole figure exists to make: a selected test covers the changed file
    # without naming it, so selection cannot be filename matching
    changed = [c["path"] for c in selection["changed"] if c.get("instrumentable")]
    assert len(changed) == 1, f"the worked example must change exactly one file, got {changed}"
    indirect = [t for t in selected if changed[0] not in t and changed[0] in
                next(r["f"] for r in rows if r["t"] == t)]
    assert indirect, (
        "no selected test reaches the changed file indirectly, so this example no longer "
        "shows why coverage-derived selection differs from matching test filenames"
    )


def test_the_diagram_refuses_a_selection_the_map_does_not_support(worked):
    rows, selection = worked
    broken = copy.deepcopy(rows)
    for row in broken:
        row["f"] = [f for f in row["f"] if not f.endswith("b.py")]
    with pytest.raises(chart.ChartError, match="map row"):
        chart.worked_example_svg(broken, selection, chart.LIGHT)


@pytest.mark.parametrize("palette", [chart.LIGHT, chart.DARK])
def test_the_diagram_animates_and_is_well_formed(worked, palette):
    import xml.etree.ElementTree as ET

    rows, selection = worked
    svg = chart.worked_example_svg(rows, selection, palette)
    root = ET.fromstring(svg)
    ns = "{http://www.w3.org/2000/svg}"
    assert len(list(root.iter(ns + "animate"))) >= 4, "the figure does not animate"
    # a <text> whose attributes leaked into its body renders them as visible words; this
    # shipped once, from patching a rendered tag with a string replace
    for node in root.iter(ns + "text"):
        assert "font-family" not in (node.text or ""), f"malformed text node: {node.text!r}"
