package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCProvider wraps a discovered IdP and exposes the two operations webtermin
// actually needs: the start URL (redirect to provider) and the callback
// exchange (token + ID-token verification).
type OIDCProvider struct {
	cfg          *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	issuer       string
	allowedScope []string
}

// NewOIDCProvider performs OIDC discovery against `issuer` and returns a
// ready-to-use provider. Returns nil + nil if Enabled() is false on the
// caller's config — callers check for nil before exposing the login button.
func NewOIDCProvider(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*OIDCProvider, error) {
	if issuer == "" || clientID == "" || clientSecret == "" {
		return nil, errors.New("oidc: issuer / client_id / client_secret are required")
	}
	if redirectURL == "" {
		return nil, errors.New("oidc: redirect_url is required")
	}
	prov, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %s: %w", issuer, err)
	}
	return &OIDCProvider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     prov.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier:     prov.Verifier(&oidc.Config{ClientID: clientID}),
		issuer:       issuer,
		allowedScope: []string{oidc.ScopeOpenID, "profile", "email"},
	}, nil
}

// AuthURL returns the URL the user should be redirected to in order to begin
// the code flow. `state` is the caller-generated nonce that links the start
// to the callback.
func (p *OIDCProvider) AuthURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Identity is the subset of an OIDC ID-token we care about for sign-in.
type Identity struct {
	Subject string
	Email   string
	Name    string
	Issuer  string
}

// Exchange validates the auth code from the callback and returns the verified
// caller identity. Caller should map Subject (or Email) to a webtermin
// account, creating the row on first sign-in.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, errors.New("oidc: no id_token in response")
	}
	idTok, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("oidc id_token verify: %w", err)
	}
	var claims struct {
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc claims: %w", err)
	}
	name := claims.PreferredUsername
	if name == "" {
		name = claims.Name
	}
	return &Identity{
		Subject: idTok.Subject,
		Email:   claims.Email,
		Name:    name,
		Issuer:  p.issuer,
	}, nil
}

// NewState mints a fresh CSRF-equivalent state cookie. Same size + format as
// our session token so the rate-limit code can treat it the same.
func NewState() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// 22 chars base64url without padding — fits comfortably in a cookie.
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// StateCookie is the short-lived cookie that links /oidc/start to /oidc/callback.
type StateCookie struct {
	Value     string
	ExpiresAt time.Time
}

func NewStateCookie() StateCookie {
	return StateCookie{
		Value:     NewState(),
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
}
