---
rfc: 0073
title: The Python satellite
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
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
The rules project the builtins, the
generic aliases of `typing` and `collections.abc`, the types of the standard library that the
registry blesses, `Literal`, `Annotated` and `NewType`.

The kernel gains the way a caller passes the argument of a parameter, for positional-only and
keyword-only parameters, and the delete accessor of a property. The stock binary gains the frontend,
the rules, the target, the stores and the plan `python`. The support matrix gains a Python column.

## Motivation

dokimi, the reference consumer, reads Python beside Go, TypeScript, Rust, Java and Kotlin. No
satellite of eidos reads or writes Python. That blocks these generators:

- A generator cannot read the dataclasses, the pydantic models and the Protocols of a Python service
  to write the Go server types or the TypeScript client types of its API.
- A generator cannot write Python dataclasses, Protocols, enums or a typed client from the
  declarations of Go, TypeScript, Java, Rust, protobuf or GraphQL.
- The completeness check and the support matrix have no Python column.

Two facts of a Python signature have no field in the model. A keyword-only parameter does not accept
a positional argument. The `timeout` of `def get(url, *, timeout: float)` is such a parameter. A
positional-only parameter does not accept a keyword argument. A generator that writes a delegating
call has to know which form each parameter accepts. A property can also have a deleter beside its
getter and its setter, and the model has no form for a deleter.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Model | `eidos-core/symbol`, `eidos-core/symbol/schema` | `symbol.Passing`, `Param.Passing` with the fact `Passing`, and `symbol.AccessorDelete` |
| Identities | `eidos-core/frontend/load` | the discriminator `del` of a delete accessor |
| Grammar | `eidos-lang/treesitter/python` | new: the pinned grammar of the fork |
| Satellite | `eidos-lang-python` | new module `go.dokimi.dev/eidos/lang/python`: the language, the comment syntax, the keys, the markers, the policies, `Stated` and `Header` |
| Projects | `eidos-lang-python/internal/project` | new: the read of `pyproject.toml`, the source roots of the build backends and the module path of a file |
| Frontend | `eidos-lang-python/frontend` | new: the partition into modules, the lowering, the resolution, the re-exports, the dependency rounds and the stores |
| Standard library | `eidos-lang-python/frontend/typeshed` | new: the stubs of typeshed's `stdlib` at a pinned commit, embedded |
| Rules | `eidos-lang-python/rules` | new: the builtins, the members, the roles, the resolution, the values, the enums and eight optional capabilities |
| Spoke and names | `eidos-lang-python/spell` | new: `Type`, `Name`, `Filename` and `Package` |
| Backend | `eidos-lang-python/backend` | new: the kit compositions of modules and of stub files, the templates, the coverage, the lowering, the scaffold and the import block |
| Toolchain adapter | `eidos-lang-python/testing` | new: the parse through CPython, the type check through pyright and the tests through `unittest` |
| Backends | the six backend modules | a stance on the fact `Passing`, and a refusal of the delete accessor |
| Workspace | `go.work` | the module |
| Binary | `cmd/eidos` | the frontend, the rules, the target, the stores and the plan `python` |
| Conformance | `eidos-conformance` | the corpus, the matrix entry and the fixture `shop` |

### Invariants

1. Every declaration construct of Python 3 and every type form of the typing specification has a form
   in the model, and the backend writes it. The tables of constructs list each form.
2. A module that CPython 3.15 compiles loads without an Error, and so does a stub file that pyright
   accepts.
3. The frontend loads each file that the backend writes into the declarations, the shapes, the
   annotations and the stamps that the emit model stated.
4. The toolchain adapter compiles, type-checks and runs the tests of each tree that the conformance
   fixtures write.
5. A construct, a fact, a kind or a name that Python does not spell reports a positioned finding.

### The constructs

Classes and their members:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `class` | a struct, with its bases in `Extends` in the order of the statement | a struct |
| `class` with the base `Protocol` | an interface, with its other bases in `Extends` | an interface |
| `class` with the base `TypedDict` | an interface with the stamp `python.typedDict` | an interface with the marker `python:typeddict` |
| `class` with the base `NamedTuple` | a struct with `NamedTuple` in `Extends` and immutable fields | a struct with the base |
| `class` with the base `Enum`, `IntEnum`, `StrEnum`, `Flag`, `IntFlag` or `ReprEnum` | an enum with a variant for each member, the data type in `Underlying`, and the bases in the stamp `python.bases` | an enum, and the marker `python:bases` for bases that `Underlying` does not decide |
| `Generic[T]` or `Protocol[T]` among the bases | the type parameters, without the base | type parameters |
| `class C[T: B = D]` | the type parameters with their bounds, constraints and defaults | type parameters |
| The keyword arguments of a class statement, such as `metaclass=`, `total=`, `closed=` and `extra_items=` | the stamp `python.keywords`, each keyword verbatim | the marker `python:keywords` |
| A decorator | an annotation with the dotted name, and the arguments verbatim | an annotation |
| A nested class | a nested declaration | a nested declaration |
| An annotated attribute of a class body, `x: T = v` | a field, with its default as its value | a field |
| `x: ClassVar[T] = v` | a field at type level | a field at type level |
| `x: Final[T] = v` | an immutable field | an immutable field |
| An attribute without an annotation, `x = v` | a field at type level without a type | a field at type level |
| `self.x: T = v` in a method, and `self.x = v` in `__init__` | a field, after the fields of the class body | a field |
| `__slots__` | a field at type level with its value | a field |
| `x: InitVar[T]` of a dataclass | a field with the stamp `python.initVar` | a field with the annotation |

