package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
)

func TestSessionsExpireAndOnlyHashesAreStored(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	svc := auth.NewService(s)
	if _, err := svc.Register(ctx, "raul", "correct horse"); err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.Login(ctx, "raul", "correct horse")
	if err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := s.db.QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token || len(stored) != 64 {
		t.Errorf("token_hash = %q; want a 64-char SHA-256, never the token", stored)
	}

	later := time.Now().Add(auth.SessionTTL + time.Minute)
	if _, err := s.SessionUser(ctx, stored, later); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("expired session err = %v, want ErrNotFound", err)
	}
	if n, err := s.DeleteExpiredSessions(ctx, later); err != nil || n != 1 {
		t.Errorf("DeleteExpiredSessions = %d, %v; want 1, nil", n, err)
	}
	if _, err := svc.Authenticate(ctx, token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Errorf("after cleanup err = %v, want ErrUnauthenticated", err)
	}
}

func TestUsernamesAreCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, filepath.Join(t.TempDir(), "printhub.db"))
	defer s.Close()

	if _, err := s.CreateUser(ctx, "raul", "hash", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "RAUL", "hash", time.Now()); !errors.Is(err, auth.ErrUserExists) {
		t.Errorf("err = %v, want ErrUserExists", err)
	}
	if _, _, err := s.UserByName(ctx, "nobody"); !errors.Is(err, auth.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
