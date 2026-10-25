# go.dokimi.dev/eidos/core

## 0.8.0

### Minor Changes

- f2467a9: Add `plugin.ImportChecker` and `backend.Builder.ImportRule` for the import rule of a target.
- f2467a9: Add `rules.ImportRules` and `Bound.Importable` for the declarations that the code of a package can refer to.
- 0ccd4f4: Add `Out.Package` to return the package of the routed file of a handle before the handler records.
- 0ccd4f4: Add `plugin.Placer`, `plugin.Placed` and `plugin.GeneratorContext.Placer` for the preview of a route.
- 0ccd4f4: Add `layout.NewPlacer` to preview the routes of the layout of one plan.
- 0ccd4f4: Add `plugin.Invocation.Placed` for the previews of routes that an invocation made.
- f2467a9: Add `rules.Scope.Route` and `layout.RouteDir` for the directory of the route of the subject of a directive.
- 0ccd4f4: Give each generator call of a plan that writes files the placer of the layout of the plan.
- 6f125b4: **Breaking:** Key `plugin.Unit.Data` by the record that `Out.Record` placed.
- f2467a9: **Breaking:** Pass the plugin of the directive to `directive.Resolver`.
- 0ccd4f4: Preview the routes of the layout of a `plugintest.Fixture` through a spoke with both halves of a target.
- 0ccd4f4: Record the routing edges of each preview of an invocation in its read record.
- f2467a9: Refuse a declaration whose routed file cannot import one of its references under `layout.UnimportableReference`.

### Patch Changes

- 53b3580: Fix `Bound.MembersOf` to promote the members of a contributor whose declaration is an alias.
- 6f125b4: Fix `Out.Record` to keep the data of each record where two records of a unit have one origin.
- 0ccd4f4: Name a declaration of the graph by the name of its identity in `layout.Describe`.
- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.

## 0.7.0

### Minor Changes

- 9c9c4bb: Add `Builder.ProvidesIn` to provide a capability in one role of a plugin with both roles.
- e1de752: Add `plugin.ErrUnclaimedNamespace` for a gate on a key of an unclaimed namespace.
- 8c07b86: Add `render.Language.FinaliseUnder`, a formatter that receives the policy of the plan.
- e1de752: Bind a plugin of `plugintest.Fixture` in the composition of the plugins of its source.
- 8c07b86: Add `backend.Builder.FinaliseUnder` to declare a formatter that receives the policy of the plan.
- e1de752: Leave a gate on a key of an absent owner unbound in a plugin that runs after a capability without a provider.
- 217c870: **Breaking:** List the names of records in `plugin.NamesOf`, each marked with the new field `NameEntry.Record`.
- 217c870: List the records of a scope in `plugin.Names.InScope`, and none in `InPackage`, `OfOrigin` or `InFileScope`.
- 9c9c4bb: Order the members of each role by the capabilities that their plugins provide in that role.
- 9c9c4bb: **Breaking:** Pass the role to `plugin.CapabilityProvider.Provides`, which returns the capabilities of that role.

### Patch Changes

- 217c870: Change the format of the state to 8, so the first run after the update is cold.
- 217c870: Fix a cold run to leave a bare reference to the name of a record as written, as a warm run does.
- 217c870: Fix a warm run to qualify a reference with a target into the kept file of a record, as a cold run does.
- 217c870: Fix a warm run to refuse a reference into a record that the run no longer emits, as a cold run does.
- e75c0ca: Require Go 1.27.3.

## 0.6.0

### Minor Changes

- 6294c29: Add `directive.Cut` to cut a payload before the first argument that opens another directive.
- 6294c29: Add `directive.Locate` to map an offset of a joined payload to its line.
- 3d0b252: Add `plugin.DependencyRound.Outputs` with the outputs that the selection of the frontend claims.
- 3d0b252: Add `load.DoorRecord.Outputs` with the outputs that the first dependency round passed.
- 3d0b252: Pass over a need of the directory of an output in the round of `frontendtest.ScriptedDependent`.
- 6294c29: Read each mark of the brand after a blank in a carrier line as the start of another directive.

### Patch Changes

- 3d0b252: Change the format of the state to 7, so the first load after the update is cold.

## 0.5.0

### Minor Changes

