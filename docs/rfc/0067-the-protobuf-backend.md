---
rfc: 0067
title: The protobuf backend
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0054, ADR-0055, ADR-0056
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0067: The protobuf backend

## Summary

The protobuf satellite reads proto2, proto3 and the editions 2023, 2024 and 2026, and it writes no
file. Its frontend refuses an `extend` block. We propose a backend that writes every construct of
the five versions, and a frontend that loads every construct of them.

A generator states each construct in the emit model through one of five channels:

| Channel | States |
|---|---|
| A field of the model | a fact that other languages share, such as `Optional` for the label `optional` and `Value` for a default |
| The tag entry `number` | the wire number of a field, as a Go struct tag states the key of a JSON field |
| An annotation | an option |
| A marker | a keyword or a statement that is not an option, such as `reserved`, through an annotation whose name contains a colon |
| The header of a file | the syntax, the package, the imports and the options of the file |

The kernel changes in three places. An `emit.File` in a unit states the header of its file, and the
render passes the header to the file skeleton. A field gains `Receives`, the type that a field
extends from outside its declaration, as `Method.Receives` names it for a method. An extension of
protobuf is such a field. A file has the packages of the files that it refers into, so a target
qualifies a reference by the package clause of the file of its referent.

A generator over protobuf sources writes a schema as its source states it, through the functions
`Stated` and `Header` of the satellite, which restate the stamps of a protobuf source in these
channels. The conformance module copies a schema with every construct in each of the five versions,
and each written file equals its source below the header of the output contract.

## Motivation

We decided that protobuf is read-only, because a schema language is read and never written. That
decision blocks these generators:

- A generator that derives a schema from the declarations of Go, TypeScript, Java or Rust has no
  target. A plan cannot select a protobuf backend, and the hub has no spoke for protobuf.
- A generator over protobuf sources cannot write a schema, such as one that adds the options of a
  validation library to the fields of a schema.
- The support matrix has no protobuf column on the render side or in the hub.

These gaps of the model block a backend for the whole language:

- The frontend refuses an `extend` block, because the model has no form for a field that a message
  of another file receives. A schema loses each custom option that it declares.
- A backend reads the emit model, and the render context has no fact store. The residue of protobuf
  is a set of stamps, such as `protobuf.field` and `protobuf.reserved`. A backend writes the residue
  only where a generator states it in the emit model.
- A routed file has the annotations of its source file, and no plugin places a directive on a
  generated file. Every schema states its syntax, its package and its imports, and most schemas
  state a file option such as `go_package`. A generator over Go has no such source file to copy.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Model | `eidos-core/symbol/schema` | `Field.Receives` and the fact `Receives`, and the fact `Methods` of `Struct.Methods` |
| Plugin contract | `eidos-core/plugin` | `File.Header`, `File.Referents`, `HeaderOf` and `Placement.Header` |
| Layout | `eidos-core/layout` | the header of a routed file, from the `emit.File` declarations of its units, and the packages of the files that a file refers into |
| Render | `eidos-core/backend/render` | the header in the file skeleton, the builtin `header`, an `emit.File` that the declaration run skips, and the package and the referents of a file in its import set |
| Backend kit | `eidos-core/backend/backendtest` | the members check passes over the members that a coverage refuses or rejects |
| Satellite | `eidos-lang-protobuf` | `Target`, `BackendVersion`, `NumberTag`, the markers, the policies, the table of feature defaults, `Stated` and `Header` |
| Frontend | `eidos-lang-protobuf/frontend` | the `extend` block |
| Rules | `eidos-lang-protobuf/rules` | a field with `Receives` is not a member of its host |
| Backend | `eidos-lang-protobuf/backend` | new: the kit composition, the templates, the coverage, the lowering, the clusters, the imports, the scaffold and the check of a written file |
| Spoke and names | `eidos-lang-protobuf/spell` | new: `Type`, `Name`, `Filename` and `Package` |
| Toolchain adapter | `eidos-lang-protobuf/testing` | new: the parse through the parser of the frontend and the compile through protocompile |
| Other backends | `eidos-lang-go`, `-typescript`, `-java` and `-rust`, each `backend` | a verdict for `Receives`, for the methods of a struct and for the annotations of a file |
| Binary | `cmd/eidos` | the target and the plan `protobuf` |
| Conformance | `eidos-conformance` | the matrix entry, the fixture `schemas` and a third plan of the service fixture |

### Invariants

1. Every construct of the five versions has a form in the emit model, and the backend writes it. The
   table of constructs lists each form.
2. The frontend loads each file that the backend writes into the declarations, the shapes, the
   annotations and the stamps that the emit model stated.
3. A declared field renders with the number of its tag entry. The backend numbers only the fields of
   a message that it synthesizes, by the position of each field in its source.
4. The toolchain adapter compiles every file that the conformance fixtures write.
5. A construct, a fact, a kind or a name that the version of its file does not spell reports a
   positioned finding.

### The constructs

