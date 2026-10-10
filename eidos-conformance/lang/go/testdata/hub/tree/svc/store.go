package svc

import "context"

// Session is one signed-in user's state.
type Session struct {
	ID      string
	Expires int64
}

// Store is the persistence seam for sessions.
//
//+acme:stub tag=test
type Store interface {
	Get(ctx context.Context, key string) (Session, error)
	Put(ctx context.Context, s Session) error
}
