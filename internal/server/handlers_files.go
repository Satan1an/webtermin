package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/files"
)

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		p = "/"
	}
	list, err := files.List(p)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"path": p, "entries": list})
}

func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	content, err := files.ReadText(p)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"path": p, "content": content})
}

type fileWriteReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (s *Server) handleFileWrite(w http.ResponseWriter, r *http.Request) {
	var req fileWriteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := files.WriteText(req.Path, req.Content); err != nil {
		s.auditUser(r, "files.write", req.Path, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "files.write", req.Path, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	dir := r.FormValue("dir")
	if dir == "" {
		writeJSONError(w, 400, "missing dir")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	defer file.Close()
	saved, err := files.Save(dir, header.Filename, file)
	if err != nil {
		s.auditUser(r, "files.upload", filepath.Join(dir, header.Filename), audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "files.upload", saved, audit.OutcomeOK, fmt.Sprintf("%d bytes", header.Size))
	writeJSON(w, 200, map[string]string{"path": saved})
}

func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	f, fi, err := files.OpenForDownload(p)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(p)+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
	_, _ = io.Copy(w, f)
	s.auditUser(r, "files.download", p, audit.OutcomeOK, "")
}

type mkdirReq struct {
	Path string `json:"path"`
}

func (s *Server) handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	var req mkdirReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := files.Mkdir(req.Path); err != nil {
		s.auditUser(r, "files.mkdir", req.Path, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "files.mkdir", req.Path, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type deleteReq struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var req deleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := files.Delete(req.Path, req.Recursive); err != nil {
		s.auditUser(r, "files.delete", req.Path, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "files.delete", req.Path, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type chmodReq struct {
	Path string `json:"path"`
	Mode string `json:"mode"` // octal string e.g. "0644"
}

func (s *Server) handleFileChmod(w http.ResponseWriter, r *http.Request) {
	var req chmodReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	mode, err := strconv.ParseUint(req.Mode, 8, 32)
	if err != nil {
		writeJSONError(w, 400, "invalid mode")
		return
	}
	if err := files.Chmod(req.Path, uint32(mode)); err != nil {
		s.auditUser(r, "files.chmod", req.Path, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditUser(r, "files.chmod", req.Path, audit.OutcomeOK, req.Mode)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
