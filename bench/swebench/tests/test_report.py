"""The publication layer, held to the guarantees it exists to enforce.

Everything here works from a synthetic ``summary.json`` and a fixture repo. The
renderer is pure, so the two guarantees it publishes are checkable without a
benchmark run behind them:

  1. **A regression rate never appears without a resolution rate beside it.**
     The check is on the renderer — ``assert_resolution_beside_regression``
     re-reads the markdown the renderer is about to return and refuses it —
     rather than on whoever reviews the table.
  2. **A divergent vanilla arm strips TDAD's published column.** Not a
     footnote, not an empty cell: the column is absent from the header, from
     the separator and from every row, and the table opens with a
     ``HARNESS DIVERGENCE`` blockquote.
"""

import hashlib
import json
import re
import subprocess
from pathlib import Path

import pytest

from agent import AgentConfig
from providers.tdad import TDAD_PIN
from preflight import instance_list_sha256
from report import (
    ARM_LABEL,
    ARM_ORDER,
    ReportError,
    assert_resolution_beside_regression,
    observed_models,
    render,
    write_config,
)

IDS = ["astropy__astropy-12907", "django__django-11039", "sympy__sympy-20049"]

SIGNED = """---
status: SIGNED
sample_size: 3
sample_seed: 20260826
instance_list_sha256: {sha}
model: {model}
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: 0.95
vanilla_equivalence_k: 2
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def arm_summary(arm, *, test_level, resolution, n=100):
    half = 0.01
    return {
        "arm": arm,
        "n": n,
        "test_level": test_level,
        "test_ci": [test_level - half, test_level + half],
        "instance_level": test_level * 2,
        "instance_ci": [test_level * 2 - half, test_level * 2 + half],
        "resolution": resolution,
        "p2p_failed": 7,
        "p2p_total": 900,
        "unapplied": 1,
        "harness_errors": 2,
    }


def summary_doc(*, reproduces=True):
    arms = {
        "vanilla": arm_summary("vanilla", test_level=0.0608, resolution=0.31),
        "tdd": arm_summary("tdd", test_level=0.0994, resolution=0.30),
        "tdad": arm_summary("tdad", test_level=0.0182, resolution=0.29),
        "rtdd": arm_summary("rtdd", test_level=0.0090, resolution=0.33),
        "rtdd_tdd": arm_summary("rtdd_tdd", test_level=0.0075, resolution=0.34),
    }
    note = (
        "local vanilla 0.0608 reproduces the published 0.0608"
        if reproduces
        else "HARNESS DIVERGENCE: local vanilla 0.2500 excludes the published 0.0608"
    )
    return {
        "arms": arms,
        "vanilla_equivalence": {"reproduces": reproduces, "note": note},
        "tdad_published": {"vanilla": 0.0608, "tdd": 0.0994, "tdad": 0.0182}
        if reproduces
        else None,
        "tdad_citation": "arXiv:2603.17973, n=100 SWE-bench Verified",
        "kill_criterion": {
            "floor": 0.95,
            "observed_single_killer_recall": 0.9640,
            "met": True,
        },
        "comparisons": [
            {
                "a": "vanilla",
                "b": "rtdd",
                "delta": 0.0518,
                "a_ci": [0.0508, 0.0708],
                "b_ci": [-0.0010, 0.0190],
                "a_resolution": 0.31,
                "b_resolution": 0.33,
                "ci_disjoint": True,
                "verdict": "rtdd lower than vanilla",
            },
            {
                "a": "tdad",
                "b": "rtdd",
                "delta": 0.0092,
                "a_ci": [0.0082, 0.0282],
                "b_ci": [-0.0010, 0.0190],
                "a_resolution": 0.29,
                "b_resolution": 0.33,
                "ci_disjoint": False,
                "verdict": "no distinguishable difference",
            },
        ],
    }


def tables(text):
    """Every markdown table in the document, as lists of header/row cell lists."""
    out = []
    current = None
    for line in text.splitlines():
        if line.startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            if current is None:
                current = [cells]
            else:
                current.append(cells)
        elif current is not None:
            out.append(current)
            current = None
    if current is not None:
        out.append(current)
    return out


# ------------------------------------------------------------------- rows


def test_render_emits_one_row_per_arm_in_the_pre_registered_order():
    text = render(summary_doc())
    positions = [text.index(ARM_LABEL[arm]) for arm in ARM_ORDER]
    assert positions == sorted(positions)
    assert ARM_ORDER == ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")


def test_every_arm_row_carries_both_regression_rates_and_the_resolution_rate():
    text = render(summary_doc())
    header, _sep, *rows = tables(text)[0]
    assert header[:5] == [
        "arm",
        "regressions (test-level)",
        "95% CI",
        "regressions (instance-level)",
        "resolved",
    ]
    assert len(rows) == len(ARM_ORDER)
    for row in rows:
        assert row[1].endswith("%")
        assert row[2].startswith("[")
        assert row[3].endswith("%")
        assert row[4].endswith("%")


def test_every_rendered_table_naming_a_regression_also_names_resolution():
    # The audit the renderer runs on itself, run again from outside on the real
    # output: no table anywhere in the document may cite a regression alone.
    assert_resolution_beside_regression(render(summary_doc()))
    assert_resolution_beside_regression(render(summary_doc(reproduces=False)))


def test_a_regression_column_without_a_resolution_column_is_refused():
    rigged = "| arm | regressions (test-level) |\n|---|---|\n| vanilla | 6.08% |\n"
    with pytest.raises(ReportError, match="resolution"):
        assert_resolution_beside_regression(rigged)


def test_the_renderer_refuses_its_own_output_when_the_resolution_column_is_dropped(
    monkeypatch,
):
    import report

    monkeypatch.setattr(report, "RESOLUTION_COLUMN", "solved-ish")
    with pytest.raises(ReportError, match="resolution"):
        report.render(summary_doc())


# ------------------------------------------------------------- divergence


def test_divergence_opens_with_the_banner_carrying_the_note():
    text = render(summary_doc(reproduces=False))
    assert text.startswith("> **HARNESS DIVERGENCE**\n")
    assert "local vanilla 0.2500 excludes the published 0.0608" in text


def test_divergence_strips_the_published_column_from_header_separator_and_rows():
    text = render(summary_doc(reproduces=False))
    assert "TDAD published" not in text
    header, sep, *rows = tables(text)[0]
    assert len(header) == 5
    assert len(sep) == 5
    for row in rows:
        assert len(row) == 5


def test_reproducing_output_carries_no_banner():
    text = render(summary_doc())
    assert "HARNESS DIVERGENCE" not in text
    assert not text.startswith(">")


def test_the_published_column_is_populated_only_for_the_three_published_arms():
    text = render(summary_doc())
    header, sep, *rows = tables(text)[0]
    assert header[5] == "TDAD published"
    assert len(sep) == 6
    cited = {row[0]: row[5] for row in rows}
    assert cited[ARM_LABEL["vanilla"]] == "6.08%"
    assert cited[ARM_LABEL["tdd"]] == "9.94%"
    assert cited[ARM_LABEL["tdad"]] == "1.82%"
    assert cited[ARM_LABEL["rtdd"]] == "—"
    assert cited[ARM_LABEL["rtdd_tdd"]] == "—"


# ---------------------------------------------------------------- precision


def test_rates_render_at_a_fixed_precision_so_tables_diff_cleanly():
    doc = summary_doc()
    doc["arms"]["rtdd"]["test_level"] = 0.0123456789
    doc["arms"]["rtdd"]["resolution"] = 0.3333333333
    row = next(r for r in tables(render(doc))[0][2:] if r[0] == ARM_LABEL["rtdd"])
    assert row[1] == "1.23%"
    assert row[4] == "33.3%"
    assert not re.search(r"\d\.\d{3,}%", render(doc))


def test_rendering_the_same_summary_twice_is_byte_identical():
    doc = summary_doc()
    assert render(doc) == render(summary_doc())
    assert render(doc).endswith("\n")


# ------------------------------------------------------- pairwise and gate


def test_pairwise_rows_carry_both_resolution_rates_beside_the_regression_delta():
    text = render(summary_doc())
    pairwise = next(t for t in tables(text) if "regression delta (test-level)" in t[0])
    header, _sep, *rows = pairwise
    assert "resolved (a → b)" in header
    assert rows[0][1] == "+5.18 pp"
    assert "31.0% → 33.0%" in rows[0]


def test_the_kill_criterion_verdict_is_printed_with_its_signed_floor():
    text = render(summary_doc())
    assert "0.9500" in text
    assert "0.9640" in text
    assert "MET" in text


# -------------------------------------------------------------- provenance


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True)


def make_repo(tmp_path: Path, *, model=None) -> Path:
    """A repo root with a signed, tagged pre-registration over the fixture sample."""
    (tmp_path / "bench" / "swebench").mkdir(parents=True)
    instances = "\n".join(IDS) + "\n"
    (tmp_path / "bench" / "swebench" / "instances.txt").write_text(
        instances, encoding="utf-8"
    )
    sha = hashlib.sha256(instances.encode("utf-8")).hexdigest()
    (tmp_path / "bench" / "PREREGISTRATION.md").write_text(
        SIGNED.format(sha=sha, model=model or AgentConfig().model), encoding="utf-8"
    )
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    git(tmp_path, "config", "user.email", "t@t")
    git(tmp_path, "config", "user.name", "t")
    git(tmp_path, "add", "-A")
    git(tmp_path, "commit", "-qm", "prereg")
    git(tmp_path, "tag", "prereg-m4")
    return tmp_path


def write_seed_statuses(repo_root: Path, arm: str, statuses: dict[str, str]) -> None:
    raw = repo_root / "bench" / "results" / "swebench" / "raw" / arm
    raw.mkdir(parents=True, exist_ok=True)
    for instance_id, status in statuses.items():
        (raw / f"{instance_id}.json").write_text(
            json.dumps({"instance_id": instance_id, "arm": arm, "seed_status": status}),
            encoding="utf-8",
        )


def config_of(repo_root: Path) -> dict:
    return json.loads(write_config(repo_root).read_text(encoding="utf-8"))


def test_write_config_writes_into_the_swebench_results_directory(tmp_path):
    root = make_repo(tmp_path)
    dest = write_config(root)
    assert dest == root / "bench" / "results" / "swebench" / "config.json"
    assert dest.exists()


def test_config_records_the_rtdd_commit_under_test(tmp_path):
    root = make_repo(tmp_path)
    head = subprocess.run(
        ["git", "-C", str(root), "rev-parse", "HEAD"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    assert config_of(root)["rtdd_commit"] == head


def test_config_records_the_tdad_pin_arm_c_actually_ran(tmp_path):
    assert config_of(make_repo(tmp_path))["tdad_pin"] == TDAD_PIN


def test_config_records_the_model_id_and_its_decoding_settings(tmp_path):
    cfg = AgentConfig()
    model = config_of(make_repo(tmp_path))["model"]
    assert model["id"] == cfg.model
    assert model["temperature"] == cfg.temperature
    assert model["max_tokens"] == cfg.max_tokens
    assert model["max_turns"] == cfg.max_turns


def test_config_refuses_a_model_that_is_not_the_pre_registered_one(tmp_path):
    root = make_repo(tmp_path, model="gpt-4o")
    with pytest.raises(ReportError, match="pre-registered"):
        write_config(root)


def write_models(repo_root: Path, arm: str, models: dict[str, str]) -> None:
    raw = repo_root / "bench" / "results" / "swebench" / "raw" / arm
    raw.mkdir(parents=True, exist_ok=True)
    for instance_id, model in models.items():
        (raw / f"{instance_id}.json").write_text(
            json.dumps({"instance_id": instance_id, "arm": arm, "model": model}),
            encoding="utf-8",
        )


def test_observed_models_folds_the_distinct_models_out_of_the_raw_records(tmp_path):
    root = make_repo(tmp_path)
    write_models(root, "vanilla", {IDS[0]: "model-a", IDS[1]: "model-a"})
    write_models(root, "rtdd", {IDS[0]: "model-b"})
    assert observed_models(root) == ["model-a", "model-b"]


def test_the_model_guard_reads_the_records_not_the_agent_default(tmp_path):
    # run_arm.py takes --model and stamps the real one into every record, so a
    # guard that compares AgentConfig's default lets another model's run publish
    # a config claiming the pre-registered one.
    root = make_repo(tmp_path)
    write_models(root, "vanilla", {IDS[0]: "gpt-4o"})
    with pytest.raises(ReportError, match="gpt-4o"):
        write_config(root)


def test_config_records_the_model_the_records_were_actually_produced_under(tmp_path):
    root = make_repo(tmp_path, model="Qwen3-Coder-30B-A3B-Instruct-AWQ")
    write_models(root, "vanilla", {IDS[0]: "Qwen3-Coder-30B-A3B-Instruct-AWQ"})
    assert config_of(root)["model"]["id"] == "Qwen3-Coder-30B-A3B-Instruct-AWQ"


def test_a_run_whose_records_name_two_models_is_refused(tmp_path):
    root = make_repo(tmp_path)
    write_models(root, "vanilla", {IDS[0]: AgentConfig().model})
    write_models(root, "rtdd", {IDS[0]: "gpt-4o"})
    with pytest.raises(ReportError, match="more than one model"):
        write_config(root)


def test_config_records_the_sample_seed_size_and_instance_list_sha256(tmp_path):
    root = make_repo(tmp_path)
    sample = config_of(root)["sample"]
    instances = root / "bench" / "swebench" / "instances.txt"
    assert sample["seed"] == 20260826
    assert sample["size"] == 3
    assert sample["instance_list_sha256"] == instance_list_sha256(instances)


def test_the_published_digest_is_the_one_the_gate_checked(tmp_path):
    # One field name, one definition. preflight hashes the normalised ids; a
    # second raw-bytes hash published under the same name would disagree with
    # the gate the moment the file gained a blank line.
    root = make_repo(tmp_path)
    instances = root / "bench" / "swebench" / "instances.txt"
    instances.write_text(instances.read_text(encoding="utf-8") + "\n\n", encoding="utf-8")
    assert instance_list_sha256(instances) != hashlib.sha256(instances.read_bytes()).hexdigest()
    assert config_of(root)["sample"]["instance_list_sha256"] == instance_list_sha256(instances)


def test_config_records_the_host_the_wall_clock_figures_came_from(tmp_path):
    host = config_of(make_repo(tmp_path))["host"]
    import platform

    assert host["platform"] == platform.platform()
    assert host["processor"] == platform.processor()
    assert host["python"] == platform.python_version()


def test_config_records_seeded_and_seed_failed_counts_per_arm(tmp_path):
    root = make_repo(tmp_path)
    write_seed_statuses(
        root,
        "rtdd",
        {IDS[0]: "seeded", IDS[1]: "seeded", IDS[2]: "seed_failed"},
    )
    seeding = config_of(root)["seeding"]
    assert seeding["rtdd"] == {"seeded": 2, "seed_failed": 1}
    # An arm that carries no map is reported as such rather than omitted.
    assert seeding["vanilla"] == {"seeded": 0, "seed_failed": 0}


def test_config_is_written_for_every_pre_registered_arm(tmp_path):
    seeding = config_of(make_repo(tmp_path))["seeding"]
    assert set(seeding) == set(ARM_ORDER)
