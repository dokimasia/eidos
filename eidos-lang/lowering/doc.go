// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package lowering contains the steps that the Lower hooks of the
// backends share.
//
// [CopyTypeParams], [CopyTypeRefs] and [CopyTypeRef] copy the emit shapes
// that a lowering states again. [ParamArgs] and [NamedPayload] are steps
// of the lowering of a sum: the type parameters as the arguments of a
// reference to the sum, and the check that each field of a payload has a
// name. [Callables] lowers the declared failures of every callable that a
// declaration contains, through a fold of the language, and
// [UniqueMethods] is the check of a host that a language without
// overloads makes first. [DeleteAccessor] refuses the deleter of a
// property in every language that has none.
//
// # Allocation contract
//
// Every copy allocates new nodes and shares nothing with its input, so
// the walks of the settle visit the tree of each output once. For an
// empty input, a copy returns nil and not an empty slice. Each function
// states its count.
//
// # Dependency position
//
// The package imports the emit model and the symbol vocabulary of the
// sdk, and the Go stdlib.
package lowering
