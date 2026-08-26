"""Content-addressed cache for replay-benchmark artifacts.

Ground truth is the expensive part of the benchmark — a full suite per commit per
variant plus an instrumented full run — so every artifact is cached by
``(repo_id, commit, variant, strategy, config_digest)``.  A change to the harness
*code* must therefore be a cheap re-run, while a change to the *config* must be a
correct cache miss rather than a silent mix of old and new numbers: the key is
salted with the :class:`replay.config.RunConfig` digest, so no entry written under
one config can ever be read back under another.

A cache entry is never half-valid.  JSON values land via a temp file and an atomic
rename; directory artifacts land as a payload directory plus a sibling
``<key>.complete`` marker written last.  Anything that fails those checks — a
truncated JSON body, a payload with no marker — is reported as a miss, so a crashed
run costs a re-computation and never a traceback.
"""

from __future__ import annotations

import hashlib
import json
import pathlib
import shutil
import tempfile
from collections.abc import Callable

from replay.config import canonical_json

DEFAULT_ROOT = pathlib.Path(__file__).resolve().parent.parent / "cache"
"""``bench/cache/`` — gitignored, so cached ground truth never enters history."""

_STAGING_PREFIX = ".staging-"


class Cache:
    def __init__(self, root: pathlib.Path, config_digest: str) -> None:
        self.root = pathlib.Path(root)
        self.config_digest = config_digest
        self.root.mkdir(parents=True, exist_ok=True)
        self._hits = 0
        self._misses = 0

    def key(self, *parts: str) -> str:
        # NUL-joined so no two distinct coordinate tuples can collide by
        # concatenation: ("ab", "c") and ("a", "bc") hash differently.
        raw = "\0".join((self.config_digest, *parts))
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()

    def _json_path(self, key: str) -> pathlib.Path:
        return self.root / "json" / key[:2] / f"{key}.json"

    def _artifact_path(self, key: str) -> pathlib.Path:
        return self.root / "artifact" / key[:2] / key

    def _complete_marker(self, key: str) -> pathlib.Path:
        # A sibling of the payload, not a file inside it, so the marker can never
        # shadow a file the caller actually stored.
        p = self._artifact_path(key)
        return p.with_name(p.name + ".complete")

    def get_json(self, key: str) -> dict | None:
        p = self._json_path(key)
        try:
            raw = p.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            return None
        try:
            obj = json.loads(raw)
        except ValueError:
            return None
        return obj if isinstance(obj, dict) else None

    def put_json(self, key: str, obj: dict) -> None:
        p = self._json_path(key)
        p.parent.mkdir(parents=True, exist_ok=True)
        fd, tmp_name = tempfile.mkstemp(dir=str(p.parent), prefix=_STAGING_PREFIX)
        tmp = pathlib.Path(tmp_name)
        try:
            with open(fd, "w", encoding="utf-8") as fh:
                fh.write(canonical_json(obj))
            tmp.replace(p)
        except BaseException:
            tmp.unlink(missing_ok=True)
            raise

    def json_or_build(self, key: str, build: Callable[[], dict]) -> dict:
        got = self.get_json(key)
        if got is not None:
            self._hits += 1
            return got
        self._misses += 1
        obj = build()
        self.put_json(key, obj)
        return obj

    def artifact(self, key: str) -> pathlib.Path | None:
        p = self._artifact_path(key)
        if not self._complete_marker(key).exists() or not p.is_dir():
            return None
        return p

    def store_artifact(self, key: str, src_dir: pathlib.Path) -> pathlib.Path:
        p = self._artifact_path(key)
        marker = self._complete_marker(key)
        p.parent.mkdir(parents=True, exist_ok=True)
        # Drop the marker first: while the payload is being replaced the entry is
        # incomplete, and a crash in the middle must read back as a miss.
        marker.unlink(missing_ok=True)
        staging = pathlib.Path(tempfile.mkdtemp(dir=str(p.parent), prefix=_STAGING_PREFIX))
        try:
            payload = staging / "d"
            shutil.copytree(src_dir, payload)
            if p.exists():
                shutil.rmtree(p)
            payload.replace(p)
        finally:
            shutil.rmtree(staging, ignore_errors=True)
        marker.write_text("", encoding="utf-8")
        return p

    def restore_artifact(self, key: str, dest_dir: pathlib.Path) -> bool:
        p = self.artifact(key)
        if p is None:
            self._misses += 1
            return False
        self._hits += 1
        dest_dir.mkdir(parents=True, exist_ok=True)
        for item in p.iterdir():
            target = dest_dir / item.name
            if item.is_dir():
                shutil.copytree(item, target, dirs_exist_ok=True)
            else:
                shutil.copy2(item, target)
        return True

    def stats(self) -> dict:
        return {"hits": self._hits, "misses": self._misses}
