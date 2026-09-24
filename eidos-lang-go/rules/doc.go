// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the Go language's projection rules: the
// decisions Go makes inside the kernel's walks.
//
// [New] returns the value a composition registers. It walks embeds
// under promotion, reads a context parameter and an error, ok or
// iterator return by their spellings, classifies the builtins by
// name with time.Time and time.Duration through the well-known
// registry, and resolves a directive's spelling the way Go scopes
// it. It derives sample, alternate and zero values from a fixed
// table for builtins, each number at its builtin's width, and from
// the declaration for a workspace type. It lifts a Go literal into a
// value of its type as canonical decimal text, refusing a value
// outside the type.
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
// lang/go/rules imports the sdk facade, lang/naming, the satellite
// root for its keys and names, and the Go stdlib, go/constant and
// go/scanner among it. It never imports the frontend: the rules read
// the sealed graph and its facts, not source.
package rules
