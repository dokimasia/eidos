// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package annotate stamps what the sealed graph proves about Go
// declarations: the semantic facts a type checker derives, stamped
// in the phase that reads across packages.
//
// [New] builds the annotator through the kit, under the plugin identity
// golang.AnnotatorName. It stamps through the keys that the Go frontend
// registers, so a composition with the annotator contains the Go
// frontend. On every struct, interface, enumeration and alias it stamps
// golang.satisfiesError and golang.satisfiesStringer when the method set
// of the type contains the one method of the interface. The method set
// is what the Go rules return: the methods that the member walk keeps,
// which promotes the methods of each embed that the graph resolves under
// the shadowing rule of Go, and the file-level methods of the package of
// the type that attach to it. Two promoted methods of one name at one
// depth cancel each other. It stamps golang.embedsInterface on a struct
// that embeds an interface the graph declares, and golang.comparable when
// the Go rules prove the type comparable. A fact stamps only when proven:
// a type that refers outside the workspace is not stamped, because absence
// is unknown and never a negative.
//
// # Dependency position
//
// lang/go/annotate imports the sdk facade, the satellite root's keys
// and lang/go/rules, whose method set, method predicate and comparability
// rule the stamps and the projections share. A workspace composition
// schedules it after the load, and no package beneath it imports it.
package annotate
