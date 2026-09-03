// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// Registry holds every registered namespace, key and group.
//
// A Registry is not safe for concurrent use. Registration happens
// while the workspace composes, which is single-threaded, and
// completes before the first fact is written; [Facts] reads it
// without locking on that contract.
type Registry struct {
	// namespaces maps a claimed namespace to its owner.
	namespaces map[string]string
	// byName resolves a boundary spelling to its dense id.
	byName map[KeyName]KeyID
	// specs holds every registered spec; KeyID n lives at index
	// n-1, so the zero id resolves to nothing.
	specs []KeySpec
	// types records each key's value type beside its spec, so a
	// lookup by name hands out a handle of the registered type only.
	types []reflect.Type
	// groups holds each group's members in registration order.
	groups map[GroupName][]KeyID
}

// NewRegistry returns a registry holding nothing.
func NewRegistry() *Registry {
	return &Registry{
		namespaces: map[string]string{},
		byName:     map[KeyName]KeyID{},
		groups:     map[GroupName][]KeyID{},
	}
}

// KeySpec is what a registration declares.
type KeySpec struct {
	// Name is the boundary spelling. Its namespace must be claimed
	// before the key registers.
	Name KeyName
	// Kinds are the declaration kinds the key may be stamped on.
	// Empty admits every kind.
	Kinds []symbol.Kind
	// Group optionally registers the key into a fact group.
	Group GroupName
	// Contract optionally promises coverage. The declaration is
	// held; nothing here checks it.
	Contract *Completeness
	// Doc states the key's semantics. Registration refuses an empty
	// one.
	Doc string
}

// Completeness is a coverage promise: the key is stamped on every
// declaration of these kinds by the end of the named phase.
type Completeness struct {
	On []symbol.Kind
	By diag.Origin
	// Severity is what a violation reports as: Error when output
	// depends on the promise, Warning when it is advisory.
	Severity diag.Severity
}

// ClaimNamespace records who owns a namespace.
//
// A namespace claimed twice is an error naming both owners. Every
// key registers into a claimed namespace, so a typo in a key's
// namespace fails at registration rather than reading as a new
// namespace.
func (r *Registry) ClaimNamespace(ns, owner string) error {
	if ns == "" {
		return errors.New("meta: the empty namespace owns nothing")
	}
	if owner == "" {
		return fmt.Errorf("meta: namespace %q names no owner: a collision could not be reported", ns)
	}
	if held, taken := r.namespaces[ns]; taken {
		return fmt.Errorf("meta: namespace %q is claimed twice: by %q and by %q", ns, held, owner)
	}
	r.namespaces[ns] = owner
	return nil
}

// Register records a key and returns its typed handle.
//
// It refuses, with an error naming both claimants where two exist: a
// name without a claimed namespace or without a local part, a name
// registered twice, and a spec without documentation. It returns an
// error rather than panicking because composition collects every
// fault in one pass.
func Register[T FactValue](r *Registry, s KeySpec) (Key[T], error) {
	local, hasLocal := s.Name.local()
	if !hasLocal || local == "" || s.Name.Namespace() == "" {
		return Key[T]{}, fmt.Errorf(
			"meta: key %q spells no namespace and local part: both are non-empty", s.Name,
		)
	}
	if _, claimed := r.namespaces[s.Name.Namespace()]; !claimed {
		return Key[T]{}, fmt.Errorf(
			"meta: key %q registers into namespace %q, which nothing claimed",
			s.Name, s.Name.Namespace(),
		)
	}
	if s.Doc == "" {
		return Key[T]{}, fmt.Errorf(
			"meta: key %q states no semantics: the parity matrix tabulates them", s.Name,
		)
	}
	if held, taken := r.byName[s.Name]; taken {
		return Key[T]{}, fmt.Errorf("meta: key %q is registered twice: %q and %q",
			s.Name, r.specs[held-1].Doc, s.Doc)
	}

	r.specs = append(r.specs, s)
	r.types = append(r.types, reflect.TypeFor[T]())
	id := KeyID(len(r.specs))
	r.byName[s.Name] = id
	if s.Group != "" {
		r.groups[s.Group] = append(r.groups[s.Group], id)
	}
	return Key[T]{id: id, name: s.Name}, nil
}

// Lookup returns the typed handle a boundary spelling names, for a
// reader that knows a key by its spelling alone: a language's
// rules reading what its frontend stamped, without the handle
// registration returned. It returns false for a spelling nothing
// registered, and for one registered under another value type, so
// a handle that exists reads what was written.
func Lookup[T FactValue](r *Registry, name KeyName) (Key[T], bool) {
	id, held := r.byName[name]
	if !held || r.types[id-1] != reflect.TypeFor[T]() {
		return Key[T]{}, false
	}
	return Key[T]{id: id, name: name}, true
}

// Resolve returns the id a boundary spelling names, and false for a
// spelling nothing registered.
func (r *Registry) Resolve(name KeyName) (KeyID, bool) {
	id, known := r.byName[name]
	return id, known
}

// Spec returns a registered key's spec, and false for an id nothing
// was assigned.
func (r *Registry) Spec(id KeyID) (KeySpec, bool) {
	if id == 0 || int(id) > len(r.specs) {
		return KeySpec{}, false
	}
	return r.specs[id-1], true
}

// Group returns a group's member keys, in registration order.
func (r *Registry) Group(g GroupName) iter.Seq[KeyID] {
	return slices.Values(r.groups[g])
}

// Keys returns every registered key's spelling, in registration
// order: what a candidate-naming refusal enumerates.
func (r *Registry) Keys() iter.Seq[KeyName] {
	return func(yield func(KeyName) bool) {
		for _, spec := range r.specs {
			if !yield(spec.Name) {
				return
			}
		}
	}
}
