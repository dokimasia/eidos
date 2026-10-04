// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/lang/go/backend"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The value fixture: the package the spelled file belongs to, and the
// packages and functions its values name.
const (
	// svcPkg is the package of the file the fixture values render
	// into, and unitsPkg another workspace package.
	svcPkg   = "example.test/svc"
	unitsPkg = "example.test/units"
	// unixName and nowName are functions of timePkg.
	unixName = "Unix"
	nowName  = "Now"
)

// Every value spelling is pinned byte for byte, for the reason the
// statement spellings are: the text is spliced into generated bodies,
// and a drift rewrites files.
func TestValue(t *testing.T) {
	t.Parallel()

	t.Run("Scaffold", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			give    emit.Value
			want    string
			wantErr bool
		}{
			{name: "writes an integer as its digits", give: emit.Literal(emit.LiteralInt, "42"), want: "\treturn 42\n"},
			{
				name: "writes a float as its digits",
				give: emit.Literal(emit.LiteralFloat, "1.5"),
				want: "\treturn 1.5\n",
			},
			{
				name: "writes a boolean as its keyword",
				give: emit.Literal(emit.LiteralBool, "true"),
				want: "\treturn true\n",
			},
			{name: "writes the absent value as nil", give: emit.Literal(emit.LiteralNil, ""), want: "\treturn nil\n"},
			{
				name: "writes a string quoted the way Go quotes",
				give: emit.Literal(emit.LiteralString, `a"b`),
				want: "\treturn \"a\\\"b\"\n",
			},
			{
				name: "writes an author's own Go text verbatim",
				give: emit.Raw(golang.Lang, "Row{ID: 1}"),
				want: "\treturn Row{ID: 1}\n",
			},
			{
				name: "writes a conversion as a call of the type",
				give: emit.Conversion(valueRef("Weight", svcPkg, "Weight"), emit.Literal(emit.LiteralFloat, "1.5")),
				want: "\treturn Weight(1.5)\n",
			},
			{
				name: "writes a struct composite with its named fields",
				give: emit.Composite(valueRef(rowName, svcPkg, rowName),
					emit.NamedField("ID", emit.Literal(emit.LiteralInt, "1")),
					emit.NamedField("Tag", emit.Literal(emit.LiteralString, "a"))),
				want: "\treturn Row{ID: 1, Tag: \"a\"}\n",
			},
			{
				name: "writes a map composite with its keyed entry",
				give: emit.Composite(ref("map[string]int"),
					emit.KeyedEntry(emit.Literal(emit.LiteralString, "k"), emit.Literal(emit.LiteralInt, "2"))),
				want: "\treturn map[string]int{\"k\": 2}\n",
			},
			{
				name: "writes a slice composite with its bare element",
				give: emit.Composite(ref("[]int"), emit.Element(emit.Literal(emit.LiteralInt, "2"))),
				want: "\treturn []int{2}\n",
			},
			{
				name: "writes a call of another package's function qualified",
				give: emit.Call(valueFn(timePkg, unixName),
					emit.Literal(emit.LiteralInt, "1"), emit.Literal(emit.LiteralInt, "0")),
				want: "\treturn time.Unix(1, 0)\n",
			},
			{
				name: "writes a call of a function in no package bare",
				give: emit.Call(symbol.Identity{Lang: golang.Lang, Name: "make"}),
				want: "\treturn make()\n",
			},
			{
				name: "writes the address of a composite",
				give: emit.Address(emit.Composite(valueRef(rowName, svcPkg, rowName))),
				want: "\treturn &Row{}\n",
			},
			{
				name:    "returns a value error for another language's raw text",
				give:    emit.Raw("typescript", "{id: 1}"),
				want:    "written in typescript",
				wantErr: true,
			},
			{
				name:    "returns a value error for a boolean outside its two spellings",
				give:    emit.Literal(emit.LiteralBool, "yes"),
				want:    "spells true or false",
				wantErr: true,
			},
			{
				name:    "returns a value error for a number without text",
				give:    emit.Literal(emit.LiteralInt, ""),
				want:    "has no text",
				wantErr: true,
			},
			{
				name:    "returns a value error for a literal kind nothing declares",
				give:    emit.Value{Kind: emit.ValueLiteral},
				want:    "no spelling for the",
				wantErr: true,
			},
			{
				name:    "returns a value error for a composite of a type that spells nothing",
				give:    emit.Composite(&emit.TypeRef{}),
				want:    "spells nothing",
				wantErr: true,
			},
			{
				name:    "returns a value error for a call of a function that spells nothing",
				give:    emit.Call(symbol.Identity{Lang: golang.Lang, Package: timePkg}),
				want:    "spells nothing",
				wantErr: true,
			},
			{
				name:    "returns a value error for the address of anything but a composite literal",
				give:    emit.Address(emit.Literal(emit.LiteralInt, "1")),
				want:    "composite literal alone",
				wantErr: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got, _, err := returned(t, tt.give)
				if !tt.wantErr {
					assert.NoError(t, err, "the value spells")
					assert.Equal(t, got, tt.want, "the Go spelling")
					return
				}
				var refused *render.ValueError
				assert.True(t, errors.As(err, &refused), "the render reports the value's own code")
				assert.Contains(t, err.Error(), tt.want, "naming what Go cannot spell")
				assert.Equal(t, refused.Lang, string(golang.Lang), "naming the target")
			})
		}

		t.Run("binds the import of every reference and callee of another package", func(t *testing.T) {
			t.Parallel()

			_, set, err := returned(t, emit.Composite(valueRef(rowName, storePkg, rowName),
				emit.NamedField("When", emit.Call(valueFn(timePkg, unixName))),
				emit.NamedField("W", emit.Conversion(
					valueRef("Weight", unitsPkg, "Weight"),
					emit.Literal(emit.LiteralInt, "1"),
				))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, set.Paths(), []string{storePkg, unitsPkg, timePkg},
				"every package the tree names, in path order")
		})

		t.Run("writes the names of the file's own package bare", func(t *testing.T) {
			t.Parallel()

			got, set, err := returned(t, emit.Composite(valueRef(rowName, svcPkg, rowName),
				emit.NamedField("ID", emit.Call(valueFn(svcPkg, "NextID"))),
				emit.NamedField("When", emit.Call(valueFn(timePkg, nowName)))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, got, "\treturn Row{ID: NextID(), When: time.Now()}\n",
				"a package never qualifies its own names")
			assert.Equal(t, set.Paths(), []string{timePkg}, "the other package alone is imported")
		})

		t.Run("qualifies a callee through the name its path assumes", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Call(valueFn(yamlPkg, "Marshal")))
			assert.NoError(t, err, "the call spells")
			assert.Equal(t, got, "\treturn "+yamlName+".Marshal()\n", "the major version is dropped")
		})

		t.Run("suffixes a callee's qualifier another package binds", func(t *testing.T) {
			t.Parallel()

			got, _, err := returned(t, emit.Composite(valueRef(rowName, storePkg, rowName),
				emit.NamedField("Old", emit.Call(valueFn(legacyPkg, "Open")))))
			assert.NoError(t, err, "the tree spells")
			assert.Equal(t, got, "\treturn store.Row{Old: "+storeName2+".Open()}\n",
				"the reference's package binds the name first")
		})

		t.Run("returns an error for a reference that does not spell", func(t *testing.T) {
			t.Parallel()

			_, _, err := returned(t, emit.Composite(ref("x.Y")))
			assert.HasError(t, err, "a qualified reference without a package is refused")
		})

		t.Run("places a value in a call the scaffold writes", func(t *testing.T) {
			t.Parallel()

			fn := nameOf("want")
			out, err := backend.Scaffold(emit.Stmt{
				Kind: emit.StmtExpr,
				Value: emit.Expr{Kind: emit.ExprCall, Fn: &fn, Args: []emit.Expr{
					nameOf("got"), emit.ValueExpr(emit.Literal(emit.LiteralInt, "42")),
				}},
			}, &render.ImportSet{})
			assert.NoError(t, err, "the statement spells")
			assert.Equal(t, string(out), "\twant(got, 42)\n", "a value is an argument like any other")
		})
	})
}

// valueRef returns a resolved reference to a struct of one package,
// spelled the way Go source spells it.
func valueRef(spelling, pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: spelling,
		Target:   symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name, Kind: symbol.KindStruct},
	}
}

// valueFn returns a callee identity.
func valueFn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: golang.Lang, Package: pkg, Name: name, Kind: symbol.KindFunction}
}

// returned runs one value through the Go scaffold as a return in a
// file of svcPkg, and returns the statement's text beside the file's
// import set.
func returned(tb assert.TB, v emit.Value) (string, *render.ImportSet, error) {
	tb.Helper()

	set := &render.ImportSet{}
	set.SetHome(svcPkg)
	out, err := backend.Scaffold(emit.Stmt{Kind: emit.StmtReturn, Value: emit.ValueExpr(v)}, set)
	return string(out), set, err
}
