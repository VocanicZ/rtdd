<!-- BEGIN rtdd (generated from protocol/PROTOCOL.md; do not edit here) -->
## rtdd

`rtdd` reports which tests cover code you changed, and which changed lines nothing covers,
from recorded coverage rather than a static graph. It reports; it never gates.

`rtdd which` prints the ranked tests covering your current changes plus the uncovered
report, and runs nothing. Untracked files count, so a file you just wrote is included.
`rtdd which --json` is the machine-readable form.

`rtdd run` runs the selection and prints the uncovered report. Exit non-zero means a test
failed, and nothing else — an empty selection and an uncovered report are both exit 0.

The uncovered report splits changed lines into covered, uncovered, and import-time.
Import-time lines execute during collection and are attributed to no test — they are
reported separately and are not a coverage gap.

An empty selection is reported as its own outcome, never as a pass.

<!-- END rtdd -->
