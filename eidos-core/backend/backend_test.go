// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend_test

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/backend"
	"go.dokimi.dev/eidos/core/backend/backendtest"
	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
	"go.dokimi.dev/eidos/core/symbol"
)

// The kit's fixture: the backend's identity, the unit it renders,
// and the helpers its vocabulary declares.
const (
	// kitName and plainName name a fully declared backend and a
	// hookless one.
	kitName   plugin.ID = "printer"
	plainName plugin.ID = "plain"
	// kitTarget is the target both declare.
	kitTarget plugin.Target = "stub"
	// kitEmitter is the plugin the fixture unit comes from.
	kitEmitter plugin.ID = "gen"
	// kitWord and kitExt spell the fixture's one filename.
	kitWord = "stub"
	kitExt  = ".txt"
	// rowName and loadName name the fixture unit's struct and function.
	rowName  = "Row"
	loadName = "Load"
	// runtimePkg is the import the fixture scaffold records.
	runtimePkg = "stub/runtime"
	// upHelper is the fixture vocabulary's one helper.
	upHelper = "up"
	// blockGroup names a group template.
	blockGroup render.GroupName = "block"
	// qualifyHelper binds a package through the file's import set,
	// storePkg is the package it binds, and storeLocal the name.
	qualifyHelper = "qualify"
	storePkg      = "svc/store"
	storeLocal    = "store"
	// kitVersion is the version the fixture declares, so a built
	// backend's version is distinguishable from the undeclared empty
	// one.
	kitVersion = "3.2.1"
	// cursorSuffix names the companion declaration the fixture's
	// lowering produces beside every struct.
	cursorSuffix = "Cursor"
	// respellPrefix is what the fixture's name convention puts in
	// front of every declared name.
	respellPrefix = "t_"
	// kitReason is the reason the fixture states for a kind it
	// refuses.
	kitReason = "the fixture language declares no such construct"
	// kitIndent is the member indent that the member case declares, and
	// memberMethod a method template that places its content through the
	// memberbody builtin.
	kitIndent    = "\t"
	memberMethod = "func {{.Name}}() {\n{{memberbody .}}}\n"
)

// The ceilings of a declaration's steps.
const (
	// newAllocs is one new declaration: the builder, the kind, refusal
	// and group maps of its language, and its helper set.
	newAllocs = 5
	// mergeAllocs is one KindTemplates, RefusedKinds or Groups of one
	// entry into a new declaration: 4 for the entry's sorted keys, which
	// slices.Sorted over maps.Keys builds, and the first group of the map
	// the entry merges into.
	mergeAllocs = 5
	// funcsAllocs is one Funcs of the fixture's vocabulary into a new
	// declaration: the import set the kit hands the part, 2 for the
	// vocabulary the part returns, 4 for the helper names' sorted list,
	// the helper set's first group, and the list of parts.
	funcsAllocs = 9
	// buildAllocs is one Build of the fixture declaration. Most of them
	// are text/template's: the parse trees of the file skeleton and the
	// two kind templates, and the function maps of the render's builtins
	// it binds each template to. The composed pass and the lowered
	// backend make the rest.
	buildAllocs = 167
	// buildSeamsAllocs is one Build of the fixture declaration with both
	// settle seams: Build's, and the backend that composes the seams.
	buildSeamsAllocs = buildAllocs + 1
	// policiesAllocs is one Policies of one spec into a new declaration:
	// the list of specs.
	policiesAllocs = 1
)

// kitText is the spelling that the fixture's spoke gives a text shape.
const kitText = "text"

// kitTextRef is the reference that the fixture's spoke returns without
// allocating, and widthPolicy is the one policy of the fixture's target.
var (
	kitTextRef  = &emit.TypeRef{Spelling: kitText}
	widthPolicy = plugin.PolicySpec{
		Key: "stub.width", Choices: []plugin.Choice{"narrow", "wide"}, Default: "narrow",
		Doc: "the width at which the fixture target spells a number",
	}
)

// kitSpoke spells a text shape as the fixture's text reference, and
// refuses every other shape.
func kitSpoke(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	if s.Form != symbol.FormText {
		return nil, errors.New("the fixture spells text alone")
	}
	return kitTextRef, nil
}

// setter is one field setter of a declaration, called through a
// function made before any measurement.
type setter struct {
	name string
	set  func(*backend.Builder) *backend.Builder
}

// merge is one method that merges entries into a declaration, with the
// ceiling of one entry merged into a new declaration.
type merge struct {
	name   string
	allocs uint64
	set    func(*backend.Builder) *backend.Builder
}

// kitFile is the fixture's file skeleton, spelling a comment so a
// declared skeleton is distinguishable from the kit default.
const kitFile = "// {{.Name}}\n{{imports}}{{decls}}"

// kitBody is the file the full fixture declaration renders: the
// skeleton's comment, the scaffold's import, the struct through the
// shared helper and, after a blank line, the function around its
// scaffolded body.
var kitBody = "// " + kitWord + kitExt + "\n" +
	"import (" + runtimePkg + ")\n" +
	"type " + strings.ToUpper(rowName) + " struct{}\n" +
	"\n" +
	"func " + loadName + "() {\n\treturn\n}\n"

