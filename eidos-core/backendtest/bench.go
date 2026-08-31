// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package backendtest

import (
	"io/fs"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The canonical benchmark scale. Every backend measures over the
// same corpus shape, so two backends' numbers mean one thing and
// a regression names a code change rather than a fixture change.
const (
	// BenchPackages is how many packages the scaled fixture holds.
	BenchPackages = 1_000
	// BenchFiles is how many source units each package holds.
	BenchFiles = 10
	// BenchDecls is how many file-level declarations each unit
	// holds; member declarations nest inside them uncounted.
	BenchDecls = 20
)

// Budget is what a satellite pins its benchmark to. An allocation
// count does not move with machine load, so exceeding the ceiling
// names a real code change; latency bounds stay beside the pinned
// baseline gates, where the machine is fixed.
type Budget struct {
	// MaxAllocs bounds allocations per rendered corpus. Zero is
	// refused: a benchmark without a ceiling records numbers
	// nobody reads.
	MaxAllocs uint64
}

// BenchRender measures one backend over its setup's fixture and
// holds it to the budget: the fixture builds once outside the
// loop, every iteration renders it whole over a fresh sink, and
// the contract checks the ceiling when the loop ends. An
// iteration reporting any finding fails the benchmark, because a
// number over a partial render measures the wrong thing.
//
// A satellite's setup returns [ScaledFixture] filtered to its
// rendered coverage, so the corpus shape stays the suite's and
// the ceiling stays the satellite's.
func BenchRender(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	if budget.MaxAllocs == 0 {
		b.Fatal("the budget states no ceiling")
	}
	r, f := setup(b)
	if f == nil || f.Emit == nil {
		b.Fatal("the setup carries no fixture")
	}
	if bk, held := r.(plugin.Backend); held {
		settleSink := diag.NewSink()
		if err := plugin.Settle(f.Emit, bk, settleSink); err != nil {
			b.Fatalf("the settle completes: %v", err)
		}
		if settleSink.Failed() {
			b.Fatal("the corpus settles clean, because a ceiling over a " +
				"partial settle measures the wrong thing")
		}
	}

	c := bench.Start(b).MaxAllocs(budget.MaxAllocs)
	defer c.End()
	for c.Loop() {
		sink := diag.NewSink()
		files, err := r.Render(f.context(sink))
		if err != nil {
			b.Fatalf("the render aborted: %v", err)
		}
		if len(files) == 0 || sink.Failed() {
			b.Fatal("the corpus renders whole and clean")
		}
	}
}

// BenchSettle measures the settle over the setup's corpus: every
// iteration builds a fresh fixture and settles it whole, because
// a settled store settles to itself and a second pass would
// measure the short circuit. The corpus build rides inside the
// number and is identical across backends, so the ceiling pins
// build plus settle and a settle regression still moves it; the
// render benchmarks settle before their loop, so the two numbers
// split the pipeline between them.
func BenchSettle(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	if budget.MaxAllocs == 0 {
		b.Fatal("the budget states no ceiling")
	}
	c := bench.Start(b).MaxAllocs(budget.MaxAllocs)
	defer c.End()
	for c.Loop() {
		r, f := setup(b)
		if f == nil || f.Emit == nil {
			b.Fatal("the setup carries no fixture")
		}
		bk, held := r.(plugin.Backend)
		if !held {
			b.Fatal("the settle takes the backend's declared seams")
		}
		sink := diag.NewSink()
		if err := plugin.Settle(f.Emit, bk, sink); err != nil {
			b.Fatalf("the settle completes: %v", err)
		}
		if sink.Failed() {
			b.Fatal("the corpus settles clean, because a ceiling over a " +
				"partial settle measures the wrong thing")
		}
	}
}

// ScaledFixture returns the benchmark corpus, filtered to a
// backend's declared kind inventory the way [CanonicalFixture]
// filters its coverage: [BenchPackages] packages of [BenchFiles]
// units, each holding [BenchDecls] declarations cycling the
// inventory's kinds in kind order, numbered so every name is
// distinct. Two calls build two equal fixtures.
func ScaledFixture(tb assert.TB, inventory map[symbol.Kind]string) *Fixture {
	tb.Helper()

	requested, valid := requestedKinds(tb, inventory)
	if !valid {
		return nil
	}
	e := plugin.NewEmit()
	n := 0
	for p := range BenchPackages {
		pkg := scaledPackageID(p)
		for file := range BenchFiles {
			decls := make([]symbol.Symbol, 0, BenchDecls)
			origins := make([]symbol.Identity, 0, BenchDecls)
			for range BenchDecls {
				d := scaledDecl(requested[n%len(requested)], n)
				n++
				decls = append(decls, d)
				if id, held := emit.OriginOf(d); held && !id.IsZero() {
					origins = append(origins, id)
				}
			}
			u := plugin.Unit{
				Plugin: emitter,
				Per:    plugin.PerSource,
				Word:   canonicalWord,
				Key:    scaledKey(p, file),
				Pkg:    pkg,
				Decls:  decls,
				// The builder numbers every name upward, so the
				// origins arrive sorted the way a flush leaves
				// them.
				Origins: origins,
			}
			if err := e.Add(u); err != nil {
				tb.Errorf("the scaled %s unit arrives: %v", u.Key, err)
				return nil
			}
		}
	}
	return &Fixture{
		Emit:     e,
		Schedule: []plugin.ID{emitter},
		Trees:    map[plugin.ID]fs.FS{emitter: canonicalTree()},
	}
}

// scaledPackageID is one benchmark package's identity.
func scaledPackageID(p int) symbol.Identity {
	name := "p" + strconv.Itoa(p)
	return symbol.Identity{
		Lang:    fixtureLang,
		Package: fixturePackage + "/" + name,
		Name:    name,
		Kind:    symbol.KindPackage,
	}
}

// scaledKey is one benchmark unit's routing key.
func scaledKey(p, file int) string {
	return fixturePackage + "/p" + strconv.Itoa(p) +
		"/f" + strconv.Itoa(file) + keyExt
}

// scaledDecl returns the nth benchmark declaration of a kind,
// numbered so names stay distinct across the corpus. The
// parameterizable kinds carry the canonical generic shapes, so a
// ceiling measures the surface the backend spells: a parameter
// list on every host, one named bound, an argument-carrying
// reference, and a method declaring parameters of its own over a
// generic receiver.
func scaledDecl(k symbol.Kind, n int) symbol.Symbol {
	i := strconv.Itoa(n)
	switch k {
	case symbol.KindEnum:
		e := &emit.Enum{
			Origin: originOf("phase"+i, symbol.KindEnum),
			Name:   "phase" + i,
		}
		e.Variants.Append(
			&emit.EnumVariant{
				Origin: memberOf("phase"+i, "open", symbol.KindEnumVariant),
				Name:   "open",
			},
			&emit.EnumVariant{
				Origin: memberOf("phase"+i, "closed", symbol.KindEnumVariant),
				Name:   "closed",
			},
		)
		return e
	case symbol.KindStruct:
		s := &emit.Struct{
			Origin:     originOf("row"+i, symbol.KindStruct),
			Doc:        []string{"row" + i + " holds one record."},
			Name:       "row" + i,
			TypeParams: []*emit.TypeParam{{Name: "T"}},
		}
		s.Fields.Append(&emit.Field{
			Origin:  memberOf("row"+i, "name", symbol.KindField),
			Comment: "unique per store",
			Name:    "name",
			Type:    typeRef("string"),
			Tag:     `json:"name"`,
		})
		s.Methods.Append(&emit.Method{
			Origin: memberOf("row"+i, "fetch", symbol.KindMethod),
			Name:   "fetch",
			Body:   emit.Body{Stmts: scaffoldStmts()},
		})
		return s
	case symbol.KindInterface:
		iface := &emit.Interface{
			Origin: originOf("store"+i, symbol.KindInterface),
			Name:   "store" + i,
			TypeParams: []*emit.TypeParam{
				{Name: "K", Bounds: []*emit.TypeRef{typeRef(boundName)}},
			},
			Extends: []*emit.TypeRef{typeRef("Closer")},
		}
		iface.Methods.Append(&emit.Method{
			Origin:  memberOf("store"+i, "get", symbol.KindMethod),
			Name:    "get",
			Params:  []*emit.Param{{Name: "key", Type: typeRef("string")}},
			Returns: []*emit.Return{{Type: typeRef("string")}},
		})
		return iface
	case symbol.KindFunction:
		return &emit.Function{
			Origin: originOf("task"+i, symbol.KindFunction),
			Name:   "task" + i,
			TypeParams: []*emit.TypeParam{
				{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
			},
			Body: emit.Body{Stmts: scaffoldStmts()},
		}
	case symbol.KindMethod:
		return &emit.Method{
			Origin: memberOf("row"+i, "track", symbol.KindMethod),
			Name:   "track",
			Receives: &emit.TypeRef{
				Spelling: "row" + i,
				Args:     []*emit.TypeRef{typeRef("T")},
			},
			TypeParams: []*emit.TypeParam{
				{Name: "U", Bounds: []*emit.TypeRef{typeRef(boundName)}},
			},
			Body: emit.Body{Stmts: []emit.Stmt{{Kind: emit.StmtReturn}}},
		}
	case symbol.KindAlias:
		return &emit.Alias{
			Origin: originOf("id"+i, symbol.KindAlias),
			Name:   "id" + i,
			TypeParams: []*emit.TypeParam{
				{Name: "T", Bounds: []*emit.TypeRef{typeRef(boundName)}},
			},
			Target: &emit.TypeRef{
				Spelling: "keyed",
				Args:     []*emit.TypeRef{typeRef("T")},
			},
		}
	case symbol.KindConstant:
		return &emit.Constant{
			Origin:  originOf("limit"+i, symbol.KindConstant),
			Comment: "rows per call",
			Name:    "limit" + i,
			Value:   "8",
		}
	default: // symbol.KindVariable, by canonicalKinds
		return &emit.Variable{
			Origin: originOf("count"+i, symbol.KindVariable),
			Name:   "count" + i,
			Type:   typeRef("int"),
			Value:  "0",
		}
	}
}
