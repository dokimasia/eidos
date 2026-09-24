// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"bytes"
	"io/fs"
	"iter"
	"maps"
	"path"
	"slices"
	"strings"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The brands the ownership check stamps under, and the suffixes
// the stamped copies take beside the file they copy.
const (
	ownBrand      output.Brand = "frontendtest"
	foreignBrand  output.Brand = "frontendtest-foreign"
	ownedSuffix                = "_owned"
	foreignSuffix              = "_foreign"
)

// AssertDeterministicParse loads the fixture twice and compares
// what each load recorded: the graphs byte for byte, the attached
// directives and classification stamps, and the findings in report
// order. Reparsing unchanged files yields the same identities, or
// diff-by-identity later stands on sand.
func AssertDeterministicParse(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	one := drive(tb, f, fx)
	two := drive(tb, f, fx)
	assert.True(tb, bytes.Equal(encoded(tb, one.graph), encoded(tb, two.graph)),
		"two parses of one fixture encode identically")
	assert.Equal(tb, entries(two.graph.Directives()), entries(one.graph.Directives()),
		"and attach the same directives")
	assert.Equal(tb, entries(two.graph.Stamps()), entries(one.graph.Stamps()),
		"and the same classification stamps")
	assert.Equal(tb, slices.Collect(two.sink.All()), slices.Collect(one.sink.All()),
		"and report the same findings in the same order")
}

// entry is one subject and what a load attached to it.
type entry[V any] struct {
	subject symbol.Identity
	values  V
}

// entries collects an enumeration the store makes in identity
// order.
func entries[V any](seq iter.Seq2[symbol.Identity, V]) []entry[V] {
	var out []entry[V]
	for id, v := range seq {
		out = append(out, entry[V]{subject: id, values: v})
	}
	return out
}

// AssertPositionedDiagnostics holds every finding to its address:
// a finding without a position or an origin is a defect in the
// frontend that reported it.
func AssertPositionedDiagnostics(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	got := drive(tb, f, fx)
	for d := range got.sink.All() {
		if d.Pos.File == "" {
			tb.Errorf("finding %s %q carries no position", d.Code, d.Msg)
		}
		if d.Origin == "" {
			tb.Errorf("finding %s %q carries no origin", d.Code, d.Msg)
		}
	}
}

// AssertClassified holds the claim to account: every selected file
// either declares into the graph or has a finding naming it, so
// nothing drops in silence. A fixture declaring classification keys
// is stamped by the load, and the recorded stamps apply cleanly
// under those keys, the way the workspace run applies them. A load
// that stamps under a fixture declaring no keys fails, because
// nothing could apply its stamps.
func AssertClassified(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	got := drive(tb, f, fx)

	declared := map[string]bool{}
	for decl := range got.graph.ByKind(symbol.KindFile) {
		if file, is := decl.(*node.File); is {
			declared[file.Path] = true
		}
	}
	named := map[string]bool{}
	for d := range got.sink.All() {
		named[d.Pos.File] = true
	}
	for _, path := range selected(tb, f, fx) {
		if !declared[path] && !named[path] {
			tb.Errorf(
				"%s is selected and neither declares nor reports: a silently dropped file",
				path,
			)
		}
	}

	stamped := false
	for range got.graph.Stamps() {
		stamped = true
		break
	}
	switch {
	case fx.Keys == nil && stamped:
		tb.Errorf("the fixture stamps and declares no keys to apply them under")
		return
	case fx.Keys == nil:
		return
	case !stamped:
		tb.Errorf("the fixture declares classification keys and the load stamps nothing: " +
			"a classifier that never stamps")
		return
	}
	registry := meta.NewRegistry()
	_, err := meta.Kernel(registry)
	assert.NoError(tb, err, "the kernel's own keys register, as the workspace registers them")
	assert.NoError(tb, fx.Keys(registry), "the fixture's keys register")
	facts := meta.NewFacts(registry)
	for id, stamps := range got.graph.Stamps() {
		for i, s := range stamps {
			err := facts.StampRaw(s, meta.Claim{
				Subject:   id,
				Authority: meta.AuthorityPlugin,
				Plugin:    s.Origin,
				Seq:       i,
				Pos:       s.Pos,
			})
			assert.NoError(tb, err, "a recorded stamp applies under the fixture's keys")
		}
	}
}

