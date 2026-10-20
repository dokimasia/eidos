---
rfc: 0073
title: The Python satellite
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-20
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0064, ADR-0065, ADR-0066, ADR-0067, ADR-0068, ADR-0069, ADR-0070
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0073: The Python satellite

## Summary

We propose the satellite `eidos-lang-python`, which reads and writes every declaration construct of
Python 3 in source files and stub files, and every type form of the typing specification. The
frontend parses through the tree-sitter grammar of the fork `dokimasia/tree-sitter-python`, which
parses Python 3.15 with its lazy imports. A unit is one module. The partition reads the source roots
of each project from its `pyproject.toml`. The load reads the standard library from typeshed's
stubs, which the satellite embeds at a pinned commit, and the installed distributions from the
site-packages of a virtual environment.

The backend writes modules and stub files for Python 3.11 to 3.15 in the layout of ruff's formatter.
A marker is an annotation whose name starts with `python:`. The backend writes each construct
without a field in the model from a marker, and the head of a module from the header of its file.
The rules project the classes of `builtins`, `typing`, `typing_extensions`, `collections.abc`,
`collections` and `datetime` that typeshed declares, the well-known types of the standard library,
and `Literal`. A class that a dataclass decorator or a `dataclass_transform` of PEP 681 marks
constructs through its generated `__init__`.

The kernel gains the way a caller passes the argument of a parameter, the delete accessor of a
property, the role of a target whose functions overload, the convention of a member by its level and
its mutability, a lowering policy that applies to a whole plan, and the C3 linearization of the
member walk. The stock binary gains the frontend, the rules, the target, the stores and the plan
`python`. The support matrix gains a Python column.

## Motivation

dokimi, the reference consumer, reads Python beside Go, TypeScript, Rust, Java and Kotlin. Before
this RFC, no satellite of eidos read or wrote Python. That blocked these generators:

- A generator cannot read the dataclasses, the pydantic models and the Protocols of a Python service
  to write the Go server types or the TypeScript client types of its API.
- A generator cannot write Python dataclasses, Protocols, enums or a typed client from the
  declarations of Go, TypeScript, Java, Rust, protobuf or GraphQL.
- The completeness check and the support matrix have no Python column.

Two facts of a Python signature had no field in the model. A keyword-only parameter does not accept
a positional argument. The `timeout` of `def get(url, *, timeout: float)` is such a parameter. A
positional-only parameter does not accept a keyword argument. A generator that writes a delegating
call has to know which form each parameter accepts. A property can also have a deleter beside its
getter and its setter, and the model had no form for a deleter.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Model | `eidos-core/symbol`, `eidos-core/symbol/schema` | `symbol.Passing`, `Param.Passing` with the fact `Passing`, and `symbol.AccessorDelete`, with the regenerated model and facades |
| Identities | `eidos-core/frontend/load` | the discriminator marks of the passing of a parameter and of the type of a receiver, `_` for a parameter without a type, and `del` for a delete accessor |
| Kernel roles | `eidos-core/plugin`, `eidos-core/backend`, `eidos-core/layout`, `eidos-core/workspace` | `plugin.Overloader` with the exemption of the collision check, `plugin.MemberRespeller`, `PolicySpec.Plan`, `RenderContext.Policy` and `Policy.Lookup` |
| Surface | `eidos-core/toolchain` | the passing of a parameter and the modifier `delete` in `Surface` |
| Member walk and projection | `eidos-core/rules` | the C3 linearization under `ShadowLinearise`, `ParamView.Passing`, the passing in the signature of a member, the three arguments of `ShapeRules.ShapeOf`, and a stream of the first type argument |
| Render kit | `eidos-core/backend/render` | `ImportSet.Enter`, `Pending`, `Body`, `SetPolicy`, `Policy` and `Has`, and the host of a body |
| Grammar | `eidos-lang/treesitter/python` | new: the pinned grammar of the fork |
| Lowering helpers | `eidos-lang/lowering` | `DeleteAccessor`, which every other backend calls |
| Satellite | `eidos-lang-python` | new module `go.dokimi.dev/eidos/lang/python`: the language, the comment syntax, the keys, the markers, the policies, `Stated`, `Header`, the builtin classes, and the literals of Python |
| Projects | `eidos-lang-python/internal/project` | new: the read of `pyproject.toml`, the source roots of the build backends and the module path of a file |
| Frontend | `eidos-lang-python/frontend` | new: the partition into modules, the lowering, the resolution, the re-exports, the dependency rounds and the stores |
| Standard library | `eidos-lang-python/frontend/typeshed` | new: the stubs of typeshed's `stdlib` at a pinned commit, embedded, and its table `VERSIONS` |
| Rules | `eidos-lang-python/rules` | new: the builtins, the members, the roles, the resolution, the values, the enums, the dataclasses and eight optional capabilities |
| Spoke and names | `eidos-lang-python/spell` | new: `Type`, `Name`, `Member`, `Filename`, `StubFilename`, `Package`, `IsIdentifier` and `IsKeyword` |
| Backend | `eidos-lang-python/backend` | new: the kit compositions of modules and of stub files, the templates, the layout, the coverage, the lowering, the scaffold and the import block |
| Toolchain adapter | `eidos-lang-python/testing` | new: the parse through CPython's `ast`, the type check through pyright in strict mode, the tests through `unittest`, and `Satisfies` |
| Backends | the six backend modules | a stance on the fact `Passing`, and a refusal of the delete accessor |
| Workspace | `go.work`, the module files | the module, the requirement of the satellite by the root module and by `eidos-conformance`, and of the fork by `eidos-lang` |
| Repository | `.ergon.yaml`, `.ergon/local` | the typeshed tree outside the licence check, Python and pyright in CI, and the lint rules of a plugin module |
| Binary | `cmd/eidos` | the frontend, the rules, the target, the stores and the plan `python` |
| Conformance | `eidos-conformance` | the corpus, the matrix entry and the fixture `shop` |

### Invariants

1. Every declaration construct of Python 3 and every type form of the typing specification has a form
   in the model, and the backend writes it. The tables of constructs list each form. A class nested
   in an enum and a construct of Python 2 in a signature have no form, and report
   `UnmodeledConstruct`.
2. A module that CPython 3.15 compiles loads without an Error, and so does a stub file that pyright
   accepts. A module whose syntax the grammar of the fork does not parse reports `UnparsedFile`.
3. The frontend loads each file that the backend writes into the declarations, the shapes, the
   annotations and the stamps that the emit model stated.
4. The toolchain adapter parses, type-checks and runs the tests of the mirror that the shop fixture
   writes. The Go toolchain type-checks its Go model, and tsc type-checks its TypeScript client.
5. A construct, a fact, a kind or a name that Python does not spell reports a positioned finding.

### The constructs

Classes and their members:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `class` | a struct, with its bases in `Extends` in the order of the statement | a struct |
| `class` with the base `Protocol` | an interface, with its other bases in `Extends` | an interface |
| `class` with the base `TypedDict`, or with a base that is a TypedDict of the same module | an interface with the stamp `python.typedDict`, with a TypedDict base of the module in `Extends` | an interface with the marker `python:typeddict` |
| `class` with the base `NamedTuple` | a struct with `NamedTuple` in `Extends` and immutable fields | a struct with the base |
| `class` with the base `Enum`, `IntEnum`, `StrEnum`, `Flag`, `IntFlag` or `ReprEnum` | an enum with a variant for each member, the data type in `Underlying`, and every base in the stamp `python.bases`. An enum has no type parameters, no `Extends` and no nested types | an enum, and the marker `python:bases` for bases that `Underlying` does not decide |
| `Generic[T]` or `Protocol[T]` among the bases | the type parameters, without the base. A class without either form takes the type variables that its bases name, in the order of their first use | type parameters |
| `class C[T: B = D]` | the type parameters with their bounds and defaults. The constraints `T: (A, B)` are one bound of the form Union, and `*Ts` and `**P` keep their marks in the name | type parameters |
| The keyword arguments and the unpacked arguments of a class statement, such as `metaclass=`, `total=`, `closed=`, `extra_items=` and `**options` | the stamp `python.keywords`, each as written | the marker `python:keywords` |
| A decorator | an annotation with the dotted name, and the arguments verbatim. A decorator that is no dotted name and no call of one is an annotation of the whole expression without arguments | an annotation |
| A nested class | a nested declaration. A class nested in an enum reports `UnmodeledConstruct` | a nested declaration |
| An annotated attribute of a class body, `x: T = v` | a field, with its default as its value | a field |
| `x: ClassVar[T] = v` | a field at type level | a field at type level |
| `x: Final[T] = v`, `x: ReadOnly[T]` | an immutable field | an immutable field |
| An attribute without an annotation, `x = v` | a field at type level without a type, with the type of its type comment. In a TypedDict and a NamedTuple the field is not at type level, and in an enum `nonmember(cast("T", v))` gives it the type `T` | a field at type level |
| An attribute without an annotation whose value is a type expression, and `X: TypeAlias = T`, outside an enum | an alias in the `Types` of the class | an alias |
| `self.x: T = v` in a method, and `self.x = v` in `__init__` | a field without a value, after the fields of the class body, at full depth. A static method and an enum declare none | a field in the class body |
| `__slots__` | a field at type level with its value | a field |
| `x: InitVar[T]` of a dataclass | a field with the stamp `python.initVar` | a field with the marker `python:initvar` |

Callables:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `def` and `async def` in a module | a function | a function |
| `def` in a class | a method, with its first parameter as its receiver | a method |
| `@staticmethod`, `@classmethod` | a method at type level, and the annotation. A static method has no receiver | a method at type level, with a receiver for a class method |
| `__init__`, `__new__` | a method that constructs. `__new__` is at type level | a method that constructs |
| `@property`, `@functools.cached_property`, `@x.getter`, `@x.setter`, `@x.deleter` | methods with the accessors get, set and delete | methods with an accessor |
| `@overload` | a callable for each overload with the annotation as written and the mark `Overload`, and the implementation without the mark | callables with the mark `Overload`, and the implementation without it |
| `@abstractmethod` of `abc`, `@final` and `@override` of `typing` or `typing_extensions` | an abstract, a final and an overriding method, and the annotation | the fields `Abstract`, `Final` and `Override` |
| A function whose own body yields | the stamp `python.generator`, at every depth | a result such as `Iterator[T]` |
| Parameter before `/` | `Passing` positional | `Passing` positional |
| Parameter after `*` or `*args` | `Passing` keyword | `Passing` keyword |
| `*args`, `**kwargs` | a variadic parameter, positional or by keyword | a variadic parameter |
| A default | the default verbatim | a default |
| A return annotation | one result, no result for `-> None` and for an `__init__` without an annotation, and one result without a type for any other function without an annotation | results |
| A type comment, `# type: (A) -> R` | the types of the signature: a type for each parameter without an annotation, where the comment lists one type for each parameter after the receiver, and the result where the signature has no return annotation | the types of the signature |

Modules:

