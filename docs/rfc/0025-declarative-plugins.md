---
rfc: 0025
title: Declarative plugins and their manifest
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-10
updated: 2026-10-11
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0018, ADR-0019
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0025: Declarative plugins and their manifest

## Summary

A declarative plugin is a directory with the manifest `plugin.yaml` and
template trees below `templates/`. It runs as a generator, and its author
does not write Go. The package `go.dokimi.dev/eidos/cli/declarative` loads
such a directory. It decodes the manifest and reports each fault at its file
and line. Then it lowers the manifest through the authoring surface into an
ordinary plugin. The workspace schedules, configures, fingerprints and checks
the plugin in the steps that it runs for a typed plugin.

The manifest contains the plugin's name and version, its directives, its
output families, its ordering, its options and its rules. A rule mirrors a
subject of one kind into a family of the plugin. For the body of each
function and method that a rule emits, the render executes a template of the
plugin from the tree of the plan's target.

These changes complete the tier:

- The config file adds a declarative plugin to a plan under
  `plans.<name>.plugins`. Build accepts it only for a plan that the binary
  marks `Extensible`.
- The plugin's version ends in a SHA-256 digest of the manifest and of every
  template file. An edit of either changes the composition's fingerprint.
- The CLI module publishes the manifest's JSON Schema as
  `plugin.schema.json`. The schema is in draft 2020-12, and an editor
  completes the manifest's keys through a modeline.
- The render pass executes every reference template with `missingkey=error`.

The conformance module runs the plugin acme-audit as a fixture in a Go plan
and a TypeScript plan, and `RunPluginSuite` passes over the plugin. The
fixture checks that the directive validates, that the templates render, and
that the output routes to the plugin's family.

## Motivation

### The specification has no rules for the tier

In the specification, a manifest has the keys `name`, `version`,
`directives`, `outputs`, `provides`, `requires`, `options` and `words`. The
loader presents each manifest as a Generator, a TemplateProvider and a
DirectiveProvider. These gaps stop such a manifest from generating anything:

- No key pairs a trigger with an output, so the generator does not emit a
  declaration, and no template of the plugin executes.
- A template renders the body of a function or a method. A plugin without Go
  code cannot build the function whose body the template renders.
- Each output has a `suffix`. A family of the authoring surface has a tag, a
  cardinality and a word, and the target spells the join. The files of one
  family are then `ledger_audit.go` in Go and `ledger.audit.ts` in
  TypeScript.
- `words` is a vocabulary for each target. The authoring surface does not
  have a word for each target, because a filename takes the family's word
  and a generated name takes the target's join.
- The manifest does not have a version of its format, so a binary cannot
  refuse a manifest whose format it does not read.

### A YAML manifest beside a kernel without dependencies

In the specification, the host is part of the kernel's `plugin/` package,
and the manifest is YAML. The kernel's runtime imports the standard library
alone, because the kernel is part of the supply-chain audit of every
consumer. The standard library does not read YAML. The CLI module reads the
config file through `go.yaml.in/yaml/v3` v3.0.5 and reports each fault at its
line.

### Users who do not write Go

Users of a consumer binary select among its plugins through the config
file. They cannot add a plugin without the Go toolchain, because the binary
composes its plugins at compile time. When the teams of an organisation need
a generator that their platform team's binary lacks, the platform team writes
it in Go, or the team builds a binary of its own. With a declarative plugin,
the team adds the generator to the binary's plans without either.

### The layer of each change

- A user writes the directory of each manifest in the config file. The
  decoder of the manifest belongs to the module that decodes YAML, which is
  `eidos-cli`.
- The lowering of a manifest uses only the authoring surface that the kernel
  exports, so the kernel does not change to build a declarative plugin.
- A generator that the config adds to a plan refines the composition, so the
  field belongs to `workspace.PlanConfig`, beside the other refinements.
- The render pass executes every reference template, so the option that
  refuses a missing key belongs there.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| The loader | eidos-cli/declarative, new | `Load`, `Schema`, `Error` and the code `IncompleteMembers` |
| The manifest schema | eidos-cli | `plugin.schema.json`, beside `config.schema.json` |
| The config file | eidos-cli/internal/config | the key `plans.<name>.plugins`, and an error from `Document.Apply` |
| Plan refinements | core/workspace | `Plan.Extensible` and `PlanConfig.Generators` |
| Reference templates | core/backend/render | the option `missingkey=error` |
| Conformance | eidos-conformance/declarative, new | the acme-audit fixture in a Go plan and a TypeScript plan |

