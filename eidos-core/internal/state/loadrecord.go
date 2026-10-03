// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"iter"
	"maps"
	"slices"
	"sync"
	"time"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// The first byte of a probes key: a bare identity some reference named
// as a candidate, or a package whose re-exports some reference followed.
const (
	probeBare     = 'b'
	probeFollowed = 'f'
)

// keySep separates the parts of a composite key. No identity part and
// no frontend name contains it, so a composite key sorts by its parts.
const keySep = 0

// LoadState is a generation's record of the load: its file records, its
// units with their regions, each frontend's doors, and the probes of
// every unit's references. It is the [load.Prior] a warm load reads,
// and it records the next load in a [Commit].
//
// # Concurrency
//
// A LoadState is safe for concurrent use: the tables it reads whole load
// once under a [sync.Once], and a region decodes on each call.
type LoadState struct {
	ctx context.Context
	g   *Generation

	filesOnce sync.Once
	files     []load.FileRecord
	filesErr  error

	unitsOnce sync.Once
	units     []unitRow
	byFirst   map[string]int
	unitsErr  error
}

// unitRow is one unit's row: its record, and where its region is.
type unitRow struct {
	record load.UnitRecord
	region RegionRef
}

// Load returns the generation's record of the load, read through ctx.
func (g *Generation) Load(ctx context.Context) *LoadState {
	return &LoadState{ctx: ctx, g: g}
}

// Anchor returns the anchor the recording run's sweep took.
func (s *LoadState) Anchor() time.Time { return s.g.Header.Anchor }

// Files returns every file record, sorted by path.
func (s *LoadState) Files() iter.Seq2[load.FileRecord, error] {
	return func(yield func(load.FileRecord, error) bool) {
		files, err := s.readFiles()
		if err != nil {
			yield(load.FileRecord{}, err)
			return
		}
		for _, f := range files {
			if !yield(f, nil) {
				return
			}
		}
	}
}

// Units returns every unit record, in splice order.
func (s *LoadState) Units() iter.Seq2[load.UnitRecord, error] {
	return func(yield func(load.UnitRecord, error) bool) {
		units, err := s.readUnits()
		if err != nil {
			yield(load.UnitRecord{}, err)
			return
		}
		for _, u := range units {
			if !yield(u.record, nil) {
				return
			}
		}
	}
}

// Region decodes a recorded unit's region.
//
// Error modes: an error for a unit the record does not contain, and an
// error wrapping [ErrDamaged] for a region that does not read whole.
func (s *LoadState) Region(u load.UnitRecord) (*store.Region, error) {
	units, err := s.readUnits()
	if err != nil {
		return nil, err
	}
	if u.Number < 0 || u.Number >= len(units) {
		return nil, fmt.Errorf("state: the record has no unit %d", u.Number)
	}
	b, err := s.g.region(s.ctx, units[u.Number].region)
	if err != nil {
		return nil, err
	}
	return DecodeRegion(b)
}

// Doors returns a frontend's recorded partition, then its dependency
// rounds in order, and nothing for a frontend the record lacks.
//
// Error modes: an error wrapping [ErrDamaged] for a table or a row that
// does not read whole.
func (s *LoadState) Doors(frontend plugin.ID) ([]load.DoorRecord, error) {
	rows, err := s.g.readers[TableDoors].all(s.ctx)
	if err != nil {
		return nil, err
	}
	prefix := append([]byte(frontend), keySep)
	var out []load.DoorRecord
	for _, e := range rows {
		if !bytes.HasPrefix(e.key, prefix) {
			continue
		}
		door, err := decodeDoor(e.row)
		if err != nil {
			return nil, err
		}
		out = append(out, door)
	}
	return out, nil
}

// Probed returns the recorded units whose references named a bare
// identity as a candidate, by number.
//
// Error modes: an error wrapping [ErrDamaged] for a table or a row that
// does not read whole.
func (s *LoadState) Probed(bare symbol.Identity) ([]int, error) {
	return s.probed(probeKey(probeBare, bare))
}

// Followed returns the recorded units whose references followed a
// re-export of a file of the package, by number.
//
// Error modes: an error wrapping [ErrDamaged] for a table or a row that
// does not read whole.
func (s *LoadState) Followed(pkg symbol.Identity) ([]int, error) {
	return s.probed(probeKey(probeFollowed, pkg))
}

