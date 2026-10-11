// Package audit records the requests of the shop.
package audit

import (
	"log"
	"net/http"
)

// Record logs the method and the path of the request r.
func Record(_ http.ResponseWriter, r *http.Request) {
	log.Printf("audit: %s %s", r.Method, r.URL.Path)
}
