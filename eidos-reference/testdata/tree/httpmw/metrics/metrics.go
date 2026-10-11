// Package metrics measures the requests of the shop.
package metrics

import (
	"log"
	"net/http"
	"time"
)

// RecordLatency returns a handler that serves a request through h and
// logs how long the request took.
func RecordLatency(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		log.Printf("latency: %s %s", r.URL.Path, time.Since(start))
	})
}