| Construct | The frontend loads | A generator states |
|---|---|---|
| A module | a package with one file, and its docstring as the documentation of the file, and of the package where the package has none. The source of a pair of a source and a stub is a second file, stamped `python.stubbed` | a routed file |
| An annotated assignment | a variable | a variable |
| `X: Final = v`, `X: Final[T] = v` | a constant, without a type for a bare `Final` | a constant |
| An assignment without an annotation | a variable without a type | a variable |
| An assignment of one name whose value is a type expression, as `Ids = list[int]` | an implicit alias, as a type checker reads it | an alias |
| A type comment, `x = v  # type: T` | the type of the variable | the type of the variable |
| `type X[T] = U`, `X: TypeAlias = U` | an alias | an alias, in the form of the target version |
| `X = NewType("X", T)` | a defined alias of `T` | a defined alias |
| `T = TypeVar("T")`, `ParamSpec`, `TypeVarTuple` | a variable with the stamp `python.typeVar`, which states the callee, and a type parameter of each declaration that names it with the bound, the constraints, the default and the variance of the call | type parameters |
| The functional forms of `NamedTuple`, `TypedDict`, an enum class and `collections.namedtuple` | the declaration of the class syntax. A named tuple of `collections` extends `NamedTuple` of `typing`, and its fields have no type | the declaration. The backend writes the functional form where the class syntax cannot state a name |
| `__all__` | the exports of the file | the marker `python:all` of the header |
| `import`, `from ... import`, a relative import, a star import, `lazy import` | the imports of the file, one for each module of `import a, b`, and the stamp `python.lazy` with the modules that it imports lazily | the imports that the spellings need, and the markers `python:import` and `python:lazy` of the header |
| `from __future__ import` | the stamp `python.future` of the file | the marker `python:future` of the header |
| `if TYPE_CHECKING:` and the checks of `sys.version_info` and `sys.platform` | the declarations of the branches that the options select | the declarations |
| The head comments: a shebang, the encoding declaration, `# type: ignore` and the directives of mypy, pyright and ruff | the stamp `python.head` of the file, each comment verbatim | the marker `python:head` of the header |
| A docstring of a module, a class, a callable or an attribute | the documentation | `Doc` |
| A `#` comment | a carrier, or the trailing comment of a declaration | `Comment` |

The types:

| Annotation | Model |
|---|---|
| `A`, `a.b.A`, `A[B, C]` | Named, with the type arguments in Args and the module of the import in Package |
| `int`, `str` and the other builtins | Named. The resolution step resolves it to its class in typeshed's `builtins`, which `ShapeRules` and `WellKnownRules` project. A reference without a target takes the same shape through `Builtin` |
| `X \| None`, `Union[X, None]` | Optional, with each `None` in Absent |
| `Optional[X]` | Optional without Absent, because the form states the optional, as a mark does |
| `X \| Y`, `Union[X, Y]` | Union, with `None` as one member of `X \| Y \| None` |
| `Callable[[A, B], R]` of `typing` or `collections.abc` | Func, the parameters and then the result |
| `Callable[..., R]`, `Callable[P, R]`, `Callable[Concatenate[A, P], R]` | Named with Args, because the parameter list is not exact. The rules fold it to Opaque |
| `tuple[A, B]`, `Tuple[A, B]`, `tuple[()]` | Tuple, immutable |
| `tuple[A, ...]` | List, immutable |
| `list[A]`, `dict[K, V]` and every other generic class | Named with Args. The rules project the generic classes of their table, and every other generic class folds to a reference with its arguments |
| `Literal[...]` | Named `Literal`, with each value as written in Args, which the rules fold by its values |
| `LiteralString`, `TypeGuard[T]`, `TypeIs[T]` | Named, which the rules fold to Text and to Bool |
| `type[C]`, `Self`, `Never`, `NoReturn`, `TypeForm[T]`, `Unpack[T]` | Named with lowered Args, which the rules fold to Opaque |
| `*Ts` | Named, with the spelling `*Ts`. `**P` occurs as the name of a type parameter alone |
| `Annotated[T, m]` | the reference of `T`, and the stamp `python.annotated` with each element of the metadata as compacted tokens |
| `Required[T]`, `NotRequired[T]`, `ReadOnly[T]`, `ClassVar[T]`, `Final[T]`, `InitVar[T]` | the reference of `T`, and the field that the qualifier sets. A parameter keeps the reference alone |
| A string annotation, `"A"` | the reference that the string contains, at the position of its content. A string that does not parse reports `UnparsedFile` at Warning, and the type keeps the string |

The sections below give the rules of each part.

### The parser

A frontend parses through a pinned library. tree-sitter is the platform for every language without
a proven pure-Go parser, and Python has no pure-Go parser that we can pin. gpython parses the grammar
of Python 3.4, which predates variable annotations, `async def` and positional-only parameters.

The upstream grammar, `tree-sitter/tree-sitter-python`, released 0.25.0, and it has not merged a
change since 2025-09-15. Its open pull requests for lazy imports and PEP 798 have no review. The fork
`dokimasia/tree-sitter-python` is the source of truth of the grammar. On top of upstream's head
`26855ea` it adds four features:

- PEP 810 lazy imports, the node `lazy_import_statement`
- PEP 798 unpacking in comprehensions
- PEP 696 type parameter defaults, the node `default_type`
- the plus sign in literal patterns of CPython 3.15

The fork parses 5,888 of the 5,891 example files of numpy, django, flask and CPython main. Three files
fail. CPython's `test_annotationlib.py` has annotations with a starred argument and with an extended
slice, and its `test_compile.py` has a dedent inside parentheses. `badsyntax_3131.py` is invalid
syntax, which CPython's tests read as such.

The fork's `go.mod` declares upstream's module path, `github.com/tree-sitter/tree-sitter-python`.
The go command applies the replace directives of the main module alone, so a consumer of eidos builds
against the fork only when the fork declares its own path. A commit of the fork changes its `go.mod`
to `github.com/dokimasia/tree-sitter-python`. `eidos-lang` requires the fork at the pseudo-version of
that commit.

```go
// Package python pins the fork dokimasia/tree-sitter-python and loads its
// grammar as [Grammar], which parses the .py and .pyi files of Python 3.15
// with its lazy imports.
//
// Linking the package compiles the grammar's parser and external scanner
// from 3.5 MB of C, so a binary that imports it builds with cgo and a C11
// compiler.
package python

// module is the path of the grammar binding's module, whose build
// version the grammar's version names.
const module = "github.com/dokimasia/tree-sitter-python"

// Grammar parses .py and .pyi files.
var Grammar = treesitter.Load("python", module, tspython.Language())
```

The grammar is ABI 15, which go-tree-sitter v0.25.0 loads. `Load` panics at the start of the
program on an ABI that the runtime refuses. Its `parser.c` is 3.5 MB of C and its external scanner
15 KB.

### Projects, units and packages

- The selection is `**/*.py` and `**/*.pyi`, negating `**/.venv/**`, `**/venv/**`,
  `**/site-packages/**`, `**/__pycache__/**`, `**/.tox/**`, `**/.nox/**` and `**/node_modules/**`.
- A unit is one module, because Python scopes a name to its module. A module is a `.py` file, a
  `.pyi` file, or both files of one name in one directory. When both exist, the unit lowers the stub,
  and the source file is a member without declarations with the stamp `python.stubbed`. A type
  checker reads the stub of a module and not its source.
- The package of a module is its dotted name with slashes, as in `shop/models` for `shop.models`. A
  package directory's `__init__.py` or `__init__.pyi` is the module of the package itself, `shop`.
  A root that is itself a package directory gives the empty package.
- The partition finds the governing `pyproject.toml` through the kit's `frontend.Nearest`, and the
  file is the ref's `Shared` input. A file outside every root also declares the probes of the
  `__init__.py` and `__init__.pyi` files of its run of package directories as shared inputs.
  `github.com/pelletier/go-toml/v2` v2.4.3 decodes the project file.
- The source root of a project is the directory in the table of its build backend:

  | Build backend | Key |
  |---|---|
  | setuptools, its `:__legacy__` backend, and a file without `build-backend` | the key `""` of `[tool.setuptools] package-dir`, and each `where` of `[tool.setuptools.packages.find]` |
  | hatchling | the `sources` of `[tool.hatch.build.targets.wheel]`, each prefix that maps to `""`, and the parent directory of each of its `packages` |
  | poetry-core | the `from` of each entry of `[tool.poetry] packages`, and the project directory for an entry without `from` |
  | uv_build | `module-root` of `[tool.uv.build-backend]` |
  | pdm-backend | `package-dir` of `[tool.pdm.build]` |
  | maturin | `python-source` of `[tool.maturin]` |

  Any other backend states no root. A root outside the workspace is dropped, and the longest root
  that contains a file decides. A project without a root in the table of its backend has the root
  `src` for the files below `src`. setuptools' automatic discovery, uv_build, pdm-backend and
  poetry-core default to that root.
- Every other file takes the root above the highest directory of the run of package directories
  above it: a file outside every source root of its project, such as a test under `tests/`, a file
  of a flat layout, and a file outside every project. pytest's default import mode and mypy find
  the name of such a module in the same way. A directory is a package directory when the claimed
  files contain its `__init__.py` or its `__init__.pyi`.
- `python.testFile` stamps a file whose name matches `test_*.py` or `*_test.py`, which is pytest's
  default `python_files`, and every `conftest.py`.
- The options fold into every unit key. Each parse and each dependency round reads a version
  `3.N` with N up to 255, and fails on any other text:

  ```go
  // Options is the declared configuration of the frontend, which every unit
  // key folds.
  type Options struct {
      // Version is the version of Python that the checks of sys.version_info
      // compare with, and the version that typeshed's VERSIONS selects the
      // standard library for, as 3.15. The empty version is 3.15.
      Version string

      // Platform is the value of sys.platform that the checks compare with,
      // such as linux, darwin or win32. The empty platform is linux.
      Platform string
  }
  ```

### The frontend

```go
// New builds the Python frontend through the kit. Nil options read Python
// 3.15 on linux. The frontend declares that the language overloads, so the
// discriminator of a callable contains the types of its parameters. It
// implements the exporter role and the dependent role over the stores of
// [Stores]. Its version folds the version of the grammar.
func New(opts *Options) plugin.Frontend

return frontend.New(python.Name, Lang, python.Syntax()).
    Version(python.FrontendVersion + "; " + pygrammar.Grammar.Version()).
    Overloads().
    Options(opts).
    Match(selection...).
    Units(partition).
    Parse(f.parse).
    Classify(frontend.TestFiles(python.TestFileKey, testFile)).
    Resolve(resolve).
    Exports(exports).
    Dependencies(f.dependencies).
    Stores(Stores).
    Keys(python.Keys).
    Build()
```

#### Classes and their kinds

The frontend sets the kind of a class from its keyword `metaclass=` and its direct bases. It reads
each through the import bindings of the file and the classes of the module. `typing_extensions`
reads as `typing`, because it declares the same forms. The first base that matches a rule decides:

- A class with the base `Protocol` is an interface. `Extends` leaves the `Protocol` base out.
  `Protocol[T]` gives the type parameters.
- A class with the base `TypedDict` is an interface with the stamp `python.typedDict`, and so is a
  class whose base is a TypedDict of the same module. `Extends` leaves the `TypedDict` base out and
  keeps a TypedDict base of the module. A field is `Optional` under `NotRequired`, and under the
  class keyword `total=False` without `Required`. A `ReadOnly` field is immutable.
- A class with the base `NamedTuple` is a struct whose fields are immutable.
- A class with an enum base is an enum. An enum base is an enum class of the module `enum`, an enum
  of the same module, or the metaclass `EnumMeta` or `EnumType`. The data type is `int` for
  `IntEnum` and `IntFlag` and `str` for `StrEnum`. For `class E(T, Enum)` it is the mixin type `T`.
- Every other class is a struct. A struct is `Abstract` where it declares an abstract method. A type
  checker refuses to instantiate such a class. A struct under `@final` is `Final`.

An attribute of an enum is a member when it has a value and its name is not a dunder, a sunder or a
private name. A value of `nonmember(...)` and a lambda do not make a member, and neither do the
names that `_ignore_` lists. Every other attribute is a field. A field without an annotation is at
type level, and an annotated field without a value is an attribute of the members.

Because the parse reads one module, a class whose enum base or TypedDict base another module
declares loads as a struct with the base in `Extends`.

#### Callables and parameters

- The receiver of a method is its first parameter, whatever its name. A static method has no
  receiver.
- A parameter has its annotation, or the type of its type comment, and no type without either. The
  types of a comment apply only where the comment lists one type for each parameter after the
  receiver. Its default is verbatim.
- `Passing` is positional for a parameter before `/`. In a signature without `/`, it is also
  positional for each leading parameter whose name starts with two underscores and does not end with
  two, up to the first variadic parameter or the first other name. The typing specification keeps
  that convention as the historical form of a positional-only parameter. `Passing` is keyword for a
  parameter after `*` or after `*args`.
