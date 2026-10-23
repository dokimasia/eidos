# go.dokimi.dev/eidos/lang/typescript

## 0.4.1

### Patch Changes

- Updated dependencies [6f125b4, 1833bae]
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/sdk@0.6.0
  - go.dokimi.dev/eidos/lang@0.3.1

## 0.4.0

### Minor Changes

- 4c06021: Add the plan policy `typescript.quote` to select the quote of each string that the backend writes.
- 4c06021: Add `typescript.AppendQuote` and `typescript.Requote` to write the string literals of TypeScript.
- 4c06021: Derive the sample of a class as a construction through its first public constructor, with the fields that it leaves unset set through `Object.assign`.
- 4c06021: **Breaking:** Derive the sample of an interface as an object literal of each required property, and refuse an interface with a required method.
- 4c06021: **Breaking:** Move `PropKey`, `MethodKey` and `EnumKey` of the TypeScript backend to `Speller`.
- 4c06021: Write a TypeScript object literal with a space inside its braces.
- 4c06021: Spell a callee with an owner as the owner and its name, as `Object.assign`.
- 4c06021: **Breaking:** Spell a TypeScript type with the spaces that Biome and Prettier write on one line.
- 4c06021: **Breaking:** Write each string of a TypeScript file in the double quote by default.

### Patch Changes

- 4c06021: Parse the TypeScript benchmark corpus in 1,710,009 allocations instead of 2,180,003.
- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, 141d240, e1de752, 8c07b86, 529673e, 217c870, 7dfcd57, d0ab2c4, 6dc24ee, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/sdk@0.5.0
  - go.dokimi.dev/eidos/lang@0.3.0

## 0.3.2

### Patch Changes

- Updated dependencies [6294c29, 3d0b252, 6902587, 12e54f3]
  - go.dokimi.dev/eidos/core@0.6.0
  - go.dokimi.dev/eidos/sdk@0.4.0
  - go.dokimi.dev/eidos/lang@0.2.3

## 0.3.1

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0
  - go.dokimi.dev/eidos/lang@0.2.2
  - go.dokimi.dev/eidos/sdk@0.3.1

## 0.3.0

### Minor Changes

- 9ce9209: Add `library.Index` with the statements of the global names of each file of the embedded library.
- 9ce9209: Load one dependency unit for each global name of the TypeScript library that a load uses, in place of one unit for the whole library.
- 9ce9209: Serve the statements of one global name of a file of the library store at `<key>/<file>#<name>`.

### Patch Changes

- d302660: Fix the resolution of a bare name that an import binds to probe no global tier.
- 597952f: Import an emitted reference from the module path of its referent relative to the file.
- 9ce9209: Stop importing the extended library into a source file that a lib directive extends.
- Updated dependencies [b9677b7, e14100f, 575bf2a, 9621f52, 61f41a7, 7494b32, e64dff0]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/sdk@0.3.0
  - go.dokimi.dev/eidos/lang@0.2.1

## 0.2.0

### Minor Changes

- 1e22241: Add `spell.Handler` and `spell.Home`, the binder of Connect-ES.
- 1e22241: Add the policy `typescript.grpc`, under whose choice `connect-es` a reference to a protobuf declaration spells as protobuf-es names it.
- f9800cb: Add `TypeIterable` and `TypeErrorOptions` to the global types of TypeScript.
- 763fb69: Add `TypeReadonlySet` to the global types of TypeScript.
- 763fb69: Add `Speller.SpellValue`, `Speller.TypeArgs` and the template helpers `spellvalue` and `typeargs`.
- e4916b2: Add `typescript.LibraryStore`, `typescript.Library` and `typescript.Global` for the packages of the library of TypeScript.
- f9800cb: Add `typescript.Grouped` to report whether a child type needs parentheses inside a composite.
- e4916b2: Add `frontend.UnknownLibrary` for a library, a target or a lib directive that TypeScript 7.0.2 does not know.
- 763fb69: Attach the carriers of the header of a module to the package of the module.
- f9800cb: Return the constructor of a global error class from `Rules.Constructors` of the TypeScript rules.
- f9800cb: Derive the samples of `unknown`, `any` and the global error classes in `Rules.DeriveSamples` of the TypeScript rules.
- 763fb69: Derive the samples and the zero values of `Set`, `ReadonlySet`, `Map` and `ReadonlyMap` as constructions.
- 1e22241: Extend the contracts of a sum in each variant interface.
- 1e22241: Leave the implementation of an overloaded constructor out of the constructors of a class.
- 1e22241: Load the implementation of an overloaded function and of an overloaded class method.
- e4916b2: Load the library that the tsconfig of each project selects from the embedded declaration files of TypeScript 7.0.2.
- 1e22241: Mark the overload signatures of a function and of a class method with `Overload`.
- 1e22241: Write a class after the class that it extends and after the decorators that its file declares.
- e4916b2: Read a declaration of the library of TypeScript as a global in the rules.
- 1e22241: Refuse a class whose overloaded method name has no implementation or two.
- 1e22241: Refuse the contracts of an enum.
- e4916b2: Spell a reference to a declaration of the library of TypeScript without an import.
- 1e22241: Spell a type parameter by its name at every depth of a type.
- 763fb69: Spell a construction value as `new C<A>(args)` with a value import of the class.
- f9800cb: Spell the well-known context as `AbortSignal`.
- 763fb69: **Breaking:** Spell names without the initialisms of Go, so `getById` keeps its spelling and `httpRow` becomes `HttpRow`.
- 76c473b: Refuse the fact `Receives` and the annotations of a file, and render the methods of a class under the fact `Methods`.
- f9800cb: Add the verdicts of the facts `Wire` and `Passing` to the coverage table of the TypeScript backend.
- f9800cb: Add the template helper `wrapdocs` and `backend.WrapDocs` to the TypeScript backend.
- 1e22241: Write an overload signature without a body right before the implementation of its name.
- f9800cb: Write a function value as an arrow function in the TypeScript backend.
- f9800cb: Add `backend.Value` to write a value of the value tree for the builtin `value`.

