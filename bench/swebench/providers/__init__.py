"""Per-arm context providers.

An arm differs from its control by its system prompt and by the extra context tool
appended here — never by a branch inside a shared tool. ``context_tools`` is the one
place that mapping lives, and it also returns the per-instance provenance the raw
result records, so a published rate can be read back against how its context was
actually built.
"""

from __future__ import annotations

from pathlib import Path

from tools import Tool
from workspace import Workspace

from . import rtdd as rtdd_provider

#: The arms that carry an RTDD map. Both share one seed cache.
RTDD_ARMS = ("rtdd", "rtdd_tdd")

CONTEXT_TOOL_ARMS = {"tdad", *RTDD_ARMS}


def context_tools(arm: str, ws: Workspace, cache: Path, row: dict) -> tuple[list[Tool], dict]:
    """Return ``(extra tools, per-instance provenance to record in the raw result)``."""
    if arm == "tdad":
        # Imported lazily: arms D and E must not be blocked on arm C's provider
        # existing, and a TDAD install failure is arm C's contingency, not theirs.
        from . import tdad as tdad_provider

        tdad_provider.index(ws)
        return [tdad_provider.impact_tool(ws)], {"tdad_pin": tdad_provider.TDAD_PIN}
    if arm in RTDD_ARMS:
        seed, status = rtdd_provider.ensure_seed(cache, ws, row)
        installed = rtdd_provider.install_seed(ws, seed)
        return (
            [rtdd_provider.which_tool(ws)],
            {"seed_status": status, "seed_installed": installed},
        )
    return [], {}
