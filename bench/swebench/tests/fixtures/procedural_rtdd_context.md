A tool named rtdd_which is available in this repository.

Given the repository's current uncommitted changes, it reports two things, derived from coverage recorded during a prior full run of this suite:

- the tests whose recorded execution touched the files you have changed, ranked by how much of each test's recorded file set the change overlaps;
- the lines you have changed that no recorded test executed, and separately the changed lines that executed only at import time and are attributed to no test.

Follow a test-driven workflow with it. First, run rtdd_which and read the ranked tests. Before changing any source file, write a test that fails because of the described problem. Then run the ranked tests again and make sure none of them regressed. Do not skip the failing-test step.
