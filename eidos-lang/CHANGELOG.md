# go.dokimi.dev/eidos/lang

## 0.2.1

### Patch Changes

- e64dff0: Return the package that an emitted reference records from `spellref.PackageOf`.
- Updated dependencies [b9677b7, e14100f, 575bf2a, 9621f52, 61f41a7, 7494b32]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/sdk@0.3.0

## 0.2.0

### Minor Changes

- c53b949: **Breaking:** Add `scaffold.Target.Func` and `scaffold.Target.Make` to spell a function value and a make.
- c53b949: Add `lowering.DeleteAccessor`, which refuses the deleter of a property in a target without properties.
- cac3c9b: Add `naming.Params` to name each parameter of a method that has no name.
- cac3c9b: Add `spellref.TypeArgs` to write type parameters or type references as the type arguments of a reference.
- c53b949: Add `textfmt.Wrap`, `textfmt.Markup` and `textfmt.DocsWidth` to wrap documentation without a block opener at the start of a line.
- cac3c9b: **Breaking:** Add `scaffold.Target.Construction`, which spells a construction of a type from the spellings of its arguments.
- c53b949: Add the package `treesitter/python` with the grammar of the fork `dokimasia/tree-sitter-python`.
- 4ee3b36: Name the shape of a type parameter "a type parameter" in `spoke.Describe`.
- 4ee3b36: Add `lowering.GroupOverloads` to put the overload signatures of a method name before its implementation.
- 4ee3b36: Add `spoke.Param` to spell the shape of a type parameter by its name.

### Patch Changes

- 4ce5b02: Fix `treesitter/typescript` to accept `abstract` and `unique` as names and keywords as tuple labels.
- c55b40a: Fix `treesitter/typescript` to apply `readonly` to the type before a union or an intersection.
- cac3c9b: Fix `treesitter.Comments.Leading` to end a run at a comment that another subject took.
- 4ce5b02: Fix `treesitter/typescript` to insert a semicolon before `<` at a line break after a type.
- c53b949: Fix `naming.Params` to name a parameter called `_` by its position.
- 4ce5b02: Fix `treesitter/typescript` to parse a bare `&` in JSX text and in a JSX string.
- c55b40a: Fix `treesitter/typescript` to parse a readonly type and an infer type after `keyof`.
- 4ce5b02: Fix `treesitter/typescript` to parse an import type wherever a primary type can appear.
- 4ce5b02: Fix `treesitter/typescript` to parse `typeof import()` in the type arguments of a call.
- cac3c9b: Fix `scaffold.Value` to pass a structural type without a spelling to the target, which restates it.
- c55b40a: Require the fork `github.com/dokimasia/tree-sitter-typescript` at its commit 839d520b in place of `github.com/tree-sitter/tree-sitter-typescript` v0.23.2.
- Updated dependencies [eed0fb8, d5e68cb, 2a85f93, 09edb8b, 78909e0, 62d7b18, 6741030, c051021, 3561c92, 4520c2f, 097c95f, 32afe4b, 840a721, 19f2a4e]
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/sdk@0.2.0

## 0.1.1

### Patch Changes

- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.
- Updated dependencies [a2d0d64]
- Updated dependencies [a2d0d64]
- Updated dependencies [4644a2b]
  - go.dokimi.dev/eidos/core@0.2.0
  - go.dokimi.dev/eidos/sdk@0.1.1

## 0.1.0

### Minor Changes

- d3c0bda: Add `treesitter.Comments`, `Tree.Problems` and the node methods `FirstOf`, `HasToken`, `Content`, `LastNamed` and `Bytes`.
- 8126b00: Add `naming.Unique`, which returns the first candidate that a set of names does not contain.
- d7c134f: Clear the spelling of a composite that `rules.Substitute` restates, and restate it in `spellref.SpellRestated` or refuse it in `spellref.SpellWith`.
- 8126b00: Refuse in `lowering.Callables` a callable with failures whose body has verbatim text, statements or a return statement in a slot.
- d3c0bda: Add `spoke.ReadOnly`, the refusal of an immutable shape in a language that writes its form as writable.
- d3c0bda: Add `lowering.ParamArgs`, `lowering.NamedPayload`, `lowering.Callables`, `spoke.Arity`, `spoke.Split` and `spellref.SpellRestated`.
- 8126b00: Spell a name value through `Target.Callee` in `scaffold.Value`.
- ebb8f4b: Spell an `emit.ExprRef` in `scaffold.Expr` through the `Callee` of the target, which qualifies the name and records the import.
- 8126b00: Add `textfmt.MarkedInline` and write an assigned marker with its argument after an equals sign.

### Patch Changes

- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- ad48d4f: Spell a reference to a type parameter by its bare name, so a generated file imports nothing for it.
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
- Updated dependencies [d7c134f]
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
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [9ff596a]
- Updated dependencies [4ffc162]
  - go.dokimi.dev/eidos/core@0.1.0
  - go.dokimi.dev/eidos/sdk@0.1.0
