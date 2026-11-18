---
id: "installer/index.js::install"
source: "installer/index.js"
start: 72
end: 131
kind: "func"
is-test: false
tags: [rtdd/code]
---
# install

#rtdd/code

## Tests

- [[installer/installer.test.js/install downloads, verifies and places the binary]] (calls)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (calls)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (calls)

## Callers

none

## Callees

- [[installer/index.js/parseArgs]] (calls)
- [[installer/index.js/log]] (calls)
- [[installer/index.js/installSkill]] (calls)
- [[installer/platform.js/target]] (calls)
- [[installer/platform.js/binaryName]] (calls)
- [[installer/platform.js/archiveName]] (calls)
- [[installer/platform.js/defaultInstallDir]] (calls)
- [[installer/release.js/baseUrl]] (calls)
- [[installer/release.js/fetchBuffer]] (calls)
- [[installer/release.js/resolveVersion]] (calls)
- [[installer/release.js/verify]] (calls)
- [[installer/targz.js/extractFile]] (calls)
- [[installer/winpath.js/addToUserPath]] (calls)
- [[internal/scan/testdata/typescript/calc.ts/add]] (calls)
