package svc

// Session is one signed-in user's state.
type Session struct {
	ID string
}

// Store is the persistence seam for sessions.
//
//+acme:stub
type Store interface {
	Get(key string) (Session, error)
	Put(s Session) error
}
