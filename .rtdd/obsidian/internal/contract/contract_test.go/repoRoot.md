---
id: "internal/contract/contract_test.go::repoRoot"
source: "internal/contract/contract_test.go"
start: 18
end: 34
kind: "func"
is-test: false
tags: [rtdd/code]
---
# repoRoot

#rtdd/code

## Tests

- [[internal/contract/contract_test.go/TestCompiledModulesAreYAMLOnly]] (calls)
- [[internal/contract/contract_test.go/TestLocalCIEntrypointIsExecutableAndRunsTheSameChecks]] (calls)
- [[internal/contract/contract_test.go/TestEveryRepoScriptIsExecutable]] (calls)
- [[internal/contract/frontend_vocabulary_test.go/TestFrontEndsUseNoV02Vocabulary]] (calls)
- [[internal/contract/nomock_test.go/TestNoGitMockExistsInTheTree]] (calls)
- [[internal/contract/nomock_test.go/TestOnlyGitctxShellsOutToGit]] (calls)
- [[internal/contract/nomock_test.go/TestOnlyTestFilesImportGittest]] (calls)
- [[internal/contract/nomock_test.go/TestTestOnlyHelpersDoNotCompileInTesting]] (calls)
- [[internal/contract/pipeline_removed_test.go/TestCoveragePipelinePackagesAreGone]] (calls)
- [[internal/contract/pipeline_removed_test.go/TestNoGoCodeReferencesTheCoveragePipeline]] (calls)
- [[internal/contract/pipeline_removed_test.go/TestNoGoFileNamesARemovedPackagesAPI]] (calls)

## Callers

- [[internal/contract/ci_workflow_test.go/repoDeclaresTestFunc]] (calls)
- [[internal/contract/contract_test.go/readRepoFile]] (calls)
- [[internal/contract/pipeline_removed_test.go/goFiles]] (calls)

## Callees

- [[go.mod/_file]] (references)
