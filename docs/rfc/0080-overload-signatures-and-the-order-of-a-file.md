---
rfc: 0080
title: Overload signatures and the order of the declarations of a file
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-20
updated: 2026-10-20
discussion: https://github.com/dokimasia/eidos/issues/42
supersedes: none
superseded-by: none
produces-adr: ADR-0081, ADR-0082
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0080: Overload signatures and the order of the declarations of a file

## Summary

We propose two changes that let TypeScript and Python write an overloaded callable and a class in
the order that their languages evaluate. A function and a method gain the mark `Overload`, which
states that the declaration is one overload signature of its name. The render orders the
declarations of a file. The functions of one name form one block with the signatures first, and a
declaration follows each declaration of the file that its target evaluates when the module runs.

## Motivation

- The TypeScript frontend loads each overload signature as a function and leaves out the
  implementation. The backend writes a body for every function and refuses a class with two
  methods of one name. It does not declare that TypeScript overloads either. A generator that
  restates an overloaded TypeScript callable fails its plan, and so does a generator that writes the
  overloads of a Python callable into TypeScript.
- The Python satellite marks an overload signature with the decorator `overload` and the
  implementation with the stamp `python.implementation`. No backend of another language reads
  either of them.
- The render writes the declarations of a file in the order of the flush, which follows the
  positions of their sources. A Go source that declares `type Admin struct{ User }` before
  `type User` gives a Python module whose class `Admin` refers to `User` before the module defines
  `User`.
  Python 3.15 raises `NameError`, pyright 1.1.413 reports `reportUndefinedVariable`, and tsc 7.0.2
  reports TS2449 for the TypeScript module.
- tsc 7.0.2 reports TS2391 for overload signatures that another declaration separates from their
  implementation. It reports the same error for a signature after the implementation and for
  signatures without an implementation.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Kernel | `eidos-core/symbol/schema` | `Overload` on `Function` and `Method`, under the new fact `Overload`, and the regenerated model |
| Kernel | `eidos-core/frontend/load` | the discriminator of the implementation of an overloaded name |
| Kernel | `eidos-core/rules` | `IsImplementation` tells the implementation of a method from its signatures |
| Kernel | `eidos-core/backend/render` | `Language.Evaluated`, `Language.Implemented` and the order of the declarations of a file |
| Kernel | `eidos-core/backend` | `Builder.Evaluated` and `Builder.Implemented` |
| SDK | `eidos-sdk` | the regenerated facade |
| Lowering | `eidos-lang/lowering` | `GroupOverloads` puts the methods of one name together |
| TypeScript | `eidos-lang-typescript` | the frontend marks an overload signature and loads the implementation. The backend overloads, writes both forms and states what a module evaluates |
| Python | `eidos-lang-python` | the frontend marks an overload. The backend writes the decorator for the mark and states what a module evaluates, and the rules read the mark |
| Go, Java, Rust, protobuf, GraphQL | the backends | a verdict on the fact `Overload` |

### The model

```go
package schema

type Function struct {
    // The other fields are unchanged.
    Overload bool `eidos:"both,fact=Overload"` // one overload signature of the name, without a body
}

type Method struct {
    // The other fields are unchanged.
    Overload bool `eidos:"both,fact=Overload"` // one overload signature of the name, without a body
}
```

An overload signature states one form of a callable. The callable of the same name without the
mark is its implementation, and the signature has no body. A language that writes a body for each
overload, such as Java, states two callables of one name without the mark.

### The identity of an implementation

The discriminator of a callable spells its parameters, so an implementation whose parameters spell
as one of its overloads has the identity of that overload. TypeScript code states such an overload
to show callers the signature of the implementation. The discriminator of the implementation of an
overloaded name starts with `=`, which no other discriminator starts with. The implementation is a
callable without the mark whose scope declares a marked callable of its name: the package for a
function, and the type for a method. A callable of a name without a marked callable keeps its
discriminator.

```go
package rules

// IsImplementation reports whether the method m is the implementation of
// an overloaded name among methods: m has no mark, and another method of
// its name and its level has the mark Overload. A reader of the
// signatures that callers see, such as the constructors of a class,
// leaves the implementation out.
func IsImplementation(m *node.Method, methods []*node.Method) bool
```

The member walk keeps the implementation beside the signatures of its name, so a generator that
restates the methods of a class restates the implementation too. `Mirror` leaves out the mark,
because a generator writes the body of the method that it mirrors.

### The frontends

