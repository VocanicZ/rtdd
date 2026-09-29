<!-- BEGIN rtdd (generated from protocol/PROTOCOL.md; do not edit here) -->
## rtdd

Check for `.rtdd/map.jsonl` first. If it exists, rtdd is set up — use the commands below. If
it does not, run `rtdd init` then `rtdd seed` once, and commit the map. `rtdd init` exiting 2
is the no-adapter refusal: nothing matched a toolchain rtdd can instrument, so add the
adapter it names or re-run with `--force`.

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
