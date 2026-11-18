---
id: "installer/platform.js::target"
source: "installer/platform.js"
start: 44
end: 53
kind: "func"
is-test: false
tags: [rtdd/code]
---
# target

#rtdd/code

## Tests

- [[installer/installer.test.js/platform rejects windows_arm64, which the release matrix does not build]] (calls)
- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

- [[installer/platform.js/goosFor]] (calls)
- [[installer/platform.js/goarchFor]] (calls)
