// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package validategen generates the validator of the requests of each
// struct that handlergen serves, and guards the handler with it.
//
// validategen has two rules, which both select their subjects through the
// fact [go.dokimi.dev/eidos/reference/handlergen.KeyHandler]. The rule on
// a struct writes the validator into the family [Word], beside the source
// of the struct:
//
//	// ValidateOrders reports whether the request for Orders is well-formed.
//	func ValidateOrders(r *http.Request) error {
//		return nil
//	}
//
// The rule on an emitted method appends the call of the validator into the
// prologue of ServeHTTP:
//
//	err := ValidateOrders(r)
//	if err != nil {
//		return
//	}
//
// The call refers to the validator by its name in the package of the
// handler. A layout that moves the family [Word] into another package
// therefore breaks the build of the handler.
//
// # Dependency position
//
// The package imports the package handlergen of the module, and the root,
// emit, meta, plugin and symbol packages of the SDK facade.
package validategen
