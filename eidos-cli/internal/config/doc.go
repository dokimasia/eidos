// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package config decodes the YAML config file of a binary built with eidos.
//
// A config file configures one workspace or lists workspaces.
// [Decode] decodes a file into a [File]. It returns an [Error] with a line
// number for each fault in the file. [Document.Apply] sets the values of a
// document on a [workspace.Builder]. [Schema] returns the JSON Schema of the
// format.
//
// # The format
//
// Every file has the key version with the value 1. A file with the key
// workspaces is a [List], and any other file is a [Document]. A list has no
// keys other than version and workspaces.
//
// The decoder converts the YAML 1.1 words yes, no, on and off to booleans
// only for a boolean field. A string field receives them as strings.
//
// # Dependency position
//
// cli/internal/config imports core/workspace, core/layout, core/ledger,
// core/directive, core/plugin, core/symbol, core/jsonschema,
// go.yaml.in/yaml/v3 and the Go stdlib.
package config
