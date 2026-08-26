"""Turn raw per-instance results into the published numbers.

Three rules are encoded here rather than trusted to discipline:

  1. **An arm cannot win by doing nothing.** Both regression rates are only ever
     produced on an :class:`ArmSummary` that also carries the resolution rate,
     the unapplied count and the harness-error count. TDAD's own regression win
     cost resolution (31% → 29%); a tool that prevents regressions by solving
     fewer problems has not won, and the shape of the output says so rather than
     leaving it to the reader to notice.
  2. **No cross-citation without a label.** TDAD's published figures are usable
     only if the local ``vanilla`` arm falls inside the pre-registered
     equivalence band around 6.08%. Otherwise the analysis emits
     ``HARNESS DIVERGENCE`` naming *both* numbers and ``TDAD_PUBLISHED`` is
     absent from the output entirely — the discrepancy is documented, not
     reconciled away.
  3. **The kill criterion is read, never chosen.** ``kill_criterion_met`` takes
     its floor from the signed pre-registration and evaluates it against M3's
     already-published Axis-2 stratified recall. It never recomputes that
     recall, and it never accepts a floor handed to it as a literal — passing a
     floor that disagrees with the signed file is a refusal, because choosing
     the bar after seeing the measurement is exactly what pre-registration
     exists to prevent.

A note on where M3's number comes from. Axis 2 publishes one
``bench/results/<repo_id>/summary.json`` per repo and the spec forbids pooling
recall across repos, so there is no single pooled figure to read. The criterion
is therefore evaluated per repo and met only if *every* repo clears the floor;
the value reported back is the worst of them. A repo that never produced a
``|F_full| == 1`` stratum carries no evidence either way and is skipped — but if
no repo produced one, the criterion is unevaluable and that is a refusal, not a
pass.
"""

from __future__ import annotations

import json
from collections.abc import Sequence
from dataclasses import asdict, dataclass
from pathlib import Path

import evaluate
from metrics import (
    InstanceResult,
    instance_level_regression_rate,
    regression_counts,
    resolution_rate,
    test_level_regression_rate,
    wilson_ci,
)
from preflight import preflight
from prereg import PreregError, load

TDAD_PUBLISHED: dict[str, float] = {"vanilla": 0.0608, "tdd": 0.0994, "tdad": 0.0182}
TDAD_CITATION = "arXiv:2603.17973, n=100 SWE-bench Verified, Qwen3-Coder 30B Q4_K_M"

#: The Axis-2 strategy whose recall the kill criterion is about, and the stratum
#: it is measured on: one failing test, no partial credit, no excuse for a miss.
KILL_STRATEGY = "rtdd"
KILL_STRATUM = "1"

#: `bench/results/` sub-directories that are not an Axis-2 repo result.
NON_REPO_RESULTS = frozenset({"swebench", ".cache"})

#: The seed status `providers.rtdd` records when the instrumented seed run
#: produced a usable map. Every other status leaves the arm running unseeded.
SEEDED = "seeded"

#: The comparisons the pre-registration is interested in, in reporting order.
COMPARISONS: tuple[tuple[str, str], ...] = (
    ("vanilla", "rtdd"),
    ("tdd", "rtdd_tdd"),
    ("tdad", "rtdd_tdd"),
    ("tdad", "rtdd"),
    ("vanilla", "tdd"),
)


@dataclass(frozen=True)
class ArmSummary:
    """One arm's whole scorecard. Resolution is a field, not an option."""

    arm: str
    n: int
    test_level: float
    test_ci: tuple[float, float]
    instance_level: float
    instance_ci: tuple[float, float]
    resolution: float
    p2p_failed: int
    p2p_total: int
    unapplied: int
    harness_errors: int


def summarize(results: Sequence[InstanceResult]) -> ArmSummary:
    """Fold one arm's per-instance results into the numbers it is published at.

    Every denominator is the whole sample: an instance whose patch never applied
    and an instance the harness lost both stay in, so an arm cannot look clean by
    producing nothing or by losing the evidence.
    """
    failed, total = regression_counts(results)
    instance_hits = sum(1 for r in results if r.p2p_failed)
    return ArmSummary(
        arm=results[0].arm if results else "?",
        n=len(results),
        test_level=test_level_regression_rate(results),
        test_ci=wilson_ci(failed, total),
        instance_level=instance_level_regression_rate(results),
        instance_ci=wilson_ci(instance_hits, len(results)),
        resolution=resolution_rate(results),
        p2p_failed=failed,
        p2p_total=total,
        unapplied=sum(1 for r in results if not r.patch_applied),
        harness_errors=sum(1 for r in results if r.patch_applied and not r.evaluated),
    )


