// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package catalog_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape"
	"go.dokimi.dev/eidos/plugin/shape/catalog"
	"go.dokimi.dev/eidos/plugin/shape/internal/shapetest"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The import path and the directory of package detectors, and the
// suffixes of its Go files and its test files.
const (
	detectorsPath = "go.dokimi.dev/eidos/plugin/shape/detectors"
	detectorsDir  = "../detectors"
	goSuffix      = ".go"
	testSuffix    = "_test.go"
)

// The names of the types of the corpus and of the struct of its methods.
const (
	valueName  = "Value"
	metaName   = "Meta"
	sourceName = "Source"
	shapesName = "Shapes"
)

// The references of the corpus.
var (
	intRef  = &node.TypeRef{Spelling: shapetest.Int}
	boolRef = &node.TypeRef{Spelling: shapetest.Bool}
	valueID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    valueName,
		Kind:    symbol.KindStruct,
	}
	metaID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    metaName,
		Kind:    symbol.KindStruct,
	}
	sourceID = symbol.Identity{
		Lang:    shapetest.Lang,
		Package: shapetest.Path,
		Name:    sourceName,
		Kind:    symbol.KindInterface,
	}
	valueRef    = &node.TypeRef{Spelling: valueName, Target: valueID}
	metaRef     = &node.TypeRef{Spelling: metaName, Target: metaID}
	sourceRef   = &node.TypeRef{Spelling: sourceName, Target: sourceID}
	optionalRef = &node.TypeRef{Form: symbol.FormOptional, Elems: []*node.TypeRef{valueRef}}
	listRef     = &node.TypeRef{Form: symbol.FormList, Elems: []*node.TypeRef{valueRef}}
	streamRef   = &node.TypeRef{Form: symbol.FormStream, Elems: []*node.TypeRef{valueRef}}
)

// corpusEntry is one callable of the corpus and the shape that ranks first
// on it.
type corpusEntry struct {
	method *node.Method
	want   shape.Shape
}

// corpusOf returns one method of the struct host for each detected shape,
// with the signature that the shape describes.
func corpusOf(host string) []corpusEntry {
	getAll := shapetest.Method(host, "GetAll", []*node.TypeRef{ctxRef, str}, listRef, failure)
	getAll.Params[1].Variadic = symbol.VariadicPositional
	return []corpusEntry{
		{shapetest.Method(host, "Count", []*node.TypeRef{ctxRef}, intRef, failure), shape.Aggregator},
		{
			shapetest.Method(host, "Store", []*node.TypeRef{ctxRef, valueRef}, valueRef, failure),
			shape.AnsweringWriter,
		},
		{getAll, shape.BatchReader},
		{shapetest.Method(host, "Close", nil, failure), shape.Closer},
		{shapetest.Method(host, "Set", []*node.TypeRef{ctxRef, str, valueRef}, failure), shape.CompositeWriter},
		{shapetest.Method(host, "Add", []*node.TypeRef{intRef, intRef}, intRef), shape.Computation},
		{shapetest.Method(host, "Delete", []*node.TypeRef{ctxRef, str}, failure), shape.Deleter},
		{shapetest.Method(host, "Start", []*node.TypeRef{ctxRef}, failure), shape.Lifecycle},
		{shapetest.Method(host, "Lookup", []*node.TypeRef{str}, valueRef, metaRef, boolRef), shape.Lookup},
		{
			shapetest.Method(host, "Pair", []*node.TypeRef{ctxRef}, intRef, valueRef, failure),
			shape.MultiAggregator,
		},
		{shapetest.Method(host, "Record", []*node.TypeRef{ctxRef, str, str, str}, failure), shape.MultiArgWriter},
		{
			shapetest.Method(host, "GetWithMeta", []*node.TypeRef{ctxRef, str}, valueRef, metaRef, failure),
			shape.MultiReader,
		},
		{shapetest.Method(host, "Mutate", []*node.TypeRef{optionalRef}), shape.Mutator},
		{shapetest.Method(host, "Ref", []*node.TypeRef{ctxRef, str}, optionalRef), shape.PointerReader},
		{shapetest.Method(host, "Err", nil, failure), shape.PoisonAccessor},
		{shapetest.Method(host, "Ready", nil, boolRef), shape.Predicate},
		{shapetest.Method(host, "Get", []*node.TypeRef{ctxRef, str}, valueRef, failure), shape.Reader},
		{shapetest.Method(host, "Peek", []*node.TypeRef{ctxRef, str}, valueRef), shape.ReaderNoError},
		{shapetest.Method(host, "Find", []*node.TypeRef{str}, valueRef, boolRef), shape.ReaderWithBool},
		{
			shapetest.Method(host, "Consume", []*node.TypeRef{ctxRef, sourceRef}, intRef, failure),
			shape.StreamConsumer,
		},
		{shapetest.Method(host, "All", []*node.TypeRef{ctxRef}, streamRef), shape.StreamReader},
		{shapetest.Method(host, "Reset", nil), shape.VoidLifecycle},
		{shapetest.Method(host, "Save", []*node.TypeRef{ctxRef, valueRef}, failure), shape.Writer},
	}
}

