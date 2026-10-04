// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontendtest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
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

// foreignBrand is the brand the ownership and key checks set against
// [Brand].
const foreignBrand output.Brand = "fixture-foreign"

// The suffixes the stamped copies take beside the file they copy.
const (
	ownedSuffix   = "_owned"
	foreignSuffix = "_foreign"
)

// AssertDeterministicParse loads the fixture twice and compares
// what each load recorded: the graphs byte for byte, the attached
// directives and classification stamps, and the findings in report
// order. Reparsing unchanged files yields the same identities,
// which every later comparison by identity depends on.
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

// AssertPositionedDiagnostics checks every finding's address: a
// finding without a position or an origin is a defect in the
// frontend that reported it.
func AssertPositionedDiagnostics(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	got := drive(tb, f, fx)
	for d := range got.sink.All() {
		if d.Pos.File == "" {
			tb.Errorf("finding %s %q has no position", d.Code, d.Msg)
		}
		if d.Origin == "" {
			tb.Errorf("finding %s %q has no origin", d.Code, d.Msg)
		}
	}
}

// AssertClassified checks the claim: every selected file
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
				Order:     meta.Order{Subject: id, Instance: i},
				Pos:       s.Pos,
			})
			assert.NoError(tb, err, "a recorded stamp applies under the fixture's keys")
		}
	}
}

// AssertOwnedExcluded checks the one exclusion the kernel makes: a
// selected file framed under the load's own brand is the
// workspace's output and never enters a unit, while the same file
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
	tree[own] = &fstest.MapFile{Data: framed(tb, f, Brand, own, source)}
	tree[foreign] = &fstest.MapFile{Data: framed(tb, f, foreignBrand, foreign, source)}
	assert.True(tb, load.Match(f.Selection(), own),
		"the stamped copy is beside its source, inside the claim")

	got := drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures, Stores: fx.Stores})
	assert.Equal(tb, got.report.Excluded, []string{own},
		"the load refuses its own output and lists it")
	for _, u := range got.report.Units {
		assert.False(tb, contains(u.Files, own), "no unit contains the refused file")
	}
	loaded := false
	for _, u := range got.report.Units {
		loaded = loaded || contains(u.Files, foreign)
	}
	assert.True(tb, loaded, "another brand's output is ordinary input in a unit")
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
	assert.NoError(tb, err, "the language's syntax frames the file")
	b, err := contract.Stamp(plugin.RenderedFile{
		Path: name, Plugins: []plugin.ID{f.Name()}, Body: source,
	})
	assert.NoError(tb, err, "the source stamps")
	return b
}

// AssertFingerprinted checks that the unit keys are honest: stable
// across two identical loads, and changed by each folded part: a
// read, a depth, a declared version, the options and the brand. The
// model fingerprint is a compiled constant no test can vary. A unit
// missing from the load a key is compared against fails the
// comparison, and never differs from nothing.
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

	rebranded := drive(tb, f, fx, func(cfg *load.Config) { cfg.Brand = foreignBrand })
	keyed(tb, baseKeys, keysOf(rebranded.report), false,
		"the brand folds into every key")

	first := selected(tb, f, fx)[0]
	touched := copyTree(tb, fx.Sources)
	touched[first] = &fstest.MapFile{Data: append(
		slices.Clone(touched[first].Data), '\n',
	)}
	perturbed, err := tryDrive(f, &Fixture{
		Sources: touched, Signatures: fx.Signatures, Stores: fx.Stores,
	})
	if err == nil {
		unit := unitHolding(base.report, first)
		keyed(tb, map[string][]byte{unit: baseKeys[unit]}, keysOf(perturbed.report), false,
			"a changed read re-keys the unit that read it")
	}
	// A language that refuses the appended byte still proved that its
	// parser read the byte. The checks above cover the fold's other
	// parts.
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
			tb.Errorf("the unit containing %s is missing from the load it is compared with: %s",
				file, why)
			continue
		}
		assert.Equal(tb, bytes.Equal(before[file], other), same, why)
	}
}

