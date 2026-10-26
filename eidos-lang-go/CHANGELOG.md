# go.dokimi.dev/eidos/lang/go

## 0.5.0

### Minor Changes

- 8dd6045: Implement `rules.ValueImportRules` in the Go rules through `golang.CheckReference`.

### Patch Changes

- 5db1e37: Fix the render of a Go value to refuse an unexported callee of another package.
- Updated dependencies [899b981, 8d1cae6, 729462f]
  - go.dokimi.dev/eidos/core@0.9.0
  - go.dokimi.dev/eidos/sdk@0.7.0
  - go.dokimi.dev/eidos/lang@0.3.2

## 0.4.0

### Minor Changes

- 25ad6fd: Add `golang.CheckReference` for the declarations that Go code of a package can refer to.
- 1163699: Add `golang.PackageBindings` for the scope of the imports of the files of a Go package.
- 25ad6fd: Declare `spell.Importable` as the import rule of the Go backend.
- 25ad6fd: Implement `rules.ImportRules` in the Go rules.
- f0b7d82: Load each unexported type whose members an exported type promotes in a signature-only Go parse.
- 1163699: Record the uses of a Go directive argument through the imports of every file of its package.
- 1163699: Resolve a bare name of a Go directive argument in the external test package of its directory.
- 25ad6fd: Resolve a bare name of a directive argument in the external test package of the route of its subject.
- 1163699: Resolve a qualifier of a Go directive argument through the imports of the other files of its package.

### Patch Changes

- 1163699: Allocate each list of the files of a parsed Go package once.
- 3203b3d: Fix the resolution in the external test package of a route to derive its path from slash paths on every system.
- 1163699: Fix `golang.AssumedName` to allocate nothing for a last element such as `vol`.
- 0fb5a7e: Fix the receiver of a generated Go method to refer to the struct of its own file.
- 25ad6fd: Fix the Go backend to refuse the import of an external test package.
- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- 2265178: Fix the samples and the zero value of a defined Go array, slice or map type to use a composite literal of the defined type.
- Updated dependencies [f2467a9, 0ccd4f4, b5864dc, 7559a1a, 53b3580, 6f125b4, 1833bae, 4702c35, 86dbe80]
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/sdk@0.6.0
  - go.dokimi.dev/eidos/lang@0.3.1

## 0.3.0

### Minor Changes

- 8716d09: Add `backend.Format` to format a Go file as gofumpt and golines format it.
- 8716d09: Add the plan policy `golang.width` to choose the width at which the backend wraps a line of code.
- 8716d09: **Breaking:** Format each rendered Go file with gofumpt and then with golines at the width of the plan.
- 8716d09: **Breaking:** Load a Go dependency package only where a declaration or a directive of the load uses it.
- 8716d09: Record a use for each declaration outside a Go file that a named type or a directive argument of the file can resolve to.
- 8716d09: Refuse to format a Go file while the environment sets `GOFUMPT_SPLIT_LONG_LINES=on`.

### Patch Changes

- e75c0ca: Declare Go 1.27.3 in the scratch module that the conformance adapter of `testing` lays out.
- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, 141d240, e1de752, 8c07b86, 529673e, 217c870, 7dfcd57, d0ab2c4, 6dc24ee, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/sdk@0.5.0
  - go.dokimi.dev/eidos/lang@0.3.0

## 0.2.3

### Patch Changes

- a92d68d: Fix the dependency round to pass over an import of a package whose directory contains an output of the brand.
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

- 8e14f81: Document the package of an emitted reference in `backend.Speller.Spell`.
- Updated dependencies [b9677b7, e14100f, 575bf2a, 9621f52, 61f41a7, 7494b32, e64dff0]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/sdk@0.3.0
  - go.dokimi.dev/eidos/lang@0.2.1

## 0.2.0

### Minor Changes

