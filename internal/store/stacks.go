package store

import (
	"database/sql"
	"errors"
	"time"
)

// Stack is the persistent record of a docker-compose-style stack. The raw
// compose YAML is stored as-is — the engine state is authoritative for what
// actually runs (queried via Docker labels), and the stored YAML is just the
// source of truth for "what was last deployed."
type Stack struct {
	ID        int64
	Name      string
	Compose   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (s *Store) CreateStack(name, compose string) (*Stack, error) {
	now := nowUnix()
	res, err := s.DB.Exec(
		`INSERT INTO stacks(name, compose, created_at, updated_at) VALUES(?, ?, ?, ?)`,
		name, compose, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetStack(id)
}

func (s *Store) GetStack(id int64) (*Stack, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, compose, created_at, updated_at FROM stacks WHERE id = ?`, id)
	return scanStack(row)
}

func (s *Store) GetStackByName(name string) (*Stack, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, compose, created_at, updated_at FROM stacks WHERE name = ?`, name)
	return scanStack(row)
}

func (s *Store) ListStacks() ([]*Stack, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, compose, created_at, updated_at FROM stacks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Stack
	for rows.Next() {
		st, err := scanStackRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStackCompose(id int64, compose string) error {
	_, err := s.DB.Exec(`UPDATE stacks SET compose = ?, updated_at = ? WHERE id = ?`,
		compose, nowUnix(), id)
	return err
}

func (s *Store) DeleteStack(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM stacks WHERE id = ?`, id)
	return err
}

func scanStack(row *sql.Row) (*Stack, error) {
	st, err := scanStackRows(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return st, err
}

type stackScanner interface {
	Scan(dest ...any) error
}

func scanStackRows(s stackScanner) (*Stack, error) {
	var st Stack
	var created, updated int64
	if err := s.Scan(&st.ID, &st.Name, &st.Compose, &created, &updated); err != nil {
		return nil, err
	}
	st.CreatedAt = time.Unix(created, 0)
	st.UpdatedAt = time.Unix(updated, 0)
	return &st, nil
}
