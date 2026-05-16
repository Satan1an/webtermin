package store

import "time"

func (s *Store) RecordLoginAttempt(ip, username string, success bool) error {
	_, err := s.DB.Exec(
		`INSERT INTO login_attempts(ip, ts, success, username) VALUES(?, ?, ?, ?)`,
		ip, time.Now().Unix(), boolToInt(success), username,
	)
	return err
}

// CountRecentFailures returns the number of failed attempts from ip since `since`.
func (s *Store) CountRecentFailures(ip string, since time.Time) (int, error) {
	var n int
	err := s.DB.QueryRow(
		`SELECT COUNT(*) FROM login_attempts WHERE ip = ? AND success = 0 AND ts >= ?`,
		ip, since.Unix(),
	).Scan(&n)
	return n, err
}

func (s *Store) PurgeOldLoginAttempts(before time.Time) error {
	_, err := s.DB.Exec(`DELETE FROM login_attempts WHERE ts < ?`, before.Unix())
	return err
}
