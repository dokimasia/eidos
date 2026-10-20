<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

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
| [0011](0011-neutral-emit-and-the-target-lowering-seams.md) | Neutral emit and the target lowering seams | Accepted |
| [0012](0012-the-sdk-contract-module.md) | The SDK facade module | Draft |
| [0013](0013-the-frontend-kit.md) | The frontend kit and its conformance suite | Draft |
| [0014](0014-the-projection-vocabulary.md) | The projection vocabulary and the rules seam | Accepted |
| [0015](0015-tree-sitter-frontends-and-dependency-stores.md) | Tree-sitter frontends, dependency stores and re-exports | Accepted |
| [0016](0016-layout-and-routing.md) | Layout, the routing of generated declarations to files | Draft |
| [0017](0017-the-end-to-end-run.md) | The end-to-end run, its manifest and its commit | Draft |
| [0018](0018-parallel-dispatch-and-slot-appends.md) | Parallel dispatch inside a bucket, and slot appends through the Emitter | Draft |
| [0019](0019-source-scopes-exports-and-checks.md) | Source scopes, plan exports and workspace checks | Accepted |
| [0020](0020-warm-runs-and-the-sealed-state.md) | Warm runs, the sealed state and re-execution by artifact | Accepted |
| [0021](0021-the-command-kernels.md) | The command kernels, the config file and the run lock | Accepted |
| [0022](0022-the-shape-catalog.md) | The shape catalog, its specs and its generated registries | Accepted |
| [0023](0023-the-cross-language-hub.md) | The cross-language hub and the TypeScript satellite | Accepted |
| [0024](0024-protobuf-editions-and-services.md) | Protobuf editions and a service in Go and TypeScript | Accepted |
| [0025](0025-declarative-plugins.md) | Declarative plugins and their manifest | Accepted |
| [0026](0026-the-reference-ensemble-and-the-canary-ring.md) | The reference ensemble and the canary ring | Accepted |
| [0027](0027-the-subject-kinds-of-a-directive.md) | The subject kinds of a directive | Accepted |
| [0028](0028-group-commit-of-outputs-and-the-record.md) | Group commit of outputs and the record | Accepted |
| [0029](0029-the-memory-layout-of-the-model.md) | The memory layout of the model | Accepted |
| [0030](0030-the-contracts-of-the-spi-and-the-authoring-api.md) | The contracts of the SPI and the authoring API | Accepted |
| [0031](0031-the-shared-helpers-of-the-satellites.md) | The shared helpers of the satellites | Accepted |
| [0032](0032-the-fixture-builder.md) | The fixture builder | Accepted |
| [0033](0033-the-shared-mechanisms-of-the-test-kits.md) | The shared mechanisms of the test kits | Accepted |
| [0034](0034-the-surface-and-documentation-checks-of-generated-output.md) | The surface and documentation checks of generated output | Accepted |
| [0035](0035-one-name-for-each-concept.md) | One name for each concept | Accepted |
| [0036](0036-the-facts-that-the-model-drops.md) | The facts that the model drops | Accepted |
| [0037](0037-five-contracts-that-the-reviewed-defects-need.md) | Five contracts that the reviewed defects need | Accepted |
| [0038](0038-the-modules-of-an-import-specifier.md) | The modules of an import specifier | Accepted |
| [0039](0039-the-meta-set-form-parameter-defaults-and-name-suggestions.md) | The meta set form, parameter defaults and name suggestions | Accepted |
| [0040](0040-typed-node-queries-and-interface-satisfaction.md) | Typed node queries and interface satisfaction | Accepted |
| [0041](0041-the-gates-of-a-rule-and-the-order-of-a-plugin.md) | The gates of a rule and the order of a plugin | Accepted |
| [0042](0042-the-error-contract-of-a-type.md) | The error contract of a type | Accepted |
| [0043](0043-required-options-and-option-values-from-text.md) | Required options and option values from text | Accepted |
| [0044](0044-the-inputs-of-a-route.md) | The inputs of a route | Accepted |
| [0045](0045-the-config-file-of-a-binary.md) | The config file of a binary | Accepted |
| [0046](0046-the-output-of-the-command-kernels.md) | The output of the command kernels | Accepted |
| [0047](0047-the-directive-registry-and-the-output-stream.md) | The directive registry and the output stream | Accepted |
| [0048](0048-unique-names.md) | Unique names | Accepted |
| [0049](0049-go-enumerations.md) | Go enumerations | Accepted |
| [0050](0050-the-shapes-and-samples-of-go-types.md) | The shapes and samples of Go types | Accepted |
| [0051](0051-the-names-of-declarations-as-values.md) | The names of declarations as values | Accepted |
| [0052](0052-go-parameters-as-subjects-and-the-legacy-build-line.md) | Go parameters as subjects and the legacy build line | Accepted |
| [0053](0053-helpers-without-a-consumer.md) | Helpers without a consumer | Accepted |
| [0054](0054-typescript-declarations-that-the-model-loses.md) | TypeScript declarations that the model loses | Accepted |
| [0055](0055-the-position-of-a-package-and-the-protobuf-facts.md) | The position of a package and the protobuf facts | Accepted |
| [0056](0056-the-references-that-a-generated-file-writes.md) | The references that a generated file writes | Accepted |
| [0057](0057-template-helpers-and-scaffold-forms.md) | Template helpers and scaffold forms | Accepted |
| [0058](0058-the-build-constraint-of-a-generated-file.md) | The build constraint of a generated file | Accepted |
| [0059](0059-package-structure-and-the-facade.md) | Package structure and the facade | Accepted |
| [0060](0060-the-reference-ensemble-after-its-review.md) | The reference ensemble after its review | Accepted |
| [0061](0061-the-declared-name-of-an-imported-go-package.md) | The declared name of an imported Go package | Accepted |
| [0062](0062-the-projection-side-of-the-support-matrix.md) | The projection side of the support matrix | Accepted |
| [0063](0063-the-assigned-argument-of-an-annotation.md) | The assigned argument of an annotation | Accepted |
| [0064](0064-the-command-line-on-go-cli.md) | The command line on go-cli | Accepted |
| [0065](0065-the-eidos-binary.md) | The eidos binary | Accepted |
| [0066](0066-the-ring-tests-a-release-from-its-tag.md) | The ring tests a release from its tag | Accepted |
| [0067](0067-the-protobuf-backend.md) | The protobuf backend | Accepted |
| [0068](0068-the-graphql-satellite.md) | The GraphQL satellite | Accepted |
| [0069](0069-constructions-module-directives-and-the-shared-helpers-of-plugins.md) | Constructions, module directives and the shared helpers of plugins | Accepted |
| [0070](0070-names-collide-in-the-routed-package.md) | Names collide in the routed package | Accepted |
| [0071](0071-wire-names.md) | Wire names | Accepted |
| [0072](0072-translated-supertypes.md) | Translated supertypes | Accepted |
| [0073](0073-the-python-satellite.md) | The Python satellite | Accepted |
| [0074](0074-wrapped-documentation-function-values-identity-lists-and-the-order-of-a-unit.md) | Wrapped documentation, function values, identity lists and the order of a unit | Draft |
| [0075](0075-dependency-packages-the-routing-of-a-placement-authored-literals-the-trailer-and-the-external-test-package.md) | Dependency packages, the routing of a placement, authored literals, the trailer and the external test package | Draft |
| [0076](0076-the-projection-rules-of-rust.md) | The projection rules of Rust | Accepted |
| [0077](0077-the-grpc-bindings-of-a-protobuf-service.md) | The gRPC bindings of a protobuf service | Accepted |
| [0078](0078-the-layout-of-a-test-fixture-and-internal-tests.md) | The layout of a test fixture and the internal tests of an external family | Accepted |
| [0079](0079-the-implements-clause-of-an-enum-and-a-sum.md) | The implements clause of an enum and a sum | Accepted |
| [0080](0080-overload-signatures-and-the-order-of-a-file.md) | Overload signatures and the order of the declarations of a file | Accepted |
| [0081](0081-a-type-parameter-inside-a-structural-type.md) | A type parameter inside a structural type | Accepted |
