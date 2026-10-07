"""Schema-3 test ids (``file::name``) turned into the ids pytest collected.

``rtdd which --json`` schema 3 names graph test nodes — ``<file>::<qualified name>``,
with ``@<line>`` appended when the scanner disambiguates a repeated definition.
The bench scores recall and cost per collected pytest id, and ``validate_selection``
refuses an id outside ``all_tests``, so every graph id is matched onto collected ids
here and nowhere else.

Matching is by **key**: a collected id keyed with its last segment's ``[params]``
removed, a graph id keyed with its ``@<line>`` removed. One graph id is therefore
every case of a parametrised function, a class-nested test keeps its class path,
and two files with a same-named test never meet.

A graph id that keys no collected id is a helper, a fixture, or a test pytest did
not collect at this tree. It is returned for counting and never widened to "every
test in its file": that would turn a missed link into a hit, which is tuning the
selection by the back door.
"""

from __future__ import annotations

import re
from collections.abc import Sequence

_PARAMS = re.compile(r"\[.*\]$")
_DUPLICATE = re.compile(r"@\d+$")


def _collected_key(test_id: str) -> str | None:
    file, sep, rest = test_id.partition("::")
    if not sep:
        return None
    # pytest's parametrisation suffix only ever sits on the last segment, and the
    # brackets may themselves contain `::`, so it is stripped from the whole rest.
    return f"{file}::{_PARAMS.sub('', rest)}"


def expand_graph_ids(
    graph_ids: Sequence[str], all_tests: Sequence[str]
) -> tuple[tuple[str, ...], tuple[str, ...]]:
    """(collected ids the graph ids name, graph ids that name no collected id).

    Both tuples keep first-seen order and are de-duplicated, so the union of two
    rounds never charges a test twice.
    """
    by_key: dict[str, list[str]] = {}
    for t in all_tests:
        key = _collected_key(t)
        if key is not None:
            by_key.setdefault(key, []).append(t)

    selected: list[str] = []
    unmatched: list[str] = []
    for gid in graph_ids:
        ids = by_key.get(_DUPLICATE.sub("", gid))
        if ids:
            selected.extend(ids)
        else:
            unmatched.append(gid)
    return tuple(dict.fromkeys(selected)), tuple(dict.fromkeys(unmatched))
