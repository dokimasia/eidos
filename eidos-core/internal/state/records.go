// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tag a record's ID spelling opens with, one for each kind, so two
// records of different kinds never spell alike.
const (
	tagValidation = 'v'
	tagInvocation = 'i'
	tagCheck      = 'c'
	tagGroup      = 'g'
)

// refSize is the size of a record's reference in a readers row: its
// kind's byte and its ID's eight bytes.
const refSize = 9

// RecordKind names the kind of execution a record keeps.
type RecordKind uint8

// The record kinds, numbered from one, so the zero reference names no
// record.
const (
	// RecordValidation is the validation of one subject's directives.
	RecordValidation RecordKind = 1
	// RecordInvocation is one invocation of a phase call.
	RecordInvocation RecordKind = 2
	// RecordCheck is one call of a workspace check.
	RecordCheck RecordKind = 3
	// RecordGroup is one group of a plan's files: the files that execute
	// together.
	RecordGroup RecordKind = 4
)

// RecordRef names one record: its kind, and its ID, the first eight bytes
// of the SHA-256 of a tag naming the kind and the record encoding of the
// record's key fields. A readers row lists the records that read an edge
// by reference, and the records' tables are keyed by ID. Two records that
// share an ID share a row, and each keeps its own entry in it.
type RecordRef struct {
	Kind RecordKind
	ID   uint64
}

// ValidationRef returns the reference of the validation of a subject's
// directives. It allocates nothing for an identity whose spelling fits in
// 255 bytes.
func ValidationRef(subject symbol.Identity) RecordRef {
	var spelling [spellingCap]byte
	h := hashOf(appendIdentity(append(spelling[:0], tagValidation), subject))
	return RecordRef{Kind: RecordValidation, ID: uint64(h)}
}

// InvocationRef returns the reference of one invocation: a plan's, or an
// annotator's under the empty plan, and its match. It allocates nothing
// for a spelling that fits in 255 bytes.
func InvocationRef(plan string, m plugin.MatchKey) RecordRef {
	var spelling [spellingCap]byte
	h := hashOf(appendInvocationKey(append(spelling[:0], tagInvocation), plan, m))
	return RecordRef{Kind: RecordInvocation, ID: uint64(h)}
}

// CheckRef returns the reference of a workspace check's call. It
// allocates nothing for a name that fits in 250 bytes.
func CheckRef(name plugin.ID) RecordRef {
	var spelling [spellingCap]byte
	h := hashOf(wire.AppendText(append(spelling[:0], tagCheck), string(name)))
	return RecordRef{Kind: RecordCheck, ID: uint64(h)}
}

// GroupRef returns the reference of the group of a plan that the unit
// key names: the group's first unit in [plugin.UnitRef.Compare] order. It
// allocates nothing for a spelling that fits in 255 bytes.
func GroupRef(plan string, key plugin.UnitRef) RecordRef {
	var spelling [spellingCap]byte
	h := hashOf(appendUnitRef(wire.AppendText(append(spelling[:0], tagGroup), plan), key))
	return RecordRef{Kind: RecordGroup, ID: uint64(h)}
}

// Compare orders two references by kind, then by ID, and returns a
// negative number, zero or a positive number as r sorts before, with or
// after o. It allocates nothing.
func (r RecordRef) Compare(o RecordRef) int {
	return cmp.Or(cmp.Compare(r.Kind, o.Kind), cmp.Compare(r.ID, o.ID))
}

// Validation is the record of one subject's directive validation: the
// directives that passed, in the order validation returned them, the
// edges the validation read, the subject's own declaration edge
// included, and the findings it reported. A subject the graph does not
// contain validates nothing, and its record keeps the finding that
// reports the missing subject.
type Validation struct {
	Subject    symbol.Identity
	Directives []directive.Directive
	Reads      []EdgeHash
	Findings   []diag.Diag
}

