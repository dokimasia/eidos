// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"bytes"
	"slices"
	"testing/fstest"

	"go.dokimi.dev/assert"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/load"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// AssertDeterministicParse loads the fixture twice and compares
// the graphs byte for byte: reparsing unchanged files yields the
// same identities, or diff-by-identity later stands on sand.
func AssertDeterministicParse(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	one := drive(tb, f, fx)
	two := drive(tb, f, fx)
	assert.True(tb, bytes.Equal(encoded(tb, one.graph), encoded(tb, two.graph)),
		"two parses of one fixture encode identically")
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
// nothing drops in silence, and the recorded classification stamps
// apply cleanly under the fixture's keys, the way the workspace
// run applies them.
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
	if !stamped {
		return
	}
	if fx.Keys == nil {
		tb.Errorf("the fixture stamps and declares no keys to apply them under")
		return
	}
	registry := meta.NewRegistry()
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

// AssertFingerprinted holds the unit keys honest: stable across
// two identical loads, and changed by each folded part — a read, a
// depth, a declared version, the options, the plugin set. The
// model fingerprint is a compiled constant no test can vary.
func AssertFingerprinted(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	base := drive(tb, f, fx)
	again := drive(tb, f, fx)
	baseKeys, againKeys := keysOf(base.report), keysOf(again.report)
	for file, key := range baseKeys {
		assert.True(tb, bytes.Equal(key, againKeys[file]),
			"an untouched unit's key is stable across two loads")
	}

	if full := fullUnit(base.report); full != "" {
		shallow := drive(tb, f, fx, func(cfg *load.Config) {
			cfg.Signatures = append(slices.Clone(fx.Signatures), full)
		})
		assert.False(tb, bytes.Equal(baseKeys[full], keysOf(shallow.report)[full]),
			"the same bytes at two depths key differently")
	}

	versioned, declares := f.(plugin.Versioned)
	assert.True(tb, declares, "the driver already refused a versionless frontend")
	inner := versioned.Version()
	vBase := drive(tb, reversion{Frontend: f, version: inner}, fx)
	vBump := drive(tb, reversion{Frontend: f, version: inner + "+frontendtest"}, fx)
	for file, key := range keysOf(vBase.report) {
		assert.False(tb, bytes.Equal(key, keysOf(vBump.report)[file]),
			"a declared version change re-keys every unit")
	}

	cBase := drive(tb, reoption{Frontend: f, options: "probe-a"}, fx)
	cMoved := drive(tb, reoption{Frontend: f, options: "probe-b"}, fx)
	for file, key := range keysOf(cBase.report) {
		assert.False(tb, bytes.Equal(key, keysOf(cMoved.report)[file]),
			"a configuration change re-keys every unit")
	}

	reset := drive(tb, f, fx, func(cfg *load.Config) {
		cfg.PluginSet = []byte("frontendtest-moved")
	})
	for file, key := range baseKeys {
		assert.False(tb, bytes.Equal(key, keysOf(reset.report)[file]),
			"the composition's fingerprint folds into every key")
	}

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
		assert.False(tb, bytes.Equal(baseKeys[unit], keysOf(perturbed.report)[unit]),
			"a changed read re-keys the unit that read it")
	}
	// A language that refuses the appended byte still proved the
	// read reached its parser; the fold's other parts stand above.
}

// AssertJailedReads proves the one door: a unit's read outside its
// files and shared inputs refuses, naming the path.
func AssertJailedReads(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	files := selected(tb, f, fx)
	u := plugin.NewSourceUnit(
		[]plugin.SourceRef{{Path: files[0]}}, fx.Sources, plugin.DepthFull,
		f.Syntax(), diag.NewSink(), f.Name(),
	)
	_, err := u.Read(files[0])
	assert.NoError(tb, err, "a member reads")

	outside := "frontendtest/outside-the-unit"
	if len(files) > 1 {
		outside = files[1]
	}
	_, err = u.Read(outside)
	assert.HasError(tb, err, "a path outside the unit refuses")
	assert.Contains(tb, err.Error(), outside, "naming the path")
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
// left in a declaration's documentation is a strip the frontend
// missed. What it does not check is the carrier marker itself,
// which is each kit's own convention.
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
		directive.Validate(id, raws, registry, keys, vsink)
		for d := range vsink.All() {
			if d.Severity == diag.SeverityError {
				tb.Errorf("an attached instance fails validation: %s %s", d.Code, d.Msg)
			}
		}
		for _, line := range decl.Docs() {
			raw, err := directive.Parse(line)
			if err == nil && names[raw.Name] {
				tb.Errorf("%s keeps a carrier line in its documentation: %q", id, line)
			}
		}
	}
	assert.True(tb, attached > 0,
		"the fixture declares schemas, so its carriers must attach something")
}

// AssertLinked holds the resolution step's outcome: every resolved
// reference targets a declaration the graph holds, a multi-package
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
			if !is || ref.Target.IsZero() {
				return true
			}
			resolved = append(resolved, ref.Target)
			if ref.Target.Package != home {
				cross = append(cross, ref.Target)
			}
			return true
		})
	}
	for _, target := range resolved {
		_, held := got.graph.Lookup(target)
		assert.True(tb, held, "a resolved reference targets a held declaration")
	}
	if packages < 2 {
		return
	}
	assert.NotEmpty(tb, cross,
		"a multi-package fixture resolving nothing across packages is a Resolve that never answers")

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
