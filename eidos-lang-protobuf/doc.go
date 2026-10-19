// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protobuf is the root of the protobuf satellite: the language's
// identity, its names and its residue, which the satellite's packages
// share.
//
// [Lang] is the source language of every loaded declaration, [Name]
// the plugin identity findings and stamps report under, [CodePrefix]
// the prefix of every diagnostic code the satellite registers,
// [Extension] the file extension, [Version] the string every unit key
// folds, and [Syntax] the comment forms. [Keys] registers the
// metadata keys the frontend stamps. [Target] is the spelling a plan
// resolves to select the backend, and [BackendVersion] the version of its
// rendered output.
//
// # Packages
//
// The module covers protobuf as a source and as a target:
//
//   - frontend loads proto2, proto3 and the editions 2023, 2024 and 2026
//     into the node graph.
//   - rules projects protobuf's declarations for the kernel's walks.
//   - spell spells filenames, declared names, packages and the types of
//     other languages.
//   - backend renders emit values as protobuf schemas.
//   - testing parses and compiles generated schemas in process.
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
// # Channels
//
// A generator states the residue of protobuf in the emit model through
// the field of the model that other languages share, the tag entry
// [NumberTag] of a wire number, an annotation for an option, a [Marker]
// for a keyword or a statement that is not an option, and the header of a
// file. [Stated] and [Header] restate the stamps of a protobuf source in
// these channels, so a generator over protobuf sources writes a schema as
// its source states it. [Policies] returns the lowering policies of the
// target, which choose the encoding of an integer of each width and sign.
//
// # Editions
//
// [EditionNamed] returns one protobuf version of the table of the
// satellite: proto2, proto3 and the editions 2023, 2024 and 2026, each with
// the defaults of its features. [Features.Apply] resolves a feature from
// the options of a declaration, so the frontend and the backend resolve
// features from one table. The satellite applies four features:
// field_presence, enum_type, message_encoding and
// default_symbol_visibility. The other features apply to the wire format,
// the JSON mapping and the checks of protoc.
//
// # Dependency position
//
// lang/protobuf imports the sdk facade and the Go stdlib.
package protobuf
