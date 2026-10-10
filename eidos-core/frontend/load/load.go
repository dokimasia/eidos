// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// DuplicateDeclaration reports two declarations spelling one
// identity: the unit's own source broken mid-edit, or a
// platform-variant collision the language must resolve. The first
// is kept and the second leaves every index.
var DuplicateDeclaration = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  37,
	Meaning: "two declarations spell one identity, and the first is kept",
})

// AmbiguousReference reports a type reference whose candidates in
// one shadowing tier match more than one declaration. The first
// match is the target: degradation a reader can ask about, not a
// failure.
var AmbiguousReference = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  38,
	Meaning: "a type reference resolves to more than one declaration",
})

// UnplacedNeed reports an import a dependent frontend places in no
// dependency unit: an import the language's toolchain rejects, or a
// store, a classpath entry or a requirement the composition lacks.
// Every reference into the import keeps its spelling alone.
var UnplacedNeed = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  47,
	Meaning: "an import places in no dependency unit, and its references keep their spellings",
})

// Config is one load's inputs.
type Config struct {
	// FS is the workspace tree every read resolves in.
	FS fs.FS

	// Frontends are the registered frontends, in composition order.
	// Each must declare a version through [plugin.Versioned],
	// because every unit key folds it.
	Frontends []plugin.Frontend

	// Sink collects the load's findings.
	Sink *diag.Sink

	// Signatures are the directory roots loaded signature-only: a
	// unit with any member under one loads at
	// [plugin.DepthSignatures], everything else at
	// [plugin.DepthFull].
	Signatures []string

	// Brand is the composition's brand. Every unit reads its
	// carriers under the brand's marks, and a claimed file with the
	// brand's provenance trailer is the workspace's own output and
	// does not load: outputs are never inputs, and the exclusion
	// runs before anything partitions. A file another brand stamped
	// is ordinary input. The walk skips the brand's state directory.
	// The load refuses a brand outside [output.Brand.Valid].
	Brand output.Brand

	// Stores maps each store's name to its read-only tree outside the
	// workspace, which dependency units read: a Go module cache, a Go
	// standard library, a JDK's ct.sym, a Maven or Gradle cache. The
	// composition opens the trees, and no key folds where they are.
	// The load refuses a name [plugin.ValidStoreName] refuses and a
	// name without a tree.
	Stores map[string]fs.FS

	// Prior is the last committed load's record, which the fingerprint
	// gate compares the tree against, and nil for a cold load.
	Prior Prior

	// Memo is the parse memo, and nil for none. A warm load consults it
	// for a unit the record cannot keep, and every load records the
	// units it parsed.
	Memo Memo
}

// Report is what one load records beside the graph.
type Report struct {
	// Units has one entry per unit, in splice order.
	Units []UnitReport

	// Excluded lists the claimed files the load refused as the
	// workspace's own outputs, sorted by path.
	Excluded []string

	// Files are the gate's records of every file the load statted,
	// sorted by path: the workspace's files and the store files the
	// units read.
	Files []FileRecord

	// Moved lists the workspace files whose record does not prove them
	// unchanged, sorted by path: a file the record lacks, a file whose
	// stat moved, and a racily clean file. Vanished lists the recorded
	// workspace files that the walk did not find, sorted by path. Was
	// lists the record's records of the moved files it lists and of the
	// vanished files, sorted by path. All three are nil for a cold load. A
	// warm run reads them to find the generated files that changed on disk
	// since the commit that wrote them, and the directories and packages
	// whose files changed.
	Moved, Vanished []string
	Was             []FileRecord

	// Doors are each frontend's door records: its partition, then its
	// dependency rounds in order.
	Doors map[plugin.ID][]DoorRecord

	// Anchor is the instant the gate's sweep began, less two seconds.
	// A record whose modification time does not precede it is racily
	// clean for the next load.
	Anchor time.Time

	// Statted and Hashed count the files the gate statted and hashed.
	Statted, Hashed int

	// Reparsed counts the units the load parsed though their keys are
	// the recorded ones: a unit a parsed or restored unit of its package
	// couples to, and a unit whose bindings a link needed.
	Reparsed int

	// Changes is what the load changed against the record it read, and
	// nil for a cold load.
	Changes *Changes

	// decoded counts the regions the load and its graph decoded from the
	// record.
	decoded *atomic.Int64
}