Callables:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `def` and `async def` in a module | a function | a function |
| `def` in a class | a method, with `self` or `cls` as its receiver | a method |
| `@staticmethod`, `@classmethod` | a method at type level, and the annotation | a method at type level |
| `__init__`, `__new__` | a method that constructs | a method that constructs |
| `@property`, `@x.setter`, `@x.deleter` | methods with the accessors get, set and delete | methods with an accessor |
| `@overload` | a callable for each overload with the annotation `overload`, and the implementation with the stamp `python.implementation` | callables with the annotation, in order |
| `@abstractmethod`, `@final`, `@override` | an abstract, a final and an overriding method, and the annotation | the field |
| A function whose own body yields | the stamp `python.generator` | the stamp |
| Parameter before `/` | `Passing` positional | `Passing` positional |
| Parameter after `*` or `*args` | `Passing` keyword | `Passing` keyword |
| `*args`, `**kwargs` | a variadic parameter, positional or by keyword | a variadic parameter |
| A default | the default verbatim | a default |
| A return annotation | one result, no result for `-> None`, and one result without a type for a function without an annotation | results |
| A type comment, `# type: (A) -> R` | the types of the signature | annotations |

Modules:

| Construct | The frontend loads | A generator states |
|---|---|---|
| A module | a package with one file, and its docstring as the documentation of the file | a routed file |
| An annotated assignment | a variable | a variable |
| `X: Final = v`, `X: Final[T] = v` | a constant | a constant |
| An assignment without an annotation | a variable without a type | a variable |
| A type comment, `x = v  # type: T` | the type of the variable | an annotation |
| `type X[T] = U`, `X: TypeAlias = U` | an alias | an alias, in the form of the target version |
| `X = NewType("X", T)` | a defined alias of `T` | a defined alias |
| `T = TypeVar("T")`, `ParamSpec`, `TypeVarTuple` | a variable with the stamp `python.typeVar`, and a type parameter of each declaration that names it | type parameters |
| The functional forms of `NamedTuple`, `TypedDict` and `Enum` | the declaration of the class syntax | the declaration |
| `__all__` | the exports of the file | the marker `python:all` of the header |
| `import`, `from ... import`, a relative import, `lazy import` | the imports of the file, and the stamp `python.lazy` with the modules that it imports lazily | the imports that the spellings need, and the marker `python:lazy` of the header |
| `from __future__ import` | the stamp `python.future` of the file | the marker `python:future` of the header |
| `if TYPE_CHECKING:` and the checks of `sys.version_info` and `sys.platform` | the declarations of the branches that the options select | the declarations |
| The head comments: a shebang, the encoding declaration, `# type: ignore` and the directives of mypy, pyright and ruff | the stamp `python.head` of the file, each comment verbatim | the marker `python:head` of the header |
| A docstring of a module, a class, a callable or an attribute | the documentation | `Doc` |
| A `#` comment | a carrier, or the trailing comment of a declaration | `Comment` |

The types:

| Annotation | Model |
|---|---|
| `A`, `a.b.A`, `A[B, C]` | Named, with the type arguments in Args and the module of the import in Package |
| `int`, `str` and the other builtins | Named without a target, which the rules project |
| `X \| None`, `Optional[X]`, `Union[X, None]` | Optional, with `None` in Absent |
| `X \| Y`, `Union[X, Y]` | Union |
| `Callable[[A, B], R]` | Func, the parameters and then the result |
| `Callable[..., R]`, `Callable[P, R]`, `Callable[Concatenate[A, P], R]` | Named with Args, because the parameter list is not exact |
| `tuple[A, B]`, `Tuple[A, B]`, `tuple[()]` | Tuple |
| `tuple[A, ...]` | List, immutable |
| `list[A]`, `dict[K, V]` and every other generic class | Named with Args, which the rules project |
| `Literal[...]`, `LiteralString`, `type[C]`, `Self`, `Never`, `NoReturn`, `TypeGuard[T]`, `TypeIs[T]`, `TypeForm[T]`, `Unpack[T]`, `*Ts`, `**P` | Named, verbatim |
| `Annotated[T, m]` | the reference of `T`, and the stamp `python.annotated` with the metadata |
| `Required[T]`, `NotRequired[T]`, `ReadOnly[T]`, `ClassVar[T]`, `Final[T]`, `InitVar[T]` | the reference of `T`, and the field or parameter that the qualifier sets |
| A string annotation, `"A"` | the reference that the string contains, at the position of its content |

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
// Package python pins the Python grammar of the fork
// dokimasia/tree-sitter-python.
package python

