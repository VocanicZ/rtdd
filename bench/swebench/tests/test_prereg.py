import subprocess
from pathlib import Path

import pytest

from prereg import Prereg, PreregError, assert_tagged, load

COMPLETE = """---
status: SIGNED
sample_size: 100
sample_seed: 20260826
instance_list_sha256: 0000000000000000000000000000000000000000000000000000000000000000
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: 0.95
vanilla_equivalence_k: 2
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def write(tmp_path: Path, text: str) -> Path:
    p = tmp_path / "PREREGISTRATION.md"
    p.write_text(text, encoding="utf-8")
    return p


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True)


def init_repo(tmp_path: Path) -> None:
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    git(tmp_path, "config", "user.email", "t@t")
    git(tmp_path, "config", "user.name", "t")


def test_complete_and_signed_loads(tmp_path):
    pr = load(write(tmp_path, COMPLETE))
    assert isinstance(pr, Prereg)
    assert pr.fields["stratified_recall_floor"] == 0.95
    assert pr.fields["sample_seed"] == 20260826


def test_unsigned_is_refused(tmp_path):
    text = COMPLETE.replace("status: SIGNED", "status: UNSIGNED")
    with pytest.raises(PreregError, match="not SIGNED"):
        load(write(tmp_path, text))


def test_missing_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95\n", "")
    with pytest.raises(PreregError, match="stratified_recall_floor"):
        load(write(tmp_path, text))


def test_blank_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95", "stratified_recall_floor:")
    with pytest.raises(PreregError, match="stratified_recall_floor"):
        load(write(tmp_path, text))


def test_placeholder_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95", "stratified_recall_floor: TBD")
    with pytest.raises(PreregError, match="numeric"):
        load(write(tmp_path, text))


def test_no_front_matter_is_refused(tmp_path):
    with pytest.raises(PreregError, match="front-matter"):
        load(write(tmp_path, "just a document\n"))


def test_missing_file_is_refused(tmp_path):
    with pytest.raises(PreregError, match="does not exist"):
        load(tmp_path / "PREREGISTRATION.md")


def test_untagged_prereg_is_refused(tmp_path):
    init_repo(tmp_path)
    p = write(tmp_path, COMPLETE)
    git(tmp_path, "add", "-A")
    git(tmp_path, "commit", "-qm", "prereg")
    with pytest.raises(PreregError, match="prereg-m4"):
        assert_tagged(tmp_path, p)
    git(tmp_path, "tag", "prereg-m4")
    assert len(assert_tagged(tmp_path, p)) == 40


def test_edit_after_tagging_is_refused(tmp_path):
    init_repo(tmp_path)
    p = write(tmp_path, COMPLETE)
    git(tmp_path, "add", "-A")
    git(tmp_path, "commit", "-qm", "prereg")
    git(tmp_path, "tag", "prereg-m4")
    write(tmp_path, COMPLETE.replace("floor: 0.95", "floor: 0.5"))
    git(tmp_path, "commit", "-qam", "sneak")
    with pytest.raises(PreregError, match="not reachable from tag prereg-m4"):
        assert_tagged(tmp_path, p)


def test_shipped_preregistration_is_unsigned_and_has_no_kill_criterion():
    shipped = Path(__file__).resolve().parents[3] / "bench" / "PREREGISTRATION.md"
    assert shipped.exists(), f"{shipped} must be committed"
    with pytest.raises(PreregError):
        load(shipped)
    text = shipped.read_text(encoding="utf-8")
    assert "status: UNSIGNED" in text
    assert "\nstratified_recall_floor:\n" in text, "the kill criterion must ship valueless"