- An `async def` is `Async`. An `async def` whose own body yields is an asynchronous generator. A
  call of it returns its iterator at once, so it is not `Async`. Its result folds to an asynchronous
  stream. The parse reads whether a body yields at signature depth too, so a dependency loads its
  generators.
- The overloads of a name load as one callable each, with the annotation `overload` and the mark
  `Overload`. The implementation loads after them without the mark.
- A getter, a setter and a deleter of one property are three methods of one name.
- A method of a Protocol is abstract where its body has no statement beside a docstring, `...` and
  `pass`. Every other method of a Protocol has `DefaultBody`, which a class that names the Protocol
  among its bases inherits.

The load spells the discriminator of a callable from its receiver and its parameters, so the
overloads, the implementation and the accessors of one name have distinct identities. Each mark
changes the calls that the callable accepts:

| Part | Discriminator |
|---|---|
| A receiver with a type | the type and `;`, before the parameters |
| A parameter | its type, and `_` for a parameter without a type |
| A parameter that a caller passes by keyword alone | its name and `=` before its type |
| A parameter that a caller passes by position alone | `/` after its type |
| `*args`, `**kwargs` | `...` and `**` before the type |
| A deleter | `del`, a keyword of Python, in place of the empty list of the getter |
| The implementation of an overloaded name | `=` before the discriminator of its parameters |

So `def __init__(self, sku: str, *, qty: int = 1)` spells `str,qty=int`, and the setter
`def total(self, value)` spells `_`. A default does not change the discriminator.

#### Attributes and variables

- An annotated attribute of a class body is a field. `ClassVar` sets the type level, and `Final`
  makes the field immutable. Python shares an attribute without an annotation between the
  instances, so it is a field at type level, with the type of its type comment and no type without
  one. In a TypedDict and a NamedTuple such an attribute is not at type level.
- The parse reads the annotated assignments to the receiver, as `self.<name>`, in the body of each
  method, and every assignment to the receiver in the body of `__init__`. Each name that the class
  body does not declare is a field without a value. A static method and an enum do not declare such
  a field.
- The parse reads four facts of a body: its docstring, these assignments, whether a callable yields,
  and whether a method of a Protocol has a statement beside a docstring, `...` and `pass`. It does
  not read any other part of a body. At signature depth it skips the assignments.
- A module-level annotated assignment is a variable, and one with `Final` a constant. An assignment
  without an annotation is a variable without a type. Each name of `a = b = v` is a variable with the
  value `v`, and each name of `a, b = v` a variable without a value.
- An assignment of one name without an annotation and without a type comment is an implicit alias
  where its value is a type expression, at the top level of a module and in a class outside an enum.
  A type expression is one of these values:
  - a subscript of a class of `builtins`, of a name of `typing`, `collections.abc` or
    `collections`, or of a class or an alias that the module declares before the statement
  - a class of `builtins`, a name of `collections.abc`, `Any`, `LiteralString`, `Never` and
    `NoReturn` of `typing`, and a class or an alias that the module declares before the statement
  - a union with `|` that has `None` or a type expression among its operands

  The parse reads one module, so a name that an import binds from another module is no type
  expression, and the assignment is a variable.
- `__all__` lowers to the exports of the file and to no variable.

#### Types

A string annotation parses through the grammar as the type that it contains, inside parentheses
where it has a line break. Its positions are the positions of its content in the file. A qualifier
that wraps the whole annotation of a field sets the field of the model and leaves the reference of
its argument. A parameter keeps the reference, and only `Annotated` stamps it:

| Qualifier | Field |
|---|---|
| `ClassVar` | `Level` type |
| `Final`, `ReadOnly` | `Mutability` immutable |
| `Required`, `NotRequired` | `Optional`, in a TypedDict alone |
| `InitVar` of `dataclasses` | the stamp `python.initVar` |
| `Annotated[T, m...]` | the stamp `python.annotated` with each element of the metadata as compacted tokens |

`Annotated` inside another type keeps its argument's reference. The stamp `python.annotated` of the
declaration then lists the metadata of an outer `Annotated` first and the whole annotation last, as
compacted tokens.

#### Resolution and re-exports

For each file, the parse records its package, the import bindings by their local names with the
absolute paths of their modules, the modules that the imports name, the star imports, and the names
that the module declares at its top level. A relative import resolves against the package of the
file. The kernel passes the enclosing class of a reference as the owner of its scope. `Resolve`
returns tiers in Python's order of scopes for an annotation:

1. The enclosing class bodies come first, innermost first, one tier each. Python evaluates the
   annotations of a class body and of its method signatures in the namespace of the class.
2. The module follows in one tier with one candidate: the declaration of the module, or else the
   declaration that a `from` import binds the name to. A module has one namespace.
3. Each star import is a tier, in statement order.
4. The module `builtins` is the last tier of a name without a dot.

A dotted name whose first name is a class of the module resolves to a member of the class, as
`a.b.C` to `C` of the class `b` nested in `a`. Through `import a`, `a.b.C` resolves to `C` in the
module `a/b` first, and then to `C` of the class `b` of the module `a`. Through `from a import c`,
`c.X` resolves to `X` of the class `c` of the module `a` first, and then to `X` in the module `a/c`.
Each candidate of a dotted name is a tier of its own.

The frontend implements `Exporter`. For a module that does not declare `name`, `Exports(scope, name)`
returns the declaration that a `from` import of the module binds `name` to. The candidates for
`name` in its star imports follow, in statement order. An `import` statement does not publish a
name. A `from` import binds a name of the module at run time, so `from shop import Order` resolves
through `shop/__init__.py` when that file imports `Order` from `shop.models`. The visibility of a
name follows the typing specification's rules of a library's interface, and does not change whether
a reference to the name resolves.

#### Visibility

| Name | Visibility |
|---|---|
| A module-level name that `__all__` lists, where the module states `__all__` | public |
| A module-level name that `__all__` does not list, where the module states `__all__` | package |
| A name without a leading underscore, and a name that starts and ends with two underscores | public |
| A module-level name with a leading underscore, where the module does not state `__all__` | package |
| A member with one leading underscore | protected |
| A member whose name starts with two underscores and does not end with two | private and hard private, because Python mangles the name |

The rows apply in order, so `__all__` decides the visibility of every module-level name of a module
that states it. The parse reads `__all__` before the walk, from its assignments, its augmented
assignments and the calls of its methods `extend` and `append`. It reads them in every branch, a
branch that a check excludes included. Signature depth leaves out every declaration that is not
public.

#### Conditions

- `TYPE_CHECKING` is true, as a name and as a member of any module, because the frontend reads a
  module as a type checker does.
- A comparison of `sys.version_info` or of a slice of it with a tuple of integers compares with the
  option `Version`. A comparison of `sys.version_info[0]` and of `sys.version_info[1]` with an
  integer compares with the major and the minor version. An element of the constant past the minor
  version, such as a micro version, makes the check unknown where it decides the comparison, and so
  does a chain of comparisons.
- `sys.platform == "<name>"`, its negation and `sys.platform.startswith("<prefix>")` compare with the
  option `Platform`.
- `not`, `and`, `or` and parentheses combine such checks.
- The parse does not load a branch whose check is false, or a branch after a check that is true. It
  stamps the file under `python.condition` with each check that excluded a branch, once each, as
  compacted tokens such as `sys.version_info<(3,12)`. The branches of any other condition load in
  order, and so does every block of a `try`, `with`, `for`, `while` and `match` statement.
- The first declaration of a name in a module or a class is its declaration, and each later one
  reports `DuplicateName` at Info. The overloads and the accessors of one name are one name.

#### Comments and docstrings

- The parse reads every file as UTF-8, the default encoding of Python 3 source. The encoding
  declarations `utf-8`, `utf_8` and `utf-8-sig` name UTF-8. A file whose encoding declaration names
  another encoding reports `ForeignEncoding`, and its bytes keep their values.
- A docstring is the first statement of a module, a class or a callable when it is a string
  literal, and an f-string or a bytes literal is none. The parse trims it as PEP 257 does: the tabs
  expand to columns of eight, the first line loses its indentation, the other lines lose the
  indentation that they share, and the blank lines at both ends go. It enters through the kit's
  `DocLines`.
- A string literal statement after an assignment in a module or a class body is the attribute's
  docstring, as Sphinx's autodoc and griffe read it. Comments between the two statements do not
  separate them.
- The head comments are the comments before the first statement. A shebang counts on line 1, and an
  encoding declaration on line 1 or 2. Each enters the stamp `python.head` as its text after `#`.
- A carrier is a `#` comment directly before a declaration or before its first decorator, and a
  carrier in the trailing comment of a declaration. The carriers above the first statement of a
  module attach to its package. A decorator whose dotted name starts with the brand is native sugar,
  which also loads as an annotation. The kit lowers `@acme.gen.table(name="t")` to the canonical
  directive through `AttachSugar`.
- A carrier on an import, a parameter, `__all__`, `_ignore_` or a statement that declares nothing
  reports `UnaddressedCarrier`, and so does a carrier in a comment outside every declaration. The
  comments inside the body of a callable belong to statements that the model does not contain, and
  the parse does not read them.
- The trailing comment of a class or a callable is the comment on the line of its name. The trailing
  comment of any other declaration is the comment after its statement on the last line of the
  statement. A type comment is no trailing comment.

#### Dependencies

The frontend implements `Dependent` over two stores:

| Store | The composition opens | Paths inside |
|---|---|---|
| `typeshed` | the embedded tree of typeshed's `stdlib` at commit `e75a39fe`, in the layout of its repository | `stdlib/<module path>.pyi`, `stdlib/<package>/__init__.pyi`, and `stdlib/VERSIONS` |
| `venv` | the site-packages of the environment that `VIRTUAL_ENV` names: `lib/python<X.Y>/site-packages`, with the version of the key `version` or `version_info` of its `pyvenv.cfg`, and else `Lib/site-packages`, the directory of Windows | `<module path>.py` and `.pyi`, `<package>/__init__.py` and `.pyi`, and `<package>-stubs/` |

`frontend.Stores(getenv func(string) string) (map[string]fs.FS, error)` builds both without running
a tool. The `venv` store is absent where `VIRTUAL_ENV` is unset. `Stores` returns an error for an
environment whose `pyvenv.cfg` does not read or states no version, and for an environment without
either directory. The `.venv` directory beside a project file of the workspace is a second source of
the same layout, through workspace paths that the selection does not claim. The round reads the
`venv` store before these directories.

Every module reads the builtins, so round one places `builtins` beside its needs. The frontend places
a need in the order of the typing specification's module resolution:

1. A module of the workspace is no need.
2. typeshed places a module that `VERSIONS` lists with a range that contains the option `Version`.
   A module that the table does not list takes the range of its nearest listed parent, as
   `json/decoder` takes the range of `json`. The stub `<module>.pyi` comes before
   `<module>/__init__.pyi`.
3. A stub-only package `<package>-stubs` in site-packages places its modules. The round does not read
   `py.typed`. A stub-only package leaves the modules that it lacks to the next step.
4. site-packages places the module in the order `<module>/__init__.pyi`, `<module>/__init__.py`,
   `<module>.pyi` and `<module>.py`.

The round reports each need that no step places with its reason. Such a need does not yield a unit.
A dependency unit is one module and loads at signature depth. A file of the workspace below a
`site-packages` directory loads as a dependency too. The package of a stub-only package drops the
suffix `-stubs`.

typeshed's `stdlib` at `e75a39fe`, committed on 2026-10-17, has 753 files in 2,868,009 bytes: 752
stubs and `VERSIONS`. typeshed is licensed under Apache-2.0, with parts under the MIT licence. The
package `frontend/typeshed` embeds the tree with `go:embed`, and contains its licence file beside the
tree and the commit as `typeshed.Commit`.

#### Findings

