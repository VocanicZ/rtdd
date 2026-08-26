"""The vocabulary every published number is expressed in.

A benchmark number is meaningless without its denominator, so no metric here ever
returns a bare float: :class:`Ratio` carries `num` and `den` and answers `None`
rather than `0.0` when `den == 0`, because "no commit qualified" and "every
qualifying commit scored zero" are different findings and a float cannot tell
them apart.

:func:`assert_single_repo` enforces the plan's never-pool rule at the only place
it can be enforced — the function that turns records into a number. A pooled
recall figure is dominated by whichever repo has the most commits, which is a
property of the corpus rather than of the tool; cross-repo figures exist only as
explicitly duration-weighted aggregates, printed in their own labelled table.

Recall, strata, selection ratio, duration fraction and escalation join this module
with Task 20; the false-signal metrics live in ``replay.falsesignal``.
"""

from __future__ import annotations

import dataclasses
from collections.abc import Sequence


class PoolingError(RuntimeError):
    """Raised when records from more than one `repo_id` reach one aggregate."""


@dataclasses.dataclass(frozen=True)
class Ratio:
    num: float
    den: float

    def value(self) -> float | None:
        return None if self.den == 0 else self.num / self.den

    def to_dict(self) -> dict:
        return {"num": self.num, "den": self.den, "value": self.value()}


def assert_single_repo(records: Sequence) -> str:
    """Return the one `repo_id` these records share, or refuse. Empty is `""`."""
    ids = {r.repo_id for r in records}
    if len(ids) > 1:
        raise PoolingError(
            f"refusing to pool across repos: {sorted(ids)}. Spec §10 forbids pooled "
            f"recall; use the duration-weighted aggregate instead."
        )
    return next(iter(ids)) if ids else ""
