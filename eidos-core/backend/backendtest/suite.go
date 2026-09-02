// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"bytes"
	"cmp"
	"slices"
	"strings"
	"testing"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// RunBackendSuite holds a renderer to the checks a render returns
// as values: the fixture is populated, two runs produce
// byte-identical files, every emit kind the fixture carries
// renders, every body arrives whole, and a file's failure reports
// positioned and attributed while the render continues. The
// header and trailer checks are the output contract's and join the
// suite with it.
func RunBackendSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("an populated fixture", func(t *testing.T) {
		t.Parallel()
		AssertPopulatedFixture(t, setup)
	})
	t.Run("byte-stable render", func(t *testing.T) {
		t.Parallel()
		AssertDeterministicRender(t, setup)
	})
	t.Run("every kind renders", func(t *testing.T) {
		t.Parallel()
		AssertSpeltKinds(t, setup)
	})
	t.Run("content arrives whole", func(t *testing.T) {
		t.Parallel()
		AssertPlacedContent(t, setup)
	})
	t.Run("a file failure continues", func(t *testing.T) {
		t.Parallel()
		AssertContinuedRender(t, setup)
	})
	t.Run("the settle preserves the structure", func(t *testing.T) {
		t.Parallel()
		AssertSettledShape(t, setup)
	})
	t.Run("stated facts stay covered", func(t *testing.T) {
		t.Parallel()
		AssertCoveredFacts(t, setup)
	})
	t.Run("every member arrives", func(t *testing.T) {
		t.Parallel()
		AssertRenderedMembers(t, setup)
	})
}

// AssertRenderedMembers settles one setup's fixture, renders it,
// and holds every member declaration to appearing in the output:
// each settled field, method and variant name occurs in the
// rendered bytes, or a finding names it. A host template that
// ranges some member lists and forgets one drops those members
// with no finding — the drop is invisible to the kind and fact
// checks, because neither visits a member a template never
// renders, so this check reads the bytes instead.
func AssertRenderedMembers(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	sink := diag.NewSink()
	if b, held := r.(plugin.Backend); held {
		if err := plugin.Settle(f.Emit, b, sink); err != nil {
			tb.Errorf("the settle completes: %v", err)
			return
		}
	}
	files, err := r.Render(f.context(sink))
	assert.NoError(tb, err, "the settled fixture renders")

	var out bytes.Buffer
	for _, file := range files {
		out.Write(file.Body)
	}
	var excused strings.Builder
	for d := range sink.All() {
		excused.WriteString(d.Msg)
		excused.WriteString("\n")
	}

	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			host, members := memberNames(d)
			for _, name := range members {
				if bytes.Contains(out.Bytes(), []byte(name)) ||
					strings.Contains(excused.String(), name) {
					continue
				}
				tb.Errorf("member %s of %s renders nowhere and no finding names it",
					name, host)
			}
		}
	}
}

// memberNames returns a declaration's name and its member names:
// what the presence check hunts for in the rendered bytes.
func memberNames(s symbol.Symbol) (string, []string) {
	var names []string
	member := func(n string) {
		if n != "" {
			names = append(names, n)
		}
	}
	switch d := s.(type) {
	case *emit.Struct:
		for _, m := range d.Fields.Items() {
			member(m.Name)
		}
		for _, m := range d.Methods.Items() {
			member(m.Name)
		}
		return d.Name, names
	case *emit.Interface:
		for _, m := range d.Fields.Items() {
			member(m.Name)
		}
		for _, m := range d.Methods.Items() {
			member(m.Name)
		}
		return d.Name, names
	case *emit.Enum:
		for _, v := range d.Variants.Items() {
			member(v.Name)
		}
		return d.Name, names
	case *emit.Sum:
		for _, v := range d.Variants.Items() {
			member(v.Name)
		}
		return d.Name, names
	default:
		return "", nil
	}
}

