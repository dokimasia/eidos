// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"encoding/binary"
	"fmt"

	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// ErrMalformed is the class of every failure of [DecodeBinary] and
// [DecodeStringTable] to read an encoding: bytes that end inside it, or
// that state a kind, a bool, a string number or a length outside the
// encoding's range. Every record of the kernel's sealed state decodes under
// the same class.
var ErrMalformed = wire.ErrMalformed

// binaryWriter appends one binary encoding to buf in the kernel's
// record format. A string is its number in t, or, where t is nil, its
// length and its bytes, which makes the encoding independent of any
// table. A kind is one byte, and [symbol.KindInvalid] encodes a nil
// symbol.
type binaryWriter struct {
	buf []byte
	t   *StringTable
}

// str writes a string.
func (w *binaryWriter) str(s string) {
	if w.t != nil {
		w.buf = binary.AppendUvarint(w.buf, w.t.Add(s))
		return
	}
	w.buf = wire.AppendText(w.buf, s)
}

// boolean writes a bool as one byte.
func (w *binaryWriter) boolean(b bool) { w.buf = wire.AppendBool(w.buf, b) }

// integer writes a signed integer.
func (w *binaryWriter) integer(i int) { w.buf = binary.AppendVarint(w.buf, int64(i)) }

// unsigned writes an unsigned integer: a list's length or a value of
// the symbol package's enums.
func (w *binaryWriter) unsigned(u uint64) { w.buf = binary.AppendUvarint(w.buf, u) }

// kind writes a symbol's kind as one byte.
func (w *binaryWriter) kind(k symbol.Kind) { w.buf = append(w.buf, byte(k)) }

// identity writes an identity's six fields.
func (w *binaryWriter) identity(id symbol.Identity) {
	w.str(string(id.Lang))
	w.str(id.Package)
	w.str(id.Owner)
	w.str(id.Name)
	w.kind(id.Kind)
	w.str(id.Disc)
}

// pos writes a position's file, line and column.
func (w *binaryWriter) pos(p position.Pos) {
	w.str(p.File)
	w.integer(p.Line)
	w.integer(p.Col)
}

// strs writes a list of strings.
func (w *binaryWriter) strs(ss []string) {
	w.unsigned(uint64(len(ss)))
	for _, s := range ss {
		w.str(s)
	}
}

// annotations writes an annotation list: each annotation's name and
// arguments.
func (w *binaryWriter) annotations(as symbol.Annotations) {
	w.unsigned(uint64(len(as)))
	for _, a := range as {
		w.str(a.Name)
		w.strs(a.Args)
	}
}

// binaryReader reads one binary encoding through a [wire.Decoder],
// each string against t, or inline where t is nil.
type binaryReader struct {
	wire.Decoder
	t *StringTable
}

// boolean reads a bool.
func (r *binaryReader) boolean() bool { return r.Bool() }

// integer reads a signed integer.
func (r *binaryReader) integer() int { return int(r.Varint()) }

// unsigned reads an unsigned integer.
func (r *binaryReader) unsigned() uint64 { return r.Uvarint() }

// count reads a list's length.
func (r *binaryReader) count() int { return r.Count() }

// kind reads a symbol's kind.
func (r *binaryReader) kind() symbol.Kind { return symbol.Kind(r.Byte()) }

// str reads a string: its number in the table, or its length and bytes
// where the reader has no table.
func (r *binaryReader) str() string {
	if r.t == nil {
		return r.Text()
	}
	i := r.Uvarint()
	s, held := r.t.At(i)
	if !held {
		r.Fail(fmt.Errorf("%w: the table numbers %d strings, and the encoding names string %d",
			ErrMalformed, r.t.Len(), i))
	}
	return s
}

// identity reads an identity's six fields.
func (r *binaryReader) identity() symbol.Identity {
	return symbol.Identity{
		Lang:    symbol.Lang(r.str()),
		Package: r.str(),
		Owner:   r.str(),
		Name:    r.str(),
		Kind:    r.kind(),
		Disc:    r.str(),
	}
}

// pos reads a position's file, line and column.
func (r *binaryReader) pos() position.Pos {
	return position.Pos{File: r.str(), Line: r.integer(), Col: r.integer()}
}

// strs reads a list of strings, nil for an empty list.
func (r *binaryReader) strs() []string {
	n := r.count()
	if n == 0 {
		return nil
	}
	out := make([]string, n)
	for i := range out {
		out[i] = r.str()
	}
	return out
}

// annotations reads an annotation list, nil for an empty list.
func (r *binaryReader) annotations() symbol.Annotations {
	n := r.count()
	if n == 0 {
		return nil
	}
	out := make(symbol.Annotations, n)
	for i := range out {
		out[i] = symbol.Annotation{Name: r.str(), Args: r.strs()}
	}
	return out
}
