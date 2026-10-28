# go.dokimi.dev/eidos/sdk

## 0.7.2

### Patch Changes

- 0348694: Add `plugin.NewDependencyUnit` to the facade.
- 83dc1f9: Update `go.dokimi.dev/assert` to v0.10.0 and every other dependency to its latest release.
- Updated dependencies [71f1a16, 83dc1f9]
  - go.dokimi.dev/eidos/core@0.11.0

## 0.7.1

### Patch Changes

- c342cf5: Copy the documentation of the unexported methods that the signature depth loads.
- Updated dependencies [4f50e1a]
  - go.dokimi.dev/eidos/core@0.10.0

## 0.7.0

### Minor Changes

- 729462f: Add `rules.ValueImportRules` to the SDK.

### Patch Changes

- Updated dependencies [899b981, 8d1cae6]
  - go.dokimi.dev/eidos/core@0.9.0

## 0.6.0

### Minor Changes

- 1833bae: **Breaking:** Key `plugin.Unit.Data` of the SDK by the record that `Out.Record` placed.
- 7559a1a: **Breaking:** Pass the plugin of the directive to `directive.Resolver` of the SDK.
- 7559a1a: Add `plugin.ImportChecker`, `plugin.ImportRule` and `rules.ImportRules` to the SDK.
- 4702c35: Add `plugin.Placer` and `plugin.Placed` to the SDK.

### Patch Changes

- b5864dc: Copy the documentation of the gaps of an alias and of the signature depth.
- 7559a1a: Copy the documentation of `rules.Scope.Route` and of the import rule of `backend.Backend` into the SDK.
- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- Updated dependencies [f2467a9, 0ccd4f4, 53b3580, 6f125b4, 86dbe80]
  - go.dokimi.dev/eidos/core@0.8.0

## 0.5.0

### Minor Changes

- 141d240: Add `Builder.ProvidesIn` to provide a capability in one role of a plugin with both roles.
- 141d240: **Breaking:** Pass the role to `plugin.CapabilityProvider.Provides`, which returns the capabilities of that role.
- 7dfcd57: Re-export `plugin.ErrUnclaimedNamespace`.

### Patch Changes

- 7dfcd57: Copy the documentation of an unbound gate into `HasKey` and `plugin.KeyBinder`.
- d0ab2c4: Copy the documentation of the two formatters of a language into the facade of `render`.
- 6dc24ee: Copy the documentation of the names of records into `plugin.NamesOf` and `plugin.Settle`.
- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, e1de752, 8c07b86, 217c870, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0

## 0.4.0

### Minor Changes

- 12e54f3: Re-export `directive.Cut` and `directive.Locate` in the facade.

### Patch Changes

- 6902587: Copy the documentation of the outputs of a dependency round into the facades of `plugin` and `frontendtest`.
- Updated dependencies [6294c29, 3d0b252]
  - go.dokimi.dev/eidos/core@0.6.0

## 0.3.1

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0

## 0.3.0

### Minor Changes

- 7494b32: Re-export `Subjects` in the facade.

### Patch Changes

- 9621f52: Regenerate the facade for the documentation of `emit.TypeRef.Emitted` and `plugin.Emit.Translates`.
- 61f41a7: Regenerate the facade for the documentation of the uses of a unit and of a dependency round.
- Updated dependencies [b9677b7, e14100f, 575bf2a]
  - go.dokimi.dev/eidos/core@0.4.0

## 0.2.0

### Minor Changes