- 3f4bf95: Order the generator half of a plugin with both roles after a generator whose capability it requires or runs after.

## 0.4.0

### Minor Changes

- b9677b7: Add `directive.Name.Bare` to return a spelling without its plugin prefix.
- b9677b7: Add `SubjectMatch.Directives` to read the instances of a directive of the plugin on the subject.
- e14100f: Add `emit.TypeRef.Emitted` to mark a reference to a declaration that the plan emits.
- b9677b7: Add `store.ReadSet.RecordKey` and `store.ReadSet.Keys` for the membership edge on a key.
- b9677b7: Add `workspace.ReadKey` for an enumeration of the subjects of a key in an explanation.
- b9677b7: Add `Subjects` to enumerate the subjects of a key under a membership edge on the key.
- 575bf2a: Add the statement `use PATH NAME` to the scripted frontend of `frontendtest`, whose dependent places the file of the use.
- 575bf2a: Add `plugin.DependencyRound.Uses` with the uses of the loaded units that the round passes for the first time.
- 575bf2a: Add `load.DoorRecord.Uses` with the uses that a dependency round passed.
- 575bf2a: Add `store.Region.Uses` with the uses that the parse of a unit recorded.
- 575bf2a: Add `plugin.SourceUnit.Use` and `plugin.SourceUnit.Uses` to record the declarations outside the workspace that a unit uses.
- 575bf2a: Add `load.UnitRecord.Uses` with the uses of the region of a unit.
- e14100f: Add `layout.UnemittedReference` for an emitted reference to a declaration that the plan does not emit.
- e14100f: Mark each bare reference that `layout.Route` qualifies into another package as emitted.
- e14100f: Qualify an emitted reference in `layout.Route` by the package of the file of its referent.
- b9677b7: Register the schema of a `Directive` wrapper around no rules, which gates nothing.
- e14100f: Report a store with an emitted reference through `plugin.Emit.Translates`.
- 575bf2a: Run a dependency round after the first when the loaded units have a use that no earlier round passed.
- b9677b7: Run an enumeration of the subjects of a key again in a warm run when a subject gains or loses the key.

### Patch Changes

- 575bf2a: Change the format of the state to 6, so the first load after the update is cold.

## 0.3.0

### Minor Changes

