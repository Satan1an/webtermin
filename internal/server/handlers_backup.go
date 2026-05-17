package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/backup"
	"github.com/Satan1an/webtermin/internal/store"
)

type backupOut struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	Paths     []string  `json:"paths"`
	CreatedAt time.Time `json:"created_at"`
}

func toBackupOut(b *store.Backup) backupOut {
	return backupOut{
		ID: b.ID, Name: b.Name, Path: b.Path,
		SizeBytes: b.SizeBytes, Paths: b.Paths, CreatedAt: b.CreatedAt,
	}
}

func (s *Server) handleBackupsList(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListBackups()
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	out := make([]backupOut, 0, len(list))
	for _, b := range list {
		out = append(out, toBackupOut(b))
	}
	writeJSON(w, 200, out)
}

type backupCreateReq struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

func (s *Server) handleBackupCreate(w http.ResponseWriter, r *http.Request) {
	var req backupCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if !backup.ValidName(req.Name) {
		writeJSONError(w, 400, "invalid backup name (alnum, _.-, 1–64 chars)")
		return
	}
	paths := req.Paths
	if len(paths) == 0 {
		paths = backup.DefaultPaths
	}
	outDir := filepath.Join(s.Cfg.DataDir, "backups")
	full, size, err := backup.Create(outDir, req.Name, paths)
	if err != nil {
		s.auditBackup(r, "backup.create", req.Name, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	u := UserFrom(r)
	rec, err := s.Store.CreateBackup(req.Name, full, size, paths, u.ID)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditBackup(r, "backup.create", req.Name, audit.OutcomeOK,
		"size="+humanSize(size)+" paths="+strings.Join(paths, ","))
	writeJSON(w, 200, toBackupOut(rec))
}

func (s *Server) handleBackupDownload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	b, err := s.Store.GetBackup(id)
	if err != nil {
		writeJSONError(w, 404, "backup not found")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+filepath.Base(b.Path)+`"`)
	http.ServeFile(w, r, b.Path)
	s.auditBackup(r, "backup.download", b.Name, audit.OutcomeOK, "")
}

func (s *Server) handleBackupDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		writeJSONError(w, 400, "invalid id")
		return
	}
	b, err := s.Store.GetBackup(id)
	if err != nil {
		writeJSONError(w, 404, "backup not found")
		return
	}
	// Remove the file first; if that fails the DB row stays for visibility.
	if err := removeIfExists(b.Path); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	if err := s.Store.DeleteBackup(b.ID); err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditBackup(r, "backup.delete", b.Name, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditBackup(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}

func removeIfExists(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func humanSize(n int64) string {
	const k = 1024
	if n < k {
		return formatInt(n) + " B"
	}
	div, exp := int64(k), 0
	for n2 := n / k; n2 >= k; n2 /= k {
		div *= k
		exp++
	}
	units := "KMGTPE"
	return formatInt(n/div) + " " + string(units[exp]) + "B"
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
