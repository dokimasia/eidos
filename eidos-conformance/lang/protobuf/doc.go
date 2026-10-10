// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf declares protobuf's conformance entry and the service
// fixture. Its tests grade protobuf against the shared feature inventory
// and run the fixture in every protobuf version.
//
// [Corpus] is protobuf's entry: the protobuf frontend and rules over a
// tree of schemas, which spell records, namespaces, services and closed
// value sets. The entry refuses the four features a schema cannot
// state: a method on a record, an overload, a standalone constant and a
// test file.
//
// # The service fixture
//
// [ComposeService] and [ServicePlans] compose one workspace over a proto
// service, which writes Go server scaffolding and TypeScript client types
// from one graph:
//
//   - The plan go-server writes svc/store_server.go: a struct for each
//     message, a sum for each oneof, an enum for each enum and the
//     interface StoreServer.
//   - The plan ts-client writes svc/store.ts: an interface for each
//     message, a union for each oneof, an enum for each enum and the
//     interface StoreClient.
//
// The schema exists in proto2, proto3 and the editions 2023, 2024 and
// 2026, and every version writes the same two files. A nested declaration
// has its flat name in both files. [EditService] is the fixture's edit,
// which the warm checks apply.
//
// # Dependency position
//
// protobuf imports the conformance package, core/frontend/frontendtest,
// core/layout, core/ledger, core/workspace, the protobuf satellite's
// frontend and rules packages, the root and the backend of the Go and the
// TypeScript satellites, the sdk facade and the Go stdlib. No package
// imports protobuf but its own tests.
package protobuf
