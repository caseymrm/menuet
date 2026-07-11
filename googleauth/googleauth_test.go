package googleauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateCodeVerifier(t *testing.T) {
	for i := 0; i < 100; i++ {
		v, err := generateCodeVerifier()
		if err != nil {
			t.Fatalf("generateCodeVerifier: %v", err)
		}
		// RFC 7636 requires 43–128 characters.
		if len(v) < 43 || len(v) > 128 {
			t.Fatalf("verifier length %d out of range [43,128]", len(v))
		}
		// base64.RawURLEncoding must round-trip (unreserved chars only).
		if _, err := base64.RawURLEncoding.DecodeString(v); err != nil {
			t.Fatalf("verifier not valid base64url: %v", err)
		}
	}
}

func TestCodeChallenge(t *testing.T) {
	tests := []string{
		"dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
		"short-verifier-value",
		generateOrFatal(t),
	}
	for _, verifier := range tests {
		sum := sha256.Sum256([]byte(verifier))
		want := base64.RawURLEncoding.EncodeToString(sum[:])
		if got := codeChallenge(verifier); got != want {
			t.Fatalf("codeChallenge(%q) = %q, want %q", verifier, got, want)
		}
		// No padding must be present.
		if strings.Contains(codeChallenge(verifier), "=") {
			t.Fatalf("codeChallenge contains padding")
		}
	}
}

func generateOrFatal(t *testing.T) string {
	t.Helper()
	v, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("generateCodeVerifier: %v", err)
	}
	return v
}

func TestRandomState(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		s, err := randomState()
		if err != nil {
			t.Fatalf("randomState: %v", err)
		}
		if len(s) < 32 {
			t.Fatalf("state too short: %d chars", len(s))
		}
		if seen[s] {
			t.Fatalf("duplicate state generated: %q", s)
		}
		seen[s] = true
	}
}

func TestCallbackHandler(t *testing.T) {
	const state = "expected-state-value"

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string // non-empty means we expect delivery on the channel
	}{
		{"correct state and code", "/callback?state=expected-state-value&code=abc123", http.StatusOK, "abc123"},
		{"wrong state", "/callback?state=nope&code=abc123", http.StatusBadRequest, ""},
		{"missing state", "/callback?code=abc123", http.StatusBadRequest, ""},
		{"missing code", "/callback?state=expected-state-value", http.StatusBadRequest, ""},
		{"wrong path", "/somewhere-else?state=expected-state-value&code=abc123", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, codeCh := newCallbackHandler(state)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			select {
			case got := <-codeCh:
				if tt.wantCode == "" {
					t.Fatalf("unexpected code delivered: %q", got)
				}
				if got != tt.wantCode {
					t.Fatalf("code = %q, want %q", got, tt.wantCode)
				}
			default:
				if tt.wantCode != "" {
					t.Fatalf("expected code %q, none delivered", tt.wantCode)
				}
			}
		})
	}
}

func TestCallbackHandlerOneShot(t *testing.T) {
	const state = "one-shot-state"
	handler, codeCh := newCallbackHandler(state)

	// First valid hit captures the code.
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/callback?state=one-shot-state&code=first", nil))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	if got := <-codeCh; got != "first" {
		t.Fatalf("first code = %q, want %q", got, "first")
	}

	// Second valid request is not served (410 Gone) and delivers nothing.
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/callback?state=one-shot-state&code=second", nil))
	if rec2.Code != http.StatusGone {
		t.Fatalf("second request status = %d, want 410", rec2.Code)
	}
	select {
	case got := <-codeCh:
		t.Fatalf("second request should deliver nothing, got %q", got)
	default:
	}
}

// TestCallbackHandlerConcurrent hammers the handler with simultaneous valid
// callbacks: exactly one must win, and no handler goroutine may block.
func TestCallbackHandlerConcurrent(t *testing.T) {
	const state = "concurrent-state"
	handler, codeCh := newCallbackHandler(state)

	const n = 20
	var wg sync.WaitGroup
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
				"/callback?state=concurrent-state&code=code-"+strconv.Itoa(i), nil))
			statuses[i] = rec.Code
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler goroutines blocked; one-shot claim is not atomic")
	}

	okCount := 0
	for _, s := range statuses {
		switch s {
		case http.StatusOK:
			okCount++
		case http.StatusGone:
		default:
			t.Fatalf("unexpected status %d", s)
		}
	}
	if okCount != 1 {
		t.Fatalf("expected exactly one 200, got %d", okCount)
	}
	if len(codeCh) != 1 {
		t.Fatalf("expected exactly one code delivered, got %d", len(codeCh))
	}
}

// makeIDToken builds an unsigned JWT with the given email claim.
func makeIDToken(t *testing.T, email string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payloadJSON, err := json.Marshal(map[string]interface{}{
		"email": email,
		"sub":   "1234567890",
		"iss":   "https://accounts.google.com",
	})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".signature-not-verified"
}

func TestEmailFromIDToken(t *testing.T) {
	tok := makeIDToken(t, "alice@example.com")
	got, err := emailFromIDToken(tok)
	if err != nil {
		t.Fatalf("emailFromIDToken: %v", err)
	}
	if got != "alice@example.com" {
		t.Fatalf("email = %q, want %q", got, "alice@example.com")
	}

	if _, err := emailFromIDToken("not.a.valid.jwt.token"); err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
	if _, err := emailFromIDToken("onlyonesegment"); err == nil {
		t.Fatal("expected error for single-segment token, got nil")
	}
}