- eed0fb8: Add `store.Graph.AddDependencyPackage` to add a package of a dependency round to a graph that `store.New` returns.
- d5e68cb: Add `rules.ContextRules` and `rules.WellKnownContext`, and put the context of a callable first among the parameters of its view.
- d5e68cb: Add `Emitter.FileOf` to key a per-source file on a declaration of the package of the subject.
- 2a85f93: Add `Emitter.Handler`, which reports an rpc that the target cannot serve under `RefusedHandler`.
- d5e68cb: Add `Emitter.Supertype`, `plugin.SupertypeSpeller` and `backend.Builder.Supertypes` to spell a supertype of another language apart from a value.
- 2a85f93: Add `Emitter.TypeParams` to translate the type parameters of a declaration with each bound as a supertype.
- d5e68cb: Add `render.ImportSet.Body` for the callable and the host of the body under render.
- d5e68cb: Add `render.ImportSet.Enter` and `ImportSet.Pending` to tell a reference to a declaration that the file writes later.
- d5e68cb: Add `render.ImportSet.Has` to report an entry that the set records.
- d5e68cb: Add `render.ImportSet.SetPolicy` and `ImportSet.Policy` for the policy of the plan under render.
- 2a85f93: Add `rules.IsImplementation` to tell the implementation of a method from its overload signatures.
- d5e68cb: Add `load.Changes.Dependency` for the packages that move between the partition and a dependency round.
- d5e68cb: **Breaking:** Add `plugin.Names.InFileScope` for the kept names of one routed scope.
- d5e68cb: Add `rules.ParamView.Passing` for the way a call passes the argument of a parameter.
- d5e68cb: Add `plugin.Index.Dependency` to report a subject of a dependency package.
- d5e68cb: Add `plugin.MemberRespeller` and `backend.Builder.RespellMember` to respell a member by its level and its mutability.
- d5e68cb: Add `plugin.NameScoper` and `backend.Builder.NameScopes` to check the file-level names of a file in the namespace of its target.
- d5e68cb: Add `plugin.Output.External` for a family whose files test their package from outside.
- d5e68cb: Add `plugin.Overloader` and `backend.Builder.Overloads` to let the overloads of a function share its name in a target that overloads.
- d5e68cb: Add `plugin.Placement.External` to pass the external mark of the family of a file to its target.
- d5e68cb: Add `plugin.Unit.Subjects` to map the origin of a declaration that a match placed for another subject to that subject.
- d5e68cb: Add `rules.ResultRules`, which `Bound.CallableOf` asks to split a return of a result type into the views of its value and its failure.
- 09edb8b: Add `rules.Mentioned` to return the type parameters that references mention.
- d5e68cb: Add `rules.RefusedAuthored` for an authored text that the rules of its language do not read as a boolean or a number.
- d5e68cb: Add `rulestest.LoadedWith` to load a fixture through a frontend with the trees of its stores.
- 78909e0: Add the optional capability `rules.ShapeRules`, which the fold asks after `WellKnownRules`.
- d5e68cb: Add `store.Graph.Dependency` to report a package that a dependency round loaded.
- d5e68cb: Add `store.RegionInfo.Dependency` to mark the region of a unit of a dependency round.
- 09edb8b: Add `TemplatesOf` to declare the template tree of a target from the templates that a plugin embeds.
- d5e68cb: Add `rules.TextRules`, which `View.Authored` asks for the value of an authored text of a text type.
- 2a85f93: Add the role `plugin.Binder` with `plugin.RPC`, `plugin.Translate` and `plugin.CanonicalHandler`.
- 2a85f93: Add `emit.TypeRef.Bound` to mark a reference to code that a gRPC binding generates.
- d5e68cb: Add the template builtin `value`, `render.Language.Value` and `backend.Builder.Value` to write a value of the value tree.
- 09edb8b: Add `emit.Construction` and the value kind `emit.ValueConstruction`, which constructs a type from argument values.
- 2a85f93: Add `Implements` to `Enum` and `Sum` on both sides of the model, under the fact `Implements`.
- d5e68cb: Add `symbol.AccessorDelete` for the deleter of a property.
- 78909e0: Add `Param.Doc` for the documentation of a parameter.
- 78909e0: **Breaking:** Add `Field.Receives` with the fact `Receives`, and state the fact `Methods` on a struct, so a backend states a verdict for both.
- d5e68cb: Add the function and make values `emit.Func` and `emit.Make`, with `emit.Value.Fresh`.
- 2a85f93: Add `plugintest.Fixture.Layout`, the input of the layout of a plan over a fixture.
- d5e68cb: Add `[]symbol.Identity` to the fact vocabulary and to the `values` of the `meta` directive.
- 2a85f93: **Breaking:** Add `Function.Overload` and `Method.Overload` with the fact `Overload`, so a backend states a verdict for it.
- 09edb8b: Add the shared helper names `render.HelperSpellValue` and `render.HelperTypeArgs`.
- 09edb8b: Add `rules.ParamView.Optional`, which reports whether a call can leave the parameter out.
- d5e68cb: Add `symbol.Passing`, `Param.Passing` and the fact `Passing` for the way a caller passes an argument.
- d5e68cb: Add `PolicySpec.Plan`, `RenderContext.Policy` and `Policy.Lookup` for a policy that applies to every file of a plan.
- 78909e0: Add `plugin.File.Referents` and the methods `SetPackage`, `Package`, `SetReferents` and `Referent` of `render.ImportSet`.
- 09edb8b: Add `OnPackage` and `PackageMatch` to handle each package and each directive on a package.
- 09edb8b: Add `Reporter` and `SubjectMatch`, which every match with a subject satisfies.
- d5e68cb: Add `OnEnumVariant` to run a rule on each variant of an enumeration.
- d5e68cb: Add `emit.Field.Wire` and the fact `Wire` to keep the name of a key of a wire format as its generator wrote it.
- d5e68cb: Add the shared helper name `render.HelperWrapDocs`.
- d5e68cb: Admit an interface as the subject of the keys `gen.sample` and `gen.alternate`.
- d5e68cb: **Breaking:** Check the collisions of file-level names in `layout.Route`, and return the names in `layout.Routes`.
- 2a85f93: Add `backend.Builder.Binder` to declare the binder of a backend.
- 2a85f93: Add `Builder.Evaluated` and `Builder.Implemented` to the backend kit.
- d5e68cb: Take the first type argument of a stream as the type of its values in the fold.
- d5e68cb: Fold a reference whose target is an identity of the well-known registry to its well-known reference.
- 2a85f93: Keep the identity of a type parameter in the `Ref` of its opaque shape.
- d5e68cb: Order the members of a type under `rules.ShadowLinearise` by its C3 linearization.
- 6741030: **Breaking:** Remove `rules.GapComputed` and list a field or a method with a computed name in the member walk.
- 2a85f93: List the contracts of an enum and a sum in the surface of a package.
- eed0fb8: Add `plugintest.Fixture.LoadDependencies` to add the packages of a store to a fixture.
- 2a85f93: Add `Out.Internal` and `plugin.Unit.Inside` to mark the declarations of an external family that test their package from inside.
- d5e68cb: Order the declarations of a unit by the source positions of their origins.
- 2a85f93: Add `render.Evaluated`, `Language.Evaluated` and `Language.Implemented` to order the declarations of a file.
- 2a85f93: **Breaking:** Pass a reference to a type parameter to the spoke of the target at every depth in `Emitter.Type` and `Emitter.Supertype`.
- d5e68cb: Pass a pair of fresh values in `rulestest.AssertDistinctSamples`.
- d5e68cb: **Breaking:** Pass the reference beside the declaration to `rules.ShapeRules.ShapeOf`.
- 2a85f93: Place a file with a declaration that tests from inside as a file that does not test from outside.
- 2a85f93: Qualify a bound reference by the package that `plugin.Binder.Home` returns.
- 2a85f93: Add the code `UnimplementedOverload` for overload signatures without an implementation and for a second implementation.
- 2a85f93: **Breaking:** Return the error of the layout from `backendtest.Fixture.Context`.
- 2a85f93: Add `backendtest.Fixture.Layout` to route the store of a fixture through the layout of a plan.
- 78909e0: Scope the name of a field with `Receives` to the type that receives it in the settle and the name table.
- d5e68cb: Spell the passing of each parameter, the type of a receiver, `_` for a parameter without a type and `del` for a deleter in the discriminator of an overload.
- 2a85f93: Start the discriminator of the implementation of an overloaded name with `=`.
- 78909e0: Make an `emit.File` in a unit the header of its routed file, with `plugin.File.Header`, `plugin.HeaderOf`, `plugin.Placement.Header` and the builtin `header`.
- 2a85f93: Follow the `Implements` of an enum and a sum in `rules.MembersOf` under `ContributesImplements`.
- d5e68cb: Write the passing of each parameter and the modifier `delete` in `toolchain.Surface`.

