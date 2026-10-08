// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/frontendtest"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tags that open the spellings of edges, pinned. Each grain has a tag
// of one byte, and findingsTag is the whole spelling of the findings
// edge. A name edge's key opens with the tag of its form.
const (
	declarationTag = 'd'
	packageTag     = 'p'
	kindTag        = 'k'
	directiveTag   = 'r'
	factTag        = 'f'
	findingsTag    = 'x'
	exportTag      = 'e'
	nameTag        = 'n'
	scopeTag       = 's'
	directoryTag   = 'v'
	unitTag        = 'u'
	filesTag       = 'w'
	byPackageTag   = 'p'
	byOriginTag    = 'o'
)

// The plan, the directory and the receiver the plan edges spell.
const (
	edgePlan     = "stubs"
	edgeDir      = "svc/api"
	edgeReceiver = "User"
)

// The cases hash the edges of a struct of the API package, of the package
// itself, of a directive's spelling, of a fact's key, and of a unit of the
// plan.
var (
	edgeStruct = symbol.Identity{
		Lang: frontendtest.ScriptedLang, Package: "svc/api", Owner: "Api", Name: "User",
		Kind: symbol.KindStruct, Disc: "1",
	}
	edgePackage   = symbol.Identity{Lang: frontendtest.ScriptedLang, Package: "svc/api", Kind: symbol.KindPackage}
	edgeDirective = directive.Name("shape:mirror")
	edgeKey       = meta.KeyName("shape.role")
	edgeUnit      = plugin.UnitRef{Plugin: "gen", Tag: "test", Pkg: edgePackage, Key: "svc/api/user.zz"}
)

// Each edge hashes to the first eight bytes of the SHA-256 of its grain's
// tag and its fields, each field behind its length, so two runs and two
// builds hash an edge alike, and two grains never meet.
func TestEdge(t *testing.T) {
	t.Parallel()

	t.Run("DeclarationEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the declaration tag and the identity", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledIdentity([]byte{declarationTag}, edgeStruct))
			assert.Equal(t, state.DeclarationEdge(edgeStruct), want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash for another identity", func(t *testing.T) {
			t.Parallel()

			other := edgeStruct
			other.Disc = "2"
			assert.NotEqual(t, state.DeclarationEdge(other), state.DeclarationEdge(edgeStruct),
				"the discriminator is part of the spelling")
		})
	})

	t.Run("PackageEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the package tag and the identity", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledIdentity([]byte{packageTag}, edgePackage))
			assert.Equal(t, state.PackageEdge(edgePackage), want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash than the declaration edge of the same identity", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.PackageEdge(edgePackage), state.DeclarationEdge(edgePackage),
				"the tags keep the two grains apart")
		})
	})

	t.Run("KindEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the kind tag and the kind", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(binary.AppendUvarint([]byte{kindTag}, uint64(symbol.KindStruct)))
			assert.Equal(t, state.KindEdge(symbol.KindStruct), want, "the spelling's SHA-256, first eight bytes")
		})
	})

	t.Run("DirectiveEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the directive tag and the spelling", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledText([]byte{directiveTag}, string(edgeDirective)))
			assert.Equal(t, state.DirectiveEdge(edgeDirective), want, "the spelling's SHA-256, first eight bytes")
		})
	})

	t.Run("FactEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the fact tag, the subject and the key", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledText(spelledIdentity([]byte{factTag}, edgeStruct), string(edgeKey)))
			assert.Equal(t, state.FactEdge(edgeStruct, edgeKey), want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash for another key", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.FactEdge(edgeStruct, "shape.kind"), state.FactEdge(edgeStruct, edgeKey),
				"the key is part of the spelling")
		})
	})

	t.Run("ExportEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged plan", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledText([]byte{exportTag}, edgePlan))
			assert.Equal(t, state.ExportEdge(edgePlan), want, "the spelling's SHA-256, first eight bytes")
		})
	})

	t.Run("NameEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged key of a bare reference", func(t *testing.T) {
			t.Parallel()

			spelling := append(spelledText([]byte{nameTag}, edgePlan), byPackageTag)
			want := pinnedHash(spelledText(spelledText(spelling, edgeDir), "User"))
			got := state.NameEdge(edgePlan, plugin.NameKey{Package: edgeDir, Emitted: "User"})
			assert.Equal(t, got, want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns the hash of the tagged key of a resolved reference", func(t *testing.T) {
			t.Parallel()

			spelling := spelledIdentity(append(spelledText([]byte{nameTag}, edgePlan), byOriginTag), edgeStruct)
			want := pinnedHash(spelledText(spelling, "User"))
			got := state.NameEdge(edgePlan, plugin.NameKey{Origin: edgeStruct, Emitted: "User"})
			assert.Equal(t, got, want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash for another plan", func(t *testing.T) {
			t.Parallel()

			k := plugin.NameKey{Package: edgeDir, Emitted: "User"}
			assert.NotEqual(t, state.NameEdge("mocks", k), state.NameEdge(edgePlan, k),
				"the plan is part of the spelling")
		})
	})

	t.Run("ScopeEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged scope", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledText(spelledText(spelledText([]byte{scopeTag}, edgePlan), edgeDir), edgeReceiver))
			assert.Equal(t, state.ScopeEdge(edgePlan, edgeDir, edgeReceiver), want,
				"the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash for another receiver", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.ScopeEdge(edgePlan, edgeDir, ""), state.ScopeEdge(edgePlan, edgeDir, edgeReceiver),
				"the receiver is part of the spelling")
		})
	})

	t.Run("DirectoryEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged directory", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledText([]byte{directoryTag}, edgeDir))
			assert.Equal(t, state.DirectoryEdge(edgeDir), want, "the spelling's SHA-256, first eight bytes")
		})
	})

	t.Run("FilesEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged package", func(t *testing.T) {
			t.Parallel()

			want := pinnedHash(spelledIdentity([]byte{filesTag}, edgePackage))
			assert.Equal(t, state.FilesEdge(edgePackage), want, "the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash than the package edge", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.FilesEdge(edgePackage), state.PackageEdge(edgePackage),
				"the tag tells the grains apart")
		})
	})

	t.Run("UnitEdge", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the hash of the tagged unit", func(t *testing.T) {
			t.Parallel()

			spelling := spelledText([]byte{unitTag}, edgePlan)
			spelling = spelledText(spelledText(spelling, string(edgeUnit.Plugin)), edgeUnit.Tag)
			spelling = spelledText(spelledIdentity(spelling, edgeUnit.Pkg), edgeUnit.Key)
			assert.Equal(t, state.UnitEdge(edgePlan, edgeUnit), pinnedHash(spelling),
				"the spelling's SHA-256, first eight bytes")
		})

		t.Run("returns another hash for another plan", func(t *testing.T) {
			t.Parallel()

			assert.NotEqual(t, state.UnitEdge("mocks", edgeUnit), state.UnitEdge(edgePlan, edgeUnit),
				"the plan is part of the spelling")
		})
	})
}

