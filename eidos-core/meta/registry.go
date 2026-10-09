// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"errors"
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strconv"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/symbol"
)

// compositionName is how a message names the composition, the
// registrant without a plugin name.
const compositionName = "the composition"

// Registry is a handle on the registered namespaces, keys and
// groups, bound to one registrant: a plugin's name, or no name for
// the composition that builds the workspace.
//
// Every handle [Registry.For] derives shares one set of
// registrations. A namespace belongs to the registrant whose handle
// claimed it, and a key registers only into a namespace its own
// registrant claimed, so no plugin registers keys under another
// plugin's namespace or the kernel's.
//
// A Registry is not safe for concurrent use. Registration happens
// while the workspace composes, which is single-threaded.
// [Registry.Seal] ends registration, and every handle refuses a
// later one. [Facts] reads a sealed registry without locking, and
// every fact store built over it sees one set of keys and groups.
type Registry struct {
	*registrations
	// registrant is who this handle claims namespaces and registers
	// keys for. The empty name is the composition.
	registrant string
}

// registrations is the state every handle of one registry shares.
type registrations struct {
	// namespaces maps a claimed namespace to its registrant.
	namespaces map[string]string
	// byName resolves a boundary spelling to its dense id.
	byName map[KeyName]KeyID
	// specs contains every registered spec. KeyID n is at index n-1,
	// so the zero id resolves to nothing.
	specs []KeySpec
	// types records each key's value type beside its spec, so a
	// lookup by name hands out a handle of the registered type only.
	types []reflect.Type
	// groups contains each group's members in registration order.
	groups map[GroupName][]KeyID
	// sealed is set by [Registry.Seal]. ClaimNamespace and Register
	// refuse once it is set.
	sealed bool
}

// NewRegistry returns an empty registry and the composition's handle
// on it.
//
// # Allocation contract
//
// NewRegistry allocates five times: the handle, the registrations every
// handle shares, and their three maps.
func NewRegistry() *Registry {
	return &Registry{registrations: &registrations{
		namespaces: map[string]string{},
		byName:     map[KeyName]KeyID{},
		groups:     map[GroupName][]KeyID{},
	}}
}

