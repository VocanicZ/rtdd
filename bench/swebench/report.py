"""Render ``summary.json`` into the tables that go in the README and docs/results.

Two of the PRD's guarantees are enforced here, at the point of publication where
they are easiest to forget:

  1. **A regression rate is never emitted without a resolution rate in the same
     row.** An arm that prevents regressions by solving fewer problems must be
     legible as such at a glance. The check is not a convention the rows happen
     to follow: :func:`assert_resolution_beside_regression` re-reads the markdown
     the renderer is about to return and refuses any table that names a
     regression without naming resolution beside it. Dropping the column is a
     crash, not a quieter table.
  2. **A divergent vanilla arm strips the published column.** When the local
     control falls outside the pre-registered equivalence band, TDAD's figures
     are absent from the header, the separator and every row — not footnoted,
     not blanked — and the document opens with a ``HARNESS DIVERGENCE``
     blockquote carrying the note that names both rates.

Rates are rendered at a fixed precision so two runs of the same benchmark diff
cleanly against each other rather than dissolving into float noise.

``write_config`` records what the numbers came from: the RTDD commit under test,
the TDAD pin arm C actually ran, the model and its decoding settings, the frozen
sample, the host the wall-clock figures belong to, and how many instances were
seeded. It refuses to write a config claiming a model other than the
pre-registered one — an arm run against a different model is not this benchmark.
"""

from __future__ import annotations

import hashlib
import json
import platform
import re
import subprocess
from pathlib import Path

import evaluate
from agent import AgentConfig
from analyze import SEEDED
from preflight import preflight
from providers.tdad import TDAD_PIN

#: The arms, in the order the pre-registration lists them. Fixed so a re-run
#: produces the same rows in the same places.
ARM_ORDER: tuple[str, ...] = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")

ARM_LABEL: dict[str, str] = {
    "vanilla": "vanilla",
    "tdd": "TDD procedural prose",
    "tdad": "TDAD static graph (+ prose)",
    "rtdd": "RTDD dynamic coverage",
    "rtdd_tdd": "RTDD dynamic coverage (+ prose)",
}

#: The header cell whose presence licenses a regression column. Named once so
#: the renderer and the audit cannot drift apart.
RESOLUTION_COLUMN = "resolved"

#: A header cell that cites a regression rate, and one that cites resolution.
#: The audit is a text check on purpose: it sees what a reader sees.
_REGRESSION_HEADER = re.compile(r"regression", re.I)
_RESOLUTION_HEADER = re.compile(r"resolv", re.I)

#: Percentages to two decimals, resolution to one: enough to separate arms,
#: few enough that noise in the last digit does not churn the diff.
_RATE = "{:.2f}%"
_RESOLUTION = "{:.1f}%"


class ReportError(RuntimeError):
    """The document the renderer produced is not publishable as written."""


def _rate(value: float) -> str:
    return _RATE.format(value * 100)


def _resolution(value: float) -> str:
    return _RESOLUTION.format(value * 100)


def _tables(text: str) -> list[list[list[str]]]:
    """Every markdown table in ``text``, as lists of cell lists."""
    out: list[list[list[str]]] = []
    current: list[list[str]] | None = None
    for line in text.splitlines():
        if line.startswith("|"):
            cells = [c.strip() for c in line.strip().strip("|").split("|")]
            current = [cells] if current is None else [*current, cells]
        elif current is not None:
            out.append(current)
            current = None
    if current is not None:
        out.append(current)
    return out


def assert_resolution_beside_regression(text: str) -> None:
    """Refuse any table that cites a regression rate without a resolution rate.

    Run by :func:`render` on its own output, so the guarantee holds for whatever
    the renderer grows into rather than only for the tables it emits today.
    """
    for table in _tables(text):
        header = table[0]
        if not any(_REGRESSION_HEADER.search(cell) for cell in header):
            continue
        if not any(_RESOLUTION_HEADER.search(cell) for cell in header):
            raise ReportError(
                f"table with header {header} cites a regression rate with no "
                "resolution rate beside it — an arm that regresses less by "
                "solving less must be legible as such in its own row"
            )