def vanilla_reproduces(local: ArmSummary, k: float) -> tuple[bool, str]:
    """Does the local control land close enough to TDAD's published 6.08%?

    The band is ``k`` Wilson half-widths either side of the local rate, with
    ``k`` pre-registered. Outside it, the note is a banner: it names the local
    rate and the published rate together so neither can be quietly dropped from
    whatever table it ends up in.
    """
    published = TDAD_PUBLISHED["vanilla"]
    lo, hi = local.test_ci
    half = (hi - lo) / 2.0
    band = (local.test_level - k * half, local.test_level + k * half)
    if band[0] <= published <= band[1]:
        return (
            True,
            f"local vanilla {local.test_level:.4f} reproduces the published "
            f"{published:.4f} (band ±{k}×{half:.4f} = "
            f"[{band[0]:.4f}, {band[1]:.4f}])",
        )
    return (
        False,
        f"HARNESS DIVERGENCE: local vanilla {local.test_level:.4f} excludes the "
        f"published {published:.4f} (band ±{k}×{half:.4f} = "
        f"[{band[0]:.4f}, {band[1]:.4f}]). TDAD's published figures are removed "
        "from the comparison columns and every claim is stated in local-relative "
        "form only.",
    )


def compare(a: ArmSummary, b: ArmSummary) -> dict:
    """Delta between two arms, plus whether their intervals actually separate.

    A delta is never reported as a difference on the strength of the point
    estimates: ``verdict`` names a lower arm only when the two Wilson intervals
    are disjoint. Both resolution rates ride along, so a regression win bought by
    solving fewer problems is visible in the same row as the win.
    """
    disjoint = a.test_ci[0] > b.test_ci[1] or b.test_ci[0] > a.test_ci[1]
    if not disjoint:
        verdict = "no distinguishable difference"
    elif b.test_level < a.test_level:
        verdict = f"{b.arm} lower than {a.arm}"
    else:
        verdict = f"{a.arm} lower than {b.arm}"
    return {
        "a": a.arm,
        "b": b.arm,
        "delta": a.test_level - b.test_level,
        "a_ci": list(a.test_ci),
        "b_ci": list(b.test_ci),
        "a_resolution": a.resolution,
        "b_resolution": b.resolution,
        "ci_disjoint": disjoint,
        "verdict": verdict,
    }


def _axis2_stratum_recalls(repo_root: Path) -> dict[str, float]:
    """M3's published `|F_full| == 1` change-level recall, per repo.

    Read from the committed Axis-2 results and never recomputed here. Repos that
    reported no such stratum carry no evidence and are absent from the mapping.
    """
    results = repo_root / "bench" / "results"
    summaries = sorted(
        path
        for path in results.glob("*/summary.json")
        if path.parent.name not in NON_REPO_RESULTS
    )
    if not summaries:
        raise FileNotFoundError(
            f"{results}/<repo_id>/summary.json — Axis 2 must be published before "
            "the kill criterion can be evaluated"
        )
    out: dict[str, float] = {}
    for path in summaries:
        doc = json.loads(path.read_text(encoding="utf-8"))
        strategy = (doc.get("strategies") or {}).get(KILL_STRATEGY) or {}
        stratum = (strategy.get("strata") or {}).get(KILL_STRATUM)
        value = (stratum or {}).get("change_level_recall", {}).get("value")
        if value is not None:
            out[doc.get("repo_id", path.parent.name)] = float(value)
    return out


def kill_criterion_met(repo_root: Path, floor: float | None = None) -> tuple[bool, float]:
    """Evaluate the pre-registered kill criterion. Returns ``(met, actual)``.

    The floor comes from the signed pre-registration. ``floor`` exists only so a
    caller may state which value it believes it is checking; a value that
    disagrees with the signed file is refused rather than honoured.

    ``actual`` is the worst repo's recall, because Axis 2 forbids pooling across
    repos: the criterion is met only when every repo clears the floor.
    """
    pr = load(repo_root / "bench" / "PREREGISTRATION.md")
    signed = float(pr.fields["stratified_recall_floor"])
    if floor is not None and float(floor) != signed:
        raise PreregError(
            f"stratified_recall_floor is {signed} in {pr.path}; refusing the "
            f"floor {floor} passed from the call site — the kill criterion is "
            "read from the signed pre-registration, never chosen at analysis time"
        )

    recalls = _axis2_stratum_recalls(repo_root)
    if not recalls:
        raise ValueError(
            "no Axis-2 repo published a |F_full| == 1 stratum for strategy "
            f"{KILL_STRATEGY!r} — the kill criterion is unevaluable, which is a "
            "refusal rather than a pass"
        )
    worst = min(recalls.values())
    return (worst >= signed, worst)