// AssertJailedReads proves the one door from the frontend's side:
// every unit reads its members through the unit, so the fold of the
// unit's reads moves when its members' bytes move. A unit's key folds
// the digests of its members, so a frontend reading its members any
// other way, such as the operating system's filesystem or a cache it
// keeps across loads, builds a region from bytes its key does not
// cover, and the parse memo serves that region stale. The check loads
// a copy of the fixture twice, then parses each unit the partition
// returned again over a copy whose every selected file gained a line
// break, and requires each unit's door fold to move. A dependency unit
// reads the stores and not the workspace, so the comparison leaves it
// out. The kernel's side of the door, a read outside the unit refusing
// and naming the path, is the plugin package's own contract.
func AssertJailedReads(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	files := selected(tb, f, fx)
	tree := copyTree(tb, fx.Sources)
	// The first load is a warm-up: a frontend caching across loads
	// fills its cache here, and serves the parses after it from it.
	drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures, Stores: fx.Stores})
	warm := drive(tb, f, &Fixture{Sources: tree, Signatures: fx.Signatures, Stores: fx.Stores})

	touched := copyTree(tb, tree)
	for _, file := range files {
		touched[file] = &fstest.MapFile{Data: append(slices.Clone(touched[file].Data), '\n')}
	}
	before := doorFolds(tb, f, warm.report, tree, fx.Stores)
	after, err := tryDoorFolds(f, warm.report, touched, fx.Stores)
	if err != nil {
		// A language refusing the appended bytes read them through a
		// door: nothing else can read the copy this check made.
		return
	}
	keyed(tb, workspaceOnly(warm.report, before), after, false,
		"every unit whose members changed folds the new bytes, because its parse read them through the unit")
}

// doorFolds parses every unit a load reported again, each through a
// fresh unit of its own over the tree and its stores, and returns each
// unit's door fold by its first member: the fold of every read its
// parse made through the unit. A parse that read its members around
// the door folds none of their bytes, so its fold does not move when
// the members' bytes do.
func doorFolds(
	tb assert.TB, f plugin.Frontend, report *load.Report, sources fs.FS, stores map[string]fs.FS,
) map[string][]byte {
	tb.Helper()

	out, err := tryDoorFolds(f, report, sources, stores)
	assert.NoError(tb, err, "every unit parses again")
	return out
}

// tryDoorFolds is [doorFolds], returning a parse's own error.
func tryDoorFolds(
	f plugin.Frontend, report *load.Report, sources fs.FS, stores map[string]fs.FS,
) (map[string][]byte, error) {
	tree := fixtureTree{FS: sources, stores: stores}
	out := make(map[string][]byte, len(report.Units))
	for _, u := range report.Units {
		door := &foldingTree{tree: tree, sum: sha256.New()}
		src := plugin.NewSourceUnit(u.Files, door, u.Depth, f.Syntax(), string(Brand), diag.NewSink(), f.Name())
		if err := f.Parse(context.Background(), src); err != nil {
			return nil, err
		}
		out[u.Files[0].Path] = door.sum.Sum(nil)
	}
	return out, nil
}

// foldingTree is the tree a unit reads through in the door check: a
// tree with its stores, folding the path and the bytes of every file
// read whole through it, in read order, each behind its length, so no
// byte of one field can pass for the boundary of the next. A unit
// reads its files whole through [plugin.ReadFile], which resolves a
// qualified path through Store, and a store's reads fold into the same
// sum.
type foldingTree struct {
	tree fs.FS
	sum  hash.Hash
}

// Open opens a file of the tree. A unit reads a file's bytes through
// ReadFile, so Open folds nothing.
func (t *foldingTree) Open(name string) (fs.File, error) { return t.tree.Open(name) }

// ReadFile returns one file's bytes and folds its path and its bytes.
// A failed read returns the tree's error and folds nothing.
func (t *foldingTree) ReadFile(name string) ([]byte, error) {
	b, err := fs.ReadFile(t.tree, name)
	if err != nil {
		return nil, err
	}
	foldField(t.sum, []byte(name))
	foldField(t.sum, b)
	return b, nil
}