- d5e68cb: Add `Emitter.FileOf` to key a per-source file on a declaration of the package of the subject.
- 09edb8b: Add `rules.Mentioned` to return the type parameters that references mention.
- d5e68cb: Add `rules.RefusedAuthored` for an authored text that the rules of its language do not read as a boolean or a number.
- 09edb8b: Add `TemplatesOf` to declare the template tree of a target from the templates that a plugin embeds.
- d5e68cb: Add the template builtin `value`, `render.Language.Value` and `backend.Builder.Value` to write a value of the value tree.
- 09edb8b: Add `emit.Construction` and the value kind `emit.ValueConstruction`, which constructs a type from argument values.
- d5e68cb: Add the function and make values `emit.Func` and `emit.Make`, with `emit.Value.Fresh`.
- d5e68cb: Add `[]symbol.Identity` to the fact vocabulary and to the `values` of the `meta` directive.
- 09edb8b: Add the shared helper names `render.HelperSpellValue` and `render.HelperTypeArgs`.
- 09edb8b: Add `rules.ParamView.Optional`, which reports whether a call can leave the parameter out.
- 09edb8b: Add `OnPackage` and `PackageMatch` to handle each package and each directive on a package.
- 09edb8b: Add `Reporter` and `SubjectMatch`, which every match with a subject satisfies.
- d5e68cb: Add `OnEnumVariant` to run a rule on each variant of an enumeration.
- d5e68cb: Add the shared helper name `render.HelperWrapDocs`.
- d5e68cb: Admit an interface as the subject of the keys `gen.sample` and `gen.alternate`.
- d5e68cb: Pass a pair of fresh values in `rulestest.AssertDistinctSamples`.
- c051021: Re-export `plugin.Binder`, `plugin.RPC`, `plugin.Translate`, `plugin.CanonicalHandler` and `RefusedHandler`.
- c051021: Re-export the documentation of `Implements` on `Enum` and `Sum`.
- 3561c92: Re-export `plugin.HeaderOf`, `emit.ReceivesOf`, `rules.ShapeRules`, `render.BuiltinHeader` and `symbol.FactReceives`.
- c051021: Re-export `FactOverload`, `IsImplementation`, `Evaluated` and `UnimplementedOverload`.
- 4520c2f: Re-export `rules.ResultRules`, `rules.TextRules`, `rules.ContextRules` and `rules.WellKnownContext`.
- 4520c2f: Re-export `NameScoper`, `SupertypeSpeller`, `Overloader`, `MemberRespeller`, `Passing`, `AccessorDelete`, the facts `Wire` and `Passing`, and `rulestest.LoadedWith`.
- 32afe4b: **Breaking:** Remove `rules.GapComputed` from the facade.

### Patch Changes

- 62d7b18: Document the dependency packages of `store.Graph` and of `plugintest.Loaded` in the facade.
- d5e68cb: Fix `rules.Complete` to keep a refused stated half.
- 840a721: Reword the docblock of `plugintest.Loaded` in the facade.
- d5e68cb: Fix `backendtest.Files` to route a file the way a plan's layout routes it, with the package that the backend's packager derives.
- Updated dependencies [eed0fb8, d5e68cb, 2a85f93, 09edb8b, 78909e0, 6741030, 097c95f, 19f2a4e]
  - go.dokimi.dev/eidos/core@0.3.0

## 0.1.1

### Patch Changes

- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.
- Updated dependencies [a2d0d64]
- Updated dependencies [a2d0d64]
- Updated dependencies [4644a2b]
  - go.dokimi.dev/eidos/core@0.2.0

## 0.1.0

### Minor Changes

