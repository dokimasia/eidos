# go.dokimi.dev/eidos/lang/rust

## 0.2.3

### Patch Changes

- Updated dependencies [6294c29, 3d0b252, 6902587, 12e54f3]
  - go.dokimi.dev/eidos/core@0.6.0
  - go.dokimi.dev/eidos/sdk@0.4.0
  - go.dokimi.dev/eidos/lang@0.2.3

## 0.2.2

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0
  - go.dokimi.dev/eidos/lang@0.2.2
  - go.dokimi.dev/eidos/sdk@0.3.1

## 0.2.1

### Patch Changes

- Updated dependencies [b9677b7, e14100f, 575bf2a, 9621f52, 61f41a7, 7494b32, e64dff0]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/sdk@0.3.0
  - go.dokimi.dev/eidos/lang@0.2.1

## 0.2.0

### Minor Changes

- 85f0e91: Add `spell.Supertype` to spell a supertrait of another language as its bare name.
- 4340d2b: Add `Speller.SpellValue`, `Speller.TypeArgs` and the template helpers `spellvalue` and `typeargs`.
- 85f0e91: Add `lang/rust/rules`, the projection rules of Rust.
- c392627: Fold the trait of a trait impl onto an enum and a data enum.
- 85f0e91: Keep the type of an associated type binding, as the `T` of `Item = T`, as the argument of its reference.
- c392627: Read the trait impls of an enum and a data enum in `ErrorContract`, `Comparable` and `ZeroValue`.
- 85f0e91: Refuse the well-known context, for which Rust has no standard type.
- c392627: Reject the fact `Overload`.
- 85f0e91: Resolve `Self` to the type of its impl block, and lower a receiver `&self` as a borrow of `Self`.
- c392627: Spell a type parameter by its name at every depth of a type.
- 85f0e91: Spell a value of a trait of another language as the trait object `Box<dyn Trait>`.
- 4340d2b: **Breaking:** Spell type names without the initialisms of Go, so `Uuid` keeps its spelling and `httpRow` becomes `HttpRow`.
- a301097: Refuse the fact `Receives` and the annotations of a file, and render the methods of a struct in its inherent impl block.
- 85f0e91: Add the verdicts of the facts `Wire` and `Passing` to the coverage table of the Rust backend.
- 85f0e91: Add the template helper `wrapdocs` and `backend.WrapDocs` to the Rust backend.
- 85f0e91: Write a function value as a closure in the Rust backend.
- 85f0e91: Add `backend.Value` to write a value of the value tree for the builtin `value`.
- 85f0e91: Write a map, a tuple, a tuple struct, a variant and an associated function as Rust values.

### Patch Changes

- 85f0e91: Fix `backend.Speller` to use the module that the first segment of a path names, as `std::io` for `io::Error`, and no item for a path from a crate.
- 4340d2b: Refuse a construction value, which Rust builds through a struct literal or a call.
- 85f0e91: Refuse a make value in the Rust backend.
- 85f0e91: Refuse the deleter of a property in the Rust lowering.
- 85f0e91: Fix `Satisfies` of the Rust toolchain adapter to report a directory or a crate root that it cannot read before it runs cargo.
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

- 786a060: Add `backend.TemplateImport` to each backend, which binds the name of an import that a template states.
- c1cb7b4: Set the mutability of a borrow and the underlying type of an enum with an integer repr, and write `#[repr(T)]` for an underlying type.
- 786a060: **Breaking:** `Speller.Params` and `Speller.Results` take the slot of a declaration by address, as a template passes `.Params` and `.Returns`.
- 32e19b9: Record an attribute that assigns its value as an assigned annotation and write it back in that form.
- c1cb7b4: Refuse an immutable list, array, tuple and map in the Rust spoke.
- b4861f0: **Breaking:** Replace the indentation argument of `docs` and `attrs` with the helpers `memberdocs` and `memberattrs`, which write at the depth of a member.
- 786a060: **Breaking:** Return `backend.Backend` of the kit from `backend.New`.
- 786a060: Add `testing.Language`, the spelling adapter of the fixture builder in `nodetest`.
- 32e19b9: Spell `rules.WellKnownError` as `Box<dyn std::error::Error>`.
- 4924138: Add `GeneratedKey`, and stamp it on a file that a foreign generator wrote.
- 32e19b9: Write the attributes of a parameter and of a payload entry.

### Patch Changes

- 32e19b9: Leave the trait of a negative impl out of the Implements of its type.
- 32e19b9: Annotate the File node of a module with the attributes of its `mod` item.
- 4924138: Attribute a comment to the declaration past a decorator or a visibility on the line of the comment.
- 32e19b9: Leave a plain comment above an item out of its documentation.
- 32e19b9: Name the binary of `src/main.rs` after its package and the build script `<crate>/build_script_build`.
- 786a060: Return an error for an absolute path of a fixture in the toolchain adapters.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
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
