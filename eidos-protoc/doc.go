// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package protoc runs an eidos composition as a protoc plugin, so protoc,
// buf generate and a remote plugin of the BSR run eidos generators over the
// descriptors of a schema.
//
// A plugin binary is one call of [Main] with the [Compose] of its
// generators, in the repository of the generators:
//
//	func main() { protoc.Main(acme.Compose) }
//
// # The run
//
// [Run] reads a CodeGeneratorRequest and writes a CodeGeneratorResponse.
// The files to generate are the tree of the run, as serialized
// FileDescriptorProtos at their proto paths. Every other file of the
// request is in the store of the descriptor frontend, which loads it as a
// dependency. The run is cold and writes into memory. The response contains
// every file of the run, with the frame of the brand.
//
// # The parameter
//
// The parameter of the request, such as the opt entries of buf.gen.yaml, is
// the config of the composition: <plugin>.<option>=<value> sets an option,
// and policy.<key>=<choice> selects the choice of a policy. A value is JSON
// where it is JSON and a string otherwise.
//
// # Errors
//
// The Error findings of a run and an entry of the parameter that names
// nothing go into the error field of the response, which fails the
// generation of protoc and buf. Every Warning and Info goes to standard
// error. A request that does not decode, a composition that does not build
// and a run that fails for another reason end [Main] with status 1.
//
// # Dependency position
//
// protoc imports core/diag, core/output, core/plugin, core/position,
// core/workspace, lang/protobuf/descriptor, google.golang.org/protobuf and
// the Go stdlib.
package protoc
