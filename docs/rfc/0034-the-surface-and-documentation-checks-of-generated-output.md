---
rfc: 0034
title: The surface and documentation checks of generated output
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0026
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0034: The surface and documentation checks of generated output

## Summary

The toolchain kit checks generated output through the compiler of its language. The output
parses, it type-checks, and its own tests pass. Two properties that a consumer of the output reads
have no check:

- The public surface. A change of a template that changes the API of the output shows in the byte
  goldens of the fixtures among every changed comment and every moved declaration, and the plan
  export lists names and spellings without signatures.
- The documentation. A generator that leaves one public declaration without a doc comment passes
  every check, and its output fails the lint of a consumer that requires doc comments.

The kit also has configuration that buys nothing. `toolchain.Setup` builds an adapter and a fixture
for each check, although the four adapters are stateless values. The Go check `AssertVets` takes an
adapter that it uses only to lay out the project.

We propose three changes to the kit:

- `toolchain.Surface` renders the public surface of generated output from the model that the
  frontend of its language loads. `toolchain.AssertSurface` matches the surface against a golden
  file, which `-update` writes.
- `toolchain.AssertDocumented` requires a doc comment on every public declaration at file level of
  the output.
- `RunToolchainSuite` takes the adapter and the fixture as values, `Setup` is deleted, and
  `gotesting.AssertVets` takes no adapter.

## Motivation

### The surface of generated output

Generated output is API, because a consumer calls the declarations that a generator writes. A
change of a template that changes the type of a parameter, drops a member or changes the value of a
constant changes the code that every consumer compiles against. The fixtures of the repository
keep byte goldens of the generated files. A change of a template rewrites many lines of each
golden, such as comments, imports and the order of declarations, and a reviewer reads the whole diff
to find the lines that change the API. The plan export lists the name and the spelling of each
generated declaration and no signature.

The model contains the surface of every declaration that a frontend loads: its visibility, its type
parameters, its parameters and results with their spellings, the members of a host and the value of
a constant. Every language of the repository has a frontend that loads it into the model. The
public surface of generated output is the model of its public declarations, written as one line for
each feature.

### The documentation of generated output

A consumer reads generated declarations in the hover of an editor. A project whose linter requires
a doc comment on each public declaration reports every undocumented declaration of generated output
that it lints. The frontends record the doc comment of each declaration in its `Doc` field. A Go
declaration in a group without a doc comment of its own receives the doc comment of the group. No
check reads the field of generated output.

### Configuration that buys nothing

`toolchain.Setup` returns an adapter and a fixture, and `RunToolchainSuite` calls it four times:
once for the gate and once for each of the three checks. The four adapters of the repository are
values of an empty struct type, so every call returns an equal adapter. A fixture is a value, and
each check lays it out in a scratch directory of its own. `gotesting.AssertVets` takes an adapter,
uses it to lay out the project and to write the language into its messages, and always runs
`go vet`, whose project only the Go adapter lays out.

## Detailed design

### Components

| Component | Package | Callers |
|---|---|---|
| `Surface` | core/toolchain | `AssertSurface`, and a test that compares a surface by other means |
| `AssertSurface` | core/toolchain | the reference test, for its Go and TypeScript output |
| `AssertDocumented` | core/toolchain | the reference test, for its Go and TypeScript output |
| `RunToolchainSuite(t, a, g)` | core/toolchain | the tests of the four adapters and of the kit |
| `AssertVets(ctx, tb, g)` | lang/go/testing | the Go adapter test, the reference test and the declarative composition test |

### Invariants

1. The surface is a function of the model. Two loads of one fixture render the same text.
2. A change of a comment, of a body, of the name of a parameter or a result of a function or a
   method, of the order of declarations or of the order of members does not change the surface. A
   consumer observes none of them. A spelling of a type keeps what its source wrote, such as the
   parameter names of a function type.
3. Every check fails on a fixture without a public declaration, because a check over nothing proves
   nothing.
4. The surface and documentation checks do not run a toolchain, so they never skip.

### The load of generated output

