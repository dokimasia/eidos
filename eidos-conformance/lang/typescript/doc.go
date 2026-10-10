// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package typescript declares TypeScript's conformance entry and
// fixtures, and its tests grade TypeScript against the shared feature
// inventory and the pipeline suite of the kernel.
//
// # Fixtures
//
//   - [Corpus] is TypeScript's entry: the TypeScript frontend and rules
//     over a tree that spells every inventory feature, one module per
//     feature root and sibling package.
//   - [ComposeStubs] composes the TypeScript pipeline fixture. stubgen
//     doubles an interface under // +acme:stub tag=test as a class that
//     implements it, and the TypeScript backend writes the class beside
//     its source with an import of the interface from './store'.
//   - [ComposeMirror] composes a Go plan over TypeScript source, whose
//     generator mirrors each interface as a Go struct. A TypeScript union
//     has no Go spelling, so a property of a union reports RefusedType.
//
// # Dependency position
//
// typescript imports the conformance package, core/frontend/frontendtest,
// core/symbol, core/workspace, core/layout and core/ledger, the root,
// frontend, rules and backend packages of the TypeScript satellite, the
// root and backend packages of the Go satellite, and the SDK facade. Its
// tests also import the assert module, sdk/toolchain, the toolchain
// adapter of the TypeScript satellite and the pipeline kit of the
// kernel, core/workspace/pipelinetest. No package imports typescript but
// its own tests.
package typescript
