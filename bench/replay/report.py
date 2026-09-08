"""What gets written to `bench/results/` — diffable, committed, and self-describing.

Three constraints shape every function here.

**Diffability.** The results are committed, so a re-run at an identical config has
to produce identical bytes. `commits.jsonl` is sorted by
`(repo_id, commit, variant, kind, strategy)` with compact separators and sorted
keys, and `summary.json` is `canonical_json`; a re-run therefore changes only the
lines whose numbers moved, and a reviewer can see exactly which commits moved.

**No result without its config.** `write_results` always emits `config.json` beside
the numbers — corpus digest, tool versions, strategy set, the identity of the
`rtdd` binary under test and the hardware fingerprint — so a table can never be
read without the run that produced it.

**The two populations never pool, and neither do two repos.** `natural` and
`probe` are built from different base trees, so every table is scoped to one
variant: the headline table is the primary variant (`natural` when it was run) and
each variant gets its own table in the by-variant section, where `probe` carries
the upper-bound label on the same line as its numbers. The only cross-repo file in
the tree is `aggregate.md`, which is duration-weighted and says so.

The `verdict:` line is unconditional. It prints in every `summary.md`, and when
RTDD does not clearly beat the naive path heuristic it prints
:data:`FAILURE_WORDING` verbatim — the pre-registered criterion fired, and the
honest outcome is to publish that rather than re-run until the number improves.
The `static verdict:` line is the same discipline applied to spec §7's second
pre-registration: it prints wherever the run carries a `static` arm, shares one body
with the first so the two rules cannot drift, and prints
:data:`STATIC_FAILURE_WORDING` verbatim when the kill condition fires.
"""

from __future__ import annotations

import pathlib
from collections.abc import Sequence

from replay import derive, falsesignal, metrics
from replay.config import RunConfig, canonical_json
from replay.hardware import Hardware
from replay.records import to_jsonl_lines
from replay.session import DriftCurve

MAP_BASED_STRATEGIES = frozenset({"rtdd", "testmon"})
"""The strategies `probe` flatters.

`probe` reverts the commit's source half over the child tree, so a strategy whose
state is a map built at the base tree gets that map seeded at the child commit —
after the change it is being asked to find. The path heuristic, the import graph,
`--lf` and random draw no benefit from that at all, so the label is per row rather
than per table.
"""

UPPER_BOUND_LABEL = "upper bound (map seeded at the child commit)"

WALLCLOCK_COLUMNS: tuple[str, ...] = (
    "full_uninstrumented_ms",
    "subset_instrumented_ms",
    "subset_uninstrumented_ms",
)
"""The three timings every wall-clock row publishes, in table order.

Named once so the aggregation, the markdown table and the audit cannot drift
apart — a column added here without a percentile beside it fails
:func:`assert_distribution_beside_mean` on the renderer's own output.
"""

WALLCLOCK_COLUMN_LABELS: dict[str, str] = {
    "full_uninstrumented_ms": "full uninstrumented",
    "subset_instrumented_ms": "subset instrumented",
    "subset_uninstrumented_ms": "subset uninstrumented",
}

#: The spread a published mean is worthless without. PRD #6 acceptance criterion 9
#: asks for exactly these three beside every wall-clock figure.
_DISTRIBUTION_HEADERS: tuple[str, ...] = ("p50", "p90", "worst")

COMPARISON_ARMS: tuple[str, ...] = ("static", "rtdd", "path", "full")
"""The §7 evidence table, in table order.

`static` is the arm under test; `rtdd` is the coverage-derived answer it is being
measured against, `path` is the pre-registered baseline, and `full` is the ceiling
every ratio is read against. A reader given `static` alone can compare it to nothing.
"""

COMPARISON_METRICS: tuple[tuple[str, str], ...] = (
    ("change_level_recall", "change recall"),
    ("selection_ratio", "selection ratio"),
    ("selected_duration_fraction", "selected duration"),
)
"""The three §7 metrics (PRD #233 AC3), as `(summary key, column label)`."""

COMPARISON_WALLCLOCK_COLUMN = "subset_uninstrumented_ms"
"""The one wall-clock column the comparison publishes.

The cost question §7 asks is what running the selection cost, so the uninstrumented
subset run is the honest figure; the other two columns stay in `## Wall-clock`, where
all three are published side by side.
"""

