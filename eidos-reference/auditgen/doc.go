// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package auditgen records each request at the end of the method ServeHTTP
// of each handler that handlergen writes.
//
// auditgen requires the capability of handlergen, and runs after errorgen
// through [Priority], so its call is the last of the epilogue. Its options
// set the function that it calls:
//
//	options:
//	    auditgen: {package: example.com/shop/httpmw/audit, func: Record}
//
// A config without the options makes auditgen report REFERENCE-0001 at
// each struct, and append nothing.
//
// # Dependency position
//
// The package imports the packages handlergen and internal/callee of the
// module, and the root, meta, plugin and symbol packages of the SDK facade.
package auditgen
