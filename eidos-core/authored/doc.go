// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package authored contains the kernel's two authored-value
// annotators: the handlers of the sample and the witness directives,
// which stamp what an author stated onto the kernel's own keys before
// any derivation runs.
//
// [Sample] reads a sample instance on a declaration of one type and
// stamps its value and its alternate as text in the source language.
// It refuses an instance that states neither under [EmptySample].
// [Witness] reads a witness instance on a generic declaration and
// stamps the resolved type onto each named type parameter. It refuses
// a key that names none of the parameters of the declaration under
// [UnknownWitnessParam], and an instance without a key under
// [EmptyWitness].
//
// Both are ordinary annotators built on the authoring API, and a
// composition lists them like any other. The kernel's workspace
// registers the two schemas and the three keys. It registers no
// plugin for a composition, so a composition that wants authored
// values lists these two.
//
// # Dependency position
//
// core/authored imports the authoring root, core/diag,
// core/directive, core/meta, core/node, core/plugin, core/position
// and the Go stdlib. It is imported by compositions, never by the
// kernel.
package authored