// AssertOwnedExcluded holds the one exclusion the kernel owns: a
// selected file framed under the load's own brand is the
// workspace's output and never reaches a unit, while the same file
// framed under another brand is ordinary input and loads. The
// check stamps the copies itself through the language's own
// comment syntax, so a fixture states nothing.
func AssertOwnedExcluded(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	files := selected(tb, f, fx)
	source, err := fs.ReadFile(fx.Sources, files[0])
	assert.NoError(tb, err, "a selected file reads")
	if len(source) == 0 || source[len(source)-1] != '\n' {
		source = append(slices.Clone(source), '\n')
	}

	ext := path.Ext(files[0])
	stem := strings.TrimSuffix(files[0], ext)
	tree := copyTree(tb, fx.Sources)
	own := stem + ownedSuffix + ext
	foreign := stem + foreignSuffix + ext
	tree[own] = &fstest.MapFile{Data: framed(tb, f, ownBrand, own, source)}
	tree[foreign] = &fstest.MapFile{Data: framed(tb, f, foreignBrand, foreign, source)}
	assert.True(tb, load.Match(f.Selection(), own),
		"the stamped copy sits beside its source, where the claim reaches it")

	got := drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures},
		func(cfg *load.Config) { cfg.Brand = ownBrand })
	assert.Equal(tb, got.report.Excluded, []string{own},
		"the load refuses its own output and lists it")
	for _, u := range got.report.Units {
		assert.False(tb, slices.Contains(u.Files, own), "no unit holds the refused file")
	}
	held := false
	for _, u := range got.report.Units {
		held = held || slices.Contains(u.Files, foreign)
	}
	assert.True(tb, held, "another brand's output is ordinary input, held by a unit")
	for decl := range got.graph.ByKind(symbol.KindFile) {
		if file, is := decl.(*node.File); is {
			assert.False(tb, file.Path == own, "the refused file declares nothing")
		}
	}
}

// framed stamps source as one brand's output through the
// language's own comment syntax.
func framed(
	tb assert.TB, f plugin.Frontend, brand output.Brand, name string, source []byte,
) []byte {
	tb.Helper()

	contract, err := output.NewContract(brand, f.Syntax())
	assert.NoError(tb, err, "the language's syntax carries the frame")
	b, err := contract.Stamp(plugin.RenderedFile{
		Name: path.Base(name), Plugins: []plugin.ID{f.Name()}, Body: source,
	})
	assert.NoError(tb, err, "the source stamps")
	return b
}

// AssertFingerprinted holds the unit keys honest: stable across
// two identical loads, and changed by each folded part — a read, a
// depth, a declared version, the options, the plugin set. The
// model fingerprint is a compiled constant no test can vary. A
// unit missing from the load a key is compared against fails the
// comparison rather than differing from nothing.
func AssertFingerprinted(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	base := drive(tb, f, fx)
	again := drive(tb, f, fx)
	baseKeys := keysOf(base.report)
	keyed(tb, baseKeys, keysOf(again.report), true,
		"an untouched unit's key is stable across two loads")

	if full := fullUnit(base.report); full != "" {
		shallow := drive(tb, f, fx, func(cfg *load.Config) {
			cfg.Signatures = append(slices.Clone(fx.Signatures), full)
		})
		keyed(tb, map[string][]byte{full: baseKeys[full]}, keysOf(shallow.report), false,
			"the same bytes at two depths key differently")
	}

	versioned, declares := f.(plugin.Versioned)
	assert.True(tb, declares, "the driver already refused a versionless frontend")
	inner := versioned.Version()
	vBase := drive(tb, reversion{Frontend: f, version: inner}, fx)
	vBump := drive(tb, reversion{Frontend: f, version: inner + "+frontendtest"}, fx)
	keyed(tb, keysOf(vBase.report), keysOf(vBump.report), false,
		"a declared version change re-keys every unit")

	cBase := drive(tb, reoption{Frontend: f, options: "probe-a"}, fx)
	cMoved := drive(tb, reoption{Frontend: f, options: "probe-b"}, fx)
	keyed(tb, keysOf(cBase.report), keysOf(cMoved.report), false,
		"a configuration change re-keys every unit")

	reset := drive(tb, f, fx, func(cfg *load.Config) {
		cfg.PluginSet = []byte("frontendtest-moved")
	})
	keyed(tb, baseKeys, keysOf(reset.report), false,
		"the composition's fingerprint folds into every key")

	first := selected(tb, f, fx)[0]
	touched := copyTree(tb, fx.Sources)
	touched[first] = &fstest.MapFile{Data: append(
		slices.Clone(touched[first].Data), '\n',
	)}
	perturbed, err := tryDrive(f, &Fixture{
		Sources: touched, Signatures: fx.Signatures,
	})
	if err == nil {
		unit := unitHolding(base.report, first)
		keyed(tb, map[string][]byte{unit: baseKeys[unit]}, keysOf(perturbed.report), false,
			"a changed read re-keys the unit that read it")
	}
	// A language that refuses the appended byte still proved the
	// read reached its parser; the fold's other parts stand above.
}

