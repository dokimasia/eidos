// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// scriptedLang is the language the fixture targets name in every
// refusal.
const scriptedLang = "scripted"

// wideArgs is the argument count of a call wider than the walk's stack
// list of eight.
const wideArgs = 9

// The allocations of the walk under a target whose methods allocate
// nothing.
const (
	// compositeAllocs is a composite with fields: the list of entries
	// the walk passes to Composite.
	compositeAllocs = 1
	// callAllocs is a call of up to eight arguments: its spelling.
	callAllocs = 1
	// wideCallAllocs is a call of more than eight arguments: its
	// spelling and the list of its spelled arguments.
	wideCallAllocs = callAllocs + 1
	// quoteAllocs is a string through the fixture's quoting, which
	// concatenates the quotes.
	quoteAllocs = 1
)

// leaves is the fixture leaf spelling: a target named scripted, whose
// absent value is none, whose strings take single quotes, and whose
// numbers take a trailing mark.
var leaves = scaffold.Leaves{
	Lang:   scriptedLang,
	Absent: "none",
	Quote:  func(text string) string { return "'" + text + "'" },
	Number: func(v emit.Value) (string, error) { return v.Text + "n", nil },
}

// scripted is a target that spells a C-family syntax. The shared walk
// runs against it without a satellite. It records every import the
// walk requests, the walk's contract with a real target.
type scripted struct {
	imported []string
	// addressed records the kind of every value the walk passed to
	// Address.
	addressed []emit.ValueKind
	// refuseRaw refuses a raw literal, the way a target meeting
	// another language's text does.
	refuseRaw bool
}

// Lang returns the fixture's language.
func (*scripted) Lang() string { return scriptedLang }

// Literal spells a scalar as written, a string in double quotes and
// nil as null. It returns an error for a raw literal where the target
// refuses raw text, and for a literal kind it has no spelling for.
func (s *scripted) Literal(v emit.Value) (string, error) {
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat, emit.LiteralBool:
		return v.Text, nil
	case emit.LiteralString:
		return `"` + v.Text + `"`, nil
	case emit.LiteralNil:
		return "null", nil
	case emit.LiteralRaw:
		if s.refuseRaw {
			return "", fmt.Errorf("scripted: %q is written in another language", v.Text)
		}
		return v.Text, nil
	default:
		return "", fmt.Errorf("scripted: no spelling for the %s literal", v.Literal)
	}
}

// Type returns the reference's spelling, and records the package of
// a resolved target as an import.
func (s *scripted) Type(t *emit.TypeRef) (string, error) {
	if t.Target.Package != "" {
		s.imported = append(s.imported, t.Target.Package)
	}
	return t.Spelling, nil
}

// Callee returns the callee's name, and records its package as an
// import.
func (s *scripted) Callee(id symbol.Identity) (string, error) {
	if id.Package != "" {
		s.imported = append(s.imported, id.Package)
	}
	return id.Name, nil
}

// Conversion spells a conversion as a call of the type.
func (*scripted) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return typ + "(" + inner + ")", nil
}

// Composite spells a composite literal in braces, each entry keyed
// by its field name or its key where it has one.
func (*scripted) Composite(_ *emit.TypeRef, typ string, entries []scaffold.Entry) (string, error) {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Name != "":
			parts = append(parts, e.Name+": "+e.Value)
		case e.Key != "":
			parts = append(parts, e.Key+": "+e.Value)
		default:
			parts = append(parts, e.Value)
		}
	}
	return typ + "{" + strings.Join(parts, ", ") + "}", nil
}

// Address spells the address of a value with a leading ampersand, and
// records the kind of the value it was given.
func (s *scripted) Address(inner emit.Value, spelled string) (string, error) {
	s.addressed = append(s.addressed, inner.Kind)
	return "&" + spelled, nil
}

// unspelling is a target that refuses callees, conversions, composites
// and addresses, so each of the walk's error paths runs against it.
type unspelling struct{ scripted }

