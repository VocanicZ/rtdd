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
"""

from __future__ import annotations

import pathlib
from collections.abc import Sequence

from replay import falsesignal, metrics
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

FAILURE_WORDING = (
    "rtdd does NOT clearly beat the naive path heuristic — per the pre-registered "
    "criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this "
    "must be stated in the README"
)
"""The pre-registered failure sentence, used verbatim so it cannot be softened."""

SUCCESS_WORDING = "rtdd beats the naive path heuristic"


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


def wallclock_table(output, hw: Hardware) -> dict:
    """The three wall-clock columns, or the refusal that replaced them under CI."""
    if not hw.wallclock_allowed():
        return {"suppressed": True, "reason": f"CI detected via {hw.ci}", "rows": {}}
    rows: dict[str, dict] = {}
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
        row["n"] += 1
        row["full_uninstrumented_ms"] += w.full_uninstrumented_ms or 0
        row["subset_instrumented_ms"] += w.subset_instrumented_ms or 0
        row["subset_uninstrumented_ms"] += w.subset_uninstrumented_ms or 0
        row["isolation_violations"] += 1 if w.isolation_violation else 0
    for row in rows.values():
        n = max(row["n"], 1)
        row["mean_full_uninstrumented_ms"] = round(row["full_uninstrumented_ms"] / n)
        row["mean_subset_instrumented_ms"] = round(row["subset_instrumented_ms"] / n)
        row["mean_subset_uninstrumented_ms"] = round(row["subset_uninstrumented_ms"] / n)
    return {"suppressed": False, "reason": "", "rows": dict(sorted(rows.items()))}


def build_summary(output, strategy_ids: Sequence[str], hw: Hardware) -> dict:
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
        "isolation": isolation_table(output),
        "wallclock": wallclock_table(output, hw),
    }


def verdict_line(summary: dict) -> str:
    """The pre-registered rtdd-vs-path comparison, printed win or lose."""
    r = summary["strategies"].get("rtdd")
    p = summary["strategies"].get("path")
    if not r or not p:
        return "verdict: not computable (rtdd or path heuristic missing from this run)"
    rr = r["change_level_recall"]["value"]
    pr = p["change_level_recall"]["value"]
    rd = r["selected_duration_fraction"]["value"]
    pd = p["selected_duration_fraction"]["value"]
    if rr is None or pr is None:
        return "verdict: not computable (no detecting commits)"
    # "Clearly beat ... at equal or better selected-duration fraction": strictly
    # better recall, and no more milliseconds spent buying it.
    beats = rr > pr and (rd is None or pd is None or rd <= pd)
    return (
        f"verdict: change-level recall rtdd={rr:.3f} vs path heuristic={pr:.3f}; "
        f"selected-duration fraction rtdd={_num(rd)} vs path={_num(pd)} — "
        f"{SUCCESS_WORDING if beats else FAILURE_WORDING}"
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
    lines.append("")
    lines.append(f"## Per-strategy — `{primary}`, all strata pooled within this repo")
    lines.append("")
    lines += _headline_rows(summary)
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
        lines.append(
            f"Suppressed: {wc['reason']}. Spec §10 permits wall-clock only from "
            f"disclosed hardware, never from CI runners."
        )
    else:
        lines.append(
            "| strategy | n | full uninstrumented | subset instrumented "
            "| subset uninstrumented | isolation violations |"
        )
        lines.append("|---|---|---|---|---|---|")
        for sid, row in wc["rows"].items():
            lines.append(
                f"| {sid} | {row['n']} | {row['mean_full_uninstrumented_ms']} ms | "
                f"{row['mean_subset_instrumented_ms'] or 'n/a'} ms | "
                f"{row['mean_subset_uninstrumented_ms']} ms | {row['isolation_violations']} |"
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
    return "\n".join(lines) + "\n"


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
) -> dict:
    """Write one repo's results directory and return the summary that was written."""
    out_dir.mkdir(parents=True, exist_ok=True)
    records = [*output.commits, *output.strategies, *output.wallclocks, *output.uncovered]
    (out_dir / "commits.jsonl").write_text("".join(to_jsonl_lines(records)), encoding="utf-8")
    summary = build_summary(output, strategy_ids, hw)
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
