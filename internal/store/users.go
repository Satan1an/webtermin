package store

import (
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID         int64
	Username   string
	PWHash     string
	TOTPSecret string
	IsAdmin    bool   // legacy flag, kept for backwards-compatibility with v0.1
	Role       string // "viewer" | "operator" | "admin" — authoritative source for permissions
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, pwHash, totpSecret, role string, isAdmin bool) (*User, error) {
	now := nowUnix()
	res, err := s.DB.Exec(
		`INSERT INTO users(username, pw_hash, totp_secret, is_admin, role, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		username, pwHash, totpSecret, boolToInt(isAdmin), role, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

func (s *Store) GetUser(id int64) (*User, error) {
	row := s.DB.QueryRow(
		`SELECT id, username, pw_hash, totp_secret, is_admin, role, created_at, updated_at
		 FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) GetUserByName(username string) (*User, error) {
	row := s.DB.QueryRow(
		`SELECT id, username, pw_hash, totp_secret, is_admin, role, created_at, updated_at
		 FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.DB.Query(
		`SELECT id, username, pw_hash, totp_secret, is_admin, role, created_at, updated_at
		 FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		var u User
		var isAdmin int
		var created, updated int64
		if err := rows.Scan(&u.ID, &u.Username, &u.PWHash, &u.TOTPSecret, &isAdmin, &u.Role, &created, &updated); err != nil {
			return nil, err
		}
		u.IsAdmin = isAdmin != 0
		u.CreatedAt = time.Unix(created, 0)
		u.UpdatedAt = time.Unix(updated, 0)
		out = append(out, &u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateUserPassword(id int64, pwHash string) error {
	_, err := s.DB.Exec(`UPDATE users SET pw_hash = ?, updated_at = ? WHERE id = ?`,
		pwHash, nowUnix(), id)
	return err
}

func (s *Store) UpdateUserTOTP(id int64, totpSecret string) error {
	_, err := s.DB.Exec(`UPDATE users SET totp_secret = ?, updated_at = ? WHERE id = ?`,
		totpSecret, nowUnix(), id)
	return err
}

func (s *Store) UpdateUserRole(id int64, role string) error {
	_, err := s.DB.Exec(`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`,
		role, nowUnix(), id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var isAdmin int
	var created, updated int64
	if err := row.Scan(&u.ID, &u.Username, &u.PWHash, &u.TOTPSecret, &isAdmin, &u.Role, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	u.CreatedAt = time.Unix(created, 0)
	u.UpdatedAt = time.Unix(updated, 0)
	return &u, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
