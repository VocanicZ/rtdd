---
id: "installer/release.js::resolveVersion"
source: "installer/release.js"
start: 35
end: 48
kind: "func"
is-test: false
tags: [rtdd/code]
---
# resolveVersion

#rtdd/code

## Tests

- [[installer/installer.test.js/resolveVersion prefers an explicit tag over asking the API at all]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

- [[installer/release.js/apiUrl]] (calls)
- [[installer/release.js/fetchBuffer]] (calls)
