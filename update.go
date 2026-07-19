package menuet

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// maxDownloadBytes caps an update download so a hostile or misbehaving origin
// can't fill the disk before the integrity check runs. Comfortably larger than
// any real menubar app bundle.
const maxDownloadBytes = 500 << 20 // 512 MiB

// downloadTimeout bounds a single update download.
const downloadTimeout = 5 * time.Minute

// updateCandidate is a source-agnostic description of an available update: the
// two update sources (GitHub Releases, custom appcast) each resolve to one of
// these, and the download/verify/install path is shared.
type updateCandidate struct {
	version  string // the offered version, for the prompt and the newer-check
	name     string // download filename
	url      string // zip download URL
	sha256   string // expected hex SHA-256 of the zip; "" for the GitHub path
	fromFeed bool   // true for the custom appcast path (enables version binding)
}

type release struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// appcast is the custom-feed manifest (AutoUpdate.FeedURL).
type appcast struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

func (a *Application) checkForUpdates() {
	checkForRestart()
	ticker := time.NewTicker(24 * time.Hour)
	for ; true; <-ticker.C {
		candidate := a.checkForUpdate()
		if candidate == nil {
			continue
		}
		button := a.Alert(Alert{
			MessageText:     fmt.Sprintf("New version of %s available", a.Name),
			InformativeText: fmt.Sprintf("Looks like %s of %s is now available- you're running %s", candidate.version, a.Name, a.AutoUpdate.Version),
			Buttons:         []string{"Update now", "Remind me later"},
		})
		if button.Button == 0 {
			if err := a.installUpdate(candidate); err != nil {
				log.Printf("Unable to update app: %v", err)
			}
		}
	}
}

// checkForUpdate resolves the configured source to an available update, or nil
// if none / on error. FeedURL wins when set.
func (a *Application) checkForUpdate() *updateCandidate {
	if a.AutoUpdate.Version == "" {
		log.Printf("Not checking updates for dev version")
		return nil
	}
	if a.AutoUpdate.FeedURL != "" {
		return checkFeed(a.AutoUpdate.FeedURL, a.AutoUpdate.Version)
	}
	release := checkForNewRelease(a.AutoUpdate.Repo, a.AutoUpdate.Version, a.AutoUpdate.AllowPrerelease)
	if release == nil {
		return nil
	}
	name, url := downloadURL(release)
	if url == "" {
		log.Printf("No .zip asset on release %s", release.TagName)
		return nil
	}
	return &updateCandidate{version: release.TagName, name: name, url: url}
}

func checkForRestart() {
	restarting := false
	for _, arg := range os.Args {
		if arg == "-restarting" {
			restarting = true
			break
		}
	}
	if !restarting {
		return
	}
	ppid := syscall.Getppid()
	log.Printf("%d: Detected restart, killing ppid %d", os.Getpid(), ppid)
	syscall.Kill(ppid, syscall.SIGTERM)
}

// checkFeed fetches and parses the custom appcast, returning a candidate only
// if it advertises a version strictly newer than currentVersion. It fails
// closed: any fetch/parse error, a malformed or missing sha256, or a
// not-newer version yields nil (no update).
func checkFeed(feedURL, currentVersion string) *updateCandidate {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		log.Printf("Error building appcast request: %v", err)
		return nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error fetching appcast: %v", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Appcast returned status %d", resp.StatusCode)
		return nil
	}
	var cast appcast
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&cast); err != nil {
		log.Printf("Error parsing appcast: %v", err)
		return nil
	}
	if cast.URL == "" || cast.SHA256 == "" {
		// A feed without a checksum is a feed with no integrity gate — refuse
		// it rather than skip verification (unlike some daemons that treat a
		// placeholder as "not ready yet", the menubar fails closed).
		log.Printf("Appcast missing url or sha256; not updating")
		return nil
	}
	if !versionNewer(currentVersion, cast.Version) {
		return nil
	}
	return &updateCandidate{
		version:  cast.Version,
		name:     filepath.Base(cast.URL),
		url:      cast.URL,
		sha256:   cast.SHA256,
		fromFeed: true,
	}
}

func checkForNewRelease(githubProject, currentVersion string, allowPrerelease bool) *release {
	releases, err := getReleasesFromGitHub(githubProject)
	if err != nil {
		log.Printf("Error fetching github releases: %v", err)
		return nil
	}
	return getReleaseToUpdateTo(releases, currentVersion, allowPrerelease)
}

// installUpdate downloads, verifies, and swaps in a candidate. Every check
// runs BEFORE the running app is touched: SHA-256 (feed only), then unzip,
// then — for a feed — the bundle's own version must be newer than what's
// running, then the codesign team pin (when VerifyTeamID is set). Only after
// all of that does replaceExecutableAndRestart move anything.
func (a *Application) installUpdate(c *updateCandidate) error {
	dir, err := os.MkdirTemp("", "menuetupdater")
	if err != nil {
		return fmt.Errorf("couldn't get tempdir: %v", err)
	}
	defer os.RemoveAll(dir)

	newAppPath, err := a.prepareUpdate(c, dir)
	if err != nil {
		return err
	}
	return replaceExecutableAndRestart(newAppPath)
}