func TestExchangeCode(t *testing.T) {
	const (
		wantCode     = "auth-code-xyz"
		wantVerifier = "the-code-verifier"
		wantClientID = "client-123.apps.googleusercontent.com"
		wantSecret   = "secret-abc"
		redirectURI  = "http://127.0.0.1:5000/callback"
	)
	idToken := makeIDToken(t, "bob@example.com")

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Errorf("ParseForm: %v", err)
			}
			checks := map[string]string{
				"code":          wantCode,
				"code_verifier": wantVerifier,
				"client_id":     wantClientID,
				"client_secret": wantSecret,
				"redirect_uri":  redirectURI,
				"grant_type":    "authorization_code",
			}
			for k, want := range checks {
				if got := r.PostForm.Get(k); got != want {
					t.Errorf("form[%s] = %q, want %q", k, got, want)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id_token":      idToken,
				"access_token":  "access-tok",
				"refresh_token": "refresh-tok",
				"expires_in":    3600,
			})
		}))
		defer srv.Close()

		cfg := Config{ClientID: wantClientID, ClientSecret: wantSecret, TokenURL: srv.URL}
		res, err := exchangeCode(context.Background(), cfg, wantCode, wantVerifier, redirectURI)
		if err != nil {
			t.Fatalf("exchangeCode: %v", err)
		}
		if res.AccessToken != "access-tok" {
			t.Errorf("AccessToken = %q", res.AccessToken)
		}
		if res.RefreshToken != "refresh-tok" {
			t.Errorf("RefreshToken = %q", res.RefreshToken)
		}
		if res.IDToken != idToken {
			t.Errorf("IDToken mismatch")
		}
		if res.Email != "bob@example.com" {
			t.Errorf("Email = %q, want bob@example.com", res.Email)
		}
		if res.Expiry.IsZero() {
			t.Errorf("Expiry not set")
		}
		if d := time.Until(res.Expiry); d < 55*time.Minute || d > 65*time.Minute {
			t.Errorf("Expiry ~1h off: %v", d)
		}
	})

	t.Run("no client secret omits it", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			if _, present := r.PostForm["client_secret"]; present {
				t.Errorf("client_secret should be absent when unset")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id_token": idToken, "access_token": "a", "expires_in": 10})
		}))
		defer srv.Close()

		cfg := Config{ClientID: wantClientID, TokenURL: srv.URL}
		if _, err := exchangeCode(context.Background(), cfg, wantCode, wantVerifier, redirectURI); err != nil {
			t.Fatalf("exchangeCode: %v", err)
		}
	})

	t.Run("non-200 is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "invalid_grant",
				"error_description": "code expired",
			})
		}))
		defer srv.Close()

		cfg := Config{ClientID: wantClientID, TokenURL: srv.URL}
		_, err := exchangeCode(context.Background(), cfg, wantCode, wantVerifier, redirectURI)
		if err == nil {
			t.Fatal("expected error for non-200 response, got nil")
		}
		if !strings.Contains(err.Error(), "invalid_grant") {
			t.Errorf("error should mention the Google error code: %v", err)
		}
	})
}

func TestBuildAuthURL(t *testing.T) {
	cfg := Config{
		ClientID: "cid",
		Scopes:   []string{"openid", "email", "profile"},
	}
	raw := buildAuthURL(cfg, "http://127.0.0.1:1234/callback", "the-challenge", "the-state")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	if got, want := u.Scheme+"://"+u.Host+u.Path, defaultAuthURL; got != want {
		t.Fatalf("auth endpoint = %q, want %q", got, want)
	}
	q := u.Query()
	want := map[string]string{
		"client_id":             "cid",
		"redirect_uri":          "http://127.0.0.1:1234/callback",
		"response_type":         "code",
		"scope":                 "openid email profile",
		"code_challenge":        "the-challenge",
		"code_challenge_method": "S256",
		"state":                 "the-state",
		"access_type":           "offline",
		"prompt":                "select_account",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("query[%s] = %q, want %q", k, got, v)
		}
	}
}

// TestSignInHappyPath drives the whole flow with OpenURL injected to act as the
// browser (it hits the local callback), and TokenURL pointed at an httptest
// server. No real browser, no network.
func TestSignInHappyPath(t *testing.T) {
	idToken := makeIDToken(t, "carol@example.com")

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id_token":      idToken,
			"access_token":  "access-tok",
			"refresh_token": "refresh-tok",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()

	// The injected "browser" parses the auth URL, extracts redirect_uri + state,
	// and calls the callback exactly as Google would after consent.
	openURL := func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		cb := q.Get("redirect_uri") + "?state=" + url.QueryEscape(q.Get("state")) + "&code=fake-auth-code"
		go func() {
			resp, err := http.Get(cb)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}

	cfg := Config{
		ClientID: "cid",
		Scopes:   []string{"openid", "email"},
		TokenURL: tokenSrv.URL,
		OpenURL:  openURL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := SignIn(ctx, cfg)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if res.Email != "carol@example.com" {
		t.Errorf("Email = %q, want carol@example.com", res.Email)
	}
	if res.AccessToken != "access-tok" || res.RefreshToken != "refresh-tok" {
		t.Errorf("tokens missing: %+v", res)
	}
}

func TestSignInRequiresClientID(t *testing.T) {
	_, err := SignIn(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error when ClientID is empty")
	}
}

func TestSignInContextCancelled(t *testing.T) {
	// OpenURL that never triggers a callback → SignIn must give up on ctx.
	cfg := Config{
		ClientID: "cid",
		OpenURL:  func(string) error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := SignIn(ctx, cfg)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("SignIn took too long to honor ctx: %v", time.Since(start))
	}
}