// The backend kit is the write side's authoring builder: the
// declaration is data, Build lowers it to the render pass, and
// the returned value implements every role the plan validates.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend named as declared", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().Name(), kitName,
				"the name is the identity")
		})

		t.Run("returns a backend for the declared target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().Target(), kitTarget,
				"the target as declared")
		})

		t.Run("returns a backend with the renderer role", func(t *testing.T) {
			t.Parallel()

			_, renders := kitBackend(kitName, kitTarget).Build().(plugin.Renderer)
			assert.True(t, renders, "a kit backend renders")
		})

		t.Run("returns a backend with the declared comment syntax", func(t *testing.T) {
			t.Parallel()

			b := kitBackend(kitName, kitTarget).Build()
			syntax, held := b.(interface{ Syntax() plugin.CommentSyntax })
			assert.True(t, held, "the output contract reads the comment syntax")
			assert.Equal(t, syntax.Syntax(), kitSyntax(), "the syntax as declared")
		})

		t.Run("renders a file through every declared piece", func(t *testing.T) {
			t.Parallel()

			files := kitRender(t, kitBackend(kitName, kitTarget).Build())
			assert.Length(t, files, 1, "the fixture assembles one file")
			assert.Equal(t, files[0].Path, kitWord+kitExt, "named by the naming")
			assert.Equal(t, string(files[0].Body), kitBody,
				"the skeleton, vocabulary, kinds and scaffold all spell")
		})

		t.Run("renders the same bytes for every build", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(name plugin.ID) ([]plugin.RenderedFile, error) {
				return kitRender(t, kitBackend(name, kitTarget).Build()), nil
			}, kitName, "a build is deterministic")
		})

		t.Run("renders the bytes a hand-built pass over the same language renders", func(t *testing.T) {
			t.Parallel()

			built := kitRender(t, kitBackend(kitName, kitTarget).Build())
			pass, err := render.New(kitName, kitLanguage())
			assert.NoError(t, err, "the same language composes by hand")
			e := kitStore(t)
			direct, err := pass.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, pass), Sink: diag.NewSink(), Plugin: kitName,
			})
			assert.NoError(t, err, "the hand-built pass renders every file")
			assert.Equal(t, built, direct, "the kit adds spelling and no semantics")
		})

		defects := []struct {
			name  string
			build func()
		}{
			{
				name:  "panics for an empty name",
				build: func() { kitBackend("", kitTarget).Build() },
			},
			{
				name:  "panics for a zero target",
				build: func() { kitBackend(kitName, "").Build() },
			},
			{
				name: "panics for one kind spelt twice",
				build: func() {
					kitBackend(kitName, kitTarget).KindTemplates(kitStructs()).Build()
				},
			},
			{
				name: "panics for a template that does not parse",
				build: func() {
					kitBackend(kitName, kitTarget).
						KindTemplates(map[symbol.Kind]string{symbol.KindEnum: "{{"}).
						Build()
				},
			},
			{
				name: "panics for one group spelt twice",
				build: func() {
					kitBackend(kitName, kitTarget).
						Groups(map[render.GroupName]string{blockGroup: "types\n"}).
						Groups(map[render.GroupName]string{blockGroup: "again\n"}).
						Build()
				},
			},
			{
				name: "panics for a cluster without group templates",
				build: func() {
					kitBackend(kitName, kitTarget).
						Cluster(func([]symbol.Symbol) []render.Clustered {
							return nil
						}).
						Build()
				},
			},
			{
				name: "panics for a policy outside the target's namespace",
				build: func() {
					stray := widthPolicy
					stray.Key = "golang.width"
					kitBackend(kitName, kitTarget).Policies(stray).Build()
				},
			},
		}
		for _, tt := range defects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Panics(t, tt.build, "a wrong declaration panics on the first Build in any test")
			})
		}

		t.Run("renders one file per unit the declared split returns", func(t *testing.T) {
			t.Parallel()

			b := kitBackend(kitName, kitTarget).
				Split(func(u plugin.Unit) []plugin.Unit {
					out := make([]plugin.Unit, 0, len(u.Decls))
					for i, d := range u.Decls {
						su := u
						su.Decls = u.Decls[i : i+1]
						su.Word = strings.ToLower(d.Kind().String())
						out = append(out, su)
					}
					return out
				}).
				Build()
			names := make([]string, 0, 2)
			for _, f := range kitRender(t, b) {
				names = append(names, f.Path)
			}
			assert.Equal(t, names, []string{
				strings.ToLower(symbol.KindFunction.String()) + kitExt,
				strings.ToLower(symbol.KindStruct.String()) + kitExt,
			}, "the split reshapes units before the naming")
		})

		settling := func() plugin.Backend {
			return kitBackend(kitName, kitTarget).
				Lower(func(s symbol.Symbol) ([]symbol.Symbol, error) {
					return []symbol.Symbol{s}, nil
				}).
				Respell(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
					return name, nil
				}).
				Build()
		}

		t.Run("returns a backend with the construct seam for a lowering", func(t *testing.T) {
			t.Parallel()

			_, lowers := settling().(plugin.Lowerer)
			assert.True(t, lowers, "the construct seam is declared")
		})

		t.Run("returns a backend with the name seam for a respell", func(t *testing.T) {
			t.Parallel()

			_, respells := settling().(plugin.Respeller)
			assert.True(t, respells, "the name seam is declared")
		})

		t.Run("returns a backend without a settle seam for a declaration without hooks", func(t *testing.T) {
			t.Parallel()

			plain := kitBackend(plainName, kitTarget).Build()
			_, lowers := plain.(plugin.Lowerer)
			assert.False(t, lowers, "no construct seam")
			_, respells := plain.(plugin.Respeller)
			assert.False(t, respells, "no name seam")
		})

		t.Run("returns an error rendering an unsettled store for a backend with both seams", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, kitUnsettled(t, settling()),
				"an unsettled store is never rendered")
		})

		t.Run("renders a settled store for a backend with both seams", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, kitSettled(t, settling()), 1, "the fixture assembles one file")
		})

		t.Run("panics naming every kit defect at once", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).
					KindTemplates(kitStructs()).
					Funcs(kitFuncs).
					Build()
			}, "collected defects panic together")
			assert.That(t, fmt.Sprint(recovered)).
				Contains("Struct kind twice", "naming the kind defect").
				Contains("helper twice", "naming the helper defect")
		})

		t.Run("panics naming every language fault at once", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				backend.New(kitName, kitTarget, kitSyntax()).Build()
			}, "an empty language is a declaration defect")
			text := fmt.Sprint(recovered)
			for _, gap := range []string{
				"kinds", "filenames", "scaffolding", "import block", "formatter",
			} {
				assert.Contains(t, text, gap, "naming the "+gap+" gap")
			}
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend whose refusals are the declared ones", func(t *testing.T) {
			t.Parallel()

			r, held := kitBackend(kitName, kitTarget).
				RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
				Build().(render.Refuser)
			assert.True(t, held, "a kit backend reads its refusals back")
			assert.Equal(t, r.RefusedKinds(), map[symbol.Kind]string{symbol.KindEnum: kitReason},
				"as declared")
		})

		t.Run("merges the refusals of two declarations", func(t *testing.T) {
			t.Parallel()

			r, held := kitBackend(kitName, kitTarget).
				RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
				RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kitReason}).
				Build().(render.Refuser)
			assert.True(t, held, "a kit backend reads its refusals back")
			assert.Equal(t, r.RefusedKinds(),
				map[symbol.Kind]string{symbol.KindEnum: kitReason, symbol.KindSum: kitReason},
				"every declaration's refusals")
		})

		t.Run("returns no refusal for a declaration without one", func(t *testing.T) {
			t.Parallel()

			r, held := kitBackend(plainName, kitTarget).Build().(render.Refuser)
			assert.True(t, held, "an undeclared refusal set still reads")
			assert.Empty(t, r.RefusedKinds(), "the backend refuses nothing")
		})

		defects := []struct {
			name  string
			build func()
			want  string
		}{
			{
				name: "panics at Build for a kind refused twice",
				build: func() {
					kitBackend(kitName, kitTarget).
						RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
						RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
						Build()
				},
				want: "refuses the " + symbol.KindEnum.String() + " kind twice",
			},
			{
				name: "panics at Build for a refused kind the templates spell",
				build: func() {
					kitBackend(kitName, kitTarget).
						RefusedKinds(map[symbol.Kind]string{symbol.KindStruct: kitReason}).
						Build()
				},
				want: "spells and refuses the " + symbol.KindStruct.String() + " kind",
			},
			{
				name: "panics at Build for a refusal without a reason",
				build: func() {
					kitBackend(kitName, kitTarget).
						RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: ""}).
						Build()
				},
				want: "without a reason",
			},
		}
		for _, tt := range defects {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				recovered := assert.Panics(t, tt.build, "a wrong refusal is a declaration defect")
				assert.Contains(t, fmt.Sprint(recovered), tt.want, "naming the defect")
			})
		}
	})

	t.Run("Coverage", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend whose coverage is the declared one", func(t *testing.T) {
			t.Parallel()

			declared := render.Coverage{
				Facts: map[symbol.Fact]render.Verdict{
					symbol.FactAbstract: render.Refuses,
				},
			}
			c, held := kitBackend(kitName, kitTarget).Coverage(declared).Build().(render.Coverer)
			assert.True(t, held, "a kit backend reads its coverage back")
			assert.Equal(t, c.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses, "as declared")
		})

		t.Run("returns an undeclared coverage for a declaration without one", func(t *testing.T) {
			t.Parallel()

			c, held := kitBackend(plainName, kitTarget).Build().(render.Coverer)
			assert.True(t, held, "an undeclared coverage still reads")
			assert.False(t, c.Coverage().Declared(), "as undeclared")
		})
	})

	t.Run("MemberIndent", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend that writes the indent before each line of a member's content", func(t *testing.T) {
			t.Parallel()

			b := backend.New(kitName, kitTarget, kitSyntax()).
				FileTemplate(kitFile).
				KindTemplates(map[symbol.Kind]string{symbol.KindMethod: memberMethod}).
				Naming(kitNaming).
				Scaffold(kitScaffold).
				MemberIndent(kitIndent).
				Imports(kitImports).
				Finalise(kitFinalise).
				Build()
			m := &emit.Method{Origin: coretest.Struct(coretest.StorePath, loadName).ID, Name: loadName}
			m.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(plugin.Unit{
				Plugin: kitEmitter, Per: plugin.PerPlan, Word: kitWord, Decls: []symbol.Symbol{m},
			}), "the unit of one method is added")
			r, renders := b.(plugin.Renderer)
			assert.True(t, renders, "a kit backend renders")
			sink := diag.NewSink()
			files, err := r.Render(&plugin.RenderContext{
				Emit: e, Files: kitFiles(t, b, e), Sink: sink, Plugin: kitName,
			})
			assert.NoError(t, err, "the pass renders the method")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "the method's unit assembles one file")
			assert.Contains(t, string(files[0].Body), "func "+loadName+"() {\n"+kitIndent+"\treturn\n}\n",
				"the scaffolded return is one level deeper")
		})
	})

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("panics at Build for a nil part", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).Funcs(nil).Build()
			}, "a nil part is a declaration defect")
			assert.Contains(t, fmt.Sprint(recovered), "nil vocabulary", "naming the defect")
		})

		t.Run("panics at Build for a helper two parts declare", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).Funcs(kitFuncs).Build()
			}, "one name spells through one helper")
			assert.Contains(t, fmt.Sprint(recovered), strconv.Quote(upHelper),
				"naming the helper")
		})

		t.Run("renders through every part bound to the file's import set", func(t *testing.T) {
			t.Parallel()

			b := backend.New(kitName, kitTarget, kitSyntax()).
				FileTemplate(kitFile).
				KindTemplates(map[symbol.Kind]string{
					symbol.KindStruct: "type {{" + upHelper + " .Name}} {{" + qualifyHelper + " " +
						strconv.Quote(storePkg) + " " + strconv.Quote(rowName) + "}}\n",
				}).
				KindTemplates(kitCallables()).
				Funcs(kitFuncs).
				Funcs(kitBinding).
				Naming(kitNaming).
				Scaffold(kitScaffold).
				Imports(kitImports).
				Finalise(kitFinalise).
				Build()
			files := kitRender(t, b)
			assert.Length(t, files, 1, "the fixture assembles one file")
			assert.ContainsInOrder(t, string(files[0].Body), []string{
				"import (" + runtimePkg + " " + storePkg + ")\n",
				"type " + strings.ToUpper(rowName) + " " + storeLocal + "." + rowName + "\n",
			}, "each part's helper renders, and the binding part's import is the file's")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		at := plugin.Placement{Path: "svc/store/stub.txt", Origin: coretest.PackageID(coretest.StorePath)}

		t.Run("returns a backend whose package half runs the declared rule", func(t *testing.T) {
			t.Parallel()

			rule := func(p plugin.Placement) (symbol.Identity, error) {
				return symbol.Identity{Package: path.Dir(p.Path), Name: storeLocal}, nil
			}
			p, held := kitBackend(kitName, kitTarget).Packages(rule).Build().(plugin.Packager)
			assert.True(t, held, "a kit backend names packages")
			got, err := p.PackageAt(at)
			assert.NoError(t, err, "the rule derives a package")
			assert.Equal(t, got, symbol.Identity{Package: coretest.StorePath, Name: storeLocal},
				"the rule's package")
		})

		t.Run("returns the rule's error", func(t *testing.T) {
			t.Parallel()

			rule := func(plugin.Placement) (symbol.Identity, error) {
				return symbol.Identity{}, errors.New("no module contains the directory")
			}
			p, held := kitBackend(kitName, kitTarget).Packages(rule).Build().(plugin.Packager)
			assert.True(t, held, "a kit backend names packages")
			_, err := p.PackageAt(at)
			assert.HasError(t, err, "the rule derives no package")
		})

		t.Run("returns the origin for a backend without a rule", func(t *testing.T) {
			t.Parallel()

			p, held := kitBackend(plainName, kitTarget).Build().(plugin.Packager)
			assert.True(t, held, "an undeclared rule still names packages")
			got, err := p.PackageAt(at)
			assert.NoError(t, err, "the origin needs no derivation")
			assert.Equal(t, got, at.Origin, "a file declares the package its declarations derive from")
		})
	})

	t.Run("FileName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared naming's spelling", func(t *testing.T) {
			t.Parallel()

			s, held := kitBackend(kitName, kitTarget).Build().(plugin.FileSpeller)
			assert.True(t, held, "a kit backend spells filenames")
			assert.Equal(t, s.FileName(kitUnit()), kitWord+kitExt, "the naming's spelling")
		})
	})

	t.Run("SplitUnit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the unit whole for a backend without a split", func(t *testing.T) {
			t.Parallel()

			s, held := kitBackend(kitName, kitTarget).Build().(plugin.FileSpeller)
			assert.True(t, held, "a kit backend spells filenames")
			parts := s.SplitUnit(kitUnit())
			assert.Length(t, parts, 1, "one part")
			assert.Length(t, parts[0].Decls, len(kitUnit().Decls), "the part keeps every declaration")
		})

		t.Run("returns the declared split's parts", func(t *testing.T) {
			t.Parallel()

			s, held := kitBackend(kitName, kitTarget).
				Split(func(u plugin.Unit) []plugin.Unit {
					first, rest := u, u
					first.Decls, rest.Decls = u.Decls[:1], u.Decls[1:]
					return []plugin.Unit{first, rest}
				}).
				Build().(plugin.FileSpeller)
			assert.True(t, held, "a kit backend spells filenames")
			assert.Length(t, s.SplitUnit(kitUnit()), 2, "the split's two parts")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared version", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend(kitName, kitTarget).
				Version(kitVersion).Build().(plugin.Versioned)
			assert.True(t, held, "a kit backend contributes to the run fingerprint")
			assert.Equal(t, b.Version(), kitVersion, "the version as declared")
		})

		t.Run("returns empty for a declaration without a version", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend(plainName, kitTarget).Build().(plugin.Versioned)
			assert.True(t, held, "an undeclared version still reads")
			assert.Equal(t, b.Version(), "", "the fingerprint folds in nothing for it")
		})
	})

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		lowering := func() plugin.Backend {
			return kitBackend(kitName, kitTarget).Lower(kitLower).Build()
		}

		t.Run("returns a backend with the construct seam", func(t *testing.T) {
			t.Parallel()

			_, lowers := lowering().(plugin.Lowerer)
			assert.True(t, lowers, "a declared lowering implements the construct seam")
		})

		t.Run("returns a backend without the name seam", func(t *testing.T) {
			t.Parallel()

			_, respells := lowering().(plugin.Respeller)
			assert.False(t, respells, "no respell was declared")
		})

		t.Run("returns the declared hook's declarations", func(t *testing.T) {
			t.Parallel()

			l, held := lowering().(plugin.Lowerer)
			assert.True(t, held, "the built backend lowers")
			row := &emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, coretest.StructName).ID,
				Name:   coretest.StructName,
			}
			out, err := l.Lower(row)
			assert.NoError(t, err, "the fixture hook takes a struct")
			assert.Equal(t, structNames(t, out),
				[]string{coretest.StructName, coretest.StructName + cursorSuffix},
				"the hook's own declarations, in its own order")
		})

		t.Run("returns an error rendering an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, lowering())
			assert.HasError(t, err, "an unsettled store is never rendered")
			assert.Contains(t, err.Error(), "unsettled", "naming why no bytes were written")
		})

		t.Run("renders the lowered declarations of a settled store", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, lowering())
			assert.Length(t, files, 1, "the fixture assembles one file")
			upper := strings.ToUpper(rowName)
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"type " + upper + " struct{}", "type " + upper + strings.ToUpper(cursorSuffix) + " struct{}"},
				"the companion the hook produced renders beside its input")
		})
	})

	t.Run("Respell", func(t *testing.T) {
		t.Parallel()

		respelling := func() plugin.Backend {
			return kitBackend(kitName, kitTarget).Respell(kitRespell).Build()
		}

		t.Run("returns a backend with the name seam", func(t *testing.T) {
			t.Parallel()

			_, respells := respelling().(plugin.Respeller)
			assert.True(t, respells, "a declared respell implements the name seam")
		})

		t.Run("returns a backend without the construct seam", func(t *testing.T) {
			t.Parallel()

			_, lowers := respelling().(plugin.Lowerer)
			assert.False(t, lowers, "no lowering was declared")
		})

		t.Run("returns the declared hook's name", func(t *testing.T) {
			t.Parallel()

			r, held := respelling().(plugin.Respeller)
			assert.True(t, held, "the built backend respells")
			got, err := r.Respell(symbol.KindInvalid, symbol.KindStruct,
				symbol.VisibilityPublic, coretest.StructName)
			assert.NoError(t, err, "the fixture hook spells every name")
			assert.Equal(t, got, respellPrefix+coretest.StructName, "the hook's own name")
		})

		t.Run("returns an error rendering an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, respelling())
			assert.HasError(t, err, "an unsettled store is never rendered")
			assert.Contains(t, err.Error(), "unsettled", "naming why no bytes were written")
		})

		t.Run("renders the respelt names of a settled store", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, respelling())
			assert.Length(t, files, 1, "the fixture assembles one file")
			assert.ContainsInOrder(t, string(files[0].Body), []string{
				"type " + strings.ToUpper(respellPrefix+rowName) + " struct{}",
				"func " + respellPrefix + loadName + "() {",
			}, "every declared name renders through the convention")
		})
	})

	t.Run("SpellType", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared spoke's spelling", func(t *testing.T) {
			t.Parallel()

			spoke, held := kitBackend(kitName, kitTarget).Types(kitSpoke).Build().(plugin.TypeSpeller)
			assert.True(t, held, "a kit backend has the spoke role")
			got, err := spoke.SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got, kitTextRef, "the reference is the declared spoke's", assert.ByIdentity())
		})

		t.Run("returns the declared spoke's refusal", func(t *testing.T) {
			t.Parallel()

			spoke, held := kitBackend(kitName, kitTarget).Types(kitSpoke).Build().(plugin.TypeSpeller)
			assert.True(t, held, "a kit backend has the spoke role")
			_, err := spoke.SpellType(rules.Leaf(symbol.FormBool, "bool"), plugin.Policy{})
			assert.HasError(t, err, "the spoke refuses a bool")
		})

		t.Run("returns an error with the target for a backend without a spoke", func(t *testing.T) {
			t.Parallel()

			spoke, held := kitBackend(kitName, kitTarget).Build().(plugin.TypeSpeller)
			assert.True(t, held, "every kit backend has the spoke role")
			_, err := spoke.SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.HasError(t, err, "the backend refuses every shape")
			assert.Contains(t, err.Error(), "target "+string(kitTarget)+" does not spell the types",
				"the error contains the target")
		})
	})

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared specs in declaration order", func(t *testing.T) {
			t.Parallel()

			depth := widthPolicy
			depth.Key = "stub.depth"
			provider, held := kitBackend(kitName, kitTarget).Policies(widthPolicy).Policies(depth).Build().(plugin.PolicyProvider)
			assert.True(t, held, "a kit backend has the policy role")
			assert.Equal(t, provider.Policies(), []plugin.PolicySpec{widthPolicy, depth},
				"the specs accumulate in order")
		})

		t.Run("returns no specs for a backend without a declaration", func(t *testing.T) {
			t.Parallel()

			provider, held := kitBackend(kitName, kitTarget).Build().(plugin.PolicyProvider)
			assert.True(t, held, "every kit backend has the policy role")
			assert.Empty(t, provider.Policies(), "the backend does not declare a policy")
		})
	})
}