// keyed compares each unit's key in one load with the same unit's
// key in another, in path order: equal where same is set, different
// otherwise. A unit the second load lacks is a failure of its own,
// because a key compared with nothing differs from it trivially.
func keyed(tb assert.TB, before, after map[string][]byte, same bool, why string) {
	tb.Helper()

	for _, file := range slices.Sorted(maps.Keys(before)) {
		other, held := after[file]
		if !held {
			tb.Errorf("the unit holding %s is missing from the load it is compared with: %s",
				file, why)
			continue
		}
		assert.Equal(tb, bytes.Equal(before[file], other), same, why)
	}
}

// AssertJailedReads proves the one door from the frontend's side:
// every unit reads its members through the unit, so a unit's key
// moves when its members' bytes move. A frontend reading its
// members any other way — the operating system's filesystem, a
// cache it keeps across loads — keys a unit by bytes it never read
// through the door, and a cache keyed that way serves a stale
// graph. The check loads a copy of the fixture twice, then a copy
// whose every selected file gained a line break, and requires every
// unit of the second load to key differently in the third. The
// kernel's side of the door, a read outside the unit refusing and
// naming the path, is the plugin package's own contract.
func AssertJailedReads(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	files := selected(tb, f, fx)
	tree := copyTree(tb, fx.Sources)
	// The first load is a warm-up: a frontend caching across loads
	// fills its cache here, and serves the second and third from it.
	drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures})
	warm := drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures})

	touched := copyTree(tb, tree)
	for _, file := range files {
		touched[file] = &fstest.MapFile{Data: append(slices.Clone(touched[file].Data), '\n')}
	}
	perturbed, err := tryDrive(f, &Fixture{Sources: touched, Signatures: fx.Signatures})
	if err != nil {
		// A language refusing the appended bytes read them through a
		// door: nothing else can read the copy this check made.
		return
	}
	keyed(tb, keysOf(warm.report), keysOf(perturbed.report), false,
		"every unit whose members changed re-keys, because its parse read them through the unit")
}

// homeOf returns a package declaration's own path, and nothing for
// a symbol outside the model.
func homeOf(s symbol.Symbol) (string, bool) {
	pkg, is := s.(*node.Package)
	if !is {
		return "", false
	}
	return pkg.ID.Package, true
}

// AssertSignatureDepth loads the fixture once full and once under
// its signature roots: the shallow graph's identities are a subset
// of the full graph's, under the same spellings, and the report
// says which units loaded shallow.
func AssertSignatureDepth(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	full := drive(tb, f, fx, func(cfg *load.Config) { cfg.Signatures = nil })
	sig := drive(tb, f, fx)

	for pkg := range sig.graph.ByKind(symbol.KindPackage) {
		for decl := range node.Declarations(pkg) {
			if decl.Identity().IsZero() {
				continue
			}
			_, held := full.graph.Lookup(decl.Identity())
			assert.True(tb, held,
				"the full load is a superset under the same identities")
		}
	}

	shallow := 0
	for _, u := range sig.report.Units {
		if u.Depth == plugin.DepthSignatures {
			shallow++
		}
	}
	assert.True(tb, shallow > 0, "a stated root loads at least one unit shallow")
}

