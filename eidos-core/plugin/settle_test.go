// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"cmp"
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The target every settle backend renders to, the backend's name,
// and the plugin the name stamps speak for.
const (
	settleTarget  plugin.Target = "t"
	settleBackend plugin.ID     = "golang"
	stampPlugin                 = "fixture"
)

// The documentation the name key registers with, the name the
// override cases write, and the name a kept file settles row to.
const (
	nameDoc      = "a declaration's name in the test target"
	overrideName = "Record"
	keptRow      = "KeptRow"
)

// nestedState is the enum state that the struct box declares. Its flat
// name is boxState.
var nestedState = symbol.Identity{Lang: "fixture", Package: "svc", Owner: "box", Name: "state", Kind: symbol.KindEnum}

// The store the allocation check and the benchmark settle, and the
// allocations of one settle of it through a respell hook that keeps
// every name.
const (
	// settleUnits is the number of units the store has.
	settleUnits = 100
	// keepingSettleAllocs is the per-kind index rebuilt, a list for each
	// unit and kind and the kinds' maps as they grow, 224 allocations,
	// and the plan of the names with the tables the references follow,
	// each sized once to the store, 19 allocations.
	keepingSettleAllocs = 224 + 19
	// readLogAllocs is what SettleWith adds to a settle of a store whose
	// units read nothing: the read log.
	readLogAllocs = 1
)

// hookless is the backend without hooks: a name and a target, and
// nothing the settle would run.
type hookless struct{ name plugin.ID }

// Name returns the backend's name, the origin of its findings.
func (b *hookless) Name() plugin.ID { return b.name }

// Target returns the target every case renders to.
func (*hookless) Target() plugin.Target { return settleTarget }

// loweringOnly declares the construct seam alone.
type loweringOnly struct {
	hookless
	fn plugin.Lower
}

// Lower lowers one declaration through the case's function.
func (b *loweringOnly) Lower(s symbol.Symbol) ([]symbol.Symbol, error) { return b.fn(s) }

// respellingOnly declares the name seam alone.
type respellingOnly struct {
	hookless
	fn plugin.Respell
}

// Respell spells one name through the case's function.
func (b *respellingOnly) Respell(
	host, kind symbol.Kind, v symbol.Visibility, name string,
) (string, error) {
	return b.fn(host, kind, v, name)
}

// lowering returns a backend whose lowering hook is fn.
func lowering(fn plugin.Lower) *loweringOnly {
	b := &loweringOnly{fn: fn}
	b.name = settleBackend
	return b
}

// respelling returns a backend whose respell hook is fn.
func respelling(fn plugin.Respell) *respellingOnly {
	b := &respellingOnly{fn: fn}
	b.name = settleBackend
	return b
}

// prefixing returns a backend whose respell hook prefixes every name
// it is handed, so a case reads the settled spelling off the emitted
// one.
func prefixing(prefix string) *respellingOnly {
	return respelling(func(_, _ symbol.Kind, _ symbol.Visibility, n string) (string, error) {
		return prefix + n, nil
	})
}

// capitalizing returns a backend whose respell hook upper-cases a
// name's first letter, the convention the override cases settle
// under.
func capitalizing() *respellingOnly {
	return respelling(func(_, _ symbol.Kind, _ symbol.Visibility, n string) (string, error) {
		return strings.ToUpper(n[:1]) + n[1:], nil
	})
}

// respelledStore is the reference-following fixture after its
// settle: a struct whose field names it and whose method's body
// reads a local and a parameter, an alias resolved to the struct, a
// variable of a composite spelling without structure, two variables
// whose bare references to the struct's name state a package, another
// one and the struct's own, and variables of structural references
// over the struct's name.
type respelledStore struct {
	box       *emit.Struct
	fetch     *emit.Method
	match     *emit.Alias
	composite *emit.Variable
	foreign   *emit.Variable
	own       *emit.Variable
	listed    *emit.Variable
	nested    *emit.Variable
	argued    *emit.Variable
	keyed     *emit.Variable
	labeled   *emit.Variable
	qualified *emit.Variable
}

// settleRespelled builds the reference-following fixture and settles
// it under a hook that marks each name by its place: T at file
// level, p on a parameter and m on a member.
func settleRespelled(t *testing.T) respelledStore {
	t.Helper()

	boxOrigin := settleOrigin("box", symbol.KindStruct)
	box := &emit.Struct{Origin: boxOrigin, Name: "box"}
	box.Fields.Append(&emit.Field{Name: "item", Type: &emit.TypeRef{Spelling: "box"}})
	fetch := &emit.Method{
		Name:   "fetch",
		Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
		Body: emit.Body{Stmts: []emit.Stmt{
			{
				Kind: emit.StmtAssign, Names: []string{"state"}, Declare: true,
				Value: emit.Expr{
					Kind: emit.ExprCall,
					Fn:   &emit.Expr{Kind: emit.ExprName, Name: "begin"},
				},
			},
			{Kind: emit.StmtExpr, Value: emit.Expr{
				Kind: emit.ExprCall,
				Fn:   &emit.Expr{Kind: emit.ExprName, Name: "commit"},
				Args: []emit.Expr{
					{Kind: emit.ExprName, Name: "state"},
					{Kind: emit.ExprName, Name: "rowCount"},
				},
			}},
		}},
	}
	box.Methods.Append(fetch)
	match := &emit.Alias{
		Origin: settleOrigin("match", symbol.KindAlias),
		Name:   "match",
		Target: &emit.TypeRef{Spelling: "box", Target: boxOrigin},
	}
	composite := &emit.Variable{
		Origin: settleOrigin("pool", symbol.KindVariable),
		Name:   "pool",
		Type:   &emit.TypeRef{Spelling: "[]box"},
	}
	foreign := &emit.Variable{
		Origin: settleOrigin("other", symbol.KindVariable),
		Name:   "other",
		Type:   &emit.TypeRef{Spelling: "box", Package: "elsewhere"},
	}
	own := &emit.Variable{
		Origin: settleOrigin("own", symbol.KindVariable),
		Name:   "own",
		Type:   &emit.TypeRef{Spelling: "box", Package: "svc"},
	}
	boxRef := func() *emit.TypeRef { return &emit.TypeRef{Spelling: "box"} }
	listed := variableOf("listed", structural(symbol.FormList, "[]box", boxRef()))
	nested := variableOf("nested", structural(symbol.FormList, "[]*box",
		structural(symbol.FormOptional, "*box", boxRef())))
	argued := variableOf("argued", structural(symbol.FormList, "[]Pair[box]",
		&emit.TypeRef{Spelling: "Pair", Args: []*emit.TypeRef{boxRef()}}))
	keyed := variableOf("keyed", structural(symbol.FormMap, "map[inbox]box",
		&emit.TypeRef{Spelling: "inbox"}, boxRef()))
	labeled := variableOf("labeled", &emit.TypeRef{
		Form: symbol.FormFunc, Spelling: "func(boxed box) error", Split: 1,
		Elems: []*emit.TypeRef{boxRef(), {Spelling: "error"}},
	})
	qualified := variableOf("qualified", structural(symbol.FormMap, "map[other.box]box",
		&emit.TypeRef{Spelling: "other.box", Package: "elsewhere"}, boxRef()))
	b := respelling(func(host, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		switch {
		case host == symbol.KindInvalid:
			return "T" + name, nil
		case kind == symbol.KindParam:
			return "p" + name, nil
		default:
			return "m" + name, nil
		}
	})
	e := storeOf(t,
		settleUnit("svc", "svc/a.src", box, match),
		settleUnit("svc", "svc/b.src", composite, foreign, own),
		settleUnit("svc", "svc/c.src", listed, nested, argued, keyed, labeled, qualified),
	)
	coretest.AssertCodes(t, settled(t, e, b))
	return respelledStore{
		box: box, fetch: fetch, match: match, composite: composite, foreign: foreign, own: own,
		listed: listed, nested: nested, argued: argued, keyed: keyed, labeled: labeled, qualified: qualified,
	}
}