// A declaration allocates its builder, each merge of a map, each
// vocabulary part and the lowered backend within their ceilings, and
// its setters allocate nothing, in the ordinary run, which runs no
// benchmark. A merge and a Build each take a declaration built outside
// the count, because each changes or freezes the declaration it is
// called on. The check runs alone, because the count includes every
// goroutine's allocations.
func TestBackendAllocs(t *testing.T) {
	syntax := kitSyntax()
	var b *backend.Builder
	assert.MaxAllocs(t, func() { b = backend.New(kitName, kitTarget, syntax) }, newAllocs,
		"New allocates the builder and its maps")
	for _, tt := range setters() {
		var got *backend.Builder
		assert.MaxAllocs(t, func() { got = tt.set(b) }, 0, tt.name+" allocates nothing")
		assert.Equal(t, got, b, tt.name+" returns its builder", assert.ByIdentity())
	}
	fresh := func() *backend.Builder { return backend.New(kitName, kitTarget, syntax) }
	for _, tt := range merges() {
		var merged *backend.Builder
		assert.MaxAllocsWithSetup(t, fresh, func(b *backend.Builder) { merged = tt.set(b) }, tt.allocs,
			tt.name+" allocates the sorted keys and the entry")
		assert.NotNil(t, merged, tt.name+" returns its builder")
	}
	var built plugin.Backend
	assert.MaxAllocsWithSetup(t, func() *backend.Builder { return declaration(false) },
		func(b *backend.Builder) { built = b.Build() }, buildAllocs,
		"Build allocates the parsed templates and the lowered backend")
	_, renders := built.(plugin.Renderer)
	assert.True(t, renders, "the lowered backend renders")
	assert.MaxAllocsWithSetup(t, func() *backend.Builder { return declaration(true) },
		func(b *backend.Builder) { built = b.Build() }, buildSeamsAllocs,
		"Build allocates the composition of both seams besides")
	_, lowers := built.(plugin.Lowerer)
	assert.True(t, lowers, "the lowered backend has the construct seam")

	spoken := kitBackend(kitName, kitTarget).Types(kitSpoke).Policies(widthPolicy).Build()
	spoke, held := spoken.(plugin.TypeSpeller)
	assert.True(t, held, "the lowered backend has the spoke role")
	text := rules.Leaf(symbol.FormText, "string")
	var (
		ref *emit.TypeRef
		err error
	)
	assert.MaxAllocs(t, func() { ref, err = spoke.SpellType(text, plugin.Policy{}) }, 0,
		"SpellType allocates what the declared spoke allocates")
	assert.NoError(t, err, "the spoke spells text")
	assert.Equal(t, ref, kitTextRef, "SpellType returns the spoke's reference", assert.ByIdentity())
	provider, held := spoken.(plugin.PolicyProvider)
	assert.True(t, held, "the lowered backend has the policy role")
	var specs []plugin.PolicySpec
	assert.MaxAllocs(t, func() { specs = provider.Policies() }, 0, "Policies allocates nothing")
	assert.Length(t, specs, 1, "Policies returns the declared spec")
}

