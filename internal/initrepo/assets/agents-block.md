<!-- BEGIN RTDD -->
## RTDD — which tests cover what you just changed

RTDD reports; you decide. It never blocks and never gates.

Before you verify a change, ask it what to run:

```
rtdd which --json
```

`selection.tests` is the ranked list of tests that can observe your change.
`tier: "empty"` means nothing was selected — that is NOT the same as "all passed".

To run that selection and get the uncovered-change signal:

```
rtdd run --json
```

Read `uncovered` in the output:

- `class: "uncovered"` — you changed these lines and no test executed them. This is the
  signal worth acting on.
- `class: "import-time"` — these lines DID execute, during import, where coverage
  attributes them to no test. Dataclasses, enums, config modules, Pydantic and Django
  models, and `__init__.py` re-exports land here. **This is not a gap.**
- `class: "covered"` — a test executed them.

`exit_code` is `1` only when a test failed. A non-empty uncovered report never changes it.

Other commands:

```
rtdd explain <file>   which tests cover this file
rtdd doctor           fan-out / coupling report (read its caveat)
rtdd verify           full suite
rtdd seed             one full instrumented run; rebuilds the map
```

Do not edit between the RTDD markers; `rtdd init` rewrites this block.
<!-- END RTDD -->
