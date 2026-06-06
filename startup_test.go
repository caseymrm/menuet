package menuet

import (
	"strings"
	"testing"
)

func TestRenderLaunchdPlistUsesLabelForLaunchdLabelKey(t *testing.T) {
	// Regression for the bug where the plist template used {{.Name}} for the
	// launchd <key>Label</key> value. Apps that left Name empty would emit a
	// plist with an empty Label and launchd would silently ignore the agent.
	rendered, err := renderLaunchdPlist(launchdPlistData{
		Name:       "Human Readable Name",
		Label:      "com.example.app",
		Executable: "/Applications/App.app/Contents/MacOS/app",
	})
	if err != nil {
		t.Fatalf("renderLaunchdPlist: %v", err)
	}
	if !strings.Contains(rendered, "<key>Label</key><string>com.example.app</string>") {
		t.Errorf("launchd Label should be the reverse-DNS identifier; rendered:\n%s", rendered)
	}
	if strings.Contains(rendered, "<string>Human Readable Name</string>") {
		t.Errorf("Name field should not leak into the plist; rendered:\n%s", rendered)
	}
}

func TestRenderLaunchdPlistWithEmptyNameStillProducesValidLabel(t *testing.T) {
	// cmd/weather and many menuet consumers only set Label, never Name.
	// Before the fix this case produced an empty <key>Label</key><string></string>.
	rendered, err := renderLaunchdPlist(launchdPlistData{
		Name:       "",
		Label:      "com.example.app",
		Executable: "/Applications/App.app/Contents/MacOS/app",
	})
	if err != nil {
		t.Fatalf("renderLaunchdPlist: %v", err)
	}
	if !strings.Contains(rendered, "<key>Label</key><string>com.example.app</string>") {
		t.Errorf("empty Name should not blank out the launchd Label; rendered:\n%s", rendered)
	}
	if strings.Contains(rendered, "<string></string>") {
		t.Errorf("plist should not contain an empty <string></string>; rendered:\n%s", rendered)
	}
}

func TestRenderLaunchdPlistContainsExecutable(t *testing.T) {
	exe := "/Applications/App.app/Contents/MacOS/app"
	rendered, err := renderLaunchdPlist(launchdPlistData{
		Label:      "com.example.app",
		Executable: exe,
	})
	if err != nil {
		t.Fatalf("renderLaunchdPlist: %v", err)
	}
	if !strings.Contains(rendered, "<key>Program</key><string>"+exe+"</string>") {
		t.Errorf("plist should embed the executable path; rendered:\n%s", rendered)
	}
}

func TestGetStartupPathWithoutLabelReturnsEmpty(t *testing.T) {
	// Regression for issue #7: previously this called log.Fatal and crashed
	// the host menubar app. The fix is to warn and return empty so callers
	// can no-op.
	a := &Application{}
	if got := a.getStartupPath(); got != "" {
		t.Errorf("getStartupPath() with no Label = %q, want empty string", got)
	}
}

func TestRunningAtStartupWithoutLabelDoesNotPanic(t *testing.T) {
	a := &Application{}
	if a.runningAtStartup() {
		t.Errorf("runningAtStartup() with no Label = true, want false")
	}
}

func TestRemoveStartupItemWithoutLabelIsNoOp(t *testing.T) {
	// Should not panic, should not attempt os.Remove("") which produces
	// confusing "remove : no such file or directory" log lines.
	a := &Application{}
	a.removeStartupItem()
}

func TestAddStartupItemWithoutLabelIsNoOp(t *testing.T) {
	// Previously this path crashed via log.Fatal in getStartupPath.
	a := &Application{}
	a.addStartupItem()
}
