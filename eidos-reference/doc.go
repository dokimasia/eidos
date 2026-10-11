// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package reference composes the reference plugins of eidos into the
// composition of the binary eidos-reference.
//
// The plugins of the module form the ensemble, the canary of the ring of
// eidos. Each plugin imports the SDK facade, as a plugin outside the
// repository does. Together the plugins use each way in which one plugin
// builds on another: a host with named slots, contributors that fill the
// slots, weavers, annotators whose facts other plugins read, plugins with
// an annotator half and a generator half, claimed file templates, test
// families and options. A change of the facade that breaks such a
// composition fails the tests of the module.
//
// # Composition
//
// [Base] returns the composition without its plans: the brand [Brand], the
// Go and the TypeScript frontends and rules, the targets of both
// languages, the annotators shapewriter and defaults, and dispatch on
// [Workers] workers. [Plans] returns two plans. The plan [GoPlan] runs
// every generator of the ensemble toward the Go backend. The plan
// [TypeScriptPlan] runs mockgen and stubgen over the TypeScript sources
// under web. [Compose] returns Base with Plans. Each call returns new plugin
// instances.
//
// handlergen, repogen and sentinel have an annotator half. The workspace
// schedules a generator with an annotator half as an annotator too, so the
// plan go lists each of the three once.
//
// # The tree
//
// testdata/tree is a workspace that sets each directive of the ensemble.
// testdata/want contains every file that the two plans write from the tree.
//
// # Dependency position
//
// The package imports core/workspace, the root, backend, frontend and rules
// packages of the Go and the TypeScript satellites, the plugin packages of
// the module, and the output and plugin packages of the SDK facade.
package reference
