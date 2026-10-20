// Copyright Dokimasia B.V. 2026
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
	"go.dokimi.dev/assert/expect"

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

// The constants of the kit's fixture name the backend, the unit that it
// renders and the helpers of its vocabulary.
const (
	// kitName is the name of the fully declared backend. plainName is the
	// name of a backend without hooks.
	kitName   plugin.ID = "printer"
	plainName plugin.ID = "plain"
	// kitTarget is the target of both backends.
	kitTarget plugin.Target = "stub"
	// kitEmitter is the plugin that emits the fixture's unit.
	kitEmitter plugin.ID = "gen"
	// The file name of the fixture's unit is kitWord followed by kitExt.
	kitWord = "stub"
	kitExt  = ".txt"
	// twoUses is a template that imports two packages under one name.
	twoUses = `{{use "a/x" "x"}}{{use "b/x" "x"}}`
	// rowName and loadName are the names of the unit's struct and
	// function, and fetchName the name of an overload signature.
	rowName   = "Row"
	loadName  = "Load"
	fetchName = "Fetch"
	// runtimePkg is the import that the fixture's scaffold records.
	runtimePkg = "stub/runtime"
	// upHelper is the one helper of the fixture's vocabulary.
	upHelper = "up"
	// blockGroup is the name of a group template.
	blockGroup render.GroupName = "block"
	// qualifyHelper is the helper that binds a package through the import
	// set of the file. storePkg is the package that it binds, and
	// storeLocal is the local name of the package.
	qualifyHelper = "qualify"
	storePkg      = "svc/store"
	storeLocal    = "store"
	// kitVersion is the version that the fixture declares. It differs
	// from the empty version of a backend that declares none.
	kitVersion = "3.2.1"
	// cursorSuffix is the suffix of the companion struct that the
	// fixture's lowering adds beside every struct.
	cursorSuffix = "Cursor"
	// respellPrefix is the prefix that the fixture's respell puts before
	// every declared name, and memberPrefix the prefix that the convention
	// of a member of the member cases puts before the name of a member.
	respellPrefix = "t_"
	memberPrefix  = "m_"
	// kitReason is the reason that the fixture gives for a kind that it
	// refuses.
	kitReason = "the fixture language declares no such construct"
	// kitIndent is the member indent of the member case. memberMethod is
	// a method template that places its content through the memberbody
	// builtin.
	kitIndent    = "\t"
	memberMethod = "func {{.Name}}() {\n{{memberbody .}}}\n"
	// sampleHelper is a helper that returns a literal value of the text
	// sevenText. valueStruct is a struct template that writes the value
	// through the value builtin.
	sampleHelper = "sample"
	sevenText    = "7"
	valueStruct  = "type {{.Name}} = {{value " + sampleHelper + "}}\n"
)