The surface check and the documentation check load the output of a fixture through one frontend,
as a workspace loads a tree.
The tree contains the files of the fixture and the files of its sources, as `Prepare` lays them
out: a generated file replaces a source file at the same path. The load runs under the context of
the test and under the brand `toolchain`. A file that the frame of that brand marks is the output
of the load itself and stays out of the load, so a fixture does not use that brand. The load reads
no store. A dependent frontend reports each import that it cannot place as a Warning, and the
checks accept a Warning.

A check reads the declarations of the files of the fixture alone. The declarations of the sources
are in the graph for the lookup of a host, and outside the check.

### The surface

```go
package toolchain

// Surface returns the public surface of the files of g that the
// frontend f claims, loaded as the checks of this package load
// generated output. The surface has a line for each public declaration
// at file level and a line for each public member and each relation of
// such a declaration. The lines of one package follow a line
// "package <path>", in byte order, and the packages follow in the order
// of their paths. A method at file level renders as a member of its
// host, so a Go method and a method in the body of a class render
// alike. A method whose host the graph contains and whose host is not
// public is not in the surface.
//
// Error modes: Surface returns the error of the load, and an error that
// joins one error for each finding of Error severity of the load.
func Surface(ctx context.Context, f plugin.Frontend, g Generated) (string, error)

// AssertSurface matches the surface of the files of g that f claims
// against the golden file at path, a slash-separated path relative to
// the directory of the test, through golden.MatchAt. Under -update the
// match writes the file. It stops the test where the load fails or
// reports an Error, and where the surface has no public declaration.
func AssertSurface(tb assert.TB, f plugin.Frontend, g Generated, path string)
```

The line of a declaration starts with its modifiers, each one lowercase word and a space, in this
order: `static`, `abstract`, `final`, `sealed`, `const`, `async`, `readonly`, `get`, `set`,
`constructor` and `indexer`. The spelling of the kind follows, which `symbol.Kind.String`
returns, for a declaration at file level. Every type, value and default in a line is the spelling
that the frontend recorded.

| Declaration | Line after the modifiers |
|---|---|
| Struct, Interface, Enum, Sum | `<Kind> <Name><TypeParams>` |
| A relation of such a host | `<Name> extends <T>`, `<Name> implements <T>`, `<Name> embeds <T>`, `<Name> permits <T>` |
| Field | `<Host>.<Name>[?] <Type>` |
| Method | `<Host>.<Name><TypeParams>(<Params>) <Results>` |
| Enum variant | `<Host>.<Name> = <Value>`, or `<Host>.<Name>` without a value |
| Sum variant | `<Host>.<Name>(<Fields>)`, each field as `<Name> <Type>` |
| Function | `Function <Name><TypeParams>(<Params>) <Results>` |
| Alias | `Alias <Name><TypeParams> = <Target>`, or `Alias <Name><TypeParams> <Target>` for a defined type |
| Constant | `Constant <Name> <Type> = <Value>` |
| Variable | `Variable <Name> <Type>` |
| A nested type | the line of its kind, with `<Host>.<Name>` as its name |

A parameter renders as its type, with `...` before the type of a variadic parameter, its label and
a colon before it in a language with labels, `?` after an optional parameter and ` = <Default>`
after a default. The results render as one type, or as a list in parentheses for more than one. A
type, a value or a result that the frontend did not record is left out of the line. Type
parameters render as `[T]`, or as `[T <Bound>]` with their bounds joined by ` & `. The surface of
the enum methods that the reference ensemble generates reads:

```text
package example.com/shop/catalog
Function ParseStatus(string) (Status, error)
Function StatusValues() []Status
Status.IsValid() bool
Status.MarshalText() ([]byte, error)
Status.String() string
Status.UnmarshalText([]byte) error
Variable ErrUnknownStatus
```

### The documentation

```go
package toolchain

// AssertDocumented requires a doc comment on every public declaration
// at file level of the files of g that f claims, loaded as the checks of
// this package load generated output. A method whose host is not public
// is outside the check. A member of a host, such as a field, needs no
// doc comment of its own. Each declaration without a doc comment fails
// the test on a record of its own, with its kind, its name and its
// file. AssertDocumented stops the test where the load fails or reports
// an Error, and where the files of g declare nothing public.
func AssertDocumented(tb assert.TB, f plugin.Frontend, g Generated)
```