### Patch Changes

- d5e68cb: Fix `workspace.Sources` to admit the dependency packages of its language whatever its patterns and its module.
- d5e68cb: Fix `rules.Complete` to keep a refused stated half.
- 6741030: Fix the load to keep two overloads whose parameters spell alike and whose type parameters differ.
- eed0fb8: Fix `plugintest.Loaded` to add each dependency package of the load beside the packages that `Source.Packages` lists.
- d5e68cb: Fix `rules.View.Authored` to lift an authored text of every type through the `LiteralFor` of its language first.
- 097c95f: Fix `plugin.Emit.Add` to refuse a nil pointer of a declaration kind.
- d5e68cb: Fix a warm load to report each declaration of a unit that moves between the partition and a dependency round as changed.
- 19f2a4e: Reword the docblocks of `store.Graph.AddDependencyPackage`, `plugintest.Fixture.LoadDependencies` and `plugintest.Loaded`.
- d5e68cb: Fix `layout.Route` to route a declaration whose origin has no overrides with the overrides of the subject that placed it.
- d5e68cb: Fix `backendtest.Files` to route a file the way a plan's layout routes it, with the package that the backend's packager derives.
- d5e68cb: Fix the dispatch to skip each declaration of a dependency package in the generate phase.
- d5e68cb: Fix `output.Contract.Stamp` to write a Go file that gofmt leaves unchanged.

## 0.2.0

### Minor Changes

- a2d0d64: Accept an extensible plan without a generator.
- a2d0d64: Locate the stores of a frontend through `Input.Getenv` only when the tree has an input file of the frontend.

