// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules holds the Go language's projection rules: the
// decisions Go returns inside the kernel's walks.
//
// [New] returns the value a composition registers. It walks embeds
// under promotion, reads a context parameter and an error, ok or
// iterator return by their spellings, classifies the builtins by
// name with time.Time and time.Duration through the well-known
// registry, resolves a directive's spelling the way Go scopes it,
// and derives sample, alternate and zero values from a fixed table
// for builtins and from the declaration for a workspace type.
//
// The value also satisfies the enum, error-value, tag, generics,
// promotion and equality capabilities: an enumeration's texts and
// exact values from the frontend's constValue stamps, the Err
// sentinel convention, struct tags, witnesses for the trivially
// closed bounds and substitution of type arguments, the settable
// members promotion reaches, and Go's comparability rule, which is
// the one the annotator stamps golang.comparable with.
//
// # Dependency position
//
// lang/go/rules imports the sdk facade, lang/naming and the
// satellite root for its keys. It never imports the frontend: the
// rules read the sealed graph and its facts, not source.
package rules
