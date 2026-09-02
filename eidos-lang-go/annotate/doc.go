// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package annotate stamps what the sealed graph proves about Go
// declarations: the semantic facts the old frontend derived
// through the type checker, re-homed to the phase that can see
// across packages.
//
// [New] builds the annotator through the kit. Per type it stamps
// golang.satisfiesError and golang.satisfiesStringer when the
// workspace-visible method set — the type's own methods, the
// package-level methods that attach to it, and the methods of
// every embed the graph resolves — carries the interface's one
// method; golang.embedsInterface on a struct embedding a type the
// graph holds as an interface; and golang.comparable when every
// field is provably comparable under Go's own rules, workspace
// types recursed and cycles guarded. Each fact stamps only when
// proven: a type reaching outside the workspace stays unstamped,
// because absence is unknown and never a negative.
//
// # Dependency position
//
// lang/go/annotate imports the sdk facade and the satellite root's
// key vocabulary; a workspace composition schedules it after the
// load, and nothing beneath imports it back.
package annotate
