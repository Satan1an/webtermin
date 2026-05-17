package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/packages"
)

type packagesStatus struct {
	Manager string `json:"manager"` // "apt" | "dnf" | "" if unsupported
}

func (s *Server) handlePackagesStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, packagesStatus{Manager: string(packages.Detect())})
}

func (s *Server) handlePackagesSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pkgs, err := packages.Search(r.Context(), q)
	if err != nil {
		writeJSONError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, pkgs)
}

func (s *Server) handlePackagesInstalled(w http.ResponseWriter, r *http.Request) {
	pkgs, err := packages.ListInstalled(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, pkgs)
}

type pkgOpReq struct {
	Names []string `json:"names"`
}

func (s *Server) handlePackagesInstall(w http.ResponseWriter, r *http.Request) {
	s.runPackagesOp(w, r, "install")
}

func (s *Server) handlePackagesRemove(w http.ResponseWriter, r *http.Request) {
	s.runPackagesOp(w, r, "remove")
}

func (s *Server) handlePackagesUpgrade(w http.ResponseWriter, r *http.Request) {
	s.runPackagesOp(w, r, "upgrade")
}

func (s *Server) runPackagesOp(w http.ResponseWriter, r *http.Request, op string) {
	var req pkgOpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	for _, n := range req.Names {
		if !packages.ValidName(n) {
			writeJSONError(w, 400, "invalid package name: "+n)
			return
		}
	}
	var err error
	switch op {
	case "install":
		err = packages.Install(r.Context(), req.Names...)
	case "remove":
		err = packages.Remove(r.Context(), req.Names...)
	case "upgrade":
		err = packages.Upgrade(r.Context(), req.Names...)
	}
	target := strings.Join(req.Names, ",")
	if err != nil {
		s.auditPkg(r, "pkg."+op, target, audit.OutcomeError, err.Error())
		writeJSONError(w, 500, err.Error())
		return
	}
	s.auditPkg(r, "pkg."+op, target, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditPkg(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