// The allocation ceilings of the steps of a declaration.
const (
	// newAllocs is the allocation count of New. New allocates the builder,
	// the three maps of the language and the helper set.
	newAllocs = 5
	// mergeAllocs is the allocation count of a KindTemplates,
	// RefusedKinds or Groups call that merges one entry into a new
	// declaration. slices.Sorted over maps.Keys allocates 4 objects for
	// the sorted keys, and the map allocates its first group.
	mergeAllocs = 5
	// funcsAllocs is the allocation count of a Funcs call that adds the
	// fixture's vocabulary to a new declaration. The call allocates the
	// import set that the kit passes to the part, 2 objects for the
	// vocabulary that the part returns, 4 for the sorted helper names, the
	// first group of the helper set and the list of parts.
	funcsAllocs = 9
	// buildAllocs is the allocation count of a Build of the fixture's
	// declaration, with or without hooks. text/template allocates most of
	// them, for the parse trees of the file skeleton and the two kind
	// templates and for the function maps that it keeps for each template.
	// The eight builtins and the helper of the fixture are nine names, so
	// each of these maps grows past one group of eight slots. The composed
	// pass and the backend allocate the rest.
	buildAllocs = 185
	// policiesAllocs is the allocation count of a Policies call that adds
	// one spec to a new declaration. The call allocates the list of specs.
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

// kitMethod is the method that the fixture's binder serves, and kitHomePkg
// the package that its home returns.
var (
	kitMethod  = &emit.Method{Name: "Serve"}
	kitHomePkg = symbol.Identity{Lang: "stub", Package: "svc/gen", Name: "gen", Kind: symbol.KindPackage}
)

// kitHandler serves kitMethod for every rpc.
func kitHandler(plugin.RPC, plugin.Policy, plugin.Translate) (*emit.Method, error) {
	return kitMethod, nil
}

// kitHome returns kitHomePkg for every target.
func kitHome(symbol.Identity, plugin.Placement) (symbol.Identity, error) { return kitHomePkg, nil }

// kitSpoke returns the fixture's text reference for a text shape, and an
// error for every other shape.
func kitSpoke(s rules.TypeShape, _ plugin.Policy) (*emit.TypeRef, error) {
	if s.Form != symbol.FormText {
		return nil, errors.New("the fixture spells text alone")
	}
	return kitTextRef, nil
}

// setter is a setter of one field of a declaration. The test calls it
// through a function that it builds before any measurement.
type setter struct {
	name string
	set  func(*backend.Builder) *backend.Builder
}

// merge is a method that merges entries into a declaration. allocs is
// the ceiling of a merge of one entry into a new declaration.
type merge struct {
	name   string
	allocs uint64
	set    func(*backend.Builder) *backend.Builder
}

// kitFile is the fixture's file skeleton. It writes a comment, so a case
// can tell a declared skeleton from the default of the kit.
const kitFile = "// {{.Name}}\n{{imports}}{{decls}}"

// kitBody is the file that the full fixture declaration renders. It has
// the comment of the skeleton, the import of the scaffold, the struct
// through the shared helper, a blank line, and the function around its
// scaffolded body.
var kitBody = "// " + kitWord + kitExt + "\n" +
	"import (" + runtimePkg + ")\n" +
	"type " + strings.ToUpper(rowName) + " struct{}\n" +
	"\n" +
	"func " + loadName + "() {\n\treturn\n}\n"

// TestBackend tests the backend kit. A declaration is data, and Build
// lowers it to a backend over the render pass.
func TestBackend(t *testing.T) {
	t.Parallel()

	t.Run("Build", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend named as declared", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().Name(), kitName,
				"the backend has the declared name")
		})

		t.Run("returns a backend for the declared target", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().Target(), kitTarget,
				"the backend renders the declared target")
		})

		t.Run("returns a backend with the declared comment syntax", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().Syntax(), kitSyntax(),
				"the output contract writes the header through the declared syntax")
		})

		t.Run("renders a file through every declared piece", func(t *testing.T) {
			t.Parallel()

			files := kitRender(t, kitBackend(kitName, kitTarget).Build())
			assert.Length(t, files, 1, "the fixture renders one file")
			assert.Equal(t, files[0].Path, kitWord+kitExt, "the naming spells the path")
			assert.Equal(t, string(files[0].Body), kitBody,
				"the skeleton, the vocabulary, the kind templates and the scaffold write the body")
		})

		t.Run("renders the same bytes for every build", func(t *testing.T) {
			t.Parallel()

			assert.Deterministic(t, func(name plugin.ID) ([]plugin.RenderedFile, error) {
				return kitRender(t, kitBackend(name, kitTarget).Build()), nil
			}, kitName, "two builds of one declaration render the same bytes")
		})

		t.Run("renders the same bytes as a hand-built pass over the same language", func(t *testing.T) {
			t.Parallel()

			built := kitRender(t, kitBackend(kitName, kitTarget).Build())
			pass, err := render.New(kitName, kitLanguage())
			assert.NoError(t, err, "the same language composes by hand")
			e := kitStore(t)
			direct, err := pass.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, pass), Sink: diag.NewSink(), Plugin: kitName,
			})
			assert.NoError(t, err, "the hand-built pass renders every file")
			assert.Equal(t, built, direct, "the kit renders the same files as the hand-built pass")
		})

		t.Run("renders the declarations of a file in the order that its language evaluates", func(t *testing.T) {
			t.Parallel()

			evaluated := func(d symbol.Symbol, yield func(string)) {
				if _, is := d.(*emit.Struct); is {
					yield(loadName)
				}
			}
			files := kitRender(t, kitBackend(kitName, kitTarget).Evaluated(evaluated).Build())
			assert.Length(t, files, 1, "the fixture renders one file")
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"func " + loadName, "type " + strings.ToUpper(rowName)},
				"the struct follows the function that it evaluates")
		})

		t.Run("reports UnimplementedOverload for an overload signature without an implementation", func(t *testing.T) {
			t.Parallel()

			fetch := coretest.ID(coretest.StorePath, fetchName, symbol.KindFunction)
			u := kitUnit()
			u.Decls = append(u.Decls, &emit.Function{Origin: fetch, Name: fetchName, Overload: true})
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(u), "the unit is added")
			b := kitBackend(kitName, kitTarget).Implemented().Build()
			sink := diag.NewSink()
			ctx := &plugin.RenderContext{Emit: e, Files: backendtest.Files(e, b), Sink: sink, Plugin: kitName}
			_, err := b.Render(ctx)
			assert.NoError(t, err, "the pass renders every file")
			coretest.AssertCodes(t, sink, render.UnimplementedOverload)
		})

		t.Run("renders an unsettled store for a backend without hooks", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, kitUnsettled(t, kitBackend(plainName, kitTarget).Build()),
				"a backend without hooks renders a store that did not settle")
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
			{
				name:  "panics for a binder without a handler",
				build: func() { kitBackend(kitName, kitTarget).Binder(nil, kitHome).Build() },
			},
			{
				name:  "panics for a binder without a home",
				build: func() { kitBackend(kitName, kitTarget).Binder(kitHandler, nil).Build() },
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
			}, "the split reshapes the units before the naming")
		})

		settling := func() backend.Backend {
			return kitBackend(kitName, kitTarget).
				Lower(func(s symbol.Symbol) ([]symbol.Symbol, error) {
					return []symbol.Symbol{s}, nil
				}).
				Respell(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
					return name, nil
				}).
				Build()
		}

		t.Run("returns an error rendering an unsettled store for a backend with both seams", func(t *testing.T) {
			t.Parallel()

			assert.HasError(t, kitUnsettled(t, settling()),
				"a backend with hooks does not render a store that did not settle")
		})

		t.Run("renders a settled store for a backend with both seams", func(t *testing.T) {
			t.Parallel()

			assert.Length(t, kitSettled(t, settling()), 1, "the fixture renders one file")
		})

		t.Run("panics naming every kit defect at once", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).
					KindTemplates(kitStructs()).
					Funcs(kitFuncs).
					Build()
			}, "the defects of one group panic together")
			assert.That(t, fmt.Sprint(recovered)).
				Contains("Struct kind twice", "the panic names the kind defect").
				Contains("helper twice", "the panic names the helper defect")
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
				assert.Contains(t, text, gap, "the panic names the missing "+gap)
			}
		})
	})

	t.Run("RefusedKinds", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend whose refusals are the declared ones", func(t *testing.T) {
			t.Parallel()

			b := kitBackend(kitName, kitTarget).
				RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
				Build()
			assert.Equal(t, b.RefusedKinds(), map[symbol.Kind]string{symbol.KindEnum: kitReason},
				"the backend refuses the declared kinds")
		})

		t.Run("merges the refusals of two declarations", func(t *testing.T) {
			t.Parallel()

			b := kitBackend(kitName, kitTarget).
				RefusedKinds(map[symbol.Kind]string{symbol.KindEnum: kitReason}).
				RefusedKinds(map[symbol.Kind]string{symbol.KindSum: kitReason}).
				Build()
			assert.Equal(t, b.RefusedKinds(),
				map[symbol.Kind]string{symbol.KindEnum: kitReason, symbol.KindSum: kitReason},
				"the backend refuses the kinds of both declarations")
		})

		t.Run("returns no refusal for a declaration without one", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, kitBackend(plainName, kitTarget).Build().RefusedKinds(), "the backend refuses nothing")
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
				assert.Contains(t, fmt.Sprint(recovered), tt.want, "the panic names the defect")
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
			b := kitBackend(kitName, kitTarget).Coverage(declared).Build()
			assert.Equal(t, b.Coverage().Of(symbol.KindStruct, symbol.FactAbstract),
				render.Refuses, "the backend has the declared verdict")
		})

		t.Run("returns an undeclared coverage for a declaration without one", func(t *testing.T) {
			t.Parallel()

			assert.False(t, kitBackend(plainName, kitTarget).Build().Coverage().Declared(),
				"the backend declares no coverage")
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
			sink := diag.NewSink()
			files, err := b.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, b), Sink: sink, Plugin: kitName,
			})
			assert.NoError(t, err, "the pass renders the method")
			coretest.AssertCodes(t, sink)
			assert.Length(t, files, 1, "the unit of the method renders one file")
			assert.Contains(t, string(files[0].Body), "func "+loadName+"() {\n"+kitIndent+"\treturn\n}\n",
				"the scaffolded return is one level deeper")
		})
	})

	t.Run("Value", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend whose templates write a value through the declared writer", func(t *testing.T) {
			t.Parallel()

			sample := func(*render.ImportSet) template.FuncMap {
				return template.FuncMap{
					sampleHelper: func() emit.Value { return emit.Literal(emit.LiteralInt, sevenText) },
				}
			}
			b := backend.New(kitName, kitTarget, kitSyntax()).
				FileTemplate(kitFile).
				KindTemplates(map[symbol.Kind]string{symbol.KindStruct: valueStruct}).
				KindTemplates(kitCallables()).
				Funcs(sample).
				Naming(kitNaming).
				Scaffold(kitScaffold).
				Value(kitValue).
				Imports(kitImports).
				Finalise(kitFinalise).
				Build()
			files := kitRender(t, b)
			assert.Length(t, files, 1, "the fixture renders one file")
			assert.Contains(t, string(files[0].Body), "type "+rowName+" = "+sevenText+"\n",
				"the struct template writes the value through the writer")
		})
	})

	t.Run("TemplateImport", func(t *testing.T) {
		t.Parallel()

		t.Run("returns a backend that refuses a use whose name another use binds", func(t *testing.T) {
			t.Parallel()

			b := backend.New(kitName, kitTarget, kitSyntax()).
				FileTemplate(kitFile).
				KindTemplates(map[symbol.Kind]string{
					symbol.KindMethod: twoUses + memberMethod,
				}).
				Naming(kitNaming).
				Scaffold(kitScaffold).
				Imports(kitImports).
				TemplateImport(func(_, name string) (string, string, bool) { return name, "", name != "" }).
				Finalise(kitFinalise).
				Build()
			m := &emit.Method{Origin: coretest.Struct(coretest.StorePath, loadName).ID, Name: loadName}
			e := plugin.NewEmit()
			assert.NoError(t, e.Add(plugin.Unit{
				Plugin: kitEmitter, Per: plugin.PerPlan, Word: kitWord, Decls: []symbol.Symbol{m},
			}), "the unit of one method is added")
			sink := diag.NewSink()
			_, err := b.Render(&plugin.RenderContext{
				Emit: e, Files: backendtest.Files(e, b), Sink: sink, Plugin: kitName,
			})
			assert.NoError(t, err, "a refused template is a finding")
			coretest.AssertCodes(t, sink, render.RefusedTemplate)
		})
	})

	t.Run("Funcs", func(t *testing.T) {
		t.Parallel()

		t.Run("panics at Build for a nil part", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).Funcs(nil).Build()
			}, "a nil part is a declaration defect")
			assert.Contains(t, fmt.Sprint(recovered), "nil vocabulary", "the panic names the defect")
		})

		t.Run("panics at Build for a helper two parts declare", func(t *testing.T) {
			t.Parallel()

			recovered := assert.Panics(t, func() {
				kitBackend(kitName, kitTarget).Funcs(kitFuncs).Build()
			}, "one name has one helper")
			assert.Contains(t, fmt.Sprint(recovered), strconv.Quote(upHelper),
				"the panic names the helper")
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
			assert.Length(t, files, 1, "the fixture renders one file")
			assert.ContainsInOrder(t, string(files[0].Body), []string{
				"import (" + runtimePkg + " " + storePkg + ")\n",
				"type " + strings.ToUpper(rowName) + " " + storeLocal + "." + rowName + "\n",
			}, "each part's helper renders, and the import of the binding part is in the file")
		})
	})

	t.Run("Packages", func(t *testing.T) {
		t.Parallel()

		at := plugin.Placement{Path: "svc/store/stub.txt", Origin: coretest.PackageID(coretest.StorePath)}

		t.Run("returns the package of the declared rule", func(t *testing.T) {
			t.Parallel()

			rule := func(p plugin.Placement) (symbol.Identity, error) {
				return symbol.Identity{Package: path.Dir(p.Path), Name: storeLocal}, nil
			}
			got, err := kitBackend(kitName, kitTarget).Packages(rule).Build().PackageAt(at)
			assert.NoError(t, err, "the rule derives a package")
			assert.Equal(t, got, symbol.Identity{Package: coretest.StorePath, Name: storeLocal},
				"the backend returns the package of the rule")
		})

		t.Run("returns the rule's error", func(t *testing.T) {
			t.Parallel()

			rule := func(plugin.Placement) (symbol.Identity, error) {
				return symbol.Identity{}, errors.New("no module contains the directory")
			}
			_, err := kitBackend(kitName, kitTarget).Packages(rule).Build().PackageAt(at)
			assert.HasError(t, err, "the rule derives no package")
		})

		t.Run("returns the origin for a backend without a rule", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(plainName, kitTarget).Build().PackageAt(at)
			assert.NoError(t, err, "the origin needs no derivation")
			assert.Equal(t, got, at.Origin, "a file declares the package of the declarations in it")
		})
	})

	t.Run("NameScopes", func(t *testing.T) {
		t.Parallel()

		pkg := coretest.PackageID(coretest.StorePath)

		t.Run("returns the namespace of the declared rule", func(t *testing.T) {
			t.Parallel()

			rule := func(p symbol.Identity) symbol.Identity {
				return symbol.Identity{Lang: p.Lang, Package: path.Dir(p.Package), Kind: symbol.KindPackage}
			}
			got := kitBackend(kitName, kitTarget).NameScopes(rule).Build().NameScope(pkg)
			assert.Equal(t, got, rule(pkg), "the backend returns the namespace of the rule")
		})

		t.Run("returns the package for a backend without a rule", func(t *testing.T) {
			t.Parallel()

			got := kitBackend(plainName, kitTarget).Build().NameScope(pkg)
			assert.Equal(t, got, pkg, "the names of a file are in the namespace of its package")
		})
	})

	t.Run("Overloads", func(t *testing.T) {
		t.Parallel()

		t.Run("reports true for a backend that declares it", func(t *testing.T) {
			t.Parallel()

			assert.True(t, kitBackend(kitName, kitTarget).Overloads().Build().Overloads(),
				"the language of the target overloads a function")
		})

		t.Run("reports false for a backend that does not declare it", func(t *testing.T) {
			t.Parallel()

			assert.False(t, kitBackend(plainName, kitTarget).Build().Overloads(),
				"two functions of one name collide in the language of the target")
		})
	})

	t.Run("FileName", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the file name of the declared naming", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, kitBackend(kitName, kitTarget).Build().FileName(kitUnit()), kitWord+kitExt,
				"the backend returns the file name of the naming")
		})
	})

	t.Run("SplitUnit", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the unit whole for a backend without a split", func(t *testing.T) {
			t.Parallel()

			parts := kitBackend(kitName, kitTarget).Build().SplitUnit(kitUnit())
			assert.Length(t, parts, 1, "the unit has one part")
			assert.Length(t, parts[0].Decls, len(kitUnit().Decls), "the part keeps every declaration")
		})

		t.Run("returns the declared split's parts", func(t *testing.T) {
			t.Parallel()

			b := kitBackend(kitName, kitTarget).
				Split(func(u plugin.Unit) []plugin.Unit {
					first, rest := u, u
					first.Decls, rest.Decls = u.Decls[:1], u.Decls[1:]
					return []plugin.Unit{first, rest}
				}).
				Build()
			assert.Length(t, b.SplitUnit(kitUnit()), 2, "the split returns two parts")
		})
	})

	t.Run("Version", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared version", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend(kitName, kitTarget).
				Version(kitVersion).Build().(plugin.Versioned)
			assert.True(t, held, "a kit backend contributes to the run fingerprint")
			assert.Equal(t, b.Version(), kitVersion, "the backend has the declared version")
		})

		t.Run("returns empty for a declaration without a version", func(t *testing.T) {
			t.Parallel()

			b, held := kitBackend(plainName, kitTarget).Build().(plugin.Versioned)
			assert.True(t, held, "a backend without a version has the role")
			assert.Equal(t, b.Version(), "", "the fingerprint folds in nothing for the backend")
		})
	})

	t.Run("Lower", func(t *testing.T) {
		t.Parallel()

		lowering := func() backend.Backend {
			return kitBackend(kitName, kitTarget).Lower(kitLower).Build()
		}
		row := &emit.Struct{
			Origin: coretest.Struct(coretest.StorePath, coretest.StructName).ID,
			Name:   coretest.StructName,
		}

		t.Run("returns the declarations of the declared hook", func(t *testing.T) {
			t.Parallel()

			out, err := lowering().Lower(row)
			assert.NoError(t, err, "the fixture hook takes a struct")
			assert.Equal(t, structNames(t, out),
				[]string{coretest.StructName, coretest.StructName + cursorSuffix},
				"the backend returns the declarations of the hook in the order of the hook")
		})

		t.Run("returns nil for a backend without a lowering", func(t *testing.T) {
			t.Parallel()

			out, err := kitBackend(plainName, kitTarget).Build().Lower(row)
			assert.NoError(t, err, "a backend without a lowering lowers every declaration")
			assert.Nil(t, out, "the settle keeps the declaration")
		})

		t.Run("returns an error rendering an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, lowering())
			assert.HasError(t, err, "a backend with a lowering does not render a store that did not settle")
			assert.Contains(t, err.Error(), "unsettled", "the error states why the render wrote no bytes")
		})

		t.Run("renders the lowered declarations of a settled store", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, lowering())
			assert.Length(t, files, 1, "the fixture renders one file")
			upper := strings.ToUpper(rowName)
			assert.ContainsInOrder(t, string(files[0].Body),
				[]string{"type " + upper + " struct{}", "type " + upper + strings.ToUpper(cursorSuffix) + " struct{}"},
				"the companion of the hook renders after its input")
		})
	})

	t.Run("Respell", func(t *testing.T) {
		t.Parallel()

		respelling := func() backend.Backend {
			return kitBackend(kitName, kitTarget).Respell(kitRespell).Build()
		}

		t.Run("returns the name of the declared hook", func(t *testing.T) {
			t.Parallel()

			got, err := respelling().Respell(symbol.KindInvalid, symbol.KindStruct,
				symbol.VisibilityPublic, coretest.StructName)
			assert.NoError(t, err, "the fixture hook spells every name")
			assert.Equal(t, got, respellPrefix+coretest.StructName, "the backend returns the name of the hook")
		})

		t.Run("returns the name unchanged for a backend without a respell", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(plainName, kitTarget).Build().Respell(symbol.KindInvalid, symbol.KindStruct,
				symbol.VisibilityPublic, coretest.StructName)
			assert.NoError(t, err, "a backend without a respell spells every name")
			assert.Equal(t, got, coretest.StructName, "the backend returns the declared name")
		})

		t.Run("returns an error rendering an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, respelling())
			assert.HasError(t, err, "a backend with a respell does not render a store that did not settle")
			assert.Contains(t, err.Error(), "unsettled", "the error states why the render wrote no bytes")
		})

		t.Run("renders the respelt names of a settled store", func(t *testing.T) {
			t.Parallel()

			files := kitSettled(t, respelling())
			assert.Length(t, files, 1, "the fixture renders one file")
			assert.ContainsInOrder(t, string(files[0].Body), []string{
				"type " + strings.ToUpper(respellPrefix+rowName) + " struct{}",
				"func " + respellPrefix + loadName + "() {",
			}, "every declared name renders through the convention")
		})
	})

	t.Run("RespellMember", func(t *testing.T) {
		t.Parallel()

		member := plugin.Member{Host: symbol.KindStruct, Kind: symbol.KindField, Level: symbol.LevelType}

		t.Run("returns the name of the declared hook of a member", func(t *testing.T) {
			t.Parallel()

			var got plugin.Member
			b := kitBackend(kitName, kitTarget).Respell(kitRespell).RespellMember(
				func(m plugin.Member, name string) (string, error) {
					got = m
					return memberPrefix + name, nil
				}).Build()
			name, err := b.RespellMember(member, coretest.StructName)
			assert.NoError(t, err, "the fixture hook spells every name")
			expect.Equal(t, name, memberPrefix+coretest.StructName, "the backend returns the name of the hook")
			expect.Equal(t, got, member, "the hook reads the member")
		})

		t.Run("returns the name of the respell hook for a backend without the hook of a member", func(t *testing.T) {
			t.Parallel()

			name, err := kitBackend(kitName, kitTarget).Respell(kitRespell).Build().RespellMember(member,
				coretest.StructName)
			assert.NoError(t, err, "the fixture hook spells every name")
			assert.Equal(t, name, respellPrefix+coretest.StructName, "the backend spells the member as any name")
		})

		t.Run("returns an error rendering an unsettled store", func(t *testing.T) {
			t.Parallel()

			err := kitUnsettled(t, kitBackend(kitName, kitTarget).RespellMember(
				func(_ plugin.Member, name string) (string, error) { return name, nil }).Build())
			assert.HasError(t, err, "a backend with the hook of a member does not render a store that did not settle")
			assert.Contains(t, err.Error(), "unsettled", "the error states why the render wrote no bytes")
		})
	})

	t.Run("SpellType", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the reference of the declared spoke", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(kitName, kitTarget).Types(kitSpoke).Build().
				SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got, kitTextRef, "the backend returns the reference of the spoke", assert.ByIdentity())
		})

		t.Run("returns the error of the declared spoke", func(t *testing.T) {
			t.Parallel()

			_, err := kitBackend(kitName, kitTarget).Types(kitSpoke).Build().
				SpellType(rules.Leaf(symbol.FormBool, "bool"), plugin.Policy{})
			assert.HasError(t, err, "the spoke does not spell a bool")
		})

		t.Run("returns an error with the target for a backend without a spoke", func(t *testing.T) {
			t.Parallel()

			_, err := kitBackend(kitName, kitTarget).Build().
				SpellType(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.HasError(t, err, "a backend without a spoke spells no shape")
			assert.Contains(t, err.Error(), "target "+string(kitTarget)+" does not spell the types",
				"the error contains the target")
		})
	})

	t.Run("SpellSupertype", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the reference of the declared supertype spelling", func(t *testing.T) {
			t.Parallel()

			super := &emit.TypeRef{Spelling: kitText}
			got, err := kitBackend(kitName, kitTarget).Types(kitSpoke).
				Supertypes(func(rules.TypeShape, plugin.Policy) (*emit.TypeRef, error) { return super, nil }).Build().
				SpellSupertype(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the supertype spelling spells text")
			assert.Equal(t, got, super, "the backend returns the supertype spelling", assert.ByIdentity())
		})

		t.Run("returns the reference of the spoke for a backend without a supertype spelling", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(kitName, kitTarget).Types(kitSpoke).Build().
				SpellSupertype(rules.Leaf(symbol.FormText, "string"), plugin.Policy{})
			assert.NoError(t, err, "the spoke spells text")
			assert.Equal(t, got, kitTextRef, "a supertype spells as a value", assert.ByIdentity())
		})
	})

	t.Run("Binder", func(t *testing.T) {
		t.Parallel()

		t.Run("serves the method of the declared handler", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(kitName, kitTarget).Binder(kitHandler, kitHome).Build().
				Handler(plugin.RPC{}, plugin.Policy{}, nil)
			assert.NoError(t, err, "the declared handler serves the rpc")
			assert.Equal(t, got, kitMethod, "the backend serves the method of the handler", assert.ByIdentity())
		})

		t.Run("serves the package of the declared home", func(t *testing.T) {
			t.Parallel()

			got, err := kitBackend(kitName, kitTarget).Binder(kitHandler, kitHome).Build().
				Home(symbol.Identity{}, plugin.Placement{})
			assert.NoError(t, err, "the declared home returns a package")
			assert.Equal(t, got, kitHomePkg, "the backend serves the package of the home")
		})
	})

	t.Run("Handler", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the canonical method for a backend without a binder", func(t *testing.T) {
			t.Parallel()

			r := plugin.RPC{
				ID:       symbol.Identity{Lang: "stub", Package: "svc", Owner: "Store", Name: kitMethod.Name},
				Callable: rules.Callable{Name: kitMethod.Name},
			}
			got, err := kitBackend(kitName, kitTarget).Build().Handler(r, plugin.Policy{}, nil)
			assert.NoError(t, err, "the canonical method serves an rpc without a failure")
			expect.Equal(t, got.Origin, r.ID, "the method derives from the rpc")
			expect.Equal(t, got.Name, r.Callable.Name, "the method has the name of the rpc")
		})
	})

	t.Run("Home", func(t *testing.T) {
		t.Parallel()

		t.Run("returns an error for a backend without a binder", func(t *testing.T) {
			t.Parallel()

			_, err := kitBackend(kitName, kitTarget).Build().Home(symbol.Identity{Name: rowName}, plugin.Placement{})
			assert.HasError(t, err, "a backend without a binder has no package for a bound reference")
			assert.Contains(t, err.Error(), "writes against no binding", "the error names the missing binding")
		})
	})

	t.Run("Policies", func(t *testing.T) {
		t.Parallel()

		t.Run("returns the declared specs in declaration order", func(t *testing.T) {
			t.Parallel()

			depth := widthPolicy
			depth.Key = "stub.depth"
			b := kitBackend(kitName, kitTarget).Policies(widthPolicy).Policies(depth).Build()
			assert.Equal(t, b.Policies(), []plugin.PolicySpec{widthPolicy, depth},
				"the specs accumulate in the order of the declarations")
		})

		t.Run("returns no specs for a backend without a declaration", func(t *testing.T) {
			t.Parallel()

			assert.Empty(t, kitBackend(kitName, kitTarget).Build().Policies(), "the backend declares no policy")
		})
	})
}

