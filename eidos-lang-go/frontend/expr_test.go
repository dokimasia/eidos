// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The import paths the package cases name: a standard library
// package, and a module path whose last element has a major version.
const (
	timePath = "time"
	yamlPath = "gopkg.in/yaml.v3"
)

// fieldsOf parses one file and returns the fields of its first
// declaration, a struct.
func fieldsOf(tb assert.TB, src string) []*node.Field {
	tb.Helper()

	file := onlyFile(tb, parsedFile(tb, nil, plugin.DepthFull, src))
	return file.Decls[0].(*node.Struct).Fields
}

// Type expressions keep their verbatim spellings with arguments split
// for instantiations, and record the import a qualified name names.
// The reference shapes the corpus depends on are pinned here through
// parsed fields.
func TestExpr(t *testing.T) {
	t.Parallel()

	t.Run("typeRef", func(t *testing.T) {
		t.Parallel()

		t.Run("splits instantiation arguments off the bare name", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t,
				"package p\n\ntype H struct {\n\ta List[int]\n\tb Pair[int, string]\n}\n")
			assert.Equal(t, fields[0].Type.Spelling, "List", "the bare name alone")
			assert.Length(t, fields[0].Type.Args, 1, "one argument split out")
			assert.Equal(t, fields[0].Type.Args[0].Spelling, "int", "as its own reference")
			assert.Length(t, fields[1].Type.Args, 2, "two for a pair")
		})

		t.Run("keeps a composite's spelling verbatim", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t,
				"package p\n\ntype H struct {\n\ta map[string]int\n\tb []*User\n\tc (chan int)\n}\n")
			assert.Equal(t, fields[0].Type.Spelling, "map[string]int", "a map is one spelling")
			assert.Equal(t, fields[1].Type.Spelling, "[]*User", "the decoration is part of it")
			assert.Equal(t, fields[2].Type.Spelling, "chan int",
				"parentheses unwrap before the spelling is taken")
		})

		t.Run("states the structure beside the spelling", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t, "package p\n\ntype H struct {\n"+
				"\ta *User\n\tb []User\n\tc [4]byte\n\td [n]byte\n\te map[string]*User\n"+
				"\tf <-chan User\n\tg func(int, ...string) (User, error)\n\th struct{ X int }\n\ti User\n}\n")
			forms := make([]symbol.TypeForm, 0, len(fields))
			for _, f := range fields {
				forms = append(forms, f.Type.Form)
			}
			assert.Equal(t, forms, []symbol.TypeForm{
				symbol.FormOptional, symbol.FormList, symbol.FormArray, symbol.FormArray,
				symbol.FormMap, symbol.FormStream, symbol.FormFunc, symbol.FormInline, symbol.FormNamed,
			}, "each composite records Go's own structure")

			assert.Equal(t, fields[0].Type.Elems[0].Spelling, "User", "a pointer's one child")
			assert.Equal(t, fields[2].Type.Length, 4, "a literal length is recorded")
			assert.Equal(t, fields[3].Type.Length, 0, "a constant length is part of the spelling")
			m := fields[4].Type
			assert.Equal(t, m.Elems[0].Spelling, "string", "a map's key comes first")
			assert.Equal(t, m.Elems[1].Form, symbol.FormOptional, "then its value, structured in turn")
			assert.Equal(t, m.Elems[1].Elems[0].Spelling, "User", "down to the name")
			assert.Equal(t, fields[5].Type.Elems[0].Spelling, "User", "a channel's element")
			fn := fields[6].Type
			assert.Length(t, fn.Elems, 4, "a function type's parameters then results")
			assert.Equal(t, fn.Split, 2, "split where the results begin")
			assert.Equal(t, fn.Elems[1].Form, symbol.FormList, "a variadic parameter is the list it is")
			assert.Equal(t, fn.Elems[2].Spelling, "User", "the first result follows")
			assert.Empty(t, fields[7].Type.Elems, "an inline body has no children")
		})

		t.Run("positions a reference at its expression", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t, "package p\n\ntype H struct {\n\ta int\n}\n")
			assert.Equal(t, fields[0].Type.Pos.Line, 4, "at the type's own line")
		})
	})

	t.Run("packageOf", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			give string
			want string
		}{
			{
				name: "records the import path an unaliased qualifier binds",
				give: "package p\n\nimport \"time\"\n\ntype H struct {\n\ta time.Duration\n}\n",
				want: timePath,
			},
			{
				name: "records the import path an alias binds",
				give: "package p\n\nimport tm \"time\"\n\ntype H struct {\n\ta tm.Duration\n}\n",
				want: timePath,
			},
			{
				name: "records the import path whose major version the qualifier drops",
				give: "package p\n\nimport \"gopkg.in/yaml.v3\"\n\ntype H struct {\n\ta yaml.Node\n}\n",
				want: yamlPath,
			},
			{
				name: "records the import path of a qualified instantiation",
				give: "package p\n\nimport \"iter\"\n\ntype H struct {\n\ta iter.Seq[int]\n}\n",
				want: golang.IterPackage,
			},
			{
				name: "records the import path of a qualified instantiation of two arguments",
				give: "package p\n\nimport \"iter\"\n\ntype H struct {\n\ta iter.Seq2[int, string]\n}\n",
				want: golang.IterPackage,
			},
			{
				name: "records no package for an unqualified name",
				give: "package p\n\nimport \"time\"\n\ntype H struct {\n\ta User\n}\n\ntype User struct{}\n",
				want: "",
			},
			{
				name: "records no package for a qualifier no import binds",
				give: "package p\n\ntype H struct {\n\ta other.Thing\n}\n",
				want: "",
			},
			{
				name: "records no package for a composite",
				give: "package p\n\nimport \"time\"\n\ntype H struct {\n\ta *time.Time\n}\n",
				want: "",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := fieldsOf(t, tt.give)[0].Type.Package
				assert.Equal(t, got, tt.want, "the package the reference's import names")
			})
		}

		t.Run("records the import path on a composite's named child", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t, "package p\n\nimport \"time\"\n\ntype H struct {\n\ta *time.Time\n}\n")
			assert.Equal(t, fields[0].Type.Elems[0].Package, timePath, "the pointer's element names the import")
		})
	})
}
