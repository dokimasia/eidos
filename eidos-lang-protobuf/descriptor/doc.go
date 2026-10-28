// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package descriptor loads protobuf descriptors into the symbol graph.
//
// [New] builds the frontend through the kit. A host of a protoc plugin
// registers it in place of the source frontend of the satellite: the tree
// contains each file to generate as a serialized FileDescriptorProto at its
// proto path, the store [Store] contains every other file of the request,
// and [Registry] contains the FileDescriptorSet of every file.
//
// # Graph
//
// The frontend lowers a descriptor into the graph that the source frontend
// lowers from the source of the same file: the packages, the declarations,
// their positions, the attached directives and the stamped facts. A
// position is the location of source_code_info with one added to its line
// and its column. A type name of a descriptor is fully qualified, and the
// resolution of the source frontend resolves it.
//
// # Comments
//
// The frontend passes the leading and trailing comments of each location
// through the unit and attaches their carriers. It refuses a carrier in a
// detached comment under [frontend.UnaddressedCarrier], and a carrier that
// the grammar refuses under [frontend.BadCarrier]. A file to generate
// without source_code_info reports [MissingSourceInfo].
//
// # Units
//
// One file is one unit, and every unit reads [Registry] as a shared input.
// The first dependency round returns one unit for each file of [Store], so
// the declarations of an imported file are dependencies.
package descriptor
