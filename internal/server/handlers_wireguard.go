package server

import (
	"encoding/json"
	"net/http"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/wireguard"
)

func (s *Server) handleWGStatus(w http.ResponseWriter, r *http.Request) {
	iface := r.URL.Query().Get("iface")
	st, err := wireguard.GetStatus(r.Context(), iface)
	if err != nil {
		writeJSONError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, st)
}

type wgAddPeerReq struct {
	Iface      string `json:"iface"`
	Comment    string `json:"comment"`
	PublicKey  string `json:"public_key"`
	AllowedIPs string `json:"allowed_ips"`
	Endpoint   string `json:"endpoint"`
}

type wgAddPeerResp struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key,omitempty"` // only set when server generated the keypair
}

func (s *Server) handleWGAddPeer(w http.ResponseWriter, r *http.Request) {
	var req wgAddPeerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if req.Iface == "" {
		req.Iface = "wg0"
	}
	pub, priv, err := wireguard.AddPeer(r.Context(), req.Iface, wireguard.PeerSpec{
		Comment: req.Comment, PublicKey: req.PublicKey,
		AllowedIPs: req.AllowedIPs, Endpoint: req.Endpoint,
	})
	if err != nil {
		s.auditWG(r, "wg.peer.add", req.Iface, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditWG(r, "wg.peer.add", req.Iface, audit.OutcomeOK,
		"comment="+req.Comment+" allowed="+req.AllowedIPs)
	writeJSON(w, 200, wgAddPeerResp{PublicKey: pub, PrivateKey: priv})
}

type wgRemovePeerReq struct {
	Iface     string `json:"iface"`
	PublicKey string `json:"public_key"`
}

func (s *Server) handleWGRemovePeer(w http.ResponseWriter, r *http.Request) {
	var req wgRemovePeerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, 400, "bad request")
		return
	}
	if req.Iface == "" {
		req.Iface = "wg0"
	}
	if err := wireguard.RemovePeer(r.Context(), req.Iface, req.PublicKey); err != nil {
		s.auditWG(r, "wg.peer.remove", req.Iface, audit.OutcomeError, err.Error())
		writeJSONError(w, 400, err.Error())
		return
	}
	s.auditWG(r, "wg.peer.remove", req.Iface, audit.OutcomeOK, req.PublicKey)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) auditWG(r *http.Request, action, target, outcome, detail string) {
	u := UserFrom(r)
	uid := u.ID
	s.Audit.Write(audit.Event{
		UserID: &uid, Username: u.Username, IP: auth.ClientIP(r),
		Action: action, Target: target, Outcome: outcome, Detail: detail,
	})
}
