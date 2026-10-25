CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL, -- bcrypt
    created_at    TEXT    NOT NULL  -- RFC 3339, UTC
) STRICT;

-- The session token itself is never stored, only its SHA-256 (hex), so a
-- leaked database does not contain usable sessions.
CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL, -- RFC 3339, UTC
    expires_at TEXT    NOT NULL  -- RFC 3339, UTC
) STRICT;

CREATE INDEX sessions_expires_at ON sessions (expires_at);

-- Who sent each job. Jobs created before authentication existed have ''.
ALTER TABLE jobs ADD COLUMN created_by TEXT NOT NULL DEFAULT '';
