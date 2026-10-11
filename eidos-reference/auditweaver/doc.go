// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package auditweaver logs the entry of each method that the plan emits
// from a declaration under the directive audit.
//
// The directive gates a rule on emitted methods, so the gate reads the
// instance on the origin of each method: a method of <T>Repo, or of the
// mock of <T>Repository, derives from the struct <T> under
// //+reference:audit. auditweaver requires the capability of debugweaver,
// so where both weave one method, the trace precedes the audit:
//
//	// Get implements ProductRepository with zero values.
//	func (p *ProductRepo) Get(ctx context.Context, id string) (*Product, error) {
//		log.Printf("debug: %s entered", "Product.Get")
//		log.Printf("audit: %s", "Product.Get")
//		return nil, nil
//	}
//
// The options package, func and format set the call.
//
// # Dependency position
//
// The package imports the packages debugweaver and internal/callee of the
// module, and the root, directive, plugin and symbol packages of the SDK
// facade.
package auditweaver
