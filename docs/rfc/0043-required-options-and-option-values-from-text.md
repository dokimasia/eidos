---
rfc: 0043
title: Required options and option values from text
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0031
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0043: Required options and option values from text

## Summary

A plugin declares its options as a tagged struct whose constructed values are the defaults, and Build
writes the values of the config over them. An option without a usable default cannot say so, so five
reference plugins report an empty option at run time. Build converts a config value into a field
through JSON, so `5s` does not set a `time.Duration`, and a refused value loses the reason of its
refusal. We propose
a required mark that Build checks, the text form of a duration, and faults that contain the reason.

## Motivation

- The options of authgen, metricgen, tracegen, errorgen and auditgen are a package and a function,
  and no default is a useful one. A config without the options builds, and each plugin reports
  `REFERENCE-0001` at each struct during the run.
- The plugin suite promises that a plugin rejects missing required keys, and the tag contract has no
  required mark.
- Build converts a config value that the field cannot take as it is through JSON. JSON calls
  `UnmarshalText` for a text into a type that implements `encoding.TextUnmarshaler`, so such a type
  refuses a text outside its set. A text such as `5s` does not decode into a `time.Duration`, whose
  JSON form is a number of nanoseconds.
- The fault of a refused value states the type of the field and the type of the value, and drops the
  error of the decoder. A closed set that refuses `slow` does not say which values it takes.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/plugin` | `ValidateOptions` accepts the tag `schema:"required"` and refuses another value of the tag. `MissingOptions` returns the required options that have the zero value of their type |
| `eidos-core/workspace` | the options step parses the text of a duration, reports the error of the decoder, and reports each missing option after the config applied |
| `eidos-core/plugin/plugintest` | `AssertOptionsSchema` reports each missing option of the plugin of the setup |
| `eidos-reference/internal/callee` | the package and the function of `Options` are required |
| `docs/architecture` | the plugin, the authoring and the workspace documents state the mark and the decoding |

### Invariants

1. A required option has a value other than the zero value of its type when a run starts.
2. The tag vocabulary of an option is the vocabulary that the JSON Schema of the config reads: `doc`
   for the description and `schema:"required"` for a required property.
3. A config value that the field takes as it is keeps its value. A text decodes through the type of
   the field, through `UnmarshalText` or as a duration.

### The required mark

```go
type Options struct {
    Package string `opt:"package" doc:"the import path of the package of the function" schema:"required"`
    Func    string `opt:"func"    doc:"the name of the function"                        schema:"required"`
}
```

```go
// MissingOptions returns one error for each option of p that the tag
// schema:"required" marks and that has the zero value of its type, in
// field order. The message of each error contains the plugin and the key
// of the option. It returns nothing for a plugin that declares no
// options. It allocates the errors that it returns.
func MissingOptions(p Plugin) []error
```

The workspace calls `MissingOptions` for every plugin whose options struct meets the tag contract,
after the config applied, and collects each error as a fault of the composition. A plugin that sets
the option in its constructor, as a test does, passes. `ValidateOptions` refuses a `schema` tag with
another value than `required`.

### Values from text

The options step writes a config value into a field in this order:

1. A value that the field takes as it is.
2. A text into a field of the type `time.Duration`, through `time.ParseDuration`.
3. Any other value through the JSON encoding of options, as it does now. JSON calls `UnmarshalText`
   for a text into a type that implements `encoding.TextUnmarshaler`.

The fault of a value that does not parse or decode contains the error of `ParseDuration` or of the
decoder, and so the error that `UnmarshalText` returns. A closed set of values is a defined type
whose `UnmarshalText` refuses a text outside the set, as Go states an enumeration. The canonical
encoding of the options is unchanged, so a `time.Duration` enters the fingerprint as its number of
nanoseconds, and a type that marshals itself enters it as it marshals.

### Failures

| Condition | Fault |
|---|---|
| A required option has the zero value | `plugin: authgen requires the option package, which has the zero value of its type` |
| A text does not parse as a duration | `workspace: option "timeout" of plugin "x" takes time.Duration: <the error of ParseDuration>` |
| The decoder refuses a value | `workspace: option "mode" of plugin "x" takes x.Mode: <the error of the decoder>` |
| A `schema` tag with another value | `plugin: x option field Mode has the schema tag "optional", and the one value of the tag is required` |

### Migration

1. Add the tag to `ValidateOptions`, add `MissingOptions`, and call it in the options step and in the
   plugin suite.
2. Parse the text of a duration in the options step, and keep the error of the decoder in each
   fault.
3. Mark the package and the function of `callee.Options` required.
4. State the mark and the decoding in the documents.

## Alternatives

### A choices mark in the tag

A tag such as `choices:"fast|safe"` closes the set of a string option without a type. The values are
then strings in a tag that the compiler does not check, and the code compares the option with string
literals. A defined type with constants is how Go and the rest of eidos state a closed set.

### A required key in the config

Build could refuse a config without the key. A test that sets the option in the constructor and does
not write a config would then fail, and a config that sets the empty value would pass. The zero value
after the config applied is the state that the plugin reads.

### A step for each text form of the standard library

`time.Time` and `net/netip.Addr` implement `encoding.TextUnmarshaler`, so JSON decodes their text
already. `time.Duration` is the one common option type of the standard library that has a text form
without implementing the interface, so it has its own step.

## Drawbacks

- A composition that leaves out the options of the five reference plugins fails Build instead of
  reporting at each struct.
- A duration option takes a text and a number of nanoseconds, so a config can state one value in two
  forms.
- The plugin package gains one function, and the options step one branch.

## Open questions

None.

## Unresolved and future work

None.

## References

- The improvement plan rows W5.A1, W5.A2 and W5.A3, and the decision D-13.
- `encoding.TextUnmarshaler`: <https://pkg.go.dev/encoding#TextUnmarshaler>.
- `time.ParseDuration`: <https://pkg.go.dev/time#ParseDuration>.