// probed returns the units a probes row lists, by number.
func (s *LoadState) probed(key []byte) ([]int, error) {
	row, held, err := s.g.readers[TableProbes].get(s.ctx, key)
	if err != nil || !held {
		return nil, err
	}
	if _, err := s.readUnits(); err != nil {
		return nil, err
	}
	d := newDecoder(row, nil)
	paths := d.texts()
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a probes row does not decode: %w", ErrDamaged, err)
	}
	out := make([]int, 0, len(paths))
	for _, p := range paths {
		if n, held := s.byFirst[p]; held {
			out = append(out, n)
		}
	}
	return out, nil
}

// readFiles reads the files table once.
func (s *LoadState) readFiles() ([]load.FileRecord, error) {
	s.filesOnce.Do(func() {
		rows, err := s.g.readers[TableFiles].all(s.ctx)
		if err != nil {
			s.filesErr = err
			return
		}
		s.files = make([]load.FileRecord, len(rows))
		for i, e := range rows {
			if s.files[i], err = decodeFile(e.key, e.row); err != nil {
				s.filesErr = err
				return
			}
		}
	})
	return s.files, s.filesErr
}

// readUnits reads the units table once, numbering the units in splice
// order.
func (s *LoadState) readUnits() ([]unitRow, error) {
	s.unitsOnce.Do(func() {
		rows, err := s.g.readers[TableUnits].all(s.ctx)
		if err != nil {
			s.unitsErr = err
			return
		}
		s.units = make([]unitRow, len(rows))
		s.byFirst = make(map[string]int, len(rows))
		for i, e := range rows {
			if s.units[i], err = decodeUnit(e.row); err != nil {
				s.unitsErr = err
				return
			}
			s.units[i].record.Number = i
			s.byFirst[string(e.key)] = i
		}
	})
	return s.units, s.unitsErr
}

// RecordLoad records a load in a commit: each file record that differs
// from the record of the state the load read, each region the load
// built, as one segment, with its unit's row, each unit the load
// dropped, each frontend's doors, and the probes of every unit whose
// region changed. A file record whose modification time does not
// precede the load's anchor is recorded with a size of zero, so the
// next gate that meets the file hashes it. prior is the state the load
// read, nil for a cold load.
//
// Error modes: the error of [AppendRegion] for a region that does not
// encode, and an error wrapping [ErrDamaged] for a prior region that
// does not read whole.
func RecordLoad(ctx context.Context, c *Commit, prior *LoadState, r *load.Report) error {
	var (
		was      map[string]load.FileRecord
		wasUnits map[string]unitRow
	)
	if prior != nil {
		files, err := prior.readFiles()
		if err != nil {
			return err
		}
		was = make(map[string]load.FileRecord, len(files))
		for _, f := range files {
			was[f.Path] = f
		}
		units, err := prior.readUnits()
		if err != nil {
			return err
		}
		wasUnits = make(map[string]unitRow, len(units))
		for _, u := range units {
			wasUnits[u.record.Files[0].Path] = u
		}
	}
	recordFiles(c, was, r)
	if err := recordUnits(ctx, c, prior, wasUnits, r); err != nil {
		return err
	}
	return recordDoors(ctx, c, prior, r)
}

// recordFiles puts each file record that differs from the prior record,
// and deletes each prior record whose file the load no longer met.
func recordFiles(c *Commit, was map[string]load.FileRecord, r *load.Report) {
	seen := make(map[string]bool, len(r.Files))
	for _, f := range r.Files {
		seen[f.Path] = true
		if !f.ModTime.Before(r.Anchor) {
			f.Size = 0
		}
		if prev, held := was[f.Path]; held && sameFile(prev, f) {
			continue
		}
		c.Put(TableFiles, []byte(f.Path), encodeFile(f))
	}
	for path := range was {
		if !seen[path] {
			c.Delete(TableFiles, []byte(path))
		}
	}
}

// sameFile reports whether two records of one file are equal.
func sameFile(a, b load.FileRecord) bool {
	return a.Size == b.Size && a.ModTime.Equal(b.ModTime) && a.Change.Equal(b.Change) &&
		a.Inode == b.Inode && a.Digest == b.Digest && a.Verdict == b.Verdict && a.Pkg == b.Pkg
}

