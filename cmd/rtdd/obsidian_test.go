package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
)

var wikilink = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]*)?\]\]`)

// obsidianEnv points HOME and the platform at a temp dir and swaps the opener for a
// recorder, so no test reads the real obsidian.json or launches Obsidian.
func obsidianEnv(t *testing.T, goos string) (home string, opened *[]string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("XDG_CONFIG_HOME", "")
	oldOS, oldOpen := obsidianGOOS, obsidianOpener
	obsidianGOOS = goos
	opened = new([]string)
	obsidianOpener = func(url string) error { *opened = append(*opened, url); return nil }
	t.Cleanup(func() { obsidianGOOS, obsidianOpener = oldOS, oldOpen })
	return home, opened
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mdFiles(t *testing.T, vault string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	filepath.WalkDir(vault, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			r, _ := filepath.Rel(vault, p)
			out[filepath.ToSlash(r)] = true
		}
		return nil
	})
	return out
}

// #575 AC1+AC2: --no-open writes one note per node at a mirrored path, links resolve.
func TestObsidianNoOpenWritesOneNotePerNode(t *testing.T) {
	_, opened := obsidianEnv(t, "linux")
	dir := explainRepo(t)
	code, out, errOut := rtdd(t, dir, "obsidian", "--no-open")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	root, _ := graphRoot()
	vault := filepath.Join(root, ".rtdd", "obsidian")
	if !strings.Contains(out, vault) {
		t.Errorf("stdout %q does not name the vault %s", out, vault)
	}
	if len(*opened) != 0 {
		t.Errorf("--no-open launched %v", *opened)
	}
	_, res, _, err := buildGraph(root, graphbuild.Options{})
	if err != nil {
		t.Fatal(err)
	}
	notes := mdFiles(t, vault)
	if len(notes) != len(res.Graph.Nodes) {
		t.Errorf("%d notes for %d nodes", len(notes), len(res.Graph.Nodes))
	}
	for p := range notes {
		body := readFile(t, filepath.Join(vault, p))
		for _, m := range wikilink.FindAllStringSubmatch(body, -1) {
			if !notes[m[1]+".md"] {
				t.Errorf("%s: link [[%s]] does not resolve", p, m[1])
			}
		}
	}
	total := readFile(t, filepath.Join(vault, "src/calc.py/total.md"))
	for _, want := range []string{
		`source: "src/calc.py"`, "start: 5", "end: 6", `kind: "func"`, "is-test: false",
		"#rtdd/code", "[[tests/test_calc.py/test_total]]", "[[src/report.py/report]]", "[[src/calc.py/add]]", "calls",
	} {
		if !strings.Contains(total, want) {
			t.Errorf("total.md lacks %q:\n%s", want, total)
		}
	}
	if b := readFile(t, filepath.Join(vault, "tests/test_calc.py/test_total.md")); !strings.Contains(b, "#rtdd/test") || !strings.Contains(b, "is-test: true") {
		t.Errorf("test note not tagged:\n%s", b)
	}
	if b := readFile(t, filepath.Join(vault, "src/calc.py/_file.md")); !strings.Contains(b, "#rtdd/file") {
		t.Errorf("file note not tagged:\n%s", b)
	}
	if ig := readFile(t, filepath.Join(root, ".rtdd", ".gitignore")); !strings.Contains(ig, "obsidian/") {
		t.Errorf(".rtdd/.gitignore = %q, want obsidian/", ig)
	}
	// A second run adds no duplicate .gitignore line.
	rtdd(t, dir, "obsidian", "--no-open")
	if ig := readFile(t, filepath.Join(root, ".rtdd", ".gitignore")); strings.Count(ig, "obsidian/") != 1 {
		t.Errorf(".rtdd/.gitignore = %q, want one obsidian/", ig)
	}
}

// #587: a fresh vault is seeded with tag colour groups and a graph main leaf; an existing
// .obsidian/ is never touched; the hint follows a successful open only.
func TestObsidianSeedsFreshVaultOnly(t *testing.T) {
	home, _ := obsidianEnv(t, "linux")
	dir := explainRepo(t)
	if code, out, e := rtdd(t, dir, "obsidian", "--no-open"); code != 0 || strings.Contains(out, "Ctrl+G") {
		t.Fatalf("exit %d out %q err %q", code, out, e)
	}
	obs := filepath.Join(dir, ".rtdd", "obsidian", ".obsidian")
	var g struct {
		ColorGroups []struct {
			Query string
			Color struct{ A, Rgb int }
		}
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(obs, "graph.json"))), &g); err != nil {
		t.Fatal(err)
	}
	var qs []string
	for _, c := range g.ColorGroups {
		if c.Color.A != 1 || c.Color.Rgb == 0 {
			t.Errorf("group %q has no colour: %+v", c.Query, c.Color)
		}
		qs = append(qs, c.Query)
	}
	if strings.Join(qs, ",") != "tag:#rtdd/test,tag:#rtdd/code,tag:#rtdd/file" {
		t.Errorf("colour groups = %v", qs)
	}
	var ws struct {
		Main struct {
			Children []struct {
				Children []struct {
					Type  string
					State struct{ Type string }
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(obs, "workspace.json"))), &ws); err != nil {
		t.Fatal(err)
	}
	if len(ws.Main.Children) != 1 || len(ws.Main.Children[0].Children) != 1 || ws.Main.Children[0].Children[0].State.Type != "graph" {
		t.Errorf("main leaf is not the graph view: %+v", ws)
	}

	// Edited settings survive a rerun byte for byte, including a rerun that opens.
	edited := map[string]string{"graph.json": `{"mine":1}`, "workspace.json": "{\"main\":{}}\n"}
	for n, c := range edited {
		os.WriteFile(filepath.Join(obs, n), []byte(c), 0o644)
	}
	cfgDir := filepath.Join(home, ".config", "obsidian")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "obsidian.json"), []byte(`{"vaults":{}}`), 0o644)
	code, out, e := rtdd(t, dir, "obsidian")
	if code != 0 {
		t.Fatalf("rerun exit %d %q", code, e)
	}
	for n, c := range edited {
		if got := readFile(t, filepath.Join(obs, n)); got != c {
			t.Errorf("%s changed: %q", n, got)
		}
	}
	if !strings.Contains(out, "Graph view: Ctrl+G") || !strings.Contains(out, "Open local graph") {
		t.Errorf("no graph hint after open: %s", out)
	}
}

// #575 AC3: a rerun drops notes of vanished nodes and leaves .obsidian/ alone.
func TestObsidianRerunRemovesStaleNotesKeepsSettings(t *testing.T) {
	obsidianEnv(t, "linux")
	dir := explainRepo(t)
	rtdd(t, dir, "obsidian", "--no-open")
	root, _ := graphRoot()
	vault := filepath.Join(root, ".rtdd", "obsidian")
	if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(vault, ".obsidian", "graph.json"), []byte("{}"), 0o644)
	os.Remove(filepath.Join(dir, "src", "report.py"))
	if code, _, e := rtdd(t, dir, "obsidian", "--no-open"); code != 0 {
		t.Fatalf("rerun exit %d %q", code, e)
	}
	if _, err := os.Stat(filepath.Join(vault, "src/report.py/report.md")); err == nil {
		t.Error("note for the deleted node survived")
	}
	if _, err := os.Stat(filepath.Join(vault, "src/report.py")); err == nil {
		t.Error("empty folder of the deleted file survived")
	}
	if _, err := os.Stat(filepath.Join(vault, ".obsidian", "graph.json")); err != nil {
		t.Error(".obsidian settings were removed")
	}
	if _, err := os.Stat(filepath.Join(vault, "src/calc.py/total.md")); err != nil {
		t.Error("live note missing after rerun")
	}
}

// #575: names are filesystem-safe, capped, unique, and case-insensitively unique.
func TestObsidianNotePathsAreSafeAndUnique(t *testing.T) {
	long := strings.Repeat("x", 300)
	nodes := []graph.Node{
		{ID: "app.js::app: Open loads <x>?", File: "app.js", Name: "app: Open loads <x>?", Kind: graph.KindTest},
		{ID: "app.js::app; Open loads _x__", File: "app.js", Name: "x", Kind: graph.KindTest},
		{ID: "a.py::<module>", File: "a.py", Name: "<module>", Kind: graph.KindFunc},
		{ID: "a.py::Foo::bar", File: "a.py", Name: "bar", Kind: graph.KindMethod},
		{ID: "a.py::Foo", File: "a.py", Name: "Foo", Kind: graph.KindClass},
		{ID: "a.py::foo", File: "a.py", Name: "foo", Kind: graph.KindFunc},
		{ID: "a.py", File: "a.py", Name: "a.py", Kind: graph.KindFile},
		{ID: "a.py::" + long, File: "a.py", Name: long, Kind: graph.KindFunc},
	}
	got := notePaths(nodes)
	seen := map[string]string{}
	for _, n := range nodes {
		p, ok := got[n.ID]
		if !ok {
			t.Fatalf("no path for %s", n.ID)
		}
		if strings.ContainsAny(p, `<>:"|?*\`) || !strings.HasSuffix(p, ".md") {
			t.Errorf("%s -> unsafe %q", n.ID, p)
		}
		for _, seg := range strings.Split(p, "/") {
			if len(seg) > 255 {
				t.Errorf("%s -> segment of %d bytes", n.ID, len(seg))
			}
		}
		if prev, dup := seen[strings.ToLower(p)]; dup {
			t.Errorf("%s and %s share %q", prev, n.ID, p)
		}
		seen[strings.ToLower(p)] = n.ID
	}
	if got["a.py::Foo::bar"] != "a.py/Foo/bar.md" || got["a.py"] != "a.py/_file.md" {
		t.Errorf("mirrored paths wrong: %q %q", got["a.py::Foo::bar"], got["a.py"])
	}
}