// TestBackendAllocs checks the allocation ceilings of a declaration and
// of the hooks of a backend, because the ordinary run runs no benchmark.
// New, each merge, each vocabulary part and Build allocate at most their
// ceilings, and the setters allocate nothing. A merge and a Build each
// take a declaration that the setup builds outside the count, because
// each call changes or freezes its declaration. The test does not run in
// parallel, because the count includes the allocations of every
// goroutine.
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
	var built backend.Backend
	assert.MaxAllocsWithSetup(t, func() *backend.Builder { return declaration(false) },
		func(b *backend.Builder) { built = b.Build() }, buildAllocs,
		"Build allocates the parsed templates and the backend")
	assert.Equal(t, built.Name(), kitName, "Build returns the declared backend")
	assert.MaxAllocsWithSetup(t, func() *backend.Builder { return declaration(true) },
		func(b *backend.Builder) { built = b.Build() }, buildAllocs,
		"Build of a declaration with hooks allocates the same as one without")
	assert.Equal(t, built.Name(), kitName, "Build returns the declared backend with hooks")

	plain := kitBackend(plainName, kitTarget).Build()
	row := &emit.Struct{Origin: coretest.Struct(coretest.StorePath, rowName).ID, Name: rowName}
	var (
		lowered []symbol.Symbol
		name    string
		err     error
	)
	assert.MaxAllocs(t, func() { lowered, err = plain.Lower(row) }, 0,
		"Lower without a hook allocates nothing")
	assert.NoError(t, err, "Lower without a hook lowers every declaration")
	assert.Nil(t, lowered, "Lower without a hook keeps the declaration")
	assert.MaxAllocs(t, func() {
		name, err = plain.Respell(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, rowName)
	}, 0, "Respell without a hook allocates nothing")
	assert.NoError(t, err, "Respell without a hook spells every name")
	assert.Equal(t, name, rowName, "Respell without a hook returns the declared name")

	spoken := kitBackend(kitName, kitTarget).Types(kitSpoke).Policies(widthPolicy).Build()
	text := rules.Leaf(symbol.FormText, "string")
	var ref *emit.TypeRef
	assert.MaxAllocs(t, func() { ref, err = spoken.SpellType(text, plugin.Policy{}) }, 0,
		"SpellType allocates what the declared spoke allocates")
	assert.NoError(t, err, "the spoke spells text")
	assert.Equal(t, ref, kitTextRef, "SpellType returns the reference of the spoke", assert.ByIdentity())
	var specs []plugin.PolicySpec
	assert.MaxAllocs(t, func() { specs = spoken.Policies() }, 0, "Policies allocates nothing")
	assert.Length(t, specs, 1, "Policies returns the declared spec")
}

