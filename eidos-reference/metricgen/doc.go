// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package metricgen adds a measuring middleware to the chain of each
// handler that handlergen serves.
//
// metricgen is the second plugin of the middleware chain. It requires the
// capability of authgen, so its statement follows the one of authgen
// whatever the order of the composition's list, and the next plugin of the
// chain requires [Capability]. Its options set the function that wraps the
// handler:
//
//	options:
//	    metricgen: {package: example.com/shop/httpmw/metrics, func: RecordLatency}
//
// Both options are required, so Build refuses a composition whose config
// leaves either empty.
//
// # Dependency position
//
// The package imports the packages authgen, handlergen and internal/callee
// of the module, and the root, meta and plugin packages of the SDK facade.
package metricgen
