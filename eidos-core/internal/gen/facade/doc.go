// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package facade generates the SDK re-export module from the
// kernel's plugin-facing packages.
//
// [Lower] parses the curated kernel packages and returns their
// exported surfaces. [Generate] renders those surfaces into the
// facade module: one alias, re-declared constant or wrapper
// function per exported symbol, each carrying the original's
// documentation with kernel import paths respelt to facade paths.
// The facade defines nothing of its own, so the kernel stays the
// single home of every definition.
//
// Emission works on syntax alone. The curated surface spells
// types from the assert module in the conformance kits, and a
// type-checking load would need that module resolvable wherever
// generation runs; reprinting the parsed declarations does not.
// Anything syntax cannot re-export faithfully — a dot import, an
// unexported type in an exported signature, a reference to a
// kernel package outside the curated list — is refused at its
// position rather than narrowed silently.
//
// # Dependency position
//
// core/internal/gen/facade imports core/internal/gosource,
// core/internal/genfile and the Go stdlib.
package facade

//go:generate go run go.dokimi.dev/eidos/core/internal/gen/facade/cmd
