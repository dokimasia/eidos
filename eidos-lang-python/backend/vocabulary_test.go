// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	"go.dokimi.dev/eidos/lang/textfmt"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// probeTemplate is the name of the template of the emitter that a body in
// the reference form executes.
const probeTemplate = "probe.tpl"

// wrapDocsAllocs is the wrap of a single-spaced text: the list of its lines,
// each of which is a part of the text.
const wrapDocsAllocs = 1

// filledLine is a word as wide as the documentation, so the wrap starts a
// line with the word after it.
var filledLine = strings.Repeat("a", textfmt.DocsWidth)

// TestVocabulary pins the names of the helpers of the template vocabulary,
// and the helper that each name calls in a template of a generator.
func TestVocabulary(t *testing.T) {
	t.Parallel()

	t.Run("names each helper that every backend declares under the name of the render package", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{
			backend.FuncDocs, backend.FuncSpell, backend.FuncSpellValue, backend.FuncTypeParams,
			backend.FuncTypeArgs, backend.FuncParams, backend.FuncResults, backend.FuncWrapDocs,
		}, []string{
			render.HelperDocs, render.HelperSpell, render.HelperSpellValue, render.HelperTypeParams,
			render.HelperTypeArgs, render.HelperParams, render.HelperResults, render.HelperWrapDocs,
		}, "the names of the shared helpers")
	})

	t.Run("names each helper of Python", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, []string{
			backend.FuncDecorators, backend.FuncStruct, backend.FuncInterface, backend.FuncEnum, backend.FuncMethod,
			backend.FuncFunction, backend.FuncAlias, backend.FuncConstant, backend.FuncVariable, backend.FuncModule,
		}, []string{
			"decorators", "struct", "interface", "enum", "method", "function", "alias", "constant", "variable",
			"module",
		}, "the names of the helpers of Python")
	})

	t.Run("WrapDocs", func(t *testing.T) {
		t.Parallel()

		t.Run("keeps a list marker on the line before it", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" - next"), []string{filledLine + " -", "next"},
				"griffe reads a docstring as Markdown")
		})

		t.Run("starts a line with a word that starts with @", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, backend.WrapDocs(filledLine+" @see next"), []string{filledLine, "@see next"},
				"a docstring has no block tags")
		})
	})

	tests := []struct {
		name       string
		template   string
		give       any
		giveParams emit.Slot[*emit.Param]
		giveReturn emit.Slot[*emit.Return]
		want       string
	}{
		{
			name: "calls Docstring under the name docs", template: "{{docs .Data}}",
			give: []string{"Load reads a row."}, want: "def load() -> None:\n\"\"\"Load reads a row.\"\"\"\n",
		},
		{
			name: "calls Spell under the name spell", template: "    x: {{spell .Data}}\n",
			give: &emit.TypeRef{Form: symbol.FormList, Elems: []*emit.TypeRef{{Spelling: "int"}}},
			want: "def load() -> None:\n    x: list[int]\n",
		},
		{
			name: "calls SpellValue under the name spellvalue", template: "    x = {{spellvalue .Data}}\n",
			give: &emit.TypeRef{Spelling: "Decimal", Package: "decimal"},
			want: "def load() -> None:\n    x = Decimal\n",
		},
		{
			name: "calls TypeParams under the name typeparams", template: "    # {{typeparams .Data}}\n",
			give: []*emit.TypeParam{{Name: "T", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}}},
			want: "def load() -> None:\n    # [T: Codec]\n",
		},
		{
			name:     "calls the type arguments of a list of references under the name typeargs",
			template: "    x: Box{{typeargs .Data}}\n", give: []*emit.TypeRef{{Spelling: "int"}, {Spelling: "str"}},
			want: "def load() -> None:\n    x: Box[int, str]\n",
		},
		{
			name:       "calls the parameter group of a slot under the name params",
			template:   "    def inner({{params .Decl.Params}}) -> None: ...\n",
			giveParams: emit.SlotOf(&emit.Param{Name: "key", Type: &emit.TypeRef{Spelling: "str"}}),
			want:       "def load(key: str) -> None:\n    def inner(key: str) -> None: ...\n",
		},
		{
			name:       "calls the result annotation of a slot under the name results",
			template:   "    def inner(){{results .Decl.Returns}}: ...\n",
			giveReturn: emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "int"}}),
			want:       "def load() -> int:\n    def inner() -> int: ...\n",
		},
		{
			name: "calls Decorators under the name decorators", template: "    # {{decorators .Data}}",
			give: &emit.Function{Name: "inner", Annotations: symbol.Annotations{{Name: "traced"}}},
			want: "def load() -> None:\n    # @traced\n",
		},
		{
			name: "calls WrapDocs under the name wrapdocs", template: "{{docs (wrapdocs .Data)}}",
			give: filledLine + " next", want: "def load() -> None:\n\"\"\"" + filledLine + "\nnext\n\"\"\"\n",
		},
	}
	// probe renders the function load with the parameters params and the
	// results returns, whose body executes the template of the emitter over
	// the data give.
	probe := func(
		t *testing.T, template string, give any, params emit.Slot[*emit.Param], returns emit.Slot[*emit.Return],
	) ([]plugin.RenderedFile, []diag.Diag) {
		t.Helper()

		e := plugin.NewEmit()
		assert.NoError(t, e.Add(plugin.Unit{
			Plugin: emitter, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
			Pkg: symbol.Identity{Lang: python.Lang, Package: unitPkg, Name: unitName, Kind: symbol.KindPackage},
			Decls: []symbol.Symbol{&emit.Function{
				Name: "load", Params: params, Returns: returns,
				Body: emit.Body{Ref: &emit.TemplateRef{Name: probeTemplate, Data: give}},
			}},
		}), "the unit arrives")
		return backendtest.Rendered(t, func(assert.TB) (kit.Backend, *backendtest.Fixture) {
			return backend.New(), &backendtest.Fixture{
				Emit: e, Schedule: []plugin.ID{emitter}, Policy: versionPolicy(t, python.Version312),
				Trees: map[plugin.ID]fs.FS{emitter: fstest.MapFS{probeTemplate: {Data: []byte(template)}}},
			}
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			files, diags := probe(t, tt.template, tt.give, tt.giveParams, tt.giveReturn)
			assert.Empty(t, diags, "the render reports nothing")
			assert.Length(t, files, 1, "the unit renders one file")
			assert.Contains(t, string(files[0].Body), tt.want, "the output of the helper")
		})
	}

	badParams := emit.SlotOf(&emit.Param{Name: "row-key"})
	badReturns := emit.SlotOf(&emit.Return{Type: &emit.TypeRef{Spelling: "A & B", Form: symbol.FormIntersection}})
	refusals := []struct {
		name     string
		template string
		give     any
		want     string
	}{
		{
			name: "reports the refusal of the parameter group of a slot", template: "{{params .Data}}",
			give: &badParams, want: `python: a parameter has an identifier as its name, and "row-key" is not one`,
		},
		{
			name: "reports the refusal of the result annotation of a slot", template: "{{results .Data}}",
			give: &badReturns, want: "python: A & B is an intersection of types, which Python declares as a Protocol",
		},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, diags := probe(t, tt.template, tt.give, emit.Slot[*emit.Param]{}, emit.Slot[*emit.Return]{})
			assert.Length(t, diags, 1, "the render reports the template")
			assert.Contains(t, diags[0].Msg, tt.want, "the refusal of the helper")
		})
	}
}

// WrapDocs allocates the list of the lines of a single-spaced text. The
// ordinary run, which runs no benchmark, checks that ceiling here.
func TestVocabularyAllocs(t *testing.T) {
	checkAllocs(t, vocabularyCalls())
}

// BenchmarkVocabulary measures the wrap of the documentation that a template
// passes to a docstring.
func BenchmarkVocabulary(b *testing.B) {
	benchCalls(b, vocabularyCalls())
}

// vocabularyCalls returns the call of WrapDocs over a text of two lines.
func vocabularyCalls() []allocCall {
	wrapped := filledLine + " next"
	var lines []string
	return []allocCall{{
		name: "WrapDocs", allocs: wrapDocsAllocs,
		call: func() { lines = backend.WrapDocs(wrapped) },
		check: func(tb assert.TB) {
			assert.Equal(tb, lines, []string{filledLine, "next"}, "WrapDocs wraps the text")
		},
	}}
}
