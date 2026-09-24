// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package model generates the node and emit declaration models from
// the symbol schema.
//
// [Lower] parses and type-checks one schema directory and returns
// its [KindSpec] list, refusing a schema that breaks the annotation
// contract. [Generate] renders that list into the committed models,
// their traversal, name respelling, slot accessors and codecs, the
// match constructors, the fact vocabulary and the Kind constants.
//
// The package imports no eidos model code. It reads the schema by
// parsing it, so the kernel needs no language satellite to build
// itself. A first run works in a tree without any generated file.
//
// # The annotation contract
//
// A schema field enters the models through a tag under [TagKey]
// that opens with a side and continues with tokens from a closed
// set. A field without the tag is skipped.
//
// Lowering refuses an unknown token, a slot tag on a field that is
// not a slice of kinds, a walk tag on a field that is neither a
// kind nor the marker, a duplicate slot name within one kind, a
// tagged embedded field, a field typed by a form the models do not
// spell, and any declaration that is neither an exported struct nor
// the marker. Every refusal names the schema position.
//
// # Dependency position
//
// core/internal/gen/model imports core/internal/gosource,
// core/internal/genfile and the Go stdlib.
package model
