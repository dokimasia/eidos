// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/internal/wire"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// The tags that open the second part of a key of the names table: an
// entry under its collision scope, or under its origin.
const (
	rowByScope  = 's'
	rowByOrigin = 'o'
)

// KeptNames is the name table that a generation records for one plan,
// read as the [plugin.Names] of the files that a warm run keeps. It leaves
// out every entry of a file that skip reports, which are the files that
// the run generates again. Each lookup scans the rows of one key prefix.
// InPackage reads the rows of the package's scope under the emitted name,
// OfOrigin the rows of the origin under the emitted name, and InScope the
// rows of the scope, which it sorts by settled name once and keeps.
//
// The methods of [plugin.Names] return no error. A lookup that meets a row
// that does not read whole or does not decode finds nothing, and
// [KeptNames.Err] returns the first such error, which wraps [ErrDamaged].
//
// # Concurrency
//
// A KeptNames is not safe for concurrent use. One plan's goroutine reads
// it, as [plugin.Names] states.
//
// # Allocation contract
//
// Each lookup allocates the rows it scans and the strings it decodes. The
// decodes share one string for each text, and InScope allocates a scope's
// entries once.
type KeptNames struct {
	s    *PhaseState
	plan string
	skip func(file string) bool
	// scopes keeps the entries of each scope that InScope read, and strings
	// the strings that the decodes share.
	scopes  map[scopeName][]plugin.NameEntry
	strings map[string]string
	err     error
}

var _ plugin.Names = (*KeptNames)(nil)

// scopeName names one collision scope: a package, and the receiver its
// methods attach to, empty for the package's other names.
type scopeName struct {
	pkg      string
	receiver string
}

// Names returns the name table that the generation records for a plan,
// without the entries of each file that skip reports. A nil skip leaves
// out no entry.
func (s *PhaseState) Names(plan string, skip func(file string) bool) *KeptNames {
	return &KeptNames{s: s, plan: plan, skip: skip, strings: map[string]string{}}
}

// InPackage returns the entry that a file-level declaration of a package
// declares under an emitted name, and false where none does. A method is
// no such declaration. The entry is the first in settled order, and it
// reports Ambiguous where two entries settle the name apart.
func (n *KeptNames) InPackage(pkg, emitted string) (plugin.NameEntry, bool) {
	var key [spellingCap]byte
	prefix := append(append(scopePrefix(key[:0], n.plan, pkg, ""), emitted...), keySep)
	var out plugin.NameEntry
	found := false
	for _, e := range n.entries(prefix) {
		switch {
		case e.Kind == symbol.KindMethod:
		case !found:
			out, found = e, true
		case e.Settled != out.Settled:
			out.Ambiguous = true
		}
	}
	return out, found
}

// OfOrigin returns the entry that a file-level declaration derived from
// an origin declares under an emitted name, the first in the order of its
// package, its receiver and its settled name, and false where none does.
func (n *KeptNames) OfOrigin(origin symbol.Identity, emitted string) (plugin.NameEntry, bool) {
	var key [spellingCap]byte
	prefix := append(append(key[:0], n.plan...), keySep, rowByOrigin, keySep)
	prefix = append(identityKey(prefix, origin), keySep)
	entries := n.entries(append(append(prefix, emitted...), keySep))
	if len(entries) == 0 {
		return plugin.NameEntry{}, false
	}
	return entries[0], true
}

// InScope returns the entries of one collision scope, sorted by settled
// name: the methods that attach to the receiver where it is set, and the
// package's other file-level names where it is empty. It reads each scope
// once and returns the same list to every later call, which the caller
// does not mutate.
func (n *KeptNames) InScope(pkg, receiver string) []plugin.NameEntry {
	at := scopeName{pkg: pkg, receiver: receiver}
	if entries, read := n.scopes[at]; read {
		return entries
	}
	var key [spellingCap]byte
	entries := n.entries(scopePrefix(key[:0], n.plan, pkg, receiver))
	slices.SortStableFunc(entries, func(a, b plugin.NameEntry) int { return strings.Compare(a.Settled, b.Settled) })
	if n.scopes == nil {
		n.scopes = map[scopeName][]plugin.NameEntry{}
	}
	n.scopes[at] = entries
	return entries
}

