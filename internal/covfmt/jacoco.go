package covfmt

import (
	"encoding/xml"
	"io"
	"path"
)

type jacocoDoc struct {
	Packages []struct {
		Name  string `xml:"name,attr"`
		Files []struct {
			Name  string `xml:"name,attr"`
			Lines []struct {
				Nr int `xml:"nr,attr"`
				Ci int `xml:"ci,attr"`
			} `xml:"line"`
		} `xml:"sourcefile"`
	} `xml:"package"`
}

// parseJacoco reports package-relative paths ("com/foo/Bar.java"); Resolve maps them onto
// the source root by suffix. A line is hit when it has covered instructions (ci > 0).
func parseJacoco(r io.Reader) (Lines, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false // JaCoCo emits a DOCTYPE pointing at report.dtd
	var doc jacocoDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	out := Lines{}
	for _, p := range doc.Packages {
		for _, f := range p.Files {
			for _, l := range f.Lines {
				if l.Ci > 0 {
					out.add(path.Join(p.Name, f.Name), l.Nr)
				}
			}
		}
	}
	return out, nil
}