// Invocation is the record of one invocation of a phase call: the
// plan, empty for an annotator, the match, the edges the invocation read,
// its subject's declaration edge included, and what it touched and
// reported, as [plugin.Invocation] states them. A phase call that
// journals no invocation of its own is one invocation under
// [plugin.WholeCall].
type Invocation struct {
	Plan     string
	Match    plugin.MatchKey
	Reads    []EdgeHash
	Exports  []string
	Units    []plugin.UnitRef
	Hosts    []plugin.EmitRef
	Claimed  []meta.FactRef
	Findings []diag.Diag
}

// Check is the record of one workspace check's call: the check, the
// edges its reader read, and the findings it reported.
type Check struct {
	Name     plugin.ID
	Reads    []EdgeHash
	Findings []diag.Diag
}

// Group is the record of one group of a plan's files: the files that
// execute together, because a backend split one unit into them, two units
// share one of them, or an emit-phase invocation matched a value of one
// and placed declarations into another.
type Group struct {
	// Plan is the group's plan, and Key the group's first unit in
	// [plugin.UnitRef.Compare] order, which names the group.
	Plan string
	Key  plugin.UnitRef
	// Units are the group's units, and Files the paths of its files,
	// each sorted.
	Units []plugin.UnitRef
	Files []string
	// Contributors are the invocations that placed a declaration into a
	// unit of the group or appended into a slot of one, in canonical
	// match order.
	Contributors []plugin.MatchKey
	// Reads are the edges that the group read beyond the reads of its
	// invocations: the edge of each of its units, and what its settle,
	// its routing and its render read.
	Reads []EdgeHash
	// Findings are the findings that the render reported for the
	// group's files.
	Findings []diag.Diag
}

// Validation returns the record of a subject's validation, and false
// where the generation has none.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Validation(subject symbol.Identity) (Validation, bool, error) {
	var out Validation
	found, err := s.lookup(TableValidations, ValidationRef(subject), func(d *decoder) bool {
		if d.identity() != subject {
			return false
		}
		out = decodeValidation(d, subject)
		return true
	})
	return out, found, err
}

// Invocation returns the record of one invocation of a plan, the empty
// plan for an annotator's, and false where the generation has none.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Invocation(plan string, m plugin.MatchKey) (Invocation, bool, error) {
	var out Invocation
	found, err := s.lookup(TableInvocations, InvocationRef(plan, m), func(d *decoder) bool {
		gotPlan, gotMatch := d.text(), d.match()
		if gotPlan != plan || gotMatch.Compare(m) != 0 {
			return false
		}
		out = decodeInvocation(d, gotPlan, gotMatch)
		return true
	})
	return out, found, err
}

// Check returns the record of a workspace check's call, and false where
// the generation has none.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Check(name plugin.ID) (Check, bool, error) {
	var out Check
	found, err := s.lookup(TableChecks, CheckRef(name), func(d *decoder) bool {
		if plugin.ID(d.text()) != name {
			return false
		}
		out = Check{Name: name, Reads: d.edges(), Findings: d.findings()}
		return true
	})
	return out, found, err
}

// Group returns the record of the group of a plan that a unit key names,
// and false where the generation has none.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Group(plan string, key plugin.UnitRef) (Group, bool, error) {
	var out Group
	found, err := s.lookup(TableGroups, GroupRef(plan, key), func(d *decoder) bool {
		gotPlan, gotKey := d.text(), d.unitRef()
		if gotPlan != plan || gotKey.Compare(key) != 0 {
			return false
		}
		out = decodeGroup(d, gotPlan, gotKey)
		return true
	})
	return out, found, err
}

// Groups returns every group that the generation keeps under the ID of a
// reference, in row order. A readers row names records by reference, and
// Groups reads the groups that such a reference names. Records whose IDs
// collide share one row, so the caller checks the plan and the key of
// each record.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Groups(ref RecordRef) ([]Group, error) {
	var out []Group
	err := s.entries(TableGroups, ref.ID, func(d *decoder) {
		plan, key := d.text(), d.unitRef()
		out = append(out, decodeGroup(d, plan, key))
	})
	return out, err
}

