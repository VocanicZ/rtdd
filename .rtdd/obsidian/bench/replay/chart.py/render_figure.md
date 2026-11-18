---
id: "bench/replay/chart.py::render_figure"
source: "bench/replay/chart.py"
start: 1273
end: 1292
kind: "func"
is-test: false
tags: [rtdd/code]
---
# render_figure

#rtdd/code

## Tests

- [[bench/tests/test_chart.py/test_light_and_dark_carry_identical_data]] (calls)
- [[bench/tests/test_chart.py/test_render_is_byte_stable]] (calls)
- [[bench/tests/test_chart.py/test_every_figure_declares_its_source]] (calls)
- [[bench/tests/test_chart.py/test_no_animation_runs_off_the_end_of_its_loop]] (calls)

## Callers

- [[bench/replay/chart.py/render_all]] (calls)

## Callees

- [[bench/replay/chart.py/ChartError]] (calls)
- [[bench/replay/chart.py/savings_svg]] (calls)
- [[bench/replay/chart.py/safety_svg]] (calls)
- [[bench/replay/chart.py/wallclock_svg]] (calls)
- [[bench/replay/chart.py/load_head_to_head]] (calls)
- [[bench/replay/chart.py/head_to_head_svg]] (calls)
- [[bench/replay/chart.py/load_worked_example]] (calls)
- [[bench/replay/chart.py/worked_example_svg]] (calls)
- [[bench/replay/chart.py/load_rounds_example]] (calls)
- [[bench/replay/chart.py/rounds_svg]] (calls)
