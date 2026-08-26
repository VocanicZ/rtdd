"""Hard ceilings. A benchmark that can run away is a benchmark that will.

Two ceilings, both absolute: dollars and wall-clock hours. The dollar figure is
an *estimate* — a local endpoint bills nothing, so the price fields are the
assumptions under which the spend was computed, and :meth:`Budget.snapshot`
writes them out beside the token counts they were applied to. A cost file that
named only a dollar figure could not be re-derived by a reader who disagreed
with the price.

:meth:`Budget.add` records before it refuses. The tokens that crossed the cap
were really spent, and the cost file written on the way out has to say so.
"""

from __future__ import annotations

import time
from dataclasses import dataclass


class BudgetExceeded(RuntimeError):
    """A ceiling was crossed; the run stops here."""


@dataclass
class Budget:
    #: Ceiling on estimated spend, in dollars.
    max_usd: float = 75.0
    #: Assumed price per million prompt tokens.
    usd_per_m_prompt: float = 0.10
    #: Assumed price per million completion tokens.
    usd_per_m_completion: float = 0.40
    #: Ceiling on wall-clock hours since :meth:`start`.
    max_wall_hours: float = 60.0
    started_at: float = 0.0
    prompt_tokens: int = 0
    completion_tokens: int = 0

    def start(self) -> None:
        self.started_at = time.time()

    def add(self, prompt_tokens: int, completion_tokens: int) -> None:
        """Account for one instance's tokens, then refuse if either ceiling is crossed."""
        self.prompt_tokens += prompt_tokens
        self.completion_tokens += completion_tokens
        if self.usd > self.max_usd:
            raise BudgetExceeded(f"spend ${self.usd:.2f} exceeded cap ${self.max_usd:.2f}")
        hours = self.wall_hours
        if hours > self.max_wall_hours:
            raise BudgetExceeded(f"wall clock {hours:.1f}h exceeded cap {self.max_wall_hours}h")

    @property
    def usd(self) -> float:
        return (
            self.prompt_tokens / 1_000_000 * self.usd_per_m_prompt
            + self.completion_tokens / 1_000_000 * self.usd_per_m_completion
        )

    @property
    def wall_hours(self) -> float:
        """Zero until :meth:`start`, so an unstarted budget reads as unspent, not as decades."""
        if not self.started_at:
            return 0.0
        return (time.time() - self.started_at) / 3600.0

    def snapshot(self) -> dict:
        """The cost record: the assumed prices beside the counts they were applied to."""
        return {
            "prompt_tokens": self.prompt_tokens,
            "completion_tokens": self.completion_tokens,
            "usd_estimated": round(self.usd, 4),
            "usd_per_m_prompt": self.usd_per_m_prompt,
            "usd_per_m_completion": self.usd_per_m_completion,
            "max_usd": self.max_usd,
            "max_wall_hours": self.max_wall_hours,
            "wall_hours": round(self.wall_hours, 3),
        }
