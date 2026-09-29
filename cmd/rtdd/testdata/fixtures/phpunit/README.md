# `phpunit` fixture

Proves the `phpunit` adapter against a real repository layout.

- **Detected by:** `phpunit.xml`. `composer.json` is committed because a PHP project has
  one and is not a marker.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

`vendor/` is absent; the pipeline test runs `composer install` in a temporary copy.
