package menuet

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#include <stdlib.h>
#import "move.h"
*/
import "C"
import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// noMoveEnv turns the move-to-Applications offer off for a process, for
// development builds that are Developer ID signed.
const noMoveEnv = "MENUET_NO_MOVE"

// moveSuppressedKey is the user default that records "Do not ask again".
const moveSuppressedKey = "menuetMoveToApplicationsSuppressed"

// moveEnv holds everything the launch-time decision depends on, so the
// decision itself is a pure function.
type moveEnv struct {
	disabled     bool   // Application.DisableMoveToApplications
	envOptOut    bool   // MENUET_NO_MOVE is set
	restarting   bool   // the auto-updater relaunched this process
	bundlePath   string // the running .app, "" when not launched from a bundle
	originalPath string // bundlePath with App Translocation resolved, "" if unresolvable
	home         string // the user's home directory
	suppressed   bool   // the user chose "Do not ask again"
}

// Reasons moveSkipReason returns. reasonInApplications is the normal case
// and is not logged.
const (
	reasonDisabled       = "disabled by DisableMoveToApplications"
	reasonEnv            = "disabled by " + noMoveEnv
	reasonRestarting     = "relaunched by the auto-updater"
	reasonNoBundle       = "not launched from an .app bundle"
	reasonUnresolved     = "translocated, and the original location is unknown"
	reasonInApplications = "already in an Applications folder"
	reasonSuppressed     = "the user chose not to be asked again"
)

// moveSkipReason returns why launch should not offer to move the app, or ""
// when it should. A "" answer is still subject to the Developer ID check,
// which needs codesign and so runs only after everything here passes.
func moveSkipReason(e moveEnv) string {
	switch {
	case e.disabled:
		return reasonDisabled
	case e.envOptOut:
		return reasonEnv
	case e.restarting:
		return reasonRestarting
	case e.bundlePath == "":
		return reasonNoBundle
	case e.originalPath == "" || looksTranslocated(e.originalPath):
		return reasonUnresolved
	case inApplicationsFolder(e.originalPath, e.home):
		return reasonInApplications
	case e.suppressed:
		return reasonSuppressed
	}
	return ""
}

// looksTranslocated reports whether path is inside an App Translocation
// mirror. The Security framework is the authority; this check only guards
// against moving from a mirror when that lookup silently fails.
func looksTranslocated(path string) bool {
	return strings.Contains(path, "/AppTranslocation/")
}

// inApplicationsFolder reports whether path is inside /Applications or
// ~/Applications, at any depth. APFS is case-insensitive by default, so the
// comparison is too; a false "yes" only means no offer.
func inApplicationsFolder(path, home string) bool {
	dirs := []string{"/Applications"}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	path = filepath.Clean(path)
	for _, dir := range dirs {
		if len(path) > len(dir) && path[len(dir)] == '/' && strings.EqualFold(path[:len(dir)], dir) {
			return true
		}
	}
	return false
}

// moveDestinationDir picks /Applications when this user can write to it, and
// ~/Applications otherwise. It never asks for administrator rights.
func moveDestinationDir(home string, writable func(string) bool) (string, error) {
	if writable("/Applications") {
		return "/Applications", nil
	}
	if home == "" {
		return "", fmt.Errorf("/Applications is not writable and the home directory is unknown")
	}
	return filepath.Join(home, "Applications"), nil
}

// parseTeamIdentifier extracts the TeamIdentifier from `codesign -dv`
// output. ok is false for ad-hoc and unsigned code, which report
// "TeamIdentifier=not set" or no line at all.
func parseTeamIdentifier(out string) (team string, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TeamIdentifier=") {
			v := strings.TrimPrefix(line, "TeamIdentifier=")
			if v == "" || v == "not set" {
				return "", false
			}
			return v, true
		}
	}
	return "", false
}