| Code | Severity | Finding |
|---|---|---|
| `PYTHON-0001` `UnparsedFile` | Error, and Warning for a string annotation | a Python file has a syntax error. The file contributes every declaration outside the statements with an error. A string annotation that does not parse keeps its string |
| `PYTHON-0002` `BadCarrier` | Error | a directive carrier is outside the kernel grammar |
| `PYTHON-0003` `UnaddressedCarrier` | Error | a directive carrier or a decorator of the brand is on a subject the model cannot address |
| `PYTHON-0004` `UnreadProject` | Warning | a `pyproject.toml` does not read or parse. Its files take the root of the run of their package directories |
| `PYTHON-0005` `UnmodeledConstruct` | Warning | a construct of the source has no form in the model: a construct of Python 2 in a signature that the grammar keeps, such as a tuple parameter, and a class that an enum declares. The declaration loads without it |
| `PYTHON-0006` `DuplicateName` | Info | a module or a class declares a name a second time, and the first declaration is kept |
| `PYTHON-0007` `ForeignEncoding` | Warning | the encoding declaration of a file names an encoding other than UTF-8. The file parses as UTF-8 |
| `PYTHON-0008` `BadMarker` | Error | a decorator of the brand is not a valid directive. It attaches nothing and remains an annotation |

The kit reports at most 10 syntax errors of a file, and then one finding with the count of the rest.
A file of a store that does not read, and a `VERSIONS` that does not parse, fail the load with an
error and do not report a finding.

### Model changes

```go
package symbol

// Passing is the way a caller passes the argument of a parameter.
//
// The zero value is [PassingDefault], the way of the language of the
// callable: by position in Go, Java, TypeScript and Rust, and by position
// or by keyword in Python, Kotlin and C#.
type Passing uint8

const (
    // PassingDefault passes the argument in the way of the language.
    PassingDefault Passing = iota
    // PassingPositional passes the argument by position alone: a Python
    // parameter before the slash of its signature.
    PassingPositional
    // PassingKeyword passes the argument by keyword alone: a Python
    // parameter after the star of its signature, and a Ruby keyword
    // parameter.
    PassingKeyword
)

const (
    // AccessorNone, AccessorGet and AccessorSet are unchanged.

    // AccessorDelete deletes the property: no parameters, no result. The
    // deleter of a Python property is one.
    AccessorDelete
)
```

```go
package schema

type Param struct {
    // The existing fields are unchanged.

    Passing symbol.Passing `eidos:"both,fact=Passing"` // by position, by keyword, or the way of the language
}
```

- `Passing` is a schema edit and a regeneration. The passing of a parameter is a field and not
  metadata, as its default is. A generator that writes a keyword-only parameter as an ordinary one
  changes the calls that the callee accepts.
- The load spells the discriminator of a delete accessor as `del`. `del` is a keyword of Python, and
  no parameter type of a Python callable has that spelling. Python is the one language with a delete
  accessor.
- The Go, Java, TypeScript and Rust backends take `Holds` on `Passing`. Their callers pass every
  argument by position, and an argument by keyword has no form in those languages. The GraphQL
  backend takes `Holds`, because a GraphQL argument always has its name. The protobuf backend takes
  `Holds`, because a protobuf method takes one request.
- The six backends refuse a method with the delete accessor under `RefusedConstruct`, through the
  shared `lowering.DeleteAccessor`. Python is the one language that deletes a property.

#### Kernel changes

The satellite needs these roles, fields and methods of the kernel beside the model:

```go
package plugin

// Overloader is the role of a target whose language declares two functions
// of one name in one module, told apart by their parameters, as Python
// declares the overloads of a function. The layout does not count the
// functions of one emitted name in one routed scope as a collision of names
// where the target overloads. Functions of two emitted names that settle to
// one name still collide, and so do two declarations of one name where one
// of them is no function.
type Overloader interface {
    // Overloads reports whether the language of the target declares the
    // overloads of a function.
    Overloads() bool
}

// Member is what the convention of a target reads of a declared member
// beside its name: the kind of its host, its own kind, its visibility, its
// level and its mutability. A method has the zero mutability.
type Member struct {
    Host       symbol.Kind
    Kind       symbol.Kind
    Visibility symbol.Visibility
    Level      symbol.Level
    Mutability symbol.Mutability
}

// MemberRespeller is the role of a backend whose convention spells a member
// by its level and its mutability, as Python keeps the capitals of a
// constant of a class. The settle respells each field and each method of a
// declaration through it in place of [Respeller.Respell], and the lowering
// of a target names a member through it in the same way.
type MemberRespeller interface {
    RespellMember(m Member, name string) (string, error)
}

type PolicySpec struct {
    // The existing fields are unchanged.

    // Plan reports a policy that applies to every file of a plan, such as
    // the version of the language that the files run on, which the render
    // reads through [RenderContext.Policy]. A config file selects its
    // choice for every plan or for one plan. The directive of the target
    // has no param for it, and the target registers no metadata key for
    // it, so no declaration overrides it.
    Plan bool
}

// Lookup returns the choice of a key as [Policy.Choice] returns it, and
// reports false for a key that the policy does not declare, such as every
// key of the zero Policy.
func (p Policy) Lookup(k PolicyKey) (Choice, bool)

type RenderContext struct {
    // The existing fields are unchanged.

    // Policy is the plan's resolved lowering policy, the one that each
    // generator of the plan receives. The zero Policy declares no key.
    Policy Policy
}
```

```go
package render

// Enter names the file-level declaration under render by the name that
// [ImportSet.Reserve] took for it.
func (s *ImportSet) Enter(name string)

// Pending reports whether name is the name of a file-level declaration of
// the file that has not finished rendering. A language that runs the
// declarations of a file in order, such as Python, writes a reference to a
// pending name in a form that it evaluates later.
func (s *ImportSet) Pending(name string) bool

// Body returns the callable whose body the pass spells and the declaration
// whose template places the body.
func (s *ImportSet) Body() (callable, host symbol.Symbol)

// SetPolicy sets the plan's lowering policy, which [ImportSet.Policy]
// returns. The render pass sets it on the set of each worker.
func (s *ImportSet) SetPolicy(p plugin.Policy)

// Policy returns the plan's lowering policy that [ImportSet.SetPolicy]
// set, and the zero Policy for a set without one.
func (s *ImportSet) Policy() plugin.Policy

// Has reports whether the set records the entry of path and name, so a
// language reads back what it recorded for the file under render.
func (s *ImportSet) Has(path, name string) bool
```

```go
package rules

type ParamView struct {
    // The existing fields are unchanged.

    // Passing is the way a call passes the argument: by position alone, by
    // keyword alone, or in the way of the language. A generator that
    // writes a delegating call passes a keyword parameter by its name.
    Passing symbol.Passing
}

type ShapeRules interface {
    // ShapeOf returns the shape of the values of the reference ref to
    // decl. The shape can depend on the type arguments of ref, as the
    // shape of Literal["a"] of Python does.
    ShapeOf(ref *node.TypeRef, decl symbol.Symbol, v View) (TypeShape, bool)
}

const (
    // The existing rules of Shadowing are unchanged.

    // ShadowLinearise takes the arrivals from the type that comes
    // first in the C3 linearization of the walked type, by the path
    // that first reached it: the method resolution order of Python. A
    // hierarchy without a linearization, which Python refuses, takes
    // the nearest arrival, the first at one depth over the rest.
    ShadowLinearise
)
```

- The kit's `Builder.Overloads` and `Builder.RespellMember` declare the two roles of a backend, and
  the kit's frontend declares `Overloads` for the discriminator.
- The signature of a member under `MembersOf` contains the passing of each parameter, so two methods
  whose parameters differ in their passing have two signatures.
- The fold gives a stream shape without children the first type argument alone as its child. So
  `Generator[Y, S, R]` of Python and `AsyncGenerator<T, TReturn, TNext>` of TypeScript fold to a
  stream of their first argument.
- `toolchain.Surface` writes the name of a parameter that a caller passes by keyword alone, a `/`
  after a parameter that a caller passes by position alone, and the modifier `delete`.

### The rules

| Question | Answer |
|---|---|
| Builtin | `int` is a signed integer of the platform's width, `float` a float of 64 bits, `bool` a bool, `str`, `Text` and `LiteralString` text, `TypeGuard[T]` and `TypeIs[T]` a bool, `bytes` and `bytearray` bytes, `object` and `Any` the top type, `None` the well-known empty value, `datetime.datetime` the well-known timestamp, `datetime.timedelta` the well-known duration, and `BaseException` and `Exception` the well-known error |
| Builtin, with type arguments | `list`, `List`, `MutableSequence`, `deque` and `Deque` a mutable list, `Sequence`, `Collection`, `Iterable` and a bare `tuple` an immutable list, `dict`, `Dict`, `MutableMapping`, `defaultdict`, `DefaultDict`, `OrderedDict` and `ChainMap` a mutable map, `Mapping` and `frozendict` an immutable map, `Iterator` and `Generator` a stream, and `AsyncIterator`, `AsyncIterable` and `AsyncGenerator` an asynchronous stream. A class without type arguments has `Any` as each child |
| Builtin, a literal | `Literal` whose values are strings is text, integers a signed integer of the platform's width, booleans a bool, bytes literals bytes, and the members of one enum a reference to the enum. `None` beside other values makes the shape optional, and `None` alone is the well-known empty value. Every other `Literal` folds to Opaque |
| Members | the fields and the methods of a class, then the members of its bases in the C3 linearization of the class. A member of the class hides the inherited members of its name |
| ParamRole | every parameter is an input |
| ReturnRoles | one role for each result: a result whose type folds to a stream is a stream, and every other result is a value. The error model is `ErrorsRaised` |
| Resolve | the declaration that the spelling of a directive names in the scope of its subject, through the tiers of the frontend over the view. A function resolves by a scan of its package, because its identity has a discriminator |
| Values | samples, zeros and literals in Python's grammar of literals |
| EnumOf | the variants. An enum whose members all have a known value is value-formed, and each variant has its value. `auto()` gives the next value of the kind of the enum, and every other enum is identifier-formed |

The Package of a reference through an import is the module of the import. The rules classify such a
reference by its module and its spelling, so `Sequence` of `collections.abc` and of `typing` are one
name. A load that reads typeshed resolves a builtin class to its class in `builtins`. The rules
classify that class through `ShapeRules` and `WellKnownRules`, and a builtin without a target
through `Builtin`. typeshed declares many forms of `typing` as variables, such as `Optional`,
`Callable` and `Literal`. A reference to one of them does not have a target, and `Builtin` classifies
it.

Python's `int` has no bound. The rules fold it to the signed integer of the platform's width. The Go
spoke writes that width as `int`, the Java spoke as `long` and the Rust spoke as `isize`. The
protobuf, TypeScript and GraphQL spokes write the choice of their policies `protobuf.int64`,
`typescript.int64` and `graphql.int64`. The kernel has no form for a set, so `set`, `frozenset`,
`Set`, `FrozenSet`, `AbstractSet` and `MutableSet` fold to Opaque, as TypeScript's `Set` does.

The rules implement these optional capabilities:

| Capability | Python |
|---|---|
| `EnumRules` | the kind of an enum from its bases: plain, `Flag`, `IntFlag`, `StrEnum`, or a hook `_generate_next_value_`. The value of each member, the zero, the duplicate and the value out of range follow from the kind |
| `WellKnownRules` | `datetime.datetime`, `datetime.timedelta`, `BaseException` and `Exception` of typeshed |
| `ShapeRules` | the classes and the forms that typeshed declares in `builtins`, `typing`, `typing_extensions`, `collections.abc`, `collections` and `datetime`. A `NewType` and a `Literal` alias fold to a reference to the alias |
| `ErrorTypeRules` | a class whose bases reach an exception class of `builtins`. Its `__str__` and its `__cause__` are part of the contract where a class outside `builtins` declares them |
| `PropertyRules` | a getter of an instance, paired with the setter of its name |
| `ConstructRules` | the first class of the chain of first bases that states a constructor: the `__init__` that a dataclass or a class of a `dataclass_transform` generates, the `__new__` that a NamedTuple generates, and a declared `__init__`, or `__new__` without `__init__` |
| `EqualityRules` | the hash, as Python demands of a key of a dict and a member of a set |
| `GenericsRules` | the witness of a type parameter: its default, its first constraint, its bound, or else `object`. `Reified` reports false, because Python erases type arguments at run time |

