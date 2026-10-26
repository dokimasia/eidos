# go.dokimi.dev/eidos/reference

## 0.1.9

### Patch Changes

- Updated dependencies [f2a676a]
  - go.dokimi.dev/eidos/lang/go@0.5.1

## 0.1.8

### Patch Changes

- Updated dependencies [899b981, 8dd6045, 8d1cae6, 729462f, 5db1e37]
  - go.dokimi.dev/eidos/core@0.9.0
  - go.dokimi.dev/eidos/lang/go@0.5.0
  - go.dokimi.dev/eidos/sdk@0.7.0
  - go.dokimi.dev/eidos/cli@0.3.1
  - go.dokimi.dev/eidos/declarative@0.1.8
  - go.dokimi.dev/eidos/lang@0.3.2
  - go.dokimi.dev/eidos/lang/typescript@0.4.2

## 0.1.7

### Patch Changes

- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- 8b056b3: Update `go.stealthscale.io/cli` to v0.7.0 and `go.stealthscale.io/cli/yaml` to v0.1.7.
- Updated dependencies [8b056b3, f2467a9, 0ccd4f4, 25ad6fd, 1163699, b5864dc, 7559a1a, 3203b3d, 53b3580, 0fb5a7e, 6f125b4, 1833bae, f0b7d82, 4702c35, 86dbe80, 2265178]
  - go.dokimi.dev/eidos/cli@0.3.0
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/lang/go@0.4.0
  - go.dokimi.dev/eidos/sdk@0.6.0
  - go.dokimi.dev/eidos/declarative@0.1.7
  - go.dokimi.dev/eidos/lang@0.3.1
  - go.dokimi.dev/eidos/lang/typescript@0.4.1

## 0.1.6

### Patch Changes

- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, 141d240, e1de752, 8716d09, 8c07b86, 4c06021, 529673e, 217c870, 7dfcd57, d0ab2c4, 6dc24ee, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/sdk@0.5.0
  - go.dokimi.dev/eidos/lang/go@0.3.0
  - go.dokimi.dev/eidos/lang/typescript@0.4.0
  - go.dokimi.dev/eidos/lang@0.3.0
  - go.dokimi.dev/eidos/cli@0.2.5
  - go.dokimi.dev/eidos/declarative@0.1.6

## 0.1.5

### Patch Changes

- Updated dependencies [6294c29, 3d0b252, 6902587, a92d68d, 12e54f3]
  - go.dokimi.dev/eidos/core@0.6.0
  - go.dokimi.dev/eidos/sdk@0.4.0
  - go.dokimi.dev/eidos/lang/go@0.2.3
  - go.dokimi.dev/eidos/cli@0.2.4
  - go.dokimi.dev/eidos/declarative@0.1.5
  - go.dokimi.dev/eidos/lang@0.2.3
  - go.dokimi.dev/eidos/lang/typescript@0.3.2

## 0.1.4

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0
  - go.dokimi.dev/eidos/cli@0.2.3
  - go.dokimi.dev/eidos/declarative@0.1.4
  - go.dokimi.dev/eidos/lang@0.2.2
  - go.dokimi.dev/eidos/lang/go@0.2.2
  - go.dokimi.dev/eidos/lang/typescript@0.3.1
  - go.dokimi.dev/eidos/sdk@0.3.1

## 0.1.3

### Patch Changes

- Updated dependencies [b9677b7, e14100f, 9ce9209, 575bf2a, 9621f52, 8e14f81, 61f41a7, d302660, 597952f, 7494b32, e64dff0]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/lang/typescript@0.3.0
  - go.dokimi.dev/eidos/sdk@0.3.0
  - go.dokimi.dev/eidos/lang/go@0.2.1
  - go.dokimi.dev/eidos/lang@0.2.1
  - go.dokimi.dev/eidos/cli@0.2.2
  - go.dokimi.dev/eidos/declarative@0.1.3

