package covfmt

import (
	"encoding/xml"
	"io"
	"path"
)

type coberturaDoc struct {
	Sources []string `xml:"sources>source"`
	Classes []struct {
		Filename string `xml:"filename,attr"`
		Lines    []struct {
			Number int   `xml:"number,attr"`
			Hits   int64 `xml:"hits,attr"`
		} `xml:"lines>line"`
	} `xml:"packages>package>classes>class"`
}

// parseCobertura joins each class filename to the report's single <source> when there is
// exactly one; with several, the filename is left for Resolve's suffix match.
func parseCobertura(r io.Reader) (Lines, error) {
	var doc coberturaDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}
	out := Lines{}
	for _, c := range doc.Classes {
		p := c.Filename
		if len(doc.Sources) == 1 && !path.IsAbs(p) {
			p = path.Join(doc.Sources[0], p)
		}
		for _, l := range c.Lines {
			if l.Hits > 0 {
				out.add(p, l.Number)
			}
		}
	}
	return out, nil
}
