---
id: "installer/installer.test.js::tempDir"
source: "installer/installer.test.js"
start: 64
end: 66
kind: "func"
is-test: false
tags: [rtdd/code]
---
# tempDir

#rtdd/code

## Tests

- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)
- [[installer/installer.test.js/the entrypoint runs when invoked through a symlink, as npx invokes it]] (calls)

## Callers

none

## Callees

none
