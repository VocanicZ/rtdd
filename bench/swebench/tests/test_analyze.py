"""The analysis layer, held to the two guarantees it exists to enforce.

Nothing here runs a benchmark. Every test works from synthetic summaries or a
handful of fixture files on disk, because an analysis that can only be checked
by spending a hundred instances of inference is an analysis nobody checks.

The two guarantees under test:

  1. **An arm cannot win by doing nothing.** ``summarize`` refuses to produce a
     regression rate without the resolution rate, the unapplied count and the
     harness-error count beside it.
  2. **No cross-citation without a label.** A local vanilla arm outside the
     pre-registered band produces ``HARNESS DIVERGENCE`` naming both numbers,
     and strips TDAD's published figures from the output entirely.
"""

import json
import subprocess
from pathlib import Path

import pytest

import analyze
from analyze import (
    TDAD_PUBLISHED,
    ArmSummary,
    analyze as run_analysis,
    compare,
    kill_criterion_met,
    summarize,
    vanilla_reproduces,
)
from metrics import InstanceResult, wilson_ci
from prereg import PreregError

ARMS = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")
IDS = ["astropy__astropy-12907", "django__django-11039", "sympy__sympy-20049"]

ROWS = {
    "astropy__astropy-12907": {
        "PASS_TO_PASS": json.dumps(["p1", "p2", "p3", "p4"]),
        "FAIL_TO_PASS": json.dumps(["f1"]),
    },
    "django__django-11039": {
        "PASS_TO_PASS": json.dumps(["q1", "q2"]),
        "FAIL_TO_PASS": json.dumps(["g1", "g2"]),
    },
    "sympy__sympy-20049": {
        "PASS_TO_PASS": json.dumps(["r1", "r2", "r3"]),
        "FAIL_TO_PASS": json.dumps(["h1"]),
    },
}

