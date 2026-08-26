package coverage

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	_ "modernc.org/sqlite"
)

// coverageDDL is the schema of a real .coverage file, copied verbatim out of
// sqlite_master on coverage.py 7.15.4.
const coverageDDL = `
CREATE TABLE coverage_schema (version integer);
CREATE TABLE meta (key text, value text, unique (key));
CREATE TABLE file (id integer primary key, path text, unique (path));
CREATE TABLE context (id integer primary key, context text, unique (context));
CREATE TABLE line_bits (
    file_id integer, context_id integer, numbits blob,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id));
CREATE TABLE arc (
    file_id integer, context_id integer, fromno integer, tono integer,
    foreign key (file_id) references file (id),
    foreign key (context_id) references context (id),
    unique (file_id, context_id, fromno, tono));
CREATE TABLE tracer (file_id integer primary key, tracer text,
    foreign key (file_id) references file (id));
`

func newCoverageDB(t *testing.T, dir string, hasArcs string) (string, *sql.DB) {
	t.Helper()
	p := filepath.Join(dir, ".coverage")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(coverageDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO coverage_schema (version) VALUES (7)`); err != nil {
		t.Fatalf("schema row: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO meta (key, value) VALUES ('version','7.15.4'), ('has_arcs', ?)`, hasArcs); err != nil {
		t.Fatalf("meta: %v", err)
	}
	return p, db
}

func insFile(t *testing.T, db *sql.DB, id int, path string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO file (id, path) VALUES (?, ?)`, id, path); err != nil {
		t.Fatalf("insert file: %v", err)
	}
}

func insContext(t *testing.T, db *sql.DB, id int, ctx string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO context (id, context) VALUES (?, ?)`, id, ctx); err != nil {
		t.Fatalf("insert context: %v", err)
	}
}

