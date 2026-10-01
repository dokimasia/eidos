// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"io/fs"
	"slices"
	"strconv"
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/bench"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The canonical benchmark scale. Every backend measures over the
// same corpus shape, so two backends' numbers mean one thing and
// a regression names a code change, never a fixture change.
const (
	// BenchPackages is the number of packages in the scaled fixture.
	BenchPackages = 1_000
	// BenchFiles is the number of source units in each package.
	BenchFiles = 10
	// BenchDecls is the number of file-level declarations in each
	// unit. Member declarations nest inside them and are not
	// counted.
	BenchDecls = 20
)

// nameKeyDoc documents the name key the settle benchmark registers.
const nameKeyDoc = "a declaration's name in the target, read by the settle"

// Budget is what a satellite pins its benchmark to. An allocation
// count does not move with machine load, so exceeding the ceiling
// names a real code change. Latency bounds belong beside the pinned
// baseline gates, where the machine is fixed.
type Budget struct {
	// MaxAllocs bounds allocations per rendered corpus. Zero is
	// refused: a benchmark without a ceiling records numbers
	// nobody reads.
	MaxAllocs uint64
}

// BenchRender measures one backend over its setup's fixture and
// fails it above the budget: the fixture builds once outside the
// loop, every iteration renders it whole over a fresh sink, and
// the contract checks the ceiling when the loop ends. An
// iteration returning no file or reporting an Error fails the
// benchmark, because a number over a partial render measures the
// wrong thing. A warning does not fail it.
//
// A satellite's setup returns [ScaledFixture] without the kinds
// its backend refuses, so the corpus shape is the suite's and the
// ceiling is the satellite's.
func BenchRender(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	if budget.MaxAllocs == 0 {
		b.Fatal("the budget states no ceiling")
	}
	r, f := setup(b)
	if f == nil || f.Emit == nil {
		b.Fatal("the setup returns no fixture")
	}
	if bk, held := r.(plugin.Backend); held {
		settleSink := diag.NewSink()
		if err := plugin.Settle(f.Emit, bk, nil, settleSink); err != nil {
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
		files, err := r.Render(f.context(r, sink))
		if err != nil {
			b.Fatalf("the render aborted: %v", err)
		}
		if len(files) == 0 || sink.Failed() {
			b.Fatal("the corpus renders whole and clean")
		}
	}
}

// BenchSettle measures the settle over the setup's corpus: every
// iteration builds a fresh fixture, because a settled store
// settles to itself and a second pass would measure the short
// circuit, and the build runs outside the measurement, so the
// ceiling pins the settle alone. The settle reads a fact store
// whose registry contains the target's name key and whose bags are
// empty, so it reads the key's index and every declared name costs
// its override lookup, the way a run's settle does. The render
// benchmarks settle before their loop, so the two numbers split the
// pipeline between them.
func BenchSettle(b *testing.B, setup Setup, budget Budget) {
	b.Helper()

	if budget.MaxAllocs == 0 {
		b.Fatal("the budget states no ceiling")
	}
	c := bench.Start(b).MaxAllocs(budget.MaxAllocs)
	defer c.End()
	for c.Loop() {
		var r plugin.Renderer
		var f *Fixture
		var sink *diag.Sink
		var facts *meta.Facts
		c.Excluding(func() {
			r, f = setup(b)
			sink = diag.NewSink()
			if bk, held := r.(plugin.Backend); held {
				facts = nameFacts(b, bk.Target())
			}
		})
		if f == nil || f.Emit == nil {
			b.Fatal("the setup returns no fixture")
		}
		bk, held := r.(plugin.Backend)
		if !held {
			b.Fatal("the settle takes the backend's declared seams")
		}
		if err := plugin.Settle(f.Emit, bk, facts, sink); err != nil {
			b.Fatalf("the settle completes: %v", err)
		}
		if sink.Failed() {
			b.Fatal("the corpus settles clean, because a ceiling over a " +
				"partial settle measures the wrong thing")
		}
	}
}

// nameFacts returns an empty fact store whose registry contains the
// target's name key, [plugin.Target.NameKey], so a settle over it
// looks up an override for every name and finds none.
func nameFacts(tb assert.TB, t plugin.Target) *meta.Facts {
	tb.Helper()

	key := t.NameKey()
	r := meta.NewRegistry()
	if err := r.ClaimNamespace(key.Namespace()); err != nil {
		tb.Fatalf("the target's namespace claims: %v", err)
	}
	if _, err := meta.Register[string](r, meta.KeySpec{Name: key, Doc: nameKeyDoc}); err != nil {
		tb.Fatalf("the target's name key registers: %v", err)
	}
	return meta.NewFacts(r)
}

// ScaledFixture returns the benchmark corpus over every canonical
// kind the backend does not refuse: [BenchPackages] packages of
// [BenchFiles] units, each of [BenchDecls] declarations cycling
// the kinds in kind order, numbered so every name is distinct, and
// ordered the way a flush leaves them. A benchmark fails on an
// Error, and a declaration of a refused kind reports one, so the
// corpus leaves out the refused kinds that [CanonicalFixture]
// emits. Two calls build two equal fixtures.
func ScaledFixture(tb assert.TB, refused map[symbol.Kind]string) *Fixture {
	tb.Helper()

	requested := slices.DeleteFunc(slices.Clone(canonicalKinds), func(k symbol.Kind) bool {
		_, out := refused[k]
		return out
	})
	if len(requested) == 0 {
		tb.Errorf("the corpus emits no declaration, because the backend refuses every canonical kind")
		return nil
	}
	slices.Sort(requested)
	e := plugin.NewEmit()
	n := 0
	for p := range BenchPackages {
		pkg := scaledPackageID(p)
		for file := range BenchFiles {
			decls := make([]symbol.Symbol, 0, BenchDecls)
			for range BenchDecls {
				decls = append(decls, scaledDecl(requested[n%len(requested)], n))
				n++
			}
			origins := flushOrder(decls)
			u := plugin.Unit{
				Plugin:  emitter,
				Per:     plugin.PerSource,
				Word:    canonicalWord,
				Key:     scaledKey(p, file),
				Pkg:     pkg,
				Decls:   decls,
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
// numbered so every name is distinct across the corpus. Each
// parameterizable kind takes the canonical generic shape, so a
// ceiling measures the surface the backend spells: a parameter
// list on every host, one named bound, a reference with type
// arguments, and a method declaring parameters of its own over a
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
	case symbol.KindSum:
		s := &emit.Sum{
			Origin:     originOf("shape"+i, symbol.KindSum),
			Name:       "shape" + i,
			TypeParams: []*emit.TypeParam{{Name: "T"}},
		}
		circle := &emit.SumVariant{
			Origin: memberOf("shape"+i, "circle", symbol.KindSumVariant),
			Name:   "circle",
		}
		circle.Fields.Append(&emit.Field{
			Origin: memberOf("circle", "item", symbol.KindField),
			Name:   "item",
			Type:   typeRef("T"),
		})
		s.Variants.Append(circle, &emit.SumVariant{
			Origin: memberOf("shape"+i, "empty", symbol.KindSumVariant),
			Name:   "empty",
		})
		return s
	case symbol.KindStruct:
		s := &emit.Struct{
			Origin:     originOf("row"+i, symbol.KindStruct),
			Doc:        []string{"row" + i + " is one record."},
			Name:       "row" + i,
			TypeParams: []*emit.TypeParam{{Name: "T"}},
		}
		// The canonical fixture states the field tag; the scaled
		// corpus leaves it out, so a target refusing tags measures
		// its render and not a warning per struct.
		s.Fields.Append(&emit.Field{
			Origin:  memberOf("row"+i, "name", symbol.KindField),
			Comment: "unique per store",
			Name:    "name",
			Type:    typeRef("string"),
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
				Args:     []*emit.TypeRef{paramOf("row"+i, "T")},
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
			Type:    typeRef("int32"),
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
