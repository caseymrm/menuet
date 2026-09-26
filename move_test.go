package menuet

import "testing"

func TestInApplicationsFolder(t *testing.T) {
	const home = "/Users/me"
	tests := []struct {
		path string
		want bool
	}{
		{"/Applications/Foo.app", true},
		{"/Applications/Utilities/Foo.app", true},
		{"/applications/Foo.app", true},
		{"/Users/me/Applications/Foo.app", true},
		{"/Users/me/Applications/Tools/Foo.app", true},
		{"/Applications", false},
		{"/ApplicationsOld/Foo.app", false},
		{"/Users/me/Downloads/Foo.app", false},
		{"/Users/other/Applications/Foo.app", false},
		{"/Volumes/Foo/Foo.app", false},
		{"/Volumes/Foo/Applications/Foo.app", false},
		{"/Users/me/Downloads/../../../Applications/Foo.app", true},
	}
	for _, tt := range tests {
		if got := inApplicationsFolder(tt.path, home); got != tt.want {
			t.Errorf("inApplicationsFolder(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
	if inApplicationsFolder("Applications/Foo.app", "") {
		t.Errorf("an empty home must not make a relative Applications path count")
	}
}

func TestLooksTranslocated(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/private/var/folders/xy/abc/T/AppTranslocation/1A2B-3C4D/d/Foo.app", true},
		{"/Users/me/Downloads/Foo.app", false},
		{"/Applications/AppTranslocationHelper.app", false},
	}
	for _, tt := range tests {
		if got := looksTranslocated(tt.path); got != tt.want {
			t.Errorf("looksTranslocated(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMoveSkipReason(t *testing.T) {
	const (
		home        = "/Users/me"
		downloads   = "/Users/me/Downloads/Foo.app"
		mirror      = "/private/var/folders/xy/abc/T/AppTranslocation/1A2B/d/Foo.app"
		inApps      = "/Applications/Foo.app"
		inUserApps  = "/Users/me/Applications/Foo.app"
		fromDMG     = "/Volumes/Foo/Foo.app"
		notResolved = ""
	)
	offer := func(e moveEnv) moveEnv { e.home = home; return e }
	tests := []struct {
		name string
		env  moveEnv
		want string
	}{
		{"downloads", offer(moveEnv{bundlePath: downloads, originalPath: downloads}), ""},
		{"disk image", offer(moveEnv{bundlePath: fromDMG, originalPath: fromDMG}), ""},
		{"translocated from downloads", offer(moveEnv{bundlePath: mirror, originalPath: downloads}), ""},
		{"translocated from Applications", offer(moveEnv{bundlePath: mirror, originalPath: inApps}), reasonInApplications},
		{"translocation unresolved", offer(moveEnv{bundlePath: mirror, originalPath: notResolved}), reasonUnresolved},
		{"resolver returned the mirror", offer(moveEnv{bundlePath: mirror, originalPath: mirror}), reasonUnresolved},
		{"in /Applications", offer(moveEnv{bundlePath: inApps, originalPath: inApps}), reasonInApplications},
		{"in ~/Applications", offer(moveEnv{bundlePath: inUserApps, originalPath: inUserApps}), reasonInApplications},
		{"go run binary", offer(moveEnv{}), reasonNoBundle},
		{"disabled by app", offer(moveEnv{disabled: true, bundlePath: downloads, originalPath: downloads}), reasonDisabled},
		{"disabled by env", offer(moveEnv{envOptOut: true, bundlePath: downloads, originalPath: downloads}), reasonEnv},
		{"updater relaunch", offer(moveEnv{restarting: true, bundlePath: downloads, originalPath: downloads}), reasonRestarting},
		{"do not ask again", offer(moveEnv{suppressed: true, bundlePath: downloads, originalPath: downloads}), reasonSuppressed},
		{"do not ask again, already moved", offer(moveEnv{suppressed: true, bundlePath: inApps, originalPath: inApps}), reasonInApplications},
	}
	for _, tt := range tests {
		if got := moveSkipReason(tt.env); got != tt.want {
			t.Errorf("%s: moveSkipReason = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMoveDestinationDir(t *testing.T) {
	tests := []struct {
		name     string
		home     string
		writable bool
		want     string
		wantErr  bool
	}{
		{"admin user", "/Users/me", true, "/Applications", false},
		{"standard user", "/Users/me", false, "/Users/me/Applications", false},
		{"no home", "", false, "", true},
	}
	for _, tt := range tests {
		got, err := moveDestinationDir(tt.home, func(string) bool { return tt.writable })
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: moveDestinationDir = %q, %v; want %q, err=%v", tt.name, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestParseTeamIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		want   string
		wantOK bool
	}{
		{"developer id", "Executable=/x\nIdentifier=com.example.foo\nFormat=app bundle with Mach-O thin (arm64)\nCodeDirectory v=20500 size=1 flags=0x10000(runtime)\nSignature size=9000\nTeamIdentifier=AZGE7WP274\nSealed Resources version=2\n", "AZGE7WP274", true},
		{"ad-hoc", "Executable=/x\nIdentifier=helloworld\nSignature=adhoc\nTeamIdentifier=not set\n", "", false},
		{"linker-signed", "Executable=/x\nIdentifier=a.out\nCodeDirectory v=20400 flags=0x20002(adhoc,linker-signed)\nSignature=adhoc\n", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		got, ok := parseTeamIdentifier(tt.out)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("%s: parseTeamIdentifier = %q, %v; want %q, %v", tt.name, got, ok, tt.want, tt.wantOK)
		}
	}
}