COMPARISON_HEADER_KEY = "arm"
"""The first header cell of a §7 comparison table, and how the guard recognises one.

A marker rather than a heuristic: :func:`assert_distribution_beside_mean` refuses a
table starting with this cell that publishes no wall-clock at all, so the column
cannot be dropped to escape the spread requirement.
"""

NOT_MEASURED = "not measured"
"""What an arm with no wall-clock record publishes.

Never a blank — a blank cell in a millisecond column reads as zero — and never another
arm's figure. A derived arm executed nothing, and the table says exactly that.
"""


class ReportError(RuntimeError):
    """The document the renderer produced is not publishable as written."""


def _tables(text: str) -> list[list[list[str]]]:
    """Every markdown table in ``text``, as lists of cell lists."""
    out: list[list[list[str]]] = []
    current: list[list[str]] | None = None
    for line in text.splitlines():
        if line.startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            current = [cells] if current is None else [*current, cells]
        elif current is not None:
            out.append(current)
            current = None
    if current is not None:
        out.append(current)
    return out


def _is_wallclock_header(header: Sequence[str]) -> bool:
    labels = WALLCLOCK_COLUMN_LABELS.values()
    return any(label in cell.lower() for cell in header for label in labels)


def _is_comparison_header(header: Sequence[str]) -> bool:
    """Whether ``header`` opens a §7 comparison table."""
    return bool(header) and header[0].strip().lower() == COMPARISON_HEADER_KEY


def assert_distribution_beside_mean(text: str) -> None:
    """Refuse any table that publishes a wall-clock figure without its spread.

    Two tables are caught, because two ways of hiding the spread exist. A table
    whose header says `mean` and stops there is the obvious one. The subtler one
    — and the one this repo actually shipped — is a column headed `subset
    uninstrumented` whose cells are means that never say so: `rtdd`'s mean of
    1283 ms sat beside a p50 of 545 ms and a worst of 4045 ms, and a reader had
    no way to see it.

    A text check on purpose, like the Axis 1 resolution audit: it sees what a
    reader sees, so the guarantee survives whatever the renderer grows into
    rather than holding only for the tables it emits today.
    """
    for table in _tables(text):
        header = table[0]
        comparison = _is_comparison_header(header)
        cites_mean = any("mean" in cell.lower() for cell in header)
        if not comparison and not cites_mean and not _is_wallclock_header(header):
            continue
        # A comparison table owes the mean itself, not only its spread: an arm that
        # measured nothing invites dropping the column entirely, and a table with no
        # wall-clock column at all would otherwise satisfy this guard by omission.
        required = ("mean", *_DISTRIBUTION_HEADERS) if comparison else _DISTRIBUTION_HEADERS
        missing = [h for h in required if not any(h in cell.lower() for cell in header)]
        if missing:
            if comparison and "mean" in missing:
                raise ReportError(
                    f"the §7 comparison table with header {header} publishes no "
                    f"wall-clock column at all ({', '.join(missing)} missing) — an arm "
                    "that measured nothing renders "
                    f"`{NOT_MEASURED}`, it does not delete the column and take the "
                    "spread requirement with it (PRD #233 acceptance criterion 4)"
                )
            raise ReportError(
                f"table with header {header} publishes a wall-clock mean with no "
                f"{', '.join(missing)} beside it — the population is bimodal, so a "
                "mean describes neither half and a reader takes it for a typical "
                "cycle (PRD #6 acceptance criterion 9)"
            )



FAILURE_WORDING = (
    "rtdd does NOT clearly beat the naive path heuristic — per the pre-registered "
    "criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this "
    "must be stated in the README"
)
"""The pre-registered failure sentence, used verbatim so it cannot be softened."""

SUCCESS_WORDING = "rtdd beats the naive path heuristic"

STATIC_FAILURE_WORDING = (
    "the static tier does NOT beat the naive path heuristic — per the pre-registration "
    "in docs/specs/2026-09-05-multi-language.md §7 it is not worth shipping as a "
    "distinct tier on this evidence and the README says so"
)
"""Spec §7's pre-registered failure sentence, used verbatim so it cannot be softened.

The rule is the one `outcomes_test.go`'s
`TestREADMEStaticVerdictMatchesTheCommittedSummaries` applies to `README.md`, and it is
applied here to the per-repo artifact so a reader of one `summary.md` is told the result
of the comparison its own by-variant table contains.
"""

