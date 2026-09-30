// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The fixture: the brackets a language writes arguments in, the
// stand-in an absent spelling takes, and the names a qualify renames.
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

// generic returns a named reference with arguments.
func generic(name string, args ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Spelling: name, Args: args}
}

// named returns a named reference without arguments.
func named(name string) *emit.TypeRef { return &emit.TypeRef{Spelling: name} }

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
