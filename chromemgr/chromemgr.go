// Package chromemgr resolves a working Chrome/Chromium executable for
// headless rendering without requiring the user to install one by hand.
//
// Resolution order, mirroring what tools like Playwright/Puppeteer do:
//  1. an explicit path (e.g. from --chrome-path / PRETTY_CONVERTER_CHROME_PATH)
//  2. a system-installed Chrome/Chromium/Edge chromedp can already find
//  3. a previously auto-downloaded build in the local cache
//  4. a freshly downloaded "chrome-headless-shell" build for this OS/arch,
//     fetched from Google's official Chrome for Testing distribution
//
// chrome-headless-shell is a small, automation-only build of Chromium (no
// GUI shell) published specifically for tools like this one. It is not
// available for every platform (notably linux/arm64 has no prebuilt
// binary today); EnsureChrome returns a clear error in that case so the
// caller can fall back to asking the user to install Chrome manually.
package chromemgr

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const versionsURL = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

// GOOS/GOARCH values referenced repeatedly below (platform mapping,
// binary naming) — named to satisfy goconst and avoid typos.
const (
	goosLinux   = "linux"
	goosDarwin  = "darwin"
	goosWindows = "windows"

	goarchAMD64 = "amd64"
	goarchARM64 = "arm64"
)

// ProgressFunc receives human-readable status updates while EnsureChrome
// downloads/extracts a browser. It may be nil.
type ProgressFunc func(msg string)

func notify(progress ProgressFunc, msg string) {
	if progress != nil {
		progress(msg)
	}
}

// SystemChromeAvailable reports whether chromedp can already launch a
// browser using its default discovery (PATH plus well-known install
// locations for Chrome/Chromium/Edge). It does no downloading.
func SystemChromeAvailable(ctx context.Context) bool {
	allocCtx, allocCancel := chromedp.NewExecAllocator(
		ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.NoSandbox,
			chromedp.Headless,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.WSURLReadTimeout(10*time.Second),
		)...,
	)
	defer allocCancel()

	cctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	cctx, cancel = context.WithTimeout(cctx, 15*time.Second)
	defer cancel()

	return chromedp.Run(cctx) == nil
}

// EnsureChrome returns a path to a usable Chrome/Chromium executable, or
// "" to mean "let chromedp use its own default discovery" (a system
// install was found, so nothing needs to be downloaded or overridden).
//
// explicitPath, when non-empty, is trusted and used as-is (after an
// existence check) — no system probe or download is attempted.
func EnsureChrome(ctx context.Context, explicitPath string, progress ProgressFunc) (string, error) {
	if explicitPath != "" {
		info, err := os.Stat(explicitPath)
		if err != nil {
			return "", fmt.Errorf("chrome-path %q: %w", explicitPath, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("chrome-path %q is a directory, not an executable", explicitPath)
		}
		return explicitPath, nil
	}

	if SystemChromeAvailable(ctx) {
		return "", nil
	}

	cache, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving cache directory: %w", err)
	}

	if path := findBinary(cache); path != "" {
		notify(progress, "using cached Chrome build at "+path)
		return path, nil
	}

	plat, err := platformStringFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	notify(progress, "downloading a headless Chrome build (one-time, ~100MB)...")
	path, err := downloadAndExtract(ctx, cache, plat, progress)
	if err != nil {
		return "", fmt.Errorf("auto-downloading Chrome: %w (or install Chrome/Chromium manually and pass --chrome-path)", err)
	}
	return path, nil
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "go-pretty-converter", "chrome")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func binaryName() string {
	if runtime.GOOS == goosWindows {
		return "chrome-headless-shell.exe"
	}
	return "chrome-headless-shell"
}

