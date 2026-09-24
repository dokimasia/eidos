// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shape is the eidos module for the callable-classification
// catalog: the shape namespace, whose stamps let a check-generating
// consumer act on a classification without deriving it again.
//
// # Scope
//
// Shape stamps facts, and consumers check them. The catalog asserts
// nothing about behavior. It stamps facts at plugin authority, and a
// one-line directive at the declaration corrects a wrong
// classification without a fork.
//
// The classification vocabulary has three forms under disjoint key
// prefixes: one shape per callable, any number of mixins, and one
// contract per instance and role. A Commit method can be
// writer-shaped, fill the commit role of a tx contract, and be atomic
// at the same time.
//
// The module contains this statement of scope and no code.
//
// # Dependency position
//
// The package imports nothing. Its test imports the assert module.
package shape
