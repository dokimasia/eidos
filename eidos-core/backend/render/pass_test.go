// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render_test

import (
	"cmp"
	"errors"
	"io/fs"
	"path"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
	"time"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The fixture's names, which every spec of the package renders with:
// the pass, the plugin that emits, the routing key it emits under, the
// file the naming spells for that key, and the declarations the cases
// render.
const (
	// passName is the identity every fixture pass composes under.
	passName plugin.ID = "printer"
	// emitter is the plugin a fixture unit comes from.
	emitter plugin.ID = "gen"
	// storeKey is the routing key a fixture unit emits under.
	storeKey = "store.go"
	// storeFile is the filename [stubNaming] spells for storeKey.
	storeFile = "store_stub.txt"
	// alphaName and betaName name the fixture's structs.
	alphaName = "Alpha"
	betaName  = "Beta"
	// handleName names the fixture's callables.
	handleName = "Handle"
	// blockGroup names the group template a fixture cluster selects.
	blockGroup render.GroupName = "block"
	// missingField is a field no fixture value has, so failing, the
	// template that reads it, fails at execute time.
	missingField = "Missing"
	failing      = "{{." + missingField + "}}"
	// refusalReason is the reason a fixture language states for a
	// kind it refuses.
	refusalReason = "the fixture language declares no such construct"
)

// The binding fixture: a helper that qualifies a name through the
// file's import set, the packages it qualifies with, and the name the
// first package binds.
const (
	// qualifyHelper, declareHelper, itemHelper, claimHelper, referHelper
	// and clauseHelper are the vocabulary helpers [qualifying] declares.
	qualifyHelper = "qualify"
	declareHelper = "declare"
	itemHelper    = "item"
	claimHelper   = "claim"
	referHelper   = "refer"
	clauseHelper  = "clause"
	// storePkg and legacyPkg are two packages whose last segment is
	// storeLocal, so the second to bind takes a suffix.
	storePkg  = "svc/store"
	legacyPkg = "legacy/store"
	// storeLocal is the name storePkg binds when no other binding
	// takes it.
	storeLocal = "store"
	// rowName is the declaration a qualified spelling names.
	rowName = "Row"
)

// The fixture language's spellings: a struct, and a function that
// places its body.
const (
	structTpl   = "type {{.Name}} struct{}\n"
	functionTpl = "func {{.Name}}() {\n{{body .}}}\n"
)

// refName is the template every reference-form fixture body names.
const refName = "method1.tpl"

// The policy of a context: the target that declares it, its key and
// choices, and the helper that writes the choice of the key.
const (
	policyTarget plugin.Target    = "fixture"
	widthKey     plugin.PolicyKey = "fixture.width"
	narrowWidth  plugin.Choice    = "narrow"
	wideWidth    plugin.Choice    = "wide"
	widthHelper                   = "width"
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
	// annotationSkeleton is a file skeleton that spells the name and
	// the first argument of each annotation of the file's source file.
	annotationSkeleton = "{{range .Annotations}}{{.Name}} {{index .Args 0}}\n{{end}}"
	// constraintName and constraintExpr are the name and the argument
	// of the annotation of the fixture's source file.
	constraintName = "go:build"
	constraintExpr = "linux"
	// headerSkeleton is a file skeleton that spells the name and the
	// first argument of each annotation of the header of the file.
	headerSkeleton = "{{range .Header.Annotations}}{{.Name}} {{index .Args 0}}\n{{end}}"
	// headerStruct is a struct spelling that writes the first argument of
	// the first annotation of the header, which it reads through the
	// header builtin.
	headerStruct = "type {{.Name}} {{with header}}{{index (index .Annotations 0).Args 0}}{{end}}\n"
	// syntaxName and syntaxArg are the name and the argument of the
	// annotation of the header that [headed] states.
	syntaxName = "protobuf:syntax"
	syntaxArg  = "proto3"
)

// qualifiedStruct is a struct spelling that qualifies rowName from
// storePkg through the [qualifying] vocabulary, declaredStruct one that
// qualifies it through the name that the package of storePkg declares,
// and referredStruct one that qualifies it through the name of the file
// of storePkg that the file refers into.
var (
	qualifiedStruct = "type {{.Name}} " +
		action(qualifyHelper, strconv.Quote(storePkg), strconv.Quote(rowName)) + "\n"
	declaredStruct = "type {{.Name}} " +
		action(declareHelper, strconv.Quote(storePkg), strconv.Quote(rowName)) + "\n"
	referredStruct = "type {{.Name}} " +
		action(referHelper, strconv.Quote(storePkg), strconv.Quote(rowName)) + "\n"
)

// TestPass checks the procedure that every language shares. The pass
// renders the routed files with their kinds in canonical order,
// finalises each file and continues past a failure.
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
				name: "returns an error for a refused kind the language spells",
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
			assert.Empty(t, p.RefusedKinds(), "the language refuses nothing")
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

		t.Run("returns each file at its routed path with its package", func(t *testing.T) {
			t.Parallel()

			left := unitOf(emitter, leftPkg+"/"+storeKey, alphaName)
			left.Pkg = coretest.Struct(leftPkg, anchorName).ID
			right := unitOf(emitter, rightPkg+"/"+storeKey, betaName)
			right.Pkg = coretest.Struct(rightPkg, anchorName).ID
			files, _ := runPass(t, language(), seeded(t, left, right))
			assert.Equal(t, paths(files), []string{leftPkg + "/" + storeFile, rightPkg + "/" + storeFile},
				"two packages that spell one filename route to two paths")
			assert.NotEqual(t, files[0].Pkg, files[1].Pkg, "two packages")
		})

		t.Run("renders the routed files alone", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			e := seeded(t, unitOf(emitter, storeKey, alphaName), unitOf(emitter, userKey, betaName))
			routed := backendtest.Files(e, p)[:1]
			routed[0].Path = "gen/" + storeFile
			files, err := p.Render(&plugin.RenderContext{
				Emit: e, Files: routed, Sink: diag.NewSink(), Plugin: passName,
			})
			assert.NoError(t, err, "the pass renders every file")
			assert.Equal(t, paths(files), []string{"gen/" + storeFile}, "the one routed file at its path")
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

		t.Run("records every plugin that appended into a file's slots", func(t *testing.T) {
			t.Parallel()

			woven := unitOf(emitter, storeKey, alphaName)
			woven.Contributors = []plugin.ID{betaPlugin, alphaPlugin}
			files, sink := runPass(t, language(), seeded(t, woven))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, files[0].Plugins, []plugin.ID{alphaPlugin, betaPlugin, emitter},
				"the emitter and its contributors, distinct and sorted")
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
			assert.Empty(t, files[0].Sources,
				"a plan file derives from no declaration")
		})

		t.Run("reports findings under the context's plugin", func(t *testing.T) {
			t.Parallel()

			p, err := render.New(passName, language())
			assert.NoError(t, err, "the language composes")
			sink := diag.NewSink()
			e := seeded(t, unspelt())
			_, err = p.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, p), Sink: sink, Plugin: composedPlugin,
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
			e := seeded(t, unspelt())
			_, err = p.Render(&plugin.RenderContext{Emit: e, Files: backendtest.Files(e, p), Sink: sink})
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
			assert.Equal(t, files[0].Path, storeFile, "its sibling renders whole")
		})

		t.Run("returns the findings about a rendered file in its Findings", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t, unspelt(), unitOf(emitter, userKey, betaName)))
			assert.Length(t, files, 2, "both files render")
			assert.Length(t, files[0].Findings, 1, "the unspelt method of the store file has a finding")
			assert.Equal(t, files[0].Findings[0].Code, render.UnspeltKind, "the finding is the unspelt kind")
		})

		t.Run("returns no Findings for a rendered file without a finding", func(t *testing.T) {
			t.Parallel()

			files, _ := runPass(t, language(), seeded(t, unspelt(), unitOf(emitter, userKey, betaName)))
			assert.Length(t, files, 2, "both files render")
			assert.Empty(t, files[1].Findings, "the user file renders clean")
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

		t.Run("returns the same bytes for every run", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(l render.Language) ([]plugin.RenderedFile, error) {
				files, _ := runPass(t, l, seeded(t,
					unitOf(weaverPlugin, storeKey, omegaName),
					unitOf(emitter, storeKey, alphaName, betaName),
					unitOf(emitter, userKey, gammaName),
				))
				return files, nil
			}, language(), "byte identity is the contract")
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
			assert.Empty(t, files, "no half-assembled file is returned")
		})

		t.Run("withholds a file whose every declaration is skipped", func(t *testing.T) {
			t.Parallel()

			u := unspelt()
			u.Decls = u.Decls[1:]
			files, sink := runPass(t, language(), seeded(t, u))
			coretest.AssertCodes(t, sink, render.UnspeltKind)
			assert.Empty(t, files, "the skipped declaration's finding explains the missing file")
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

		t.Run("passes the annotations of the source file to the skeleton", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = annotationSkeleton
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			e := seeded(t, unitOf(emitter, storeKey))
			files := backendtest.Files(e, p)
			assert.Length(t, files, 1, "the unit routes into one file")
			files[0].Annotations = symbol.Annotations{{Name: constraintName, Args: []string{constraintExpr}}}
			sink := diag.NewSink()
			got, err := p.Render(&plugin.RenderContext{Emit: e, Files: files, Sink: sink, Plugin: passName})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertCodes(t, sink)
			assert.Length(t, got, 1, "the pass returns the file")
			assert.Equal(t, string(got[0].Body), constraintName+" "+constraintExpr+"\n",
				"the skeleton writes the annotation of the source file")
		})

		t.Run("passes the header of the file to the skeleton", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = headerSkeleton
			files, sink := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "the unit routes into one file")
			assert.Equal(t, string(files[0].Body), syntaxName+" "+syntaxArg+"\n",
				"the skeleton writes the annotation of the header")
		})

		t.Run("returns the header of the file to a kind template", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Kinds[symbol.KindStruct] = headerStruct
			files, sink := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			coretest.AssertCodes(t, sink)
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" "+syntaxArg+"\n",
				"the template reads the argument of the header")
		})

		t.Run("renders the skeleton of a file whose one declaration is an emit.File", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.File = nameSkeleton
			files, sink := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey))))
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "a file of a header alone renders")
			assert.Equal(t, string(files[0].Body), storeFile+"\n", "the skeleton alone")
		})

		t.Run("reports RefusedFact for the annotations of a header that the coverage refuses", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = headerCoverage(render.Refuses)
			_, sink := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			assert.HasSuffix(t, reported(t, sink, render.RefusedFact),
				symbol.FactAnnotations.String()+" stated on a "+symbol.KindFile.String(),
				"the finding names the fact of the file")
		})

		t.Run("renders a file whose header the coverage refuses", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = headerCoverage(render.Refuses)
			files, _ := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			assert.Length(t, files, 1, "the file renders without its header")
		})

		t.Run("reports RejectedFact for the annotations of a header that the coverage rejects", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = headerCoverage(render.Rejects)
			_, sink := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			coretest.AssertCodes(t, sink, render.RejectedFact)
		})

		t.Run("withholds a file whose header the coverage rejects", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Coverage = headerCoverage(render.Rejects)
			files, _ := runPass(t, l, seeded(t, headed(unitOf(emitter, storeKey, alphaName))))
			assert.Empty(t, files, "the target cannot write the file with the header or without it")
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
					f.Path+" imports what it binds")
			}
		})

		t.Run("binds an import under the name that the names of the context declare", func(t *testing.T) {
			t.Parallel()

			files := renderNamed(t, declaring)
			assert.Equal(t, string(files[0].Body),
				"use "+storePkg+" as "+storeDeclared+"\n"+
					"type "+alphaName+" "+storeDeclared+"."+rowName+"\n",
				"the import of the file binds the declared name")
		})

		t.Run("lists each package whose declared name the file read", func(t *testing.T) {
			t.Parallel()

			files := renderNamed(t, declaring)
			assert.Equal(t, files[0].Packages, []string{storePkg}, "the file read the name of the store package")
		})

		t.Run("lists no package for a context without names", func(t *testing.T) {
			t.Parallel()

			files := renderNamed(t, nil)
			assert.Empty(t, files[0].Packages, "the file read no declared name")
		})

		t.Run("gives the import set of each file the policy of the context", func(t *testing.T) {
			t.Parallel()

			l := language()
			l.Funcs = func(set *render.ImportSet) template.FuncMap {
				return template.FuncMap{widthHelper: func() plugin.Choice {
					c, _ := set.Policy().Lookup(widthKey)
					return c
				}}
			}
			l.Kinds[symbol.KindStruct] = "type {{.Name}} " + action(widthHelper) + "\n"
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			e := seeded(t, unitOf(emitter, storeKey, alphaName), unitOf(emitter, userKey, betaName))
			sink := diag.NewSink()
			files, err := p.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, p), Sink: sink, Plugin: passName, Policy: widthPolicy(t),
			})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 2, "the units route into two files")
			for _, f := range files {
				assert.HasSuffix(t, string(f.Body), " "+string(wideWidth)+"\n", f.Path+" reads the chosen width")
			}
		})

		t.Run("gives the import set of each file the files that it refers into", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = referredStruct
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			e := seeded(t, unitOf(emitter, storeKey, alphaName), unitOf(emitter, userKey, betaName))
			routed := backendtest.Files(e, p)
			assert.Length(t, routed, 2, "the units route into two files")
			routed[0].Referents = []symbol.Identity{
				{Lang: referentLang, Package: storePkg, Name: storeDeclared, Kind: symbol.KindPackage},
			}
			sink := diag.NewSink()
			files, err := p.Render(&plugin.RenderContext{Emit: e, Files: routed, Sink: sink, Plugin: passName})
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 2, "both files render")
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" "+storeDeclared+"."+rowName+"\n",
				"the file qualifies the name with the package of the file that it refers into")
			assert.Equal(t, string(files[1].Body), "type "+betaName+" "+rowName+"\n",
				"a file without referents writes the name bare")
		})

		t.Run("gives the import set of each file the package of the file", func(t *testing.T) {
			t.Parallel()

			l := binding()
			l.Kinds[symbol.KindStruct] = "type {{.Name}} {{" + clauseHelper + "}}\n"
			p, err := render.New(passName, l)
			assert.NoError(t, err, "the language composes")
			e := seeded(t, unitOf(emitter, storeKey, alphaName))
			routed := backendtest.Files(e, p)
			assert.Length(t, routed, 1, "the unit routes into one file")
			routed[0].Pkg = symbol.Identity{
				Lang: referentLang, Package: storePkg, Name: storeDeclared, Kind: symbol.KindPackage,
			}
			sink := diag.NewSink()
			files, err := p.Render(&plugin.RenderContext{Emit: e, Files: routed, Sink: sink, Plugin: passName})
			assert.NoError(t, err, "the pass renders the file")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "the file renders")
			assert.Equal(t, string(files[0].Body), "type "+alphaName+" "+storeDeclared+"\n",
				"the helper reads the name of the package clause of the file")
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
					"\n"+
					"type "+storeLocal+" "+storeLocal2+"."+rowName+"\n",
				"the import binds around the struct a later unit declares")
		})
	})
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