// BenchmarkBackend measures each step of a declaration: the builder,
// the setters, a merge of each map, a vocabulary part, and the lowering.
func BenchmarkBackend(b *testing.B) {
	syntax := kitSyntax()

	b.Run("New", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(newAllocs)
		defer c.End()
		var got *backend.Builder
		for c.Loop() {
			got = backend.New(kitName, kitTarget, syntax)
		}
		assert.NotNil(b, got, "New returns a builder")
	})

	for _, tt := range setters() {
		b.Run(tt.name, func(b *testing.B) {
			builder := backend.New(kitName, kitTarget, syntax)
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got *backend.Builder
			for c.Loop() {
				got = tt.set(builder)
			}
			assert.Equal(b, got, builder, tt.name+" returns its builder", assert.ByIdentity())
		})
	}

	for _, tt := range merges() {
		b.Run(tt.name, func(b *testing.B) {
			b.Run("one entry into a new declaration", func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var builder *backend.Builder
				for c.Loop() {
					c.Excluding(func() { builder = backend.New(kitName, kitTarget, syntax) })
					tt.set(builder)
				}
				assert.NotNil(b, builder, tt.name+" merges into the declaration")
			})
		})
	}

	b.Run("Build", func(b *testing.B) {
		b.Run("the fixture declaration", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(buildAllocs)
			defer c.End()
			var (
				builder *backend.Builder
				got     plugin.Backend
			)
			for c.Loop() {
				c.Excluding(func() { builder = declaration(false) })
				got = builder.Build()
			}
			_, renders := got.(plugin.Renderer)
			assert.True(b, renders, "the lowered backend renders")
		})

		b.Run("the fixture declaration with both seams", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(buildSeamsAllocs)
			defer c.End()
			var (
				builder *backend.Builder
				got     plugin.Backend
			)
			for c.Loop() {
				c.Excluding(func() { builder = declaration(true) })
				got = builder.Build()
			}
			_, lowers := got.(plugin.Lowerer)
			assert.True(b, lowers, "the lowered backend has the construct seam")
		})
	})

	spoken := kitBackend(kitName, kitTarget).Types(kitSpoke).Policies(widthPolicy).Build()

	b.Run("SpellType", func(b *testing.B) {
		spoke, held := spoken.(plugin.TypeSpeller)
		assert.True(b, held, "the lowered backend has the spoke role")
		text := rules.Leaf(symbol.FormText, "string")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var ref *emit.TypeRef
		for c.Loop() {
			ref, _ = spoke.SpellType(text, plugin.Policy{})
		}
		assert.Equal(b, ref, kitTextRef, "SpellType returns the spoke's reference", assert.ByIdentity())
	})

	b.Run("Policies", func(b *testing.B) {
		provider, held := spoken.(plugin.PolicyProvider)
		assert.True(b, held, "the lowered backend has the policy role")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var specs []plugin.PolicySpec
		for c.Loop() {
			specs = provider.Policies()
		}
		assert.Length(b, specs, 1, "Policies returns the declared spec")
	})
}