// Validations returns every validation that the generation keeps under
// the ID of a reference, in row order. A readers row names records by
// reference, and Validations reads the validations that such a reference
// names. Records whose IDs collide share one row, so the caller checks
// the subject of each record.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Validations(ref RecordRef) ([]Validation, error) {
	var out []Validation
	err := s.entries(TableValidations, ref.ID, func(d *decoder) {
		out = append(out, decodeValidation(d, d.identity()))
	})
	return out, err
}

// Invocations returns every invocation that the generation keeps under
// the ID of a reference, in row order. A readers row names records by
// reference, and Invocations reads the invocations that such a reference
// names. Records whose IDs collide share one row, so the caller checks
// the plan and the match of each record.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Invocations(ref RecordRef) ([]Invocation, error) {
	var out []Invocation
	err := s.entries(TableInvocations, ref.ID, func(d *decoder) {
		plan, m := d.text(), d.match()
		out = append(out, decodeInvocation(d, plan, m))
	})
	return out, err
}

// Checks returns every check call that the generation keeps under the ID
// of a reference, in row order. A readers row names records by reference,
// and Checks reads the checks that such a reference names. Records whose
// IDs collide share one row, so the caller checks the name of each record.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Checks(ref RecordRef) ([]Check, error) {
	var out []Check
	err := s.entries(TableChecks, ref.ID, func(d *decoder) {
		name := plugin.ID(d.text())
		out = append(out, Check{Name: name, Reads: d.edges(), Findings: d.findings()})
	})
	return out, err
}

// Readers returns the records whose read record lists an edge's hash,
// sorted, and none for an edge nothing read.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) Readers(h EdgeHash) ([]RecordRef, error) {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], uint64(h))
	row, held, err := s.g.readers[TableReaders].get(s.ctx, key[:])
	if err != nil || !held {
		return nil, err
	}
	refs, err := decodeReaders(row)
	if err != nil {
		return nil, fmt.Errorf("%w: the readers row of edge %016x does not decode: %w", ErrDamaged, uint64(h), err)
	}
	return refs, nil
}

// lookup reads the row of one record's ID and hands each entry's decoder
// to match, which decodes the entry's key fields and reports whether they
// are the record's, and then decodes the rest. It reports whether an
// entry matched.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) lookup(t Table, ref RecordRef, match func(*decoder) bool) (bool, error) {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], ref.ID)
	row, held, err := s.g.readers[t].get(s.ctx, key[:])
	if err != nil || !held {
		return false, err
	}
	rows := newDecoder(row, nil)
	for range rows.Count() {
		d := newDecoder(rows.Bytes(), nil)
		matched := match(d)
		if err := d.Err(); err != nil {
			return false, fmt.Errorf("%w: a %s row does not decode: %w", ErrDamaged, t, err)
		}
		if matched {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("%w: a %s row does not decode: %w", ErrDamaged, t, err)
	}
	return false, nil
}

// entries reads the row of one ID and passes a decoder over each entry to
// decode, which decodes the whole entry. For an ID that the table does
// not contain, entries decodes nothing.
//
// Error modes: an error wrapping [ErrDamaged] for a row that does not
// read whole or does not decode.
func (s *PhaseState) entries(t Table, id uint64, decode func(*decoder)) error {
	var key [8]byte
	binary.BigEndian.PutUint64(key[:], id)
	row, held, err := s.g.readers[t].get(s.ctx, key[:])
	if err != nil || !held {
		return err
	}
	rows := newDecoder(row, nil)
	for range rows.Count() {
		d := newDecoder(rows.Bytes(), nil)
		decode(d)
		if err := d.Err(); err != nil {
			return fmt.Errorf("%w: a %s row does not decode: %w", ErrDamaged, t, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: a %s row does not decode: %w", ErrDamaged, t, err)
	}
	return nil
}

// appendReaders appends a readers row to dst: the number of records,
// then each record's kind and ID, in the order of reads, the reads of one
// edge sorted by record.
func appendReaders(dst []byte, reads []edgeRead) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(reads)))
	for _, r := range reads {
		dst = binary.BigEndian.AppendUint64(append(dst, byte(r.ref.Kind)), r.ref.ID)
	}
	return dst
}

