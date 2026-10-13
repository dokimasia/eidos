// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package tracegen adds a tracing middleware to the chain of each handler
// that handlergen serves.
//
// tracegen is the last plugin of the middleware chain. It requires the
// capability of metricgen, so its statement follows the one of metricgen
// whatever the order of the composition's list. Its options set the
// function that wraps the handler:
//
//	options:
//	    tracegen: {package: example.com/shop/httpmw/trace, func: StartSpan}
//
// Both options are required, so Build refuses a composition whose config
// leaves either empty.
//
// # Dependency position
//
// The package imports the packages handlergen, internal/callee and
// metricgen of the module, and the root, meta and plugin packages of the
// SDK facade.
package tracegen