STATIC_SUCCESS_WORDING = (
    "the static tier beats the naive path heuristic at comparable or better selected "
    "duration — per the pre-registration in docs/specs/2026-09-05-multi-language.md §7 "
    "it carries its weight as a distinct tier"
)


def fmt(r: dict | None) -> str:
    """A `Ratio` dict as `"0.933 (14/15)"`. A number is never printed without its denominator."""
    if r is None:
        return "n/a"
    v = r.get("value")
    num, den = r.get("num", 0), r.get("den", 0)
    if v is None:
        return f"n/a ({num:g}/{den:g})"
    return f"{v:.3f} ({num:g}/{den:g})"


def _num(v: float | None) -> str:
    return "n/a" if v is None else f"{v:.3f}"


def primary_variant(variants: Sequence[str]) -> str:
    """The variant the headline table and the verdict are computed over.

    `natural` when it was run: it is the real population, and a verdict taken from
    `probe` would be a verdict taken from an upper bound.
    """
    if "natural" in variants:
        return "natural"
    return variants[0] if variants else ""


def isolation_table(output) -> dict:
    """Subset runs that did not reproduce what the full suite saw.

    Deliberately outside :func:`wallclock_table`: a violation is a correctness
    finding, not a timing number, so the CI wall-clock refusal must not hide it.
    """
    rows: dict[str, dict] = {}
    for w in output.wallclocks:
        row = rows.setdefault(w.strategy, {"sampled": 0, "isolation_violations": 0})
        row["sampled"] += 1
        row["isolation_violations"] += 1 if w.isolation_violation else 0
    return {
        "sampled": sum(r["sampled"] for r in rows.values()),
        "isolation_violations": sum(r["isolation_violations"] for r in rows.values()),
        "rows": dict(sorted(rows.items())),
    }


def wallclock_table(output, hw: Hardware, *, wallclock_enabled: bool = True) -> dict:
    """The three wall-clock columns, or the refusal that replaced them.

    Two distinct refusals share this table, and a reader must not confuse them:
    CI detection is the library's own guard (spec §10), while ``wallclock_enabled``
    is an operator's `--no-wallclock` — typically because the run shares a
    contended machine and a timing taken there would be inflated and, thanks to
    the durable cache, silently reused forever after.
    """
    if not hw.wallclock_allowed():
        return {"suppressed": True, "reason": f"CI detected via {hw.ci}", "rows": {}}
    if not wallclock_enabled:
        return {
            "suppressed": True,
            "reason": (
                "withheld by operator (--no-wallclock) — measured on a contended "
                "workstation, timings would be inflated and, once cached, reused "
                "forever; re-run on a quiet box to publish them"
            ),
            "rows": {},
        }
    rows: dict[str, dict] = {}
    samples: dict[str, dict[str, list[int]]] = {}
    for w in output.wallclocks:
        row = rows.setdefault(
            w.strategy,
            {
                "n": 0,
                "full_uninstrumented_ms": 0,
                "subset_instrumented_ms": 0,
                "subset_uninstrumented_ms": 0,
                "isolation_violations": 0,
            },
        )
        seen = samples.setdefault(w.strategy, {c: [] for c in WALLCLOCK_COLUMNS})
        row["n"] += 1
        for column in WALLCLOCK_COLUMNS:
            value = getattr(w, column) or 0
            row[column] += value
            seen[column].append(value)
        row["isolation_violations"] += 1 if w.isolation_violation else 0
    for strategy, row in rows.items():
        n = max(row["n"], 1)
        for column in WALLCLOCK_COLUMNS:
            # The mean stays — it is the one number that answers "what did the
            # whole sample cost" — but it is never emitted alone. A bimodal
            # population (a cycle that selects nothing against one that selects
            # the hub) has a mean no cycle produced, so the spread ships beside it.
            row[f"mean_{column}"] = round(row[column] / n)
            row[f"p50_{column}"] = metrics.percentile(samples[strategy][column], 0.5)
            row[f"p90_{column}"] = metrics.percentile(samples[strategy][column], 0.9)
            row[f"worst_{column}"] = metrics.percentile(samples[strategy][column], 1.0)
    return {"suppressed": False, "reason": "", "rows": dict(sorted(rows.items()))}


