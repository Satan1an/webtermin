package server

import (
	"encoding/json"
	"errors"
	"fmt"
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
	Op    string   `json:"op,omitempty"` // plan only
	Names []string `json:"names"`
	// ConfirmRemove lists the removals the admin saw in the plan and accepted.
	// Anything the real run would remove beyond this aborts the operation.
	ConfirmRemove []string `json:"confirm_remove,omitempty"`
}

type pkgOpError struct {
	Error string         `json:"error"`
	Plan  *packages.Plan `json:"plan,omitempty"`
}

func decodePkgReq(w http.ResponseWriter, r *http.Request) (*pkgOpReq, bool) {
	var req pkgOpReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return nil, false
	}
	for _, n := range req.Names {
		if !packages.ValidName(n) {
			writeJSONError(w, 400, "invalid package name: "+n)
			return nil, false
		}
	}
	return &req, true
}

// handlePackagesPlan dry-runs an operation so the UI can show exactly what
// would be installed, upgraded and removed before the admin confirms.
func (s *Server) handlePackagesPlan(w http.ResponseWriter, r *http.Request) {
	req, ok := decodePkgReq(w, r)
	if !ok {
		return
	}
	if req.Op != "install" && req.Op != "remove" && req.Op != "upgrade" {
		writeJSONError(w, 400, "op must be install, remove or upgrade")
		return
	}
	if req.Op != "upgrade" && len(req.Names) == 0 {
		writeJSONError(w, 400, "names required")
		return
	}
	p, err := packages.MakePlan(r.Context(), req.Op, req.Names...)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, p)
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
	req, ok := decodePkgReq(w, r)
	if !ok {
		return
	}
	if op != "upgrade" && len(req.Names) == 0 {
		writeJSONError(w, 400, "names required")
		return
	}
	target := strings.Join(req.Names, ",")
	p, err := packages.Execute(r.Context(), op, req.Names, req.ConfirmRemove)
	if err != nil {
		s.auditPkg(r, "pkg."+op, target, audit.OutcomeError, err.Error())
		status := 500
		switch {
		case errors.Is(err, packages.ErrBusy), errors.Is(err, packages.ErrNeedsConfirm):
			status = 409
		case errors.Is(err, packages.ErrBlocked):
			status = 403
			if len(p.Blocked) > 0 {
				b := p.Blocked[0]
				err = fmt.Errorf("%w: %s", err, strings.TrimPrefix(b.Name+": "+b.Reason, ": "))
			}
		}
		writeJSON(w, status, pkgOpError{Error: err.Error(), Plan: p})
		return
	}
	detail := ""
	if len(p.Remove) > 0 {
		detail = "removed: " + strings.Join(p.Remove, ",")
	}
	s.auditPkg(r, "pkg."+op, target, audit.OutcomeOK, detail)
	writeJSON(w, 200, map[string]any{"ok": true, "plan": p})
}

func (s *Server) auditPkg(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