// The ceilings of [BenchmarkPass]. The canonical-scale renders were
// measured over 24 fresh processes, and each ceiling allows eight
// standard deviations above the mean: the render's worker goroutines
// and their frames allocate as the runtime schedules them.
const (
	// newPassAllocs is one composition of the fixture language. Most of
	// them are text/template's parse trees of the default file skeleton
	// and the two kind templates, and the function maps of the render's
	// builtins each template binds.
	newPassAllocs = 143
	// refusedKindsAllocs is one copy of a refusal map of one entry: the
	// map and its group.
	refusedKindsAllocs = 2
	// renderOneAllocs is one Render of one file of one struct: 27 for
	// the clone of the parsed templates and the eight builtins that bind
	// them to the file's import set, about 22 for the worker, its frame
	// and its import set, and about 23 for the file: the template's run
	// through reflection, its buffers and the rendered bytes.
	renderOneAllocs = 72
	// renderStructsAllocs is one render of 1,000 files of 200 structs:
	// 613,293 on average with a standard deviation of 6. Each struct's
	// template run through reflection allocates 3, and each file about
	// 13 for its buffers, its import set and its bytes.
	renderStructsAllocs = 613_293 + 8*6
	// renderOrderedAllocs is the same render in a language that orders
	// the declarations of a file: 613,364 on average over 8 runs with a
	// standard deviation of 11. Each of the four workers grows the memory
	// of the order once for its largest file, about 18 allocations, and a
	// file allocates nothing more for its order.
	renderOrderedAllocs = 613_364 + 8*11
	// renderBindingAllocs is the same render with every struct binding
	// one import through the vocabulary: 1,815,374 on average with a
	// standard deviation of 14. A binding of a package bound before
	// allocates nothing, and the 6 more each struct allocates are the
	// helper's own string join and text/template's reflective call,
	// which every helper call costs.
	renderBindingAllocs = 1_815_374 + 8*14
	// renderReferencesAllocs is one render of 1,000 files of 200
	// functions whose bodies each reference one template in the
	// emitting plugin's tree: 3,213,480 on average with a standard
	// deviation of 16, about 16 for each function's parsed reference
	// template and the body it places.
	renderReferencesAllocs = 3_213_480 + 8*16
)