// Decoded returns how many regions the load and its graph have decoded
// from the record so far. A graph decodes a kept unit's region the
// first time a read needs it, so the count grows while the graph is
// read.
func (r *Report) Decoded() int {
	if r.decoded == nil {
		return 0
	}
	return int(r.decoded.Load())
}

// UnitReport is one unit's record.
type UnitReport struct {
	// Frontend parsed the unit.
	Frontend plugin.ID
	// Files are the unit's members, in partition order, each with its
	// shared inputs.
	Files []plugin.SourceRef
	// Depth is what the unit loaded at.
	Depth plugin.Depth
	// Key is the unit's finished key, folded as the package
	// documentation states.
	Key []byte
	// Round is zero for a unit the partition returned, and the
	// dependency round's number for a unit a [plugin.Dependent]
	// frontend returned.
	Round int
	// From states where the load took the unit's region from.
	From From
	// Restorable reports whether a parse memo can restore the unit. It is
	// false for a unit of a frontend that implements [plugin.Importer],
	// because the unit's references need its parse to import their
	// targets.
	Restorable bool
	// Region is the unit's region where the load built or changed it:
	// every unit it parsed or restored from the memo, and every kept
	// unit whose references it selected again into other targets. It is
	// nil for a unit the load kept from the last generation unchanged.
	Region *store.Region
	// Imports are the imports the unit's files name, sorted by path.
	Imports []Import
}

// unit is one compilation unit on its way through the pipeline.
type unit struct {
	frontend plugin.Frontend
	files    []plugin.SourceRef
	depth    plugin.Depth
	// door is the fold of the record of the door that shaped the unit:
	// the partition's, shared per frontend, or the dependency round's,
	// shared per round.
	door    []byte
	config  []byte // the frontend's options, canonically encoded
	version string
	round   int
	key     []byte
	// record is the last load's record of the unit, matched by its
	// first member, and nil for a unit the history lacks.
	record *UnitRecord
	// from is where the load takes the unit's region from. A parsed
	// unit has its sink and its source unit, and the region the load
	// builds from them. A kept unit's region decodes from the history on
	// first use, and a restored unit's is the memo's.
	from   From
	sink   *diag.Sink
	src    *plugin.SourceUnit
	region *store.Region
	// relinked reports that a kept or restored unit's references select
	// other targets than its region records, so its region changed.
	relinked bool
	// imports are the imports the unit's files name.
	imports []Import
	// linked are the findings the splice and the assignment reported
	// about the unit's declarations, and resolved the records of the
	// references the link resolved through the frontend, by reference.
	linked   []diag.Diag
	resolved map[*node.TypeRef]store.Link
}

// parsed reports whether the load parsed the unit.
func (u *unit) parsed() bool { return u.from == FromParse }

