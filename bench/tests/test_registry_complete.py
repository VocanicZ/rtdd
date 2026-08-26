"""PRD #4's completeness gate, made mechanical.

The PRD promises RTDD is scored against seven named baselines. A baseline that
quietly stops being registered would not fail any other test — every per-strategy
test imports its own module — so the suite would stay green while the published
comparison lost a competitor. This test is the one place that fails for it.

The assertion is exact equality, not a superset: an eighth entry appearing here
means the registry grew a strategy the PRD does not describe, and the PRD and
this list have to move together.
"""

from __future__ import annotations

import replay.strategies.full  # noqa: F401
import replay.strategies.importgraph  # noqa: F401
import replay.strategies.lastfailed  # noqa: F401
import replay.strategies.pathheuristic  # noqa: F401
import replay.strategies.randomratio  # noqa: F401
import replay.strategies.rtdd  # noqa: F401
import replay.strategies.testmon  # noqa: F401
import replay.strategies.xdist  # noqa: F401
from replay.strategies.base import all_ids

# The seven baselines PRD #4 names, plus the system under test.
REQUIRED_BASELINES = frozenset(
    {
        "testmon",  # 1 — method-level AST checksums
        "path",  # 2 — naive path heuristic
        "lf",  # 3 — pytest --lf
        "importgraph",  # 4 — static import graph
        "xdist",  # 5 — parallel full suite
        "random",  # 6 — random ratio-matched sample
        "full",  # 7 — the full suite
    }
)
SYSTEM_UNDER_TEST = "rtdd"
REQUIRED = REQUIRED_BASELINES | {SYSTEM_UNDER_TEST}


def test_every_required_baseline_is_registered():
    missing = REQUIRED - set(all_ids())
    assert not missing, f"PRD #4 baselines missing from the registry: {sorted(missing)}"


def test_the_registry_holds_exactly_the_required_strategies():
    assert set(all_ids()) == REQUIRED