#### Members

The member walk follows the method resolution order of Python. The kernel visits the bases of a
class in the order of the class statement and records the bases of each type that it visits. A name
that arrives from two or more types takes the members of the type that comes first in the C3
linearization of the class: the class, then the merge of the linearizations of its bases with the
list of its bases. In `class Both(Left, Right)` with `class Left(Base)` and `class Right(Base)`, the
order is `Both`, `Left`, `Right`, `Base`. So the `run` of `Right` hides the `run` of `Base`, although
the walk reaches `Base` first through `Left`. A hierarchy without a linearization, which Python
refuses at run time, and a cycle take the nearest arrival, the first at one depth over the rest.

#### Dataclasses

A class is a dataclass under the decorator `dataclass` of `dataclasses`, and under a
`dataclass_transform` of PEP 681 of `typing` or `typing_extensions`:

- A decorator whose function the transform marks makes its class a dataclass, with the arguments of
  the call of the decorator as its options.
- A metaclass or a base that the transform marks makes a subclass a dataclass, with the keywords of
  its class statement as its options, to the default depth of the kernel. The marked class itself is
  no dataclass.
- The arguments `eq_default`, `order_default`, `kw_only_default` and `frozen_default` of the
  transform are the defaults of the options of its name without `_default`. An option of the class
  overrides the default.
- A field whose value calls `field` of `dataclasses` or a field specifier of `field_specifiers`
  states the options of the field. A field specifier also takes `alias`, the name of the parameter,
  and `factory`, a default factory.

The generated `__init__` takes the fields of the base dataclasses first, and a field that the class
declares again keeps its place. A field at type level, a field of `field(init=False)` and the
sentinel `_: KW_ONLY` have no parameter. A field after `KW_ONLY`, of `field(kw_only=True)` or under
`kw_only=True` passes by keyword, and the keyword parameters follow the others. The sentinel loads as
a field, because the class body declares it.

#### Results

`ReturnRoles` reads the declared type of a result and not the stamp `python.generator`. The stamp
states how the body produces the values, and the annotation states what a caller may do with them.
So a generator annotated `Iterable[T]` returns an immutable list, and one annotated `Iterator[T]`
returns a stream.

### The backend

The backend is two kit compositions of one target. `New` writes modules, and `Stubs` writes stub
files with the body `...`:

```go
// New returns the Python backend that writes modules, files with the
// extension .py, for the version of Python that the policy python.version
// selects: the module's declared pieces composed through the kernel's kit
// into a [backend.Backend]. Its spoke is [spell.Type], which spells a type
// of another language under the target's lowering policies,
// [python.Policies]. Rendered files finalise through [Layout.Format], which
// lays them out as ruff's formatter does, because no formatter of Python is
// a library of Go and hermeticity refuses a formatter that the machine
// provides.
//
// Where the table of the standard library of typeshed, which the module
// embeds, does not parse, every module fails to start with the error of the
// table.
func New() backend.Backend

// Stubs returns the Python backend that writes stub files, files with the
// extension .pyi, as [New] composes the backend of modules, with
// [LowerStub] as its lowering. Every callable has the body ..., and every
// default and every value is ..., except the value of a constant and of a
// member of an enum, which is part of its type.
// A stub file writes no annotation as a string, because a type checker reads
// a stub without running it, and it has the blank lines that ruff's
// formatter writes in a stub file. A table of typeshed that does not parse
// fails every stub file as it fails every module of New.
func Stubs() backend.Backend
```

Both build through one composition. The version of `Stubs` ends in `; stub`, so a plan that changes
from one backend to the other changes the fingerprint of its composition:

```go
func compose(versionsText []byte, naming func(plugin.Unit) string, stub bool) backend.Backend {
    versions, err := typeshed.ParseVersions(versionsText)
    im := importer{versions: versions, err: err}
    version, lower := python.BackendVersion, Lower
    if stub {
        version, lower = version+stubVersion, LowerStub
    }
    return backend.New(python.Name, python.Target, python.Syntax()).
        Version(version).
        FileTemplate(FileTemplate).
        KindTemplates(KindTemplates()).
        RefusedKinds(RefusedKinds()).
        Coverage(Coverage()).
        Funcs(vocabulary(im, stub)).
        Naming(naming).
        Packages(spell.Package).
        Overloads().
        Respell(spell.Name).
        RespellMember(spell.Member).
        Types(spell.Type).
        Policies(python.Policies()...).
        Lower(lower).
        Scaffold(Scaffold).
        MemberIndent(python.Indent).
        Imports(im.Imports).
        TemplateImport(TemplateImport).
        Finalise(Layout{Stub: stub}.Format).
        Build()
}
```

`New` composes with `spell.Filename`, and `Stubs` with `spell.StubFilename`. The templates mark the
bracket groups, and `Layout.Format` lays out each file as ruff's formatter does:

- four spaces of indentation, and double quotes, or single quotes for a text that contains a double
  quote and no single quote
- two blank lines around a top-level definition and one around a method in a module, and the blank
  lines of ruff's rules for a stub file
- a signature, the bases of a class and the arguments of a decorator on one line up to 88 columns
- the items of a longer group on one indented line where they fit
- one item on each line with a trailing comma where they do not, and where a trailing comment
  follows an item

#### Markers

```go
// MarkerPrefix opens the name of every [Marker]. A Python name cannot
// contain a colon, so no decorator has the name of a marker.
const MarkerPrefix = "python:"

// Marker is the name of an annotation that states a construct of Python
// that the model has no field for. A generator states a marker as an
// annotation with the name of the marker, and the backend writes the
// construct for it and no decorator.
type Marker string

// The markers of the Python target.
const (
    // TypedDictMarker marks an interface as a TypedDict.
    TypedDictMarker Marker = MarkerPrefix + "typeddict"
    // BasesMarker states the bases of an enum, one argument for each base,
    // such as IntFlag.
    BasesMarker Marker = MarkerPrefix + "bases"
    // KeywordsMarker states the keyword arguments of a class statement, one
    // argument for each keyword, such as total=False.
    KeywordsMarker Marker = MarkerPrefix + "keywords"
    // AnnotatedMarker states the metadata of the Annotated type of a field,
    // a parameter, a variable, a constant or an alias, one argument for
    // each element.
    AnnotatedMarker Marker = MarkerPrefix + "annotated"
    // InitVarMarker marks a field of a dataclass whose type is InitVar[T],
    // which the generated __init__ takes and the instance does not keep.
    InitVarMarker Marker = MarkerPrefix + "initvar"
    // AllMarker is a header marker. Its arguments are the names of the
    // __all__ of the file.
    AllMarker Marker = MarkerPrefix + "all"
    // LazyMarker is a header marker. Its arguments are the modules that the
    // file imports with the keyword lazy.
    LazyMarker Marker = MarkerPrefix + "lazy"
    // FutureMarker is a header marker. Its arguments are the features of the
    // future statement of the file.
    FutureMarker Marker = MarkerPrefix + "future"
    // HeadMarker is a header marker. Its arguments are the comments of the
    // head of the file, each without its #.
    HeadMarker Marker = MarkerPrefix + "head"
    // ImportMarker is a header marker. Its arguments are the imports of the
    // file, one statement for each name that an import binds, such as
    // from dataclasses import field, import json and from shop import *.
    // The backend writes each in the import block of the file, so the
    // names that a value or a decorator of the file uses are bound.
    ImportMarker Marker = MarkerPrefix + "import"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is not a valid marker. Valid does not allocate.
func (m Marker) Valid() bool

// Header reports whether m is a marker of the header of a file, which the
// backend writes into the head of the module. Header does not allocate.
func (m Marker) Header() bool

// Marks reports whether m is a marker of a declaration of the kind k: the
// TypedDict marker of an interface, the bases marker of an enum, the
// keywords marker of a struct, an interface or an enum, the annotated
// marker of a field, a parameter, a variable, a constant or an alias, the
// InitVar marker of a field, and a header marker of a file. The backend
// refuses a marker on a declaration of another kind. Marks reports false
// for a marker that the satellite does not declare, and does not allocate.
func (m Marker) Marks(k symbol.Kind) bool
```

#### The header

The skeleton of a file is `{{module header decls}}`. The helper `module` writes the head of a module
from the header of its file, in five parts:

1. the comments of `python:head`
2. the documentation of the header, as the module docstring
3. the import block. The future statement of `python:future` is its first section, and the
   statements of `python:import` are in it
4. `__all__` of `python:all`. Without the marker, the writer lists the public names of the file where
   one of them starts with an underscore, which Python makes private without `__all__`
5. the type variables of the module, sorted by name

An import of a module that `python:lazy` lists takes the keyword `lazy`, in an `import` statement
and in a `from` statement. The render withholds a file whose header states `python:lazy` below the
version 3.15, and a file whose header states an annotation that is no header marker.

The import block leaves out each import whose bound name the parts after the block do not use. The
helper renders `__all__`, the type variables and the declarations first, and scans them for runs of
the characters of an identifier. An import whose bound name is no such run leaves the block, so a
header that restates every import of its source writes only the imports that the file uses. pyright
in strict mode refuses an unused import. A run inside a string counts, because a string annotation
uses the names that it contains. A future statement, a star import and an entry of the set that
marks a fact of the file never leave. Neither does a re-export, `import json as json` or
`from a import b as b`. The typing specification makes its name a public name of the module, and
pyright in strict mode accepts it unused. The block writes a re-export with its redundant alias, as
a statement of its own.

#### Kinds

| Emit kind | Python |
|---|---|
| Struct | `class` with its stated types as bases, `Generic[...]` for the type variables of the module, `ABC` where it is abstract and has no other base and no metaclass, and `@final` where it is final. A NamedTuple writes no `Final` |
| Interface | `class X(Bases, Protocol)`, with a read-only property for an immutable field of the instances, and a TypedDict with the marker `python:typeddict` |
| Enum | `class X(Enum)`, `IntEnum` for an underlying integer, `StrEnum` for underlying text, `(T, Enum)` for another underlying type, and the bases of `python:bases`. A member without a value is `auto()` |
| Alias | `type X = T` from the version 3.12, `X: TypeAlias = T` for 3.11 and for a generic alias whose parameters are type variables of the module, and `X = NewType("X", T)` for a defined alias |
| Sum | a frozen dataclass `<Sum><Variant>` for each variant with its fields and the type parameters of the sum, and the alias `<Sum>` of their union |
| Function | `def`, or `async def` where it is asynchronous |
| Method | `def` in its class, `__init__` or `__new__` where it constructs, `__getitem__` with `__setitem__` for an indexer, and the decorators of its level, its accessor and its facts |
| Field | `name: T = v` in its class, and the forms that follow the table |
| Variable | `name: T = v`, `Final[T]` where it is immutable, `name = v` without a type, and the dynamic type without a type and a value |
| Constant | `NAME: Final = v`, and `NAME: Final[T] = v` where it is typed. A stub file keeps the value |

A field takes these forms:

- An optional field is `T | None`, with the value `None` where it states none.
- A field at type level is `ClassVar[T]`. An immutable field is `Final[T]`, and an immutable field at
  type level is `ClassVar[Final[T]]` from the version 3.13 and `Final[T]` before it.
- A key of a TypedDict is `NotRequired[T]` where it is optional. Under `total=False` an optional key
  is `T` and a required key `Required[T]`. An immutable key is `ReadOnly[T]`.
- `InitVar[T]` and `Annotated[T, ...]` wrap the type for the markers `python:initvar` and
  `python:annotated`.