// Load drives every frontend over the tree: the gate, select,
// partition, key, keep or restore or parse, load the dependencies,
// splice, resolve, and build one region per parsed unit. It returns a
// graph [store.Sealed] returns over the units' regions, and the report.
// A nil graph means nothing loaded, and the error states why.
//
// A load with [Config.Prior] set keeps every unit whose key the record
// states, restores a unit whose key the memo contains, and parses the
// rest, together with every kept unit they couple to. A load without a
// prior parses every unit whose key the memo lacks.
//
// Error modes: a configuration [check] refuses, a frontend's error, a
// unit or a dependency round that breaks the contract, and the prior's
// error for a record that does not read whole.
func Load(ctx context.Context, cfg Config) (*store.Graph, *Report, error) {
	if err := check(cfg); err != nil {
		return nil, nil, err
	}
	l, err := newLoader(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return l.load()
}

// loader is one load: its configuration and tree, the gate, the
// history it compares the tree with, the doors it recorded, and its
// counts of the regions it decoded and the units it parsed though the
// history kept them.
type loader struct {
	ctx      context.Context
	cfg      Config
	tree     storeTree
	gate     *gate
	history  *history
	doors    map[plugin.ID][]DoorRecord
	decoded  *atomic.Int64
	reparsed int
}

// newLoader returns the load of one configuration: its gate over the
// tree, anchored now, and the history its prior records.
//
// Error modes: the prior's error for a record that does not read whole.
func newLoader(ctx context.Context, cfg Config) (*loader, error) {
	tree := storeTree{FS: cfg.FS, stores: cfg.Stores}
	g, err := newGate(cfg, tree, time.Now())
	if err != nil {
		return nil, err
	}
	decoded := &atomic.Int64{}
	h, err := readHistory(cfg.Prior, decoded)
	if err != nil {
		return nil, err
	}
	return &loader{
		ctx: ctx, cfg: cfg, tree: tree, gate: g, history: h,
		doors: map[plugin.ID][]DoorRecord{}, decoded: decoded,
	}, nil
}

// load runs the load's steps and returns its graph and its report.
func (l *loader) load() (*store.Graph, *Report, error) {
	inputs, excluded, claimed, err := survey(l.cfg, l.gate)
	if err != nil {
		return nil, nil, err
	}
	units, err := l.unitsOf(inputs, claimed)
	if err != nil {
		return nil, nil, err
	}
	removed := l.removed(units)

	var parsed []*unit
	for _, u := range units {
		if u.parsed() {
			parsed = append(parsed, u)
			for d := range u.sink.All() {
				l.cfg.Sink.Report(d)
			}
		}
	}
	packages, scopes := splice(parsed, l.cfg.Sink)
	ix := assign(packages, l.cfg.Sink)
	ix.kept = newKeptIndex(l, units)
	w := link(packages, scopes, ix, l.cfg.Sink)
	delta, err := l.delta(units, removed)
	if err == nil {
		err = l.relink(w, units, delta)
	}
	if err != nil {
		return nil, nil, err
	}

	report := &Report{
		Units:    make([]UnitReport, len(units)),
		Excluded: excluded,
		Doors:    l.doors,
		Anchor:   l.gate.anchor,
		decoded:  l.decoded,
	}
	for i, u := range units {
		if u.parsed() {
			u.region = regionOf(u, ix)
			if l.cfg.Memo != nil {
				l.cfg.Memo.Put(u.key, u.region)
			}
		}
		report.Units[i] = u.report()
		l.declare(u)
		l.replay(u)
	}
	if !l.history.cold() {
		if report.Changes, err = l.changes(units, removed); err != nil {
			return nil, nil, err
		}
	}
	report.Files = l.gate.records()
	report.Moved, report.Vanished, report.Was = l.gate.changed()
	report.Statted, report.Hashed = l.gate.statted, l.gate.hashed
	report.Reparsed = l.reparsed
	return store.Sealed(newSource(l.history, units)), report, nil
}

// survey walks the tree through the gate, matches every frontend's
// claim over it, and judges the claimed files. It returns the inputs
// per frontend, the outputs it excluded, and every claimed path.
func survey(cfg Config, g *gate) ([][]string, []string, map[string]bool, error) {
	if err := g.walk(); err != nil {
		return nil, nil, nil, err
	}
	claims, err := claim(cfg.Frontends, g.walked)
	if err != nil {
		return nil, nil, nil, err
	}
	inputs, excluded, err := g.judge(claims)
	if err != nil {
		return nil, nil, nil, err
	}
	claimed := make(map[string]bool, len(g.walked))
	for _, paths := range claims {
		for _, path := range paths {
			claimed[path] = true
		}
	}
	return inputs, excluded, claimed, nil
}

// unitsOf partitions the inputs into units, keys each, keeps, restores
// or parses each, loads the dependencies, and parses every kept unit
// the parsed ones couple to, recording every door. It returns the units
// in splice order.
//
// Error modes: the error of a frontend, of a door that breaks the
// contract, and of a record that does not read whole.
func (l *loader) unitsOf(inputs [][]string, claimed map[string]bool) ([]*unit, error) {
	units, err := l.partitionAll(inputs)
	if err == nil {
		err = keyAll(units, l.gate, l.cfg.Brand)
	}
	if err == nil {
		err = l.classify(units)
	}
	if err != nil {
		return nil, err
	}
	deps, err := l.dependencies(claimed, units)
	if err != nil {
		return nil, err
	}
	units = append(units, deps...)
	slices.SortFunc(units, byFirstMember)
	return units, l.settle(units)
}

// classify takes each unit's region from the history where the history
// records the unit's key, from the memo where the memo contains the key,
// and from a parse otherwise, and parses the units that parse. A unit of
// a language whose frontend is a [plugin.Importer] does not restore,
// because its references need its parse to import their targets.
//
// Error modes: the error of a frontend's parse.
func (l *loader) classify(units []*unit) error {
	var parse []*unit
	for _, u := range units {
		u.record = l.history.first(u.files[0].Path)
		if u.record != nil && bytes.Equal(u.record.Key, u.key) {
			u.from = FromGeneration
			continue
		}
		if l.restore(u) {
			u.from = FromMemo
			continue
		}
		u.from = FromParse
		parse = append(parse, u)
	}
	return parseAll(l.ctx, l.cfg, l.tree, parse)
}

// restore takes a unit's region from the memo, and reports whether the
// memo contained the unit's key.
func (l *loader) restore(u *unit) bool {
	if l.cfg.Memo == nil {
		return false
	}
	if _, imports := u.frontend.(plugin.Importer); imports {
		return false
	}
	r, hit := l.cfg.Memo.Get(u.key)
	if hit {
		u.region = r
	}
	return hit
}

// declare records the package each file of a unit is declared in: a
// kept unit's from its record's summary, without a decode.
func (l *loader) declare(u *unit) {
	if u.region == nil {
		for _, f := range u.record.Summary.Files {
			l.gate.declare(f.Pkg, f.Path)
		}
		return
	}
	for _, p := range u.region.Packages {
		for _, f := range p.Files {
			l.gate.declare(p.ID, f.Path)
		}
	}
}

// replay reports a kept or restored unit's findings into the load's
// sink: what its region records, its links' findings included, and a
// kept unit's from its record, without a decode. A parsed unit's
// findings reported as the load made them.
func (l *loader) replay(u *unit) {
	switch {
	case u.parsed():
	case u.region == nil:
		for _, d := range u.record.Findings {
			l.cfg.Sink.Report(d)
		}
	default:
		for _, d := range u.region.Findings {
			l.cfg.Sink.Report(d)
		}
		for _, link := range u.region.Links {
			for _, d := range link.Findings {
				l.cfg.Sink.Report(d)
			}
		}
	}
}

// report returns a unit's record in the load's report. A kept unit the
// load did not relink reports no region.
func (u *unit) report() UnitReport {
	_, imports := u.frontend.(plugin.Importer)
	r := UnitReport{
		Frontend:   u.frontend.Name(),
		Files:      u.files,
		Depth:      u.depth,
		Key:        u.key,
		Round:      u.round,
		From:       u.from,
		Restorable: !imports,
		Imports:    u.importList(),
	}
	if u.from != FromGeneration || u.relinked {
		r.Region = u.region
	}
	return r
}

// check refuses a configuration the load cannot run: no tree, no
// sink, a brand outside the grammar, a frontend without a version, or
// a store it cannot address.
func check(cfg Config) error {
	if cfg.FS == nil {
		return errors.New("load: no tree to read")
	}
	if cfg.Sink == nil {
		return errors.New("load: no sink to report into")
	}
	if !cfg.Brand.Valid() {
		return fmt.Errorf(
			"load: %q is not a brand, and carriers and outputs are read under the composition's brand",
			string(cfg.Brand),
		)
	}
	for _, f := range cfg.Frontends {
		if _, versioned := f.(plugin.Versioned); !versioned {
			return fmt.Errorf("load: frontend %s declares no version, which every unit key folds", f.Name())
		}
	}
	return checkStores(cfg.Stores)
}

// claim matches every frontend's selection over the tree, in
// composition order. Two frontends claiming one file is a
// composition defect and refuses naming both: selection claims
// partition the tree, and an overlap resolved by splice order
// would resolve by accident.
func claim(frontends []plugin.Frontend, files []string) ([][]string, error) {
	claimed := make(map[string]plugin.ID, len(files))
	out := make([][]string, len(frontends))
	var segments []string
	for i, f := range frontends {
		selection := f.Selection()
		for _, pattern := range selection {
			if err := checkPattern(pattern); err != nil {
				return nil, fmt.Errorf("load: frontend %s: %w", f.Name(), err)
			}
		}
		m := compile(selection)
		for _, path := range files {
			segments = split(segments, path)
			if !m.claims(segments) {
				continue
			}
			if by, taken := claimed[path]; taken {
				return nil, fmt.Errorf(
					"load: %s is claimed by %s and by %s, and selection claims partition the tree",
					path, by, f.Name(),
				)
			}
			claimed[path] = f.Name()
			out[i] = append(out[i], path)
		}
	}
	return out, nil
}

// partitionAll partitions each frontend's inputs into units through a
// recorded door, or keeps the history's partition where it would return
// the same units, records each frontend's door, checks the partition
// contract, and fixes the splice order: units sorted by their first
// file's path, which is unique because claims do not overlap.
//
// Error modes: the error of a frontend's partition, of a partition that
// breaks the contract, of options the canonical encoding cannot see
// whole, and of a record that does not read whole.
func (l *loader) partitionAll(inputs [][]string) ([]*unit, error) {
	var units []*unit
	for i, f := range l.cfg.Frontends {
		if len(inputs[i]) == 0 {
			continue
		}
		record, kept, err := l.recordedPartition(f, inputs[i])
		if err != nil {
			return nil, err
		}
		if !kept {
			if record, err = l.partition(f, inputs[i]); err != nil {
				return nil, err
			}
		}
		config, err := encodeOptions(f)
		if err != nil {
			return nil, err
		}
		l.doors[f.Name()] = append(l.doors[f.Name()], record)
		door := record.fold()
		versioned, _ := f.(plugin.Versioned) // asserted when the load began
		version := versioned.Version()
		for _, part := range record.Units {
			units = append(units, &unit{
				frontend: f,
				files:    part,
				depth:    depthOf(part, l.cfg.Signatures),
				door:     door,
				config:   config,
				version:  version,
			})
		}
	}
	slices.SortFunc(units, byFirstMember)
	return units, nil
}

// partition runs a frontend's partition over its claimed inputs through
// a recorded door, and checks the partition contract.
//
// Error modes: the frontend's error, wrapped, and a partition that
// breaks the contract.
func (l *loader) partition(f plugin.Frontend, inputs []string) (DoorRecord, error) {
	refs := make([]plugin.SourceRef, len(inputs))
	for j, path := range inputs {
		refs[j] = plugin.SourceRef{Path: path}
	}
	reader := &recordingReader{fsys: l.tree, gate: l.gate}
	parts, err := f.Partition(l.ctx, refs, reader)
	if err != nil {
		return DoorRecord{}, fmt.Errorf("load: partition %s: %w", f.Name(), err)
	}
	if err := checkPartition(f.Name(), inputs, parts); err != nil {
		return DoorRecord{}, err
	}
	return DoorRecord{Reads: reader.reads, Units: parts}, nil
}

// recordedPartition returns the history's partition of a frontend, and
// reports true, where the frontend claims the files it claimed then and
// everything the partition read reads the same, so the partition would
// return the same units.
//
// Error modes: the error of a record that does not read whole, and of a
// file the partition read that fails to read for a cause other than its
// absence.
func (l *loader) recordedPartition(f plugin.Frontend, inputs []string) (DoorRecord, bool, error) {
	doors, err := l.history.doorsOf(f.Name())
	if err != nil {
		return DoorRecord{}, false, err
	}
	at := slices.IndexFunc(doors, func(d DoorRecord) bool { return d.Round == 0 })
	if at < 0 || !sameMembers(doors[at].Units, inputs) {
		return DoorRecord{}, false, nil
	}
	for _, r := range doors[at].Reads {
		if same, err := unchangedRead(l.gate, r); err != nil || !same {
			return DoorRecord{}, false, err
		}
	}
	return doors[at], true, nil
}

// sameMembers reports whether the members of a partition's units are
// exactly the claimed paths.
func sameMembers(parts [][]plugin.SourceRef, claimed []string) bool {
	members := make([]string, 0, len(claimed))
	for _, part := range parts {
		for _, ref := range part {
			members = append(members, ref.Path)
		}
	}
	slices.Sort(members)
	return slices.Equal(members, slices.Sorted(slices.Values(claimed)))
}

// keyAll folds every unit's key from the gate's digests. The units'
// folds share one buffer, so a unit allocates its key alone once the
// buffer holds the longest fold.
func keyAll(units []*unit, g *gate, brand output.Brand) error {
	var fold []byte
	for _, u := range units {
		var err error
		if fold, err = appendFold(fold[:0], u, g, brand); err != nil {
			return err
		}
		u.key = unitKey(fold)
	}
	return nil
}

// byFirstMember orders units by their first member's path, the
// splice order. Qualified paths sort among workspace paths, and the
// order is total because no file is a member of two units.
func byFirstMember(a, b *unit) int {
	return strings.Compare(a.files[0].Path, b.files[0].Path)
}

// checkPartition checks a frontend's partition against the
// contract: every claimed file a member of exactly one unit, no
// member outside the claim, no empty unit.
func checkPartition(name plugin.ID, claimed []string, parts [][]plugin.SourceRef) error {
	counts := make(map[string]int, len(claimed))
	for _, path := range claimed {
		counts[path] = 0
	}
	for _, part := range parts {
		if len(part) == 0 {
			return fmt.Errorf("load: partition %s: a unit with no members has nothing to parse", name)
		}
		for _, ref := range part {
			n, ours := counts[ref.Path]
			if !ours {
				return fmt.Errorf(
					"load: partition %s: %s is a member the selection never claimed",
					name, ref.Path,
				)
			}
			counts[ref.Path] = n + 1
		}
	}
	for _, path := range claimed {
		switch counts[path] {
		case 1:
		case 0:
			return fmt.Errorf("load: partition %s: %s is claimed and in no unit", name, path)
		default:
			return fmt.Errorf("load: partition %s: %s is in %d units, and every file is in exactly one",
				name, path, counts[path])
		}
	}
	return nil
}

// encodeOptions returns the frontend's options in the canonical
// encoding every key folds, and nothing for a frontend that
// declares none. A knob that changes the graph without changing a
// read must key, so options the encoding cannot see whole refuse.
func encodeOptions(f plugin.Frontend) ([]byte, error) {
	encoded, err := plugin.EncodeOptions(f)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return encoded, nil
}

// depthOf picks a unit's depth: signature-only when any member is
// under a declared root.
func depthOf(files []plugin.SourceRef, roots []string) plugin.Depth {
	for _, ref := range files {
		for _, root := range roots {
			if root == "" {
				continue
			}
			if ref.Path == root || strings.HasPrefix(ref.Path, root+"/") {
				return plugin.DepthSignatures
			}
		}
	}
	return plugin.DepthFull
}

// parseAll parses the units in parallel, each into its own source
// unit and its own sink, so scheduling orders neither the graph
// nor the findings. A frontend's returned error is fatal to the
// whole load and cancels the rest.
func parseAll(ctx context.Context, cfg Config, tree storeTree, units []*unit) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workers := min(len(units), runtime.GOMAXPROCS(0))
	sem := make(chan struct{}, max(workers, 1))
	failures := make([]error, len(units))
	var wg sync.WaitGroup
	for i, u := range units {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			u.sink = diag.NewSink()
			u.src = plugin.NewSourceUnit(
				u.files, tree, u.depth, u.frontend.Syntax(),
				string(cfg.Brand), u.sink, u.frontend.Name(),
			)
			if err := u.frontend.Parse(ctx, u.src); err != nil {
				failures[i] = fmt.Errorf("load: parse %s: %w", u.frontend.Name(), err)
				cancel()
			}
		}()
	}
	wg.Wait()

	if err := errors.Join(failures...); err != nil {
		return err
	}
	// The parent context may have been cancelled with every parse
	// clean. A skipped unit has no source unit to key, so the load
	// cannot finish.
	for _, u := range units {
		if u.src == nil {
			return fmt.Errorf("load: %w", ctx.Err())
		}
	}
	return nil
}