// For returns a handle on the same registrations bound to one
// registrant: the name its claims record, and the name its
// registrations must match. The empty name returns the composition's
// handle. For allocates the handle, one allocation.
func (r *Registry) For(registrant string) *Registry {
	return &Registry{registrations: r.registrations, registrant: registrant}
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
	// Contract optionally promises coverage. The registry keeps the
	// declaration and checks nothing.
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

// ClaimNamespace claims a namespace for the handle's registrant.
//
// A namespace claimed twice is an error naming both registrants.
// Every key registers into a claimed namespace, so a typo in a key's
// namespace fails at registration and never reads as a new
// namespace.
//
// Error modes: a claim after [Registry.Seal], the empty namespace, and
// a namespace another claim took.
//
// # Allocation contract
//
// ClaimNamespace allocates only where the namespace map grows. The
// registry's first claim allocates once.
func (r *Registry) ClaimNamespace(ns string) error {
	if r.sealed {
		return fmt.Errorf("meta: namespace %q is claimed after the seal: registration ends there", ns)
	}
	if ns == "" {
		return errors.New("meta: the empty namespace names nothing to claim")
	}
	if held, taken := r.namespaces[ns]; taken {
		return fmt.Errorf("meta: namespace %q is claimed twice: by %s and by %s",
			ns, registrantName(held), registrantName(r.registrant))
	}
	r.namespaces[ns] = r.registrant
	return nil
}

// Register records a key and returns its typed handle.
//
// It refuses the following, with an error naming both claimants
// where two exist:
//   - a registration after [Registry.Seal];
//   - a name without a claimed namespace or without a local part;
//   - a name in a namespace another registrant claimed;
//   - a name registered twice;
//   - a spec without documentation;
//   - a key and a group with one spelling, in either registration
//     order. A meta drop names a key or a group by its spelling, so
//     one spelling must name one of them.
//
// It returns an error rather than panicking because composition
// collects every fault in one pass.
//
// # Allocation contract
//
// Register allocates what the registry keeps: the growth of the spec
// list, the type list and the name map, and for a key in a group the
// growth of the group map and of the group's member list. The first key
// of a registry allocates three times outside a group and five times in
// one.
func Register[T FactValue](r *Registry, s KeySpec) (Key[T], error) {
	if r.sealed {
		return Key[T]{}, fmt.Errorf("meta: key %q registers after the seal: registration ends there", s.Name)
	}
	local, hasLocal := s.Name.local()
	if !hasLocal || local == "" || s.Name.Namespace() == "" {
		return Key[T]{}, fmt.Errorf(
			"meta: key %q spells no namespace and local part: both are non-empty", s.Name,
		)
	}
	registrant, claimed := r.namespaces[s.Name.Namespace()]
	if !claimed {
		return Key[T]{}, fmt.Errorf(
			"meta: key %q registers into namespace %q, which nothing claimed",
			s.Name, s.Name.Namespace(),
		)
	}
	if registrant != r.registrant {
		return Key[T]{}, fmt.Errorf(
			"meta: %s registers key %q into namespace %q, which %s claimed",
			registrantName(r.registrant), s.Name, s.Name.Namespace(), registrantName(registrant),
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
	if _, grouped := r.groups[GroupName(s.Name)]; grouped {
		return Key[T]{}, fmt.Errorf("meta: key %q spells a registered group", s.Name)
	}
	if _, keyed := r.byName[KeyName(s.Group)]; s.Group != "" && keyed {
		return Key[T]{}, fmt.Errorf("meta: key %q registers into group %q, which spells a registered key",
			s.Name, s.Group)
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
// a handle that exists reads what was written. It allocates nothing.
func Lookup[T FactValue](r *Registry, name KeyName) (Key[T], bool) {
	id, held := r.byName[name]
	if !held || r.types[id-1] != reflect.TypeFor[T]() {
		return Key[T]{}, false
	}
	return Key[T]{id: id, name: name}, true
}

// Claimant returns the registrant that claimed a namespace: a plugin's
// name, or the empty name for the composition. It reports false for a
// namespace that nothing claimed. Every key of a namespace registers
// through its claimant, so the claimant is the registrant of each key of
// the namespace. It allocates nothing.
func (r *Registry) Claimant(ns string) (string, bool) {
	registrant, claimed := r.namespaces[ns]
	return registrant, claimed
}

// Resolve returns the id a boundary spelling names, and false for a
// spelling nothing registered. It allocates nothing.
func (r *Registry) Resolve(name KeyName) (KeyID, bool) {
	id, known := r.byName[name]
	return id, known
}

// Spec returns a registered key's spec, and false for an id nothing
// was assigned. The spec's kind lists are the registry's own, so a
// caller reads them and never writes them. It allocates nothing.
func (r *Registry) Spec(id KeyID) (KeySpec, bool) {
	if id == 0 || int(id) > len(r.specs) {
		return KeySpec{}, false
	}
	return r.specs[id-1], true
}

// Group returns a group's member keys, in registration order. A range
// over the result allocates nothing.
func (r *Registry) Group(g GroupName) iter.Seq[KeyID] {
	return slices.Values(r.groups[g])
}

// Keys returns every registered key's spelling, in registration
// order: what a candidate-naming refusal enumerates. A range over the
// result allocates nothing.
func (r *Registry) Keys() iter.Seq[KeyName] {
	return func(yield func(KeyName) bool) {
		for _, spec := range r.specs {
			if !yield(spec.Name) {
				return
			}
		}
	}
}

// Seal ends registration: a namespace or key arriving after it is
// refused. The workspace seals once every plugin and the
// composition registered, before any fact store is built. Sealing a
// sealed registry changes nothing. Seal allocates nothing.
func (r *Registry) Seal() { r.sealed = true }

// typeOf returns the value type a key registered with, nil for an
// id nothing was assigned.
func (r *Registry) typeOf(id KeyID) reflect.Type {
	if id == 0 || int(id) > len(r.types) {
		return nil
	}
	return r.types[id-1]
}

// nameOf returns a key's boundary spelling without copying its spec,
// and false for an id nothing was assigned.
func (r *Registry) nameOf(id KeyID) (KeyName, bool) {
	if id == 0 || int(id) > len(r.specs) {
		return "", false
	}
	return r.specs[id-1].Name, true
}

// registrantName spells a registrant for a message: its quoted name,
// or the composition for the empty one.
func registrantName(registrant string) string {
	if registrant == "" {
		return compositionName
	}
	return strconv.Quote(registrant)
}