SIGNED = """---
status: SIGNED
sample_size: 3
sample_seed: 20260826
instance_list_sha256: {sha}
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: {floor}
vanilla_equivalence_k: {k}
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def summary(arm, failed, total, n=100):
    """A synthetic :class:`ArmSummary` at a chosen test-level rate."""
    return ArmSummary(
        arm=arm,
        n=n,
        test_level=failed / total,
        test_ci=wilson_ci(failed, total),
        instance_level=0.0,
        instance_ci=(0.0, 0.0),
        resolution=0.30,
        p2p_failed=failed,
        p2p_total=total,
        unapplied=0,
        harness_errors=0,
    )


def result(instance_id, arm, p2p_failed=(), f2p_passed=None, applied=True, evaluated=True):
    p2p = json.loads(ROWS[instance_id]["PASS_TO_PASS"])
    f2p = json.loads(ROWS[instance_id]["FAIL_TO_PASS"])
    return InstanceResult(
        instance_id=instance_id,
        arm=arm,
        p2p_total=len(p2p),
        p2p_failed=tuple(p2p_failed),
        f2p_total=len(f2p),
        f2p_passed=len(f2p) if f2p_passed is None else f2p_passed,
        patch_applied=applied,
        evaluated=evaluated,
    )


# ---------------------------------------------------------------- summarize


def test_summarize_reports_resolution_beside_every_regression_rate():
    results = [
        result(IDS[0], "rtdd", p2p_failed=("p1",)),
        result(IDS[1], "rtdd"),
        result(IDS[2], "rtdd"),
    ]
    s = summarize(results)
    assert s.arm == "rtdd"
    assert s.n == 3
    assert s.p2p_failed == 1
    assert s.p2p_total == 9
    assert s.test_level == pytest.approx(1 / 9)
    assert s.instance_level == pytest.approx(1 / 3)
    # The do-nothing check: two of three instances resolved, and it is on the
    # same object as the rate, not somewhere a reader has to go and find it.
    assert s.resolution == pytest.approx(2 / 3)
    assert s.test_ci[0] < s.test_level < s.test_ci[1]
    assert s.instance_ci[0] < s.instance_level < s.instance_ci[1]


def test_summarize_counts_unapplied_patches_and_harness_errors_separately():
    results = [
        result(IDS[0], "tdad", applied=False, evaluated=False),
        result(IDS[1], "tdad", applied=True, evaluated=False),
        result(IDS[2], "tdad"),
    ]
    s = summarize(results)
    assert s.unapplied == 1
    assert s.harness_errors == 1
    # Neither vanishes from the denominators.
    assert s.n == 3
    assert s.p2p_total == 9
    assert s.resolution == pytest.approx(1 / 3)


def test_an_arm_that_produced_nothing_scores_zero_regressions_and_zero_resolution():
    results = [result(i, "vanilla", applied=False, evaluated=False) for i in IDS]
    s = summarize(results)
    assert s.test_level == 0.0
    assert s.resolution == 0.0
    assert s.unapplied == 3


# -------------------------------------------------------- equivalence band


def test_vanilla_at_the_published_rate_reproduces():
    ok, note = vanilla_reproduces(summary("vanilla", 608, 10000), k=2)
    assert ok
    assert "reproduces" in note


def test_vanilla_far_from_the_published_rate_diverges():
    ok, note = vanilla_reproduces(summary("vanilla", 2500, 10000), k=2)
    assert not ok
    assert "HARNESS DIVERGENCE" in note


def test_divergence_note_names_both_numbers():
    _, note = vanilla_reproduces(summary("vanilla", 2500, 10000), k=2)
    assert "0.0608" in note
    assert "0.2500" in note


# ------------------------------------------------------------------ compare


def test_compare_reports_disjoint_intervals():
    out = compare(summary("tdad", 182, 10000), summary("rtdd", 90, 10000))
    assert out["delta"] == pytest.approx(0.0182 - 0.0090)
    assert out["ci_disjoint"] is True


def test_compare_reports_overlapping_intervals_as_a_tie():
    out = compare(summary("tdad", 182, 10000), summary("rtdd", 175, 10000))
    assert out["ci_disjoint"] is False
    assert out["verdict"] == "no distinguishable difference"


def test_compare_names_the_lower_arm_only_when_the_intervals_are_disjoint():
    out = compare(summary("tdad", 182, 10000), summary("rtdd", 90, 10000))
    assert out["verdict"] == "rtdd lower than tdad"


def test_compare_carries_both_resolution_rates_so_a_do_nothing_win_is_visible():
    a = summary("tdad", 182, 10000)
    b = summary("rtdd", 90, 10000)
    out = compare(a, b)
    assert out["a_resolution"] == pytest.approx(a.resolution)
    assert out["b_resolution"] == pytest.approx(b.resolution)


# ----------------------------------------------------------- kill criterion


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True)


def make_repo(tmp_path: Path, *, floor="0.95", k="2", status="SIGNED") -> Path:
    """A repo root with a signed, tagged pre-registration over the fixture sample."""
    import hashlib

    (tmp_path / "bench" / "swebench").mkdir(parents=True)
    instances = "\n".join(IDS) + "\n"
    (tmp_path / "bench" / "swebench" / "instances.txt").write_text(instances, encoding="utf-8")
    sha = hashlib.sha256(instances.encode("utf-8")).hexdigest()
    text = SIGNED.format(sha=sha, floor=floor, k=k).replace("status: SIGNED", f"status: {status}")
    (tmp_path / "bench" / "PREREGISTRATION.md").write_text(text, encoding="utf-8")
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    git(tmp_path, "config", "user.email", "t@t")
    git(tmp_path, "config", "user.name", "t")
    git(tmp_path, "add", "-A")
    git(tmp_path, "commit", "-qm", "prereg")
    git(tmp_path, "tag", "prereg-m4")
    return tmp_path


def write_m3(repo_root: Path, recalls: dict[str, float | None], strategy="rtdd") -> None:
    """Publish M3's Axis-2 per-repo summaries at the given `|F_full| == 1` recalls."""
    for repo_id, value in recalls.items():
        out = repo_root / "bench" / "results" / repo_id
        out.mkdir(parents=True, exist_ok=True)
        strata = (
            {}
            if value is None
            else {
                "1": {
                    "n": 20,
                    "change_level_recall": {"num": 20 * value, "den": 20, "value": value},
                    "test_level_recall_micro": {"num": 20 * value, "den": 20, "value": value},
                }
            }
        )
        (out / "summary.json").write_text(
            json.dumps({"repo_id": repo_id, "strategies": {strategy: {"strata": strata}}}),
            encoding="utf-8",
        )