| Construct | The frontend loads | A generator states |
|---|---|---|
| `syntax` or `edition` statement | the stamp `protobuf.syntax` of the file | the header marker `protobuf:syntax` |
| `package` statement | the namespace, and the stamp `protobuf.package` | the header marker `protobuf:package`, or the package of the origin |
| `import`, `import public`, `import weak` and `import option` | the stamp `protobuf.import` | the header marker `protobuf:import`, beside the imports of the references |
| File option | the stamp `protobuf.options` of the file | a header annotation |
| `features` option | the stamp `protobuf.features` | an annotation |
| `message` | a struct | a struct |
| Nested `message` and `enum` | a nested type | a nested type |
| Field | a field, and the stamp `protobuf.field` | a field, and the tag entry `number` |
| `repeated` | the list form | the list form |
| `optional`, and explicit presence in an edition | the optional form | the optional form |
| `required`, and `LEGACY_REQUIRED` | the stamp `protobuf.label` | the marker `protobuf:required` |
| `map<K, V>` | the map form, and the stamp `protobuf.mapEntry` | the map form |
| `oneof` | a nested sum, and the stamp `protobuf.oneof` | a nested sum |
| `group`, and the encoding `DELIMITED` | a nested message, and the stamp `protobuf.delimited` | a nested struct, and the marker `protobuf:delimited` |
| `default` | the value of the field | the value of the field |
| `json_name` | the stamp `protobuf.jsonName` | the annotation `json_name` |
| Field option | the stamp `protobuf.options` | an annotation |
| `reserved` | the stamp `protobuf.reserved` | the marker `protobuf:reserved` |
| `extensions` | the stamp `protobuf.extensions` | the marker `protobuf:extensions` |
| `extend` | fields with `Receives` | fields with `Receives` |
| `enum`, and its values | an enum, and a variant with its number as its value | an enum, and a variant with its number as its value |
| `allow_alias` and the options of an enum value | the stamp `protobuf.options` | an annotation |
| Closed enum | the stamp `protobuf.closed` | the marker `protobuf:closed` |
| `local` and `export` | the stamp `protobuf.local` | the marker `protobuf:local` |
| `service` | an interface | an interface |
| `rpc`, and `stream` | an abstract method, and the stream form | a method, and the stream form |
| Options of a message, a oneof, an enum, a service and an rpc | the stamp `protobuf.options` | an annotation |
| Comments | the documentation, and the trailing comment | `Doc` and `Comment` |

The backend writes each construct in the syntax of the version of its file. The sections below give
the rules of each part.

### The tag entry `number`

`Field.Tags` is the slot of the entries of a field's tag. The contract of a tag entry states that a
target joins the entries of a field into its tag syntax. For protobuf that syntax is the number
after the name of a field. protoc-gen-go writes the same number into the Go struct tag of a
generated field, such as `protobuf:"varint,1,opt,name=seconds,proto3"`. The satellite declares the
key of the entry:

```go
// NumberTag is the key of the tag entry that states the wire number of a
// field in decimal, such as 3. The protobuf backend writes the number after
// the name of the field and refuses a field of a message without the entry.
const NumberTag = "number"
```

A generator over Go sources reads the number from a Go struct tag or from a directive. The tags of a
field are a slot, so a plugin can append the entry to a field that another plugin emitted. The
backend refuses an entry with another key, as the contract of a tag requires of a target without
such a key.

### Markers

A keyword or a statement that is not an option is a marker: an annotation whose name starts with
`protobuf:`. An option name is an identifier, a dotted path or an extension name in parentheses, so
no option has the name of a marker. The satellite declares the markers:

```go
// The markers of the protobuf target. The backend writes a keyword or a
// statement for each of them, and no option.
const (
    // SyntaxMarker is a header marker. Its one argument is proto2, proto3,
    // 2023, 2024 or 2026, and the backend writes the syntax or the edition
    // statement of the file from it.
    SyntaxMarker = "protobuf:syntax"
    // PackageMarker is a header marker. Its one argument is the package
    // that the file declares, such as acme.svc.v1.
    PackageMarker = "protobuf:package"
    // ImportMarker is a header marker. Its one argument is the path of one
    // import, after public, weak or option where the import has that
    // modifier, as in public acme/svc/v1/types.proto.
    ImportMarker = "protobuf:import"
    // RequiredMarker marks a required field: the label required in proto2,
    // and the presence LEGACY_REQUIRED in an edition.
    RequiredMarker = "protobuf:required"
    // DelimitedMarker marks a field whose message is encoded delimited: a
    // group in proto2, and the encoding DELIMITED in an edition.
    DelimitedMarker = "protobuf:delimited"
    // NumberMarker states the wire number of the oneof member of a sum
    // variant that does not have exactly one field.
    NumberMarker = "protobuf:number"
    // ReservedMarker states the reserved numbers and names of a message or
    // an enum. Each argument is a number, a range such as 9 to 11 or
    // 40 to max, or a name.
    ReservedMarker = "protobuf:reserved"
    // ExtensionsMarker states one extension range declaration of a
    // message. Each argument is a range, and a last argument in brackets
    // is the options of the declaration as written.
    ExtensionsMarker = "protobuf:extensions"
    // ClosedMarker marks a closed enum: every enum of proto2, and the enum
    // type CLOSED in an edition.
    ClosedMarker = "protobuf:closed"
    // LocalMarker marks a message or an enum that another file cannot
    // reference, which the editions 2024 and 2026 write with local.
    LocalMarker = "protobuf:local"
)
```

### The file header

A unit of a generator can contain an `emit.File`, whose annotations the model declares as the
directives of a file. The kernel reads each such declaration as part of the header of the file that
its unit routes to:

- `layout.Route` collects the documentation and the annotations of every `emit.File` of the units of
  a routed file, in unit order. An annotation that two of them state alike appears once, and so does
  a documentation that two of them state alike.
- `plugin.Placement` gains the annotations of the header, so a package rule can read a package that
  the header states.
- The render passes the header to the file skeleton beside the annotations of the source file. The
  builtin `header` returns it to every template of the file. The declaration run skips an
  `emit.File`.
- The coverage verdict of the annotations of a file applies to the header. The Go, TypeScript, Java
  and Rust backends declare Refuses for the annotations of a file, because their skeletons do not
  write a header. A header in those targets reports `RefusedFact`.

```go
// Header is the file-level statement of the generators of the file: the
// documentation and the annotations of every emit.File that its units
// contain, in unit order. An annotation that two of them state alike
// appears once, and so does a documentation. The zero Header states
// nothing.
Header emit.File
```

`plugin.Placement` gains the annotations alone:

```go
// Header is the annotations of the header of the routed file, which
// [File.Header] describes.
Header symbol.Annotations
```

A generator states the header through the accumulator of its family, as in
`e.File().Append(&emit.File{Annotations: header})`. Two handlers of one source file can state one
header, and the merge keeps one copy of each annotation.

### Extensions

`Field` gains `Receives`:

