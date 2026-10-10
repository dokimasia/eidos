// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package java declares Java's conformance entry, and its tests grade
// Java against the shared feature inventory.
//
// [Corpus] is Java's entry: the Java frontend over a tree that spells
// every inventory feature but a constant outside a type, which Java
// does not have.
//
// # Dependency position
//
// java imports the conformance package, core/frontend/frontendtest,
// core/symbol and the Java satellite's frontend. No package imports
// java but its own tests.
package java
