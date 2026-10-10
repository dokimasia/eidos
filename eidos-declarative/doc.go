// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package declarative loads declarative plugins. A declarative plugin is a
// directory with a manifest and template trees, and it runs as a generator
// without Go.
//
// [Load] reads a plugin directory. It decodes the manifest plugin.yaml,
// checks it, and builds an ordinary generator through the authoring surface
// of the kernel. A workspace schedules, configures, fingerprints and checks
// the generator as it does a typed plugin, and a config file adds it to a
// plan that the binary marks extensible. [Schema] returns the JSON Schema of
// the manifest, which an editor completes the manifest against.
//
// # The plugin directory
//
// The directory contains plugin.yaml and, below templates, one template
// tree for each target, such as templates/golang and templates/typescript.
// A plugin without templates serves every target, and its rules emit no
// body. Load reads no other file, and it opens every file through
// [os.OpenInRoot], so a symbolic link that leaves the directory fails the
// read.
//
// # The manifest
//
// The manifest has the keys format, name, version, priority, provides,
// requires, directives, outputs, options and rules. The format is 1. The
// decoder refuses an unknown key, and [Error] positions each fault at its
// file and line. Load refuses every declaration that the authoring surface
// refuses with a panic, so a manifest never makes Build panic.
//
// A rule matches subjects of one kind, under a directive of the manifest or
// for every subject of the kind in the sources of a plan, and emits one
// declaration for each match into one output family:
//
//   - A mirror of a function is a function with the subject's signature.
//   - A mirror of an interface is an interface with the subject's
//     effective methods and properties.
//   - An implementation of an interface is a struct with the field next of
//     the subject's type and a method for each effective method of the
//     subject. An interface with a property has no implementation.
//   - A mirror of a struct is a struct with the subject's effective fields.
//
// The name of each declaration is the family's word joined to the
// subject's name, as in LedgerAudit. The rule's template renders the body
// of each function and method that it emits, over the declaration as .Decl
// and the subject, the params of the gating directive and the plugin's
// options as .Data. The render executes the template with missingkey=error,
// so a key that the data does not contain fails the template.
//
// # Files that a template writes
//
// A family with a file template claims its files. The rules emit records
// into such a family: each declaration that a rule emits, with the same
// data beside it. The family's template then writes each file whole from
// its records, such as a Go test file with a test function for each
// subject, through the target's spelling of types and its import set. A
// record declares no name of its file, so no reference, collision check
// or export reads it.
//
// # The version
//
// The version of a declarative plugin is the version of its manifest, a
// plus sign, and the hexadecimal SHA-256 of the manifest and of every
// template file. An edit of either changes the fingerprint of the
// composition, so the next run runs cold.
//
// # Errors
//
// [Load] returns one *[Error] for each fault of a plugin directory, joined.
// A run reports two codes at a subject, and the plugin emits nothing for
// that subject:
//
//   - [IncompleteMembers], DECLARATIVE-0001, at a subject whose members the
//     rules of its language cannot list in full.
//   - [UndelegatedProperty], DECLARATIVE-0002, at an interface with a
//     property under a rule that emits implementations.
//
// # Dependency position
//
// eidos-declarative imports the kernel's root authoring package, core/diag,
// core/directive, core/emit, core/jsonschema, core/node, core/plugin,
// core/rules, core/symbol, go.yaml.in/yaml/v3 and the Go stdlib. The
// command kernels of eidos-cli import it to load the plugins of a config
// file. Nothing in the kernel imports it.
package declarative
