// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package auditgen records each request at the end of the method ServeHTTP
// of each handler that handlergen writes.
//
// auditgen requires the capability of handlergen, and runs after the
// capability of errorgen where the composition contains errorgen. So its
// call follows the call of errorgen in the epilogue. Its options set the
// function that it calls:
//
//	options:
//	    auditgen: {package: example.com/shop/httpmw/audit, func: Record}
//
// Both options are required, so Build refuses a composition whose config
// leaves either empty.
//
// # Dependency position
//
// The package imports the packages errorgen, handlergen and internal/callee
// of the module, and the root, meta and plugin packages of the SDK facade.
package auditgen
