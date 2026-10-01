// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
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

	// PluginSet is the composition's fingerprint, folded into every
	// unit key: a recorded graph contains stamps a changed plugin
	// set reinterprets. A composed workspace derives it with
	// [go.dokimi.dev/eidos/core/workspace.Workspace.Fingerprint], and
	// a hand-written literal is a fixture's shortcut, never a
	// production caller's.
	PluginSet []byte

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
	// is ordinary input. The load refuses a brand outside
	// [output.Brand.Valid].
	Brand output.Brand

	// Stores maps each store's name to its read-only tree outside the
	// workspace, which dependency units read: a Go module cache, a Go
	// standard library, a JDK's ct.sym, a Maven or Gradle cache. The
	// composition opens the trees, and no key folds where they are.
	// The load refuses a name [plugin.ValidStoreName] refuses and a
	// name without a tree.
	Stores map[string]fs.FS
}

// Report is what one load records beside the graph.
type Report struct {
	// Units has one entry per parsed unit, in splice order.
	Units []UnitReport

	// Excluded lists the claimed files the load refused as the
	// workspace's own outputs, sorted by path.
	Excluded []string
}

// UnitReport is one unit's record.
type UnitReport struct {
	// Frontend parsed the unit.
	Frontend plugin.ID
	// Files are the unit's members, in partition order.
	Files []string
	// Depth is what the unit loaded at.
	Depth plugin.Depth
	// Key is the unit's finished key, folded as the package
	// documentation states.
	Key []byte
	// Round is zero for a unit the partition returned, and the
	// dependency round's number for a unit a [plugin.Dependent]
	// frontend returned.
	Round int
}

// unit is one compilation unit on its way through the pipeline.
type unit struct {
	frontend plugin.Frontend
	files    []plugin.SourceRef
	depth    plugin.Depth
	// partition is the read fold of the door that shaped the unit:
	// the partition's, shared per frontend, or the dependency round's,
	// shared per round.
	partition []byte
	config    []byte // the frontend's options, canonically encoded
	version   string
	round     int
	sink      *diag.Sink
	src       *plugin.SourceUnit
}

// Load drives every frontend over the tree: select, partition,
// parse, load the dependencies, splice, resolve, seal. It returns
// the sealed graph and the report. A nil graph means nothing loaded,
// and the error states why.
func Load(ctx context.Context, cfg Config) (*store.Graph, *Report, error) {
	if cfg.FS == nil {
		return nil, nil, errors.New("load: no tree to read")
	}
	if cfg.Sink == nil {
		return nil, nil, errors.New("load: no sink to report into")
	}
	if !cfg.Brand.Valid() {
		return nil, nil, fmt.Errorf(
			"load: %q is not a brand, and carriers and outputs are read under the composition's brand",
			string(cfg.Brand),
		)
	}
	for _, f := range cfg.Frontends {
		if _, versioned := f.(plugin.Versioned); !versioned {
			return nil, nil, fmt.Errorf(
				"load: frontend %s declares no version, which every unit key folds", f.Name(),
			)
		}
	}
	if err := checkStores(cfg.Stores); err != nil {
		return nil, nil, err
	}
	tree := storeTree{FS: cfg.FS, stores: cfg.Stores}

	files, err := treeFiles(cfg.FS)
	if err != nil {
		return nil, nil, err
	}
	claims, err := claim(cfg.Frontends, files)
	if err != nil {
		return nil, nil, err
	}
	claimed := make(map[string]bool, len(files))
	for _, paths := range claims {
		for _, path := range paths {
			claimed[path] = true
		}
	}
	excluded, err := dropOutput(cfg, claims)
	if err != nil {
		return nil, nil, err
	}
	units, err := partitionAll(ctx, cfg, tree, claims)
	if err != nil {
		return nil, nil, err
	}
	err = parseAll(ctx, cfg, tree, units)
	if err != nil {
		return nil, nil, err
	}
	deps, err := dependencies(ctx, cfg, tree, claimed, units)
	if err != nil {
		return nil, nil, err
	}
	units = append(units, deps...)
	slices.SortFunc(units, byFirstMember)
	for _, u := range units {
		for d := range u.sink.All() {
			cfg.Sink.Report(d)
		}
	}

	packages, scopes, attachments, stamps := splice(units, cfg.Sink)
	ix := assign(packages, cfg.Sink)
	link(packages, scopes, ix, cfg.Sink)

	g := store.New()
	for _, sp := range packages {
		if err := g.AddPackage(sp.pkg); err != nil {
			return nil, nil, fmt.Errorf("load: %w", err)
		}
	}
	if err := attach(g, attachments, ix); err != nil {
		return nil, nil, err
	}
	if err := attachStamps(g, stamps, ix); err != nil {
		return nil, nil, err
	}
	g.Freeze()

	report := &Report{Units: make([]UnitReport, 0, len(units)), Excluded: excluded}
	for _, u := range units {
		report.Units = append(report.Units, UnitReport{
			Frontend: u.frontend.Name(),
			Files:    memberPaths(u.files),
			Depth:    u.depth,
			Key:      unitKey(u, cfg.PluginSet, cfg.Brand),
			Round:    u.round,
		})
	}
	return g, report, nil
}

