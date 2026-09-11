"""The figures: the committed numbers, drawn, with the same rules the tables follow.

RTDD's claim is a trade: run a fraction of the suite, keep the safety of running all of
it. That is two numbers per strategy — how much was skipped, and what was still caught —
and a reader holding four tables in their head to compare them is a reader who will take
the cheapest row for the best one. These figures put the trade on two axes so the shape
of it is visible at a glance.

A picture is a claim with its arithmetic hidden, so three rules from `report.py` carry
over unchanged and each is enforced by a test in `tests/test_chart.py`.

**Nothing is drawn that was not measured.** Every mark carries
`data-strategy`/`data-metric`/`data-value` read straight out of `summary.json`, and its
geometry is that value's position on the axis — there is no path by which a figure
carries a number the records do not.

**A missing measurement is never drawn as zero.** `change_level_recall` is `null`
wherever a population detected nothing, and a scatter point on the floor would read as
"caught none of them" when what happened is that there was nothing to catch. Those
strategies are named under the plot instead. The same rule keeps `static` — a derived
arm that executed nothing — out of the wall-clock figure rather than in it at 0 ms.

**The verdict travels with the figure.** The safety chart renders
`report.secondary_verdict_lines` for the population it plots, so a reader who looks only
at the picture still learns that the pre-registered criterion was not met.

Figures are emitted as a light/dark pair per name and committed under
`docs/results/figures/`; `tests/test_chart.py` re-renders them from the committed
summaries and fails if the bytes differ, which is to the pictures what
`outcomes_test.go` is to the README.
"""

from __future__ import annotations

import dataclasses
import html
import json
import pathlib
from collections.abc import Sequence

from replay import report

# --- geometry ---------------------------------------------------------------------

WIDTH = 900
LABEL_WIDTH = 118
PLOT_WIDTH = 560
VALUE_GUTTER = 150
MARGIN = 28
ROW_HEIGHT = 30
BAR_HEIGHT = 9

NOT_COMPUTABLE = "not computable — the population detected nothing, so recall has no denominator"
NOT_MEASURED = report.NOT_MEASURED

#: Order is fixed so a re-render cannot churn the diff. `rtdd-vs-full` leads because it
#: is the only figure that answers the project's own question rather than the
#: pre-registered one; the `axis2-*` figures answer the baselines a reviewer will ask for.
FIGURES: tuple[str, ...] = ("how-it-picks", "rtdd-vs-full", "axis2-savings", "axis2-safety", "axis2-wallclock")

REPO_ROOT = pathlib.Path(__file__).resolve().parents[2]

#: The head-to-head, baseline first so the bars read grey-then-blue. No third strategy
#: may enter this tuple: the moment a cheaper baseline appears beside them, the figure
#: stops answering "is RTDD better than running everything" and starts answering
#: "is the map worth building", which is what the axis2 figures are for.
HEAD_TO_HEAD: tuple[str, ...] = ("full", "rtdd")

HEAD_TO_HEAD_SUMMARY = REPO_ROOT / "bench" / "results" / "paired" / "flask" / "summary.json"
AGENT_SESSION = REPO_ROOT / "bench" / "results" / "agent-session" / "flask.json"

#: Drawn under the head-to-head figure, because a figure this favourable travels without
#: its table and must carry its own limits.
HEAD_TO_HEAD_CAVEATS: tuple[str, ...] = (
    "Parity is the ceiling, not a tie broken in RTDD's favour: a full run catches what it catches by definition.",
    "Five detecting commits, one repository, and `probe` seeds the map at the child commit — an upper bound, not a general result.",
)

#: The one population in the corpus with any ground truth at all (flask, 3 detecting
#: commits). It is an upper bound and the figure says so on its face.
SAFETY_REPO = "flask"
SAFETY_VARIANT = "probe"


@dataclasses.dataclass(frozen=True)
class Palette:
    """Colour only. The data marks are identical across palettes, and tested to be."""

    name: str
    bg: str
    ink: str
    muted: str
    grid: str
    rtdd: str
    other: str
    full: str
    warn: str


LIGHT = Palette(
    name="light",
    bg="#ffffff",
    ink="#1f2328",
    muted="#656d76",
    grid="#d8dee4",
    rtdd="#0969da",
    other="#8250df",
    full="#6e7781",
    warn="#bc4c00",
)

DARK = Palette(
    name="dark",
    bg="#0d1117",
    ink="#e6edf3",
    muted="#9198a1",
    grid="#30363d",
    rtdd="#4493f8",
    other="#c297ff",
    full="#8b949e",
    warn="#f0883e",
)

PALETTES: tuple[Palette, ...] = (LIGHT, DARK)

FONT = "ui-sans-serif, -apple-system, Segoe UI, Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"


class ChartError(ValueError):
    """A figure was asked for something the committed records do not contain."""


# --- small helpers ----------------------------------------------------------------


def _esc(text: str) -> str:
    return html.escape(str(text), quote=True)


def _frac(value: float) -> str:
    return f"{value:.3f}"