def build_summary(
    output, strategy_ids: Sequence[str], hw: Hardware, *, wallclock_enabled: bool = True
) -> dict:
    """Every published number for one repo, in the shape `summary.json` is written from."""
    repo_id = metrics.assert_single_repo([*output.commits, *output.strategies])
    variants = sorted({c.variant for c in output.commits})
    per_variant: dict[str, dict] = {}
    for variant in variants:
        commits = [c for c in output.commits if c.variant == variant]
        sels = [s for s in output.strategies if s.variant == variant]
        per_variant[variant] = {sid: metrics.summarise(commits, sels, sid) for sid in strategy_ids}

    primary = primary_variant(variants)
    head_commits = [c for c in output.commits if c.variant == primary]
    head_uncovered = [u for u in output.uncovered if u.variant == primary]
    return {
        "repo_id": repo_id,
        "n_commits": len(output.commits),
        "variants": variants,
        "primary_variant": primary,
        "skipped": output.skipped,
        "strategies": per_variant.get(
            primary, {sid: metrics.summarise([], [], sid) for sid in strategy_ids}
        ),
        "by_variant": per_variant,
        "false_signal": falsesignal.summarise(head_uncovered, len(head_commits)),
        "rtdd_run_errors": len(getattr(output, "rtdd_run_errors", ())),
        "isolation": isolation_table(output),
        "wallclock": wallclock_table(output, hw, wallclock_enabled=wallclock_enabled),
    }


def _compare(a: dict, p: dict, arm: str, success: str, failure: str) -> str | None:
    """The pre-registered comparison of one arm against `path`, or None if unscored.

    One body, two verdicts. `rtdd` and `static` were each pre-registered against the
    same baseline on the same criterion — strictly better change-level recall bought at
    no more of the suite's duration — and the two are computed here rather than twice,
    so a change to the rule cannot reach one verdict and miss the other. A population
    where either arm has no detecting commit scores nothing and returns None: that is
    `not computable`, which is not a loss.
    """
    ar = a["change_level_recall"]["value"]
    pr = p["change_level_recall"]["value"]
    if ar is None or pr is None:
        return None
    ad = a["selected_duration_fraction"]["value"]
    pd = p["selected_duration_fraction"]["value"]
    # "Clearly beat ... at equal or better selected-duration fraction": strictly
    # better recall, and no more milliseconds spent buying it.
    beats = ar > pr and (ad is None or pd is None or ad <= pd)
    return (
        f"change-level recall {arm}={ar:.3f} vs path heuristic={pr:.3f}; "
        f"selected-duration fraction {arm}={_num(ad)} vs path={_num(pd)} — "
        f"{success if beats else failure}"
    )


def _primary_verdict(
    summary: dict, arm: str, prefix: str, missing: str, success: str, failure: str
) -> str:
    a = summary["strategies"].get(arm)
    p = summary["strategies"].get("path")
    if not a or not p:
        return f"{prefix}: not computable ({missing})"
    body = _compare(a, p, arm, success, failure)
    if body is None:
        return f"{prefix}: not computable (no detecting commits)"
    return f"{prefix}: {body}"


def _secondary_verdicts(
    summary: dict, arm: str, prefix: str, success: str, failure: str
) -> list[str]:
    primary = summary.get("primary_variant")
    out: list[str] = []
    for variant, per in sorted(summary.get("by_variant", {}).items()):
        if variant == primary:
            continue
        a, p = per.get(arm), per.get("path")
        if not a or not p:
            continue
        bound = (
            ", upper bound — map seeded at the child commit, never pooled with "
            "`natural`"
            if variant == "probe"
            else ""
        )
        body = _compare(a, p, arm, success, failure)
        if body:
            out.append(f"{prefix} ({variant}{bound}): {body}")
    return out


def verdict_line(summary: dict) -> str:
    """The pre-registered rtdd-vs-path comparison, printed win or lose."""
    return _primary_verdict(
        summary,
        "rtdd",
        "verdict",
        "rtdd or path heuristic missing from this run",
        SUCCESS_WORDING,
        FAILURE_WORDING,
    )


def secondary_verdict_lines(summary: dict) -> list[str]:
    """The same comparison on every non-primary variant, each labelled.

    Real commits are pushed green, so a `natural` population can detect nothing at
    all and leave the pre-registered comparison reading `not computable` — a
    summary that gates nothing. `probe` exists in the plan for exactly that case.
    It never becomes the verdict: it seeds every map-based strategy at the child
    commit and is therefore an upper bound, so it is printed under its own label,
    beside the primary line and never in place of it.
    """
    return _secondary_verdicts(summary, "rtdd", "verdict", SUCCESS_WORDING, FAILURE_WORDING)