- The TypeScript frontend marks a function signature and a method signature of a class where the
  scope declares two or more callables of the name. The implementation after the signatures loads
  as a function or a method of its own, whose discriminator starts with `=`. An abstract method
  signature keeps `Abstract` and takes no mark. The methods of an interface take no mark,
  because an interface has no implementation.
- The Python frontend marks a function and a method that the decorator `overload` of typing
  decorates. The decorator stays among the annotations, as the other decorators of a fact do. The
  stamp `python.implementation` is removed, because the callable of an overloaded name without the
  mark is the implementation.

### The order of a file

```go
package render

// Evaluated yields the names that the language evaluates when a module
// runs the declaration d. Each name is the declared name of a declaration
// of the file that d needs defined before it, such as the base of a class.
// The render ignores a name that no declaration of the file declares.
type Evaluated func(d symbol.Symbol, yield func(name string))

type Language struct {
    // The other fields are unchanged.
    Evaluated Evaluated // nil orders by the flush and the overload blocks alone
    // Implemented states that each overloaded name of a file has one
    // implementation after its overload signatures, as a module of
    // TypeScript or Python has. A stub file of Python has none.
    Implemented bool
}
```

```go
package backend

// Evaluated sets the names that the language of the target evaluates when
// a module runs a declaration. Unset, the render keeps the order of the
// flush apart from the overload blocks.
func (b *Builder) Evaluated(e render.Evaluated) *Builder

// Implemented declares that each overloaded name of a file of the target
// has one implementation after its overload signatures.
func (b *Builder) Implemented() *Builder
```

A file that a family claims renders through the template of the family and keeps its order. The
render orders the declarations of each other file before it writes them:

1. Each declaration of each unit of the file has its position in the order of the flush.
2. The functions of one declared name form one block at the position of the first of them. The
   block holds the marked signatures in their order and then the other functions of the name.
   Every other declaration is a block of its own.
3. Where the language states `Evaluated`, a block follows each block of a name that one of its
   declarations evaluates. Among the blocks that can come next, the block at the lowest position
   comes first.
4. A cycle of evaluated names keeps the order of the flush for its blocks. The backend then
   reports the reference that comes too early, as the Python backend reports a base that the
   module declares later.
5. Where the language is `Implemented`, the render reports a block with a marked signature and no
   implementation under the new code `UnimplementedOverload` and skips the block. It reports each
   implementation after the first of a block under the same code and skips it.

A cluster renders at the position of its first member in the order of the file. A language that
states neither `Evaluated` nor `Implemented` keeps the order of the flush, and so does a file without
a block of two functions and without an edge. Each worker of the render grows the memory of the
order once for its largest file. After that the order allocates only for its findings.

### The lowering

```go
package lowering

// GroupOverloads returns methods with the methods of each overloaded name
// and level together at the position of the first of them: the overload
// signatures in their order, then the other methods of the name. An
// overloaded name has a method with the mark Overload, and every other
// method keeps its order. GroupOverloads returns methods itself where
// every overloaded name already stands together with its signatures
// first, and allocates a new list otherwise.
func GroupOverloads(methods []*emit.Method) []*emit.Method
```

The TypeScript and Python backends order the methods of a class in their lowering, because the
template of a class writes its methods in the order of the class.

### The backends

| Target | `Overload` | What a module evaluates |
|---|---|---|
| TypeScript | renders: a signature without a body, and the implementation after it | the superclass of a class, and the decorators of a class and of its members |
| Python | renders: the decorator `overload` | the bases of a class and of a Protocol, the bases or the mixin of an enum, the decorators of a class and of a callable, and the same names of a nested class |
| Go | rejects, because Go declares one function of a name | nothing |
| Java | rejects, because a Java method has a body unless it is abstract | nothing |
| Rust | rejects, because Rust has no overloads | nothing |
| protobuf | rejects, because an rpc takes one request | nothing |
| GraphQL | rejects, because a field has one argument list | nothing |

- The TypeScript backend declares `Overloads`, so the layout counts the functions of one name as
  the overloads of one function. The backend is `Implemented`. Its lowering orders the methods of a
  class with `GroupOverloads`. It refuses a class whose overloaded method name has no
  implementation or two, unless the signatures are abstract.
- The modules backend of Python is `Implemented` and states `Evaluated`. Its stubs backend states
  neither because a type checker reads a stub file without running it. The backend writes the
  decorator `overload` for the mark where the annotations do not state it. The decorator is the
  first decorator line, as in typeshed. Its lowering marks a callable whose annotations state the
  decorator, so the writer reads the mark alone. The lowering also orders the methods of a class
  with `GroupOverloads`. It refuses a class whose overloaded method name has two implementations.
  In a module it also refuses a name without one outside a Protocol unless the signatures are
  abstract.
