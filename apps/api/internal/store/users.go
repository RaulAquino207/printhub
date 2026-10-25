package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/RaulAquino207/printhub/apps/api/internal/auth"
)

// CreateUser inserts a user, returning auth.ErrUserExists for a taken name
// (compared case-insensitively).
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, now time.Time) (auth.User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, formatTime(now),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return auth.User{}, auth.ErrUserExists
		}
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	return auth.User{ID: id, Username: username}, nil
}

// UserByName returns a user and its password hash, or auth.ErrNotFound.
func (s *Store) UserByName(ctx context.Context, username string) (auth.User, string, error) {
	var (
		u    auth.User
		hash string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, "", auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, "", fmt.Errorf("get user: %w", err)
	}
	return u, hash, nil
}

// CreateSession stores a session by the hash of its token.
func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, now, expires time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, formatTime(now), formatTime(expires),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionUser returns the owner of an unexpired session, or auth.ErrNotFound.
// RFC 3339 UTC strings of equal precision compare correctly as text.
func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (auth.User, error) {
	var u auth.User
	err := s.db.QueryRowContext(ctx,
		`SELECT u.id, u.username FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ?`,
		tokenHash, formatTime(now),
	).Scan(&u.ID, &u.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("get session: %w", err)
	}
	return u, nil
}

// DeleteSession removes a session; a missing one is not an error.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes sessions expired at now and reports how many.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, formatTime(now))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return res.RowsAffected()
}
