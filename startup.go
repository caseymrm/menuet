package menuet

import (
	"bytes"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"text/template"
)

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
	path := a.getStartupPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func (a *Application) removeStartupItem() {
	path := a.getStartupPath()
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil {
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