// decodeReaders decodes a readers row.
//
// Error modes: an error wrapping [wire.ErrMalformed] for a row that does
// not decode, and for a kind outside the record kinds.
func decodeReaders(row []byte) ([]RecordRef, error) {
	d := newDecoder(row, nil)
	n := d.Count()
	if d.Err() == nil && n*refSize != d.Len() {
		d.Fail(fmt.Errorf("%w: a readers row of %d records has %d bytes", wire.ErrMalformed, n, d.Len()))
	}
	out := make([]RecordRef, 0, n)
	for rest := d.Rest(); len(rest) >= refSize && d.Err() == nil; rest = rest[refSize:] {
		kind := RecordKind(rest[0])
		if kind < RecordValidation || kind > RecordGroup {
			d.Fail(fmt.Errorf("%w: record kind %d", wire.ErrMalformed, kind))
			break
		}
		out = append(out, RecordRef{Kind: kind, ID: binary.BigEndian.Uint64(rest[1:refSize])})
	}
	return out, d.Err()
}

// appendValidation appends a validation's entry to dst: the subject,
// then the directives, the reads and the findings.
func appendValidation(dst []byte, v *Validation) []byte {
	e := encoder{buf: dst}
	e.identity(v.Subject)
	e.uvarint(uint64(len(v.Directives)))
	for i := range v.Directives {
		e.directive(&v.Directives[i])
	}
	e.edges(v.Reads)
	e.findings(v.Findings)
	return e.buf
}

// decodeValidation decodes the rest of a validation's entry, after its
// subject.
func decodeValidation(d *decoder, subject symbol.Identity) Validation {
	out := Validation{Subject: subject}
	if n := d.Count(); n > 0 {
		out.Directives = make([]directive.Directive, n)
		for i := range out.Directives {
			out.Directives[i] = d.directive()
		}
	}
	out.Reads = d.edges()
	out.Findings = d.findings()
	return out
}

// appendInvocation appends an invocation's entry to dst: the plan and
// the match, then the reads and what the invocation touched and
// reported.
func appendInvocation(dst []byte, inv *Invocation) []byte {
	e := encoder{buf: appendInvocationKey(dst, inv.Plan, inv.Match)}
	e.invocationBody(inv)
	return e.buf
}

// appendInvocationKey appends an invocation's key fields to dst: its
// plan, then its match's plugin, rule, subject, instance and host. It
// keeps nothing of dst, so a spelling built over a buffer on the stack
// does not move to the heap.
func appendInvocationKey(dst []byte, plan string, m plugin.MatchKey) []byte {
	return appendMatchKey(wire.AppendText(dst, plan), m)
}

// appendMatchKey appends a match's key fields to dst: its plugin, rule,
// subject, instance and host, as [decoder.match] reads them. It keeps
// nothing of dst.
func appendMatchKey(dst []byte, m plugin.MatchKey) []byte {
	dst = wire.AppendText(dst, string(m.Plugin))
	dst = binary.AppendVarint(dst, int64(m.Rule))
	dst = appendIdentity(dst, m.Subject)
	dst = binary.AppendVarint(dst, int64(m.Instance))
	dst = appendUnitRef(dst, m.Host.Unit)
	return binary.AppendVarint(dst, int64(m.Host.Index))
}

// appendUnitRef appends a unit's reference to dst: its plugin, its
// family's tag, its package and its key.
func appendUnitRef(dst []byte, u plugin.UnitRef) []byte {
	dst = wire.AppendText(dst, string(u.Plugin))
	dst = wire.AppendText(dst, u.Tag)
	dst = appendIdentity(dst, u.Pkg)
	return wire.AppendText(dst, u.Key)
}

