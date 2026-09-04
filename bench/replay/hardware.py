from __future__ import annotations

import dataclasses
import hashlib
import os
import pathlib
import platform
import sys
from collections.abc import Mapping

CI_ENV_VARS = (
    "CI",
    "GITHUB_ACTIONS",
    "GITLAB_CI",
    "BUILDKITE",
    "CIRCLECI",
    "JENKINS_URL",
    "TRAVIS",
    "TEAMCITY_VERSION",
    "TF_BUILD",
    "CODEBUILD_BUILD_ID",
    "DRONE",
    "APPVEYOR",
)

_FALSEY = {"", "0", "false", "no", "off"}


class CIWallClockRefused(RuntimeError):
    pass


def detect_ci(env: Mapping[str, str]) -> str | None:
    for name in CI_ENV_VARS:
        val = env.get(name)
        if val is not None and val.strip().lower() not in _FALSEY:
            return name
    return None


def _cpu_model() -> str:
    p = pathlib.Path("/proc/cpuinfo")
    if p.exists():
        for line in p.read_text(encoding="utf-8", errors="replace").splitlines():
            if line.lower().startswith("model name"):
                return line.split(":", 1)[1].strip()
    return platform.processor() or platform.machine() or "unknown"


def _mem_total_kb() -> int:
    p = pathlib.Path("/proc/meminfo")
    if p.exists():
        for line in p.read_text(encoding="utf-8", errors="replace").splitlines():
            if line.startswith("MemTotal:"):
                return int(line.split()[1])
    return 0


@dataclasses.dataclass(frozen=True)
class Hardware:
    cpu_model: str
    cpu_count: int
    mem_total_kb: int
    platform: str
    python_version: str
    ci: str | None

    def to_dict(self) -> dict:
        return {
            "cpu_model": self.cpu_model,
            "cpu_count": self.cpu_count,
            "mem_total_kb": self.mem_total_kb,
            "platform": self.platform,
            "python_version": self.python_version,
            "ci": self.ci,
            "fingerprint": self.fingerprint(),
        }

    @classmethod
    def from_dict(cls, d: dict) -> Hardware:
        """Read back a disclosed-hardware block. `fingerprint` is derived, never stored."""
        return cls(
            cpu_model=d["cpu_model"],
            cpu_count=int(d["cpu_count"]),
            mem_total_kb=int(d["mem_total_kb"]),
            platform=d["platform"],
            python_version=d["python_version"],
            ci=d["ci"],
        )

    def fingerprint(self) -> str:
        raw = (
            f"{self.cpu_model}|{self.cpu_count}|{self.mem_total_kb}|"
            f"{self.platform}|{self.python_version}"
        )
        return hashlib.sha256(raw.encode("utf-8")).hexdigest()[:16]

    def wallclock_allowed(self) -> bool:
        return self.ci is None


def probe(env: Mapping[str, str] | None = None) -> Hardware:
    e = os.environ if env is None else env
    return Hardware(
        cpu_model=_cpu_model(),
        cpu_count=os.cpu_count() or 1,
        mem_total_kb=_mem_total_kb(),
        platform=platform.platform(),
        python_version=sys.version.split()[0],
        ci=detect_ci(e),
    )


def require_wallclock(hw: Hardware) -> None:
    if not hw.wallclock_allowed():
        raise CIWallClockRefused(
            f"wall-clock measurement refused: {hw.ci} is set. Spec §10 permits wall-clock "
            f"numbers only from disclosed hardware, never from CI runners. Re-run on a "
            f"machine you can name, or pass --no-wallclock to compute the other metrics."
        )