def _data(strategy: str, metric: str, value: str) -> str:
    """The attribute triple every plotted mark carries, always in this order."""
    return f'data-strategy="{_esc(strategy)}" data-metric="{_esc(metric)}" data-value="{_esc(value)}"'


def _text(x: float, y: float, body: str, *, fill: str, size: float = 12, anchor: str = "start", mono: bool = False, weight: str = "normal") -> str:
    family = MONO if mono else FONT
    return (
        f'<text x="{x:.1f}" y="{y:.1f}" font-family="{family}" font-size="{size}" '
        f'font-weight="{weight}" fill="{fill}" text-anchor="{anchor}">{_esc(body)}</text>'
    )


def _colour(strategy: str, palette: Palette) -> str:
    if strategy == "rtdd":
        return palette.rtdd
    if strategy in ("full", "xdist"):
        return palette.full
    return palette.other


def _wrap(body: str, limit: int) -> list[str]:
    words = body.split()
    lines: list[str] = []
    current = ""
    for word in words:
        candidate = f"{current} {word}".strip()
        if len(candidate) > limit and current:
            lines.append(current)
            current = word
        else:
            current = candidate
    if current:
        lines.append(current)
    return lines


def _svg(width: int, height: int, body: Sequence[str], palette: Palette) -> str:
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
        f'viewBox="0 0 {width} {height}" role="img">',
        f'<rect x="0" y="0" width="{width}" height="{height}" fill="{palette.bg}"/>',
        *body,
        "</svg>",
    ]
    return "\n".join(parts) + "\n"


def _source_note(y: float, palette: Palette, body: str) -> str:
    return _text(MARGIN, y, body, fill=palette.muted, size=10.5, mono=True)


def _ordered(rows: dict, key: str) -> list[str]:
    """Cheapest first, ties broken by name so a re-render is byte-stable."""

    def sort_key(name: str) -> tuple[float, str]:
        value = rows[name].get(key, {}).get("value")
        return (1.0 if value is None else value, name)

    return sorted(rows, key=sort_key)


# --- figure 1: how much of the suite each strategy runs ----------------------------


def savings_svg(summaries: Sequence[dict], palette: Palette) -> str:
    """Tests selected and duration selected, per strategy, per repo.

    Both metrics are drawn because they come apart: a strategy can select few tests and
    still spend most of the suite's time if the tests it picks are the slow ones. The
    `full` row is the 1.000 reference at the plot's full width, so every other bar is
    read as the fraction of a whole run it replaces.
    """
    panels = []
    y = MARGIN + 46
    for summary in summaries:
        rows = summary["strategies"]
        repo = summary["repo_id"]
        panels.append(
            _text(MARGIN, y, f"{repo} — {rows['full']['cycles']} cycles, natural population", fill=palette.ink, size=13, weight="600")
        )
        y += 12
        panels.append(
            _text(
                MARGIN,
                y,
                f"{rows['full']['selection_ratio']['den']} tests, "
                f"{rows['full']['selected_duration_fraction']['den']} ms of recorded test time",
                fill=palette.muted,
                size=10.5,
            )
        )
        y += 20

        x0 = MARGIN + LABEL_WIDTH
        for gridline in (0.25, 0.5, 0.75, 1.0):
            gx = x0 + PLOT_WIDTH * gridline
            panels.append(
                f'<line x1="{gx:.1f}" y1="{y:.1f}" x2="{gx:.1f}" y2="{y + ROW_HEIGHT * len(rows):.1f}" '
                f'stroke="{palette.grid}" stroke-width="1"/>'
            )
            panels.append(
                _text(gx, y - 5, f"{int(gridline * 100)}%", fill=palette.muted, size=9.5, anchor="middle")
            )

        for name in _ordered(rows, "selected_duration_fraction"):
            row = rows[name]
            tests = row["selection_ratio"]["value"]
            time = row["selected_duration_fraction"]["value"]
            colour = _colour(name, palette)
            weight = "700" if name == "rtdd" else "normal"
            panels.append(_text(MARGIN, y + 12, name, fill=palette.ink, size=11.5, mono=True, weight=weight))

            for offset, value, metric, opacity in (
                (2, tests, "selection_ratio", "0.45"),
                (2 + BAR_HEIGHT + 2, time, "selected_duration_fraction", "1"),
            ):
                width = PLOT_WIDTH * value
                panels.append(
                    f'<rect x="{x0:.1f}" y="{y + offset:.1f}" width="{width:.1f}" height="{BAR_HEIGHT}" '
                    f'rx="2" fill="{colour}" opacity="{opacity}" '
                    f"{_data(name, f'{repo}/{metric}', _frac(value))}/>"
                )

            panels.append(
                _text(
                    x0 + PLOT_WIDTH + 12,
                    y + 15,
                    f"{_frac(tests)} tests   {_frac(time)} time",
                    fill=palette.muted,
                    size=10.5,
                    mono=True,
                )
            )
            y += ROW_HEIGHT
        y += 26

    header = [
        _text(MARGIN, MARGIN + 4, "How much of the suite each strategy runs", fill=palette.ink, size=17, weight="700"),
        _text(
            MARGIN,
            MARGIN + 24,
            "Lower is cheaper. Pale bar = share of tests selected; solid bar = share of the suite's test time.",
            fill=palette.muted,
            size=11.5,
        ),
    ]
    footer_y = y + 4
    footer = [
        _source_note(footer_y, palette, "source: bench/results/<repo>/summary.json · strategies.<name> · natural population"),
        _text(
            MARGIN,
            footer_y + 16,
            "Cost alone does not rank these — a strategy that selects nothing is cheapest and catches nothing. See the safety figure.",
            fill=palette.muted,
            size=10.5,
        ),
    ]
    return _svg(WIDTH, int(footer_y + 30), header + panels + footer, palette)


