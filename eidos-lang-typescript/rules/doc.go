// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules implements the projection rules of TypeScript, which are
// the decisions of the language inside the kernel's walks.
//
// [New] returns the value that a composition registers. The value
// implements the source-rules contract and the capabilities for enums,
// generics, properties, constructors and equality. It does not implement
// the capabilities for error values, tags, declared failure types,
// ownership and promotion, because TypeScript has none of them.
//
// # Decisions
//
//   - A type inherits members through its extends clause and then its
//     implements clause. A name that arrives more than once is kept as an
//     overload. A member of the type's own declaration hides the
//     inherited members of the same name.
//   - A parameter of the type AbortSignal is the context, and every other
//     parameter is an input. A return of an asynchronous stream is a
//     stream. A TypeScript signature does not declare its errors, so
//     every callable has the error model of raised errors.
//   - The builtin table classifies the global types by name when no
//     import binds the name. The table has string, number, boolean, Date,
//     Uint8Array, the arrays, the records and maps, and the asynchronous
//     iterables.
//   - A name in a directive resolves in TypeScript's scope order: the
//     namespaces, the module and its imports, and then the global
//     package.
//   - A literal follows TypeScript's grammar, and a number is a double.
//     The values of an enum are the values of its constant enum
//     expressions.
//   - Every type is comparable, because === and a Map can compare any two
//     values.
//
// # Values
//
// A number is a double, so a derived or lifted number has 64 bits. A
// bigint, undefined, a Date and a Uint8Array have no literal in the value
// model, so the rules write them as TypeScript text. Only a TypeScript
// backend can render that text. An object literal that sets one property
// of a class or an interface is asserted to the type. So is a member of
// an enum.
//
// # Dependency position
//
// lang/typescript/rules imports the sdk facade, lang/naming,
// lang/numeric, the satellite root for its language, its namespace key,
// its module paths and its literal readers, and the Go stdlib. It never
// imports the frontend: the rules read the sealed graph and its facts,
// not source.
package rules
