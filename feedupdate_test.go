package menuet

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
		c, err := checkFeed(srv.URL, "0.1.7", "")
		if err != nil && c != nil {
			t.Errorf("checkFeed returned both a candidate and %v", err)
		}
		return c
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

	origVer, origSign, origID := bundleVersionFn, verifyCodesignFn, runningBundleIDFn
	defer func() { bundleVersionFn, verifyCodesignFn, runningBundleIDFn = origVer, origSign, origID }()
	runningBundleIDFn = func() (string, error) { return "com.example.app", nil }

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
		verifyCodesignFn = func(path, team, bundleID string) error {
			signCalled = true
			if team != "TEAM123456" {
				t.Errorf("team = %q", team)
			}
			if bundleID != "com.example.app" {
				t.Errorf("bundleID = %q, want the running app's", bundleID)
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
		verifyCodesignFn = func(string, string, string) error { t.Fatal("codesign should not run"); return nil }
		c := candidate()
		c.sha256 = "deadbeef"
		if _, err := app().prepareUpdate(c, t.TempDir()); err == nil {
			t.Fatal("expected sha mismatch error")
		}
	})

	t.Run("stale bundle version aborts before codesign", func(t *testing.T) {
		// Feed claims 0.2.0 but the actual bundle is old — the downgrade attack.
		bundleVersionFn = func(string) (string, error) { return "0.0.9", nil }
		verifyCodesignFn = func(string, string, string) error { t.Fatal("codesign should not run"); return nil }
		if _, err := app().prepareUpdate(candidate(), t.TempDir()); err == nil {
			t.Fatal("expected version-binding to reject a stale bundle")
		}
	})

	t.Run("codesign failure aborts", func(t *testing.T) {
		bundleVersionFn = func(string) (string, error) { return "0.2.0", nil }
		verifyCodesignFn = func(string, string, string) error { return errFakeSign }
		if _, err := app().prepareUpdate(candidate(), t.TempDir()); err == nil {
			t.Fatal("expected codesign failure to abort")
		}
	})

	t.Run("unreadable running bundle ID fails closed", func(t *testing.T) {
		bundleVersionFn = func(string) (string, error) { return "0.2.0", nil }
		runningBundleIDFn = func() (string, error) { return "", errFakeSign }
		defer func() { runningBundleIDFn = func() (string, error) { return "com.example.app", nil } }()
		verifyCodesignFn = func(string, string, string) error { t.Fatal("codesign should not run"); return nil }
		if _, err := app().prepareUpdate(candidate(), t.TempDir()); err == nil {
			t.Fatal("expected a missing bundle ID to abort")
		}
	})
}

type fakeSignError struct{}

func (*fakeSignError) Error() string { return "fake codesign failure" }

var errFakeSign = &fakeSignError{}

func TestVerifyCodesignRejectsUnsigned(t *testing.T) {
	// An unsigned directory must fail the real verifier — guards against a
	// refactor loosening it to a bare --verify that would pass ad-hoc.
	if err := verifyCodesignTeam(t.TempDir(), "AZGE7WP274", "com.example.app"); err == nil {
		t.Error("an unsigned directory must fail codesign verification")
	}
}

func TestDesignatedRequirementPinsBundleID(t *testing.T) {
	req, err := designatedRequirement("AZGE7WP274", "com.github.caseymrm.nightswatch")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`identifier "com.github.caseymrm.nightswatch"`,
		`leaf[subject.OU] = "AZGE7WP274"`,
		`anchor apple generic`,
	} {
		if !strings.Contains(req, want) {
			t.Errorf("requirement %q lacks %q", req, want)
		}
	}
	// The requirement must compile: csreq parses the same language codesign
	// -R does, so a syntax slip can't silently turn into "always fails".
	out := filepath.Join(t.TempDir(), "req.bin")
	if b, err := exec.Command("/usr/bin/csreq", "-r", req, "-b", out).CombinedOutput(); err != nil {
		t.Fatalf("csreq rejected the requirement: %v: %s", err, b)
	}
	for _, bad := range []string{"", `com.x" or anchor apple`, "com.x\nmore", "com x"} {
		if _, err := designatedRequirement("AZGE7WP274", bad); err == nil {
			t.Errorf("designatedRequirement accepted bundle ID %q", bad)
		}
	}
}

// captureLog redirects the standard logger for the test and returns the buffer.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

const testFeedToken = "md_app_s3cr3t-T0KEN"

func TestCheckFeedSendsBearer(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(appcast{Version: "0.2.0", URL: "http://" + r.Host + "/a.zip", SHA256: "abc"})
	}))
	defer srv.Close()

	c, err := checkFeed(srv.URL, "0.1.0", testFeedToken)
	if err != nil || c == nil {
		t.Fatalf("checkFeed = %v, %v", c, err)
	}
	if c.bearer != testFeedToken {
		t.Error("same-origin download must carry the feed token")
	}
	if _, err := checkFeed(srv.URL, "0.1.0", ""); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Bearer "+testFeedToken || got[1] != "" {
		t.Errorf("Authorization headers = %q, want [Bearer <token>, none]", got)
	}
}

