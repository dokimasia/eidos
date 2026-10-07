// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backendtest

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/core/backend/render"
	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// unplaced are the codes of a body that does not arrive whole:
// conflicting forms, a reference resolving to nothing, and pending slot
// content its template dropped.
var unplaced = []diag.Code{render.BodyConflict, render.UnresolvedRef, render.DroppedSlots}

// RunBackendSuite runs the eight checks a render returns as values
// against a renderer: the fixture is populated, two runs produce
// byte-identical files, every emit kind in the fixture renders or
// reports its declared refusal, every body arrives whole, a file's
// failure reports positioned and attributed while the render
// continues, the settle preserves the structure, the declared fact
// coverage matches the refusals, and every member arrives in its
// host's file. The header and trailer checks are the output
// contract's, in [AssertStamped].
func RunBackendSuite(t *testing.T, setup Setup) {
	t.Helper()

	t.Run("a populated fixture", func(t *testing.T) {
		t.Parallel()
		AssertPopulatedFixture(t, setup)
	})
	t.Run("byte-stable render", func(t *testing.T) {
		t.Parallel()
		AssertDeterministicRender(t, setup)
	})
	t.Run("every kind renders or is refused", func(t *testing.T) {
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
	t.Run("stated facts are covered", func(t *testing.T) {
		t.Parallel()
		AssertCoveredFacts(t, setup)
	})
	t.Run("every member arrives", func(t *testing.T) {
		t.Parallel()
		AssertRenderedMembers(t, setup)
	})
}

// AssertRenderedMembers settles one setup's fixture, renders it,
// and fails on a member declaration missing from the output: each
// settled field, method and variant name occurs as a whole word in
// a file that also contains its host's name, or a finding names
// it. A host template that ranges some member lists and forgets one
// drops those members with no finding. The kind and fact checks
// never visit a member a template never renders, so this check
// reads the rendered bytes. It reads only the files that contain
// the host's name, and a declaration elsewhere that shares a
// member's name does not count for the member. A host of a kind the
// backend refuses renders nothing, so the check skips its members.
// Each member reports on its own, as Host.Member, in the store's order.
func AssertRenderedMembers(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	sink := diag.NewSink()
	if b, held := r.(plugin.Backend); held {
		assert.NoError(tb, plugin.Settle(f.Emit, b, nil, sink), "the settle completes")
	}
	files, err := r.Render(f.context(r, sink))
	assert.NoError(tb, err, "the settled fixture renders")

	var excused strings.Builder
	for d := range sink.All() {
		excused.WriteString(d.Msg)
		excused.WriteString("\n")
	}

	refused := refusedKinds(r)
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			if _, skipped := refused[d.Kind()]; skipped {
				continue
			}
			host, members := memberNames(d)
			for _, name := range members {
				accounted := renderedBeside(files, host, name) || strings.Contains(excused.String(), name)
				expect.True(tb, accounted, host+"."+name+" renders beside its host, or a finding names it")
			}
		}
	}
}

// renderedBeside reports whether one file contains both the host's
// name and the member's name as whole words.
func renderedBeside(files []plugin.RenderedFile, host, member string) bool {
	for _, file := range files {
		if containsWord(file.Body, host) && containsWord(file.Body, member) {
			return true
		}
	}
	return false
}

// containsWord reports whether word occurs in body as a whole
// identifier: an occurrence counts only where neither neighbouring
// byte can continue an identifier.
func containsWord(body []byte, word string) bool {
	if word == "" {
		return false
	}
	for from := 0; from < len(body); {
		at := bytes.Index(body[from:], []byte(word))
		if at < 0 {
			return false
		}
		start, end := from+at, from+at+len(word)
		if (start == 0 || !identByte(body[start-1])) && (end == len(body) || !identByte(body[end])) {
			return true
		}
		from = start + 1
	}
	return false
}

// identByte reports whether c can continue an identifier in any
// target: an ASCII letter, a digit, an underscore, or a byte of a
// multi-byte UTF-8 letter.
func identByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= utf8.RuneSelf
}

// memberNames returns a declaration's name and its member names:
// what the presence check searches the rendered bytes for.
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

// refusedKinds returns the kinds a renderer declares refused, and
// nil for a renderer that declares none.
func refusedKinds(r plugin.Renderer) map[symbol.Kind]string {
	if rf, declares := r.(render.Refuser); declares {
		return rf.RefusedKinds()
	}
	return nil
}

