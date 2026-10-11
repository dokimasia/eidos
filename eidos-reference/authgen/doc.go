// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package authgen adds an authentication middleware to the chain of each
// handler that handlergen serves.
//
// authgen is the first plugin of the middleware chain. It requires the
// capability of middlewaregen, and the next plugin of the chain requires
// [Capability]. Its options set the function that wraps the handler:
//
//	options:
//	    authgen: {package: example.com/shop/httpmw/auth, func: RequireAuth}
//
// With the options of the example, the chain of OrdersMiddleware opens
// with h = auth.RequireAuth(h). A config without the options makes authgen
// report REFERENCE-0001 at each struct, and append nothing.
//
// # Dependency position
//
// The package imports the packages handlergen, middlewaregen and
// internal/callee of the module, and the root, meta, plugin and symbol
// packages of the SDK facade.
package authgen