// TestPassAllocs checks the ceilings of New, Coverage, RefusedKinds,
// FileName, SplitUnit and the render of one file in the ordinary run,
// which runs no benchmark. Only [BenchmarkPass] checks the ceilings of
// the renders at the canonical scale, because they take too long to
// repeat 101 times. cmp.Or keeps the first error of the calls of each
// count without allocating. The check runs alone, because the count
// includes the allocations of every goroutine.
func TestPassAllocs(t *testing.T) {
	l := language()
	var (
		p   *render.Pass
		err error
	)
	assert.MaxAllocs(t, func() {
		var nerr error
		p, nerr = render.New(passName, l)
		err = cmp.Or(err, nerr)
	}, newPassAllocs, "New allocates the parsed templates and the pass")
	assert.NoError(t, err, "the language composes")
	var coverage render.Coverage
	assert.MaxAllocs(t, func() { coverage = p.Coverage() }, 0, "Coverage allocates nothing")
	assert.False(t, coverage.Declared(), "the fixture language declares no coverage")
	refusing := refusingPass(t)
	var refused map[symbol.Kind]string
	assert.MaxAllocs(t, func() { refused = refusing.RefusedKinds() }, refusedKindsAllocs,
		"RefusedKinds allocates the copy it returns")
	assert.Length(t, refused, 1, "RefusedKinds returns the one refusal")
	u := unitOf(emitter, storeKey, alphaName)
	var name string
	assert.MaxAllocs(t, func() { name = p.FileName(u) }, 1, "FileName allocates the spelled name")
	assert.Equal(t, name, storeFile, "FileName spells the unit's file")
	var parts []plugin.Unit
	assert.MaxAllocs(t, func() { parts = p.SplitUnit(u) }, 1, "SplitUnit allocates the list of the whole unit")
	assert.Length(t, parts, 1, "an unsplit unit files whole")
	ctx := renderOne(t, p)
	var files []plugin.RenderedFile
	assert.MaxAllocs(t, func() {
		var rerr error
		files, rerr = p.Render(ctx)
		err = cmp.Or(err, rerr)
	}, renderOneAllocs, "Render allocates the call's frame and the file")
	assert.NoError(t, err, "the file renders")
	assert.Length(t, files, 1, "one file")
}

