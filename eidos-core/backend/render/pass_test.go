// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"errors"
	"io/fs"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The pass's fixture: the packages, keys, plugins and declarations
// the cases route and render.
const (
	// leftPkg and rightPkg are two packages that spell one filename.
	leftPkg  = "example.com/left"
	rightPkg = "example.com/right"
	// anchorName names the declaration a package identity is taken
	// from.
	anchorName = "Anchor"
	// userKey, brokenKey, alphaKey and betaKey are further routing
	// keys, and userFile, alphaFile and betaFile the files they spell.
	userKey   = "user.go"
	brokenKey = "broken.go"
	alphaKey  = "alpha.go"
	betaKey   = "beta.go"
	userFile  = "user_stub.txt"
	alphaFile = "alpha_stub.txt"
	betaFile  = "beta_stub.txt"
	// weaverPlugin contributes to a file another plugin also emits.
	weaverPlugin plugin.ID = "weaver"
	// composedPlugin is the identity a composition renders under.
	composedPlugin plugin.ID = "composed"
	// brokenName names the declaration the formatter fails on.
	brokenName = "Broken"
	// omegaName, gammaName and registryName name further structs.
	omegaName    = "Omega"
	gammaName    = "Gamma"
	registryName = "Registry"
	// unparseable is the error a failing fixture formatter returns.
	unparseable = "unparseable"
	// nameSkeleton is a file skeleton that spells the file's name
	// alone, so a file of no declarations still renders bytes.
	nameSkeleton = "{{.Name}}\n"
)

// qualifiedStruct is a struct spelling that qualifies rowName from
// storePkg through the [qualifying] vocabulary.
var qualifiedStruct = "type {{.Name}} " +
	action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName)) + "\n"