// storeTree is the tree every door of one load reads: the workspace
// tree, and the load's stores beside it. It keeps the workspace
// tree's own fast paths for whole-file reads and listings.
type storeTree struct {
	fs.FS
	stores map[string]fs.FS
}

// Store returns one of the load's stores.
func (t storeTree) Store(name string) (fs.FS, bool) {
	s, held := t.stores[name]
	return s, held
}

// ReadFile reads one workspace file through the workspace tree's own
// fast path where it has one.
func (t storeTree) ReadFile(name string) ([]byte, error) { return fs.ReadFile(t.FS, name) }

// ReadDir lists one workspace directory through the workspace tree's
// own fast path where it has one.
func (t storeTree) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(t.FS, name) }

// checkStores refuses a store the load could not address: a name a
// qualified path cannot spell, or a name without a tree.
func checkStores(stores map[string]fs.FS) error {
	for _, name := range slices.Sorted(maps.Keys(stores)) {
		if !plugin.ValidStoreName(name) {
			return fmt.Errorf("load: %q cannot name a store: a store's name is not empty, "+
				"and it contains neither a colon nor a slash", name)
		}
		if stores[name] == nil {
			return fmt.Errorf("load: store %s has no tree to read", name)
		}
	}
	return nil
}

// spliced is one merged package, the language it belongs to, whether
// that language's frontend is a [plugin.Importer], whether it reports
// that the language overloads, and the unit each of its files came
// from.
type spliced struct {
	pkg       *node.Package
	lang      symbol.Lang
	origin    diag.Origin
	importer  bool
	overloads bool
	units     map[*node.File]*unit
}

