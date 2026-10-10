// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads protobuf schemas into the symbol graph.
//
// [New] returns the frontend a composition registers. It parses
// through the experimental parser of bufbuild/protocompile and lowers
// the tree into the node model.
//
// # Unit grain
//
// One file is one unit, which is protobuf's own compilation grain.
// The file's package clause is the namespace its declarations load
// under, split on its dots into the package path, and the load's
// splice merges files sharing a package.
//
// # Versions
//
// The frontend loads proto2, proto3 and the editions 2023, 2024 and
// 2026, and reads a file's version from its syntax or edition
// statement. A file without a statement is proto2. Edition 2026 has the
// grammar of Edition 2024, which the parser implements, so a 2026 file
// parses from a copy whose edition value is 2024, and every position is
// the position in the file. The frontend reports a statement outside
// the table under [UnknownEdition], and does not load any declaration
// of the file.
//
// The frontend resolves four features. Each feature starts from the
// version's default, as descriptor.proto of protobuf v36.0 declares it.
// The options of the file, of each enclosing message, of a oneof and of
// the declaration itself then override it in that order. proto2 and
// proto3 set some features through their syntax: a required label, a
// group and an optional label.
//
// # Projection
//
//   - A message is a struct, a nested message a nested type.
//   - A oneof is a sum whose variants are its members.
//   - A proto2 group is a nested message of the group's name, and a field
//     of the group's name in lower case, marked delimited.
//   - An enum is the enum kind, each variant with its declared number,
//     and an enum whose resolved enum_type is CLOSED is marked closed.
//   - A service is an interface, an rpc an abstract method, which is
//     asynchronous where its response is not a stream.
//   - repeated is a list, map the map form over key and value, and
//     a streaming rpc side an asynchronous stream.
//   - A singular field with explicit presence is the optional form:
//     proto2's and proto3's optional label, and an edition's resolved
//     field_presence. A required field has the label stamp.
//   - A field of a message that is encoded delimited is marked
//     delimited, and a message or an enum that another file cannot
//     reference is marked local.
//
// # Comments
//
// Comments attribute the way protoc's source info attributes them.
// The group directly above a declaration is its documentation, and
// a group a blank line separates from it is detached and documents
// nothing. The comment after a declaration's last token, or after
// the opening brace of a message, an enum, a oneof, a service or an
// rpc body, is its trailing comment. The syntax statement's comments
// are the file's and the package statement's are the package's. A
// carrier in any of these attaches to the declaration, and one in a
// comment no declaration takes reports under [UnaddressedCarrier].
//
// # Resolution
//
// Resolution lists the candidates in protoc's probe order, one tier per
// scope: the message a reference is written in, each enclosing
// message, the file's namespace, each namespace above it, then the
// root. A oneof is not a scope. A reference with a leading dot is fully
// qualified, and resolution does not probe the enclosing scopes for it. A well-known type does not resolve against the graph. An
// import option imports the options of a file and none of its types,
// so resolution does not search that file for a declaration.
//
// # Refusals
//
//   - [UnknownEdition]: a syntax or an edition outside the frontend's
//     table of versions.
//   - [RefusedExtension]: an extend block changes a declaration the
//     file does not declare.
//   - [UnparsedFile]: a syntax error, positioned, the file still
//     contributing what the parser recovered. The parser's warnings,
//     which are about a schema's style, do not report.
//   - [BadCarrier] and [UnaddressedCarrier]: a directive carrier the
//     grammar refused, or one on a subject the model cannot address.
//
// # Dependency position
//
// lang/protobuf/frontend imports the sdk facade, lang/protobuf, the Go
// stdlib, and the ast, parser, report, seq, source, source/length, token
// and token/keyword packages of protocompile's experimental compiler.
package frontend
