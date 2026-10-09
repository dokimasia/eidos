# Requests for comments

An RFC proposes a design and carries the argument with it: why the change is
needed, what the interfaces look like, what else we considered, and what it
costs. Rejected RFCs stay here. They record what someone already proposed and
why we turned it down.

Write an RFC before the code when a change alters a contract shape, affects
more than one consumer, or has several plausible shapes worth comparing. If
the decision is already made and there is nothing left to argue, write an
[ADR](../adr/README.md) instead.

| # | Title | Status |
|---|---|---|
| [0001](0001-symbol-model-contract.md) | The symbol schema and its contract | Accepted |
| [0002](0002-model-generator.md) | The model generator (internal/gen/model) | Accepted |
| [0003](0003-diagnostics-and-store.md) | Diagnostics and the node store | Accepted |
| [0004](0004-metadata-facts.md) | Metadata keys and the fact store | Accepted |
| [0005](0005-directive-grammar.md) | The directive grammar, schemas and validation | Accepted |
| [0006](0006-authoring-surface-and-dispatch.md) | The authoring surface and dispatch | Accepted |
| [0007](0007-build-steps.md) | The Build steps and the fixture run | Accepted |
| [0008](0008-emit-body-content.md) | The emit body and the scaffolding vocabulary | Review |
| [0009](0009-render-pass-and-backend-kit.md) | The render pass and the backend kit | Review |
| [0010](0010-output-contract.md) | The output contract | Review |
| [0011](0011-neutral-emit-and-the-target-lowering-seams.md) | Neutral emit and the target lowering seams | Review |
| [0012](0012-the-sdk-contract-module.md) | The SDK facade module | Draft |
| [0013](0013-the-frontend-kit.md) | The frontend kit and its conformance suite | Draft |
| [0014](0014-the-projection-vocabulary.md) | The projection vocabulary and the rules seam | Accepted |
| [0015](0015-tree-sitter-frontends-and-dependency-stores.md) | Tree-sitter frontends, dependency stores and re-exports | Draft |
| [0016](0016-layout-and-routing.md) | Layout, the routing of generated declarations to files | Draft |
| [0017](0017-the-end-to-end-run.md) | The end-to-end run, its manifest and its commit | Draft |
| [0018](0018-parallel-dispatch-and-slot-appends.md) | Parallel dispatch inside a bucket, and slot appends through the Emitter | Draft |
| [0019](0019-source-scopes-exports-and-checks.md) | Source scopes, plan exports and workspace checks | Accepted |
| [0020](0020-warm-runs-and-the-sealed-state.md) | Warm runs, the sealed state and re-execution by artifact | Accepted |
| [0021](0021-the-command-kernels.md) | The command kernels, the config file and the run lock | Accepted |
| [0022](0022-the-shape-catalog.md) | The shape catalog, its specs and its generated registries | Draft |