- The rules of TypeScript and Python leave the implementation out of the constructors of a class
  through `IsImplementation`.

### Failures

| Failure | Effect |
|---|---|
| The mark on a target that rejects it | the render reports `RejectedFact`, and the declaration does not render |
| Overload signatures of a function name without an implementation, in a file of a language that is `Implemented` | the render reports the block under `UnimplementedOverload` and skips it |
| A second implementation of a function name, in a file of a language that is `Implemented` | the render reports it under `UnimplementedOverload` and skips it |
| An overloaded method name of a TypeScript or a Python class without one implementation | the lowering refuses the class |
| A cycle of evaluated names | the cycle keeps the order of the flush, and the backend refuses the reference that comes too early |

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| The mark `Overload` on a function and a method | in | TypeScript and Python split an overloaded callable into signatures and one implementation, and a backend of another language reads neither a decorator nor a stamp | none known |
| The implementation in the model of TypeScript | in | a generator that restates an overloaded callable writes the implementation, which tsc requires | none known |
| One implementation for each overloaded name of a module | in | tsc 7.0.2 reports TS2391 and pyright 1.1.413 reports `reportNoOverloadImplementation` for a module without one | none known |
| The order by evaluated names | in | Python and TypeScript evaluate the base of a class when the module runs, and the flush follows the order of a source of another language | none known |
| The order by the values of a file | out | a default, an initializer and the value of a constant are text, so the render cannot read the names that they evaluate | a value tree for every value |
| The order by types | out | Python below 3.14 writes a later annotation as a string, Python 3.14 evaluates an annotation when a reader asks for it, and TypeScript erases its types | none known |
| The implementation in the member walk | in | the implementation is a member of its class, and a generator that restates the methods of a class needs it | a reader that lists the signatures of a class alone |
| The grouping of a method name without a mark | out | a class keeps the order of its methods, and the TypeScript and Python backends refuse two implementations of a name | none known |
| The order and the check of a Python stub file | out | a type checker reads a stub file without running it, and pyright 1.1.413 accepts overload signatures without an implementation in a stub file | none known |

## Alternatives

### A marker of each satellite

Python would keep the decorator `overload`, and TypeScript would read a marker of its own. A
generator that translates between the two would map one marker to the other, and the other
backends would not read either marker.

### The body as the mark

No field of the emit model would separate a signature from an empty implementation, because every
callable has a body and an empty body is a stub that a generator fills. Under this alternative a
function without a body is a signature.

### The last function of a name as the implementation

The Python writer treats every function of a name before the last as an overload. The flush
follows the positions of the sources, so the last function of a name is the implementation only
where the source puts it last.

### The order in each backend

A lowering sees one declaration, and a TypeScript or a Python file contains the declarations of
each unit that routes to it. Only the render sees every declaration of a file.

## Drawbacks

- `Function` and `Method` gain one field and the model gains one fact. Each of the seven backends
  states a verdict on it, and so does each coverage table of a test.
- The TypeScript frontend loads one more callable for each overloaded name. A reader that lists the
  callables of a TypeScript module lists the implementation beside the signatures.
- The render orders each file of TypeScript and each module of Python. The order costs one pass
  over the declarations of a file and a sort of its blocks. A render of 1,000 files of 200
  declarations each allocated 613,356 to 613,390 times with the order in eight runs. Without the
  order it allocated about 613,285 times.
- A TypeScript class whose decorator is a function of the file follows the function, although
  TypeScript hoists the function.
- The removal of the stamp `python.implementation` changes the keys of the Python satellite, and
  the discriminator of each Python implementation changes.
- The kit of a backend gains two settings, and the render gains one code.

## Open questions

None.

## Unresolved and future work

- The names that a default, an initializer and the value of a constant evaluate.

## References

| What | Where |
|---|---|
| Issue 42, the overload signatures of TypeScript | <https://github.com/dokimasia/eidos/issues/42> |
| Issue 43, the order of the declarations of a file | <https://github.com/dokimasia/eidos/issues/43> |
| The TypeScript handbook, function overloads | <https://www.typescriptlang.org/docs/handbook/2/functions.html> |
| The specification of the typing of Python, overloads | <https://typing.python.org/en/latest/spec/overload.html> |
