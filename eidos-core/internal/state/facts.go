// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"fmt"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/internal/grow"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// The sizes the fact tables' buffers grow by: the room a row and its key
// take in most runs, which a buffer makes before each row, and the least
// capacities of the buffer of every row's bytes and of the list of rows.
const (
	factRoom  = 256
	factBytes = 4096
	factRows  = 256
)

// Claims returns every claim the generation keeps on a subject's bag, in
// the order [meta.Facts.Bags] yielded them, and none for a subject
// nothing claimed. It is the claims half of the [meta.BagSource] a
// restored fact store reads.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Claims(subject symbol.Identity) ([]meta.StoredClaim, error) {
	row, held, err := s.g.readers[TableClaims].get(s.ctx, identityKey(nil, subject))
	if err != nil || !held {
		return nil, err
	}
	d := newDecoder(row, nil)
	out := make([]meta.StoredClaim, d.Count())
	for i := range out {
		out[i] = d.storedClaim(subject)
	}
	if err := d.Err(); err != nil {
		return nil, fmt.Errorf("%w: the claims row of %s does not decode: %w", ErrDamaged, subject, err)
	}
	return out, nil
}

// Present returns the subjects on which a key read present when the
// generation was recorded, in identity order, and none for a key that
// read present nowhere. It reads the present table once and lists every
// key's subjects, so a later call reads nothing. It is the presence half
// of the [meta.BagSource] a restored fact store reads.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole, and for a key that does not decode.
func (s *PhaseState) Present(k meta.KeyName) ([]symbol.Identity, error) {
	s.presentOnce.Do(func() {
		rows, err := s.g.readers[TablePresent].all(s.ctx)
		if err != nil {
			s.presentErr = err
			return
		}
		s.present = map[meta.KeyName][]symbol.Identity{}
		for _, e := range rows {
			key, rest, found := bytes.Cut(e.key, []byte{keySep})
			subject, parsed := parseIdentityKey(rest)
			if !found || !parsed {
				s.presentErr = fmt.Errorf("%w: a present key does not decode", ErrDamaged)
				return
			}
			name := meta.KeyName(key)
			s.present[name] = append(s.present[name], subject)
		}
	})
	return s.present[k], s.presentErr
}

// claimRows returns the claims table's rows of a fact store, sorted by
// key: each bag that keeps a claim under its subject's identity key. It
// encodes every key and row into one buffer, which doubles when it
// fills, and makes the room a row of most runs takes before it encodes
// the row.
//
// Error modes: the error of a value outside the fact vocabulary.
func claimRows(facts *meta.Facts) ([]entry, error) {
	var (
		rows []entry
		e    encoder
	)
	for subject, claims := range facts.Bags() {
		start := len(e.buf)
		e.buf = identityKey(grow.Room(e.buf, factRoom, factBytes), subject)
		keyEnd := len(e.buf)
		e.uvarint(uint64(len(claims)))
		for i := range claims {
			if err := e.storedClaim(&claims[i]); err != nil {
				return nil, fmt.Errorf("state: record the claims on %s: %w", subject, err)
			}
		}
		rows = append(grow.Room(rows, 1, factRows), entry{
			key: e.buf[start:keyEnd:keyEnd], row: e.buf[keyEnd:len(e.buf):len(e.buf)],
		})
	}
	return sortedRows(rows), nil
}

// presentRows returns the present table's rows of a fact store, sorted
// by key: one row for each fact that reads present, under its key's name
// and its subject's identity key, with no value. Every name the registry
// lists resolves. It encodes every key into one buffer, which doubles
// when it fills.
func presentRows(facts *meta.Facts) []entry {
	var (
		rows []entry
		keys []byte
	)
	r := facts.Registry()
	for name := range r.Keys() {
		k, _ := r.Resolve(name)
		for subject := range facts.ByKey(k) {
			start := len(keys)
			keys = append(append(grow.Room(keys, factRoom, factBytes), name...), keySep)
			keys = identityKey(keys, subject)
			rows = append(grow.Room(rows, 1, factRows), entry{key: keys[start:len(keys):len(keys)], row: []byte{}})
		}
	}
	return sortedRows(rows)
}

// storedClaim writes one recorded claim, without its subject, which the
// row's key states: its key or its group, its envelope, whether it is a
// drop, and its value.
//
// Error modes: the error of [encoder.value] for a value outside the fact
// vocabulary.
func (e *encoder) storedClaim(c *meta.StoredClaim) error {
	e.text(string(c.Key))
	e.text(string(c.Group))
	e.uvarint(uint64(c.Claim.Authority))
	e.varint(int64(c.Claim.Bucket))
	e.text(string(c.Claim.Plugin))
	e.varint(int64(c.Claim.Order.Rule))
	e.identity(c.Claim.Order.Subject)
	e.varint(int64(c.Claim.Order.Instance))
	e.pos(c.Claim.Pos)
	e.uvarint(uint64(len(c.Claim.Derived)))
	for _, r := range c.Claim.Derived {
		e.identity(r.Subject)
		e.text(string(r.Key))
	}
	e.boolean(c.Drop)
	return e.value(c.Value)
}

// storedClaim reads one recorded claim on a subject.
func (d *decoder) storedClaim(subject symbol.Identity) meta.StoredClaim {
	out := meta.StoredClaim{Key: meta.KeyName(d.text()), Group: meta.GroupName(d.text())}
	out.Claim = meta.Claim{
		Subject:   subject,
		Authority: meta.Authority(d.Uvarint()),
		Bucket:    int(d.Varint()),
		Plugin:    diag.Origin(d.text()),
		Order:     meta.Order{Rule: int(d.Varint()), Subject: d.identity(), Instance: int(d.Varint())},
		Pos:       d.pos(),
	}
	if n := d.Count(); n > 0 {
		out.Claim.Derived = make([]meta.Read, n)
		for i := range out.Claim.Derived {
			out.Claim.Derived[i] = meta.Read{Subject: d.identity(), Key: meta.KeyName(d.text())}
		}
	}
	out.Drop = d.Bool()
	out.Value = d.value()
	return out
}
