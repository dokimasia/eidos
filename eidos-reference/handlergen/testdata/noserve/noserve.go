// Package noserve declares structs under the directive handler without the
// method Serve that a handler calls.
package noserve

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