// ambiguousRow returns a hook that settles a struct and a function
// of one emitted name apart, and the two declarations in two units,
// so a bare reference to the name matches diverging spellings.
func ambiguousRow(extra ...symbol.Symbol) (*respellingOnly, []plugin.Unit) {
	b := respelling(func(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		if kind == symbol.KindStruct {
			return "S" + name, nil
		}
		return "F" + name, nil
	})
	rowStruct := &emit.Struct{Origin: settleOrigin("row", symbol.KindStruct), Name: "row"}
	rowFn := &emit.Function{Origin: settleOrigin("rowfn", symbol.KindFunction), Name: "row"}
	other := settleUnit("svc", "svc/b.src", rowFn)
	other.Plugin = "second"
	return b, []plugin.Unit{settleUnit("svc", "svc/a.src", append([]symbol.Symbol{rowStruct}, extra...)...), other}
}

// protectedRefusing returns a hook that refuses a protected name and
// prefixes a parameter with p.
func protectedRefusing() *respellingOnly {
	return respelling(func(_, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
		if v == symbol.VisibilityProtected {
			return "", errors.New("go: no case spells a protected scope")
		}
		if kind == symbol.KindParam {
			return "p" + name, nil
		}
		return name, nil
	})
}

// verbatimLoad returns a function whose verbatim body reads its
// parameter, under a hook that respells a parameter to snake case.
func verbatimLoad() (*emit.Function, *respellingOnly) {
	load := &emit.Function{
		Origin: settleOrigin("load", symbol.KindFunction),
		Name:   "load",
		Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
		Body:   emit.Body{Verbatim: "\tuse(rowCount)\n"},
	}
	b := respelling(func(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
		if kind == symbol.KindParam {
			return strings.ToLower(name) + "_p", nil
		}
		return name, nil
	})
	return load, b
}

