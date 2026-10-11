// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package shapewriter stamps the fact shapewriter.writer on each struct
// that writes bytes, as an io.Writer does.
//
// shapewriter is an annotator with two rules. The bare rule detects a
// struct that declares the method Write of a writer. The method takes one
// parameter of bytes that is not variadic, and returns a count of the
// platform's width and an error. The rules of the language of the struct project the signature, so
// the rule reads the shapes of the types and not their spellings. A method
// that an embedded field promotes does not count.
//
// The directive writer overrides the detection in both directions.
// //+reference:writer stamps the fact on a struct without such a method,
// and //-reference:writer excludes a struct from the bare rule. The fact
// store reads the absence of a fact as false, so shapewriter stamps true
// alone.
//
// # Facts
//
// [KeyWriter] is a bool on a struct. A plugin that reads the writers gates
// on it through meta.Named.
//
// # Dependency position
//
// The package imports the root, directive, meta, plugin, rules and symbol
// packages of the SDK facade.
package shapewriter