### Invariants

1. A declarative plugin is an ordinary plugin. The loader builds it through
   `eidos.NewPlugin` and `Builder.Build`, and the workspace runs it through
   the steps of a typed plugin with the same declarations.
2. The loader refuses every declaration that `Builder.Build` refuses with a
   panic, so a manifest never makes Build panic. It reports each fault of a
   plugin directory as an `*Error` at a file and a line.
3. An edit of `plugin.yaml` or of a file below `templates/` changes the
   plugin's version. The composition's fingerprint folds the version, so the
   next run after the edit runs cold.
4. A declarative plugin cannot read a file outside its directory or run a
   program. The loader reads only the plugin's directory. The templates call
   the functions of the plan's backend, the render's builtins and the
   builtins of text/template, and these functions do not read a file, the
   environment or the network.
5. A declarative plugin is a generator. It does not register a metadata key,
   and it does not stamp a fact.
6. Each declaration of a declarative plugin is an emit value. The settle
   names it, checks it for collisions and records it in the plan's export,
   as it does for a typed plugin.
7. A plan runs a declarative plugin only where the binary marks the plan
   `Extensible`.

### The plugin directory

```text
acme-audit/
    plugin.yaml
    templates/
        golang/
            function.tmpl
            method.tmpl
        typescript/
            function.tmpl
            method.tmpl
```

- `plugin.yaml` is the manifest. The loader refuses a directory without it.
- Each directory below `templates/` is the template tree of one target, and
  its name is the target's name, such as `golang` or `typescript`. The loader
  declares each tree through `Builder.For(target, eidos.Templates(tree))`.
  Build refuses a plan whose target has no tree of the plugin, as it refuses
  such a plan for a typed plugin that declares its trees through `For`.
- A plugin without `templates/` does not declare a tree. Its rules do not
  emit a body, so the plugin serves every target.
- The loader does not read any other file, so a README beside the manifest
  does not change the plugin.

The loader opens every file through `os.OpenInRoot`, with the plugin
directory as the root. A symbolic link that leaves the directory fails the
read, and the loader reports the file.

### The manifest

```yaml
# yaml-language-server: $schema=../../../../../eidos-cli/plugin.schema.json
format: 1
name: acme-audit
version: 0.3.0
directives:
    - name: audit
      doc: Generates code that records each call of the declaration.
      params:
          - key: level
            type: string
            choices: [debug, info, warn]
            required: true
            doc: The level of each record.
          - key: sink
            type: reference
            resolve: callable-in-scope
            required: true
            doc: The function that receives each record.
outputs:
    - per: source
      word: audit
provides: [audit]
options:
    - key: redact
      type: bool
      default: true
      doc: When true, a record leaves out the arguments of the call.
rules:
    - on: interface
      directive: audit
      emit: implementation
      template: method.tmpl
    - on: function
      directive: audit
      emit: mirror
      template: function.tmpl
```

The loader decodes the manifest with `yaml.Decoder.KnownFields` set, so an
unknown key is an error. A key is optional unless the table marks it
required.

| Key | Value | The loader passes it to |
|---|---|---|
| `format` | `1`, required | nothing. The message of the Error for another value contains the formats that the binary reads |
| `name` | an identifier, required | `eidos.NewPlugin` |
| `version` | a string, required | `Builder.Version`, with the directory's digest appended |
| `priority` | an integer, 0 by default | `Builder.Priority(plugin.RoleGenerator, p)` |
| `provides`, `requires` | lists of capability labels | `Builder.Provides` and `Builder.Requires` |
| `directives` | a list of directives | one `directive.Schema` in one `eidos.Directive` wrapper for each directive |
| `outputs` | a list of families, required | `Builder.Output` |
| `options` | a list of options | `Builder.Options` |
| `rules` | a list of rules, required | `Builder.Handle` |

The name must be an identifier, because the prefixed spelling of a directive
starts with the plugin's name. An identifier is a letter followed by letters,
digits, hyphens and underscores.

#### Directives