// BenchmarkPass measures the pass's methods, and the render at the
// canonical scale: 1000 per-package files of 200 declarations, 200k
// template executions, through a pass-through formatter, so the number
// is the pass and the engine, not a real language's spelling. Every
// render runs over a store and files routed before the measurement.
func BenchmarkPass(b *testing.B) {
	b.Run("New", func(b *testing.B) {
		l := language()
		c := bench.Start(b).MaxAllocs(newPassAllocs)
		defer c.End()
		var (
			p   *render.Pass
			err error
		)
		for c.Loop() {
			p, err = render.New(passName, l)
		}
		assert.NoError(b, err, "the language composes")
		assert.NotNil(b, p, "New returns the pass")
	})

	b.Run("Coverage", func(b *testing.B) {
		p := composedPass(b)
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var got render.Coverage
		for c.Loop() {
			got = p.Coverage()
		}
		assert.False(b, got.Declared(), "the fixture language declares no coverage")
	})

	b.Run("RefusedKinds", func(b *testing.B) {
		p := refusingPass(b)
		c := bench.Start(b).MaxAllocs(refusedKindsAllocs)
		defer c.End()
		var got map[symbol.Kind]string
		for c.Loop() {
			got = p.RefusedKinds()
		}
		assert.Length(b, got, 1, "RefusedKinds returns the one refusal")
	})

	b.Run("FileName", func(b *testing.B) {
		p, u := composedPass(b), unitOf(emitter, storeKey, alphaName)
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got string
		for c.Loop() {
			got = p.FileName(u)
		}
		assert.Equal(b, got, storeFile, "FileName spells the unit's file")
	})

	b.Run("SplitUnit", func(b *testing.B) {
		p, u := composedPass(b), unitOf(emitter, storeKey, alphaName)
		c := bench.Start(b).MaxAllocs(1)
		defer c.End()
		var got []plugin.Unit
		for c.Loop() {
			got = p.SplitUnit(u)
		}
		assert.Length(b, got, 1, "an unsplit unit files whole")
	})

	b.Run("Render", func(b *testing.B) {
		b.Run("one file of one struct", func(b *testing.B) {
			p := composedPass(b)
			ctx := renderOne(b, p)
			c := bench.Start(b).MaxAllocs(renderOneAllocs)
			defer c.End()
			var (
				files []plugin.RenderedFile
				err   error
			)
			for c.Loop() {
				files, err = p.Render(ctx)
			}
			assert.NoError(b, err, "the file renders")
			assert.Length(b, files, 1, "one file")
		})

		b.Run("1,000 files of 200 structs", func(b *testing.B) {
			e := benchStore(b, structDecl, benchStruct)
			benchRender(b, composedPass(b), e, nil, renderStructsAllocs)
		})

		b.Run("1,000 files of 200 structs in a language that orders them", func(b *testing.B) {
			e := benchStore(b, structDecl, benchStruct)
			l := language()
			l.Evaluated = extending
			l.Implemented = true
			p, err := render.New(passName, l)
			assert.NoError(b, err, "the ordering language composes")
			benchRender(b, p, e, nil, renderOrderedAllocs)
		})

		b.Run("1,000 files of 200 structs binding one import each", func(b *testing.B) {
			e := benchStore(b, structDecl, benchStruct)
			l := binding()
			l.Kinds[symbol.KindStruct] = qualifiedStruct
			p, err := render.New(passName, l)
			assert.NoError(b, err, "the binding language composes")
			benchRender(b, p, e, nil, renderBindingAllocs)
		})

		b.Run("1,000 files of 200 functions with referenced bodies", func(b *testing.B) {
			e := benchStore(b, func(path, name string) symbol.Symbol {
				f := &emit.Function{Origin: coretest.Struct(path, name).ID, Name: name}
				f.Body = refBody()
				return f
			}, benchFunc)
			benchRender(b, composedPass(b), e, refTree("\tref()\n"+action(render.BuiltinSlots)), renderReferencesAllocs)
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
		assert.NoError(b, e.Add(u), "the unit is added")
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

	routed := backendtest.Files(e, p)
	c := bench.Start(b).MaxAllocs(ceiling)
	defer c.End()
	var (
		files []plugin.RenderedFile
		sink  *diag.Sink
		err   error
	)
	for c.Loop() {
		sink = diag.NewSink()
		files, err = p.Render(&plugin.RenderContext{
			Emit: e, Files: routed, Trees: trees, Sink: sink, Plugin: passName,
		})
	}
	assert.NoError(b, err, "every file renders")
	assert.Length(b, files, benchPackages, "one file per package")
	assert.False(b, sink.Failed(), "every file renders clean")
}

// composedPass returns the pass over the fixture language.
func composedPass(tb assert.TB) *render.Pass {
	tb.Helper()

	p, err := render.New(passName, language())
	assert.NoError(tb, err, "the language composes")
	return p
}

// refusingPass returns the pass over the fixture language refusing the
// enum kind.
func refusingPass(tb assert.TB) *render.Pass {
	tb.Helper()

	l := language()
	l.Refused = map[symbol.Kind]string{symbol.KindEnum: refusalReason}
	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the refusing language composes")
	return p
}

// renderOne returns the render context of one unit of one struct,
// routed through p, after one render that builds what a process builds
// once.
func renderOne(tb assert.TB, p *render.Pass) *plugin.RenderContext {
	tb.Helper()

	e := seeded(tb, unitOf(emitter, storeKey, alphaName))
	ctx := &plugin.RenderContext{Emit: e, Files: backendtest.Files(e, p), Sink: diag.NewSink(), Plugin: passName}
	_, err := p.Render(ctx)
	assert.NoError(tb, err, "the file renders before the measurement")
	return ctx
}

// stubNaming spells every unit as word, key stem and a fixture
// extension: the shape a target's naming returns.
func stubNaming(u plugin.Unit) string {
	stem := strings.TrimSuffix(path.Base(u.Key), ".go")
	if stem == "." {
		stem = "plan"
	}
	return stem + "_" + u.Word + ".txt"
}

// language returns the smallest valid language: a struct
// spelling, a callable spelling that places its body, a scaffold
// printer for names and returns, a writer of values, and a pass-through
// formatter.
func language() render.Language {
	return render.Language{
		Kinds: map[symbol.Kind]string{
			symbol.KindStruct:   structTpl,
			symbol.KindFunction: functionTpl,
		},
		Naming:   stubNaming,
		Scaffold: scaffold,
		Value:    valueOf,
		Imports:  pathImports,
		Finalise: func(src []byte) ([]byte, error) { return src, nil },
	}
}

// pathImports renders the set's paths on one line, and nothing for an
// empty set.
func pathImports(set *render.ImportSet) string {
	if set.Len() == 0 {
		return ""
	}
	return "import (" + strings.Join(set.Paths(), " ") + ")\n"
}

// action spells one template action over the given words, so a
// fixture template names a builtin or a helper through its constant.
func action(words ...string) string {
	return "{{" + strings.Join(words, " ") + "}}"
}

// widthPolicy returns the resolved policy of policyTarget, which selects
// wideWidth for widthKey.
func widthPolicy(tb assert.TB) plugin.Policy {
	tb.Helper()

	p, err := plugin.NewPolicy(policyTarget, []plugin.PolicySpec{{
		Key: widthKey, Choices: []plugin.Choice{narrowWidth, wideWidth}, Default: narrowWidth,
		Doc: "the width of a line",
	}}, map[plugin.PolicyKey]plugin.Choice{widthKey: wideWidth})
	assert.NoError(tb, err, "the policy of the fixture resolves")
	return p
}

// helpers returns a vocabulary that binds nothing to the file's
// import set: every file calls the same functions.
func helpers(fm template.FuncMap) func(*render.ImportSet) template.FuncMap {
	return func(*render.ImportSet) template.FuncMap { return fm }
}

// qualifying is a vocabulary bound to the file's import set, the way a
// backend's speller is:
//
//   - qualify binds a package under the package's last segment and
//     spells a name through the bound name, or bare where the package
//     is the file's own.
//   - declare binds a package under the name that its package clause
//     declares, and under its last segment where the set has no such
//     name, and spells a name through the bound name.
//   - item imports one declaration and spells the name it binds.
//   - claim claims one declaration's simple name and spells the name,
//     or the path and the name where another binding takes it.
func qualifying(set *render.ImportSet) template.FuncMap {
	return template.FuncMap{
		qualifyHelper: func(pkg, name string) string {
			local := set.Bind(pkg, path.Base(pkg))
			if local == "" {
				return name
			}
			return local + "." + name
		},
		declareHelper: func(pkg, name string) string {
			bound := path.Base(pkg)
			if declared, known := set.Declared(pkg); known {
				bound = declared
			}
			return set.Bind(pkg, bound) + "." + name
		},
		itemHelper: func(pkg, name string) string {
			return set.BindItem(pkg, name, false)
		},
		claimHelper: func(pkg, name string) string {
			if set.Claim(pkg, name) {
				return name
			}
			return pkg + "." + name
		},
		referHelper: func(pkg, name string) string {
			if referent, held := set.Referent(pkg); held {
				return referent.Name + "." + name
			}
			return name
		},
		clauseHelper: func() string { return set.Package().Name },
	}
}

// binding returns the fixture language with the [qualifying]
// vocabulary and an import block that writes every entry on a line
// of its own, the bound name beside the path.
func binding() render.Language {
	l := language()
	l.Funcs = qualifying
	l.Imports = namedImports
	return l
}

// namedImports renders every entry on a line of its own, the bound
// name beside the path, so a case reads which name each import binds.
func namedImports(set *render.ImportSet) string {
	var b strings.Builder
	for _, e := range set.Entries() {
		b.WriteString("use " + e.Path + " as " + e.Name + "\n")
	}
	return b.String()
}

// scaffold spells the two statement kinds the fixtures use: a bare
// name evaluated for effect, and a return. A dotted name records
// its head as an import, the way a real printer records what it
// qualifies with.
func scaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	switch s.Kind {
	case emit.StmtExpr:
		if s.Value.Kind == emit.ExprValue {
			return nil, render.RefuseValue("fixture", "the fixture spells no value")
		}
		if head, _, qualified := strings.Cut(s.Value.Name, "."); qualified {
			set.Add(head)
		}
		return []byte("\t" + s.Value.Name + "()\n"), nil
	case emit.StmtReturn:
		return []byte("\treturn\n"), nil
	default:
		return nil, errors.New("the fixture spells names and returns only")
	}
}

// valueOf is the fixture's writer of values. It writes a value with a
// callee as the package and the name of the callee, and records the
// package as an import. It writes any other value as its text, and
// refuses a value without text, as a target refuses a form that it has
// no syntax for.
func valueOf(v emit.Value, set *render.ImportSet) (string, error) {
	switch {
	case !v.Callee.IsZero():
		set.Add(v.Callee.Package)
		return v.Callee.Package + "." + v.Callee.Name, nil
	case v.Text != "":
		return v.Text, nil
	default:
		return "", render.RefuseValue("fixture", "the fixture writes no %s without text", v.Kind)
	}
}

// unspellable returns a statement whose value the fixture language
// has no form for.
func unspellable() emit.Stmt {
	return emit.Stmt{
		Kind:  emit.StmtExpr,
		Value: emit.ValueExpr(emit.Literal(emit.LiteralInt, "1")),
	}
}

// call returns the one-line scaffold statement naming n.
func call(n string) emit.Stmt {
	return emit.Stmt{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: n}}
}

// fn returns a per-source unit containing one function whose body is
// body.
func fn(key, name string, body emit.Body) plugin.Unit {
	u := unitOf(emitter, key)
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, name).ID,
		Name:   name,
	}
	f.Body = body
	u.Decls = append(u.Decls, f)
	return u
}

