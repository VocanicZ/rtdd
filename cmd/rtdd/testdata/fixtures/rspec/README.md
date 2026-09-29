# `rspec` fixture

Proves the `rspec` adapter against a real repository layout.

- **Detected by:** `.rspec`. The `Gemfile` names `rspec_junit_formatter`, which is what
  makes RSpec able to emit JUnit XML at all — `requires` can only check the `rspec`
  binary, so the gem is named in the reason a human reads.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

The fixture is only ever run by the pipeline test, in a temporary copy.