| Key | Value | Field of `directive.Schema` or `directive.ParamSpec` |
|---|---|---|
| `name` | an identifier, required | `Schema.Name`, and `Schema.Plugin` takes the plugin's name |
| `doc` | a string, required | `Schema.Doc` |
| `deprecated` | a string | `Schema.Deprecated` |
| `params` | a list of params | `Schema.Params` |
| `params[].key` | an identifier, required | `ParamSpec.Key` |
| `params[].type` | `string`, `int`, `bool`, `list` or `reference`, required | `ParamSpec.Type` |
| `params[].of` | the type of the elements of a list: `string`, `int`, `bool` or `reference` | `ParamSpec.ListOf` |
| `params[].resolve` | `callable-in-scope`, `package-var`, `value-field`, `host-param`, `member-on-handle` or `type-in-scope` | `ParamSpec.Resolution` |
| `params[].required` | a boolean | `ParamSpec.Required` |
| `params[].choices` | a list of strings | `ParamSpec.Choices` |
| `params[].doc` | a string, required | `ParamSpec.Doc` |
| `params[].deprecated` | a string | `ParamSpec.Deprecated` |

The loader registers each schema in a registry of its own through
`Registry.Register`, and reports each refusal at the line of the directive.
A schema that registers there also registers in the workspace, unless another
plugin registers the same canonical spelling.

A manifest cannot set these fields of a schema:

- `Positional` and `Open`. Each value of a keyed param has its key, and a
  template reads each value by its key.
- `Variants`, `Roles` and `RolesRequired`. A variant or a role selects what a
  handler does, and a declarative plugin does not have a handler.
- `Repeatable`. Each instance gates one emission, and a second emission from
  one subject has the name of the first.
- `Negatable` and `Overrides`. The kernel's `skip plugin=` directive excludes
  a subject from the bare rules of a plugin.
- `Requires` and `ConflictsWith`. A constraint between directives does not
  change what a rule emits, and the fixture does not use one.
- The resolutions `metadata-key` and `diagnostic-code`. A declarative plugin
  does not have metadata keys or diagnostic codes.

#### Families and ordering

| Key | Value | Field of `plugin.Output` |
|---|---|---|
| `outputs[].tag` | a string, `""` by default | `Tag` |
| `outputs[].per` | `source`, `package` or `plan`, required | `Per`: `PerSource`, `PerPackage` or `PerPlan` |
| `outputs[].word` | a string, required | `Word` |

A Go plan writes the family of the manifest as `ledger_audit.go`, and a
TypeScript plan writes it as `ledger.audit.ts`, because each target spells
the filename from the word. The loader refuses a family without a cardinality
or a word, and two families with one tag.

Build checks the capability labels of `provides` and `requires` against the
providers of the composition, as it checks the labels of a typed plugin.
`priority` is the plugin's priority in the generator role.

#### Options

| Key | Value |
|---|---|
| `options[].key` | a lower-case letter followed by letters and digits, required |
| `options[].type` | `string`, `int`, `bool` or `list`, required. A list contains strings |
| `options[].default` | a value of the type. Without it, the default is the type's zero value |
| `options[].doc` | a string, required |

The loader builds the options struct with `reflect.StructOf`:

- Each option is one exported field. The field's name is the key with its
  first letter in upper case.
- The field's type is `string`, `int64`, `bool` or `[]string`.
- The field has the tags `opt` and `json` with the key, and the tag `doc`
  with the doc.

The loader sets each field to its default and passes a pointer to the struct
to `Builder.Options`. From there, Build treats the struct as it treats the
options of a typed plugin:

- It validates the struct through `plugin.ValidateOptions`.
- It populates the struct from `options.<name>` of the config file.
- It folds the struct's encoding through `plugin.EncodeOptions` into the
  fingerprint.

#### Rules

| Key | Value |
|---|---|
| `rules[].on` | the subject's kind: `function`, `interface` or `struct`, required |
| `rules[].directive` | the name of a directive of the manifest. A rule without it is bare, and it runs for every subject of its kind in the plan's sources |
| `rules[].output` | the tag of a family, `""` by default |
| `rules[].emit` | `mirror` or `implementation`, required |
| `rules[].template` | the path of a template file inside each tree |

The loader puts the rules of one directive into one `eidos.Directive`
wrapper, because the authoring surface registers each directive once. A rule
on functions becomes an `eidos.OnFunction` rule, and a rule on interfaces or
structs becomes an `eidos.OnInterface` or an `eidos.OnStruct` rule. Each rule
takes the Emitter as its effect.

A rule emits one declaration into its family for each match:

| `on` | `emit` | The declaration | Bodies |
|---|---|---|---|
| `function` | `mirror` | a function with the subject's type parameters, parameters and results | the template, required |
| `interface` | `mirror` | an interface with the subject's effective methods | none, so a template is an Error |
| `interface` | `implementation` | a struct with the field `next` of the subject's type, and a method for each effective method of the subject | the template, required |
| `struct` | `mirror` | a struct with the subject's effective fields | none, so a template is an Error |

