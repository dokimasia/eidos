// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/node"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tag a stored value states its type with: nothing, or one of the
// fact vocabulary's five types.
const (
	valueNone     = 0
	valueString   = 1
	valueInt      = 2
	valueBool     = 3
	valueStrings  = 4
	valueIdentity = 5
)

// encoder appends one record's values to buf in the kernel's record
// format. A string is its number in t, or, where t is nil, its length
// and its bytes. An instant is a presence byte and then its Unix
// nanoseconds, so the zero instant has an encoding of its own.
type encoder struct {
	buf []byte
	t   *node.StringTable
}

// text writes a string.
func (e *encoder) text(s string) {
	if e.t != nil {
		e.buf = binary.AppendUvarint(e.buf, e.t.Add(s))
		return
	}
	e.buf = wire.AppendText(e.buf, s)
}

// uvarint writes an unsigned integer.
func (e *encoder) uvarint(u uint64) { e.buf = binary.AppendUvarint(e.buf, u) }

// varint writes a signed integer.
func (e *encoder) varint(i int64) { e.buf = binary.AppendVarint(e.buf, i) }

// boolean writes a bool.
func (e *encoder) boolean(b bool) { e.buf = wire.AppendBool(e.buf, b) }

// digest writes a SHA-256 digest's 32 bytes.
func (e *encoder) digest(d [sha256.Size]byte) { e.buf = append(e.buf, d[:]...) }

// bytes writes a byte string.
func (e *encoder) bytes(b []byte) { e.buf = wire.AppendBytes(e.buf, b) }

// instant writes an instant.
func (e *encoder) instant(t time.Time) {
	e.boolean(!t.IsZero())
	if !t.IsZero() {
		e.varint(t.UnixNano())
	}
}

// texts writes a list of strings.
func (e *encoder) texts(ss []string) {
	e.uvarint(uint64(len(ss)))
	for _, s := range ss {
		e.text(s)
	}
}

// identity writes an identity's six fields.
func (e *encoder) identity(id symbol.Identity) {
	e.text(string(id.Lang))
	e.text(id.Package)
	e.text(id.Owner)
	e.text(id.Name)
	e.uvarint(uint64(id.Kind))
	e.text(id.Disc)
}

// identities writes a list of identities.
func (e *encoder) identities(ids []symbol.Identity) {
	e.uvarint(uint64(len(ids)))
	for _, id := range ids {
		e.identity(id)
	}
}

// pos writes a position.
func (e *encoder) pos(p position.Pos) {
	e.text(p.File)
	e.varint(int64(p.Line))
	e.varint(int64(p.Col))
}

// finding writes one finding: its code, severity, position, message,
// origin and related positions.
func (e *encoder) finding(d diag.Diag) {
	e.text(string(d.Code.Prefix))
	e.uvarint(uint64(d.Code.Number))
	e.uvarint(uint64(d.Severity))
	e.pos(d.Pos)
	e.text(d.Msg)
	e.text(string(d.Origin))
	e.uvarint(uint64(len(d.Related)))
	for _, p := range d.Related {
		e.pos(p)
	}
}

// findings writes a list of findings.
func (e *encoder) findings(ds []diag.Diag) {
	e.uvarint(uint64(len(ds)))
	for _, d := range ds {
		e.finding(d)
	}
}

// raw writes one raw directive instance.
func (e *encoder) raw(r directive.Raw) {
	e.text(string(r.Name))
	e.uvarint(uint64(len(r.Args)))
	for _, a := range r.Args {
		e.text(a.Key)
		e.rawValue(a.Value)
		e.varint(int64(a.Col))
	}
	e.pos(r.Pos)
	e.boolean(r.Negated)
	e.boolean(r.DirectiveShaped)
}

// rawValue writes one argument value and its list elements.
func (e *encoder) rawValue(v directive.RawValue) {
	e.text(v.Text)
	e.boolean(v.Quoted)
	e.uvarint(uint64(len(v.List)))
	for _, item := range v.List {
		e.rawValue(item)
	}
}

// stamp writes one raw classification stamp.
//
// Error modes: the error of [encoder.value] for a value outside the
// fact vocabulary.
func (e *encoder) stamp(s meta.RawStamp) error {
	e.text(string(s.Key))
	if err := e.value(s.Value); err != nil {
		return err
	}
	e.pos(s.Pos)
	e.text(string(s.Origin))
	return nil
}

