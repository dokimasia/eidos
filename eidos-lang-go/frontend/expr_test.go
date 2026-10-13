// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

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
			assert.Equal(t, spellingsOf(fields[7].Type.Elems), []string{"int"},
				"an inline body has the type of its field as its child")
		})

		tests := []struct {
			name      string
			give      string
			wantTerms []string
		}{
			{
				name:      "lowers a union constraint to a union of its terms",
				give:      "T ~int64 | time.Duration",
				wantTerms: []string{"int64", "time.Duration"},
			},
			{
				name:      "lowers an approximation alone to a union of one term",
				give:      "T ~int",
				wantTerms: []string{"int"},
			},
			{
				name:      "lowers a union of three terms to three children",
				give:      "T int | string | bool",
				wantTerms: []string{"int", "string", "bool"},
			},
			{
				name:      "lowers a term in parentheses as the term inside them",
				give:      "T (int) | string,",
				wantTerms: []string{"int", "string"},
			},
			{
				name:      "lowers the composite of an approximation as one term",
				give:      "T ~[]E, E any",
				wantTerms: []string{"[]E"},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				bound := boundOf(t, tt.give)
				expect.That(t, bound.Form).Equal(symbol.FormUnion, "a constraint element is a union")
				expect.That(t, spellingsOf(bound.Elems)).Equal(tt.wantTerms, "each term is a child, in order")
			})
		}

		t.Run("keeps the approximation mark in the spelling of a union", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, boundOf(t, "T ~int64 | time.Duration").Spelling, "~int64 | time.Duration",
				"the spelling of the union marks the approximated term")
		})

		t.Run("records the import path of a qualified term", func(t *testing.T) {
			t.Parallel()

			terms := boundOf(t, "T ~int64 | time.Duration").Elems
			assert.Length(t, terms, 2, "the union has two terms")
			assert.Equal(t, terms[1].Package, timePath, "the term resolves as any reference does")
		})

		t.Run("positions a reference at its expression", func(t *testing.T) {
			t.Parallel()

			fields := fieldsOf(t, "package p\n\ntype H struct {\n\ta int\n}\n")
			assert.Equal(t, fields[0].Type.Pos.Line, 4, "at the type's own line")
		})
	})

	t.Run("inlineFields", func(t *testing.T) {
		t.Parallel()

		t.Run("records an inline struct's named fields as the reference's fields", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "struct {\n\t\tX, y int\n\t\tZ string\n\t}")
			assert.Equal(t, fieldNames(ref.Fields), []string{"X", "y", "Z"}, "one field per bound name")
		})

		t.Run("records an inline field's visibility from its name", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "struct{ X, y int }")
			assert.Equal(t, ref.Fields[1].Visibility, symbol.VisibilityPackage, "a lowercase name is unexported")
		})

		t.Run("records an inline field's tag unquoted", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, inlineOf(t, "", "struct{ X int `json:\"x\"` }").Fields[0].Tag, `json:"x"`,
				"the tag a declared field records")
		})

		t.Run("records the import path of an inline field's type", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "import \"time\"\n\n", "struct{ D time.Duration }")
			assert.Equal(t, ref.Fields[0].Type.Package, timePath, "the member's type resolves as any reference does")
		})

		t.Run("leaves an embedded field out of an inline struct's fields", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "struct {\n\t\tio.Reader\n\t\tX int\n\t}")
			assert.Equal(t, fieldNames(ref.Fields), []string{"X"}, "the spelling keeps the embed")
		})

		t.Run("leaves a blank field out of an inline struct's fields", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "struct {\n\t\t_ int\n\t\tX int\n\t}")
			assert.Equal(t, fieldNames(ref.Fields), []string{"X"}, "padding binds nothing")
		})

		t.Run("records the type of each entry of an inline struct as a child", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "import (\n\t\"io\"\n\t\"time\"\n)\n\n",
				"struct {\n\t\tio.Reader\n\t\t_ int\n\t\tX, Y time.Duration\n\t}")
			assert.Equal(t, spellingsOf(ref.Elems), []string{"io.Reader", "int", "time.Duration"},
				"an embedded and a blank entry have a child too")
			assert.Equal(t, ref.Elems[0].Package, "io", "a child records the import path of its qualifier")
		})

		t.Run("gives each field of an inline struct a type of its own", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "struct{ X, Y int }")
			assert.Length(t, ref.Elems, 1, "the entry has one child")
			expect.NotEqual(t, ref.Fields[0].Type, ref.Elems[0], "the field and the child are two nodes",
				assert.ByIdentity())
			expect.NotEqual(t, ref.Fields[0].Type, ref.Fields[1].Type, "the two fields are two nodes",
				assert.ByIdentity())
		})
	})

	t.Run("inlineMethods", func(t *testing.T) {
		t.Parallel()

		t.Run("records an inline interface's methods as the reference's methods", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "interface {\n\t\tRead(p []byte) (n int, err error)\n\t\tclose()\n\t}")
			assert.Equal(t, methodNames(ref.Methods), []string{"Read", "close"}, "each method in order")
		})

		t.Run("lowers an inline method as abstract", func(t *testing.T) {
			t.Parallel()

			assert.True(t, inlineOf(t, "", "interface{ Close() error }").Methods[0].Abstract,
				"an interface's method has no body")
		})

		t.Run("lowers an inline method's variadic parameter as one argument's type", func(t *testing.T) {
			t.Parallel()

			p := inlineOf(t, "", "interface{ Log(format string, args ...int) }").Methods[0].Params[1]
			assert.Equal(t, p.Variadic, symbol.VariadicPositional, "the ... marks it")
			assert.Equal(t, p.Type.Spelling, "int", "typed as one argument it takes")
		})

		t.Run("lowers an inline method's named results", func(t *testing.T) {
			t.Parallel()

			m := inlineOf(t, "", "interface{ Read(p []byte) (n int, err error) }").Methods[0]
			assert.Equal(t, m.Returns[1].Name, "err", "each result under its name")
		})

		t.Run("lowers an inline method's unnamed parameter by its type", func(t *testing.T) {
			t.Parallel()

			p := inlineOf(t, "", "interface{ Write([]byte) error }").Methods[0].Params[0]
			assert.Equal(t, p.Type.Spelling, "[]byte", "the parameter's type")
		})

		t.Run("lowers an inline method's unnamed result by its type", func(t *testing.T) {
			t.Parallel()

			r := inlineOf(t, "", "interface{ Write([]byte) error }").Methods[0].Returns[0]
			assert.Equal(t, r.Type.Spelling, "error", "the result's type")
		})

		t.Run("leaves an embedded interface out of an inline interface's methods", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "interface {\n\t\tio.Reader\n\t\tClose() error\n\t}")
			assert.Equal(t, methodNames(ref.Methods), []string{"Close"}, "the spelling keeps the embed")
		})

		t.Run("leaves a constraint element out of an inline interface's methods", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "interface {\n\t\t~int\n\t\tString() string\n\t}")
			assert.Equal(t, methodNames(ref.Methods), []string{"String"}, "the spelling keeps the type set")
		})

		t.Run("leaves a blank method out of an inline interface's methods", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "interface {\n\t\t_()\n\t\tClose() error\n\t}")
			assert.Equal(t, methodNames(ref.Methods), []string{"Close"}, "a blank method binds nothing")
		})

		t.Run("records the types of an inline interface as children in the order of its elements", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "import (\n\t\"io\"\n\t\"time\"\n)\n\n",
				"interface {\n\t\tio.Reader\n\t\tRead(p []byte) (n int, err error)\n"+
					"\t\t~int | time.Duration\n\t\t_(s string)\n\t}")
			assert.Equal(t, spellingsOf(ref.Elems),
				[]string{"io.Reader", "[]byte", "int", "error", "~int | time.Duration", "string"},
				"an embed, the types of each method and a constraint element, a blank method included")
		})

		t.Run("records a function type element of an inline interface as one child", func(t *testing.T) {
			t.Parallel()

			ref := inlineOf(t, "", "interface{ func() }")
			assert.Length(t, ref.Elems, 1, "the element is one child")
			assert.Equal(t, ref.Elems[0].Form, symbol.FormFunc, "a function type and not a method")
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

// fieldsOf parses one file and returns the fields of its first
// declaration, a struct.
func fieldsOf(tb testing.TB, src string) []*node.Field {
	tb.Helper()

	file := onlyFile(tb, parsedFile(tb, nil, plugin.DepthFull, src))
	return file.Decls[0].(*node.Struct).Fields
}

// inlineOf parses one file whose first declaration is a struct H of
// one field of a type, and returns the field's type.
func inlineOf(tb testing.TB, imports, typ string) *node.TypeRef {
	tb.Helper()

	return fieldsOf(tb, "package p\n\n"+imports+"type H struct {\n\ta "+typ+"\n}\n")[0].Type
}

// boundOf parses one file whose first declaration is a struct G with the
// type parameter list params, and returns the bound of its first
// parameter. The file imports time.
func boundOf(tb testing.TB, params string) *node.TypeRef {
	tb.Helper()

	src := "package p\n\nimport \"time\"\n\ntype G[" + params + "] struct{ d time.Duration }\n"
	file := onlyFile(tb, parsedFile(tb, nil, plugin.DepthFull, src))
	return file.Decls[0].(*node.Struct).TypeParams[0].Bounds[0]
}

// spellingsOf returns the spellings of refs, in order.
func spellingsOf(refs []*node.TypeRef) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Spelling)
	}
	return out
}

// fieldNames returns the names of fields, in order.
func fieldNames(fields []*node.Field) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

// methodNames returns the names of methods, in order.
func methodNames(methods []*node.Method) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		out = append(out, m.Name)
	}
	return out
}
