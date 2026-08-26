"""The draw is a pure function, so it is tested as one — no network.

`load_rows` is the only thing in `sample.py` that touches the network, and it is
exercised separately against a stub `datasets` module. Everything else takes plain
dict rows, so the eligibility rule, the seeded draw and the hash are reproducible
on a machine with no HuggingFace access at all.
"""

import hashlib
import json
import subprocess
import sys
from pathlib import Path

import pytest

from preflight import instance_list_sha256, preflight
from prereg import PreregError
from sample import DATASET, draw, eligible_ids, load_rows, sha256_ids

SWEBENCH = Path(__file__).resolve().parents[1]
FROZEN = SWEBENCH / "instances.txt"
PREREGISTRATION = SWEBENCH.parent / "PREREGISTRATION.md"


def rows(n: int) -> dict[str, dict]:
    out = {}
    for i in range(n):
        p2p = [] if i % 17 == 0 else [f"t{i}_{j}" for j in range(3)]
        out[f"repo__proj-{i:04d}"] = {
            "instance_id": f"repo__proj-{i:04d}",
            "PASS_TO_PASS": json.dumps(p2p),
            "FAIL_TO_PASS": json.dumps([f"f{i}"]),
        }
    return out


# --- eligibility -----------------------------------------------------------


def test_empty_p2p_is_ineligible():
    e = eligible_ids(rows(50))
    assert "repo__proj-0000" not in e
    assert "repo__proj-0017" not in e
    assert "repo__proj-0001" in e


def test_eligible_is_sorted():
    e = eligible_ids(rows(50))
    assert e == sorted(e)


def test_eligible_accepts_a_list_valued_p2p_column():
    """`datasets` may hand back a list rather than a JSON string."""
    listy = {
        "a__a-1": {"instance_id": "a__a-1", "PASS_TO_PASS": ["t1"]},
        "a__a-2": {"instance_id": "a__a-2", "PASS_TO_PASS": []},
    }
    assert eligible_ids(listy) == ["a__a-1"]


# --- the draw --------------------------------------------------------------


def test_draw_is_deterministic():
    r = rows(300)
    a = draw(r, seed=20260826, n=100)
    b = draw(r, seed=20260826, n=100)
    assert a == b
    assert len(a) == 100
    assert a == sorted(a)


def test_draw_changes_with_seed():
    r = rows(300)
    assert draw(r, seed=20260826, n=100) != draw(r, seed=1, n=100)


def test_draw_never_returns_an_ineligible_instance():
    r = rows(300)
    assert set(draw(r, seed=20260826, n=100)) <= set(eligible_ids(r))


def test_draw_refuses_when_the_eligible_pool_is_too_small():
    with pytest.raises(ValueError, match="eligible"):
        draw(rows(10), seed=20260826, n=100)


# --- the hash --------------------------------------------------------------


def test_sha_is_stable_and_newline_terminated():
    ids = ["a", "b", "c"]
    assert sha256_ids(ids) == hashlib.sha256(b"a\nb\nc\n").hexdigest()


# --- the dataset loader, without a network -------------------------------


def test_dataset_is_swebench_verified():
    assert DATASET == "princeton-nlp/SWE-bench_Verified"


def test_load_rows_keys_by_instance_id(monkeypatch):
    import types

    captured = {}

    def fake_load_dataset(name, split):
        captured["name"] = name
        captured["split"] = split
        return [
            {"instance_id": "a__a-1", "PASS_TO_PASS": "[\"t1\"]"},
            {"instance_id": "a__a-2", "PASS_TO_PASS": "[]"},
        ]

    stub = types.ModuleType("datasets")
    stub.load_dataset = fake_load_dataset
    monkeypatch.setitem(sys.modules, "datasets", stub)

    got = load_rows()
    assert captured == {"name": DATASET, "split": "test"}
    assert sorted(got) == ["a__a-1", "a__a-2"]
    assert got["a__a-1"]["PASS_TO_PASS"] == '["t1"]'


# --- the frozen sample -----------------------------------------------------


def frozen_ids() -> list[str]:
    return [ln for ln in FROZEN.read_text(encoding="utf-8").splitlines() if ln.strip()]


def test_frozen_file_is_one_hundred_ids_one_per_line():
    text = FROZEN.read_text(encoding="utf-8")
    lines = text.split("\n")
    assert lines[-1] == "", "instances.txt must end with a newline"
    ids = lines[:-1]
    assert len(ids) == 100
    assert all(ln == ln.strip() and ln for ln in ids)


def test_frozen_file_is_sorted_and_free_of_duplicates():
    ids = frozen_ids()
    assert ids == sorted(ids)
    assert len(set(ids)) == len(ids)


def test_frozen_file_sha_is_reproducible_from_the_file_alone():
    """What `sha256sum bench/swebench/instances.txt` prints is the pre-registered value."""
    ids = frozen_ids()
    raw = hashlib.sha256(FROZEN.read_bytes()).hexdigest()
    assert sha256_ids(ids) == raw
    assert instance_list_sha256(FROZEN) == raw


def test_prereg_carries_the_frozen_files_sha():
    sha = instance_list_sha256(FROZEN)
    assert f"instance_list_sha256: {sha}" in PREREGISTRATION.read_text(encoding="utf-8")


# --- the frozen sample, through the preflight gate -------------------------

SIGNED = """---
status: SIGNED
sample_size: 100
sample_seed: 20260826
instance_list_sha256: {sha}
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: 0.95
vanilla_equivalence_k: 2
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def make_repo(tmp_path: Path, *, instances: str, sha: str) -> Path:
    (tmp_path / "bench" / "swebench").mkdir(parents=True)
    (tmp_path / "bench" / "PREREGISTRATION.md").write_text(
        SIGNED.format(sha=sha), encoding="utf-8"
    )
    (tmp_path / "bench" / "swebench" / "instances.txt").write_text(instances, encoding="utf-8")
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    for args in (
        ("config", "user.email", "t@t"),
        ("config", "user.name", "t"),
        ("add", "-A"),
        ("commit", "-qm", "prereg"),
        ("tag", "prereg-m4"),
    ):
        subprocess.run(["git", "-C", str(tmp_path), *args], check=True)
    return tmp_path


@pytest.fixture
def frozen_text() -> str:
    return FROZEN.read_text(encoding="utf-8")


def test_preflight_passes_on_the_frozen_file(tmp_path, frozen_text):
    root = make_repo(tmp_path, instances=frozen_text, sha=instance_list_sha256(FROZEN))
    pr = preflight(root)
    assert int(pr.fields["sample_size"]) == 100
    assert int(pr.fields["sample_seed"]) == 20260826


def test_preflight_refuses_when_one_line_of_the_frozen_file_is_edited(tmp_path, frozen_text):
    ids = frozen_ids()
    edited = frozen_text.replace(ids[0] + "\n", ids[0] + "-tampered\n", 1)
    assert edited != frozen_text
    root = make_repo(tmp_path, instances=edited, sha=instance_list_sha256(FROZEN))
    with pytest.raises(PreregError, match="sha256"):
        preflight(root)
