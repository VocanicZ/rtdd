# Axis 2 — duration-weighted aggregate

Per spec §10 recall is **never pooled** across repos. The rows below weight each repo by its total suite duration and exist only to give a single ordering; the per-repo tables are the result.

| strategy | duration-weighted selected fraction | repos |
|---|---|---|
| full | 1.000 | 3 |
| importgraph | 0.141 | 3 |
| lf | 0.007 | 3 |
| path | 0.011 | 3 |
| random | 0.067 | 3 |
| rtdd | 0.064 | 3 |
| testmon | 0.096 | 3 |
| xdist | 1.000 | 3 |
