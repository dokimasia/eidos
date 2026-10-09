// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk/diag"
	"go.dokimi.dev/eidos/sdk/frontendtest"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The paths of the spec files of the fixtures, the brand of a parse, and
// the pinned values of the frontend.
const (
	shapePath    = "spec/shapes/probe.yaml"
	mixinPath    = "spec/mixins/probe.yaml"
	contractPath = "spec/contracts/probe.yaml"
	otherPath    = "spec/shapes/other.yaml"
	ghostPath    = "spec/shapes/ghost.yaml"
	brand        = "acme"
	version      = "1"
	selection    = "spec/**/*.yaml"
	commentMark  = "#"
	shapesDir    = "shapes"
	probeName    = "probe"
	otherName    = "other"
	claimText    = "a claim"
	paramDoc     = "a param"
	bindingDoc   = "a binding"
	writerName   = "writer"
	readerName   = "reader"
	beginRole    = "begin"
	endRole      = "end"
	limitKey     = "limit"
	readKey      = "read"
	axisKey      = "axis"
	modeKey      = "mode"
	sampleKey    = "sample"
	valueKey     = "value"
	storedKey    = "stored"
	arityJoin    = "="
)

// The three valid specs of the fixtures, one of each form. A case appends
// the sections that it states after the base, so the sections of the base
// keep their lines.
const (
	shapeBase = "name: probe\n" +
		"form: shape\n" +
		"claim: a claim\n" +
		"observation: an observation\n" +
		"falsifiability: a falsifiability\n" +
		"counterexamples:\n" +
		"  edge: an edge\n"
	mixinBase = "name: probe\n" +
		"form: mixin\n" +
		"claim: a claim\n" +
		"observation: an observation\n" +
		"falsifiability: a falsifiability\n" +
		"counterexamples:\n" +
		"  edge: an edge\n"
	contractBase = "name: probe\n" +
		"form: contract\n" +
		"claim: a claim\n" +
		"observation: an observation\n" +
		"falsifiability: a falsifiability\n" +
		"counterexamples:\n" +
		"  edge: an edge\n" +
		"roles:\n" +
		"  begin: {arity: one}\n" +
		"  end: {arity: optional}\n"
)

// unit is what one parse of the spec frontend left. It has the graph that
// the unit built, and the findings in report order.
type unit struct {
	graph    *plugin.GraphBuilder
	findings []diag.Diag
}

// parse parses one unit of the files through the spec frontend, its
// members in path order, and returns what the parse left. A parse that
// returns an error fails the test.
func parse(tb testing.TB, files map[string]string) unit {
	tb.Helper()

	fsys := fstest.MapFS{}
	var refs []plugin.SourceRef
	for _, path := range slices.Sorted(maps.Keys(files)) {
		fsys[path] = &fstest.MapFile{Data: []byte(files[path])}
		refs = append(refs, plugin.SourceRef{Path: path})
	}
	sink := diag.NewSink()
	f := specfront.New()
	u := plugin.NewSourceUnit(refs, fsys, plugin.DepthFull, f.Syntax(), brand, sink, f.Name())
	assert.NoError(tb, f.Parse(tb.Context(), u), "the unit parses")
	return unit{graph: u.Graph(), findings: slices.Collect(sink.All())}
}

// structs returns the structs that the unit declared, in declaration
// order.
func (u unit) structs() []*node.Struct {
	var out []*node.Struct
	for _, pkg := range u.graph.Packages() {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if st, is := d.(*node.Struct); is {
					out = append(out, st)
				}
			}
		}
	}
	return out
}

// stamps returns the stamps that the unit recorded on a declaration, by
// their keys.
func (u unit) stamps(subject symbol.Symbol) map[meta.KeyName]any {
	out := map[meta.KeyName]any{}
	for _, r := range u.graph.StampRecords() {
		if r.Subject == subject {
			out[r.Stamp.Key] = r.Stamp.Value
		}
	}
	return out
}

