// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shapetest builds the fixtures that the tests of the shape
// catalog run the plugins of the catalog over.
//
// A fixture declares callables of the test language [Lang], whose
// [Rules] classify parameters and returns by the spelling of their types,
// so the tests run the detectors without a language satellite. [Method],
// [Function] and [Package] build the declarations. [New] returns a [Run]
// over the packages, which is a plugintest fixture with the rules and the
// keys of the catalog. [Run.Declare] records directive instances,
// [Run.Classify] runs the plugin shape and the plugin shapecheck, and
// [Run.Consume] runs a generator that reads the classifications.
//
// # Dependency position
//
// The package imports the packages shape and catalog, the root, diag,
// directive, node, plugin, plugintest, position, rules, rulestest and
// symbol packages of the SDK facade, and the assert module.
package shapetest
