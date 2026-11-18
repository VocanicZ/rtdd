package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// Seams: tests swap the platform and the launcher so none reads the real obsidian.json
// or starts Obsidian.
var (
	obsidianGOOS   = runtime.GOOS
	obsidianOpener = openURL
)

const (
	obsidianIgnoreLine = "obsidian/"
	maxNoteSegment     = 100 // bytes; well under every filesystem's 255
	fileNoteName       = "_file"
)

// cmdObsidian implements `rtdd obsidian [--no-open]` (#575): it refreshes the graph as
// `rtdd which` does, writes it as an Obsidian vault in .rtdd/obsidian/, registers the
// vault in Obsidian's obsidian.json and opens it. It runs no test.
func cmdObsidian(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("obsidian", flag.ContinueOnError)
	fs.SetOutput(stderr)
	noOpen := fs.Bool("no-open", false, "write the vault only: no registration, no launch")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd obsidian [--no-open]")
		return 2
	}
	root, err := graphRoot()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: %v\n", err)
		return 3
	}
	_, res, code, err := buildGraph(root, graphbuild.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: %v\n", err)
		return code
	}
	vault := filepath.Join(root, ".rtdd", "obsidian")
	if err := seedVault(vault); err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: %v\n", err)
		return 3
	}
	if err := writeVault(vault, res.Graph); err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: %v\n", err)
		return 3
	}
	if err := ignoreVault(root); err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: %v\n", err)
		return 3
	}
	fmt.Fprintf(stdout, "vault: %s\n", vault)
	if *noOpen {
		return 0
	}
	home, _ := os.UserHomeDir()
	cfg := findObsidianConfig(obsidianConfigDirs(obsidianGOOS, home, os.Getenv("APPDATA"), os.Getenv("XDG_CONFIG_HOME")))
	if cfg == "" {
		fmt.Fprintln(stdout, "Obsidian's obsidian.json was not found: open this folder as a vault in Obsidian.")
		return 0
	}
	id, err := registerVault(cfg, vault)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: cannot register the vault in %s: %v\nopen this folder as a vault in Obsidian.\n", cfg, err)
		return 3
	}
	if err := obsidianOpener("obsidian://open?vault=" + id); err != nil {
		fmt.Fprintf(stderr, "rtdd obsidian: cannot launch Obsidian: %v\n", err)
		return 3
	}
	fmt.Fprintln(stdout, "If Obsidian was already running and does not show the vault, restart it: a running Obsidian keeps its vault list in memory.")
	fmt.Fprintln(stdout, graphViewHint)
	return 0
}

const graphViewHint = `Graph view: Ctrl+G. A node's neighbours: open its note, then "Open local graph".`

// seedVault gives a vault being created its first-open settings: tag colour groups in
// .obsidian/graph.json and a main leaf showing the graph view in .obsidian/workspace.json.
// A vault that already has a .obsidian/ is left alone: the owner's settings win.
func seedVault(vault string) error {
	dir := filepath.Join(vault, ".obsidian")
	if _, err := os.Lstat(dir); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	group := func(tag string, rgb int) map[string]any {
		return map[string]any{"query": "tag:#rtdd/" + tag, "color": map[string]any{"a": 1, "rgb": rgb}}
	}
	files := map[string]any{
		"graph.json": map[string]any{"colorGroups": []any{
			group("test", 0x3fb950), group("code", 0x58a6ff), group("file", 0xd29922),
		}},
		"workspace.json": map[string]any{
			"main": map[string]any{
				"id": "rtddmain00000001", "type": "split", "direction": "vertical",
				"children": []any{map[string]any{
					"id": "rtddtabs00000001", "type": "tabs",
					"children": []any{map[string]any{
						"id": "rtddgraph0000001", "type": "leaf",
						"state": map[string]any{"type": "graph", "state": map[string]any{}, "icon": "lucide-git-fork", "title": "Graph view"},
					}},
				}},
			},
			"active": "rtddgraph0000001",
		},
	}
	for name, v := range files {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// obsidianConfigDirs lists where obsidian.json may live on goos, in the order to try.
func obsidianConfigDirs(goos, home, appdata, xdg string) []string {
	switch goos {
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "obsidian")}
	case "windows":
		return []string{filepath.Join(appdata, "obsidian")}
	}
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(xdg, "obsidian"),
		filepath.Join(home, ".var", "app", "md.obsidian.Obsidian", "config", "obsidian"),
		filepath.Join(home, "snap", "obsidian", "current", ".config", "obsidian"),
	}
}

