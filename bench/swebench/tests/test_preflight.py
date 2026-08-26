import subprocess
import sys
from pathlib import Path

import pytest

from preflight import instance_list_sha256, preflight
from prereg import PreregError

SIGNED = """---
status: SIGNED
sample_size: 3
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

INSTANCES = "astropy__astropy-12907\ndjango__django-11039\nsympy__sympy-20049\n"


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True)


def make_repo(tmp_path: Path, *, prereg_text: str, instances: str | None) -> Path:
    (tmp_path / "bench" / "swebench").mkdir(parents=True)
    (tmp_path / "bench" / "PREREGISTRATION.md").write_text(prereg_text, encoding="utf-8")
    if instances is not None:
        (tmp_path / "bench" / "swebench" / "instances.txt").write_text(instances, encoding="utf-8")
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    git(tmp_path, "config", "user.email", "t@t")
    git(tmp_path, "config", "user.name", "t")
    git(tmp_path, "add", "-A")
    git(tmp_path, "commit", "-qm", "prereg")
    git(tmp_path, "tag", "prereg-m4")
    return tmp_path


def signed_for(instances: str) -> str:
    import hashlib

    ids = [ln.strip() for ln in instances.splitlines() if ln.strip()]
    sha = hashlib.sha256(("\n".join(ids) + "\n").encode("utf-8")).hexdigest()
    return SIGNED.format(sha=sha)


def test_instance_list_sha256_ignores_blank_lines_and_trailing_whitespace(tmp_path):
    a = tmp_path / "a.txt"
    b = tmp_path / "b.txt"
    a.write_text("one\ntwo\n", encoding="utf-8")
    b.write_text("\n  one  \n\ntwo\n\n", encoding="utf-8")
    assert instance_list_sha256(a) == instance_list_sha256(b)


def test_signed_tagged_and_matching_passes(tmp_path):
    root = make_repo(tmp_path, prereg_text=signed_for(INSTANCES), instances=INSTANCES)
    pr = preflight(root)
    assert pr.fields["stratified_recall_floor"] == 0.95


def test_missing_instance_list_is_a_clean_refusal(tmp_path):
    root = make_repo(tmp_path, prereg_text=signed_for(INSTANCES), instances=None)
    with pytest.raises(PreregError, match="instances.txt"):
        preflight(root)


def test_instance_list_sha_mismatch_is_refused(tmp_path):
    tampered = INSTANCES.replace("sympy__sympy-20049", "sympy__sympy-99999")
    root = make_repo(tmp_path, prereg_text=signed_for(INSTANCES), instances=tampered)
    with pytest.raises(PreregError, match="sha256"):
        preflight(root)


def test_instance_count_mismatch_is_refused(tmp_path):
    short = "astropy__astropy-12907\ndjango__django-11039\n"
    root = make_repo(tmp_path, prereg_text=signed_for(short), instances=short)
    with pytest.raises(PreregError, match="pre-registered 3"):
        preflight(root)


def test_unsigned_prereg_is_refused_by_preflight(tmp_path):
    text = signed_for(INSTANCES).replace("status: SIGNED", "status: UNSIGNED")
    root = make_repo(tmp_path, prereg_text=text, instances=INSTANCES)
    with pytest.raises(PreregError, match="not SIGNED"):
        preflight(root)


def test_untagged_prereg_is_refused_by_preflight(tmp_path):
    root = make_repo(tmp_path, prereg_text=signed_for(INSTANCES), instances=INSTANCES)
    git(root, "tag", "-d", "prereg-m4")
    with pytest.raises(PreregError, match="prereg-m4"):
        preflight(root)


def test_cli_refuses_this_unsigned_repo_with_exit_code_3():
    script = Path(__file__).resolve().parents[1] / "preflight.py"
    proc = subprocess.run([sys.executable, str(script)], capture_output=True, text=True)
    assert proc.returncode == 3, proc.stderr
    assert "PREFLIGHT REFUSED" in proc.stderr
    assert "Traceback" not in proc.stderr