// headed returns u with an emit.File before its declarations, whose one
// annotation states the syntax of the file.
func headed(u plugin.Unit) plugin.Unit {
	header := &emit.File{Annotations: symbol.Annotations{{Name: syntaxName, Args: []string{syntaxArg}}}}
	u.Decls = append([]symbol.Symbol{header}, u.Decls...)
	return u
}

// headerCoverage returns a coverage that renders every fact, with the
// verdict v for the annotations of a file.
func headerCoverage(v render.Verdict) render.Coverage {
	facts := map[symbol.Fact]render.Verdict{}
	for _, f := range symbol.Facts() {
		facts[f] = render.Renders
	}
	return render.Coverage{
		Facts:  facts,
		Except: map[symbol.Kind]map[symbol.Fact]render.Verdict{symbol.KindFile: {symbol.FactAnnotations: v}},
	}
}

// besideAlpha returns u with the struct alphaName before its
// declarations, so a case that skips a declaration of u still has a
// file to read.
func besideAlpha(u plugin.Unit) plugin.Unit {
	u.Decls = append([]symbol.Symbol{unitOf(emitter, u.Key, alphaName).Decls[0]}, u.Decls...)
	return u
}

// unitOf returns one flushed unit of structs with the given names.
func unitOf(p plugin.ID, key string, names ...string) plugin.Unit {
	decls := make([]symbol.Symbol, 0, len(names))
	for _, n := range names {
		decls = append(decls, &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, n).ID,
			Name:   n,
		})
	}
	return plugin.Unit{
		Plugin: p, Tag: "", Per: plugin.PerSource,
		Word: "stub", Key: key, Decls: decls,
	}
}

