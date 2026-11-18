---
id: "internal/gitctx/listfiles.go::ListFiles"
source: "internal/gitctx/listfiles.go"
start: 14
end: 33
kind: "func"
is-test: false
tags: [rtdd/code]
---
# ListFiles

#rtdd/code

## Tests

- [[internal/gitctx/listfiles_test.go/TestListFilesIsTrackedPlusUntrackedMinusIgnoredAndDeleted]] (calls)
- [[internal/graphbuild/overlay_test.go/TestStaleSetSources]] (calls)

## Callers

- [[cmd/rtdd/doctor.go/cmdDoctor]] (calls)
- [[internal/graphbuild/build.go/Build]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/git]] (calls)