- A field of an enum is an attribute that is no member. A field of the members is `name: T` without a
  value, because an enum sets the attributes of its members in `__init__`. A field of the enum at type
  level with a value is `name = nonmember(cast("T", value))`. pyright refuses an annotation of such
  an assignment, so `cast` states the type. A stub file writes every field of an enum without a
  value.

The other rules of the kinds:

- A TypedDict with a key that is no identifier and an enum with a member that is no identifier take
  the functional form. So does a NamedTuple whose fields have no types, no documentation and no
  markers, where one of the names starts with an underscore. It writes `namedtuple` of
  `collections` with `rename=True`, the one form of such a name:

  ```python
  Row = namedtuple("Row", ["id", "_1"], rename=True, defaults=[0])
  ```

  Each underscore name is `_` and the index of its field, as `namedtuple` renames it.
- The backend writes a type parameter in the syntax of PEP 695 from the version 3.12. It writes the
  type variables of the module for 3.11, for 3.12 where a parameter has a default, and in every
  version where a parameter states a variance. The parameter list of that declaration then switches
  to type variables. The head lists them after `__all__`, with `Generic[T]` among the bases of a
  class and `Protocol[T]` for a Protocol. A type variable whose name the file binds already takes the
  lowest free number, as `T2`.
- The backend writes a default as `[T = D]` from the version 3.13. Below 3.13 it writes the default
  as `default=` in the `TypeVar` of `typing_extensions`. A name of `typing` that the target version
  lacks, such as `ReadOnly` below 3.13, imports from `typing_extensions`.
- PEP 695 infers the variance of a type parameter. So the backend writes a declared variance as a
  `TypeVar` with `covariant=True` or `contravariant=True` in every version.
- An annotation that refers to a declaration of the file that has not finished rendering is a string
  for the versions 3.11 to 3.13. Python 3.14 evaluates an annotation when a reader asks for it, so a
  forward reference there does not need quotes, and a stub file has none. The bound of a type
  variable of the module is a string where it refers to a declaration of the file. A base of a class
  that refers to such a declaration is refused, because Python evaluates a base when it runs the
  statement.
- The backend writes `/` after the last positional-only parameter. It writes `*` before the first
  keyword-only parameter that does not follow `*args`.
- A method at type level without a receiver is `@staticmethod`, and one with a receiver is
  `@classmethod`. `__new__` takes no decorator of its level, because Python makes it a static method.
  An abstract method of a Protocol takes no `@abstractmethod`.

#### The lowering

`Lower` refuses each construct of this table under `RefusedConstruct`, and the settle withholds the
declaration. It also synthesizes a Protocol for each intersection, and the dataclasses and the alias
of a sum. `LowerStub` refuses the same constructs.

| Construct | Reason |
|---|---|
| An annotation whose name is neither a dotted Python name nor a marker of the satellite, an annotation on a kind that Python does not decorate, an annotation that assigns its argument, and an argument of a decorator that does not parse as an argument of a call | a decorator is a dotted name or a call of one, on a class or a callable |
| A marker that the satellite does not declare, a marker on a declaration of another kind, and a marker that a declaration states twice | each marker has one meaning on one kind |
| A value that does not parse as the value of an assignment, and a default that does not parse as one expression without a star | the text is Python source |
| A keyword variadic parameter that is not last, a positional variadic parameter after a keyword-only parameter, a parameter after a keyword-only one that a caller passes otherwise, a positional-only parameter after a parameter that a caller passes otherwise, and a parameter without a default after a parameter with one | Python orders the parameters of a signature by how a caller passes them |
| A setter or a deleter before the getter of its name | an accessor belongs to a property |
| A name that a class declares twice, outside the overloads and the accessors of one name | a class declares a name once |
| A sum without a variant, a sum with methods, and a field of a variant without a name | a union joins at least one type and has no members |

The coverage rejects `Receives`, so the render leaves out a member with the fact under
`RejectedFact`. Python declares a member in its class.

#### Facts

| Verdict | Facts |
|---|---|
| Renders | Comment, Annotations, Visibility, Async, TypeParams, TypeParamDefault, Variance, MultiReturn, Level, Abstract, Final, Override, DefaultBody, Accessor, Constructs, Indexer, Optional, Mutability, HardPrivate, ParamDefault, Variadic, Passing, Value, Wire, Extends, Implements, Embeds, Properties, Defined, Types, Fields on an enum, Methods on a struct and an enum, Underlying |
| `Holds` | Throws, ConstEnum, Level on a struct, Sealed |
| Refuses | NamedReturn, Label, Computed, Tag, Permits, Methods on a sum, ConstParam, Optional on a method, Comment on a result, and Comment and Annotations on an embed |
| Rejects | Receives |

These verdicts need a reason:

- Visibility renders through the name: one leading underscore for a declaration of a module that is
  not public and for a protected member, and two for a private and a hard private member, which
  Python mangles.
- MultiReturn renders as a tuple of the results, which is how Python returns two values.
- Optional renders as `NotRequired[T]` in a TypedDict, as `T` alone under `total=False`, and as
  `T | None = None` on a field of a class and on a parameter. A value of the class then lacks the
  field by its value `None`.
- Wire renders as the name that the generator wrote, because the settle does not respell a key of a
  wire format.
- Embeds renders as bases, because the method resolution order of Python finds the members of a
  base as the promotion of Go finds the members of an embedded type. A base has no comment and no
  decorator, so Comment and Annotations refuse on an embed.
- An indexer renders as `__getitem__`, with `__setitem__` where it is mutable.
- Fields on an enum render as the attributes that are no members.
- Throws takes `Holds`, because Python raises without declaring. Sealed takes `Holds`. Python has
  no sealed classes, and a subclass outside the module does not raise an error.
- Level on a struct takes `Holds`, because a nested class is at the level of its type.
- NamedReturn refuses, because a tuple has no names of its members. Comment refuses on a result,
  because a result has no line of its own.
- Constructs renders as `__init__` and `__new__`. A Protocol has no constructor, so the writer
  refuses a constructing method of an interface.

#### The spoke

| Form | Python |
|---|---|
| Signed and unsigned integers of every width | `int` |
| Float of 64 bits or fewer, and of the platform's width | `float`, a double that holds each of their values |
| Bool, Text, Bytes | `bool`, `str`, `bytes` |
| Well-known timestamp and duration | `datetime` and `timedelta`, which the file imports from `datetime` |
| Well-known empty | `None` |
| Well-known error | `Exception` |
| Dynamic | the choice of `python.dynamic` |
| Optional of T | `T \| None` |
| List of T | `list[T]`, and `Sequence[T]` of `collections.abc` for an immutable list |
| Array of N of T | a tuple of N members of `T`, and `bytes` for an array of 8-bit unsigned integers |
| Map of K to V | `dict[K, V]`, and `Mapping[K, V]` of `collections.abc` for an immutable map |
| Tuple | `tuple[A, B]`, and `tuple[()]` without members |
| Union | `A \| B`, and `Never` of `typing` without members |
| Func | `Callable[[A, B], R]` of `collections.abc`, `None` without a result, and a tuple for two results or more |
| Stream of T | `Iterator[T]`, and `AsyncIterator[T]` for an asynchronous stream, of `collections.abc` |
| Borrow of T | `T` |
| Wildcard | its upper bound, and `object` for an unbounded wildcard |
| Intersection of interfaces | a Protocol that `Lower` synthesizes, which extends each member |
| Reference, Sum | the flat name of the referent, such as `SessionState`, with its type arguments in brackets |

`Lower` names the Protocol of an intersection after the declaration and the member that states the
type, such as `RowItems` for the field `items` of `Row`. The member of a constant, a variable and an
alias is `Type`, and a second Protocol of one member takes a number. An alias of an intersection
becomes the Protocol under the name of the alias.

The spoke refuses these shapes, because Python has no form for them or the canonical shape lacks
what the form needs:

- an intersection with a member that is not an interface, and an intersection without members
- an inline type, because the canonical shape has the types of its body and not the names of its
  members
- an opaque shape, a wildcard with a lower bound, and an array of an unstated length
- a float of more than 64 bits, whose values a double of Python does not hold

| Policy | Choices | Default |
|---|---|---|
| `python.version` | `3.11`, `3.12`, `3.13`, `3.14`, `3.15` | `3.11` |
| `python.dynamic` | `object`, `Any` | `object` |

The default version is the oldest version that the Python core developers support, so a written
file runs on every supported version. `object` is the top type of Python. `Any` turns the checks of a
type checker off for its values. A config file selects either choice for every plan or for one plan.
The param `dynamic` of the `python` directive selects the dynamic type for one declaration. The
render writes the syntax of one version in every file, so `python.version` is a policy of the plan,
and the directive has no param for it.

#### Names

| Declaration | Convention | Example |
|---|---|---|
| class, Protocol, TypedDict, enum, sum, variant of a sum, alias, type variable | CapWords | `OrderItem` |
| function, method, parameter, field, variable | snake_case | `order_items` |
| constant, enum member | UPPER_SNAKE_CASE | `MAX_ITEMS` |
| module, file | snake_case, with `.py` or `.pyi` | `order_items.py` |

These are the conventions of PEP 8, and the enum convention of Python's enum guide. The conventions
have these exceptions:

- A variable in CapWords or in UPPER_SNAKE_CASE keeps its spelling. A field at type level and an
  immutable field keep a spelling without a lower-case letter, through `spell.Member`.
- A type variable keeps its suffix `_co` or `_contra` and its marks `*` and `**`.
- A name that is a keyword of Python takes a trailing underscore, as PEP 8 recommends, so `class`
  becomes `class_`, and a module named for a keyword is `class_.py`. A soft keyword, such as `type`
  or `match`, keeps its spelling.
- A name that starts and ends with two underscores keeps its spelling.
- The visibility sets the leading underscores, and a hard private member is private with two.
- A field or an enum member whose name is no identifier keeps its spelling, which the functional
  form of a TypedDict or an enum writes.
- The conversion recognises no initialism, so `http_server` becomes `HttpServer`, and a name in its
  style, such as `HTTPServerError`, keeps its spelling.

The file of a unit joins the stem of its file key, the word of its family and its tag with
underscores, and converts them together.

#### Files and imports

`spell.Package` names the module of a routed file as the frontend names the module of a file that it
loads. The module is the first of these:

1. In a directory where the load placed a Python file, the package of the directory joined with the
   stem of the file. The package of the directory is the module of its `__init__.py`, or the parent
   of the module of another Python file in it.
2. Below the output directory of a plan that states an import base, the import base joined with the
   path of the file below that directory.
3. The workspace path of the file without its extension.

An `__init__.py` is the module of its directory, and a stub file the module of its stem. A segment
that is no identifier or that is a keyword fails the file, because no import can name the module.

Every routed file is a module of its own, so the layout qualifies a reference into another file. The
import block binds each referenced declaration:

- A declaration of another module is `from <module> import <name>`. A name that a declaration or an
  earlier import of the file takes already imports under the name with the lowest free number, as
  `Order as Order2`.
- A nested class imports through its outer class, as `Store.Key`. A reference through a module keeps
  its spelling and imports the module, as `json.JSONDecoder` imports `json`.
- A declaration of a module that the header imports with a star writes its name and imports
  nothing, unless a declaration of the file takes the name or the name starts with an underscore,
  which a star import does not bind.

The block groups the imports as isort's defaults do: the future statement, the standard library, the
third-party modules and the modules of the plan. `typing_extensions` and `_typeshed` are third-party
modules. A module of the plan is a module of the top-level package of the file or of a routed file
that the file refers into. In each group, the `import` statements come first and then the `from`
statements, each in the natural order without regard to case, with the names of a statement in the
order of isort by type. A `from` statement longer than the line wraps inside parentheses. The backend
reads typeshed's `VERSIONS` for the target version to tell a module of the standard library. It
writes an `__init__.py` only where a generator states one, because a namespace package imports
without one.

### Restating a Python source