// BenchmarkBackend measures each step of a declaration and the hooks of
// a backend without them. The steps are New, the setters, a merge of
// each map, a vocabulary part and Build.
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
		for _, tt := range []struct {
			name  string
			hooks bool
		}{
			{name: "the fixture declaration", hooks: false},
			{name: "the fixture declaration with both hooks", hooks: true},
		} {
			b.Run(tt.name, func(b *testing.B) {
				c := bench.Start(b).MaxAllocs(buildAllocs)
				defer c.End()
				var (
					builder *backend.Builder
					got     backend.Backend
				)
				for c.Loop() {
					c.Excluding(func() { builder = declaration(tt.hooks) })
					got = builder.Build()
				}
				assert.Equal(b, got.Name(), kitName, "Build returns the declared backend")
			})
		}
	})

	plain := kitBackend(plainName, kitTarget).Build()
	row := &emit.Struct{Origin: coretest.Struct(coretest.StorePath, rowName).ID, Name: rowName}

	b.Run("Lower", func(b *testing.B) {
		b.Run("a backend without a lowering", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got []symbol.Symbol
			for c.Loop() {
				got, _ = plain.Lower(row)
			}
			assert.Nil(b, got, "Lower without a hook keeps the declaration")
		})
	})

	b.Run("Respell", func(b *testing.B) {
		b.Run("a backend without a respell", func(b *testing.B) {
			c := bench.Start(b).MaxAllocs(0)
			defer c.End()
			var got string
			for c.Loop() {
				got, _ = plain.Respell(symbol.KindInvalid, symbol.KindStruct, symbol.VisibilityPublic, rowName)
			}
			assert.Equal(b, got, rowName, "Respell without a hook returns the declared name")
		})
	})

	spoken := kitBackend(kitName, kitTarget).Types(kitSpoke).Policies(widthPolicy).Build()

	b.Run("SpellType", func(b *testing.B) {
		text := rules.Leaf(symbol.FormText, "string")
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var ref *emit.TypeRef
		for c.Loop() {
			ref, _ = spoken.SpellType(text, plugin.Policy{})
		}
		assert.Equal(b, ref, kitTextRef, "SpellType returns the reference of the spoke", assert.ByIdentity())
	})

	b.Run("Policies", func(b *testing.B) {
		c := bench.Start(b).MaxAllocs(0)
		defer c.End()
		var specs []plugin.PolicySpec
		for c.Loop() {
			specs = spoken.Policies()
		}
		assert.Length(b, specs, 1, "Policies returns the declared spec")
	})
}