// findObsidianConfig is the first obsidian.json among dirs, "" if none exists.
func findObsidianConfig(dirs []string) string {
	for _, d := range dirs {
		p := filepath.Join(d, "obsidian.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// registerVault makes sure the obsidian.json at cfg lists vault, reusing the id of an
// entry with the same path, and returns the id. Nothing else in the file changes; the
// write is atomic.
func registerVault(cfg, vault string) (string, error) {
	b, err := os.ReadFile(cfg)
	if err != nil {
		return "", err
	}
	top := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &top); err != nil {
		return "", err
	}
	vaults := map[string]json.RawMessage{}
	if raw, ok := top["vaults"]; ok {
		if err := json.Unmarshal(raw, &vaults); err != nil {
			return "", err
		}
	}
	for id, raw := range vaults {
		var v struct{ Path string }
		if json.Unmarshal(raw, &v) == nil && filepath.Clean(v.Path) == filepath.Clean(vault) {
			return id, nil
		}
	}
	var rb [8]byte
	if _, err := rand.Read(rb[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(rb[:])
	vaults[id], _ = json.Marshal(map[string]any{"path": vault, "ts": time.Now().UnixMilli()})
	top["vaults"], _ = json.Marshal(vaults)
	out, err := json.Marshal(top)
	if err != nil {
		return "", err
	}
	tmp := cfg + ".rtdd-tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, cfg); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return id, nil
}

// openURL hands url to the platform's opener.
func openURL(url string) error {
	var c *exec.Cmd
	switch obsidianGOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("cmd", "/c", "start", "", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Run()
}

// ignoreVault adds obsidian/ to .rtdd/.gitignore unless a line already names it, so
// repositories set up by an older init ignore the vault too.
func ignoreVault(root string) error {
	p := filepath.Join(root, ".rtdd", ".gitignore")
	b, err := os.ReadFile(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l == obsidianIgnoreLine || l == "/"+obsidianIgnoreLine {
			return nil
		}
	}
	body := string(b)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(body+obsidianIgnoreLine+"\n"), 0o644)
}

// safeSegment makes s one path segment valid on every OS and at most maxNoteSegment bytes.
func safeSegment(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`<>:"|?*\/[]#^`, r) {
			return '_'
		}
		return r
	}, s)
	if len(s) > maxNoteSegment {
		n := maxNoteSegment
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n]
	}
	s = strings.TrimRight(s, ". ")
	if s == "" {
		return "_"
	}
	return s
}

// notePaths maps each node ID to its note's slash-separated path inside the vault,
// mirroring the repo: file/a/b.md for file::a::b, file/_file.md for the file node. Names
// are filesystem-safe and unique even case-insensitively.
func notePaths(nodes []graph.Node) map[string]string {
	out := make(map[string]string, len(nodes))
	taken := map[string]bool{}
	sorted := slices.Clone(nodes)
	slices.SortFunc(sorted, func(a, b graph.Node) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start), strings.Compare(a.ID, b.ID))
	})
	for _, n := range sorted {
		var segs []string
		for _, s := range strings.Split(n.File, "/") {
			segs = append(segs, safeSegment(s))
		}
		if n.Kind == graph.KindFile || n.ID == n.File {
			segs = append(segs, fileNoteName)
		} else {
			for _, s := range strings.Split(strings.TrimPrefix(strings.TrimPrefix(n.ID, n.File), "::"), "::") {
				segs = append(segs, safeSegment(s))
			}
		}
		base := strings.Join(segs, "/")
		p := base + ".md"
		for i := 2; taken[strings.ToLower(p)]; i++ {
			p = base + "_" + strconv.Itoa(i) + ".md"
		}
		taken[strings.ToLower(p)] = true
		out[n.ID] = p
	}
	return out
}

// writeVault writes one note per node of g under vault, then removes every other .md
// note and every emptied folder, leaving the vault's .obsidian/ settings alone.
func writeVault(vault string, g graph.Graph) error {
	paths := notePaths(g.Nodes)
	keep := map[string]bool{}
	for _, n := range g.Nodes {
		keep[paths[n.ID]] = true
		p := filepath.Join(vault, filepath.FromSlash(paths[n.ID]))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(renderNote(n, rounds.LinksOf(g, n.ID), paths)), 0o644); err != nil {
			return err
		}
	}
	var dirs []string
	filepath.WalkDir(vault, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(vault, p)
		rel = filepath.ToSlash(rel)
		switch {
		case rel == ".obsidian":
			return filepath.SkipDir
		case d.IsDir():
			dirs = append(dirs, p)
		case strings.HasSuffix(rel, ".md") && !keep[rel]:
			os.Remove(p)
		}
		return nil
	})
	for i := len(dirs) - 1; i > 0; i-- {
		os.Remove(dirs[i]) // fails, harmlessly, unless the folder is empty
	}
	return os.MkdirAll(vault, 0o755)
}

// renderNote is n's note: frontmatter, its tag, then its tests, callers and callees.
func renderNote(n graph.Node, l rounds.Links, paths map[string]string) string {
	tag := "rtdd/code"
	switch {
	case n.Kind == graph.KindFile:
		tag = "rtdd/file"
	case n.IsTest:
		tag = "rtdd/test"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %s\nsource: %s\nstart: %d\nend: %d\nkind: %s\nis-test: %t\ntags: [%s]\n---\n",
		strconv.Quote(n.ID), strconv.Quote(n.File), n.Start, n.End, strconv.Quote(string(n.Kind)), n.IsTest, tag)
	fmt.Fprintf(&b, "# %s\n\n#%s\n", n.Name, tag)
	for _, sec := range []struct {
		head  string
		links []rounds.Link
	}{{"Tests", l.Tests}, {"Callers", l.Callers}, {"Callees", l.Callees}} {
		fmt.Fprintf(&b, "\n## %s\n\n", sec.head)
		if len(sec.links) == 0 {
			b.WriteString("none\n")
		}
		for _, x := range sec.links {
			if p, ok := paths[x.Node.ID]; ok {
				fmt.Fprintf(&b, "- [[%s]] (%s)\n", strings.TrimSuffix(p, ".md"), x.Relation)
			}
		}
	}
	return b.String()
}
