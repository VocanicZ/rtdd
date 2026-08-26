"""The single gate every arm runner calls first.

No arm may spend a token until the pre-registration is complete, SIGNED, tagged
`prereg-m4`, and describing the very instance list that is about to be run.
"""

from __future__ import annotations

import hashlib
import sys
from pathlib import Path

from prereg import Prereg, PreregError, assert_tagged, load


def instance_list_sha256(path: Path) -> str:
    ids = [line.strip() for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    return hashlib.sha256(("\n".join(ids) + "\n").encode("utf-8")).hexdigest()


def preflight(repo_root: Path) -> Prereg:
    prereg_path = repo_root / "bench" / "PREREGISTRATION.md"
    pr = load(prereg_path)
    assert_tagged(repo_root, prereg_path)

    instances = repo_root / "bench" / "swebench" / "instances.txt"
    if not instances.exists():
        raise PreregError(f"{instances} does not exist — run sample.py and freeze it")

    actual = instance_list_sha256(instances)
    expected = str(pr.fields["instance_list_sha256"])
    if actual != expected:
        raise PreregError(
            f"instances.txt sha256 {actual} != pre-registered {expected} — "
            "the sample was changed after signing"
        )

    count = len([ln for ln in instances.read_text(encoding="utf-8").splitlines() if ln.strip()])
    if count != int(pr.fields["sample_size"]):
        raise PreregError(
            f"instances.txt has {count} ids, pre-registered {pr.fields['sample_size']}"
        )
    return pr


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    try:
        gate = preflight(root)
    except PreregError as exc:
        print(f"PREFLIGHT REFUSED: {exc}", file=sys.stderr)
        raise SystemExit(3)
    print(
        f"preflight ok: floor={gate.fields['stratified_recall_floor']} "
        f"n={gate.fields['sample_size']} seed={gate.fields['sample_seed']}"
    )