// kitSyntax returns the comment syntax of the fixture's language.
func kitSyntax() plugin.CommentSyntax {
	return plugin.CommentSyntax{Line: []string{"//"}}
}

// kitStructs returns the struct template of the fixture's language. A
// satellite splits its kind templates across declarations in the same
// way. The struct template calls the shared helper, so the vocabulary
// takes part in every render.
func kitStructs() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindStruct: "type {{" + upHelper + " .Name}} struct{}\n",
	}
}

// kitCallables returns the function template of the fixture's language.
func kitCallables() map[symbol.Kind]string {
	return map[symbol.Kind]string{
		symbol.KindFunction: "func {{.Name}}() {\n{{body .}}}\n",
	}
}

// kitFuncs is the shared template vocabulary of the fixture. It binds
// nothing to the import set of the file.
func kitFuncs(*render.ImportSet) template.FuncMap {
	return template.FuncMap{upHelper: strings.ToUpper}
}

// kitBinding is a vocabulary part that binds to the import set of the
// file. Its helper binds a package under the last segment of its path,
// and qualifies a name with the bound name.
func kitBinding(set *render.ImportSet) template.FuncMap {
	return template.FuncMap{qualifyHelper: func(pkg, name string) string {
		return set.Bind(pkg, path.Base(pkg)) + "." + name
	}}
}

