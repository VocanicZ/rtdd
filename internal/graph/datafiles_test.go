package graph

import "testing"

// #573: a log, a transcript, a text note or a data table is never a test file, even under
// test/ or tests/ — with the defaults, and with the pre-fix defaults `rtdd init` wrote
// into .rtdd/config.yaml, which replace the defaults wholesale.
func TestDataAndLogFilesAreNeverTestFiles(t *testing.T) {
	preFix := DefaultConfig()
	files := []string{
		"test/smoke/logs/run.log", "tests/run.jsonl", "test/notes.txt", "tests/out/a.out",
		"tests/twins.tsv", "tests/cases.csv", "tests/test_data.log", "test/test_x.txt",
	}
	for name, cfg := range map[string]Config{"defaults": DefaultConfig(), "pre-fix config": preFix} {
		for _, f := range files {
			if IsTestFile(f, cfg) {
				t.Errorf("%s: IsTestFile(%q) = true", name, f)
			}
		}
		nodes := []Node{{ID: "test/smoke/logs/run.log::setup", File: "test/smoke/logs/run.log", Name: "setup", Kind: KindFunc}}
		Classify(nodes, cfg)
		if nodes[0].IsTest {
			t.Errorf("%s: a func-looking line of a log is a test", name)
		}
	}
	for _, f := range []string{"tests/test_a.py", "test/test_a.sh", "tests/helper.rb", "a_test.go"} {
		if !IsTestFile(f, preFix) {
			t.Errorf("IsTestFile(%q) = false: code under test/ is still a test file", f)
		}
	}
}