// Detections lists the detectors in the order of precedence, so the
// plugin shape stamps the shape that the yields_to lists rank first.
func TestRegistryWiring(t *testing.T) {
	t.Parallel()

	t.Run("Detections", func(t *testing.T) {
		t.Parallel()

		t.Run("lists every detected shape once", func(t *testing.T) {
			t.Parallel()

			detections := catalog.Detections()
			got := make([]shape.Shape, 0, len(detections))
			for _, d := range detections {
				got = append(got, d.Shape)
			}
			var want []shape.Shape
			for _, s := range shape.Specs() {
				if s.Detected {
					want = append(want, shape.Shape(s.Name))
				}
			}
			assert.Permutation(t, got, want, "the wiring has a detector for each detected spec")
		})

		t.Run("lists each exported function of package detectors", func(t *testing.T) {
			t.Parallel()

			dir := filepath.FromSlash(detectorsDir)
			entries, err := os.ReadDir(dir)
			assert.NoError(t, err, "the directory of package detectors reads")
			fset := token.NewFileSet()
			var exported []string
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), goSuffix) || strings.HasSuffix(e.Name(), testSuffix) {
					continue
				}
				f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
				assert.NoError(t, err, "the file "+e.Name()+" parses")
				for _, d := range f.Decls {
					if fn, isFunc := d.(*ast.FuncDecl); isFunc && fn.Recv == nil && fn.Name.IsExported() {
						exported = append(exported, detectorsPath+"."+fn.Name.Name)
					}
				}
			}
			detections := catalog.Detections()
			wired := make([]string, 0, len(detections))
			for _, d := range detections {
				wired = append(wired, runtime.FuncForPC(reflect.ValueOf(d.Detect).Pointer()).Name())
			}
			assert.Permutation(t, wired, exported, "the wiring lists each exported function of package detectors")
		})

		t.Run("lists each shape after every shape that it yields to", func(t *testing.T) {
			t.Parallel()

			placed := map[shape.Shape]bool{}
			for _, d := range catalog.Detections() {
				s, _ := shape.SpecOf(d.Shape)
				for _, y := range s.YieldsTo {
					expect.True(t, placed[y], "the shape "+string(d.Shape)+" comes after "+string(y))
				}
				placed[d.Shape] = true
			}
		})

		t.Run("returns a new slice on each call", func(t *testing.T) {
			t.Parallel()

			first := catalog.Detections()
			first[0].Shape = ghostName
			assert.NotEqual(t, catalog.Detections()[0].Shape, shape.Shape(ghostName),
				"a change of the list of one call leaves another call unchanged")
		})

		t.Run("ranks the shape of each callable of the corpus first", func(t *testing.T) {
			t.Parallel()

			entries, detected := classifyCorpus(t)
			for _, e := range entries {
				got := detected[e.method.Name]
				assert.NotEmpty(t, got, "a detector reports "+e.method.Name)
				expect.Equal(t, shape.Shape(got[0]), e.want, "the shape of "+e.method.Name+" ranks first")
			}
		})

		t.Run("orders every pair of shapes of one callable by the yields_to lists", func(t *testing.T) {
			t.Parallel()

			_, detected := classifyCorpus(t)
			for name, shapes := range detected {
				for i, first := range shapes {
					for _, later := range shapes[i+1:] {
						expect.True(t, yields(later, first, map[string]bool{}),
							"on "+name+", the shape "+later+" yields to "+first+" through the yields_to lists")
					}
				}
			}
		})
	})
}

// corpusPackage returns the package of the corpus. It declares the types
// that the signatures of the corpus refer to, and the methods of
// [corpusOf] on each host. It also returns the entries of the first host.
func corpusPackage(hosts ...string) (*node.Package, []corpusEntry) {
	decls := []node.Declaration{
		&node.Struct{ID: valueID, Name: valueName},
		&node.Struct{ID: metaID, Name: metaName},
		&node.Interface{ID: sourceID, Name: sourceName},
	}
	var first []corpusEntry
	for i, host := range hosts {
		entries := corpusOf(host)
		if i == 0 {
			first = entries
		}
		for _, e := range entries {
			decls = append(decls, e.method)
		}
	}
	return shapetest.Package(decls...), first
}

// classifyCorpus classifies the corpus, and returns its entries and the
// detected shapes of each method by its name.
func classifyCorpus(t *testing.T) ([]corpusEntry, map[string][]string) {
	t.Helper()

	pkg, entries := corpusPackage(shapesName)
	r := shapetest.New(t, pkg)
	assert.Empty(t, r.Classify(t), "the corpus classifies without a finding")
	detected := map[string][]string{}
	r.Consume(t, sdk.OnMethod(func(m *sdk.MethodMatch, _ *sdk.Emitter) error {
		detected[m.Method.Name], _ = sdk.Fact(m, meta.Named[[]string](shape.KeyDetected))
		return nil
	}))
	return entries, detected
}

// yields reports whether the shape from yields to the shape to through a
// chain of yields_to lists, without passing a shape twice.
func yields(from, to string, seen map[string]bool) bool {
	seen[from] = true
	s, _ := shape.SpecOf(from)
	for _, y := range s.YieldsTo {
		if string(y) == to || !seen[string(y)] && yields(string(y), to, seen) {
			return true
		}
	}
	return false
}
