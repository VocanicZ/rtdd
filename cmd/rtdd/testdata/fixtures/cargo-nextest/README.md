# `cargo-nextest` fixture

Proves the `cargo-nextest` adapter against a real repository layout.

- **Detected by:** `.config/nextest.toml`, which is where JUnit output is configured —
  nextest has no `--junit` flag. `Cargo.toml` is committed because a Rust crate has one
  and is deliberately not a marker: it announces a Rust repo, not a nextest one.
- **Correspondence:** `src/calc.rs` → `tests/calc.rs`, via `test_for`'s `tests/{name}.rs`.

No runner is executed; `Cargo.lock`, `target/` and a dependency tree are all absent
because detection reads names and `test_for` asks only whether a path exists.
