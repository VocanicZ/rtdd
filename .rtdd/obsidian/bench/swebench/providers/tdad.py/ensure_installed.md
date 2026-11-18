---
id: "bench/swebench/providers/tdad.py::ensure_installed"
source: "bench/swebench/providers/tdad.py"
start: 124
end: 198
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ensure_installed

#rtdd/code

## Tests

- [[bench/swebench/tests/test_providers_tdad.py/test_ensure_installed_clones_the_pin_and_returns_the_resolved_sha]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_a_second_call_is_a_no_op_that_returns_the_same_sha]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_tdads_own_code_is_not_edited]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_a_pin_that_is_not_in_the_repository_is_unavailable]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_an_unclonable_repository_is_unavailable]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_a_failed_install_is_unavailable]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_a_cli_that_will_not_run_is_unavailable]] (calls)
- [[bench/swebench/tests/test_providers_tdad.py/test_a_failed_install_is_not_cached_as_a_success]] (calls)

## Callers

- [[bench/swebench/providers/__init__.py/context_tools]] (calls)

## Callees

- [[bench/swebench/providers/tdad.py/TdadUnavailable]] (calls)
- [[bench/swebench/providers/tdad.py/_uv_bin]] (calls)
- [[bench/swebench/providers/tdad.py/_run]] (calls)
- [[bench/swebench/providers/tdad.py/_has_commit]] (calls)