def static_verdict_line(summary: dict) -> str:
    """Spec §7's static-vs-path comparison for the primary variant, win or lose.

    The `TS` tier was pre-registered against the same `path` baseline the map was, and
    until #366 the result of that comparison appeared only in `README.md`: a reader of
    one repo's `summary.md` saw `static` and `path` tie in the by-variant table and was
    told nothing about it. The per-repo artifact now states its own result, by the same
    rule `outcomes_test.go` derives the README's claim from the committed summaries.
    """
    return _primary_verdict(
        summary,
        "static",
        "static verdict",
        "the static arm or path heuristic is missing from this run",
        STATIC_SUCCESS_WORDING,
        STATIC_FAILURE_WORDING,
    )


def static_secondary_verdict_lines(summary: dict) -> list[str]:
    """Spec §7's comparison on every non-primary variant, each labelled.

    `probe` is the only population in the published corpus with any ground truth, so it
    is the population the kill condition actually turns on — and it is still an upper
    bound, so it is labelled as one on its own line rather than promoted to the primary
    verdict.
    """
    return _secondary_verdicts(
        summary, "static", "static verdict", STATIC_SUCCESS_WORDING, STATIC_FAILURE_WORDING
    )


def _headline_rows(summary: dict) -> list[str]:
    lines = [
        "| strategy | cycles | detecting | change recall | test recall (micro) "
        "| test recall (macro) | selection ratio | selected duration | escalation |",
        "|---|---|---|---|---|---|---|---|---|",
    ]
    for sid, s in summary["strategies"].items():
        lines.append(
            f"| {sid} | {s['cycles']} | {s['detecting_commits']} | "
            f"{fmt(s['change_level_recall'])} | {fmt(s['test_level_recall_micro'])} | "
            f"{fmt(s['test_level_recall_macro'])} | {fmt(s['selection_ratio'])} | "
            f"{fmt(s['selected_duration_fraction'])} | {fmt(s['escalation_rate'])} |"
        )
    return lines


def _comparison_wallclock_cells(summary: dict, arm: str) -> list[str]:
    """One arm's four wall-clock cells, or four explicit not-measured ones.

    Three states collapse to the same honest cell and none of them to a blank: the
    arm is derived and executed nothing, the run was on a CI runner, or the operator
    withheld timings. In all three no number exists, so none is printed — not one
    synthesised from `durations_ms`, which is another execution's per-test time, and
    not one borrowed from an arm that really ran.
    """
    wc = summary.get("wallclock") or {}
    row = (wc.get("rows") or {}).get(arm)
    if wc.get("suppressed") or not row:
        return [NOT_MEASURED] * (1 + len(_DISTRIBUTION_HEADERS))
    return [
        _ms(row.get(f"{stat}_{COMPARISON_WALLCLOCK_COLUMN}"))
        for stat in ("mean", *_DISTRIBUTION_HEADERS)
    ]


def comparison_table(summary: dict) -> list[str]:
    """The §7 evidence table: `static` against `rtdd`, `path` and `full`.

    Three metrics per arm — change-level recall, selection ratio, selected-duration
    fraction (PRD #233 AC3) — and the measured wall-clock beside them, mean and spread
    together so :func:`assert_distribution_beside_mean` covers these rows like any
    other. Arms absent from this run are absent from the table; nothing is imputed.
    """
    arms = [a for a in COMPARISON_ARMS if a in summary.get("strategies", {})]
    label = WALLCLOCK_COLUMN_LABELS[COMPARISON_WALLCLOCK_COLUMN]
    stats = " | ".join((f"mean {label}", *_DISTRIBUTION_HEADERS))
    metrics_header = " | ".join(name for _, name in COMPARISON_METRICS)
    cells = 1 + len(COMPARISON_METRICS) + 1 + len(_DISTRIBUTION_HEADERS)
    lines = [
        f"| {COMPARISON_HEADER_KEY} | {metrics_header} | {stats} |",
        "|" + "---|" * cells,
    ]
    unmeasured: list[str] = []
    for arm in arms:
        s = summary["strategies"][arm]
        scores = " | ".join(fmt(s.get(key)) for key, _ in COMPARISON_METRICS)
        wall = _comparison_wallclock_cells(summary, arm)
        if wall[0] == NOT_MEASURED:
            unmeasured.append(arm)
        lines.append(f"| `{arm}` | {scores} | {' | '.join(wall)} |")
    if unmeasured:
        lines.append("")
        lines.append(
            "`"
            + "`, `".join(unmeasured)
            + "` carry no wall-clock record in this run — a derived arm **executed "
            "nothing** at all, and any arm can simply have gone unsampled. Nothing was "
            f"invented to fill the gap: the cells read `{NOT_MEASURED}` rather than a "
            "blank that would read as zero, a figure synthesised from `durations_ms` "
            "(another execution's per-test time), or a row borrowed from an arm that "
            "really ran. The cost those arms do publish is the **selected duration** "
            "column, a ratio of the same commit's own recorded per-test durations and "
            "therefore independent of the machine."
        )
    return lines


