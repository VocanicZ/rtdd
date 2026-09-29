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
				Mi int `xml:"mi,attr"`
				Ci int `xml:"ci,attr"`
			} `xml:"line"`
		} `xml:"sourcefile"`
	} `xml:"package"`
}

// parseJacoco reports package-relative paths ("com/foo/Bar.java"); Resolve maps them onto
// the source root by suffix. A line is hit when it has covered instructions (ci > 0) and
// executable when it has any (mi+ci > 0).
func parseJacoco(r io.Reader) (Report, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false // JaCoCo emits a DOCTYPE pointing at report.dtd
	var doc jacocoDoc
	if err := dec.Decode(&doc); err != nil {
		return Report{}, err
	}
	out := newReport()
	for _, p := range doc.Packages {
		for _, f := range p.Files {
			for _, l := range f.Lines {
				// A line with no instructions at all (mi+ci == 0) is not executable.
				if l.Mi+l.Ci > 0 {
					out.add(path.Join(p.Name, f.Name), l.Nr, l.Ci > 0)
				}
			}
		}
	}
	return out, nil
}
