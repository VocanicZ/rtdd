package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func readProtocolMD(t *testing.T) (string, error) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))
	return string(src), err
}
