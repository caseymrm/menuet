// Package googleauth implements a reusable "Sign in with Google" flow for
// native desktop apps using OAuth 2.0 with a loopback redirect and PKCE.
//
// It is the Apple/Google-recommended native-app flow: it opens the system
// browser to Google, catches the redirect on a transient 127.0.0.1:<random>
// HTTP server, exchanges the authorization code for tokens, and returns them.
// No embedded webview and no confidential client secret are required — PKCE is
// the real protection.
//
// The package depends only on the standard library. It imports nothing from the
// menuet root package, so any Go program can use it by supplying its own Google
// "Desktop app" OAuth client ID and scopes.
//
// Security note: any local application can initiate this flow using the
// (extractable) desktop client ID; the browser consent step is the only gate.
// This is inherent to native-app OAuth, not a flaw of this package.
package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	defaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL = "https://oauth2.googleapis.com/token"

	// defaultTimeout bounds the flow when the caller's context has no deadline.
	defaultTimeout = 3 * time.Minute

	// exchangeTimeout bounds the token exchange. It is deliberately separate
	// from the callback wait: once the browser redirect arrives, the exchange
	// is a fast machine-to-machine call that should not inherit whatever little
	// time is left on the (human-paced) consent deadline.
	exchangeTimeout = 30 * time.Second
)

// AuthError is returned by SignIn when Google redirects to the callback with an
// OAuth error rather than a code — most commonly the user declining consent
// (Code == "access_denied"). Callers can distinguish it with errors.As to, for
// example, treat a cancellation differently from a real failure.
type AuthError struct {
	Code        string // OAuth error code, e.g. "access_denied".
	Description string // Human-readable detail, may be empty.
}

func (e *AuthError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Description)
	}
	return e.Code
}

// Config configures a single sign-in attempt.
type Config struct {
	// ClientID is the Google "Desktop app" OAuth client ID. Required.
	ClientID string

	// ClientSecret is optional. Google desktop clients are issued one, but it
	// is not treated as confidential — PKCE is the real gate. Sent in the token
	// exchange when present.
	ClientSecret string

	// Scopes are the OAuth scopes to request, e.g. {"openid","email","profile"}.
	// At least one is required.
	Scopes []string

	// Prompt sets Google's prompt parameter. Defaults to "select_account".
	//
	// If your app needs a RefreshToken for offline access, set this to
	// "consent": Google issues a refresh token only on the first consent for an
	// account, so under the "select_account" default a repeat sign-in of an
	// already-consented account returns an empty RefreshToken. "consent"
	// re-shows the consent screen every time, so only use it when you actually
	// store and use the refresh token.
	Prompt string

	// AuthURL optionally overrides the Google authorization endpoint.
	AuthURL string

	// TokenURL optionally overrides the Google token endpoint.
	TokenURL string

	// OpenURL optionally overrides how the authorization URL is opened. The
	// default opens the system browser (open on macOS, xdg-open on Linux,
	// rundll32 on Windows). Injectable so tests need not launch a real browser.
	OpenURL func(string) error
}

// Result holds the tokens and identity returned by a successful sign-in.
type Result struct {
	IDToken      string // JWT — send to your backend to verify identity.
	AccessToken  string
	RefreshToken string
	Email        string
	Expiry       time.Time

	// Nonce is the OIDC nonce that was sent in the auth request. Your backend
	// must verify the id_token's "nonce" claim equals this value; otherwise an
	// unexpired id_token captured elsewhere could be replayed. Always set on a
	// successful sign-in (generation failure aborts the flow).
	Nonce string
}

func (c Config) authURL() string {
	if c.AuthURL != "" {
		return c.AuthURL
	}
	return defaultAuthURL
}

func (c Config) tokenURL() string {
	if c.TokenURL != "" {
		return c.TokenURL
	}
	return defaultTokenURL
}

func (c Config) openURL() func(string) error {
	if c.OpenURL != nil {
		return c.OpenURL
	}
	return openBrowser
}

func (c Config) prompt() string {
	if c.Prompt != "" {
		return c.Prompt
	}
	return "select_account"
}

// openBrowser opens u in the user's default browser using the platform's
// standard launcher.
func openBrowser(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Run()
	default:
		return exec.Command("xdg-open", u).Run()
	}
}

// SignIn runs the loopback + PKCE flow and returns the resulting tokens.
//
// It blocks until the browser redirect is received, the context is cancelled,
// or a timeout elapses (3 minutes when ctx has no deadline). Call it off any UI
// goroutine.
func SignIn(ctx context.Context, cfg Config) (*Result, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("googleauth: ClientID is required")
	}
	if len(cfg.Scopes) == 0 {
		return nil, errors.New("googleauth: at least one scope is required")
	}

	// Bound the flow if the caller supplied no deadline of their own.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
	}

	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("googleauth: generating code verifier: %w", err)
	}
	challenge := codeChallenge(verifier)

	state, err := randomState()
	if err != nil {
		return nil, fmt.Errorf("googleauth: generating state: %w", err)
	}

	// A random OIDC nonce, echoed by Google in the id_token, lets the backend
	// reject replayed tokens. randomState produces a suitable high-entropy value.
	nonce, err := randomState()
	if err != nil {
		return nil, fmt.Errorf("googleauth: generating nonce: %w", err)
	}

	// Bind explicitly to 127.0.0.1 (never 0.0.0.0) so the callback server is
	// only reachable from this machine.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("googleauth: opening loopback listener: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	handler, resultCh := newCallbackHandler(state)
	server := &http.Server{Handler: handler}
	go server.Serve(listener)
	// Shutdown (not Close) lets an in-flight callback finish writing its page to
	// the browser before the server tears down; Close would sever it mid-write.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	authURL := buildAuthURL(cfg, redirectURI, challenge, state, nonce)
	if err := cfg.openURL()(authURL); err != nil {
		return nil, fmt.Errorf("googleauth: opening browser: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("googleauth: waiting for callback: %w", ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, fmt.Errorf("googleauth: sign-in failed: %w", res.err)
		}
		// Give the exchange its own budget, decoupled from the consent wait.
		exchangeCtx, cancel := context.WithTimeout(context.Background(), exchangeTimeout)
		defer cancel()
		result, err := exchangeCode(exchangeCtx, cfg, res.code, verifier, redirectURI)
		if err != nil {
			return nil, err
		}
		result.Nonce = nonce
		return result, nil
	}
}