// Callee returns an error: the target has no callee form.
func (unspelling) Callee(symbol.Identity) (string, error) {
	return "", errors.New("scripted: no callee form")
}

// Conversion returns an error: the target has no conversion form.
func (unspelling) Conversion(*emit.TypeRef, string, string) (string, error) {
	return "", errors.New("scripted: no conversion form")
}

// Composite returns an error: the target has no composite form.
func (unspelling) Composite(*emit.TypeRef, string, []scaffold.Entry) (string, error) {
	return "", errors.New("scripted: no composite form")
}

// Address returns an error: the target has no address form.
func (unspelling) Address(emit.Value, string) (string, error) {
	return "", errors.New("scripted: no address form")
}

// passthrough is a target that composes nothing: each method returns a
// spelling the walk passed it, so an allocation count measures the walk
// alone. It allocates nothing.
type passthrough struct{}

// Lang returns the fixture's language.
func (passthrough) Lang() string { return scriptedLang }

// Literal returns the literal's text.
func (passthrough) Literal(v emit.Value) (string, error) { return v.Text, nil }

// Type returns the reference's spelling.
func (passthrough) Type(t *emit.TypeRef) (string, error) { return t.Spelling, nil }

// Callee returns the callee's name.
func (passthrough) Callee(id symbol.Identity) (string, error) { return id.Name, nil }

// Conversion returns the inner value's spelling.
func (passthrough) Conversion(_ *emit.TypeRef, _, inner string) (string, error) { return inner, nil }

// Composite returns the type's spelling.
func (passthrough) Composite(_ *emit.TypeRef, typ string, _ []scaffold.Entry) (string, error) {
	return typ, nil
}

// Address returns the inner value's spelling.
func (passthrough) Address(_ emit.Value, spelled string) (string, error) { return spelled, nil }

