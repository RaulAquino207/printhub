package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
)

// Authenticator checks credentials and sessions. *auth.Service satisfies it.
type Authenticator interface {
	Login(ctx context.Context, username, password string) (string, auth.User, error)
	Authenticate(ctx context.Context, token string) (auth.User, error)
	Logout(ctx context.Context, token string) error
}

const (
	sessionCookie  = "printhub_session"
	maxLoginBytes  = 4 << 10
	csrfHeaderName = "X-Requested-With"
)

// userKey is the context key for the authenticated user. An unexported type
// guarantees no other package can collide with it.
type userKey struct{}

// userFrom returns the user stored in ctx by requireUser.
func userFrom(ctx context.Context) (auth.User, bool) {
	u, ok := ctx.Value(userKey{}).(auth.User)
	return u, ok
}

// requireUser rejects requests without a valid session cookie and stores the
// user in the request context for the handlers.
func (s *server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if c, err := r.Cookie(sessionCookie); err == nil {
			token = c.Value
		}

		user, err := s.auth.Authenticate(r.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in to continue")
			return
		}
		if err != nil {
			s.logger.Error("authenticate", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not check the session")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

// checkCSRF protects state-changing requests (ADR 0003). Together with the
// SameSite=Lax cookie:
//   - Origin must be present and allowed, so a page on another site cannot
//     send the request with the user's cookie;
//   - a custom header is required, which browsers only send cross-origin
//     after a CORS preflight that unknown origins fail.
func checkCSRF(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get(csrfHeaderName) == "" || !slices.Contains(allowedOrigins, r.Header.Get("Origin")) {
				writeError(w, http.StatusForbidden, "csrf_rejected",
					"state-changing requests need an allowed Origin and the "+csrfHeaderName+" header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	Username string `json:"username"`
}

type sessionResponse struct {
	User userResponse `json:"user"`
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expected a JSON body with username and password")
		return
	}

	token, user, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.logger.Warn("failed login", "username", req.Username)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if err != nil {
		s.logger.Error("login", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not sign in")
		return
	}

	http.SetCookie(w, s.sessionCookie(token, int(auth.SessionTTL/time.Second)))
	s.logger.Info("login", "username", user.Username)
	writeJSON(w, http.StatusOK, sessionResponse{User: userResponse{Username: user.Username}})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(r.Context(), c.Value); err != nil {
			s.logger.Error("logout", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "could not sign out")
			return
		}
	}
	// MaxAge < 0 tells the browser to delete the cookie now.
	http.SetCookie(w, s.sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	writeJSON(w, http.StatusOK, sessionResponse{User: userResponse{Username: user.Username}})
}

// sessionCookie builds the session cookie. No Domain attribute: the cookie
// stays host-only on the API's host, the narrowest scope (ADR 0003).
func (s *server) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true, // invisible to JavaScript, so XSS cannot steal it
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cookieSecure, // HTTPS only; off for http://localhost
	}
}