```go
// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one, such
// as [eidos.StructMatch] and [eidos.GraphMatch].
type Match interface {
    eidos.Matcher
    Reader() *store.Reader
}

// Stated returns the markers that restate the stamps of the Python frontend
// on the declaration id, in this order: the TypedDict marker of an
// interface, the bases of an enum, the keywords of a class statement, the
// metadata of an Annotated type and the InitVar marker of a field. A
// generator over Python sources appends them to the annotations of the
// emit declaration that it writes for id. The Python backend then writes
// the declaration as its source states it. A declaration that the frontend
// did not stamp has no markers, and Stated returns nil for it.
//
// Each read records on the invocation of m.
func Stated(m eidos.Matcher, id symbol.Identity) symbol.Annotations

// Header returns the header of the file that declares d, and nil for a file
// without a docstring, an __all__, a future statement, a lazy import, a
// head comment and an import. d is a declaration or a file. The
// documentation of the header is the module docstring. Its annotations are
// the header markers of the other five, in this order: the names of
// __all__, the features of the future statements, the modules that the
// file imports lazily, the comments of the head, and the imports of the
// file. The import marker states one statement for each name that an
// import binds, in the order of the imports, with the absolute name of the
// module of a relative import. An __all__ without names states the all
// marker without arguments.
//
// A generator appends a header that is not nil to the accumulator of the file
// that it writes from the source, as in e.File().Append(h). The layout merges
// the headers that two handlers state alike.
//
// Each read records on the invocation of m.
func Header(m Match, d node.Declaration) *emit.File
```

The header restates every import of its source, and the import block of the backend leaves out each
import that the written file does not use.

### The toolchain adapter

```go
// New returns the Python toolchain adapter for the target version, a
// choice of [python.VersionPolicy], such as [python.Version312]. The empty
// choice is the default of the policy, 3.11. The adapter of a choice that
// the policy does not declare returns an error that wraps errVersion from
// Layout and from every check.
func New(version plugin.Choice) toolchain.Adapter
```

- `Available` reports whether `python3` and `pyright` are on the path, and names the first binary
  that it did not find.
- The layout writes `pyrightconfig.json` at the root of the scratch tree, with the target version
  and `"typeCheckingMode": "strict"`, unless the output has one there.
- The parse runs `python3 -I` over a script that parses each module and stub file with `ast.parse`
  and the `feature_version` of the target version. It returns one error that lists each syntax error
  with its file, its line and its column. `feature_version` is the best effort of `ast`: it refuses a
  type statement below 3.12, and it does not refuse a lazy import below 3.15. A project without a
  Python file and an interpreter older than the target version are errors.
- The type check runs `pyright --outputjson --pythonversion <version> -p <dir>`, and returns one
  error that lists each diagnostic of severity error with its rule. A pyright that does not run, that
  does not write a report, or that fails without a diagnostic is an error too.
- `RunTests` runs `python3 -I` over a script that discovers the files `test_*.py` through
  `unittest.defaultTestLoader.discover` and prints the counts as JSON last. A failure, an error and an
  unexpected success count as failed. A skip and an expected failure count as skipped.
- `Satisfies` type-checks the project, writes a probe module that returns a value of the type as the
  contract, and reports false where pyright refuses the probe under `reportReturnType` alone. A type
  is `module:Class`, `module.Class` or a builtin.

CI installs Python 3.15 through `actions/setup-python` v7.0.0 and pyright 1.1.414 through npm, as it
installs TypeScript for tsc. The jobs of the gate on Linux, macOS and Windows, the ring job and the
nightly jobs install them.

### The binary

```go
Frontends(gofrontend.New(nil), tsfrontend.New(), javafrontend.New(nil), rustfrontend.New(nil),
    protofrontend.New(), graphqlfrontend.New(), pythonfrontend.New(nil)).
Rules(gorules.New(), tsrules.New(), protorules.New(), graphqlrules.New(), pythonrules.New()).
Targets(golang.Target, typescript.Target, java.Target, rust.Target, protobuf.Target, graphql.Target,
    python.Target).

workspace.Plan{
    Name:       string(python.Target),
    Sources:    workspace.Sources{Lang: python.Lang, Packages: []string{treeSources}},
    Backend:    pythonbackend.New(),
    Extensible: true,
},
```

A run calls `frontend.Stores` of the Python frontend only when the tree has a Python file, as it does
for the stores of Go and Java. The load calls the store locator of each frontend that has an input
file, and the command passes the environment of its process.

### Tests

The corpus of the conformance module declares these verdicts on the feature inventory. It is one
project of uv_build whose module root is the root of the corpus:

| Feature | Verdict | Reason |
|---|---|---|
| struct_fields, struct_methods | projects | annotated attributes, and methods |
| method_overloads | projects | `@overload` |
| constants | projects | `Final` |
| cross_package_ref | projects | an import of another module |
| composite_refs | projects | `X \| None`, `dict[str, X]` and `Callable[[Target], Exception]` |
| builtin_ref | projects | the builtins |
| directive_carrier | projects | `#` carriers |
| test_classification | projects | `test_*.py` |
| interfaces | projects | a Protocol |
| enum_values | projects | an enum |
| directive_sugar | projects | a decorator of the brand |

The conformance module gains the fixture `shop`, a project of uv_build with the source root `src`.
Its modules state the constructs of the tables above:

| Module | Constructs |
|---|---|
| `shop/__init__.py` | a docstring, the re-exports of the package, `__all__`, and a constant with an attribute docstring |
| `shop/models.py` | a frozen dataclass, a NamedTuple, a dataclass with a nested `StrEnum`, and a dataclass with `kw_only=True`, `ClassVar`, `field(default_factory=...)`, `Final`, `InitVar` and `__post_init__` |
| `shop/catalog.py` | a `StrEnum` with a field of its members and a field of the enum, and an `IntFlag` with `auto()` |
| `shop/contracts.py` | a Protocol with a property, a TypedDict with `total=False`, `Required` and `ReadOnly`, a TypedDict with `NotRequired`, and an abstract base class |
| `shop/generics.py` | a future statement, a `TypeVar` with a bound, a variance and a default, a `NewType`, both forms of an alias, generic classes in both syntaxes, `**P` and `*Ts` |
| `shop/service.py` | a head comment, `import json`, a lazy import, `__slots__`, a property with a setter and a deleter, class and static methods, overloads, positional-only and keyword-only parameters, `**kwargs: Unpack[...]` and an async function |
| `shop/compat.py` | a check of `sys.version_info`, an async generator, and the fields that `__init__` assigns |
| `shop/inventory.py` | `__new__`, `@final`, `@override`, `Literal`, `Annotated`, `tuple[int, ...]`, a star import, a carrier and a decorator of the brand |
| `shop/forms.py` | type comments, an import under `TYPE_CHECKING`, a check of `sys.platform`, the functional forms of `NamedTuple`, `namedtuple`, `TypedDict` and an enum, `Optional`, `Union`, a string annotation and a relative import |
| `shop/legacy.py`, `shop/legacy.pyi` | a source and its stub file |
| `shop/vendor.pyi` | a stub-only module |
| `acme/__init__.py` | the function of the decorator of the brand |

The composition sets `python.version` to 3.15, the version of the syntax of the sources. Its plans
run in the order python-mirror, go-model and ts-client. The workspace suite also runs the composition
without its last plan, and Build refuses the choice of a version for a composition without a Python
plan, so the mirror cannot be the last plan. The edit of the suite makes a field of `Line` a float,
which changes the output of the first plan and not its export. The plans write these files:

- `python-mirror` restates each declaration through `Stated` and each file through `Header`. It
  writes the mirror of each module beside its source as `<stem>_mirror.py`, and the mirror of a stub
  file as a module, because a composition renders the target through one backend. Each mirror
  equals its source without its carrier lines, between the header and the provenance trailer that
  the output contract adds. These files have a mirror in another spelling:
  - `compat.py` loads the branch that the options select. Its async generator and the fields that
    `__init__` assigns are facts of bodies, which the model does not contain
  - `legacy.py` declares nothing, because a type checker reads its stub
  - `legacy.pyi` and `vendor.pyi` mirror as modules, with the blank lines of a module
  - `forms.py` states the forms that the backend writes in one spelling
- The mirror of each mirror equals the mirror, so the frontend loads a mirror back with the stamps of
  its source. The toolchain adapter for 3.15 parses, type-checks and tests the mirrors with the
  module `test_shop.py`.
- `go-model` writes the Go model of the classes in one file. Each dataclass and each NamedTuple
  becomes a struct of its fields of the instances, and each Protocol an interface. A TypedDict
  becomes a struct with a pointer field for each key that is not required. An enum becomes a defined
  type with a constant for each member. The Go toolchain type-checks the file.
- `ts-client` writes the TypeScript interfaces, enums and aliases of the classes, with each field as
  a wire field that keeps its Python name. tsc type-checks the file.
- The carrier `#+acme:skip plugin=clientgen` leaves `Audit` out of the client, and the decorator
  `@acme.skip(plugin="modelgen")` leaves `Recount` out of the model.

The matrix entry of Python fills the read side, the projection side, the render side, the table of
sugar and the hub. The hub has a row for each choice of `python.version` and `python.dynamic`.

### Failures

| Failure | Code | Effect |
|---|---|---|
| A syntax error | `UnparsedFile` | the file contributes every declaration outside the statements with an error |
| A string annotation that does not parse | `UnparsedFile` at Warning | the type keeps the string |
| A `pyproject.toml` that does not read or parse | `UnreadProject` | its files take the root of the run of their package directories |
| A carrier outside the kernel grammar | `BadCarrier` | the carrier attaches nothing |
| A carrier that the frontend cannot place | `UnaddressedCarrier` | the carrier does not apply |
| A decorator of the brand that is no valid directive | `BadMarker` | the decorator remains an annotation |
| A construct without a form in the model | `UnmodeledConstruct` | the declaration loads without it |
| A second declaration of a name | `DuplicateName` | the first declaration is kept |
| Two declarations of one identity, such as two callables of one discriminator | `DuplicateDeclaration` of the kernel | the first declaration is kept |
| A file of a store that does not read, and a `VERSIONS` that does not parse | an error, without a code | the load fails |
| An encoding declaration of another encoding than UTF-8 | `ForeignEncoding` | the file parses as UTF-8 |
| An import that no store places | `UnplacedNeed` | its references keep their spellings |
| A construct that the lowering refuses | `RefusedConstruct` | the settle withholds the declaration |
| A name that the convention of Python cannot spell | `RefusedName` | the settle withholds the declaration |
| A method at the level of a file | `RefusedKind` | the render skips the declaration |
| A stated fact without an idiom | `RefusedFact` | the declaration renders without the fact, with a warning |
| A member with `Receives` | `RejectedFact` | the render leaves the declaration out |
| A construct that a writer refuses | `RefusedTemplate` | the render skips the declaration, and the file where the head refuses |
| A value that the writer cannot spell | `UnspeltValue` | the render skips the declaration |
| Two declarations of a file that settle to one name, outside the overloads of one function | `CollidingNames` | the plan fails |
| A shape that the spoke refuses | `RefusedType` | the generator's emitter reports it at the source declaration, and the plan fails |

### Migration

`Param.Passing` is a schema edit and a regeneration of the model. The model fingerprint changes, so
every sealed state runs cold once after it. The fact `Passing` follows `Variadic` in the list of
facts. `AccessorDelete` adds a value to an enumeration, and the Python frontend is the one frontend
that sets it. The coverage suite checks that a backend takes a stance on every fact. A released
backend without a stance on `Passing` fails the suite against a kernel with the fact, so the kernel
and every backend release together. `ShapeRules.ShapeOf` takes the reference beside its
declaration, so the GraphQL rules, the other implementation, change their signature. A workspace
with Python files loads them in the stock binary. The plan `python` does not write a file without a
plugin.

## Alternatives

### The upstream grammar

`tree-sitter/tree-sitter-python` has not merged a change since 2025-09-15, and it does not parse a
lazy import. A frontend on it would refuse every module of Python 3.15 that imports lazily.