### The suite and the vet check

```go
package toolchain

// RunToolchainSuite runs the three checks of the kit over g with the
// adapter a: the output parses, it type-checks, and its tests pass. The
// checks gate on the toolchain once. Each check runs in a parallel
// subtest under the context of that subtest, and lays g out in a
// scratch directory of its own.
func RunToolchainSuite(t *testing.T, a Adapter, g Generated)

package testing // go.dokimi.dev/eidos/lang/go/testing

// AssertVets runs go vet over g under ctx, in a project that the Go
// adapter lays out.
func AssertVets(ctx context.Context, tb assert.TB, g toolchain.Generated)
```

`toolchain.Setup` is deleted. A fixture is a value, and no check changes it.

### Failures

| Case | Behaviour |
|---|---|
| A load that returns an error | `Surface` returns it. The asserts stop the test |
| A load that reports an Error | `Surface` returns an error with each Error. The asserts stop the test |
| A fixture without a public declaration | `AssertSurface` and `AssertDocumented` stop the test |
| A golden that differs from the surface | `AssertSurface` stops the test, through `golden.MatchAt` |
| A public declaration without a doc comment | `AssertDocumented` fails the test once for each declaration |
| A file framed under the brand `toolchain` | the load leaves it out, as it leaves out its own output |

### Migration

1. The tests of the four adapters and of the kit call `RunToolchainSuite` with an adapter and a
   fixture.
2. Every caller of `AssertVets` drops the adapter.
3. The reference test checks the surface of its Go and TypeScript output against goldens under
   `testdata/surface`, and requires a doc comment on each of their public declarations.

## Alternatives

- **A method of the adapter for each check.** Each language would render its surface and check its
  documentation with its own tools: `go doc`, the declaration files that `tsc` writes, `javap` and
  the lints of `javac` and `rustc`. Each language would render a format of its own, a golden would
  change with the version of each tool, and each check would skip on a machine without the
  toolchain. Go and TypeScript have no stock check of documentation. We render from the model,
  which every frontend fills in the same way.
- **A golden for each generated file.** One golden for each file would show a declaration that
  moves between files as a removal and an addition in two goldens. One golden for each fixture shows
  it as no change, and keeps the number of goldens at one for each language of a fixture.
- **The plan export as the surface.** The export lists names and spellings, and its document is a
  persisted format. A signature in the export would change that format for every consumer of an
  export, to serve a test.
- **A documentation rule for each language.** Go requires doc comments on exported declarations by
  convention, Java and Rust lint public members as well, and TypeScript has no rule. One rule over
  the declarations at file level covers what every convention requires, and a generator documents
  its members in the same template.

## Drawbacks

- `core/toolchain` gains six imports: the load, the plugin package, the node package, the store,
  the output package and the diagnostics package. The SDK does not re-export the load, so the kit
  uses the load as an implementation dependency alone, as the plugin kit does.
- The surface is as accurate as the frontend of its language. A frontend that drops a member drops
  it from the surface too. The conformance corpus of each language checks what its frontend loads.
- The receiver form of a Go method is a stamp and not a field of the model, so the surface does not
  show a change of a method from a value receiver to a pointer receiver.
- Eight test files change their calls of the suite and of the vet check, and the reference test
  gains two goldens.

## Open questions

None.

## Unresolved and future work

- This proposal does not check generated Java or Rust output, because no fixture of the repository
  generates Java or Rust output that a toolchain test reads. The checks take the frontend of either
  language unchanged.

## References

- [RFC-0033: The shared mechanisms of the test kits](0033-the-shared-mechanisms-of-the-test-kits.md)
- [Testing and conformance](../architecture/13-testing-and-conformance.md)
- [Languages](../architecture/11-languages.md)
- [The api tool of the Go project](https://github.com/golang/go/tree/master/src/cmd/api)
- [API Extractor](https://api-extractor.com/)
