// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package debugweaver logs the entry of each method that the plan emits
// from a repository.
//
// debugweaver selects the emitted methods whose origin repogen stamps with
// [go.dokimi.dev/eidos/reference/repogen.KeyRepository]. It requires the
// capabilities of repogen and of mockgen, so it weaves the methods of the
// implementation <T>Repo and of the mock of <T>Repository after both
// plugins placed them. It skips a method without a receiver, because the
// methods of an interface do not render a body:
//
//	// Get implements ProductRepository with zero values.
//	func (p *ProductRepo) Get(ctx context.Context, id string) (*Product, error) {
//		log.Printf("debug: %s entered", "Product.Get")
//		return nil, nil
//	}
//
// The options package, func and format set the call, and a product sets
// them to its own logger.
//
// # Dependency position
//
// The package imports the packages internal/callee, mockgen and repogen of
// the module, and the root, meta, plugin and symbol packages of the SDK
// facade.
package debugweaver