```go
// Receives names the type that a field attaches to when the field is
// declared outside that type's own declaration: a protobuf extension, a
// Kotlin extension property. Host remains the declaration that contains
// the field, or the file of a field at file level. A field declared inside
// its type leaves Receives nil.
Receives *TypeRef `eidos:"both,walk,fact=Receives"`
```

The kernel treats the field this way:

- The load gives the field the identity of the scope that declares it, whatever its `Receives`. A
  method takes the identity of the type that its `Receives` names, because a Go method belongs to
  the method set of a type of its own package. An extension extends a message of any package, and
  its name belongs to the scope that declares it.
- The resolution step resolves `Receives` as it resolves the other references of the walk, and the
  importer role records the file of the extended message.
- The name table lists a field with `Receives` under the spelling of its `Receives`, as it lists a
  method with `Receives`. The settle then checks the name of an extension against the other members
  of the extended message. `emit.DeclaredName` returns the empty string for a field, as for a
  method, because a GraphQL extension adds its field to the extended type and not to the file. A
  protobuf extension that clashes with a message of its package fails in the compiler.
- Every backend states a verdict for the fact `Receives`. Go, TypeScript, Java and Rust refuse it,
  because none of them has an extension field.

The frontend loads an `extend` block this way:

- At file level, each field of the block is a field among the declarations of the file, and its
  `Receives` is the extended message as written.
- Inside a message, each field of the block is a field among the nested declarations of the
  message. The message is the scope of the extension, and the extension is not a field of the
  message.
- Each field has the stamps of any field: its number, its label, its `json_name`, its options and its
  delimited mark. A group inside a block is a message in the scope of the block and a field with
  `Receives`, as a group inside a message is.
- The comments above a block document its first field, because a block is not a declaration of the
  model.
- The frontend does not report `RefusedExtension` any more, and the satellite does not reuse its
  number.

The rules leave a field with `Receives` out of the members of its host, because an extension adds no
data to the message that declares it. The backend clusters the fields of one scope by their
`Receives` and writes each cluster as one `extend` block.

### Versions

The header marker `protobuf:syntax` selects the version of a file. A file without the marker is
proto3, the version that every protobuf toolchain reads. The backend writes each construct by the
version of its file:

| Construct | proto2 | proto3 | Edition 2023 | Editions 2024 and 2026 |
|---|---|---|---|---|
| Version statement | `syntax = "proto2";` | `syntax = "proto3";` | `edition = "2023";` | `edition = "2024";` and `edition = "2026";` |
| A singular field with presence | `optional` | `optional` | explicit presence | explicit presence |
| A singular field without presence | refused | no label | implicit presence | implicit presence |
| The marker `protobuf:required` | `required` | refused | `LEGACY_REQUIRED` | `LEGACY_REQUIRED` |
| A default | `[default = …]` | refused | `[default = …]` | `[default = …]` |
| The marker `protobuf:delimited` | a group | refused | the encoding `DELIMITED` | the encoding `DELIMITED` |
| An enum without the marker `protobuf:closed` | refused | open | open | open |
| The marker `protobuf:closed` | closed | refused | the enum type `CLOSED` | the enum type `CLOSED` |
| The marker `protobuf:local` | refused | refused | refused | `local` |
| A reserved name | `"name"` | `"name"` | `name` | `name` |
| An extended message | any message | a message of `descriptor.proto` | any message | any message |
| An open enum whose first value is not 0 | none | refused | refused | refused |

An edition states presence, encoding and the openness of an enum through features. The backend
resolves the features of a declaration as the frontend does. It starts from the defaults of the
version, and applies the features annotations of the header, of each enclosing message, of a oneof
and of the declaration itself in that order. Where the resolved value differs from the value that a
shape or a marker needs, the backend adds the feature to the declaration, such as
`[features.field_presence = IMPLICIT]`. A stated feature that contradicts a shape or a marker is
refused. The root package of the satellite declares the table of the defaults of each version, so
the frontend and the backend resolve features from one table.

The backend writes a keyword of visibility only where the visibility of a declaration differs from
the default of its scope, which the feature `default_symbol_visibility` resolves. A declaration with
the marker `protobuf:local` writes `local` where the default exports it, and a declaration without
the marker writes `export` where the default makes it local. Edition 2024 makes a nested declaration
local by default, and edition 2026 makes every declaration local. The visibility STRICT exports no
nested message, and it exports a nested enum only where its message has one reserved range,
`1 to max`, so the backend refuses any other nested declaration without the marker there.

The settle cannot refuse a construct for its version, because the layout routes a declaration into
its file after the settle. The templates read the version from the header through the builtin
`header`. A helper of the vocabulary fails the kind template of a declaration whose construct the
version refuses, and the render reports `RefusedTemplate` with the reason and skips the
declaration.

### Fields and presence

The field template writes the label or the feature of a field from its shape and its markers:

| Shape | Written |
|---|---|
| T | `T name = N;`, with the label or the feature of a field without presence |
| Optional of T | `optional T name = N;` in proto2 and proto3, and explicit presence in an edition |
| List of T | `repeated T name = N;` |
| Map of K to V | `map<K, V> name = N;` |

A field of a message type always has presence. In an edition, the backend writes such a field
without a feature whatever its shape, so the frontend loads it back as optional. A field with
`Optional`, such as a TypeScript property with a question mark, has presence as an optional form
does.

The value of a field is its default, which the backend writes as `[default = value]` in proto2 and
in an edition. The value is protobuf syntax, so a string default keeps its quotes.

### Oneofs and groups

- A sum that a struct nests is a oneof of its message, with a member for each variant. A member
  has the name of its variant, which the respell writes in snake case. A variant with one field
  writes that field as its member, under the number of the tag entry of the field. The member has
  the documentation, the comment and the options of the field, or those of the variant where the
  field states none.
- A variant with two or more fields writes a member whose type is a message that the backend
  synthesizes in the scope of the oneof's message. The message has the name of the variant in
  Pascal case and the fields of the variant. A field without a name is `field_<position>`, and a
  field without a number takes its position. A variant without a field writes a member of type
  `google.protobuf.Empty`. Each of these members takes its number from the marker `protobuf:number`
  of its variant.
