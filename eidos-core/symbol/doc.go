// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

// Package symbol is the declaration vocabulary shared by the node
// and emit models.
//
// [Kind] discriminates the declaration kinds, one constant per
// schema struct. [Identity] names a declaration across runs,
// machines and reparses; it is the join that read sets, exports,
// manifests, drift and explain key on. [Symbol], [Membered] and
// [Typed] are the walk interfaces neutral code is written against.
// [Visibility], [Level] and [Variance] are the normalized enums the
// kinds carry, and [Lang] is the typed source-language name.
//
// # Dependency position
//
// core/symbol imports only core/position and the Go stdlib.
package symbol
