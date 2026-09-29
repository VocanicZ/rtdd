package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestPipelineCargo(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH")
	}
	if _, err := exec.LookPath("cargo-llvm-cov"); err != nil {
		t.Skip("cargo-llvm-cov not on PATH")
	}
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/fixtures/cargo")); err != nil {
		t.Fatal(err)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, "src/store.rs", "\npub fn extra(k: &str) -> String {\n    format!(\"{}!\", k)\n}\n", 3, "tests/api.rs")
}

// A workspace keeps its integration tests under each member (crates/*/tests/*.rs), and
// `cargo llvm-cov --test {name}` from the root runs a member's target.
func TestPipelineCargoWorkspace(t *testing.T) {
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skip("cargo not on PATH")
	}
	if _, err := exec.LookPath("cargo-llvm-cov"); err != nil {
		t.Skip("cargo-llvm-cov not on PATH")
	}
	dir := t.TempDir()
	pkg := func(name, deps string) string {
		return "[package]\nname = \"" + name + "\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\n" + deps
	}
	for rel, body := range map[string]string{
		"Cargo.toml":                "[workspace]\nmembers = [\"crates/store\", \"crates/api\", \"crates/calc\"]\nresolver = \"2\"\n",
		".gitignore":                "target/\nCargo.lock\n.rtdd/\n",
		"crates/store/Cargo.toml":   pkg("store", ""),
		"crates/store/src/lib.rs":   "pub fn get(k: &str) -> String {\n    format!(\"v:{}\", k)\n}\n",
		"crates/api/Cargo.toml":     pkg("api", "store = { path = \"../store\" }\n"),
		"crates/api/src/lib.rs":     "pub fn handle(k: &str) -> String {\n    store::get(k)\n}\n",
		"crates/api/tests/api.rs":   "#[test]\nfn handles() {\n    assert_eq!(api::handle(\"a\"), \"v:a\");\n}\n",
		"crates/calc/Cargo.toml":    pkg("calc", ""),
		"crates/calc/src/lib.rs":    "pub fn add(a: i32, b: i32) -> i32 {\n    a + b\n}\n",
		"crates/calc/tests/calc.rs": "#[test]\nfn adds() {\n    assert_eq!(calc::add(1, 2), 3);\n}\n",
	} {
		gittest.Write(t, dir, rel, body)
	}
	if err := gittest.InitRepo(dir, "init"); err != nil {
		t.Fatal(err)
	}
	pipelineCheck(t, dir, "crates/store/src/lib.rs", "\npub fn extra(k: &str) -> String {\n    format!(\"{}!\", k)\n}\n", 3, "crates/api/tests/api.rs")
}