// The spec frontend declares each spec as a struct, and passes the
// conformance checks of a frontend that apply to a language without
// references.
func TestFrontend(t *testing.T) {
	t.Parallel()

	setup := func(assert.TB) (plugin.Frontend, *frontendtest.Fixture) {
		return specfront.New(), &frontendtest.Fixture{
			Sources: fstest.MapFS{
				shapePath:    {Data: []byte(shapeBase)},
				mixinPath:    {Data: []byte(mixinBase)},
				contractPath: {Data: []byte(contractBase)},
			},
			Keys: specfront.Keys,
		}
	}

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a frontend that parses deterministically", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertDeterministicParse(t, setup)
		})

		t.Run("returns a frontend that positions every finding", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertPositionedDiagnostics(t, setup)
		})

		t.Run("returns a frontend that drops no file", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertClassified(t, setup)
		})

		t.Run("returns a frontend that refuses its own outputs", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertOwnedExcluded(t, setup)
		})

		t.Run("returns a frontend whose unit keys fold every part", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertFingerprinted(t, setup)
		})

		t.Run("returns a frontend that reads through the unit alone", func(t *testing.T) {
			t.Parallel()

			frontendtest.AssertJailedReads(t, setup)
		})

		t.Run("returns a frontend of the language of the specs", func(t *testing.T) {
			t.Parallel()

			f := specfront.New()
			expect.Equal(t, f.Name(), specfront.ID, "the frontend reports under its name")
			expect.Equal(t, f.Lang(), specfront.Lang, "every spec is a declaration of the language of the specs")
			expect.Equal(
				t,
				f.Syntax(),
				plugin.CommentSyntax{Line: []string{commentMark}},
				"a YAML comment opens with #",
			)
			expect.Equal(t, f.Selection(), []string{selection}, "the frontend claims every YAML file below spec")
			versioned, held := f.(plugin.Versioned)
			assert.True(t, held, "the frontend states a version")
			expect.Equal(t, versioned.Version(), version, "the version is pinned")
		})
	})

	t.Run("Partition", func(t *testing.T) {
		t.Parallel()

		t.Run("makes each directory one unit in the order of its first file", func(t *testing.T) {
			t.Parallel()

			units, err := specfront.New().Partition(t.Context(), []plugin.SourceRef{
				{Path: mixinPath}, {Path: shapePath}, {Path: otherPath},
			}, nil)
			assert.NoError(t, err, "the partition needs no bytes")
			assert.Equal(t, units, [][]plugin.SourceRef{
				{{Path: mixinPath}},
				{{Path: shapePath}, {Path: otherPath}},
			}, "the mixins are one unit and the shapes another")
		})
	})

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		t.Run("declares a struct for each spec with the claim as its documentation", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{shapePath: shapeBase})
			assert.Empty(t, got.findings, "the spec is valid")
			structs := got.structs()
			assert.Length(t, structs, 1, "the unit declares the spec")
			expect.Equal(t, structs[0].Name, probeName, "the struct has the name of the spec")
			expect.Equal(t, structs[0].Doc, []string{claimText}, "the struct documents the claim")
			expect.Equal(
				t,
				got.stamps(structs[0]),
				map[meta.KeyName]any{specfront.KeyForm: string(specfront.FormShape)},
				"a shape that no detector reports has its form alone",
			)
		})

		t.Run("declares a package for the directory of the specs", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{shapePath: shapeBase})
			pkgs := got.graph.Packages()
			assert.Length(t, pkgs, 1, "the unit declares one package")
			expect.Equal(t, pkgs[0].Name, shapesDir, "the package has the name of the directory")
			assert.Length(t, pkgs[0].Files, 1, "the package has the spec file")
			expect.Equal(t, pkgs[0].Files[0].Path, shapePath, "the file has the path of the spec")
		})

		t.Run("stamps the detection and the precedence of a shape", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{shapePath: shapeBase +
				"detected: true\n" +
				"precedence:\n" +
				"  yields_to: [writer, reader]\n"})
			assert.Empty(t, got.findings, "the spec is valid")
			assert.Length(t, got.structs(), 1, "the unit declares the spec")
			assert.Equal(t, got.stamps(got.structs()[0]), map[meta.KeyName]any{
				specfront.KeyForm: string(specfront.FormShape), specfront.KeyDetected: true,
				specfront.KeyYields: []string{writerName, readerName},
			}, "the struct has the form, the detection and the yields_to list")
		})

		t.Run("stamps a documentary mixin", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{mixinPath: mixinBase + "documentary: true\n"})
			assert.Length(t, got.structs(), 1, "the unit declares the spec")
			assert.Equal(t, got.stamps(got.structs()[0]), map[meta.KeyName]any{
				specfront.KeyForm: string(specfront.FormMixin), specfront.KeyDocumentary: true,
			}, "the struct has the form and the documentary mark")
		})

		t.Run("stamps the roles of a contract in order with their arities", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{contractPath: contractBase})
			assert.Length(t, got.structs(), 1, "the unit declares the spec")
			assert.Equal(t, got.stamps(got.structs()[0]), map[meta.KeyName]any{
				specfront.KeyForm: string(specfront.FormContract),
				specfront.KeyRoles: []string{
					beginRole + arityJoin + string(specfront.ArityOne),
					endRole + arityJoin + string(specfront.ArityOptional),
				},
			}, "the struct has the roles in the order of the spec")
		})

		t.Run("declares a field for each param with the stamps of its values", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{contractPath: contractBase +
				"params:\n" +
				"  - {key: limit, type: int, minimum: 2, required: true, doc: a param}\n" +
				"  - {key: read, type: reference, resolve: callable-in-scope, roles: [begin], doc: a param}\n" +
				"  - {key: axis, type: reference, resolve: host-param, also_on: [read], counterexample: true, doc: a param}\n" +
				"  - {key: mode, type: string, excludes: [limit], doc: a param}\n"})
			assert.Empty(t, got.findings, "the spec is valid")
			assert.Length(t, got.structs(), 1, "the unit declares the spec")
			fields := got.structs()[0].Fields
			assert.Length(t, fields, 4, "the struct has a field for each param")
			tests := []struct {
				name   string
				typ    specfront.ParamType
				stamps map[meta.KeyName]any
			}{
				{
					limitKey,
					specfront.TypeInt,
					map[meta.KeyName]any{specfront.KeyMinimum: int64(2), specfront.KeyRequired: true},
				},
				{readKey, specfront.TypeReference, map[meta.KeyName]any{
					specfront.KeyResolve: string(specfront.ResolveCallableInScope),
					specfront.KeyApplies: []string{beginRole},
				}},
				{axisKey, specfront.TypeReference, map[meta.KeyName]any{
					specfront.KeyResolve: string(specfront.ResolveHostParam), specfront.KeyAlsoOn: []string{readKey},
					specfront.KeyCounterexample: true,
				}},
				{modeKey, specfront.TypeString, map[meta.KeyName]any{specfront.KeyExcludes: []string{limitKey}}},
			}
			for i, tt := range tests {
				expect.Equal(t, fields[i].Name, tt.name, "the field has the key of the param")
				expect.Equal(t, fields[i].Type.Spelling, string(tt.typ), "the field has the type of the param")
				expect.Equal(t, fields[i].Doc, []string{paramDoc}, "the field documents the param")
				expect.Equal(t, got.stamps(fields[i]), tt.stamps, "the field has the stamps of the values of the param")
			}
		})

		t.Run("declares a field for each binding after the params", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{shapePath: shapeBase +
				"params:\n" +
				"  - {key: sample, type: string, doc: a param}\n" +
				"bindings:\n" +
				"  value: {from: input, index: 0, doc: a binding}\n" +
				"  stored: {from: result, index: 1, doc: a binding}\n"})
			assert.Empty(t, got.findings, "the spec is valid")
			assert.Length(t, got.structs(), 1, "the unit declares the spec")
			fields := got.structs()[0].Fields
			assert.Length(t, fields, 3, "the struct has a field for the param and each binding")
			expect.Equal(
				t,
				[]string{fields[0].Name, fields[1].Name, fields[2].Name},
				[]string{sampleKey, valueKey, storedKey},
				"the bindings follow the params in the order of the file",
			)
			expect.Equal(t, fields[1].Type.Spelling, string(specfront.TypeReference), "a binding is a reference")
			expect.Equal(t, fields[1].Doc, []string{bindingDoc}, "the field documents the binding")
			expect.Equal(t, got.stamps(fields[2]), map[meta.KeyName]any{
				specfront.KeyFrom: string(specfront.SourceResult), specfront.KeyIndex: int64(1),
			}, "the field has the list and the index of the binding")
		})

		t.Run("declares no struct for a spec with a fault", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{otherPath: shapeBase})
			assert.NotEmpty(t, got.findings, "the spec is in a file of another name")
			assert.Empty(t, got.structs(), "the unit declares nothing for the spec")
		})

		t.Run("returns the error of a file that does not read", func(t *testing.T) {
			t.Parallel()

			f := specfront.New()
			u := plugin.NewSourceUnit(
				[]plugin.SourceRef{{Path: ghostPath}},
				fstest.MapFS{},
				plugin.DepthFull,
				f.Syntax(),
				brand,
				diag.NewSink(),
				f.Name(),
			)
			assert.HasError(t, f.Parse(t.Context(), u), "the file is not in the tree")
		})
	})

	t.Run("Resolve", func(t *testing.T) {
		t.Parallel()

		t.Run("returns no candidate", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, specfront.New().Resolve(plugin.ImportScope{}, otherName),
				"a spec refers to nothing outside itself")
		})
	})
}