- A sum at file level is a message with the name of the sum and one oneof, whose name is the name of
  the sum in snake case. A protobuf schema states a sum type as a message with a oneof, so the
  backend writes a sum at file level in that form.
- proto2 writes a field with the marker `protobuf:delimited` as a group. Its type must be a struct
  that the scope of the field declares, and the name of the field must be the name of that struct
  in lower case, as protoc names the field of a group. The backend writes the group from the field
  and the struct, and the struct does not render beside it. A delimited field of any other form is
  refused in proto2.

### Enums

- The value of a variant is its number as protobuf writes it, in decimal, in hexadecimal or in octal,
  and negative values are allowed. The lowering parses it with protobuf's integer syntax and refuses
  a number outside the range of an `int32`.
- Two variants with one number need the annotation `allow_alias` with the argument `true` on the
  enum.
- The annotations of a variant are its options, as in `ACTIVE = 1 [deprecated = true];`.

### Ranges

Reserved numbers and names take the marker `protobuf:reserved`. protobuf refuses names and numbers
in one statement, so the backend writes each run of numbers and each run of names as one statement,
in the order of the arguments. A name is quoted in proto2 and proto3, and an identifier in an
edition. The frontend joins the entries of every reserved statement of a declaration in source
order, so a restated declaration keeps its order.

Each extension range declaration of a message is one marker `protobuf:extensions`, with its ranges
and its options. A message can state the marker more than once.

### Options and features

An annotation writes an option at its level:

| Declaration | Option |
|---|---|
| Field, enum value | `[name = value]` after the number or the value, the options joined with commas |
| Message, oneof, enum, service, rpc | `option name = value;` at the start of the body |
| File | `option name = value;` after the imports |

The name of the annotation is the name of the option as protobuf writes it, with the parentheses of a
custom option, such as `(buf.validate.field).string.min_len`. Its one argument is the value in
protobuf's syntax, which can be a message literal in text format. A features annotation, such as
`features.utf8_validation`, is an option as well. The annotation `json_name` takes a quoted string.

A header annotation that sets a custom option needs the import of the file that declares the
option. An option is not a type reference, so the generator states that import in the header.

### Services

An interface writes a `service`, and each method writes an `rpc`. A method takes one request and
returns one response, as gRPC requires:

- A method with one parameter of a message type takes that message, under the stream form where the
  parameter streams.
- Any other method takes a message that the backend synthesizes beside the service, named
  `<Service><Method>Request`. The message has one field for each parameter, in order and numbered
  from 1. A named parameter gives its field its name. An unnamed parameter is `param`, or
  `param_<position>` in a method with more than one parameter. A variadic parameter is a repeated
  field, a keyword one is a map from string, and an optional one is an optional. The field has the
  documentation, the comment, the default and the annotations of its parameter. A method without a
  parameter has an empty request.
- The response follows the same rule, under the name `<Service><Method>Response`. A named result
  gives its field its name. An unnamed result is `result`, or `result_<position>` in a method with
  more than one result.
- A streaming side whose element is not a message streams a synthesized message with one field,
  `value`. The lowering refuses a stream beside another parameter or another result, because a
  stream of gRPC is the whole request or the whole response. It refuses a parameter with a label,
  because a field has one name, and two rpcs of one name in one service.
- The annotations of a method are the options of its rpc, which the backend writes in a body.

buf's lint rules in the category STANDARD require a request and a response named after the rpc and
unique across the schema. The synthesized names meet both rules.

### The spoke

`spell.Type` spells each canonical form this way:

| Form | protobuf |
|---|---|
| Signed integer of 8, 16 or 32 bits | `int32`, or the choice of `protobuf.int32` |
| Signed integer of 64 bits or of the platform's width | `int64`, or the choice of `protobuf.int64` |
| Unsigned integer of 8, 16 or 32 bits | `uint32`, or the choice of `protobuf.uint32` |
| Unsigned integer of 64 bits or of the platform's width | `uint64`, or the choice of `protobuf.uint64` |
| Float of 32 bits, of 64 bits | `float`, `double` |
| Bool, Text, Bytes | `bool`, `string`, `bytes` |
| Optional of T | T under the optional form |
| List of T | T under the list form |
| Map of K to V | the map form |
| Asynchronous stream of T | T under the stream form |
| Dynamic | `google.protobuf.Value` |
| Well-known timestamp, duration, empty value | `google.protobuf.Timestamp`, `google.protobuf.Duration`, `google.protobuf.Empty` |
| Reference, Sum | the referent's flat name |

protobuf spells one integer of each width in two or three encodings. A policy for each width and
sign chooses the encoding, and the default of each policy is the varint encoding:

| Policy | Choices |
|---|---|
| `protobuf.int32` | `int32`, `sint32`, `sfixed32` |
| `protobuf.int64` | `int64`, `sint64`, `sfixed64` |
| `protobuf.uint32` | `uint32`, `fixed32` |
| `protobuf.uint64` | `uint64`, `fixed64` |

A config file selects a choice for every plan or for one plan, and the param of the policy's name on
the `protobuf` directive selects it for one declaration. A generator over protobuf sources keeps the
encoding of its source, because the emitter copies a reference of the target's own language as
written.

The lowering writes a shape that protobuf has no form for as the wrapper message that the protobuf
ecosystem uses. The backend synthesizes the wrapper in the scope of the message of the field, as
protoc synthesizes the entry message of a map field. `F` is the name of the field in PascalCase:

