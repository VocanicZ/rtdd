package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

// tarGz builds an archive shaped like the one .goreleaser.yaml ships: the binary plus the
// README and LICENSE that travel with it.
func tarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("tar body %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("zip body %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// TestExtractBinaryTakesOnlyTheBinaryFromATarball asserts the README and LICENSE riding
// along in the archive are not what gets written over the running command.
func TestExtractBinaryTakesOnlyTheBinaryFromATarball(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"README.md": "not the binary",
		"rtdd":      "ELF-ish binary bytes",
		"LICENSE":   "not the binary either",
	})
	got, err := extractBinary(archive, "rtdd_9.9.9_linux_amd64.tar.gz", "rtdd")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if string(got) != "ELF-ish binary bytes" {
		t.Errorf("extractBinary returned %q, want the rtdd entry", got)
	}
}

// TestExtractBinaryReadsAZipForWindows covers the one platform whose archive is not a
// tarball. Handing a zip to the tar reader is a failure, not a fallback.
func TestExtractBinaryReadsAZipForWindows(t *testing.T) {
	archive := zipArchive(t, map[string]string{
		"README.md": "not the binary",
		"rtdd.exe":  "PE-ish binary bytes",
	})
	got, err := extractBinary(archive, "rtdd_9.9.9_windows_amd64.zip", "rtdd.exe")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if string(got) != "PE-ish binary bytes" {
		t.Errorf("extractBinary returned %q, want the rtdd.exe entry", got)
	}
}

// TestExtractBinaryRefusesAnArchiveWithoutTheBinary is the case that would otherwise
// truncate the installed command to nothing.
func TestExtractBinaryRefusesAnArchiveWithoutTheBinary(t *testing.T) {
	archive := tarGz(t, map[string]string{"README.md": "only docs in here"})
	if _, err := extractBinary(archive, "rtdd_9.9.9_linux_amd64.tar.gz", "rtdd"); err == nil {
		t.Fatal("extractBinary accepted an archive carrying no rtdd entry")
	}
}

// TestExtractBinaryRefusesAnArchiveInTheWrongFormat guards the reader being chosen by the
// name rather than by sniffing, which is what makes format_overrides testable at all.
func TestExtractBinaryRefusesAnArchiveInTheWrongFormat(t *testing.T) {
	if _, err := extractBinary(zipArchive(t, map[string]string{"rtdd": "x"}), "rtdd_9.9.9_linux_amd64.tar.gz", "rtdd"); err == nil {
		t.Fatal("extractBinary read a zip through the tar reader")
	}
	if _, err := extractBinary(tarGz(t, map[string]string{"rtdd.exe": "x"}), "rtdd_9.9.9_windows_amd64.zip", "rtdd.exe"); err == nil {
		t.Fatal("extractBinary read a tarball through the zip reader")
	}
}