// #575 AC5: obsidian.json location per platform.
func TestObsidianConfigDirs(t *testing.T) {
	for _, c := range []struct {
		goos string
		want []string
	}{
		{"linux", []string{"/h/.config/obsidian", "/h/.var/app/md.obsidian.Obsidian/config/obsidian", "/h/snap/obsidian/current/.config/obsidian"}},
		{"darwin", []string{"/h/Library/Application Support/obsidian"}},
		{"windows", []string{filepath.Join("/h/AppData", "obsidian")}},
	} {
		got := obsidianConfigDirs(c.goos, "/h", "/h/AppData", "")
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %v, want %v", c.goos, got, c.want)
		}
	}
	if got := obsidianConfigDirs("linux", "/h", "", "/x"); got[0] != "/x/obsidian" {
		t.Errorf("XDG_CONFIG_HOME ignored: %v", got)
	}
}

// #575 AC5: registers (reusing the id on rerun), preserves everything else, opens by id.
func TestObsidianRegistersAndOpensVault(t *testing.T) {
	home, opened := obsidianEnv(t, "linux")
	cfgDir := filepath.Join(home, ".var/app/md.obsidian.Obsidian/config/obsidian") // flatpak fallback
	os.MkdirAll(cfgDir, 0o755)
	cfgFile := filepath.Join(cfgDir, "obsidian.json")
	orig := `{"vaults":{"abc0123456789def":{"path":"/other/vault","ts":1,"open":true}},"cli":true,"frame":"hidden"}`
	os.WriteFile(cfgFile, []byte(orig), 0o644)
	dir := explainRepo(t)

	code, out, errOut := rtdd(t, dir, "obsidian")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	root, _ := graphRoot()
	vault := filepath.Join(root, ".rtdd", "obsidian")
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(readFile(t, cfgFile)), &cfg); err != nil {
		t.Fatal(err)
	}
	if string(cfg["cli"]) != "true" || string(cfg["frame"]) != `"hidden"` {
		t.Errorf("other keys changed: %v", cfg)
	}
	var vaults map[string]struct {
		Path string
		Ts   int64
		Open bool
	}
	json.Unmarshal(cfg["vaults"], &vaults)
	if v := vaults["abc0123456789def"]; v.Path != "/other/vault" || !v.Open || v.Ts != 1 {
		t.Errorf("other vault changed: %+v", v)
	}
	id := ""
	for k, v := range vaults {
		if v.Path == vault {
			id = k
			if v.Ts == 0 {
				t.Error("ts not set")
			}
		}
	}
	if len(id) != 16 || len(vaults) != 2 {
		t.Fatalf("vault not added once under a 16-hex id: %v", vaults)
	}
	if len(*opened) != 1 || (*opened)[0] != "obsidian://open?vault="+id {
		t.Errorf("opened %v, want obsidian://open?vault=%s", *opened, id)
	}
	for _, want := range []string{vault, "restart"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q: %s", want, out)
		}
	}
	rtdd(t, dir, "obsidian")
	json.Unmarshal([]byte(readFile(t, cfgFile)), &cfg)
	json.Unmarshal(cfg["vaults"], &vaults)
	if len(vaults) != 2 || len(*opened) != 2 || (*opened)[1] != (*opened)[0] {
		t.Errorf("rerun did not reuse the id: %v %v", vaults, *opened)
	}
}

