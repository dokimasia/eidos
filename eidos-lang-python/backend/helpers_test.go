// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/python/backend"
	kit "go.dokimi.dev/eidos/sdk/backend"
	"go.dokimi.dev/eidos/sdk/backendtest"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// emitter is the plugin of the unit that the specs that render emit.
const emitter plugin.ID = "gen"

// The word, the key, the package and the package name of the unit that the
// specs that render emit. The package is the module of the source file of
// the unit, so the unit routes to the module shop.models_gen beside it.
const (
	unitWord = "gen"
	unitKey  = "shop/models.src"
	unitPkg  = "shop/models"
	unitName = "models"
)

// home is the package path of the module of the unit that the specs render,
// and the home of the import set of a writer of the specs.
const home = "shop/models_gen"

// The names of the declarations and the members that two or more specs
// write.
const (
	rowName   = "Row"
	nodeName  = "Node"
	phaseName = "Phase"
	idField   = "id"
	nameField = "name"
)

// The marks of a bracket group that the vocabulary writes, which the
// specs state in their expectations and in the inputs of the layout.
const (
	hugMark     = "\x01"
	listMark    = "\x02"
	sepMark     = "\x03"
	closeMark   = "\x04"
	commentMark = "\x05"
)

// The types that the specs of the writers give their members, decimalRef, a
// class of another module of the standard library, and bytesRef, a class of
// the module builtins. A writer reads a type and never writes one, so
// parallel cases share them. The settle of a render rewrites the types of
// its declarations in place, so a spec that renders builds its own types.
// The lowering replaces each intersection in place, so a spec that lowers
// one builds its own types too.
var (
	intRef     = &emit.TypeRef{Spelling: "int"}
	strRef     = &emit.TypeRef{Spelling: "str"}
	rowRef     = &emit.TypeRef{Spelling: rowName}
	decimalRef = &emit.TypeRef{Spelling: "Decimal", Package: "decimal"}
	bytesRef   = &emit.TypeRef{Spelling: "bytes", Package: "builtins"}
)

// boundParam is a type parameter with a bound, which the specs declare
// beside a parameter of the same name without one.
var boundParam = &emit.TypeParam{Name: "T", Bounds: []*emit.TypeRef{{Spelling: "Codec"}}}

// The markers of a class that the specs state: a TypedDict, and the
// keywords of a class statement.
var (
	typedDictMarker = symbol.Annotation{Name: string(python.TypedDictMarker)}
	keywordsMarker  = symbol.Annotation{Name: string(python.KeywordsMarker), Args: []string{"frozen=True"}}
)

// partialKeyword is the keyword of the class statement of a TypedDict whose
// keys are optional unless Required states otherwise.
const partialKeyword = "total=False"

// overloadDecorator is the decorator of typing that marks an overload of a
// function or a method.
const overloadDecorator = "overload"

// The import markers of a header that the specs of the writers of a kind
// state: the import of json, and an argument that is no import statement,
// with the refusal of that argument.
var (
	jsonImport = symbol.Annotation{Name: string(python.ImportMarker), Args: []string{"import json"}}
	badImport  = symbol.Annotation{Name: string(python.ImportMarker), Args: []string{"print(1)"}}
)

// badImportRefusal is the refusal of the argument of [badImport].
const badImportRefusal = `python: an argument of the import marker is no import statement: "print(1)"`

// allocCall is one call that an allocation test and a benchmark share: the
// method that it calls, which is the name of its benchmark, the case that it
// measures where the method has more than one call, its ceiling, the call,
// and the check of the result of the call. A case whose ceiling only a
// benchmark checks sets bench, which measures the case in place of the
// call, and no list that an allocation test reads contains it.
type allocCall struct {
	name     string
	caseName string
	allocs   uint64
	call     func()
	check    func(tb assert.TB)
	bench    func(b *testing.B)
}

