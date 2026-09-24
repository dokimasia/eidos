// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package annotate stamps what the sealed graph proves about Go
// declarations: the semantic facts a type checker derives, stamped
// in the phase that reads across packages.
//
// [New] builds the annotator through the kit. On every struct,
// interface, enumeration and alias it stamps golang.satisfiesError
// and golang.satisfiesStringer when the type's visible method set
// contains the interface's one method. The set is the type's own
// methods, the file-level methods of its package that attach to it,
// and the methods of every embed the graph resolves. It stamps
// golang.embedsInterface on a struct that embeds an interface the
// graph declares, and golang.comparable when the Go rules
// prove the type comparable. A fact stamps only when proven: a type
// that refers outside the workspace is not stamped, because absence
// is unknown and never a negative.
//
// # Dependency position
//
// lang/go/annotate imports the sdk facade, the satellite root's keys
// and lang/go/rules, whose comparability rule the stamp and the
// projection share. A workspace composition schedules it after the
// load, and no package beneath it imports it.
package annotate
