"""Deterministic, pre-registered instance draw from SWE-bench Verified.

The draw is a pure function of (dataset rows, seed, n) so anyone can reproduce
`instances.txt` from the pre-registered seed. Instances with an empty PASS_TO_PASS
set are ineligible: with no test passing before the patch there is nothing that can
regress, so keeping them would silently shrink the denominator the whole benchmark
divides by.

`instances.txt` is frozen. It is written once, committed, and never regenerated —
`preflight.py` refuses to launch an arm unless its sha256 still matches the value a
human pre-registered.
"""

from __future__ import annotations

import hashlib
import json
import random
import sys
from pathlib import Path

DATASET = "princeton-nlp/SWE-bench_Verified"
SPLIT = "test"


def load_rows() -> dict[str, dict]:
    """The only network call in this module; every other function takes plain rows."""
    from datasets import load_dataset

    ds = load_dataset(DATASET, split=SPLIT)
    return {row["instance_id"]: dict(row) for row in ds}


def _p2p(row: dict) -> list[str]:
    raw = row["PASS_TO_PASS"]
    return json.loads(raw) if isinstance(raw, str) else list(raw)


def eligible_ids(rows: dict[str, dict]) -> list[str]:
    return sorted(iid for iid, row in rows.items() if _p2p(row))


def draw(rows: dict[str, dict], seed: int, n: int) -> list[str]:
    pool = eligible_ids(rows)
    if len(pool) < n:
        raise ValueError(f"only {len(pool)} eligible instances, need {n}")
    return sorted(random.Random(seed).sample(pool, n))


def sha256_ids(ids: list[str]) -> str:
    """The hash of the file `instances.txt` is, so `sha256sum` reproduces it."""
    return hashlib.sha256(("\n".join(ids) + "\n").encode("utf-8")).hexdigest()


if __name__ == "__main__":
    seed = int(sys.argv[1]) if len(sys.argv) > 1 else 20260826
    n = int(sys.argv[2]) if len(sys.argv) > 2 else 100
    rows = load_rows()
    pool = eligible_ids(rows)
    ids = draw(rows, seed, n)
    out = Path(__file__).with_name("instances.txt")
    out.write_text("\n".join(ids) + "\n", encoding="utf-8")
    print(f"total={len(rows)} eligible={len(pool)} excluded_empty_p2p={len(rows) - len(pool)}")
    print(f"wrote {out} n={len(ids)}")
    print(f"instance_list_sha256: {sha256_ids(ids)}")