The loader refuses any other pair at the rule's line. It also refuses a
second rule with the kind and the family of an earlier rule, because both
rules would emit one name for each subject.

Every emitted declaration follows these rules:

- **Names.** The declaration's name is `Emitter.JoinName` of the family's
  word and the subject's name, so the interface `Ledger` and the word
  `audit` give `LedgerAudit`. A member keeps its name.
- **Origins.** A name override on the subject or on a member applies to the
  mirror, because the origin of the declaration is the subject and the
  origin of each member is the member that it mirrors.
- **Members.** The loader reads the members through `rules.Bound.MembersOf`,
  so a mirror contains the members that an embedded or an extended type
  contributes. Where the member set has gaps, the loader reports
  `IncompleteMembers` at the subject and does not emit the declaration.
- **Types.** The loader translates each type through `Emitter.Type` with the
  member as the owner, and it does not emit a declaration with a type that
  the target cannot spell. `Emitter.Type` reports such a type under
  `RefusedType`.
- **Visibility and documentation.** The declaration has the subject's
  visibility and no documentation. Each member keeps the visibility and the
  documentation of the member that it mirrors. The field `next` has package
  visibility.
- **Receivers.** Each method of an implementation has a receiver, because
  its body reads the field `next` through the receiver. The receiver's name
  is the first letter of the struct's name in lower case. Where a parameter
  of a method of the struct has that name, the loader appends the smallest
  number that makes the name unique among the parameters. Go and Rust write
  the receiver, and TypeScript and Java do not.
- **Supertypes.** The loader adds the subject to the struct's `Implements`.
  TypeScript and Java write the clause, and Go satisfies the interface
  through the struct's methods alone.

#### Template data

The loader claims the body of each function and method that a rule emits
through `Emitter.Ref`. The render executes the template as it executes the
reference template of a typed plugin, over two fields:

- `.Decl` is the function or the method, with the names that the settle
  spelled for the target.
- `.Data` is the payload that the loader builds for the match.

```go
// data is the payload of each template reference of a declarative plugin.
type data struct {
    // Subject is the name of the subject in its source.
    Subject string
    // Directive contains each param of the gating directive by key. A
    // string and a reference are strings, an int is an int64, a bool is a
    // bool, and a list is a []any of its elements. An optional param that
    // the instance does not write is nil. Directive is empty under a bare
    // rule.
    Directive map[string]any
    // Options contains the plugin's options by key, with the values that
    // the config populated.
    Options map[string]any
}
```

The tree `templates/golang` contains `method.tmpl`:

```text
{{.Data.Directive.sink}}({{printf "%q" .Decl.Name}}, {{printf "%q" .Data.Directive.level}}{{if not .Data.Options.redact}}{{range .Decl.Params}}, {{.Name}}{{end}}{{end}})
{{slots}}{{if .Decl.Returns}}return {{end}}{{.Decl.Receiver.Name}}.next.{{.Decl.Name}}({{range $i, $p := .Decl.Params}}{{if $i}}, {{end}}{{$p.Name}}{{end}})
```

The file `svc/ledger.go` contains this interface:

```go
// Ledger posts amounts to accounts.
//
//+acme:audit level=info sink=record
type Ledger interface {
    // Post adds cents to account, and returns the identifier of the entry.
    Post(account string, cents int64) string
}
```

With `redact: false` in the config file, the Go plan writes
`svc/ledger_audit.go`:

```go
type LedgerAudit struct {
    next Ledger
}

// Post adds cents to account, and returns the identifier of the entry.
func (l LedgerAudit) Post(account string, cents int64) string {
    record("Post", "info", account, cents)
    return l.next.Post(account, cents)
}
```

When a template uses a key that a map of `.Data` does not contain, the
execution fails under `RefusedTemplate`, because the render executes every
reference template with `missingkey=error`. So the generated source never
contains the text `<no value>`.

### The version and the fingerprint

`Version` of a declarative plugin returns the manifest's `version`, a plus
sign, and the hexadecimal SHA-256 of the directory's fold. The fold contains
these parts in this order:

1. The bytes of `plugin.yaml`.
2. For each regular file below `templates/`, in the lexical order of the
   slash-separated paths, the path and the SHA-256 of the file's bytes.

