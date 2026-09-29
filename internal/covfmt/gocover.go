package covfmt

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// parseGocover reads `go test -coverprofile` output:
// "mode: set" then "path:startLine.startCol,endLine.endCol numStmts count".
// A block with count > 0 hits every line from startLine to endLine.
func parseGocover(r io.Reader) (Lines, error) {
	out := Lines{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	if !sc.Scan() || !strings.HasPrefix(sc.Text(), "mode:") {
		return nil, errors.New("missing mode line")
	}
	for sc.Scan() {
		line := sc.Text()
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			continue
		}
		fields := strings.Fields(line[colon+1:])
		if len(fields) != 3 {
			continue
		}
		if c, err := strconv.ParseInt(fields[2], 10, 64); err != nil || c == 0 {
			continue
		}
		span := strings.SplitN(fields[0], ",", 2)
		if len(span) != 2 {
			continue
		}
		start, err1 := strconv.Atoi(strings.SplitN(span[0], ".", 2)[0])
		end, err2 := strconv.Atoi(strings.SplitN(span[1], ".", 2)[0])
		if err1 != nil || err2 != nil || end < start {
			continue
		}
		for n := start; n <= end; n++ {
			out.add(line[:colon], n)
		}
	}
	return out, sc.Err()
}