// prepareUpdate performs everything up to (but not including) the swap: it
// downloads into dir, runs all integrity and authenticity checks, and returns
// the path to the verified .app. It is separated from installUpdate so the
// full verify chain is testable without renaming/relaunching a real bundle.
func (a *Application) prepareUpdate(c *updateCandidate, dir string) (string, error) {
	log.Printf("Downloading archive...")
	archivefile, err := downloadArchive(dir, c.name, c.url)
	if err != nil {
		return "", err
	}
	if c.sha256 != "" {
		if err := verifySHA256(archivefile, c.sha256); err != nil {
			return "", err
		}
	}
	log.Printf("Unzipping bundle...")
	newAppPath, err := unzipBundle(archivefile)
	if err != nil {
		return "", err
	}
	if newAppPath == "" {
		return "", fmt.Errorf("update archive has no top-level .app bundle")
	}
	if c.fromFeed {
		// The manifest's version claim decided whether to prompt, but the
		// bundle's OWN version is the real gate: a compromised feed could
		// advertise a high version while pointing at an old, still-validly-
		// signed (and possibly vulnerable) build. Require the actual bundle to
		// be newer than what's running.
		bundleVersion, err := bundleVersionFn(newAppPath)
		if err != nil {
			return "", fmt.Errorf("couldn't read updated bundle version: %w", err)
		}
		if !versionNewer(a.AutoUpdate.Version, bundleVersion) {
			return "", fmt.Errorf("refusing update: bundle version %q is not newer than %q", bundleVersion, a.AutoUpdate.Version)
		}
	}
	if a.AutoUpdate.VerifyTeamID != "" {
		if err := verifyCodesignFn(newAppPath, a.AutoUpdate.VerifyTeamID); err != nil {
			return "", fmt.Errorf("update failed codesign verification: %w", err)
		}
	}
	return newAppPath, nil
}

func replaceExecutableAndRestart(newAppPath string) error {
	currentExecutable, currentAppPath := appPath()
	if currentExecutable == "" {
		log.Fatalf("Cannot update app, can't find executable")
	}
	if currentAppPath == "" {
		log.Fatalf("Cannot update app, not running in Mac app bundle (%s doesn't have /Contents/MacOS)", currentExecutable)
	}
	backupAppPath := currentAppPath + ".updating"
	log.Printf("Updating app (%s to %s)", currentAppPath, newAppPath)
	log.Printf("Move %s to %s", currentAppPath, backupAppPath)
	err := os.Rename(currentAppPath, backupAppPath)
	if err != nil {
		return err
	}
	log.Printf("Move %s to %s", newAppPath, currentAppPath)
	err = os.Rename(newAppPath, currentAppPath)
	if err != nil {
		err := os.Rename(backupAppPath, currentAppPath)
		if err != nil {
			return fmt.Errorf("os.Rename roll back: %v", err)
		}
		return fmt.Errorf("os.Rename move (rollled back): %v", err)
	}
	log.Printf("Removing")
	err = os.RemoveAll(backupAppPath)
	if err != nil {
		return err
	}
	log.Printf("Executing")
	cmd := exec.Command(currentExecutable, "-restarting")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func appPath() (string, string) {
	currentPath, err := os.Executable()
	if err != nil {
		log.Fatalf("os.Executable: %v", err)
	}
	return currentPath, bundlePathForExecutable(currentPath)
}

// bundlePathForExecutable returns the .app bundle path containing the given
// executable, or "" if the executable is not inside one. A bundle layout is
// recognized when the executable lives at .../<Something>.app/Contents/MacOS/<binary>.
func bundlePathForExecutable(execPath string) string {
	d := strings.Split(execPath, string(os.PathSeparator))
	if len(d) < 5 || d[len(d)-2] != "MacOS" || d[len(d)-3] != "Contents" {
		return ""
	}
	return strings.Join(d[0:len(d)-3], string(os.PathSeparator))
}

func getReleasesFromGitHub(project string) ([]release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", project)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	releases := make([]release, 0)
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&releases)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("Could not check for updates: no releases found")
	}
	return releases, nil
}

func downloadURL(release *release) (string, string) {
	name := ""
	url := ""
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".zip") {
			name = asset.Name
			url = asset.DownloadURL
			break
		}
	}
	return name, url
}

func downloadArchive(tempdir, name, url string) (string, error) {
	filename := filepath.Join(tempdir, name)
	out, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't create file in tempdir: %v", err)
	}
	defer out.Close()
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't open url: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Not updating, download returned status %d", resp.StatusCode)
	}
	// Cap the copy so a hostile origin can't fill the disk. LimitReader stops
	// at the cap; a file exactly at the cap is indistinguishable from a
	// truncated one, so treat hitting it as an error.
	n, err := io.Copy(out, io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't copy data: %v", err)
	}
	if n > maxDownloadBytes {
		return "", fmt.Errorf("Not updating, download exceeds %d bytes", maxDownloadBytes)
	}
	return filename, nil
}