// offerMoveToApplications runs on the main thread after the NSApplication
// exists and before its run loop starts. When the user accepts and the move
// succeeds, it relaunches the app from the new location and exits.
func (a *Application) offerMoveToApplications() {
	e := moveEnv{
		disabled:  a.DisableMoveToApplications,
		envOptOut: os.Getenv(noMoveEnv) != "",
	}
	for _, arg := range os.Args[1:] {
		if arg == "-restarting" {
			e.restarting = true
		}
	}
	if exe, err := os.Executable(); err == nil {
		e.bundlePath = bundlePathForExecutable(exe)
	}
	if e.bundlePath != "" {
		e.originalPath = originalBundlePath(e.bundlePath)
	}
	e.home, _ = os.UserHomeDir()
	if !e.disabled && !e.envOptOut {
		e.suppressed = Defaults().Boolean(moveSuppressedKey)
	}
	if reason := moveSkipReason(e); reason != "" {
		if reason != reasonInApplications && reason != reasonNoBundle {
			log.Printf("menuet: not offering to move to Applications: %s", reason)
		}
		return
	}

	bundleID, err := bundlePlistString(e.bundlePath, "CFBundleIdentifier")
	if err != nil {
		log.Printf("menuet: not offering to move to Applications: %v", err)
		return
	}
	team, err := developerIDTeam(e.bundlePath, bundleID)
	if err != nil {
		log.Printf("menuet: not offering to move to Applications: not Developer ID signed: %v", err)
		return
	}
	destDir, err := moveDestinationDir(e.home, dirWritable)
	if err != nil {
		log.Printf("menuet: not offering to move to Applications: %v", err)
		return
	}
	dest := filepath.Join(destDir, filepath.Base(e.originalPath))
	name := a.Name
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(e.originalPath), ".app")
	}

	button, suppressed := moveAlert("Move to Applications folder?",
		fmt.Sprintf("%s works best from the Applications folder. It can move itself to %s and relaunch from there.", name, destDir),
		"Move to Applications", "Do Not Move", true)
	if button != 0 {
		if suppressed {
			Defaults().SetBoolean(moveSuppressedKey, true)
		}
		return
	}
	if err := moveBundle(e.originalPath, dest, bundleID, team); err != nil {
		log.Printf("menuet: move to Applications failed: %v", err)
		moveAlert(fmt.Sprintf("Could not move %s", name),
			fmt.Sprintf("%v\n\n%s will keep running from its current location.", err, name), "OK", "", false)
		return
	}
	if err := trash(e.originalPath); err != nil {
		// A read-only disk image cannot be trashed. The copy in
		// Applications is still good, so relaunch anyway.
		log.Printf("menuet: moved to %s but could not trash %s: %v", dest, e.originalPath, err)
	}
	if err := relaunchAfterExit(dest); err != nil {
		log.Printf("menuet: moved to %s but could not relaunch: %v", dest, err)
		moveAlert(fmt.Sprintf("%s moved", name),
			fmt.Sprintf("%s is now in %s, but could not relaunch itself. Quit it and open it from there.", name, destDir), "OK", "", false)
		return
	}
	log.Printf("menuet: moved to %s; relaunching", dest)
	os.Exit(0)
}

