---
id: "bench/swebench/prompts.py::extract_context"
source: "bench/swebench/prompts.py"
start: 202
end: 224
kind: "func"
is-test: false
tags: [rtdd/code]
---
# extract_context

#rtdd/code

## Tests

- [[bench/swebench/tests/test_prompts.py/test_extract_context_returns_the_block_of_every_context_arm]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_extract_context_returns_none_for_a_control_arm]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_extract_context_round_trips_context_block]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_two_blocks_are_an_error_rather_than_a_best_guess]] (calls)
- [[bench/swebench/tests/test_prompts.py/test_a_stray_delimiter_is_an_error]] (calls)

## Callers

none

## Callees

- [[bench/swebench/prompts.py/ContextBlockError]] (calls)
