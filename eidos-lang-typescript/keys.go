// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace every typescript key registers
// under.
const namespace = "typescript"

// The satellite's classification keys, which the frontend stamps.
const (
	// TestFileKey classifies a file Jest's default match names a test:
	// one under a __tests__ directory, one named test or spec, and one
	// whose name ends in .test or .spec before its extension.
	TestFileKey meta.KeyName = "typescript.testFile"

	// NamespaceKey records, on the package a namespace declares, the
	// namespace's dotted name as the file spells it, such as A.B.
	NamespaceKey meta.KeyName = "typescript.namespace"

	// CallSignatureKey records the call signatures of an interface or
	// of the object type an alias names, each as written, because the
	// model has no member for a callable object.
	CallSignatureKey meta.KeyName = "typescript.callSignature"

	// AmbientKey marks a declaration whose implementation is elsewhere:
	// one a declare statement or a declare block states, and every
	// declaration of a declaration file.
	AmbientKey meta.KeyName = "typescript.ambient"

	// GeneratorKey marks a generator function or method, which its *
	// declares, because the model has no generator mark.
	GeneratorKey meta.KeyName = "typescript.generator"

	// DefiniteAssignmentKey marks a property declared with !, which its
	// class assigns where the compiler cannot see. Such a property is
	// neither optional nor initialized in its declaration.
	DefiniteAssignmentKey meta.KeyName = "typescript.definiteAssignment"

	// ParameterPropertyKey marks a constructor parameter that declares
	// a field, and the field it declares, so the parameter list and the
	// field list each state which entry is both.
	ParameterPropertyKey meta.KeyName = "typescript.parameterProperty"

	// OptionalKey marks a method declared with ?, which a value of its
	// type may lack, because the model's method has no optional mark.
	OptionalKey meta.KeyName = "typescript.optional"

	// ReadonlyKey marks an index signature declared readonly, whose
	// entries a holder of the value cannot assign.
	ReadonlyKey meta.KeyName = "typescript.readonly"
)

// Keys claims the typescript namespace and registers every typescript
// key, in the form a composition and a corpus fixture declare them.
// It returns the claim's error, or every registration's error joined.
//
// # Allocation contract
//
// Keys allocates the nine kind lists of the keys, and the registry
// allocates its namespace claim and the growth of its lists and maps to
// nine keys: 24 allocations into a fresh registry.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestFileKey, Kinds: []symbol.Kind{symbol.KindFile},
		Doc: "marks a file Jest's default match names a test",
	})
	_, namespaceErr := meta.Register[string](r, meta.KeySpec{
		Name: NamespaceKey, Kinds: []symbol.Kind{symbol.KindPackage},
		Doc: "records the dotted name of the namespace a package declares",
	})
	_, callErr := meta.Register[[]string](r, meta.KeySpec{
		Name: CallSignatureKey, Kinds: []symbol.Kind{symbol.KindInterface, symbol.KindAlias},
		Doc: "records the call signatures of a callable object type as written",
	})
	_, ambientErr := meta.Register[bool](r, meta.KeySpec{
		Name: AmbientKey,
		Kinds: []symbol.Kind{
			symbol.KindStruct, symbol.KindInterface, symbol.KindEnum, symbol.KindAlias,
			symbol.KindFunction, symbol.KindConstant, symbol.KindVariable,
		},
		Doc: "marks a declaration whose implementation is elsewhere",
	})
	_, generatorErr := meta.Register[bool](r, meta.KeySpec{
		Name: GeneratorKey, Kinds: []symbol.Kind{symbol.KindFunction, symbol.KindMethod},
		Doc: "marks a generator function or method",
	})
	_, definiteErr := meta.Register[bool](r, meta.KeySpec{
		Name: DefiniteAssignmentKey, Kinds: []symbol.Kind{symbol.KindField},
		Doc: "marks a property declared with !, which its class assigns where the compiler cannot see",
	})
	_, propertyErr := meta.Register[bool](r, meta.KeySpec{
		Name: ParameterPropertyKey, Kinds: []symbol.Kind{symbol.KindField, symbol.KindParam},
		Doc: "marks a constructor parameter that declares a field, and the field it declares",
	})
	_, optionalErr := meta.Register[bool](r, meta.KeySpec{
		Name: OptionalKey, Kinds: []symbol.Kind{symbol.KindMethod},
		Doc: "marks a method declared with ?, which a value of its type may lack",
	})
	_, readonlyErr := meta.Register[bool](r, meta.KeySpec{
		Name: ReadonlyKey, Kinds: []symbol.Kind{symbol.KindMethod},
		Doc: "marks an index signature declared readonly",
	})
	return errors.Join(testErr, namespaceErr, callErr, ambientErr, generatorErr, definiteErr, propertyErr,
		optionalErr, readonlyErr)
}
