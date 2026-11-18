---
id: "installer/platform.js::binaryName"
source: "installer/platform.js"
start: 56
end: 58
kind: "func"
is-test: false
tags: [rtdd/code]
---
# binaryName

#rtdd/code

## Tests

- [[installer/installer.test.js/binaryName is rtdd.exe on Windows only]] (calls)
- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

none
