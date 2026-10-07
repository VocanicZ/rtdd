<!-- BEGIN rtdd (generated from protocol/PROTOCOL.md; do not edit here) -->
## rtdd

rtdd names the tests a change needs, in rounds: Round 1 tests the code you changed, Round 2
its direct neighbours, Round 3 is the full suite once at the end. rtdd runs no tests: you
run each round with the project's own test command.

1. Edit code (test first, per TDD).
2. Run `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's own test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.

`rtdd which` prints the changed nodes, Rounds 1–3 and `untested`, and runs nothing.
Untracked files count. `rtdd which --json` is the machine-readable form.

Empty Rounds 1 and 2 print `no linked test`: no linked test, never a pass.

<!-- END rtdd -->
