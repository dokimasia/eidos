// Copyright Dokimasia B.V. 2026
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
	// paramName is a type parameter of rowName.
	paramName = "T"
)

// spelledAllocs is the spelling of a reference with arguments: one
// buffer sized to the result.
const spelledAllocs = 1

// restatedAllocs is the restatement of a list whose child spells
// otherwise: the list of the spellings of its children and the text that
// the fixture's restate returns.
const restatedAllocs = 2

// typeArgsAllocs is the type arguments of two type parameters: the text,
// in one buffer of its size.
const typeArgsAllocs = 1

// errRestate is the refusal of the restate that a case passes.
var errRestate = errors.New("spellref_test: the restate refuses the composite")

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
				assert.ErrorIs(t, err, refusal, "the qualify error wraps through")
			})
		}

		unspelled := []struct {
			name string
			give *emit.TypeRef
		}{
			{
				name: "returns an error for a composite without a spelling",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{named(paramName)}},
			},
			{
				name: "returns an error for a child composite without a spelling",
				give: &emit.TypeRef{Form: symbol.FormList, Spelling: "T[][]", Elems: []*emit.TypeRef{
					{Form: symbol.FormList, Elems: []*emit.TypeRef{named(paramName)}},
				}},
			},
		}
		for _, tt := range unspelled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spellref.SpellWith(tt.give, opener, closer, standIn, renaming)
				assert.HasError(t, err, "the spelling of the composite does not follow from its structure")
			})
		}
	})

	t.Run("SpellRestated", func(t *testing.T) {
		t.Parallel()

		refused := func(*emit.TypeRef, []string) (string, error) { return "", errRestate }
		suffixed := func(_ *emit.TypeRef, elems []string) (string, error) {
			if len(elems) != 1 {
				return "", errRestate
			}
			return elems[0] + "[]", nil
		}
		list := func(child *emit.TypeRef) *emit.TypeRef {
			return &emit.TypeRef{Form: symbol.FormList, Spelling: child.Spelling + "[]", Elems: []*emit.TypeRef{child}}
		}

		tests := []struct {
			name string
			give *emit.TypeRef
			want string
		}{
			{name: "writes a named reference through qualify", give: named(rowName), want: renamed},
			{name: "writes each argument through qualify", give: generic("List", named(rowName)), want: "List<Row2>"},
			{name: "writes the stand-in for a missing reference", give: nil, want: standIn},
			{name: "restates a composite whose child spells otherwise", give: list(named(rowName)), want: "Row2[]"},
			{
				name: "restates a composite around a restated child", give: list(list(named(rowName))),
				want: "Row2[][]",
			},
			{
				name: "restates a composite inside the brackets of an argument",
				give: generic("Page", list(named(rowName))), want: "Page<Row2[]>",
			},
			{
				name: "restates a composite without a spelling",
				give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{named(paramName)}},
				want: paramName + "[]",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spellref.SpellRestated(tt.give, opener, closer, standIn, renaming, suffixed)
				assert.NoError(t, err, "the reference spells")
				assert.Equal(t, got, tt.want, "the spelling has each name that qualify returns")
			})
		}

		t.Run("keeps the spelling of a composite whose children spell as written", func(t *testing.T) {
			t.Parallel()

			got, err := spellref.SpellRestated(list(named("Col")), opener, closer, standIn, renaming, refused)
			assert.NoError(t, err, "the composite spells without restate")
			assert.Equal(t, got, "Col[]", "the composite keeps its written spelling")
		})

		t.Run("passes the spelling of each child to restate", func(t *testing.T) {
			t.Parallel()

			pair := &emit.TypeRef{
				Form: symbol.FormMap, Spelling: "Map<Row, Col>", Elems: []*emit.TypeRef{named(rowName), named("Col")},
			}
			var got []string
			_, err := spellref.SpellRestated(pair, opener, closer, standIn, renaming,
				func(_ *emit.TypeRef, elems []string) (string, error) {
					got = elems
					return "", nil
				})
			assert.NoError(t, err, "the composite spells")
			assert.Equal(t, got, []string{renamed, "Col"}, "restate receives the children in order")
		})

		refusal := errors.New("refused")
		failing := func(t *emit.TypeRef) (string, error) {
			if t.Spelling == rowName {
				return "", refusal
			}
			return t.Spelling, nil
		}
		failed := []struct {
			name string
			give *emit.TypeRef
		}{
			{name: "returns the error of qualify for a named reference", give: named(rowName)},
			{name: "returns the error of qualify for an argument", give: generic("List", named(rowName))},
			{name: "returns the error of qualify for a child of a composite", give: list(named(rowName))},
		}
		for _, tt := range failed {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := spellref.SpellRestated(tt.give, opener, closer, standIn, failing, suffixed)
				assert.ErrorIs(t, err, refusal, "the error of qualify is returned")
			})
		}

		t.Run("returns the error of restate", func(t *testing.T) {
			t.Parallel()

			_, err := spellref.SpellRestated(list(named(rowName)), opener, closer, standIn, renaming, refused)
			assert.ErrorIs(t, err, errRestate, "the error of restate is returned")
		})
	})

	t.Run("TypeArgs", func(t *testing.T) {
		t.Parallel()

		spelled := func(t *emit.TypeRef) (string, error) { return spellref.Spell(t, opener, closer, standIn), nil }
		tests := []struct {
			name string
			give any
			want string
		}{
			{
				name: "writes the names of type parameters inside the given brackets",
				give: []*emit.TypeParam{{Name: paramName}, {Name: rowName}},
				want: opener + paramName + ", " + rowName + closer,
			},
			{
				name: "writes each type reference through spell",
				give: []*emit.TypeRef{named(rowName), nested()},
				want: "<Row, Map<K, List<V>>>",
			},
			{name: "writes nothing for an empty list of type parameters", give: []*emit.TypeParam{}, want: ""},
			{name: "writes nothing for an empty list of type references", give: []*emit.TypeRef{}, want: ""},
			{name: "writes nothing for nil", give: nil, want: ""},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := spellref.TypeArgs(tt.give, opener, closer, spelled)
				assert.NoError(t, err, "TypeArgs writes the list")
				assert.Equal(t, got, tt.want, "TypeArgs writes the type arguments")
			})
		}

		t.Run("returns an error for a list of another type", func(t *testing.T) {
			t.Parallel()

			_, err := spellref.TypeArgs([]string{paramName}, opener, closer, spelled)
			assert.HasError(t, err, "a list of text is no list of type arguments")
			assert.Contains(t, err.Error(), "[]string", "the error names the type of the list")
		})

		t.Run("returns the error of spell", func(t *testing.T) {
			t.Parallel()

			refusing := func(*emit.TypeRef) (string, error) { return "", errRestate }
			_, err := spellref.TypeArgs([]*emit.TypeRef{named(rowName)}, opener, closer, refusing)
			assert.ErrorIs(t, err, errRestate, "the error of spell is returned")
		})
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

		t.Run("returns the recorded package of a target of another language", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{
				Spelling: rowName, Package: modulePkg,
				Target: symbol.Identity{Lang: otherLang, Package: targetPkg, Name: rowName},
			}
			assert.Equal(t, spellref.PackageOf(ref, lang), modulePkg,
				"the layout records the package of the file that declares a translated reference's referent")
		})

		t.Run("returns no package for a reference to a type parameter", func(t *testing.T) {
			t.Parallel()

			ref := &emit.TypeRef{
				Spelling: paramName,
				Target: symbol.Identity{
					Lang: lang, Package: targetPkg, Owner: rowName, Name: paramName, Kind: symbol.KindTypeParam,
				},
			}
			assert.Equal(t, spellref.PackageOf(ref, lang), "", "no package exports a type parameter")
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
	suffixed := func(_ *emit.TypeRef, elems []string) (string, error) { return elems[0] + "[]", nil }
	assert.MaxAllocs(t, func() {
		got, err = spellref.SpellRestated(ref, opener, closer, standIn, asWritten, suffixed)
	}, spelledAllocs, "SpellRestated allocates the text of a reference with arguments")
	assert.Equal(t, got, "Map<K, List<V>>", "SpellRestated writes the arguments")
	assert.MaxAllocs(t, func() {
		got, err = spellref.SpellRestated(list, opener, closer, standIn, asWritten, suffixed)
	}, 0, "SpellRestated allocates nothing for a composite spelled as written")
	assert.Equal(t, got, "Row[]", "SpellRestated returns the written spelling of the composite")
	assert.MaxAllocs(t, func() {
		got, err = spellref.SpellRestated(list, opener, closer, standIn, renaming, suffixed)
	}, restatedAllocs, "SpellRestated allocates the spellings of the children and the restatement")
	assert.NoError(t, err, "SpellRestated restates the composite")
	assert.Equal(t, got, "Row2[]", "SpellRestated returns the restatement")
	assert.MaxAllocs(t, func() { got = spellref.PackageOf(target, lang) }, 0, "PackageOf allocates nothing")
	assert.Equal(t, got, targetPkg, "PackageOf returns the target's package")
	var params any = []*emit.TypeParam{{Name: paramName}, {Name: rowName}}
	assert.MaxAllocs(t, func() { got, err = spellref.TypeArgs(params, opener, closer, nil) }, typeArgsAllocs,
		"TypeArgs allocates the text that it writes")
	assert.Equal(t, got, "<T, Row>", "TypeArgs writes the names of the type parameters")
	assert.NoError(t, err, "TypeArgs writes every list")
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

	list := &emit.TypeRef{Form: symbol.FormList, Spelling: "Row[]", Elems: []*emit.TypeRef{named(rowName)}}
	suffixed := func(_ *emit.TypeRef, elems []string) (string, error) { return elems[0] + "[]", nil }
	restated := []struct {
		name    string
		give    *emit.TypeRef
		qualify spellref.Qualify
		allocs  uint64
		want    string
	}{
		{
			name: "a reference with arguments", give: ref, qualify: asWritten, allocs: spelledAllocs,
			want: "Map<K, List<V>>",
		},
		{name: "a reference without arguments", give: leaf, qualify: asWritten, want: rowName},
		{name: "a composite spelled as written", give: list, qualify: asWritten, want: "Row[]"},
		{
			name: "a composite whose child spells otherwise", give: list, qualify: renaming, allocs: restatedAllocs,
			want: "Row2[]",
		},
	}
	b.Run("SpellRestated", func(b *testing.B) {
		for _, tt := range restated {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = spellref.SpellRestated(tt.give, opener, closer, standIn, tt.qualify, suffixed)
				}
				assert.NoError(b, err, "SpellRestated spells the reference")
				assert.Equal(b, got, tt.want, "SpellRestated writes the reference")
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

	b.Run("TypeArgs", func(b *testing.B) {
		var params any = []*emit.TypeParam{{Name: paramName}, {Name: rowName}}
		c := bench.Start(b).MaxAllocs(typeArgsAllocs)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = spellref.TypeArgs(params, opener, closer, nil)
		}
		assert.NoError(b, err, "TypeArgs writes the list")
		assert.Equal(b, got, "<T, Row>", "TypeArgs writes the names of the type parameters")
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
