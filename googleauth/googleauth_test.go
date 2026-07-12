package googleauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
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

	// Injected exchange echoes the code into the Result so the test can assert
	// it flowed through (and that exchange ran at all).
	exchange := func(code string) (*Result, error) {
		return &Result{AccessToken: "exchanged:" + code}, nil
	}

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string // non-empty: expect a Result whose token carries this code
		wantErr    bool   // true: expect an error result delivered
	}{
		{"correct state and code", "/callback?state=expected-state-value&code=abc123", http.StatusOK, "abc123", false},
		{"oauth error redirect", "/callback?state=expected-state-value&error=access_denied&error_description=denied", http.StatusOK, "", true},
		{"wrong state", "/callback?state=nope&code=abc123", http.StatusBadRequest, "", false},
		{"missing state", "/callback?code=abc123", http.StatusBadRequest, "", false},
		{"missing code and error", "/callback?state=expected-state-value", http.StatusBadRequest, "", false},
		{"wrong path", "/somewhere-else?state=expected-state-value&code=abc123", http.StatusNotFound, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, resultCh := newCallbackHandler(state, exchange)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			select {
			case got := <-resultCh:
				switch {
				case tt.wantErr:
					if got.err == nil {
						t.Fatalf("expected error result, got %+v", got.result)
					}
				case tt.wantCode == "":
					t.Fatalf("unexpected result delivered: %+v", got)
				case got.result == nil || got.result.AccessToken != "exchanged:"+tt.wantCode:
					t.Fatalf("result = %+v, want token exchanged:%s", got.result, tt.wantCode)
				}
			default:
				if tt.wantCode != "" || tt.wantErr {
					t.Fatalf("expected a delivered result, none came")
				}
			}
		})
	}
}

func TestCallbackHandlerOneShot(t *testing.T) {
	const state = "one-shot-state"
	exchange := func(code string) (*Result, error) { return &Result{AccessToken: code}, nil }
	handler, resultCh := newCallbackHandler(state, exchange)

	// First valid hit runs the exchange and delivers the result.
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/callback?state=one-shot-state&code=first", nil))
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}
	if got := <-resultCh; got.result == nil || got.result.AccessToken != "first" {
		t.Fatalf("first result = %+v, want token first", got.result)
	}

	// Second valid request is not served (410 Gone) and delivers nothing.
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/callback?state=one-shot-state&code=second", nil))
	if rec2.Code != http.StatusGone {
		t.Fatalf("second request status = %d, want 410", rec2.Code)
	}
	select {
	case got := <-resultCh:
		t.Fatalf("second request should deliver nothing, got %+v", got)
	default:
	}
}

// TestCallbackHandlerConcurrent hammers the handler with simultaneous valid
// callbacks: exactly one must win, and no handler goroutine may block.
func TestCallbackHandlerConcurrent(t *testing.T) {
	const state = "concurrent-state"
	handler, resultCh := newCallbackHandler(state, func(code string) (*Result, error) {
		return &Result{AccessToken: code}, nil
	})

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
	if len(resultCh) != 1 {
		t.Fatalf("expected exactly one result delivered, got %d", len(resultCh))
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

	t.Run("200 with no access_token is an error", func(t *testing.T) {
		// A misbehaving endpoint that returns 200 with an empty body must not
		// be accepted as a successful sign-in.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{})
		}))
		defer srv.Close()

		cfg := Config{ClientID: wantClientID, TokenURL: srv.URL}
		_, err := exchangeCode(context.Background(), cfg, wantCode, wantVerifier, redirectURI)
		if err == nil {
			t.Fatal("expected error for 200 response with no access_token, got nil")
		}
	})

	t.Run("malformed id_token is an error", func(t *testing.T) {
		// A well-formed token response whose id_token can't be decoded is a
		// real problem — it must surface, not be silently dropped.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "a",
				"id_token":     "not-a-jwt",
				"expires_in":   3600,
			})
		}))
		defer srv.Close()

		cfg := Config{ClientID: wantClientID, TokenURL: srv.URL}
		_, err := exchangeCode(context.Background(), cfg, wantCode, wantVerifier, redirectURI)
		if err == nil {
			t.Fatal("expected error for malformed id_token, got nil")
		}
	})
}

func TestBuildAuthURL(t *testing.T) {
	cfg := Config{
		ClientID: "cid",
		Scopes:   []string{"openid", "email", "profile"},
	}
	raw := buildAuthURL(cfg, "http://127.0.0.1:1234/callback", "the-challenge", "the-state", "the-nonce")
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
		"nonce":                 "the-nonce",
		"prompt":                "select_account",
	}
	for k, v := range want {
		if got := q.Get(k); got != v {
			t.Errorf("query[%s] = %q, want %q", k, got, v)
		}
	}
	// The default is identity-only: no offline access is requested, so no
	// refresh token is issued and there is nothing long-lived to store.
	if _, ok := q["access_type"]; ok {
		t.Errorf("default must not request access_type=offline, got %q", q.Get("access_type"))
	}
}

