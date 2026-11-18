---
id: "installer/winpath.js::readUserPath"
source: "installer/winpath.js"
start: 23
end: 32
kind: "func"
is-test: false
tags: [rtdd/code]
---
# readUserPath

#rtdd/code

## Tests

- [[installer/installer.test.js/readUserPath treats a missing registry value as an empty PATH]] (calls)

## Callers

- [[installer/winpath.js/addToUserPath]] (calls)

## Callees

- [[installer/installer.test.js/addToUserPath does nothing when the directory is already on PATH/run]] (calls)
- [[installer/installer.test.js/addToUserPath treats a differently-cased entry as already present/run]] (calls)
- [[installer/installer.test.js/addToUserPath appends when the directory is absent/run]] (calls)
- [[installer/installer.test.js/readUserPath treats a missing registry value as an empty PATH/run]] (calls)
