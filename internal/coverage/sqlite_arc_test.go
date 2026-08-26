package coverage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func insArc(t *testing.T, db *sql.DB, fileID, ctxID, fromno, tono int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO arc (file_id, context_id, fromno, tono) VALUES (?, ?, ?, ?)`,
		fileID, ctxID, fromno, tono); err != nil {
		t.Fatalf("insert arc: %v", err)
	}
}

// Measured: with `[run] branch = True` coverage.py sets meta.has_arcs='1', writes
// ZERO line_bits rows, and puts everything in `arc`. The line_bits query returns
// nothing and the map comes out empty with exit 0 — a silent corruption path.
//
// The arcs below are verbatim from a real branch-mode .coverage, and the expected
// line sets are the ones the SAME code produced under line mode.
func TestReadSQLiteBranchModeUsesArcTable(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "1")
	abs := func(rel string) string { return filepath.Join(repo, filepath.FromSlash(rel)) }

	insFile(t, db, 1, abs("src/logic.py"))
	insFile(t, db, 2, abs("src/constants.py"))
	insContext(t, db, 1, "")
	insContext(t, db, 2, "tests/test_a.py::test_add|run")
	insContext(t, db, 3, "tests/test_b.py::test_mul|run")

	// src/logic.py, empty context. Negative numbers are scope entry/exit sentinels.
	for _, a := range [][2]int{{-1, 1}, {1, 4}, {4, 8}, {8, 12}, {12, -1}} {
		insArc(t, db, 1, 1, a[0], a[1])
	}
	// src/constants.py, empty context.
	for _, a := range [][2]int{{-5, 5}, {-1, 1}, {1, 3}, {3, 5}, {5, 6}, {5, 7}, {6, -1}, {6, 5}, {7, -5}} {
		insArc(t, db, 2, 1, a[0], a[1])
	}
	// src/logic.py, test_add: enter the function body, run line 5, return.
	insArc(t, db, 1, 2, -4, 5)
	insArc(t, db, 1, 2, 5, -4)
	// src/logic.py, test_mul.
	insArc(t, db, 1, 3, -8, 9)
	insArc(t, db, 1, 3, 9, -8)

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}

	wantImport := map[string][]int{
		"src/logic.py":     {1, 4, 8, 12},   // identical to the line_bits answer
		"src/constants.py": {1, 3, 5, 6, 7}, // identical to the line_bits answer
	}
	if !reflect.DeepEqual(res.ImportTime, wantImport) {
		t.Errorf("ImportTime =\n  %+v\nwant\n  %+v", res.ImportTime, wantImport)
	}

	if len(res.PerTest) != 2 {
		t.Fatalf("len(PerTest) = %d, want 2; got %+v", len(res.PerTest), res.PerTest)
	}
	want := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	if !reflect.DeepEqual(res.PerTest, want) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", res.PerTest, want)
	}
}

func TestReadSQLiteBranchModeWithNoArcsIsNotAnError(t *testing.T) {
	repo := t.TempDir()
	dbPath, _ := newCoverageDB(t, repo, "1")
	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	if len(res.PerTest) != 0 || len(res.ImportTime) != 0 {
		t.Fatalf("res = %+v, want empty", res)
	}
}

func TestReadSQLiteHasArcsAcceptsPythonicTruth(t *testing.T) {
	for _, v := range []string{"1", "True", "true"} {
		repo := t.TempDir()
		dbPath, db := newCoverageDB(t, repo, v)
		insFile(t, db, 1, filepath.Join(repo, "src", "logic.py"))
		insContext(t, db, 1, "tests/test_a.py::test_add|run")
		insArc(t, db, 1, 1, -4, 5)
		res, err := ReadSQLite(dbPath, repo)
		if err != nil {
			t.Fatalf("has_arcs=%q: ReadSQLite: %v", v, err)
		}
		if len(res.PerTest) != 1 {
			t.Fatalf("has_arcs=%q: len(PerTest) = %d, want 1 (the arc path was not taken)", v, len(res.PerTest))
		}
	}
}

// A store written by a coverage.py old enough — or damaged enough — to lack the
// has_arcs row must fall back to line mode, not error. Erroring here would turn a
// readable store into exit 3 on a run that measured fine.
func TestReadSQLiteMissingHasArcsMetaIsLineMode(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "0")
	if _, err := db.Exec(`DELETE FROM meta WHERE key = 'has_arcs'`); err != nil {
		t.Fatalf("delete meta: %v", err)
	}
	insFile(t, db, 1, filepath.Join(repo, "src", "logic.py"))
	insContext(t, db, 1, "tests/test_a.py::test_add|run")
	insLineBits(t, db, 1, 1, []byte{0x20}) // -> line 5

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite with no has_arcs row: %v", err)
	}
	want := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
	}
	if !reflect.DeepEqual(res.PerTest, want) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", res.PerTest, want)
	}
}

// Path normalisation, out-of-repo dropping and the setup|run|teardown collapse are
// properties of the accumulator, not of either query — the arc path must show them
// identically to the line_bits path.
func TestReadSQLiteBranchModeNormalisesLikeLineMode(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "1")

	insFile(t, db, 1, filepath.Join(repo, "src", "logic.py"))
	// [run] relative_files = True writes repo-relative paths.
	insFile(t, db, 2, "src/other.py")
	// Outside the repo: dropped, not crashed on.
	insFile(t, db, 3, "/usr/lib/python3/site-packages/attrs/__init__.py")

	insContext(t, db, 1, "tests/test_c.py::test_with_fixture|setup")
	insContext(t, db, 2, "tests/test_c.py::test_with_fixture|run")
	insContext(t, db, 3, "tests/test_c.py::test_with_fixture|teardown")

	insArc(t, db, 1, 1, -2, 2) // setup -> line 2
	insArc(t, db, 1, 2, -4, 5) // run -> line 5
	insArc(t, db, 1, 3, -6, 6) // teardown -> line 6
	insArc(t, db, 2, 2, 3, 4)  // relative path -> lines 3,4
	insArc(t, db, 3, 2, 1, 2)  // site-packages -> dropped

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	want := []TestCoverage{{
		Test: "tests/test_c.py::test_with_fixture",
		Files: map[string][]int{
			"src/logic.py": {2, 5, 6},
			"src/other.py": {3, 4},
		},
	}}
	if !reflect.DeepEqual(res.PerTest, want) {
		t.Fatalf("PerTest =\n  %+v\nwant\n  %+v", res.PerTest, want)
	}
}