// kitNaming returns the word of a unit followed by the fixture's
// extension.
func kitNaming(u plugin.Unit) string { return u.Word + kitExt }

// kitScaffold writes a return statement, and returns an error for every
// other kind. It records an import, as a printer records each package
// that a qualified name needs.
func kitScaffold(s emit.Stmt, set *render.ImportSet) ([]byte, error) {
	if s.Kind != emit.StmtReturn {
		return nil, errors.New("the fixture spells returns only")
	}
	set.Add(runtimePkg)
	return []byte("\treturn\n"), nil
}

// kitValue writes the text of a value. It records an import, as a writer
// records each package that a qualified name needs.
func kitValue(v emit.Value, set *render.ImportSet) (string, error) {
	set.Add(runtimePkg)
	return v.Text, nil
}

// kitImports renders the collected set as a one-line block.
func kitImports(set *render.ImportSet) string {
	if set.Len() == 0 {
		return ""
	}
	return "import (" + strings.Join(set.Paths(), " ") + ")\n"
}

// kitFinalise returns the source unchanged.
func kitFinalise(src []byte) ([]byte, error) { return src, nil }

// kitLanguage returns the fixture's language as a render.Language value.
// A case builds a pass from it by hand and compares the pass with the
// backend of Build.
func kitLanguage() render.Language {
	kinds := kitStructs()
	maps.Copy(kinds, kitCallables())
	return render.Language{
		Kinds: kinds, File: kitFile, Funcs: kitFuncs,
		Naming: kitNaming, Scaffold: kitScaffold,
		Imports: kitImports, Finalise: kitFinalise,
	}
}