// The settle is the stage between a plan's schedule and its render:
// constructs lower, names respell, references follow, and the store
// marks itself settled once.
func TestSettle(t *testing.T) {
	t.Parallel()

	t.Run("Settle", func(t *testing.T) {
		t.Parallel()

		t.Run("marks the store settled", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}))
			settled(t, e, prefixing("p"))
			assert.True(t, e.Settled(), "the store marks settled")
		})

		t.Run("runs no hook over a settled store", func(t *testing.T) {
			t.Parallel()

			calls := 0
			b := respelling(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				calls++
				return name, nil
			})
			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}))
			settled(t, e, b)
			before := calls
			settled(t, e, b)
			assert.Equal(t, calls, before, "a settled store settles to itself")
		})

		t.Run("leaves every declaration as emitted for a backend without hooks", func(t *testing.T) {
			t.Parallel()

			count := &emit.Variable{Origin: settleOrigin("count", symbol.KindVariable), Name: "count"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", count))
			settled(t, e, &hookless{name: settleBackend})
			assert.Equal(t, count.Name, "count", "the name is the emitted one")
		})

		// row is a declaration of the fixture language, which is another
		// language than the backends' target.
		row := settleOrigin("row", symbol.KindStruct)
		translations := []struct {
			name    string
			backend plugin.Backend
			give    *emit.TypeRef
			want    bool
		}{
			{
				name: "records a translated reference under a backend with a respell hook", backend: keeping(),
				give: &emit.TypeRef{Spelling: "row", Target: row}, want: true,
			},
			{
				name:    "records a translated reference under a backend with a lowering hook alone",
				backend: lowering(func(symbol.Symbol) ([]symbol.Symbol, error) { return nil, nil }),
				give:    &emit.TypeRef{Spelling: "row", Target: row}, want: true,
			},
			{
				name: "records a translated reference under a backend without hooks", backend: &hookless{},
				give: &emit.TypeRef{Spelling: "row", Target: row}, want: true,
			},
			{
				name: "records no translated reference for a target of the backend's language", backend: keeping(),
				give: &emit.TypeRef{Spelling: "row", Target: symbol.Identity{Lang: symbol.Lang(settleTarget)}},
				want: false,
			},
			{
				name: "records no translated reference for a bare reference", backend: &hookless{},
				give: &emit.TypeRef{Spelling: "row"}, want: false,
			},
		}
		for _, tt := range translations {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				holder := &emit.Struct{Origin: settleOrigin("holder", symbol.KindStruct), Name: "holder"}
				holder.Fields.Append(&emit.Field{Name: "next", Type: tt.give})
				e := storeOf(t, settleUnit("svc", "svc/a.src", holder))
				settled(t, e, tt.backend)
				assert.Equal(t, e.Translates(), tt.want, "the store records whether a reference translates")
			})
		}

		t.Run("records no translated reference of a withheld declaration", func(t *testing.T) {
			t.Parallel()

			holder := &emit.Struct{Origin: settleOrigin("holder", symbol.KindStruct), Name: "holder"}
			holder.Fields.Append(&emit.Field{Name: "next", Type: &emit.TypeRef{Spelling: "row", Target: row}})
			e := storeOf(t, settleUnit("svc", "svc/a.src", holder))
			refusing := respelling(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				return "", errors.New("plugin_test: the target refuses " + name)
			})
			coretest.AssertCodes(t, settled(t, e, refusing), plugin.RefusedName)
			assert.False(t, e.Translates(), "a withheld declaration renders no reference")
		})

		t.Run("lowers a construct into the target's shapes", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Alias{Origin: settleOrigin("state", symbol.KindEnum), Name: "state"}))
			coretest.AssertCodes(t, settled(t, e, lowering(splitAlias)))
			for u := range e.Units() {
				assert.Length(t, u.Decls, 2, "one declaration lowers into two")
			}
		})

		t.Run("rebuilds the per-kind index over the lowered declarations", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Alias{Origin: settleOrigin("state", symbol.KindEnum), Name: "state"}))
			settled(t, e, lowering(splitAlias))
			aliases, structs := 0, 0
			for range e.ByKind(symbol.KindAlias) {
				aliases++
			}
			for range e.ByKind(symbol.KindStruct) {
				structs++
			}
			assert.Equal(t, []int{aliases, structs}, []int{0, 1},
				"the index lists the target's shape and not the lowered kind")
		})

		t.Run("discards the reference of a declaration a lowering replaced", func(t *testing.T) {
			t.Parallel()

			alias := &emit.Alias{Origin: settleOrigin("state", symbol.KindEnum), Name: "state"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", alias))
			_, held := e.Ref(alias)
			assert.True(t, held, "the emitted alias has a reference before the settle")
			settled(t, e, lowering(splitAlias))
			_, held = e.Ref(alias)
			assert.False(t, held, "the lowered alias has none after it")
		})

		t.Run("keeps the declaration a lowering returns nil for", func(t *testing.T) {
			t.Parallel()

			kept := &emit.Method{Origin: settleOrigin("track", symbol.KindMethod), Name: "track", Async: true}
			e := storeOf(t, settleUnit("svc", "svc/a.src", kept))
			coretest.AssertCodes(t, settled(t, e, lowering(clearAsync)))
			for u := range e.Units() {
				assert.Equal(t, u.Decls, []symbol.Symbol{kept}, "the declaration itself is kept")
			}
		})

		t.Run("keeps the in-place rewrite of a lowering that returns nil", func(t *testing.T) {
			t.Parallel()

			kept := &emit.Method{Origin: settleOrigin("track", symbol.KindMethod), Name: "track", Async: true}
			e := storeOf(t, settleUnit("svc", "svc/a.src", kept))
			settled(t, e, lowering(clearAsync))
			assert.False(t, kept.Async, "the lowering rewrote the declaration in place")
		})

		t.Run("reports RefusedConstruct for a construct that fails to lower", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Sum{Origin: settleOrigin("shape", symbol.KindSum), Name: "shape"}))
			coretest.AssertCodes(t, settled(t, e, lowering(refuseSum)), plugin.RefusedConstruct)
		})

		t.Run("withholds a construct that fails to lower", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Sum{Origin: settleOrigin("shape", symbol.KindSum), Name: "shape"},
				&emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"},
			))
			settled(t, e, lowering(refuseSum))
			for u := range e.Units() {
				assert.Equal(t, []symbol.Kind{u.Decls[0].Kind()}, []symbol.Kind{symbol.KindConstant},
					"only the sibling is left")
			}
		})

		t.Run("returns an error for a lowering that drops its input's origin", func(t *testing.T) {
			t.Parallel()

			b := lowering(func(symbol.Symbol) ([]symbol.Symbol, error) {
				return []symbol.Symbol{&emit.Struct{Name: "made"}}, nil
			})
			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Alias{Origin: settleOrigin("state", symbol.KindAlias), Name: "state"}))
			err := plugin.Settle(e, b, nil, diag.NewSink())
			assert.HasError(t, err, "every output has the input's origin")
		})

		respelled := []struct {
			name string
			got  func(respelledStore) string
			want string
		}{
			{
				name: "respells a file-level name",
				got:  func(s respelledStore) string { return s.box.Name }, want: "Tbox",
			},
			{
				name: "respells a field under its host",
				got:  func(s respelledStore) string { return s.box.Fields.Items()[0].Name }, want: "mitem",
			},
			{
				name: "respells a method under its host",
				got:  func(s respelledStore) string { return s.fetch.Name }, want: "mfetch",
			},
			{
				name: "respells a parameter under its callable",
				got:  func(s respelledStore) string { return s.fetch.Params[0].Name }, want: "prowCount",
			},
			{
				name: "rewrites a resolved reference to its origin's settled name",
				got:  func(s respelledStore) string { return s.match.Target.Spelling }, want: "Tbox",
			},
			{
				name: "rewrites a bare reference through the package's table",
				got:  func(s respelledStore) string { return s.box.Fields.Items()[0].Type.Spelling }, want: "Tbox",
			},
			{
				name: "leaves a composite spelling without elements as written",
				got:  func(s respelledStore) string { return s.composite.Type.Spelling }, want: "[]box",
			},
			{
				name: "rewrites a structural reference's spelling with its element's settled name",
				got:  func(s respelledStore) string { return s.listed.Type.Spelling }, want: "[]Tbox",
			},
			{
				name: "rewrites a structural reference's spelling with a nested element's settled name",
				got:  func(s respelledStore) string { return s.nested.Type.Spelling }, want: "[]*Tbox",
			},
			{
				name: "rewrites a nested structural reference's spelling with its element's settled name",
				got:  func(s respelledStore) string { return s.nested.Type.Elems[0].Spelling }, want: "*Tbox",
			},
			{
				name: "rewrites a structural reference's spelling with a type argument's settled name",
				got:  func(s respelledStore) string { return s.argued.Type.Spelling }, want: "[]Pair[Tbox]",
			},
			{
				name: "leaves a longer name that ends in an element's emitted name as written",
				got:  func(s respelledStore) string { return s.keyed.Type.Spelling }, want: "map[inbox]Tbox",
			},
			{
				name: "leaves a parameter label that begins with an element's emitted name as written",
				got:  func(s respelledStore) string { return s.labeled.Type.Spelling }, want: "func(boxed Tbox) error",
			},
			{
				name: "leaves a qualified name in a structural reference's spelling as written",
				got:  func(s respelledStore) string { return s.qualified.Type.Spelling }, want: "map[other.box]Tbox",
			},
			{
				name: "leaves a bare reference that states another package as written",
				got:  func(s respelledStore) string { return s.foreign.Type.Spelling }, want: "box",
			},
			{
				name: "rewrites a bare reference that states its own package through the package's table",
				got:  func(s respelledStore) string { return s.own.Type.Spelling }, want: "Tbox",
			},
			{
				name: "leaves a declared local as written",
				got:  func(s respelledStore) string { return s.fetch.Body.Stmts[1].Value.Args[0].Name }, want: "state",
			},
			{
				name: "rewrites a body reference to its parameter's settled name",
				got: func(s respelledStore) string {
					return s.fetch.Body.Stmts[1].Value.Args[1].Name
				},
				want: "prowCount",
			},
			{
				name: "leaves an undeclared callable as written",
				got:  func(s respelledStore) string { return s.fetch.Body.Stmts[1].Value.Fn.Name }, want: "commit",
			},
		}
		for _, tt := range respelled {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.got(settleRespelled(t)), tt.want, "the spelling after the settle")
			})
		}

		t.Run("reports CollidingNames for two names settling to one in a package", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				&emit.Constant{Origin: settleOrigin("alpha", symbol.KindConstant), Name: "alpha", Value: "1"},
				&emit.Constant{Origin: settleOrigin("beta", symbol.KindConstant), Name: "beta", Value: "2"},
			))
			coretest.AssertCodes(t, settled(t, e, respelling(same)), plugin.CollidingNames)
		})

		t.Run("keeps the emitted names of a package collision", func(t *testing.T) {
			t.Parallel()

			alpha := &emit.Constant{Origin: settleOrigin("alpha", symbol.KindConstant), Name: "alpha", Value: "1"}
			beta := &emit.Constant{Origin: settleOrigin("beta", symbol.KindConstant), Name: "beta", Value: "2"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", alpha, beta))
			settled(t, e, respelling(same))
			assert.Equal(t, []string{alpha.Name, beta.Name}, []string{"alpha", "beta"},
				"both names are the emitted ones")
		})

		t.Run("orders collision findings by scope before settled spelling", func(t *testing.T) {
			t.Parallel()

			settles := map[string]string{
				"alpha": "Same", "beta": "Same",
				"gamma": "Other", "delta": "Other",
				"epsilon": "Same", "zeta": "Same",
			}
			b := respelling(func(_, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				return settles[name], nil
			})
			constant := func(name string) *emit.Constant {
				return &emit.Constant{Origin: settleOrigin(name, symbol.KindConstant), Name: name, Value: "1"}
			}
			e := storeOf(t,
				settleUnit("svc", "svc/a.src",
					constant("gamma"), constant("delta"), constant("epsilon"), constant("zeta")),
				settleUnit("api", "api/a.src", constant("alpha"), constant("beta")),
			)
			got := messages(settled(t, e, b))
			assert.Length(t, got, 3, "one finding per group")
			expect.Contains(t, got[0], "in api", "the scope orders the groups")
			expect.Contains(t, got[1], `to "Other"`, "and inside one scope the settled spelling does")
			expect.Contains(t, got[2], `to "Same"`, "so the last group settles to the latest spelling")
		})

		t.Run("settles one method name on two receivers without a collision", func(t *testing.T) {
			t.Parallel()

			first := &emit.Method{
				Origin: settleOrigin("row.track", symbol.KindMethod), Name: "track",
				Receives: &emit.TypeRef{Spelling: "row"},
			}
			second := &emit.Method{
				Origin: settleOrigin("box.track", symbol.KindMethod), Name: "track",
				Receives: &emit.TypeRef{Spelling: "box"},
			}
			e := storeOf(t, settleUnit("svc", "svc/a.src", first, second))
			coretest.AssertCodes(t, settled(t, e, capitalizing()))
			assert.Equal(t, []string{first.Name, second.Name}, []string{"Track", "Track"},
				"each receiver declares the settled name")
		})

		t.Run("keeps the emitted names of two spellings of one method on one receiver", func(t *testing.T) {
			t.Parallel()

			twice := &emit.Method{
				Origin: settleOrigin("row.track2", symbol.KindMethod), Name: "Track",
				Receives: &emit.TypeRef{Spelling: "row"},
			}
			again := &emit.Method{
				Origin: settleOrigin("row.track3", symbol.KindMethod), Name: "track",
				Receives: &emit.TypeRef{Spelling: "row"},
			}
			e := storeOf(t, settleUnit("svc", "svc/a.src", twice, again))
			coretest.AssertCodes(t, settled(t, e, capitalizing()), plugin.CollidingNames)
			assert.Equal(t, []string{twice.Name, again.Name}, []string{"Track", "track"},
				"each keeps its emitted name")
		})

		t.Run("keeps the emitted names of a member collision", func(t *testing.T) {
			t.Parallel()

			b := respelling(func(host, _ symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				if host == symbol.KindInvalid {
					return name, nil
				}
				return "same", nil
			})
			box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
			box.Fields.Append(
				&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: "int"}},
				&emit.Field{Name: "y", Type: &emit.TypeRef{Spelling: "int"}},
			)
			e := storeOf(t, settleUnit("svc", "svc/a.src", box))
			coretest.AssertCodes(t, settled(t, e, b), plugin.CollidingNames)
			fields := box.Fields.Items()
			assert.Equal(t, []string{fields[0].Name, fields[1].Name}, []string{"x", "y"},
				"the members keep their emitted names")
		})

		t.Run("reports AmbiguousReference for a bare reference to diverging names", func(t *testing.T) {
			t.Parallel()

			holder := &emit.Variable{
				Origin: settleOrigin("ref", symbol.KindVariable), Name: "ref",
				Type: &emit.TypeRef{Spelling: "row"},
			}
			b, units := ambiguousRow(holder)
			coretest.AssertReports(t, settled(t, storeOf(t, units...), b), plugin.AmbiguousReference)
		})

		t.Run("leaves an ambiguous bare reference as written", func(t *testing.T) {
			t.Parallel()

			holder := &emit.Variable{
				Origin: settleOrigin("ref", symbol.KindVariable), Name: "ref",
				Type: &emit.TypeRef{Spelling: "row"},
			}
			b, units := ambiguousRow(holder)
			settled(t, storeOf(t, units...), b)
			assert.Equal(t, holder.Type.Spelling, "row", "the reference is left as written")
		})

		t.Run("reports RefusedName for a name that fails to respell", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", protectedBox()))
			coretest.AssertCodes(t, settled(t, e, protectedRefusing()), plugin.RefusedName)
		})

		t.Run("withholds a declaration whose name fails to respell", func(t *testing.T) {
			t.Parallel()

			keep := &emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", protectedBox(), keep))
			settled(t, e, protectedRefusing())
			for u := range e.Units() {
				assert.Equal(t, u.Decls, []symbol.Symbol{keep}, "only the sibling is left")
			}
		})

		t.Run("reports VerbatimParams for a verbatim body's renamed parameter", func(t *testing.T) {
			t.Parallel()

			load, b := verbatimLoad()
			e := storeOf(t, settleUnit("svc", "svc/a.src", load))
			coretest.AssertCodes(t, settled(t, e, b), plugin.VerbatimParams)
		})

		t.Run("keeps the parameter name a verbatim body reads", func(t *testing.T) {
			t.Parallel()

			load, b := verbatimLoad()
			e := storeOf(t, settleUnit("svc", "svc/a.src", load))
			settled(t, e, b)
			assert.Equal(t, load.Params[0].Name, "rowCount", "the spelling the text reads")
		})

		t.Run("leaves a verbatim body's text untouched", func(t *testing.T) {
			t.Parallel()

			load, b := verbatimLoad()
			e := storeOf(t, settleUnit("svc", "svc/a.src", load))
			settled(t, e, b)
			assert.Equal(t, load.Body.Verbatim, "\tuse(rowCount)\n", "the text as emitted")
		})

		t.Run("names the method in a verbatim pin's finding", func(t *testing.T) {
			t.Parallel()

			scan := &emit.Method{
				Origin:   settleOrigin("scan", symbol.KindMethod),
				Name:     "Scan",
				Receives: &emit.TypeRef{Spelling: "row"},
				Params:   []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
				Body:     emit.Body{Verbatim: "\tuse(rowCount)\n"},
			}
			b := respelling(func(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				if kind == symbol.KindParam {
					return strings.ToLower(name) + "_p", nil
				}
				return strings.ToLower(name), nil
			})
			e := storeOf(t, settleUnit("svc", "svc/a.src", scan))
			assert.Contains(t, messages(settled(t, e, b))[0], "scan",
				"the finding names the callable whose signature the text pinned")
		})

		t.Run("respells a parameter outside a callable", func(t *testing.T) {
			t.Parallel()

			loose := &emit.Param{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}
			e := storeOf(t, settleUnit("svc", "svc/a.src", loose))
			coretest.AssertCodes(t, settled(t, e, prefixing("p")))
			assert.Equal(t, loose.Name, "prowCount", "no verbatim body reads it")
		})

		t.Run("leaves a parameter the hook added during planning as emitted", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{
				Origin: settleOrigin("load", symbol.KindFunction),
				Name:   "load",
				Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
			}
			grown := false
			// A hook that edits the declaration it is handed is a
			// backend defect, and the apply pass replays the plan by
			// position without shifting onto what it never planned.
			b := respelling(func(_, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				if kind == symbol.KindParam && !grown {
					grown = true
					load.Params = append(load.Params,
						&emit.Param{Name: "limit", Type: &emit.TypeRef{Spelling: "int"}})
				}
				return "p" + name, nil
			})
			e := storeOf(t, settleUnit("svc", "svc/a.src", load))
			coretest.AssertCodes(t, settled(t, e, b))
			assert.Equal(t, []string{load.Name, load.Params[0].Name, load.Params[1].Name},
				[]string{"pload", "prowCount", "limit"}, "the planned names settle, and the added one does not")
		})

		guarded := []struct {
			name string
			got  func(*emit.Function) string
			want string
		}{
			{
				name: "rewrites a guard's name to its parameter's settled name",
				got:  func(f *emit.Function) string { return f.Body.Stmts[0].Name },
				want: "prowCount",
			},
			{
				name: "rewrites a reference under a guard",
				got:  func(f *emit.Function) string { return f.Body.Stmts[0].Then[0].Value.Args[0].Name },
				want: "prowCount",
			},
			{
				name: "rewrites a prologue reference through the package's table",
				got:  func(f *emit.Function) string { return f.Body.Prologue.Items()[0].Value.Fn.Name },
				want: "pmax",
			},
			{
				name: "rewrites a declared slot's reference through the package's table",
				got:  func(f *emit.Function) string { return f.Body.Slots[0].Slot.Items()[0].Value.Name },
				want: "pmax",
			},
			{
				name: "rewrites an epilogue reference to its parameter's settled name",
				got:  func(f *emit.Function) string { return f.Body.Epilogue.Items()[0].Value.Name },
				want: "prowCount",
			},
		}
		for _, tt := range guarded {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.got(settleGuarded(t)), tt.want, "the spelling after the settle")
			})
		}

		reassigned := []struct {
			name string
			got  func([]emit.Stmt) string
		}{
			{
				name: "rewrites the target of an assignment that declares nothing",
				got:  func(s []emit.Stmt) string { return s[0].Names[0] },
			},
			{
				name: "rewrites an assignment's value",
				got:  func(s []emit.Stmt) string { return s[0].Value.Args[0].Name },
			},
			{
				name: "rewrites a reassigned parameter in a later statement",
				got:  func(s []emit.Stmt) string { return s[1].Value.Name },
			},
		}
		for _, tt := range reassigned {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				assert.Equal(t, tt.got(settleReassigned(t)), "prowCount", "the parameter's settled name")
			})
		}

		t.Run("resolves a reference after a guard in the package", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{
				Origin: settleOrigin("load", symbol.KindFunction),
				Name:   "load",
				Body: emit.Body{Stmts: []emit.Stmt{
					{
						Kind: emit.StmtGuard, Name: "err",
						Then: []emit.Stmt{{
							Kind: emit.StmtAssign, Names: []string{"max"}, Declare: true,
							Value: emit.Expr{Kind: emit.ExprName, Name: "err"},
						}},
					},
					{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: "max"}},
				}},
			}
			limit := &emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", load, limit))
			coretest.AssertCodes(t, settled(t, e, prefixing("p")))
			assert.Equal(t, load.Body.Stmts[1].Value.Name, "pmax",
				"a name the guard's block declares is out of scope after it")
		})

		t.Run("rewrites a body after a withheld declaration through its own parameters", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{
				Origin: settleOrigin("load", symbol.KindFunction),
				Name:   "load",
				Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
				Body: emit.Body{Stmts: []emit.Stmt{{
					Kind: emit.StmtReturn, Value: emit.Expr{Kind: emit.ExprName, Name: "rowCount"},
				}}},
			}
			e := storeOf(t, settleUnit("svc", "svc/a.src", protectedBox(), load))
			coretest.AssertCodes(t, settled(t, e, protectedRefusing()), plugin.RefusedName)
			assert.Equal(t, load.Body.Stmts[0].Value.Name, "prowCount",
				"the body reads its own parameter, not the withheld neighbour's renames")
		})

		t.Run("positions each naming finding at its unit", func(t *testing.T) {
			t.Parallel()

			b := respelling(func(host, kind symbol.Kind, _ symbol.Visibility, name string) (string, error) {
				switch {
				case kind == symbol.KindStruct:
					return "S" + name, nil
				case host != symbol.KindInvalid:
					return "same", nil
				default:
					return "Same", nil
				}
			})
			box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
			box.Fields.Append(
				&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: "int"}},
				&emit.Field{Name: "y", Type: &emit.TypeRef{Spelling: "box"}},
			)
			alpha := &emit.Constant{Origin: settleOrigin("alpha", symbol.KindConstant), Name: "alpha", Value: "1"}
			beta := &emit.Constant{Origin: settleOrigin("beta", symbol.KindConstant), Name: "beta", Value: "2"}
			boxFn := &emit.Function{Origin: settleOrigin("boxfn", symbol.KindFunction), Name: "box"}
			other := settleUnit("svc", "svc/b.src", boxFn)
			other.Plugin = "second"
			sink := settled(t, storeOf(t, settleUnit("svc", "svc/a.src", box, alpha, beta), other), b)
			coretest.AssertPositioned(t, sink)
		})

		t.Run("settles two overloads of one emitted name without a collision", func(t *testing.T) {
			t.Parallel()

			box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
			box.Methods.Append(
				&emit.Method{Name: "get", Params: []*emit.Param{
					{Name: "id", Type: &emit.TypeRef{Spelling: "int"}},
				}},
				&emit.Method{Name: "get", Params: []*emit.Param{
					{Name: "key", Type: &emit.TypeRef{Spelling: "string"}},
				}},
			)
			e := storeOf(t, settleUnit("svc", "svc/a.src", box))
			coretest.AssertCodes(t, settled(t, e, prefixing("p")))
			methods := box.Methods.Items()
			assert.Equal(t, []string{methods[0].Name, methods[1].Name}, []string{"pget", "pget"},
				"both overloads settle to one name")
		})

		t.Run("rewrites the arguments of a call without a callee", func(t *testing.T) {
			t.Parallel()

			load := &emit.Function{
				Origin: settleOrigin("load", symbol.KindFunction),
				Name:   "load",
				Body: emit.Body{Stmts: []emit.Stmt{{
					Kind: emit.StmtExpr, Value: emit.Expr{
						Kind: emit.ExprCall,
						Args: []emit.Expr{{Kind: emit.ExprName, Name: "max"}},
					},
				}}},
			}
			limit := &emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", load, limit))
			coretest.AssertCodes(t, settled(t, e, prefixing("p")))
			assert.Equal(t, load.Body.Stmts[0].Value.Args[0].Name, "pmax", "the argument follows the table")
		})

		t.Run("reports AmbiguousReference for a body reference to diverging names", func(t *testing.T) {
			t.Parallel()

			caller := &emit.Function{
				Origin: settleOrigin("call", symbol.KindFunction), Name: "call",
				Body: emit.Body{Stmts: []emit.Stmt{
					{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: "row"}},
				}},
			}
			b, units := ambiguousRow(caller)
			coretest.AssertReports(t, settled(t, storeOf(t, units...), b), plugin.AmbiguousReference)
		})

		t.Run("leaves an ambiguous body reference as written", func(t *testing.T) {
			t.Parallel()

			caller := &emit.Function{
				Origin: settleOrigin("call", symbol.KindFunction), Name: "call",
				Body: emit.Body{Stmts: []emit.Stmt{
					{Kind: emit.StmtExpr, Value: emit.Expr{Kind: emit.ExprName, Name: "row"}},
				}},
			}
			b, units := ambiguousRow(caller)
			settled(t, storeOf(t, units...), b)
			assert.Equal(t, caller.Body.Stmts[0].Value.Name, "row", "the reference is left as written")
		})

		t.Run("returns nil for a nil store", func(t *testing.T) {
			t.Parallel()

			assert.NoError(t, plugin.Settle(nil, nil, nil, diag.NewSink()), "nothing to settle is not a fault")
		})

		t.Run("marks a store without a backend settled", func(t *testing.T) {
			t.Parallel()

			e := plugin.NewEmit()
			assert.NoError(t, plugin.Settle(e, nil, nil, diag.NewSink()), "the settle completes")
			assert.True(t, e.Settled(), "the render never waits on the store")
		})

		t.Run("replaces the hook's spelling with an override at directive authority", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, overridden(t, overrideName, meta.AuthorityDirective), overrideName,
				"a directive's name is the settled one")
		})

		t.Run("replaces the hook's spelling with an override at manual authority", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, overridden(t, overrideName, meta.AuthorityManual), overrideName,
				"consumer tooling outranks a directive, so its name applies too")
		})

		t.Run("ignores a name stamp at plugin authority", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, overridden(t, overrideName, meta.AuthorityPlugin), "Row",
				"the hook spells the target's convention")
		})

		t.Run("replaces the hook's spelling with an override that ranks above a plugin's stamp", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, "Row", meta.AuthorityPlugin)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, overrideName, "the directive's name ranks above the stamp of the lowering entry")
		})

		t.Run("ignores an empty override", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, overridden(t, "", meta.AuthorityDirective), "Row", "an empty name names nothing")
		})

		t.Run("ignores an override a manual empty write outranks", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			stampName(t, facts, key, origin, "", meta.AuthorityManual)
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, "Row", "only the claim that ranks first counts")
		})

		t.Run("ignores a dropped override", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			assert.NoError(t, facts.DropKey(key.ID(), meta.Claim{Subject: origin, Authority: meta.AuthorityManual}),
				"the drop applies")
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, "Row", "a drop that ranks first leaves no override")
		})

		t.Run("ignores an override on a declaration derived from the origin", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			mock := &emit.Struct{Origin: origin, Name: "rowMock"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", mock))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, mock.Name, "RowMock", "the override names the origin and not what derives from it")
		})

		t.Run("ignores an override on the zero identity for a declaration without an origin", func(t *testing.T) {
			t.Parallel()

			row := &emit.Struct{Name: "row"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, symbol.Identity{}, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, "Row", "a declaration without an origin renders no source declaration")
		})

		t.Run("ignores an override whose origin name fails to respell", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("_row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			// The hook refuses the origin's name and still returns a
			// spelling, which the settle must not read.
			b := respelling(func(_, _ symbol.Kind, _ symbol.Visibility, n string) (string, error) {
				if strings.HasPrefix(n, "_") {
					return "Row", errors.New("t: a leading underscore spells no name")
				}
				return strings.ToUpper(n[:1]) + n[1:], nil
			})
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, b, facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, "Row", "a refused spelling matches nothing")
		})

		t.Run("ignores an override where the composition registered no name key", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			facts := meta.NewFacts(meta.NewRegistry())
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Name, "Row", "nothing could have written an override")
		})

		t.Run("replaces the hook's spelling of a member with its override", func(t *testing.T) {
			t.Parallel()

			itemOrigin := settleOrigin("item", symbol.KindField)
			row := &emit.Struct{Origin: settleOrigin("row", symbol.KindStruct), Name: "row"}
			row.Fields.Append(&emit.Field{Origin: itemOrigin, Name: "item", Type: &emit.TypeRef{Spelling: "int"}})
			facts, key := nameFacts(t)
			stampName(t, facts, key, itemOrigin, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", row))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, row.Fields.Items()[0].Name, overrideName, "the field takes its override")
		})

		t.Run("applies the override of a nested origin to its declaration under the flat name", func(t *testing.T) {
			t.Parallel()

			state := &emit.Enum{Origin: nestedState, Name: nestedState.FlatName()}
			facts, key := nameFacts(t)
			stampName(t, facts, key, nestedState, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", state))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, state.Name, overrideName, "the file-level declaration takes the override")
		})

		t.Run("ignores the override of a nested origin for a declaration under its own name", func(t *testing.T) {
			t.Parallel()

			state := &emit.Enum{Origin: nestedState, Name: nestedState.Name}
			facts, key := nameFacts(t)
			stampName(t, facts, key, nestedState, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", state))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, state.Name, "State", "the declaration keeps the hook's spelling")
		})

		t.Run("rewrites a reference to an overridden declaration", func(t *testing.T) {
			t.Parallel()

			origin := settleOrigin("row", symbol.KindStruct)
			row := &emit.Struct{Origin: origin, Name: "row"}
			holder := &emit.Variable{
				Origin: settleOrigin("ref", symbol.KindVariable), Name: "ref",
				Type: &emit.TypeRef{Spelling: "row", Target: origin},
			}
			facts, key := nameFacts(t)
			stampName(t, facts, key, origin, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", row, holder))
			assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
			assert.Equal(t, holder.Type.Spelling, overrideName, "the reference follows the override")
		})

		t.Run("withholds a declaration whose name fails to respell despite an override", func(t *testing.T) {
			t.Parallel()

			box := protectedBox()
			itemOrigin := settleOrigin("item", symbol.KindField)
			box.Fields.Items()[0].Origin = itemOrigin
			facts, key := nameFacts(t)
			stampName(t, facts, key, itemOrigin, overrideName, meta.AuthorityDirective)
			e := storeOf(t, settleUnit("svc", "svc/a.src", box))
			sink := diag.NewSink()
			assert.NoError(t, plugin.Settle(e, protectedRefusing(), facts, sink), "the settle completes")
			coretest.AssertCodes(t, sink, plugin.RefusedName)
		})
	})

	t.Run("SettleWith", func(t *testing.T) {
		t.Parallel()

		rowOrigin := settleOrigin("row", symbol.KindStruct)
		kept := keptNames{
			{Package: "svc", Origin: rowOrigin, Kind: symbol.KindStruct, Emitted: "row", Settled: keptRow},
		}
		plain := &hookless{name: settleBackend}

		t.Run("rewrites a bare reference to the settled name that others list", func(t *testing.T) {
			t.Parallel()

			holder := variableOf("holder", &emit.TypeRef{Spelling: "row"})
			e := storeOf(t, settleUnit("svc", "svc/a.src", holder))
			coretest.AssertCodes(t, settledWith(t, e, prefixing("p"), kept))
			assert.Equal(t, holder.Type.Spelling, keptRow, "the reference follows the kept struct")
		})

		t.Run("reports AmbiguousReference when others settle a name apart from the store", func(t *testing.T) {
			t.Parallel()

			holder := variableOf("holder", &emit.TypeRef{Spelling: "row"})
			e := storeOf(t, settleUnit("svc", "svc/a.src", &emit.Struct{Origin: rowOrigin, Name: "row"}, holder))
			coretest.AssertReports(t, settledWith(t, e, prefixing("p"), kept), plugin.AmbiguousReference)
		})

		t.Run("rewrites a resolved reference to the settled name that others list for its origin", func(t *testing.T) {
			t.Parallel()

			holder := variableOf("holder", &emit.TypeRef{Spelling: "row", Target: rowOrigin})
			e := storeOf(t, settleUnit("svc", "svc/a.src", holder))
			coretest.AssertCodes(t, settledWith(t, e, prefixing("p"), kept))
			assert.Equal(t, holder.Type.Spelling, keptRow, "the reference follows the kept declaration")
		})

		t.Run("returns a NameRead for a reference to the declaration of another unit", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t,
				settleUnit("svc", "svc/a.src", &emit.Struct{Origin: rowOrigin, Name: "row"}),
				settleUnit("svc", "svc/b.src", variableOf("holder", &emit.TypeRef{Spelling: "row"})),
			)
			assert.Equal(t, readsOf(t, e, prefixing("p"), nil).Read,
				[]plugin.NameRead{{Unit: 1, Key: plugin.NameKey{Package: "svc", Emitted: "row"}}},
				"the second unit reads the struct of the first unit")
		})

		t.Run("returns a NameRead for a reference that no unit declares", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", variableOf("holder", &emit.TypeRef{Spelling: "int"})))
			assert.Equal(t, readsOf(t, e, prefixing("p"), nil).Read,
				[]plugin.NameRead{{Key: plugin.NameKey{Package: "svc", Emitted: "int"}}},
				"the unit reads a name that nothing declares")
		})

		t.Run("returns no NameRead for a reference to a name of the same unit", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", linkedRow(rowOrigin)))
			assert.Empty(t, readsOf(t, e, prefixing("p"), nil).Read, "the field names the struct of its own unit")
		})

		t.Run("returns a NameRead for a reference to a name of the same unit that others list", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", linkedRow(rowOrigin)))
			assert.Equal(t, readsOf(t, e, prefixing("p"), kept).Read,
				[]plugin.NameRead{{Key: plugin.NameKey{Package: "svc", Emitted: "row"}}},
				"a kept file declares the name as well")
		})

		t.Run("numbers the unit of a read by its place in Units order", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t,
				settleUnit("svc", "svc/b.src", variableOf("late", &emit.TypeRef{Spelling: "long"})),
				settleUnit("svc", "svc/a.src", variableOf("early", &emit.TypeRef{Spelling: "int"})),
			)
			assert.Equal(t, readsOf(t, e, prefixing("p"), nil).Read, []plugin.NameRead{
				{Unit: 0, Key: plugin.NameKey{Package: "svc", Emitted: "int"}},
				{Unit: 1, Key: plugin.NameKey{Package: "svc", Emitted: "long"}},
			}, "the unit of svc/a.src arrived second and sorts first")
		})

		t.Run("sorts the reads of one unit by key", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				variableOf("first", &emit.TypeRef{Spelling: "zeta"}),
				variableOf("second", &emit.TypeRef{Spelling: "alpha"}),
			))
			assert.Equal(t, readsOf(t, e, prefixing("p"), nil).Read, []plugin.NameRead{
				{Key: plugin.NameKey{Package: "svc", Emitted: "alpha"}},
				{Key: plugin.NameKey{Package: "svc", Emitted: "zeta"}},
			}, "alpha sorts before zeta")
		})

		t.Run("returns a FactRead of the name key for the origin of a declaration", func(t *testing.T) {
			t.Parallel()

			facts, _ := nameFacts(t)
			e := storeOf(t, settleUnit("svc", "svc/a.src", &emit.Struct{Origin: rowOrigin, Name: "row"}))
			got, err := plugin.SettleWith(e, capitalizing(), facts, diag.NewSink(), nil)
			assert.NoError(t, err, "the settle completes")
			want := []plugin.FactRead{{Fact: meta.FactRef{Subject: rowOrigin, Key: settleTarget.NameKey()}}}
			assert.Equal(t, got.Facts, want, "the settle reads the override of the struct's origin")
		})

		t.Run("returns no FactRead for a name key that the registry lacks", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", &emit.Struct{Origin: rowOrigin, Name: "row"}))
			got, err := plugin.SettleWith(e, capitalizing(), meta.NewFacts(meta.NewRegistry()), diag.NewSink(), nil)
			assert.NoError(t, err, "the settle completes")
			assert.Empty(t, got.Facts, "no claim can name an unregistered key")
		})

		t.Run("returns a NameRead for a bare reference when the backend has no respell hook", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src", variableOf("holder", &emit.TypeRef{Spelling: "row"})))
			assert.Equal(t, readsOf(t, e, plain, nil).Read,
				[]plugin.NameRead{{Key: plugin.NameKey{Package: "svc", Emitted: "row"}}},
				"the layout resolves the bare reference")
		})

		t.Run("returns no NameRead for a resolved reference when the backend has no respell hook", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				variableOf("holder", &emit.TypeRef{Spelling: "row", Target: rowOrigin})))
			assert.Empty(t, readsOf(t, e, plain, nil).Read, "a reference with a target reads no name")
		})

		t.Run("returns no NameRead for a qualified reference when the backend has no respell hook", func(t *testing.T) {
			t.Parallel()

			e := storeOf(t, settleUnit("svc", "svc/a.src",
				variableOf("holder", &emit.TypeRef{Spelling: "box", Package: "elsewhere"})))
			assert.Empty(t, readsOf(t, e, plain, nil).Read, "a reference into another package reads no name")
		})

		t.Run("returns the zero Settled for a nil store", func(t *testing.T) {
			t.Parallel()

			got, err := plugin.SettleWith(nil, nil, nil, diag.NewSink(), nil)
			assert.NoError(t, err, "nothing to settle is not a fault")
			assert.Equal(t, got, plugin.Settled{}, "the settle reads nothing")
		})
	})
}

