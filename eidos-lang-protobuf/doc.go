// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf is the module protobuf takes as a source
// language for eidos workspaces.
//
// # Scope
//
// The satellite is read-only by design: a schema language is read
// and never written, so its shape is a frontend and projection
// rules with no lowering, backend or sdk — a complete satellite of
// the read-only shape rather than a partial one.
//
// Protobuf's projection decisions, which the frontend is built
// against:
//
//   - A oneof is the tagged Sum shape, never an untagged union.
//   - A message is a struct, a service an interface, an enum the
//     Enum kind carrying its declared values.
//   - A well-known type keeps its qualified spelling; a reference
//     the workspace does not hold resolves to nothing and degrades
//     visibly, which is the read side's own rule.
//
// The frontend reads bufbuild/protocompile, pinned as a library
// rather than a machine-supplied toolchain, so one workspace
// resolves one parser everywhere.
//
// The module holds this statement of scope and no code.
package protobuf
