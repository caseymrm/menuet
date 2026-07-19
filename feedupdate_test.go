package menuet

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		current, offered string
		want             bool
	}{
		{"0.1.7", "0.1.8", true},
		{"0.1.7", "0.2.0", true},
		{"0.1.7", "1.0.0", true},
		{"0.1.7", "0.1.7", false}, // equal is not newer
		{"0.1.7", "0.1.6", false}, // older
		{"0.1.7", "0.1.70", true}, // numeric, not lexical (70 > 7)
		{"v0.9", "v0.10", true},   // v-prefix stripped, 10 > 9
		{"1.2", "1.2.1", true},    // extra component
		{"1.2.1", "1.2", false},
		{"0.1.7", "", false},        // unparseable offered -> fail closed
		{"", "0.1.8", false},        // unparseable current -> fail closed
		{"0.1.7", "garbage", false}, // fail closed
		{"0.1.7", "1.x", false},     // partially numeric -> fail closed
	}
	for _, c := range cases {
		if got := versionNewer(c.current, c.offered); got != c.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", c.current, c.offered, got, c.want)
		}
	}
}

func TestCheckFeedNewerOnly(t *testing.T) {
	serve := func(cast appcast) *updateCandidate {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(cast)
		}))
		defer srv.Close()
		return checkFeed(srv.URL, "0.1.7")
	}

	if c := serve(appcast{Version: "0.1.8", URL: "https://x/a.zip", SHA256: "abc"}); c == nil {
		t.Error("expected a candidate for a newer version")
	} else if c.version != "0.1.8" || !c.fromFeed || c.sha256 != "abc" {
		t.Errorf("candidate = %+v", c)
	}
	if c := serve(appcast{Version: "0.1.7", URL: "https://x/a.zip", SHA256: "abc"}); c != nil {
		t.Error("must not offer the same version")
	}
	if c := serve(appcast{Version: "0.1.6", URL: "https://x/a.zip", SHA256: "abc"}); c != nil {
		t.Error("must not offer an older version (no downgrade)")
	}
	// Missing sha256 -> fail closed even though the version is newer.
	if c := serve(appcast{Version: "0.2.0", URL: "https://x/a.zip"}); c != nil {
		t.Error("must refuse an appcast with no sha256")
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")
	os.WriteFile(path, []byte("hello"), 0o600)
	sum := sha256.Sum256([]byte("hello"))
	good := hex.EncodeToString(sum[:])

	if err := verifySHA256(path, good); err != nil {
		t.Errorf("matching digest should verify: %v", err)
	}
	if err := verifySHA256(path, strings.ToUpper(good)); err != nil {
		t.Errorf("digest compare must be case-insensitive: %v", err)
	}
	if err := verifySHA256(path, "deadbeef"); err == nil {
		t.Error("mismatched digest must fail")
	}
}

// zipEntry is a file to put in a test zip. link marks a symlink entry.
type zipEntry struct {
	name string
	body string
	link bool
}

func makeZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "update.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name}
		switch {
		case strings.HasSuffix(e.name, "/"):
			hdr.SetMode(os.ModeDir | 0o755)
		case e.link:
			hdr.SetMode(os.ModeSymlink | 0o777)
		default:
			hdr.SetMode(0o644)
		}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(e.name, "/") {
			w.Write([]byte(e.body))
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUnzipBundleFindsTopLevelApp(t *testing.T) {
	path := makeZip(t, []zipEntry{
		{name: "My App.app/"},
		{name: "My App.app/Contents/"},
		{name: "My App.app/Contents/MacOS/"},
		{name: "My App.app/Contents/MacOS/myapp", body: "binary"},
	})
	bundle, err := unzipBundle(path)
	if err != nil {
		t.Fatalf("unzipBundle: %v", err)
	}
	if filepath.Base(bundle) != "My App.app" {
		t.Errorf("bundle = %q, want .../My App.app", bundle)
	}
}

func TestUnzipBundleRejectsZipSlip(t *testing.T) {
	// A classic Zip-Slip payload: escape the destination via ../.
	path := makeZip(t, []zipEntry{
		{name: "../../evil.txt", body: "pwned"},
	})
	if _, err := unzipBundle(path); err == nil {
		t.Fatal("unzipBundle must reject a path-traversal entry")
	} else if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("unexpected error: %v", err)
	}
	// And it must not have written the file outside the destination.
	escaped := filepath.Join(filepath.Dir(filepath.Dir(path)), "evil.txt")
	if _, err := os.Stat(escaped); err == nil {
		t.Error("zip-slip file was written outside the destination")
	}
}

