package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/network"
)

func (s *Server) handleNetworkList(w http.ResponseWriter, r *http.Request) {
	ifs, err := network.List(r.Context())
	if err != nil {
		if errors.Is(err, network.ErrNotAvailable) {
			writeJSON(w, 200, map[string]any{
				"available":  false,
				"interfaces": []any{},
			})
			return
		}
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"available": true, "interfaces": ifs})
}

func (s *Server) handleNetworkHostname(w http.ResponseWriter, r *http.Request) {
	h, err := network.Hostname(r.Context())
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"hostname": h})
}

type setHostnameReq struct {
	Hostname string `json:"hostname"`
}

func (s *Server) handleNetworkSetHostname(w http.ResponseWriter, r *http.Request) {
	var req setHostnameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := network.SetHostname(r.Context(), req.Hostname); err != nil {
		s.auditNet(r, "network.hostname", req.Hostname, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditNet(r, "network.hostname", req.Hostname, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type setStaticReq struct {
	Address string   `json:"address"` // x.x.x.x/yy
	Gateway string   `json:"gateway"`
	DNS     []string `json:"dns"`
}

func (s *Server) handleNetworkSetStatic(w http.ResponseWriter, r *http.Request) {
	iface := r.PathValue("name")
	if !network.ValidIface(iface) {
		writeJSONError(w, 400, "invalid interface name")
		return
	}
	var req setStaticReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := network.SetStatic(r.Context(), iface, req.Address, req.Gateway, req.DNS); err != nil {
		s.auditNet(r, "network.static", iface, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditNet(r, "network.static", iface, audit.OutcomeOK, req.Address)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleNetworkSetDHCP(w http.ResponseWriter, r *http.Request) {
	iface := r.PathValue("name")
	if !network.ValidIface(iface) {
		writeJSONError(w, 400, "invalid interface name")
		return
	}
	if err := network.SetDHCP(r.Context(), iface); err != nil {
		s.auditNet(r, "network.dhcp", iface, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditNet(r, "network.dhcp", iface, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

type setDNSReq struct {
	DNS []string `json:"dns"`
}

func (s *Server) handleNetworkSetDNS(w http.ResponseWriter, r *http.Request) {
	iface := r.PathValue("name")
	if !network.ValidIface(iface) {
		writeJSONError(w, 400, "invalid interface name")
		return
	}
	var req setDNSReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if err := network.SetDNS(r.Context(), iface, req.DNS); err != nil {
		s.auditNet(r, "network.dns", iface, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditNet(r, "network.dns", iface, audit.OutcomeOK, "")
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditNet(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
