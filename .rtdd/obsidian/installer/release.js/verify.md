---
id: "installer/release.js::verify"
source: "installer/release.js"
start: 77
end: 84
kind: "func"
is-test: false
tags: [rtdd/code]
---
# verify

#rtdd/code

## Tests

- [[installer/installer.test.js/verify refuses an archive with no checksum entry]] (calls)
- [[installer/installer.test.js/verify refuses an archive whose bytes do not match the published digest]] (calls)
- [[installer/installer.test.js/verify accepts the bytes the checksum was computed over]] (calls)

## Callers

- [[installer/index.js/install]] (calls)

## Callees

- [[installer/release.js/sha256]] (calls)
- [[installer/release.js/expectedChecksum]] (calls)
