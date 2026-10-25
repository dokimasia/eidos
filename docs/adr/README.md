<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# Architecture decision records

Each record holds one decision: what we decided, what else we considered, and
what it costs. Once we accept a record, nobody changes what it argues. If the
decision changes, write a new record that supersedes the old one, then link
the old record to the new one.

The table in [21-decisions.md](../architecture/21-decisions.md) lists every
settled decision. Look there first. Write a record here when the event you
said would make you reconsider a decision actually happens, or when the
reasoning no longer fits in one row of that table.
[ADR-0001](0001-use-adrs-for-architecture-decisions.md) says why we work this
way.

| # | Title | Status |
|---|---|---|
| [0001](0001-use-adrs-for-architecture-decisions.md) | Use ADRs for architecture decisions | Accepted |
| [0002](0002-identity-carries-source-language.md) | Identity carries the source language | Accepted |
| [0003](0003-json-codecs-round-trip.md) | JSON codecs round-trip | Accepted |
| [0004](0004-generate-kind-enum-from-schema.md) | Generate the Kind enum from the schema | Accepted |
| [0005](0005-concern-grouped-generated-files.md) | Group generated files by concern | Accepted |
| [0006](0006-host-is-an-identity.md) | The owner back-pointer is an identity | Accepted |
| [0007](0007-assert-through-the-assert-module.md) | Test code asserts through the assert module | Superseded by [0008](0008-pin-the-assert-module-by-version.md) |
| [0008](0008-pin-the-assert-module-by-version.md) | Pin the assert module by version | Accepted |
| [0009](0009-the-carrier-mark-follows-the-brand.md) | The carrier mark follows the brand | Accepted |
| [0010](0010-export-entries-have-a-five-part-key.md) | An export entry has a five-part key and no signature | Accepted |
| [0011](0011-region-string-tables-and-hashed-edges.md) | Each region has its own string table, and edges are hashed | Accepted |
| [0012](0012-package-reads-depend-on-every-member.md) | A package read whole depends on every member | Accepted |
| [0013](0013-the-parse-memo-is-keyed-by-the-executable.md) | The parse memo is keyed by the executable and can be shared | Accepted |
| [0014](0014-the-protobuf-frontend-resolves-four-features.md) | The protobuf frontend resolves the four features it applies | Accepted |
| [0015](0015-the-delimited-mark-is-stamped-at-the-parse.md) | The delimited mark is stamped at the parse | Accepted |
| [0016](0016-the-proto2-schema-marks-message-fields-optional.md) | The proto2 service schema marks its message fields optional | Accepted |
| [0017](0017-the-service-fixture-edit-narrows-a-field.md) | The service fixture's edit narrows a field | Accepted |
| [0018](0018-the-declarative-host-is-a-module.md) | The declarative host is a module of its own | Accepted |
| [0019](0019-a-family-claims-its-files-through-a-template.md) | A family claims its files through a file template | Accepted, superseded in part by [0099](0099-a-reference-with-a-target-resolves-to-a-record.md) |
| [0020](0020-the-interface-rules-account-for-properties.md) | The interface rules account for properties | Accepted |
| [0021](0021-the-receiver-avoids-the-names-of-its-signature.md) | The receiver of an implementation avoids the names of its signature | Accepted |
| [0022](0022-a-rules-implementation-reads-a-named-key-through-fact.md) | A rules implementation reads a named key through rules.Fact | Accepted |
| [0023](0023-a-template-passes-a-signature-slot-by-address.md) | A template passes a signature slot to its helper by address | Accepted |
| [0024](0024-the-shared-helpers-keep-the-departures-of-their-implementation.md) | The shared helpers keep the departures of their implementation | Accepted |
| [0025](0025-the-kit-mechanisms-keep-the-departures-of-their-implementation.md) | The kit mechanisms keep the departures of their implementation | Accepted |
| [0026](0026-the-surface-checks-keep-the-departures-of-their-implementation.md) | The surface checks keep the departures of their implementation | Accepted |
| [0027](0027-the-renames-keep-the-departures-of-their-implementation.md) | The renames keep the departures of their implementation | Accepted |
| [0028](0028-the-facts-of-the-model-keep-the-departures-of-their-implementation.md) | The facts of the model keep the departures of their implementation | Accepted |
| [0029](0029-the-five-contracts-keep-the-departures-of-their-implementation.md) | The five contracts keep the departures of their implementation | Accepted |
| [0030](0030-the-error-contract-keeps-the-departures-of-its-implementation.md) | The error contract keeps the departures of its implementation | Accepted |
| [0031](0031-the-required-options-keep-the-departures-of-their-implementation.md) | The required options keep the departures of their implementation | Accepted |
| [0032](0032-the-inputs-of-a-route-keep-the-departures-of-their-implementation.md) | The inputs of a route keep the departures of their implementation | Accepted |
| [0033](0033-the-config-file-of-a-binary-keeps-the-departures-of-its-implementation.md) | The config file of a binary keeps the departures of its implementation | Accepted |
| [0034](0034-the-output-of-the-command-kernels-keeps-the-departures-of-its-implementation.md) | The output of the command kernels keeps the departures of its implementation | Accepted |
| [0035](0035-the-legacy-build-line-keeps-the-departure-of-its-implementation.md) | The legacy build line keeps the departure of its implementation | Accepted |
| [0036](0036-the-position-of-a-package-keeps-the-departure-of-its-implementation.md) | The position of a package keeps the departure of its implementation | Accepted |
| [0037](0037-the-children-of-an-inline-reference-are-copies.md) | The children of an inline reference are copies | Accepted |
| [0038](0038-the-facade-keeps-the-identifiers-of-its-packages.md) | The facade keeps the identifiers of its packages | Accepted |
| [0039](0039-the-doubles-keep-the-spellings-of-their-targets.md) | The doubles keep the spellings of their targets | Accepted |
| [0040](0040-the-reference-ensemble-keeps-the-departures-of-its-implementation.md) | The reference ensemble keeps the departures of its implementation | Accepted |
| [0041](0041-the-options-of-a-plugin-keep-their-two-refusals.md) | The options of a plugin keep their two refusals | Accepted |
| [0042](0042-the-toolchain-suite-requires-the-toolchain-before-its-parse.md) | The toolchain suite requires the toolchain before its parse | Accepted |
| [0043](0043-the-declared-names-keep-the-departures-of-their-implementation.md) | The declared names keep the departures of their implementation | Accepted |
| [0044](0044-the-lowering-seams-keep-the-departures-of-their-implementation.md) | The lowering seams keep the departures of their implementation | Accepted |
| [0045](0045-the-tree-sitter-frontends-keep-the-departures-of-their-implementation.md) | The tree-sitter frontends keep the departures of their implementation | Accepted |
| [0046](0046-the-shape-catalog-keeps-the-departures-of-its-implementation.md) | The shape catalog keeps the departures of its implementation | Accepted |
| [0047](0047-the-cross-language-hub-keeps-the-departures-of-its-implementation.md) | The cross-language hub keeps the departures of its implementation | Accepted |
| [0048](0048-the-command-line-of-a-binary-is-a-go-cli-app.md) | The command line of a binary is a go-cli app | Accepted |
| [0049](0049-the-config-file-of-a-workspace-sets-the-values-of-options.md) | The config file of a workspace sets the values of options | Accepted |
| [0050](0050-eidos-publishes-the-binary-eidos.md) | eidos publishes the binary eidos | Accepted |
| [0051](0051-an-extensible-plan-can-have-no-generator.md) | An extensible plan can have no generator | Accepted |
| [0052](0052-a-run-locates-the-stores-of-the-frontends-with-input-files.md) | A run locates the stores of the frontends with input files | Accepted |
| [0053](0053-the-ring-tests-a-release-from-the-tree-of-its-tag.md) | The ring tests a release from the tree of its tag | Accepted |
| [0054](0054-protobuf-has-a-backend-for-every-construct.md) | protobuf has a backend for every construct of its five versions | Accepted |
| [0055](0055-a-generator-states-the-header-of-a-routed-file.md) | A generator states the header of a routed file | Accepted |
| [0056](0056-a-field-declared-outside-its-type-has-receives.md) | A field declared outside its type has Receives | Accepted |
| [0057](0057-eidos-reads-and-writes-graphql-through-gqlparser.md) | eidos reads and writes GraphQL through gqlparser | Accepted |
| [0058](0058-a-graphql-unit-is-a-schema-and-its-documents.md) | A GraphQL unit is a schema and its documents | Accepted |
| [0059](0059-a-parameter-has-documentation.md) | A parameter has documentation | Accepted |
| [0060](0060-eidos-lang-graphql-requires-the-gqlparser-fork.md) | eidos-lang-graphql requires the fork of gqlparser | Accepted |
| [0061](0061-file-level-names-collide-in-the-namespace-of-their-file.md) | File-level names collide in the namespace of their routed file | Accepted |
| [0062](0062-a-wire-field-keeps-the-name-that-its-generator-wrote.md) | A wire field keeps the name that its generator wrote | Accepted |
| [0063](0063-a-supertype-translates-through-the-spelling-of-its-position.md) | A supertype translates through the spelling of its position | Accepted |
| [0064](0064-eidos-reads-python-through-the-fork-of-tree-sitter-python.md) | eidos reads Python through the fork of tree-sitter-python | Accepted |
| [0065](0065-a-python-unit-is-one-module.md) | A Python unit is one module | Accepted |
| [0066](0066-the-python-frontend-reads-the-standard-library-from-typeshed.md) | The Python frontend reads the standard library from typeshed | Accepted |
| [0067](0067-a-parameter-states-how-a-caller-passes-its-argument.md) | A parameter states how a caller passes its argument | Accepted |
| [0068](0068-a-property-has-a-delete-accessor.md) | A property has a delete accessor | Accepted |
| [0069](0069-a-target-that-overloads-lets-the-overloads-of-a-function-share-its-name.md) | A target that overloads lets the overloads of a function share its name | Accepted |
| [0070](0070-a-backend-respells-a-member-by-its-level-and-its-mutability.md) | A backend respells a member by its level and its mutability | Accepted |
| [0071](0071-the-kernel-splits-a-return-of-a-result-type.md) | The kernel splits a return of a result type | Accepted |
| [0072](0072-an-authored-text-takes-the-value-that-its-language-writes.md) | An authored text takes the value that its language writes | Accepted |
| [0073](0073-the-rust-rules-fold-the-standard-library-through-a-table.md) | The Rust rules fold the standard library through a table | Accepted |
| [0074](0074-the-projection-of-an-rpc-states-its-context-and-its-failure.md) | The projection of an rpc states its context and its failure | Accepted |
| [0075](0075-a-policy-of-the-target-selects-the-grpc-binding.md) | A policy of the target selects the gRPC binding | Accepted |
| [0076](0076-the-ring-reports-a-release-of-an-earlier-line.md) | The ring reports a release of an earlier compatibility line | Accepted |
| [0077](0077-a-bound-reference-takes-the-package-of-its-binder.md) | A bound reference takes the package that the binder of its target returns | Accepted |
| [0078](0078-a-test-fixture-routes-through-the-layout-of-a-plan.md) | A test fixture routes through the layout of a plan | Accepted |
| [0079](0079-a-handler-marks-the-internal-tests-of-an-external-family.md) | A handler marks the internal tests of an external family | Accepted |
| [0080](0080-an-enum-and-a-sum-state-their-contracts.md) | An enum and a sum state the contracts that they implement | Accepted |
| [0081](0081-a-callable-marks-an-overload-signature.md) | A function and a method mark an overload signature | Accepted |
| [0082](0082-the-render-orders-the-declarations-of-a-file.md) | The render orders the declarations of a file | Accepted |
| [0083](0083-the-spoke-spells-a-type-parameter.md) | The spoke spells a type parameter at every depth of a type | Accepted |
| [0084](0084-the-typescript-frontend-loads-the-library-of-each-project.md) | The TypeScript frontend loads the library of each project from the embedded release | Accepted |
| [0085](0085-the-member-walk-lists-a-computed-member.md) | The member walk lists a member with a computed name | Accepted |
| [0086](0086-colliding-overloads-spell-their-type-parameters.md) | The discriminator of colliding overloads spells their type parameters | Accepted |
| [0087](0087-eidos-reads-typescript-through-the-fork-of-tree-sitter-typescript.md) | eidos reads TypeScript through the fork of tree-sitter-typescript | Accepted |
| [0088](0088-the-layout-qualifies-an-emitted-reference.md) | The layout qualifies an emitted reference by the file of its referent | Accepted |
| [0089](0089-an-import-hides-the-global-scope-from-its-name.md) | An import hides the global scope from the name that it binds | Accepted |
| [0090](0090-a-dependency-round-passes-the-uses-of-units.md) | A dependency round passes the uses of the loaded units | Accepted |
| [0091](0091-the-typescript-library-loads-the-names-that-a-load-uses.md) | The TypeScript library loads one unit for each global name that a load uses | Accepted |
| [0092](0092-a-handler-reads-the-instances-of-its-directives.md) | A handler reads the instances of the directives of its plugin on its subject | Accepted |
| [0093](0093-a-handler-enumerates-the-subjects-of-a-key.md) | A handler enumerates the subjects of a key under a membership edge | Accepted |
| [0094](0094-a-carrier-line-contains-one-directive-for-each-mark.md) | A carrier line contains one directive for each mark | Accepted |
| [0095](0095-a-typescript-sample-constructs-its-class.md) | A TypeScript sample constructs its class and sets every required property of its interface | Accepted |
| [0096](0096-a-typescript-file-has-the-spaces-and-quotes-of-its-formatters.md) | A TypeScript file has the spaces and the quotes of Biome and Prettier | Accepted |
| [0097](0097-a-go-file-has-the-format-of-gofumpt-and-golines.md) | A Go file has the format of gofumpt and golines | Accepted |
| [0098](0098-the-go-frontend-loads-the-packages-that-a-load-uses.md) | The Go frontend loads the packages that a load uses | Accepted, superseded in part by [0102](0102-the-go-rules-resolve-a-directive-argument-in-the-scope-of-its-package.md) |
| [0099](0099-a-reference-with-a-target-resolves-to-a-record.md) | A reference with a target resolves to a record | Accepted |
| [0100](0100-the-workspace-leaves-a-gate-on-a-key-of-an-absent-owner-unbound.md) | The workspace leaves a gate on a key of an absent owner unbound | Accepted |
| [0101](0101-a-plugin-with-both-roles-provides-a-capability-in-one-role.md) | A plugin with both roles provides a capability in one role | Accepted |
| [0102](0102-the-go-rules-resolve-a-directive-argument-in-the-scope-of-its-package.md) | The Go rules resolve a directive argument in the scope of its package | Accepted |
| [0103](0103-a-record-keeps-the-data-of-the-call-that-places-it.md) | A record keeps the data of the call that places it | Accepted |
| [0104](0104-the-member-walk-follows-an-alias-and-an-unexported-embed-of-a-dependency.md) | The member walk follows an alias and an unexported embed of a dependency | Accepted |
