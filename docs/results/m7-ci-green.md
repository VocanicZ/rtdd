# The hosted CI run that proves `main` is green — 2026-09-09

`.github/workflows/ci.yml` was `state: disabled_manually` from 2026-08-26 onward. Its
last runs that day all failed, and every one of them failed the same way: the job was
never started at all. The annotation on run `32967317835` reads

> The job was not started because recent account payments have failed or your spending
> limit needs to be increased. Please check the 'Billing & plans' section in your settings

so the workflow was switched off rather than fixed, and **every commit of M6a through M6e
merged with no CI at all**. The file stayed in the tree the whole time, which is exactly
why the state below is read from the API and not inferred from the file's presence.

Three things were done to get the run recorded here.

1. **The workflow was re-enabled** — `PUT
   /repos/VocanicZ/rtdd/actions/workflows/342815183/enable`. The billing condition that
   stopped the 2026-08-26 runs no longer holds: run `34399178363`, the first run after
   the enable, started and executed its steps.
2. **`ci.yml` gained a `workflow_dispatch:` trigger.** Nothing was removed or narrowed;
   `pull_request` and `push` to `main` are untouched. It exists so the next proof of the
   workflow does not need a commit invented to push at it.
3. **The `prereg` job gained a `rtdd on PATH` step.** Run `34399178363` was red on its
   `bench replay suite` step:

   ```
   replay.rtddio.RtddError: rtdd binary not found or not executable: 'rtdd'
   ([Errno 2] No such file or directory: 'rtdd')
   ```

   `bench/tests/test_replay.py::test_a_base_tree_rtdd_refuses_to_seed_is_skipped_not_fatal`
   drives the *shipped* binary through `bench/replay/rtddio.py`, which shells out to
   `rtdd` on `PATH`. A developer box has one installed and a bare runner does not, so the
   failure was **environmental, not a defect**: the suite failed for want of a tool. The
   fix builds the binary from the same checkout the tests run against — the tool under
   test is the tree under test — and skips nothing. `526 passed` became `527 passed`.

No other step changed. No gate was removed, narrowed or made to skip.

## The cold-cache facts

The bench steps ran with **no `RTDD_BENCH_CACHE`** — the workflow does not set it and
does not mention it, which `TestCIWorkflowPrimesNoBenchCache` keeps true — and with
**zero bytes of the ~201 MB replay store restored**. Nothing in `ci.yml` restores
`~/.cache/rtdd-bench`; the one cache in the job is `astral-sh/setup-uv`'s wheel cache,
which is a Python package cache and not the replay store. #218 keeps that store off
limits, and this run is the proof the hosted gate never needed it.

## The record

```json
{
  "checked_with": "gh api repos/VocanicZ/rtdd/actions/workflows --jq '.workflows[] | select(.path==\".github/workflows/ci.yml\")'",
  "workflow": {
    "path": ".github/workflows/ci.yml",
    "state": "active",
    "id": 342815183
  },
  "run": {
    "id": 34399975325,
    "url": "https://github.com/VocanicZ/rtdd/actions/runs/34399975325",
    "head_sha": "f55503e7b89da9ad7df70d5c33126922ccde8227",
    "head_branch": "main",
    "event": "push",
    "jobs": {
      "test": "success",
      "prereg": "success"
    },
    "bench_steps": {
      "rtdd on PATH": "success",
      "prereg gate": "success",
      "arm-composition gate": "success",
      "acceptance gate": "success",
      "arm-driver dry run": "success",
      "corpus admission gate": "success",
      "bench replay suite": "success"
    },
    "cache": {
      "RTDD_BENCH_CACHE": "unset",
      "restored_bytes": 0
    }
  }
}
```

Read back with:

```bash
gh api repos/VocanicZ/rtdd/actions/workflows --jq '.workflows[] | select(.path==".github/workflows/ci.yml")'
gh run view 34399975325 -R VocanicZ/rtdd --json databaseId,conclusion,headSha,headBranch,event,url
gh run view 34399975325 -R VocanicZ/rtdd --json jobs --jq '.jobs[] | {name, conclusion}'
gh run view 34399975325 -R VocanicZ/rtdd --json jobs --jq '.jobs[] | select(.name=="prereg") | .steps[] | {name, conclusion}'
```
