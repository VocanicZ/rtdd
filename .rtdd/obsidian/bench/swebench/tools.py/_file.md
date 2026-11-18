---
id: "bench/swebench/tools.py"
source: "bench/swebench/tools.py"
start: 1
end: 185
kind: "file"
is-test: false
tags: [rtdd/file]
---
# tools.py

#rtdd/file

## Tests

- [[bench/swebench/tests/test_agent.py/test_a_reply_without_a_tool_call_stops_the_loop]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_finish_tool_stops_the_loop]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_turn_ceiling_stops_the_loop]] (references)
- [[bench/swebench/tests/test_agent.py/test_a_raising_budget_hook_stops_the_loop]] (references)
- [[bench/swebench/tests/test_agent.py/test_token_counts_accumulate_across_turns]] (references)
- [[bench/swebench/tests/test_agent.py/test_missing_usage_does_not_break_accounting]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_patch_is_what_the_tools_actually_changed]] (references)
- [[bench/swebench/tests/test_agent.py/test_an_agent_that_changed_nothing_returns_an_empty_patch]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_system_prompt_is_passed_through_verbatim]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_registered_tools_are_advertised_to_the_model]] (references)
- [[bench/swebench/tests/test_agent.py/test_an_unknown_tool_is_reported_to_the_model_and_the_loop_continues]] (references)
- [[bench/swebench/tests/test_agent.py/test_unparsable_tool_arguments_are_reported_to_the_model]] (references)
- [[bench/swebench/tests/test_agent.py/test_a_crashing_tool_is_data_not_a_run_failure]] (references)
- [[bench/swebench/tests/test_agent.py/test_every_tool_call_gets_a_reply_keyed_by_its_id]] (references)
- [[bench/swebench/tests/test_agent.py/test_the_scaffold_holds_no_arm_specific_branch]] (references)

## Callers

- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)

## Callees

none
