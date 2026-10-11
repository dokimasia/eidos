// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package mockgen generates a mock of each interface under the directive
// mock, and of each interface that the plan emits.
//
// mockgen has two rules. The rule of the directive mocks an interface of
// the source under //+reference:mock. The rule over the emitted
// interfaces is bare, so it mocks every interface that an earlier plugin
// of the plan emits, such as a repository of repogen. mockgen requires the
// capability of repogen for that reason. The kernel directive skip
// plugin=mockgen on the origin of an emitted interface excludes the
// interface.
//
// A mock has a function field for each method, and each method of the mock
// calls its field:
//
//	// StoreMock is a mock of Store. Each method calls its function field.
//	type StoreMock struct {
//		// GetFunc is the function that Get calls.
//		GetFunc func(ctx context.Context, id string) (*Product, error)
//	}
//
// The model has no neutral spelling of a function type, and the field is
// optional in TypeScript and required in Go. So the family [Word] claims
// its files through the template mock.tmpl of each target, and each mock
// is a record of a file. The template writes the fields and the calls of
// the target. It places the body of each method, so a weaver that appends
// into the prologue of a method of a mock appends into the file. The tag
// [TestTag] routes the family into the test files of the target.
//
// # Dependency position
//
// The package imports the packages internal/double, internal/tree and
// repogen of the module, and the root, directive, emit, plugin and symbol
// packages of the SDK facade. It serves Go and TypeScript, so it imports
// no satellite.
package mockgen