// AssertCoveredFacts holds a backend's declared fact coverage to
// the render: the declaration is total over the fact set, its
// exceptions stay on facts their kind can state, the settled
// fixture's run reports exactly the refusals the declaration
// states, and no stated fact meets an undeclared verdict. A
// renderer declaring no coverage passes vacuously, which is what a
// backend predating the contract runs under.
func AssertCoveredFacts(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	if f == nil || f.Emit == nil {
		tb.Errorf("the setup carries no fixture")
		return
	}
	c, covered := r.(render.Coverer)
	if !covered || !c.Coverage().Declared() {
		return
	}
	coverage := c.Coverage()

	for _, fact := range symbol.Facts() {
		if coverage.Facts[fact] == render.VerdictUndeclared {
			tb.Errorf("the coverage takes no stance on %s: a declaration is "+
				"total over the fact set", fact)
		}
	}
	can := emit.KindFacts()
	for kind, facts := range coverage.Except {
		for fact := range facts {
			if !slices.Contains(can[kind], fact) {
				tb.Errorf("the coverage excepts %s on a %s, which states no "+
					"such fact", fact, kind)
			}
		}
	}

	if b, held := r.(plugin.Backend); held {
		sink := diag.NewSink()
		if err := plugin.Settle(f.Emit, b, sink); err != nil {
			tb.Errorf("the settle completes: %v", err)
			return
		}
		if sink.Failed() {
			tb.Errorf("the suite's fixture settles clean")
			return
		}
	}
	type statedOn struct {
		kind symbol.Kind
		fact symbol.Fact
	}
	expect := map[statedOn]int{}
	total := 0
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			emit.Facts(d, func(_ symbol.Symbol, kind symbol.Kind, fact symbol.Fact) {
				if coverage.Of(kind, fact) == render.Refuses {
					expect[statedOn{kind: kind, fact: fact}]++
					total++
				}
			})
		}
	}

	sink := diag.NewSink()
	if _, err := r.Render(f.context(sink)); err != nil {
		tb.Errorf("the render completes: %v", err)
		return
	}
	var refusals []string
	for _, d := range slices.Collect(sink.All()) {
		switch d.Code {
		case render.UndeclaredFact:
			tb.Errorf("a stated fact met no verdict: %s", d.Msg)
		case render.RefusedFact:
			refusals = append(refusals, d.Msg)
		}
	}
	assert.Equal(tb, len(refusals), total,
		"the run reports exactly the refusals the declaration states")
	for key := range expect {
		held := slices.ContainsFunc(refusals, func(msg string) bool {
			return strings.Contains(msg, key.fact.String()) &&
				strings.Contains(msg, key.kind.String())
		})
		assert.True(tb, held, "a stated "+key.fact.String()+" on a "+
			key.kind.String()+" reports its refusal")
	}
}

// RenderSettled settles one setup's fixture and renders it once,
// requiring a clean run, so a satellite's own convention pins read
// the rendered bytes without repeating the plumbing.
func RenderSettled(tb assert.TB, setup Setup) []plugin.RenderedFile {
	tb.Helper()

	files, diags := runRender(tb, setup)
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			tb.Errorf("the settled fixture renders clean: %s", d.Msg)
		}
	}
	return files
}

