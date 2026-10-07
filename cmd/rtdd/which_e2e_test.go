package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// e2eRepo is a committed Python+Go project with the same shape in each language: an
// edited function (price/Price) that calls a callee (tax/Tax) and is called by a caller
// (checkout/Checkout), each with its own test, an unrelated function with a test, and an
// orphan with none. Each test file lists its tests out of alphabetical order, so only a
// file-then-line order puts them where the assertions expect. No toolchain is ever run:
// the files are only ever read by the scanner.
func e2eRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "shop/shop.go", "package shop\n\n"+
		"func Tax(p int) int {\n\treturn p / 10\n}\n\n"+
		"func Price(p int) int {\n\treturn p + Tax(p)\n}\n\n"+
		"func Checkout(ps []int) int {\n\tn := 0\n\tfor _, p := range ps {\n\t\tn += Price(p)\n\t}\n\treturn n\n}\n\n"+
		"func Unrelated() int {\n\treturn 7\n}\n\n"+
		"func Orphan() int {\n\treturn 0\n}\n")
	gittest.Write(t, dir, "shop/shop_test.go", "package shop\n\nimport \"testing\"\n\n"+
		"func TestUnrelated(t *testing.T) {\n\tif Unrelated() != 7 {\n\t\tt.Fatal(\"Unrelated\")\n\t}\n}\n\n"+
		"func TestTax(t *testing.T) {\n\tif Tax(10) != 1 {\n\t\tt.Fatal(\"Tax\")\n\t}\n}\n\n"+
		"func TestPrice(t *testing.T) {\n\tif Price(10) != 11 {\n\t\tt.Fatal(\"Price\")\n\t}\n}\n\n"+
		"func TestCheckout(t *testing.T) {\n\tif Checkout([]int{10}) != 11 {\n\t\tt.Fatal(\"Checkout\")\n\t}\n}\n")
	gittest.Write(t, dir, "src/tax.py", "def tax(p):\n    return p // 10\n")
	gittest.Write(t, dir, "src/shop.py", "from src.tax import tax\n\n\n"+
		"def price(p):\n    return p + tax(p)\n\n\n"+
		"def checkout(ps):\n    return sum(price(p) for p in ps)\n\n\n"+
		"def unrelated():\n    return 7\n\n\n"+
		"def orphan():\n    return 0\n")
	gittest.Write(t, dir, "tests/test_shop.py", "from src.shop import checkout, price, unrelated\n\n\n"+
		"def test_unrelated():\n    assert unrelated() == 7\n\n\n"+
		"def test_price():\n    assert price(10) == 11\n\n\n"+
		"def test_checkout():\n    assert checkout([10]) == 11\n")
	gittest.Write(t, dir, "tests/test_tax.py", "from src.tax import tax\n\n\ndef test_tax():\n    assert tax(10) == 1\n")
	gittest.Commit(t, dir, "fixture")
	return dir
}

// e2eEdit rewrites one line of file: the first line containing old becomes new.
func e2eEdit(t *testing.T, dir, file, old, new string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s has no %q", file, old)
	}
	gittest.Write(t, dir, file, strings.Replace(string(b), old, new, 1))
}

// e2eWhich is the part of `rtdd which --json` the end-to-end cases assert on.
type e2eWhich struct {
	ChangedNodes []struct {
		ID string `json:"id"`
	} `json:"changed_nodes"`
	Rounds []struct {
		Round int `json:"round"`
		Tests []struct {
			ID string `json:"id"`
		} `json:"tests"`
	} `json:"rounds"`
	Untested []string `json:"untested"`
}

func (w e2eWhich) changed() []string {
	out := []string{}
	for _, n := range w.ChangedNodes {
		out = append(out, n.ID)
	}
	return out
}

func (w e2eWhich) round(i int) []string {
	out := []string{}
	for _, tc := range w.Rounds[i-1].Tests {
		out = append(out, tc.ID)
	}
	return out
}

// runE2EWhich runs `rtdd which --json` (and, for its text, `rtdd which`) in dir behind a
// tripwire PATH, failing if either ran anything but git.
func runE2EWhich(t *testing.T, dir string) (e2eWhich, string) {
	t.Helper()
	marker := tripwirePATH(t)
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var w e2eWhich
	if err := json.Unmarshal([]byte(out), &w); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(w.Rounds) != 3 {
		t.Fatalf("rounds = %d, want 3:\n%s", len(w.Rounds), out)
	}
	code, text, errOut := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("rtdd which = %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("rtdd which ran %s", b)
	}
	return w, text
}

func assertIDs(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v\nwant exactly %v (file, then line)", what, got, want)
	}
}

// PRD #410 AC7: editing one function selects exactly the tests that call it in Round 1
// and exactly its callers' and callees' tests in Round 2 — in Python and Go, from the
// graph alone, running no python, pytest or go.
func TestWhichEndToEndEditingOneFunctionSelectsItsCallersAndNeighboursTests(t *testing.T) {
	dir := e2eRepo(t)
	e2eEdit(t, dir, "shop/shop.go", "\treturn p + Tax(p)\n", "\treturn p + Tax(p) + 0\n")
	e2eEdit(t, dir, "src/shop.py", "    return p + tax(p)\n", "    return p + tax(p) + 0\n")
	w, _ := runE2EWhich(t, dir)

	assertIDs(t, "changed_nodes", w.changed(), []string{"shop/shop.go::Price", "src/shop.py::price"})
	assertIDs(t, "Round 1", w.round(1), []string{"shop/shop_test.go::TestPrice", "tests/test_shop.py::test_price"})
	assertIDs(t, "Round 2", w.round(2), []string{
		"shop/shop_test.go::TestTax",      // callee's test, line 11
		"shop/shop_test.go::TestCheckout", // caller's test, line 23
		"tests/test_shop.py::test_checkout",
		"tests/test_tax.py::test_tax",
	})
	assertIDs(t, "untested", w.Untested, []string{})
	for _, unrelated := range []string{"TestUnrelated", "test_unrelated"} {
		for i := 1; i <= 2; i++ {
			for _, id := range w.round(i) {
				if strings.HasSuffix(id, "::"+unrelated) {
					t.Errorf("Round %d selected the unrelated %s", i, id)
				}
			}
		}
	}
}

// PRD #410 AC7: editing a function no test reaches lists it as untested, and the text
// output says "no linked test" rather than reading as a pass.
func TestWhichEndToEndEditingAFunctionWithNoLinkedTestListsItUntested(t *testing.T) {
	dir := e2eRepo(t)
	e2eEdit(t, dir, "shop/shop.go", "func Orphan() int {\n\treturn 0\n", "func Orphan() int {\n\treturn 1\n")
	e2eEdit(t, dir, "src/shop.py", "def orphan():\n    return 0\n", "def orphan():\n    return 1\n")
	w, text := runE2EWhich(t, dir)

	assertIDs(t, "changed_nodes", w.changed(), []string{"shop/shop.go::Orphan", "src/shop.py::orphan"})
	assertIDs(t, "Round 1", w.round(1), []string{})
	assertIDs(t, "Round 2", w.round(2), []string{})
	assertIDs(t, "untested", w.Untested, []string{"shop/shop.go::Orphan", "src/shop.py::orphan"})
	if !strings.Contains(text, "no linked test") {
		t.Errorf("text output does not say %q:\n%s", "no linked test", text)
	}
	if !strings.Contains(text, "untested:\n  shop/shop.go::Orphan\n  src/shop.py::orphan\n") {
		t.Errorf("text output does not list both orphans as untested:\n%s", text)
	}
}
