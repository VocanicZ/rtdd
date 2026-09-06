# `rspec` fixture

Proves the `rspec` adapter against a real repository layout.

- **Detected by:** `.rspec`. The `Gemfile` names `rspec_junit_formatter`, which is what
  makes RSpec able to emit JUnit XML at all — `requires` can only check the `rspec`
  binary, so the gem is named in the reason a human reads.
- **Correspondence:** `lib/calc.rb` → `spec/calc_spec.rb`. `test_for`'s first template,
  `spec/{dir}/{name}_spec.rb`, would name `spec/lib/calc_spec.rb`, which this repository
  does not have; the second, `spec/{name}_spec.rb`, resolves.

No runner is executed.