// A settle allocates nothing for a backend without hooks, and the
// rebuilt index with the plan of the names for a respell hook, in the
// ordinary run, which runs no benchmark. SettleWith adds its read log.
// Each counted settle takes a store of its own, built outside the
// count, and the count keeps the first error of its settles, which
// cmp.Or returns without allocating. The check runs alone, because the
// count includes every goroutine's allocations.
func TestSettleAllocs(t *testing.T) {
	for _, tt := range settleCases() {
		sink := diag.NewSink()
		var err error
		assert.MaxAllocsWithSetup(t, func() *plugin.Emit { return settleStore(t) }, func(e *plugin.Emit) {
			err = cmp.Or(err, plugin.Settle(e, tt.backend, nil, sink))
		}, tt.allocs, "Settle through "+tt.name+" allocates what it rebuilds")
		assert.MaxAllocsWithSetup(t, func() *plugin.Emit { return settleStore(t) }, func(e *plugin.Emit) {
			_, werr := plugin.SettleWith(e, tt.backend, nil, sink, nil)
			err = cmp.Or(err, werr)
		}, tt.withAllocs, "SettleWith through "+tt.name+" adds its read log")
		assert.NoError(t, err, "every settle through "+tt.name+" completes")
		assert.Empty(t, messages(sink), "the settle reports nothing")
	}
}

