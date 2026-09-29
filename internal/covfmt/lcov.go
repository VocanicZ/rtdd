package covfmt

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// parseLcov reads SF:/DA: records. DA is "line,count[,checksum]".
func parseLcov(r io.Reader) (Lines, error) {
	out := Lines{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	file := ""
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "SF:"):
			file = strings.TrimPrefix(line, "SF:")
		case line == "end_of_record":
			file = ""
		case strings.HasPrefix(line, "DA:") && file != "":
			parts := strings.Split(strings.TrimPrefix(line, "DA:"), ",")
			if len(parts) < 2 {
				continue
			}
			n, err1 := strconv.Atoi(parts[0])
			c, err2 := strconv.ParseInt(parts[1], 10, 64)
			if err1 == nil && err2 == nil && c > 0 {
				out.add(file, n)
			}
		}
	}
	return out, sc.Err()
}
