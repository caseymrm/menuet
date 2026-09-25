package privateupdate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caseymrm/menuet/v2"
)

const (
	testBundleID = "com.example.private"
	testInvite   = "mi_private_inv1te-CODE"
	testToken    = "md_private_dev1ce-TOKEN"
)

// fakeKeychain replaces runSecurity for one test. It models the two commands
// the package uses and records every argv and stdin, so tests can check the
// token never appears on a command line.
type fakeKeychain struct {
	items map[string]string // account -> token
	argvs [][]string
	fail  int // when non-zero, every command exits with this code
}

func useFakeKeychain(t *testing.T) *fakeKeychain {
	t.Helper()
	k := &fakeKeychain{items: map[string]string{}}
	orig := runSecurity
	runSecurity = k.run
	t.Cleanup(func() { runSecurity = orig })
	origName := computerName
	computerName = func() (string, error) { return "Test Mac", nil }
	t.Cleanup(func() { computerName = origName })
	return k
}

func flag(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func (k *fakeKeychain) run(stdin string, args ...string) ([]byte, int, error) {
	k.argvs = append(k.argvs, args)
	if k.fail != 0 {
		return nil, k.fail, nil
	}
	if len(args) == 1 && args[0] == "-i" {
		args = strings.Fields(stdin)
	}
	if flag(args, "-s") != keychainService {
		return nil, 2, nil
	}
	switch args[0] {
	case "find-generic-password":
		tok, ok := k.items[flag(args, "-a")]
		if !ok {
			return nil, errSecItemNotFound, nil
		}
		return []byte(tok + "\n"), 0, nil
	case "add-generic-password":
		k.items[flag(args, "-a")] = flag(args, "-w")
		return nil, 0, nil
	}
	return nil, 2, nil
}

func TestTokenAbsentVersusError(t *testing.T) {
	k := useFakeKeychain(t)
	if tok, err := Token(testBundleID); tok != "" || err != nil {
		t.Errorf("not enrolled: Token = %q, %v; want \"\", nil", tok, err)
	}
	k.items[testBundleID] = testToken
	if tok, err := Token(testBundleID); tok != testToken || err != nil {
		t.Errorf("enrolled: Token = %q, %v", tok, err)
	}
	k.fail = 51 // e.g. the keychain is locked or denied access
	if _, err := Token(testBundleID); err == nil {
		t.Error("a failed Keychain read must be an error, not \"not enrolled\"")
	}
}

func enrollServer(t *testing.T, status int, body string) (*httptest.Server, *map[string]string) {
	t.Helper()
	got := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/enroll" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestEnrollStoresToken(t *testing.T) {
	k := useFakeKeychain(t)
	srv, got := enrollServer(t, http.StatusOK, `{"device_token":"`+testToken+`"}`)

	if err := Enroll(context.Background(), srv.URL+"/", testBundleID, " "+testInvite+"\n", "0.1.0"); err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	want := map[string]string{"invite": testInvite, "name": "Test Mac", "version": "0.1.0"}
	for key, v := range want {
		if (*got)[key] != v {
			t.Errorf("request %s = %q, want %q", key, (*got)[key], v)
		}
	}
	if k.items[testBundleID] != testToken {
		t.Errorf("stored token = %q", k.items[testBundleID])
	}
	for _, argv := range k.argvs {
		if strings.Contains(strings.Join(argv, " "), testToken) {
			t.Errorf("token visible on the security command line: %q", argv)
		}
	}
}

func TestEnrollInviteNotValid(t *testing.T) {
	k := useFakeKeychain(t)
	k.items[testBundleID] = "md_private_old"
	srv, _ := enrollServer(t, http.StatusNotFound, `{"error":"`+testInvite+`"}`)

	err := Enroll(context.Background(), srv.URL, testBundleID, testInvite, "0.1.0")
	if !errors.Is(err, ErrInviteNotValid) {
		t.Fatalf("err = %v, want ErrInviteNotValid", err)
	}
	if k.items[testBundleID] != "md_private_old" {
		t.Error("a refused invite must leave the stored token unchanged")
	}
}

func TestEnrollFailuresHideSecrets(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		fail   int
	}{
		{"server error", http.StatusInternalServerError, `{"echo":"` + testInvite + `"}`, 0},
		{"malformed token", http.StatusOK, `{"device_token":"md bad\nadd-generic-password"}`, 0},
		{"empty token", http.StatusOK, `{}`, 0},
		{"keychain write fails", http.StatusOK, `{"device_token":"` + testToken + `"}`, 45},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := useFakeKeychain(t)
			k.fail = c.fail
			srv, _ := enrollServer(t, c.status, c.body)
			err := Enroll(context.Background(), srv.URL, testBundleID, testInvite, "0.1.0")
			if err == nil {
				t.Fatal("expected an error")
			}
			if strings.Contains(err.Error(), testInvite) || strings.Contains(err.Error(), testToken) {
				t.Errorf("error leaks a secret: %v", err)
			}
			if c.fail == 0 && len(k.items) != 0 {
				t.Error("nothing may be stored on failure")
			}
		})
	}
}

func TestEnrollRejectsBadInputBeforeNetwork(t *testing.T) {
	useFakeKeychain(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	defer srv.Close()
	if err := Enroll(context.Background(), srv.URL, testBundleID, "mi x -A", "0.1.0"); !errors.Is(err, ErrInviteNotValid) {
		t.Errorf("invite with spaces: err = %v", err)
	}
	if err := Enroll(context.Background(), srv.URL, "com.x -A", testInvite, "0.1.0"); err == nil {
		t.Error("bundle ID with spaces must be refused")
	}
}

func TestPromptAndEnroll(t *testing.T) {
	srv, _ := enrollServer(t, http.StatusOK, `{"device_token":"`+testToken+`"}`)
	notFound, _ := enrollServer(t, http.StatusNotFound, ``)

	// script answers the prompt with (button, input) and records the result alert.
	script := func(button int, input string, shown *[]string) func(menuet.Alert) menuet.AlertClicked {
		return func(a menuet.Alert) menuet.AlertClicked {
			*shown = append(*shown, a.MessageText)
			if len(a.Inputs) == 1 {
				return menuet.AlertClicked{Button: button, Inputs: []string{input}}
			}
			return menuet.AlertClicked{}
		}
	}

	t.Run("success", func(t *testing.T) {
		k := useFakeKeychain(t)
		var shown []string
		if err := promptAndEnroll(context.Background(), script(0, testInvite, &shown), srv.URL, testBundleID, "0.1.0"); err != nil {
			t.Fatal(err)
		}
		if k.items[testBundleID] != testToken || len(shown) != 2 || shown[1] != "Enrolled for updates" {
			t.Errorf("stored %q, alerts %q", k.items[testBundleID], shown)
		}
	})
	t.Run("invite not valid", func(t *testing.T) {
		useFakeKeychain(t)
		var shown []string
		err := promptAndEnroll(context.Background(), script(0, testInvite, &shown), notFound.URL, testBundleID, "0.1.0")
		if !errors.Is(err, ErrInviteNotValid) || len(shown) != 2 || shown[1] != "Invite not valid" {
			t.Errorf("err %v, alerts %q", err, shown)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		k := useFakeKeychain(t)
		var shown []string
		err := promptAndEnroll(context.Background(), script(1, testInvite, &shown), srv.URL, testBundleID, "0.1.0")
		if !errors.Is(err, ErrCanceled) || len(shown) != 1 || len(k.argvs) != 0 {
			t.Errorf("err %v, alerts %q, keychain calls %d", err, shown, len(k.argvs))
		}
	})
}
