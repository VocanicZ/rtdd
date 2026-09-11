package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrNotComparable and ErrNotWritable separate a configuration problem the user can fix
// from a broken environment. The command maps the first to exit 2 and everything else to
// exit 3, so the exit code is a property of the failure and not of its wording.
var (
	ErrNotComparable = errors.New("the running version cannot be ordered against a release")
	ErrNotWritable   = errors.New("the destination cannot be written")
)

// maxArchiveBytes bounds the download itself, before anything is decompressed.
const maxArchiveBytes = 256 << 20

// defaultTimeout is the whole-request budget. An update that hangs is worse than one that
// fails: the user is waiting on a command they expected to be quick.
const defaultTimeout = 60 * time.Second

// Options configures one update. The zero value updates the running binary from the
// project's real release URLs.
type Options struct {
	// Tag installs exactly this release instead of resolving the latest one. Naming a tag
	// is a decision already taken, so it is an override and not a hint: it installs even
	// when it is older than what is running, which is how a rollback works.
	Tag string
	// CheckOnly reports what it found and writes nothing.
	CheckOnly bool
	// Target is the binary to replace. Empty means the running one.
	Target string
	// GOOS and GOARCH select the release archive. Empty means this host.
	GOOS, GOARCH string
	// APIURL and BaseURL mirror install.sh's RTDD_API_URL and RTDD_BASE_URL.
	APIURL, BaseURL string
	// Client is the HTTP client to use. Empty means one with defaultTimeout.
	Client *http.Client
}

// Result is what happened, in the terms the command prints.
type Result struct {
	Current  string // the version that was running
	Latest   string // the tag that was resolved or named
	Target   string // the binary that was, or would be, replaced
	Outdated bool   // a newer release than Current exists
	Updated  bool   // Target now holds Latest
}

// Update replaces the rtdd binary with a published release.
//
// current is passed in rather than read from the build, so that nothing here is coupled to
// what ldflags stamped into any particular binary.
func Update(current string, opts Options) (Result, error) {
	target, err := resolveTarget(opts.Target)
	if err != nil {
		return Result{Current: current}, err
	}
	res := Result{Current: current, Target: target}

	goos, goarch := opts.GOOS, opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	apiURL, baseURL := opts.APIURL, opts.BaseURL
	if apiURL == "" {
		apiURL = defaultAPIURL
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	tag := opts.Tag
	if tag == "" {
		if tag, err = latestTag(client, apiURL); err != nil {
			return res, err
		}
		res.Latest = tag
		cmp, cerr := compare(tag, current)
		if cerr != nil {
			return res, fmt.Errorf("cannot tell whether %s is newer than the running version %q (%v): %w. Name the release to install with --version", tag, current, cerr, ErrNotComparable)
		}
		if cmp <= 0 {
			return res, nil
		}
		res.Outdated = true
	} else {
		res.Latest = tag
		if cmp, cerr := compare(tag, current); cerr == nil && cmp > 0 {
			res.Outdated = true
		}
	}
	if opts.CheckOnly {
		return res, nil
	}

	// The temp file is created before anything is downloaded, so an unwritable destination
	// costs a failed syscall rather than several megabytes and a wait.
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".rtdd-update-*")
	if err != nil {
		return res, fmt.Errorf("cannot write into %s (%v): %w", dir, err, ErrNotWritable)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // a no-op once the rename below has moved it away
	}()

	archive := archiveName(tag, goos, goarch)
	archiveURL := strings.TrimSuffix(baseURL, "/") + "/" + tag + "/" + archive
	body, err := fetch(client, archiveURL)
	if err != nil {
		return res, err
	}
	sums, err := fetch(client, strings.TrimSuffix(baseURL, "/")+"/"+tag+"/checksums.txt")
	if err != nil {
		return res, err
	}
	if err := verify(body, sums, archive); err != nil {
		return res, err
	}
	bin, err := extractBinary(body, archive, binaryName(goos))
	if err != nil {
		return res, err
	}

	if _, err := tmp.Write(bin); err != nil {
		return res, fmt.Errorf("write the new binary into %s: %w", dir, err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return res, fmt.Errorf("make the new binary executable: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return res, fmt.Errorf("close the new binary: %w", err)
	}
	if err := swap(tmpName, target); err != nil {
		return res, err
	}
	res.Updated = true
	return res, nil
}

// resolveTarget finds the binary to replace. The running binary is resolved through any
// symlink, so updating a linked rtdd replaces the real file rather than the link. A target
// named by the caller is taken exactly as given.
func resolveTarget(target string) (string, error) {
	if target != "" {
		return target, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find the running rtdd binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

// swap moves the staged binary over the installed one. Windows cannot replace a running
// executable but can rename it, so the live file is moved aside first and swept after.
func swap(staged, target string) error {
	if runtime.GOOS == "windows" {
		old := target + ".old"
		os.Remove(old)
		if err := os.Rename(target, old); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("move the running binary aside: %w", err)
		}
		if err := os.Rename(staged, target); err != nil {
			os.Rename(old, target) // put it back rather than leave nothing installed
			return fmt.Errorf("install the new binary at %s: %w", target, err)
		}
		os.Remove(old)
		return nil
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("install the new binary at %s: %w", target, err)
	}
	return nil
}

// latestTag reads tag_name out of the GitHub releases API, the same field install.sh greps
// for. A draft release is invisible to this endpoint, which is why docs/RELEASING.md makes
// publishing the draft its own step.
func latestTag(client *http.Client, apiURL string) (string, error) {
	body, err := fetch(client, apiURL)
	if err != nil {
		return "", fmt.Errorf("could not resolve the latest release from %s: %w", apiURL, err)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("could not read the latest release from %s: %w", apiURL, err)
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("%s named no release; if a release exists it may still be an unpublished draft", apiURL)
	}
	return payload.TagName, nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if len(b) > maxArchiveBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes", url, maxArchiveBytes)
	}
	return b, nil
}

// verify is install.sh's checksum step: find this archive's line in checksums.txt and
// compare. A missing entry is a failure, never a skip - an unlisted archive is exactly what
// a substituted one looks like.
func verify(archive, checksums []byte, archiveFilename string) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == archiveFilename {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt carries no entry for %s", archiveFilename)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", archiveFilename, want, got)
	}
	return nil
}
