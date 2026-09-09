"""Prove the bench cache is cold before a gate claims it ran without one.

`.github/workflows/ci.yml` runs the bench gates on a hosted runner that has never
seen `RTDD_BENCH_CACHE` and has no `~/.cache/rtdd-bench` — the store of collected
suites, full-suite outcomes and repo mirrors that every local run has had. Reproducing
that state locally is easy to get wrong in a way nothing reports: point the variable at
an empty directory, run the gates, watch them pass, and conclude the runner is fine —
while `bench/swebench/run_arm.py` was reading the developer's warm store the whole time
through `Path.home()`, which the variable does not move.

That failure is silent by construction, so it gets an explicit check that runs *before*
the gates. Two stores must be cold:

``replay``
    ``RTDD_BENCH_CACHE`` if set, else ``bench/cache/`` — what :data:`replay.cli.CACHE`
    resolves to and what :class:`replay.cache.Cache` is rooted at.
``driver``
    ``~/.cache/rtdd-bench`` — ``run_arm.py``'s ``CACHE_ROOT``, reached through ``HOME``
    alone. ``bench/swebench`` is a separate uv project, so the literal is mirrored here
    and `tests/test_coldcache.py` reads the driver's source to keep the mirror honest.

Cold means *no entries*, not *no directory*: a store that exists and is empty is cold.
This module never creates, reads into, or writes either store — it only counts files
under them. Deleting, re-keying or fingerprinting the real store is #218, an open human
decision, and nothing here touches it.
"""

from __future__ import annotations

import argparse
import dataclasses
import os
import pathlib
from collections.abc import Mapping, Sequence

BENCH = pathlib.Path(__file__).resolve().parents[1]
"""``bench/`` — the same anchor :mod:`replay.cli` uses for its fallback cache path."""

#: The environment variable that moves the replay store, and only the replay store.
ENV_VAR = "RTDD_BENCH_CACHE"

#: What a state records when the variable is not set, or set to the empty string. Both
#: mean the same thing to :mod:`replay.cli` — the fallback path — and the word is the
#: one the CI evidence uses, so `""` is never reported as if it were a configured path.
UNSET = "unset"


@dataclasses.dataclass(frozen=True)
class Store:
    """One cache store: what reads it, where it is, and how many entries it holds."""

    name: str
    path: pathlib.Path
    entries: int

    @property
    def cold(self) -> bool:
        return self.entries == 0


@dataclasses.dataclass(frozen=True)
class CacheState:
    """Every store a bench gate could reach, as one process saw them."""

    env: str
    stores: tuple[Store, ...]

    @property
    def cold(self) -> bool:
        return all(s.cold for s in self.stores)


def replay_root(env: Mapping[str, str], bench: pathlib.Path = BENCH) -> pathlib.Path:
    """The store :mod:`replay.cli` would use, resolved exactly as it resolves it."""
    return pathlib.Path(env.get(ENV_VAR) or bench / "cache").expanduser()


def driver_root(env: Mapping[str, str]) -> pathlib.Path:
    """The store ``bench/swebench/run_arm.py`` would use — ``HOME``, never ``ENV_VAR``."""
    home = env.get("HOME") or str(pathlib.Path.home())
    return pathlib.Path(home).expanduser() / ".cache" / "rtdd-bench"


def _entries(root: pathlib.Path) -> int:
    """Files under ``root``, or 0 when it does not exist. Never creates anything."""
    if not root.is_dir():
        return 0
    return sum(1 for p in root.rglob("*") if p.is_file())


def probe(env: Mapping[str, str] | None = None, bench: pathlib.Path = BENCH) -> CacheState:
    """Read both stores without touching either."""
    env = os.environ if env is None else env
    return CacheState(
        env=env.get(ENV_VAR) or UNSET,
        stores=(
            Store("replay", replay_root(env, bench), _entries(replay_root(env, bench))),
            Store("driver", driver_root(env), _entries(driver_root(env))),
        ),
    )


def findings(state: CacheState) -> list[str]:
    """One line per store that is not cold. Empty when the state is what CI has."""
    return [
        f"{s.name} cache {s.path} holds {s.entries} entr{'y' if s.entries == 1 else 'ies'}; "
        f"a cold-cache run must not be able to read it"
        for s in state.stores
        if not s.cold
    ]


def main(argv: Sequence[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--home", default=None, help="check this HOME instead of the caller's")
    parser.add_argument(
        "--replay-cache",
        default=None,
        help=f"check this path instead of the caller's {ENV_VAR}",
    )
    args = parser.parse_args(argv)

    env = dict(os.environ)
    if args.home is not None:
        env["HOME"] = args.home
    if args.replay_cache is not None:
        env[ENV_VAR] = args.replay_cache

    state = probe(env)
    print(f"{ENV_VAR}: {state.env}")
    for store in state.stores:
        print(f"{store.name} cache: {store.path} — {store.entries} entries, "
              f"{'cold' if store.cold else 'WARM'}")

    found = findings(state)
    for line in found:
        print(f"NOT COLD: {line}")
    if found:
        print("refusing: the bench gates would not be proving anything on this machine")
        return 1
    print("cold: both bench cache stores are empty")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