// verifySHA256 fails closed unless the file's SHA-256 matches expected (hex).
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", got, expected)
	}
	return nil
}

// unzipBundle extracts filename next to itself and returns the path to the
// top-level .app bundle, or "" if there isn't one.
//
// It rejects two classes of malicious entry BEFORE anything is verified,
// because extraction happens before the SHA/codesign checks and the archive
// is attacker-controlled on the custom-feed path:
//   - Zip-Slip: an entry whose path escapes the destination directory.
//   - Symlinks: a symlink entry could let a later codesign-verified bundle
//     resolve to different code at exec time, or point a write outside the tree.
func unzipBundle(filename string) (string, error) {
	destination := filepath.Dir(filename)
	bundle := ""
	r, err := zip.OpenReader(filename)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(destination, f.Name)
		// Containment check: the cleaned path must stay inside destination.
		if fpath != destination && !strings.HasPrefix(fpath, destination+string(os.PathSeparator)) {
			return "", fmt.Errorf("refusing update: archive entry %q escapes the destination", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing update: archive contains a symlink (%q)", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(fpath, os.ModePerm); err != nil {
				return "", err
			}
			if strings.HasSuffix(f.Name, ".app/") && !strings.Contains(filepath.Dir(f.Name), "/") {
				bundle = fpath
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			rc.Close()
			return "", err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return "", err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return "", err
		}
	}
	return bundle, nil
}

func getReleaseToUpdateTo(releases []release, currentVersion string, allowPrerelease bool) *release {
	if !allowPrerelease {
		filtered := releases[:0:0]
		for _, r := range releases {
			if !r.Prerelease {
				filtered = append(filtered, r)
			}
		}
		releases = filtered
	}
	if len(releases) == 0 {
		log.Printf("No github releases found")
		return nil
	}
	found := false
	for ind, release := range releases {
		if release.TagName == currentVersion {
			if ind == 0 {
				log.Printf("Not updating, latest version already running")
				return nil
			}
			found = true
			break
		}
	}
	if !found {
		log.Printf("Our version isn't on the page, not updating")
		return nil
	}
	return &releases[0]
}

// versionNewer reports whether offered is strictly newer than current. Both are
// compared as dotted numeric versions (a leading "v" is ignored). It fails
// closed: if either can't be parsed as a dotted-numeric version, it returns
// false (do not update) rather than guess.
func versionNewer(current, offered string) bool {
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	o, ok := parseVersion(offered)
	if !ok {
		return false
	}
	for i := 0; i < len(c) || i < len(o); i++ {
		var cv, ov int
		if i < len(c) {
			cv = c[i]
		}
		if i < len(o) {
			ov = o[i]
		}
		if ov != cv {
			return ov > cv
		}
	}
	return false // equal
}

// parseVersion splits a dotted-numeric version ("v1.2.3" -> [1 2 3]). ok is
// false if it's empty or any component isn't a non-negative integer.
func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, false
		}
		out[i] = n
	}
	return out, true
}

// bundleVersionFn reads a .app's CFBundleShortVersionString. It's a package
// var so tests can supply a fake without a real bundle. verifyCodesignFn is
// likewise injectable so the verify chain is testable without a signed app.
var bundleVersionFn = bundleShortVersion
var verifyCodesignFn = verifyCodesignTeam

// bundleShortVersion reads CFBundleShortVersionString from the bundle's
// Info.plist via plutil (always present on macOS).
func bundleShortVersion(appPath string) (string, error) {
	plist := filepath.Join(appPath, "Contents", "Info.plist")
	out, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-", plist).Output()
	if err != nil {
		return "", fmt.Errorf("plutil read CFBundleShortVersionString: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// verifyCodesignTeam requires appPath to be validly Developer-ID signed by
// teamID. A bare `codesign --verify` passes on ANY signature (even ad-hoc), so
// it only proves the bundle wasn't corrupted after signing — not that we signed
// it. The designated requirement below pins Apple's anchor, the Developer ID
// Application leaf, and the team OU, so a validly-signed bundle from a different
// team is rejected. --deep verifies nested code in the bundle, not just the top
// executable.
func verifyCodesignTeam(appPath, teamID string) error {
	requirement := fmt.Sprintf(
		`=anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6]`+
			` and certificate leaf[field.1.2.840.113635.100.6.1.13]`+
			` and certificate leaf[subject.OU] = %q`, teamID)
	out, err := exec.Command("/usr/bin/codesign", "--verify", "--deep", "--strict",
		"-R", requirement, appPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign verify (team %q): %w: %s", teamID, err, strings.TrimSpace(string(out)))
	}
	return nil
}
