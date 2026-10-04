// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package typescript declares TypeScript's conformance entry, and its
// tests grade TypeScript against the shared feature inventory.
//
// [Corpus] is TypeScript's entry: the TypeScript frontend over a tree
// that spells every inventory feature, one module per feature root and
// sibling package.
//
// # Dependency position
//
// typescript imports the conformance package,
// core/frontend/frontendtest, core/symbol and the TypeScript
// satellite's frontend. No package imports typescript but its own
// tests.
package typescript