func TestCheckFeedWithholdsBearerCrossOrigin(t *testing.T) {
	buf := captureLog(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(appcast{Version: "0.2.0", URL: "https://elsewhere.example/a.zip", SHA256: "abc"})
	}))
	defer srv.Close()
	c, err := checkFeed(srv.URL, "0.1.0", testFeedToken)
	if err != nil || c == nil {
		t.Fatalf("checkFeed = %v, %v", c, err)
	}
	if c.bearer != "" {
		t.Error("the feed token must not follow a download URL on another origin")
	}
	if strings.Contains(buf.String(), testFeedToken) {
		t.Error("the feed token leaked into the log")
	}
}

func TestDownloadArchiveSendsBearer(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Write([]byte("zip"))
	}))
	defer srv.Close()
	if _, err := downloadArchive(t.TempDir(), "a.zip", srv.URL, testFeedToken); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer "+testFeedToken {
		t.Errorf("Authorization = %q", got)
	}
}

// TestUpdateAuthFailedHook drives the feed and the download against servers
// that refuse the token, and checks the hook fires with the status and that
// neither the log nor the error text carries the token.
func TestUpdateAuthFailedHook(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			buf := captureLog(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			var fired []int
			a := &Application{}
			a.AutoUpdate.Version = "0.1.0"
			a.AutoUpdate.FeedURL = srv.URL
			a.AutoUpdate.FeedToken = testFeedToken
			a.AutoUpdate.OnUpdateAuthFailed = func(s int) { fired = append(fired, s) }

			if c := a.checkForUpdate(); c != nil {
				t.Fatalf("candidate = %+v, want none", c)
			}
			if len(fired) != 1 || fired[0] != status {
				t.Errorf("hook calls = %v, want [%d]", fired, status)
			}

			_, err := downloadArchive(t.TempDir(), "a.zip", srv.URL, testFeedToken)
			var se *httpStatusError
			if !errors.As(err, &se) || se.status != status {
				t.Errorf("download error = %v, want status %d", err, status)
			}
			if err != nil && strings.Contains(err.Error(), testFeedToken) {
				t.Error("the feed token leaked into an error")
			}
			if strings.Contains(buf.String(), testFeedToken) {
				t.Error("the feed token leaked into the log")
			}
		})
	}
}

func TestUpdateAuthFailedDefaultLogs(t *testing.T) {
	buf := captureLog(t)
	(&Application{}).updateAuthFailed(http.StatusForbidden)
	if !strings.Contains(buf.String(), "403") {
		t.Errorf("log = %q, want the status", buf.String())
	}
}

func TestCheckFeedNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if c, err := checkFeed(srv.URL, "0.1.0", ""); c != nil || err != nil {
		t.Errorf("204 = %v, %v; want no candidate and no error", c, err)
	}
}

func TestCheckFeedSendsVersion(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-App-Version")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if _, err := checkFeed(srv.URL, "0.1.7", testFeedToken); err != nil {
		t.Fatal(err)
	}
	if got != "0.1.7" {
		t.Errorf("X-App-Version = %q, want 0.1.7", got)
	}
}

// TestCredentialedRedirects checks that a request carrying the feed token
// follows a redirect only to the exact same origin.
func TestCredentialedRedirects(t *testing.T) {
	check := func(from, to string) error {
		orig, _ := http.NewRequest(http.MethodGet, from, nil)
		next, _ := http.NewRequest(http.MethodGet, to, nil)
		return credentialedClient.CheckRedirect(next, []*http.Request{orig})
	}
	allowed := [][2]string{
		{"https://updates.example/v1/a", "https://updates.example/v1/b"},
		{"https://updates.example/a", "https://UPDATES.example/b"},
	}
	for _, c := range allowed {
		if err := check(c[0], c[1]); err != nil {
			t.Errorf("%s -> %s refused: %v", c[0], c[1], err)
		}
	}
	refused := [][2]string{
		{"https://updates.example/a", "https://evil.updates.example/a"}, // subdomain
		{"https://updates.example/a", "https://updates.example:8443/a"}, // other port
		{"https://updates.example/a", "http://updates.example/a"},       // downgrade
		{"https://updates.example/a", "https://elsewhere.example/a"},
	}
	for _, c := range refused {
		if err := check(c[0], c[1]); err == nil {
			t.Errorf("%s -> %s allowed", c[0], c[1])
		}
	}
}

// TestFeedTokenNotRedirectedToOtherPort drives the real client: the feed and
// the download both redirect to a server on another port, which must never
// see the token.
func TestFeedTokenNotRedirectedToOtherPort(t *testing.T) {
	var leaked []string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = append(leaked, r.Header.Get("Authorization"))
		w.Write([]byte("zip"))
	}))
	defer other.Close()
	redirector := httptest.NewServer(http.RedirectHandler(other.URL+"/x", http.StatusFound))
	defer redirector.Close()

	if _, err := checkFeed(redirector.URL, "0.1.0", testFeedToken); err == nil {
		t.Error("feed: expected the cross-origin redirect to fail")
	} else if strings.Contains(err.Error(), testFeedToken) {
		t.Error("feed: error leaks the token")
	}
	if _, err := downloadArchive(t.TempDir(), "a.zip", redirector.URL, testFeedToken); err == nil {
		t.Error("download: expected the cross-origin redirect to fail")
	}
	if len(leaked) != 0 {
		t.Errorf("the other origin received %d requests", len(leaked))
	}

	// Without a token (the GitHub path), cross-origin redirects still work:
	// GitHub release assets redirect to another host.
	if _, err := downloadArchive(t.TempDir(), "a.zip", redirector.URL, ""); err != nil {
		t.Errorf("tokenless download must follow the redirect: %v", err)
	}
}
