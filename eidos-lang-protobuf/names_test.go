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

// The well-known types the lookup cases name, and the files that
// declare them.
const (
	timestampName = "google.protobuf.Timestamp"
	timestampFile = "google/protobuf/timestamp.proto"
	valueName     = "google.protobuf.Value"
	structFile    = "google/protobuf/struct.proto"
	wrapperName   = "google.protobuf.StringValue"
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

		t.Run("reports true for every scalar type", func(t *testing.T) {
			t.Parallel()

			for _, spelling := range []string{
				"double", "float", "int32", "int64", "uint32", "uint64", "sint32", "sint64",
				"fixed32", "fixed64", "sfixed32", "sfixed64", "bool", "string", "bytes",
			} {
				assert.True(t, protobuf.IsScalar(spelling), spelling+" is a scalar")
			}
		})

		t.Run("reports false for a spelling outside the scalars", func(t *testing.T) {
			t.Parallel()

			for _, spelling := range []string{"Row", "int"} {
				assert.False(t, protobuf.IsScalar(spelling), spelling+" is a message or another language's builtin")
			}
		})
	})

	t.Run("WellKnown", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the fully-qualified name of a well-known type", func(t *testing.T) {
			t.Parallel()

			name, known := protobuf.WellKnown(timestampName)
			assert.True(t, known, "the qualified spelling names Timestamp")
			assert.Equal(t, name, timestampName, "under its fully-qualified name")
		})

		t.Run("returns the name of a rooted spelling without the leading dot", func(t *testing.T) {
			t.Parallel()

			name, known := protobuf.WellKnown(" ." + wrapperName + " ")
			assert.True(t, known, "the rooted spelling names a wrapper")
			assert.Equal(t, name, wrapperName, "the dot and the whitespace removed")
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

	t.Run("WellKnownImport", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the file that declares a well-known type", func(t *testing.T) {
			t.Parallel()

			file, known := protobuf.WellKnownImport(timestampName)
			assert.True(t, known, "the spelling names Timestamp")
			assert.Equal(t, file, timestampFile, "the import path protoc resolves it under")
		})

		t.Run("returns the file that declares a well-known type spelled rooted", func(t *testing.T) {
			t.Parallel()

			file, known := protobuf.WellKnownImport("." + valueName)
			assert.True(t, known, "the rooted spelling names Value")
			assert.Equal(t, file, structFile, "Value is declared beside Struct")
		})

		t.Run("reports false outside the well-known set", func(t *testing.T) {
			t.Parallel()

			file, known := protobuf.WellKnownImport("google.protobuf.SourceContext")
			assert.False(t, known, "a type the projection does not map")
			assert.Empty(t, file, "names no file")
		})
	})

	t.Run("Candidates", func(t *testing.T) {
		t.Parallel()

		t.Run("returns one tier per scope from the innermost message to the root", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates(namesPkg, namesChain, "Key")
			assert.Length(t, got, 5, "two messages, two namespaces and the root")
			assert.Equal(t, got[0][0], candidate("acme.store.Row.Inner", "", "Key"),
				"the innermost message comes first, the longest package first within it")
			assert.Contains(t, got[0], candidate(namesPkg, namesChain, "Key"),
				"the tier offers the same name as a type nested in the chain")
			assert.Contains(t, got[1], candidate(namesPkg, "Row", "Key"), "then the enclosing message")
			assert.Contains(t, got[2], candidate(namesPkg, "", "Key"), "then the package")
			assert.Contains(t, got[3], candidate("acme", "", "Key"), "then the namespace above it")
			assert.Equal(t, got[4], []symbol.Identity{candidate("", "", "Key")}, "the root last")
		})

		t.Run("offers a dotted spelling at every split of every scope", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates("acme", "", "v1.User")
			assert.Contains(t, got[0], candidate("acme.v1", "", "User"),
				"a sub-package of the file's own namespace is one split")
			assert.Contains(t, got[0], candidate("acme", "v1", "User"),
				"a type nested in a message of the namespace is another")
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

		tests := []struct {
			name string
			give string
		}{
			{name: "returns no candidate for a scalar", give: "int64"},
			{name: "returns no candidate for a well-known type", give: "google.protobuf.Duration"},
			{name: "returns no candidate for a rooted well-known type", give: ".google.protobuf.Duration"},
			{name: "returns no candidate for an empty spelling", give: "  "},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Empty(t, protobuf.Candidates(namesPkg, "", tt.give), "no declaration is asked for")
			})
		}

		t.Run("probes the root alone from a file with no package", func(t *testing.T) {
			t.Parallel()

			got := protobuf.Candidates("", "", "Row")
			assert.Equal(t, got, plugin.Candidates{{candidate("", "", "Row")}},
				"the unnamed namespace is the root")
		})
	})
}