// The Go, Java, Rust and TypeScript backends spell their values through
// Value and Leaves. The walk's order, its recursion and its refusals
// are pinned for all four.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "writes a number as its text", give: integer("42"), want: "42"},
			{name: "writes a string in quotes", give: emit.Literal(emit.LiteralString, "hi"), want: `"hi"`},
			{
				name: "writes the absent value as the target spells it",
				give: emit.Literal(emit.LiteralNil, ""), want: "null",
			},
			{
				name: "writes a conversion around its inner value",
				give: emit.Conversion(ref("svc", "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
				want: "Weight(1.5)",
			},
			{
				name: "writes a composite's named fields in order",
				give: emit.Composite(ref("svc", "Row"),
					emit.ValueField{Name: "ID", Value: integer("1")},
					emit.ValueField{Name: "Tag", Value: emit.Literal(emit.LiteralString, "a")}),
				want: `Row{ID: 1, Tag: "a"}`,
			},
			{
				name: "writes a composite's keyed entry",
				give: emit.Composite(ref("svc", "Index"),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), integer("2"))),
				want: `Index{"k": 2}`,
			},
			{
				name: "writes a composite's positional element",
				give: emit.Composite(ref("svc", "List"), emit.Element(integer("2"))),
				want: "List{2}",
			},
			{name: "writes raw text in the target's language", give: emit.Raw(scriptedLang, "Row{}"), want: "Row{}"},
			{
				name: "writes a call with its arguments",
				give: emit.Call(fn("time", "Unix"), integer("1"), integer("0")),
				want: "Unix(1, 0)",
			},
			{name: "writes a call without arguments", give: emit.Call(fn("time", "Now")), want: "Now()"},
			{
				name: "writes a call with more than eight arguments",
				give: emit.Call(fn("m", "Max"), numbers(wideArgs)...),
				want: "Max(1, 2, 3, 4, 5, 6, 7, 8, 9)",
			},
			{
				name: "writes an address around its inner value",
				give: emit.Address(emit.Composite(ref("svc", "Row"))), want: "&Row{}",
			},
			{
				name: "writes nested forms",
				give: emit.Address(emit.Conversion(ref("svc", "Weight"),
					emit.Call(fn("m", "Sum"), integer("3")))),
				want: "&Weight(Sum(3))",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := scaffold.Value(&scripted{}, tt.give)
				assert.NoError(t, err, "the fixture spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		t.Run("passes Address the value it takes the address of", func(t *testing.T) {
			t.Parallel()

			target := &scripted{}
			_, err := scaffold.Value(target, emit.Address(emit.Composite(ref("svc", "Row"))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, target.addressed, []emit.ValueKind{emit.ValueComposite},
				"the target reads the inner value's kind, not only its spelling")
		})

		t.Run("records the imports the tree needs in walk order", func(t *testing.T) {
			t.Parallel()

			target := &scripted{}
			_, err := scaffold.Value(target, emit.Composite(ref("svc", "Row"),
				emit.ValueField{Name: "When", Value: emit.Call(fn("time", "Unix"))},
				emit.ValueField{Name: "W", Value: emit.Conversion(ref("units", "Weight"), integer("1"))}))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, target.imported, []string{"svc", "time", "units"},
				"every reference and callee records its import, the outermost first")
		})

		walkErrors := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "returns a value error for a kind nothing declares", give: emit.Value{}, want: "no spelling"},
			{
				name: "returns a value error for a conversion that names no type",
				give: emit.Value{Kind: emit.ValueConversion, Inner: &emit.Value{}}, want: "names no type",
			},
			{
				name: "returns a value error for a conversion that wraps nothing",
				give: emit.Value{Kind: emit.ValueConversion, Type: ref("svc", "W")}, want: "wraps nothing",
			},
			{
				name: "returns a value error for a composite that names no type",
				give: emit.Value{Kind: emit.ValueComposite}, want: "names no type",
			},
			{
				name: "returns a value error for a composite whose type spells nothing",
				give: emit.Composite(&emit.TypeRef{}), want: "spells nothing",
			},
			{
				name: "returns a value error for an address that wraps nothing",
				give: emit.Value{Kind: emit.ValueAddress}, want: "wraps nothing",
			},
			{
				name: "returns a value error for a call that names no callee",
				give: emit.Value{Kind: emit.ValueCall}, want: "names no callee",
			},
			{
				name: "returns a value error for a call of a function without a name",
				give: emit.Call(symbol.Identity{Lang: scriptedLang, Package: "time"}), want: "spells nothing",
			},
			{
				name: "returns a value error for a broken value inside a composite",
				give: emit.Composite(ref("svc", "Row"), emit.ValueField{Name: "f", Value: emit.Value{}}),
				want: "no spelling",
			},
			{
				name: "returns a value error for a broken value inside a call",
				give: emit.Call(fn("m", "F"), emit.Value{}), want: "no spelling",
			},
			{
				name: "returns a value error for a broken key inside a composite",
				give: emit.Composite(ref("svc", "Index"), emit.KeyedEntry(emit.Value{}, integer("2"))),
				want: "no spelling",
			},
		}
		for _, tt := range walkErrors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Value(&scripted{}, tt.give)
				refused, isValue := errors.AsType[*render.ValueError](err)
				assert.True(t, isValue, "the walk refuses a value as a value refusal")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what is missing")
				assert.Equal(t, refused.Lang, scriptedLang, "naming the target it was spelling for")
			})
		}

		targetErrors := []struct {
			name   string
			give   emit.Value
			target scaffold.Target
			want   string
		}{
			{
				name:   "returns the target's error for a literal it has no form for",
				give:   emit.Literal(emit.LiteralRaw, "Row{}"),
				target: &scripted{refuseRaw: true}, want: "another language",
			},
			{
				name:   "returns the target's error for a conversion it has no form for",
				give:   emit.Conversion(ref("svc", "W"), integer("1")),
				target: &unspelling{}, want: "no conversion form",
			},
			{
				name:   "returns the target's error for a composite it has no form for",
				give:   emit.Composite(ref("svc", "Row")),
				target: &unspelling{}, want: "no composite form",
			},
			{
				name:   "returns the target's error for an address it has no form for",
				give:   emit.Address(integer("1")),
				target: &unspelling{}, want: "no address form",
			},
			{
				name:   "returns the target's error for a callee it has no form for",
				give:   emit.Call(fn("time", "Now")),
				target: &unspelling{}, want: "no callee form",
			},
		}
		for _, tt := range targetErrors {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := scaffold.Value(tt.target, tt.give)
				assert.HasError(t, err, "the target refuses")
				assert.Contains(t, err.Error(), tt.want, "the target's refusal passes through")
			})
		}
	})

	t.Run("Literal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "writes an integer through the number spelling", give: integer("42"), want: "42n"},
			{
				name: "writes a float through the number spelling",
				give: emit.Literal(emit.LiteralFloat, "1.5"), want: "1.5n",
			},
			{name: "writes a string through the quoting", give: emit.Literal(emit.LiteralString, "hi"), want: "'hi'"},
			{name: "writes true as itself", give: emit.Literal(emit.LiteralBool, "true"), want: "true"},
			{name: "writes false as itself", give: emit.Literal(emit.LiteralBool, "false"), want: "false"},
			{name: "writes the absent value as the target's", give: emit.Literal(emit.LiteralNil, ""), want: "none"},
			{name: "writes raw text in the target's language", give: emit.Raw(scriptedLang, "Row{}"), want: "Row{}"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := leaves.Literal(tt.give)
				assert.NoError(t, err, "the leaf spells")
				assert.Equal(t, got, tt.want, "the spelling")
			})
		}

		t.Run("writes a number as its text without a number spelling", func(t *testing.T) {
			t.Parallel()

			got, err := plainLeaves().Literal(integer("0x2A"))
			assert.NoError(t, err, "the number spells")
			assert.Equal(t, got, "0x2A", "as the derivation wrote it")
		})

		refusals := []struct {
			name string
			give emit.Value
			want string
		}{
			{name: "returns a value error for a number without text", give: integer(""), want: "has no text"},
			{
				name: "returns a value error for a boolean outside its two spellings",
				give: emit.Literal(emit.LiteralBool, "yes"), want: "true or false",
			},
			{
				name: "returns a value error for raw text in another language",
				give: emit.Raw("other", "{}"), want: `"{}" is written in other, not scripted`,
			},
			{
				name: "returns a value error for a literal kind nothing declares",
				give: emit.Value{Kind: emit.ValueLiteral}, want: "no spelling",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := leaves.Literal(tt.give)
				refused, isValue := errors.AsType[*render.ValueError](err)
				assert.True(t, isValue, "a leaf refuses as a value refusal")
				assert.Contains(t, err.Error(), tt.want, "the refusal names what it cannot spell")
				assert.Equal(t, refused.Lang, scriptedLang, "naming the target")
			})
		}
	})
}

