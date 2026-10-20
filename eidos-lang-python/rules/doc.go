// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the projection rules of Python.
//
// [New] returns the value that a composition registers. It satisfies the
// source-rules contract and the capabilities for enums, well-known types,
// shapes, error types, properties, constructors, equality and generics.
//
// # Decisions
//
//   - A class inherits the members of its bases in the C3 linearization of
//     the class, as the method resolution order of Python finds an
//     attribute.
//   - Every parameter is an input. A result whose type folds to a stream is
//     a stream, and every callable raises under [rules.ErrorsRaised].
//   - The classes of builtins, typing, collections.abc and collections that
//     typeshed declares have the shapes of [Rules.ShapeOf], and a name
//     without a target has the same shape through [Rules.Builtin]. datetime,
//     timedelta, Exception and BaseException are well-known types.
//   - Literal has the shape of its values, and an enum has the values that
//     Python 3.13 and later give its members, auto() included.
//   - A class constructs through its __init__, its __new__, the __init__
//     that a dataclass generates, or the __new__ that a named tuple
//     generates. A dataclass_transform of PEP 681 on a decorator, a base or
//     a metaclass makes a class a dataclass under the defaults of the
//     transform. A getter of a property pairs with the setter of its name.
//   - A type works as a key of a dict where it hashes, as Python demands.
//   - The spelling of a directive resolves in the scopes of an annotation:
//     the enclosing classes, the module and its imports, its star imports
//     and builtins.
//   - A literal reads in the grammar of the literals of Python.
//
// # Dependency position
//
// lang/python/rules imports the sdk facade, lang/python and its spell
// package, lang/numeric and the Go stdlib.
package rules