// AssertSettledShape settles one setup's fixture and holds the
// settle to the changes its seams may declare: the unit count,
// keys and origins survive whatever runs, and a backend declaring
// no construct lowering keeps every declaration equal to a fresh
// build once every declared name normalizes. A settle reporting an
// Error over the suite's fixture fails the check, because the
// canonical declarations spell in every convention.
func AssertSettledShape(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	if f == nil || f.Emit == nil {
		tb.Errorf("the setup carries no fixture")
		return
	}
	b, held := r.(plugin.Backend)
	if !held {
		return // a hand-rolled renderer declares no seams
	}
	sink := diag.NewSink()
	if err := plugin.Settle(f.Emit, b, sink); err != nil {
		tb.Errorf("the settle completes: %v", err)
		return
	}
	assert.True(tb, !sink.Failed(), "the suite's fixture settles clean")

	_, fresh := setup(tb)
	settled := slices.Collect(f.Emit.Units())
	emitted := slices.Collect(fresh.Emit.Units())
	assert.Equal(tb, len(settled), len(emitted),
		"the settle adds and drops no unit")
	for i := range settled {
		if i >= len(emitted) {
			return
		}
		assert.Equal(tb, settled[i].Key, emitted[i].Key,
			"a routing key survives the settle")
		assert.Equal(tb, settled[i].Origins, emitted[i].Origins,
			"and so does a unit's provenance")
	}

	if _, lowers := r.(plugin.Lowerer); lowers {
		return // a lowering may reshape declarations; origins held above
	}
	normalizeStore(settled)
	normalizeStore(emitted)
	for i := range settled {
		assert.Equal(tb, settled[i].Decls, emitted[i].Decls,
			"a respell changes name fields and reference spellings alone")
	}
}

// normalizeStore rewrites every declared name and every reference
// spelling to one fixed form, per package the way the settle's own
// table scopes, so two stores compare modulo names.
func normalizeStore(units []plugin.Unit) {
	tables := map[string]map[string]bool{}
	for _, u := range units {
		table := tables[u.Pkg.Package]
		if table == nil {
			table = map[string]bool{}
			tables[u.Pkg.Package] = table
		}
		for _, d := range u.Decls {
			_ = emit.RespellNames(d, func(
				host symbol.Symbol, kind symbol.Kind, v symbol.Visibility, name string,
			) (string, error) {
				table[name] = true
				return normalName, nil
			})
		}
	}
	for _, u := range units {
		table := tables[u.Pkg.Package]
		for _, d := range u.Decls {
			for s := range emit.All(d) {
				switch t := s.(type) {
				case *emit.TypeRef:
					if table[t.Spelling] {
						t.Spelling = normalName
					}
				case *emit.Function:
					normalizeBody(&t.Body, table)
				case *emit.Method:
					normalizeBody(&t.Body, table)
				}
			}
		}
	}
}

// normalName is the one spelling normalization writes.
const normalName = "n"

// normalizeBody rewrites a body's structured names where they
// match a declared name, mirroring what the settle may rewrite.
func normalizeBody(b *emit.Body, table map[string]bool) {
	if b.Verbatim != "" {
		return
	}
	normalizeStmts(b.Prologue.Items(), table)
	normalizeStmts(b.Stmts, table)
	for _, s := range b.Slots {
		if s != nil {
			normalizeStmts(s.Slot.Items(), table)
		}
	}
	normalizeStmts(b.Epilogue.Items(), table)
}

// normalizeStmts rewrites one statement run's names.
func normalizeStmts(stmts []emit.Stmt, table map[string]bool) {
	for i := range stmts {
		s := &stmts[i]
		normalizeExpr(&s.Value, table)
		if s.Name != "" && table[s.Name] {
			s.Name = normalName
		}
		normalizeStmts(s.Then, table)
	}
}

// normalizeExpr rewrites one expression tree's names.
func normalizeExpr(x *emit.Expr, table map[string]bool) {
	if x == nil {
		return
	}
	switch x.Kind {
	case emit.ExprName:
		if table[x.Name] {
			x.Name = normalName
		}
	case emit.ExprCall:
		normalizeExpr(x.Fn, table)
		for i := range x.Args {
			normalizeExpr(&x.Args[i], table)
		}
	}
}

