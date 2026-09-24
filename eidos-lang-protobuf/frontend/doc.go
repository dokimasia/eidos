// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package frontend loads protobuf schemas into the symbol graph.
//
// [New] returns the frontend a composition registers. It parses
// through bufbuild/protocompile's syntax parser and lowers the tree
// into the node model.
//
// # Unit grain
//
// One file is one unit, which is protobuf's own compilation grain.
// The file's package clause is the namespace its declarations load
// under, split on its dots into the package path, and the load's
// splice merges files sharing a package.
//
// # Projection
//
//   - A message is a struct, a nested message a nested type.
//   - A oneof is a sum whose variants are its members.
//   - An enum is the enum kind, each variant with its declared
//     number.
//   - A service is an interface, an rpc an abstract method.
//   - repeated is a list, map the map form over key and value, and
//     a streaming rpc side a stream.
//   - A field with presence is the optional form: proto2's and
//     proto3's optional label, and an edition's explicit
//     features.field_presence, resolved from the field up through
//     its messages to the file and then the edition default.
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
// Resolution names candidates in protoc's probe order, one tier per
// scope: the message a reference is written in, each enclosing
// message, the file's namespace, each namespace above it, then the
// root. A oneof is no scope. A leading dot states the whole path and
// probes nothing outward, and a well-known type never resolves.
//
// # Refusals
//
//   - [RefusedExtension]: an extend block changes a declaration the
//     file does not declare.
//   - [RefusedGroup]: a group is one declaration the model
//     represents as two.
//   - [UnparsedFile]: a syntax error, positioned, the file still
//     contributing what the parser recovered.
//   - [BadCarrier] and [UnaddressedCarrier]: a directive carrier the
//     grammar refused, or one on a subject the model cannot address.
//
// # Dependency position
//
// lang/protobuf/frontend imports the sdk facade, lang/protobuf and
// protocompile's ast, parser and reporter packages.
package frontend
