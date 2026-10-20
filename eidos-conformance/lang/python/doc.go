// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package python declares the conformance entry of Python and the shop
// fixture. Its tests grade Python against the shared feature inventory, run
// the shop fixture through the frame of the workspace, and mirror the shop
// through the Python backend.
//
// [Corpus] is the entry of Python: the Python frontend and rules over a
// tree of modules, which spell classes, methods with overloads, constants,
// Protocols, enums, carriers and markers. The tree is one project of a
// pyproject.toml at its root, so the package of a module is its dotted name
// with slashes.
//
// # The shop fixture
//
// [ComposeShop] and [ShopPlans] compose one workspace over a project of
// uv_build, whose modules are below the source root src. The modules state
// the constructs of Python: dataclasses, a NamedTuple, TypedDicts, enums,
// Protocols, an abstract class, generic classes in both syntaxes, aliases,
// overloads, the passing of parameters, properties, asynchronous functions,
// a lazy import, a check of the version, a stub file beside its source and a
// stub file alone. The plans write three families of files from one graph:
//
//   - [MirrorPlan] restates each declaration through the stamps of its
//     source and writes the mirror of each file beside it. Each source is in
//     the layout of the Python backend, so each written file equals its
//     source between the frame of the output contract, without its carriers.
//   - [ModelPlan] writes the Go structs of the dataclasses, the NamedTuples
//     and the TypedDicts, the Go interfaces of the Protocols, and the Go
//     enums of the enums.
//   - [ClientPlan] writes the TypeScript interfaces, enums and aliases of the
//     same declarations.
//
// [EditShop] is the edit of the fixture, which the warm checks apply.
//
// # Dependency position
//
// python imports the conformance package, core/frontend/frontendtest,
// core/layout, core/ledger, core/symbol, core/workspace, the root and the
// backend, frontend and rules packages of the Python satellite, the root and
// the backend of the Go and the TypeScript satellites, the sdk facade and the
// Go stdlib. No package imports python but its own tests and the matrix.
package python