def test_kill_criterion_reads_the_floor_from_the_signed_preregistration(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.97, "flask": 0.96})
    met, actual = kill_criterion_met(root)
    assert met is True
    # The worst repo is the reported value: recall is never pooled across repos.
    assert actual == pytest.approx(0.96)


def test_kill_criterion_is_not_met_when_any_repo_falls_below_the_floor(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.99, "flask": 0.80})
    met, actual = kill_criterion_met(root)
    assert met is False
    assert actual == pytest.approx(0.80)


def test_kill_criterion_refuses_a_floor_that_disagrees_with_the_signed_file(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.99})
    with pytest.raises(PreregError, match="stratified_recall_floor"):
        kill_criterion_met(root, 0.50)


def test_kill_criterion_accepts_a_floor_that_matches_the_signed_file(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.99})
    met, actual = kill_criterion_met(root, 0.95)
    assert met is True
    assert actual == pytest.approx(0.99)


def test_kill_criterion_refuses_an_unsigned_preregistration(tmp_path):
    root = make_repo(tmp_path, floor="0.95", status="UNSIGNED")
    write_m3(root, {"httpie": 0.99})
    with pytest.raises(PreregError, match="not SIGNED"):
        kill_criterion_met(root)


def test_kill_criterion_refuses_when_m3_has_not_been_published(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    with pytest.raises(FileNotFoundError, match="Axis 2"):
        kill_criterion_met(root)


def test_kill_criterion_ignores_axis_1s_own_results_directory(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.99})
    swebench_dir = root / "bench" / "results" / "swebench"
    swebench_dir.mkdir(parents=True, exist_ok=True)
    (swebench_dir / "summary.json").write_text(json.dumps({"arms": {}}), encoding="utf-8")
    met, actual = kill_criterion_met(root)
    assert met is True
    assert actual == pytest.approx(0.99)


def test_kill_criterion_refuses_when_no_repo_reports_the_single_test_stratum(tmp_path):
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": None})
    with pytest.raises(ValueError, match=r"\|F_full\| == 1"):
        kill_criterion_met(root)


# ------------------------------------------------------------------ analyze


def write_arm(repo_root: Path, arm: str, failures: dict[str, list[str]], seed_status=None) -> None:
    """One arm's raw records and harness reports over the whole fixture sample."""
    raw_dir = repo_root / "bench" / "results" / "swebench" / "raw" / arm
    raw_dir.mkdir(parents=True, exist_ok=True)
    report_dir = repo_root / "bench" / "results" / "swebench" / "reports" / arm
    report_dir.mkdir(parents=True, exist_ok=True)
    for instance_id in IDS:
        record = {"instance_id": instance_id, "arm": arm, "patch": "diff --git a b\n"}
        if seed_status is not None:
            record["seed_status"] = seed_status.get(instance_id, "seeded")
        (raw_dir / f"{instance_id}.json").write_text(json.dumps(record), encoding="utf-8")

        p2p = json.loads(ROWS[instance_id]["PASS_TO_PASS"])
        f2p = json.loads(ROWS[instance_id]["FAIL_TO_PASS"])
        failed = failures.get(instance_id, [])
        (report_dir / f"{instance_id}.json").write_text(
            json.dumps(
                {
                    instance_id: {
                        "patch_successfully_applied": True,
                        "tests_status": {
                            "PASS_TO_PASS": {
                                "success": [t for t in p2p if t not in failed],
                                "failure": failed,
                            },
                            "FAIL_TO_PASS": {"success": f2p, "failure": []},
                        },
                    }
                }
            ),
            encoding="utf-8",
        )


