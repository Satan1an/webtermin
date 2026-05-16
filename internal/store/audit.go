package store

import (
	"database/sql"
	"time"
)

type AuditEntry struct {
	ID       int64     `json:"id"`
	Time     time.Time `json:"time"`
	UserID   *int64    `json:"user_id,omitempty"`
	Username string    `json:"username"`
	IP       string    `json:"ip"`
	Action   string    `json:"action"`
	Target   string    `json:"target"`
	Outcome  string    `json:"outcome"` // "ok" | "error" | "denied"
	Detail   string    `json:"detail"`
}

func (s *Store) WriteAudit(e AuditEntry) error {
	var uid any
	if e.UserID != nil {
		uid = *e.UserID
	}
	_, err := s.DB.Exec(
		`INSERT INTO audit_log(ts, user_id, username, ip, action, target, outcome, detail)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		time.Now().Unix(), uid, e.Username, e.IP, e.Action, e.Target, e.Outcome, e.Detail,
	)
	return err
}

func (s *Store) ListAudit(limit int) ([]AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.DB.Query(
		`SELECT id, ts, user_id, username, ip, action, target, outcome, detail
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var ts int64
		var uid sql.NullInt64
		if err := rows.Scan(&e.ID, &ts, &uid, &e.Username, &e.IP, &e.Action, &e.Target, &e.Outcome, &e.Detail); err != nil {
			return nil, err
		}
		e.Time = time.Unix(ts, 0)
		if uid.Valid {
			v := uid.Int64
			e.UserID = &v
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
