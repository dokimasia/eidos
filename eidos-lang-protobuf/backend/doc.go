// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package backend renders emit declarations as protobuf schemas: proto2,
// proto3 and the editions 2023, 2024 and 2026.
//
// [New] composes the backend through the kernel's backend kit. The pieces
// are the file skeleton [FileTemplate], the block template [BlockTemplate]
// of a message, an enum, a oneof and a service, the fact [Coverage], the
// template vocabulary in [Funcs], the filename, package and name spelling of
// the spell package, the lowering [Lower], the extend [Cluster] and its
// [Groups], the statement printer [Scaffold], the import renderer
// [Imports], and the finalising step [Check].
//
// # Layout
//
// The templates write the layout of buf format: two spaces for each level
// of a block, a blank line between two declarations at file level and
// between the parts of the header, the imports and the file options in the
// order of buf format, an empty block as {}, and two or more compact options
// on lines of their own. A block has no blank line. The value of an option
// is written as the generator states it.
//
// # Channels
//
// A generator states each construct of protobuf through one channel of the
// emit model:
//
//   - a field of the model states a fact that other languages share, such
//     as the optional form for presence and the value of a field for its
//     default;
//   - the tag entry number states the wire number of a field;
//   - an annotation states an option, and an annotation whose name starts
//     with protobuf: is a marker that states a keyword or a statement;
//   - the header of a file, an emit.File in a unit, states the syntax, the
//     package, the imports and the options of the file.
//
// # Versions
//
// The marker protobuf:syntax of the header selects the version of a file,
// and a file without the marker is proto3. [Scope] resolves the features of
// each declaration as the protobuf frontend does, and the vocabulary writes
// each construct in the version of its file. A construct that the version
// does not have refuses the declaration at the render.
//
// # Dependency position
//
// lang/protobuf/backend imports the sdk's backend, emit, render and symbol
// facades, the module root for the language identity, the markers, the
// features and the well-known types, lang/protobuf/spell for the spoke and
// the spelling of names, files and packages, lang/protobuf/internal/parse
// for the check of a written file, the naming and spoke helpers of
// eidos-lang, and the Go stdlib.
package backend
