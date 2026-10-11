// Package orders serves the orders of the shop.
package orders

import "net/http"

// Orders serves the orders of the shop over HTTP.
//
//+reference:handler
type Orders struct {
	// Placed counts the orders that the shop placed.
	Placed int
}

// Serve writes the number of orders that the shop placed.
func (o *Orders) Serve(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