# --- figure 2: what the saving costs in safety -------------------------------------


def safety_svg(summary: dict, variant: str, palette: Palette) -> str:
    """Change-level recall against selected-duration fraction, for one population.

    This is the figure that answers the actual question: a strategy is only worth
    running if the time it saves does not cost the catches a full suite would have made.
    `full` sits at (1.000, 1.000) by definition — the whole suite, every catch — and the
    useful corner is the top-left.
    """
    rows = summary["by_variant"][variant]
    repo = summary["repo_id"]

    plot_x = MARGIN + 54
    plot_y = MARGIN + 62
    plot_w = 520
    plot_h = 320

    body = [
        _text(MARGIN, MARGIN + 4, "What the saving costs in safety", fill=palette.ink, size=17, weight="700"),
        _text(
            MARGIN,
            MARGIN + 24,
            f"{repo} · {variant} population — change-level recall against the share of the suite's time spent",
            fill=palette.muted,
            size=11.5,
        ),
        _text(
            MARGIN,
            MARGIN + 40,
            "Top-left is the goal: catches everything a full run catches, on a fraction of the time.",
            fill=palette.muted,
            size=11.5,
        ),
    ]

    for i in range(5):
        gy = plot_y + plot_h * i / 4
        body.append(
            f'<line x1="{plot_x:.1f}" y1="{gy:.1f}" x2="{plot_x + plot_w:.1f}" y2="{gy:.1f}" '
            f'stroke="{palette.grid}" stroke-width="1"/>'
        )
        body.append(
            _text(plot_x - 10, gy + 4, _frac(1 - i / 4), fill=palette.muted, size=9.5, anchor="end", mono=True)
        )
        gx = plot_x + plot_w * i / 4
        body.append(
            f'<line x1="{gx:.1f}" y1="{plot_y:.1f}" x2="{gx:.1f}" y2="{plot_y + plot_h:.1f}" '
            f'stroke="{palette.grid}" stroke-width="1"/>'
        )
        body.append(
            _text(gx, plot_y + plot_h + 16, _frac(i / 4), fill=palette.muted, size=9.5, anchor="middle", mono=True)
        )

    body.append(
        _text(plot_x + plot_w / 2, plot_y + plot_h + 34, "share of the suite's test time spent  →  more expensive", fill=palette.muted, size=11, anchor="middle")
    )
    body.append(
        f'<g transform="translate({MARGIN - 6},{plot_y + plot_h / 2}) rotate(-90)">'
        + _text(0, 0, "change-level recall  →  safer", fill=palette.muted, size=11, anchor="middle")
        + "</g>"
    )

    computable = [name for name in sorted(rows) if rows[name]["change_level_recall"]["value"] is not None]
    missing = [name for name in sorted(rows) if name not in computable]

    # Strategies land on top of each other — `full` and `xdist` both run everything, and
    # `static` selects exactly what `path` does on this corpus. Every one of them still
    # gets its own mark, because a reader counting points must find them all, but the
    # labels are merged: two labels drawn at one coordinate are unreadable, and moving a
    # point apart to make room would be drawing a number the records do not contain.
    points: list[tuple[float, float, list[str]]] = []
    for name in computable:
        row = rows[name]
        recall = row["change_level_recall"]["value"]
        cost = row["selected_duration_fraction"]["value"]
        cx = round(plot_x + plot_w * cost, 1)
        cy = round(plot_y + plot_h * (1 - recall), 1)
        colour = _colour(name, palette)
        radius = 7 if name == "rtdd" else 5
        body.append(
            f'<g {_data(name, "selected_duration_fraction", _frac(cost))}>'
            f'<circle cx="{cx:.1f}" cy="{cy:.1f}" r="{radius}" fill="{colour}" '
            f'stroke="{palette.bg}" stroke-width="1.5" '
            f"{_data(name, 'change_level_recall', _frac(recall))}/></g>"
        )
        for px_, py_, names in points:
            if (px_, py_) == (cx, cy):
                names.append(name)
                break
        else:
            points.append((cx, cy, [name]))

    for cx, cy, names in _placed(points, plot_y, plot_h):
        row = rows[names[0]]
        recall = row["change_level_recall"]
        cost = row["selected_duration_fraction"]["value"]
        label = f"{'/'.join(names)}  {recall['num']}/{recall['den']} caught @ {_frac(cost)}"
        anchor = "end" if cx > plot_x + plot_w * 0.62 else "start"
        dx = -12 if anchor == "end" else 12
        body.append(
            _text(
                cx + dx,
                cy + 4,
                label,
                fill=palette.ink,
                size=10.5,
                anchor=anchor,
                mono=True,
                weight="700" if "rtdd" in names else "normal",
            )
        )

    y = plot_y + plot_h + 52
    if missing:
        body.append(
            _text(MARGIN, y, f"{NOT_COMPUTABLE}: {', '.join(missing)}", fill=palette.warn, size=10.5)
        )
        y += 16
    if variant != summary["primary_variant"]:
        body.append(
            _text(
                MARGIN,
                y,
                "upper bound — every map-based strategy is seeded at the child commit in this population, and it is never pooled with natural.",
                fill=palette.warn,
                size=10.5,
            )
        )
        y += 16

    verdict = _verdict_for(summary, variant)
    for line in _wrap(verdict, 118):
        body.append(_text(MARGIN, y, line, fill=palette.ink, size=10.5, mono=True))
        y += 14

    y += 6
    body.append(_source_note(y, palette, f"source: bench/results/{repo}/summary.json · by_variant.{variant} · verdict from replay/report.py"))
    return _svg(WIDTH, int(y + 20), body, palette)


