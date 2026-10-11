// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package handlergen generates the HTTP handler of each struct under the
// directive handler.
//
// handlergen is the host of the handler set of the ensemble. Its annotator
// half stamps [KeyHandler] on each struct under //+reference:handler that
// declares the method Serve(w, r). The other plugins of the set select
// their subjects through the fact, and do not read the directive. Its
// generator half writes the handler of each struct with the fact into the
// family [Word], beside the source of the struct:
//
//	// OrdersHandler serves Orders over HTTP.
//	type OrdersHandler struct {
//		Orders *Orders
//	}
//
//	// ServeHTTP calls Serve between the statements that other plugins contribute.
//	func (o *OrdersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//		o.Orders.Serve(w, r)
//	}
//
// A plugin that extends the handler requires [Capability], and appends
// into the prologue or the epilogue of the method [ServeHTTP].
//
// # Diagnostics
//
// [MissingServe] is an Error at a struct under the directive without the
// method Serve.
//
// # Dependency position
//
// The package imports the root of the Go satellite, and the root, diag,
// directive, emit, meta, node, plugin and symbol packages of the SDK facade.
package handlergen
