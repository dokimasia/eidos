// Package errors recovers the requests of the shop that panic.
package errors

import (
	"log"
	"net/http"
)

// Recover recovers the panic of a handler when a deferred call runs it. It
// logs the panic and writes an internal server error to w. A call that is
// not deferred finds no panic and does nothing.
func Recover(w http.ResponseWriter, r *http.Request) {
	if v := recover(); v != nil {
		log.Printf("panic: %s %v", r.URL.Path, v)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
