#!/usr/bin/env bash
# Runs as a whole: `bash test/test_bug_flow.sh`.
source "$(dirname "$0")/../lib/drive.sh"

stub_bug() {
  git() {
    :
  }
  gh() {
    echo "gh $*"
  }
}

test_files_a_bug() {
  stub_bug
  file_bug "it broke" | grep -q "issue create"
}

test_files_a_bug && echo ok
