---
id: "bench/swebench/tools.py::base_tools"
source: "bench/swebench/tools.py"
start: 60
end: 185
kind: "func"
is-test: false
tags: [rtdd/code]
---
# base_tools

#rtdd/code

## Tests

- [[bench/swebench/tests/test_agent.py/test_a_reply_without_a_tool_call_stops_the_loop]] (calls)
- [[bench/swebench/tests/test_agent.py/test_the_finish_tool_stops_the_loop]] (calls)
- [[bench/swebench/tests/test_agent.py/test_the_turn_ceiling_stops_the_loop]] (calls)
- [[bench/swebench/tests/test_agent.py/test_a_raising_budget_hook_stops_the_loop]] (calls)
- [[bench/swebench/tests/test_agent.py/test_token_counts_accumulate_across_turns]] (calls)
- [[bench/swebench/tests/test_agent.py/test_missing_usage_does_not_break_accounting]] (calls)
- [[bench/swebench/tests/test_agent.py/test_the_patch_is_what_the_tools_actually_changed]] (calls)
- [[bench/swebench/tests/test_agent.py/test_an_agent_that_changed_nothing_returns_an_empty_patch]] (calls)
- [[bench/swebench/tests/test_agent.py/test_the_system_prompt_is_passed_through_verbatim]] (calls)
- [[bench/swebench/tests/test_agent.py/test_the_registered_tools_are_advertised_to_the_model]] (calls)
- [[bench/swebench/tests/test_agent.py/test_an_unknown_tool_is_reported_to_the_model_and_the_loop_continues]] (calls)
- [[bench/swebench/tests/test_agent.py/test_unparsable_tool_arguments_are_reported_to_the_model]] (calls)
- [[bench/swebench/tests/test_agent.py/test_a_crashing_tool_is_data_not_a_run_failure]] (calls)
- [[bench/swebench/tests/test_agent.py/test_every_tool_call_gets_a_reply_keyed_by_its_id]] (calls)
- [[bench/swebench/tests/test_tools.py/test_every_tool_carries_an_openai_function_schema]] (calls)

## Callers

- [[bench/swebench/tests/test_tools.py/tools_by_name]] (calls)

## Callees

- [[bench/swebench/tools.py/base_tools/t]] (calls)