### Patch Changes

- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.

## 0.1.0

### Minor Changes

- 8a44142: Add `plugin.Output.Template`, `Out.Record` and `render.MixedClaim`, so a file template writes each file of a family from the records of the family.
- 8a44142: Add `workspace.Plan.Extensible` and `workspace.PlanConfig.Generators`, which add the generators of a config to an extensible plan.
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
- 4ffc162: Apply the path of the kernel `out` directive on a package to each declaration of the package.
- d7c134f: **Breaking:** Return the bound plugin from `plugin.KeyBinder.BindKeys`, and leave the receiver unchanged.
- d7c134f: Add `ImportSet.Use` and `Language.TemplateImport`, so the `use` builtin binds the name of a template's import in the name assignment of the file.
- d7c134f: **Breaking:** Replace `Builder.Build` with `Generator`, `Annotator` and `Dual`, which panic where the rules do not fit the role.
- d7c134f: Add `nodetest`, whose `Builder.Build` loads the declarations of a test through the kernel's load.
- d7c134f: Add `plugintest.AssertCleanRun`, `plugintest.AssertEmptyRun` and `backendtest.AssertEmptyRender`, and run them in the suites.
- d7c134f: Add `diag.Diag.Positioned`, and fail a finding without a file in every kit through it.
- d7c134f: Add `toolchain.AssertDocumented`, which requires a doc comment on each public declaration at file level of generated output.
- d7c134f: Add `toolchain.Surface` and `toolchain.AssertSurface`, which render the public surface of generated output and match it against a golden file.
- d7c134f: Add `meta.LoadClaim` and `frontendtest.Registry`, and file the stamps of a load through them in the workspace and the kits.
- d7c134f: Clear the spelling of a composite that `rules.Substitute` restates, and restate it in `spellref.SpellRestated` or refuse it in `spellref.SpellWith`.
- 4ffc162: Add `meta.KeySpec.Choices`, refuse a `meta set` of a value outside the choices of its key, and register each key of a lowering policy with its choices.
- 42fa23a: Add `ledger.GroupWriter`, and commit the files of `output.Disk` and the blobs of `ledger.Dir.WriteAll` as groups through one order barrier and one durability barrier.
- 4ffc162: Add `rules.Bound.Satisfies`, `rules.Bound.Implementers` and `rules.SatisfactionRules`, which the Go rules implement.
- d7c134f: Compare every declaration of a plugin across repeated reads and two builds in `plugintest.AssertStableDeclaration`.
- 4ffc162: Give a directive schema without a plugin the name of the plugin that `Builder` lowers.
- d7c134f: Add `plugin.ContractVersion`, and run a sealed state of another contract version cold.
- 4ffc162: Add `Description.Disabled`, and list the plans that the config disables in `plan` and their plugins in `version`.
- d7c134f: **Breaking:** Remove `node.TypeRef.ID`. `TypeRef.Identity` returns the zero identity.
- d7c134f: **Breaking:** Embed `plugin.Versioned` in `plugin.Frontend`.
- 8a44142: **Breaking:** Execute each reference template with `missingkey=error`, so a template that reads a key that its data does not contain fails under `RefusedTemplate`.
- 4ffc162: Add `Explanation.Declarations` and the event `explain.declaration` for the declarations that an identity or a key at a position names.
- 4ffc162: Record the route of each unit in the group record of format 3 and return it in `workspace.ExplainedFile.Routes`.
- d7c134f: Add `rules.WellKnownRules`, and fold a resolved `time.Time` and `time.Duration` to their well-known identities.
- 4ffc162: Fold an inline reference to the shape that the `Builtin` of its language returns, and keep it inline where the language returns `Opaque`.
- d7c134f: **Breaking:** Fold `Lock` into `ledger.Ledger`, and refuse a composition with a ledger and no output at Build.
- d7c134f: **Breaking:** Fold `Overwrite` into `output.Sink`, and remove `output.Overwriter`.
- d7c134f: Keep the mutability of a reference in `rules.TypeShape`, and copy the asynchrony and the mutability of a reference in `rules.EmitRef`.
- 8a44142: **Breaking:** Add the file templates of a tree as the fourth parameter of `render.Pass.Lint`.
- 4ffc162: Add `workspace.Builder.Plugins`, which lists the plugins of a composition without a build.
- d7c134f: Add `plugintest.Source` and `plugintest.Loaded`, which load a plugin fixture from a source tree.
- d7c134f: Add `frontendtest.Fixture.Empty` and `frontendtest.AssertEmptyLoad`, which loads a tree whose files declare nothing.
- d7c134f: **Breaking:** Make `Params`, `Returns`, `Embeds` and the new `Field.Tags` emit slots, replace `Field.Tag` with `TagEntry` entries, and add `emit.SlotOf`.
- 4ffc162: Add `workspace.PlanConfig.Plugins`, `PlanConfig.Families` and `layout.Refinement.Over` to merge the refinements of a config over the declared ones.
- d7c134f: Copy the optional mark, the accessor, the index and construct forms, the hard privacy, the mutability and the computed mark of a method in `Mirror`.
- d7c134f: **Breaking:** Rename `workspace.RecordKind`, its constants, `ExplainedRecord` and `Explanation.Records` to the `Execution` names, and the event `explain.record` to `explain.execution`.
- d7c134f: **Breaking:** Rename `Errorf`, `Warnf` and `Infof` of `GraphMatch` to `ErrorfAt`, `WarnfAt` and `InfofAt`.
- d7c134f: **Breaking:** Rename the `Bucket` fields of `meta.Claim`, the phase contexts, `workspace.Component` and `plugintest.Fixture` to `Turn`, and print the field `turn` in `plan` and `explain`.
- 4ffc162: Add `Not`, `NameEquals` and `HasSlot` to the predicates of a gate.
- d7c134f: **Breaking:** Add `After` to `plugin.CapabilityProvider` and `Builder`, to order a plugin after a capability without requiring the capability.
- 4ffc162: Parse the text of a `time.Duration` option and wrap the error of the decoder in the fault of an option value.
- 4ffc162: **Breaking:** Take the struct tag of the keys in `jsonschema.Of`, which `jsonschema.TagYAML` and `jsonschema.TagJSON` name.
- 4ffc162: Add `node.DeclsOf`, `node.DeclNamed` and `node.MemberNamed`, which query declarations by their Go type and name.
- 4ffc162: **Breaking:** Pass a `rules.Scope` to `LiteralFor`, add `View.ScopeOf`, and lift an authored text of a type without a literal through `LiteralFor`.
- d7c134f: **Breaking:** Make the fields of `rules.View` unexported, and add `rules.NewView`, `View.Reader` and `View.Kernel`.
- 4ffc162: Add `Workspace.Directives` and `Registry.Schemas` for the directive schemas of a composition.
- 4ffc162: Add `symbol.Annotation.Assigned` for an annotation that assigns its argument and raise the format of the sealed state to 4.
- 4ffc162: Add `Modules` to `node.Import` and `node.Export`, the packages that a specifier names where a configuration of the workspace maps it.
- 4ffc162: Add `plugin.Route` and `plugin.RouteInput` and set the route of each unit that `layout.Route` places.
- 4ffc162: **Breaking:** Report `MissingParam` for a `meta` instance that writes neither `drop` nor `set`, or both.
- d7c134f: Add `meta.KeySpec.Derived` and `DerivedMetadataKey`, and register the summary keys of the shape catalog as derived.
- d7c134f: Add `SourceUnit.RefuseCarriers`, `SourceUnit.RefuseSugar`, `SourceUnit.ReportSyntax` and `GraphBuilder.StampModule`.
- d7c134f: Add `render.Coverage.RefusesEmbeds` and the names of the shared vocabulary helpers, from `render.HelperDocs` to `render.HelperResults`.
- 4ffc162: Report a nil generator and a missing backend of a plan that the config disables in `workspace.Builder.Build`.
- d7c134f: Add the verdict `render.Rejects` and the finding `RejectedFact`, which leave out a declaration that the target cannot render with a stated fact or without it.
- d7c134f: **Breaking:** Rename `SamplesOf` of `rules.SourceRules` to `DeriveSamples`.
- d7c134f: Report a computed member of the member walk as the gap `rules.GapComputed`.
- d7c134f: Report a plugin that implements part of `plugin.CapabilityProvider` or `plugin.TemplateProvider` at Build.
- 4ffc162: Accept a sample directive that states the alternate alone, and report `EmptySample` and `EmptyWitness` for a sample or a witness directive that states nothing.
- d7c134f: Report a template helper that text/template cannot call under `render.UnbindableHelper` instead of a panic.
- 4ffc162: Report each required option at its zero value in `plugintest.AssertOptionsSchema`.
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
- d7c134f: **Breaking:** Return an error from `workspacetest.Fixture.Compose`, run `pipelinetest.RunPipelineSuite` over a `workspacetest.Fixture`, and remove `pipelinetest.Fixture`.
- d7c134f: Add `workspacetest.AssertRerunUnchanged`, `workspacetest.Want`, `workspacetest.AssertGolden` and `workspacetest.Generated`.
- 4ffc162: Add `rules.ErrorTypeRules`, `rules.ErrorContract`, `Bound.ErrorContract` and the well-known identity `rules.WellKnownError`.
- 4ffc162: Add `output.Stream`, which writes the files that its sinks commit to one writer.
- d7c134f: **Breaking:** Add `rules.Substitute`, and remove `Substitute` from `rules.GenericsRules`.
- d7c134f: Add `Emitter.Fields`, which translates the fields of a declaration or reports that a type has no spelling.
- 4ffc162: Give a `time.Duration` the JSON types string and integer, and a type that unmarshals its text the type string, in `jsonschema.Of`.
- 4ffc162: Write the build constraint of a Go source file above the package clause of each file that a generator derives from it alone.