// moveBundle copies src to dest and verifies the copy before it touches
// anything the user owns. A copy of the same app already at dest goes to the
// Trash; anything else at dest stops the move. If dest's copy is running, the
// move stops too, because quitting another process on the user's behalf could
// lose its unsaved state.
func moveBundle(src, dest, bundleID, team string) error {
	if _, err := checkDestination(dest, bundleID); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	// Stage the copy next to dest, so the final rename stays on one volume and
	// a failed check leaves nothing half-installed.
	staging, err := os.MkdirTemp(filepath.Dir(dest), ".menuet-move-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, filepath.Base(dest))
	// ditto keeps symlinks, extended attributes, and the stapled ticket, so
	// the code signature survives the copy.
	if out, err := exec.Command("/usr/bin/ditto", src, staged).CombinedOutput(); err != nil {
		return fmt.Errorf("copying the app failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// Finder clears quarantine when the user moves an app; a copy made any
	// other way keeps it, and macOS would translocate the copy again. The
	// user has already approved this code by running it.
	if out, err := exec.Command("/usr/bin/xattr", "-d", "-r", "com.apple.quarantine", staged).CombinedOutput(); err != nil {
		return fmt.Errorf("clearing quarantine failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// The same check the auto-updater applies before a swap.
	if err := verifyCodesignTeam(staged, team, bundleID); err != nil {
		return fmt.Errorf("the copied app failed its signature check, so nothing was moved: %v", err)
	}
	// The copy and the codesign check take seconds, so check dest again
	// right before replacing it.
	replacing, err := checkDestination(dest, bundleID)
	if err != nil {
		return err
	}
	if replacing {
		if err := trash(dest); err != nil {
			return fmt.Errorf("could not move the old copy at %s to the Trash: %v", dest, err)
		}
	}
	return os.Rename(staged, dest)
}

// checkDestination reports whether dest holds a copy of this app that the
// move may replace. It fails when that copy is running or when dest holds
// something else.
func checkDestination(dest, bundleID string) (replacing bool, err error) {
	if otherInstanceRunningAt(bundleID, dest) {
		return false, fmt.Errorf("A copy is already running from %s. Quit it, then open this copy again.", dest)
	}
	if _, err := os.Lstat(dest); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	id, err := bundlePlistString(dest, "CFBundleIdentifier")
	if err != nil || id != bundleID {
		return false, fmt.Errorf("%s already exists and is a different app.", dest)
	}
	return true, nil
}

// developerIDTeam returns the team that signed the bundle, and fails unless
// the bundle is validly Developer ID signed with bundleID. This is the rule
// that keeps `make run` and other ad-hoc builds from ever seeing the offer.
func developerIDTeam(bundle, bundleID string) (string, error) {
	out, err := exec.Command("/usr/bin/codesign", "-dv", bundle).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("codesign -dv: %v: %s", err, strings.TrimSpace(string(out)))
	}
	team, ok := parseTeamIdentifier(string(out))
	if !ok {
		return "", fmt.Errorf("no team identifier (ad-hoc or unsigned)")
	}
	if err := verifyCodesignTeam(bundle, team, bundleID); err != nil {
		return "", err
	}
	return team, nil
}

// relaunchAfterExit starts a detached shell that waits for this process to
// exit, then opens dest. Waiting first keeps two instances of the same menu
// bar app from running at once.
func relaunchAfterExit(dest string) error {
	const script = `while /bin/kill -0 "$1" 2>/dev/null; do /bin/sleep 0.1; done; exec /usr/bin/open "$2"`
	cmd := exec.Command("/bin/sh", "-c", script, "sh", strconv.Itoa(os.Getpid()), dest)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}

func dirWritable(dir string) bool {
	const wOK = 0x2
	return syscall.Access(dir, wOK) == nil
}

func originalBundlePath(bundle string) string {
	cpath := C.CString(bundle)
	defer C.free(unsafe.Pointer(cpath))
	var translocated C.bool
	coriginal := C.menuetOriginalBundlePath(cpath, &translocated)
	if coriginal == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(coriginal))
	return C.GoString(coriginal)
}

func moveAlert(message, info, first, second string, showSuppression bool) (int, bool) {
	cmessage, cinfo, cfirst := C.CString(message), C.CString(info), C.CString(first)
	defer C.free(unsafe.Pointer(cmessage))
	defer C.free(unsafe.Pointer(cinfo))
	defer C.free(unsafe.Pointer(cfirst))
	var csecond *C.char
	if second != "" {
		csecond = C.CString(second)
		defer C.free(unsafe.Pointer(csecond))
	}
	var suppressed C.bool
	button := C.menuetMoveAlert(cmessage, cinfo, cfirst, csecond, C.bool(showSuppression), &suppressed)
	return int(button), bool(suppressed)
}

func otherInstanceRunningAt(bundleID, path string) bool {
	cid, cpath := C.CString(bundleID), C.CString(path)
	defer C.free(unsafe.Pointer(cid))
	defer C.free(unsafe.Pointer(cpath))
	return bool(C.menuetOtherInstanceRunningAt(cid, cpath))
}

func trash(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	cerr := C.menuetTrash(cpath)
	if cerr == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(cerr))
	return fmt.Errorf("%s", C.GoString(cerr))
}
