// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"errors"
	"fmt"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace of every golang key.
const namespace = "golang"

// These constants are the classification and fact keys of the
// satellite. The root declares them because one namespace claim serves
// both roles. The frontend stamps what a parse can see, and the
// annotator stamps what the sealed graph proves.
const (
	// TestFileKey classifies a file the go tool treats as a test.
	TestFileKey meta.KeyName = "golang.testFile"

	// ConstraintKey classifies a file whose build constraint falls
	// outside the load's tag set. The value is the constraint as one
	// go:build expression: its build lines and the tags that its name
	// implies, joined.
	ConstraintKey meta.KeyName = "golang.constraint"

	// GeneratedKey classifies a file with the Go convention's
	// generated marker. The value is the marker line as written.
	GeneratedKey meta.KeyName = "golang.generated"

	// CgoKey classifies a file importing "C", whose preamble the
	// model does not represent.
	CgoKey meta.KeyName = "golang.cgo"

	// TypeSetKey records an interface's constraint elements, unions
	// and approximations, as their verbatim spellings.
	TypeSetKey meta.KeyName = "golang.typeSet"

	// ConstraintInterfaceKey marks an interface with type-set
	// elements. Such an interface is usable as a bound and never as the
	// type of a value.
	ConstraintInterfaceKey meta.KeyName = "golang.constraintInterface"

	// EmptyInterfaceKey marks an interface with no members at all.
	EmptyInterfaceKey meta.KeyName = "golang.emptyInterface"

	// ReceiverIsPointerKey marks a method declared on a pointer
	// receiver.
	ReceiverIsPointerKey meta.KeyName = "golang.receiverIsPointer"

	// UnderlyingKindKey records the kind of the underlying type of a
	// defined type. The kind is basic, named, pointer, slice, array,
	// map, chan or func.
	UnderlyingKindKey meta.KeyName = "golang.underlyingKind"

	// IterSeqKey and IterSeq2Key mark a callable whose first
	// result is the iterator type of that arity.
	IterSeqKey  meta.KeyName = "golang.iterSeq"
	IterSeq2Key meta.KeyName = "golang.iterSeq2"

	// ConstValueKey records the exact value of a constant where the
	// scope of its package suffices to evaluate it. The value includes
	// the ordinal arithmetic of an iota row.
	ConstValueKey meta.KeyName = "golang.constValue"

	// HiddenFieldsKey marks a struct whose lowering left out a field: a
	// blank field at every depth, and an unexported field at the
	// signature depth. The model does not keep the type of such a field,
	// so the fields of the struct do not decide whether it is comparable.
	HiddenFieldsKey meta.KeyName = "golang.hiddenFields"

	// SatisfiesErrorKey and SatisfiesStringerKey mark a type whose
	// workspace-visible method set has the interface's one method.
	// EmbedsInterfaceKey marks a struct embedding a type the graph
	// declares as an interface. ComparableKey marks a type every
	// field of which is provably comparable. Each key stamps only what
	// the sealed graph proves, so an absent fact is unknown and never
	// a negative.
	SatisfiesErrorKey    meta.KeyName = "golang.satisfiesError"
	SatisfiesStringerKey meta.KeyName = "golang.satisfiesStringer"
	EmbedsInterfaceKey   meta.KeyName = "golang.embedsInterface"
	ComparableKey        meta.KeyName = "golang.comparable"
)

// Handles contains the typed keys through which the annotator stamps.
// [Register] returns them to the role that registers the keys, and
// [Lookup] returns them to the annotator.
type Handles struct {
	SatisfiesError    meta.Key[bool]
	SatisfiesStringer meta.Key[bool]
	EmbedsInterface   meta.Key[bool]
	Comparable        meta.Key[bool]
}

// Keys registers every golang key, in the form a composition and a
// corpus fixture declare them. It allocates what [Register] allocates.
func Keys(r *meta.Registry) error {
	_, err := Register(r)
	return err
}

