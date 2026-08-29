# Glossary

One line per term, for readers entering mid-set. Each term's
defining document carries the contract.

| Term | Meaning | Doc |
|---|---|---|
| workspace | the composition frame: one read side, N plans, one merged manifest | [08](08-workspace-and-plans.md) |
| plan | one write side — generators, layout, exactly one backend, one sink, a source scope; a value, not a plugin | [08](08-workspace-and-plans.md) |
| symbol | one declaration in the canonical model, with a stable cross-run identity | [02](02-symbol-model.md) |
| node / emit | the two generated models: what frontends produce / what generators produce | [02](02-symbol-model.md) |
| freeze | the seal after Annotate; structural writes refused from then on | [08](08-workspace-and-plans.md) |
| bag | a symbol's typed metadata container (`meta.Bag`) | [04](04-metadata.md) |
| authority | the override ladder on every metadata write: `plugin < directive < manual` | [04](04-metadata.md) |
| completeness contract | a key's promise ("stamped on every X by phase Y"), verified in audit mode | [04](04-metadata.md) |
| projection | a neutral view of language facts (Callable, TypeShape, Resolve, Members, Values) | [03](03-projection.md) |
| tier | where a language question may live: 1 mandatory, 2 optional-by-assertion, 3 language sdk | [03](03-projection.md) |
| degradation ladder | project fully → partially+meta → opaque+meta → refuse at lowering | [03](03-projection.md) |
| carrier | where a directive physically sits in source (`//+gen:`, attributes) | [05](05-directives.md) |
| directive | the canonical parsed annotation: one grammar, schema'd params | [05](05-directives.md) |
| slot | a typed append point on an emit value; bodies carry `prologue`/`epilogue` by construction | [07](07-rendering.md) |
| TemplateRef | a body claimed by its emitter's named template, executed by the backend at render time | [07](07-rendering.md) |
| scaffolding vocabulary | the deliberately minimal body statements: delegate call, return, assignment, guard | [07](07-rendering.md) |
| funcmap | a language's shared template vocabulary, registered once by its backend | [07](07-rendering.md) |
| lowering | a target language's spelling of canonical shapes — the hub's write half | [10](10-cross-language.md) |
| policy | the resolved contested-mapping choices a lowering receives (`ts.int64` → …) | [10](10-cross-language.md) |
| plan export | a plan's published, typed summary of what it generated; the only cross-plan edge | [08](08-workspace-and-plans.md) |
| WorkspaceCheck | the Close-time role reading the records (manifests, exports, graph, facts); diagnostics only | [06](06-plugins.md) |
| read set | the recorded (symbol, key) inputs of a derived artifact; feeds explain and invalidation | [09](09-incrementality.md) |
| fingerprint gate | the stat/hash pass above the loader that decides whether anything loads | [09](09-incrementality.md) |
| early cutoff | recompute only when a read *value* changed, not merely its input | [09](09-incrementality.md) |
| sealed graph | the persisted symbol graph; region-lazy, open cost proportional to the dirty region | [09](09-incrementality.md) |
| warm≡cold | the rung proving cached and cold runs produce byte-identical manifests | [13](13-testing-and-conformance.md) |
| rung | one layer of the conformance ladder | [13](13-testing-and-conformance.md) |
| manifest | the versioned record of every generated file: plan, hash, provenance | [17](17-output-and-determinism.md) |
| drift | a manifested file whose on-disk hash no longer matches — hand-edited, never overwritten | [17](17-output-and-determinism.md) |
| adoption | first-run claiming of byte-equal pre-existing files into the manifest | [17](17-output-and-determinism.md) |
| sweep | deletion of manifested files whose producer no longer exists | [17](17-output-and-determinism.md) |
| tag | a named companion output of a plugin (`test`, `docs`); empty tag = primary | [18](18-routing-and-layout.md) |
| satellite | a language module of the fixed anatomy, depending only on the kernel | [11](11-languages.md) |
| read-only language | a satellite shipping frontend + rules and nothing else (protobuf) | [11](11-languages.md) |
| declarative plugin | plugin-as-files (manifest + templates), instantiated by the kernel's loader | [06](06-plugins.md) |
| command kernels | the complete CLI implementations a consumer's binary composes | [20](20-cli.md) |
| canary ring | the satellites the kernel runs against HEAD before tagging | [15](15-compatibility.md) |
| boundary string | a human-typed spelling (YAML, directive, CLI) that resolves against a registry at Build | [README](README.md) |
| trigger | when a rule's handler fires: bare (every subject), directive-gated, fact-gated, emit, or graph | [06b](06b-authoring.md) |
| match | a handler's read surface: the subject plus pre-bound rules, reader, and scoped reporting | [06b](06b-authoring.md) |
| accumulator output | a `PerPackage`/`PerPlan` file filled across matches, ordered by subject identity | [18](18-routing-and-layout.md) |
| Link | the post-Load step resolving type spellings to canonical identities | [02](02-symbol-model.md) |
| kit | the frontend/backend authoring surface: declarations plus a few language callbacks; ritual kit-owned | [11](11-languages.md) |
| unit | the language-defined load granule; `u.Read` is its only filesystem door, jailed and fingerprint-tracked | [11](11-languages.md) |
| repeatable directive | a schema-declared directive that may appear per subject more than once; handlers fire per instance | [05](05-directives.md) |
| Workspace / Run | the immutable composition (registries, compiled dispatch plan) vs one invocation's mutable state (ledger, graph, facts, plans, diag) | [08](08-workspace-and-plans.md) |
| artifact | one generated file — the re-execution granule; its ledger row decides dirty by lookup | [09](09-incrementality.md) |
| sealed state | what a run persists for the next: sealed graph + fact store + artifact table, generational | [08](08-workspace-and-plans.md) |
| dispatch plan | the Build-time compiled index of every subscription (by kind, directive, fact key), owned by the Workspace | [08](08-workspace-and-plans.md) |
| subscription record | a rule's gate tuple as data (`kind`, `directive`/`factKey`, phase) — dispatch index and dirty-routing key in one | [06b](06b-authoring.md) |
| provenance trailer | the file-final `<brand>:provenance sha256:<body-hash>` line — per-file ownership and drift proof, manifest-independent | [17](17-output-and-determinism.md) |
| fact group | a writer-declared bundle of metadata keys, droppable as one public name via `meta drop` | [04](04-metadata.md) |
| Members | the Tier-1 projection: a type's effective member set across embeds and supertypes, with contributor provenance | [03](03-projection.md) |
| Build | the once-per-composition validation ladder producing the immutable Workspace; every boundary string resolves here | [08](08-workspace-and-plans.md) |
| parse memo | the content-addressed store of parsed regions keyed by unit fingerprint — history's cache; the sealed state is the edit loop's | [09](09-incrementality.md) |
| Reader | the tracked, scope-filtered read handle; targeted reads record identity edges, enumerations record set-membership edges | [02](02-symbol-model.md) |
| classification form | shape / mixin / contract — the spec field pinning a classification's cardinality, arbitration, inference, and directive shape | [12](12-shape-catalog.md) |