// recordUnits adds every region the load built as one segment, puts the
// row of each unit whose region or record changed, deletes the row of
// each unit the load dropped, releases every region the next generation
// no longer references, and records the probes of every unit whose
// region changed.
func recordUnits(
	ctx context.Context, c *Commit, prior *LoadState, was map[string]unitRow, r *load.Report,
) error {
	var (
		blobs [][]byte
		built []int
	)
	for i, u := range r.Units {
		if u.Region == nil {
			continue
		}
		b, err := AppendRegion(nil, u.Region)
		if err != nil {
			return fmt.Errorf("state: record unit %s: %w", u.Files[0].Path, err)
		}
		blobs = append(blobs, b)
		built = append(built, i)
	}
	refs, err := c.AddRegions(blobs)
	if err != nil {
		return err
	}

	probes := newProbeSet()
	kept := make(map[string]bool, len(r.Units))
	for k, i := range built {
		u := r.Units[i]
		first := u.Files[0].Path
		kept[first] = true
		if prev, held := was[first]; held {
			c.Release(prev.region)
			if err := probes.drop(ctx, prior, prev, first); err != nil {
				return err
			}
		}
		probes.add(u.Region, first)
		c.Put(TableUnits, []byte(first), encodeUnit(unitRowOf(u, refs[k])))
	}
	for _, u := range r.Units {
		kept[u.Files[0].Path] = true
	}
	for first, prev := range was {
		if kept[first] {
			continue
		}
		c.Release(prev.region)
		c.Delete(TableUnits, []byte(first))
		if err := probes.drop(ctx, prior, prev, first); err != nil {
			return err
		}
	}
	return probes.record(ctx, c, prior)
}

// recordDoors puts each door whose row differs from the prior row, and
// deletes each prior door the load no longer has.
func recordDoors(ctx context.Context, c *Commit, prior *LoadState, r *load.Report) error {
	var was map[string][]byte
	if prior != nil {
		rows, err := prior.g.readers[TableDoors].all(ctx)
		if err != nil {
			return err
		}
		was = make(map[string][]byte, len(rows))
		for _, e := range rows {
			was[string(e.key)] = e.row
		}
	}
	seen := map[string]bool{}
	for _, frontend := range slices.Sorted(maps.Keys(r.Doors)) {
		for i, door := range r.Doors[frontend] {
			key := doorKey(frontend, i)
			seen[string(key)] = true
			row := encodeDoor(door)
			if bytes.Equal(was[string(key)], row) {
				continue
			}
			c.Put(TableDoors, key, row)
		}
	}
	for key := range was {
		if !seen[key] {
			c.Delete(TableDoors, []byte(key))
		}
	}
	return nil
}

// probeSet is the probes rows a commit changes: for each key, the units
// to drop from its row and the units to add.
type probeSet struct {
	dropped map[string]map[string]bool
	added   map[string]map[string]bool
}

// newProbeSet returns an empty set of changes.
func newProbeSet() *probeSet {
	return &probeSet{dropped: map[string]map[string]bool{}, added: map[string]map[string]bool{}}
}

// add records a region's probes under its unit's first member: each
// bare candidate of its links, and each package whose re-exports one
// followed.
func (p *probeSet) add(r *store.Region, first string) {
	for key := range probeKeys(r) {
		mark(p.added, key, first)
	}
}

// drop records that a unit no longer contributes its prior region's
// probes.
func (p *probeSet) drop(ctx context.Context, prior *LoadState, prev unitRow, first string) error {
	b, err := prior.g.region(ctx, prev.region)
	if err != nil {
		return err
	}
	r, err := DecodeRegion(b)
	if err != nil {
		return err
	}
	for key := range probeKeys(r) {
		mark(p.dropped, key, first)
	}
	return nil
}

// record puts each changed probes row: the prior row's units without
// the dropped ones and with the added ones, sorted, and deletes a row
// left empty.
func (p *probeSet) record(ctx context.Context, c *Commit, prior *LoadState) error {
	keys := slices.Concat(slices.Collect(maps.Keys(p.dropped)), slices.Collect(maps.Keys(p.added)))
	slices.Sort(keys)
	for _, key := range slices.Compact(keys) {
		units := map[string]bool{}
		if prior != nil {
			row, held, err := prior.g.readers[TableProbes].get(ctx, []byte(key))
			if err != nil {
				return err
			}
			if held {
				d := newDecoder(row, nil)
				for _, u := range d.texts() {
					units[u] = true
				}
				if err := d.Err(); err != nil {
					return fmt.Errorf("%w: a probes row does not decode: %w", ErrDamaged, err)
				}
			}
		}
		for u := range p.dropped[key] {
			delete(units, u)
		}
		for u := range p.added[key] {
			units[u] = true
		}
		if len(units) == 0 {
			c.Delete(TableProbes, []byte(key))
			continue
		}
		e := &encoder{}
		e.texts(slices.Sorted(maps.Keys(units)))
		c.Put(TableProbes, []byte(key), e.buf)
	}
	return nil
}

// mark adds one unit to a key's set.
func mark(sets map[string]map[string]bool, key, unit string) {
	if sets[key] == nil {
		sets[key] = map[string]bool{}
	}
	sets[key][unit] = true
}