// seeded returns an emit store containing the given units.
func seeded(tb assert.TB, units ...plugin.Unit) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	assert.Total(tb, e.Add, units, "the fixture unit is added")
	return e
}

// runPass builds the pass over the language and renders the store,
// routed into files through the pass's own filename half.
func runPass(
	tb assert.TB, l render.Language, e *plugin.Emit,
) ([]plugin.RenderedFile, *diag.Sink) {
	tb.Helper()

	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p), Sink: sink, Plugin: passName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	return files, sink
}

// renderNamed renders one struct of the [binding] language that spells
// declaredStruct, with names as the names of the context, and returns
// the rendered files. It fails the test where the render reports a
// finding.
func renderNamed(tb assert.TB, names func(path string) (string, bool)) []plugin.RenderedFile {
	tb.Helper()

	l := binding()
	l.Kinds[symbol.KindStruct] = declaredStruct
	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	e := seeded(tb, unitOf(emitter, storeKey, alphaName))
	sink := diag.NewSink()
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p), Sink: sink, Plugin: passName, Names: names,
	})
	assert.NoError(tb, err, "the pass renders every file")
	coretest.AssertCodes(tb, sink)
	assert.Length(tb, files, 1, "the unit routes into one file")
	return files
}

// refTree returns the emitting plugin's tree containing src under
// [refName].
func refTree(src string) map[plugin.ID]fs.FS {
	return map[plugin.ID]fs.FS{
		emitter: fstest.MapFS{refName: &fstest.MapFile{Data: []byte(src)}},
	}
}