// AssertCoveredFacts checks a backend's declared fact coverage
// against the render: the declaration is total over the fact set,
// its exceptions name facts their kind can state, the settled
// fixture's run reports exactly the refusals the declaration
// states, and no stated fact meets an undeclared verdict. A
// declaration of a kind the backend refuses renders nothing, so the
// check expects no fact of it to report. A renderer declaring no
// coverage fails: without a declaration the guard is disarmed and
// every narrowing goes silent, which is the defect class the
// contract exists to refuse.
//
// The setup's fixture, the declaration, the settle and the render stop
// the check where they fail, because nothing after them has an input.
// The declaration's totality, its exceptions and the run's refusals
// each report on their own, so one run names every way the coverage
// falls short.
func AssertCoveredFacts(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	assert.NotNil(tb, f, "the setup returns a fixture")
	assert.NotNil(tb, f.Emit, "the fixture contains a store")
	c, covered := r.(render.Coverer)
	assert.True(tb, covered, "the renderer reports its fact coverage, because an undeclared "+
		"coverage disarms the narrowing guard")
	coverage := c.Coverage()
	assert.True(tb, coverage.Declared(), "the renderer declares its fact coverage, because an "+
		"undeclared coverage disarms the narrowing guard")

	for _, fact := range symbol.Facts() {
		expect.NotEqual(tb, coverage.Facts[fact], render.VerdictUndeclared, "the coverage takes a stance on "+
			fact.String()+": a declaration is total over the fact set")
	}
	can := emit.KindFacts()
	for _, kind := range slices.Sorted(maps.Keys(coverage.Except)) {
		for _, fact := range slices.Sorted(maps.Keys(coverage.Except[kind])) {
			expect.Contains(tb, can[kind], fact,
				"the coverage excepts "+fact.String()+" on a "+kind.String()+", which states it")
		}
	}

	if b, held := r.(plugin.Backend); held {
		sink := diag.NewSink()
		assert.NoError(tb, plugin.Settle(f.Emit, b, nil, sink), "the settle completes")
		assert.False(tb, sink.Failed(), "the suite's fixture settles clean")
	}
	type statedOn struct {
		kind symbol.Kind
		fact symbol.Fact
	}
	stated := map[statedOn]int{}
	refused := refusedKinds(r)
	for u := range f.Emit.Units() {
		for _, d := range u.Decls {
			if _, skipped := refused[d.Kind()]; skipped {
				continue
			}
			emit.Facts(d, func(_ symbol.Symbol, kind symbol.Kind, fact symbol.Fact) {
				if coverage.Of(kind, fact) == render.Refuses {
					stated[statedOn{kind: kind, fact: fact}]++
				}
			})
		}
	}

	sink := diag.NewSink()
	_, err := r.Render(f.context(r, sink))
	assert.NoError(tb, err, "the render completes")
	// Per key and not per total: matching counts with one refusal
	// missing and one undeclared cancel out, and the undeclared half
	// is exactly the silent narrowing the coverage exists to catch.
	seen := map[statedOn]int{}
	for d := range sink.All() {
		expect.NotEqual(tb, d.Code, render.UndeclaredFact, "every stated fact meets a verdict: "+d.Msg)
		if d.Code != render.RefusedFact {
			continue
		}
		matched := false
		for key := range stated {
			// The exact tail is what tells a SumVariant's refusal from a
			// Sum's, and a TypeParamDefault's from a ParamDefault's.
			if strings.HasSuffix(d.Msg, refusalTail(key.fact, key.kind)) {
				seen[key]++
				matched = true
				break
			}
		}
		expect.True(tb, matched, "the declaration refuses what the run refused: "+d.Msg)
	}
	expect.Equal(tb, seen, stated, "each refused fact reports its refusal once per statement, "+
		"under the kind it is stated on")
}

// refusalTail returns the end of the render pass's refused-fact
// finding: the fact, then the kind it was stated on.
func refusalTail(fact symbol.Fact, kind symbol.Kind) string {
	return " no spelling for " + fact.String() + " stated on a " + kind.String()
}

// RenderSettled settles one setup's fixture and renders it once,
// and fails on an Error other than a declared kind refusal, so a
// satellite's own convention pins read the rendered bytes without
// repeating the plumbing. The canonical fixture emits every kind,
// so a backend that refuses one reports it over the fixture and
// still renders clean.
func RenderSettled(tb assert.TB, setup Setup) []plugin.RenderedFile {
	tb.Helper()

	files, diags := runRender(tb, setup)
	for _, d := range diags {
		if d.Code != render.RefusedKind {
			expect.NotEqual(tb, d.Severity, diag.SeverityError, "the settled fixture renders clean: "+d.Msg)
		}
	}
	return files
}