// value writes a value of the fact vocabulary behind its tag, and the
// none tag for nil.
//
// Error modes: an error naming the type of a value outside the
// vocabulary: a string, an int64, a bool, a []string or an identity.
func (e *encoder) value(v any) error {
	switch x := v.(type) {
	case nil:
		e.buf = append(e.buf, valueNone)
	case string:
		e.buf = append(e.buf, valueString)
		e.text(x)
	case int64:
		e.buf = append(e.buf, valueInt)
		e.varint(x)
	case bool:
		e.buf = append(e.buf, valueBool)
		e.boolean(x)
	case []string:
		e.buf = append(e.buf, valueStrings)
		e.texts(x)
	case symbol.Identity:
		e.buf = append(e.buf, valueIdentity)
		e.identity(x)
	default:
		return fmt.Errorf("state: a value of type %T is outside the fact vocabulary", v)
	}
	return nil
}

// decoder reads one record's values through a [wire.Decoder], each
// string against t, or inline where t is nil.
type decoder struct {
	wire.Decoder
	t *node.StringTable
}

// newDecoder returns a decoder over b, its strings numbered in t, or
// inline where t is nil.
func newDecoder(b []byte, t *node.StringTable) *decoder {
	return &decoder{Decoder: wire.NewDecoder(b), t: t}
}

// text reads a string.
func (d *decoder) text() string {
	if d.t == nil {
		return d.Text()
	}
	i := d.Uvarint()
	s, held := d.t.At(i)
	if !held {
		d.Fail(fmt.Errorf("%w: the table numbers %d strings, and the record names string %d",
			wire.ErrMalformed, d.t.Len(), i))
	}
	return s
}

// instant reads an instant.
func (d *decoder) instant() time.Time {
	if !d.Bool() {
		return time.Time{}
	}
	return time.Unix(0, d.Varint())
}

// texts reads a list of strings, nil for an empty list.
func (d *decoder) texts() []string {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = d.text()
	}
	return out
}

// identity reads an identity's six fields.
func (d *decoder) identity() symbol.Identity {
	return symbol.Identity{
		Lang:    symbol.Lang(d.text()),
		Package: d.text(),
		Owner:   d.text(),
		Name:    d.text(),
		Kind:    symbol.Kind(d.Uvarint()),
		Disc:    d.text(),
	}
}

// identities reads a list of identities, nil for an empty list.
func (d *decoder) identities() []symbol.Identity {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]symbol.Identity, n)
	for i := range out {
		out[i] = d.identity()
	}
	return out
}

// pos reads a position.
func (d *decoder) pos() position.Pos {
	return position.Pos{File: d.text(), Line: int(d.Varint()), Col: int(d.Varint())}
}

// finding reads one finding.
func (d *decoder) finding() diag.Diag {
	out := diag.Diag{
		Code:     diag.Code{Prefix: diag.Prefix(d.text()), Number: int(d.Uvarint())},
		Severity: diag.Severity(d.Uvarint()),
		Pos:      d.pos(),
		Msg:      d.text(),
		Origin:   diag.Origin(d.text()),
	}
	if n := d.Count(); n > 0 {
		out.Related = make([]position.Pos, n)
		for i := range out.Related {
			out.Related[i] = d.pos()
		}
	}
	return out
}

// findings reads a list of findings, nil for an empty list.
func (d *decoder) findings() []diag.Diag {
	n := d.Count()
	if n == 0 {
		return nil
	}
	out := make([]diag.Diag, n)
	for i := range out {
		out[i] = d.finding()
	}
	return out
}

// raw reads one raw directive instance.
func (d *decoder) raw() directive.Raw {
	out := directive.Raw{Name: directive.Name(d.text())}
	if n := d.Count(); n > 0 {
		out.Args = make([]directive.RawArg, n)
		for i := range out.Args {
			out.Args[i] = directive.RawArg{Key: d.text(), Value: d.rawValue(), Col: int(d.Varint())}
		}
	}
	out.Pos = d.pos()
	out.Negated = d.Bool()
	out.DirectiveShaped = d.Bool()
	return out
}

// rawValue reads one argument value and its list elements.
func (d *decoder) rawValue() directive.RawValue {
	out := directive.RawValue{Text: d.text(), Quoted: d.Bool()}
	if n := d.Count(); n > 0 {
		out.List = make([]directive.RawValue, n)
		for i := range out.List {
			out.List[i] = d.rawValue()
		}
	}
	return out
}

// stamp reads one raw classification stamp.
func (d *decoder) stamp() meta.RawStamp {
	return meta.RawStamp{
		Key:    meta.KeyName(d.text()),
		Value:  d.value(),
		Pos:    d.pos(),
		Origin: diag.Origin(d.text()),
	}
}

// value reads a value of the fact vocabulary, and nil for the none tag.
func (d *decoder) value() any {
	switch tag := d.Byte(); tag {
	case valueNone:
		return nil
	case valueString:
		return d.text()
	case valueInt:
		return d.Varint()
	case valueBool:
		return d.Bool()
	case valueStrings:
		return d.texts()
	case valueIdentity:
		return d.identity()
	default:
		d.Fail(fmt.Errorf("%w: value tag %d is outside the fact vocabulary", wire.ErrMalformed, tag))
		return nil
	}
}
