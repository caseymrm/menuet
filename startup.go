package menuet

/*
int menuetSMAppServiceRegister(void);
int menuetSMAppServiceUnregister(void);
int menuetSMAppServiceStatus(void);
*/
import "C"

import (
	"bytes"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"text/template"
)

// smAppServiceRegister attempts the macOS 13+ Service Management path.
// Returns true on success. Returns false if the API is unavailable
// (older macOS) or if registration failed (e.g. the bundle isn't signed
// in a way SMAppService accepts) — caller should fall back to the
// LaunchAgent path.
func smAppServiceRegister() bool {
	return int(C.menuetSMAppServiceRegister()) == 0
}

func smAppServiceUnregister() bool {
	return int(C.menuetSMAppServiceUnregister()) == 0
}

// smAppServiceEnabled reports whether the SMAppService backend currently
// considers this app registered (enabled OR requires-approval — both
// mean "set up via SMAppService"). Returns false if the API is
// unavailable or the app isn't registered.
func smAppServiceEnabled() bool {
	return int(C.menuetSMAppServiceStatus()) == 1
}

func (a *Application) getStartupPath() string {
	if a.Label == "" {
		log.Println("Warning: no application Label set, cannot manage Start at Login")
		return ""
	}
	u, err := user.Current()
	if err != nil {
		log.Printf("user.Current: %v", err)
		return ""
	}
	return u.HomeDir + "/Library/LaunchAgents/" + a.Label + ".plist"
}

func (a *Application) runningAtStartup() bool {
	// SMAppService is the modern (macOS 13+) backend and what users
	// expect: the System Settings → Login Items panel reflects it. Check
	// it first; fall back to looking for our LaunchAgent plist.
	if smAppServiceEnabled() {
		return true
	}
	path := a.getStartupPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (a *Application) removeStartupItem() {
	// Remove from both backends so we don't leave stale entries behind
	// if the user previously used one and now the other (or both got
	// registered during a transitional period).
	smAppServiceUnregister()
	path := a.getStartupPath()
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("os.Remove: %v", err)
	}
}

var launchdOnce sync.Once
var launchdTemplate *template.Template

type launchdPlistData struct {
	Name       string
	Label      string
	Executable string
}

func renderLaunchdPlist(data launchdPlistData) (string, error) {
	launchdOnce.Do(func() {
		launchdTemplate = template.Must(template.New("launchdConfig").Parse(launchdString))
	})
	var buf bytes.Buffer
	if err := launchdTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (a *Application) addStartupItem() {
	// macOS 13+ Service Management. Shows the app under System Settings
	// → Login Items so the user can revoke it from the standard place.
	// Requires the bundle to be signed in a way SMAppService accepts
	// (Developer ID or App Store); ad-hoc-signed dev builds will fail
	// here and we fall through to the LaunchAgent path.
	if smAppServiceRegister() {
		return
	}

	// Legacy LaunchAgent plist. Works for unsigned/ad-hoc dev builds and
	// for macOS 12 and earlier (where SMAppService doesn't exist).
	path := a.getStartupPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		log.Printf("os.MkdirAll: %v", err)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		log.Printf("os.Executable: %v", err)
		return
	}
	rendered, err := renderLaunchdPlist(launchdPlistData{
		Name:       a.Name,
		Label:      a.Label,
		Executable: executable,
	})
	if err != nil {
		log.Printf("renderLaunchdPlist: %v", err)
		return
	}
	if err := os.WriteFile(path, []byte(rendered), 0644); err != nil {
		log.Printf("os.WriteFile: %v", err)
		return
	}
}

var launchdString = `
<?xml version='1.0' encoding='UTF-8'?>
 <!DOCTYPE plist PUBLIC \"-//Apple Computer//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\" >
 <plist version='1.0'>
   <dict>
     <key>Label</key><string>{{.Label}}</string>
     <key>Program</key><string>{{.Executable}}</string>
     <key>StandardOutPath</key><string>/tmp/{{.Label}}-out.log</string>
     <key>StandardErrorPath</key><string>/tmp/{{.Label}}-err.log</string>
     <key>RunAtLoad</key><true/>
   </dict>
</plist>
`