// AssertSettledShape settles one setup's fixture and fails on a
// change its seams may not declare: the unit count, keys and
// origins survive whatever runs, and a backend declaring no
// construct lowering keeps every declaration equal to a fresh
// build once every declared name normalizes. A settle reporting an
// Error over the suite's fixture fails the check, because the
// canonical declarations spell in every convention. A changed unit
// count stops the check, and each unit's key, provenance and
// declarations then report on their own.
func AssertSettledShape(tb assert.TB, setup Setup) {
	tb.Helper()

	r, f := setup(tb)
	assert.NotNil(tb, f, "the setup returns a fixture")
	assert.NotNil(tb, f.Emit, "the fixture contains a store")
	b, held := r.(plugin.Backend)
	if !held {
		return // a hand-rolled renderer declares no seams
	}
	sink := diag.NewSink()
	assert.NoError(tb, plugin.Settle(f.Emit, b, nil, sink), "the settle completes")
	assert.False(tb, sink.Failed(), "the suite's fixture settles clean")

	_, fresh := setup(tb)
	settled := slices.Collect(f.Emit.Units())
	emitted := slices.Collect(fresh.Emit.Units())
	assert.Length(tb, settled, len(emitted), "the settle adds and drops no unit")
	for i := range min(len(settled), len(emitted)) {
		expect.Equal(tb, settled[i].Key, emitted[i].Key, "a routing key survives the settle")
		expect.Equal(tb, settled[i].Origins, emitted[i].Origins, "and so does a unit's provenance")
	}

	if _, lowers := r.(plugin.Lowerer); lowers {
		return // a lowering may reshape declarations; the loop checked the origins
	}
	normalizeStore(settled)
	normalizeStore(emitted)
	for i := range min(len(settled), len(emitted)) {
		expect.Equal(tb, settled[i].Decls, emitted[i].Decls,
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
				_, _ symbol.Symbol, _ symbol.Kind, _ symbol.Visibility, name string,
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
// and the fatality rule checked, because a renderer returns an
// error for a defect in the pass's own inputs, never for a problem
// with one file.
func runRender(tb assert.TB, setup Setup) ([]plugin.RenderedFile, []diag.Diag) {
	tb.Helper()

	r, f := setup(tb)
	sink := diag.NewSink()
	if b, held := r.(plugin.Backend); held {
		assert.NoError(tb, plugin.Settle(f.Emit, b, nil, sink), "the settle completes")
	}
	files, err := r.Render(f.context(r, sink))
	assert.NoError(tb, err,
		"a file's problem attaches to the sink and the render continues")
	return files, slices.Collect(sink.All())
}

// AssertPopulatedFixture fails on an empty store: a suite over a
// store without units passes every check vacuously and proves
// nothing about the backend.
func AssertPopulatedFixture(tb assert.TB, setup Setup) {
	tb.Helper()

	_, f := setup(tb)
	assert.NotNil(tb, f.Emit, "the fixture contains a store")
	assert.NotEmpty(tb, slices.Collect(f.Emit.Units()), "the fixture emits at least one unit")
}

// rendered is what one isolated render returns as values: its files,
// and its findings in canonical order.
type rendered struct {
	files    []plugin.RenderedFile
	findings []diag.Diag
}

// AssertDeterministicRender renders isolated setups, one per call
// of [assert.Deterministic], and fails unless every render returns
// the files of the first: the same paths, the same packages, the same
// bytes, which is the byte-identity contract as values. The findings
// must match as a set too. Only their order is the run's, because the
// pass reports in completion order.
func AssertDeterministicRender(tb assert.TB, setup Setup) {
	tb.Helper()

	assert.Deterministic(tb, func(s Setup) (rendered, error) {
		files, findings := runRender(tb, s)
		slices.SortFunc(findings, diag.Diag.Compare)
		return rendered{files: files, findings: findings}, nil
	}, setup, "isolated renders return the same files and report the same findings")
}

// AssertSpeltKinds renders once and fails on a declaration of a
// kind the language neither spells nor refuses. The canonical
// fixture emits every kind an emit declaration takes at file level,
// so over it each kind renders, reports under [render.RefusedKind]
// with the reason the backend declares, or fails this check under
// [render.UnspeltKind]. Each finding reports on its own.
func AssertSpeltKinds(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		expect.NotEqual(tb, d.Code, render.UnspeltKind,
			"every kind the fixture emits has a spelling or a declared refusal: "+d.Msg)
	}
}

// AssertPlacedContent renders once and fails on a body that does
// not arrive whole: conflicting forms, a reference resolving to
// nothing, or pending slot content its template dropped. Each finding
// reports on its own.
func AssertPlacedContent(tb assert.TB, setup Setup) {
	tb.Helper()

	_, diags := runRender(tb, setup)
	for _, d := range diags {
		expect.NotContains(tb, unplaced, d.Code, "every body arrives whole: "+d.Msg)
	}
}

// AssertContinuedRender renders once and fails on a breach of the
// failure semantics: the call returns no error, every finding
// states a position and the suite's origin, and a file reported
// unformatted is withheld from the values. Each finding reports on its
// own.
func AssertContinuedRender(tb assert.TB, setup Setup) {
	tb.Helper()

	files, diags := runRender(tb, setup)
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	for _, d := range diags {
		expect.NotEqual(tb, d.Pos, position.Pos{}, "every finding states a position: "+d.Msg)
		expect.Equal(tb, d.Origin, origin, "every finding names the context's plugin as its origin: "+d.Msg)
		if d.Code == render.UnformattedFile {
			expect.NotContains(tb, paths, d.Pos.File, "a file the formatter refused remains withheld")
		}
	}
}
