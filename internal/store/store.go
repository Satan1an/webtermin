package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	// Pure-Go SQLite driver (no CGO).
	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

var ErrNotFound = errors.New("not found")

func Open(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "webtermin.db")
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // sqlite + writes serialised
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT NOT NULL UNIQUE,
    pw_hash      TEXT NOT NULL,
    totp_secret  TEXT NOT NULL DEFAULT '',
    is_admin     INTEGER NOT NULL DEFAULT 0,
    role         TEXT NOT NULL DEFAULT 'admin',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id           TEXT PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token   TEXT NOT NULL,
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    ip           TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ts         INTEGER NOT NULL,
    user_id    INTEGER,
    username   TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    action     TEXT NOT NULL,
    target     TEXT NOT NULL DEFAULT '',
    outcome    TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id);

CREATE TABLE IF NOT EXISTS login_attempts (
    ip         TEXT NOT NULL,
    ts         INTEGER NOT NULL,
    success    INTEGER NOT NULL,
    username   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_ts ON login_attempts(ip, ts);

CREATE TABLE IF NOT EXISTS api_tokens (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT NOT NULL,
    hash          TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL,
    owner_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at    INTEGER NOT NULL,
    last_used_at  INTEGER NOT NULL DEFAULT 0,
    expires_at    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_api_tokens_owner ON api_tokens(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_api_tokens_hash ON api_tokens(hash);
`

func (s *Store) migrate() error {
	if _, err := s.DB.Exec(schema); err != nil {
		return err
	}
	// Upgrade-path: pre-RBAC databases need the column added explicitly.
	// `ADD COLUMN` is safe under SQLite — it's a metadata-only operation.
	return s.ensureColumn("users", "role", "TEXT NOT NULL DEFAULT 'admin'")
}

// ensureColumn adds `column` to `table` only if it doesn't already exist.
// Returns nil for both "already there" and "added successfully".
func (s *Store) ensureColumn(table, column, definition string) error {
	rows, err := s.DB.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	_, err = s.DB.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func nowUnix() int64 { return time.Now().Unix() }
