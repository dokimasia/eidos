// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package middlewaregen generates the middleware function of each struct
// that handlergen serves.
//
// middlewaregen selects its subjects through the fact
// [go.dokimi.dev/eidos/reference/handlergen.KeyHandler], and requires the
// capability of handlergen. It writes the function into the family [Word],
// beside the source of the struct:
//
//	// OrdersMiddleware wraps h in the middleware of Orders.
//	func OrdersMiddleware(h http.Handler) http.Handler {
//		h = auth.RequireAuth(h)
//		return h
//	}
//
// The body of the function has the fixed composition of every body: the
// prologue, the content, the named slots and the epilogue. The return
// follows the slots, so the content of the body is the template
// middleware.tmpl, which places the prologue, the named slot [ChainSlot]
// and the epilogue, then returns h. A plugin of the chain finds the slot
// through emit.Body.Slot and appends one statement into it. The order of
// the statements is the order of the capabilities that the plugins of the
// chain require of each other.
//
// # Dependency position
//
// The package imports the packages handlergen and internal/tree of the
// module, the root of the Go satellite, and the root, emit, meta, plugin and
// symbol packages of the SDK facade.
package middlewaregen
