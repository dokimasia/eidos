// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rust

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace every rust key registers under.
const namespace = "rust"

// The satellite's classification keys, which the frontend stamps.
const (
	// TestKey classifies an item #[test] or #[cfg(test)] marks, and
	// every file of a tests/ target.
	TestKey meta.KeyName = "rust.test"

	// CfgKey records on a file each cfg predicate, as written, that
	// falls outside the load's set and keeps an item of the file out.
	CfgKey meta.KeyName = "rust.cfg"

	// UnionKey marks the struct a union declaration declares.
	UnionKey meta.KeyName = "rust.union"

	// VisibilityKey records a visibility restricted to a path, as
	// written: pub(super), pub(self) or pub(in path). The declaration's
	// visibility is Internal, which states no path.
	VisibilityKey meta.KeyName = "rust.visibility"

	// LifetimeParamsKey records a declaration's lifetime parameters as
	// written, for which the model has no type parameter.
	LifetimeParamsKey meta.KeyName = "rust.lifetimeParams"
)

// Keys claims the rust namespace and registers every rust key, in the
// shape a composition and a corpus fixture declare them. It returns
// the claim's error, or every registration's error joined.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestKey,
		Doc:  "marks an item #[test] or #[cfg(test)] marks, and a file of a tests/ target",
	})
	_, cfgErr := meta.Register[[]string](r, meta.KeySpec{
		Name: CfgKey, Kinds: []symbol.Kind{symbol.KindFile},
		Doc: "records the cfg predicates outside the load's set that keep items of a file out",
	})
	_, unionErr := meta.Register[bool](r, meta.KeySpec{
		Name: UnionKey, Kinds: []symbol.Kind{symbol.KindStruct},
		Doc: "marks the struct a union declaration declares",
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
		Doc: "records a declaration's lifetime parameters as written",
	})
	return errors.Join(testErr, cfgErr, unionErr, visibilityErr, lifetimeErr)
}
