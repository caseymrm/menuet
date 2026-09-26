// Package privateupdate enrolls a Mac with a private menuet update server and
// keeps the resulting device token in the login Keychain.
//
// A private app ships one signed, notarized build. The server gives each
// enrolled Mac its own device token, and the app sends that token as
// AutoUpdate.FeedToken so that only enrolled Macs can fetch the appcast and
// the download. Wiring:
//
//	tok, err := privateupdate.Token(bundleID) // "" when not enrolled
//	if err != nil {
//		log.Printf("reading update token: %v", err) // treat as not enrolled
//	}
//	app.AutoUpdate.FeedURL = "https://updates.menuet.app/v1/apps/myapp/appcast"
//	app.AutoUpdate.FeedToken = tok
//	app.AutoUpdate.VerifyTeamID = "ABCDE12345"
//
// When tok is "", show a menu item that calls PromptAndEnroll. A new token
// takes effect at the next launch.
//
// The package has no cgo of its own. It shells out to /usr/bin/security and
// /usr/sbin/scutil, and imports menuet only for the Alert in PromptAndEnroll.
package privateupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/caseymrm/menuet/v2"
)

// keychainService is the Keychain service name for every app's token. The
// bundle ID is the account, so apps don't collide.
const keychainService = "menuet-update-token"

// enrollTimeout bounds Enroll when the caller's context has no deadline.
const enrollTimeout = 30 * time.Second

// errSecItemNotFound is the exit status of `security find-generic-password`
// when no item matches.
const errSecItemNotFound = 44

// ErrInviteNotValid means the server refused the invite: it is unknown,
// expired, or used up. The server does not say which.
var ErrInviteNotValid = errors.New("invite not valid")

// ErrCanceled means the user dismissed the enrollment prompt.
var ErrCanceled = errors.New("enrollment canceled")

// tokenPattern matches invite and device tokens ("mi_<app>_<base64url>",
// "md_<app>_<base64url>"). bundleIDPattern matches a CFBundleIdentifier.
// Both values go into a `security -i` command line, which splits on
// whitespace, so anything else is refused rather than escaped.
var (
	tokenPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	bundleIDPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
)

// runSecurity runs /usr/bin/security with stdin and args. It returns stdout
// and the exit code; err is set only when the command could not run. It is a
// variable so tests never touch the real Keychain.
var runSecurity = func(stdin string, args ...string) (stdout []byte, code int, err error) {
	cmd := exec.Command("/usr/bin/security", args...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out, ee.ExitCode(), nil
	}
	return out, 0, err
}

