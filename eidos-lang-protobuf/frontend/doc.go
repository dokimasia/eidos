// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads protobuf schemas into the symbol graph.
//
// [New] builds the frontend through the kit, the value that a composition
// registers. The frontend parses through the experimental parser of
// bufbuild/protocompile and lowers the tree into the node model.
//
// # Unit grain
//
// One file is one unit, which is the compilation grain of protobuf. The
// package clause of the file is the namespace under which its declarations
// load, split at its dots into the package path, and the splice of the load
// merges the files of one package.
//
// # Versions
//
// The frontend loads proto2, proto3 and the editions 2023, 2024 and 2026,
// and reads the version of a file from its syntax or edition statement. A
// file without a statement is proto2. Edition 2026 has the grammar of
// Edition 2024, which the parser implements, so a 2026 file parses from a
// copy whose edition value is 2024, and every position is the position in
// the file. The frontend reports a statement outside the table under
// [UnknownEdition], and does not load any declaration of the file.
//
// The frontend resolves four features. Each feature starts from the
// default of the version, as descriptor.proto of protobuf v36.0 declares
// it. The options of the file, of each enclosing message, of a oneof and of
// the declaration itself then override it in that order. An option sets a
// feature by its own name, as features.field_presence = IMPLICIT does, or
// through a message literal, as features = { field_presence: IMPLICIT }
// does. proto2 and proto3 set some features through their syntax: a
// required label, a group and an optional label.
//
// # Projection
//
//   - A message is a struct, and a nested message is a nested type.
//   - A oneof is a sum whose variants are its members.
//   - A proto2 group is a nested message with the name of the group, and a
//     field with the name of the group in lower case, marked delimited.
//   - A field of an extend block is a field whose Receives is the extended
//     message as written. At file level it is a declaration of the file,
//     and inside a message it is a nested declaration of the message, so it
//     is no field of the message that declares it. The comments above the
//     block document its first field.
//   - An enum is the enum kind, each variant with its declared number, and
//     an enum whose resolved enum_type is CLOSED is marked closed.
//   - A service is an interface, and an rpc is an abstract method, which
//     is asynchronous where its response is not a stream.
//   - repeated is a list, map is the map form over the key and the value,
//     and a streaming side of an rpc is an asynchronous stream.
//   - A singular field with explicit presence is the optional form: the
//     optional label of proto2 and proto3, and the resolved field_presence
//     of an edition. A required field has the label stamp.
//   - A field of a message that is encoded delimited is marked delimited,
//     and a message or an enum that another file cannot reference is
//     marked local.
//
// # Comments
//
// The frontend attributes comments as the source info of protoc does. The
// group directly above a declaration is its documentation, and a group
// that a blank line separates from it is detached and documents nothing.
// The comment after the last token of a declaration, or after the opening
// brace of the body of a message, an enum, a oneof, a service or an rpc,
// is its trailing comment. The comments of the syntax statement document
// the file, and the comments of the package statement document the
// package. A carrier in any of these attaches to the declaration. A
// carrier in a comment that no declaration takes, or on an import, reports
// under [UnaddressedCarrier].
//
// # Resolution
//
// Resolution lists the candidates in the probe order of protoc, one tier
// per scope: the message in which a reference is written, each enclosing
// message, the namespace of the file, each namespace above it, and then
// the root. A oneof is not a scope. A reference with a leading dot is
// fully qualified, and resolution does not probe the enclosing scopes for
// it. A well-known type does not resolve against the graph. An import
// option imports the options of a file and none of its types, so
// resolution does not search that file for a declaration.
//
// A proto import names a file, so the frontend has the importer role: it
// returns the import among the imports of a referencing file that names
// the file of a declaration.
//
// # Refusals
//
//   - [UnknownEdition]: a syntax or an edition outside the table of
//     versions of the frontend.
//   - [UnparsedFile]: a syntax error at its position. The file still
//     contributes what the parser recovered. The warnings of the parser,
//     which are about the style of a schema, do not report.
//   - [BadCarrier] and [UnaddressedCarrier]: a directive carrier that the
//     grammar refused, or a carrier on a subject that the model cannot
//     address.
//
// # Dependency position
//
// lang/protobuf/frontend imports the sdk facade, lang/protobuf, the Go
// standard library, and the ast, parser, report, seq, source,
// source/length, token and token/keyword packages of the experimental
// compiler of protocompile.
package frontend