// kitSyntax returns the comment forms a fixture language declares.
func kitSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// kitStructs and kitCallables split the kind inventory across two
// declarations, the way a satellite groups its spellings. The struct
// spelling calls the shared helper, so the vocabulary is exercised
// too.
func kitStructs() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct: "type {{" + upHelper + " .Name}} struct{}\n",
	}
}

func kitCallables() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
	}
}

// kitFuncs is the fixture's shared template vocabulary, which binds
// nothing to the file's import set.
func kitFuncs(*render.ImportSet) template.FuncMap {
	return template.FuncMap{upHelper: strings.ToUpper}
}

// kitBinding is a vocabulary part bound to the file's import set: its
// helper binds a package under the package's last segment and spells a
// name through the bound name.
func kitBinding(set *render.ImportSet) template.FuncMap {
	return template.FuncMap{qualifyHelper: func(pkg, name string) string {
		return set.Bind(pkg, path.Base(pkg)) + "." + name
	}}
}

// kitNaming spells every unit as its word under a fixture
// extension.
func kitNaming(u plugin.Unit) string { return u.Word + kitExt }

// kitScaffold spells the one statement kind the fixture emits,
// recording an import the way a real printer records what it
// qualifies with.
func kitScaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	if s.Kind != emit.StmtReturn {
		return nil, errors.New("the fixture spells returns only")
	}
	set.Add(runtimePkg)
	return []byte("\treturn\n"), nil
}

