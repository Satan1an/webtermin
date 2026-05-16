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
	IsAdmin    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, pwHash, totpSecret string, isAdmin bool) (*User, error) {
	now := nowUnix()
	res, err := s.DB.Exec(
		`INSERT INTO users(username, pw_hash, totp_secret, is_admin, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		username, pwHash, totpSecret, boolToInt(isAdmin), now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

func (s *Store) GetUser(id int64) (*User, error) {
	row := s.DB.QueryRow(
		`SELECT id, username, pw_hash, totp_secret, is_admin, created_at, updated_at
		 FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Store) GetUserByName(username string) (*User, error) {
	row := s.DB.QueryRow(
		`SELECT id, username, pw_hash, totp_secret, is_admin, created_at, updated_at
		 FROM users WHERE username = ?`, username)
	return scanUser(row)
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

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var isAdmin int
	var created, updated int64
	if err := row.Scan(&u.ID, &u.Username, &u.PWHash, &u.TOTPSecret, &isAdmin, &created, &updated); err != nil {
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