// Register claims the golang namespace for the handle's registrant,
// registers every golang key through the same handle, and returns
// the annotator's handles.
//
// Error modes: the namespace another registrant claimed, a sealed
// registry, and a key whose spelling a group took. Each is the
// registry's error. A failed claim registers no key. Otherwise Register
// registers every key that it can, and returns the errors of the others
// joined.
//
// # Allocation contract
//
// Register allocates the eleven kind lists of the keys, and the
// registry allocates its namespace claim and the growth of its lists
// and maps to seventeen keys: 29 allocations into a fresh registry.
func Register(r *meta.Registry) (Handles, error) {
	var h Handles
	if err := r.ClaimNamespace(namespace); err != nil {
		return h, err
	}
	var errs []error
	file := []symbol.Kind{symbol.KindFile}
	structs := []symbol.Kind{symbol.KindStruct}
	types := []symbol.Kind{
		symbol.KindStruct, symbol.KindInterface, symbol.KindAlias,
		symbol.KindEnum, symbol.KindSum,
	}
	for _, spec := range []meta.KeySpec{
		{
			Name: TestFileKey, Kinds: file,
			Doc: "marks a file the go tool treats as a test",
		},
		{
			Name: ConstraintKey, Kinds: file,
			Doc: "records the build constraint that kept a file's declarations out",
		},
		{
			Name: GeneratedKey, Kinds: file,
			Doc: "records the generated-file marker a foreign generator wrote",
		},
		{
			Name: CgoKey, Kinds: file,
			Doc: "marks a file importing C, whose preamble the model does not represent",
		},
		{
			Name: TypeSetKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "records an interface's constraint elements as written",
		},
		{
			Name: ConstraintInterfaceKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "marks an interface with type-set elements",
		},
		{
			Name: EmptyInterfaceKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "marks an interface with no members",
		},
		{
			Name: ReceiverIsPointerKey, Kinds: []symbol.Kind{symbol.KindMethod},
			Doc: "marks a method on a pointer receiver",
		},
		{
			Name: UnderlyingKindKey, Kinds: []symbol.Kind{symbol.KindAlias, symbol.KindEnum},
			Doc: "records the kind of the underlying type of a defined type",
		},
		{
			Name: IterSeqKey, Kinds: []symbol.Kind{symbol.KindFunction, symbol.KindMethod},
			Doc: "marks a callable returning iter.Seq",
		},
		{
			Name: IterSeq2Key, Kinds: []symbol.Kind{symbol.KindFunction, symbol.KindMethod},
			Doc: "marks a callable returning iter.Seq2",
		},
		{
			Name: ConstValueKey, Kinds: []symbol.Kind{symbol.KindConstant, symbol.KindEnumVariant},
			Doc: "records a constant's exact value where the package evaluates it",
		},
		{
			Name: HiddenFieldsKey, Kinds: structs,
			Doc: "marks a struct whose lowering left out a blank or an unexported field",
		},
		{
			Name: SatisfiesErrorKey, Kinds: types,
			Doc: "marks a type whose visible method set satisfies error",
		},
		{
			Name: SatisfiesStringerKey, Kinds: types,
			Doc: "marks a type whose visible method set satisfies fmt.Stringer",
		},
		{
			Name: EmbedsInterfaceKey, Kinds: structs,
			Doc: "marks a struct embedding a type the graph declares as an interface",
		},
		{
			Name: ComparableKey, Kinds: types,
			Doc: "marks a type every field of which is provably comparable",
		},
	} {
		key, err := registerKey(r, spec)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		switch spec.Name {
		case SatisfiesErrorKey:
			h.SatisfiesError = key
		case SatisfiesStringerKey:
			h.SatisfiesStringer = key
		case EmbedsInterfaceKey:
			h.EmbedsInterface = key
		case ComparableKey:
			h.Comparable = key
		default:
			// A key that the annotator does not stamp has no handle.
		}
	}
	return h, errors.Join(errs...)
}

// Lookup returns the annotator's handles from a registry in which
// [Register] registered the golang keys, such as the registry of a
// composition with the Go frontend. It registers nothing.
//
// Error modes: Lookup returns an error for a key of the handles that the
// registry does not contain as a boolean key.
func Lookup(r *meta.Registry) (Handles, error) {
	var h Handles
	for _, k := range [...]struct {
		name meta.KeyName
		key  *meta.Key[bool]
	}{
		{name: SatisfiesErrorKey, key: &h.SatisfiesError},
		{name: SatisfiesStringerKey, key: &h.SatisfiesStringer},
		{name: EmbedsInterfaceKey, key: &h.EmbedsInterface},
		{name: ComparableKey, key: &h.Comparable},
	} {
		key, registered := meta.Lookup[bool](r, k.name)
		if !registered {
			return Handles{}, fmt.Errorf(
				"golang: the registry contains no boolean key %s, which the Go frontend registers", k.name)
		}
		*k.key = key
	}
	return h, nil
}

// registerKey registers one key under the type its consumers read.
// The returned handle is zero for everything but the boolean keys
// the annotator stamps.
func registerKey(r *meta.Registry, spec meta.KeySpec) (meta.Key[bool], error) {
	switch spec.Name {
	case ConstraintKey, GeneratedKey, UnderlyingKindKey, ConstValueKey:
		_, err := meta.Register[string](r, spec)
		return meta.Key[bool]{}, err
	case TypeSetKey:
		_, err := meta.Register[[]string](r, spec)
		return meta.Key[bool]{}, err
	default:
		return meta.Register[bool](r, spec)
	}
}