// BenchmarkSettle measures one plan's settle over a store of a hundred
// units, each settle on a store built outside the measurement.
func BenchmarkSettle(b *testing.B) {
	b.Run("Settle", func(b *testing.B) {
		for _, tt := range settleCases() {
			b.Run(tt.name, func(b *testing.B) {
				var (
					e    *plugin.Emit
					sink *diag.Sink
				)
				fresh := func() { e, sink = settleStore(b), diag.NewSink() }
				c := bench.Start(b).MaxAllocs(tt.allocs)
				defer c.End()
				var err error
				for c.Loop() {
					c.Excluding(fresh)
					err = plugin.Settle(e, tt.backend, nil, sink)
				}
				assert.NoError(b, err, "the store settles")
				assert.True(b, e.Settled(), "the store is marked settled")
			})
		}
	})

	b.Run("SettleWith", func(b *testing.B) {
		for _, tt := range settleCases() {
			b.Run(tt.name, func(b *testing.B) {
				var (
					e    *plugin.Emit
					sink *diag.Sink
				)
				fresh := func() { e, sink = settleStore(b), diag.NewSink() }
				c := bench.Start(b).MaxAllocs(tt.withAllocs)
				defer c.End()
				var err error
				for c.Loop() {
					c.Excluding(fresh)
					_, err = plugin.SettleWith(e, tt.backend, nil, sink, nil)
				}
				assert.NoError(b, err, "the store settles")
				assert.True(b, e.Settled(), "the store is marked settled")
			})
		}
	})
}