- c0ecc95: Add `RenamePredeclared` to rename each parameter and result whose name Go predeclares.
- bca12c4: Add `spell.Handler` and `spell.Home`, the binder of grpc-go and connect-go.
- c0ecc95: Add `Speller.TypeArgs` and the template helpers `spellvalue` and `typeargs`.
- 21875ef: Add the policy `golang.grpc`, under whose choices `grpc-go` and `connect-go` a reference to a protobuf declaration spells as protoc-gen-go names it.
- 21875ef: Derive the samples of a function type, a channel and the empty interface in `Rules.DeriveSamples` of the Go rules.
- c0ecc95: Derive `struct{}{}` as the one value of the empty struct, and the samples of a map of empty structs from its keys.
- bca12c4: Mark a reference to a protobuf declaration bound under the choices `grpc-go` and `connect-go`.
- bca12c4: Reject the fact `Overload`.
- 21875ef: Resolve a name that a full import path qualifies in `Rules.Resolve` and `Rules.LiteralFor` of the Go rules.
- bca12c4: Spell a type parameter by its name at every depth of a type.
- 21875ef: Spell a function value as a function literal and a make value as a call of `make` in the Go backend.
- 21875ef: Spell the well-known context as `context.Context`.
- afb1411: Refuse the fact `Receives` and the annotations of a file, and render the methods of a struct under the fact `Methods`.
- 21875ef: Add the verdicts of the facts `Wire` and `Passing` to the coverage table of the Go backend.
- 21875ef: Add the template helper `wrapdocs` and `backend.WrapDocs` to the Go backend.
- 21875ef: Write each test file of an external family into the external test package of its directory.
- 21875ef: Add `backend.Value` to write a value of the value tree for the builtin `value`.
- bca12c4: Add `spell.Split` to write the internal tests of an external family into an internal test file.

### Patch Changes

- c0ecc95: Refuse a construction value, which Go builds through a composite literal or a call.
- 21875ef: Refuse the deleter of a property in the Go lowering.
- 21875ef: Cover the refusal of the deleter of a property in the spec of `backend.Lower`.
- 66c5fe9: Fix the Go spoke to spell an optional of an interface as the interface.
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

- 17a7a8a: Add `golang.Qualifier`, and import a package whose path ends in a keyword, such as `example.com/go`, under an escaped name such as `go_`.
- 2e6ca0a: Attach a directive carrier on a Go parameter or a result to each name of its field.
- 786a060: Add `testing.AssertParallel`, which requires every generated Go test and subtest to call `Parallel`.
- 4ffc162: Add `rules.Bound.Satisfies`, `rules.Bound.Implementers` and `rules.SatisfactionRules`, which the Go rules implement.
- 786a060: Add `backend.TemplateImport` to each backend, which binds the name of an import that a template states.
- 2e6ca0a: Derive the samples of a generic type at its type arguments, the samples and the zero of a complex number, and the zero of an inline type, and fold `interface{}` as `any`.
- 786a060: **Breaking:** Rename `Rules.SamplesOf` to `DeriveSamples`, and remove `Rules.Substitute`.
- d7c134f: Add `rules.WellKnownRules`, and fold a resolved `time.Time` and `time.Duration` to their well-known identities.
- 2e6ca0a: **Breaking:** Fold `error` to `rules.WellKnownError`, state the error contract of a Go type, replace `IsSentinelName` with `SentinelBase`, and export `MethodSet` and `NullaryString`.
- 786a060: Add the template helpers `tag`, which joins the tag entries of a field and refuses an entry that `reflect.StructTag` cannot read back, and `embeds`.
- 786a060: Lower an enum to its underlying type, and bound its out-of-range value by that type.
- 4ffc162: List the types of an inline type body as the children of its reference, so a backend imports the names inside the body.
- 2e6ca0a: Lower a Go union constraint such as `~int | time.Duration` to a `Union` of its terms, and import the package of each term.
- 2e6ca0a: Add `golang.HiddenFieldsKey`, which the frontend stamps on a struct whose lowering left out a blank or an unexported field.
- 7ed5ea5: Name the Go annotator `golang-facts`, and add `golang.Lookup`, so one composition contains the annotator and the Go backend.
- 786a060: **Breaking:** Rename `golang.Scope` and `NewScope` to `Bindings` and `NewBindings`, and the key constants to `ReceiverIsPointerKey` and `UnderlyingKindKey`.
- 2e6ca0a: Return the value form, the texts without the type name, the zero and an out-of-range value from the underlying type of an enumeration in `EnumOf`.
- 2e6ca0a: Promote a defined type to an enumeration over the typed constants of every file of its package, a type over `bool` or another defined type included.
- 2e6ca0a: **Breaking:** Read a constant, a variable or a call of a function without arguments in the `rules.Scope` of `Rules.LiteralFor`.
- 786a060: **Breaking:** `Speller.Params` and `Speller.Results` take the slot of a declaration by address, as a template passes `.Params` and `.Returns`.
- 2e6ca0a: **Breaking:** Return an error from `Directives` and `MemberDirectives` for an assigned annotation.
- 786a060: Refuse an immutable list, array and map in the Go spoke.
- 786a060: Reject async, override, a default body, a type level, a computed name and the mutability of an index signature in the Go coverage.
- a54b3c8: **Breaking:** Replace the indentation argument of `docs` and `directives` with the helpers `memberdocs` and `memberdirectives`, which write at the depth of a member.
- 786a060: Add `golang.Restate`, which the spoke and the backend share to write the Go syntax of a composite type.
- 786a060: **Breaking:** Return `backend.Backend` of the kit from `backend.New`.
- d7c134f: **Breaking:** Run `toolchain.RunToolchainSuite` over an adapter and a fixture as values, remove `toolchain.Setup`, and remove the adapter parameter of `gotesting.AssertVets`.
- 786a060: Add `testing.Language`, the spelling adapter of the fixture builder in `nodetest`.
- 4ffc162: Write the build constraint of a Go source file above the package clause of each file that a generator derives from it alone.

