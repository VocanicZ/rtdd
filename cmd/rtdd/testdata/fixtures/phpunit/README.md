# `phpunit` fixture

Proves the `phpunit` adapter against a real repository layout.

- **Detected by:** `phpunit.xml`. `composer.json` is committed because a PHP project has
  one and is not a marker.
- **Correspondence:** `src/Calc.php` → `tests/CalcTest.php`, via `test_for`'s
  `tests/{name}Test.php`.

No runner is executed; `vendor/` is absent.
