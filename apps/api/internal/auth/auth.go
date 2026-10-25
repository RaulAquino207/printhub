// Package auth manages PrintHub users and their sessions: password checks,
// session tokens and their lifetime.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials is returned for an unknown user or a wrong
	// password, without saying which, so usernames cannot be probed.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrUnauthenticated means the session token is missing, unknown or expired.
	ErrUnauthenticated = errors.New("not authenticated")
	// ErrUserExists is returned when registering a taken username.
	ErrUserExists = errors.New("username already taken")
	// ErrNotFound is returned by stores when a user or session does not exist.
	ErrNotFound = errors.New("not found")
)

// User is an account allowed to use PrintHub.
type User struct {
	ID       int64
	Username string
}

// Store persists users and sessions. *store.Store satisfies it.
type Store interface {
	CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (User, error)
	// UserByName returns ErrNotFound when no user has that name.
	UserByName(ctx context.Context, username string) (User, string, error)
	CreateSession(ctx context.Context, tokenHash string, userID int64, now, expires time.Time) error
	// SessionUser returns ErrNotFound when the session does not exist or has
	// expired at now.
	SessionUser(ctx context.Context, tokenHash string, now time.Time) (User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
}

const (
	// SessionTTL is how long a session lasts after login.
	SessionTTL = 30 * 24 * time.Hour
	// minPasswordBytes and maxPasswordBytes bound passwords; bcrypt ignores
	// anything after 72 bytes, so longer passwords are refused instead.
	minPasswordBytes = 8
	maxPasswordBytes = 72
)

var validUsername = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// dummyHash is compared against when a user does not exist, so a login takes
// the same time whether or not the username is valid.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("printhub-timing-equalizer"), bcrypt.DefaultCost)

// Service implements registration, login and session checks.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService returns a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Register creates a user. Usernames are 3–32 characters of a-z, 0-9, ".",
// "_" or "-"; passwords are 8–72 bytes.
func (s *Service) Register(ctx context.Context, username, password string) (User, error) {
	if !validUsername.MatchString(username) {
		return User{}, fmt.Errorf("username must be 3-32 characters of a-z, 0-9, '.', '_' or '-'")
	}
	if len(password) < minPasswordBytes || len(password) > maxPasswordBytes {
		return User{}, fmt.Errorf("password must be %d-%d bytes long", minPasswordBytes, maxPasswordBytes)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.store.CreateUser(ctx, username, string(hash), s.now())
}

// Login checks the credentials and starts a session. It returns the session
// token, which the caller hands to the client (in a cookie) and never stores.
func (s *Service) Login(ctx context.Context, username, password string) (string, User, error) {
	user, hash, err := s.store.UserByName(ctx, username)
	switch {
	case errors.Is(err, ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", User{}, ErrInvalidCredentials
	case err != nil:
		return "", User{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", User{}, ErrInvalidCredentials
	}

	token := rand.Text() // 26 base32 characters, about 130 random bits
	now := s.now()
	if err := s.store.CreateSession(ctx, hashToken(token), user.ID, now, now.Add(SessionTTL)); err != nil {
		return "", User{}, fmt.Errorf("create session: %w", err)
	}
	return token, user, nil
}

// Authenticate returns the user owning a valid session token.
func (s *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthenticated
	}
	user, err := s.store.SessionUser(ctx, hashToken(token), s.now())
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	return user, err
}

// Logout ends a session. Ending an unknown session is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, hashToken(token))
}

// DeleteExpiredSessions removes sessions past their expiry.
func (s *Service) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx, s.now())
}

// hashToken is what the database stores instead of the token itself: a
// leaked database does not contain usable session tokens. A fast hash is
// enough because tokens are long and random, unlike passwords.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
