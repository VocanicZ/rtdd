---
id: "bench/replay/strategies/__init__.py"
source: "bench/replay/strategies/__init__.py"
start: 1
end: 23
kind: "file"
is-test: false
tags: [rtdd/file]
---
# __init__.py

#rtdd/file

## Tests

- [[bench/tests/test_envsetup.py/test_a_real_provision_produces_an_interpreter_that_imports_the_repo]] (references)
- [[bench/tests/test_strategy_importgraph.py/test_module_name]] (references)
- [[bench/tests/test_strategy_importgraph.py/test_build_graph_resolves_a_relative_import_inside_a_package_init]] (references)
- [[bench/tests/test_strategy_importgraph.py/test_build_graph_resolves_a_relative_import_inside_a_nested_package_init]] (references)
- [[bench/tests/test_strategy_importgraph.py/test_a_change_reaches_tests_transitively_through_a_package]] (references)
- [[bench/tests/test_strategy_pathheuristic.py/test_a_changed_package_init_reaches_nothing]] (references)

## Callers

- [[bench/replay/strategies/importgraph.py/module_name]] (references)
- [[bench/replay/strategies/importgraph.py/_package_of]] (references)
- [[bench/tests/synthrepo.py/build_synth_repo]] (references)
- [[bench/tests/test_replay.py/_repo_whose_red_survives_the_child]] (references)
- [[docs/audits/2026-08-26-design-audit.md/_file]] (references)
- [[docs/outcomes/README.negative.md/_file]] (references)
- [[docs/plans/02-m1b-python-adapter.md/_file]] (references)
- [[docs/plans/03-m2-signal.md/_file]] (references)
- [[docs/plans/04-m3-replay-benchmark.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/08-one-pipeline.md/_file]] (references)
- [[docs/results/n4-bench-rounds.md/_file]] (references)
- [[docs/specs/2026-08-26-rtdd-design.md/_file]] (references)

## Callees

none