### Patch Changes

- 786a060: Derive int for an empty interface bound in a view without facts, which reads no type set.
- 2e6ca0a: Refuse a Go import whose path is not valid or whose `internal` tree does not contain the file, and a list in which some parameters have names and others do not.
- 2e6ca0a: Type the samples, the zero and a literal of an enumeration by its underlying type, so an enumeration over `bool` or a fractional float gets values that Go accepts.
- 2e6ca0a: Derive the samples and the zero of a resolved `time.Time` and `time.Duration`, and read the comparability table for a resolved standard library type outside the view.
- ade234c: Find GOROOT two levels above the go binary on PATH before the frontend follows the link of the binary.
- 2e6ca0a: Import a package under the name that its package clause declares, and write the name of an import that differs from that name.
- 2e6ca0a: Open the doc of each constant of a lowered enumeration with the name of the constant.
- 7ed5ea5: Stamp `golang.satisfiesError` and `golang.satisfiesStringer` on a type whose method returns an alias of `string`.
- 2e6ca0a: Report a struct with hidden fields as comparable only from a table of standard library types, and an inline struct from its recorded fields.
- 2e6ca0a: Keep the open module of `vendor/modules.txt` across a short module line and keep the highest version of each path.
- 2e6ca0a: Read the `// +build` lines of a Go file without a `//go:build` line, as go/build reads them.
- 786a060: Refuse a satisfaction claim whose type contains an import path before the build in `testing.New`.
- 786a060: Return an error for an absolute path of a fixture in the toolchain adapters.
- 786a060: Refuse in the Go backend's speller a channel and a variadic list whose spelling the substitution cleared.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- 786a060: Resolve a declaration that shadows a predeclared type before the predeclared type.
- 2e6ca0a: Resolve the variant of an enumeration as a package variable, as Go declares it a constant.
- 786a060: Show the source of the probe in the error of a satisfaction probe that fails for a reason other than the contract.
- 2e6ca0a: Write a Go conversion to a type that starts with `*`, `<-` or `func(` with the type in parentheses, such as `(*T)(nil)`.
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