def render(summary: dict) -> str:
    """Turn ``summary.json`` into the published markdown. Pure."""
    reproduces = summary["vanilla_equivalence"]["reproduces"]
    published = summary["tdad_published"] or {}
    lines: list[str] = []

    if not reproduces:
        lines += [
            "> **HARNESS DIVERGENCE**",
            ">",
            f"> {summary['vanilla_equivalence']['note']}",
            "",
        ]

    columns = [
        "arm",
        "regressions (test-level)",
        "95% CI",
        "regressions (instance-level)",
        RESOLUTION_COLUMN,
    ]
    if reproduces:
        columns.append("TDAD published")
    lines += [_row(columns), _separator(len(columns))]

    for arm in ARM_ORDER:
        s = summary["arms"].get(arm)
        if s is None:
            continue
        lo, hi = s["test_ci"]
        cells = [
            ARM_LABEL[arm],
            _rate(s["test_level"]),
            f"[{_rate(lo)}, {_rate(hi)}]",
            _rate(s["instance_level"]),
            _resolution(s["resolution"]),
        ]
        if reproduces:
            cited = published.get(arm)
            cells.append(_rate(cited) if cited is not None else "—")
        lines.append(_row(cells))

    detail = ["arm", "n", "P2P failed / total", "unapplied patches", "harness errors"]
    lines += ["", _row(detail), _separator(len(detail))]
    for arm in ARM_ORDER:
        s = summary["arms"].get(arm)
        if s is None:
            continue
        lines.append(
            _row(
                [
                    ARM_LABEL[arm],
                    str(s["n"]),
                    f"{s['p2p_failed']} / {s['p2p_total']}",
                    str(s["unapplied"]),
                    str(s["harness_errors"]),
                ]
            )
        )

    pairwise = [
        "comparison",
        "regression delta (test-level)",
        "CIs disjoint",
        f"{RESOLUTION_COLUMN} (a → b)",
        "verdict",
    ]
    lines += ["", "## Pairwise", "", _row(pairwise), _separator(len(pairwise))]
    for c in summary["comparisons"]:
        lines.append(
            _row(
                [
                    f"{ARM_LABEL.get(c['a'], c['a'])} vs {ARM_LABEL.get(c['b'], c['b'])}",
                    f"{c['delta'] * 100:+.2f} pp",
                    "yes" if c["ci_disjoint"] else "no",
                    f"{_resolution(c['a_resolution'])} → {_resolution(c['b_resolution'])}",
                    c["verdict"],
                ]
            )
        )

    kc = summary["kill_criterion"]
    lines += [
        "",
        "## Pre-registered kill criterion",
        "",
        f"Floor (signed before the run): **{kc['floor']:.4f}** stratified "
        "change-level recall on the `|F_full| == 1` stratum.",
        f"Observed in M3: **{kc['observed_single_killer_recall']:.4f}**.",
        f"Result: **{'MET' if kc['met'] else 'NOT MET'}**.",
        "",
        f"TDAD figures, where shown, are cited from {summary['tdad_citation']}.",
    ]

    text = "\n".join(lines) + "\n"
    assert_resolution_beside_regression(text)
    return text


def _row(cells: list[str]) -> str:
    return "| " + " | ".join(cells) + " |"


def _separator(width: int) -> str:
    return "|" + "---|" * width


def seed_counts(repo_root: Path, arm: str) -> dict[str, int]:
    """How many of this arm's instances ran with a usable map, and how many not.

    Both numbers are published. An arm whose seeding failed still ran, with a map
    that selects nothing, and reporting only the successes would flatter it.
    """
    raw = evaluate.results_dir(repo_root) / "raw" / arm
    counts = {"seeded": 0, "seed_failed": 0}
    if not raw.exists():
        return counts
    for path in sorted(raw.glob("*.json")):
        status = json.loads(path.read_text(encoding="utf-8")).get("seed_status")
        if status is None:
            continue
        counts["seeded" if status == SEEDED else "seed_failed"] += 1
    return counts


def write_config(repo_root: Path) -> Path:
    """Record what the published numbers came from, into ``config.json``."""
    pr = preflight(repo_root)
    cfg = AgentConfig()
    if cfg.model != str(pr.fields["model"]):
        raise ReportError(
            f"the agent is configured for {cfg.model!r} but the pre-registered "
            f"model is {pr.fields['model']!r} — a run against another model is "
            "not this benchmark and may not be published as it"
        )

    instances = repo_root / "bench" / "swebench" / "instances.txt"
    config = {
        "rtdd_commit": _git(repo_root, "rev-parse", "HEAD"),
        "tdad_pin": TDAD_PIN,
        "model": {
            "id": cfg.model,
            "endpoint": cfg.base_url,
            "temperature": cfg.temperature,
            "max_tokens": cfg.max_tokens,
            "max_turns": cfg.max_turns,
        },
        "sample": {
            "seed": int(pr.fields["sample_seed"]),
            "size": int(pr.fields["sample_size"]),
            "instance_list_sha256": hashlib.sha256(instances.read_bytes()).hexdigest(),
        },
        "host": {
            "platform": platform.platform(),
            "processor": platform.processor(),
            "python": platform.python_version(),
        },
        "seeding": {arm: seed_counts(repo_root, arm) for arm in ARM_ORDER},
        "note": (
            "Wall-clock figures come from this machine, never from a CI runner "
            "(spec §10)."
        ),
    }
    dest = evaluate.results_dir(repo_root) / "config.json"
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(config, indent=2), encoding="utf-8")
    return dest


def _git(repo_root: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(repo_root), *args],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    summary = json.loads(
        (evaluate.results_dir(root) / "summary.json").read_text(encoding="utf-8")
    )
    out = evaluate.results_dir(root) / "tables.md"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(render(summary), encoding="utf-8")
    config = write_config(root)
    print(f"wrote {out}")
    print(f"wrote {config}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