### Patch Changes

- e4916b2: Fix `Rules.EnumOf` to derive a text outside a string enum that has a member with the empty text.
- f9800cb: Fix `Rules.DeriveSamples` of the TypeScript rules to construct an `Error` for the type `Error`.
- f9800cb: Fix `Speller.Params` to put the element of a rest parameter in parentheses where TypeScript needs them.
- df52360: Fix the frontend to load a readonly array or tuple before `|` or `&` as an immutable list or tuple.
- 1e22241: Fix `backend.Lower` to leave out the variance of the type parameters of functions, methods and aliases of types that are not object types.
- 763fb69: Fix the comments above `export default class {}` to document and annotate the class alone.
- f9800cb: Fix `backend.Imports` of TypeScript to mark each type-only specifier of a value import with `type`.
- e4916b2: Fix the merge of an interface to skip an equal method and a property of a name that the interface has.
- e4916b2: Fix `Rules.LiteralFor` to read the name of a constant or a variable and a call of a function in the scope of the declaration.
- e4916b2: Fix the backend to refuse a reference to a declaration of the library that a declaration of the file hides.
- f9800cb: Refuse a make value in the TypeScript backend.
- f9800cb: Refuse the deleter of a property in the TypeScript lowering.
- c55b40a: Require the fork `github.com/dokimasia/tree-sitter-typescript` at its commit 839d520b in place of `github.com/tree-sitter/tree-sitter-typescript` v0.23.2.
- e4916b2: Fix `spell.Name` to spell a field, a method and an enum variant whose name is a reserved word.
- Updated dependencies [4ce5b02, eed0fb8, d5e68cb, 2a85f93, c53b949, cac3c9b, 09edb8b, 78909e0, c55b40a, 4ee3b36, 62d7b18, 6741030, c051021, 3561c92, 4520c2f, 097c95f, 32afe4b, 840a721, 19f2a4e]
  - go.dokimi.dev/eidos/lang@0.2.0
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/sdk@0.2.0

## 0.1.1

### Patch Changes

- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.
- Updated dependencies [a2d0d64]
- Updated dependencies [a2d0d64]
- Updated dependencies [4644a2b]
  - go.dokimi.dev/eidos/core@0.2.0
  - go.dokimi.dev/eidos/lang@0.1.1
  - go.dokimi.dev/eidos/sdk@0.1.1

## 0.1.0

### Minor Changes