| Shape of the field `f` | Written |
|---|---|
| List of a list or of a map | `repeated FItem f`, with `message FItem` and its field `values` of the element |
| List of an optional scalar | `repeated` the well-known wrapper of the scalar, such as `google.protobuf.Int32Value` |
| List of an optional message or enum | `repeated FItem f`, with `message FItem` and its optional field `value` |
| Map to a list or to a map | `map<K, FValue> f`, with `message FValue` and its field `values` |
| Map to an optional scalar | `map<K, V>` with the well-known wrapper of the scalar as V |
| Map to an optional message or enum | `map<K, FValue> f`, with `message FValue` and its optional field `value` |
| Map whose key is not an integer, a bool or text | `repeated FEntry f`, with `message FEntry`, its field `key` numbered 1 and its field `value` numbered 2 |
| Optional of a list or of a map | the list or the map, whose absent value is the empty one |
| A oneof member of a list or of a map | a member whose type is the wrapper of the list or the map |

A wrapper whose field needs a wrapper nests the next wrapper by the same rule. The spoke refuses the
shapes that protobuf and its ecosystem do not map: a scalar of 128 bits, an array, a tuple, a union,
an intersection, a function type, a synchronous stream, a borrow, a wildcard, an inline type, an
opaque type, the well-known error, a reference with type arguments, an optional of an optional and
a read-only list or map. It refuses a form whose child it refuses, and the error contains the source
spelling of the child.

### Names

| Declaration | Convention | Example |
|---|---|---|
| message, enum, service, rpc | PascalCase | `SessionState` |
| field, oneof, extension | snake_case | `expires_at` |
| enum value | SCREAMING_SNAKE_CASE | `ACTIVE` |
| file | snake_case with `.proto` | `store_schema.proto` |

These are the conventions of protobuf's style guide. The respell refuses a name at a visibility other
than public. It does not join the name of an enum to its values, because the hook receives the kind
of the host and not its name. A generator that needs the prefix that the style guide suggests
writes it into the name of each variant.

### Files, packages and imports

`spell.Package` makes each routed file a package of its own, as the TypeScript rule makes each file a
module. The package path of a file is its import path:

- With an import base in the plan, a file below the output directory of the plan has the import base
  joined with its path below that directory. An import base of `.` makes the output directory the
  root of the imports.
- Any other file has its workspace-relative path.

The package clause of a file is the first package of this list:

1. The argument of the header marker `protobuf:package`.
2. The package of the origin, where the units of the file derive from a protobuf package.
3. The directories of the import path, joined with dots, each in snake case. buf's lint rule
   `PACKAGE_DIRECTORY_MATCH` requires that match in its categories MINIMAL, BASIC and STANDARD. A file
   at the root of the imports has no package statement.

The layout qualifies a reference to a declaration of another file with the import path of that
file, and the speller imports the file. The speller qualifies the name with the package of the other
file where it differs from the package of the file. The kernel gives the speller both packages:

```go
// Referents are the packages of the other files that the layout
// qualified a reference of the file into, each once and sorted by
// package path. A target whose package clause does not follow from the
// package path, such as protobuf, reads the clause of such a file from
// the Name of its package.
Referents []symbol.Identity
```

The render sets the package of the file in its import set through `ImportSet.SetPackage`, and the
referents through `ImportSet.SetReferents`. `ImportSet.Referent` returns the package of one file of
the referents. A reference to a declaration of protobuf is written as its source writes it, so a
generator over protobuf sources keeps the spelling of its source.

The import block lists the imports of the header and the imports of the references, each path once
and in sorted order. A header import with a modifier writes its modifier. A well-known type imports
the file that declares it. A written file has this skeleton, whose header and provenance trailer the
output contract writes through the line comment:

```proto
syntax = "proto3";

package acme.svc;

import "acme/svc/session.proto";
import "google/protobuf/timestamp.proto";

option go_package = "example.com/acme/svc/svcpb";

// Store serves sessions.
service Store {
  rpc Get(GetRequest) returns (Session) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}
```

The documentation of the header comes before the syntax statement.

### Restating a protobuf source

The root package of the satellite restates the stamps of a protobuf source in the channels of the
emit model:

```go
// Stated returns the emit statement of the residue that the protobuf
// frontend stamped on the declaration id: the annotations of its options
// and of its features as written, its markers, and the tag entry of its
// number. A generator over protobuf sources appends them to the emit
// declaration that it writes for id, so the protobuf backend writes the
// declaration as its source states it. Each read records on the
// invocation of m.
func Stated(m eidos.Matcher, id symbol.Identity) Statement

// Statement is the emit statement of the residue of one declaration.
type Statement struct {
    // Annotations are the options and the markers, in the order of the
    // stamps.
    Annotations symbol.Annotations
    // Number is the tag entry of the wire number of a field, and nil for
    // every other declaration.
    Number *emit.TagEntry
}

// Header returns the header of the file that declares d: its syntax, its
// package and its imports as markers, its options as annotations, and the
// documentation of its syntax and package statements. Each read records
// on the invocation of m.
func Header(m Match, d node.Declaration) *emit.File

// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one.
type Match interface {
    eidos.Matcher
    Reader() *store.Reader
}
```

| Stamp | Channel |
|---|---|
| `protobuf.field` | the tag entry `number` |
| `protobuf.label` | the marker `protobuf:required` |
| `protobuf.jsonName` | the annotation `json_name` |
| `protobuf.options` and `protobuf.features` | one annotation for each entry |
| `protobuf.delimited` | the marker `protobuf:delimited` |
| `protobuf.reserved` | the marker `protobuf:reserved` |
| `protobuf.extensions` | one marker `protobuf:extensions` for each entry |
| `protobuf.closed` | the marker `protobuf:closed` |
| `protobuf.local` | the marker `protobuf:local` |
| `protobuf.syntax`, `protobuf.package` and `protobuf.import` | the header markers |
| `protobuf.oneof`, `protobuf.mapEntry` and `protobuf.stream` | none, because the nested sum, the map form and the stream form state them |

### The backend

