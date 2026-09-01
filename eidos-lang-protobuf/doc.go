// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf makes protobuf a source language for eidos
// workspaces.
//
// The module is read-only by design: a schema language is read,
// never written, so it provides a frontend and projection rules
// and no lowering, backend, or sdk — a complete satellite of the
// read-only shape, not a partial one.
//
// # Projection facts
//
//   - oneof projects onto the tagged Sum shape, never the
//     untagged Union.
//   - Messages project as structs, services as interfaces, enums
//     onto the Enum kind with their declared values.
//   - Well-known types (Timestamp, Duration) map into the type
//     hub's blessed references, shipped as policy defaults so
//     contested spellings (int64 width classes) resolve per
//     workspace.
//
// # Parsing
//
// The frontend parses with the pinned bufbuild/protocompile
// library — pure Go, declarative, never a machine-supplied
// toolchain — so the same workspace resolves the same parser
// everywhere.
package protobuf
