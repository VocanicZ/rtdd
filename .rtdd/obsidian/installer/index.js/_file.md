---
id: "installer/index.js"
source: "installer/index.js"
start: 1
end: 177
kind: "file"
is-test: false
tags: [rtdd/file]
---
# index.js

#rtdd/file

## Tests

- [[installer/installer.test.js/platform maps Node names onto release asset names]] (references)
- [[installer/installer.test.js/platform rejects an OS or architecture with no published binary]] (references)
- [[installer/installer.test.js/platform rejects windows_arm64, which the release matrix does not build]] (references)
- [[installer/installer.test.js/archiveName mirrors the goreleaser name_template and drops the leading v]] (references)
- [[installer/installer.test.js/binaryName is rtdd.exe on Windows only]] (references)
- [[installer/installer.test.js/the Windows default install directory needs no admin rights]] (references)
- [[installer/installer.test.js/extractFile pulls a named member out of a tar.gz]] (references)
- [[installer/installer.test.js/extractFile reads a member whose size is not a multiple of the block size]] (references)
- [[installer/installer.test.js/extractFile names what the archive actually held when the member is missing]] (references)
- [[installer/installer.test.js/listEntries stops at the end-of-archive marker rather than reading padding]] (references)
- [[installer/installer.test.js/expectedChecksum matches the whole filename field, not a substring]] (references)
- [[installer/installer.test.js/expectedChecksum reports nothing rather than guessing when the asset is absent]] (references)
- [[installer/installer.test.js/verify refuses an archive with no checksum entry]] (references)
- [[installer/installer.test.js/verify refuses an archive whose bytes do not match the published digest]] (references)
- [[installer/installer.test.js/verify accepts the bytes the checksum was computed over]] (references)
- [[installer/installer.test.js/resolveVersion prefers an explicit tag over asking the API at all]] (references)
- [[installer/installer.test.js/addToUserPath does nothing when the directory is already on PATH]] (references)
- [[installer/installer.test.js/addToUserPath treats a differently-cased entry as already present]] (references)
- [[installer/installer.test.js/addToUserPath appends when the directory is absent]] (references)
- [[installer/installer.test.js/readUserPath treats a missing registry value as an empty PATH]] (references)
- [[installer/installer.test.js/parseArgs reads flags and environment, with flags winning]] (references)
- [[installer/installer.test.js/parseArgs rejects an unknown option instead of ignoring it]] (references)
- [[installer/installer.test.js/install downloads, verifies and places the binary]] (references)
- [[installer/installer.test.js/install resolves the latest release when no tag is pinned]] (references)
- [[installer/installer.test.js/install aborts and writes nothing when the checksum does not match]] (references)
- [[installer/installer.test.js/the entrypoint runs when invoked through a symlink, as npx invokes it]] (references)
- [[installer/installer.test.js/the entrypoint runs when invoked through a symlink, as npx invokes it]] (references)

## Callers

- [[internal/installtest/install_test.go/runInstaller]] (references)
- [[internal/selfupdate/drift_test.go/installerSource]] (references)
- [[package.json/_file]] (references)

## Callees

- [[installer/platform.js/_file]] (references)
- [[installer/release.js/_file]] (references)
- [[installer/targz.js/_file]] (references)
- [[installer/winpath.js/_file]] (references)
