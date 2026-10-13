---
adr: 0033
title: The config file of a binary keeps the departures of its implementation
status: Accepted
date: 2026-10-12
supersedes: none
superseded-by: none
rfc: RFC-0045
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0033: The config file of a binary keeps the departures of its implementation

## Status

Accepted

## Context

The accepted design adds the key `extensions` with `Member.Extension`, the name and the error of a
config file, the tag argument of `jsonschema.Of`, `Builder.Plugins`, `cli.Schema` and the kernel
command `schema`. Its components table puts the schema of the options of a plugin in the command
line, and its text makes an option with the mark `schema:"required"` a required property.

The plugin package declares the tag contract of an options struct and the checks of the contract.
A composition can set a required option in Go, and the binary then accepts a config file without
it. The decoder reads an extension through the YAML tag of a field, so it cannot select every name
that a consumer can choose. Each change is listed here, because an accepted design is not amended.

## Decision

We keep the code as built where it departs from the accepted design. The departures are these.

| Design | Code | Reason |
|---|---|---|
| The command line builds the schema of the options of each plugin | `plugin.OptionsSchema` returns the schema of the options of one plugin, and `cli.Schema` collects the schema of each plugin | The plugin package declares the tags `opt`, `doc` and `schema` and the checks of the contract. The command line would otherwise read the three tags in a second place |
| An option with the mark `schema:"required"` is a required property | An option with the mark is a required property while the composition leaves it at the zero value of its type | The schema accepts every config file that the binary accepts. A binary whose composition sets the option accepts a file without it |
| `extensions` maps any name to any value | `Member.Extension` panics for an empty name, for the name `-` and for a name with a comma | The decoder reads the extension through a YAML tag. It leaves a field out for the tag `-` and ends the key of a tag at a comma, so it cannot select an extension of such a name. A name is a constant of the binary, so a panic reports a defect of the binary as the other misuse panics of the command line do |

## Alternatives Considered

### The tags of an option in the command line

`cli.Schema` could read the tags of each options struct itself. The command line and the plugin
package would then both contain the names of the three tags, and a change of the contract would need
both.

### A required property for every marked option

The schema could require each marked option, as the design states. A repository whose composition
sets the option would then see an editor refuse a config file that the binary runs.

### An error for a name that the decoder cannot select

`Extension` could return an error for such a name. The caller passes a constant, so the error would
report a defect of the binary at run time, and each caller would check an error that the input of
the user cannot cause.

## Consequences

- The SDK facade re-exports `plugin.OptionsSchema`.
- The schema of a binary depends on the options that its composition sets in Go, so a binary that
  sets a required option prints a schema that does not require it.
- A consumer chooses an extension name that is not `-` and has no comma.

## References

- [RFC-0045](../rfc/0045-the-config-file-of-a-binary.md)