// kitImports renders the collected set as a one-line block.
func kitImports(set *render.ImportSet) string {
	if set.Len() == 0 {
		return ""
	}
	return "import (" + strings.Join(set.Paths(), " ") + ")\n"
}

// kitFinalise is the pass-through fixture formatter.
func kitFinalise(src []byte) ([]byte, error) { return src, nil }

// kitLanguage returns the same language as values, for the
// hand-built pass the kit's Build must lower to.
func kitLanguage() render.Language {
	kinds := kitStructs()
	maps.Copy(kinds, kitCallables())
	return render.Language{
		Kinds: kinds, File: kitFile, Funcs: kitFuncs,
		Naming: kitNaming, Scaffold: kitScaffold,
		Imports: kitImports, Finalise: kitFinalise,
	}
}

// kitBackend returns the full fixture declaration, ready to Build
// or to break one piece of.
func kitBackend(name plugin.ID, target plugin.Target) *backend.Builder {
	return backend.New(name, target, kitSyntax()).
		FileTemplate(kitFile).
		KindTemplates(kitStructs()).
		KindTemplates(kitCallables()).
		Funcs(kitFuncs).
		Naming(kitNaming).
		Scaffold(kitScaffold).
		Imports(kitImports).
		Finalise(kitFinalise)
}