func TestBuildAuthURLPromptOverride(t *testing.T) {
	cfg := Config{ClientID: "cid", Scopes: []string{"openid"}, Prompt: "consent"}
	raw := buildAuthURL(cfg, "http://127.0.0.1:1/callback", "c", "s", "n")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	if got := u.Query().Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want consent", got)
	}
}

func TestBuildAuthURLOffline(t *testing.T) {
	// Offline opt-in must set access_type=offline AND force prompt=consent, so
	// Google reliably returns a refresh token (select_account only issues one on
	// first consent). The two flags travel together — the caller can't land in
	// the incoherent offline+select_account middle.
	cfg := Config{ClientID: "cid", Scopes: []string{"openid"}, Offline: true}
	raw := buildAuthURL(cfg, "http://127.0.0.1:1/callback", "c", "s", "n")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	q := u.Query()
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
	if got := q.Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want consent when Offline", got)
	}
}

func TestBuildAuthURLOfflinePromptStillOverridable(t *testing.T) {
	// An explicit Prompt wins even with Offline, for callers who know what they
	// want (e.g. offline access without re-consent on every sign-in).
	cfg := Config{ClientID: "cid", Scopes: []string{"openid"}, Offline: true, Prompt: "select_account"}
	raw := buildAuthURL(cfg, "http://127.0.0.1:1/callback", "c", "s", "n")
	u, _ := url.Parse(raw)
	q := u.Query()
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
	if got := q.Get("prompt"); got != "select_account" {
		t.Errorf("prompt = %q, want select_account (explicit override)", got)
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
	var sentNonce string
	openURL := func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		sentNonce = q.Get("nonce")
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
	// A nonce must be generated, sent in the auth request, and surfaced so the
	// backend can bind the id_token to this sign-in.
	if sentNonce == "" {
		t.Error("no nonce sent in auth request")
	}
	if res.Nonce != sentNonce {
		t.Errorf("Result.Nonce = %q, want %q (the nonce sent to Google)", res.Nonce, sentNonce)
	}
}

// TestSignInExchangeFailureRendersFailurePage is the F5 regression: when the
// token exchange fails after a valid redirect, the browser must see the failure
// page, not an optimistic "You're signed in."
func TestSignInExchangeFailureRendersFailurePage(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "code expired",
		})
	}))
	defer tokenSrv.Close()

	pageCh := make(chan string, 1)
	openURL := func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		cb := q.Get("redirect_uri") + "?state=" + url.QueryEscape(q.Get("state")) + "&code=fake-auth-code"
		go func() {
			resp, err := http.Get(cb)
			if err != nil {
				pageCh <- "GET error: " + err.Error()
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			pageCh <- string(body)
		}()
		return nil
	}

	cfg := Config{ClientID: "cid", Scopes: []string{"openid"}, TokenURL: tokenSrv.URL, OpenURL: openURL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := SignIn(ctx, cfg); err == nil {
		t.Fatal("expected an error when the exchange fails")
	}

	select {
	case page := <-pageCh:
		if strings.Contains(page, "You're signed in") {
			t.Errorf("browser shown the success page despite a failed exchange:\n%s", page)
		}
		if !strings.Contains(page, "didn't complete") {
			t.Errorf("expected the failure page, got:\n%s", page)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("browser never received a page")
	}
}

func TestSignInRequiresScopes(t *testing.T) {
	_, err := SignIn(context.Background(), Config{
		ClientID: "cid",
		OpenURL:  func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected error when Scopes is empty")
	}
}

// TestSignInUserCancelled drives the flow but has the injected browser return
// Google's error redirect (as if the user clicked Deny). SignIn must return
// promptly with a typed *AuthError, not hang until the timeout.
func TestSignInUserCancelled(t *testing.T) {
	openURL := func(authURL string) error {
		u, err := url.Parse(authURL)
		if err != nil {
			return err
		}
		q := u.Query()
		cb := q.Get("redirect_uri") + "?state=" + url.QueryEscape(q.Get("state")) +
			"&error=access_denied&error_description=" + url.QueryEscape("user denied consent")
		go func() {
			resp, err := http.Get(cb)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}

	cfg := Config{ClientID: "cid", Scopes: []string{"openid"}, OpenURL: openURL}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := SignIn(ctx, cfg)
	if err == nil {
		t.Fatal("expected an error when the user cancels consent")
	}
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "access_denied" {
		t.Errorf("AuthError.Code = %q, want access_denied", authErr.Code)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("SignIn hung for %v after cancel; should return promptly", elapsed)
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
