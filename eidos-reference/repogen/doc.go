// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package repogen generates a repository and an implementation of it for
// each struct under the directive repo.
//
// repogen is the host of the repository set of the ensemble. Its annotator
// half stamps [KeyRepository] on each struct under //+reference:repo, and
// mockgen and debugweaver select their subjects through the fact. Its
// generator half writes, into the family [Word] beside the source of the
// struct, the interface <T>Repository and the struct <T>Repo:
//
//	// Get implements ProductRepository with zero values.
//	func (p *ProductRepo) Get(ctx context.Context, id string) (*Product, error) {
//		return nil, nil
//	}
//
// A return of the scaffolding vocabulary writes one value, and Get returns
// two, so the body of each method of the implementation is the template
// repo.tmpl. The template places the slots of the body, into which a weaver
// appends, and returns the zero values of the results.
//
// # Dependency position
//
// The package imports the root of the Go satellite, and the root,
// directive, emit, meta, node, plugin and symbol packages of the SDK
// facade.
package repogen
