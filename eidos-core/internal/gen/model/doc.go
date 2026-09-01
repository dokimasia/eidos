// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package model generates the node and emit declaration models from
// the symbol schema.
//
// [Lower] parses and type-checks one schema directory and returns
// its [KindSpec] list, refusing a schema that breaks the annotation
// contract. [Generate] renders that list into the committed models,
// their traversal, rewiring and codecs, and the Kind constants.
//
// The package imports no eidos model code. It reads the schema by
// parsing it, so the kernel needs no language satellite to build
// itself, and a first run works in a tree holding no generated file
// yet.
//
// # The annotation contract
//
// Every schema field carries an `eidos` tag opening with a side and
// followed by tokens from a closed set; see [TagKey]. Lowering
// refuses an unknown token, a slot or owner tag on a field that is
// not a slice of kinds, a walk tag on a field that is neither a
// kind nor the marker, an owner slice whose element declares no
// host, a walk-tagged host, a duplicate slot name within one kind,
// and any declaration that is neither an exported struct nor the
// marker. Every refusal names the schema position.
//
// # Dependency position
//
// core/internal/gen/model imports core/internal/gosource,
// core/internal/genfile and the Go stdlib.
package model