## 0.1.2

### Patch Changes

- bb34e68: Check the setter of a field of a map of empty structs in the tests that builder writes.
- bb34e68: Declare the template tree of each target through `sdk.TemplatesOf`, and remove `internal/tree`.
- bb34e68: Declare the type parameters of a call record of stubgen through `rules.Mentioned`, with each parameter that their bounds use.
- 00b8f57: Expect a blank line before the provenance trailer of each golden file.
- 00b8f57: Expect the declarations of each golden file in the order of the positions of their origins.
- c53b949: Fix `naming.Params` to name a parameter called `_` by its position.
- bb34e68: Name the parameters of a double through `naming.Params` and `golang.RenamePredeclared`.
- 2d5aa6c: Report a release of the ring that requires an earlier compatibility line of the kernel instead of testing it.
- c55b40a: Require the fork `github.com/dokimasia/tree-sitter-typescript` at its commit 839d520b in place of `github.com/tree-sitter/tree-sitter-typescript` v0.23.2.
- bb34e68: Write the type arguments of the templates of builder and stubgen through `typeargs`.
- Updated dependencies [4ce5b02, eed0fb8, d5e68cb, 2a85f93, c53b949, cac3c9b, c0ecc95, 09edb8b, 78909e0, bca12c4, 1e22241, 21875ef, f9800cb, 763fb69, e4916b2, c55b40a, 4ee3b36, 62d7b18, 6741030, df52360, c051021, 3561c92, 4520c2f, 097c95f, 32afe4b, 840a721, 19f2a4e, 66c5fe9, afb1411, 76c473b, ba70166]
  - go.dokimi.dev/eidos/lang@0.2.0
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/sdk@0.2.0
  - go.dokimi.dev/eidos/lang/go@0.2.0
  - go.dokimi.dev/eidos/lang/typescript@0.2.0
  - go.dokimi.dev/eidos/declarative@0.1.2
  - go.dokimi.dev/eidos/cli@0.2.1

## 0.1.1

### Patch Changes

- c6a4676: Run `eidos-reference` as an app of `go.stealthscale.io/cli`.
- 78613ca: Test each release of the ring from the tree of its tag against the kernel and the SDK facade of the checkout.
- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.
- Updated dependencies [a2d0d64]
- Updated dependencies [a2d0d64]
- Updated dependencies [a044621]
- Updated dependencies [a044621]
- Updated dependencies [a044621]
- Updated dependencies [4644a2b]
  - go.dokimi.dev/eidos/core@0.2.0
  - go.dokimi.dev/eidos/cli@0.2.0
  - go.dokimi.dev/eidos/declarative@0.1.1
  - go.dokimi.dev/eidos/lang@0.1.1
  - go.dokimi.dev/eidos/lang/go@0.1.1
  - go.dokimi.dev/eidos/lang/typescript@0.1.1
  - go.dokimi.dev/eidos/sdk@0.1.1

## 0.1.0

### Minor Changes

- c978bb3: Add the nineteen reference plugins, their composition in `Compose`, the binary `eidos-reference` and the program of the ring job.
- d789b21: Check each field of a generic struct's builder at the witnesses of its type parameters.
- 1f3e67e: Generate the mock and the stub of a generic interface with the type parameters of the interface, and test a generic stub at the witnesses of its type parameters.
- e3a7c99: **Breaking:** Remove `errorgen.Priority` and `auditgen.Priority`, and run auditgen after the capability of errorgen.
- e3a7c99: Read the default of a builder field as a constant, a variable or a call in the scope of its struct, such as `time.Second`.
- e3a7c99: Take the form and the texts of an enumeration from `rules.EnumRules` in `enum`, and report `NoProjection` and `UnknownText`.
- e3a7c99: Register the sample, the witness and the Go annotators in `Base`, and check the setters of a builder with the alternate sample, the value outside the variants of an enum and a sentinel that is never assigned.
- 1f3e67e: Report `REFERENCE-0004` for a constraint interface under `mock` or `stub`, and `STUBGEN-0001` for a method with type parameters under `stub`.
- e3a7c99: **Breaking:** Require the options `package` and `func` of authgen, metricgen, tracegen, errorgen and auditgen.
- d2e6b59: Report `REFERENCE-0005` and `REFERENCE-0006` where the options of a plugin name a package or a function that the workspace does not contain.
- d789b21: **Breaking:** Return `sdk.Dual` from `handlergen.New`, `repogen.New` and `sentinel.New`, and order `debugweaver` after the repository and mock capabilities without requiring them.