The loader writes the length of each part in eight bytes in front of the
part, as the workspace does when it folds a template tree. The plugin of the
preceding section has the version `0.3.0+` followed by 64 hexadecimal
digits. In Semantic Versioning, such a suffix is build metadata.

The composition's fingerprint folds the version of each plugin, and the
sealed state records the fingerprint. So an edit of the manifest or of a
template makes the next run run cold. The workspace also folds every file of
each template tree at the start of each run. `watch` reads a manifest once at
its start, and it reads the templates at each run.

A unit test of the package asserts that each of these edits changes
`Version` and `Workspace.Fingerprint`:

- one byte of `plugin.yaml`
- one byte of a template
- the name of a template
- a new template file

The test also asserts that a new file outside `plugin.yaml` and `templates/`
does not change either.

### The loader

```go
// Package declarative loads declarative plugins. A declarative plugin is a
// directory with a manifest and template trees, and it runs as a generator
// without Go.
package declarative

// Load reads the plugin directory at the absolute path dir. It reads the
// manifest plugin.yaml and the template trees below templates, and returns
// the generator that eidos.NewPlugin builds from the manifest. Each call
// returns a new generator, because a plugin instance belongs to one
// workspace.
//
// Error modes: an error that joins one *Error for each fault of the
// directory, in the order of the files and their lines.
func Load(dir string) (plugin.Generator, error)

// Schema returns the JSON Schema of the manifest format, draft 2020-12, as
// indented JSON that ends with a newline. It builds the schema from the
// yaml, doc and schema tags of the manifest's types through jsonschema.Of,
// and returns the same bytes on each call.
func Schema() []byte

// Error is a fault in a plugin directory.
type Error struct {
    // File is the path of the file with the fault.
    File string
    // Line is the line of the fault, counted from 1. It is 0 for a fault of
    // a file as a whole.
    Line int
    // Msg describes the fault.
    Msg string
}

// Error returns the fault in the form FILE:LINE: MSG, or in the form
// FILE: MSG when Line is 0.
func (e *Error) Error() string

// IncompleteMembers reports a subject whose members the rules of its
// language cannot list in full. The finding is an Error at the subject, and
// the plugin does not emit a declaration for the subject.
var IncompleteMembers = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
    Number:  71,
    Meaning: "a declarative plugin cannot mirror a declaration whose members are incomplete",
})
```

The loader reports these faults:

| Fault | The position of the Error |
|---|---|
| No `plugin.yaml` | the directory, without a line |
| YAML that does not parse, an unknown key, a value of the wrong type or a key written twice | the line of the value, as the decoder reports it |
| A `format` other than 1 | the line of `format` |
| A name that is not an identifier, or an empty version | the line of the key |
| A schema that `Registry.Register` refuses | the line of the directive |
| A directive that no rule refers to | the line of the directive |
| A family without a cardinality or a word, or two families with one tag | the line of the family |
| An empty capability label | the line of the list |
| An option key that is not an identifier, a key written twice, or a default of another type | the line of the option |
| A rule with a pair of `on` and `emit` that the table does not contain, an unknown directive, or a tag that no family has | the line of the rule |
| A template on a rule that emits no body, or no template on a rule that emits one | the line of the rule |
| A template that a tree does not contain | the line of the rule |
| Two rules with one kind and one family | the line of the second rule |
| A file directly below `templates/`, or a link that leaves the directory | the file, without a line |

The YAML decoder reports a type mismatch and an unknown key with the line of
the value. The types of a directive, a family, an option and a rule record
the line of their YAML node, so the loader reports each of their checks at
that line.

Build reports the faults that depend on the composition, as it reports them
for a typed plugin:

- a capability without a provider
- two plugins with one name
- a plan whose target has no tree of the plugin
- an option value that does not populate the struct

`UnresolvedRef` reports a template that does not parse, and its message
contains text/template's file and line. `RefusedTemplate` reports a template
that fails at execution. Both are findings of a run and not faults of Build.

### Plans and the config file

`workspace.Plan` and `workspace.PlanConfig` gain one field each:

```go
type Plan struct {
    // Name, Sources, DependsOn, Generators, Backend and Layout are
    // unchanged.

    // Extensible admits generators from the config. Build appends the
    // generators of the plan's PlanConfig.Generators to Generators. For a
    // plan that is not extensible, Build refuses them with an error that
    // contains the plan's name.
    Extensible bool
}

type PlanConfig struct {
    // Sources, Disabled, Policy, Dir, ImportBase and Policies are
    // unchanged.

    // Generators are the generators that the config adds to the plan,
    // after the generators of the plan. A generator in the Generators of
    // two plans is one plugin that serves both plans.
    Generators []plugin.Generator
}
```

