package coverage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"

	_ "modernc.org/sqlite" // pure-Go driver, no cgo — the binary must stay static

	"github.com/VocanicZ/rtdd/internal/paths"
)

const lineBitsQuery = `SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb
  JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id`

const arcQuery = `SELECT DISTINCT f.path, c.context, a.fromno, a.tono FROM arc a
  JOIN file f ON f.id = a.file_id JOIN context c ON c.id = a.context_id`

// ReadSQLite reads coverage.py's .coverage store directly. It *is* the bipartite
// test<->file relation, and is the only export format that carries a test
// identifier at a size that scales (spec §4, audit A6).
//
// Paths in `file` are absolute unless the host set [run] relative_files = True;
// both are normalised through internal/paths against repoRoot, and anything
// outside the repo (site-packages, the stdlib) is dropped.
func ReadSQLite(dbPath, repoRoot string) (*Result, error) {
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("coverage: %s is unreadable: %w", dbPath, err)
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("coverage: opening %s: %w", dbPath, err)
	}
	defer db.Close()

	acc := newAccumulator(repoRoot)

	hasArcs, err := readHasArcs(db)
	if err != nil {
		return nil, fmt.Errorf("coverage: reading meta in %s: %w", dbPath, err)
	}
	if hasArcs {
		if err := readArcs(db, acc); err != nil {
			return nil, fmt.Errorf("coverage: %s: %w", dbPath, err)
		}
		return acc.result(), nil
	}
	if err := readLineBits(db, acc); err != nil {
		return nil, fmt.Errorf("coverage: %s: %w", dbPath, err)
	}
	return acc.result(), nil
}

// readHasArcs reports whether the store was recorded with branch coverage on, in
// which case `line_bits` is empty and `arc` holds everything. A store with no
// has_arcs row is line mode: erroring there would turn a readable store into a
// fatal exit on a run that measured fine.
func readHasArcs(db *sql.DB) (bool, error) {
	var v string
	err := db.QueryRow(`SELECT value FROM meta WHERE key = 'has_arcs'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// coverage.py has written both the int and the repr of the bool over its life.
	return v == "1" || v == "True" || v == "true", nil
}

func readLineBits(db *sql.DB, acc *accumulator) error {
	rows, err := db.Query(lineBitsQuery)
	if err != nil {
		return fmt.Errorf("querying line_bits: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, ctx string
		var numbits []byte
		if err := rows.Scan(&path, &ctx, &numbits); err != nil {
			return fmt.Errorf("scanning line_bits: %w", err)
		}
		acc.add(path, ctx, Numbits(numbits))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading line_bits: %w", err)
	}
	return nil
}

// readArcs reconstructs executed lines from the `arc` table. Measured: with
// [run] branch = True — a very common host setting — line_bits has ZERO rows and
// every executed line lives in arc. Reading line_bits there yields an empty map on
// a run that exited 0, the same silent corruption shape as COVERAGE_CORE=sysmon.
//
// Executed lines = {fromno > 0} union {tono > 0}; negative values are scope
// entry/exit sentinels and zero is the synthetic module frame. The rule was
// verified to reproduce the line-mode answer exactly on every measured
// file/context pair.
func readArcs(db *sql.DB, acc *accumulator) error {
	rows, err := db.Query(arcQuery)
	if err != nil {
		return fmt.Errorf("querying arc: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, ctx string
		var fromno, tono int
		if err := rows.Scan(&path, &ctx, &fromno, &tono); err != nil {
			return fmt.Errorf("scanning arc: %w", err)
		}
		acc.add(path, ctx, []int{fromno, tono}) // acc.add drops the non-positive sentinels
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading arc: %w", err)
	}
	return nil
}

// accumulator collects (file, context, lines) triples into a Result, unioning
// the |setup, |run and |teardown phases of one test into a single entry.
type accumulator struct {
	repoRoot   string
	perTest    map[string]map[string]map[int]bool
	importTime map[string]map[int]bool
}

func newAccumulator(repoRoot string) *accumulator {
	return &accumulator{
		repoRoot:   repoRoot,
		perTest:    map[string]map[string]map[int]bool{},
		importTime: map[string]map[int]bool{},
	}
}

func (a *accumulator) add(rawPath, rawCtx string, lines []int) {
	rel, ok := paths.Normalize(a.repoRoot, rawPath)
	if !ok {
		return // outside the repo: site-packages, the stdlib, a sibling checkout
	}
	testID, _, ok := NormalizeContext(rawCtx)
	if !ok {
		set := a.importTime[rel]
		if set == nil {
			set = map[int]bool{}
			a.importTime[rel] = set
		}
		addLines(set, lines)
		return
	}
	files := a.perTest[testID]
	if files == nil {
		files = map[string]map[int]bool{}
		a.perTest[testID] = files
	}
	set := files[rel]
	if set == nil {
		set = map[int]bool{}
		files[rel] = set
	}
	addLines(set, lines)
}

// addLines drops non-positive line numbers: coverage records "line 0" for an
// empty __init__.py, and the arc table uses negative numbers as scope
// entry/exit sentinels. Neither is a real source line.
func addLines(set map[int]bool, lines []int) {
	for _, l := range lines {
		if l > 0 {
			set[l] = true
		}
	}
}

func (a *accumulator) result() *Result {
	res := &Result{ImportTime: make(map[string][]int, len(a.importTime))}
	for rel, set := range a.importTime {
		res.ImportTime[rel] = sortedKeys(set)
	}
	ids := make([]string, 0, len(a.perTest))
	for id := range a.perTest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		files := make(map[string][]int, len(a.perTest[id]))
		for rel, set := range a.perTest[id] {
			files[rel] = sortedKeys(set)
		}
		res.PerTest = append(res.PerTest, TestCoverage{Test: id, Files: files})
	}
	return res
}

func sortedKeys(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
