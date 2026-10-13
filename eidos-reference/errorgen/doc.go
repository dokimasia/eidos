// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package errorgen calls a function of the request at the end of the
// method ServeHTTP of each handler that handlergen writes.
//
// errorgen requires the capability of handlergen. auditgen runs after the
// capability of errorgen, so the call of errorgen precedes the call of
// auditgen in the epilogue where the composition contains both. Its options
// set the function that it calls:
//
//	options:
//	    errorgen: {package: example.com/shop/httpmw/errors, func: Recover}
//
// Both options are required, so Build refuses a composition whose config
// leaves either empty.
//
// # Dependency position
//
// The package imports the packages handlergen and internal/callee of the
// module, and the root, meta and plugin packages of the SDK facade.
package errorgen
