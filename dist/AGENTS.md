<!-- BEGIN rtdd (generated from protocol/PROTOCOL.md; do not edit here) -->
## rtdd

If `.rtdd/map.jsonl` does not exist, every selection is the full suite. Run `rtdd seed` once
and commit the map; `rtdd run` keeps it fresh after that.

`rtdd` reports which tests cover code you changed, and which changed lines nothing covers,
from recorded coverage rather than a static graph. It reports; it never gates.

`rtdd which` prints the ranked tests covering your current changes plus the uncovered
report, and runs nothing. Untracked files count, so a file you just wrote is included.
`rtdd which --json` is the machine-readable form.

`rtdd run` runs the selection and prints the uncovered report. Exit non-zero means a test
failed, and nothing else — an empty selection and an uncovered report are both exit 0.

The uncovered report splits changed lines into covered and uncovered: a line is covered
when some test file's own run executed it this cycle.

An empty selection is reported as its own outcome, never as a pass.

<!-- END rtdd -->
