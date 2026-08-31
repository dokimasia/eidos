// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package meta

import (
	"strings"

	"go.dokimi.dev/eidos/core/symbol"
)

// KeyName is a key's boundary spelling: dotted segments with the
// owning namespace first, as in "shape.role". It appears at the
// boundary — a directive parameter, an attribution argument — and
// resolves against the registry. Code holds the typed [Key].
type KeyName string

// namespaceSep separates a key name's segments; the namespace is
// everything before the first one.
const namespaceSep = "."

// Namespace returns the segment before the first dot, which names
// the module that owns the key.
func (n KeyName) Namespace() string {
	ns, _, _ := strings.Cut(string(n), namespaceSep)
	return ns
}

// local returns the part after the namespace, and false when the
// name carries none.
func (n KeyName) local() (string, bool) {
	_, rest, found := strings.Cut(string(n), namespaceSep)
	return rest, found
}

// KeyID is the dense form gate tuples and indexes hold. It is
// assigned at registration and belongs to one composition: nothing
// durable stores it, because codecs and the sealed state carry
// names. The zero KeyID names no key.
type KeyID uint32

// FactValue is the closed value vocabulary; nothing else registers.
// A fact that needs structure becomes flat keys under a fact group,
// and a fact that names a declaration carries an identity rather
// than a string.
type FactValue interface {
	string | int64 | bool | []string | symbol.Identity
}

// Key is the typed handle registration returns. Reads, writes and
// gate predicates all go through it, so the value type is checked
// where the code compiles rather than where the run fails.
//
// The zero Key names nothing: every handle comes from [Register].
type Key[T FactValue] struct {
	id   KeyID
	name KeyName
}

// Name returns the key's boundary spelling.
func (k Key[T]) Name() KeyName { return k.name }

// ID returns the key's dense id.
func (k Key[T]) ID() KeyID { return k.id }

// IsZero reports whether the key names nothing.
func (k Key[T]) IsZero() bool { return k.id == 0 }

// GroupName names a fact group: a bundle a writer declares, such as
// every key its classification stamps. The name is public API and
// the membership is the writer's to grow.
type GroupName string
