"""The mechanical definition of a regression.

A regression is a test id that appears in the SWE-bench evaluation report's
``tests_status.PASS_TO_PASS.failure`` list — a test that passed before the
agent's patch and fails after it. It is never inferred from prose, never from an
LLM judgement, and never from a diff. There is no other definition anywhere in
this harness; this module is it.

Two anti-gaming rules are encoded here rather than trusted to discipline:

  1. ``p2p_total`` comes from the dataset row, never from the report. A crashed
     or truncated evaluation therefore cannot shrink an arm's denominator and
     make it look clean by failing to produce output.
  2. An instance whose patch is empty or fails to apply contributes zero
     regressions AND zero resolutions and stays in both denominators, so an arm
     cannot lower its regression rate by producing nothing.

Two rates are reported because they answer different questions and the prior art
reports one of them:

  - the test-level rate — failing P2P tests over all P2P tests in the sample —
    which is what a maintainer feels as broken test volume;
  - the instance-level rate — instances with at least one regression over all
    instances — which is what TDAD's published 6.08% / 9.94% figures measure.

``resolution_rate`` matches SWE-bench's own resolution definition: every
FAIL_TO_PASS test passes and no PASS_TO_PASS test fails. It is printed beside
every regression rate so a do-nothing arm is visibly a do-nothing arm.
"""

from __future__ import annotations

import json
import math
from collections.abc import Sequence
from dataclasses import dataclass


@dataclass(frozen=True)
class InstanceResult:
    """One (instance, arm) cell of the results grid."""

    instance_id: str
    arm: str
    p2p_total: int  # from the DATASET, never from the report
    p2p_failed: tuple[str, ...]
    f2p_total: int
    f2p_passed: int
    patch_applied: bool
    evaluated: bool


def _ids(row: dict, key: str) -> list[str]:
    """SWE-bench rows carry these as JSON strings; some loaders hand back lists."""
    raw = row[key]
    return json.loads(raw) if isinstance(raw, str) else list(raw)


def from_report(
    instance_id: str,
    arm: str,
    dataset_row: dict,
    report: dict,
    patch_applied: bool,
) -> InstanceResult:
    """Fold one evaluation report into an :class:`InstanceResult`.

    The denominators come from ``dataset_row``. The report supplies only which
    tests failed, and only from ``tests_status.PASS_TO_PASS.failure``.
    """
    p2p_total = len(_ids(dataset_row, "PASS_TO_PASS"))
    f2p_total = len(_ids(dataset_row, "FAIL_TO_PASS"))

    entry = report.get(instance_id) if isinstance(report, dict) else None
    status = (entry or {}).get("tests_status") or {}
    evaluated = bool(status) and patch_applied

    if not evaluated:
        return InstanceResult(
            instance_id=instance_id,
            arm=arm,
            p2p_total=p2p_total,
            p2p_failed=(),
            f2p_total=f2p_total,
            f2p_passed=0,
            patch_applied=patch_applied,
            evaluated=False,
        )

    p2p_failed = tuple(status.get("PASS_TO_PASS", {}).get("failure", []))
    f2p_passed = len(status.get("FAIL_TO_PASS", {}).get("success", []))
    return InstanceResult(
        instance_id=instance_id,
        arm=arm,
        p2p_total=p2p_total,
        p2p_failed=p2p_failed,
        f2p_total=f2p_total,
        f2p_passed=f2p_passed,
        patch_applied=True,
        evaluated=True,
    )


def test_level_regression_rate(results: Sequence[InstanceResult]) -> float:
    """Failing P2P tests over all P2P tests the sample was supposed to keep green."""
    denom = sum(r.p2p_total for r in results)
    if denom == 0:
        return 0.0
    return sum(len(r.p2p_failed) for r in results) / denom


def instance_level_regression_rate(results: Sequence[InstanceResult]) -> float:
    """Instances carrying at least one regression, over all instances."""
    if not results:
        return 0.0
    return sum(1 for r in results if r.p2p_failed) / len(results)


def resolution_rate(results: Sequence[InstanceResult]) -> float:
    """SWE-bench resolution: all FAIL_TO_PASS pass and no PASS_TO_PASS fails.

    The denominator is every instance in the sample, so an unapplied patch or a
    crashed evaluation counts as unresolved rather than vanishing.
    """
    if not results:
        return 0.0
    solved = sum(
        1
        for r in results
        if r.evaluated and not r.p2p_failed and r.f2p_passed == r.f2p_total
    )
    return solved / len(results)


def regression_counts(results: Sequence[InstanceResult]) -> tuple[int, int]:
    """(failing P2P tests, total P2P tests) — the pair every CI is computed on."""
    return sum(len(r.p2p_failed) for r in results), sum(r.p2p_total for r in results)


def wilson_ci(successes: int, trials: int, z: float = 1.96) -> tuple[float, float]:
    """Wilson score interval for a binomial proportion.

    Chosen over the normal approximation because the rates here are small and
    the interval must stay inside [0, 1] at the boundaries: zero successes give
    an exact zero lower bound, all successes an exact unit upper bound, and zero
    trials a degenerate (0, 0) rather than a ZeroDivisionError.
    """
    if trials == 0:
        return (0.0, 0.0)
    p = successes / trials
    denom = 1 + z * z / trials
    centre = (p + z * z / (2 * trials)) / denom
    half = (z / denom) * math.sqrt(p * (1 - p) / trials + z * z / (4 * trials * trials))
    # At p == 0 and p == 1 the interval closes on the boundary exactly; pin those
    # ends rather than leaving floating-point dust either side of them.
    low = 0.0 if successes == 0 else max(0.0, centre - half)
    high = 1.0 if successes == trials else min(1.0, centre + half)
    return (low, high)