### Patch Changes

- 354ba3d: Touch a file that a frontend claims in `workspacetest.AssertTouched`, and exempt the files of a claimed family from the export check of `workspacetest.AssertExported`.
- 69477f7: Refuse at workspace Build a family that a generator declares twice, or without a cardinality, a word or a template path inside a tree.
- 4ffc162: Copy the tree of the fixture into the directory of `workspacetest.AssertCycleRefused` before the build.
- 4ffc162: List in each `UnknownKey` refusal the keys that the instance accepts under its role.
- 5c2df17: Explain the declaration that the spelling of an identity names where its package path has a dot in its last element.
- d7c134f: Fail the test on a panic of a phase call in `plugintest.Fixture.Annotate` and `Fixture.Generate`.
- 1f081a5: Gate all three checks of `RunToolchainSuite` on the toolchain, and return the error of a `tsc` that does not run from the TypeScript `Parse`.
- 38284d3: Index the declarations of each package of a sealed `store.Graph` in a sorted slice, and build the index of every package on a pool before an enumeration of all of them.
- 4ffc162: Join every defect of a policy spec in the error of `plugin.NewPolicy`.
- 7646457: Lay out the files of `toolchain.Generated.Sources` beneath the generated files in `toolchain.Prepare`.
- d7c134f: List each generated file with its line numbers in the contract of a failed toolchain assertion.
- 4ffc162: List the types of an inline type body as the children of its reference, so a backend imports the names inside the body.
- d7c134f: Run the loads and parses of `frontendtest` under the context of the test.
- d7c134f: Run the load of `rulestest.Loaded` under the context of its TB.
- d7c134f: Rename the parameters of `Emitter.JoinName` to `tail` and `head`.
- 4fd2025: Commit the files of a package whose last path element contains a dot, such as a protobuf package, in a run with patterns.
- 4ffc162: Place a package that a frontend leaves without a position at the position of its first file.
- 4ffc162: Give a field whose tag is `-,` the key `-` in `jsonschema.Of`, as the YAML and the JSON decoders do.
- 85d4b42: Refuse a composition at Build where a plugin requires a capability whose provider runs after it.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- 1ce59e5: Render a nested declaration into a buffer that the render frame reuses for each depth.
- 60bf040: Run cold instead of panicking on a sealed state whose run index has a block outside its run.
- d7c134f: Run the loads, the runs and the record reads of the workspace and pipeline kits under the context of the test.
- 4ffc162: Leave a sealed directive registry as it is on a second `Registry.Seal`.
- d7c134f: Skip each check of a role or a surface that a plugin lacks in `plugintest.RunPluginSuite`, with the reason.
- 9ff596a: Write the blobs of `ledger.Dir` on Windows without a sync of their directory.
