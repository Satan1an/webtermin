package server

import (
	"context"
	"net/http"

	"github.com/Satan1an/webtermin/internal/store"
)

type ctxKey int

const (
	ctxKeySession ctxKey = iota
	ctxKeyUser
)

func withSession(ctx context.Context, s *store.Session, u *store.User) context.Context {
	ctx = context.WithValue(ctx, ctxKeySession, s)
	ctx = context.WithValue(ctx, ctxKeyUser, u)
	return ctx
}

func SessionFrom(r *http.Request) *store.Session {
	if v := r.Context().Value(ctxKeySession); v != nil {
		return v.(*store.Session)
	}
	return nil
}

func UserFrom(r *http.Request) *store.User {
	if v := r.Context().Value(ctxKeyUser); v != nil {
		return v.(*store.User)
	}
	return nil
}
