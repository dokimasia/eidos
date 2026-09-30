// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the Go language's projection rules: the
// decisions Go makes inside the kernel's walks.
//
// [New] returns the value a composition registers. It walks embeds
// under promotion, reads a context parameter and an error, ok or
// iterator return, classifies the builtins by name with time.Time
// and time.Duration through the well-known registry, and resolves a
// directive's spelling through [golang.Scope], the probe the load's
// resolution step uses. A predeclared type classifies by its
// spelling, and a standard library type by the package its
// reference's import names and its name, so a reference through an
// aliased import classifies alike. It derives sample, alternate and
// zero values from a fixed table for builtins, each number at its
// builtin's width, and from the declaration for a workspace type. It
// lifts a Go literal into a value of its type as canonical decimal
// text, refusing a value outside the type.
//
// The value also satisfies the enum, error-value, tag, generics,
// promotion and equality capabilities: an enumeration's texts and
// exact values from the frontend's constValue stamps, the Err
// sentinel convention, struct tags, a witness for every bound int
// satisfies, read off the bound's declaration where the view
// contains one, substitution of type arguments, the settable members
// promotion adds, and Go's comparability rule, which is the one the
// annotator stamps golang.comparable with.
//
// # Dependency position
//
// lang/go/rules imports the sdk facade, lang/naming, lang/numeric,
// the satellite root for its keys, names and import scope, and the Go
// stdlib, go/constant, go/scanner and go/token among it. It never
// imports the frontend: the rules read the sealed graph and its
// facts, not source.
package rules