// kitBackend returns the full fixture declaration. A case builds it, or
// changes one part of it before the build.
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

// kitUnit returns a plan unit with a struct and a function. The body of
// the function contains one return statement.
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

// kitLower is the fixture's lowering. It lowers a struct to the struct
// and a companion with the same origin. It returns nil for every other
// declaration, so the settle keeps it.
func kitLower(s symbol.Symbol) ([]symbol.Symbol, error) {
	row, held := s.(*emit.Struct)
	if !held {
		return nil, nil
	}
	return []symbol.Symbol{row, &emit.Struct{
		Origin: row.Origin, Name: row.Name + cursorSuffix,
	}}, nil
}

// kitRespell is the fixture's name convention. It puts respellPrefix
// before every name of every kind.
func kitRespell(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
	return respellPrefix + name, nil
}

// structNames returns the names of the declarations that a lowering
// returns. The fixture's hook returns structs alone.
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

// kitStore returns an unsettled store with the fixture's unit.
func kitStore(tb assert.TB) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	assert.NoError(tb, e.Add(kitUnit()), "the fixture unit is added")
	return e
}

// kitUnsettled renders the fixture's store through b without a settle,
// and returns the error of the render.
func kitUnsettled(tb assert.TB, b plugin.Backend) error {
	tb.Helper()

	_, err := b.Render(&plugin.RenderContext{
		Emit: kitStore(tb), Sink: diag.NewSink(), Plugin: kitName,
	})
	return err
}

