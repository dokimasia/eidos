// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package lowering holds what the backends' Lower hooks share: the
// emit-shape copies a fold restates — [CopyTypeParams],
// [CopyTypeRefs], [CopyTypeRef] — and [UniqueMethods], the member
// check a language without overloads makes before it spells.
//
// # Allocation contract
//
// Every copy allocates fresh nodes and shares nothing with its
// input, so the settle's walks visit each output's tree once; an
// empty input returns nil rather than an empty slice.
//
// # Dependency position
//
// The package imports the sdk's emit model and nothing else of the
// framework.
package lowering
