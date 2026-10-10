// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"strings"

	"go.dokimi.dev/eidos/core/symbol"
)

// KeyName is a key's boundary spelling: dotted segments with the
// namespace first, as in "shape.role". It appears at the boundary,
// in a directive parameter or an attribution argument, and resolves
// against the registry. Code uses the typed [Key].
type KeyName string

// namespaceSep separates a key name's segments. The namespace is
// everything before the first one.
const namespaceSep = "."

// Namespace returns the segment before the first dot: the namespace
// a registrant claims before it registers the key. A name without a dot
// is its own namespace. Namespace allocates nothing, because the
// segment shares the name's bytes.
func (n KeyName) Namespace() string {
	ns, _, _ := strings.Cut(string(n), namespaceSep)
	return ns
}

// local returns the part after the namespace, and false when the
// name has no separator.
func (n KeyName) local() (string, bool) {
	_, rest, found := strings.Cut(string(n), namespaceSep)
	return rest, found
}

// KeyID is the dense form gate tuples and indexes store. Registration
// assigns it, and it is valid in one composition only. Nothing
// durable stores it: codecs and the sealed state record names. The
// zero KeyID names no key.
type KeyID uint32

// FactValue is the closed value vocabulary, and [Register] accepts
// no other value type. A fact that needs structure becomes flat keys
// under a fact group, and a fact that names a declaration stores an
// identity, not a string.
type FactValue interface {
	string | int64 | bool | []string | symbol.Identity
}

// Key is the typed handle registration returns. Reads, writes and
// gate predicates all go through it, so the compiler checks the
// value type.
//
// A handle comes from [Register], or from [Named] for a key that another
// registrant registers. A named handle has the name alone, and a read
// resolves it in the registry of the store that it reads. The zero Key
// has neither an id nor a name.
type Key[T FactValue] struct {
	id   KeyID
	name KeyName
}

// Named returns a handle on the key with the name name, for a plugin whose
// predicates or handlers read a key that another plugin registers. The
// handle has no id. A gate on it binds when the workspace builds, and a
// read resolves the name in the registry of the store that it reads. A
// read of a name that nothing registered under T finds no fact. A write
// through the handle is refused, because a registrant writes through the
// handle that [Register] returned. Named allocates nothing.
func Named[T FactValue](name KeyName) Key[T] {
	return Key[T]{name: name}
}

// Name returns the key's boundary spelling, empty for the zero Key. It
// allocates nothing.
func (k Key[T]) Name() KeyName { return k.name }

// ID returns the key's dense id, zero for the zero Key and for a named
// handle. It allocates nothing.
func (k Key[T]) ID() KeyID { return k.id }

// IsZero reports whether the key is the zero Key, which has neither an id
// nor a name. A named handle is not zero. It allocates nothing.
func (k Key[T]) IsZero() bool { return k.id == 0 && k.name == "" }

// resolved returns the handle that a read uses. That handle is k where k
// has an id or is the zero Key, and otherwise the handle that r registered
// under the name of k. It reports false for a name that r did not
// register under T. It allocates nothing.
func (k Key[T]) resolved(r *Registry) (Key[T], bool) {
	if k.id != 0 || k.name == "" {
		return k, true
	}
	return Lookup[T](r, k.name)
}

// GroupName names a fact group: a bundle a writer declares, such as
// every key its classification stamps. The name is public API, and
// the writer may add members.
type GroupName string