// findBinary walks dir looking for an already-extracted chrome-headless-shell
// binary, regardless of how the archive it came from nested its contents.
// It stops at the first match (filepath.SkipAll) and aborts on the first
// unreadable entry rather than silently continuing.
func findBinary(dir string) string {
	name := binaryName()
	var found string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// platformStringFor maps a Go GOOS/GOARCH pair to the platform identifier
// used by the Chrome for Testing JSON API and download URLs. Chrome for
// Testing does not publish linux/arm64 or windows/arm64 builds as of this
// writing, matching go-pretty-converter's own release matrix minus linux/arm64 —
// that combination surfaces as an explicit error rather than a silent
// failure.
func platformStringFor(goos, goarch string) (string, error) {
	switch goos {
	case goosLinux:
		if goarch == goarchAMD64 {
			return "linux64", nil
		}
	case goosDarwin:
		switch goarch {
		case goarchAMD64:
			return "mac-x64", nil
		case goarchARM64:
			return "mac-arm64", nil
		}
	case goosWindows:
		if goarch == goarchAMD64 {
			return "win64", nil
		}
	}
	return "", fmt.Errorf(
		"no prebuilt Chrome-for-Testing binary for %s/%s — install Chrome/Chromium manually and pass --chrome-path (or set PRETTY_CONVERTER_CHROME_PATH)",
		goos, goarch,
	)
}

type versionManifest struct {
	Channels map[string]struct {
		Version   string `json:"version"`
		Downloads struct {
			ChromeHeadlessShell []struct {
				Platform string `json:"platform"`
				URL      string `json:"url"`
			} `json:"chrome-headless-shell"`
		} `json:"downloads"`
	} `json:"channels"`
}

func downloadAndExtract(ctx context.Context, cache, platform string, progress ProgressFunc) (string, error) {
	manifest, err := fetchManifest(ctx)
	if err != nil {
		return "", err
	}

	stable, ok := manifest.Channels["Stable"]
	if !ok {
		return "", fmt.Errorf("version manifest has no Stable channel")
	}

	var downloadURL string
	for _, d := range stable.Downloads.ChromeHeadlessShell {
		if d.Platform == platform {
			downloadURL = d.URL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no chrome-headless-shell build for platform %q (Chrome %s)", platform, stable.Version)
	}

	versionDir := filepath.Join(cache, stable.Version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(cache, stable.Version+"-"+platform+".zip")
	if err := downloadFile(ctx, downloadURL, zipPath, progress); err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(zipPath) }()

	notify(progress, "extracting Chrome build...")
	if err := unzip(zipPath, versionDir); err != nil {
		return "", fmt.Errorf("extracting archive: %w", err)
	}

	path := findBinary(versionDir)
	if path == "" {
		return "", fmt.Errorf("downloaded archive did not contain %q", binaryName())
	}
	if runtime.GOOS != goosWindows {
		if err := os.Chmod(path, 0o755); err != nil {
			return "", err
		}
	}
	notify(progress, "Chrome ready at "+path)
	return path, nil
}

func fetchManifest(ctx context.Context) (*versionManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching version manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching version manifest: unexpected status %s", resp.Status)
	}

	var manifest versionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("parsing version manifest: %w", err)
	}
	return &manifest, nil
}

func downloadFile(ctx context.Context, url, dest string, progress ProgressFunc) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	// The Chrome for Testing manifest publishes no signed checksum, but the
	// binaries are served straight from Google Cloud Storage, which returns
	// the object's own MD5 in X-Goog-Hash. Checking the downloaded bytes
	// against it is not supply-chain provenance (a compromised origin object
	// would carry a matching header), but it does catch transport-level
	// corruption and a class of MITM that swaps response bytes without also
	// recomputing this header — worth doing since we're about to chmod +x
	// and execute the result. #nosec G401 -- MD5 chosen to match GCS's own
	// object-integrity header, not used for any security-sensitive purpose.
	wantMD5, haveMD5 := gcsMD5FromHeader(resp.Header)
	hasher := md5.New() //nolint:gosec

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	// Registered before the Close defer below so it runs after it (defers
	// run LIFO): removing dest while out is still open fails silently on
	// Windows, which — unlike POSIX — doesn't allow deleting an open file.
	defer func() {
		if err != nil {
			_ = os.Remove(dest)
		}
	}()
	defer func() { _ = out.Close() }()

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 256*1024)
	lastReport := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			hasher.Write(buf[:n])
			written += int64(n)
			if progress != nil && time.Since(lastReport) > 750*time.Millisecond {
				if total > 0 {
					notify(progress, fmt.Sprintf("downloading Chrome... %d%%", written*100/total))
				} else {
					notify(progress, fmt.Sprintf("downloading Chrome... %dMB", written/(1024*1024)))
				}
				lastReport = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	if haveMD5 {
		if gotMD5 := hasher.Sum(nil); !equalDigest(gotMD5, wantMD5) {
			return fmt.Errorf("downloaded file %s failed integrity check (checksum mismatch) — possible corrupted or tampered download", url)
		}
	} else {
		// Not fatal — GCS omits X-Goog-Hash in some configurations — but
		// worth surfacing since the file downloaded here is about to be
		// chmod +x'd and executed with no integrity check at all in that
		// case.
		notify(progress, fmt.Sprintf("warning: no MD5 checksum available for %s, skipping integrity check", url))
	}
	return nil
}

// gcsMD5FromHeader extracts the base64-encoded MD5 digest Google Cloud
// Storage reports for an object via X-Goog-Hash (e.g. "md5=<base64>",
// possibly alongside a "crc32c=<base64>" entry in the same or a separate
// header line).
func gcsMD5FromHeader(h http.Header) (digest []byte, ok bool) {
	for _, line := range h.Values("X-Goog-Hash") {
		for _, part := range strings.Split(line, ",") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 || kv[0] != "md5" {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(kv[1])
			if err != nil {
				continue
			}
			return decoded, true
		}
	}
	return nil, false
}

func equalDigest(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// unzip extracts src into dest, guarding against Zip Slip (archive entries
// that try to write outside dest via "../" path traversal).
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	destClean := filepath.Clean(dest) + string(os.PathSeparator)

	for _, f := range r.File {
		targetPath := filepath.Join(dest, filepath.Clean(f.Name))
		if !strings.HasPrefix(targetPath, destClean) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		if err := extractFile(f, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	mode := f.Mode()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, rc)
	return err
}
