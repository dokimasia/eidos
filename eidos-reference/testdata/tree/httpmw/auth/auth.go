// Package auth authenticates the requests of the shop.
package auth

import "net/http"

// RequireAuth returns a handler that serves a request through h when the
// request has an Authorization header, and refuses it otherwise.
func RequireAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}
