---
rfc: 0045
title: The config file of a binary
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0033
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0045: The config file of a binary

## Summary

The config file of a binary refines its composition. The decoder refuses every key that the format
does not have, so a consumer cannot keep a setting of its own in the file. The name of
the file and the error of a fault are internal to the command line, so every consumer and test kit
spells the name again. The published JSON Schema allows any key under the options of a plugin, so an
editor cannot complete an option. We propose a reserved key for the settings of a consumer, an
exported name and error, and a kernel command that prints the schema of the binary with the options
of its plugins. We refuse a key for the verbosity of the output.

## Motivation

- A consumer with settings of its own needs a second file or the environment, because the decoder
  refuses an unknown key.
- The name of the config file is `ledger.StateDir(brand) + ".yaml"`. The command line spells the
  rule in two files, and the acceptance kit, two of its test hosts and the tests of the reference
  ensemble spell it again.
- `config.Error` has the file, the line and the message of a fault. The package is internal, so a
  consumer that shows the fault in an editor parses the message.
- `options` in the published schema is an object with any keys. The plugin document promises that an
  editor completes the options of a plugin as it completes the keys of a workspace.
- The previous version of eidos read a key `verbose` from its config file.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/jsonschema` | `Of` takes the struct tag that contains the key of a field. A `time.Duration` allows a text and an integer, and a type whose pointer implements `encoding.TextUnmarshaler` is a text |
| `eidos-core/workspace` | `Builder.Plugins` lists the plugins that a composition declares, without a build |
| `eidos-cli/internal/config` | the document key `extensions` |
| `eidos-cli` | `ConfigName`, `ConfigError`, `Member.Extension`, `Schema` and the kernel command `schema` |
| `eidos-cli/acceptancetest` | the checks and the test hosts name the config file through `ConfigName`, and the checks of every kernel command cover `schema` |
| `eidos-reference` | the tests name the config file through `ConfigName` |
| `docs/architecture` | the CLI document, the distribution document and the plugin document |

### Invariants

1. The kernel decodes every key of a config file strictly, except the values under `extensions`,
   which it never reads.
2. One function spells the name of the config file of a brand.
3. The schema that `schema` prints accepts every config file that the binary accepts.
4. The kernel commands are eight: run, plan, explain, prune, doctor, watch, version and schema.

### The settings of a consumer

```yaml
version: 1
options:
    stubgen: {suffix: _stub}
extensions:
    reference:
        canary: {rounds: 4}
```

`extensions` maps a name to any value. The decoder keeps the value of each name, and a command of the
consumer reads one through a member that `Open` returns:

```go
// Extension decodes the value of the extension name of the config file of
// m into v, with the strict rules of the file. It reports false where m has
// no config file or the file has no such extension.
//
// Error modes: an error that wraps one *ConfigError for each key that v
// does not declare and for each value that does not decode.
func (m Member) Extension(name string, v any) (bool, error)
```

A member of a list reads the extensions of its own config file. A list of workspaces has no
extensions. The JSON Schema of the file allows any value under `extensions`.

The YAML decoder accepts an unknown key when it decodes one node of a document. It refuses the key
when it decodes the whole file. So `Extension` decodes the bytes of the file again into a type that
has the type of v at the place of the extension and a node at the place of each other key. A fault
then has its line in the file. The member keeps the bytes of its config file for this decode.

### The name and the error of a config file

```go
// ConfigName returns the name of the config file of a brand: the name of
// the state directory of the brand and the extension .yaml, such as
// .acme.yaml.
func ConfigName(brand output.Brand) string

// ConfigError is a fault in a config file: the file, the line and the
// message of the fault.
type ConfigError = config.Error
```

`Open`, `Configure` and `Member.Extension` return errors that wrap a `*ConfigError` for each fault of
a file. The command line, the acceptance kit, its test hosts and the tests of the reference ensemble
name the file through `ConfigName`.

### The schema of a binary

```go
// Schema returns the JSON Schema of the config file of the binary that
// compose composes, draft 2020-12, as indented JSON that ends with a
// newline. It is the published schema with one property under options
// for each plugin of the composition that declares options. Each such
// property is a closed object with one property for each option.
func Schema(compose Compose) []byte
```

The property of an option has the key of its `opt` tag, the description of its `doc` tag and the type
of its field. An option with the tag `schema:"required"` is a required property. The object under
`options` admits more names beside the plugins of the composition, because a config file adds the
declarative plugins of a plan directory.

`Schema` reads the plugins of the composition before a build, because a composition with a required
option does not build without its config file:

```go
// Plugins returns the plugins that the composition declares, each once by
// name, in the order of the roster: the annotators, then the generators
// and the backend of each plan, then the checks. It does not apply the
// config and does not build the composition.
func (b *Builder) Plugins() []plugin.Plugin
```

The kernel command `schema` prints the schema and does not read a config file. Text output writes the
schema, and JSON output writes a `schema` event whose field `schema` is the schema object. A user
writes the schema into the repository and points an editor at it:

```sh
acme schema > .acme.schema.json
```

`jsonschema.Of` takes the struct tag that contains the key of a field: `yaml` for the document of a
config file, and `json` for the value of an option, which the options step decodes through JSON. A
field without a key has the key that the decoder of the tag gives it: the name of the field in lower
case for `yaml`, and the name of the field for `json`. `Of` gives a `time.Duration` the types string
and integer, because both decoders take a duration text and a number. It gives a type whose pointer
implements `encoding.TextUnmarshaler` the type string, unless the type implements `Schemer`.

### The verbosity of the output

Verdict: out. The config file states the composition that everyone who runs the binary in the
repository shares. Verbosity is the preference of one person for one command, so it belongs to the
flags. A host that cannot pass a flag, such as an editor integration, would reverse the verdict, and
an environment variable would then serve it better than the shared file.

### Failures

| Condition | Report |
|---|---|
| A key under an extension that the type of the consumer does not declare | a `*ConfigError` at the line of the key |
| A value under an extension of another type than the field | a `*ConfigError` at the line of the value |
| A plugin whose options struct breaks the tag contract | `Schema` leaves the plugin out, and Build reports the fault |

### Migration

1. Give `jsonschema.Of` the tag argument and the two text types.
2. Add `ConfigName` and `ConfigError`, and name the file through `ConfigName` in every package.
3. Add `extensions` and `Member.Extension`.
4. Add `Builder.Plugins`, `Schema` and the command `schema`, and cover the command in the checks of
   the acceptance kit.
5. State the key, the name, the error and the command in the documents.

## Alternatives

### Keys with a prefix

A consumer could write keys with a prefix, such as `x-canary`, at any level of the file. The decoder
would then accept an unknown key with the prefix at every level. One reserved key keeps every other
key strict.

### A file of the consumer

A consumer could read a second file beside the config file. Its settings would then live apart from
the composition that they refine, and each consumer would repeat the search for the file.

### The schema as a flag of version

`version --schema` could print the schema. The output of `version` is a list of events about the
binary, and the schema is one document that an editor reads.

### A generated schema in the repository of the consumer

A consumer could generate the schema at build time. The generator needs the composition, so it runs
the binary anyway, and every consumer writes the same generator.

## Drawbacks

- The kernel commands become eight, so the list of commands of a host gains one line.
- `jsonschema.Of` takes a second argument, so its callers change.
- The schema of a binary is a document that each repository writes again after a change of the
  options of a plugin.

## Open questions

None.

## Unresolved and future work

None.

## References

- The command kernels and the config file: RFC-0021.
- The required mark of an option: RFC-0043.
