package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Satan1an/webtermin/internal/audit"
	"github.com/Satan1an/webtermin/internal/auth"
	"github.com/Satan1an/webtermin/internal/config"
	"github.com/Satan1an/webtermin/internal/store"
)

type Server struct {
	Cfg     *config.Config
	Store   *store.Store
	Auth    *auth.Service
	Audit   *audit.Logger
	Log     *slog.Logger
	WebFS   fs.FS // built React app, or nil for dev
	OIDC    *auth.OIDCProvider
	HTTPSrv *http.Server
}

func New(cfg *config.Config, st *store.Store, lg *slog.Logger, webFS fs.FS) *Server {
	s := &Server{
		Cfg:   cfg,
		Store: st,
		Auth:  auth.New(st, cfg),
		Audit: audit.New(st, lg),
		Log:   lg,
		WebFS: webFS,
	}
	// Best-effort OIDC discovery. If the IdP is briefly unreachable at boot,
	// log it and continue — local login still works.
	if cfg.OIDC.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p, err := auth.NewOIDCProvider(ctx,
			cfg.OIDC.Issuer, cfg.OIDC.ClientID, cfg.OIDC.ClientSecret, cfg.OIDC.RedirectURL)
		if err != nil {
			lg.Warn("oidc disabled: discovery failed", "err", err, "issuer", cfg.OIDC.Issuer)
		} else {
			lg.Info("oidc ready", "issuer", cfg.OIDC.Issuer)
			s.OIDC = p
		}
	}
	return s
}

// Run starts the HTTPS server and blocks until ctx is done.
func (s *Server) Run(ctx context.Context) error {
	cert, certPath, keyPath, err := EnsureTLSCertificate(
		s.Cfg.Server.TLSCert, s.Cfg.Server.TLSKey, s.Cfg.DataDir)
	if err != nil {
		return err
	}
	s.Log.Info("tls certificate ready", "cert", certPath, "key", keyPath)

	mux := s.routes()
	s.HTTPSrv = &http.Server{
		Addr:         s.Cfg.Server.Listen,
		Handler:      s.securityHeaders(s.requestLogger(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // 0 — WebSocket / SSE handlers may stream for minutes.
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		},
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.HTTPSrv.Shutdown(shutdownCtx)
	}()

	// Periodic session/login-attempt sweep.
	go s.bgSweep(ctx)

	s.Log.Info("listening", "addr", s.Cfg.Server.Listen)
	err = s.HTTPSrv.ListenAndServeTLS("", "")
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) bgSweep(ctx context.Context) {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.Store.DeleteExpiredSessions()
			_ = s.Store.PurgeOldLoginAttempts(time.Now().Add(-24 * time.Hour))
		}
	}
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			s.Log.Debug("http",
				"method", r.Method, "path", r.URL.Path,
				"status", rec.status, "dur_ms", time.Since(start).Milliseconds(),
				"ip", auth.ClientIP(r),
			)
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
		s.ResponseWriter.WriteHeader(code)
	}
}

// Hijack passes through for WebSocket upgrades.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := s.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