// kitSettled settles the fixture's store through b and renders it. It
// asserts that both steps run clean.
func kitSettled(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	e := kitStore(tb)
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(e, b, nil, sink), "the plan settles once")
	coretest.AssertCodes(tb, sink)
	files, err := b.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, b), Sink: sink, Plugin: kitName,
	})
	assert.NoError(tb, err, "the settled store renders")
	coretest.AssertCodes(tb, sink)
	return files
}

// kitRender renders the fixture's store through b, and asserts that the
// render runs clean.
func kitRender(tb assert.TB, b plugin.Backend) []plugin.RenderedFile {
	tb.Helper()

	e := kitStore(tb)
	sink := diag.NewSink()
	files, err := b.Render(&plugin.RenderContext{
		Emit: e, Files: backendtest.Files(e, b), Sink: sink, Plugin: kitName,
	})
	assert.NoError(tb, err, "the pass renders every file")
	coretest.AssertCodes(tb, sink)
	return files
}

// setters returns every setter that writes one field of a declaration.
func setters() []setter {
	split := render.Split(func(u plugin.Unit) []plugin.Unit { return []plugin.Unit{u} })
	rule := plugin.PackageRule(func(p plugin.Placement) (symbol.Identity, error) { return p.Origin, nil })
	cluster := render.Cluster(func([]symbol.Symbol) []render.Clustered { return nil })
	evaluated := render.Evaluated(func(symbol.Symbol, func(string)) {})
	return []setter{
		{name: "Version", set: func(b *backend.Builder) *backend.Builder { return b.Version(kitVersion) }},
		{name: "FileTemplate", set: func(b *backend.Builder) *backend.Builder { return b.FileTemplate(kitFile) }},
		{name: "Scaffold", set: func(b *backend.Builder) *backend.Builder { return b.Scaffold(kitScaffold) }},
		{name: "Value", set: func(b *backend.Builder) *backend.Builder { return b.Value(kitValue) }},
		{name: "Naming", set: func(b *backend.Builder) *backend.Builder { return b.Naming(kitNaming) }},
		{name: "Split", set: func(b *backend.Builder) *backend.Builder { return b.Split(split) }},
		{name: "Packages", set: func(b *backend.Builder) *backend.Builder { return b.Packages(rule) }},
		{name: "Cluster", set: func(b *backend.Builder) *backend.Builder { return b.Cluster(cluster) }},
		{name: "Evaluated", set: func(b *backend.Builder) *backend.Builder { return b.Evaluated(evaluated) }},
		{name: "Implemented", set: func(b *backend.Builder) *backend.Builder { return b.Implemented() }},
		{name: "Imports", set: func(b *backend.Builder) *backend.Builder { return b.Imports(kitImports) }},
		{name: "Finalise", set: func(b *backend.Builder) *backend.Builder { return b.Finalise(kitFinalise) }},
		{name: "Coverage", set: func(b *backend.Builder) *backend.Builder { return b.Coverage(render.Coverage{}) }},
		{name: "Lower", set: func(b *backend.Builder) *backend.Builder { return b.Lower(kitLower) }},
		{name: "Respell", set: func(b *backend.Builder) *backend.Builder { return b.Respell(kitRespell) }},
		{name: "Types", set: func(b *backend.Builder) *backend.Builder { return b.Types(kitSpoke) }},
		{name: "Supertypes", set: func(b *backend.Builder) *backend.Builder { return b.Supertypes(kitSpoke) }},
		{name: "MemberIndent", set: func(b *backend.Builder) *backend.Builder { return b.MemberIndent(kitIndent) }},
		{name: "Binder", set: func(b *backend.Builder) *backend.Builder { return b.Binder(kitHandler, kitHome) }},
		{
			name: "TemplateImport",
			set: func(b *backend.Builder) *backend.Builder {
				return b.TemplateImport(func(string, string) (string, string, bool) { return "", "", false })
			},
		},
	}
}

// merges returns every method that merges entries into a declaration.
// Each merges one entry, and has its ceiling.
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

// declaration returns the whole fixture declaration, with both settle
// hooks where hooks is true.
func declaration(hooks bool) *backend.Builder {
	b := kitBackend(kitName, kitTarget)
	if hooks {
		b.Lower(kitLower).Respell(kitRespell)
	}
	return b
}
