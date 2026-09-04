from __future__ import annotations

import dataclasses
import hashlib
import importlib.metadata
import json

import replay

TRACKED_TOOLS = (
    "pytest",
    "pytest-testmon",
    "pytest-xdist",
    "pytest-cov",
    "pytest-reportlog",
    "coverage",
)


def canonical_json(obj: object) -> str:
    return json.dumps(obj, sort_keys=True, separators=(",", ":")) + "\n"


def tool_versions(env: dict[str, str] | None = None) -> tuple[tuple[str, str], ...]:
    out: list[tuple[str, str]] = []
    for name in TRACKED_TOOLS:
        try:
            out.append((name, importlib.metadata.version(name)))
        except importlib.metadata.PackageNotFoundError:
            out.append((name, "absent"))
    return tuple(sorted(out))


@dataclasses.dataclass(frozen=True)
class RunConfig:
    corpus_digest: str
    rtdd_version: str
    tool_versions: tuple[tuple[str, str], ...]
    strategies: tuple[str, ...]
    variants: tuple[str, ...]
    replay_commits: int
    wallclock_sample: int
    random_seed: int
    harness_version: str = replay.__version__

    def to_dict(self) -> dict:
        return {
            "corpus_digest": self.corpus_digest,
            "rtdd_version": self.rtdd_version,
            "tool_versions": {k: v for k, v in self.tool_versions},
            "strategies": list(self.strategies),
            "variants": list(self.variants),
            "replay_commits": self.replay_commits,
            "wallclock_sample": self.wallclock_sample,
            "random_seed": self.random_seed,
            "harness_version": self.harness_version,
        }

    @classmethod
    def from_dict(cls, d: dict) -> RunConfig:
        """Read back a `config.json` `config` block, exactly as `to_dict` wrote it.

        The round trip is what lets a published result be re-derived from its own
        committed records without re-running the benchmark — the config that
        stamped the numbers is read, never re-probed, so a rebuild on a different
        machine cannot quietly restamp them with today's tool versions.
        """
        return cls(
            corpus_digest=d["corpus_digest"],
            rtdd_version=d["rtdd_version"],
            tool_versions=tuple(sorted((k, v) for k, v in d["tool_versions"].items())),
            strategies=tuple(d["strategies"]),
            variants=tuple(d["variants"]),
            replay_commits=int(d["replay_commits"]),
            wallclock_sample=int(d["wallclock_sample"]),
            random_seed=int(d["random_seed"]),
            harness_version=d["harness_version"],
        )

    def digest(self) -> str:
        return hashlib.sha256(canonical_json(self.to_dict()).encode("utf-8")).hexdigest()