- 8a44142: Add `plugin.Output.Template`, `Out.Record` and `render.MixedClaim`, so a file template writes each file of a family from the records of the family.
- 79553b5: Add `MirrorTypeParams` and `TypeArgs`, which copy the type parameters of a generic declaration onto a declaration that a plugin derives from it.
- 4ffc162: Add `node.CloneRef`, which copies a type reference and every node below it.
- 4ffc162: Add `RenderContext.Names`, `RenderedFile.Packages` and `ImportSet.Declared` for the declared names of the packages of a run, and render a file again after an edit renames a package whose name the file read.
- d7c134f: Add `Optional`, `Mutability` and `Computed` to a method, `Computed` to a field, `Mutability` and `Absent` to a type reference, and `Underlying` to an enum.
- d7c134f: Add `plugin.Composite`, `plugin.FuncType`, `plugin.Zip`, `plugin.Absent` and `plugin.UserHomeVar`.
- d7c134f: Add `toolchain.Run`, `toolchain.RunTimeout`, `toolchain.LookPaths`, `toolchain.Scratch`, `toolchain.WriteFile`, `toolchain.Sources` and `toolchain.Probe`.
- 4ffc162: Add `emit.ValueName` and `emit.Name` for the name of a declaration as a value.
- d7c134f: Add `Bound.Tag`, `Bound.SentinelName` and `Bound.IsSentinelName`, which report false for a language without the capability.
- 81c58aa: Add `emit.ExprRef` and `emit.RefExpr`, a scaffolding expression that refers to a function, a constant or a variable of another package.
- 4ffc162: Add `plugin.OptionsSchema`, which returns the JSON Schema of the options of a plugin in a config file.
- 2bac013: Add `Schema.Kinds`, which `Builder.Build` derives from the rules, and report `MisplacedDirective` for a directive on a declaration of another kind.
- d7c134f: Add `frontend.TestFiles`, `frontend.Nearest` and `frontend.Builder.Imports`.
- e28d437: Add `TagTest`, the well-known tag of a test companion.
- d7c134f: **Breaking:** Return the bound plugin from `plugin.KeyBinder.BindKeys`, and leave the receiver unchanged.
- d7c134f: Add `ImportSet.Use` and `Language.TemplateImport`, so the `use` builtin binds the name of a template's import in the name assignment of the file.
- d7c134f: **Breaking:** Replace `Builder.Build` with `Generator`, `Annotator` and `Dual`, which panic where the rules do not fit the role.
- d7c134f: Add `nodetest`, whose `Builder.Build` loads the declarations of a test through the kernel's load.
- d7c134f: Add `plugintest.AssertCleanRun`, `plugintest.AssertEmptyRun` and `backendtest.AssertEmptyRender`, and run them in the suites.
- d7c134f: Add `diag.Diag.Positioned`, and fail a finding without a file in every kit through it.
- d7c134f: Add `toolchain.AssertDocumented`, which requires a doc comment on each public declaration at file level of generated output.
- d7c134f: Add `toolchain.Surface` and `toolchain.AssertSurface`, which render the public surface of generated output and match it against a golden file.
- d7c134f: Add `meta.LoadClaim` and `frontendtest.Registry`, and file the stamps of a load through them in the workspace and the kits.
- 4ffc162: Add `meta.KeySpec.Choices`, refuse a `meta set` of a value outside the choices of its key, and register each key of a lowering policy with its choices.
- 4ffc162: Add `rules.Bound.Satisfies`, `rules.Bound.Implementers` and `rules.SatisfactionRules`, which the Go rules implement.
- d7c134f: Compare every declaration of a plugin across repeated reads and two builds in `plugintest.AssertStableDeclaration`.
- 4ffc162: Give a directive schema without a plugin the name of the plugin that `Builder` lowers.
- d7c134f: Add `plugin.ContractVersion`, and run a sealed state of another contract version cold.
- d7c134f: **Breaking:** Remove `node.TypeRef.ID`. `TypeRef.Identity` returns the zero identity.
- d7c134f: **Breaking:** Embed `plugin.Versioned` in `plugin.Frontend`.
- d7c134f: Add `rules.WellKnownRules`, and fold a resolved `time.Time` and `time.Duration` to their well-known identities.
- d7c134f: **Breaking:** Fold `Lock` into `ledger.Ledger`, and refuse a composition with a ledger and no output at Build.
- d7c134f: **Breaking:** Fold `Overwrite` into `output.Sink`, and remove `output.Overwriter`.
- d7c134f: Keep the mutability of a reference in `rules.TypeShape`, and copy the asynchrony and the mutability of a reference in `rules.EmitRef`.
- 8a44142: **Breaking:** Add the file templates of a tree as the fourth parameter of `render.Pass.Lint`.
- d7c134f: Add `plugintest.Source` and `plugintest.Loaded`, which load a plugin fixture from a source tree.
- d7c134f: Add `frontendtest.Fixture.Empty` and `frontendtest.AssertEmptyLoad`, which loads a tree whose files declare nothing.
- d7c134f: **Breaking:** Make `Params`, `Returns`, `Embeds` and the new `Field.Tags` emit slots, replace `Field.Tag` with `TagEntry` entries, and add `emit.SlotOf`.
- 4ffc162: Add `workspace.PlanConfig.Plugins`, `PlanConfig.Families` and `layout.Refinement.Over` to merge the refinements of a config over the declared ones.
- d7c134f: Copy the optional mark, the accessor, the index and construct forms, the hard privacy, the mutability and the computed mark of a method in `Mirror`.
- d7c134f: **Breaking:** Rename `Errorf`, `Warnf` and `Infof` of `GraphMatch` to `ErrorfAt`, `WarnfAt` and `InfofAt`.
- d7c134f: **Breaking:** Rename the `Bucket` fields of `meta.Claim`, the phase contexts, `workspace.Component` and `plugintest.Fixture` to `Turn`, and print the field `turn` in `plan` and `explain`.
- 4ffc162: Add `Not`, `NameEquals` and `HasSlot` to the predicates of a gate.
- d7c134f: **Breaking:** Add `After` to `plugin.CapabilityProvider` and `Builder`, to order a plugin after a capability without requiring the capability.
- 4ffc162: **Breaking:** Take the struct tag of the keys in `jsonschema.Of`, which `jsonschema.TagYAML` and `jsonschema.TagJSON` name.
- 4ffc162: Add `node.DeclsOf`, `node.DeclNamed` and `node.MemberNamed`, which query declarations by their Go type and name.
- 4ffc162: **Breaking:** Pass a `rules.Scope` to `LiteralFor`, add `View.ScopeOf`, and lift an authored text of a type without a literal through `LiteralFor`.
- d7c134f: **Breaking:** Make the fields of `rules.View` unexported, and add `rules.NewView`, `View.Reader` and `View.Kernel`.
- 4ffc162: Add `symbol.Annotation.Assigned` for an annotation that assigns its argument and raise the format of the sealed state to 4.
- 4ffc162: Add `Modules` to `node.Import` and `node.Export`, the packages that a specifier names where a configuration of the workspace maps it.
- 4ffc162: Add `plugin.Route` and `plugin.RouteInput` and set the route of each unit that `layout.Route` places.
- d7c134f: Add `meta.KeySpec.Derived` and `DerivedMetadataKey`, and register the summary keys of the shape catalog as derived.
- d7c134f: Add `SourceUnit.RefuseCarriers`, `SourceUnit.RefuseSugar`, `SourceUnit.ReportSyntax` and `GraphBuilder.StampModule`.
- d7c134f: Add `render.Coverage.RefusesEmbeds` and the names of the shared vocabulary helpers, from `render.HelperDocs` to `render.HelperResults`.
- d7c134f: Add the verdict `render.Rejects` and the finding `RejectedFact`, which leave out a declaration that the target cannot render with a stated fact or without it.
- d7c134f: **Breaking:** Rename `SamplesOf` of `rules.SourceRules` to `DeriveSamples`.
- d7c134f: Report a computed member of the member walk as the gap `rules.GapComputed`.
- 4ffc162: Accept a sample directive that states the alternate alone, and report `EmptySample` and `EmptyWitness` for a sample or a witness directive that states nothing.
- d7c134f: Report a template helper that text/template cannot call under `render.UnbindableHelper` instead of a panic.
- 4ffc162: Add `plugin.MissingOptions` for the option tag `schema:"required"` and fail Build for a required option at its zero value.
- d7c134f: **Breaking:** Add the methods that a plan calls to `plugin.Backend`, and return `backend.Backend` with `render.Coverer` from the backend kit.
- d7c134f: **Breaking:** Add `rules.Resolve`, `rules.Resolver`, `rules.ResolveError` and `rules.Unaliased`, and remove `TypeName` from `rules.SourceRules` and `rules.Bound`.
- d7c134f: Add `rules.View.WithRules`, so a binding that a rules implementation makes resolves the rules of every language.
- 4ffc162: **Breaking:** Replace `IsSentinelName` of `rules.ErrorValueRules` and `rules.Bound` with `SentinelBase`, which returns the base of a sentinel name.
- d7c134f: Add `backendtest.Rendered`, `backendtest.Fixture.Context` and `rulestest.Fixture.Findings`.
- 4ffc162: Add `emit.NameOf`, which returns the name of an emitted declaration.
- d7c134f: **Breaking:** Run `toolchain.RunToolchainSuite` over an adapter and a fixture as values, remove `toolchain.Setup`, and remove the adapter parameter of `gotesting.AssertVets`.
- d7c134f: **Breaking:** Seal the slots of every value in the store of a plan until the settle, so a direct `Slot.Append` panics there, and add `Slot.Replace`, `emit.AppendSealed` and `emit.Seal`.
- 4ffc162: Add `meta set=<key>` with `value` or `values`, `directive.ParamSpec.Default`, the nearest registered name in `UnclaimedName`, and `meta.Registry.Type`.
- 4ffc162: Add `rules.ErrorTypeRules`, `rules.ErrorContract`, `Bound.ErrorContract` and the well-known identity `rules.WellKnownError`.
- 4ffc162: Add `output.Stream`, which writes the files that its sinks commit to one writer.
- d7c134f: **Breaking:** Add `rules.Substitute`, and remove `Substitute` from `rules.GenericsRules`.
- d7c134f: Add `Emitter.Fields`, which translates the fields of a declaration or reports that a type has no spelling.
- 4ffc162: Give a `time.Duration` the JSON types string and integer, and a type that unmarshals its text the type string, in `jsonschema.Of`.
- 4ffc162: Write the build constraint of a Go source file above the package clause of each file that a generator derives from it alone.

### Patch Changes

- d7c134f: Fail the test on a panic of a phase call in `plugintest.Fixture.Annotate` and `Fixture.Generate`.
- 1f081a5: Gate all three checks of `RunToolchainSuite` on the toolchain, and return the error of a `tsc` that does not run from the TypeScript `Parse`.
- d7c134f: List each generated file with its line numbers in the contract of a failed toolchain assertion.
- d7c134f: Run the loads and parses of `frontendtest` under the context of the test.
- d7c134f: Run the load of `rulestest.Loaded` under the context of its TB.
- 4ffc162: Give a field whose tag is `-,` the key `-` in `jsonschema.Of`, as the YAML and the JSON decoders do.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- d7c134f: Skip each check of a role or a surface that a plugin lacks in `plugintest.RunPluginSuite`, with the reason.
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