// treeFiles walks the tree and returns every file's slash path, in
// the walk's lexical order.
func treeFiles(fsys fs.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load: walk the tree: %w", err)
	}
	return out, nil
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

// dropOutput drops every claimed file with a provenance trailer under
// the load's own brand from the claims, and returns what it
// dropped, sorted by path. The proof is read from the bytes and not
// matched against declared output families, because an out=
// redirect and the orphaned output of a removed plugin match no
// current declaration. A claimed file costs one read of its last
// [output.TailSize] bytes, and a read of the whole file only where
// those bytes contain a trailer's key.
func dropOutput(cfg Config, claims [][]string) ([]string, error) {
	var excluded []string
	tail := make([]byte, output.TailSize)
	for i, paths := range claims {
		kept := paths[:0]
		for _, path := range paths {
			stamped, err := stampedBy(cfg.FS, path, cfg.Brand, tail)
			if err != nil {
				return nil, fmt.Errorf("load: read %s: %w", path, err)
			}
			if stamped {
				excluded = append(excluded, path)
				continue
			}
			kept = append(kept, path)
		}
		claims[i] = kept
	}
	slices.Sort(excluded)
	return excluded, nil
}

// stampedBy reports whether a file has a provenance trailer under a
// brand. It probes the file's tail into buf first, and reads the
// whole file only where the tail contains a trailer's key.
func stampedBy(fsys fs.FS, path string, brand output.Brand, buf []byte) (bool, error) {
	keyed, err := tailKeyed(fsys, path, buf)
	if err != nil {
		return false, err
	}
	if !keyed {
		return false, nil
	}
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return false, err
	}
	prov, stamped := output.Read(b)
	return stamped && prov.Brand == brand, nil
}

// tailKeyed reports whether a file's last len(buf) bytes contain a
// trailer's key, reading them into buf through [io.Seeker]. A file
// that does not implement io.Seeker reports true, so its caller
// reads it whole.
func tailKeyed(fsys fs.FS, path string, buf []byte) (bool, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	s, seeks := f.(io.Seeker)
	if !seeks {
		return true, nil
	}
	end, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return false, err
	}
	start := max(end-int64(len(buf)), 0)
	if _, err = s.Seek(start, io.SeekStart); err != nil {
		return false, err
	}
	n, err := io.ReadFull(f, buf[:end-start])
	if err != nil {
		return false, err
	}
	return output.HasTrailerKey(buf[:n]), nil
}