#: Every P2P test failing in every instance — an arm nowhere near the published
#: 6.08%, which is what the equivalence band exists to catch.
ALL_FAILING = {i: json.loads(ROWS[i]["PASS_TO_PASS"]) for i in IDS}


def full_repo(tmp_path: Path, *, vanilla_failures=None) -> Path:
    root = make_repo(tmp_path, floor="0.95")
    write_m3(root, {"httpie": 0.97})
    for arm in ARMS:
        write_arm(root, arm, {})
    if vanilla_failures:
        write_arm(root, "vanilla", vanilla_failures)
    write_arm(root, "rtdd", {}, seed_status={IDS[0]: "seed_failed"})
    return root


def test_analyze_writes_the_summary_file_with_every_arm(tmp_path):
    root = full_repo(tmp_path)
    out = run_analysis(root, rows=ROWS)
    dest = root / "bench" / "results" / "swebench" / "summary.json"
    assert dest.exists()
    on_disk = json.loads(dest.read_text(encoding="utf-8"))
    assert on_disk == out
    assert set(on_disk["arms"]) == set(ARMS)
    for arm in ARMS:
        assert on_disk["arms"][arm]["resolution"] == pytest.approx(1.0)


def test_analyze_publishes_the_equivalence_and_kill_criterion_verdicts(tmp_path):
    root = full_repo(tmp_path)
    out = run_analysis(root, rows=ROWS)
    assert "reproduces" in out["vanilla_equivalence"]
    assert "note" in out["vanilla_equivalence"]
    assert out["kill_criterion"]["floor"] == pytest.approx(0.95)
    assert out["kill_criterion"]["observed_single_killer_recall"] == pytest.approx(0.97)
    assert out["kill_criterion"]["met"] is True


def test_analyze_publishes_seed_failed_and_unapplied_counts(tmp_path):
    root = full_repo(tmp_path)
    out = run_analysis(root, rows=ROWS)
    assert out["seed_failed"]["rtdd"] == 1
    assert out["seed_failed"]["rtdd_tdd"] == 0
    assert out["unapplied"] == {arm: 0 for arm in ARMS}


def test_analyze_suppresses_the_published_figures_when_vanilla_diverges(tmp_path):
    # A vanilla arm that breaks every P2P test cannot be reproducing 6.08%.
    root = full_repo(tmp_path, vanilla_failures=ALL_FAILING)
    out = run_analysis(root, rows=ROWS)
    assert out["vanilla_equivalence"]["reproduces"] is False
    assert out["tdad_published"] is None
    assert "HARNESS DIVERGENCE" in out["vanilla_equivalence"]["note"]
    # The banner names 6.08% on purpose — that is the divergence being
    # documented. What must not survive is the figures as *data*: nothing
    # downstream can pick them up and put them in a comparison column.
    body = json.dumps({k: v for k, v in out.items() if k != "vanilla_equivalence"})
    for value in TDAD_PUBLISHED.values():
        assert repr(value) not in body


def test_analyze_keeps_the_published_figures_when_vanilla_reproduces(tmp_path):
    # A local vanilla arm whose band admits 6.08%; the citation then survives.
    root = full_repo(tmp_path)
    out = run_analysis(root, rows=ROWS)
    assert out["vanilla_equivalence"]["reproduces"] is True
    assert out["tdad_published"] == TDAD_PUBLISHED
    assert analyze.TDAD_CITATION in json.dumps(out)


def test_analyze_refuses_when_the_preregistration_is_not_signed(tmp_path):
    root = make_repo(tmp_path, floor="0.95", status="UNSIGNED")
    write_m3(root, {"httpie": 0.97})
    for arm in ARMS:
        write_arm(root, arm, {})
    with pytest.raises(PreregError, match="not SIGNED"):
        run_analysis(root, rows=ROWS)