In a config file, the key `plugins` of a plan contains the plan's plugin
directories:

```yaml
version: 1
plans:
    go-audit:
        plugins: [plugins/acme-audit]
    ts-audit:
        plugins: [plugins/acme-audit]
options:
    acme-audit: {redact: false}
```

| Key | Value | Applies through |
|---|---|---|
| `plans.<name>.plugins` | a list of directories, absolute or relative to the root, with slashes | `declarative.Load`, then `PlanConfig.Generators` |

`Document.Apply` loads each directory once for each workspace, so a directory
in two plans is one plugin in both plans. Apply returns the joined errors of
the loads. `Open` reports them as config errors, and the command exits 64.
The JSON Schema of the config file gains the key.

The author of a binary with a fixed product does not mark a plan
`Extensible`, so the binary's users cannot change its output. The author of
a platform team's binary marks the plans that its teams may extend.

### The JSON Schema and editors

`Schema` builds the schema through `jsonschema.Of` from the manifest's Go
types, so each key that the decoder accepts is a property of the schema.
Each property has the field's `doc` tag as its description. The type of an
enumeration implements `jsonschema.Schemer`, and its schema has an `enum` of
the spellings. The key `format` has the schema `{"type": "integer",
"const": 1}`. The CLI module contains the schema as
`eidos-cli/plugin.schema.json`. A golden test compares the file with
`Schema`, and each release publishes the file beside `config.schema.json`.

The first line of a manifest refers to its schema through a modeline. The
first line of the fixture's manifest is:

```yaml
# yaml-language-server: $schema=../../../../../eidos-cli/plugin.schema.json
```

yaml-language-server reads the modeline, and it validates draft 2020-12 since
release 1.20.0. It resolves a relative path from the manifest's directory.
JetBrains IDEs read the same comment since 2023.3. The modeline of a user's
manifest refers to the file of the release that built the binary.

A new format is a new value of `format` with a new schema. The schema of one
format grows only by optional keys, so a manifest that is valid against it
remains valid.

### The fixture

`eidos-conformance/declarative` contains the plugin of the preceding
sections and a Go tree:

```text
testdata/
    tree/
        .acme.yaml                  the config file of the preceding section
        go.mod
        svc/ledger.go               Ledger under //+acme:audit level=info sink=record
        svc/record.go               record(op, level string, args ...any)
        ops/transfer.go             Transfer under //+acme:audit level=warn sink=record
        ops/record.go               record(op, level string, args ...any)
        plugins/acme-audit/         the plugin directory
    want/
        svc/ledger_audit.go
        ops/transfer_audit.go
        ops/transfer.audit.ts
```

The composition has the Go frontend and rules, the Go and TypeScript
targets, and two extensible plans. The plan `go-audit` reads `svc` and `ops`
toward the Go backend. The plan `ts-audit` reads `ops` alone toward the
TypeScript backend. A TypeScript implementation of `Ledger` would refer to a
Go declaration that the plan does not emit.

The tests do these checks:

- They open the tree through `cli.Open`, run it cold, and compare the
  written files with `want/`.
- They run the tree again, and assert that the warm run writes the same files
  and reports the same findings as the cold run.
- They run `plugintest.RunPluginSuite` over the generator that
  `declarative.Load` returns, with a fixture of the package `svc`.
- They assert that `level=trace` fails validation under `BadSpelling`, and
  that a `sink` that does not refer to a function fails under
  `UnresolvedReference`.

`RunPluginSuite` runs over the generator without a change to the suite. The
Go and TypeScript backends do not export a render language. So the suite's
fixture does not have one, and the suite skips the template lint. The
end-to-end runs render both templates of the Go tree and `function.tmpl` of
the TypeScript tree. No run of the fixture renders `method.tmpl` of the
TypeScript tree, because the plan `ts-audit` does not read an interface.

### Cost

- `Load` reads the manifest and every template file once, and computes one
  SHA-256 over them. It runs once for each workspace that `Open` builds.
- A match allocates the declaration, its members, the translated types, one
  `TemplateRef` for each body, and one payload with the map of the
  directive's params. The loader builds the map of the options once, at the
  first match.
- The render parses each template of a declarative plugin once in each
  worker, as it does for a typed plugin.
