# go.dokimi.dev/eidos/lang/java

## 0.2.6

### Patch Changes

- Updated dependencies [899b981, 8d1cae6, 729462f]
  - go.dokimi.dev/eidos/core@0.9.0
  - go.dokimi.dev/eidos/sdk@0.7.0
  - go.dokimi.dev/eidos/lang@0.3.2

## 0.2.5

### Patch Changes

- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- Updated dependencies [f2467a9, 0ccd4f4, b5864dc, 7559a1a, 53b3580, 6f125b4, 1833bae, 4702c35, 86dbe80]
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/sdk@0.6.0
  - go.dokimi.dev/eidos/lang@0.3.1

## 0.2.4

### Patch Changes

- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, 141d240, e1de752, 8c07b86, 529673e, 217c870, 7dfcd57, d0ab2c4, 6dc24ee, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/sdk@0.5.0
  - go.dokimi.dev/eidos/lang@0.3.0

## 0.2.3

### Patch Changes

- 1d46c7f: Fix the dependency round to pass over a need of a package whose directory contains an output of the brand.
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

- b4c580b: Add `spell.Member` to keep the screaming snake case of a constant of a class.
- b2027eb: Add `Speller.TypeArgs` and the template helpers `spellvalue` and `typeargs`.
- ddf666f: Extend the contracts of a sum in the sealed interface that the sum lowers to.
- ddf666f: Read the interfaces of an enum in source and in a class file.
- b4c580b: Refuse the well-known context, for which Java has no standard type.
- ddf666f: Reject the fact `Overload`.
- b2027eb: Spell a construction value as a class instance creation, such as `new HashSet<Long>(List.of(1L))`.
- ddf666f: Spell a type parameter by its name at every depth of a type.
- b2027eb: **Breaking:** Spell names and file names without the initialisms of Go, so `getById` keeps its spelling and `httpRow` becomes `HttpRow`.
- 1352d04: Refuse the fact `Receives` and the annotations of a file.
- b4c580b: Add the verdicts of the facts `Wire` and `Passing` to the coverage table of the Java backend.
- b4c580b: Add the template helper `wrapdocs` and `backend.WrapDocs` to the Java backend.
- b4c580b: Write a function value as a lambda in the Java backend.
- b4c580b: Add `backend.Value` to write a value of the value tree for the builtin `value`.
- ddf666f: Write the contracts of an enum after `implements`.

### Patch Changes

- b4c580b: Refuse a make value in the Java backend.
- b4c580b: Refuse the deleter of a property in the Java lowering.
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
- 786a060: **Breaking:** `Speller.Params` and `Speller.Results` take the slot of a declaration by address, as a template passes `.Params` and `.Returns`.
- cdf6ced: **Breaking:** Return an error from `Annotate` and `MemberAnnotate` for an assigned annotation.
- 7c41fe7: Refuse an immutable list, array and map in the Java spoke.
- 54420d5: **Breaking:** Replace the indentation argument of `docs` and `annotate` with the helpers `memberdocs` and `memberannotate`, which write at the depth of a member.
- 786a060: **Breaking:** Return `backend.Backend` of the kit from `backend.New`.
- 786a060: Add `testing.Language`, the spelling adapter of the fixture builder in `nodetest`.
- cdf6ced: Spell `rules.WellKnownError` as `Exception`.
- 4924138: Add `GeneratedKey`, and stamp it on a file that a foreign generator wrote.
- cdf6ced: Write the annotations of a parameter before its type.

### Patch Changes

- cdf6ced: Fail the load of a release that no directory of ct.sym lists.
- cdf6ced: Keep the `Annotation` supertype of an interface that is not an annotation interface.
- f9af85e: Read the user's home directory from USERPROFILE on Windows for the Maven and Gradle stores.
- cdf6ced: Refuse the name of a field of no class for the reason that refuses a function of no class.
- 786a060: Return an error for an absolute path of a fixture in the toolchain adapters.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- 786a060: Show the source of the probe in the error of a satisfaction probe that fails for a reason other than the contract.
- cdf6ced: Leave the type annotations out of the spelling of a wildcard.
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
