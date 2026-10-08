// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Changes lists what a load changed against the record of the previous
// load, by identity. A declaration's subtree covers its members, so a
// changed member also changes the declarations that contain it,
// including its file. The lists of identities are in identity order, and
// Spellings is in name order.
type Changes struct {
	// Appeared lists the declarations that only the load declares.
	Appeared []symbol.Identity
	// Disappeared lists the declarations that only the record declares.
	Disappeared []symbol.Identity
	// Changed lists the declarations that the load and the record both
	// declare with a different subtree, position, reference target,
	// directive or stamp.
	Changed []symbol.Identity
	// Packages lists the packages that contain a declaration that
	// appeared, disappeared or changed, and the packages whose own fields,
	// directives or stamps changed.
	Packages []symbol.Identity
	// Directed lists the subjects whose raw directives differ. The list
	// includes packages, and subjects that the graph does not contain.
	Directed []symbol.Identity
	// Spellings lists the directive spellings that a subject gained or
	// lost. An enumeration by directive reads these spellings.
	Spellings []directive.Name
	// Restamped lists the subjects whose classification stamps differ.
	Restamped []symbol.Identity
	// Renamed lists the packages that the load and the record both
	// declare under different names: a package clause that an edit
	// changed.
	Renamed []symbol.Identity
}

// fingerprint is what a change of one declaration compares: the digest
// of its subtree's encoding, which covers its position and its
// references' targets, and the directives and stamps attached to it.
type fingerprint struct {
	digest     [sha256.Size]byte
	directives []directive.Raw
	stamps     []meta.RawStamp
}

// equal reports whether two fingerprints state one declaration.
func (f fingerprint) equal(o fingerprint) bool {
	return f.digest == o.digest &&
		reflect.DeepEqual(f.directives, o.directives) && reflect.DeepEqual(f.stamps, o.stamps)
}

// side is one side of a load's comparison. It contains the fingerprint of
// each declaration, and one fingerprint of a package's own fields and the
// name it declares the package under for each unit that contributes to
// the package. It also contains the raw directives and stamps that the
// units attach to each subject. Each list keeps the order in which the
// units were added.
type side struct {
	decls      map[symbol.Identity]fingerprint
	packages   map[symbol.Identity][]fingerprint
	names      map[symbol.Identity][]string
	directives map[symbol.Identity][]directive.Raw
	stamps     map[symbol.Identity][]meta.RawStamp
}

// newSide returns an empty side.
func newSide() *side {
	return &side{
		decls:      map[symbol.Identity]fingerprint{},
		packages:   map[symbol.Identity][]fingerprint{},
		names:      map[symbol.Identity][]string{},
		directives: map[symbol.Identity][]directive.Raw{},
		stamps:     map[symbol.Identity][]meta.RawStamp{},
	}
}

// add adds the fingerprints of a region's declarations and package parts,
// and the raw directives and stamps that the region attaches.
//
// Error modes: the error of [node.AppendBinary] for a symbol the model
// does not declare.
func (s *side) add(r *store.Region) error {
	for id, raws := range r.Directives {
		s.directives[id] = append(s.directives[id], raws...)
	}
	for id, stamps := range r.Stamps {
		s.stamps[id] = append(s.stamps[id], stamps...)
	}
	for _, p := range r.Packages {
		own := *p
		own.Files = nil
		b, err := node.AppendBinary(nil, &own, nil)
		if err != nil {
			return fmt.Errorf("load: fingerprint %s: %w", p.ID, err)
		}
		s.packages[p.ID] = append(s.packages[p.ID], fingerprint{
			digest: sha256.Sum256(b), directives: r.Directives[p.ID], stamps: r.Stamps[p.ID],
		})
		s.names[p.ID] = append(s.names[p.ID], p.Name)
		for _, f := range p.Files {
			var walkErr error
			node.Walk(f, func(sym symbol.Symbol) bool {
				d, names := sym.(node.Declaration)
				if !names || d.Identity().IsZero() || walkErr != nil {
					return walkErr == nil
				}
				enc, err := node.AppendBinary(nil, sym, nil)
				if err != nil {
					walkErr = fmt.Errorf("load: fingerprint %s: %w", d.Identity(), err)
					return false
				}
				id := d.Identity()
				s.decls[id] = fingerprint{
					digest: sha256.Sum256(enc), directives: r.Directives[id], stamps: r.Stamps[id],
				}
				return true
			})
			if walkErr != nil {
				return walkErr
			}
		}
	}
	return nil
}

// changes compares the regions of the units the load parsed, restored
// or relinked, and of the history's units the load no longer has, with
// the history's regions of the same units, and returns what changed. A
// relinked unit's recorded region decodes again, because the relink
// rewrote the load's copy.
//
// Error modes: the error of a record that does not read whole, and of a
// fingerprint.
func (l *loader) changes(units []*unit, removed []*UnitRecord) (*Changes, error) {
	before, after := newSide(), newSide()
	for _, u := range units {
		if u.from == FromGeneration && !u.relinked {
			continue
		}
		if u.record != nil {
			old, err := l.recordedRegion(u)
			if err == nil {
				err = before.add(old)
			}
			if err != nil {
				return nil, err
			}
		}
		if err := after.add(u.region); err != nil {
			return nil, err
		}
	}
	for _, rec := range removed {
		old, err := l.history.region(rec)
		if err == nil {
			err = before.add(old)
		}
		if err != nil {
			return nil, err
		}
	}
	return compare(before, after), nil
}

