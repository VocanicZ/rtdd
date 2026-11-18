---
id: "installer/platform.js::archiveName"
source: "installer/platform.js"
start: 67
end: 70
kind: "func"
is-test: false
tags: [rtdd/code]
---
# archiveName

#rtdd/code

## Tests

- [[installer/installer.test.js/archiveName mirrors the goreleaser name_template and drops the leading v]] (calls)
- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

none