// decodeInvocation decodes the rest of an invocation's entry, after its
// plan and its match.
func decodeInvocation(d *decoder, plan string, m plugin.MatchKey) Invocation {
	out := Invocation{Plan: plan, Match: m, Reads: d.edges(), Exports: d.texts()}
	if n := d.Count(); n > 0 {
		out.Units = make([]plugin.UnitRef, n)
		for i := range out.Units {
			out.Units[i] = d.unitRef()
		}
	}
	if n := d.Count(); n > 0 {
		out.Hosts = make([]plugin.EmitRef, n)
		for i := range out.Hosts {
			out.Hosts[i] = plugin.EmitRef{Unit: d.unitRef(), Index: int(d.Varint())}
		}
	}
	if n := d.Count(); n > 0 {
		out.Claimed = make([]meta.FactRef, n)
		for i := range out.Claimed {
			out.Claimed[i] = meta.FactRef{Subject: d.identity(), Key: meta.KeyName(d.text())}
		}
	}
	out.Findings = d.findings()
	return out
}

// appendCheck appends a check's entry to dst: the check's name, then the
// reads and the findings.
func appendCheck(dst []byte, c *Check) []byte {
	e := encoder{buf: dst}
	e.text(string(c.Name))
	e.edges(c.Reads)
	e.findings(c.Findings)
	return e.buf
}

// appendGroup appends a group's entry to dst: the plan and the key unit,
// then the units, the files, the contributors, the reads and the
// findings.
func appendGroup(dst []byte, g *Group) []byte {
	e := encoder{buf: appendUnitRef(wire.AppendText(dst, g.Plan), g.Key)}
	e.uvarint(uint64(len(g.Units)))
	for _, u := range g.Units {
		e.buf = appendUnitRef(e.buf, u)
	}
	e.texts(g.Files)
	e.uvarint(uint64(len(g.Contributors)))
	for i := range g.Contributors {
		e.buf = appendMatchKey(e.buf, g.Contributors[i])
	}
	e.edges(g.Reads)
	e.findings(g.Findings)
	return e.buf
}

// decodeGroup decodes the rest of a group's entry, after its plan and
// its key unit.
func decodeGroup(d *decoder, plan string, key plugin.UnitRef) Group {
	out := Group{Plan: plan, Key: key}
	if n := d.Count(); n > 0 {
		out.Units = make([]plugin.UnitRef, n)
		for i := range out.Units {
			out.Units[i] = d.unitRef()
		}
	}
	out.Files = d.texts()
	if n := d.Count(); n > 0 {
		out.Contributors = make([]plugin.MatchKey, n)
		for i := range out.Contributors {
			out.Contributors[i] = d.match()
		}
	}
	out.Reads = d.edges()
	out.Findings = d.findings()
	return out
}

// invocationBody writes the rest of an invocation's entry: the reads,
// the exports, the units, the hosts, the claimed facts and the findings.
func (e *encoder) invocationBody(inv *Invocation) {
	e.edges(inv.Reads)
	e.texts(inv.Exports)
	e.uvarint(uint64(len(inv.Units)))
	for _, u := range inv.Units {
		e.buf = appendUnitRef(e.buf, u)
	}
	e.uvarint(uint64(len(inv.Hosts)))
	for _, h := range inv.Hosts {
		e.buf = appendUnitRef(e.buf, h.Unit)
		e.varint(int64(h.Index))
	}
	e.uvarint(uint64(len(inv.Claimed)))
	for _, f := range inv.Claimed {
		e.identity(f.Subject)
		e.text(string(f.Key))
	}
	e.findings(inv.Findings)
}

// edges writes a read record: its length, then each hash's eight bytes,
// big-endian.
func (e *encoder) edges(hs []EdgeHash) {
	e.uvarint(uint64(len(hs)))
	for _, h := range hs {
		e.buf = binary.BigEndian.AppendUint64(e.buf, uint64(h))
	}
}