// buildAuthURL constructs the Google authorization URL.
func buildAuthURL(cfg Config, redirectURI, challenge, state, nonce string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(cfg.Scopes, " "))
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("access_type", "offline")
	q.Set("prompt", cfg.prompt())
	return cfg.authURL() + "?" + q.Encode()
}

// callbackResult is what a valid /callback delivers: either an authorization
// code, or an err when Google reported an OAuth error (e.g. the user declined).
type callbackResult struct {
	code string
	err  error
}

// newCallbackHandler returns a one-shot handler that serves only /callback with
// the expected state. The first valid request delivers a callbackResult on the
// returned channel; every other request (wrong path, wrong/missing state, or
// any request after the first valid one) gets an error status and delivers
// nothing. A callback carrying an OAuth error (rather than a code) is a valid,
// terminal outcome — it claims the one-shot and delivers an *AuthError.
func newCallbackHandler(state string) (http.Handler, <-chan callbackResult) {
	resultCh := make(chan callbackResult, 1)

	// http.Server may run handlers concurrently, so claiming the one-shot must be
	// atomic: without the mutex two simultaneous callbacks could both pass the
	// check and the second send would block a server goroutine forever.
	var mu sync.Mutex
	var delivered bool

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		oauthErr := q.Get("error")
		code := q.Get("code")
		if oauthErr == "" && code == "" {
			// Neither a code nor an error: a malformed callback. Don't burn the
			// one-shot on it — the real redirect may still be coming.
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}

		mu.Lock()
		alreadyDelivered := delivered
		delivered = true
		mu.Unlock()

		if alreadyDelivered {
			http.Error(w, "sign-in already completed", http.StatusGone)
			return
		}

		if oauthErr != "" {
			resultCh <- callbackResult{err: &AuthError{Code: oauthErr, Description: q.Get("error_description")}}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, failurePage)
			return
		}

		resultCh <- callbackResult{code: code}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, callbackPage)
	})

	return mux, resultCh
}

const callbackPage = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Signed in</title></head>
<body style="font-family:-apple-system,sans-serif;text-align:center;padding-top:4rem">
<h2>You're signed in.</h2>
<p>You can close this tab and return to the app.</p>
</body></html>`

const failurePage = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Sign-in failed</title></head>
<body style="font-family:-apple-system,sans-serif;text-align:center;padding-top:4rem">
<h2>Sign-in didn't complete.</h2>
<p>You can close this tab and return to the app to try again.</p>
</body></html>`

// tokenResponse mirrors the JSON returned by Google's token endpoint.
type tokenResponse struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// exchangeCode exchanges an authorization code for tokens at the token endpoint.
// It is separated from SignIn so it can be unit-tested against an httptest
// server without a browser.
func exchangeCode(ctx context.Context, cfg Config, code, verifier, redirectURI string) (*Result, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.tokenURL(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("googleauth: building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("googleauth: token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("googleauth: reading token response: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("googleauth: parsing token response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		if tr.Error != "" {
			return nil, fmt.Errorf("googleauth: token exchange failed (status %d): %s: %s", resp.StatusCode, tr.Error, tr.ErrorDesc)
		}
		return nil, fmt.Errorf("googleauth: token exchange failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// A 200 with no access_token is not a successful exchange — guard against a
	// misbehaving endpoint returning an empty or partial body as success.
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("googleauth: token response missing access_token (status %d)", resp.StatusCode)
	}

	result := &Result{
		IDToken:      tr.IDToken,
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
	}
	if tr.ExpiresIn > 0 {
		result.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	if tr.IDToken != "" {
		// Extract email from the ID token payload. The signature is NOT verified
		// here — that is the backend's responsibility. A decode failure means the
		// id_token is malformed, which is a real problem, so surface it rather
		// than returning a blank identity. (A well-formed token that simply lacks
		// an email claim yields "" with no error.)
		email, err := emailFromIDToken(tr.IDToken)
		if err != nil {
			return nil, fmt.Errorf("googleauth: extracting email from id_token: %w", err)
		}
		result.Email = email
	}

	return result, nil
}

// emailFromIDToken base64url-decodes the payload (middle) segment of a JWT and
// extracts the "email" claim. It does not verify the signature.
func emailFromIDToken(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("googleauth: malformed id_token: expected 3 segments, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("googleauth: decoding id_token payload: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("googleauth: parsing id_token claims: %w", err)
	}
	return claims.Email, nil
}

// generateCodeVerifier returns a high-entropy PKCE code verifier. RFC 7636
// permits 43–128 characters from the unreserved set; base64url encoding 64
// random bytes yields 86 characters, well within range.
func generateCodeVerifier() (string, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge computes the S256 PKCE challenge: base64url(sha256(verifier)),
// no padding.
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randomState returns a random opaque state value for CSRF protection.
func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
