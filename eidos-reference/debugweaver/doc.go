// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package debugweaver logs the entry of each method that the plan emits
// from a repository.
//
// debugweaver selects the emitted structs whose origin repogen stamps with
// [go.dokimi.dev/eidos/reference/repogen.KeyRepository], and weaves each of
// their methods. It runs after the capabilities of repogen and of mockgen
// where the composition contains them, so it weaves the methods of the
// implementation <T>Repo and of the mock of <T>Repository after both
// plugins placed them. It triggers on the struct, because the methods of a
// struct render bodies, and the methods of an interface do not:
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
// the module, and the root, meta and plugin packages of the SDK facade.
package debugweaver