// kitUnit returns one flushed plan unit of a struct and a function
// whose body scaffolds a return.
func kitUnit() plugin.Unit {
	f := &emit.Function{
		Origin: coretest.Struct(coretest.StorePath, loadName).ID,
		Name:   loadName,
	}
	f.Body = emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}}
	return plugin.Unit{
		Plugin: kitEmitter, Per: plugin.PerPlan, Word: kitWord,
		Decls: []symbol.Symbol{
			&emit.Struct{
				Origin: coretest.Struct(coretest.StorePath, rowName).ID,
				Name:   rowName,
			},
			f,
		},
	}
}

// kitLower is the fixture's construct lowering: a struct becomes
// itself and a companion with the same origin, and every other
// declaration passes through untouched.
func kitLower(s symbol.Symbol) ([]symbol.Symbol, error) {
	row, held := s.(*emit.Struct)
	if !held {
		return nil, nil
	}
	return []symbol.Symbol{row, &emit.Struct{
		Origin: row.Origin, Name: row.Name + cursorSuffix,
	}}, nil
}

// kitRespell is the fixture's name convention: every declared name
// takes the prefix, whatever its kind.
func kitRespell(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	return respellPrefix + name, nil
}

// structNames reads the declared names off a lowering's result,
// which the fixture hook returns as structs alone.
func structNames(tb assert.TB, decls []symbol.Symbol) []string {
	tb.Helper()

	out := make([]string, 0, len(decls))
	for _, d := range decls {
		s, held := d.(*emit.Struct)
		assert.True(tb, held, "the fixture lowering returns structs")
		if !held {
			continue
		}
		out = append(out, s.Name)
	}
	return out
}

