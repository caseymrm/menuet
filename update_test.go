package menuet

import "testing"

func TestGetReleaseToUpdateTo(t *testing.T) {
	r := func(tag string) release { return release{TagName: tag} }
	releases := []release{r("v1.2.0"), r("v1.1.0"), r("v1.0.0")}

	tests := []struct {
		name           string
		releases       []release
		currentVersion string
		wantTag        string // "" means nil result
	}{
		{
			name:           "empty release list returns nil",
			releases:       nil,
			currentVersion: "v1.0.0",
			wantTag:        "",
		},
		{
			name:           "already on latest returns nil",
			releases:       releases,
			currentVersion: "v1.2.0",
			wantTag:        "",
		},
		{
			name:           "behind by one returns latest",
			releases:       releases,
			currentVersion: "v1.1.0",
			wantTag:        "v1.2.0",
		},
		{
			name:           "behind by two returns latest",
			releases:       releases,
			currentVersion: "v1.0.0",
			wantTag:        "v1.2.0",
		},
		{
			name:           "current version not in list returns nil (we don't downgrade strangers)",
			releases:       releases,
			currentVersion: "v0.9.0",
			wantTag:        "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getReleaseToUpdateTo(tt.releases, tt.currentVersion)
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
