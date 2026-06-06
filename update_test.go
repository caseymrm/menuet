package menuet

import "testing"

func TestGetReleaseToUpdateTo(t *testing.T) {
	stable := func(tag string) release { return release{TagName: tag} }
	pre := func(tag string) release { return release{TagName: tag, Prerelease: true} }
	stables := []release{stable("v1.2.0"), stable("v1.1.0"), stable("v1.0.0")}
	mixed := []release{
		stable("v2.0.0"),
		pre("v2.0.0-beta1"),
		stable("v1.0.0"),
	}

	tests := []struct {
		name            string
		releases        []release
		currentVersion  string
		allowPrerelease bool
		wantTag         string // "" means nil result
	}{
		{
			name:           "empty release list returns nil",
			releases:       nil,
			currentVersion: "v1.0.0",
			wantTag:        "",
		},
		{
			name:           "already on latest returns nil",
			releases:       stables,
			currentVersion: "v1.2.0",
			wantTag:        "",
		},
		{
			name:           "behind by one returns latest",
			releases:       stables,
			currentVersion: "v1.1.0",
			wantTag:        "v1.2.0",
		},
		{
			name:           "behind by two returns latest",
			releases:       stables,
			currentVersion: "v1.0.0",
			wantTag:        "v1.2.0",
		},
		{
			name:           "current version not in list returns nil",
			releases:       stables,
			currentVersion: "v0.9.0",
			wantTag:        "",
		},

		// Prerelease handling
		{
			name:            "stable channel skips prereleases when picking latest",
			releases:        mixed,
			currentVersion:  "v1.0.0",
			allowPrerelease: false,
			wantTag:         "v2.0.0",
		},
		{
			name:            "stable channel skips prereleases even when only prereleases exist",
			releases:        []release{pre("v1.0.0-beta1")},
			currentVersion:  "v0.9.0",
			allowPrerelease: false,
			wantTag:         "",
		},
		{
			name:            "user on a prerelease with stable channel does not downgrade",
			releases:        mixed,
			currentVersion:  "v2.0.0-beta1",
			allowPrerelease: false,
			wantTag:         "",
		},
		{
			name:            "beta channel offers a newer prerelease over the current stable",
			releases:        mixed,
			currentVersion:  "v1.0.0",
			allowPrerelease: true,
			wantTag:         "v2.0.0",
		},
		{
			name:            "beta channel updates a prerelease to the latest including prereleases",
			releases:        []release{pre("v2.0.0-beta2"), pre("v2.0.0-beta1")},
			currentVersion:  "v2.0.0-beta1",
			allowPrerelease: true,
			wantTag:         "v2.0.0-beta2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getReleaseToUpdateTo(tt.releases, tt.currentVersion, tt.allowPrerelease)
			if tt.wantTag == "" {
				if got != nil {
					t.Errorf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("got nil, want release with tag %q", tt.wantTag)
			}
			if got.TagName != tt.wantTag {
				t.Errorf("got tag %q, want %q", got.TagName, tt.wantTag)
			}
		})
	}
}

func TestDownloadURL(t *testing.T) {
	tests := []struct {
		name     string
		assets   []struct{ name, url string }
		wantName string
		wantURL  string
	}{
		{
			name:     "no assets returns empty",
			assets:   nil,
			wantName: "",
			wantURL:  "",
		},
		{
			name: "single zip asset is picked",
			assets: []struct{ name, url string }{
				{"App.zip", "https://example.com/App.zip"},
			},
			wantName: "App.zip",
			wantURL:  "https://example.com/App.zip",
		},
		{
			name: "first zip asset wins over later ones",
			assets: []struct{ name, url string }{
				{"App.zip", "https://example.com/App.zip"},
				{"App-debug.zip", "https://example.com/App-debug.zip"},
			},
			wantName: "App.zip",
			wantURL:  "https://example.com/App.zip",
		},
		{
			name: "non-zip assets are skipped",
			assets: []struct{ name, url string }{
				{"README.md", "https://example.com/README.md"},
				{"App.tar.gz", "https://example.com/App.tar.gz"},
				{"App.zip", "https://example.com/App.zip"},
			},
			wantName: "App.zip",
			wantURL:  "https://example.com/App.zip",
		},
		{
			name: "only non-zip assets returns empty",
			assets: []struct{ name, url string }{
				{"App.dmg", "https://example.com/App.dmg"},
			},
			wantName: "",
			wantURL:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rel := &release{}
			for _, a := range tt.assets {
				rel.Assets = append(rel.Assets, struct {
					Name        string `json:"name"`
					DownloadURL string `json:"browser_download_url"`
				}{Name: a.name, DownloadURL: a.url})
			}
			gotName, gotURL := downloadURL(rel)
			if gotName != tt.wantName || gotURL != tt.wantURL {
				t.Errorf("got (%q, %q), want (%q, %q)", gotName, gotURL, tt.wantName, tt.wantURL)
			}
		})
	}
}

func TestBundlePathForExecutable(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want string
	}{
		{
			name: "executable inside a .app bundle",
			exec: "/Applications/My App.app/Contents/MacOS/myapp",
			want: "/Applications/My App.app",
		},
		{
			name: "nested .app bundle",
			exec: "/Users/casey/Projects/foo/Foo.app/Contents/MacOS/foo",
			want: "/Users/casey/Projects/foo/Foo.app",
		},
		{
			name: "loose executable returns empty",
			exec: "/usr/local/bin/myapp",
			want: "",
		},
		{
			name: "executable in wrong subdirectory returns empty",
			exec: "/Applications/My App.app/Contents/Resources/myapp",
			want: "",
		},
		{
			name: "executable missing the .app/Contents/MacOS layer returns empty",
			exec: "/Applications/myapp",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bundlePathForExecutable(tt.exec); got != tt.want {
				t.Errorf("bundlePathForExecutable(%q) = %q, want %q", tt.exec, got, tt.want)
			}
		})
	}
}
