package auth_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
	"github.com/RaulAquino207/printhub/apps/api/internal/store"
)

// These tests use the real SQLite store, so they also cover the queries.
// They live in package auth_test because store imports auth.

func newService(t *testing.T) (*auth.Service, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "printhub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return auth.NewService(st), st
}

func TestRegisterValidates(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()

	for _, tt := range []struct{ user, pass string }{
		{"ab", "long enough"},             // username too short
		{"Raul", "long enough"},           // uppercase
		{"raul aquino", "long enough"},    // space
		{"raul", "short"},                 // password too short
		{"raul", strings.Repeat("x", 73)}, // past bcrypt's 72-byte limit
	} {
		if _, err := s.Register(ctx, tt.user, tt.pass); err == nil {
			t.Errorf("Register(%q, %d bytes) succeeded, want an error", tt.user, len(tt.pass))
		}
	}

	if _, err := s.Register(ctx, "raul", "correct horse"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := s.Register(ctx, "raul", "another password"); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("duplicate Register err = %v, want ErrUserExists", err)
	}
}

func TestLoginAndSessions(t *testing.T) {
	s, _ := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "raul", "correct horse"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Login(ctx, "raul", "wrong password"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("wrong password err = %v, want ErrInvalidCredentials", err)
	}
	if _, _, err := s.Login(ctx, "nobody", "correct horse"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("unknown user err = %v, want ErrInvalidCredentials", err)
	}

	token, user, err := s.Login(ctx, "raul", "correct horse")
	if err != nil || token == "" || user.Username != "raul" {
		t.Fatalf("Login = %q, %+v, %v", token, user, err)
	}

	got, err := s.Authenticate(ctx, token)
	if err != nil || got.Username != "raul" {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}

	for _, bad := range []string{"", "not-a-token", token + "x"} {
		if _, err := s.Authenticate(ctx, bad); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("Authenticate(%q) err = %v, want ErrUnauthenticated", bad, err)
		}
	}

	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("after logout err = %v, want ErrUnauthenticated", err)
	}
}