def seed_failures(repo_root: Path, arm: str) -> int:
    """Instances of this arm whose seed run did not produce a usable map.

    A failed seed is published rather than dropped: the instance still ran, with
    an unseeded map that selects nothing, and hiding that would flatter the arm.
    """
    raw = evaluate.results_dir(repo_root) / "raw" / arm
    if not raw.exists():
        return 0
    failed = 0
    for path in sorted(raw.glob("*.json")):
        record = json.loads(path.read_text(encoding="utf-8"))
        status = record.get("seed_status")
        if status is not None and status != SEEDED:
            failed += 1
    return failed


def _jsonable(value):
    """Render a pre-registered value as JSON.

    YAML parses ``signed_at: 2026-08-27`` into a ``date``, which JSON has no
    type for. Stringifying it here rather than at write time keeps the returned
    dict byte-identical to the file that was written, so a caller reading either
    one is reading the same thing.
    """
    if isinstance(value, (str, int, float, bool)) or value is None:
        return value
    if isinstance(value, (list, tuple)):
        return [_jsonable(v) for v in value]
    if isinstance(value, dict):
        return {str(k): _jsonable(v) for k, v in value.items()}
    return str(value)


def analyze(repo_root: Path, rows: dict[str, dict] | None = None) -> dict:
    """Produce and write ``bench/results/swebench/summary.json``.

    ``rows`` is the SWE-bench dataset, injectable so the suite runs against
    synthetic summaries with no network and no benchmark run behind it.
    """
    pr = preflight(repo_root)
    k = float(pr.fields["vanilla_equivalence_k"])

    summaries: dict[str, ArmSummary] = {}
    for arm in pr.fields["arms"]:
        results = evaluate.collect(repo_root, arm, rows)
        if results:
            summaries[arm] = summarize(results)

    if "vanilla" not in summaries:
        raise ValueError(
            "the vanilla arm has no results — the equivalence band is what "
            "licenses every cross-citation, so there is nothing to publish "
            "without it"
        )

    reproduces, note = vanilla_reproduces(summaries["vanilla"], k)
    met, observed = kill_criterion_met(repo_root)

    out = {
        "prereg": _jsonable(dict(pr.fields)),
        "arms": {name: _jsonable(asdict(s)) for name, s in summaries.items()},
        "vanilla_equivalence": {"reproduces": reproduces, "note": note},
        # Absent, not merely unused: an outside-band vanilla arm means these
        # numbers may not appear in any column downstream reads.
        "tdad_published": TDAD_PUBLISHED if reproduces else None,
        "tdad_citation": TDAD_CITATION,
        "kill_criterion": {
            "floor": float(pr.fields["stratified_recall_floor"]),
            "observed_single_killer_recall": observed,
            "met": met,
        },
        "seed_failed": {name: seed_failures(repo_root, name) for name in summaries},
        "unapplied": {name: s.unapplied for name, s in summaries.items()},
        "harness_errors": {name: s.harness_errors for name, s in summaries.items()},
        "comparisons": [
            compare(summaries[a], summaries[b])
            for a, b in COMPARISONS
            if a in summaries and b in summaries
        ],
    }
    dest = evaluate.results_dir(repo_root) / "summary.json"
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(out, indent=2), encoding="utf-8")
    return out


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    result = analyze(root)
    print(result["vanilla_equivalence"]["note"])
    print(
        "kill criterion:",
        "MET" if result["kill_criterion"]["met"] else "NOT MET",
        f"({result['kill_criterion']['observed_single_killer_recall']:.4f} vs floor "
        f"{result['kill_criterion']['floor']:.4f})",
    )
    for arm, s in result["arms"].items():
        print(
            f"  {arm}: test-level {s['test_level']:.4f} "
            f"instance-level {s['instance_level']:.4f} "
            f"resolution {s['resolution']:.4f} "
            f"unapplied {s['unapplied']} harness-errors {s['harness_errors']}"
        )
    for c in result["comparisons"]:
        print(f"  {c['a']} vs {c['b']}: delta {c['delta']:+.4f}  {c['verdict']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
