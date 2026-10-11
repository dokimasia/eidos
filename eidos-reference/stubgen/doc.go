// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package stubgen generates a stub of each interface under the directive
// stub, and a test of each stub.
//
// A stub records the arguments of each call. It has a function field and a
// list of call records for each method. Each method appends a record of
// its arguments to its list, and then calls its field:
//
//	// StoreStub is a stub of Store. Each method records its arguments and calls its function field.
//	type StoreStub struct {
//		// GetFunc is the function that Get calls.
//		GetFunc func(ctx context.Context, id string) (*Product, error)
//		// GetCalls contains the arguments of each call of Get, in order.
//		GetCalls []StoreGetCall
//	}
//
// The model has no neutral spelling of a function type, and the field is
// optional in TypeScript and required in Go. So the family [Word] claims
// its files through the template stub.tmpl of each target, and each stub
// is a record of a file. The template writes the fields, the call records
// and the methods of the target.
//
// The family [Word] under the tag [TestTag] claims the test files through
// the template stub.test.tmpl. In Go, the file checks that the stub
// implements its interface, and tests each method. In TypeScript, the file
// assigns a new stub to a constant of the interface's type, and tsc checks
// the assignment. The TypeScript template imports the stub from the module
// of the same name without the tag, so a config that routes the two
// families apart breaks the import.
//
// # Diagnostics
//
// stubgen writes no stub of an interface without a double. The package
// internal/double reports why.
//
// # Dependency position
//
// The package imports the packages internal/double and internal/tree of
// the module, the root, directive, emit and plugin packages of the SDK
// facade, and the Go stdlib. It serves Go and TypeScript, so it imports no
// satellite.
package stubgen