The backend is a kit composition, as every backend is. The kit requires a scaffold, and the scaffold
of protobuf refuses every statement, because a protobuf file does not contain a body. The templates
write the layout of buf format: an indentation of two spaces, a member on each line, an empty block
as `{}`, one option in brackets on the line of its field and two or more on lines of their own. An
empty block with a trailing comment closes on the line after the comment, because the comment ends
its line. A
rendered file finalises through `Check`, which parses the file with the parser of the frontend and
refuses a file that does not parse. The printer of protocompile under the preset of buf format is
the oracle of the layout in the specs of the backend. It does not finalise a file: a render of the
scaled corpus of the kit counted 94 million allocations with the printer, and counts 13.7 million
with `Check`.

```go
// New returns the protobuf rendering backend, which writes proto2, proto3
// and edition files. Its spoke is [spell.Type]. The templates write the
// layout of buf format, and rendered files finalise through [Check],
// which refuses a file that the parser of protocompile does not read.
func New() backend.Backend {
    return backend.New(protobuf.Name, protobuf.Target, protobuf.Syntax()).
        Version(protobuf.BackendVersion).
        FileTemplate(FileTemplate).
        KindTemplates(KindTemplates()).
        RefusedKinds(RefusedKinds()).
        Coverage(Coverage()).
        Funcs(Funcs).
        Naming(spell.Filename).
        Packages(spell.Package).
        Respell(spell.Name).
        Types(spell.Type).
        Policies(protobuf.Policies()...).
        Lower(Lower).
        Cluster(Cluster).
        Groups(Groups()).
        Scaffold(Scaffold).
        Imports(Imports).
        TemplateImport(TemplateImport).
        Finalise(Check).
        Build()
}
```

The root package of the satellite gains the target and the version of the backend. `Version`
remains the version of the frontend:

```go
// Target names the rendering target a plan resolves to select the
// protobuf backend. It is the spelling of Lang, so the emitter copies a
// reference of a protobuf source as written into a plan of the target.
const Target plugin.Target = "protobuf"

// BackendVersion is the backend's behavior version, folded into the
// composition fingerprint: bump it with any change to the rendered output.
const BackendVersion = "0.1.0"
```

#### Kinds

| Emit kind | protobuf |
|---|---|
| Struct | `message` |
| Sum that a struct nests | `oneof` |
| Sum at file level | `message` with one `oneof` |
| Enum | `enum` |
| Interface | `service` |
| Field with `Receives` | a member of an `extend` block |
| Function, Method, Variable, Constant, Alias | refused kinds, because protobuf declares none of them |

The backend's `Cluster` gathers the fields with `Receives` of one unit by the spelling of their
`Receives`, and the group template `extend` writes each cluster. The template of a message writes the
fields with `Receives` among its nested declarations in the same way.

#### The lowering

`Lower` refuses each construct of this table under `RefusedConstruct`, and the settle withholds the
declaration. The lowering also synthesizes the wrappers, the requests and the responses of the
preceding sections.

| Construct | Reason |
|---|---|
| A field of a message without the tag entry `number`, with two or more entries, or with an entry of another key | invariant 3, and protobuf has no other tag |
| A number outside 1 to 536,870,911, or from 19,000 to 19,999 | protobuf accepts the first range and reserves the second for its implementation |
| Two fields of one message with one number, the members of its oneofs included | a number identifies one field of a message |
| Two members of one message with one name: its fields, its oneofs and their members, its nested declarations and its extensions | a name identifies one member of a message |
| A oneof without a variant | a oneof has at least one member |
| A variant without exactly one field and without the marker `protobuf:number`, or with one field and the marker | the member of such a variant takes the number of the marker, and any other member the number of its field |
| An immutable field | a message has no read-only field |
| A field at file level without `Receives` | a file declares no field of its own |
| A field without `Receives` among the nested declarations of a message, and a field with `Receives` among its fields | an extension is a nested declaration, and a field is not one |
| A nested declaration of a message of a kind other than a message, an enum, a oneof or an extension | a message nests no other kind |
| An enum without a variant, or a variant without a value | every enum value has a number |
| A value that is not a number in decimal, hexadecimal or octal, or outside the range of an `int32` | an enum value is an `int32` in the integer syntax of protobuf |
| Two variants with one value, without the annotation `allow_alias` | protoc refuses a duplicate number without the option |
| An option without exactly one argument, or with a name that is not the name of an option | an option has one value |
| A marker that the satellite does not declare, a marker on a declaration of another kind, or a marker with another number of arguments | each marker has one meaning on one kind |
| An import marker with a modifier other than `public`, `weak` and `option`, or without a path | an import has one path and at most one modifier |
| An interface with a field, or with two rpcs of one name | a service declares rpcs alone, each once |
| A stream beside another parameter or another result, and a parameter with a label | a stream of gRPC is the whole request or response, and a field has one name |

#### Facts

| Verdict | Facts |
|---|---|
| Renders | Comment, Tag, Annotations, Optional on a field, Types on a struct, Value on an enum variant and on a field, MultiReturn, NamedReturn, Variadic, Receives |
| `Holds` | Visibility, Async, Throws, Final, Mutability, Abstract on a method, Level on a struct |
| Refuses | Abstract on a struct, Override, DefaultBody, HardPrivate, Optional on a method and on a parameter, Label, ParamDefault, ConstEnum, Underlying, Fields, Methods, Sealed, Types on an interface, Embeds, Extends, Implements, Permits, Properties, Defined, Variance, ConstParam, TypeParamDefault |
| Rejects | TypeParams, Level on any other kind, Accessor, Indexer, Constructs, Computed |

These verdicts need a reason:

- Visibility takes the verdict `Holds`. A message without the marker `protobuf:local` is visible to
  every importing file, and the respell refuses a name at a visibility other than public.
- Mutability takes `Holds`, because a message is mutable in every protobuf runtime, and the lowering
  refuses an immutable field.
- Async and Throws take `Holds` for a method, because an rpc crosses a connection and can fail with
  a status.
- Abstract takes `Holds` on a method, because an rpc has no body, and the frontend loads every rpc as
  abstract.
- Methods take Refuses, so a struct with methods renders as a message without them, with a warning.
  The model had no fact for the methods of a struct, so a backend could not state the verdict. The
  field `Struct.Methods` gains the fact `Methods`, and the kit's members check passes over the
  methods of a struct where the coverage refuses the fact.
