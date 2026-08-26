# Axis 2 — duration-weighted aggregate

Per spec §10 recall is **never pooled** across repos. The rows below weight each repo by its total suite duration and exist only to give a single ordering; the per-repo tables are the result.

| strategy | duration-weighted selected fraction | repos |
|---|---|---|
| full | 1.000 | 1 |
| importgraph | 0.039 | 1 |
| lf | 0.784 | 1 |
| path | 0.048 | 1 |
| random | 0.225 | 1 |
| rtdd | 0.249 | 1 |
| testmon | 0.150 | 1 |
| xdist | 1.000 | 1 |
