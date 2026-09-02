// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shape is the module the callable-classification catalog
// takes: the shape namespace, whose stamps let check-generating
// consumers act on a classification instead of re-deriving it.
//
// # Scope
//
// Shape stamps and consumers check. The catalog asserts nothing
// about behavior: it stamps facts at plugin authority, and a
// wrong inference is a one-line directive at the declaration
// rather than a fork.
//
// The classification vocabulary is three forms under disjoint key
// prefixes — one shape per callable, any number of mixins, one
// contract per instance and role — so a Commit method can be
// writer-shaped, hold the commit role of a tx contract, and be
// atomic at once.
//
// The module holds this statement of scope and no code.
package shape
