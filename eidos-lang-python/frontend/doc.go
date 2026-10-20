// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads Python modules and stub files into the node graph
// through the frontend kit and the pinned grammar of the satellite.
//
// [New] builds the frontend. It claims the files that end in .py and .pyi,
// without the virtual environments, the installed distributions and the
// caches of a workspace. A unit is one module, because Python scopes a name
// to its module: a source file, a stub file, or both files of one stem in
// one directory, of which the stub states the types. The package of a
// module is its dotted name with slashes, as shop/models for shop.models,
// below the source root that the pyproject.toml of its project states.
//
// # Declarations
//
// The kind of a class follows from its direct bases: a Protocol and a
// TypedDict are interfaces, a class of enum is an enum, and every other
// class is a struct. The members of a class are its methods, its nested
// classes, its annotated attributes and the attributes that its methods
// assign to their receiver. A function and a method load with their
// parameters, each passed by position, by keyword or in the way of the
// language, their results, their type parameters and their decorators. A
// callable under the decorator overload of typing has the mark Overload,
// and the callable of its name without the mark is the implementation. A
// module loads its variables, its constants under Final, its aliases, its
// type variables, its imports and its __all__. An assignment without an
// annotation whose value is a type expression, such as Ids = list[int], is
// an implicit alias, as a type checker reads it. The functional forms of
// NamedTuple, TypedDict, the classes of enum and namedtuple of collections
// load as their class syntax loads. A named tuple of collections extends
// NamedTuple of typing, and its fields have no type.
//
// The frontend stamps what the model has no field for under the keys of
// the satellite root: the base TypedDict, the bases of an enum, the
// keywords of a class statement, InitVar, a callable that yields, a type
// variable, the metadata of Annotated, and the lazy imports, the future
// statements, the comments of the head and the excluded branches of a
// file.
//
// # Types
//
// A type reference keeps its spelling. Its structural form is the form
// that the syntax states: Optional for X | None, a Union, a Func for
// Callable, a Tuple and a List for tuple, and Named for every other type.
// A string annotation is the type that it contains.
//
// # Conditions
//
// TYPE_CHECKING is true, and the checks of sys.version_info and
// sys.platform compare with [Options]. A branch that a check excludes does
// not load, and every branch of any other condition loads.
//
// # Resolution
//
// A name resolves in Python's order of scopes: the enclosing classes, the
// module and its imports, its star imports and builtins. The frontend is an
// exporter, so a name that a module imports resolves through the module.
//
// # Dependencies
//
// The dependency rounds place the modules that the workspace imports and
// does not declare: the standard library from the embedded stubs of
// typeshed under [TypeshedStore], and the installed distributions from the
// site-packages directory of a virtual environment under [VenvStore], which
// [Stores] locates.
//
// # Findings
//
// The codes of the frontend open with PYTHON: [UnparsedFile],
// [BadCarrier], [UnaddressedCarrier], [UnreadProject],
// [UnmodeledConstruct], [DuplicateName], [ForeignEncoding] and [BadMarker].
//
// # Dependency position
//
// frontend imports lang/python, its spell, internal/project and
// frontend/typeshed packages, lang/numeric, lang/treesitter and its python
// grammar, the sdk facade and the Go stdlib.
package frontend