// scopeEntry joins one file's recorded bindings to the frontend
// whose Resolve reads them.
type scopeEntry struct {
	frontend plugin.Frontend
	file     *node.File
	bindings any
}

// splice merges the unit graphs in unit order. Two units
// contributing one language-and-path merge into one package, files
// appended in unit order, and the scope records pass through with
// their frontends. A merged package keeps the first name and the first
// non-empty documentation, because a language whose unit is one file
// states its package documentation in one file of many.
//
// The merge builds a package node of its own and leaves every unit's
// node with that unit's files alone, so each unit's region is the
// unit's own nodes. A package of one unit is that unit's node.
func splice(units []*unit, sink *diag.Sink) ([]*spliced, []scopeEntry) {
	type mergeKey struct {
		lang symbol.Lang
		path string
	}
	merged := make(map[mergeKey]*spliced)
	var (
		packages []*spliced
		scopes   []scopeEntry
	)
	for _, u := range units {
		lang := u.frontend.Lang()
		origin := u.frontend.Name()
		_, importer := u.frontend.(plugin.Importer)
		overloads := u.frontend.Overloads()
		gb := u.src.Graph()
		for _, p := range gb.Packages() {
			key := mergeKey{lang: lang, path: p.ID.Package}
			// The canonical identity is written on every unit's node, so
			// an attachment recorded against a unit's own package node
			// resolves to the merged package's identity.
			p.ID = symbol.Identity{Lang: lang, Package: p.ID.Package, Kind: symbol.KindPackage}
			kept, met := merged[key]
			if !met {
				kept = &spliced{
					pkg: p, lang: lang, origin: origin, importer: importer, overloads: overloads,
					units: make(map[*node.File]*unit, len(p.Files)),
				}
				merged[key] = kept
				packages = append(packages, kept)
			} else {
				if p.Name != "" && kept.pkg.Name != "" && p.Name != kept.pkg.Name {
					u.warnf(sink, DuplicateDeclaration, p.Pos, origin,
						"package %s is declared %q and %q, and the first name is kept",
						p.ID.Package, kept.pkg.Name, p.Name)
				}
				kept.pkg = mergedWith(kept.pkg, p)
			}
			for _, f := range p.Files {
				kept.units[f] = u
			}
		}
		for _, s := range gb.Scopes() {
			scopes = append(scopes, scopeEntry{frontend: u.frontend, file: s.File, bindings: s.Bindings})
		}
	}
	return packages, scopes
}

