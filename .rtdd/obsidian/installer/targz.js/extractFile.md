---
id: "installer/targz.js::extractFile"
source: "installer/targz.js"
start: 86
end: 93
kind: "func"
is-test: false
tags: [rtdd/code]
---
# extractFile

#rtdd/code

## Tests

- [[installer/installer.test.js/extractFile pulls a named member out of a tar.gz]] (calls)
- [[installer/installer.test.js/extractFile reads a member whose size is not a multiple of the block size]] (calls)
- [[installer/installer.test.js/extractFile names what the archive actually held when the member is missing]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

- [[installer/targz.js/listEntries]] (calls)
