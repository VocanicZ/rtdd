---
id: "bench/replay/envsetup.py::provision"
source: "bench/replay/envsetup.py"
start: 118
end: 159
kind: "func"
is-test: false
tags: [rtdd/code]
---
# provision

#rtdd/code

## Tests

- [[bench/tests/test_envsetup.py/test_provisions_a_venv_and_runs_every_install_command]] (calls)
- [[bench/tests/test_envsetup.py/test_refuses_an_install_command_that_is_not_uv_pip_install]] (calls)
- [[bench/tests/test_envsetup.py/test_a_second_provision_with_the_same_recipe_does_no_work]] (calls)
- [[bench/tests/test_envsetup.py/test_changing_the_install_recipe_reprovisions]] (calls)
- [[bench/tests/test_envsetup.py/test_activate_puts_the_venv_bin_first_on_path]] (calls)
- [[bench/tests/test_envsetup.py/test_a_real_provision_produces_an_interpreter_that_imports_the_repo]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_replay]] (calls)
- [[bench/replay/cli.py/cmd_session]] (calls)

## Callees

- [[bench/replay/envsetup.py/RepoEnv]] (calls)
- [[bench/replay/envsetup.py/recipe_digest]] (calls)
- [[bench/replay/envsetup.py/_check]] (calls)