// The walk allocates a composite's entries and a call's spelling, and
// a leaf allocates nothing of its own. The ordinary run, which runs no
// benchmark, checks those ceilings here.
func TestValueAllocs(t *testing.T) {
	lit, conv, addr := integer("42"), emit.Conversion(ref("svc", "Weight"), integer("1")),
		emit.Address(emit.Composite(ref("svc", "Row")))
	row, unix, wide := rowValue(), unixCall(), emit.Call(fn("m", "Max"), numbers(wideArgs)...)
	plain, text := plainLeaves(), emit.Literal(emit.LiteralString, "hi")
	var (
		got string
		err error
	)
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, lit) }, 0,
		"Value allocates nothing for a literal")
	assert.Equal(t, got, "42", "Value spells the literal")
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, conv) }, 0,
		"Value allocates nothing for a conversion")
	assert.Equal(t, got, "1", "Value spells the conversion")
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, addr) }, 0,
		"Value allocates nothing for an address")
	assert.Equal(t, got, "Row", "Value spells the address")
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, row) }, compositeAllocs,
		"Value allocates the entries of a composite")
	assert.Equal(t, got, "Row", "Value spells the composite")
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, unix) }, callAllocs,
		"Value allocates the spelling of a call")
	assert.Equal(t, got, "Unix(1, 0)", "Value spells the call")
	assert.MaxAllocs(t, func() { got, err = scaffold.Value(passthrough{}, wide) }, wideCallAllocs,
		"Value allocates the argument list of a call above eight arguments")
	assert.Equal(t, got, "Max(1, 2, 3, 4, 5, 6, 7, 8, 9)", "Value spells the wide call")
	assert.NoError(t, err, "Value spells every tree")
	assert.MaxAllocs(t, func() { got, err = plain.Literal(lit) }, 0,
		"Literal allocates nothing for a number without a number spelling")
	assert.Equal(t, got, "42", "Literal writes the number")
	assert.MaxAllocs(t, func() { got, err = leaves.Literal(text) }, quoteAllocs,
		"Literal allocates only what the quoting allocates")
	assert.Equal(t, got, "'hi'", "Literal quotes the string")
	assert.NoError(t, err, "Literal spells every leaf")
}