// settleOrigin returns a distinct origin identity per name.
func settleOrigin(name string, k symbol.Kind) symbol.Identity {
	return symbol.Identity{Lang: "fixture", Package: "svc", Name: name, Kind: k}
}

// settleUnit returns one unit under the given package path.
func settleUnit(pkg, key string, decls ...symbol.Symbol) plugin.Unit {
	return plugin.Unit{
		Plugin: "gen",
		Per:    plugin.PerSource,
		Word:   "gen",
		Key:    key,
		Pkg:    symbol.Identity{Package: pkg, Name: "svc", Kind: symbol.KindPackage},
		Decls:  decls,
	}
}

// storeOf returns a store with the units, refusing nothing.
func storeOf(t *testing.T, units ...plugin.Unit) *plugin.Emit {
	t.Helper()

	e := plugin.NewEmit()
	assert.Total(t, e.Add, units, "the unit arrives")
	return e
}

// settled settles a store under a backend without facts and
// returns the sink the settle reported to.
func settled(t *testing.T, e *plugin.Emit, b plugin.Backend) *diag.Sink {
	t.Helper()

	sink := diag.NewSink()
	assert.NoError(t, plugin.Settle(e, b, nil, sink), "the settle completes")
	return sink
}

// settledWith settles a store under a backend without facts against
// the names of kept files, and returns the sink the settle reported to.
func settledWith(t *testing.T, e *plugin.Emit, b plugin.Backend, kept plugin.Names) *diag.Sink {
	t.Helper()

	sink := diag.NewSink()
	_, err := plugin.SettleWith(e, b, nil, sink, kept)
	assert.NoError(t, err, "the settle completes")
	return sink
}