def static_model_disclosure() -> list[str]:
    """The `test_for` templates the static arm models with, and how to read them.

    :data:`derive.STATIC_TEST_FOR` calls itself a PUBLISHED input rather than an
    implementation detail, and it is one: change a template and `static`'s selection
    ratio, its selected-duration fraction and therefore the kill-condition verdict all
    move. Rendering it here is what makes that claim true — the templates are read from
    `derive` itself, never re-typed, so the number and its most load-bearing input are
    auditable side by side and cannot drift.
    """
    lines = [
        "`static` models the `TS` tier (spec §4.1) over this corpus and is **derived** "
        "from the records above, never re-run. Level 1 is `test_for` correspondence, "
        "resolved against the test files each commit collected, first match wins:",
        "",
    ]
    lines += [f"- `{tmpl}`" for tmpl in derive.STATIC_TEST_FOR]
    lines.append("")
    lines.append(
        f"Level 2 is the committed `{derive.LEVEL2_SOURCE}` selection — that baseline "
        "measures exactly the transitive-import question level 2 asks, and it ran on "
        "every replayed commit. Level 3 (path proximity) orders and never admits, so it "
        "cannot change the selected set and none of the metrics above depend on it. Two "
        f"consequences follow: `static ⊇ {derive.LEVEL2_SOURCE}` **by construction**, so "
        "beating that baseline is arithmetic rather than a finding, which is why the "
        "pre-registered comparison is against `path`; and `adapters/python.yaml` "
        "declares no `test_for`, so the adapter modelled here **does not ship** — the "
        "row answers what the static tier WOULD have selected on this corpus, which is "
        "the question §7 pre-registers."
    )
    return lines


