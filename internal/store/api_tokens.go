package store

import (
	"database/sql"
	"errors"
	"time"
)

// APIToken is the public record for a programmatic-access token. The plaintext
// token value is never persisted — only its SHA-256 hash. The plaintext is
// returned once at creation time and that's the user's only chance to copy it.
type APIToken struct {
	ID          int64
	Name        string
	Hash        string // SHA-256 hex, used only for lookup
	Role        string
	OwnerUserID int64
	CreatedAt   time.Time
	LastUsedAt  time.Time // zero value means "never used"
	ExpiresAt   time.Time // zero value means "no expiry"
}

func (s *Store) CreateAPIToken(name, hash, role string, ownerID int64, expiresAt time.Time) (*APIToken, error) {
	now := nowUnix()
	exp := int64(0)
	if !expiresAt.IsZero() {
		exp = expiresAt.Unix()
	}
	res, err := s.DB.Exec(
		`INSERT INTO api_tokens(name, hash, role, owner_user_id, created_at, expires_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		name, hash, role, ownerID, now, exp,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAPIToken(id)
}

func (s *Store) GetAPIToken(id int64) (*APIToken, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, hash, role, owner_user_id, created_at, last_used_at, expires_at
		 FROM api_tokens WHERE id = ?`, id)
	return scanAPIToken(row)
}

func (s *Store) GetAPITokenByHash(hash string) (*APIToken, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, hash, role, owner_user_id, created_at, last_used_at, expires_at
		 FROM api_tokens WHERE hash = ?`, hash)
	return scanAPIToken(row)
}

// ListAPITokens returns tokens; if ownerID > 0 it filters to that owner.
func (s *Store) ListAPITokens(ownerID int64) ([]*APIToken, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if ownerID > 0 {
		rows, err = s.DB.Query(
			`SELECT id, name, hash, role, owner_user_id, created_at, last_used_at, expires_at
			 FROM api_tokens WHERE owner_user_id = ? ORDER BY id DESC`, ownerID)
	} else {
		rows, err = s.DB.Query(
			`SELECT id, name, hash, role, owner_user_id, created_at, last_used_at, expires_at
			 FROM api_tokens ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*APIToken
	for rows.Next() {
		t, err := scanAPITokenRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAPIToken(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM api_tokens WHERE id = ?`, id)
	return err
}

func (s *Store) TouchAPIToken(id int64) error {
	_, err := s.DB.Exec(`UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

type apiTokenScanner interface {
	Scan(dest ...any) error
}

func scanAPIToken(row *sql.Row) (*APIToken, error) {
	t, err := scanAPITokenRows(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func scanAPITokenRows(s apiTokenScanner) (*APIToken, error) {
	var t APIToken
	var created, lastUsed, expires int64
	if err := s.Scan(&t.ID, &t.Name, &t.Hash, &t.Role, &t.OwnerUserID, &created, &lastUsed, &expires); err != nil {
		return nil, err
	}
	t.CreatedAt = time.Unix(created, 0)
	if lastUsed > 0 {
		t.LastUsedAt = time.Unix(lastUsed, 0)
	}
	if expires > 0 {
		t.ExpiresAt = time.Unix(expires, 0)
	}
	return &t, nil
}