// probeKeys returns a region's probes keys: each bare candidate its
// links name or a followed re-export offered, and each package whose
// re-exports a link followed.
func probeKeys(r *store.Region) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, l := range r.Links {
			for _, tier := range l.Tiers {
				for _, c := range tier {
					if !yield(string(probeKey(probeBare, bare(c)))) {
						return
					}
				}
			}
			for _, c := range l.Reached {
				if !yield(string(probeKey(probeBare, bare(c)))) {
					return
				}
			}
			for _, f := range l.Followed {
				if !yield(string(probeKey(probeFollowed, f.PackageIdentity()))) {
					return
				}
			}
		}
	}
}

// bare strips an identity to what a resolver can spell.
func bare(id symbol.Identity) symbol.Identity {
	return symbol.Identity{Lang: id.Lang, Package: id.Package, Owner: id.Owner, Name: id.Name}
}

// probeKey returns a probes key: its kind's byte and the identity's key.
func probeKey(kind byte, id symbol.Identity) []byte {
	return identityKey([]byte{kind}, id)
}

// identityKey appends an identity's key to dst: its six parts, each
// followed by the separator, so keys sort the way identities compare.
func identityKey(dst []byte, id symbol.Identity) []byte {
	for _, part := range []string{string(id.Lang), id.Package, id.Owner, id.Name} {
		dst = append(dst, part...)
		dst = append(dst, keySep)
	}
	dst = append(dst, byte(id.Kind), keySep)
	return append(dst, id.Disc...)
}

// doorKey returns a door's key: the frontend's name, the separator, and
// the door's place, the partition first.
func doorKey(frontend plugin.ID, place int) []byte {
	key := append([]byte(frontend), keySep)
	return binary.BigEndian.AppendUint32(key, uint32(place))
}

// unitRowOf returns the row of a unit a load built a region for.
func unitRowOf(u load.UnitReport, ref RegionRef) unitRow {
	findings := slices.Clone(u.Region.Findings)
	for _, l := range u.Region.Links {
		findings = append(findings, l.Findings...)
	}
	return unitRow{
		record: load.UnitRecord{
			Frontend: u.Frontend,
			Files:    u.Files,
			Depth:    u.Depth,
			Round:    u.Round,
			Key:      u.Key,
			Summary:  u.Region.Info(),
			Imports:  u.Imports,
			Findings: findings,
		},
		region: ref,
	}
}

// encodeFile returns a file record's row. The path is the key.
func encodeFile(f load.FileRecord) []byte {
	e := &encoder{}
	e.varint(f.Size)
	e.instant(f.ModTime)
	e.instant(f.Change)
	e.uvarint(f.Inode)
	e.digest(f.Digest)
	e.uvarint(uint64(f.Verdict))
	e.identity(f.Pkg)
	return e.buf
}

// decodeFile decodes a file record's row under its key.
func decodeFile(key, row []byte) (load.FileRecord, error) {
	d := newDecoder(row, nil)
	f := load.FileRecord{
		Path:    string(key),
		Size:    d.Varint(),
		ModTime: d.instant(),
		Change:  d.instant(),
		Inode:   d.Uvarint(),
		Digest:  d.Digest(),
		Verdict: load.Verdict(d.Uvarint()),
		Pkg:     d.identity(),
	}
	if err := d.Err(); err != nil {
		return load.FileRecord{}, fmt.Errorf("%w: the record of %s does not decode: %w", ErrDamaged, key, err)
	}
	return f, nil
}

// encodeUnit returns a unit's row.
func encodeUnit(u unitRow) []byte {
	e := &encoder{}
	rec := u.record
	e.text(string(rec.Frontend))
	e.refs(rec.Files)
	e.uvarint(uint64(rec.Depth))
	e.uvarint(uint64(rec.Round))
	e.bytes(rec.Key)
	e.text(u.region.Segment)
	e.uvarint(uint64(u.region.Offset))
	e.uvarint(uint64(u.region.Length))
	e.summary(rec.Summary)
	e.imports(rec.Imports)
	e.findings(rec.Findings)
	return e.buf
}

