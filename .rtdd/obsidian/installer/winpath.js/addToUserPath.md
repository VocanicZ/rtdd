---
id: "installer/winpath.js::addToUserPath"
source: "installer/winpath.js"
start: 47
end: 62
kind: "func"
is-test: false
tags: [rtdd/code]
---
# addToUserPath

#rtdd/code

## Tests

- [[installer/installer.test.js/addToUserPath does nothing when the directory is already on PATH]] (calls)
- [[installer/installer.test.js/addToUserPath treats a differently-cased entry as already present]] (calls)
- [[installer/installer.test.js/addToUserPath appends when the directory is absent]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

- [[installer/installer.test.js/addToUserPath does nothing when the directory is already on PATH/run]] (calls)
- [[installer/installer.test.js/addToUserPath treats a differently-cased entry as already present/run]] (calls)
- [[installer/installer.test.js/addToUserPath appends when the directory is absent/run]] (calls)
- [[installer/installer.test.js/readUserPath treats a missing registry value as an empty PATH/run]] (calls)
- [[installer/winpath.js/readUserPath]] (calls)
- [[installer/winpath.js/samePath]] (calls)
