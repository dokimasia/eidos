// Package tracing traces the requests of the shop.
package tracing

import (
	"log"
	"net/http"
)

// StartSpan returns a handler that logs the start and the end of a request
// around the call of h.
func StartSpan(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("span: start %s", r.URL.Path)
		h.ServeHTTP(w, r)
		log.Printf("span: end %s", r.URL.Path)
	})
}
