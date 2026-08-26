// Package pytestfixture materialises a tiny, self-contained pytest project on
// disk so the coverage, report and runner packages can be tested against the
// real toolchain rather than against a mock of it.
//
// The fixture's line numbers and test ids are pinned by fixture_test.go, because
// every coverage assertion in M1b depends on them.
//
// Test-only: nothing under cmd/ may import this package.
package pytestfixture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// PyProject makes the directory a pytest rootdir, which is what makes coverage
// contexts and report-log nodeids repo-root-relative.
const PyProject = "[project]\nname = \"rtddfixture\"\nversion = \"0.1.0\"\n"

// Constants is audit A1 in miniature: imported and asserted on by two passing
// tests, attributed to zero test contexts. Import-time lines: 1, 3, 5, 6, 7.
const Constants = `from dataclasses import dataclass

MAX = 10

@dataclass
class Cfg:
    a: int = 1
`

// Logic has import-time lines 1, 4, 8, 12; add's body is line 5, mul's is line 9,
// and unused's line 13 is covered by nothing.
const Logic = `from src.constants import MAX


def add(a, b):
    return a + b


def mul(a, b):
    return a * b


def unused(x):
    return x - 1
`

// TestA produces the parametrised ids `test_param[1-one two]` (a space) and
// `test_param[2-a-b]` (a hyphen), both of which must round-trip as selectors.
const TestA = `import pytest
from src.logic import add
from src.constants import MAX, Cfg


def test_add():
    assert add(1, 2) == 3


@pytest.mark.parametrize("n,label", [(1, "one two"), (2, "a-b")])
def test_param(n, label):
    assert add(n, 0) == n
    assert isinstance(label, str)


def test_const():
    assert MAX == 10
    assert Cfg().a == 1
`

// TestB carries one deliberate failure and one skip, so a run over it exits 1 and
// exercises the "fail" and "skip" status paths.
const TestB = `import pytest
from src.logic import mul


def test_mul():
    assert mul(2, 3) == 6


def test_fail():
    assert mul(2, 3) == 7


@pytest.mark.skip(reason="nope")
def test_skipped():
    assert False
`

// Files is the whole fixture, keyed by repo-relative slash path.
var Files = map[string]string{
	"pyproject.toml":    PyProject,
	"src/__init__.py":   "",
	"src/constants.py":  Constants,
	"src/logic.py":      Logic,
	"tests/__init__.py": "",
	"tests/test_a.py":   TestA,
	"tests/test_b.py":   TestB,
}

// Materialize writes the fixture project into dir. It touches nothing else, needs
// no network, and may be called repeatedly over the same tree.
func Materialize(dir string) error {
	for rel, body := range Files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("pytestfixture: %w", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return fmt.Errorf("pytestfixture: %w", err)
		}
	}
	return nil
}

// InitGit turns dir into a git repository with one commit containing the whole
// fixture, so internal/gitctx can compute a changed set and a HEAD SHA.
//
// The shell-out itself lives in internal/gitctx/gittest: internal/gitctx is the
// only part of the tree permitted to invoke git, and internal/contract enforces it.
func InitGit(dir string) error {
	if err := gittest.InitRepo(dir, "fixture"); err != nil {
		return fmt.Errorf("pytestfixture: %w", err)
	}
	return nil
}

// HavePytest reports whether `pytest` is on PATH. Integration tests skip without it.
func HavePytest() bool {
	_, err := exec.LookPath("pytest")
	return err == nil
}