// Every edge hashes without allocating in the ordinary run, which runs no
// benchmark: its spelling fits the stack buffer. The check runs alone,
// because the count includes every goroutine's allocations.
func TestEdgeAllocs(t *testing.T) {
	var got state.EdgeHash
	assert.MaxAllocs(t, func() { got = state.DeclarationEdge(edgeStruct) }, 0,
		"DeclarationEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.PackageEdge(edgePackage) }, 0, "PackageEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.KindEdge(symbol.KindStruct) }, 0, "KindEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.DirectiveEdge(edgeDirective) }, 0, "DirectiveEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.FactEdge(edgeStruct, edgeKey) }, 0, "FactEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.ExportEdge(edgePlan) }, 0, "ExportEdge allocates nothing")
	key := plugin.NameKey{Origin: edgeStruct, Emitted: "User"}
	assert.MaxAllocs(t, func() { got = state.NameEdge(edgePlan, key) }, 0, "NameEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.ScopeEdge(edgePlan, edgeDir, edgeReceiver) }, 0,
		"ScopeEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.DirectoryEdge(edgeDir) }, 0, "DirectoryEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.FilesEdge(edgePackage) }, 0, "FilesEdge allocates nothing")
	assert.MaxAllocs(t, func() { got = state.UnitEdge(edgePlan, edgeUnit) }, 0, "UnitEdge allocates nothing")
	assert.NotEqual(t, got, 0, "the last hash is of an edge")
}

// BenchmarkEdge measures the hash of each grain's edge.
func BenchmarkEdge(b *testing.B) {
	key := plugin.NameKey{Origin: edgeStruct, Emitted: "User"}
	benches := []struct {
		name string
		hash func() state.EdgeHash
	}{
		{name: "DeclarationEdge", hash: func() state.EdgeHash { return state.DeclarationEdge(edgeStruct) }},
		{name: "PackageEdge", hash: func() state.EdgeHash { return state.PackageEdge(edgePackage) }},
		{name: "KindEdge", hash: func() state.EdgeHash { return state.KindEdge(symbol.KindStruct) }},
		{name: "DirectiveEdge", hash: func() state.EdgeHash { return state.DirectiveEdge(edgeDirective) }},
		{name: "FactEdge", hash: func() state.EdgeHash { return state.FactEdge(edgeStruct, edgeKey) }},
		{name: "ExportEdge", hash: func() state.EdgeHash { return state.ExportEdge(edgePlan) }},
		{name: "NameEdge", hash: func() state.EdgeHash { return state.NameEdge(edgePlan, key) }},
		{name: "ScopeEdge", hash: func() state.EdgeHash { return state.ScopeEdge(edgePlan, edgeDir, edgeReceiver) }},
		{name: "DirectoryEdge", hash: func() state.EdgeHash { return state.DirectoryEdge(edgeDir) }},
		{name: "FilesEdge", hash: func() state.EdgeHash { return state.FilesEdge(edgePackage) }},
		{name: "UnitEdge", hash: func() state.EdgeHash { return state.UnitEdge(edgePlan, edgeUnit) }},
	}
	for _, bb := range benches {
		b.Run(bb.name, func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got state.EdgeHash
			for c.Loop() {
				got = bb.hash()
			}
			assert.NotEqual(b, got, 0, "the hash is of an edge")
		})
	}
}

// pinnedHash returns the hash the edge format defines for a spelling: the
// first eight bytes of its SHA-256, big-endian.
func pinnedHash(spelling []byte) state.EdgeHash {
	sum := sha256.Sum256(spelling)
	return state.EdgeHash(binary.BigEndian.Uint64(sum[:8]))
}

// spelledText appends a text field as the edge format spells it: its
// length as an unsigned varint, then its bytes.
func spelledText(dst []byte, s string) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(s)))
	return append(dst, s...)
}

// spelledIdentity appends an identity's six fields as the edge format
// spells them: the language, the package, the owner and the name as
// texts, the kind as an unsigned varint, and the discriminator as a text.
func spelledIdentity(dst []byte, id symbol.Identity) []byte {
	for _, part := range []string{string(id.Lang), id.Package, id.Owner, id.Name} {
		dst = spelledText(dst, part)
	}
	dst = binary.AppendUvarint(dst, uint64(id.Kind))
	return spelledText(dst, id.Disc)
}