// AssertAttachedDirectives validates every attached instance under
// the fixture's schemas at the suite's stand-in freeze, after the
// resolution step — the same point the workspace validates at. It
// also holds the comment pipeline to its exclusion: a carrier line
// left in a declaration's documentation, with or without the
// carrier mark, is a strip the frontend missed. What it does not
// check is the carrier marker itself, which is each kit's own
// convention.
func AssertAttachedDirectives(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	got := drive(tb, f, fx)

	registry := directive.NewRegistry()
	names := map[directive.Name]bool{}
	for _, s := range fx.Schemas {
		assert.NoError(tb, registry.Register(s), "a fixture schema registers")
		names[s.Canonical()] = true
		names[s.Name] = true
	}
	assert.Empty(tb, registry.Seal(), "the fixture's registry seals")
	keys := meta.NewRegistry()
	if fx.Keys != nil {
		assert.NoError(tb, fx.Keys(keys), "the fixture's keys register")
	}

	attached := 0
	for id, raws := range got.graph.Directives() {
		attached += len(raws)
		decl, held := got.graph.Lookup(id)
		if !held {
			tb.Errorf("directives on %s name a subject the graph does not hold", id)
			continue
		}
		vsink := diag.NewSink()
		directive.Validate(id, raws, registry, keys, nil, vsink)
		for d := range vsink.All() {
			if d.Severity == diag.SeverityError {
				tb.Errorf("an attached instance fails validation: %s %s", d.Code, d.Msg)
			}
		}
		for _, line := range decl.Docs() {
			raw, err := directive.Parse(strings.TrimPrefix(line, plugin.CarrierMark))
			if err == nil && names[raw.Name] {
				tb.Errorf("%s keeps a carrier line in its documentation: %q", id, line)
			}
		}
	}
	assert.True(tb, attached > 0,
		"the fixture declares schemas, so its carriers must attach something")
}

// AssertLinked checks the resolution step's outcome: at least one
// in-graph spelling resolves, every resolved reference targets a
// declaration in the graph, every reference left unresolved — a
// builtin, an external — keeps its spelling, a multi-package
// fixture resolves across its packages, and the tracked reader
// joins the same targets afterwards.
func AssertLinked(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	got := drive(tb, f, fx)

	packages := 0
	var resolved, cross []symbol.Identity
	for pkg := range got.graph.ByKind(symbol.KindPackage) {
		home, is := homeOf(pkg)
		if !is {
			continue
		}
		packages++
		node.Walk(pkg, func(s symbol.Symbol) bool {
			ref, is := s.(*node.TypeRef)
			if !is {
				return true
			}
			if ref.Target.IsZero() {
				if ref.Spelling == "" {
					tb.Errorf("a reference in %s resolves to nothing and spells nothing: "+
						"a builtin or an external keeps its spelling", home)
				}
				return true
			}
			resolved = append(resolved, ref.Target)
			if ref.Target.Package != home {
				cross = append(cross, ref.Target)
			}
			return true
		})
	}
	assert.NotEmpty(tb, resolved,
		"the fixture's in-graph spellings resolve: a Resolve that returns no candidate links nothing")
	for _, target := range resolved {
		_, held := got.graph.Lookup(target)
		assert.True(tb, held, "a resolved reference targets a held declaration")
	}
	if packages < 2 {
		return
	}
	assert.NotEmpty(tb, cross,
		"a multi-package fixture resolving nothing across packages is a Resolve that never answers")
	if len(cross) == 0 {
		return
	}

	reader, err := got.graph.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	_, held := reader.Lookup(cross[0])
	assert.True(tb, held, "the tracked reader joins across the packages")
}

// fullUnit returns the first member of a unit the base load parsed
// full, and nothing when every unit already loads shallow.
func fullUnit(report *load.Report) string {
	for _, u := range report.Units {
		if u.Depth == plugin.DepthFull {
			return u.Files[0]
		}
	}
	return ""
}

// unitHolding returns the first member of the unit holding a file.
func unitHolding(report *load.Report, path string) string {
	for _, u := range report.Units {
		if slices.Contains(u.Files, path) {
			return u.Files[0]
		}
	}
	return path
}

// reversion re-declares the frontend's version and keeps
// everything else the inner's, so a version delta is the one
// difference between two loads. It answers for the options too,
// nil where the inner declares none, so both sides of a comparison
// share one options shape.
type reversion struct {
	plugin.Frontend
	version string
}

// Version returns the wrapper's own.
func (w reversion) Version() string { return w.version }

// Options returns the inner's, and nil where it declares none.
func (w reversion) Options() any {
	if op, is := w.Frontend.(plugin.OptionsProvider); is {
		return op.Options()
	}
	return nil
}

// reoption re-declares the frontend's options and keeps the
// declared version, so a configuration delta is the one difference
// between two loads.
type reoption struct {
	plugin.Frontend
	options any
}

// Version returns the inner's declared version, and nothing for a
// frontend the driver would refuse anyway.
func (w reoption) Version() string {
	if versioned, declares := w.Frontend.(plugin.Versioned); declares {
		return versioned.Version()
	}
	return ""
}

// Options returns the wrapper's own.
func (w reoption) Options() any { return w.options }
