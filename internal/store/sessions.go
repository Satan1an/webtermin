package store

import (
	"database/sql"
	"errors"
	"time"
)

type Session struct {
	ID         string
	UserID     int64
	CSRFToken  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
	IP         string
	UserAgent  string
}

func (s *Store) CreateSession(id string, userID int64, csrf string, ttl time.Duration, ip, ua string) (*Session, error) {
	now := time.Now()
	exp := now.Add(ttl)
	_, err := s.DB.Exec(
		`INSERT INTO sessions(id, user_id, csrf_token, created_at, expires_at, last_seen_at, ip, user_agent)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, csrf, now.Unix(), exp.Unix(), now.Unix(), ip, ua,
	)
	if err != nil {
		return nil, err
	}
	return &Session{
		ID: id, UserID: userID, CSRFToken: csrf,
		CreatedAt: now, ExpiresAt: exp, LastSeenAt: now, IP: ip, UserAgent: ua,
	}, nil
}

func (s *Store) GetSession(id string) (*Session, error) {
	row := s.DB.QueryRow(
		`SELECT id, user_id, csrf_token, created_at, expires_at, last_seen_at, ip, user_agent
		 FROM sessions WHERE id = ?`, id)
	var sess Session
	var created, expires, seen int64
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &created, &expires, &seen, &sess.IP, &sess.UserAgent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sess.CreatedAt = time.Unix(created, 0)
	sess.ExpiresAt = time.Unix(expires, 0)
	sess.LastSeenAt = time.Unix(seen, 0)
	return &sess, nil
}

func (s *Store) TouchSession(id string) error {
	_, err := s.DB.Exec(`UPDATE sessions SET last_seen_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}
