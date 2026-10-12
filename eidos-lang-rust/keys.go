// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace that every rust key registers under.
const namespace = "rust"

// GeneratedMark is the text that marks the first comment of a file that a
// foreign generator wrote, by the convention of the generators of the
// ecosystem, such as prost-build.
const GeneratedMark = "@generated"

// The classification keys of the satellite, which the frontend stamps.
const (
	// TestKey classifies an item that #[test] or #[cfg(test)] marks, and
	// every file of a tests/ target.
	TestKey meta.KeyName = "rust.test"

	// GeneratedKey records, on a file whose first comment contains
	// [GeneratedMark], that comment as written.
	GeneratedKey meta.KeyName = "rust.generated"

	// CfgKey records, on a file, each cfg predicate outside the set of the
	// load that keeps an item of the file out, as written.
	CfgKey meta.KeyName = "rust.cfg"

	// UnionKey marks the struct that a union declaration declares.
	UnionKey meta.KeyName = "rust.union"

	// VisibilityKey records a visibility restricted to a path as written:
	// pub(super), pub(self) or pub(in path). The visibility of the
	// declaration is Internal, which has no path.
	VisibilityKey meta.KeyName = "rust.visibility"

	// LifetimeParamsKey records the lifetime parameters of a declaration as
	// written, for which the model has no type parameter.
	LifetimeParamsKey meta.KeyName = "rust.lifetimeParams"
)

// Keys claims the rust namespace and registers every rust key, in the form
// that a composition and a corpus fixture declare them. It returns the
// error of the claim, or the errors of every registration joined.
//
// # Allocation contract
//
// Keys allocates the three kind lists of the keys, and the registry
// allocates its namespace claim and the growth of its lists and maps to
// six keys: 13 allocations into a fresh registry.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	file := []symbol.Kind{symbol.KindFile}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestKey,
		Doc:  "marks an item that #[test] or #[cfg(test)] marks, and a file of a tests/ target",
	})
	_, generatedErr := meta.Register[string](r, meta.KeySpec{
		Name: GeneratedKey, Kinds: file,
		Doc: "records the first comment of a file that a foreign generator wrote",
	})
	_, cfgErr := meta.Register[[]string](r, meta.KeySpec{
		Name: CfgKey, Kinds: file,
		Doc: "records the cfg predicates outside the set of the load that keep items of a file out",
	})
	_, unionErr := meta.Register[bool](r, meta.KeySpec{
		Name: UnionKey, Kinds: []symbol.Kind{symbol.KindStruct},
		Doc: "marks the struct that a union declaration declares",
	})
	_, visibilityErr := meta.Register[string](r, meta.KeySpec{
		Name: VisibilityKey,
		Doc:  "records a visibility restricted to a path as written",
	})
	_, lifetimeErr := meta.Register[[]string](r, meta.KeySpec{
		Name: LifetimeParamsKey,
		Kinds: []symbol.Kind{
			symbol.KindStruct, symbol.KindEnum, symbol.KindSum, symbol.KindInterface,
			symbol.KindAlias, symbol.KindFunction, symbol.KindMethod,
		},
		Doc: "records the lifetime parameters of a declaration as written",
	})
	return errors.Join(testErr, generatedErr, cfgErr, unionErr, visibilityErr, lifetimeErr)
}