// directive writes one validated directive: its name, its variant, its
// positional values, its keyed values sorted by key, its role, its
// position, its instance, whether it is negated and whether its schema
// overrides.
func (e *encoder) directive(d *directive.Directive) {
	e.text(string(d.Name))
	e.text(d.Variant)
	e.uvarint(uint64(len(d.Args)))
	for _, v := range d.Args {
		e.param(v)
	}
	e.uvarint(uint64(len(d.Params)))
	for _, k := range slices.Sorted(maps.Keys(d.Params)) {
		e.text(string(k))
		e.param(d.Params[k])
	}
	e.text(d.Role)
	e.pos(d.Pos)
	e.varint(int64(d.Instance))
	e.boolean(d.Negated)
	e.boolean(d.Overrides)
}

// param writes one validated value whole: its type and every field, a
// list's elements each in turn. Validation fills only the field the type
// names, so the others cost a byte each, and a value reads back as it was
// written whatever its type.
func (e *encoder) param(v directive.Value) {
	e.uvarint(uint64(v.Kind))
	e.text(v.Str)
	e.varint(v.Int)
	e.boolean(v.Bool)
	e.uvarint(uint64(len(v.List)))
	for _, item := range v.List {
		e.param(item)
	}
	e.text(v.Ref)
	e.identity(v.Target)
}

// match reads a match's key: its plugin, its rule, its subject, its
// instance and its host.
func (d *decoder) match() plugin.MatchKey {
	return plugin.MatchKey{
		Plugin:   plugin.ID(d.text()),
		Rule:     plugin.RuleID(d.Varint()),
		Subject:  d.identity(),
		Instance: int(d.Varint()),
		Host:     plugin.EmitRef{Unit: d.unitRef(), Index: int(d.Varint())},
	}
}

// edges reads a read record, nil for an empty one.
func (d *decoder) edges() []EdgeHash {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]EdgeHash, 0, n)
	for range n {
		rest := d.Rest()
		if len(rest) < 8 {
			d.Fail(fmt.Errorf("%w: a read record ends inside a hash", wire.ErrMalformed))
			return nil
		}
		out = append(out, EdgeHash(binary.BigEndian.Uint64(rest)))
		d.Skip(8)
	}
	return out
}

// unitRef reads a unit's reference.
func (d *decoder) unitRef() plugin.UnitRef {
	return plugin.UnitRef{Plugin: plugin.ID(d.text()), Tag: d.text(), Pkg: d.identity(), Key: d.text()}
}

// directive reads one validated directive. A directive written with no
// keyed value reads with an empty map, as validation builds one.
func (d *decoder) directive() directive.Directive {
	out := directive.Directive{Name: directive.Name(d.text()), Variant: d.text()}
	if n := d.Count(); n > 0 {
		out.Args = make([]directive.Value, n)
		for i := range out.Args {
			out.Args[i] = d.param()
		}
	}
	n := d.Count()
	out.Params = make(map[directive.ParamKey]directive.Value, n)
	for range n {
		k := directive.ParamKey(d.text())
		out.Params[k] = d.param()
	}
	out.Role = d.text()
	out.Pos = d.pos()
	out.Instance = int(d.Varint())
	out.Negated = d.Bool()
	out.Overrides = d.Bool()
	return out
}

// param reads one validated value, a list with no element as nil.
func (d *decoder) param() directive.Value {
	out := directive.Value{
		Kind: directive.ParamType(d.Uvarint()),
		Str:  d.text(),
		Int:  d.Varint(),
		Bool: d.Bool(),
	}
	if n := d.Count(); n > 0 {
		out.List = make([]directive.Value, n)
		for i := range out.List {
			out.List[i] = d.param()
		}
	}
	out.Ref = d.text()
	out.Target = d.identity()
	return out
}
