// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package specfront loads the specs of the shape catalog. Each shape,
// mixin and contract has one YAML file below spec/ of the catalog module.
//
// The frontend that [New] returns declares each spec as a struct of the
// language [Lang], with one field for each param and each binding. It
// stamps the values that the declarations do not state under the keys
// that [Keys] registers. The registry generator reads the graph and
// writes the registry of the catalog.
//
// # The documents
//
// A spec decodes into the document of its directory: [Shape] in
// spec/shapes, [Mixin] in spec/mixins and [Contract] in spec/contracts.
// The decoder refuses an unknown section, and each vocabulary type, such
// as [ParamType] and [Arity], refuses a spelling outside its constants.
// [Schema] derives the published JSON Schema from the same types, so an
// editor and the decoder check the same rules.
//
// # Failure semantics
//
// Every fault of a spec is an Error under [SpecInvalid], at the key or the
// value at fault. The registry generator reports [SpecDuplicate] and
// [PrecedenceCycle], which it finds in the identifiers and the precedence
// of the specs together. An Error in the load commits nothing, so an
// invalid spec writes no file.
//
// # The key layout of the catalog
//
// [Namespace], [Summaries] and the Part constants are the spellings of the
// keys of the catalog, which the generator writes into the registry. The
// frontend refuses a spec whose name is the part of a summary key, and a
// param or a binding whose key is in [ReservedKeys].
//
// # Dependency position
//
// The package imports the diag, directive, frontend, jsonschema, meta,
// node, plugin, position and symbol packages of the SDK facade, and
// go.yaml.in/yaml/v3.
package specfront
