"""Per-arm context providers.

An arm differs from its control by its system prompt and by the extra context tool
appended here — never by a branch inside a shared tool. ``context_tools`` is the one
place that mapping lives, and it also returns the per-instance provenance the raw
result records, so a published rate can be read back against how its context was
actually built.

The arm name is checked against the pre-registered five rather than being allowed to
fall through. A typo that returned "no extra tool" would run silently and publish as
a real arm, which is the one failure mode this registry can produce that nothing
downstream could detect.
"""

from __future__ import annotations

from pathlib import Path

from tools import Tool
from workspace import Workspace

from . import rtdd as rtdd_provider

#: The arms of the pre-registration, in the order its table lists them.
ARMS = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")

#: The arms that carry an RTDD map. Both share one seed cache.
RTDD_ARMS = ("rtdd", "rtdd_tdd")

CONTEXT_TOOL_ARMS = {"tdad", *RTDD_ARMS}


class UnknownArm(ValueError):
    """An arm that is not one of the pre-registered five."""


def context_tools(arm: str, ws: Workspace, cache: Path, row: dict) -> tuple[list[Tool], dict]:
    """Return ``(extra tools, per-instance provenance to record in the raw result)``."""
    if arm not in ARMS:
        raise UnknownArm(f"{arm!r} is not a pre-registered arm; expected one of {list(ARMS)}")
    if arm == "tdad":
        # Imported lazily: arms D and E must not be blocked on arm C's provider
        # existing, and a TDAD install failure is arm C's contingency, not theirs.
        from . import tdad as tdad_provider

        # TdadUnavailable is deliberately not caught. Arm C without TDAD's map is
        # arm A wearing arm C's name, and publishing that as a measured arm is the
        # one outcome the contingency exists to prevent.
        pin = tdad_provider.ensure_installed(cache)
        tdad_provider.index(ws)
        return [tdad_provider.impact_tool(ws)], {"tdad_pin": pin}
    if arm in RTDD_ARMS:
        seed, status = rtdd_provider.ensure_seed(cache, ws, row)
        installed = rtdd_provider.install_seed(ws, seed)
        return (
            [rtdd_provider.which_tool(ws)],
            {"seed_status": status, "seed_installed": installed},
        )
    return [], {}
