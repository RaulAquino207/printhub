package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
)

const (
	testToken  = "test-token"
	testOrigin = "http://localhost:3000"
)

var testUser = auth.User{ID: 1, Username: "raul"}

// fakeAuth accepts one user ("raul" / "correct horse") and one token.
type fakeAuth struct{ loggedOut *string }

func (fakeAuth) Login(_ context.Context, username, password string) (string, auth.User, error) {
	if username == "raul" && password == "correct horse" {
		return testToken, testUser, nil
	}
	return "", auth.User{}, auth.ErrInvalidCredentials
}

func (fakeAuth) Authenticate(_ context.Context, token string) (auth.User, error) {
	if token == testToken {
		return testUser, nil
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (f fakeAuth) Logout(_ context.Context, token string) error {
	if f.loggedOut != nil {
		*f.loggedOut = token
	}
	return nil
}

// signedIn makes every request look like it comes from the web app with a
// valid session: session cookie, allowed Origin and the CSRF header.
func signedIn(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: testToken})
		if r.Header.Get("Origin") == "" {
			r.Header.Set("Origin", testOrigin)
		}
		r.Header.Set(csrfHeaderName, "PrintHub")
		h.ServeHTTP(w, r)
	})
}

func newAuthRouter(loggedOut *string) http.Handler {
	return NewRouter(Dependencies{
		Logger:      slog.New(slog.DiscardHandler),
		CORSOrigins: []string{testOrigin},
		Printers:    fakeDirectory{},
		Auth:        fakeAuth{loggedOut: loggedOut},
	})
}

type reqOpt func(*http.Request)

func withCookie(r *http.Request) { r.AddCookie(&http.Cookie{Name: sessionCookie, Value: testToken}) }
func withCSRF(r *http.Request) {
	r.Header.Set(csrfHeaderName, "PrintHub")
	r.Header.Set("Origin", testOrigin)
}
func withOrigin(o string) reqOpt    { return func(r *http.Request) { r.Header.Set("Origin", o) } }
func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

func do(h http.Handler, method, path, body string, opts ...reqOpt) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for _, o := range opts {
		o(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginSetsSessionCookie(t *testing.T) {
	h := newAuthRouter(nil)

	rec := do(h, http.MethodPost, "/api/v1/auth/login", `{"username":"raul","password":"correct horse"}`, withCSRF)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	c := cookies[0]
	if c.Name != sessionCookie || c.Value != testToken || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" || c.MaxAge <= 0 {
		t.Errorf("cookie = %+v; want HttpOnly, SameSite=Lax, Path=/, host-only, persistent", c)
	}
	var body sessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.User.Username != "raul" {
		t.Errorf("body = %+v, %v", body, err)
	}
}

func TestLoginFailures(t *testing.T) {
	h := newAuthRouter(nil)

	tests := []struct {
		name     string
		body     string
		opts     []reqOpt
		wantCode int
		wantErr  string
	}{
		{"wrong password", `{"username":"raul","password":"nope"}`, []reqOpt{withCSRF}, 401, "invalid_credentials"},
		{"malformed body", `{"username":`, []reqOpt{withCSRF}, 400, "invalid_request"},
		{"no CSRF header", `{"username":"raul","password":"correct horse"}`, []reqOpt{withOrigin(testOrigin)}, 403, "csrf_rejected"},
		{"foreign origin", `{"username":"raul","password":"correct horse"}`, []reqOpt{withOrigin("http://evil.example"), withHeader(csrfHeaderName, "x")}, 403, "csrf_rejected"},
		{"no origin", `{"username":"raul","password":"correct horse"}`, []reqOpt{withHeader(csrfHeaderName, "x")}, 403, "csrf_rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(h, http.MethodPost, "/api/v1/auth/login", tt.body, tt.opts...)
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantCode, rec.Body)
			}
			if code := decodeError(t, rec).Code; code != tt.wantErr {
				t.Errorf("code = %q, want %q", code, tt.wantErr)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Error("a cookie was set on a failed login")
			}
		})
	}
}

func TestProtectedRoutesNeedASession(t *testing.T) {
	h := newAuthRouter(nil)

	for _, path := range []string{"/api/v1/printers", "/api/v1/jobs", "/api/v1/auth/me"} {
		if rec := do(h, http.MethodGet, path, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without cookie = %d, want 401", path, rec.Code)
		}
	}
	// A valid cookie is not enough for a state-changing request without CSRF proof.
	if rec := do(h, http.MethodPost, "/api/v1/jobs/x/cancel", "", withCookie); rec.Code != http.StatusForbidden {
		t.Errorf("POST without CSRF header = %d, want 403", rec.Code)
	}
	// Health stays public, for monitoring.
	if rec := do(h, http.MethodGet, "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /healthz = %d, want 200", rec.Code)
	}

	rec := do(h, http.MethodGet, "/api/v1/auth/me", "", withCookie)
	var body sessionResponse
	if rec.Code != http.StatusOK || json.NewDecoder(rec.Body).Decode(&body) != nil || body.User.Username != "raul" {
		t.Errorf("GET /auth/me = %d %+v, want 200 raul", rec.Code, body)
	}
}

func TestLogoutClearsTheSession(t *testing.T) {
	var loggedOut string
	h := newAuthRouter(&loggedOut)

	rec := do(h, http.MethodPost, "/api/v1/auth/logout", "", withCookie, withCSRF)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body)
	}
	if loggedOut != testToken {
		t.Errorf("session %q ended, want %q", loggedOut, testToken)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || c[0].Name != sessionCookie || c[0].MaxAge >= 0 {
		t.Errorf("cookies = %+v, want the session cookie deleted", c)
	}
}
