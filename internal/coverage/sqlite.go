package coverage

import (
	"database/sql"
	"fmt"
	"os"
	"sort"

	_ "modernc.org/sqlite" // pure-Go driver, no cgo — the binary must stay static

	"github.com/VocanicZ/rtdd/internal/paths"
)

const lineBitsQuery = `SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb
  JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id`

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

	rows, err := db.Query(lineBitsQuery)
	if err != nil {
		return nil, fmt.Errorf("coverage: querying line_bits in %s: %w", dbPath, err)
	}
	defer rows.Close()
	for rows.Next() {
		var path, ctx string
		var numbits []byte
		if err := rows.Scan(&path, &ctx, &numbits); err != nil {
			return nil, fmt.Errorf("coverage: scanning line_bits in %s: %w", dbPath, err)
		}
		acc.add(path, ctx, Numbits(numbits))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("coverage: reading line_bits in %s: %w", dbPath, err)
	}

	return acc.result(), nil
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
