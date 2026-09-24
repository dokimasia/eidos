// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package protobuf_test

import (
	"testing"

	"go.dokimi.dev/assert"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The package and message chain the candidate cases resolve from.
const (
	namesPkg   = "acme.store"
	namesChain = "Row.Inner"
)

// candidate returns an identity the candidates name, with no kind.
func candidate(pkg, owner, name string) symbol.Identity {
	return symbol.Identity{Lang: protobuf.Lang, Package: pkg, Owner: owner, Name: name}
}

// The name grammar is what the frontend and the rules both resolve
// through, so its tiers, its splits and its exclusions are pinned.
func TestNames(t *testing.T) {
	t.Parallel()

	t.Run("IsScalar", func(t *testing.T) {
		t.Parallel()

		t.Run("reports the fifteen scalar types and nothing else", func(t *testing.T) {
			t.Parallel()

			for _, spelling := range []string{
				"double", "float", "int32", "int64", "uint32", "uint64", "sint32", "sint64",
				"fixed32", "fixed64", "sfixed32", "sfixed64", "bool", "string", "bytes",
			} {
				assert.True(t, protobuf.IsScalar(spelling), spelling+" is a scalar")
			}
			assert.False(t, protobuf.IsScalar("Row"), "a message is not")
			assert.False(t, protobuf.IsScalar("int"), "and neither is another language's builtin")
		})
	})

	t.Run("WellKnown", func(t *testing.T) {
		t.Parallel()

		t.Run("names a well-known type with or without the leading dot", func(t *testing.T) {
			t.Parallel()

			name, known := protobuf.WellKnown("google.protobuf.Timestamp")
			assert.True(t, known, "the qualified spelling names Timestamp")
			assert.Equal(t, name, "google.protobuf.Timestamp", "under its fully-qualified name")
			name, known = protobuf.WellKnown(" .google.protobuf.StringValue ")
			assert.True(t, known, "the rooted spelling names a wrapper")
			assert.Equal(t, name, "google.protobuf.StringValue", "without the leading dot")
		})

		t.Run("reports false outside the well-known set", func(t *testing.T) {
			t.Parallel()

			for _, spelling := range []string{
				"google.protobuf.SourceContext", "google.api.Http", "Timestamp", "acme.Timestamp",
			} {
				_, known := protobuf.WellKnown(spelling)
				assert.False(t, known, spelling+" is not a well-known type the projection maps")
			}
		})
	})

	t.Run("Candidates", func(t *testing.T) {
		t.Parallel()

		t.Run("probes the message chain, the package and the root, one tier each", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates(namesPkg, namesChain, "Key")
			assert.Length(t, got, 5, "two messages, two namespaces and the root")
			assert.Equal(t, got[0][0], candidate("acme.store.Row.Inner", "", "Key"),
				"the innermost message comes first, the longest package first within it")
			assert.Contains(t, got[0], candidate(namesPkg, namesChain, "Key"),
				"and the tier offers the same name as a type nested in the chain")
			assert.Contains(t, got[1], candidate(namesPkg, "Row", "Key"), "then the enclosing message")
			assert.Contains(t, got[2], candidate(namesPkg, "", "Key"), "then the package")
			assert.Contains(t, got[3], candidate("acme", "", "Key"), "then the namespace above it")
			assert.Equal(t, got[4], []symbol.Identity{candidate("", "", "Key")}, "and the root last")
		})

		t.Run("offers a dotted spelling at every split of every scope", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates("acme", "", "v1.User")
			assert.Contains(t, got[0], candidate("acme.v1", "", "User"),
				"a sub-package of the file's own namespace is one split")
			assert.Contains(t, got[0], candidate("acme", "v1", "User"),
				"and a type nested in a message of the namespace another")
			assert.Contains(t, got[1], candidate("v1", "", "User"), "the root tier offers the package reading too")
		})

		t.Run("returns one tier for a rooted spelling", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates(namesPkg, namesChain, ".dep.Target.Inner")
			assert.Length(t, got, 1, "a leading dot probes nothing outward")
			assert.Equal(t, got[0], []symbol.Identity{
				candidate("dep.Target", "", "Inner"),
				candidate("dep", "Target", "Inner"),
				candidate("", "dep.Target", "Inner"),
			}, "every split of the path, the longest package first")
		})

		t.Run("returns no candidate for what no declaration declares", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, protobuf.Candidates(namesPkg, "", "int64"), "a scalar")
			assert.Empty(t, protobuf.Candidates(namesPkg, "", "google.protobuf.Duration"), "a well-known type")
			assert.Empty(t, protobuf.Candidates(namesPkg, "", ".google.protobuf.Duration"),
				"rooted or not")
			assert.Empty(t, protobuf.Candidates(namesPkg, "", "  "), "and an empty spelling")
		})

		t.Run("probes the root alone from a file with no package", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates("", "", "Row")
			assert.Equal(t, got, plugin.Candidates{{candidate("", "", "Row")}},
				"the unnamed namespace is the root")
		})
	})
}