// #575 AC5: Windows registers under %APPDATA% and launches through cmd /c start.
func TestObsidianWindowsConfigLocation(t *testing.T) {
	home, opened := obsidianEnv(t, "windows")
	cfgDir := filepath.Join(home, "AppData", "obsidian")
	os.MkdirAll(cfgDir, 0o755)
	os.WriteFile(filepath.Join(cfgDir, "obsidian.json"), []byte(`{"vaults":{}}`), 0o644)
	dir := explainRepo(t)
	if code, _, e := rtdd(t, dir, "obsidian"); code != 0 {
		t.Fatalf("exit %d %q", code, e)
	}
	if !strings.Contains(readFile(t, filepath.Join(cfgDir, "obsidian.json")), `"path"`) || len(*opened) != 1 {
		t.Error("windows vault not registered/opened")
	}
}

// #575 AC5: no obsidian.json -> vault written, hint printed, exit 0, nothing launched.
func TestObsidianWithoutObsidianInstalled(t *testing.T) {
	home, opened := obsidianEnv(t, "linux")
	dir := explainRepo(t)
	code, out, _ := rtdd(t, dir, "obsidian")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "open this folder as a vault in Obsidian") {
		t.Errorf("no hint: %s", out)
	}
	if len(*opened) != 0 {
		t.Errorf("launched %v", *opened)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "obsidian", "obsidian.json")); err == nil {
		t.Error("created an obsidian.json")
	}
	if len(mdFiles(t, filepath.Join(dir, ".rtdd", "obsidian"))) == 0 {
		t.Error("vault not written")
	}
}

// #575: --no-open leaves obsidian.json byte-identical.
func TestObsidianNoOpenDoesNotRegister(t *testing.T) {
	home, opened := obsidianEnv(t, "linux")
	cfgDir := filepath.Join(home, ".config", "obsidian")
	os.MkdirAll(cfgDir, 0o755)
	orig := `{ "vaults": {} }`
	os.WriteFile(filepath.Join(cfgDir, "obsidian.json"), []byte(orig), 0o644)
	rtdd(t, explainRepo(t), "obsidian", "--no-open")
	if got := readFile(t, filepath.Join(cfgDir, "obsidian.json")); got != orig || len(*opened) != 0 {
		t.Errorf("--no-open touched obsidian: %q %v", got, *opened)
	}
}

// #575 AC6: --help documents the command.
func TestObsidianInHelp(t *testing.T) {
	_, out, _ := rtdd(t, newTestRepo(t), "--help")
	if !strings.Contains(out, "rtdd obsidian") {
		t.Errorf("usage lacks obsidian:\n%s", out)
	}
}
