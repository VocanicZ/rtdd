---
id: "internal/gitctx/graphgit.go::BlobIDs"
source: "internal/gitctx/graphgit.go"
start: 29
end: 50
kind: "func"
is-test: false
tags: [rtdd/code]
---
# BlobIDs

#rtdd/code

## Tests

- [[internal/gitctx/graphgit_test.go/TestBlobIDsChangeOnlyWhenACommitChangesTheFile]] (calls)
- [[internal/gitctx/graphgit_test.go/TestBlobIDsOnAnUnbornHeadIsEmpty]] (calls)
- [[internal/graphbuild/cache_test.go/TestCacheIsGraphifyShapedWithBuiltAtCommitAndBlobIDs]] (calls)

## Callers

- [[internal/graphbuild/build.go/Build]] (calls)

## Callees

- [[internal/gitctx/gitctx.go/git]] (calls)
