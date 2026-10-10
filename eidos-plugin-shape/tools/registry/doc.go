// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package registry generates the registry of the shape catalog from the
// specs that package specfront loads.
//
// [New] returns the generator. It writes the registry into package shape,
// the root package of the catalog module at [CatalogPath], and the wiring
// into package catalog. A test of the tools module runs the generator and
// compares its files with the files of the catalog module, so a spec
// without a regenerated registry fails the test.
//
// # The generated declarations
//
// For each spec, package shape gains the constant of its name, typed by
// its form, and the constants of its param keys, binding keys and roles.
// It also gains a params struct with one field for each binding and each
// param, a reader such as WriterOf, and the unexported handles of the
// keys of the spec. Specs returns the description of every spec, which the
// plugins of package catalog read to register their keys and their
// directives. Package catalog gains Detections, which refers to the
// detector of each detected shape in package detectors, so a detected spec
// without a detector does not compile.
//
// # Identifiers
//
// A name of a spec, a param, a binding or a role is lowercase words joined
// by hyphens. The generator joins the words in Pascal case for an exported
// identifier, with each initialism in capitals, and in camel case for an
// unexported one. The Go backend spells a declaration the same way, so its
// settle keeps every identifier. Two specs that give one identifier are an
// Error.
//
// # Dependency position
//
// The package imports package specfront, the root, emit, meta, node,
// plugin, position and symbol packages of the SDK facade, and the naming
// package of eidos-lang.
package registry