- 4924138: Set `noUnusedLocals`, `noUnusedParameters` and `exactOptionalPropertyTypes` in the default tsconfig of `testing.New`.
- 786a060: Add `backend.TemplateImport` to each backend, which binds the name of an import that a template states.
- 786a060: **Breaking:** Rename `Rules.SamplesOf` to `DeriveSamples`, and remove `Rules.Substitute`.
- 4ffc162: List the types of an inline type body as the children of its reference, so a backend imports the names inside the body.
- 4924138: Read the absent members of an optional for its zero value and its literals in place of its spelling.
- 786a060: **Breaking:** `Speller.Params` and `Speller.Results` take the slot of a declaration by address, as a template passes `.Params` and `.Returns`.
- 9d3c527: **Breaking:** Return an error from `Decorators` and `MemberDecorators` for an assigned annotation.
- 11c05a9: **Breaking:** Replace the indentation argument of `docs` and `decorators` with the helpers `memberdocs` and `memberdecorators`, which write at the depth of a member.
- 4924138: Report `ReadonlyRest` for a rest parameter of a readonly array.
- 9d3c527: Report `RelativeAugmentation` for a module augmentation of a relative module and load nothing from it.
- 9d3c527: Record the packages that a tsconfig maps a specifier onto, resolve a directive through them, and add `typescript.IndexModule`, `typescript.DefaultExport` and `typescript.Relative`.
- 4924138: Add `typescript.Restate`, and spell the references of the backend through `spellref.SpellRestated`.
- 786a060: **Breaking:** Return `backend.Backend` of the kit from `backend.New`.
- 4924138: **Breaking:** Remove `typescript.OptionalKey` and `typescript.ReadonlyKey`, and set the optional, readonly, computed and absent facts in the model.
- df3883d: Add `IsIdentifierStart` and `IsIdentifierPart`, and accept every ECMAScript identifier as a bare name in the settle, the backend and the rules.
- 4924138: Spell an immutable list, tuple and map as `readonly T[]`, `readonly [A, B]`, `Readonly<Record<K, V>>` and `ReadonlyMap<K, V>`.
- 786a060: Add `testing.Language`, the spelling adapter of the fixture builder in `nodetest`.
- 4924138: Stamp `gen.module` and `gen.moduleRoot` from the nearest `package.json`, and report one that does not read under BadPackage.
- 4924138: Add `GeneratedKey`, and stamp it on a file that a foreign generator wrote.
- 9d3c527: Stamp `typescript.abstract` and `typescript.conditionalType`, and read the comments of imports and exports.
- 9d3c527: Fold `Error` to `rules.WellKnownError`, state the error contract of a class that extends an error class, and add `ErrorClass` with the global error classes.
- 9d3c527: **Breaking:** Take a `rules.Scope` in `Rules.LiteralFor`.
- 4924138: Write an optional method, a readonly index signature, a computed key and a hard name with its `#`.

### Patch Changes

- 4924138: Attribute a comment to the declaration past a decorator or a visibility on the line of the comment.
- 9d3c527: Treat a member of a namespace in a script as public only where the namespace exports it.
- 1f081a5: Gate all three checks of `RunToolchainSuite` on the toolchain, and return the error of a `tsc` that does not run from the TypeScript `Parse`.
- 5d5eb8f: Refuse a class or an interface that declares a member name twice, unless the members overload an interface method or an abstract method.
- 5d5eb8f: Refuse a sum variant whose payload has an entry named `kind`, the discriminant of the lowered union.
- 786a060: Return an error for an absolute path of a fixture in the toolchain adapters.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- 9d3c527: Ignore `baseUrl` and `paths` where a tsconfig chain fails to load.
- 9d3c527: Return the empty object as the zero of a `Record` and of an object with one index signature.
- 786a060: Show the source of the probe in the error of a satisfaction probe that fails for a reason other than the contract.
- 4924138: Warn RefusedFact for an embed and its facts, and render the declaration without them.
- Updated dependencies [8a44142]
- Updated dependencies [8a44142]
- Updated dependencies [79553b5]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [81c58aa]
- Updated dependencies [4ffc162]
- Updated dependencies [2bac013]
- Updated dependencies [d7c134f]
- Updated dependencies [e28d437]
- Updated dependencies [4ffc162]
- Updated dependencies [d3c0bda]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [354ba3d]
- Updated dependencies [d7c134f]
- Updated dependencies [69477f7]
- Updated dependencies [d7c134f]
- Updated dependencies [8126b00]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [42fa23a]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [576db82]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [8a44142]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [5c2df17]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [1f081a5]
- Updated dependencies [38284d3]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [7646457]
- Updated dependencies [8a44142]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4fd2025]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [8126b00]
- Updated dependencies [d7c134f]
- Updated dependencies [d3c0bda]
- Updated dependencies [85d4b42]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [1ce59e5]
- Updated dependencies [60bf040]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d3c0bda]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [8126b00]
- Updated dependencies [ebb8f4b]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [ad48d4f]
- Updated dependencies [8126b00]
- Updated dependencies [9ff596a]
- Updated dependencies [4ffc162]
  - go.dokimi.dev/eidos/core@0.1.0
  - go.dokimi.dev/eidos/sdk@0.1.0
  - go.dokimi.dev/eidos/lang@0.1.0