// The pass is the procedure every language shares: group through the
// naming, render kinds in canonical order, finalise per file and
// continue past a failure.
func TestPass(t *testing.T) {
	t.Parallel()

	t.Run("New", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error naming every gap of an empty language", func(t *testing.T) {
			t.Parallel()

			_, err := render.New(passName, render.Language{})
			assert.HasError(t, err, "a language declaring nothing composes into nothing")
			for _, gap := range []string{
				"spells no kinds", "spells no filenames", "spells no scaffolding",
				"renders no import block", "declares no formatter",
			} {
				assert.Contains(t, err.Error(), gap, "the error names "+gap)
			}
		})

		t.Run("returns an error for a kind template that does not parse", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindEnum] = "{{"
			_, err := render.New(passName, l)
			assert.HasError(t, err, "an unparseable spelling never renders")
			assert.Contains(t, err.Error(), symbol.KindEnum.String(), "naming the kind")
		})

		t.Run("returns an error for a group template that does not parse", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Groups = map[render.GroupName]string{blockGroup: "{{"}
			_, err := render.New(passName, l)
			assert.HasError(t, err, "an unparseable group spelling never renders")
			assert.Contains(t, err.Error(), string(blockGroup), "naming the group")
		})

		t.Run("returns an error for a file skeleton that does not parse", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = "{{"
			_, err := render.New(passName, l)
			assert.HasError(t, err, "an unparseable skeleton never assembles a file")
			assert.Contains(t, err.Error(), "file skeleton", "naming the skeleton")
		})

		t.Run("parses the templates against the vocabulary's names", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = qualifiedStruct
			_, err := render.New(passName, l)
			assert.NoError(t, err, "a helper bound per file resolves at the parse")
		})

		t.Run("returns an error for a Cluster without group templates", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Cluster = func([]symbol.Symbol) []render.Clustered { return nil }
			_, err := render.New(passName, l)
			assert.HasError(t, err, "clustering needs group templates")
			assert.Contains(t, err.Error(), "group templates", "naming the gap")
		})

		refusals := []struct {
			name string
			give map[symbol.Kind]string
			want string
		}{
			{
				name: "returns an error for a kind the language spells and refuses",
				give: map[symbol.Kind]string{symbol.KindStruct: refusalReason},
				want: "spells and refuses the " + symbol.KindStruct.String() + " kind",
			},
			{
				name: "returns an error for a refusal without a reason",
				give: map[symbol.Kind]string{symbol.KindEnum: ""},
				want: "refuses the " + symbol.KindEnum.String() + " kind without a reason",
			},
		}
		for _, tt := range refusals {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				l := language()
				l.Refused = tt.give
				_, err := render.New(passName, l)
				assert.HasError(t, err, "a contradictory refusal never composes")
				assert.Contains(t, err.Error(), tt.want, "the error names the kind")
			})
		}
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		// composed composes the fixture language refusing the enum
		// kind, and returns the pass beside the map the language
		// declared.
		composed := func(tb assert.TB) (*render.Pass, map[symbol.Kind]string) {
			tb.Helper()

			l := language()
			l.Refused = map[symbol.Kind]string{symbol.KindEnum: refusalReason}
			p, err := render.New(passName, l)
			assert.NoError(tb, err, "the language composes")
			return p, l.Refused
		}

		t.Run("returns the refusals the language declares", func(t *testing.T) {
			t.Parallel()

			p, _ := composed(t)
			assert.Equal(t, p.RefusedKinds(), map[symbol.Kind]string{symbol.KindEnum: refusalReason},
				"the consumer reads the refusals the render reports")
		})

		t.Run("returns a map the caller may modify", func(t *testing.T) {
			t.Parallel()

			p, _ := composed(t)
			delete(p.RefusedKinds(), symbol.KindEnum)
			assert.Equal(t, p.RefusedKinds()[symbol.KindEnum], refusalReason,
				"the pass keeps the refusal it composed")
		})

		t.Run("returns the refusals the language declared at New", func(t *testing.T) {
			t.Parallel()

			p, declared := composed(t)
			delete(declared, symbol.KindEnum)
			assert.Equal(t, p.RefusedKinds()[symbol.KindEnum], refusalReason,
				"a later change to the language's map changes no composed pass")
		})

		t.Run("returns no refusal for a language that declares none", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			assert.Length(t, p.RefusedKinds(), 0, "the language refuses nothing")
		})
	})

	t.Run("Coverage", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the language's declared coverage", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = render.Coverage{Facts: map[symbol.Fact]render.Verdict{
				symbol.FactAbstract: render.Refuses,
			}}
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			assert.Equal(t, p.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses, "the consumer reads the data the guard reads")
		})

		t.Run("returns an undeclared coverage for a language that declares none", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			assert.False(t, p.Coverage().Declared(), "the guard is off")
		})
	})

	t.Run("Render", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a nil context", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			_, err = p.Render(nil)
			assert.HasError(t, err, "a nil context names no store")
			assert.Contains(t, err.Error(), "emit store", "naming what it needs")
		})

		t.Run("returns an error for a context without an emit store", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			_, err = p.Render(&plugin.RenderContext{Sink: diag.NewSink()})
			assert.HasError(t, err, "a context without a store has nothing to render")
			assert.Contains(t, err.Error(), "emit store", "naming what it needs")
		})

		t.Run("returns an error for a context without a sink", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			_, err = p.Render(&plugin.RenderContext{Emit: seeded(t)})
			assert.HasError(t, err, "a context without a sink has nowhere to report")
			assert.Contains(t, err.Error(), "sink", "naming what it needs")
		})

		t.Run("renders two files for two packages that spell one filename", func(t *testing.T) {
			t.Parallel()

			left := unitOf(emitter, leftPkg+"/"+storeKey, alphaName)
			left.Pkg = coretest.Struct(leftPkg, anchorName).ID
			right := unitOf(emitter, rightPkg+"/"+storeKey, betaName)
			right.Pkg = coretest.Struct(rightPkg, anchorName).ID
			files, _ := runPass(t, language(), seeded(t, left, right))
			assert.Length(t, files, 2, "the package addresses the file")
			assert.Equal(t, files[0].Name, files[1].Name, "one spelled name")
			assert.NotEqual(t, files[0].Pkg, files[1].Pkg, "two packages")
		})

		t.Run("records every plugin that contributed to a file", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				unitOf(betaPlugin, storeKey, betaName),
				unitOf(alphaPlugin, storeKey, alphaName),
			))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "two plugins sharing a filename assemble one file")
			assert.Equal(t, files[0].Plugins, []plugin.ID{alphaPlugin, betaPlugin},
				"every emitter, distinct and sorted")
		})

		t.Run("records each routing key a file derives from once", func(t *testing.T) {
			t.Parallel()

			files, sink := runPass(t, language(), seeded(t,
				unitOf(betaPlugin, storeKey, betaName),
				unitOf(alphaPlugin, storeKey, alphaName),
			))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, files[0].Sources, []string{storeKey}, "the shared key, once")
		})

		t.Run("records no source for a plan file", func(t *testing.T) {
			t.Parallel()

			plan := unitOf(emitter, "", registryName)
			plan.Per = plugin.PerPlan
			files, _ := runPass(t, language(), seeded(t, plan))
			assert.Length(t, files, 1, "the plan unit renders")
			assert.Equal(t, files[0].Plugins, []plugin.ID{emitter}, "its emitter alone")
			assert.Length(t, files[0].Sources, 0,
				"a plan file derives from no declaration")
		})

		t.Run("reports findings under the context's plugin", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			_, err = p.Render(&plugin.RenderContext{
				Emit: seeded(t, unspelt()), Sink: sink, Plugin: composedPlugin,
			})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertReports(t, sink, render.UnspeltKind)
			for d := range sink.All() {
				assert.Equal(t, d.Origin, composedPlugin,
					"the composition's identity, not the pass's own name")
			}
		})

		t.Run("reports findings under the pass's name for a context without a plugin", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			_, err = p.Render(&plugin.RenderContext{Emit: seeded(t, unspelt()), Sink: sink})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertReports(t, sink, render.UnspeltKind)
			for d := range sink.All() {
				assert.Equal(t, d.Origin, passName, "the pass's own name")
			}
		})

		t.Run("renders the remaining files past a format failure", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Finalise = func(src []byte) ([]byte, error) {
				if strings.Contains(string(src), brokenName) {
					return nil, errors.New(unparseable)
				}
				return src, nil
			}
			files, sink := runPass(t, l, seeded(t,
				unitOf(emitter, brokenKey, brokenName),
				unitOf(emitter, storeKey, alphaName),
			))
			coretest.AssertCodes(t, sink, render.UnformattedFile)
			assert.Length(t, files, 1, "the unformatted file is withheld")
			assert.Equal(t, files[0].Name, storeFile, "its sibling renders whole")
		})

		t.Run("reports findings in file order whatever order the workers finish in", func(t *testing.T) {
			t.Parallel()

			released := make(chan struct{})
			l := language()
			l.Finalise = func(src []byte) ([]byte, error) {
				if strings.Contains(string(src), alphaName) {
					// The first file waits for the second to finish, so
					// with two workers the second file's finding is
					// reported first. One worker cannot run the second
					// file concurrently, so the wait ends by timeout and
					// the order is file order either way.
					select {
					case <-released:
					case <-time.After(2 * time.Second):
					}
					return nil, errors.New(unparseable)
				}
				defer close(released)
				return nil, errors.New(unparseable)
			}
			_, sink := runPass(t, l, seeded(t,
				unitOf(emitter, alphaKey, alphaName),
				unitOf(emitter, betaKey, betaName),
			))
			got := []string{}
			for d := range sink.All() {
				got = append(got, d.Pos.File)
			}
			assert.Equal(t, got, []string{alphaFile, betaFile}, "the report order is the file order")
		})

		t.Run("positions findings at the package-qualified file", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Finalise = func([]byte) ([]byte, error) { return nil, errors.New(unparseable) }
			u := unitOf(emitter, leftPkg+"/"+storeKey, alphaName)
			u.Pkg = coretest.Struct(leftPkg, anchorName).ID
			_, sink := runPass(t, l, seeded(t, u))
			coretest.AssertReports(t, sink, render.UnformattedFile)
			for d := range sink.All() {
				assert.Equal(t, d.Pos.File, leftPkg+"/"+storeFile,
					"the position names the package the file belongs to")
			}
		})

		t.Run("returns the same bytes for two runs", func(t *testing.T) {
			t.Parallel()

			build := func() *plugin.Emit {
				return seeded(t,
					unitOf(weaverPlugin, storeKey, omegaName),
					unitOf(emitter, storeKey, alphaName, betaName),
					unitOf(emitter, userKey, gammaName),
				)
			}
			first, _ := runPass(t, language(), build())
			second, _ := runPass(t, language(), build())
			assert.Equal(t, second, first, "byte identity is the contract")
		})

		t.Run("reports RefusedTemplate naming a file the skeleton fails on", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = failing
			_, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Contains(t, reported(t, sink, render.RefusedTemplate), storeFile,
				"the finding names the file")
		})

		t.Run("withholds a file the skeleton fails on", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = failing
			files, _ := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			assert.Length(t, files, 0, "no half-assembled file is returned")
		})

		t.Run("withholds a file whose every declaration is skipped", func(t *testing.T) {
			t.Parallel()

			u := unspelt()
			u.Decls = u.Decls[1:]
			files, sink := runPass(t, language(), seeded(t, u))
			coretest.AssertCodes(t, sink, render.UnspeltKind)
			assert.Length(t, files, 0, "the skipped declaration's finding explains the missing file")
		})

		t.Run("renders the skeleton of a file without declarations", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = nameSkeleton
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey)))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "a file of no declarations skips none")
			assert.Equal(t, string(files[0].Body), storeFile+"\n", "the skeleton alone")
		})

		t.Run("binds the language vocabulary to the file's import set", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = qualifiedStruct
			files, sink := runPass(t, l, seeded(t, unitOf(emitter, storeKey, alphaName)))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal+"\n"+
					"type "+alphaName+" "+storeLocal+"."+rowName+"\n",
				"the helper's binding is the file's import")
		})

		t.Run("renders the import in every file that binds it", func(t *testing.T) {
			t.Parallel()

			// More files than workers, so one worker renders two files
			// through one set.
			n := runtime.GOMAXPROCS(0) + 1
			units := make([]plugin.Unit, 0, n)
			for i := range n {
				units = append(units, unitOf(emitter, strconv.Itoa(i)+".go", alphaName))
			}
			l := binding()
			l.Kinds[symbol.KindStruct] = qualifiedStruct
			files, sink := runPass(t, l, seeded(t, units...))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, n, "one file per unit")
			for _, f := range files {
				assert.HasPrefix(t, string(f.Body), "use "+storePkg+" as "+storeLocal+"\n",
					f.Name+" imports what it binds")
			}
		})

		t.Run("reserves a name the file declares before the first declaration renders", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = qualifiedStruct
			files, sink := runPass(t, l, seeded(t,
				unitOf(alphaPlugin, storeKey, alphaName),
				unitOf(betaPlugin, storeKey, storeLocal),
			))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeLocal2+"\n"+
					"type "+alphaName+" "+storeLocal2+"."+rowName+"\n"+
					"type "+storeLocal+" "+storeLocal2+"."+rowName+"\n",
				"the import binds around the struct a later unit declares")
		})
	})
}

