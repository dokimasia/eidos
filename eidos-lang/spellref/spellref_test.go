// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The brackets a language writes arguments in, the stand-in an absent
// spelling takes, and the names a qualify renames.
const (
	opener  = "<"
	closer  = ">"
	standIn = "?"
	// rowName is the name the fixture's qualify renames to renamed.
	rowName = "Row"
	renamed = "Row2"
	// lang and otherLang are the backend's language and another.
	lang      symbol.Lang = "typescript"
	otherLang symbol.Lang = "golang"
	// modulePkg and targetPkg are the packages a reference records and
	// its target declares.
	modulePkg = "rows"
	targetPkg = "svc/rows"
)

// spelledAllocs is the spelling of a reference with arguments: one
// buffer sized to the result.
const spelledAllocs = 1

// One walk serves every bracket pair, so the recursion, the stand-in
// and the composite rule are pinned once.
func TestSpellRef(t *testing.T) {
	t.Parallel()

	t.Run("Spell", func(t *testing.T) {
		t.Parallel()

		t.Run("writes arguments recursively inside the given brackets", func(t *testing.T) {
			t.Parallel()

			ref := generic("Map", named("K"), generic("List", named("V")))
			assert.Equal(t, spellref.Spell(ref, opener, closer, standIn), "Map<K, List<V>>",
				"each argument list in the brackets")
			assert.Equal(t, spellref.Spell(ref, "[", "]", standIn), "Map[K, List[V]]",
				"whichever brackets the language states")
		})

		t.Run("writes the stand-in for a missing reference", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellref.Spell(nil, opener, closer, standIn), standIn, "the language's stand-in")
		})

		t.Run("writes the stand-in for a reference that spells nothing", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellref.Spell(&emit.TypeRef{}, opener, closer, standIn), standIn,
				"the language's stand-in")
		})

		t.Run("writes the stand-in for an argument that spells nothing", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, spellref.Spell(generic("Map", nil, named("V")), opener, closer, standIn), "Map<?, V>",
				"the stand-in inside the brackets")
		})
	})

	t.Run("SpellWith", func(t *testing.T) {
		t.Parallel()

		t.Run("writes a named reference through qualify", func(t *testing.T) {
			t.Parallel()

			got, err := spellref.SpellWith(named(rowName), opener, closer, standIn, renaming)
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, got, renamed, "the name qualify returns")
		})

		t.Run("writes each argument through qualify", func(t *testing.T) {
			t.Parallel()

			got, err := spellref.SpellWith(generic("List", named(rowName)), opener, closer, standIn, renaming)
			assert.NoError(t, err, "the reference spells")
			assert.Equal(t, got, "List<"+renamed+">", "the argument renamed inside the brackets")
		})

		t.Run("writes the stand-in for a missing reference", func(t *testing.T) {
			t.Parallel()

			got, err := spellref.SpellWith(nil, opener, closer, standIn, renaming)
			assert.NoError(t, err, "a missing reference spells")
			assert.Equal(t, got, standIn, "the language's stand-in")
		})

		t.Run("writes the stand-in for an argument that spells nothing", func(t *testing.T) {
			t.Parallel()

			got, err := spellref.SpellWith(generic("List", &emit.TypeRef{}), opener, closer, standIn, renaming)
			assert.NoError(t, err, "an argument that spells nothing spells")
			assert.Equal(t, got, "List<?>", "the stand-in inside the brackets")
		})

		t.Run("keeps a composite's spelling where every child spells as written", func(t *testing.T) {
			t.Parallel()

			list := &emit.TypeRef{Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{named(rowName)}}
			got, err := spellref.SpellWith(list, opener, closer, standIn, asWritten)
			assert.NoError(t, err, "the composite spells")
			assert.Equal(t, got, "Row[]", "the written spelling")
		})

		t.Run("returns an error for a composite whose child spells otherwise", func(t *testing.T) {
			t.Parallel()

			list := &emit.TypeRef{Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{named(rowName)}}
			_, err := spellref.SpellWith(list, opener, closer, standIn, renaming)
			assert.HasError(t, err, "the spelling cannot be restated")
			assert.Contains(t, err.Error(), "Row[]", "naming the composite")
		})

		refusal := errors.New("refused")
		failing := func(t *emit.TypeRef) (string, error) {
			if t.Spelling == rowName {
				return "", refusal
			}
			return t.Spelling, nil
		}
		tests := []struct {
			name string
			give *emit.TypeRef
		}{
			{name: "returns the error qualify returns for a named reference", give: named(rowName)},
			{name: "returns the error qualify returns for an argument", give: generic("List", named(rowName))},
			{
				name: "returns the error qualify returns for a composite's child",
				give: &emit.TypeRef{Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{named(rowName)}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spellref.SpellWith(tt.give, opener, closer, standIn, failing)
				assert.True(t, errors.Is(err, refusal), "the qualify error wraps through")
			})
		}
	})

	t.Run("PackageOf", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the recorded package of a reference without a target", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{Spelling: rowName, Package: modulePkg}
			assert.Equal(t, spellref.PackageOf(ref, lang), modulePkg, "the import the reference names")
		})

		t.Run("returns the target's package for a target of the language", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{
				Spelling: rowName, Package: modulePkg,
				Target: symbol.Identity{Lang: lang, Package: targetPkg, Name: rowName},
			}
			assert.Equal(t, spellref.PackageOf(ref, lang), targetPkg, "the declaring package")
		})

		t.Run("returns nothing for a target of another language", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{
				Spelling: rowName, Package: modulePkg,
				Target: symbol.Identity{Lang: otherLang, Package: targetPkg, Name: rowName},
			}
			assert.Equal(t, spellref.PackageOf(ref, lang), "", "no import of this language names it")
		})
	})
}

