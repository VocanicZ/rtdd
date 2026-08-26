import json

import pytest

# `metrics.test_level_regression_rate` is a metric, not a test; importing it by
# name would make pytest collect it as one. The module is imported whole and the
# rates are called through it so the public interface keeps its intended names.
import metrics
from metrics import (
    InstanceResult,
    from_report,
    instance_level_regression_rate,
    regression_counts,
    resolution_rate,
    wilson_ci,
)


ROW = {
    "instance_id": "django__django-1",
    "PASS_TO_PASS": json.dumps(["p1", "p2", "p3", "p4"]),
    "FAIL_TO_PASS": json.dumps(["f1", "f2"]),
}


def report(p2p_fail, f2p_pass):
    return {
        "django__django-1": {
            "tests_status": {
                "PASS_TO_PASS": {
                    "success": [t for t in ["p1", "p2", "p3", "p4"] if t not in p2p_fail],
                    "failure": list(p2p_fail),
                },
                "FAIL_TO_PASS": {
                    "success": list(f2p_pass),
                    "failure": [t for t in ["f1", "f2"] if t not in f2p_pass],
                },
            }
        }
    }


def test_regression_is_a_p2p_failure():
    r = from_report("django__django-1", "rtdd", ROW, report(["p2"], ["f1", "f2"]), True)
    assert r.p2p_failed == ("p2",)
    assert r.p2p_total == 4


def test_regressions_are_read_from_nothing_but_the_p2p_failure_list():
    # A report whose FAIL_TO_PASS half is a disaster and whose prose fields
    # scream about breakage still yields zero regressions: only the
    # PASS_TO_PASS.failure list defines one.
    rep = report([], [])
    rep["django__django-1"]["resolved"] = False
    rep["django__django-1"]["notes"] = "the patch broke three unrelated tests"
    r = from_report("django__django-1", "rtdd", ROW, rep, True)
    assert r.p2p_failed == ()
    assert instance_level_regression_rate([r]) == 0.0


def test_denominator_comes_from_the_dataset_not_the_report():
    truncated = report(["p2"], ["f1"])
    truncated["django__django-1"]["tests_status"]["PASS_TO_PASS"]["success"] = []
    r = from_report("django__django-1", "rtdd", ROW, truncated, True)
    assert r.p2p_total == 4


def test_dataset_row_may_carry_plain_lists_as_well_as_json_strings():
    row = {
        "instance_id": "django__django-1",
        "PASS_TO_PASS": ["p1", "p2", "p3", "p4"],
        "FAIL_TO_PASS": ["f1", "f2"],
    }
    r = from_report("django__django-1", "rtdd", row, report(["p1"], ["f1", "f2"]), True)
    assert r.p2p_total == 4
    assert r.f2p_total == 2


def test_unapplied_patch_has_no_regressions_but_stays_in_the_denominator():
    r = from_report("django__django-1", "vanilla", ROW, {}, patch_applied=False)
    assert r.p2p_failed == ()
    assert r.p2p_total == 4
    assert r.evaluated is False
    assert resolution_rate([r]) == 0.0
    assert metrics.test_level_regression_rate([r]) == 0.0
    assert instance_level_regression_rate([r]) == 0.0


def test_a_crashed_evaluation_is_unevaluated_but_keeps_its_denominator():
    # The patch applied; the harness died before writing tests_status.
    r = from_report("django__django-1", "rtdd", ROW, {"django__django-1": {}}, True)
    assert r.evaluated is False
    assert r.p2p_failed == ()
    assert regression_counts([r]) == (0, 4)
    assert resolution_rate([r]) == 0.0


def test_a_do_nothing_arm_cannot_win():
    # Four instances, no patch at all: zero regressions AND zero resolutions.
    nothing = [from_report("django__django-1", "lazy", ROW, {}, False) for _ in range(4)]
    assert metrics.test_level_regression_rate(nothing) == 0.0
    assert resolution_rate(nothing) == 0.0
    assert regression_counts(nothing) == (0, 16)


def test_test_level_rate_pools_tests_not_instances():
    a = from_report("django__django-1", "x", ROW, report(["p1", "p2"], []), True)
    b = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    assert metrics.test_level_regression_rate([a, b]) == pytest.approx(2 / 8)
    assert instance_level_regression_rate([a, b]) == pytest.approx(0.5)


def test_instance_level_rate_counts_instances_with_at_least_one_regression():
    many = from_report("django__django-1", "x", ROW, report(["p1", "p2", "p3"], []), True)
    one = from_report("django__django-1", "x", ROW, report(["p4"], []), True)
    clean = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    results = [many, one, clean]
    assert instance_level_regression_rate(results) == pytest.approx(2 / 3)
    assert metrics.test_level_regression_rate(results) == pytest.approx(4 / 12)


def test_resolution_requires_all_f2p_and_no_p2p_failure():
    solved = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    broke = from_report("django__django-1", "x", ROW, report(["p1"], ["f1", "f2"]), True)
    partial = from_report("django__django-1", "x", ROW, report([], ["f1"]), True)
    assert resolution_rate([solved]) == 1.0
    assert resolution_rate([broke]) == 0.0
    assert resolution_rate([partial]) == 0.0


def test_resolution_rate_denominator_is_the_whole_sample():
    solved = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    skipped = from_report("django__django-1", "x", ROW, {}, False)
    assert resolution_rate([solved, skipped, skipped]) == pytest.approx(1 / 3)


def test_empty_sample_rates_are_zero():
    assert metrics.test_level_regression_rate([]) == 0.0
    assert instance_level_regression_rate([]) == 0.0
    assert resolution_rate([]) == 0.0
    assert regression_counts([]) == (0, 0)


def test_instance_result_is_frozen():
    r = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    assert isinstance(r, InstanceResult)
    with pytest.raises(Exception):
        r.p2p_total = 0  # type: ignore[misc]


def test_wilson_ci_brackets_the_point_estimate():
    lo, hi = wilson_ci(6, 100)
    assert lo < 0.06 < hi
    assert 0.0 <= lo and hi <= 1.0


def test_wilson_ci_matches_known_values():
    # Published Wilson score intervals at z=1.96.
    assert wilson_ci(6, 100) == pytest.approx((0.027786, 0.124770), abs=1e-6)
    assert wilson_ci(1, 10) == pytest.approx((0.017876, 0.404156), abs=1e-6)
    assert wilson_ci(15, 20) == pytest.approx((0.531295, 0.888140), abs=1e-6)
    assert wilson_ci(0, 20) == pytest.approx((0.0, 0.161130), abs=1e-6)


def test_wilson_ci_of_zero_successes_has_zero_lower_bound():
    lo, hi = wilson_ci(0, 100)
    assert lo == 0.0
    assert hi > 0.0


def test_wilson_ci_of_all_successes_has_a_unit_upper_bound():
    lo, hi = wilson_ci(100, 100)
    assert hi == 1.0
    assert lo < 1.0


def test_wilson_ci_of_zero_trials_is_degenerate_not_an_error():
    assert wilson_ci(0, 0) == (0.0, 0.0)


def test_wilson_ci_widens_as_z_grows():
    lo95, hi95 = wilson_ci(6, 100)
    lo99, hi99 = wilson_ci(6, 100, z=2.576)
    assert lo99 < lo95 and hi99 > hi95
