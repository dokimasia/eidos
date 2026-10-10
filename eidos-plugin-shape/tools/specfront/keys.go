// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// keyspace is the namespace of the keys that the spec frontend stamps.
const keyspace = "shapespec"

// The keys that the spec frontend stamps on a spec's struct.
const (
	// KeyForm is the form of the spec: shape, mixin or contract.
	KeyForm meta.KeyName = keyspace + ".form"
	// KeyDetected marks a detected shape.
	KeyDetected meta.KeyName = keyspace + ".detected"
	// KeyDocumentary marks a documentary mixin or contract.
	KeyDocumentary meta.KeyName = keyspace + ".documentary"
	// KeyYields lists the detected shapes that rank before a shape.
	KeyYields meta.KeyName = keyspace + ".yields"
	// KeyRoles lists the roles of a contract in the order of the spec,
	// each as the name, an equals sign and the arity, such as begin=one.
	KeyRoles meta.KeyName = keyspace + ".roles"
)

// The keys that the spec frontend stamps on the field of a param.
const (
	// KeyResolve is the resolution kind of a reference param.
	KeyResolve meta.KeyName = keyspace + ".resolve"
	// KeyRequired marks a param that an instance writes.
	KeyRequired meta.KeyName = keyspace + ".required"
	// KeyCounterexample marks a param whose value is an input that no
	// derivation could invent.
	KeyCounterexample meta.KeyName = keyspace + ".counterexample"
	// KeyApplies lists the roles of a contract under which a param
	// applies.
	KeyApplies meta.KeyName = keyspace + ".applies"
	// KeyMinimum is the least value of an int param.
	KeyMinimum meta.KeyName = keyspace + ".minimum"
	// KeyExcludes lists the params that an instance does not write beside
	// a param.
	KeyExcludes meta.KeyName = keyspace + ".excludes"
	// KeyAlsoOn lists the callable params whose callables also declare
	// the parameter that a host-param reference resolves to.
	KeyAlsoOn meta.KeyName = keyspace + ".also-on"
)

// The keys that the spec frontend stamps on the field of a binding. A
// field with [KeyFrom] is a binding, and every other field is a param.
const (
	// KeyFrom is the list that a binding's index counts in.
	KeyFrom meta.KeyName = keyspace + ".from"
	// KeyIndex is the position of a binding in its list.
	KeyIndex meta.KeyName = keyspace + ".index"
)

// Keys claims the namespace shapespec and registers every key that the
// spec frontend stamps. The spec frontend registers the keys through its
// role, under the language's spelling.
//
// Error modes: the error of the claim of the namespace, such as a
// namespace that another registrant claimed, and otherwise the errors of
// every registration, joined.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(keyspace); err != nil {
		return err
	}
	structs := []symbol.Kind{symbol.KindStruct}
	fields := []symbol.Kind{symbol.KindField}
	strs := []meta.KeySpec{
		{Name: KeyForm, Kinds: structs, Doc: "the form of a spec: shape, mixin or contract"},
		{Name: KeyResolve, Kinds: fields, Doc: "the resolution kind of a reference param"},
		{Name: KeyFrom, Kinds: fields, Doc: "the list that a binding counts in: input or result"},
	}
	bools := []meta.KeySpec{
		{Name: KeyDetected, Kinds: structs, Doc: "marks a detected shape"},
		{Name: KeyDocumentary, Kinds: structs, Doc: "marks a mixin or a contract that licenses no check"},
		{Name: KeyRequired, Kinds: fields, Doc: "marks a param that every instance writes"},
		{Name: KeyCounterexample, Kinds: fields, Doc: "marks a param whose value no derivation could invent"},
	}
	lists := []meta.KeySpec{
		{Name: KeyYields, Kinds: structs, Doc: "the detected shapes that rank before a shape"},
		{Name: KeyRoles, Kinds: structs, Doc: "the roles of a contract with their arities, as role=arity"},
		{Name: KeyApplies, Kinds: fields, Doc: "the roles of a contract under which a param applies"},
		{Name: KeyExcludes, Kinds: fields, Doc: "the params that an instance does not write beside a param"},
		{Name: KeyAlsoOn, Kinds: fields, Doc: "the callable params whose callables declare a host parameter too"},
	}
	ints := []meta.KeySpec{
		{Name: KeyMinimum, Kinds: fields, Doc: "the least value of an int param"},
		{Name: KeyIndex, Kinds: fields, Doc: "the position of a binding in its list"},
	}
	var errs []error
	for _, s := range strs {
		_, err := meta.Register[string](r, s)
		errs = append(errs, err)
	}
	for _, s := range bools {
		_, err := meta.Register[bool](r, s)
		errs = append(errs, err)
	}
	for _, s := range lists {
		_, err := meta.Register[[]string](r, s)
		errs = append(errs, err)
	}
	for _, s := range ints {
		_, err := meta.Register[int64](r, s)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
