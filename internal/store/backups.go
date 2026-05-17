package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Backup is the record of a single tar.gz snapshot. The actual file lives on
// disk under DataDir/backups/; this row only tracks where + when + how big.
type Backup struct {
	ID        int64
	Name      string
	Path      string
	SizeBytes int64
	Paths     []string // paths included in this archive
	CreatedAt time.Time
	CreatedBy *int64
}

func (s *Store) CreateBackup(name, path string, size int64, paths []string, createdBy int64) (*Backup, error) {
	now := nowUnix()
	res, err := s.DB.Exec(
		`INSERT INTO backups(name, path, size_bytes, paths, created_at, created_by)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		name, path, size, strings.Join(paths, "\n"), now, createdBy,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetBackup(id)
}

func (s *Store) GetBackup(id int64) (*Backup, error) {
	row := s.DB.QueryRow(
		`SELECT id, name, path, size_bytes, paths, created_at, created_by
		 FROM backups WHERE id = ?`, id)
	return scanBackup(row)
}

func (s *Store) ListBackups() ([]*Backup, error) {
	rows, err := s.DB.Query(
		`SELECT id, name, path, size_bytes, paths, created_at, created_by
		 FROM backups ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Backup
	for rows.Next() {
		b, err := scanBackupRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBackup(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM backups WHERE id = ?`, id)
	return err
}

func scanBackup(row *sql.Row) (*Backup, error) {
	b, err := scanBackupRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

type backupScanner interface {
	Scan(dest ...any) error
}

func scanBackupRow(s backupScanner) (*Backup, error) {
	var b Backup
	var paths string
	var created int64
	var createdBy sql.NullInt64
	if err := s.Scan(&b.ID, &b.Name, &b.Path, &b.SizeBytes, &paths, &created, &createdBy); err != nil {
		return nil, err
	}
	b.CreatedAt = time.Unix(created, 0)
	if paths != "" {
		b.Paths = strings.Split(paths, "\n")
	}
	if createdBy.Valid {
		v := createdBy.Int64
		b.CreatedBy = &v
	}
	return &b, nil
}