// computerName returns the Mac's user-visible name ("Casey's MacBook"). It is
// a variable for tests.
var computerName = func() (string, error) {
	out, err := exec.Command("/usr/sbin/scutil", "--get", "ComputerName").Output()
	if err != nil {
		return "", fmt.Errorf("scutil --get ComputerName: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// loginKeychain returns the path of the user's login keychain. Reads and
// writes name it explicitly, so the token never lands in, or is read from,
// whatever other keychain is the default or first in the search list.
func loginKeychain() (string, error) {
	out, code, err := runSecurity("", "login-keychain")
	if err != nil {
		return "", fmt.Errorf("running security: %w", err)
	}
	if code != 0 {
		return "", fmt.Errorf("security login-keychain exited %d", code)
	}
	// The output is the path in double quotes, indented.
	path := strings.Trim(strings.TrimSpace(string(out)), `"`)
	if path == "" {
		return "", errors.New("no login keychain")
	}
	return path, nil
}

// enrollClient follows only same-origin redirects. The default client replays
// the request body, which holds the invite, on a 307 or 308 to any host.
var enrollClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		orig := via[0].URL
		if req.URL.Scheme != orig.Scheme || !strings.EqualFold(req.URL.Host, orig.Host) {
			return errors.New("refusing a redirect to another origin during enrollment")
		}
		return nil
	},
}

// Token returns the device token stored for bundleID in the login keychain.
// It returns "" and a nil error when this Mac is not enrolled, and an error
// only when the Keychain read itself fails.
func Token(bundleID string) (string, error) {
	if !bundleIDPattern.MatchString(bundleID) {
		return "", fmt.Errorf("invalid bundle identifier %q", bundleID)
	}
	keychain, err := loginKeychain()
	if err != nil {
		return "", err
	}
	out, code, err := runSecurity("", "find-generic-password", "-a", bundleID, "-s", keychainService, "-w", keychain)
	if err != nil {
		return "", fmt.Errorf("running security: %w", err)
	}
	switch code {
	case 0:
		return strings.TrimSpace(string(out)), nil
	case errSecItemNotFound:
		return "", nil
	default:
		return "", fmt.Errorf("security find-generic-password exited %d", code)
	}
}

// storeToken writes token for bundleID, replacing any earlier one. The command
// goes to `security -i` on stdin: `-w <token>` as an argument would show the
// token in every process listing while security runs.
func storeToken(bundleID, token string) error {
	if !bundleIDPattern.MatchString(bundleID) {
		return fmt.Errorf("invalid bundle identifier %q", bundleID)
	}
	if !tokenPattern.MatchString(token) {
		return errors.New("server returned a malformed device token")
	}
	keychain, err := loginKeychain()
	if err != nil {
		return err
	}
	// security -i honors double quotes, so a path with spaces works; a path
	// that could end the quote or the line is refused.
	if strings.ContainsAny(keychain, "\"\\\n\r") {
		return fmt.Errorf("unsupported login keychain path %q", keychain)
	}
	line := fmt.Sprintf("add-generic-password -U -a %s -s %s -w %s \"%s\"\n", bundleID, keychainService, token, keychain)
	_, code, err := runSecurity(line, "-i")
	if err != nil {
		return fmt.Errorf("running security: %w", err)
	}
	if code != 0 {
		// security's own output can echo the command, so it stays out of the
		// error.
		return fmt.Errorf("security add-generic-password exited %d", code)
	}
	return nil
}

// Enroll exchanges invite for a device token at <baseURL>/v1/enroll and
// stores the token in the login Keychain for bundleID. version is the running
// app's version; the server records it with this Mac's ComputerName. It
// returns ErrInviteNotValid when the server refuses the invite. Error text
// never contains the invite or the token.
func Enroll(ctx context.Context, baseURL, bundleID, invite, version string) error {
	if !bundleIDPattern.MatchString(bundleID) {
		return fmt.Errorf("invalid bundle identifier %q", bundleID)
	}
	invite = strings.TrimSpace(invite)
	if !tokenPattern.MatchString(invite) {
		return ErrInviteNotValid
	}
	endpoint, err := url.JoinPath(baseURL, "v1", "enroll")
	if err != nil {
		return fmt.Errorf("invalid update server URL: %w", err)
	}
	name, err := computerName()
	if err != nil {
		return err
	}
	body, err := json.Marshal(struct {
		Invite  string `json:"invite"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}{invite, name, version})
	if err != nil {
		return err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, enrollTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building enroll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := enrollClient.Do(req)
	if err != nil {
		return fmt.Errorf("contacting update server: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrInviteNotValid
	default:
		return fmt.Errorf("update server returned status %d", resp.StatusCode)
	}
	var out struct {
		DeviceToken string `json:"device_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return fmt.Errorf("parsing enroll response: %w", err)
	}
	return storeToken(bundleID, out.DeviceToken)
}

// PromptAndEnroll asks the user for an invite code, calls Enroll, and reports
// the result in a second alert. It blocks until the user dismisses the
// alerts, so call it from a goroutine such as a menu item's Clicked. It
// returns ErrCanceled when the user cancels the prompt.
func PromptAndEnroll(ctx context.Context, app *menuet.Application, baseURL, bundleID, version string) error {
	return promptAndEnroll(ctx, app.Alert, baseURL, bundleID, version)
}

func promptAndEnroll(ctx context.Context, alert func(menuet.Alert) menuet.AlertClicked, baseURL, bundleID, version string) error {
	clicked := alert(menuet.Alert{
		MessageText:     "Enroll for updates",
		InformativeText: "Paste the invite code you were sent.",
		Buttons:         []string{"Enroll", "Cancel"},
		Inputs:          []menuet.AlertInput{{Placeholder: "mi_…"}},
	})
	if clicked.Button != 0 || len(clicked.Inputs) == 0 {
		return ErrCanceled
	}
	err := Enroll(ctx, baseURL, bundleID, clicked.Inputs[0], version)
	switch {
	case err == nil:
		alert(menuet.Alert{
			MessageText:     "Enrolled for updates",
			InformativeText: "This Mac will check for updates the next time the app starts.",
		})
	case errors.Is(err, ErrInviteNotValid):
		alert(menuet.Alert{
			MessageText:     "Invite not valid",
			InformativeText: "Check the code, or ask for a new invite. Invites expire and have a limited number of uses.",
		})
	default:
		alert(menuet.Alert{
			MessageText:     "Enrollment failed",
			InformativeText: err.Error(),
		})
	}
	return err
}