def render_markdown(summary: dict, cfg: RunConfig, hw: Hardware) -> str:
    """The published per-repo table. One repo, one primary variant, one verdict line."""
    primary = summary["primary_variant"] or "n/a"
    lines: list[str] = []
    lines.append(f"# Axis 2 — real-commit replay: `{summary['repo_id']}`")
    lines.append("")
    lines.append(
        f"**Hardware:** {hw.cpu_model}, {hw.cpu_count} cores, "
        f"{hw.mem_total_kb // 1024} MiB, {hw.platform}, Python {hw.python_version} "
        f"· fingerprint `{hw.fingerprint()}`"
    )
    lines.append(
        f"**Binary:** {cfg.rtdd_version} · **corpus digest:** `{cfg.corpus_digest[:12]}` "
        f"· **config digest:** `{cfg.digest()[:12]}`"
    )
    lines.append("**Tools:** " + ", ".join(f"{k} {v}" for k, v in cfg.tool_versions))
    lines.append(
        f"**Replayed commits:** {summary['n_commits']} "
        f"(skipped: {len(summary['skipped'])}) · shipped defaults only, no tuning flags"
    )
    lines.append("")
    lines.append(verdict_line(summary))
    for extra in secondary_verdict_lines(summary):
        lines.append("")
        lines.append(extra)
    lines.append("")
    lines.append(f"## Per-strategy — `{primary}`, all strata pooled within this repo")
    lines.append("")
    lines += _headline_rows(summary)
    if "static" in summary.get("strategies", {}):
        lines.append("")
        lines.append("## The static arm")
        lines.append("")
        lines.append(
            "Spec §7 asks what the `TS` static tier is worth against the corpus where "
            "the coverage-derived answer is already known. `static` is scored here "
            "against `rtdd`, the naive `path` baseline and the `full` ceiling, on the "
            "three metrics the pre-registration names."
        )
        lines.append("")
        lines.append(static_verdict_line(summary))
        for extra in static_secondary_verdict_lines(summary):
            lines.append("")
            lines.append(extra)
        lines.append("")
        lines += comparison_table(summary)
        lines.append("")
        lines += static_model_disclosure()
    lines.append("")
    lines.append(
        "## Stratified by |F_full| — the `|F_full| == 1` stratum is where selection "
        "safety is genuinely under test"
    )
    lines.append("")
    lines.append("| strategy | stratum | n | change recall | test recall (micro) |")
    lines.append("|---|---|---|---|---|")
    for sid, s in summary["strategies"].items():
        for stratum, v in s["strata"].items():
            lines.append(
                f"| {sid} | {stratum} | {v['n']} | {fmt(v['change_level_recall'])} | "
                f"{fmt(v['test_level_recall_micro'])} |"
            )
    lines.append("")
    lines.append("## Uncovered-report false signal")
    fs = summary["false_signal"]
    lines.append("")
    lines.append(f"- fired on {fs['fired']} of {fs['cycles']} cycles ({fmt(fs['fire_rate'])})")
    lines.append(f"- change-level false-signal rate: {fmt(fs['change_false_signal_rate'])}")
    lines.append(f"- line-level false-signal rate: {fmt(fs['line_false_signal_rate'])}")
    lines.append(
        f"- `rtdd run` refused on {summary.get('rtdd_run_errors', 0)} of {fs['cycles']} "
        "cycles — the shipped "
        "binary exits 2 rather than execute a map that names a test the tree no longer "
        "collects, and those cycles have no uncovered report"
    )
    lines.append("")
    lines.append("## Isolation")
    iso = summary["isolation"]
    lines.append("")
    lines.append(
        "A subset run that does not reproduce `F_full ∩ selected` is a finding, not a "
        "timing number, so it is published here whether or not wall-clock was permitted."
    )
    lines.append("")
    lines.append(
        f"- isolation violations: {iso['isolation_violations']} of {iso['sampled']} "
        f"really-executed subset runs"
    )
    for sid, row in iso["rows"].items():
        lines.append(f"  - `{sid}`: {row['isolation_violations']} of {row['sampled']}")
    lines.append("")
    lines.append("## Wall-clock")
    wc = summary["wallclock"]
    lines.append("")
    if wc["suppressed"]:
        note = (
            " Spec §10 permits wall-clock only from disclosed hardware, never from "
            "CI runners."
            if hw.ci
            else ""
        )
        lines.append(f"Suppressed: {wc['reason']}.{note}")
    else:
        lines += _wallclock_header()
        lines += _wallclock_rows(wc)
        lines.append("")
        lines.append(
            "One row per measurement rather than one cell: the population is bimodal — a "
            "cycle whose strategy selected nothing costs almost nothing, a cycle that "
            "selected the hub costs nearly a full run — so the mean sits between two modes "
            "and describes neither. `p50`, `p90` and `worst` are nearest-rank over the "
            "per-cycle samples in `commits.jsonl`, so each is a cycle that really ran. "
            "Isolation violations are per strategy and published above, under `## Isolation`."
        )
        lines.append("")
        lines.append(
            "A strategy that carries `Selection.exec_args` — `xdist` is the only one in the "
            "shipped set — runs **both** subset columns with those flags (`pytest -n auto`); "
            "per-test coverage contexts survive the parallel instrumented run, so that column "
            "is not silently serial either. The `full uninstrumented` column is always the "
            "serial full suite, which is what makes the two directly comparable."
        )
    lines.append("")
    lines.append("## By variant")
    lines.append("")
    lines.append(
        "`probe` seeds every map-based strategy at the child commit and is therefore an "
        "**upper bound** on their selection quality; it gets its own table and is never "
        "pooled with `natural`."
    )
    for variant, table in summary["by_variant"].items():
        lines.append("")
        lines.append(f"### `{variant}`")
        lines.append("")
        lines.append(
            "| strategy | cycles | change recall | test recall (micro) | selection ratio "
            "| selected duration | bound |"
        )
        lines.append("|---|---|---|---|---|---|---|")
        for sid, s in table.items():
            upper = variant == "probe" and sid in MAP_BASED_STRATEGIES
            bound = UPPER_BOUND_LABEL if upper else "—"
            lines.append(
                f"| {sid} | {s['cycles']} | {fmt(s['change_level_recall'])} | "
                f"{fmt(s['test_level_recall_micro'])} | {fmt(s['selection_ratio'])} | "
                f"{fmt(s['selected_duration_fraction'])} | {bound} |"
            )
    lines.append("")
    text = "\n".join(lines) + "\n"
    # The renderer audits its own output, so the guarantee holds for whatever
    # render_markdown grows into rather than only for the tables it emits today.
    assert_distribution_beside_mean(text)
    return text