- This proposal does not include a measurement of the generator.
  `BenchmarkGenerate` measures the two rules of the fixture over the scaled
  corpus of 200,000 declarations, and its budget comes from that run.

### Migration

| Change | Affects | Step |
|---|---|---|
| `Plan.Extensible` and `PlanConfig.Generators` | a consumer that lets its users add plugins | `Extensible: true` on each plan that its users may extend |
| `plans.<name>.plugins` | config files and `config.schema.json` | a regeneration of the schema |
| An error from `Document.Apply` | `Open` | none outside the CLI module |
| `missingkey=error` | the reference templates of every plugin | none, because no plugin of this repository makes a template reference |
| `IncompleteMembers`, EID-0071 | the index of diagnostic codes | one row |
| The specification | the documents on plugins, on distribution and on the module topology, and the glossary | the manifest of acme-audit, the module of the host, and the opt-in of a plan |

## Alternatives considered

### The loader in the kernel, with a JSON manifest

The kernel's package `plugin/declarative` decodes `plugin.json` through
`encoding/json`. The kernel keeps its zero dependencies, and every consumer
receives the host.

**Why not:** a JSON file does not have comments, and the config file of the
same binary is YAML. `encoding/json` reports a byte offset and not a line.
Each fault would need a second pass over the file to find its line.

### The manifest types in the kernel, decoded by the CLI

The kernel declares the manifest's types and the lowering. The CLI decodes
YAML into those types.

**Why not:** a rule with an unknown directive has its line only in the CLI's
YAML node, so the kernel would report a path that the CLI maps back to a
line. The format would then belong to two modules, and the CLI is the only
caller of the lowering.

### A module of its own

`go.dokimi.dev/eidos/declarative` depends on the kernel and on the YAML
library.

**Why not:** the CLI module depends on both already, and a user adds a
plugin in the config file that the CLI reads. A module of its own adds a
release and a version range for one package.

### Claimed file templates

A rule appends its subject to a family as a record, and a file template for
each target renders the whole file from the records. Helm, Copier and
openapi-generator render files from data in the same way, and the
specification of the render describes such a claim.

**Why not:** the names in a file that a template writes are not emit
declarations, so the settle does not name, check or export them:

- A second plugin can declare a name that a plugin declared in the same
  package.
- A dependent plan cannot refer to the declarations of the file.
- `explain` cannot map an identifier to its origin.

The render pass has no claimed file template for any plugin.

### Weaving into the bodies of another plugin

A rule appends a statement to the prologue of each method that another
plugin emits. The typed acme-audit weaver of the end-to-end fixture does
this in Go.

**Why not:** a slot contains only scaffolding statements, and a scaffolding
statement has no template form. The manifest would need a syntax for a
statement and for the arguments of a call. An argument such as the method's
first parameter needs an expression, and the format would then freeze a
second statement vocabulary.

### A rules language

A rule contains expressions over the subject, with conditions and loops, as
the handler of a typed plugin does.

**Why not:** a manifest with logic is a program in a language without a
compiler, a debugger or tests. The tier stops at the conditionals of a
template. A plugin that needs more logic is a typed plugin.

### Process and WebAssembly plugins

protoc runs a plugin as a process that reads a request and writes a
response. sqlc runs a plugin as a process or as a WebAssembly module, and it
pins each module by its SHA-256.

**Why not:** typed metadata and the shared graph work in one process alone,
so plugins compose at compile time. A process plugin receives a serialized
graph. Its output depends on a program outside the binary.

### Configured plugins in every plan

Build adds a configured plugin to any plan. `Plan` does not have the field
`Extensible`.

**Why not:** the author of a binary with a fixed product could not stop the
binary's users from changing its output. The plan is the unit whose
generators the author of the binary chooses.

### An opt-in for the whole binary

`cli.Kernels` and `cli.Main` take an option that admits `plugins` in every
plan.

**Why not:** `Main` takes its extra commands as a variadic parameter, so the
option needs a second entry point. A binary that admits plugins into one plan
and not into another needs an opt-in for each plan.

### A copy of the templates at Load

`Load` reads every template into memory, and the render reads the copy. The
digest and the render then read the same bytes.

**Why not:** the workspace already folds each tree at the start of each run,
so an edit between the fold and the render makes the next run run cold. A
copy would make `watch` render the templates as they were at its start.

### A suffix for each output

Each output of the manifest has a tag and a suffix, as in the specification.