// checkAllocs checks the ceiling of every call in the ordinary run, and the
// result of each call.
func checkAllocs(t *testing.T, calls []allocCall) {
	t.Helper()

	for _, c := range calls {
		msg := c.name + " allocates within its ceiling"
		if c.caseName != "" {
			msg = c.name + " for " + c.caseName + " allocates within its ceiling"
		}
		assert.MaxAllocs(t, c.call, c.allocs, msg)
		c.check(t)
	}
}

// benchCalls measures every call under the bench contract at its ceiling:
// one sub-benchmark for each method, in the order of the first call of each
// method, and inside it one for each case of a method with cases.
func benchCalls(b *testing.B, calls []allocCall) {
	b.Helper()

	var methods []string
	byMethod := map[string][]allocCall{}
	for _, c := range calls {
		if _, seen := byMethod[c.name]; !seen {
			methods = append(methods, c.name)
		}
		byMethod[c.name] = append(byMethod[c.name], c)
	}
	for _, name := range methods {
		cases := byMethod[name]
		b.Run(name, func(b *testing.B) {
			if len(cases) == 1 && cases[0].caseName == "" {
				benchCall(b, cases[0])
				return
			}
			for _, tt := range cases {
				b.Run(tt.caseName, func(b *testing.B) { benchCall(b, tt) })
			}
		})
	}
}

// benchCall measures one call under the bench contract at its ceiling, and
// checks the result of the last call. The contract warms up with one call,
// so the count leaves out what the first call initialises. A case that sets
// bench runs it instead.
func benchCall(b *testing.B, tt allocCall) {
	b.Helper()

	if tt.bench != nil {
		tt.bench(b)
		return
	}
	c := bench.Start(b).Warmup(1).MaxAllocs(tt.allocs)
	defer c.End()
	for c.Loop() {
		tt.call()
	}
	tt.check(b)
}

// versionPolicy returns the policy of the Python target that chooses the
// version v.
func versionPolicy(tb assert.TB, v plugin.Choice) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(python.Target, python.Policies(), map[plugin.PolicyKey]plugin.Choice{
		python.VersionPolicy: v,
	})
	assert.NoError(tb, err, "the policy chooses a declared version")
	return p
}

// newWriter returns a writer of a module, or of a stub file where stub is
// true, for the version v, and the import set of its file, whose home is
// [home].
func newWriter(tb assert.TB, v plugin.Choice, stub bool) (*backend.Writer, *render.ImportSet) {
	tb.Helper()

	set := &render.ImportSet{}
	set.SetHome(home)
	set.SetPolicy(versionPolicy(tb, v))
	return backend.NewWriter(set, stub), set
}

// importsOf returns the imports that set records, without the entries that
// are no imports, whose paths start with a NUL byte: the type variables,
// the public names and the modules that the file imports lazily.
func importsOf(set *render.ImportSet) []render.Entry {
	imports := []render.Entry{}
	for _, e := range set.Entries() {
		if e.Path[0] != 0 {
			imports = append(imports, e)
		}
	}
	return imports
}

// rendered renders the declarations decls as the one unit of a fixture
// through b under the policy p, and returns the body of the one file of the
// render and the findings of the settle and of the render.
func rendered(tb assert.TB, b kit.Backend, p plugin.Policy, decls ...symbol.Symbol) (string, []diag.Diag) {
	tb.Helper()

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(plugin.Unit{
		Plugin: emitter, Per: plugin.PerSource, Word: unitWord, Key: unitKey,
		Pkg:   symbol.Identity{Lang: python.Lang, Package: unitPkg, Name: unitName, Kind: symbol.KindPackage},
		Decls: decls,
	}), "the unit arrives")
	files, diags := backendtest.Rendered(tb, func(assert.TB) (kit.Backend, *backendtest.Fixture) {
		return b, &backendtest.Fixture{Emit: e, Schedule: []plugin.ID{emitter}, Policy: p}
	})
	assert.Length(tb, files, 1, "the unit renders one file")
	return string(files[0].Body), diags
}