// refBody returns a body referencing [refName] and nothing else, so
// a case adds only the slots it tests.
func refBody() emit.Body {
	return emit.Body{Ref: &emit.TemplateRef{Name: refName}}
}

// refused returns a statement the fixture's scaffold cannot spell:
// the printer failure an error path needs.
func refused() emit.Stmt {
	return emit.Stmt{Kind: emit.StmtGuard, Name: "err"}
}

// method returns a per-source unit containing one method whose body
// is body, the second callable kind the body builtin takes.
func method(key, name string, body emit.Body) plugin.Unit {
	u := unitOf(emitter, key)
	m := &emit.Method{
		Origin: coretest.Method(coretest.StorePath, coretest.StructName, name).ID,
		Name:   name,
	}
	m.Body = body
	u.Decls = append(u.Decls, m)
	return u
}

// renderRef renders one function whose body is b through the
// language l, with trees as the emitting plugin's template trees, and
// returns the file's bytes beside the run's findings. An empty result
// means the file was withheld.
func renderRef(
	tb assert.TB, l render.Language, trees map[plugin.ID]fs.FS, b emit.Body,
) (string, *diag.Sink) {
	tb.Helper()

	p, err := render.New(passName, l)
	assert.NoError(tb, err, "the language composes")
	sink := diag.NewSink()
	e := seeded(tb, fn(storeKey, handleName, b))
	files, err := p.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, p),
		Trees: trees, Sink: sink, Plugin: passName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	if len(files) == 0 {
		return "", sink
	}
	return string(files[0].Body), sink
}

// reported returns the message of the first finding under code,
// which is what a case asserting the wording reads.
func reported(tb assert.TB, sink *diag.Sink, code diag.Code) string {
	tb.Helper()

	coretest.AssertReports(tb, sink, code)
	for d := range sink.All() {
		if d.Code == code {
			return d.Msg
		}
	}
	return ""
}