**Why not:** a family's word lets each target spell its join and its
extension, and a suffix is the spelling of one target. One word gives
`ledger_audit.go` in Go and `ledger.audit.ts` in TypeScript.

### Words for each target

The manifest has a word for each target below the target's tree, as in the
specification's `words: {marker: "Audit"}`.

**Why not:** a filename takes the family's word, and a generated name takes
the target's join. A template spells any other text itself.

### A function map without builtins

The loader replaces `printf`, `call` and the other builtins of text/template
with functions that refuse, and it passes no value with methods to a
template. Helm removed `env` and `expandenv` from its functions. Helm also
gated `getHostByName` after CVE-2023-25165.

**Why not:** Helm removed functions that read the host or the network. The
builtins of text/template and the functions of the backends read only their
arguments. A template can still make a run slow or fail. A template that
calls itself runs until text/template stops it at a depth of 100,000 calls.
A bound on that cost needs a bound on the time and the memory of an
execution, and text/template does not have one.

## Drawbacks

- The manifest format is public API. Each key of this proposal is frozen at
  the first release that contains it.
- The loader adds a package of seven source files, `doc.go` among them, and
  six test files to the CLI module.
- A TypeScript plan reports `RefusedType` for a Go method with a
  `context.Context` parameter or an `error` result, and it does not emit the
  method's mirror. A mirror keeps every parameter and result of its subject,
  because the tier does not have projections.
- The struct of an implementation does not have a constructor, and its field
  `next` has package visibility. Code outside the package of the struct
  cannot build it.
- Each name is `JoinName` of the word and the subject's name. A plugin that
  needs another name needs Go.
- The version of a declarative plugin is 65 characters longer than the
  version in its manifest, and the command `version` prints it.
- `watch` reads a manifest once. An edit of a manifest takes effect when
  `watch` starts again.
- No check lints the templates of a declarative plugin before a run. A fault
  of a template is a finding of the run that renders it.
- `missingkey=error` applies to every reference template. A template of a
  typed plugin that uses a missing key fails where it wrote `<no value>`.
- The kernel gains two fields and one refusal, the render pass one option,
  and the index of codes one code.

## Open questions

None.

## Unresolved and future work

- This proposal does not add a claimed file template for typed or
  declarative plugins.
- This proposal does not lint the templates of a declarative plugin against
  the backend of each plan. Build and `doctor` do not run such a lint. The Go
  and TypeScript backends do not export a `render.Language`, so a suite
  fixture cannot lint a tree against them.
- The JSON Schema of the config file does not contain the options of each
  plugin, typed or declarative.
- SchemaStore does not have an entry for the manifest's schema.
- This proposal leaves out the pairs of `on` and `emit` for enums, sums,
  aliases, constants and variables.

## References

| What | Where |
|---|---|
| Helm: `Chart.yaml`, `apiVersion` and `values.schema.json` | https://helm.sh/docs/topics/charts/ |
| Helm's template functions, and CVE-2023-25165 | https://github.com/helm/helm/blob/main/pkg/engine/funcs.go, https://github.com/helm/helm/security/advisories/GHSA-pwcw-6f5g-gxf8 |
| Copier: question types and `--trust` | https://copier.readthedocs.io/en/stable/configuring/ |
| openapi-generator: template files and their families | https://openapi-generator.tech/docs/customization |
| sqlc: process and WebAssembly plugins | https://docs.sqlc.dev/en/latest/guides/plugins.html |
| protoc's plugin protocol | https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/compiler/plugin.proto |
| yaml-language-server: the modeline and the drafts that it validates | https://github.com/redhat-developer/yaml-language-server/blob/main/README.md |
| JetBrains IJPL-64388: the `$schema` comments in YAML | https://youtrack.jetbrains.com/issue/IJPL-64388 |
| JSON Schema draft 2020-12 | https://json-schema.org/draft/2020-12 |
| Semantic Versioning 2.0.0: build metadata | https://semver.org/#spec-item-10 |
| text/template: the security model and `missingkey` | https://pkg.go.dev/text/template |
| `os.OpenInRoot` | https://pkg.go.dev/os#OpenInRoot |
| The plugin specification | [06-plugins.md](../architecture/06-plugins.md) |
| The authoring surface | [06b-authoring.md](../architecture/06b-authoring.md) |
| Rendering | [07-rendering.md](../architecture/07-rendering.md) |
| Distribution and the CLI | [14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md) |
| The module topology | [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md) |
