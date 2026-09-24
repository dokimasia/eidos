// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf is the satellite protobuf schemas load through.
//
// [Lang] is the source language of every loaded declaration, [Name]
// the plugin identity findings and stamps report under, [Extension]
// the file extension, [Version] the string every unit key folds, and
// [Syntax] the comment forms. [Keys] registers the metadata keys the
// frontend stamps.
//
// # Shape
//
// Read-only: the module ships a frontend and projection rules, and
// no lowering, backend or sdk.
//
// # Names
//
// The frontend and the rules resolve a spelling through one grammar.
// [Candidates] returns protoc's probe order as shadowing tiers,
// [IsScalar] reports the scalar types that name no declaration, and
// [WellKnown] reports the well-known types a projection maps by name
// and never resolves against the graph.
//
// # Residue
//
// The frontend stamps what protobuf states and the projection has no
// kind for.
//
//   - [FieldKey] is a field's wire number, [LabelKey] a required
//     label, [JSONNameKey] a stated json_name, and [MapEntryKey]
//     marks a field written as a map.
//   - [ReservedKey] is the reserved ranges and names,
//     [ExtensionsKey] the extension ranges with their options.
//   - [OptionsKey] is the options a declaration states, at every
//     level that states them. [FeaturesKey] is the edition features,
//     stamped apart because they decide what an absent value means.
//   - [SyntaxKey] is the syntax or edition, [PackageKey] the proto
//     package, [ImportKey] the imports with public and weak marked,
//     [OneofKey] the oneof a sum projected from, [StreamKey] which
//     side of an rpc streams.
//
// # Editions
//
// The edition and every features option are stamped as written at
// each level that states them. The frontend resolves
// features.field_presence from the field up through its messages to
// the file and then the edition's default, which is explicit
// presence, and projects a singular field's presence as its form.
// Resolving any other feature is the consumer's.
//
// # Dependency position
//
// lang/protobuf imports the sdk facade and the Go stdlib.
package protobuf