- MultiReturn, NamedReturn and Variadic render through the synthesized requests and responses.
- Level takes Rejects, because a static member is not data of a message. It holds on a struct,
  because a nested message binds to no instance of its message.

### The toolchain adapter

The `testing` package implements the kernel's toolchain skeleton for protobuf without a toolchain on
the machine:

- The parse reads each file through the parser of the protobuf frontend, the package
  `internal/parse` of the satellite, and fails on any syntax error. A plugin module does not import
  the load of the kernel, so the adapter does not run the frontend itself. The conformance checks
  load the written files through the frontend.
- The type check compiles the files through the experimental compiler of protocompile at the pinned
  commit, whose parser the frontend already uses. Its query `queries.Link` links the references and
  interprets the options of the files. It reads them through `source.NewMap` over the scratch tree
  and `source.WKTs` for the well-known types. The compiler at that commit knows the editions up to
  2024. The adapter compiles a 2026 file as the frontend parses one, from a copy whose edition is
  2024. The check then covers the grammar, the references and the options of a 2026 file, and not
  the defaults that 2026 changes.

`Available` reports true on every machine, because both steps run in process. A protobuf file does
not contain tests or implementations. So `RunTests` and `Satisfies` return an error that identifies
the missing capability, as the kernel's `toolchain.Adapter` requires of an operation that a language
cannot perform.

### The binary

`cmd/eidos` composes the target and one more extensible plan, as it does for each of its targets:

```go
Targets(golang.Target, typescript.Target, java.Target, rust.Target, protobuf.Target).

workspace.Plan{
    Name:       string(protobuf.Target),
    Sources:    workspace.Sources{Lang: protobuf.Lang, Packages: []string{treeSources}},
    Backend:    protobackend.New(),
    Extensible: true,
},
```

The stock plan reads protobuf sources, as each stock plan reads the sources of its own language. A
plan from the sources of another language to the protobuf target is part of a composition that a
consumer builds.

### Tests

- The backend's spec files run `backendtest.RunBackendSuite`, `AssertCoveredFacts`,
  `AssertSpeltKinds` and `AssertRenderedMembers` over the kernel's canonical fixture. The fixture
  states no wire number, so the specs give each field the number of its position, each enum value
  its position and each variant without one field a marker `protobuf:number`. The benchmarks of the
  render and the settle run over the kernel's scaled fixture, from which the specs remove the type
  parameters that protobuf rejects. The specs pin each template, each refusal of the lowering and
  of a version, each row of the spoke and each row of the respell in a table, and they compare each
  written file with the printer of protocompile under the preset of buf format.
- The kernel's spec files pin the merge of the header in the layout, the builtin `header`, the skip
  of an `emit.File` in the declaration run, the referents of a file, and the identity of a field
  with `Receives`.
- The conformance module gains the fixture `schemas`, with one source tree for each of the five
  versions. Each tree states every construct of its version, and every construct of the table of
  constructs appears in at least one tree. The plan `proto-mirror` runs a generator that restates
  each declaration through `Stated` and each file through `Header`. It writes each schema under the
  directory `mirror`, which is the root of the imports of the written files, and every file of the
  fixture has the name `schema.proto`, which the plan gives each file that it writes. So a written
  file has the import path of its source. Each source is written in the layout of the backend,
  so each written file equals its source between the header and the provenance trailer that the
  output contract adds. The layout orders the declarations of a file by their identities, so a
  source lists its declarations at file level in the order of their names, which puts its extend
  blocks last. A source documents the first field of an extend block, because the frontend loads
  the documentation of a block as the documentation of its first field. The toolchain adapter
  compiles each written file.
- The service fixture gains the plan `proto-schema`, which writes the mirror of the schema of each
  of its five versions at `mirror/svc/store.proto`. The schema does not list its declarations in the
  order of their names, so the mirror differs from it. The toolchain adapter compiles the mirror,
  and a mirror of the mirror equals the mirror, so the frontend loads a written schema with the
  stamps of its source.
- The matrix entry of protobuf gains its backend, so the render side and the hub of the support
  matrix have a protobuf column, with a probe for each choice of the four policies of protobuf.

### Failures

| Failure | Code | Effect |
|---|---|---|
| A kind that protobuf does not declare | `RefusedKind` | the render skips the declaration |
| A construct that the lowering refuses | `RefusedConstruct` | the settle withholds the declaration |
| A construct that the version of its file refuses | `RefusedTemplate` | the render skips the declaration |
| A name at a visibility other than public | `RefusedName` | the settle withholds the declaration |
| A stated fact without an idiom | `RefusedFact` | the declaration renders without the fact, with a warning |
| A stated fact that cannot render either way | `RejectedFact` | the render leaves the declaration out |
| A shape that the spoke refuses | `RefusedType` | the generator's emitter reports it at the source declaration |

### Migration

`Field.Receives` is a schema edit and a regeneration of the model, so every sealed state runs cold
once. The other changes add behavior.

| Change | Affects | Step |
|---|---|---|
| `Field.Receives` and the fact `Receives` | every backend | state a verdict for the fact |
| The fact `Methods` of `Struct.Methods` | every backend | state a verdict for the methods of a struct |
| The file header | the Go, TypeScript, Java and Rust backends | declare Refuses for the annotations of a file |
| The `extend` block | a generator over protobuf sources | handle a field with `Receives`, or skip it |
| The plan `protobuf` | users of the stock binary | none, because an extensible plan without a plugin does not write a file |
| `protobuf.NumberTag` | a generator of the protobuf target | append the entry to each field it emits |

## Alternatives

### Keep protobuf read-only

A reader of protobuf does not need a backend, and the read-only anatomy remains valid for a
satellite without one. The generators of the motivation then remain without a target.

### Write editions alone