// Python parses .py and .pyi files.
var Python *treesitter.Grammar
```

The grammar is ABI 15, which go-tree-sitter v0.25.0 loads. Its `parser.c` is 3.5 MB of C and its
external scanner 15 KB.

### Projects, units and packages

- The selection is `**/*.py` and `**/*.pyi`, negating `**/.venv/**`, `**/venv/**`,
  `**/site-packages/**`, `**/__pycache__/**`, `**/.tox/**`, `**/.nox/**` and `**/node_modules/**`.
- A unit is one module, because Python scopes a name to its module. A module is a `.py` file, a
  `.pyi` file, or both files of one name in one directory. When both exist, the unit lowers the stub,
  and the source file is a member without declarations with the stamp `python.stubbed`. A type
  checker reads the stub of a module and not its source.
- The package of a module is its dotted name with slashes, as in `shop/models` for `shop.models`. A
  package directory's `__init__.py` is the module of the package itself, `shop`.
- The partition finds the governing `pyproject.toml` by probing upward from each file, and the file
  is the ref's `Shared` input. `github.com/BurntSushi/toml` v1.6.0 reads it.
- The source root of a project is the directory in the table of its build backend:

  | Build backend | Key |
  |---|---|
  | setuptools | the key `""` of `[tool.setuptools] package-dir`, and each `where` of `[tool.setuptools.packages.find]` |
  | hatchling | the `sources` of `[tool.hatch.build.targets.wheel]`, and the parent directory of each of its `packages` |
  | poetry-core | the `from` of each entry of `[tool.poetry] packages` |
  | uv_build | `module-root` of `[tool.uv.build-backend]` |
  | pdm-backend | `package-dir` of `[tool.pdm.build]` |
  | maturin | `python-source` of `[tool.maturin]` |

  A project without a root in the table of its backend has the root `src` when the directory exists,
  and its own directory otherwise. setuptools' automatic discovery, uv_build, pdm-backend and
  poetry-core default to these roots.
- A file outside every source root of its project, such as a test under `tests/`, and a file outside
  every project have the root above the highest directory of the run of `__init__.py` files above
  them. pytest's default import mode and mypy find the name of such a module in the same way.
- `python.testFile` stamps a file whose name matches `test_*.py` or `*_test.py`, which is pytest's
  default `python_files`, and every `conftest.py`.
- The options fold into every unit key:

  ```go
  // Options is the Python frontend's configuration.
  type Options struct {
      // Version is the Python version that the checks of sys.version_info
      // compare with, and the version that typeshed's VERSIONS selects
      // the standard library for, as 3.15. The empty version is 3.15.
      Version string

      // Platform is the value of sys.platform that the checks compare
      // with, such as linux, darwin or win32. The empty platform is linux.
      Platform string
  }
  ```

### The frontend

```go
return frontend.New(python.Name, python.Lang, python.Syntax()).
    Version(python.FrontendVersion + "; " + pygrammar.Python.Version()).
    Overloads().
    Options(o).
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

The frontend sets the kind of a class from its direct bases. It reads each base through the import
bindings of the file and the classes of the module:

- A class with the base `Protocol` of `typing` or `typing_extensions` is an interface. `Extends`
  leaves the `Protocol` base out. `Protocol[T]` gives the type parameters.
- A class with the base `TypedDict` is an interface with the stamp `python.typedDict`. Its field is
  `Optional` under `NotRequired`, and under the class keyword `total=False` without `Required`. A
  `ReadOnly` field is immutable.
- A class with an enum base is an enum. An enum base is an enum class of the module `enum`, an enum
  of the same module, or the metaclass `EnumMeta` or `EnumType`. The members of the enum are the
  attributes that the typing specification counts as members. Every other attribute is a field at
  type level. The data type is `int` for `IntEnum` and `IntFlag` and `str` for `StrEnum`. For
  `class E(T, Enum)` it is the mixin type `T`.
- Every other class is a struct. A struct is `Abstract` where it declares an abstract method. A type
  checker refuses to instantiate such a class. A struct under `@final` is `Final`.

Because the parse reads one module, a class whose enum base another module declares loads as a
struct with the base in `Extends`.

#### Callables and parameters

- The receiver of a method is its first parameter, except for a static method. A class method's
  receiver is `cls`.
- A parameter has its annotation or the type of its type comment, and no type without either. Its
  default is verbatim.
- `Passing` is positional for a parameter before `/`. It is also positional for a leading parameter
  whose name starts with two underscores and does not end with two. The typing specification keeps
  that convention as the historical form of a positional-only parameter. `Passing` is keyword for a
  parameter after `*` or after `*args`.
- An `async def` is `Async`. An `async def` whose own body yields is an asynchronous generator. A
  call of it returns its iterator at once, so it is not `Async`. Its result folds to an asynchronous
  stream.
- The overloads of a name load as one callable each, with the annotation `overload`. The
  implementation loads after them with the stamp `python.implementation`. The load spells the
  discriminator of each from its parameters, so the overloads and the implementation have distinct
  identities.
- A getter, a setter and a deleter of one property are three methods. The setter's discriminator is
  its parameter. The getter and the deleter take no parameter, so the load spells the deleter's
  discriminator `del`.
- A method of a Protocol is abstract where its body has no statement beside a docstring, `...` and
  `pass`. Every other method of a Protocol has `DefaultBody`, which a class that names the Protocol
  among its bases inherits.

#### Attributes and variables

- An annotated attribute of a class body is a field. `ClassVar` sets the type level, and `Final`
  makes the field immutable. Python shares an attribute without an annotation between the
  instances, so it is a field at type level without a type.
- The parse reads the annotated assignments to `self.<name>` in the body of each method, and every
  assignment to `self.<name>` in the body of `__init__`. Each name that the class body does not
  declare is a field.
- The parse reads three facts of a body: these assignments, whether a function yields, and whether a
  method of a Protocol has a statement beside a docstring, `...` and `pass`. It does not read any
  other part of a body. At signature depth it does not read a body at all.
- A module-level annotated assignment is a variable, and one with `Final` a constant. An assignment
  without an annotation is a variable without a type. Each name of `a = b = v` is a variable with the
  value `v`, and each name of `a, b = v` a variable without a value.
- `__all__` lowers to the exports of the file and to no variable.

#### Types

A string annotation parses through the grammar as the type that it contains. Its positions are the
positions of its content in the file. A qualifier that wraps the whole annotation of a field or a
parameter sets the field of the model and leaves the reference of its argument:

| Qualifier | Field |
|---|---|
| `ClassVar` | `Level` type |
| `Final`, `ReadOnly` | `Mutability` immutable |
| `Required`, `NotRequired` | `Optional` |
| `InitVar` | the stamp `python.initVar` |
| `Annotated[T, m...]` | the stamp `python.annotated` with each element of the metadata verbatim |

`Annotated` inside another type keeps its argument's reference, and the stamp `python.annotated` of
the declaration contains the whole annotation verbatim.

#### Resolution and re-exports

For each file, the parse records its package, the enclosing classes of each declaration, the import
bindings with their absolute module paths, and the star imports. A relative import resolves against
the package of the file. `Resolve` returns tiers in Python's order of scopes for an annotation:

1. The enclosing class bodies come first, innermost first, one tier each. Python evaluates the
   annotations of a class body and of its method signatures in the namespace of the class.
2. The declarations of the module and its import bindings follow in one tier, because a module has
   one namespace.
3. Each star import is a tier, in statement order.
4. The module `builtins` is the last tier.

A dotted name `a.b.C` resolves to `C` in the module `a/b` when an import binds `a` to a module. It
resolves to the member `b.C` of the class `a` when `a` is a class. A name that `from a import c`
binds resolves to `c` in the module `a`, and then to the module `a/c`.

The frontend implements `Exporter`. For a module that does not declare `name`, `Exports(scope, name)`
returns the candidates of the module's import binding of `name`. The candidates for `name` in its
star imports follow, in statement order. Every import of a module binds a name of the module at run
time, so `from shop.models import Order` resolves through `shop/__init__.py` when that file imports
`Order`. The visibility of a name follows the typing specification's rules of a library's interface.
Those rules do not change whether a reference to the name resolves.

#### Visibility

| Name | Visibility |
|---|---|
| A name without a leading underscore, and a name that starts and ends with two underscores | public |
| A module-level name that `__all__` does not list, where the module states `__all__` | package |
| A module-level name with one leading underscore | package |
| A member with one leading underscore | protected |
| A member whose name starts with two underscores and does not end with two | private |

Signature depth leaves out every declaration that is not public.

#### Conditions

- `TYPE_CHECKING` of `typing` and `typing_extensions` is true, because the frontend reads a module as
  a type checker does.
- A comparison of `sys.version_info` with a tuple, and of `sys.version_info[0]` with an integer,
  compares with the option `Version`.
- `sys.platform == "<name>"`, its negation and `sys.platform.startswith("<prefix>")` compare with the
  option `Platform`.
- `not`, `and` and `or` combine such checks.
- The parse does not load a branch that a check excludes, and it stamps the file with the check
  under `python.condition`. The branches of any other condition load in order, and so does every
  clause of a `try` statement.
- The first declaration of a name in a module is its declaration, and each later one reports
  `DuplicateName` at Info. The overloads and the accessors of one name are one name.

#### Comments and docstrings

- The parse reads every file as UTF-8, the default encoding of Python 3 source. A file whose
  encoding declaration names another encoding reports `ForeignEncoding`, and its bytes keep their
  values.
- A docstring is the first statement of a module, a class or a callable when it is a string
  literal. It enters through the kit's `DocLines`, after the trim of PEP 257: the indentation of the
  first line, the common indentation of the other lines, and the blank lines at both ends go.
- A string literal statement directly after an assignment in a module or a class body is the
  attribute's docstring, as Sphinx's autodoc and griffe read it.
- A carrier is a `#` comment directly before a declaration or before its first decorator. A
  decorator whose dotted name starts with the brand is native sugar. The kit lowers
  `@acme.gen.table(name="t")` to the canonical directive through `AttachSugar`.
- The trailing comment of a declaration is a `#` comment on the line of its name.

#### Dependencies

The frontend implements `Dependent` over two stores:

| Store | The composition opens | Paths inside |
|---|---|---|
| `typeshed` | the embedded tree of typeshed's `stdlib` at commit `e75a39fe` | `<module path>.pyi`, `<package>/__init__.pyi`, and `VERSIONS` |
| `venv` | the site-packages of the environment that `VIRTUAL_ENV` names: `lib/python<X.Y>/site-packages`, with the version of its `pyvenv.cfg`, and `Lib/site-packages` on Windows | `<module path>.py` and `.pyi`, `<package>/__init__.py` and `.pyi`, and `<package>-stubs/` |

`frontend.Stores(getenv func(string) string) (map[string]fs.FS, error)` builds both without running
a tool. The `venv` store is absent where `VIRTUAL_ENV` is unset. A project's `.venv` directory in the
workspace is a second source of the same layout, through workspace paths that the selection does not
claim.

Every module reads the builtins, so round one places `builtins` beside its needs. The frontend places
a need in the order of the typing specification's module resolution:

1. A module of the workspace is no need.
2. typeshed places a module that `VERSIONS` lists, alone or by its top-level package, with a range
   that contains the option `Version`.
3. A stub-only package `<package>-stubs` in site-packages places its modules. A partial stub
   package leaves its missing modules to the package itself.
4. site-packages places the module, the `.pyi` file before the `.py` file.

The round reports each need that no step places, with its reason, and the need does not yield a
unit. A dependency unit is one module, and it loads at signature depth.

typeshed's `stdlib` at `e75a39fe`, committed on 2026-10-17, has 856 files, 752 of them stubs, in
3,102,958 bytes. typeshed is licensed under Apache-2.0, with parts under the MIT licence. The
package `frontend/typeshed` contains the tree, both licences and the commit. It embeds the tree with
`go:embed`.

#### Findings

| Code | Severity | Finding |
|---|---|---|
| `PYTHON-0001` `UnparsedFile` | Error | a Python file has a syntax error. The file contributes every declaration outside the statements with an error |
| `PYTHON-0002` `BadCarrier` | Warning | a directive carrier is outside the kernel grammar |
| `PYTHON-0003` `UnaddressedCarrier` | Warning | a directive carrier is on a subject the model cannot address |
| `PYTHON-0004` `UnreadProject` | Warning | a `pyproject.toml` does not read or parse. Its files take the root of the run of `__init__.py` files |
| `PYTHON-0005` `UnmodeledConstruct` | Warning | a signature has a construct of Python 2 that the grammar keeps, such as a tuple parameter. The declaration loads without it |
| `PYTHON-0006` `DuplicateName` | Info | a module declares a name a second time, and the first declaration is kept |
| `PYTHON-0007` `UnreadStore` | Error | a file of a store or a line of `VERSIONS` does not read |
| `PYTHON-0008` `ForeignEncoding` | Warning | the encoding declaration of a file names an encoding other than UTF-8. The file parses as UTF-8 |

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
- The six backends refuse a method with the delete accessor under `RefusedConstruct`. Python is the
  one language that deletes a property.

### The rules

| Question | Answer |
|---|---|
| Builtin | `int` is a signed integer of the platform's width, `float` a float of 64 bits, `bool` a bool, `str` text, `bytes` and `bytearray` bytes, `object` and `Any` the top type, `datetime.datetime` the well-known timestamp, `datetime.timedelta` the well-known duration, and `BaseException` and `Exception` the well-known error |
| Builtin, with type arguments | `list` and `MutableSequence` a mutable list, `Sequence`, `Collection` and `Iterable` an immutable list, `dict`, `MutableMapping` and `defaultdict` a mutable map, `Mapping` and `frozendict` an immutable map, `Iterator` and `Generator` a stream, `AsyncIterator`, `AsyncIterable` and `AsyncGenerator` an asynchronous stream |
| Builtin, a literal | `Literal` whose values are strings is text, integers a signed integer, booleans a bool, and the members of one enum a reference to the enum |
| Members | the fields and the methods of a class, and the members of its bases in the order of its bases |
| ParamRole | every parameter is an input |
| ReturnRoles | one result, and the error model `ErrorsRaised` |
| Resolve | the declaration of a dotted name in the scope, through the tiers of the frontend |
| Values | samples, zeros and literals in Python's grammar of literals |
| EnumOf | the variants, with the value of each member, or its name for `auto()` |

The Package of a reference through an import is the module of the import. The rules classify such a
reference by its module and its spelling, so `Sequence` of `collections.abc` and of `typing` are one
name. A load that reads typeshed resolves a builtin to its class in `builtins`. The rules classify
that class through `ShapeRules` and `WellKnownRules`, and a builtin without a target through
`Builtin`.

Python's `int` has no bound. The rules fold it to the signed integer of the platform's width, which
OpenAPI writes as an integer without a format. The Go spoke writes that width as `int`, the Java
spoke as `long`, the Rust spoke as `isize` and the protobuf spoke as an integer of 64 bits. The
TypeScript and GraphQL spokes write the choice of a policy. The kernel has no form for a set, so `set`
and `frozenset` fold to Opaque, as TypeScript's `Set` does.

The rules implement these optional capabilities:

| Capability | Python |
|---|---|
| `EnumRules` | the members of an enum |
| `WellKnownRules` | `datetime.datetime`, `datetime.timedelta`, `BaseException` and `Exception` of typeshed |
| `ShapeRules` | the builtins of typeshed, a `NewType` as the shape of its base, and a `Literal` alias |
| `ErrorTypeRules` | a class that extends `BaseException` |
| `PropertyRules` | a getter paired with the setter of its name |
| `ConstructRules` | `__init__`, `__new__` without `__init__`, and the generated `__init__` of a dataclass and of a NamedTuple |
| `EqualityRules` | `__eq__`, and the equality of a dataclass, a NamedTuple and an enum |
| `GenericsRules` | `Reified` reports false, because Python erases type arguments at run time |

### The backend

The backend is two kit compositions of one target. `New` writes modules, and `Stubs` writes stub
files with the body `...`:

```go
// New returns the Python backend that writes modules, files with the
// extension .py, for the target version of the policy python.version.
func New() backend.Backend

// Stubs returns the Python backend that writes stub files, files with the
// extension .pyi. Every callable has the body ..., and every default and
// every value is ....
func Stubs() backend.Backend
```

Both build through one composition:

```go
backend.New(python.Name, python.Target, python.Syntax()).
    Version(python.BackendVersion).
    FileTemplate(FileTemplate).
    KindTemplates(KindTemplates()).
    Coverage(Coverage()).
    Funcs(Funcs).
    Naming(naming).
    Packages(spell.Package).
    Respell(spell.Name).
    Types(spell.Type).
    Policies(python.Policies()...).
    Lower(Lower).
    MemberIndent(python.Indent).
    Scaffold(scaffold).
    Imports(Imports).
    Finalise(textfmt.Normalize).
    Build()
```

The templates write the layout of ruff's formatter:

- four spaces of indentation and double quotes
- two blank lines around a top-level definition, and one around a method
- a signature on one line up to 88 columns
- the parameters of a longer signature on one indented line where they fit
- one parameter on each line with a trailing comma where they do not

#### Markers

```go
// MarkerPrefix opens the name of every [Marker]. A Python name cannot
// contain a colon, so no decorator has the name of a marker.
const MarkerPrefix = "python:"

// Marker is the name of an annotation that states a construct of Python
// without a field in the model. A generator states a marker as an
// annotation with the name of the marker, and the backend writes the
// construct for it and no decorator.
type Marker string

// The markers of the Python target.
const (
    // TypedDictMarker marks an interface as a TypedDict.
    TypedDictMarker Marker = MarkerPrefix + "typeddict"
    // BasesMarker states the bases of an enum, one argument for each
    // base, such as IntFlag.
    BasesMarker Marker = MarkerPrefix + "bases"
    // KeywordsMarker states the keyword arguments of a class statement,
    // one argument for each keyword, such as total=False.
    KeywordsMarker Marker = MarkerPrefix + "keywords"
    // AnnotatedMarker states the metadata of the Annotated type of a
    // field, a parameter, a variable or an alias, one argument for each
    // element.
    AnnotatedMarker Marker = MarkerPrefix + "annotated"
    // AllMarker is a header marker. Its arguments are the names of the
    // __all__ of the file.
    AllMarker Marker = MarkerPrefix + "all"
    // LazyMarker is a header marker. Its arguments are the modules that
    // the file imports with the keyword lazy.
    LazyMarker Marker = MarkerPrefix + "lazy"
    // FutureMarker is a header marker. Its arguments are the features of
    // the future statement of the file.
    FutureMarker Marker = MarkerPrefix + "future"
    // HeadMarker is a header marker. Its arguments are the comments of the
    // head of the file, each without its #.
    HeadMarker Marker = MarkerPrefix + "head"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is not a valid marker.
func (m Marker) Valid() bool
```

#### The header

The backend writes the head of a module from the header of its file, in this order:

1. the comments of `python:head`
2. the documentation of the header, as the module docstring
3. the future statement of `python:future`
4. the import block
5. `__all__` of `python:all`

An import of a module that `python:lazy` lists takes the keyword `lazy`. The lowering refuses the
marker below the version 3.15.

#### Kinds

| Emit kind | Python |
|---|---|
| Struct | `class`, with `ABC` among its bases where it is abstract, and `@final` where it is final |
| Interface | `class X(Protocol)`, and a TypedDict with the marker `python:typeddict` |
| Enum | `class X(Enum)`, `IntEnum` for an underlying integer, `StrEnum` for underlying text, `(T, Enum)` for another underlying type, and the bases of `python:bases` |
| Alias | `type X = T` from the version 3.12, `X: TypeAlias = T` for 3.11, and `X = NewType("X", T)` for a defined alias |
| Sum | a frozen dataclass `<Sum><Variant>` for each variant with its fields, and the alias `<Sum>` of their union |
| Function | `def`, or `async def` where it is asynchronous |
| Method | `def` in its class, `__init__` where it constructs, and the decorators of its level and its accessor |
| Field | `name: T = v` in its class, `ClassVar[T]` at type level, `Final[T]` where it is immutable |
| Variable | `name: T = v` |
| Constant | `NAME: Final = v`, and `NAME: Final[T] = v` where it is typed |

- The backend writes a type parameter in the syntax of PEP 695 from the version 3.12. For 3.11 it
  writes a module-level `TypeVar` before the first declaration that uses the parameter, and
  `Generic[T]` among the bases of a class. It writes a default as `[T = D]` from the version 3.13.
  Below 3.13 it writes the default as `default=` in the `TypeVar` of `typing_extensions`.
- PEP 695 infers the variance of a type parameter. So the backend writes a declared variance as a
  `TypeVar` with `covariant=True` or `contravariant=True` in every version.
- A reference to a class that its file declares later is a string annotation for the versions 3.11 to
  3.13. Python 3.14 evaluates an annotation when a reader asks for it, so a forward reference there
  does not need quotes.
- The backend writes `/` after the last positional-only parameter. It writes `*` before the first
  keyword-only parameter that does not follow `*args`.

#### The lowering

`Lower` refuses each construct of this table under `RefusedConstruct`, and the settle withholds the
declaration. It also synthesizes the classes of the spoke.

| Construct | Reason |
|---|---|
| An annotation whose name is neither a dotted Python name nor a marker of the satellite | a decorator is a dotted name |
| A marker that the satellite does not declare, a marker on a declaration of another kind, and a marker that a declaration states twice | each marker has one meaning on one kind |
| A default, a value or an argument of a decorator that does not parse as a Python expression | the text is Python source |
| A keyword-only parameter before a positional one, a positional-only parameter after a keyword one, and a variadic parameter that is not last in its form | Python orders the parameters of a signature by how a caller passes them |
| A method with the delete accessor without a getter of its name | a deleter belongs to a property |
| The marker `python:lazy` below the version 3.15 | lazy imports are a construct of 3.15 |
| A field or a method with `Receives` | Python declares a member in its class |
| A name that a class declares twice, outside the overloads and the accessors of one name | a class declares a name once |
| A sum without a variant | a union joins at least one type |

#### Facts

| Verdict | Facts |
|---|---|
| Renders | Comment, Annotations, Visibility, Async, TypeParams, TypeParamDefault, Variance, MultiReturn, Level, Abstract, Final, Override, DefaultBody, Accessor, Constructs, Indexer, Optional, Mutability, HardPrivate, ParamDefault, Variadic, Passing, Value, Extends, Implements, Embeds, Properties, Defined, Types, Methods on a struct and an enum, Underlying |
| `Holds` | Throws, ConstEnum, Level on a struct, Sealed |
| Refuses | NamedReturn, Label, Computed, Tag, Permits, Fields on an enum, Methods on a sum, ConstParam, Optional on a method, Constructs on an interface |
| Rejects | Receives |

These verdicts need a reason:

- Visibility renders through the name: a leading underscore for package, protected and internal,
  and two for private and for a hard private member, which Python mangles.
- MultiReturn renders as a tuple of the results, which is how Python returns two values.
- Optional renders as `NotRequired[T]` in a TypedDict, and as `T | None = None` on a field of a
  class, which is how the code generators of OpenAPI and JSON Schema write a property that is not
  required.
- Embeds renders as bases, because the method resolution order of Python finds the members of a
  base as the promotion of Go finds the members of an embedded type.
- An indexer renders as `__getitem__`, with `__setitem__` where it is mutable.
- Throws takes `Holds`, because Python raises without declaring. Sealed takes `Holds`. Python has
  no sealed classes, and a subclass outside the module does not raise an error.
- NamedReturn refuses, because a tuple has no names of its members.

#### The spoke

| Form | Python |
|---|---|
| Signed and unsigned integers of every width | `int` |
| Float of 32 or 64 bits | `float` |
| Bool, Text, Bytes | `bool`, `str`, `bytes` |
| Well-known timestamp and duration | `datetime.datetime`, `datetime.timedelta` |
| Well-known empty | `None` |
| Well-known error | `Exception` |
| Dynamic | the choice of `python.dynamic` |
| Optional of T | `T \| None` |
| List of T | `list[T]`, and `Sequence[T]` of `collections.abc` for an immutable list |
| Array of N of T | a tuple of N members of `T`, and `bytes` for an array of 8-bit unsigned integers |
| Map of K to V | `dict[K, V]`, and `Mapping[K, V]` of `collections.abc` for an immutable map |
| Tuple | `tuple[A, B]`, and `tuple[()]` without members |
| Union | `A \| B` |
| Func | `Callable[[A, B], R]` of `collections.abc`, `None` without a result, and a tuple for two results or more |
| Stream of T | `Iterator[T]`, and `AsyncIterator[T]` for an asynchronous stream |
| Borrow of T | `T` |
| Wildcard | its bound, and `object` for an unbounded wildcard |
| Intersection of interfaces | a synthesized Protocol `<Type><Member>` that extends each member |
| Inline with fields alone | a synthesized TypedDict `<Type><Member>` |
| Inline with methods | a synthesized Protocol `<Type><Member>` |
| Reference, Sum | the referent's name, with its type arguments in brackets |

The spoke refuses an intersection with a member that is not an interface, and an opaque shape,
because Python has no form for either.

| Policy | Choices | Default |
|---|---|---|
| `python.version` | `3.11`, `3.12`, `3.13`, `3.14`, `3.15` | `3.11` |
| `python.dynamic` | `object`, `Any` | `object` |

The default version is the oldest version that the Python core developers support, so a written
file runs on every supported version. `object` is the top type of Python. `Any` turns the checks of a
type checker off for its values.

#### Names

| Declaration | Convention | Example |
|---|---|---|
| class, Protocol, TypedDict, enum, alias, type variable | CapWords | `OrderItem` |
| function, method, parameter, field, variable | snake_case | `order_items` |
| constant, enum member | UPPER_SNAKE_CASE | `MAX_ITEMS` |
| module, file | snake_case, with `.py` or `.pyi` | `order_items.py` |

These are the conventions of PEP 8, and the enum convention of Python's enum guide. A name that is a
keyword of Python takes a trailing underscore, as PEP 8 recommends, so `class` becomes `class_`. A soft
keyword, such as `type` or `match`, keeps its spelling. A name that starts and ends with two
underscores keeps its spelling.

#### Files and imports

`spell.Package` returns the module of a routed file: its path without the extension, and the
directory of an `__init__.py`. Every routed file is a module of its own, so the layout qualifies a
reference into another file. The import block writes `from <module> import <name>` for each
referenced declaration. It groups the imports as isort's defaults do: the future statement, the
standard library, the third-party modules and the modules of the plan. In each group, the `import`
statements come first and then the `from` statements, each sorted by module. The backend reads
typeshed's `VERSIONS` to tell a module of the standard library. It writes an `__init__.py` only where
a generator states one, because a namespace package imports without one.

### Restating a Python source

```go
// Stated returns the markers that restate the stamps of the Python
// frontend on the declaration id, in this order: the TypedDict marker of
// an interface, the bases of an enum, the keywords of a class statement
// and the metadata of an Annotated type. A generator over Python sources
// appends them to the annotations of the emit declaration that it writes
// for id. The Python backend then writes the declaration as its source
// states it. A declaration that the frontend did not stamp has no
// markers.
//
// Each read records on the invocation of m.
func Stated(m eidos.Matcher, id symbol.Identity) symbol.Annotations

// Header returns the header of the file that declares d, and nil for a
// file without a docstring, an __all__, a future statement, a lazy import
// and a head comment. d is a declaration or a file. The documentation of
// the header is the module docstring, and its annotations are the header
// markers of the other four, in that order.
//
// Each read records on the invocation of m.
func Header(m Match, d node.Declaration) *emit.File
```

### The toolchain adapter

- `Available` reports whether `python3` and `pyright` are on the path.
- The layout writes `pyrightconfig.json` at the root of the scratch tree, with the version of the
  policy `python.version`, unless the output has one there.
- The parse runs `python3` over a script that parses each file with `ast.parse` and the
  `feature_version` of the target version. It returns an error for each syntax error, with its file,
  its line and its column.
- The type check runs `pyright --outputjson --pythonversion <version>` over the tree, and returns an
  error for each diagnostic of severity error.
- `RunTests` runs `python3 -m unittest discover` over the tree for the files `test_*.py`.
- `Satisfies` writes a probe module that returns a value of the type as the contract, and reports
  whether pyright accepts the probe.

CI installs Python 3.15 and pyright 1.1.414, as it installs TypeScript for tsc.

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

The store locator of the binary calls `frontend.Stores` of the Python frontend only when the tree has
a Python file, as it does for the stores of Go and Java.

### Tests

The corpus of the satellite declares these verdicts on the feature inventory:

| Feature | Verdict | Reason |
|---|---|---|
| struct_fields, struct_methods | projects | annotated attributes, and methods |
| method_overloads | projects | `@overload` |
| constants | projects | `Final` |
| cross_package_ref | projects | an import of another module |
| composite_refs | projects | `X \| None`, `dict[str, X]` and `Callable[[X], X]` |
| builtin_ref | projects | the builtins |
| directive_carrier | projects | `#` carriers |
| test_classification | projects | `test_*.py` |
| interfaces | projects | a Protocol |
| enum_values | projects | an enum |
| directive_sugar | projects | a decorator of the brand |

The conformance module gains the fixture `shop`, a project with a `pyproject.toml` of uv_build and
the source root `src`. Its modules state every construct of the tables above: dataclasses with
defaults and keyword-only fields, a NamedTuple, a TypedDict with `total=False`, `Required`,
`NotRequired` and `ReadOnly`, a `StrEnum` and an `IntFlag`, a Protocol with a property, an abstract
base class, generic classes in both syntaxes with a bound and a default, a `NewType`, both forms of an
alias, overloads, positional-only and keyword-only parameters, `**kwargs: Unpack[...]`, a property
with a setter and a deleter, class and static methods, async functions and an async generator, a
lazy import, a version check, `__all__`, a stub file beside its source and a stub-only module. The
plans of the fixture write these files:

- `go-model` writes the Go model of the classes in one file. Each dataclass and each NamedTuple
  becomes a struct, and each Protocol an interface. A TypedDict becomes a struct with an optional
  field for each field that is not required. An enum becomes a defined type with a constant for each
  member. The Go toolchain type-checks the file.
- `ts-client` writes the TypeScript interfaces, enums and aliases of the classes. tsc type-checks the
  file.
- `python-mirror` restates each declaration through `Stated` and each file through `Header`, and
  writes the mirror of each module under `mirror/`. Each written file equals its source without its
  carriers, between the header and the provenance trailer that the output contract adds. The
  toolchain adapter parses, type-checks and tests the mirror, and the frontend loads it back with the
  stamps of its sources.

The matrix entry of Python fills the read side, the projection side, the render side and the hub.

### Failures

| Failure | Code | Effect |
|---|---|---|
| A syntax error | `UnparsedFile` | the file contributes every declaration outside the statements with an error |
| A `pyproject.toml` that does not read or parse | `UnreadProject` | its files take the root of the run of `__init__.py` files |
| A carrier that the frontend cannot place | `UnaddressedCarrier` | the carrier does not apply |
| A construct of Python 2 in a signature | `UnmodeledConstruct` | the declaration loads without it |
| A second declaration of a name | `DuplicateName` | the first declaration is kept |
| A store that does not read | `UnreadStore` | the load fails |
| An encoding declaration of another encoding than UTF-8 | `ForeignEncoding` | the file parses as UTF-8 |
| An import that no store places | `UnplacedNeed` | its references keep their spellings |
| A construct that the lowering refuses | `RefusedConstruct` | the settle withholds the declaration |
| A stated fact without an idiom | `RefusedFact` | the declaration renders without the fact, with a warning |
| A shape that the spoke refuses | `RefusedType` | the generator's emitter reports it at the source declaration |

### Migration

`Param.Passing` is a schema edit and a regeneration of the model, and every sealed state runs cold
once after it. `AccessorDelete` adds a value to an enumeration. The Python frontend is the one
frontend that sets the value. The coverage suite checks that a backend takes a stance on every fact.
A released backend without a stance on `Passing` fails the suite against a kernel with the fact, so
the kernel and every backend release together. A workspace with Python files loads them in the stock
binary. The plan `python` does not write a file without a plugin.

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
templates write the layout of ruff's formatter instead.

### A future statement in every module

Every module for a version below 3.14 could start with `from __future__ import annotations`. The
statement turns every annotation into a string at run time. pydantic and the other readers of
`__annotations__` then have to evaluate each string. A string annotation for a forward reference alone
keeps every other annotation a value.

## Drawbacks

- The kernel grows by one type, `symbol.Passing`, with three values, one field of `Param` with its
  fact, one value of `symbol.Accessor`, and one rule of the discriminator. The Go, TypeScript, Java,
  Rust, protobuf and GraphQL backends each take a stance on the new fact and refuse the new accessor.
- A new fact fails the coverage suite of every released backend, so the kernel release with
  `Passing` is a breaking release of the backends.
- The repository takes on typeshed's `stdlib`: 856 files and 3.1 MB under Apache-2.0 and MIT. A
  refresh of the embedded tree is a commit of the new tree.
- The fork's `go.mod` declares its own path, so every change taken from upstream keeps that line.
- Every binary that links the satellite compiles 3.5 MB of parser C under cgo.
- The parse reads one module. An enum whose enum base another module declares loads as a struct. A
  type variable that another module declares does not give a type parameter to the functions that
  use it.
- `Annotated` inside another type keeps its metadata in a stamp alone, and the backend writes it only
  where it wraps the whole annotation of a declaration.
- `set` and `frozenset` fold to Opaque, and `int` folds to the platform's width.
- One load evaluates the checks of one version and one platform.
- The parse reads the bodies of methods for the assignments to `self` and for `yield`.
- The tests of the toolchain adapter need `python3` and pyright on the machine.

## Open questions

None.

## Unresolved and future work

- The types that a type checker infers for the declarations that state none.
- A kernel form for sets.
- The third-party stubs of typeshed beside the embedded standard library.
- Cython's `.pyx` files and the cells of notebooks.

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
| setuptools, package discovery | <https://setuptools.pypa.io/en/latest/userguide/package_discovery.html> |
| hatch, the build configuration | <https://hatch.pypa.io/latest/config/build/> |
| Poetry, the `packages` setting | <https://python-poetry.org/docs/pyproject/> |
| uv, the build backend | <https://docs.astral.sh/uv/concepts/build-backend/> |
| pdm-backend, the build configuration | <https://backend.pdm-project.org/build_config/> |
| maturin, the project layout | <https://www.maturin.rs/project_layout.html> |
| pytest, the default `python_files` | <https://docs.pytest.org/en/stable/reference/reference.html#confval-python_files> |
| pyright, the command line | <https://github.com/microsoft/pyright/blob/main/docs/command-line.md> |
| ruff, the formatter | <https://docs.astral.sh/ruff/formatter/> |
