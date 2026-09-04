// Package protocol parses protocol/PROTOCOL.md, the single source for every
// generated agent front-end, and renders it per target.
//
// The source carries per-target body variants because one body cannot serve all
// three targets well: a Claude Code SKILL.md is progressively disclosed and can
// be long, an AGENTS.md is always in context and must be short, and a Cursor
// .mdc needs glob frontmatter. Templating one body into three wrappers would
// leave two of the three wrong while a byte-level drift check still passed.
package protocol

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// KnownTargets is closed on purpose: a typo in a targets= list must fail the
// build rather than silently drop a section from one front-end.
var KnownTargets = map[string]bool{"skill": true, "agents": true, "mdc": true}

type Section struct {
	ID       string
	Title    string
	Targets  []string
	Order    int
	Body     string
	Variants map[string]string
}

type Doc struct {
	Version  int
	Sections []Section
}

func (s Section) BodyFor(target string) string {
	if v, ok := s.Variants[target]; ok {
		return v
	}
	return s.Body
}

func (s Section) HasTarget(target string) bool {
	for _, t := range s.Targets {
		if t == target {
			return true
		}
	}
	return false
}

func (d *Doc) For(target string) []Section {
	out := make([]Section, 0, len(d.Sections))
	for _, s := range d.Sections {
		if s.HasTarget(target) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

const (
	metaPrefix      = "<!-- rtdd:meta "
	sectionPrefix   = "<!-- rtdd:section "
	sectionEnd      = "<!-- rtdd:endsection -->"
	variantPrefix   = "<!-- rtdd:variant "
	variantEnd      = "<!-- rtdd:endvariant -->"
	directiveSuffix = " -->"
)

// attrs parses `key=value key="quoted value"` into a map.
func attrs(s string) (map[string]string, error) {
	out := map[string]string{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return nil, fmt.Errorf("attribute without '=': %q", rest)
		}
		key := strings.TrimSpace(rest[:eq])
		rest = rest[eq+1:]
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("unterminated quote in attribute %q", key)
			}
			value = rest[1 : 1+end]
			rest = rest[2+end:]
		} else {
			sp := strings.IndexAny(rest, " \t")
			if sp < 0 {
				value, rest = rest, ""
			} else {
				value, rest = rest[:sp], rest[sp:]
			}
		}
		out[key] = value
		rest = strings.TrimSpace(rest)
	}
	return out, nil
}

func directive(line, prefix string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, directiveSuffix) {
		return "", false
	}
	return line[len(prefix) : len(line)-len(directiveSuffix)], true
}

func Parse(src string) (*Doc, error) {
	lines := strings.Split(src, "\n")
	doc := &Doc{}
	seen := map[string]bool{}
	sawMeta := false

	for i := 0; i < len(lines); i++ {
		if raw, ok := directive(lines[i], metaPrefix); ok {
			a, err := attrs(raw)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			v, err := strconv.Atoi(a["version"])
			if err != nil {
				return nil, fmt.Errorf("line %d: meta version must be an integer", i+1)
			}
			doc.Version = v
			sawMeta = true
			continue
		}

		raw, ok := directive(lines[i], sectionPrefix)
		if !ok {
			continue
		}
		a, err := attrs(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		sec := Section{ID: a["id"], Title: a["title"], Variants: map[string]string{}}
		if sec.ID == "" {
			return nil, fmt.Errorf("line %d: section has no id", i+1)
		}
		if sec.Title == "" {
			return nil, fmt.Errorf("line %d: section %q has no title", i+1, sec.ID)
		}
		if seen[sec.ID] {
			return nil, fmt.Errorf("line %d: duplicate section id %q", i+1, sec.ID)
		}
		seen[sec.ID] = true
		if a["targets"] == "" {
			return nil, fmt.Errorf("line %d: section %q has no targets", i+1, sec.ID)
		}
		for _, t := range strings.Split(a["targets"], ",") {
			t = strings.TrimSpace(t)
			if !KnownTargets[t] {
				return nil, fmt.Errorf("line %d: section %q: unknown target %q", i+1, sec.ID, t)
			}
			sec.Targets = append(sec.Targets, t)
		}
		sec.Order, err = strconv.Atoi(a["order"])
		if err != nil {
			return nil, fmt.Errorf("line %d: section %q: order must be an integer", i+1, sec.ID)
		}

		var body []string
		closed := false
		for i++; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == sectionEnd {
				closed = true
				break
			}
			if vraw, ok := directive(lines[i], variantPrefix); ok {
				va, err := attrs(vraw)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}
				target := va["target"]
				if !KnownTargets[target] {
					return nil, fmt.Errorf("line %d: section %q: unknown variant target %q", i+1, sec.ID, target)
				}
				var vbody []string
				vclosed := false
				for i++; i < len(lines); i++ {
					if strings.TrimSpace(lines[i]) == variantEnd {
						vclosed = true
						break
					}
					vbody = append(vbody, lines[i])
				}
				if !vclosed {
					return nil, fmt.Errorf("section %q: unterminated variant for %q", sec.ID, target)
				}
				sec.Variants[target] = strings.TrimSpace(strings.Join(vbody, "\n"))
				continue
			}
			body = append(body, lines[i])
		}
		if !closed {
			return nil, fmt.Errorf("section %q: unterminated (missing %s)", sec.ID, sectionEnd)
		}
		sec.Body = strings.TrimSpace(strings.Join(body, "\n"))
		if sec.Body == "" {
			return nil, fmt.Errorf("section %q: empty body", sec.ID)
		}
		doc.Sections = append(doc.Sections, sec)
	}

	if !sawMeta {
		return nil, fmt.Errorf("missing %sversion=N%s", metaPrefix, directiveSuffix)
	}
	if len(doc.Sections) == 0 {
		return nil, fmt.Errorf("no sections found")
	}
	return doc, nil
}
