# Glossary

One line per term, for readers who start in the middle of the set.
The document named in each row carries the full contract.

| Term | Meaning | Doc |
|---|---|---|
| workspace | the composition frame: one read side, N plans, one merged manifest | [08](08-workspace-and-plans.md) |
| plan | one write side: generators, a layout, exactly one backend, one sink and a source scope. A value, not a plugin | [08](08-workspace-and-plans.md) |
| symbol | one declaration in the canonical model, with an identity that survives across runs | [02](02-symbol-model.md) |
| node / emit | the two generated models: what frontends produce, and what generators produce | [02](02-symbol-model.md) |
| freeze | the seal after Annotate. From then on the store refuses structural writes | [08](08-workspace-and-plans.md) |
| bag | a symbol's typed metadata container, `meta.Bag` | [04](04-metadata.md) |
| authority | the override order on every metadata write: `plugin < directive < manual` | [04](04-metadata.md) |
| completeness contract | a key's promise that it is stamped on every X by phase Y, checked in audit mode | [04](04-metadata.md) |
| projection | a neutral view of language facts: Callable, TypeShape, Resolve, Members, Values | [03](03-projection.md) |
| tier | where a language question may live: 1 mandatory, 2 optional and found by asserting, 3 the language sdk | [03](03-projection.md) |
| degradation scale | the four levels a source construct can sit at: projected fully, projected partly with metadata, opaque with metadata, or refused at lowering | [03](03-projection.md) |
| carrier | where a directive physically sits in source, such as `//+gen:` or an attribute | [05](05-directives.md) |
| directive | the canonical parsed annotation: one grammar, params checked against a schema | [05](05-directives.md) |
| slot | a typed append point on an emit value. Every body carries `prologue` and `epilogue` | [07](07-rendering.md) |
| TemplateRef | a body claimed by a named template from its emitter, run by the backend at render time | [07](07-rendering.md) |
| scaffolding vocabulary | the deliberately small set of body statements: delegate call, return, assignment, guard | [07](07-rendering.md) |
| funcmap | a language's shared template vocabulary, registered once by its backend | [07](07-rendering.md) |
| lowering | how a target language spells the canonical shapes. The write half of the hub | [10](10-cross-language.md) |
| policy | the contested mappings a lowering receives already resolved, such as `ts.int64` | [10](10-cross-language.md) |
| plan export | a plan's published, typed summary of what it generated. The only edge between plans | [08](08-workspace-and-plans.md) |
| WorkspaceCheck | the role that runs at Close, reads the records (manifests, exports, graph, facts) and produces diagnostics only | [06](06-plugins.md) |
| read set | the (symbol, key) inputs recorded for a derived artifact. `explain` and invalidation both use it | [09](09-incrementality.md) |
| fingerprint gate | the stat and hash pass above the loader that decides whether anything loads | [09](09-incrementality.md) |
| early cutoff | recompute only when a read *value* changed, not merely when its input was touched | [09](09-incrementality.md) |
| sealed graph | the persisted symbol graph. Regions decode on first touch, so opening it costs what the run reads | [09](09-incrementality.md) |
| warm≡cold | the check proving that a cached run and a cold run produce byte-identical manifests | [13](13-testing-and-conformance.md) |
| check | one suite of the conformance set | [13](13-testing-and-conformance.md) |
| manifest | the versioned record of every generated file: its plan, its hash, and what it derives from | [17](17-output-and-determinism.md) |
| drift | a manifested file whose hash on disk no longer matches. Someone edited it, and eidos never overwrites it | [17](17-output-and-determinism.md) |
| adoption | claiming a byte-equal pre-existing file into the manifest on the first run | [17](17-output-and-determinism.md) |
| sweep | deleting manifested files whose producer no longer exists | [17](17-output-and-determinism.md) |
| tag | a named companion output of a plugin, such as `test` or `docs`. The empty tag is the primary output | [18](18-routing-and-layout.md) |
| satellite | a language module of the fixed anatomy, depending only on the kernel | [11](11-languages.md) |
| eidos-lang | the shared tree-sitter bindings and pinned grammars. It sits below the satellites and registers no language | [11](11-languages.md) |
| read-only language | a satellite that ships a frontend and rules and stops, as protobuf does | [11](11-languages.md) |
| declarative plugin | a plugin written as files, a manifest plus templates, loaded by the kernel | [06](06-plugins.md) |
| command kernels | the complete CLI implementations a consumer's binary composes | [20](20-cli.md) |
| canary ring | the satellites the kernel runs against HEAD before it tags a release | [15](15-compatibility.md) |
| boundary string | a spelling a human types in YAML, a directive or the CLI, resolved against a registry at Build | [README](README.md) |
| trigger | what makes a rule's handler run: every subject, a directive, a stamped fact, an emit value, or the graph | [06b](06b-authoring.md) |
| match | a handler's read surface: the subject, plus rules, reader and reporting already bound | [06b](06b-authoring.md) |
| accumulator output | a `PerPackage` or `PerPlan` file filled across matches, ordered by subject identity | [18](18-routing-and-layout.md) |
| Link | the step after Load that resolves type spellings to canonical identities | [02](02-symbol-model.md) |
| kit | the frontend and backend authoring surface: declarations plus a few language callbacks, with the kit owning the rest | [11](11-languages.md) |
| unit | the load granule the language defines. `u.Read` is its only way to reach the filesystem, and every read is fingerprinted | [11](11-languages.md) |
| repeatable directive | a directive its schema allows more than once per subject. The handler runs once per instance | [05](05-directives.md) |
| Workspace / Run | the immutable composition (registries, compiled dispatch plan) against one invocation's mutable state (ledger, graph, facts, plans, diagnostics) | [08](08-workspace-and-plans.md) |
| artifact | one generated file. It is the re-execution granule, and its ledger row decides whether it is dirty | [09](09-incrementality.md) |
| sealed state | what a run persists for the next one: the sealed graph, the fact store and the artifact table, written per generation | [08](08-workspace-and-plans.md) |
| dispatch plan | the index of every subscription, compiled at Build by kind, directive and fact key, owned by the Workspace | [08](08-workspace-and-plans.md) |
| subscription record | a rule's gate as data (`kind`, `directive` or `factKey`, phase). It is both the dispatch index and the dirty-routing key | [06b](06b-authoring.md) |
| provenance trailer | the `<brand>:provenance sha256:<body-hash>` line ending every generated file. It proves ownership and detects drift without the manifest | [17](17-output-and-determinism.md) |
| fact group | a bundle of metadata keys its writer declares, which `meta drop` can remove under one public name | [04](04-metadata.md) |
| Members | the Tier-1 projection giving a type's effective member set across embeds and supertypes, and where each member came from | [03](03-projection.md) |
| Build | the validation sequence that runs once per composition and produces the immutable Workspace. Every boundary string resolves here | [08](08-workspace-and-plans.md) |
| parse memo | the content-addressed store of parsed regions keyed by unit fingerprint. It returns branch switches, where the sealed state returns the edit loop | [09](09-incrementality.md) |
| Reader | the tracked, scope-filtered read handle. Targeted reads record identity edges; enumerations record set-membership edges | [02](02-symbol-model.md) |
| classification form | shape, mixin or contract: the spec field that pins a classification's cardinality, arbitration, inference and directive shape | [12](12-shape-catalog.md) |