// readsOf settles a store under a backend without facts against the
// names of kept files, and returns what the settle read.
func readsOf(t *testing.T, e *plugin.Emit, b plugin.Backend, kept plugin.Names) plugin.Settled {
	t.Helper()

	got, err := plugin.SettleWith(e, b, nil, diag.NewSink(), kept)
	assert.NoError(t, err, "the settle completes")
	return got
}

// linkedRow returns a struct named row of an origin, whose field next
// names the struct.
func linkedRow(origin symbol.Identity) *emit.Struct {
	row := &emit.Struct{Origin: origin, Name: "row"}
	row.Fields.Append(&emit.Field{Name: "next", Type: &emit.TypeRef{Spelling: "row"}})
	return row
}

// messages returns a sink's findings as their message text, in
// report order. A case about the order findings arrive in compares
// against this, because the codes alone cannot tell one collision
// from another.
func messages(s *diag.Sink) []string {
	var out []string
	for d := range s.All() {
		out = append(out, d.Msg)
	}
	return out
}

// nameFacts returns a fact store whose registry contains the test
// target's name key, and the key's handle.
func nameFacts(t *testing.T) (*meta.Facts, meta.Key[string]) {
	t.Helper()

	r := meta.NewRegistry()
	name := settleTarget.NameKey()
	assert.NoError(t, r.ClaimNamespace(name.Namespace()), "the target's namespace claims")
	key, err := meta.Register[string](r, meta.KeySpec{Name: name, Doc: nameDoc})
	assert.NoError(t, err, "the name key registers")
	return meta.NewFacts(r), key
}