func insLineBits(t *testing.T, db *sql.DB, fileID, ctxID int, numbits []byte) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO line_bits (file_id, context_id, numbits) VALUES (?, ?, ?)`,
		fileID, ctxID, numbits); err != nil {
		t.Fatalf("insert line_bits: %v", err)
	}
}

// The fixture below reproduces, row for row, a .coverage measured on this machine
// from `COVERAGE_CORE=ctrace pytest --cov --cov-context=test`.
func TestReadSQLiteLineBits(t *testing.T) {
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "0")

	abs := func(rel string) string { return filepath.Join(repo, filepath.FromSlash(rel)) }

	// file.path is ABSOLUTE unless the host sets [run] relative_files = True.
	insFile(t, db, 1, abs("src/__init__.py"))
	insFile(t, db, 2, abs("src/logic.py"))
	insFile(t, db, 3, abs("src/constants.py"))
	// A path outside the repo (site-packages) must be dropped, not crash.
	insFile(t, db, 4, "/usr/lib/python3/site-packages/attrs/__init__.py")

	insContext(t, db, 1, "")
	insContext(t, db, 2, "tests/test_a.py::test_add|run")
	insContext(t, db, 3, "tests/test_a.py::test_param[1-one two]|run")
	insContext(t, db, 4, "tests/test_a.py::test_param[2-a-b]|run")
	insContext(t, db, 5, "tests/test_b.py::test_mul|run")
	insContext(t, db, 6, "tests/test_b.py::test_fail|run")
	insContext(t, db, 7, "tests/test_c.py::test_with_fixture|setup")
	insContext(t, db, 8, "tests/test_c.py::test_with_fixture|teardown")

	insLineBits(t, db, 1, 1, []byte{0x01})       // empty __init__.py -> line 0, dropped
	insLineBits(t, db, 3, 1, []byte{0xEA})       // constants.py import-time -> 1,3,5,6,7
	insLineBits(t, db, 2, 1, []byte{0x12, 0x11}) // logic.py import-time -> 1,4,8,12
	insLineBits(t, db, 2, 2, []byte{0x20})       // test_add -> 5
	insLineBits(t, db, 2, 3, []byte{0x20})
	insLineBits(t, db, 2, 4, []byte{0x20})
	insLineBits(t, db, 2, 5, []byte{0x00, 0x02}) // test_mul -> 9
	insLineBits(t, db, 2, 6, []byte{0x00, 0x02}) // test_fail -> 9
	insLineBits(t, db, 2, 7, []byte{0x04})       // test_with_fixture setup -> 2
	insLineBits(t, db, 2, 8, []byte{0x40})       // test_with_fixture teardown -> 6
	insLineBits(t, db, 4, 2, []byte{0xFF})       // site-packages, dropped

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}

	// Import-time: attributed to NO test. src/constants.py is the audit A1 case —
	// a dataclass plus a module constant, imported and asserted by passing tests,
	// with zero test attribution.
	wantImport := map[string][]int{
		"src/__init__.py":  {},
		"src/logic.py":     {1, 4, 8, 12},
		"src/constants.py": {1, 3, 5, 6, 7},
	}
	if !reflect.DeepEqual(res.ImportTime, wantImport) {
		t.Errorf("ImportTime =\n  %+v\nwant\n  %+v", res.ImportTime, wantImport)
	}

	wantPerTest := []TestCoverage{
		{Test: "tests/test_a.py::test_add", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_a.py::test_param[1-one two]", Files: map[string][]int{"src/logic.py": {5}}},
		{Test: "tests/test_a.py::test_param[2-a-b]", Files: map[string][]int{"src/logic.py": {5}}},
		// setup and teardown phases collapse into ONE entry for the test.
		{Test: "tests/test_c.py::test_with_fixture", Files: map[string][]int{"src/logic.py": {2, 6}}},
		{Test: "tests/test_b.py::test_fail", Files: map[string][]int{"src/logic.py": {9}}},
		{Test: "tests/test_b.py::test_mul", Files: map[string][]int{"src/logic.py": {9}}},
	}
	// PerTest is sorted by Test.
	byID := map[string]map[string][]int{}
	for _, tc := range res.PerTest {
		byID[tc.Test] = tc.Files
	}
	if len(res.PerTest) != len(wantPerTest) {
		t.Fatalf("len(PerTest) = %d, want %d; got %+v", len(res.PerTest), len(wantPerTest), res.PerTest)
	}
	for _, w := range wantPerTest {
		got, ok := byID[w.Test]
		if !ok {
			t.Errorf("PerTest missing %q", w.Test)
			continue
		}
		if !reflect.DeepEqual(got, w.Files) {
			t.Errorf("PerTest[%q].Files = %+v, want %+v", w.Test, got, w.Files)
		}
	}
	for i := 1; i < len(res.PerTest); i++ {
		if res.PerTest[i-1].Test >= res.PerTest[i].Test {
			t.Fatalf("PerTest not sorted by Test at index %d: %q then %q",
				i, res.PerTest[i-1].Test, res.PerTest[i].Test)
		}
	}
}

func TestReadSQLiteRelativeFiles(t *testing.T) {
	// A host with [run] relative_files = True writes repo-relative paths.
	repo := t.TempDir()
	dbPath, db := newCoverageDB(t, repo, "0")
	insFile(t, db, 1, "src/logic.py")
	insContext(t, db, 1, "tests/test_a.py::test_add|run")
	insLineBits(t, db, 1, 1, []byte{0x20})

	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite: %v", err)
	}
	if len(res.PerTest) != 1 {
		t.Fatalf("len(PerTest) = %d, want 1", len(res.PerTest))
	}
	want := map[string][]int{"src/logic.py": {5}}
	if !reflect.DeepEqual(res.PerTest[0].Files, want) {
		t.Fatalf("Files = %+v, want %+v", res.PerTest[0].Files, want)
	}
}

func TestReadSQLiteMissingFileIsAnError(t *testing.T) {
	repo := t.TempDir()
	_, err := ReadSQLite(filepath.Join(repo, ".coverage"), repo)
	if err == nil {
		t.Fatal("ReadSQLite on a missing .coverage = nil error; a missing store must be fatal (exit 3), never an empty map")
	}
}

func TestReadSQLiteEmptyStore(t *testing.T) {
	repo := t.TempDir()
	dbPath, _ := newCoverageDB(t, repo, "0")
	res, err := ReadSQLite(dbPath, repo)
	if err != nil {
		t.Fatalf("ReadSQLite on an empty store: %v", err)
	}
	if len(res.PerTest) != 0 {
		t.Errorf("PerTest = %+v, want empty", res.PerTest)
	}
	if res.ImportTime == nil {
		t.Error("ImportTime is nil, want an empty non-nil map")
	}
}
