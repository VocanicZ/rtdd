---
id: "installer/installer.test.js::tarball"
source: "installer/installer.test.js"
start: 19
end: 40
kind: "func"
is-test: false
tags: [rtdd/code]
---
# tarball

#rtdd/code

## Tests

- [[installer/installer.test.js/extractFile pulls a named member out of a tar.gz]] (calls)
- [[installer/installer.test.js/extractFile reads a member whose size is not a multiple of the block size]] (calls)
- [[installer/installer.test.js/extractFile names what the archive actually held when the member is missing]] (calls)
- [[installer/installer.test.js/listEntries stops at the end-of-archive marker rather than reading padding]] (calls)
- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)

## Callers

none

## Callees

none