// kitStore returns the fixture store, unsettled.
func kitStore(tb assert.TB) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(kitUnit()), "the fixture unit is added")
	return e
}

// kitUnsettled renders the fixture store through b without
// settling it first, and returns the render's error.
func kitUnsettled(tb assert.TB, b plugin.Backend) error {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	_, err := r.Render(&plugin.RenderContext{
		Emit: kitStore(tb), Sink: diag.NewSink(), Plugin: kitName,
	})
	return err
}

// kitSettled settles the fixture store through b's declared seams
// and renders it, asserting both steps run clean.
func kitSettled(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	e := kitStore(tb)
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles once")
	coretest.AssertCodes(tb, sink)
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Files: kitFiles(tb, b, e), Sink: sink, Plugin: kitName,
	})
	assert.NoError(tb, err, "the settled store renders")
	coretest.AssertCodes(tb, sink)
	return files
}

// kitRender renders the fixture store through b's renderer role
// and asserts the pass ran clean.
func kitRender(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	r, held := b.(plugin.Renderer)
	assert.True(tb, held, "a kit backend renders")
	e := kitStore(tb)
	sink := diag.NewSink()
	files, err := r.Render(&plugin.RenderContext{
		Emit: e, Files: kitFiles(tb, b, e), Sink: sink, Plugin: kitName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	coretest.AssertCodes(tb, sink)
	return files
}

// kitFiles routes the fixture store into files through b's filename
// half, the way the suite routes a hand-built store.
func kitFiles(tb assert.TB, b plugin.Backend, e *plugin.Emit) []plugin.File {
	tb.Helper()

	s, spells := b.(plugin.FileSpeller)
	assert.True(tb, spells, "a kit backend spells filenames")
	return backendtest.Files(e, s)
}

// setters returns every setter that writes one field of a declaration.
func setters() []setter {
	split := render.Split(func(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} })
	rule := plugin.PackageRule(func(p plugin.Placement) (symbol.Identity, error) { return p.Origin, nil })
	cluster := render.Cluster(func([]symbol.Symbol) []render.Clustered { return nil })
	return []setter{
		{name: "Version", set: func(b *backend.Builder) *backend.Builder { return b.Version(kitVersion) }},
		{name: "FileTemplate", set: func(b *backend.Builder) *backend.Builder { return b.FileTemplate(kitFile) }},
		{name: "Scaffold", set: func(b *backend.Builder) *backend.Builder { return b.Scaffold(kitScaffold) }},
		{name: "Naming", set: func(b *backend.Builder) *backend.Builder { return b.Naming(kitNaming) }},
		{name: "Split", set: func(b *backend.Builder) *backend.Builder { return b.Split(split) }},
		{name: "Packages", set: func(b *backend.Builder) *backend.Builder { return b.Packages(rule) }},
		{name: "Cluster", set: func(b *backend.Builder) *backend.Builder { return b.Cluster(cluster) }},
		{name: "Imports", set: func(b *backend.Builder) *backend.Builder { return b.Imports(kitImports) }},
		{name: "Finalise", set: func(b *backend.Builder) *backend.Builder { return b.Finalise(kitFinalise) }},
		{name: "Coverage", set: func(b *backend.Builder) *backend.Builder { return b.Coverage(render.Coverage{}) }},
		{name: "Lower", set: func(b *backend.Builder) *backend.Builder { return b.Lower(kitLower) }},
		{name: "Respell", set: func(b *backend.Builder) *backend.Builder { return b.Respell(kitRespell) }},
		{name: "Types", set: func(b *backend.Builder) *backend.Builder { return b.Types(kitSpoke) }},
		{name: "MemberIndent", set: func(b *backend.Builder) *backend.Builder { return b.MemberIndent(kitIndent) }},
	}
}

// merges returns every method that merges entries into a declaration,
// each with one entry and its ceiling.
func merges() []merge {
	kinds := kitStructs()
	refused := map[symbol.Kind]string{symbol.KindSum: kitReason}
	groups := map[render.GroupName]string{blockGroup: "{{range .}}{{.}}{{end}}"}
	return []merge{
		{
			name: "KindTemplates", allocs: mergeAllocs,
			set: func(b *backend.Builder) *backend.Builder { return b.KindTemplates(kinds) },
		},
		{
			name: "RefusedKinds", allocs: mergeAllocs,
			set: func(b *backend.Builder) *backend.Builder { return b.RefusedKinds(refused) },
		},
		{
			name: "Groups", allocs: mergeAllocs,
			set: func(b *backend.Builder) *backend.Builder { return b.Groups(groups) },
		},
		{
			name: "Funcs", allocs: funcsAllocs,
			set: func(b *backend.Builder) *backend.Builder { return b.Funcs(kitFuncs) },
		},
		{
			name: "Policies", allocs: policiesAllocs,
			set: func(b *backend.Builder) *backend.Builder { return b.Policies(widthPolicy) },
		},
	}
}

// declaration returns a whole fixture declaration, with both settle
// seams where seams is set.
func declaration(seams bool) *backend.Builder {
	b := kitBackend(kitName, kitTarget)
	if seams {
		b.Lower(kitLower).Respell(kitRespell)
	}
	return b
}