// stampName writes a name on an origin at one authority.
func stampName(
	t *testing.T, facts *meta.Facts, key meta.Key[string],
	origin symbol.Identity, name string, a meta.Authority,
) {
	t.Helper()

	err := meta.Stamp(facts, key, name, meta.Claim{Subject: origin, Authority: a, Plugin: stampPlugin})
	assert.NoError(t, err, "the name stamps")
}

// overridden settles one struct named row under the capitalizing
// hook with a name written on its origin at one authority, and
// returns the settled spelling.
func overridden(t *testing.T, name string, a meta.Authority) string {
	t.Helper()

	origin := settleOrigin("row", symbol.KindStruct)
	row := &emit.Struct{Origin: origin, Name: "row"}
	facts, key := nameFacts(t)
	stampName(t, facts, key, origin, name, a)
	e := storeOf(t, settleUnit("svc", "svc/a.src", row))
	assert.NoError(t, plugin.Settle(e, capitalizing(), facts, diag.NewSink()), "the settle completes")
	return row.Name
}

// structural returns a structural reference of a form, spelled as
// written, over elements.
func structural(form symbol.TypeForm, spelling string, elems ...*emit.TypeRef) *emit.TypeRef {
	return &emit.TypeRef{Form: form, Spelling: spelling, Elems: elems}
}

// variableOf returns a variable of svc named name, of the type t.
func variableOf(name string, t *emit.TypeRef) *emit.Variable {
	return &emit.Variable{Origin: settleOrigin(name, symbol.KindVariable), Name: name, Type: t}
}

// settleGuarded builds a callable whose body reads its parameter
// under a guard and a package constant from its prologue, a declared
// slot and its epilogue, settles it under a prefixing hook, and
// returns the callable.
func settleGuarded(t *testing.T) *emit.Function {
	t.Helper()

	body := emit.Body{Stmts: []emit.Stmt{{
		Kind: emit.StmtGuard, Name: "rowCount",
		Then: []emit.Stmt{{Kind: emit.StmtExpr, Value: emit.Expr{
			Kind: emit.ExprCall,
			Args: []emit.Expr{{Kind: emit.ExprName, Name: "rowCount"}},
		}}},
	}}}
	body.Prologue.Append(emit.Stmt{
		Kind: emit.StmtExpr,
		Value: emit.Expr{
			Kind: emit.ExprCall,
			Fn:   &emit.Expr{Kind: emit.ExprName, Name: "max"},
		},
	})
	body.Declare("checks").Append(emit.Stmt{
		Kind:  emit.StmtExpr,
		Value: emit.Expr{Kind: emit.ExprName, Name: "max"},
	})
	body.Epilogue.Append(emit.Stmt{
		Kind:  emit.StmtExpr,
		Value: emit.Expr{Kind: emit.ExprName, Name: "rowCount"},
	})
	load := &emit.Function{
		Origin: settleOrigin("load", symbol.KindFunction),
		Name:   "load",
		Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
		Body:   body,
	}
	limit := &emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}
	e := storeOf(t, settleUnit("svc", "svc/a.src", load, limit))
	coretest.AssertCodes(t, settled(t, e, prefixing("p")))
	return load
}

// settleReassigned builds a callable that reassigns its parameter
// and returns it, settles it under a prefixing hook, and returns its
// body's statements.
func settleReassigned(t *testing.T) []emit.Stmt {
	t.Helper()

	load := &emit.Function{
		Origin: settleOrigin("load", symbol.KindFunction),
		Name:   "load",
		Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
		Body: emit.Body{Stmts: []emit.Stmt{
			{
				Kind: emit.StmtAssign, Names: []string{"rowCount"},
				Value: emit.Expr{
					Kind: emit.ExprCall,
					Fn:   &emit.Expr{Kind: emit.ExprName, Name: "clamp"},
					Args: []emit.Expr{{Kind: emit.ExprName, Name: "rowCount"}},
				},
			},
			{Kind: emit.StmtReturn, Value: emit.Expr{Kind: emit.ExprName, Name: "rowCount"}},
		}},
	}
	e := storeOf(t, settleUnit("svc", "svc/a.src", load))
	coretest.AssertCodes(t, settled(t, e, prefixing("p")))
	return load.Body.Stmts
}

// protectedBox returns a struct whose one field is protected.
func protectedBox() *emit.Struct {
	box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
	box.Fields.Append(&emit.Field{
		Name: "item", Visibility: symbol.VisibilityProtected,
		Type: &emit.TypeRef{Spelling: "int"},
	})
	return box
}

// settleCases returns the backends the allocation check and the
// benchmark settle through, with the allocations of one Settle and of
// one SettleWith of [settleStore].
func settleCases() []struct {
	name       string
	backend    plugin.Backend
	allocs     uint64
	withAllocs uint64
} {
	return []struct {
		name       string
		backend    plugin.Backend
		allocs     uint64
		withAllocs uint64
	}{
		{
			name: "a backend without hooks", backend: &hookless{name: "printer"},
			allocs: 0, withAllocs: readLogAllocs,
		},
		{
			name: "a respell hook that keeps every name", backend: keeping(),
			allocs: keepingSettleAllocs, withAllocs: keepingSettleAllocs + readLogAllocs,
		},
	}
}

// keeping returns a backend whose respell hook keeps every name, so a
// settle measures the kernel's work and none of the hook's.
func keeping() *respellingOnly {
	return respelling(func(_, _ symbol.Kind, _ symbol.Visibility, n string) (string, error) {
		return n, nil
	})
}

// settleStore returns a store of settleUnits units in one package, each
// of two structs of three methods with names of their own.
func settleStore(tb assert.TB) *plugin.Emit {
	tb.Helper()

	e := plugin.NewEmit()
	for i := range settleUnits {
		decls := make([]symbol.Symbol, 0, 2)
		for j := range 2 {
			name := "Row" + strconv.Itoa(i) + "_" + strconv.Itoa(j)
			s := &emit.Struct{Origin: structID(name), Name: name}
			for m := range 3 {
				s.Methods.Append(&emit.Method{Origin: structID(name), Name: "Method" + strconv.Itoa(m)})
			}
			decls = append(decls, s)
		}
		u := unit("stubgen", "unit"+strconv.Itoa(i)+".go")
		u.Decls = decls
		assert.NoError(tb, e.Add(u), "the unit arrives")
	}
	return e
}

// splitAlias lowers an alias into a struct and a constant, both of
// its origin, and keeps any other declaration.
func splitAlias(s symbol.Symbol) ([]symbol.Symbol, error) {
	if a, is := s.(*emit.Alias); is {
		return []symbol.Symbol{
			&emit.Struct{Origin: a.Origin, Name: a.Name},
			&emit.Constant{Origin: a.Origin, Name: a.Name + "Limit", Value: "1"},
		}, nil
	}
	return []symbol.Symbol{s}, nil
}

// clearAsync rewrites a method in place and returns nil, which
// keeps the declaration.
func clearAsync(s symbol.Symbol) ([]symbol.Symbol, error) {
	if m, is := s.(*emit.Method); is {
		m.Async = false
	}
	return nil, nil
}

// refuseSum refuses a sum and keeps any other declaration.
func refuseSum(s symbol.Symbol) ([]symbol.Symbol, error) {
	if _, is := s.(*emit.Sum); is {
		return nil, errors.New("go: no idiom spells a sum")
	}
	return []symbol.Symbol{s}, nil
}

// same settles every name to one spelling.
func same(_, _ symbol.Kind, _ symbol.Visibility, _ string) (string, error) {
	return "Same", nil
}
