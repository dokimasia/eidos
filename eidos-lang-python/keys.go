// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package python

import (
	"errors"

	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// namespace is the namespace of metadata of every python key.
const namespace = "python"

// The classification and fact keys of the satellite. The frontend stamps
// them for the constructs of Python that the model has no field for.
const (
	// TestFileKey marks a file that the default collection of pytest finds
	// as a test: one named test_*.py or *_test.py, and every conftest.py.
	TestFileKey meta.KeyName = "python.testFile"

	// StubbedKey marks a source file that a stub file of the same name in
	// the same directory states the types of. A type checker reads the stub
	// and not the source, so the source file declares nothing.
	StubbedKey meta.KeyName = "python.stubbed"

	// TypedDictKey marks an interface that loads from a class with the base
	// TypedDict.
	TypedDictKey meta.KeyName = "python.typedDict"

	// BasesKey lists the bases of an enum as written, such as IntFlag, in
	// the order of the class statement.
	BasesKey meta.KeyName = "python.bases"

	// KeywordsKey lists the keyword arguments and the unpacked arguments of
	// a class statement as written, such as total=False, metaclass=Meta,
	// *bases and **options, in the order of the statement.
	KeywordsKey meta.KeyName = "python.keywords"

	// InitVarKey marks a field of a dataclass whose annotation is
	// InitVar[T], which the generated __init__ takes and the instance does
	// not keep.
	InitVarKey meta.KeyName = "python.initVar"

	// GeneratorKey marks a function or a method whose own body yields.
	GeneratorKey meta.KeyName = "python.generator"

	// TypeVarKey marks a variable that a call of TypeVar, ParamSpec or
	// TypeVarTuple declares. The value is the name of the callee.
	TypeVarKey meta.KeyName = "python.typeVar"

	// AnnotatedKey lists the metadata of an Annotated type as written: the
	// elements after the type where Annotated wraps the whole annotation of
	// a declaration, and the whole annotation where Annotated is inside
	// another type.
	AnnotatedKey meta.KeyName = "python.annotated"

	// LazyKey lists the modules that a file imports with the keyword lazy,
	// in the order of the statements.
	LazyKey meta.KeyName = "python.lazy"

	// FutureKey lists the features of the future statements of a file, in
	// the order of the statements.
	FutureKey meta.KeyName = "python.future"

	// HeadKey lists the comments of the head of a file, each without its #:
	// a shebang, an encoding declaration, # type: ignore, and the directives
	// of mypy, pyright and ruff.
	HeadKey meta.KeyName = "python.head"

	// ConditionKey lists the checks of sys.version_info, sys.platform and
	// TYPE_CHECKING that excluded a branch of a file, each once and as
	// written.
	ConditionKey meta.KeyName = "python.condition"
)

// The values of [TypeVarKey]: the callees that declare a type variable.
const (
	TypeVar      = "TypeVar"
	ParamSpec    = "ParamSpec"
	TypeVarTuple = "TypeVarTuple"
)

// Keys claims the python namespace of metadata for the registrant of the
// handle, and registers every key in it through the same handle. The
// frontend stamps the marks as booleans, the type variable as text and the
// lists as lists of text.
//
// A composition passes Keys to the workspace builder, and a corpus fixture
// passes it to the conformance runner, so both register one set. After a
// failed claim of the namespace, Keys does not register a key. Otherwise it
// registers every key that it can, and returns the errors of the others
// joined. A call through another registrant fails, because a namespace has
// one registrant.
func Keys(r *meta.Registry) error {
	if err := r.ClaimNamespace(namespace); err != nil {
		return err
	}
	var errs []error
	file := []symbol.Kind{symbol.KindFile}
	callable := []symbol.Kind{symbol.KindFunction, symbol.KindMethod}
	for _, spec := range []meta.KeySpec{
		{Name: TestFileKey, Kinds: file, Doc: "marks a file that the default collection of pytest finds as a test"},
		{Name: StubbedKey, Kinds: file, Doc: "marks a source file whose types a stub file of the same name states"},
		{
			Name: TypedDictKey, Kinds: []symbol.Kind{symbol.KindInterface},
			Doc: "marks an interface that loads from a TypedDict",
		},
		{
			Name: InitVarKey, Kinds: []symbol.Kind{symbol.KindField},
			Doc: "marks a field of a dataclass that the generated __init__ takes and the instance does not keep",
		},
		{Name: GeneratorKey, Kinds: callable, Doc: "marks a function or a method whose own body yields"},
	} {
		if _, err := meta.Register[bool](r, spec); err != nil {
			errs = append(errs, err)
		}
	}
	if _, err := meta.Register[string](r, meta.KeySpec{
		Name: TypeVarKey, Kinds: []symbol.Kind{symbol.KindVariable},
		Doc: "the callee that declares a type variable: TypeVar, ParamSpec or TypeVarTuple",
	}); err != nil {
		errs = append(errs, err)
	}
	for _, spec := range []meta.KeySpec{
		{Name: BasesKey, Kinds: []symbol.Kind{symbol.KindEnum}, Doc: "the bases of an enum, as written"},
		{
			Name: KeywordsKey, Kinds: []symbol.Kind{symbol.KindStruct, symbol.KindInterface, symbol.KindEnum},
			Doc: "the keyword arguments and the unpacked arguments of a class statement, as written",
		},
		{
			Name: AnnotatedKey,
			Kinds: []symbol.Kind{
				symbol.KindField, symbol.KindParam, symbol.KindReturn, symbol.KindVariable, symbol.KindConstant,
				symbol.KindAlias,
			},
			Doc: "the metadata of an Annotated type, as written",
		},
		{Name: LazyKey, Kinds: file, Doc: "the modules that a file imports with the keyword lazy"},
		{Name: FutureKey, Kinds: file, Doc: "the features of the future statements of a file"},
		{Name: HeadKey, Kinds: file, Doc: "the comments of the head of a file, each without its #"},
		{
			Name: ConditionKey, Kinds: file,
			Doc: "the checks of the version, the platform and TYPE_CHECKING that excluded a branch",
		},
	} {
		if _, err := meta.Register[[]string](r, spec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
