# Axis 2 — duration-weighted aggregate

Per spec §10 recall is **never pooled** across repos. The rows below weight each repo by its total suite duration and exist only to give a single ordering; the per-repo tables are the result.

| strategy | duration-weighted selected fraction | repos |
|---|---|---|
| full | 1.000 | 2 |
| importgraph | 0.417 | 2 |
| lf | 0.019 | 2 |
| path | 0.033 | 2 |
| random | 0.199 | 2 |
| rtdd | 0.190 | 2 |
| testmon | 0.238 | 2 |
| xdist | 1.000 | 2 |