// mergedWith returns the merge of a package and the next unit's node of
// it: a node of its own, so no unit's node gains another unit's files.
// It keeps the first name and the first non-empty documentation.
func mergedWith(kept, next *node.Package) *node.Package {
	merged := *kept
	merged.Files = append(slices.Clip(kept.Files), next.Files...)
	if merged.Name == "" {
		merged.Name = next.Name
	}
	if len(merged.Doc) == 0 {
		merged.Doc = next.Doc
	}
	return &merged
}

// warnf reports one finding the splice, the assignment or the link
// reported about the unit's declarations into the load's sink, and
// records it as the unit's, so the unit's region replays it.
func (u *unit) warnf(sink *diag.Sink, c diag.Code, at position.Pos, by diag.Origin, format string, args ...any) {
	d := diag.Diag{Code: c, Severity: diag.SeverityWarning, Pos: at, Msg: fmt.Sprintf(format, args...), Origin: by}
	sink.Report(d)
	u.linked = append(u.linked, d)
}

// regionOf returns one unit's region: the unit's own package nodes,
// its attachments resolved to their identities, its references' link
// record, and every finding its parse and its link reported. A subject
// on a dropped duplicate attaches to the identity that is kept, and a
// subject the resolution step never identified is a frontend defect and
// panics.
func regionOf(u *unit, ix *index) *store.Region {
	gb := u.src.Graph()
	r := &store.Region{
		Packages: gb.Packages(),
		Links:    u.number(gb.Packages()),
		Findings: append(slices.Collect(u.sink.All()), u.linked...),
	}
	for _, a := range gb.Attachments() {
		id := subjectIdentity(a.Subject, ix)
		if id.IsZero() {
			panic(fmt.Sprintf(
				"load: %s attached a directive to a %s the resolution step never identified",
				u.frontend.Name(), a.Subject.Kind(),
			))
		}
		if r.Directives == nil {
			r.Directives = map[symbol.Identity][]directive.Raw{}
		}
		r.Directives[id] = append(r.Directives[id], a.Raw)
	}
	for _, s := range gb.StampRecords() {
		id := subjectIdentity(s.Subject, ix)
		if id.IsZero() {
			panic(fmt.Sprintf(
				"load: %s stamped a %s the resolution step never identified",
				u.frontend.Name(), s.Subject.Kind(),
			))
		}
		if r.Stamps == nil {
			r.Stamps = map[symbol.Identity][]meta.RawStamp{}
		}
		// The kernel sets the origin, so every stamp is attributed to
		// the frontend that recorded it.
		stamp := s.Stamp
		stamp.Origin = u.frontend.Name()
		r.Stamps[id] = append(r.Stamps[id], stamp)
	}
	return r
}

// number returns the unit's link records in the order of the
// depth-first walk of its packages, each numbered by its reference's
// place among the walk's type references.
func (u *unit) number(packages []*node.Package) []store.Link {
	if len(u.resolved) == 0 {
		return nil
	}
	out := make([]store.Link, 0, len(u.resolved))
	at := 0
	for _, p := range packages {
		node.Walk(p, func(s symbol.Symbol) bool {
			ref, is := s.(*node.TypeRef)
			if !is {
				return true
			}
			if l, held := u.resolved[ref]; held {
				l.Ref = at
				out = append(out, l)
			}
			at++
			return true
		})
	}
	return out
}

// subjectIdentity resolves an attachment subject: its assigned
// identity, or the kept twin's for a dropped duplicate.
func subjectIdentity(s symbol.Symbol, ix *index) symbol.Identity {
	if decl, names := s.(node.Declaration); names {
		if id := decl.Identity(); !id.IsZero() {
			return id
		}
	}
	return ix.dropped[s]
}