def _wallclock_header() -> list[str]:
    """The wall-clock table's header, built from the columns it must publish.

    Assembled rather than typed out so a stat cannot be dropped from the header
    while the rows still carry it, or the reverse.
    """
    stats = " | ".join(("mean", *_DISTRIBUTION_HEADERS))
    cells = 4 + len(_DISTRIBUTION_HEADERS)
    return [f"| strategy | measurement | n | {stats} |", "|" + "---|" * cells]


def _wallclock_rows(wc: dict) -> list[str]:
    """One markdown row per (strategy, measurement), mean and spread together."""
    out: list[str] = []
    for sid, row in wc["rows"].items():
        for column in WALLCLOCK_COLUMNS:
            cells = " | ".join(
                _ms(row.get(f"{stat}_{column}")) for stat in ("mean", "p50", "p90", "worst")
            )
            out.append(f"| {sid} | {WALLCLOCK_COLUMN_LABELS[column]} | {row['n']} | {cells} |")
    return out


def _ms(value: object) -> str:
    """A millisecond cell. `0` is a measurement — only a missing sample is `n/a`."""
    return "n/a" if value is None else f"{value} ms"


def render_aggregate(summaries: Sequence[dict]) -> str:
    """The one cross-repo table, duration-weighted and labelled as such."""
    lines = [
        "# Axis 2 — duration-weighted aggregate",
        "",
        "Per spec §10 recall is **never pooled** across repos. The rows below weight each "
        "repo by its total suite duration and exist only to give a single ordering; the "
        "per-repo tables are the result.",
        "",
        "| strategy | duration-weighted selected fraction | repos |",
        "|---|---|---|",
    ]
    strategies = sorted({sid for s in summaries for sid in s["strategies"]})
    for sid in strategies:
        mine = [s for s in summaries if sid in s["strategies"]]
        num = sum(s["strategies"][sid]["selected_duration_fraction"]["num"] for s in mine)
        den = sum(s["strategies"][sid]["selected_duration_fraction"]["den"] for s in mine)
        val = "n/a" if den == 0 else f"{num / den:.3f}"
        lines.append(f"| {sid} | {val} | {len(mine)} |")
    return "\n".join(lines) + "\n"


def write_results(
    out_dir: pathlib.Path,
    output,
    cfg: RunConfig,
    hw: Hardware,
    strategy_ids: Sequence[str],
    drift: DriftCurve | None = None,
    *,
    wallclock_enabled: bool = True,
) -> dict:
    """Write one repo's results directory and return the summary that was written."""
    out_dir.mkdir(parents=True, exist_ok=True)
    records = [*output.commits, *output.strategies, *output.wallclocks, *output.uncovered]
    (out_dir / "commits.jsonl").write_text("".join(to_jsonl_lines(records)), encoding="utf-8")
    summary = build_summary(output, strategy_ids, hw, wallclock_enabled=wallclock_enabled)
    (out_dir / "summary.json").write_text(canonical_json(summary), encoding="utf-8")
    (out_dir / "summary.md").write_text(render_markdown(summary, cfg, hw), encoding="utf-8")
    (out_dir / "config.json").write_text(
        canonical_json(
            {
                "config": cfg.to_dict(),
                "config_digest": cfg.digest(),
                "hardware": hw.to_dict(),
            }
        ),
        encoding="utf-8",
    )
    if drift is not None:
        (out_dir / "drift.json").write_text(canonical_json(drift.to_dict()), encoding="utf-8")
    return summary


def write_aggregate(results_root: pathlib.Path, summaries: Sequence[dict]) -> pathlib.Path:
    """Write the sole cross-repo file, `aggregate.md`, and return its path."""
    results_root.mkdir(parents=True, exist_ok=True)
    path = results_root / "aggregate.md"
    path.write_text(render_aggregate(summaries), encoding="utf-8")
    return path