### Patch Changes

- 17a7a8a: Add `golang.Qualifier`, and import a package whose path ends in a keyword, such as `example.com/go`, under an escaped name such as `go_`.
- 5ab21a0: Report `HANDLERGEN-0001` for a method `Serve` whose parameters are not an `http.ResponseWriter` and an `*http.Request`.
- e3a7c99: Declare the default of the `methods` parameter of `enum` in its schema.
- 056e7db: Declare `TestTag` of builder, enum, mockgen, sentinel and stubgen as `sdk.TagTest`.
- e3a7c99: Gate the epilogue, chain and guard rules on the name and the slot of the emitted value, and trigger the weavers on emitted structs.
- e3a7c99: Leave a field whose default tag is empty without a declared default in defaults.
- cba6bf0: List the kinds of `audit`, so a run reports the directive on a declaration that is not a struct or an interface.
- e3a7c99: Give each unnamed parameter of a double a name that no named parameter or result of its method has.
- d789b21: Read the parameters and results of a declaration through their slots in the templates of the reference plugins.
- e3a7c99: Rename a parameter or a result of a double with the name of a predeclared identifier of Go, such as `append`.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- 25510de: Resolve a relative root of the canary ring against the working directory of the command.
- 7dacd04: Spell a value outside the variants of an enumeration in the underlying type of the enumeration, so a large unsigned value keeps its sign.
- 4822bb4: Stamp no `shapewriter.writer` on a type whose method `Write` takes a variadic parameter.
- e3a7c99: Take the cause of an error type from its error contract in `sentinel`.
- Updated dependencies [cd47b5c]
- Updated dependencies [8a44142]
- Updated dependencies [2d40a7e]
- Updated dependencies [8a44142]
- Updated dependencies [79553b5]
- Updated dependencies [4ffc162]
- Updated dependencies [17a7a8a]
- Updated dependencies [c045cd4]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [81c58aa]
- Updated dependencies [4ffc162]
- Updated dependencies [2d40a7e]
- Updated dependencies [2bac013]
- Updated dependencies [d7c134f]
- Updated dependencies [e28d437]
- Updated dependencies [786a060]
- Updated dependencies [d69491d]
- Updated dependencies [4ffc162]
- Updated dependencies [2e6ca0a]
- Updated dependencies [4924138]
- Updated dependencies [d3c0bda]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [a074cab]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [354ba3d]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [69477f7]
- Updated dependencies [2e6ca0a]
- Updated dependencies [2d40a7e]
- Updated dependencies [d7c134f]
- Updated dependencies [4924138]
- Updated dependencies [8126b00]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [42fa23a]
- Updated dependencies [4ffc162]
- Updated dependencies [2d40a7e]
- Updated dependencies [d7c134f]
- Updated dependencies [d69491d]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [2e6ca0a]
- Updated dependencies [2e6ca0a]
- Updated dependencies [2e6ca0a]
- Updated dependencies [786a060]
- Updated dependencies [4ffc162]
- Updated dependencies [576db82]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [8a44142]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [5c2df17]
- Updated dependencies [9d3c527]
- Updated dependencies [d7c134f]
- Updated dependencies [2d40a7e]
- Updated dependencies [ade234c]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [2e6ca0a]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [1f081a5]
- Updated dependencies [2e6ca0a]
- Updated dependencies [38284d3]
- Updated dependencies [4ffc162]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [786a060]
- Updated dependencies [7646457]
- Updated dependencies [8a44142]
- Updated dependencies [2d40a7e]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [96cb289]
- Updated dependencies [2e6ca0a]
- Updated dependencies [d7c134f]
- Updated dependencies [2e6ca0a]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [2d40a7e]
- Updated dependencies [1915c6e]
- Updated dependencies [d7c134f]
- Updated dependencies [7ed5ea5]
- Updated dependencies [786a060]
- Updated dependencies [d69491d]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4fd2025]
- Updated dependencies [4ffc162]
- Updated dependencies [2e6ca0a]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [2d40a7e]
- Updated dependencies [2d40a7e]
- Updated dependencies [d69491d]
- Updated dependencies [2d40a7e]
- Updated dependencies [2e6ca0a]
- Updated dependencies [2e6ca0a]
- Updated dependencies [7ed5ea5]
- Updated dependencies [2e6ca0a]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [2e6ca0a]
- Updated dependencies [2e6ca0a]
- Updated dependencies [012e4b8]
- Updated dependencies [4924138]
- Updated dependencies [4ffc162]
- Updated dependencies [2d40a7e]
- Updated dependencies [4ffc162]
- Updated dependencies [2e6ca0a]
- Updated dependencies [786a060]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [2d40a7e]
- Updated dependencies [4ffc162]
- Updated dependencies [8126b00]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [5d5eb8f]
- Updated dependencies [5d5eb8f]
- Updated dependencies [d3c0bda]
- Updated dependencies [85d4b42]
- Updated dependencies [786a060]
- Updated dependencies [9d3c527]
- Updated dependencies [2e6ca0a]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [2d40a7e]
- Updated dependencies [a54b3c8]
- Updated dependencies [11c05a9]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4924138]
- Updated dependencies [9d3c527]
- Updated dependencies [f4aa524]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [9d3c527]
- Updated dependencies [9d3c527]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [2e6ca0a]
- Updated dependencies [d7c134f]
- Updated dependencies [4924138]
- Updated dependencies [786a060]
- Updated dependencies [c7f0b83]
- Updated dependencies [4ffc162]
- Updated dependencies [9d3c527]
- Updated dependencies [d7c134f]
- Updated dependencies [786a060]
- Updated dependencies [4ffc162]
- Updated dependencies [1ce59e5]
- Updated dependencies [60bf040]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [2d40a7e]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4924138]
- Updated dependencies [d7c134f]
- Updated dependencies [d3c0bda]
- Updated dependencies [d7c134f]
- Updated dependencies [786a060]
- Updated dependencies [d7c134f]
- Updated dependencies [8126b00]
- Updated dependencies [ebb8f4b]
- Updated dependencies [df3883d]
- Updated dependencies [4924138]
- Updated dependencies [786a060]
- Updated dependencies [4924138]
- Updated dependencies [4924138]
- Updated dependencies [9d3c527]
- Updated dependencies [56e2119]
- Updated dependencies [012e4b8]
- Updated dependencies [9d3c527]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [9d3c527]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4924138]
- Updated dependencies [2e6ca0a]
- Updated dependencies [ad48d4f]
- Updated dependencies [8126b00]
- Updated dependencies [9ff596a]
- Updated dependencies [4ffc162]
- Updated dependencies [4924138]
  - go.dokimi.dev/eidos/cli@0.1.0
  - go.dokimi.dev/eidos/core@0.1.0
  - go.dokimi.dev/eidos/sdk@0.1.0
  - go.dokimi.dev/eidos/lang/go@0.1.0
  - go.dokimi.dev/eidos/declarative@0.1.0
  - go.dokimi.dev/eidos/lang/typescript@0.1.0
  - go.dokimi.dev/eidos/lang@0.1.0
