# Axis 2 — duration-weighted aggregate

Per spec §10 recall is **never pooled** across repos. The rows below weight each repo by its total suite duration and exist only to give a single ordering; the per-repo tables are the result.

| strategy | duration-weighted selected fraction | repos |
|---|---|---|
| full | 1.000 | 1 |
| importgraph | 0.047 | 1 |
| lf | 0.790 | 1 |
| path | 0.056 | 1 |
| random | 0.258 | 1 |
| rtdd | 0.282 | 1 |
| testmon | 0.190 | 1 |
| xdist | 1.000 | 1 |
