// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"slices"
	"sync"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// AppendRegion appends a region's blob to dst: the region's string
// table, then its body, then a CRC-32C over both. The body is the
// packages, each through [node.AppendBinary]; the directives and the
// stamps, each subject in identity order; the links; and the findings.
// Every string is a number in the region's own table, so a region
// decodes without any other blob, in a generation or in the memo, and
// one region encodes to the same bytes on every run.
//
// Error modes: the error of [node.AppendBinary] for a symbol the model
// does not declare, and an error for a stamp value outside the fact
// vocabulary. dst comes back unchanged with either.
//
// # Allocation contract
//
// AppendRegion takes its string table, its body's buffer and the list
// it sorts subjects in from a pool and returns them, so an encoding
// allocates only to grow them past a region larger than any before, and
// to grow dst.
func AppendRegion(dst []byte, r *store.Region) ([]byte, error) {
	s, _ := scratches.Get().(*scratch)
	defer scratches.Put(s)
	s.table.Reset()
	e := encoder{buf: s.body[:0], t: &s.table}
	err := e.region(r, &s.subjects)
	s.body = e.buf
	if err != nil {
		return dst, err
	}
	start := len(dst)
	dst, _ = s.table.AppendBinary(dst)
	dst = append(dst, e.buf...)
	return binary.LittleEndian.AppendUint32(dst, crc32.Checksum(dst[start:], castagnoli)), nil
}

// scratch is one region encoding's string table, body buffer and the
// list its subjects sort in, recycled across encodings through
// [scratches].
type scratch struct {
	table    node.StringTable
	body     []byte
	subjects []symbol.Identity
}

// scratches recycles the scratch of region encodings.
var scratches = sync.Pool{New: func() any { return new(scratch) }}

// DecodeRegion decodes a blob [AppendRegion] wrote.
//
// Error modes: an error wrapping [ErrDamaged] for a blob shorter than
// its trailer, whose CRC-32C does not match, whose string table or body
// does not decode, or that has bytes after its body.
//
// # Allocation contract
//
// DecodeRegion allocates the string table, every declaration and list
// of the region, and its maps. Its strings are substrings of the
// table's one string.
func DecodeRegion(b []byte) (*store.Region, error) {
	if len(b) < trailerSize {
		return nil, fmt.Errorf("%w: a region of %d bytes has no trailer", ErrDamaged, len(b))
	}
	body := b[:len(b)-trailerSize]
	if crc32.Checksum(body, castagnoli) != binary.LittleEndian.Uint32(b[len(body):]) {
		return nil, fmt.Errorf("%w: a region fails its CRC-32C", ErrDamaged)
	}
	t, read, err := node.DecodeStringTable(body)
	if err != nil {
		return nil, fmt.Errorf("%w: a region's string table: %w", ErrDamaged, err)
	}
	d := newDecoder(body[read:], t)
	r := d.region()
	if d.Err() == nil && d.Len() > 0 {
		d.Fail(fmt.Errorf("%d bytes follow the region's body", d.Len()))
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: a region does not decode: %w", ErrDamaged, err)
	}
	return r, nil
}

// region writes a region's body, sorting the subjects of its directives
// and its stamps in the list subjects points at.
func (e *encoder) region(r *store.Region, subjects *[]symbol.Identity) error {
	e.uvarint(uint64(len(r.Packages)))
	for _, p := range r.Packages {
		var err error
		if e.buf, err = node.AppendBinary(e.buf, p, e.t); err != nil {
			return err
		}
	}
	*subjects = sortedKeys(*subjects, r.Directives)
	e.uvarint(uint64(len(*subjects)))
	for _, subject := range *subjects {
		e.identity(subject)
		raws := r.Directives[subject]
		e.uvarint(uint64(len(raws)))
		for _, raw := range raws {
			e.raw(raw)
		}
	}
	*subjects = sortedKeys(*subjects, r.Stamps)
	e.uvarint(uint64(len(*subjects)))
	for _, subject := range *subjects {
		e.identity(subject)
		stamps := r.Stamps[subject]
		e.uvarint(uint64(len(stamps)))
		for _, s := range stamps {
			if err := e.stamp(s); err != nil {
				return err
			}
		}
	}
	e.uvarint(uint64(len(r.Links)))
	for _, l := range r.Links {
		e.uvarint(uint64(l.Ref))
		e.uvarint(uint64(len(l.Tiers)))
		for _, tier := range l.Tiers {
			e.identities(tier)
		}
		e.identities(l.Followed)
		e.identities(l.Reached)
		e.findings(l.Findings)
	}
	e.findings(r.Findings)
	return nil
}

// region reads a region's body.
func (d *decoder) region() *store.Region {
	r := &store.Region{}
	if n := d.Count(); n > 0 {
		r.Packages = make([]*node.Package, n)
		for i := range r.Packages {
			r.Packages[i] = d.pkg()
		}
	}
	if n := d.Count(); n > 0 {
		r.Directives = make(map[symbol.Identity][]directive.Raw, n)
		for range n {
			subject := d.identity()
			raws := make([]directive.Raw, d.Count())
			for i := range raws {
				raws[i] = d.raw()
			}
			r.Directives[subject] = raws
		}
	}
	if n := d.Count(); n > 0 {
		r.Stamps = make(map[symbol.Identity][]meta.RawStamp, n)
		for range n {
			subject := d.identity()
			stamps := make([]meta.RawStamp, d.Count())
			for i := range stamps {
				stamps[i] = d.stamp()
			}
			r.Stamps[subject] = stamps
		}
	}
	if n := d.Count(); n > 0 {
		r.Links = make([]store.Link, n)
		for i := range r.Links {
			l := store.Link{Ref: int(d.Uvarint())}
			if tiers := d.Count(); tiers > 0 {
				l.Tiers = make([][]symbol.Identity, tiers)
				for j := range l.Tiers {
					l.Tiers[j] = d.identities()
				}
			}
			l.Followed = d.identities()
			l.Reached = d.identities()
			l.Findings = d.findings()
			r.Links[i] = l
		}
	}
	r.Findings = d.findings()
	return r
}

// pkg reads one package through [node.DecodeBinary] against the
// decoder's table.
func (d *decoder) pkg() *node.Package {
	s, read, err := node.DecodeBinary(d.Rest(), d.t)
	if err != nil {
		d.Fail(err)
		return nil
	}
	d.Skip(read)
	p, is := s.(*node.Package)
	if !is {
		d.Fail(fmt.Errorf("a region holds a %v where a package belongs", kindOf(s)))
		return nil
	}
	return p
}

// sortedKeys returns a map's subjects in identity order, in dst's
// storage.
func sortedKeys[V any](dst []symbol.Identity, m map[symbol.Identity]V) []symbol.Identity {
	dst = dst[:0]
	for subject := range m {
		dst = append(dst, subject)
	}
	slices.SortFunc(dst, symbol.Identity.Compare)
	return dst
}

// kindOf returns a symbol's kind, and the invalid kind for nil.
func kindOf(s symbol.Symbol) symbol.Kind {
	if s == nil {
		return symbol.KindInvalid
	}
	return s.Kind()
}
