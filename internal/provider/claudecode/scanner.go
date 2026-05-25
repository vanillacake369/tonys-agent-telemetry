package claudecode

import (
	"bufio"
	"io"
)

func newJSONLScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	return s
}