// runRender is one check's render call: a fresh setup, a fresh sink
// and the fatality rule held, because a renderer returns an error
// for a defect in the pass's own inputs, never for a problem with
// one file.
func runRender(tb assert.TB, setup Setup) ([]plugin.RenderedFile, []diag.Diag) {
	tb.Helper()

	r, f := setup(tb)
	sink := diag.NewSink()
	if b, held := r.(plugin.Backend); held {
		if err := plugin.Settle(f.Emit, b, sink); err != nil {
			tb.Errorf("the settle completes: %v", err)
			return nil, nil
		}
	}
	files, err := r.Render(f.context(sink))
	assert.NoError(tb, err,
		"a file's problem attaches to the sink and the render continues")
	return files, slices.Collect(sink.All())
}

// AssertPopulatedFixture refuses an empty store: a suite over a
// store holding no units passes every check vacuously and proves
// nothing about the backend.
func AssertPopulatedFixture(tb assert.TB, setup Setup) {
	tb.Helper()

	_, f := setup(tb)
	assert.True(tb, f.Emit != nil, "the fixture carries a store")
	if f.Emit == nil {
		return
	}
	units := 0
	for range f.Emit.Units() {
		units++
	}
	assert.True(tb, units > 0, "the fixture emits at least one unit")
}

// AssertDeterministicRender renders two isolated setups and holds
// the files byte-equal: the same names, the same packages, the
// same bytes, which is the byte-identity contract as values. The
// findings must match as a set too; only their order is the run's,
// because the pass reports in completion order.
func AssertDeterministicRender(tb assert.TB, setup Setup) {
	tb.Helper()

	first, firstDiags := runRender(tb, setup)
	second, secondDiags := runRender(tb, setup)
	assert.Equal(tb, first, second,
		"two isolated renders produce the same bytes")
	assert.Equal(tb, sorted(firstDiags), sorted(secondDiags),
		"and report the same findings")
}

// sorted orders findings canonically, so two valid runs reporting
// one set in two completion orders compare equal.
func sorted(diags []diag.Diag) []diag.Diag {
	slices.SortFunc(diags, func(a, b diag.Diag) int {
		return cmp.Or(
			cmp.Compare(a.Pos.File, b.Pos.File),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Col, b.Pos.Col),
			cmp.Compare(a.Code.String(), b.Code.String()),
			cmp.Compare(a.Msg, b.Msg),
			cmp.Compare(string(a.Origin), string(b.Origin)),
		)
	})
	return diags
}

// AssertSpeltKinds renders once and refuses a kind the language
// cannot spell: whatever inventory the fixture emits, the backend
// holds a spelling for it. The fixture owns the coverage, so a
// backend claims the full kind set by emitting the full kind set.
func AssertSpeltKinds(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		assert.True(tb, d.Code != render.UnspeltKind,
			"every kind the fixture emits has a spelling: "+d.Msg)
	}
}

// AssertPlacedContent renders once and holds every body to arriving
// whole: no conflicting forms, no reference resolving to nothing,
// no pending slot content dropped by its template.
func AssertPlacedContent(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		assert.True(tb,
			d.Code != render.BodyConflict &&
				d.Code != render.UnresolvedRef &&
				d.Code != render.DroppedSlots,
			"every body arrives whole: "+d.Msg)
	}
}

// AssertContinuedRender renders once and holds the failure
// semantics: the call returns no error, every finding carries a
// position and the suite's origin, and a file reported unformatted
// is withheld from the values.
func AssertContinuedRender(tb assert.TB, setup Setup) {
	tb.Helper()

	files, diags := runRender(tb, setup)
	names := make(map[string]struct{}, len(files))
	for _, f := range files {
		names[f.Name] = struct{}{}
	}
	for _, d := range diags {
		assert.True(tb, !d.Pos.IsZero(),
			"every finding carries a position: "+d.Msg)
		assert.Equal(tb, d.Origin, origin,
			"every finding carries the context's plugin as origin")
		if d.Code == render.UnformattedFile {
			_, returned := names[d.Pos.File]
			assert.False(tb, returned,
				"a file the formatter refused stays withheld: "+d.Pos.File)
		}
	}
}