### A pure-Go parser

gpython parses the grammar of Python 3.4. No other Go library parses Python 3.

### CPython's `ast` through an interpreter

The frontend would run the `python3` of the machine and read its syntax tree. The interpreter of each
machine parses its own version of the language, so two machines would load different graphs from one
workspace. A frontend does not run a tool for that reason.

### A type checker as the frontend

pyright runs on Node and mypy on an interpreter. A frontend on either would infer the types that a
module does not annotate. Two machines with two versions of the checker would then load two graphs
from one workspace, which the rule against toolchain frontends refuses.

### The standard library of the interpreter

The load would read the standard library from the `lib` directory of the interpreter of a
`pyvenv.cfg`. Little of that source has annotations, and the modules in C have no source at all.
`math` and `_datetime` are two of them. typeshed annotates every module of the standard library.

### typeshed from the machine

The composition would locate the typeshed that a type checker installs, such as the copy of mypy or of
pyright. Its version follows the installed checker, as the standard library of Go follows GOROOT.
The embedded copy gives every machine one standard library, and a refresh is a commit.

### A unit per package directory

Python scopes a name to its module, so two modules of one directory both declare `User` under two
identities. A unit per directory would make them one package.

### Visibility at run time

Python does not enforce a visibility, and every name could load as public. The typing specification
makes an underscore name and a name outside `__all__` private to its library. A generator that writes
a public API needs that distinction.

### Kinds from resolved bases

The frontend would set the kind of a class after Link resolves its bases. An enum base of another
module would then make an enum. The kernel fixes the kind of a declaration at the parse. Link does
not change a kind.

### The label for keyword-only parameters

`Param.Label` is the name that a caller writes for a parameter with two names, the outer name of a
Swift parameter. Whether a caller may pass the argument by position is a separate fact.

### A stamp for the passing of a parameter

A stamp `python.passing` on each parameter would leave the model unchanged. A generator into another
language reads the model and not the keys of Python. The way a caller passes an argument also changes
the calls that the callee accepts, as its default does.

### The deleter folded into the getter

A stamp on the getter of a property could record its deleter. The deleter would then lose its
docstring and its decorators, and a generator could not write its body. The delete accessor costs one
value of an enumeration and one rule of the discriminator instead.

### A formatter

No formatter of Python is a Go library, and a backend does not run a tool such as ruff or black. The
templates mark the bracket groups, and `Layout.Format` lays out each file as ruff's formatter does.

### The depth order of the member walk

The walk of the kernel could take the nearest arrival of a name, as it does for the JVM languages. In
`class Both(Left, Right)` with `class Left(Grand)`, where `Grand` and `Right` both declare `run`, the
nearest arrival is the `run` of `Right`, one level up. Python takes the `run` of `Grand`, because
the C3 linearization `Both`, `Left`, `Grand`, `Right` puts `Grand` before `Right`. A generator that
delegates to an inherited method would then call another method than Python does. The
linearization costs the walk a record of the bases of each type that it visits.

### Unused imports left to the reader

The written file could keep every import that its header restates. pyright in strict mode refuses
an unused import. A mirror whose generator drops a declaration would then fail its type check. The
frontend loads a module before a generator chooses its declarations, so it cannot drop the imports
either. The backend renders the declarations before the import block and decides.

### The functional form of a named tuple

The backend could write every NamedTuple through `namedtuple` of `collections`. Its fields have no
types, so a type checker reads each of them as `Any`. The class syntax keeps the types, and the
backend takes the functional form only where a field name starts with an underscore, which the class
syntax refuses.

### A future statement in every module

Every module for a version below 3.14 could start with `from __future__ import annotations`. The
statement turns every annotation into a string at run time. pydantic and the other readers of
`__annotations__` then have to evaluate each string. A string annotation for a forward reference alone
keeps every other annotation a value.

## Drawbacks

- The model grows by one type, `symbol.Passing`, with three values, one field of `Param` with its
  fact, and one value of `symbol.Accessor`. The discriminator gains five marks: `name=`, `/`, the
  type of a receiver with `;`, `_` and `del`.
- The kernel grows by two roles, `Overloader` and `MemberRespeller`, one field of `PolicySpec` and of
  `RenderContext`, one method of `Policy`, six methods of `ImportSet`, one field of `ParamView`, one
  rule of `Shadowing`, and two arguments of `ShapeRules.ShapeOf`. The Go, TypeScript, Java, Rust,
  protobuf and GraphQL backends each take a stance on the new fact and refuse the new accessor
  through `lowering.DeleteAccessor`.
- A new fact fails the coverage suite of every released backend, so the kernel release with
  `Passing` is a breaking release of the backends.
- The repository takes on typeshed's `stdlib`: 753 files and 2.9 MB under Apache-2.0 and MIT. A
  refresh of the embedded tree is a commit of the new tree.
- The fork's `go.mod` declares its own path, so every change taken from upstream keeps that line.
- Every binary that links the satellite compiles 3.5 MB of parser C under cgo.
- The parse reads one module. An enum or a TypedDict whose base another module declares loads as a
  struct. A type variable that another module declares does not give a type parameter to the
  functions that use it. `Id = models.UserId` loads as a variable, because the declaration of the
  imported name is in another module.
- `Annotated` inside another type keeps its metadata in a stamp alone, and the backend writes it only
  where it wraps the whole annotation of a declaration.
- `set`, `frozenset` and the set classes of `typing` and `collections.abc` fold to Opaque. Every other
  spoke refuses an opaque shape, so a field of a set writes into no other language, and the rules
  derive no sample and no zero for it. `int` folds to the platform's width.
- An immutable list or map, such as `Sequence[T]`, `Mapping[K, V]` and `tuple[T, ...]`, writes into
  Python, TypeScript and GraphQL alone. The Go, Java, Rust and protobuf spokes refuse the
  immutability.
- One load evaluates the checks of one version and one platform.
- The parse reads the bodies of methods for the assignments to the receiver, and the bodies of
  callables for `yield`.
- `New` and `Stubs` are two backends of the target `python`. Build refuses two backends of one target
  among the enabled plans, so one composition writes modules or stub files, and a project that needs
  both runs two compositions.
- The backend of modules writes the declarations of a file in the order that a module evaluates
  them: a base before its class, and the overload signatures of a function right before its
  implementation. A cycle of bases keeps the order of the flush of the kernel, and the backend
  refuses the base that comes too early. The backend of stub files keeps the order of the flush.
- The mirror of a module drops what only a body states: the check of a version that the options
  decided, the body of an async generator and the assignments of `__init__`.
- The tests of the toolchain adapter need `python3` and pyright on the machine.

## Open questions

None.

## Unresolved and future work

- The types that a type checker infers for the declarations that state none.
- A kernel form for sets. A set is a construct of Python, and Java, Rust and TypeScript declare set
  types. The form changes the closed set of forms of the kernel and needs a spelling in every spoke,
  so its design is an RFC of its own. This RFC folds a set to Opaque.
- The third-party stubs of typeshed beside the embedded standard library. A virtual environment with
  an installed stub-only package provides them through the `venv` store.
- Cython's `.pyx` files and the cells of notebooks, which are files of other formats.
- An order of the declarations of a file in which each follows the declarations that its definition
  evaluates, which the render of the kernel decides for Python and TypeScript alike.

## References

| What | Where |
|---|---|
| The fork of the grammar, head `1d213667` | <https://github.com/dokimasia/tree-sitter-python> |
| The upstream grammar | <https://github.com/tree-sitter/tree-sitter-python> |
| gpython | <https://github.com/go-python/gpython> |
| typeshed at `e75a39fe`, its `stdlib` and its licence | <https://github.com/python/typeshed/tree/e75a39fec6f375f5002d54c5607b44bedc025a4b> |
| The Python typing specification | <https://typing.python.org/en/latest/spec/index.html> |
| Distributing type information: the module resolution order and the library interface | <https://typing.python.org/en/latest/spec/distributing.html> |
| Enumerations | <https://typing.python.org/en/latest/spec/enums.html> |
| Type checker directives: TYPE_CHECKING, version and platform checks | <https://typing.python.org/en/latest/spec/directives.html> |
| What's New in Python 3.15 | <https://docs.python.org/3.15/whatsnew/3.15.html> |
| The status of Python versions | <https://devguide.python.org/versions/> |
| PEP 8, naming conventions | <https://peps.python.org/pep-0008/> |
| PEP 257, docstring conventions | <https://peps.python.org/pep-0257/> |
| PEP 420, namespace packages | <https://peps.python.org/pep-0420/> |
| PEP 561, distributing type information | <https://peps.python.org/pep-0561/> |
| PEP 570, positional-only parameters | <https://peps.python.org/pep-0570/> |
| PEP 3102, keyword-only arguments | <https://peps.python.org/pep-3102/> |
| PEP 695, type parameter syntax | <https://peps.python.org/pep-0695/> |
| PEP 696, type parameter defaults | <https://peps.python.org/pep-0696/> |
| PEP 649, the deferred evaluation of annotations | <https://peps.python.org/pep-0649/> |
| PEP 810, explicit lazy imports | <https://peps.python.org/pep-0810/> |
| PEP 798, unpacking in comprehensions | <https://peps.python.org/pep-0798/> |
| PEP 681, data class transforms | <https://peps.python.org/pep-0681/> |
| The unary plus in the literal patterns of CPython 3.15, gh-145239 | <https://github.com/python/cpython/pull/148566> |
| The typing specification of dataclasses and `dataclass_transform` | <https://typing.python.org/en/latest/spec/dataclasses.html> |
| Barrett et al., A Monotonic Superclass Linearization for Dylan, OOPSLA 1996, the C3 linearization | <https://doi.org/10.1145/236337.236343> |
| The Python 2.3 method resolution order | <https://docs.python.org/3/howto/mro.html> |
| `collections.namedtuple` | <https://docs.python.org/3/library/collections.html#collections.namedtuple> |
| `enum.nonmember` | <https://docs.python.org/3/library/enum.html#enum.nonmember> |
| `ast.parse` and its `feature_version` | <https://docs.python.org/3/library/ast.html#ast.parse> |
| unittest, test discovery | <https://docs.python.org/3/library/unittest.html#unittest-test-discovery> |
| pyright, the configuration and its strict mode | <https://github.com/microsoft/pyright/blob/main/docs/configuration.md> |
| isort, the options | <https://pycqa.github.io/isort/docs/configuration/options.html> |
| Sphinx autodoc | <https://www.sphinx-doc.org/en/master/usage/extensions/autodoc.html> |
| griffe | <https://mkdocstrings.github.io/griffe/> |
| mypy, mapping file paths to modules | <https://mypy.readthedocs.io/en/stable/running_mypy.html#mapping-file-paths-to-modules> |
| go-toml/v2 v2.4.3 | <https://github.com/pelletier/go-toml> |
| go-tree-sitter v0.25.0 | <https://github.com/tree-sitter/go-tree-sitter> |
| The Go modules reference, the replace directive | <https://go.dev/ref/mod#go-mod-file-replace> |
| setuptools, package discovery | <https://setuptools.pypa.io/en/latest/userguide/package_discovery.html> |
| hatch, the build configuration | <https://hatch.pypa.io/latest/config/build/> |
| Poetry, the `packages` setting | <https://python-poetry.org/docs/pyproject/> |
| uv, the build backend | <https://docs.astral.sh/uv/concepts/build-backend/> |
| pdm-backend, the build configuration | <https://backend.pdm-project.org/build_config/> |
| maturin, the project layout | <https://www.maturin.rs/project_layout.html> |
| pytest, the default `python_files` | <https://docs.pytest.org/en/stable/reference/reference.html#confval-python_files> |
| pyright, the command line | <https://github.com/microsoft/pyright/blob/main/docs/command-line.md> |
| ruff, the formatter | <https://docs.astral.sh/ruff/formatter/> |
| The order of the declarations of a file, eidos issue 43 | <https://github.com/dokimasia/eidos/issues/43> |
