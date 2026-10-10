// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf is the satellite protobuf schemas load through.
//
// [Lang] is the source language of every loaded declaration, [Name]
// the plugin identity findings and stamps report under, [CodePrefix]
// the prefix of every diagnostic code the satellite registers,
// [Extension] the file extension, [Version] the string every unit key
// folds, and [Syntax] the comment forms. [Keys] registers the
// metadata keys the frontend stamps.
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
//   - [OptionsKey] is the options that a declaration states.
//     [FeaturesKey] is the edition features. The frontend stamps them
//     apart from the other options, because they define what an absent
//     value means.
//   - [SyntaxKey] is the syntax or edition, [PackageKey] the proto
//     package, [ImportKey] the imports with public, weak and option
//     marked, [OneofKey] the oneof a sum projected from, [StreamKey]
//     which side of an rpc streams.
//   - [ClosedKey] marks a closed enum, [DelimitedKey] a field whose
//     message is encoded delimited, and [LocalKey] a message or an enum
//     that another file cannot reference.
//
// # Editions
//
// The frontend loads proto2, proto3 and the editions 2023, 2024 and
// 2026. It stamps the version, and every features option on the
// declaration that states it. The frontend resolves four features from
// the version's default through the declarations that enclose a field,
// an enum or a message: field_presence, enum_type, message_encoding and
// default_symbol_visibility. It projects a singular field's presence as
// its form, and marks a closed enum, a delimited field and a local
// declaration. The other features apply to the wire format, the JSON
// mapping and the checks of protoc.
//
// # Dependency position
//
// lang/protobuf imports the sdk facade and the Go stdlib.
package protobuf