// A reference with arguments spells into one buffer, and a reference
// without them returns its spelling, in the ordinary run, which runs no
// benchmark.
func TestSpellRefAllocs(t *testing.T) {
	ref, leaf := nested(), named(rowName)
	list := &emit.TypeRef{Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{named(rowName)}}
	target := &emit.TypeRef{Spelling: rowName, Target: symbol.Identity{Lang: lang, Package: targetPkg, Name: rowName}}
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() { got = spellref.Spell(ref, opener, closer, standIn) }, spelledAllocs,
		"Spell allocates the buffer of a reference with arguments")
	assert.Equal(t, got, "Map<K, List<V>>", "Spell writes the arguments")
	assert.MaxAllocs(t, func() { got = spellref.Spell(leaf, opener, closer, standIn) }, 0,
		"Spell allocates nothing for a reference without arguments")
	assert.MaxAllocs(t, func() { got, err = spellref.SpellWith(ref, opener, closer, standIn, asWritten) },
		spelledAllocs, "SpellWith allocates the buffer of a reference with arguments")
	assert.NoError(t, err, "SpellWith spells the reference")
	assert.MaxAllocs(t, func() { got, err = spellref.SpellWith(list, opener, closer, standIn, asWritten) }, 0,
		"SpellWith allocates nothing for a composite spelled as written")
	assert.Equal(t, got, "Row[]", "SpellWith returns the composite's spelling")
	assert.MaxAllocs(t, func() { got = spellref.PackageOf(target, lang) }, 0, "PackageOf allocates nothing")
	assert.Equal(t, got, targetPkg, "PackageOf returns the target's package")
}

// BenchmarkSpellRef measures the spelling a backend writes for every
// reference it renders: a generic with a generic argument, and one
// without arguments.
func BenchmarkSpellRef(b *testing.B) {
	ref, leaf := nested(), named(rowName)
	spells := []struct {
		name   string
		give   *emit.TypeRef
		allocs uint64
		want   string
	}{
		{name: "a reference with arguments", give: ref, allocs: spelledAllocs, want: "Map<K, List<V>>"},
		{name: "a reference without arguments", give: leaf, want: rowName},
	}
	b.Run("Spell", func(b *testing.B) {
		for _, tt := range spells {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var got string
				for c.Loop() {
					got = spellref.Spell(tt.give, opener, closer, standIn)
				}
				assert.Equal(b, got, tt.want, "Spell writes the reference")
			})
		}
	})
	b.Run("SpellWith", func(b *testing.B) {
		for _, tt := range spells {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = spellref.SpellWith(tt.give, opener, closer, standIn, asWritten)
				}
				assert.NoError(b, err, "SpellWith spells the reference")
				assert.Equal(b, got, tt.want, "SpellWith writes the reference")
			})
		}
	})

	b.Run("PackageOf", func(b *testing.B) {
		target := &emit.TypeRef{
			Spelling: rowName,
			Target:   symbol.Identity{Lang: lang, Package: targetPkg, Name: rowName},
		}
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got string
		for c.Loop() {
			got = spellref.PackageOf(target, lang)
		}
		assert.Equal(b, got, targetPkg, "PackageOf returns the target's package")
	})
}

// generic returns a named reference with arguments.
func generic(name string, args ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Args: args}
}

// named returns a named reference without arguments.
func named(name string) *emit.TypeRef { return &emit.TypeRef{Spelling: name} }

// nested returns Map<K, List<V>>: a generic whose second argument is a
// generic.
func nested() *emit.TypeRef {
	return generic("Map", named("K"), generic("List", named("V")))
}

// renaming is a qualify that spells rowName as renamed and every other
// reference as written.
func renaming(t *emit.TypeRef) (string, error) {
	if t.Spelling == rowName {
		return renamed, nil
	}
	return t.Spelling, nil
}

// asWritten is a qualify that spells every reference as written.
func asWritten(t *emit.TypeRef) (string, error) { return t.Spelling, nil }
