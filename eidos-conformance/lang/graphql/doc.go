// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package graphql declares the conformance entry of GraphQL and the catalog
// fixture. Its tests grade GraphQL against the shared feature inventory, run
// the catalog fixture through the frame of the workspace, and mirror the
// catalog through the GraphQL backend.
//
// [Corpus] is the entry of GraphQL: the GraphQL frontend and rules over a
// tree of schemas, which spell records, fields with arguments, interfaces,
// enums and carriers. A feature with a sibling package has a configuration
// of graphql-config, because a unit of GraphQL reads another package only
// as a shared package of its schema. The entry refuses the four features
// that a schema cannot state: an overload, a standalone constant, a test
// file and a marker of the brand.
//
// # The catalog fixture
//
// [ComposeCatalog] and [CatalogPlans] compose one workspace over a schema
// that spans two directories, which one configuration of graphql-config
// names with the documents of the schema. The schema states every construct
// of the type system, and the documents state every construct of the
// executable language. A document mixes both languages. The plans write
// three families of files from one graph:
//
//   - [ModelPlan] writes the Go structs, interfaces and enums of the schema,
//     and a resolver interface for each root operation type.
//   - [ClientPlan] writes the TypeScript interfaces, enums and aliases of the
//     schema, and the result and the variables of each executable
//     definition.
//   - [MirrorPlan] restates each declaration through the stamps of its source
//     and writes each file under [MirrorDir]. Each source is in the layout of
//     the GraphQL backend, so each written file equals its source between the
//     frame of the output contract, without its carriers.
//
// [EditCatalog] is the edit of the fixture, which the warm checks apply.
//
// # Dependency position
//
// graphql imports the conformance package, core/frontend/frontendtest,
// core/layout, core/ledger, core/workspace, lang/naming, the root and the
// backend, frontend and rules packages of the GraphQL satellite, the root
// and the backend of the Go and the TypeScript satellites, the sdk facade
// and the Go stdlib. No package imports graphql but its own tests and the
// matrix.
package graphql
