// Package noserve declares structs under the directive handler without the
// method Serve that a handler calls.
package noserve

import "net/http"

// Ledger declares no method Serve.
//
//+reference:handler
type Ledger struct{}

// Report declares a method Serve with one parameter.
//
//+reference:handler
type Report struct{}

// Serve writes the report to w.
func (r *Report) Serve(w []byte) {}

// Tally declares a method Serve with two parameters of other types.
//
//+reference:handler
type Tally struct{}

// Serve adds a and b.
func (t *Tally) Serve(a, b int) {}

// Copy declares a method Serve that takes the request by value.
//
//+reference:handler
type Copy struct{}

// Serve writes the copy to w.
func (c *Copy) Serve(w http.ResponseWriter, r http.Request) {}

// Cookie declares a method Serve that takes a cookie in place of the
// request.
//
//+reference:handler
type Cookie struct{}

// Serve writes the cookie to w.
func (c *Cookie) Serve(w http.ResponseWriter, k *http.Cookie) {}
