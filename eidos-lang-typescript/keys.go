// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the metadata namespace of every typescript key.
const namespace = "typescript"

// GeneratedMark is the text that the generators of the ecosystem write into
// the first comment of a file that they generate.
const GeneratedMark = "@generated"

// The frontend stamps these classification keys of the satellite.
const (
	// TestFileKey marks a file that the default match of Jest finds as a
	// test: one under a __tests__ directory, one named test or spec, and
	// one whose name ends in .test or .spec before its extension.
	TestFileKey meta.KeyName = "typescript.testFile"

	// GeneratedKey records the first comment of a file as written, where
	// the comment contains [GeneratedMark].
	GeneratedKey meta.KeyName = "typescript.generated"

	// NamespaceKey records the dotted name of a namespace, such as A.B, on
	// the package that the namespace declares.
	NamespaceKey meta.KeyName = "typescript.namespace"

	// CallSignatureKey records the call signatures of an interface or of
	// the object type of an alias, each in the layout of the spelling of a
	// type. The model has no member for a callable object.
	CallSignatureKey meta.KeyName = "typescript.callSignature"

	// AmbientKey marks a declaration whose implementation is elsewhere,
	// such as one in a declare block or in a declaration file.
	AmbientKey meta.KeyName = "typescript.ambient"

	// GeneratorKey marks a generator function or method, which has a *
	// before its name. The model has no generator mark.
	GeneratorKey meta.KeyName = "typescript.generator"

	// DefiniteAssignmentKey marks a property declared with !, which its
	// class assigns where the compiler cannot see. Such a property is
	// neither optional nor initialized in its declaration.
	DefiniteAssignmentKey meta.KeyName = "typescript.definiteAssignment"

	// ParameterPropertyKey marks a constructor parameter that declares a
	// field, and the field that the parameter declares. A reader of the
	// parameter list or the field list finds the entries that are both.
	ParameterPropertyKey meta.KeyName = "typescript.parameterProperty"

	// AbstractKey marks a property that an abstract class declares with
	// abstract, which a subclass declares. The model keeps the abstract mark
	// of a method alone.
	AbstractKey meta.KeyName = "typescript.abstract"

	// ConditionalTypeKey records the spelling of a conditional type, such
	// as T extends string ? A : B, on the alias whose target it is. The
	// model keeps the target as a named reference with that spelling.
	ConditionalTypeKey meta.KeyName = "typescript.conditionalType"
)

// Keys claims the typescript namespace and registers every typescript
// key. A composition and a corpus fixture call it to declare the keys. It
// returns the error of the claim, or the joined errors of every
// registration.
//
// # Allocation contract
//
// Keys allocates the ten kind lists of the keys. The registry allocates
// its namespace claim and the growth of its lists and maps to ten keys.
// A registration into a fresh registry allocates 25 times.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	_, testErr := meta.Register[bool](r, meta.KeySpec{
		Name: TestFileKey, Kinds: []symbol.Kind{symbol.KindFile},
		Doc: "marks a file that the default match of Jest finds as a test",
	})
	_, generatedErr := meta.Register[string](r, meta.KeySpec{
		Name: GeneratedKey, Kinds: []symbol.Kind{symbol.KindFile},
		Doc: "records the first comment of a file that a foreign generator wrote",
	})
	_, namespaceErr := meta.Register[string](r, meta.KeySpec{
		Name: NamespaceKey, Kinds: []symbol.Kind{symbol.KindPackage},
		Doc: "records the dotted name of the namespace that a package declares",
	})
	_, callErr := meta.Register[[]string](r, meta.KeySpec{
		Name: CallSignatureKey, Kinds: []symbol.Kind{symbol.KindInterface, symbol.KindAlias},
		Doc: "records the call signatures of a callable object type in the layout of a spelled type",
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
		Doc: "marks a constructor parameter that declares a field, and the field that it declares",
	})
	_, abstractErr := meta.Register[bool](r, meta.KeySpec{
		Name: AbstractKey, Kinds: []symbol.Kind{symbol.KindField},
		Doc: "marks an abstract property, which a subclass declares",
	})
	_, conditionalErr := meta.Register[string](r, meta.KeySpec{
		Name: ConditionalTypeKey, Kinds: []symbol.Kind{symbol.KindAlias},
		Doc: "records the spelling of the conditional type that an alias names",
	})
	return errors.Join(testErr, generatedErr, namespaceErr, callErr, ambientErr, generatorErr, definiteErr,
		propertyErr, abstractErr, conditionalErr)
}
