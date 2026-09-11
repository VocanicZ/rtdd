package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
)

// maxBinaryBytes bounds what will be read out of an archive into memory. The shipped
// binary is a few megabytes; this leaves room for it to grow an order of magnitude and
// still refuses a decompression bomb.
const maxBinaryBytes = 256 << 20

// extractBinary returns the named entry from a release archive. The reader is chosen by
// the archive's own name, not by sniffing its bytes: .goreleaser.yaml's format_overrides
// is the claim that Windows ships a zip and everything else a tarball, and an archive that
// does not match the name it was published under is a failure rather than a fallback.
func extractBinary(archive []byte, archiveFilename, want string) ([]byte, error) {
	switch {
	case strings.HasSuffix(archiveFilename, ".zip"):
		return fromZip(archive, want)
	case strings.HasSuffix(archiveFilename, ".tar.gz"):
		return fromTarGz(archive, want)
	default:
		return nil, fmt.Errorf("%s has no archive extension this command can read", archiveFilename)
	}
}

func fromTarGz(archive []byte, want string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read the archive as gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read the archive as tar: %w", err)
		}
		if h.Name != want {
			continue
		}
		return readAll(tr, want)
	}
	return nil, fmt.Errorf("the archive contains no %s entry", want)
}

func fromZip(archive []byte, want string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("read the archive as zip: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s inside the archive: %w", want, err)
		}
		defer rc.Close()
		return readAll(rc, want)
	}
	return nil, fmt.Errorf("the archive contains no %s entry", want)
}

// readAll reads one archive entry under a size bound.
func readAll(r io.Reader, name string) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxBinaryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s out of the archive: %w", name, err)
	}
	if len(b) > maxBinaryBytes {
		return nil, fmt.Errorf("%s in the archive exceeds %d bytes", name, maxBinaryBytes)
	}
	return b, nil
}