// partitionAll partitions each frontend's claim into units, checks
// the partition contract, and fixes the splice order: units sorted
// by their first file's path, which is unique because claims do
// not overlap.
func partitionAll(ctx context.Context, cfg Config, tree storeTree, claims [][]string) ([]*unit, error) {
	var units []*unit
	for i, f := range cfg.Frontends {
		if len(claims[i]) == 0 {
			continue
		}
		refs := make([]plugin.SourceRef, len(claims[i]))
		for j, path := range claims[i] {
			refs[j] = plugin.SourceRef{Path: path}
		}
		reader := &recordingReader{fsys: tree, reads: sha256.New()}
		parts, err := f.Partition(ctx, refs, reader)
		if err != nil {
			return nil, fmt.Errorf("load: partition %s: %w", f.Name(), err)
		}
		if contractErr := checkPartition(f.Name(), claims[i], parts); contractErr != nil {
			return nil, contractErr
		}
		config, err := encodeOptions(f)
		if err != nil {
			return nil, err
		}
		partition := reader.reads.Sum(nil)
		versioned, _ := f.(plugin.Versioned) // asserted when the load began
		version := versioned.Version()
		for _, part := range parts {
			units = append(units, &unit{
				frontend:  f,
				files:     part,
				depth:     depthOf(part, cfg.Signatures),
				partition: partition,
				config:    config,
				version:   version,
			})
		}
	}
	slices.SortFunc(units, byFirstMember)
	return units, nil
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

// recordingReader is a recorded door over the workspace tree and the
// load's stores: the partition's, whose sum every unit of the
// partition keys on, and a dependency round's, whose sum every unit
// of the round keys on. A read that fails records nothing, because
// it read nothing.
type recordingReader struct {
	fsys  fs.FS
	reads hash.Hash
}

// listingMark opens the record of a directory listing. A read's
// record opens with its path, which is never empty and never contains
// a NUL byte, so the two kinds of record cannot pass for each other.
const listingMark = "\x00list\x00"

// Read returns one file's bytes and records the read.
func (r *recordingReader) Read(path string) ([]byte, error) {
	b, err := plugin.ReadFile(r.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("load: read %s: %w", path, err)
	}
	r.reads.Write([]byte(path))
	r.reads.Write([]byte{0})
	r.reads.Write(b)
	r.reads.Write([]byte{0})
	return b, nil
}

// ReadDir returns one directory's entries sorted by name and records
// the listing: the path, and each entry's name, a directory's behind
// a trailing slash.
func (r *recordingReader) ReadDir(path string) ([]fs.DirEntry, error) {
	entries, err := plugin.ReadDir(r.fsys, path)
	if err != nil {
		return nil, fmt.Errorf("load: list %s: %w", path, err)
	}
	r.reads.Write([]byte(listingMark))
	r.reads.Write([]byte(path))
	r.reads.Write([]byte{0})
	for _, e := range entries {
		r.reads.Write([]byte(e.Name()))
		if e.IsDir() {
			r.reads.Write([]byte{'/'})
		}
		r.reads.Write([]byte{0})
	}
	r.reads.Write([]byte{0})
	return entries, nil
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
// that language's frontend is a [plugin.Importer], and whether it
// reports that the language overloads.
type spliced struct {
	pkg       *node.Package
	lang      symbol.Lang
	origin    diag.Origin
	importer  bool
	overloads bool
}

// scopeEntry joins one file's recorded bindings to the frontend
// whose Resolve reads them.
type scopeEntry struct {
	frontend plugin.Frontend
	file     *node.File
	bindings any
}

// attachEntry is one recorded attachment and its origin.
type attachEntry struct {
	subject symbol.Symbol
	raw     directive.Raw
	origin  diag.Origin
}

// stampEntry is one recorded classification stamp, its origin
// already the kernel's.
type stampEntry struct {
	subject symbol.Symbol
	stamp   meta.RawStamp
}

// splice merges the unit graphs in unit order. Two units
// contributing one language-and-path merge into one package, files
// appended in unit order, and the scope and attachment records pass
// through with their frontends. A merged package keeps the first
// name and the first non-empty documentation, because a language
// whose unit is one file states its package documentation in one
// file of many.
func splice(units []*unit, sink *diag.Sink) ([]*spliced, []scopeEntry, []attachEntry, []stampEntry) {
	type mergeKey struct {
		lang symbol.Lang
		path string
	}
	merged := make(map[mergeKey]*spliced)
	var (
		packages    []*spliced
		scopes      []scopeEntry
		attachments []attachEntry
		stamps      []stampEntry
	)
	for _, u := range units {
		lang := u.frontend.Lang()
		origin := u.frontend.Name()
		_, importer := u.frontend.(plugin.Importer)
		overloads := u.frontend.Overloads()
		gb := u.src.Graph()
		for _, p := range gb.Packages() {
			key := mergeKey{lang: lang, path: p.ID.Package}
			// The canonical identity is written on every unit's
			// node, merged or kept, so an attachment recorded
			// against a merged-away package node still resolves to
			// the identity that is kept.
			p.ID = symbol.Identity{Lang: lang, Package: p.ID.Package, Kind: symbol.KindPackage}
			kept, met := merged[key]
			if !met {
				kept = &spliced{
					pkg: p, lang: lang, origin: origin, importer: importer, overloads: overloads,
				}
				merged[key] = kept
				packages = append(packages, kept)
				continue
			}
			if p.Name != "" && kept.pkg.Name != "" && p.Name != kept.pkg.Name {
				sink.Warnf(DuplicateDeclaration, p.Pos, origin,
					"package %s is declared %q and %q, and the first name is kept",
					p.ID.Package, kept.pkg.Name, p.Name)
			}
			if kept.pkg.Name == "" {
				kept.pkg.Name = p.Name
			}
			if len(kept.pkg.Doc) == 0 {
				kept.pkg.Doc = p.Doc
			}
			kept.pkg.Files = append(kept.pkg.Files, p.Files...)
		}
		for _, s := range gb.Scopes() {
			scopes = append(scopes, scopeEntry{frontend: u.frontend, file: s.File, bindings: s.Bindings})
		}
		for _, a := range gb.Attachments() {
			attachments = append(attachments, attachEntry{subject: a.Subject, raw: a.Raw, origin: origin})
		}
		for _, s := range gb.StampRecords() {
			// The kernel sets the origin, so every stamp is attributed
			// to the frontend that recorded it.
			s.Stamp.Origin = origin
			stamps = append(stamps, stampEntry{subject: s.Subject, stamp: s.Stamp})
		}
	}
	return packages, scopes, attachments, stamps
}

// attach records every attachment on its assigned identity. A
// subject on a dropped duplicate attaches to the identity that is
// kept, and a subject the resolution step never identified is a
// frontend defect and panics.
func attach(g *store.Graph, attachments []attachEntry, ix *index) error {
	byID := map[symbol.Identity][]directive.Raw{}
	var order []symbol.Identity
	for _, e := range attachments {
		id := subjectIdentity(e.subject, ix)
		if id.IsZero() {
			panic(fmt.Sprintf(
				"load: %s attached a directive to a %s the resolution step never identified",
				e.origin, e.subject.Kind(),
			))
		}
		if _, met := byID[id]; !met {
			order = append(order, id)
		}
		byID[id] = append(byID[id], e.raw)
	}
	for _, id := range order {
		if err := g.AttachDirectives(id, byID[id]); err != nil {
			return fmt.Errorf("load: %w", err)
		}
	}
	return nil
}

// attachStamps records every classification stamp on its assigned
// identity, resolving subjects the way [attach] does.
func attachStamps(g *store.Graph, stamps []stampEntry, ix *index) error {
	byID := map[symbol.Identity][]meta.RawStamp{}
	var order []symbol.Identity
	for _, e := range stamps {
		id := subjectIdentity(e.subject, ix)
		if id.IsZero() {
			panic(fmt.Sprintf(
				"load: %s stamped a %s the resolution step never identified",
				e.stamp.Origin, e.subject.Kind(),
			))
		}
		if _, met := byID[id]; !met {
			order = append(order, id)
		}
		byID[id] = append(byID[id], e.stamp)
	}
	for _, id := range order {
		if err := g.AttachStamps(id, byID[id]); err != nil {
			return fmt.Errorf("load: %w", err)
		}
	}
	return nil
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

// memberPaths returns the refs' paths, in partition order.
func memberPaths(refs []plugin.SourceRef) []string {
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = ref.Path
	}
	return out
}