// decodeUnit decodes a unit's row.
func decodeUnit(row []byte) (unitRow, error) {
	d := newDecoder(row, nil)
	var u unitRow
	u.record.Frontend = plugin.ID(d.text())
	u.record.Files = d.refs()
	u.record.Depth = plugin.Depth(d.Uvarint())
	u.record.Round = int(d.Uvarint())
	u.record.Key = bytes.Clone(d.Bytes())
	u.region = RegionRef{Segment: d.text(), Offset: int64(d.Uvarint()), Length: int64(d.Uvarint())}
	u.record.Summary = d.summary()
	u.record.Imports = d.imports()
	u.record.Findings = d.findings()
	if err := d.Err(); err != nil {
		return unitRow{}, fmt.Errorf("%w: a unit's row does not decode: %w", ErrDamaged, err)
	}
	if len(u.record.Files) == 0 {
		return unitRow{}, fmt.Errorf("%w: a unit's row lists no member", ErrDamaged)
	}
	return u, nil
}

// encodeDoor returns a door's row.
func encodeDoor(door load.DoorRecord) []byte {
	e := &encoder{}
	e.uvarint(uint64(door.Round))
	e.uvarint(uint64(len(door.Reads)))
	for _, r := range door.Reads {
		e.text(r.Path)
		e.digest(r.Digest)
	}
	e.uvarint(uint64(len(door.Needs)))
	for _, n := range door.Needs {
		e.text(n.Path)
		e.texts(n.From)
	}
	e.uvarint(uint64(len(door.Units)))
	for _, u := range door.Units {
		e.refs(u)
	}
	e.findings(door.Findings)
	return e.buf
}

// decodeDoor decodes a door's row.
func decodeDoor(row []byte) (load.DoorRecord, error) {
	d := newDecoder(row, nil)
	door := load.DoorRecord{Round: int(d.Uvarint())}
	if n := d.Count(); n > 0 {
		door.Reads = make([]load.Digested, n)
		for i := range door.Reads {
			door.Reads[i] = load.Digested{Path: d.text(), Digest: d.Digest()}
		}
	}
	if n := d.Count(); n > 0 {
		door.Needs = make([]plugin.Need, n)
		for i := range door.Needs {
			door.Needs[i] = plugin.Need{Path: d.text(), From: d.texts()}
		}
	}
	if n := d.Count(); n > 0 {
		door.Units = make([][]plugin.SourceRef, n)
		for i := range door.Units {
			door.Units[i] = d.refs()
		}
	}
	door.Findings = d.findings()
	if err := d.Err(); err != nil {
		return load.DoorRecord{}, fmt.Errorf("%w: a door's row does not decode: %w", ErrDamaged, err)
	}
	return door, nil
}

// refs writes a unit's members: each path and its shared inputs.
func (e *encoder) refs(refs []plugin.SourceRef) {
	e.uvarint(uint64(len(refs)))
	for _, r := range refs {
		e.text(r.Path)
		e.texts(r.Shared)
	}
}

// refs reads a unit's members.
func (d *decoder) refs() []plugin.SourceRef {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]plugin.SourceRef, n)
	for i := range out {
		out[i] = plugin.SourceRef{Path: d.text(), Shared: d.texts()}
	}
	return out
}

// imports writes a unit's imports: each path, the files that import it
// and the position of its first import.
func (e *encoder) imports(imports []load.Import) {
	e.uvarint(uint64(len(imports)))
	for _, imp := range imports {
		e.text(imp.Path)
		e.texts(imp.Files)
		e.pos(imp.At)
	}
}

// imports reads a unit's imports, nil for none.
func (d *decoder) imports() []load.Import {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]load.Import, n)
	for i := range out {
		out[i] = load.Import{Path: d.text(), Files: d.texts(), At: d.pos()}
	}
	return out
}

// summary writes a region's summary.
func (e *encoder) summary(s store.RegionInfo) {
	e.identities(s.Packages)
	e.uvarint(uint64(len(s.Files)))
	for _, f := range s.Files {
		e.text(f.Path)
		e.identity(f.Pkg)
	}
	e.uvarint(uint64(len(s.Kinds)))
	for _, k := range s.Kinds {
		e.uvarint(uint64(k))
	}
	e.uvarint(uint64(len(s.Directives)))
	for _, n := range s.Directives {
		e.text(string(n))
	}
}

// summary reads a region's summary.
func (d *decoder) summary() store.RegionInfo {
	var s store.RegionInfo
	s.Packages = d.identities()
	if n := d.Count(); n > 0 {
		s.Files = make([]store.RegionFile, n)
		for i := range s.Files {
			s.Files[i] = store.RegionFile{Path: d.text(), Pkg: d.identity()}
		}
	}
	if n := d.Count(); n > 0 {
		s.Kinds = make([]symbol.Kind, n)
		for i := range s.Kinds {
			s.Kinds[i] = symbol.Kind(d.Uvarint())
		}
	}
	for range d.Count() {
		s.Directives = append(s.Directives, directive.Name(d.text()))
	}
	return s
}
