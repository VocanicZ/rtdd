---
id: "bench/replay/strategies/importgraph.py::build_graph"
source: "bench/replay/strategies/importgraph.py"
start: 78
end: 88
kind: "func"
is-test: false
tags: [rtdd/code]
---
# build_graph

#rtdd/code

## Tests

- [[bench/tests/test_strategy_importgraph.py/test_build_graph_resolves_a_relative_import_inside_a_package_init]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_build_graph_resolves_a_relative_import_inside_a_nested_package_init]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_build_graph_skips_vendored_and_generated_trees]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_a_change_reaches_tests_transitively_through_a_package]] (calls)
- [[bench/tests/test_strategy_importgraph.py/test_graph_over_the_synth_repo_selects_the_right_tests]] (calls)

## Callers

- [[bench/replay/strategies/importgraph.py/ImportGraph/select]] (calls)

## Callees

- [[bench/replay/strategies/importgraph.py/module_name]] (calls)
- [[bench/replay/strategies/importgraph.py/_package_of]] (calls)
- [[bench/replay/strategies/importgraph.py/parse_imports]] (calls)