// recordedRegion returns the history's region of a unit as the record
// states it: a fresh decode for a relinked unit, and the history's
// decode for any other.
//
// Error modes: the prior's error for a region that does not read whole.
func (l *loader) recordedRegion(u *unit) (*store.Region, error) {
	if !u.relinked {
		return l.history.region(u.record)
	}
	r, err := l.history.prior.Region(*u.record)
	if err != nil {
		return nil, fmt.Errorf("load: read the region of %s: %w", u.record.Files[0].Path, err)
	}
	l.decoded.Add(1)
	return r, nil
}

// compare returns what changed between two sides.
func compare(before, after *side) *Changes {
	c := &Changes{}
	packages := map[symbol.Identity]bool{}
	for id, f := range after.decls {
		was, held := before.decls[id]
		switch {
		case !held:
			c.Appeared = append(c.Appeared, id)
		case !was.equal(f):
			c.Changed = append(c.Changed, id)
		default:
			continue
		}
		packages[id.PackageIdentity()] = true
	}
	for id := range before.decls {
		if _, held := after.decls[id]; !held {
			c.Disappeared = append(c.Disappeared, id)
			packages[id.PackageIdentity()] = true
		}
	}
	for id, parts := range after.packages {
		if !slices.EqualFunc(parts, before.packages[id], fingerprint.equal) {
			packages[id] = true
		}
	}
	for id, parts := range before.packages {
		if _, held := after.packages[id]; !held && len(parts) > 0 {
			packages[id] = true
		}
	}
	for id, names := range after.names {
		if was, held := before.names[id]; held && !slices.Equal(slices.Sorted(slices.Values(was)),
			slices.Sorted(slices.Values(names))) {
			c.Renamed = append(c.Renamed, id)
		}
	}
	slices.SortFunc(c.Renamed, symbol.Identity.Compare)
	slices.SortFunc(c.Appeared, symbol.Identity.Compare)
	slices.SortFunc(c.Disappeared, symbol.Identity.Compare)
	slices.SortFunc(c.Changed, symbol.Identity.Compare)
	c.Packages = slices.SortedFunc(maps.Keys(packages), symbol.Identity.Compare)
	c.Directed = differing(before.directives, after.directives)
	c.Restamped = differing(before.stamps, after.stamps)
	spellings := map[directive.Name]bool{}
	for _, id := range c.Directed {
		had, has := spelled(before.directives[id]), spelled(after.directives[id])
		for n := range had {
			spellings[n] = spellings[n] || !has[n]
		}
		for n := range has {
			spellings[n] = spellings[n] || !had[n]
		}
	}
	for n, moved := range spellings {
		if moved {
			c.Spellings = append(c.Spellings, n)
		}
	}
	slices.Sort(c.Spellings)
	return c
}

// differing returns the subjects whose lists differ between two sides, in
// identity order. A subject that only one side lists is one of them.
func differing[T any](before, after map[symbol.Identity][]T) []symbol.Identity {
	var out []symbol.Identity
	for id, now := range after {
		if !reflect.DeepEqual(before[id], now) {
			out = append(out, id)
		}
	}
	for id := range before {
		if _, held := after[id]; !held {
			out = append(out, id)
		}
	}
	slices.SortFunc(out, symbol.Identity.Compare)
	return out
}

// spelled returns the set of spellings that the raw instances use.
func spelled(raws []directive.Raw) map[directive.Name]bool {
	out := make(map[directive.Name]bool, len(raws))
	for _, r := range raws {
		out[r.Name] = true
	}
	return out
}

// delta returns the bare identities of the declarations a type position
// can name that appeared or disappeared between the history and the
// load, whatever unit declares them: what can move the target of a kept
// reference.
//
// Error modes: the error of a record that does not read whole.
func (l *loader) delta(units []*unit, removed []*UnitRecord) ([]symbol.Identity, error) {
	if l.history.cold() {
		return nil, nil
	}
	before, now := map[symbol.Identity]bool{}, map[symbol.Identity]bool{}
	for _, u := range units {
		if u.from == FromGeneration {
			continue
		}
		var decls []declaration
		if u.parsed() {
			decls = declarationsOf(u.src.Graph().Packages())
		} else {
			decls = declarationsOf(u.region.Packages)
		}
		for _, d := range decls {
			now[d.id] = true
		}
		if u.record != nil {
			old, err := l.history.region(u.record)
			if err != nil {
				return nil, err
			}
			for _, d := range declarationsOf(old.Packages) {
				before[d.id] = true
			}
		}
	}
	for _, rec := range removed {
		old, err := l.history.region(rec)
		if err != nil {
			return nil, err
		}
		for _, d := range declarationsOf(old.Packages) {
			before[d.id] = true
		}
	}
	var out []symbol.Identity
	for id := range now {
		if !before[id] && targetable(id.Kind) {
			out = append(out, bareOf(id))
		}
	}
	for id := range before {
		if !now[id] && targetable(id.Kind) {
			out = append(out, bareOf(id))
		}
	}
	return out, nil
}