// Err returns the first error that a lookup met, which wraps
// [ErrDamaged], and nil where every row it read decoded.
func (n *KeptNames) Err() error { return n.err }

// entries returns the entries of the rows under a key prefix, in key
// order, without the entries of a file that skip reports. A row that does
// not decode ends the lookup, which then keeps its error.
func (n *KeptNames) entries(prefix []byte) []plugin.NameEntry {
	if n.err != nil {
		return nil
	}
	rows, err := n.s.g.readers[TableNames].scan(n.s.ctx, prefix)
	if err != nil {
		n.err = err
		return nil
	}
	out := make([]plugin.NameEntry, 0, len(rows))
	for _, e := range rows {
		d := decoder{Decoder: wire.NewDecoder(e.row), interned: n.strings}
		entry := decodeNameRow(&d)
		if err := d.Err(); err != nil {
			n.err = fmt.Errorf("%w: a names row of plan %q does not decode: %w", ErrDamaged, n.plan, err)
			return nil
		}
		if n.skip == nil || !n.skip(entry.File) {
			out = append(out, entry)
		}
	}
	return out
}

// scopePrefix appends the prefix of a scope's rows to dst: the plan, the
// scope tag, the package and the receiver, each closed by the separator.
func scopePrefix(dst []byte, plan, pkg, receiver string) []byte {
	dst = append(append(dst, plan...), keySep, rowByScope, keySep)
	dst = append(append(dst, pkg...), keySep)
	return append(append(dst, receiver...), keySep)
}

// scopeRowKey appends the key of a name's row under its collision scope to
// dst: the scope's prefix, then the emitted name, the separator and the
// settled name. A plan that commits declares one settled name once in a
// scope, so the key is the entry's own.
func scopeRowKey(dst []byte, plan string, e *plugin.NameEntry) []byte {
	dst = append(append(scopePrefix(dst, plan, e.Package, e.Receiver), e.Emitted...), keySep)
	return append(dst, e.Settled...)
}

// originRowKey appends the key of a name's row under its origin to dst:
// the plan, the origin tag, the origin and the emitted name, each closed
// by the separator, then the package, the receiver and the settled name,
// which make the key the entry's own.
func originRowKey(dst []byte, plan string, e *plugin.NameEntry) []byte {
	dst = append(append(dst, plan...), keySep, rowByOrigin, keySep)
	dst = append(identityKey(dst, e.Origin), keySep)
	dst = append(append(dst, e.Emitted...), keySep)
	dst = append(append(dst, e.Package...), keySep)
	dst = append(append(dst, e.Receiver...), keySep)
	return append(dst, e.Settled...)
}

// rowPlan returns the plan of a names row's key: the bytes before the
// first separator.
func rowPlan(key []byte) []byte {
	plan, _, _ := bytes.Cut(key, []byte{keySep})
	return plan
}

// appendNameRow appends a name's row to dst: the file first, so a commit
// decides from the file alone which prior rows it keeps, then the file's
// package and the entry's package, receiver, origin, kind, emitted name
// and settled name.
func appendNameRow(dst []byte, e *plugin.NameEntry) []byte {
	enc := encoder{buf: dst}
	enc.text(e.File)
	enc.identity(e.FilePkg)
	enc.text(e.Package)
	enc.text(e.Receiver)
	enc.identity(e.Origin)
	enc.uvarint(uint64(e.Kind))
	enc.text(e.Emitted)
	enc.text(e.Settled)
	return enc.buf
}

// decodeNameRow decodes a name's row.
func decodeNameRow(d *decoder) plugin.NameEntry {
	return plugin.NameEntry{
		File:     d.text(),
		FilePkg:  d.identity(),
		Package:  d.text(),
		Receiver: d.text(),
		Origin:   d.identity(),
		Kind:     symbol.Kind(d.Enum()),
		Emitted:  d.text(),
		Settled:  d.text(),
	}
}
