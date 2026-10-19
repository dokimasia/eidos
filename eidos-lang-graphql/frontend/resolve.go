// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The kinds that a name of a type of the schema can declare: an object type
// or an input object, an interface, an enum, and a union or a scalar. A
// name in the package of the documents declares the struct or the sum of a
// selection.
var (
	schemaKinds   = [...]symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias}
	documentKinds = [...]symbol.Kind{symbol.KindStruct, symbol.KindSum}
)

// bindings is what the parse of a file records for its resolution: the
// package of the schema of the file, the shared packages of the schema in
// the order of their schemas, and the names of the types that the schema
// declares by an extension alone. A file of the documents has the package
// of its documents too, and an empty documents is a file of the schema.
// Every file of a unit shares the bindings of its kind, and the lowering of
// the extensions of the unit fills extended after the files record them.
type bindings struct {
	documents string
	schema    string
	shared    []string
	extended  map[string]bool
}

// candidates is the result of one resolution while it grows: the tiers, the
// one array that the identities of every tier share, and the name.
type candidates struct {
	out  plugin.Candidates
	ids  []symbol.Identity
	name string
}

// tier appends the tier of the name in one package under one owner, one
// candidate for each kind. The array of identities has the room of every
// tier, so a tier allocates nothing.
func (c *candidates) tier(pkg, owner string, kinds []symbol.Kind) {
	start := len(c.ids)
	for _, k := range kinds {
		c.ids = append(c.ids, symbol.Identity{Lang: graphql.Lang, Package: pkg, Owner: owner, Name: c.name, Kind: k})
	}
	c.out = append(c.out, c.ids[start:len(c.ids):len(c.ids)])
}

// resolve returns what a type spelling can mean where it is written. A
// built-in scalar returns no candidate, and the rules project it. Every
// other name returns one tier per scope, innermost first, so the
// resolution step binds the innermost declaration of a name, and an outer
// one of the same name raises no ambiguity:
//
//   - in a file of the documents, each struct of the chain of selections
//     that contains the reference, innermost first, which declares the
//     structs and the sums of the selections of its fields, and then the
//     package of the documents;
//   - the package of the schema;
//   - each shared package, in the order of [bindings].
//
// A name that the schema declares by an extension alone resolves in the
// shared packages before the package of the schema, so a reference binds
// the type that the extension extends where a shared package declares it.
//
// # Allocation contract
//
// resolve allocates the list of tiers and one array for the candidates of
// every tier, and the joined chain of an owner that a selection nests.
func resolve(scope plugin.ImportScope, spelling string) plugin.Candidates {
	b, recorded := scope.Bindings.(*bindings)
	if !recorded || b == nil || graphql.IsBuiltin(spelling) {
		return nil
	}
	var chain string
	tiers, ids := 1+len(b.shared), len(schemaKinds)*(1+len(b.shared))
	if b.documents != "" {
		chain = chainOf(scope.Owner)
		depth := 1
		if chain != "" {
			depth += strings.Count(chain, symbol.OwnerSep) + 1
		}
		tiers, ids = tiers+depth, ids+depth*len(documentKinds)
	}
	c := candidates{out: make(plugin.Candidates, 0, tiers), ids: make([]symbol.Identity, 0, ids), name: spelling}
	if b.documents != "" {
		for ; chain != ""; chain = enclosing(chain) {
			c.tier(b.documents, chain, documentKinds[:])
		}
		c.tier(b.documents, "", documentKinds[:])
	}
	extended := b.extended[spelling]
	if !extended {
		c.tier(b.schema, "", schemaKinds[:])
	}
	for _, pkg := range b.shared {
		c.tier(pkg, "", schemaKinds[:])
	}
	if extended {
		c.tier(b.schema, "", schemaKinds[:])
	}
	return c.out
}

// chainOf returns the chain of the owners of a reference: the owner and the
// name of the struct or the sum that encloses it, joined, and the empty
// chain at file level.
func chainOf(owner symbol.Identity) string {
	switch {
	case owner.Name == "":
		return ""
	case owner.Owner == "":
		return owner.Name
	default:
		return owner.Owner + symbol.OwnerSep + owner.Name
	}
}

// enclosing returns the chain without its innermost owner, and the empty
// chain for a chain of one owner.
func enclosing(chain string) string {
	before, _, nested := strings.CutLast(chain, symbol.OwnerSep)
	if !nested {
		return ""
	}
	return before
}
