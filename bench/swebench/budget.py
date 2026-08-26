"""Hard ceilings. A benchmark that can run away is a benchmark that will.

Two ceilings, both absolute: dollars and wall-clock hours. The dollar figure is
an *estimate* — a local endpoint bills nothing, so the price fields are the
assumptions under which the spend was computed, and :meth:`Budget.snapshot`
writes them out beside the token counts they were applied to. A cost file that
named only a dollar figure could not be re-derived by a reader who disagreed
with the price.

:meth:`Budget.add` records before it refuses. The tokens that crossed the cap
were really spent, and the cost file written on the way out has to say so.

Both ceilings are ceilings on the *benchmark*, not on one invocation of it. A
resumed arm calls :meth:`Budget.restore` with the cost file its previous run
left behind, so the spend accumulates across crashes instead of starting again
from zero each time — otherwise ``--max-usd 75`` would authorise $75 per
restart, and the cost file rewritten on the way out would erase the spend it is
supposed to record.
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
    #: Wall-clock hours carried over from the runs this one is resuming.
    prior_wall_hours: float = 0.0

    def start(self) -> None:
        self.started_at = time.time()

    def restore(self, snapshot: dict) -> None:
        """Adopt a previous run's spend, so a resume tops it up rather than restarting it.

        Takes a :meth:`snapshot` — the cost file the previous run wrote — and
        adds its counts to this budget's. A missing key reads as zero: a cost
        file that predates a field is a partial record, not a reason to refuse
        to resume.
        """
        self.prompt_tokens += int(snapshot.get("prompt_tokens", 0) or 0)
        self.completion_tokens += int(snapshot.get("completion_tokens", 0) or 0)
        self.prior_wall_hours += float(snapshot.get("wall_hours", 0.0) or 0.0)

    def check(self) -> None:
        """Refuse if either ceiling is already crossed.

        Called before the next instance as well as after one, so a resume that
        is already over the cap spends nothing more rather than one more
        instance's worth.
        """
        if self.usd > self.max_usd:
            raise BudgetExceeded(f"spend ${self.usd:.2f} exceeded cap ${self.max_usd:.2f}")
        hours = self.wall_hours
        if hours > self.max_wall_hours:
            raise BudgetExceeded(f"wall clock {hours:.1f}h exceeded cap {self.max_wall_hours}h")

    def add(self, prompt_tokens: int, completion_tokens: int) -> None:
        """Account for one instance's tokens, then refuse if either ceiling is crossed."""
        self.prompt_tokens += prompt_tokens
        self.completion_tokens += completion_tokens
        self.check()

    @property
    def usd(self) -> float:
        return (
            self.prompt_tokens / 1_000_000 * self.usd_per_m_prompt
            + self.completion_tokens / 1_000_000 * self.usd_per_m_completion
        )

    @property
    def wall_hours(self) -> float:
        """This run's elapsed hours plus whatever the runs it resumed already burned.

        Zero elapsed until :meth:`start`, so an unstarted budget reads as
        unspent, not as decades.
        """
        elapsed = 0.0 if not self.started_at else (time.time() - self.started_at) / 3600.0
        return self.prior_wall_hours + elapsed

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
