// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package config decodes the YAML config file of a binary built with eidos.
//
// A config file configures one workspace or lists workspaces.
// [Decode] decodes a file into a [File], and [DecodeMember] decodes the
// config file of a member of a list into a [Document]. Each returns an
// [Error] with a line number for each fault in the file. [Document.Apply]
// sets the values of a document on a [workspace.Builder], and loads the
// declarative plugins that the plans of the document list. [Schema] returns
// the JSON Schema of the format.
//
// # The format
//
// Every file has the key version with the value 1. A file with the key
// workspaces is a [List], and any other file is a [Document]. A list has no
// other key of a document.
//
// The command line of a binary reads the values of its options from the
// config file of the run, under the keys that [Keys] does not return. Decode
// skips those keys. The config file of a member of a list sets no options,
// so DecodeMember returns an Error for each of them.
//
// The decoder converts the YAML 1.1 words yes, no, on and off to booleans
// only for a boolean field. A string field receives them as strings.
//
// # Dependency position
//
// cli/internal/config imports core/workspace, core/layout, core/ledger,
// core/directive, core/plugin, core/symbol, core/jsonschema, declarative,
// go.yaml.in/yaml/v3 and the Go stdlib.
package config