// unspelt returns a unit whose second declaration has no template in
// the fixture language, so a render reports one finding.
func unspelt() plugin.Unit {
	u := unitOf(emitter, storeKey, alphaName)
	u.Decls = append(u.Decls, &emit.Method{
		Origin: coretest.Method(coretest.StorePath, coretest.StructName, handleName).ID,
		Name:   handleName,
	})
	return u
}

// The benchmarks' scale: per-package files of per-file declarations.
const (
	benchPackages = 1_000
	benchDecls    = 200
	// benchPkgPrefix opens every benchmark package's path, and
	// benchStruct and benchFunc every declaration's name.
	benchPkgPrefix = "example.com/pkg"
	benchStruct    = "S"
	benchFunc      = "F"
)

// benchStore returns an emit store of [benchPackages] per-package
// units, each of [benchDecls] declarations that decl builds from a
// package path and a name.
func benchStore(b *testing.B, decl func(path, name string) symbol.Symbol, prefix string) *plugin.Emit {
	b.Helper()

	e := plugin.NewEmit()
	for i := range benchPackages {
		path := benchPkgPrefix + strconv.Itoa(i)
		u := unitOf(emitter, path+"/"+storeKey)
		u.Pkg = coretest.Struct(path, anchorName).ID
		for d := range benchDecls {
			u.Decls = append(u.Decls, decl(path, prefix+strconv.Itoa(d)))
		}
		if err := e.Add(u); err != nil {
			b.Fatalf("Add: unexpected error: %v", err)
		}
	}
	return e
}

