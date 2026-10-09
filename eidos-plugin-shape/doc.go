// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shape declares the vocabulary of the shape catalog. It names
// every classification that the plugins of the catalog stamp on callables,
// and it reads the classifications back for the plugins that consume them.
//
// The catalog stamps facts, and a consumer acts on them. A generator of
// checks reads the classification of a callable instead of deriving it
// from the signature again. The catalog asserts nothing about behaviour,
// and a directive at the declaration corrects a wrong classification
// without a fork of the catalog.
//
// # Forms
//
// A classification has one of three forms, each under its own keys:
//
//   - A shape describes the signature of a callable, such as [Writer]. A
//     callable has at most one shape. A detector of package detectors
//     classifies callables as a detected shape, and a directive declares
//     any shape.
//   - A mixin states a property of a callable, such as [Atomic]. A
//     callable has any number of mixins.
//   - A contract binds the functions and the methods of one package to
//     the roles of a protocol, such as the roles [TxBegin], [TxCommit] and
//     [TxRollback] of [Tx]. An id separates two instances of one contract
//     in a package, and [InstanceOf] returns the members of an instance.
//
// One Commit method can be a writer, play the commit role of a tx
// contract, and be atomic.
//
// # Specs
//
// One YAML file below spec/ describes each classification, and
// spec.schema.json is its JSON Schema. The generator in the module tools
// writes registry.gen.go of this package and catalog/registry_wiring.gen.go
// from the specs. registry.gen.go declares one constant for each
// classification and each role, one constant for each key of a param and a
// binding, one params struct and one reader for each classification, and
// [Specs].
//
// # Keys
//
// Every key of the catalog is in the namespace shape. The summary keys
// [KeyDetected], [KeyShape], [KeyMixed], [KeyMember] and [KeyClassified]
// mark what a callable has. The family key of a classification is the
// namespace, the name of the spec and the part shape, mixin or role, such
// as shape.writer.shape. A contract also has the key of the id of its
// instance, such as shape.tx.id. A param or a binding has the key of its
// name, such as shape.writer.reads. Every key of a classification is in
// the fact group of the namespace and the name, such as shape.writer, so a
// meta drop of the group removes the classification.
//
// # Consumers
//
//   - [Any], [Is], [Has], [In] and [Plays] return the predicates that gate
//     a rule on a classification.
//   - A reader such as [WriterOf] returns the params of one
//     classification, [Of] returns every classification of a callable,
//     and [InstanceOf] returns a contract instance.
//   - [Specs] and [SpecOf] return the descriptions of the specs.
//
// A predicate and a reader read through a named handle, so a consumer
// registers no key of the catalog. An annotator that reads a
// classification requires [Capability], so the workspace runs it after
// the plugin shape.
//
// A generator of a check for each writer gates its rule on the shape, and
// reads the params of the writer in the handler:
//
//	sdk.NewPlugin("roundtrip").
//		Output(plugin.Output{Per: plugin.PerSource, Word: "roundtrip"}).
//		Handle(sdk.Where(shape.Is(shape.Writer),
//			sdk.OnMethod(func(m *sdk.MethodMatch, e *sdk.Emitter) error {
//				w, _ := shape.WriterOf(m)
//				// w.Value is the parameter of the written value, and
//				// w.Reads is the reader that observes the write.
//				return nil
//			}))).
//		Build()
//
// # Authority
//
// A directive stamps its classification at directive authority, which
// ranks before the plugin authority of a detection. The shape directive
// keeps the detectors off its callable, so a declared shape replaces the
// detected one.
//
// # Dependency position
//
// The package imports the SDK facade's root, directive, meta, node,
// plugin, rules, store and symbol packages.
//
//go:generate go -C tools test . -run TestCompose -update
package shape
