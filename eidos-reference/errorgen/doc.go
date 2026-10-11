// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package errorgen calls a function of the request at the end of the
// method ServeHTTP of each handler that handlergen writes.
//
// errorgen requires the capability of handlergen, and orders its call
// against the call of auditgen through [Priority]: a priority orders a
// plugin against every generator of the role, so the call of errorgen
// precedes the call of auditgen in the epilogue. Its options set the
// function that it calls:
//
//	options:
//	    errorgen: {package: example.com/shop/httpmw/errors, func: Recover}
//
// A config without the options makes errorgen report REFERENCE-0001 at
// each struct, and append nothing.
//
// # Dependency position
//
// The package imports the packages handlergen and internal/callee of the
// module, and the root, meta, plugin and symbol packages of the SDK facade.
package errorgen