// structDecl builds one benchmark struct.
func structDecl(path, name string) symbol.Symbol {
	return &emit.Struct{Origin: coretest.Struct(path, name).ID, Name: name}
}

// benchRender renders e through p once per iteration under the
// allocation ceiling, with trees as the emitting plugin's trees.
func benchRender(b *testing.B, p *render.Pass, e *plugin.Emit, trees map[plugin.ID]fs.FS, ceiling uint64) {
	b.Helper()

	c := bench.Start(b).MaxAllocs(ceiling)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := p.Render(&plugin.RenderContext{
			Emit: e, Trees: trees, Sink: sink, Plugin: passName,
		})
		if err != nil {
			b.Fatalf("Render: unexpected error: %v", err)
		}
		if len(files) != benchPackages || sink.Failed() {
			b.Fatal("every file renders clean")
		}
	}
}

// BenchmarkPass measures the procedure at the canonical scale: 1000
// per-package files of 200 declarations, 200k template executions,
// through a pass-through formatter, so the number is the pass and
// the engine, not a real language's spelling.
func BenchmarkPass(b *testing.B) {
	e := benchStore(b, structDecl, benchStruct)
	p, err := render.New(passName, language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}
	benchRender(b, p, e, nil, 680_000)
}

// BenchmarkPassBinding measures the same scale with every declaration
// binding one import through the vocabulary, so the number is the
// per-reference cost of the file's name assignment. A binding of a
// package bound before allocates nothing. The six allocations each
// declaration adds over [BenchmarkPass] are the helper's own string
// join and text/template's reflective call, which every helper call
// costs: 1.82M allocations were measured, and the ceiling is 1.9M.
func BenchmarkPassBinding(b *testing.B) {
	e := benchStore(b, structDecl, benchStruct)
	l := binding()
	l.Kinds[symbol.KindStruct] = qualifiedStruct
	p, err := render.New(passName, l)
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}
	benchRender(b, p, e, nil, 1_900_000)
}

// BenchmarkPassReferences measures the body-claiming path at the
// same scale: 1000 files of 200 functions, each body a reference to
// one template in the emitting plugin's tree, so every declaration
// executes a parsed reference template.
func BenchmarkPassReferences(b *testing.B) {
	e := benchStore(b, func(path, name string) symbol.Symbol {
		f := &emit.Function{Origin: coretest.Struct(path, name).ID, Name: name}
		f.Body = refBody()
		return f
	}, benchFunc)
	p, err := render.New(passName, language())
	if err != nil {
		b.Fatalf("New: unexpected error: %v", err)
	}
	benchRender(b, p, e, refTree("\tref()\n"+action(render.BuiltinSlots)), 3_500_000)
}
