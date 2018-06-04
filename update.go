package menuet

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Release represents one release of the software, currently a GitHub release
type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var calledCheckForRestart bool

// CheckForRestart should be called on startup, it is responsible for killing the parent process during a restart
func CheckForRestart() {
	if calledCheckForRestart {
		log.Printf("calledCheckForRestart should only be called once")
		return
	}
	calledCheckForRestart = true
	restarting := false
	for _, arg := range os.Args {
		if arg == "-restarting" {
			restarting = true
			break
		}
	}
	if !restarting {
		log.Printf("%d: not a restart", os.Getpid())
		return
	}
	ppid := syscall.Getppid()
	log.Printf("%d: Detected restart, killing ppid %d", os.Getpid(), ppid)
	syscall.Kill(ppid, syscall.SIGTERM)
}

// CheckForNewRelease returns the newest release if this one is out of date
func CheckForNewRelease(githubProject, currentVersion string) *Release {
	if !calledCheckForRestart {
		log.Printf("Skipping CheckForNewRelease because CheckForRestart is not being called first")
		return nil
	}
	if currentVersion == "" {
		log.Printf("Not checking updates for dev version")
		return nil
	}
	releases, err := getReleasesFromGitHub(githubProject)
	if err != nil {
		log.Printf("Error fetching github releases: %v", err)
		return nil
	}
	return getReleaseToUpdateTo(releases, currentVersion)
}

// UpdateApp downloads and replaces the current app bundle with the new one
func UpdateApp(release *Release) error {
	if !calledCheckForRestart {
		return fmt.Errorf("Skipping UpdateApp because CheckForRestart is not being called first")
	}
	name, url := downloadURL(release)
	dir, err := ioutil.TempDir("", "menuetupdater")
	if err != nil {
		return fmt.Errorf("Not updating, couldn't get tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	archivefile, err := downloadArchive(dir, name, url)
	if err != nil {
		return err
	}
	newAppPath, err := unzipBundle(archivefile)
	if err != nil {
		return err
	}
	return replaceExecutableAndRestart(newAppPath)
}

func replaceExecutableAndRestart(newAppPath string) error {
	currentExecutable, currentAppPath := appPath()
	backupAppPath := currentAppPath + ".updating"
	log.Printf("Updating app (%s to %s)", currentAppPath, newAppPath)
	err := os.Rename(currentAppPath, backupAppPath)
	if err != nil {
		return err
	}
	err = os.Rename(newAppPath, currentAppPath)
	if err != nil {
		err := os.Rename(backupAppPath, currentAppPath)
		if err != nil {
			return fmt.Errorf("os.Rename roll back: %v", err)
		}
		return fmt.Errorf("os.Rename move (rollled back): %v", err)
	}
	err = os.RemoveAll(backupAppPath)
	if err != nil {
		return err
	}
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
	d := strings.Split(currentPath, string(os.PathSeparator))
	if len(d) < 5 || d[len(d)-2] != "MacOS" || d[len(d)-3] != "Contents" {
		log.Fatalf("Cannot update app, not running in Mac app bundle (%s doesn't have /Contents/MacOS)", currentPath)
	}
	return currentPath, strings.Join(d[0:len(d)-3], string(os.PathSeparator))
}

func getReleasesFromGitHub(project string) ([]Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", project)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	releases := make([]Release, 0)
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

func downloadURL(release *Release) (string, string) {
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
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't open url: %v", err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("Not updating, couldn't copy data: %v", err)
	}
	return filename, nil
}

func unzipBundle(filename string) (string, error) {
	destination := filepath.Dir(filename)
	bundle := ""
	r, err := zip.OpenReader(filename)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		fpath := filepath.Join(destination, f.Name)
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(fpath, os.ModePerm); err != nil {
				return "", err
			}
			if strings.HasSuffix(f.Name, ".app/") && !strings.Contains(filepath.Dir(f.Name), "/") {
				bundle = fpath
			}
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return "", err
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return "", err
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			if err != nil {
				return "", err
			}
		}
	}
	return bundle, nil
}

func getReleaseToUpdateTo(releases []Release, currentVersion string) *Release {
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