// Store returns one of the tree's stores, folding its reads into the
// same sum, and false for a store the tree does not provide.
func (t *foldingTree) Store(name string) (fs.FS, bool) {
	stores, provides := t.tree.(plugin.StoreFS)
	if !provides {
		return nil, false
	}
	store, held := stores.Store(name)
	if !held {
		return nil, false
	}
	return &foldingTree{tree: store, sum: t.sum}, true
}

// foldField writes one field into a sum behind its length.
func foldField(h hash.Hash, field []byte) {
	var size [binary.MaxVarintLen64]byte
	h.Write(binary.AppendUvarint(size[:0], uint64(len(field))))
	h.Write(field)
}

// workspaceOnly keeps the entries of the units the partition returned,
// leaving out the dependency units, whose reads are the stores' and
// not the workspace's.
func workspaceOnly(report *load.Report, byFirst map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(byFirst))
	for _, u := range report.Units {
		if u.Round == 0 {
			out[u.Files[0].Path] = byFirst[u.Files[0].Path]
		}
	}
	return out
}

// fixtureTree is a fixture's tree with its stores beside it, which a
// unit parsed outside a load resolves a qualified path through.
type fixtureTree struct {
	fs.FS
	stores map[string]fs.FS
}

// Store returns one of the fixture's stores.
func (t fixtureTree) Store(name string) (fs.FS, bool) {
	s, held := t.stores[name]
	return s, held
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
// its signature roots. The shallow graph's identities are a subset
// of the full graph's, under the same spellings, and the report
// states which units loaded shallow. Every identity the fixture
// lists in [Fixture.Dropped] is in the full graph and absent from
// the shallow one, so a frontend that ignores depth fails a fixture
// that lists one.
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

	// A dependency unit loads shallow under any root, so only the
	// units the partition returned count.
	shallow := 0
	for _, u := range sig.report.Units {
		if u.Round == 0 && u.Depth == plugin.DepthSignatures {
			shallow++
		}
	}
	assert.True(tb, shallow > 0, "a stated root loads at least one unit shallow")

	for _, id := range fx.Dropped {
		_, loaded := full.graph.Lookup(id)
		assert.True(tb, loaded, "a listed identity loads at full depth: "+id.String())
		_, kept := sig.graph.Lookup(id)
		assert.False(tb, kept, "a signature-only load drops a listed identity: "+id.String())
	}
}

// AssertAttachedDirectives validates every attached instance under
// the fixture's schemas at the freeze the suite runs in the
// workspace's place, after the resolution step, which is the point
// the workspace validates at. It also checks the comment pipeline's
// exclusion: a carrier line left in a declaration's documentation,
// under any of the brand's marks or without one, is a strip the
// frontend missed.
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
			tb.Errorf("directives on %s name a subject the graph does not contain", id)
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
			payload := line
			if _, cut, isCarrier := plugin.CutCarrier(line, string(Brand)); isCarrier {
				payload = cut
			}
			raw, err := directive.Parse(payload)
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
// declaration in the graph, every reference left unresolved keeps
// its spelling, a multi-package fixture resolves across its
// packages, and the tracked reader joins the same targets
// afterwards. A builtin and an external are the references a load
// leaves unresolved.
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
		assert.True(tb, held, "a resolved reference targets a declaration in the graph")
	}
	if packages < 2 {
		return
	}
	assert.NotEmpty(tb, cross,
		"a multi-package fixture resolving nothing across packages is a Resolve that never returns a candidate")
	if len(cross) == 0 {
		return
	}

	reader, err := got.graph.Reader(store.NewReadSet(), nil)
	assert.NoError(tb, err, "the sealed graph hands out a reader")
	_, held := reader.Lookup(cross[0])
	assert.True(tb, held, "the tracked reader joins across the packages")
}