// BenchmarkValue measures the walk over a call and a composite, and one
// leaf, each under a target whose methods allocate nothing.
func BenchmarkValue(b *testing.B) {
	trees := []struct {
		name   string
		give   emit.Value
		allocs uint64
		want   string
	}{
		{name: "a call", give: unixCall(), allocs: callAllocs, want: "Unix(1, 0)"},
		{name: "a composite", give: rowValue(), allocs: compositeAllocs, want: "Row"},
	}
	b.Run("Value", func(b *testing.B) {
		for _, tt := range trees {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var (
					got string
					err error
				)
				for c.Loop() {
					got, err = scaffold.Value(passthrough{}, tt.give)
				}
				assert.NoError(b, err, "Value spells the tree")
				assert.Equal(b, got, tt.want, "Value writes the spelling")
			})
		}
	})

	b.Run("Literal", func(b *testing.B) {
		plain, lit := plainLeaves(), integer("42")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var (
			got string
			err error
		)
		for c.Loop() {
			got, err = plain.Literal(lit)
		}
		assert.NoError(b, err, "Literal spells the number")
		assert.Equal(b, got, "42", "Literal writes the number")
	})
}

// integer returns an integer literal.
func integer(text string) emit.Value { return emit.Literal(emit.LiteralInt, text) }

// numbers returns the integer literals 1 to n.
func numbers(n int) []emit.Value {
	out := make([]emit.Value, 0, n)
	for i := range n {
		out = append(out, integer(strconv.Itoa(i+1)))
	}
	return out
}

// ref returns a reference to a declaration in one package.
func ref(pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: name,
		Target:   symbol.Identity{Lang: scriptedLang, Package: pkg, Name: name, Kind: symbol.KindStruct},
	}
}

// fn returns a callee identity in one package.
func fn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: scriptedLang, Package: pkg, Name: name, Kind: symbol.KindFunction}
}

// rowValue returns a composite of Row with two named fields.
func rowValue() emit.Value {
	return emit.Composite(ref("svc", "Row"),
		emit.ValueField{Name: "ID", Value: integer("1")},
		emit.ValueField{Name: "Tag", Value: integer("2")})
}

// unixCall returns the call time.Unix(1, 0).
func unixCall() emit.Value { return emit.Call(fn("time", "Unix"), integer("1"), integer("0")) }

// plainLeaves returns the fixture leaf spelling without a number
// spelling, so a number writes the text the derivation wrote.
func plainLeaves() scaffold.Leaves {
	plain := leaves
	plain.Number = nil
	return plain
}
