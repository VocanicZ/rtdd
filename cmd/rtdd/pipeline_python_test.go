package main

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelinePython(t *testing.T) {
	pytest, err := exec.LookPath("pytest")
	if err != nil {
		t.Skip("pytest not on PATH")
	}
	py := filepath.Join(filepath.Dir(pytest), "python")
	if _, err := exec.LookPath(py); err != nil {
		py = "python3"
	}
	if err := exec.Command(py, "-c", "import pytest_cov").Run(); err != nil {
		t.Skip("pytest-cov not importable")
	}
	dir := t.TempDir()
	for rel, body := range map[string]string{
		// The old adapter assumed the same thing: real repos import their package via an
		// installed package, a root conftest.py, or pytest's pythonpath. This fixture uses
		// pythonpath.
		"pytest.ini":          "[pytest]\npythonpath = .\n",
		"app/__init__.py":     "",
		"app/store.py":        "def get(k): return \"v:\" + k\n",
		"app/api.py":          "from app.store import get\ndef handle(k): return get(k)\n",
		"tests/test_api.py":   "from app.api import handle\ndef test_handle(): assert handle(\"a\") == \"v:a\"\n",
		"tests/test_other.py": "def test_other(): assert 1 + 1 == 2\n",
		".gitignore":          "__pycache__/\n.rtdd/\n",
	} {
		gittest.Write(t, dir, rel, body)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, "app/store.py", "def extra(k):\n    return k + \"!\"\n", 2, "tests/test_api.py")
}