// AssertDependencies checks the dependency rounds of a frontend in
// the [plugin.Dependent] role: at least one unit arrives from a round,
// and a changed byte in the first member of the first dependency unit
// moves the door fold of that unit, because its parse read the member
// through the unit. The kernel parses every dependency unit at
// [plugin.DepthSignatures] and refuses a member the selection claims,
// so the check leaves both to the load. It copies the store it
// changes, so a fixture's stores are small trees and never a
// machine's module cache.
func AssertDependencies(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	if _, dependent := f.(plugin.Dependent); !dependent {
		tb.Errorf("the frontend is not in the dependent role, so no dependency round runs")
		return
	}
	got := drive(tb, f, fx)
	var first string
	for _, u := range got.report.Units {
		if u.Round > 0 {
			first = u.Files[0].Path
			break
		}
	}
	if first == "" {
		tb.Errorf("no dependency round returned a unit")
		return
	}

	sources, stores := fx.Sources, fx.Stores
	if store, inner, qualified := plugin.CutStorePath(first); qualified {
		copied := copyTree(tb, fx.Stores[store])
		copied[inner] = &fstest.MapFile{Data: append(slices.Clone(copied[inner].Data), '\n')}
		stores = maps.Clone(fx.Stores)
		stores[store] = copied
	} else {
		copied := copyTree(tb, fx.Sources)
		copied[first] = &fstest.MapFile{Data: append(slices.Clone(copied[first].Data), '\n')}
		sources = copied
	}
	before := doorFolds(tb, f, got.report, fx.Sources, fx.Stores)
	after, err := tryDoorFolds(f, got.report, sources, stores)
	if err != nil {
		// A language refusing the appended byte read it through the
		// unit: nothing else can read the copy this check made.
		return
	}
	keyed(tb, map[string][]byte{first: before[first]}, after, false,
		"a changed byte in a dependency member moves the door fold of the unit that read it")
}

// AssertReexports checks the resolution step's following of
// re-exports for a frontend in the [plugin.Exporter] role: every
// identity the fixture lists in [Fixture.Reexported], a declaration
// the fixture's references name only through a re-export, is the
// target of a reference in the graph.
func AssertReexports(tb assert.TB, setup Setup) {
	tb.Helper()

	f, fx := setup(tb)
	if _, exports := f.(plugin.Exporter); !exports {
		tb.Errorf("the frontend is not in the exporter role, so no re-export is followed")
		return
	}
	if len(fx.Reexported) == 0 {
		tb.Errorf("the fixture lists no declaration a re-export publishes, so nothing is checked")
		return
	}
	got := drive(tb, f, fx)
	targets := map[symbol.Identity]bool{}
	for pkg := range got.graph.ByKind(symbol.KindPackage) {
		node.Walk(pkg, func(s symbol.Symbol) bool {
			if ref, is := s.(*node.TypeRef); is && !ref.Target.IsZero() {
				targets[ref.Target] = true
			}
			return true
		})
	}
	for _, id := range fx.Reexported {
		assert.True(tb, targets[id], "a reference through a re-export targets "+id.String())
	}
}

// fullUnit returns the first member of a unit the base load parsed
// full, and nothing when every unit already loads shallow.
func fullUnit(report *load.Report) string {
	for _, u := range report.Units {
		if u.Depth == plugin.DepthFull {
			return u.Files[0].Path
		}
	}
	return ""
}

// unitHolding returns the first member of the unit that contains a
// file.
func unitHolding(report *load.Report, path string) string {
	for _, u := range report.Units {
		if contains(u.Files, path) {
			return u.Files[0].Path
		}
	}
	return path
}

// contains reports whether a unit's members include a path.
func contains(members []plugin.SourceRef, path string) bool {
	return slices.ContainsFunc(members, func(ref plugin.SourceRef) bool { return ref.Path == path })
}

// reversion re-declares the frontend's version and keeps
// everything else the inner's, so a version delta is the one
// difference between two loads. It returns the options too, nil
// where the inner declares none, so both sides of a comparison
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