def _placed(
    points: list[tuple[float, float, list[str]]], plot_y: float, plot_h: float
) -> list[tuple[float, float, list[str]]]:
    """Nudge label baselines apart vertically until none overlaps another.

    Only the label moves; the point it names stays exactly where its two numbers put
    it. Four strategies share recall 1.000 on flask's probe, so without this the top of
    the plot is one illegible smear.
    """
    placed: list[tuple[float, float, list[str]]] = []
    taken: list[float] = []
    for cx, cy, names in sorted(points, key=lambda p: (p[1], p[0])):
        label_y = cy
        step = 0
        while any(abs(label_y - t) < 13 for t in taken):
            step += 1
            offset = 13 * ((step + 1) // 2) * (1 if step % 2 else -1)
            label_y = cy + offset
            label_y = min(max(label_y, plot_y + 6), plot_y + plot_h - 4)
            if step > 12:
                break
        taken.append(label_y)
        placed.append((cx, label_y, names))
    return placed


def _verdict_for(summary: dict, variant: str) -> str:
    """The pre-registered comparison for this population, taken from `report.py`.

    Never re-derived here: the figure and the table must not be able to disagree about
    whether the criterion was met.
    """
    if variant == summary["primary_variant"]:
        line = report.verdict_line(summary)
    else:
        wanted = f"verdict ({variant}"
        matches = [l for l in report.secondary_verdict_lines(summary) if l.startswith(wanted)]
        if not matches:
            raise ChartError(f"no verdict line for variant {variant!r}")
        line = matches[0]
    return line.split("): ", 1)[-1].split(": ", 1)[-1] if "): " in line else line.split(": ", 1)[-1]


# --- figure 3: what a cycle actually costs -----------------------------------------


def wallclock_svg(summaries: Sequence[dict], palette: Palette) -> str:
    """p50, p90 and worst against the full suite — the distribution, never a bare mean.

    The population is bimodal: a cycle whose strategy selected nothing costs almost
    nothing, a cycle that selected the hub costs nearly a full run. A mean sits between
    the two modes and describes neither, so the mean is deliberately not plotted. The
    dashed line is the same repo's full uninstrumented suite; a bar crossing it is a
    strategy that cost more than running everything.
    """
    body = [
        _text(MARGIN, MARGIN + 4, "What one cycle actually costs", fill=palette.ink, size=17, weight="700"),
        _text(
            MARGIN,
            MARGIN + 24,
            "Bar = median cycle. Tick = p90. Dot = worst cycle. Dashed = the full suite, uninstrumented.",
            fill=palette.muted,
            size=11.5,
        ),
    ]

    y = MARGIN + 52
    omitted: list[str] = []
    for summary in summaries:
        repo = summary["repo_id"]
        rows = summary["wallclock"]["rows"]
        omitted += [
            f"{repo}/{name}"
            for name in sorted(summary["strategies"])
            if name not in rows
        ]
        reference = rows["full"]["mean_full_uninstrumented_ms"]
        scale_max = max(max(r["worst_subset_uninstrumented_ms"] for r in rows.values()), reference)

        body.append(_text(MARGIN, y, f"{repo} — full suite {reference} ms", fill=palette.ink, size=13, weight="600"))
        body.append(
            _text(
                MARGIN + LABEL_WIDTH + PLOT_WIDTH + 12,
                y,
                "p50 · p90 · worst",
                fill=palette.muted,
                size=9.5,
                mono=True,
            )
        )
        y += 18

        x0 = MARGIN + LABEL_WIDTH
        ref_x = x0 + PLOT_WIDTH * reference / scale_max
        panel_top = y
        ordered = sorted(rows, key=lambda n: (rows[n]["p90_subset_uninstrumented_ms"], n))

        for name in ordered:
            row = rows[name]
            p50 = row["p50_subset_uninstrumented_ms"]
            p90 = row["p90_subset_uninstrumented_ms"]
            worst = row["worst_subset_uninstrumented_ms"]
            colour = _colour(name, palette)
            weight = "700" if name == "rtdd" else "normal"
            body.append(_text(MARGIN, y + 14, name, fill=palette.ink, size=11.5, mono=True, weight=weight))

            def px(ms: int) -> float:
                return x0 + PLOT_WIDTH * ms / scale_max

            body.append(
                f'<line x1="{x0:.1f}" y1="{y + 10:.1f}" x2="{px(worst):.1f}" y2="{y + 10:.1f}" '
                f'stroke="{colour}" stroke-width="1" opacity="0.4"/>'
            )
            body.append(
                f'<rect x="{x0:.1f}" y="{y + 5:.1f}" width="{max(px(p50) - x0, 1.0):.1f}" height="{BAR_HEIGHT}" '
                f'rx="2" fill="{colour}" '
                f"{_data(name, f'{repo}/p50_subset_uninstrumented_ms', str(p50))}/>"
            )
            body.append(
                f'<rect x="{px(p90):.1f}" y="{y + 3:.1f}" width="2" height="{BAR_HEIGHT + 4}" '
                f'rx="1" fill="{colour}" '
                f"{_data(name, f'{repo}/p90_subset_uninstrumented_ms', str(p90))}/>"
            )
            body.append(
                f'<circle cx="{px(worst):.1f}" cy="{y + 9.5:.1f}" r="3.5" fill="{palette.bg}" stroke="{colour}" stroke-width="1.5" '
                f"{_data(name, f'{repo}/worst_subset_uninstrumented_ms', str(worst))}/>"
            )
            body.append(
                _text(
                    x0 + PLOT_WIDTH + 12,
                    y + 14,
                    f"{p50} · {p90} · {worst} ms",
                    fill=palette.muted,
                    size=10,
                    mono=True,
                )
            )
            y += 24

        body.append(
            f'<line x1="{ref_x:.1f}" y1="{panel_top - 4:.1f}" x2="{ref_x:.1f}" y2="{y:.1f}" '
            f'stroke="{palette.warn}" stroke-width="1.5" stroke-dasharray="4 3" '
            f"{_data('full', f'{repo}/reference_full_uninstrumented_ms', str(reference))}/>"
        )
        y += 22

    if omitted:
        body.append(
            _text(
                MARGIN,
                y,
                f"{NOT_MEASURED}: {', '.join(omitted)} — a derived arm executed nothing, and a blank would read as zero.",
                fill=palette.warn,
                size=10.5,
            )
        )
        y += 16
    body.append(_source_note(y, palette, "source: bench/results/<repo>/summary.json · wallclock.rows · subset uninstrumented"))
    return _svg(WIDTH, int(y + 20), body, palette)


# --- figure 0: the head-to-head the README leads with ------------------------------


def _ms(value: float) -> str:
    return f"{int(round(value))} ms"


def load_head_to_head() -> tuple[dict, dict]:
    """The two committed records this figure is drawn from, and nothing else.

    Kept separate from the corpus summaries `cmd_chart` collects: the paired population
    lives under its own results subtree (it is a different commit selection, not a
    different repo) and the agent-session record is not a replay at all — it is the
    shipped binary driven against a real clone, which is the only measurement that
    includes RTDD's own cost.
    """
    try:
        summary = json.loads(HEAD_TO_HEAD_SUMMARY.read_text(encoding="utf-8"))
        session = json.loads(AGENT_SESSION.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise ChartError(f"the head-to-head figure needs {exc.filename}") from exc
    return summary, session


def head_to_head_svg(summary: dict, session: dict, palette: Palette) -> str:
    """RTDD against running the whole suite. No third strategy appears here.

    Every other figure in this directory answers the pre-registered question — is a
    coverage map worth building, against the cheap baselines a reviewer will name. This
    one answers the project's own question: does an agent that runs RTDD spend less time
    than an agent that runs everything, and does it still catch what everything catches.
    Those are different questions, and mixing the baselines into this figure is what made
    the earlier ones unreadable for it.
    """
    rows = summary["strategies"]
    for name in HEAD_TO_HEAD:
        if name not in rows:
            raise ChartError(f"the head-to-head figure needs the {name!r} row")

    x0 = MARGIN + LABEL_WIDTH
    body: list[str] = [
        _text(MARGIN, MARGIN + 4, "RTDD against running the whole suite", fill=palette.ink, size=17, weight="700"),
        _text(
            MARGIN,
            MARGIN + 24,
            "The only comparison drawn here. Grey = run every test after every change. Blue = RTDD.",
            fill=palette.muted,
            size=11.5,
        ),
    ]
    y = MARGIN + 52

    # --- panel 1: the loop RTDD exists for, shipped binary, its own cost included ---
    body.append(
        _text(MARGIN, y, "The agent's loop — 10 edits to one module, real flask clone", fill=palette.ink, size=13, weight="600")
    )
    y += 12
    body.append(
        _text(
            MARGIN,
            y,
            f"{session['suite_tests']} tests in the suite, {session['selected_tests']} selected · shorter is better",
            fill=palette.muted,
            size=10.5,
        )
    )
    y += 20

    ms = session["session_ms"]
    baseline = float(ms["full"])
    loop = (
        ("full suite", "full", baseline, "baseline"),
        ("rtdd run", "rtdd", float(ms["always"]), f"{baseline / float(ms['always']):.2f}× faster"),
        ("  --record=auto", "rtdd", float(ms["auto"]), f"{baseline / float(ms['auto']):.2f}× faster"),
    )
    for label, strategy, value, note in loop:
        colour = _colour(strategy, palette)
        weight = "700" if strategy == "rtdd" else "normal"
        body.append(_text(MARGIN, y + 12, label, fill=palette.ink, size=11.5, mono=True, weight=weight))
        body.append(
            f'<rect x="{x0:.1f}" y="{y + 3:.1f}" width="{PLOT_WIDTH * (value / baseline):.1f}" '
            f'height="{BAR_HEIGHT + 3}" rx="2" fill="{colour}" '
            f"{_data(strategy, f'session_ms/{label.strip()}', _ms(value))}/>"
        )
        body.append(
            _text(x0 + PLOT_WIDTH + 12, y + 14, f"{_ms(value)}   {note}", fill=palette.muted, size=10.5, mono=True)
        )
        y += ROW_HEIGHT
    y += 20

    # --- panel 2: the parity claim, on the only population that can measure it ------
    detecting = rows["full"]["change_level_recall"]["den"]
    body.append(
        _text(MARGIN, y, "What it catches, against what a full run catches", fill=palette.ink, size=13, weight="600")
    )
    y += 12
    body.append(
        _text(
            MARGIN,
            y,
            f"flask, paired population, {detecting} detecting commits · taller is better, except the last row",
            fill=palette.muted,
            size=10.5,
        )
    )
    y += 20

    stratum_of = {name: rows[name].get("strata", {}).get("1", {}) for name in HEAD_TO_HEAD}
    measures = (
        ("caught the change", "change_level_recall", lambda n: rows[n]["change_level_recall"], False),
        ("…where one test fails", "strata/1/change_level_recall", lambda n: stratum_of[n].get("change_level_recall"), False),
        ("share of suite time", "selected_duration_fraction", lambda n: rows[n]["selected_duration_fraction"], True),
    )
    for label, metric, pick, lower_is_better in measures:
        body.append(_text(MARGIN, y + 16, label, fill=palette.ink, size=11.5))
        for index, name in enumerate(HEAD_TO_HEAD):
            cell = pick(name)
            if cell is None or cell.get("value") is None:
                body.append(
                    _text(x0, y + 16 + index * 13, f"{name}: {NOT_COMPUTABLE}", fill=palette.warn, size=10, mono=True)
                )
                continue
            value = float(cell["value"])
            colour = _colour(name, palette)
            offset = 2 + index * (BAR_HEIGHT + 3)
            body.append(
                f'<rect x="{x0:.1f}" y="{y + offset:.1f}" width="{PLOT_WIDTH * value:.1f}" '
                f'height="{BAR_HEIGHT}" rx="2" fill="{colour}" '
                f"{_data(name, metric, _frac(value))}/>"
            )
            shown = f"{_frac(value)}"
            if cell.get("den") is not None and not lower_is_better:
                shown += f" ({cell['num']}/{cell['den']})"
            body.append(
                _text(x0 + PLOT_WIDTH + 12, y + offset + 8, f"{name}  {shown}", fill=palette.muted, size=10, mono=True)
            )
        y += ROW_HEIGHT + 6

    y += 6
    footer = [
        _source_note(y, palette, f"source: {HEAD_TO_HEAD_SUMMARY.relative_to(REPO_ROOT).as_posix()} · {AGENT_SESSION.relative_to(REPO_ROOT).as_posix()}"),
    ]
    for offset, line in enumerate(HEAD_TO_HEAD_CAVEATS):
        footer.append(_text(MARGIN, y + 16 + offset * 13, line, fill=palette.muted, size=10.5))
    return _svg(WIDTH, int(y + 20 + 13 * len(HEAD_TO_HEAD_CAVEATS)), body + footer, palette)


# --- figure: the worked example, animated ----------------------------------------

WORKED_EXAMPLE = "docs/results/worked-example"

#: One loop. Cues below are seconds on this timeline.
DUR = 7.0
CUE_CHANGE = 0.4      # the changed feature starts pulsing
CUE_TRACE = 1.3       # the coverage edges out of it light up
CUE_RUN = 2.0         # both panels start running tests
STEP = 0.85           # one test's worth of running time
CUE_HOLD = 6.3        # everything is settled and holds until the loop restarts


def _t(seconds: float) -> float:
    """A cue in seconds -> its keyTime fraction on the loop."""
    return round(seconds / DUR, 5)


def load_worked_example(root: pathlib.Path) -> tuple[list[dict], dict]:
    base = root / WORKED_EXAMPLE
    try:
        rows = [json.loads(l) for l in (base / "map.jsonl").read_text().splitlines() if l.strip()]
        selection = json.loads((base / "selection-feature-b.json").read_text())
    except FileNotFoundError as exc:
        raise ChartError(f"the worked-example figure needs {exc.filename}") from exc
    if not rows:
        raise ChartError(f"{base / 'map.jsonl'} is empty")
    return rows, selection


def _short(path: str) -> str:
    """`src/b.py` -> `b.py`, `tests/test_a.py::test_a` -> `test_a`."""
    if "::" in path:
        return path.split("::", 1)[1]
    return path.rsplit("/", 1)[-1]


def _anim(attr: str, values: str, key_times: str, *, calc: str = "discrete") -> str:
    return (
        f'<animate attributeName="{attr}" values="{values}" keyTimes="{key_times}" '
        f'dur="{DUR}s" calcMode="{calc}" repeatCount="indefinite"/>'
    )


def worked_example_svg(rows: Sequence[dict], selection: dict, palette: Palette) -> str:
    """One change, two approaches, the same three tests.

    The point the prose could not make: `test_a` never mentions the changed file. It is
    selected because the seed run watched it execute that file, which is the whole
    difference between this and matching `tests/test_<module>.py`.
    """
    changed = "src/b.py"
    selected = set(selection["selection"]["tests"])
    tests = [r["t"] for r in rows]
    covers = {r["t"]: set(r["f"]) for r in rows}
    for t in tests:
        if changed not in covers[t] and t in selected:
            raise ChartError(f"{t} is selected but its map row does not carry {changed}")

    W, ROW_Y, FEAT_Y = 900, 196, 104
    cols = {t: 118 + i * 262 for i, t in enumerate(tests)}
    body: list[str] = [
        _text(MARGIN, MARGIN + 4, "How RTDD picks tests", fill=palette.ink, size=17, weight="700"),
        _text(MARGIN, MARGIN + 24,
              f"You changed one feature. {_short(changed)} is called by one other feature, and that is enough to matter.",
              fill=palette.muted, size=11.5),
    ]

    # --- the code: features on top, their tests below, coverage drawn between --------
    feats = ["src/a.py", "src/b.py", "src/c.py"]
    fx = {f: 118 + i * 262 for i, f in enumerate(feats)}
    body.append(f'<path d="M{fx["src/a.py"] + 52:.0f} {FEAT_Y + 16} L{fx["src/b.py"] - 52:.0f} {FEAT_Y + 16}" '
                f'stroke="{palette.muted}" stroke-width="1.5" marker-end="url(#arrow-{palette.name})"/>')
    body.append(_text((fx["src/a.py"] + fx["src/b.py"]) / 2, FEAT_Y + 8, "calls",
                      fill=palette.muted, size=9.5, anchor="middle"))

    for f in feats:
        is_changed = f == changed
        colour = palette.warn if is_changed else palette.muted
        box = (f'<rect x="{fx[f] - 52}" y="{FEAT_Y}" width="104" height="32" rx="6" '
               f'fill="none" stroke="{colour}" stroke-width="{2.5 if is_changed else 1.5}"')
        if is_changed:
            box += '>' + _anim("stroke-opacity", "0.25;1;1;1", f"0;{_t(CUE_CHANGE)};{_t(CUE_HOLD)};1", calc="linear") + "</rect>"
        else:
            box += "/>"
        body.append(box)
        body.append(_text(fx[f], FEAT_Y + 21, _short(f), fill=palette.ink, size=12, anchor="middle", mono=True,
                          weight="700" if is_changed else "normal"))
    body.append(_text(fx[changed], FEAT_Y - 10, "you changed this", fill=palette.warn, size=10, anchor="middle"))

    # coverage edges: every (test, file) pair the map actually recorded
    for t in tests:
        for f in feats:
            if f not in covers[t]:
                continue
            live = f == changed
            edge = (f'<path d="M{cols[t]} {ROW_Y - 14} C{cols[t]} {ROW_Y - 44}, {fx[f]} {FEAT_Y + 62}, {fx[f]} {FEAT_Y + 34}" '
                    f'fill="none" stroke="{palette.warn if live else palette.grid}" stroke-width="{2 if live else 1.2}" '
                    f'stroke-dasharray="4 3"')
            if live:
                edge += ">" + _anim("stroke-opacity", "0.15;0.15;1;1", f"0;{_t(CUE_TRACE)};{_t(CUE_TRACE + 0.35)};1", calc="linear") + "</path>"
            else:
                edge += "/>"
            body.append(edge)

    for t in tests:
        body.append(_text(cols[t], ROW_Y, _short(t), fill=palette.ink, size=12, anchor="middle", mono=True))
        body.append(_text(cols[t], ROW_Y + 14,
                          "covers " + ", ".join(sorted(_short(f) for f in covers[t] if f in feats)),
                          fill=palette.muted, size=9.5, anchor="middle"))

    # --- the two panels -------------------------------------------------------------
    panels = (
        ("Run everything", tests, palette.full, 28),
        ("RTDD", [t for t in tests if t in selected], palette.rtdd, 470),
    )
    PANEL_Y, PANEL_W = 262, 402
    for title, runs, colour, px in panels:
        body.append(f'<rect x="{px}" y="{PANEL_Y}" width="{PANEL_W}" height="146" rx="8" fill="none" stroke="{palette.grid}"/>')
        body.append(_text(px + 16, PANEL_Y + 24, title, fill=palette.ink, size=13, weight="700"))
        body.append(_text(px + PANEL_W - 16, PANEL_Y + 24, f"{len(runs)} of {len(tests)} tests",
                          fill=palette.muted, size=11, anchor="end", mono=True))
        order = 0
        for t in tests:
            y = PANEL_Y + 44 + tests.index(t) * 24
            running = t in runs
            body.append(_text(px + 16, y + 13, _short(t), fill=palette.ink if running else palette.muted,
                              size=11, mono=True))
            bar_x, bar_w = px + 104, PANEL_W - 132
            body.append(f'<rect x="{bar_x}" y="{y + 4}" width="{bar_w}" height="11" rx="3" fill="{palette.grid}" opacity="0.5"/>')
            if running:
                start = CUE_RUN + order * STEP
                body.append(
                    f'<rect x="{bar_x}" y="{y + 4}" width="{bar_w}" height="11" rx="3" fill="{colour}">'
                    + _anim("width", f"0;0;{bar_w};{bar_w}",
                            f"0;{_t(start)};{_t(start + STEP)};1", calc="linear")
                    + "</rect>"
                )
                order += 1
            else:
                body.append(_text(bar_x + 6, y + 13, "skipped — nothing it covers changed",
                                  fill=palette.muted, size=9.5))
        done = CUE_RUN + len(runs) * STEP
        # Built directly rather than by patching `_text`: this element needs both a
        # starting opacity and a child <animate>, and string-surgery on a rendered tag
        # closed it early — the attributes rendered as visible text in the figure.
        body.append(
            f'<text x="{px + 16:.1f}" y="{PANEL_Y + 130:.1f}" font-family="{FONT}" font-size="11" '
            f'font-weight="700" fill="{colour}" text-anchor="start" opacity="0">'
            f'{_esc(f"caught the change · finished after {len(runs)} tests")}'
            + _anim("opacity", "0;0;1;1", f"0;{_t(done)};{_t(done + 0.25)};1", calc="linear")
            + "</text>"
        )

    body.append(_source_note(438, palette,
                             f"source: {WORKED_EXAMPLE}/map.jsonl · {WORKED_EXAMPLE}/selection-feature-b.json"))
    defs = (f'<defs><marker id="arrow-{palette.name}" viewBox="0 0 10 10" refX="9" refY="5" '
            f'markerWidth="6" markerHeight="6" orient="auto-start-reverse">'
            f'<path d="M0 0 L10 5 L0 10 z" fill="{palette.muted}"/></marker></defs>')
    return _svg(W, 460, [defs, *body], palette)

# --- emission ----------------------------------------------------------------------


def render_figure(figure: str, summaries: Sequence[dict], palette: Palette) -> str:
    if figure == "how-it-picks":
        rows, selection = load_worked_example(REPO_ROOT)
        return worked_example_svg(rows, selection, palette)
    if figure == "rtdd-vs-full":
        summary, session = load_head_to_head()
        return head_to_head_svg(summary, session, palette)
    if figure == "axis2-savings":
        return savings_svg(summaries, palette)
    if figure == "axis2-wallclock":
        return wallclock_svg(summaries, palette)
    if figure == "axis2-safety":
        chosen = [s for s in summaries if s["repo_id"] == SAFETY_REPO]
        if not chosen:
            raise ChartError(f"the safety figure needs {SAFETY_REPO}'s summary")
        return safety_svg(chosen[0], SAFETY_VARIANT, palette)
    raise ChartError(f"unknown figure {figure!r}")


def figure_names() -> tuple[str, ...]:
    return tuple(f"{figure}-{palette.name}.svg" for figure in FIGURES for palette in PALETTES)


def render_all(summaries: Sequence[dict]) -> dict[str, str]:
    """Every figure in both palettes, keyed `<figure>-<palette>.svg`."""
    return {
        f"{figure}-{palette.name}.svg": render_figure(figure, summaries, palette)
        for figure in FIGURES
        for palette in PALETTES
    }


def write_figures(summaries: Sequence[dict], out_dir: pathlib.Path) -> list[pathlib.Path]:
    out_dir.mkdir(parents=True, exist_ok=True)
    written = []
    for name, svg in render_all(summaries).items():
        path = out_dir / name
        path.write_text(svg)
        written.append(path)
    return written
