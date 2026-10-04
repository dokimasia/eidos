// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf declares protobuf's conformance entry, and its tests
// grade protobuf against the shared feature inventory.
//
// [Corpus] is protobuf's entry: the protobuf frontend and rules over a
// tree of schemas, which spell records, namespaces, services and closed
// value sets. The entry refuses the four features a schema cannot
// state: a method on a record, an overload, a standalone constant and a
// test file.
//
// # Dependency position
//
// protobuf imports the conformance package, core/frontend/frontendtest,
// and the protobuf satellite's root, frontend and rules packages. No
// package imports protobuf but its own tests.
package protobuf
