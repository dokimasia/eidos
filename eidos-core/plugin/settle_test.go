// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"errors"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/internal/coretest"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// settleBackend is the hookless backend: a name and a target, and
// nothing the settle would run.
type settleBackend struct{ name plugin.ID }

func (b *settleBackend) Name() plugin.ID     { return b.name }
func (*settleBackend) Target() plugin.Target { return "t" }

// loweringOnly declares the construct seam alone.
type loweringOnly struct {
	settleBackend
	fn plugin.Lower
}

func (b *loweringOnly) Lower(s symbol.Symbol) ([]symbol.Symbol, error) { return b.fn(s) }

// respellingOnly declares the name seam alone.
type respellingOnly struct {
	settleBackend
	fn plugin.Respell
}

func (b *respellingOnly) Respell(
	host, kind symbol.Kind, v symbol.Visibility, name string,
) (string, error) {
	return b.fn(host, kind, v, name)
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

// storeOf returns a store holding the units, refusing nothing.
func storeOf(t *testing.T, units ...plugin.Unit) *plugin.Emit {
	t.Helper()

	e := plugin.NewEmit()
	for _, u := range units {
		assert.NoError(t, e.Add(u), "the unit arrives")
	}
	return e
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

// prefixing returns a backend whose respell hook prefixes every
// name it is handed, so a case reads the settled spelling straight
// off the emitted one.
func prefixing(name plugin.ID, prefix string) *respellingOnly {
	b := &respellingOnly{}
	b.name = name
	b.fn = func(host, kind symbol.Kind, v symbol.Visibility, n string) (string, error) {
		return prefix + n, nil
	}
	return b
}

// The settle is the stage between a plan's schedule and its
// render: constructs lower, names respell, references follow, and
// the store marks itself settled exactly once.
func TestSettle(t *testing.T) {
	t.Parallel()

	t.Run("marks the store settled, and a settled store settles to itself", func(t *testing.T) {
		t.Parallel()

		calls := 0
		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			calls++
			return name, nil
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src",
			&emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")
		assert.True(t, e.Settled(), "the store marks settled")

		before := calls
		assert.NoError(t, plugin.Settle(e, b, sink), "a second settle completes")
		assert.Equal(t, calls, before, "and changes nothing, so the hooks never rerun")
	})

	t.Run("a hookless backend settles the store untouched", func(t *testing.T) {
		t.Parallel()

		e := storeOf(t, settleUnit("svc", "svc/a.src",
			&emit.Variable{Origin: settleOrigin("count", symbol.KindVariable), Name: "count"}))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, &settleBackend{name: "golang"}, sink),
			"the settle completes")
		assert.True(t, e.Settled(), "the store marks settled")
		for u := range e.Units() {
			v, held := u.Decls[0].(*emit.Variable)
			assert.True(t, held && v.Name == "count", "the declaration stands as emitted")
		}
	})

	t.Run("lowers a construct into the target's shapes and reindexes", func(t *testing.T) {
		t.Parallel()

		origin := settleOrigin("state", symbol.KindEnum)
		b := &loweringOnly{}
		b.name = "golang"
		b.fn = func(s symbol.Symbol) ([]symbol.Symbol, error) {
			if a, held := s.(*emit.Alias); held {
				return []symbol.Symbol{
					&emit.Struct{Origin: a.Origin, Name: a.Name},
					&emit.Constant{Origin: a.Origin, Name: a.Name + "Limit", Value: "1"},
				}, nil
			}
			return []symbol.Symbol{s}, nil
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src",
			&emit.Alias{Origin: origin, Name: "state"}))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")
		coretest.AssertCodes(t, sink)

		for u := range e.Units() {
			assert.Equal(t, len(u.Decls), 2, "one declaration lowers into two")
		}
		aliases, structs := 0, 0
		for range e.ByKind(symbol.KindAlias) {
			aliases++
		}
		for range e.ByKind(symbol.KindStruct) {
			structs++
		}
		assert.Equal(t, aliases, 0, "the index forgets the lowered kind")
		assert.Equal(t, structs, 1, "and holds the target's shape")
	})

	t.Run("a nil answer keeps the declaration as it stands", func(t *testing.T) {
		t.Parallel()

		b := &loweringOnly{}
		b.name = "golang"
		b.fn = func(s symbol.Symbol) ([]symbol.Symbol, error) {
			if m, held := s.(*emit.Method); held {
				m.Async = false // an in-place rewrite returns nil the same way
			}
			return nil, nil
		}
		origin := settleOrigin("track", symbol.KindMethod)
		kept := &emit.Method{Origin: origin, Name: "track", Async: true}
		e := storeOf(t, settleUnit("svc", "svc/a.src", kept))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")
		coretest.AssertCodes(t, sink)

		for u := range e.Units() {
			assert.Length(t, u.Decls, 1, "the declaration stays")
			assert.True(t, u.Decls[0] == symbol.Symbol(kept), "itself")
		}
		assert.False(t, kept.Async, "carrying the in-place rewrite")
	})

	t.Run("withholds a refused construct under a positioned finding", func(t *testing.T) {
		t.Parallel()

		b := &loweringOnly{}
		b.name = "golang"
		b.fn = func(s symbol.Symbol) ([]symbol.Symbol, error) {
			if _, held := s.(*emit.Sum); held {
				return nil, errors.New("go: no idiom spells a sum")
			}
			return []symbol.Symbol{s}, nil
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src",
			&emit.Sum{Origin: settleOrigin("shape", symbol.KindSum), Name: "shape"},
			&emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"},
		))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.RefusedConstruct)
		for u := range e.Units() {
			assert.Equal(t, len(u.Decls), 1, "the refused declaration is withheld")
			assert.Equal(t, u.Decls[0].Kind(), symbol.KindConstant,
				"and its sibling survives")
		}
	})

	t.Run("a lowering dropping the origin is a defect", func(t *testing.T) {
		t.Parallel()

		b := &loweringOnly{}
		b.name = "golang"
		b.fn = func(s symbol.Symbol) ([]symbol.Symbol, error) {
			return []symbol.Symbol{&emit.Struct{Name: "made"}}, nil
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src",
			&emit.Alias{Origin: settleOrigin("state", symbol.KindAlias), Name: "state"}))
		err := plugin.Settle(e, b, diag.NewSink())
		assert.HasError(t, err, "every output carries the input's origin")
	})

	t.Run("respells names and the references that follow them", func(t *testing.T) {
		t.Parallel()

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

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			switch {
			case host == symbol.KindInvalid:
				return "T" + name, nil
			case kind == symbol.KindParam:
				return "p" + name, nil
			default:
				return "m" + name, nil
			}
		}
		e := storeOf(t,
			settleUnit("svc", "svc/a.src", box, match),
			settleUnit("svc", "svc/b.src", composite),
		)
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")
		coretest.AssertCodes(t, sink)

		assert.Equal(t, box.Name, "Tbox", "a file-level name settles")
		assert.Equal(t, box.Fields.Items()[0].Name, "mitem", "a member settles under its host")
		assert.Equal(t, fetch.Name, "mfetch", "and so does a member method")
		assert.Equal(t, fetch.Params[0].Name, "prowCount", "a parameter settles under the callable")
		assert.Equal(t, match.Name, "Tmatch", "every file-level name settles")
		assert.Equal(t, match.Target.Spelling, "Tbox",
			"a resolved reference follows its origin")
		assert.Equal(t, box.Fields.Items()[0].Type.Spelling, "Tbox",
			"a bare reference follows the package's table")
		assert.Equal(t, composite.Type.Spelling, "[]box",
			"a composite spelling stands as written")

		stmts := fetch.Body.Stmts
		assert.Equal(t, stmts[1].Value.Args[0].Name, "state",
			"a declared local stands")
		assert.Equal(t, stmts[1].Value.Args[1].Name, "prowCount",
			"a body reference follows its parameter")
		assert.Equal(t, stmts[1].Value.Fn.Name, "commit",
			"an undeclared callable stands")
	})

	t.Run("reverts a package collision under one finding", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			return "Same", nil
		}
		alpha := &emit.Constant{Origin: settleOrigin("alpha", symbol.KindConstant), Name: "alpha", Value: "1"}
		beta := &emit.Constant{Origin: settleOrigin("beta", symbol.KindConstant), Name: "beta", Value: "2"}
		e := storeOf(t, settleUnit("svc", "svc/a.src", alpha, beta))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.CollidingNames)
		assert.Equal(t, alpha.Name, "alpha", "the first keeps its emitted name")
		assert.Equal(t, beta.Name, "beta", "and so does the second")
	})

	t.Run("reports one collision group per scope, in one order", func(t *testing.T) {
		t.Parallel()

		settles := map[string]string{
			"alpha": "Same", "beta": "Same",
			"gamma": "Other", "delta": "Other",
			"epsilon": "Same", "zeta": "Same",
		}
		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			return settles[name], nil
		}
		constant := func(name string) *emit.Constant {
			return &emit.Constant{
				Origin: settleOrigin(name, symbol.KindConstant), Name: name, Value: "1",
			}
		}
		e := storeOf(t,
			settleUnit("svc", "svc/a.src",
				constant("gamma"), constant("delta"), constant("epsilon"), constant("zeta")),
			settleUnit("api", "api/a.src", constant("alpha"), constant("beta")),
		)
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink,
			plugin.CollidingNames, plugin.CollidingNames, plugin.CollidingNames)
		got := messages(sink)
		assert.Length(t, got, 3, "one finding per group")
		assert.Contains(t, got[0], "in api", "the scopes order the findings")
		assert.Contains(t, got[1], `to "Other"`,
			"and inside one scope the settled spelling does")
		assert.Contains(t, got[2], `to "Same"`,
			"so two runs over one store report in one order")
	})

	t.Run("two receivers declare one method name without meeting", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			return strings.ToUpper(name[:1]) + name[1:], nil
		}
		first := &emit.Method{
			Origin:   settleOrigin("row.track", symbol.KindMethod),
			Name:     "track",
			Receives: &emit.TypeRef{Spelling: "row"},
		}
		second := &emit.Method{
			Origin:   settleOrigin("box.track", symbol.KindMethod),
			Name:     "track",
			Receives: &emit.TypeRef{Spelling: "box"},
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", first, second))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")
		coretest.AssertCodes(t, sink)
		assert.Equal(t, first.Name, "Track", "the first settles")
		assert.Equal(t, second.Name, "Track", "and so does the second")

		twice := &emit.Method{
			Origin:   settleOrigin("row.track2", symbol.KindMethod),
			Name:     "Track",
			Receives: &emit.TypeRef{Spelling: "row"},
		}
		again := &emit.Method{
			Origin:   settleOrigin("row.track3", symbol.KindMethod),
			Name:     "track",
			Receives: &emit.TypeRef{Spelling: "row"},
		}
		e2 := storeOf(t, settleUnit("svc", "svc/a.src", twice, again))
		sink2 := diag.NewSink()
		assert.NoError(t, plugin.Settle(e2, b, sink2), "the settle completes")
		coretest.AssertCodes(t, sink2, plugin.CollidingNames)
		assert.Equal(t, twice.Name, "Track", "one receiver's two spellings collide")
		assert.Equal(t, again.Name, "track", "so each keeps its emitted name")
	})

	t.Run("reverts a member collision within one host", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if host == symbol.KindInvalid {
				return name, nil
			}
			return "same", nil
		}
		box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
		box.Fields.Append(
			&emit.Field{Name: "x", Type: &emit.TypeRef{Spelling: "int"}},
			&emit.Field{Name: "y", Type: &emit.TypeRef{Spelling: "int"}},
		)
		e := storeOf(t, settleUnit("svc", "svc/a.src", box))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.CollidingNames)
		assert.Equal(t, box.Fields.Items()[0].Name, "x", "the members keep their emitted names")
		assert.Equal(t, box.Fields.Items()[1].Name, "y", "both of them")
	})

	t.Run("an ambiguous bare reference stands under a finding", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if kind == symbol.KindStruct {
				return "S" + name, nil
			}
			return "F" + name, nil
		}
		rowStruct := &emit.Struct{Origin: settleOrigin("row", symbol.KindStruct), Name: "row"}
		rowFn := &emit.Function{Origin: settleOrigin("rowfn", symbol.KindFunction), Name: "row"}
		holder := &emit.Variable{
			Origin: settleOrigin("hold", symbol.KindVariable),
			Name:   "hold",
			Type:   &emit.TypeRef{Spelling: "row"},
		}
		other := settleUnit("svc", "svc/b.src", rowFn)
		other.Plugin = "second"
		e := storeOf(t, settleUnit("svc", "svc/a.src", rowStruct, holder), other)
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertReports(t, sink, plugin.AmbiguousReference)
		assert.Equal(t, holder.Type.Spelling, "row", "and the reference stands as written")
	})

	t.Run("withholds a declaration whose name refuses", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if v == symbol.VisibilityProtected {
				return "", errors.New("go: no case carries a protected scope")
			}
			return name, nil
		}
		box := &emit.Struct{Origin: settleOrigin("box", symbol.KindStruct), Name: "box"}
		box.Fields.Append(&emit.Field{
			Name: "item", Visibility: symbol.VisibilityProtected,
			Type: &emit.TypeRef{Spelling: "int"},
		})
		keep := &emit.Constant{Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1"}
		e := storeOf(t, settleUnit("svc", "svc/a.src", box, keep))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.RefusedName)
		for u := range e.Units() {
			assert.Equal(t, len(u.Decls), 1, "the refused declaration is withheld whole")
			assert.Equal(t, u.Decls[0].Kind(), symbol.KindConstant, "and its sibling survives")
		}
	})

	t.Run("pins a verbatim body's parameters under a finding", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "rust"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if kind == symbol.KindParam {
				return strings.ToLower(name) + "_p", nil
			}
			return name, nil
		}
		load := &emit.Function{
			Origin: settleOrigin("load", symbol.KindFunction),
			Name:   "load",
			Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
			Body:   emit.Body{Verbatim: "\tuse(rowCount)\n"},
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", load))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.VerbatimParams)
		assert.Equal(t, load.Params[0].Name, "rowCount",
			"the parameter keeps the spelling the text reads")
		assert.Equal(t, load.Body.Verbatim, "\tuse(rowCount)\n",
			"and the text stands untouched")
	})

	t.Run("a verbatim pin on a method names the method", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "rust"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if kind == symbol.KindParam {
				return strings.ToLower(name) + "_p", nil
			}
			return strings.ToLower(name), nil
		}
		scan := &emit.Method{
			Origin:   settleOrigin("scan", symbol.KindMethod),
			Name:     "Scan",
			Receives: &emit.TypeRef{Spelling: "row"},
			Params:   []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
			Body:     emit.Body{Verbatim: "\tuse(rowCount)\n"},
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", scan))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink, plugin.VerbatimParams)
		assert.Contains(t, messages(sink)[0], "scan",
			"the finding names the callable whose signature the text pinned")
		assert.Equal(t, scan.Params[0].Name, "rowCount",
			"and the parameter keeps the spelling the text reads")
	})

	t.Run("a bare parameter at file level is pinned by nothing", func(t *testing.T) {
		t.Parallel()

		loose := &emit.Param{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}
		e := storeOf(t, settleUnit("svc", "svc/a.src", loose))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, prefixing("golang", "p"), sink),
			"the settle completes")

		coretest.AssertCodes(t, sink)
		assert.Equal(t, loose.Name, "prowCount",
			"a parameter sitting outside a callable reads no verbatim body, "+
				"so it settles like any other name")
	})

	t.Run("a name the plan never recorded stands as emitted", func(t *testing.T) {
		t.Parallel()

		load := &emit.Function{
			Origin: settleOrigin("load", symbol.KindFunction),
			Name:   "load",
			Params: []*emit.Param{{Name: "rowCount", Type: &emit.TypeRef{Spelling: "int"}}},
		}
		grown := false
		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			// A hook reaching into the declaration it is handed is
			// a backend defect; the apply pass replays the plan
			// positionally and must not shift onto what it never
			// planned.
			if kind == symbol.KindParam && !grown {
				grown = true
				load.Params = append(load.Params,
					&emit.Param{Name: "limit", Type: &emit.TypeRef{Spelling: "int"}})
			}
			return "p" + name, nil
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", load))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertCodes(t, sink)
		assert.Equal(t, load.Name, "pload", "the planned name settles")
		assert.Equal(t, load.Params[0].Name, "prowCount", "and so does the planned parameter")
		assert.Equal(t, load.Params[1].Name, "limit",
			"while the one the hook grew mid-plan stands as emitted")
	})

	t.Run("follows a name through the body's slots and guards", func(t *testing.T) {
		t.Parallel()

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
		limit := &emit.Constant{
			Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1",
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", load, limit))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, prefixing("golang", "p"), sink),
			"the settle completes")

		coretest.AssertCodes(t, sink)
		assert.Equal(t, load.Body.Stmts[0].Name, "prowCount",
			"a guarded name follows the parameter it names")
		assert.Equal(t, load.Body.Stmts[0].Then[0].Value.Args[0].Name, "prowCount",
			"and so does a reference nested under the guard")
		assert.Equal(t, load.Body.Prologue.Items()[0].Value.Fn.Name, "pmax",
			"a prologue reference follows the package's table")
		assert.Equal(t, load.Body.Slots[0].Slot.Items()[0].Value.Name, "pmax",
			"a declared slot's statements follow it too")
		assert.Equal(t, load.Body.Epilogue.Items()[0].Value.Name, "prowCount",
			"and so does the epilogue's")
	})

	t.Run("a call without a callee stands", func(t *testing.T) {
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
		limit := &emit.Constant{
			Origin: settleOrigin("max", symbol.KindConstant), Name: "max", Value: "1",
		}
		e := storeOf(t, settleUnit("svc", "svc/a.src", load, limit))
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, prefixing("golang", "p"), sink),
			"the settle completes")

		coretest.AssertCodes(t, sink)
		assert.Equal(t, load.Body.Stmts[0].Value.Args[0].Name, "pmax",
			"a call naming no callee still has its arguments followed")
	})

	t.Run("an ambiguous body reference stands under a finding", func(t *testing.T) {
		t.Parallel()

		b := &respellingOnly{}
		b.name = "golang"
		b.fn = func(host, kind symbol.Kind, v symbol.Visibility, name string) (string, error) {
			if kind == symbol.KindStruct {
				return "S" + name, nil
			}
			return "F" + name, nil
		}
		rowStruct := &emit.Struct{Origin: settleOrigin("row", symbol.KindStruct), Name: "row"}
		rowFn := &emit.Function{Origin: settleOrigin("rowfn", symbol.KindFunction), Name: "row"}
		caller := &emit.Function{
			Origin: settleOrigin("call", symbol.KindFunction),
			Name:   "call",
			Body: emit.Body{Stmts: []emit.Stmt{{
				Kind:  emit.StmtExpr,
				Value: emit.Expr{Kind: emit.ExprName, Name: "row"},
			}}},
		}
		other := settleUnit("svc", "svc/b.src", rowFn)
		other.Plugin = "second"
		e := storeOf(t, settleUnit("svc", "svc/a.src", rowStruct, caller), other)
		sink := diag.NewSink()
		assert.NoError(t, plugin.Settle(e, b, sink), "the settle completes")

		coretest.AssertReports(t, sink, plugin.AmbiguousReference)
		assert.Equal(t, caller.Body.Stmts[0].Value.Name, "row",
			"a body reference matching two settled spellings stands as written")
	})

	t.Run("a nil store or backend settles to nothing", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, plugin.Settle(nil, nil, diag.NewSink()),
			"nothing to settle is not a fault")
		e := plugin.NewEmit()
		assert.NoError(t, plugin.Settle(e, nil, diag.NewSink()),
			"a store without a backend settles")
		assert.True(t, e.Settled(), "and marks itself, so the render never waits on it")
	})
}