func TestUnzipBundleRejectsSymlink(t *testing.T) {
	path := makeZip(t, []zipEntry{
		{name: "App.app/"},
		{name: "App.app/Contents/MacOS/binary", body: "/etc/passwd", link: true},
	})
	if _, err := unzipBundle(path); err == nil {
		t.Fatal("unzipBundle must reject a symlink entry")
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestPrepareUpdateVerifyChain drives download -> sha -> unzip -> version-bind
// -> codesign with the codesign and version readers mocked, so the ordering
// and fail-closed behavior are exercised without a real signed bundle.
func TestPrepareUpdateVerifyChain(t *testing.T) {
	zipPath := makeZip(t, []zipEntry{
		{name: "App.app/"},
		{name: "App.app/Contents/"},
		{name: "App.app/Contents/MacOS/"},
		{name: "App.app/Contents/MacOS/app", body: "v2 binary"},
	})
	zipBytes, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(zipBytes)
	goodSHA := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer srv.Close()

	origVer, origSign := bundleVersionFn, verifyCodesignFn
	defer func() { bundleVersionFn, verifyCodesignFn = origVer, origSign }()

	candidate := func() *updateCandidate {
		return &updateCandidate{version: "0.2.0", name: "App.zip", url: srv.URL, sha256: goodSHA, fromFeed: true}
	}
	app := func() *Application {
		a := &Application{}
		a.AutoUpdate.Version = "0.1.0"
		a.AutoUpdate.VerifyTeamID = "TEAM123456"
		return a
	}

	t.Run("happy path passes every gate", func(t *testing.T) {
		bundleVersionFn = func(string) (string, error) { return "0.2.0", nil }
		signCalled := false
		verifyCodesignFn = func(path, team string) error {
			signCalled = true
			if team != "TEAM123456" {
				t.Errorf("team = %q", team)
			}
			return nil
		}
		p, err := app().prepareUpdate(candidate(), t.TempDir())
		if err != nil {
			t.Fatalf("prepareUpdate: %v", err)
		}
		if filepath.Base(p) != "App.app" {
			t.Errorf("path = %q", p)
		}
		if !signCalled {
			t.Error("codesign verification was not run")
		}
	})

	t.Run("sha mismatch aborts before unzip", func(t *testing.T) {
		bundleVersionFn = func(string) (string, error) { return "0.2.0", nil }
		verifyCodesignFn = func(string, string) error { t.Fatal("codesign should not run"); return nil }
		c := candidate()
		c.sha256 = "deadbeef"
		if _, err := app().prepareUpdate(c, t.TempDir()); err == nil {
			t.Fatal("expected sha mismatch error")
		}
	})

	t.Run("stale bundle version aborts before codesign", func(t *testing.T) {
		// Feed claims 0.2.0 but the actual bundle is old — the downgrade attack.
		bundleVersionFn = func(string) (string, error) { return "0.0.9", nil }
		verifyCodesignFn = func(string, string) error { t.Fatal("codesign should not run"); return nil }
		if _, err := app().prepareUpdate(candidate(), t.TempDir()); err == nil {
			t.Fatal("expected version-binding to reject a stale bundle")
		}
	})

	t.Run("codesign failure aborts", func(t *testing.T) {
		bundleVersionFn = func(string) (string, error) { return "0.2.0", nil }
		verifyCodesignFn = func(string, string) error { return errFakeSign }
		if _, err := app().prepareUpdate(candidate(), t.TempDir()); err == nil {
			t.Fatal("expected codesign failure to abort")
		}
	})
}

type fakeSignError struct{}

func (*fakeSignError) Error() string { return "fake codesign failure" }

var errFakeSign = &fakeSignError{}

func TestVerifyCodesignRejectsUnsigned(t *testing.T) {
	// An unsigned directory must fail the real verifier — guards against a
	// refactor loosening it to a bare --verify that would pass ad-hoc.
	if err := verifyCodesignTeam(t.TempDir(), "AZGE7WP274"); err == nil {
		t.Error("an unsigned directory must fail codesign verification")
	}
}