Edition 2023 expresses every construct of proto2 and proto3 through features, and protobuf's own
tooling upgrades a proto2 or a proto3 file to an edition. A backend that writes edition 2023 alone
would drop the label rules of proto2 and the refusals of proto3. A consumer whose toolchain does not
read editions could then not use the output. A generator that adds options to a proto3
schema would also rewrite every file of the schema into another syntax.

### Extensions as stamps

The frontend could stamp the extended message of each extension field as text, without a field of
the model. A reference in a stamp does not resolve, so a generator into Go could not spell the
extended message. A field with `Receives` resolves as every other reference of the walk does.

### An extend block as a struct

Each `extend` block could load as a struct whose `Extends` names the extended message.
A block has no name. A struct named after the extended message collides with every other block of
the package that extends the same message, because the load merges the files of one package. The
fields of such a struct would also take the identity of the extended message, while protobuf names
an extension in the scope that declares it.

### A field of the model for the number

The symbol schema could gain `Field.Number` with a fact of its own. Every other backend would state
one more verdict. The protobuf frontend would move the number from its stamp into the field and
break every reader of the stamp. A tag entry avoids both, because the emit model already has a slot
that a target joins into its tag syntax.

### Options as tag entries

A field could state its options as more entries of its tag. Messages, enums and services have options
and no tag, so they need annotations anyway. Annotations serve every level through one channel.

### A fact store in the render context

The backend could read the stamps from the origin of each declaration. The render context of every
backend would then contain the fact store, each read would need a tracked edge for the warm run, and
a generator over Go could still not state a stamp.

### File options from the source file alone

The layout gives a routed file the annotations of its one source file. The protobuf frontend could
record the file-level statements as annotations of the file, and the layout would pass them to the
backend of a mirror without a header. A generator over Go has no protobuf source file. A file of a package or of a plan
has no source file at all, so a generator could not state the syntax or the options of such a file.

### A printer of descriptors

protoreflect v2.0.0, released on 2026-10-10, prints descriptors to `.proto` files through
`protoprint` and builds them through `protobuilder`. A backend over it would build a descriptor from
each emit declaration and print it. It would bypass the kit's kind templates, so the template
overrides and the slots of plugins would not apply to protobuf. The version rules and the wrappers
would still be the backend's own code.

### Number the fields in declaration order

A generator over a language without wire numbers would not need a source fact. A field that moves or
disappears then shifts the numbers of the fields after it, and a client of the old schema reads its
data into other fields without an error.

## Drawbacks

- The kernel changes in five packages for three features: one schema field with one fact, one more
  fact on a field of the schema, three fields and one function of the plugin contract, a merge in
  the layout, the header with one builtin in the render, two setters of the import set, and the
  rules of the members check of the backend kit. Every sealed state runs cold once.
- Each of the four other backends declares three more verdicts.
- A refusal for the version of a file happens at render, not at the settle, so its finding has the
  code of a template and not of a construct.
- The number is text. The lowering parses it, and the type system does not check that a generator
  writes it. A generator that writes the entry into a plan of Go also writes `number:"3"` into the Go
  struct tag of the field.
- An option is text in protobuf's syntax, so a wrong value fails the compile of the toolchain adapter
  and not the run. The lowering does not check whether a reference refers to a message or an enum
  either.
- The synthesized requests, responses and wrappers number their fields by position. A parameter
  added in front of another one renumbers the fields after it.
- Files of one directory share a protobuf package. The settle checks names for each package of the
  layout, which is a file here, so a message name that two such files declare fails in the compiler
  and not in the run.
- The documentation of a package statement joins the documentation of the file, because the header
  has one documentation.
- Enum values have no prefix, so buf's STANDARD category reports `ENUM_VALUE_PREFIX` for a generator
  that does not write one.
- The toolchain adapter checks a 2026 file under the defaults of 2024.

## Open questions

None.

## Unresolved and future work

None.

## References

- [RFC-0024: Protobuf editions and a service in Go and TypeScript](0024-protobuf-editions-and-services.md),
  whose service fixture this RFC extends.
- [RFC-0058: The build constraint of a generated file](0058-the-build-constraint-of-a-generated-file.md),
  whose verdict on a directive that a plugin places on a generated file this RFC reverses.
- [RFC-0065: The eidos binary](0065-the-eidos-binary.md), the stock plans.
- The decision log, rows D7 (protobuf is read-only), D16 (parsers), D112 (protobuf's residue) and D149
  (the annotations of a routed file): [21-decisions.md](../architecture/21-decisions.md).
- The contract of a tag entry, `TagEntry` in `eidos-core/symbol/schema/typeref.go`, of an annotation,
  `Annotation` in `eidos-core/symbol/annotation.go`, and of `Method.Receives` in
  `eidos-core/symbol/schema/callable.go`.
- The tag that protoc-gen-go writes: `timestamppb/timestamp.pb.go` of google.golang.org/protobuf
  v1.36.12.
- [Language guide (proto3)](https://protobuf.dev/programming-guides/proto3/): the zero value of an
  enum, the members of a oneof, the reserved statement and the extensions of custom options.
- [Feature settings for editions](https://protobuf.dev/programming-guides/editions/): field presence,
  `LEGACY_REQUIRED`, the closed enum, the reserved names of an edition, and `export` and `local`.
- [Style guide](https://protobuf.dev/programming-guides/style/): the naming conventions and the
  advice on packages and directories.
- [buf lint rules](https://buf.build/docs/lint/rules/): `PACKAGE_DIRECTORY_MATCH`,
  `ENUM_VALUE_PREFIX`, `RPC_REQUEST_STANDARD_NAME`, `RPC_RESPONSE_STANDARD_NAME` and
  `RPC_REQUEST_RESPONSE_UNIQUE`.
- protocompile at v0.14.2-0.20260917202354-386f9fcfc7b9: the experimental compiler in
  `experimental/ir` and `experimental/incremental/queries`, and the editions it knows in
  `experimental/ast/syntax/syntax.go`.
- [protoreflect v2.0.0](https://github.com/jhump/protoreflect/releases/tag/v2.0.0): `protoprint`
  and `protobuilder`.
