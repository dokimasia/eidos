// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace every golang key registers
// under.
const namespace = "golang"

// The satellite's classification and fact keys, declared at the
// root because one namespace claim serves both roles: the frontend
// stamps what a parse can see, the annotator what the sealed graph
// proves.
const (
	// TestFileKey classifies a file the go tool treats as a test.
	TestFileKey meta.KeyName = "golang.testFile"

	// ConstraintKey classifies a file whose build constraint falls
	// outside the load's tag set. The value is the constraint as
	// written or filename-implied.
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
	// elements: usable as a bound, never as a value's type.
	ConstraintInterfaceKey meta.KeyName = "golang.constraintInterface"

	// EmptyInterfaceKey marks an interface with no members at all.
	EmptyInterfaceKey meta.KeyName = "golang.emptyInterface"

	// ReceiverPointerKey marks a method declared on a pointer
	// receiver.
	ReceiverPointerKey meta.KeyName = "golang.receiverIsPointer"

	// UnderlyingKey records a defined type's underlying shape:
	// basic, named, pointer, slice, array, map, chan or func.
	UnderlyingKey meta.KeyName = "golang.underlyingKind"

	// IterSeqKey and IterSeq2Key mark a callable whose first
	// result is the iterator type of that arity.
	IterSeqKey  meta.KeyName = "golang.iterSeq"
	IterSeq2Key meta.KeyName = "golang.iterSeq2"

	// ConstValueKey records a constant's exact evaluated value
	// where the package's own scope suffices to evaluate it, an iota
	// row's ordinal arithmetic included.
	ConstValueKey meta.KeyName = "golang.constValue"

	// SatisfiesErrorKey and SatisfiesStringerKey mark a type whose
	// workspace-visible method set has the interface's one method.
	// EmbedsInterfaceKey marks a struct embedding a type the graph
	// declares as an interface. ComparableKey marks a type every
	// field of which is provably comparable. Each stamps only what
	// the sealed graph proves: an absent fact is unknown, never a
	// negative.
	SatisfiesErrorKey    meta.KeyName = "golang.satisfiesError"
	SatisfiesStringerKey meta.KeyName = "golang.satisfiesStringer"
	EmbedsInterfaceKey   meta.KeyName = "golang.embedsInterface"
	ComparableKey        meta.KeyName = "golang.comparable"
)

// Handles are the typed keys the annotator stamps through,
// returned by [Register] to whichever role composes the
// registration.
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
// registry's error, returned as it is.
//
// # Allocation contract
//
// Register allocates the eleven kind lists of the keys, and the
// registry allocates its namespace claim and the growth of its lists
// and maps to sixteen keys: 28 allocations into a fresh registry.
func Register(r *meta.Registry) (Handles, error) {
	var h Handles
	if err := r.ClaimNamespace(namespace); err != nil {
		return h, err
	}
	file := []symbol.Kind{symbol.KindFile}
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
			Name: ReceiverPointerKey, Kinds: []symbol.Kind{symbol.KindMethod},
			Doc: "marks a method on a pointer receiver",
		},
		{
			Name: UnderlyingKey, Kinds: []symbol.Kind{symbol.KindAlias, symbol.KindEnum},
			Doc: "records a defined type's underlying shape",
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
			Name: SatisfiesErrorKey, Kinds: types,
			Doc: "marks a type whose visible method set satisfies error",
		},
		{
			Name: SatisfiesStringerKey, Kinds: types,
			Doc: "marks a type whose visible method set satisfies fmt.Stringer",
		},
		{
			Name: EmbedsInterfaceKey, Kinds: []symbol.Kind{symbol.KindStruct},
			Doc: "marks a struct embedding a type the graph declares as an interface",
		},
		{
			Name: ComparableKey, Kinds: types,
			Doc: "marks a type every field of which is provably comparable",
		},
	} {
		key, err := registerKey(r, spec)
		if err != nil {
			return h, err
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
	return h, nil
}

// registerKey registers one key under the type its consumers read.
// The returned handle is zero for everything but the boolean keys
// the annotator stamps.
func registerKey(r *meta.Registry, spec meta.KeySpec) (meta.Key[bool], error) {
	switch spec.Name {
	case ConstraintKey, GeneratedKey, UnderlyingKey, ConstValueKey:
		_, err := meta.Register[string](r, spec)
		return meta.Key[bool]{}, err
	case TypeSetKey:
		_, err := meta.Register[[]string](r, spec)
		return meta.Key[bool]{}, err
	default:
		return meta.Register[bool](r, spec)
	}
}
