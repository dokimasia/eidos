---
rfc: 0068
title: The GraphQL satellite
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-19
updated: 2026-10-19
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0057, ADR-0058, ADR-0059
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0068: The GraphQL satellite

## Summary

We propose the satellite `eidos-lang-graphql`, which reads and writes every construct of GraphQL in
the September 2025 edition of the specification. The type system covers the schema definition and
its extension, scalars, object types, interfaces, unions, enums, input objects, directive
definitions, applied directives, descriptions, default values and the extension of every kind of
type. The executable language covers operations, fragments, variables and selections.

The frontend parses through `github.com/vektah/gqlparser/v2`, the parser of gqlgen. It is pure Go
and keeps the comments of a document with their positions. gqlparser refuses three forms of the
specification, and the satellite contributes the fixes upstream. Each schema loads as one unit with
its documents. A unit contains the files that a graphql-config project or a gqlgen configuration
lists, or else the files of one directory. The frontend types each selection of a document against
the schema of its unit, so an operation loads as a function whose result is the struct of its
selection.

The backend writes GraphQL documents from the emit model. A keyword that the model has no field for
is a marker, an annotation whose name contains a colon, such as `graphql:input`. The schema
statement is the header of a file, which a generator states through an `emit.File` in its unit. An
operation and a fragment write their selection sets from the verbatim body of a function. Shapes
that GraphQL has no built-in type for take the custom scalars of scalars.graphql.org or a
synthesized type, as the GraphQL ecosystem maps them.

The kernel gains the documentation of a parameter for the description of an argument, and the
optional rules capability `ShapeRules` for the shape of a custom scalar. The stock binary gains the
frontend, the rules, the target and the plan `graphql`. The support matrix gains a GraphQL column.

## Motivation

The satellites of eidos do not read or write GraphQL. That blocks these generators:

- A generator cannot read a GraphQL schema to write the Go server types or the TypeScript client
  types of an API, as it reads a protobuf service.
- A generator of typed clients cannot read the operations of an application, as graphql-codegen
  reads them to write a TypeScript type for the result of each operation.
- A generator cannot write a GraphQL schema or an operation from the declarations of Go,
  TypeScript, Java, Rust or protobuf.
- The completeness check and the support matrix have no GraphQL column.

A GraphQL schema spans files and often directories. gqlgen reads every file that the globs of its
configuration match, such as `graph/*.graphqls`, and a graphql-config project lists the globs of its
schema in the same way. A unit of one directory would split such a schema, and an extension would
lose the type that it extends.

## Detailed design

### Components

| Component | Package | Change |
|---|---|---|
| Model | `eidos-core/symbol/schema` | `Param.Doc` |
| Rules capability | `eidos-core/rules` | `ShapeRules`, which the fold asks for the shape of a declaration |
| Satellite | `eidos-lang-graphql` | new module `go.dokimi.dev/eidos/lang/graphql`: the language, the comment syntax, the keys, the markers, the kinds of operations, the built-in directives, the policies, the table of scalars, `Stated`, `Header`, `Scalar` and `ScalarShape` |
| Documents | `eidos-lang-graphql/internal/document` | new: the split of a file into its two languages, the parse of each part, the load of a schema and the rules of the validation of a client |
| Frontend | `eidos-lang-graphql/frontend` | new: the partition into schemas, the parse through the document package, the typing of selections and the lowering |
| Rules | `eidos-lang-graphql/rules` | new: the built-in scalars, the scalars of the table, the members, the roles, the resolution, the values and the enums |
| Spoke and names | `eidos-lang-graphql/spell` | new: `Type`, `Name`, `Filename` and `Package` |
| Backend | `eidos-lang-graphql/backend` | new: the kit composition, the templates, the coverage, the lowering, the clusters, the scaffold and the import block |
| Toolchain adapter | `eidos-lang-graphql/testing` | new: the parse and the validation of schemas and documents through the document package |
| Workspace | `go.work` | the module |
| Binary | `cmd/eidos` | the frontend, the rules, the target and the plan `graphql` |
| Conformance | `eidos-conformance` | the corpus, the matrix entry and the fixture `catalog` |

### Invariants

1. Every construct of GraphQL has a form in the model, and the backend writes it. The tables of
   constructs list each form.
2. A schema that gqlparser's validator accepts loads without an Error, and so does a document that
   the validator accepts against its schema.
3. The frontend loads each file that the backend writes into the declarations, the shapes, the
   annotations and the stamps that the emit model stated.
4. The toolchain adapter validates each schema and each document that the conformance fixtures
   write.
5. A construct, a fact, a kind or a name that GraphQL does not spell reports a positioned finding.

### The constructs

The type system:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `schema` | the documentation and the annotations of its file, the stamps `graphql.schema` and `graphql.roots` of the file, and `graphql.operation` on each root type | the header of a file with its root markers, and the marker `graphql:operation` on each root type |
| `extend schema` | the annotations of its file, and its stamps | the header of a file with the marker `graphql:extend` |
| `scalar` | a defined alias without a target | a defined alias without a target |
| `type` | a struct, with its interfaces in `Implements` | a struct |
| Field without arguments | a field | a field |
| Field with arguments | a method with a parameter for each argument and one result | a method |
| Argument and its default | a parameter, with its default and its documentation | a parameter |
| `interface`, and `implements` | an interface, with its interfaces in `Extends` | an interface |
| `union` | an alias whose target is the union of its members | an alias whose target is a union of references |
| `enum` and its values | an enum, and a variant for each value | an enum |
| `input` | a struct, and the stamp `graphql.input` | a struct with the marker `graphql:input` |
| Input field and its default | a field, with its default as its value | a field |
| `directive` | an interface with a field for each argument, and the stamps `graphql.directive` and `graphql.repeatable` | an interface with the markers `graphql:directive` and `graphql:repeatable` |
| Applied directive | an annotation | an annotation |
| `@oneOf`, `@deprecated` and `@specifiedBy` | an annotation | an annotation |
| Description | the documentation of its declaration | `Doc` |
| `#` comment | a carrier, or the trailing comment of a declaration | `Comment` |
| Extension of a type of the schema | the type, with the members and the directives of the extension | a field or a method whose `Receives` names the type |
| Extension of a type outside the schema | the declaration, and the stamp `graphql.extension` | a declaration with the marker `graphql:extend` |

The executable language:

| Construct | The frontend loads | A generator states |
|---|---|---|
| `query`, `mutation` and `subscription` | a function with a parameter for each variable and the struct of its selection as its result, and the stamps `graphql.operation` and `graphql.selection` | a function with the marker `graphql:operation` and its selection set as its verbatim body |
| Anonymous operation | a function named after the base name of its file, and the stamp `graphql.anonymous` | a function with the marker `graphql:anonymous` |
| Variable and its default | a parameter, with its default, its documentation and its directives | a parameter |
| `fragment` | a function with the struct of its selection as its result, and the stamps `graphql.fragment` and `graphql.selection` | a function with the marker `graphql:fragment` and its selection set as its verbatim body |
| Selection set on an object type | a struct, with a field for each response key | the verbatim body |
| Selection set that selects fields of particular types of an interface or a union | a sum with a variant for each possible type | the verbatim body |
| Field, alias and `__typename` | a field named after its response key, and the stamp `graphql.coordinate` | the verbatim body |
| `@include` and `@skip` | the optional form of the field, and an annotation | the verbatim body |
| Fragment spread and inline fragment | the fields that they select, and the stamp `graphql.spreads` | the verbatim body |
| Directive of an operation, a fragment or a variable | an annotation | an annotation |
| Description of an operation, a fragment or a variable | the documentation | `Doc` |

The sections below give the rules of each part.

### The parser

tree-sitter is the parser platform of the frontends, with two exceptions where a proven pure-Go
parser exists: the standard library's parser for Go and protocompile for protobuf. gqlparser is such
a parser for GraphQL:

- It is the parser of gqlgen, under the MIT license. Its release v2.5.63 of 2026-10-14 follows
  v2.5.62 of 2026-10-08 and v2.5.61 of 2026-10-07.
- Its module requires `github.com/agnivade/levenshtein`, `github.com/stretchr/testify` and
  `go.yaml.in/yaml/v3`, and no cgo.
- `parser.ParseSchema` keeps the `#` comments of a schema in its tree, with their positions.

A probe parsed a sample schema with v2.5.63 and measured these properties:

- The tree contained each of the 11 comments of the sample. A comment attaches before or after the
  description of a definition, a field, an argument or an enum value, at the end of a definition,
  to a root operation type, and at the end of the document.
- A comment after a field on the same line attaches to the next field. Its position tells it apart
  from a comment on the line of the next field.
- The tree did not contain 4 of 6 comments in rarer places: inside a default value, before the
  closing parenthesis of the arguments, before a directive and before a member of a union.
- `Position.Start` and `Position.End` count runes and not bytes. The column counts runes, as the
  column of the protobuf frontend does.
- `Line` and `Column` are exact for every token but two. The lexer gives a string the column of the
  rune after its opening quote, and a block string the line where it ends. The parser reports a
  syntax error at such a string at the same place.
- The parser dedents a block string, as the specification requires.
- The parser accepts an extension of every kind, an interface that implements an interface, a
  repeatable directive and `@oneOf`.

A second probe parsed executable documents with `parser.ParseQuery` and validated them with
`validator.Validate` against a schema:

- The validator gave each selected field the definition of the schema field that it selects, and
  the object type of its selection set. A field of an inline fragment had the type of the fragment's
  condition.
- The parser accepted an anonymous operation and the variables of a fragment, which gqlparser parses
  beyond the specification.
- `parser.ParseSchema` refused every executable definition, and `parser.ParseQuery` refused every
  type system definition. A document that mixes both languages parsed with neither.

The parser refuses three forms of the specification:

| Form | Example |
|---|---|
| An interface extension that adds interfaces, with or without fields | `extend interface B implements A` |
| A braced Unicode escape | `"\u{1F600}"` |
| The description of an operation, a fragment or a variable, which the September 2025 edition added | `"Fetches a user." query GetUser { … }` |

The tracker of gqlparser has no issue or pull request about any of the three. Its pull request 476,
merged on 2026-09-30, decodes a surrogate pair of `\u` escapes and leaves the braced form refused.
The satellite contributes a fix for each form upstream as a pull request, and pins the first release
of gqlparser that contains the three. A mixed document does not need a fix, because the frontend
parses each language from a copy of the file of its own.

### Schemas, units and packages

A unit is one schema, and its package is the schema. The partition groups the files this way:

1. It reads the configurations of the directory of each selected file and of every directory above
   it, up to the root of the workspace. A directory has the first configuration of graphql-config
   in the order in which graphql-config searches its names, and the first configuration of gqlgen
   in the order of gqlgen's search.
2. A configuration of graphql-config defines a schema for its keys `schema` and `documents`, and one
   for each project of its key `projects`. A configuration of gqlgen defines one schema for its key
   `schema`, which is `schema.graphql` where the configuration does not state it.
3. A schema claims each file that an entry of its keys `schema` and `documents` matches. A project
   of graphql-config also claims each file that its key `include` matches and its key `exclude`
   does not. Each entry is relative to the directory of its configuration.
4. A file joins the unit of the first schema that claims it. The configurations of the directory of
   the file come first, then those of each directory above it, and then those of every other
   directory in path order, because an entry can match a file outside the directory of its
   configuration. The projects of a configuration are in their order. Every later schema that
   claims the file reads the package of that unit as a shared package.
5. A file outside every schema joins the unit of the files of its directory.

An entry of graphql-config is a glob of minimatch, with `**` for any number of directories and
braces for alternatives. A leading `!` removes the files that the rest of the entry matches from the
entries of its key. An entry of gqlgen is a glob of Go, or a walk of the directory before its `**`.
gqlgen matches the rest of the path below that directory against the expression of the entry. An
entry that is not a path, such as a URL or a schema written in place, does not match a file. A
legacy configuration of graphql-config states the keys `schemaPath`, `includes` and `excludes`,
which graphql-config reads as `schema`, `include` and `exclude`.

graphql-config searches 26 names in a directory: 21 file names, `package.json` with its key
`graphql`, and the four legacy names from `.graphqlconfig` to `.graphqlconfig.yml`. A `package.json`
without the key is not a configuration, so the search continues after it. gqlgen searches
`.gqlgen.yml`, `gqlgen.yml` and `gqlgen.yaml`. The frontend reads the files in JSON, in YAML and in
TOML, and the key of `package.json`. `.graphqlrc` and `.graphqlconfig` without an extension are
YAML, of which JSON is a subset. The frontend parses YAML through `go.yaml.in/yaml/v3`, which
gqlparser requires already, and TOML through `github.com/pelletier/go-toml/v2` v2.4.3. It matches the
globs of graphql-config through `github.com/bmatcuk/doublestar/v4` v4.10.3. go-toml and doublestar
do not require another module.

The package of a schema is the directory of its configuration, and `<directory>/<project>` for a
project of the key `projects`. The package of a directory's unit is the directory. Every
configuration that the frontend reads is a shared input of every unit, so the parse of each unit
decides its schema and its shared packages from the same configurations. An edit of a configuration
changes the key of every unit.

The executable definitions of a unit load under a package of their own, the package of the unit
followed by `/documents`. GraphQL keeps the names of operations and of fragments apart from the
names of types, and the second package keeps the result types of the operations apart from the
types of the schema. A document joins the unit of its schema, so the frontend types its selections
while it parses the unit.

An entry of a key `documents` can match the source files of another language, such as
`src/**/*.tsx`, whose template literals contain GraphQL. The GraphQL frontend claims GraphQL files
alone, so such an entry does not select a file of it. A document inside a string of another
language is a value of that language.

The frontend does not read three kinds of configuration:

- 12 of the names of graphql-config are TypeScript and JavaScript files, such as
  `graphql.config.ts`. Such a file is code, and the frontend does not run it.
- A configuration that does not decode defines no schema.
- A configuration of gqlgen with an entry that gqlgen refuses defines no schema, because gqlgen does
  not load it. gqlgen refuses a glob that does not parse, a walk without a directory before its
  `**`, and a walk whose expression does not compile.

Such a configuration is a shared input of the unit of the first selected file in or below its
directory, and the parse of that unit reports it under `UnreadConfig`. The files that it would claim
join the units of other schemas or of their directories. gqlgen also refuses a walk of a directory
that does not exist. The partition reads the selected files and the configurations, and no listing
of a directory, so such a walk does not match a file.

A name resolves within the package of its schema, and then within its shared packages in the order
of their schemas. A name that the schema declares by an extension alone resolves in the shared
packages first. A reference then binds the type that the extension extends. A name in a document
resolves in the structs of the selections that enclose it, from the innermost one outward. It then
resolves in the package of the documents, and last as a name of its schema. The rules project the
name of a built-in scalar, which has no candidate.

### The frontend

The frontend claims the files `**/*.graphql`, `**/*.graphqls` and `**/*.gql`. It lowers each
definition as the table of constructs states.

A nullable type loads as the optional form of its type, and a non-null type as the type itself. A
list loads as the list form. So `[String!]` is an optional list of text, and `String` is an
optional text.

A defined alias without a target is a type of its own, so a reference to a custom scalar folds to a
reference and not to another type. A custom scalar whose `@specifiedBy` URL is in the table of
scalars projects as the shape of the table.

An extension merges into its type where the schema declares the type. The type then has the
members, the interfaces and the directives of every extension, as the specification defines the
effect of an extension. An extension of a type that the schema does not declare loads as that
declaration with the stamp `graphql.extension`. A subgraph of a federated graph declares such an
extension for a type of another subgraph.

The frontend stamps the file of a schema statement with `graphql.schema`, whose value is `schema`
where the file has a schema definition and `extend` where it has extensions alone. `graphql.roots`
lists the root operation types of the statements of the file as written, such as `query: Query`. The
frontend stamps `graphql.operation`, with the value `query`, `mutation` or `subscription`, on each
root type that a statement names. A schema without a definition takes the types `Query`, `Mutation`
and `Subscription` as its roots, as the specification defines the default names.

A directive definition loads as an interface, as the Java frontend loads an annotation type. Its
locations are the stamp `graphql.directive`. A repeatable directive also has the stamp
`graphql.repeatable`.

#### Mixed documents

A document can mix type system definitions and executable definitions. The frontend lexes the file
through gqlparser's lexer and assigns each definition at the top level to its language by its first
keyword. It parses the type system from a copy of the file whose executable definitions are blanks,
and the executable language from a copy whose type system definitions are blanks. Each copy keeps the
line breaks of the file, so every position in a copy is the position in the file, as in the protobuf
frontend's copy of an edition 2026 file.

The frontend records each error of the lexer and lexes again from the start of the next line,
because the lexer of gqlparser stops at its first error. The error leaves out the runes from the end
of the token before it to the end of its line. Its message is the message of gqlparser without the
location, because a lexer that resumes counts its lines from the resume. A parse that fails leaves
out the definition with the error and parses its copy again, so a syntax error costs the definition
that contains it and no other definition. The frontend sets the line and the column of each token
from the rune where the token starts. A syntax error at a string or a block string is then at its
opening quote.

#### Operations and fragments

The frontend validates the documents of a unit against the schema of the unit through gqlparser's
validator, which gives each selected field the definition of the schema field that it selects. The
validation runs the rules of a client: the default rules of gqlparser without the three rules that
graphql-tools leaves out for a client. Those three refuse a fragment that the documents do not
spread, a variable that the operation does not use, and a directive that the schema does not define,
such as a directive of a client cache. The documents of a unit validate together, so a spread can
refer to a fragment of another document. The frontend checks each document for an anonymous
operation beside another operation. The rule of unique operation names reads the operations that
have a name, because the anonymous operations of two documents do not share a name. The frontend
then lowers each executable definition:

- An operation is a function named after the operation, in the package of the documents. It has one
  parameter for each variable and one result, the struct of its selection, named
  `<Operation><Kind>` with `Query`, `Mutation` or `Subscription` as the kind. The result of a
  subscription is an asynchronous stream of its struct, because a subscription responds with a
  sequence of results.
- An anonymous operation is the only operation of its document, and its function takes the base
  name of the file without its extension.
- A fragment is a function named after the fragment, whose result is the struct
  `<Fragment>Fragment`. The variables of a fragment, which gqlparser parses beyond the
  specification, are its parameters.
- A variable is a parameter with its type, its default, its description and its directives.

graphql-codegen names the result types of operations and fragments in the same way, as in
`GetUserQuery` and `UserFieldsFragment`.

A selection set loads as the type of the value that a response contains for it:

- On an object type, it is a struct with one field for each response key. The struct of a field's
  selection is a nested type of the enclosing struct, named after the response key.
- On an interface or a union, it is a struct where every field of the selection applies to every
  possible type. Where a fragment selects fields of particular types, it is a sum with a variant for
  each possible type that the schema lists, and each variant has the fields that apply to its type.
- A field has the type that the schema gives the field it selects, with the struct or the sum of its
  selection in place of the named type. Every node of that type is at the field in the document,
  because the schema writes the type in another file. The stamp `graphql.coordinate` of the field
  is the schema coordinate of the field that it selects, such as `User.name`.
- A field under `@include` or `@skip`, or inside a fragment with either directive or with `@defer`,
  is optional, because the response can lack it.
- A fragment spread and an inline fragment add the fields that they select to the struct or the
  variant of each type that their condition includes. The stamp `graphql.spreads` of a struct or a
  variant lists the fragments that it spreads, in source order.

The stamp `graphql.operation` of the function is the kind of the operation, and the stamp
`graphql.fragment` is the type condition of a fragment. The stamp `graphql.selection` is the
selection set as written, from its opening brace to its closing brace. A client sends that text with
its operation.

The files of the shared packages of a unit are shared inputs of the unit, so its parse reads every
file of the schema that types its documents. The frontend loads that schema through
`validator.ValidateSchemaDocument` of gqlparser, over the documents that the copies of the files
parsed and one shared parse of the prelude of gqlparser. The prelude declares the built-in scalars,
the built-in directives and the types of introspection. `gqlparser.LoadSchema` parses its sources
itself, and a mixed document parses with neither parser of gqlparser. Where the load fails, the
frontend reports the message of gqlparser at the position of the error under `InvalidDocument`, and
the documents of the unit load without types.

A field that the validator does not type has a reference without a target, spelled with its schema
coordinate. The validator reports such a field, as it reports every other error of a document, and
the frontend reports each error under `InvalidDocument`.

#### Comments and descriptions

A description is the documentation of a GraphQL declaration. It enters through the kit's
`DocLines`, because the parser returns it as clean text. A `#` comment is neither documentation nor
a directive. The carriers of the comments directly above a declaration and of the comment after it
on its line attach to the declaration, and that comment is its trailing comment. The frontend reads
nothing else from a comment. `Syntax` returns the line form `#` and no directive convention, so a
description line such as `todo:x` is documentation.

The frontend attributes each comment from the tokens of its one lexing of the file. A carrier in a
comment that the frontend attributes to no declaration reports
`UnaddressedCarrier` at its position, so a carrier in one of the rarer places does not disappear
without a finding.

#### Findings

| Code | Finding |
|---|---|
| `UnparsedFile` | a syntax error, at the token that caused it |
| `InvalidDocument` | an error of gqlparser's validator in a document, at its position |
| `UnreadConfig` | a configuration in TypeScript or JavaScript, a configuration that does not decode, and a configuration of gqlgen with an entry that gqlgen refuses, at the file |
| `BadCarrier` | a carrier that the kernel's grammar refused, at its line |
| `UnaddressedCarrier` | a carrier in a comment that the frontend attributes to no declaration, at the carrier |

### The documentation of a parameter

GraphQL documents each argument with a description of its own, and `Param` has no documentation.
`Param` gains it:

```go
// Doc is the documentation of the parameter, such as the description of a
// GraphQL argument. A language that documents a parameter in the
// documentation of its callable leaves it empty.
Doc []string `eidos:"both"`
```

The field is a schema edit and a regeneration of the model, without a fact, as the documentation of
every other declaration has none. The GraphQL frontend and backend read and write it. A generator
into another language writes the documentation of a parameter into the documentation of its
callable, where that language places it.

### The rules

| Question | Answer |
|---|---|
| Builtin | `Int` is a signed integer of 32 bits, `Float` a float of 64 bits, `String` and `ID` text, `Boolean` a bool |
| ShapeOf | a custom scalar with a `@specifiedBy` URL of the table has the shape of the table |
| Members | the fields and the methods that a type declares, because an object type restates the fields of its interfaces |
| ParamRole | every argument is an input |
| ReturnRoles | one result |
| Resolve | the declaration of a directive's spelling in the package |
| Values | samples, zeros and literals in GraphQL's grammar of values |
| EnumOf | the variants, each with its name as its value, as GraphQL serializes an enum value |

A reference folds to a reference to its declaration, unless the rules of its language bless the
declaration as a well-known type. A custom scalar whose specification is a signed integer of 64 bits
is neither. The kernel's rules gain an optional capability for such a declaration:

```go
// ShapeRules is the optional capability of a language that declares a type
// whose values have the shape of another form, such as a GraphQL custom
// scalar whose specification is a signed integer of 64 bits. The fold asks
// the rules of the language of a target after [WellKnownRules] and before
// it classifies the target.
type ShapeRules interface {
    // ShapeOf returns the shape of the values of decl, and reports false
    // for a declaration whose references fold to a reference to it. The
    // fold gives the shape the spelling of the reference. ShapeOf reads
    // through v, so each read records.
    ShapeOf(decl symbol.Symbol, v View) (TypeShape, bool)
}
```

The GraphQL rules implement it through `ScalarShape`. A reference to a scalar `Timestamp` with the
URL of `DateTime` then folds to the well-known timestamp, which every spoke spells.

### The table of scalars

The root package of the satellite declares the scalars of scalars.graphql.org that have a shape of
the model:

| Scalar | Specification | Shape |
|---|---|---|
| `DateTime` | `https://scalars.graphql.org/andimarek/date-time.html` | the well-known timestamp |
| `Duration` | `https://scalars.graphql.org/chillicream/duration.html` | the well-known duration |
| `Long` | `https://scalars.graphql.org/chillicream/long.html` | a signed integer of 64 bits |
| `UnsignedInt` | `https://scalars.graphql.org/chillicream/unsigned-int.html` | an unsigned integer of 32 bits |
| `UnsignedLong` | `https://scalars.graphql.org/chillicream/unsigned-long.html` | an unsigned integer of 64 bits |
| `Base64String` | `https://scalars.graphql.org/chillicream/base64-string.html` | bytes |
| `Any` | `https://scalars.graphql.org/chillicream/any.html` | Dynamic |

The rules read the table to project a custom scalar, and the spoke reads it to spell a choice of a
policy. `ScalarShape` returns the shape of a custom scalar, and `Scalar` returns the declaration of
one scalar of the table:

```go
// ScalarShape returns the shape of a custom scalar with the annotations a.
// It returns the shape of the entry of the table whose URL is the argument
// of the annotation specifiedBy in a. ScalarShape reports false where a has
// no such annotation, and for a URL that the table does not contain.
func ScalarShape(a symbol.Annotations) (rules.TypeShape, bool)

// Scalar returns the emit declaration of the scalar of the table named
// name. The declaration is a defined alias without a target, with the
// annotation specifiedBy and the URL of its specification. Scalar reports
// false for a name that the table does not contain. A generator appends the
// declaration once to the schema whose references spell the scalar.
func Scalar(name string) (*emit.Alias, bool)
```

The frontend writes each argument of an applied directive as its name, a colon, a space and its
value in a canonical form, such as `url: "https://scalars.graphql.org/chillicream/long.html"`. A comparison of
the argument with the argument of each entry as text is then exact.

### The backend

The backend is a kit composition. It declares the policies of the spoke. The kit requires a scaffold
and an import renderer of every language. GraphQL has no statements and no imports, so the scaffold
refuses every statement and the import renderer returns the empty block:

```go
// New returns the GraphQL rendering backend, which writes the schema
// definition language and executable documents. Its spoke is [spell.Type],
// which spells a type of another language in GraphQL under the policies of
// the satellite. Files finalise through the shared normalizer, because the
// templates write the layout of graphql-js.
func New() backend.Backend {
    return backend.New(graphql.Name, graphql.Target, graphql.Syntax()).
        Version(graphql.BackendVersion).
        FileTemplate(FileTemplate).
        KindTemplates(KindTemplates()).
        RefusedKinds(RefusedKinds()).
        Coverage(Coverage()).
        Funcs(Funcs).
        Naming(spell.Filename).
        Packages(spell.Package).
        Respell(spell.Name).
        Types(spell.Type).
        Policies(graphql.Policies()...).
        Lower(Lower).
        Cluster(Cluster).
        Groups(Groups()).
        Scaffold(Scaffold).
        Imports(Imports).
        Finalise(textfmt.Normalize).
        Build()
}
```

#### Markers

A marker states a GraphQL keyword that the model has no field for. It is an annotation whose name
starts with `graphql:`. A GraphQL name cannot contain a colon, so no directive has the name of a
marker. The satellite declares the markers:

```go
// MarkerPrefix opens the name of every [Marker]. A GraphQL name cannot
// contain a colon, so no directive has the name of a marker.
const MarkerPrefix = "graphql:"

// Marker is the name of an annotation that states a keyword of GraphQL that
// the model has no field for. A generator states a marker as an annotation
// with the name of the marker, and the backend writes the keyword or the
// statement for it and no directive.
type Marker string

// The markers of the GraphQL target.
const (
    // InputMarker marks a struct as an input object, and a sum as an input
    // object with the directive oneOf.
    InputMarker Marker = MarkerPrefix + "input"
    // ExtendMarker marks a declaration as the extension of a type, and a
    // header as the extension of the schema.
    ExtendMarker Marker = MarkerPrefix + "extend"
    // OperationMarker marks the root type of an operation, and a function
    // as an operation. Its one argument is query, mutation or subscription.
    OperationMarker Marker = MarkerPrefix + "operation"
    // AnonymousMarker marks an operation that the backend writes without
    // its name.
    AnonymousMarker Marker = MarkerPrefix + "anonymous"
    // FragmentMarker marks a function as a fragment. Its one argument is
    // the type condition of the fragment.
    FragmentMarker Marker = MarkerPrefix + "fragment"
    // QueryMarker is a header marker. Its one argument is the name of the
    // root type of queries in the schema statement of the file.
    QueryMarker Marker = MarkerPrefix + "query"
    // MutationMarker is a header marker. Its one argument is the name of
    // the root type of mutations in the schema statement of the file.
    MutationMarker Marker = MarkerPrefix + "mutation"
    // SubscriptionMarker is a header marker. Its one argument is the name
    // of the root type of subscriptions in the schema statement of the file.
    SubscriptionMarker Marker = MarkerPrefix + "subscription"
    // DirectiveMarker marks an interface as the definition of a directive,
    // with the locations of the directive as its arguments, such as OBJECT
    // and FIELD_DEFINITION. The fields of the interface are the arguments
    // of the directive.
    DirectiveMarker Marker = MarkerPrefix + "directive"
    // RepeatableMarker marks the definition of a repeatable directive.
    RepeatableMarker Marker = MarkerPrefix + "repeatable"
)

// Valid reports whether m is one of the markers that the satellite
// declares. The backend refuses an annotation whose name starts with
// [MarkerPrefix] and is not a valid marker.
func (m Marker) Valid() bool

// Operation is a kind of operation of GraphQL. It is the keyword of an
// operation, the argument of [OperationMarker] and the value of
// [OperationKey].
type Operation string

// The kinds of operation of GraphQL.
const (
    OperationQuery        Operation = "query"
    OperationMutation     Operation = "mutation"
    OperationSubscription Operation = "subscription"
)

// Valid reports whether o is one of the three kinds of operation. The
// backend refuses the marker of an operation whose argument is not valid.
func (o Operation) Valid() bool
```

The satellite also declares a constant for each built-in directive that it reads: `specifiedBy`,
`oneOf`, `deprecated`, `include`, `skip` and `defer`.

#### The schema statement

The header of a file states its schema statement. The backend writes `schema` with the documentation
of the header as its description, the GraphQL annotations of the header as its directives, and the
root markers as its root operation types. A header with the marker `graphql:extend` writes
`extend schema`, which has no description. The backend does not write a statement for a header
without a root marker, a directive and documentation. A schema whose root types have the default
names does not need one.

The lowering reads the marker `graphql:operation` of a struct apart from the header. A struct whose
marker has the argument `subscription` is the root of subscriptions. The lowering refuses a field of
a stream type in any other struct, and the marker does not write a statement.

#### Kinds

| Emit kind | SDL |
|---|---|
| Struct | `type`, and `input` with the marker `graphql:input` |
| Interface | `interface`, and `directive` with the marker `graphql:directive` |
| Enum | `enum` |
| Alias of a union of references | `union` |
| Defined alias without a target | `scalar` |
| Sum | a `union` of object types, one for each variant, and an `input` with `@oneOf` under the marker `graphql:input` |
| Field or method with `Receives` | a member of an `extend` block |
| Function | an operation with the marker `graphql:operation`, and a fragment with the marker `graphql:fragment` |
| Variable, Constant | refused kinds, because a schema declares neither |

The writer of each kind writes the description of its declaration, so a kind template calls one
helper. The helper `docs` of the vocabulary serves the templates of generators. A trailing comment
ends on the line of its declaration. The writer writes each line terminator of a comment as a space.

A declaration with the marker `graphql:extend` writes `extend` before its keyword, as in
`extend union` and `extend scalar`. A struct writes its fields as fields without arguments, and then
its methods as fields with arguments. A nested declaration of a type moves to file level after the
type, under its emitted name and with the origin of the type, because GraphQL has no nested types
and the settle gives every output of a lowering the origin of its input.

The backend's `Cluster` gathers the fields and the methods with `Receives` of one unit by the
spelling of their `Receives`. The group template `extend` writes each cluster as one block, with its
members in the order of the unit. A block extends an interface where the target of `Receives` is an
interface, and an object type otherwise. The identity of a struct does not state whether it is an
input object. An extension of an input object takes the marker `graphql:extend` instead.

A sum writes a union named after the sum. Each variant writes an object type `<Sum><Variant>` with
the fields of the variant. Under the marker `graphql:input`, a sum writes an input object with
`@oneOf` and one nullable field for each variant:

- A variant with one field takes the type of that field.
- A variant with two or more fields takes a synthesized input object `<Sum><Variant>Input` with
  those fields.
- A variant without a field takes `Boolean`, whose presence selects the variant.

The specification requires at least one field of an object type, so the lowering refuses an
object-side variant without a field.

#### Operations and fragments

A function with the marker `graphql:operation` writes an operation:

- The argument of the marker is the keyword of the operation, and the name of the function is the
  name of the operation. The marker `graphql:anonymous` leaves the name out. The variables of an
  anonymous operation follow its keyword after a space, as graphql-js prints them:
  `query ($id: ID!) {`.
- An anonymous query without a description, variables and directives writes its selection set
  alone, because the shorthand of a query takes none of them.
- Each parameter writes a variable: `$` and its name, its type through the spoke, its default, its
  documentation as its description and its annotations as its directives.
- The annotations of the function are the directives of the operation, and its documentation is its
  description.
- The verbatim body of the function is the selection set, which the backend writes without the
  white space around it.

A function with the marker `graphql:fragment` writes `fragment <Name> on <Type>`, with the argument of
the marker as its type condition, its parameters as its variables, and its body as its selection set.

The results of such a function do not write anything, because the selection set states what a
response contains. A generator into another language writes the result types of a client from the
structs that the frontend loads.

#### The lowering

`Lower` refuses each construct of this table under `RefusedConstruct`, and the settle withholds the
declaration. The lowering also synthesizes the types of the spoke and the payloads of the next
section.

| Construct | Reason |
|---|---|
| An annotation whose name is neither a GraphQL name nor a marker of the satellite | the name of a directive is a GraphQL name |
| A marker that the satellite does not declare, a marker on a declaration of another kind or without the marker that it qualifies, a marker with arguments outside its contract, and a marker that a declaration states twice | each marker has one meaning on one kind |
| An argument of a directive that is not a name, a colon and a constant value | an argument of an applied directive is a named value |
| A header that states a schema definition without a root operation type, and an extension of the schema without a directive or a root operation type | a schema definition names a root operation type, and an extension of the schema adds one or a directive |
| An extension of a type or of the schema with documentation | an extension has no description |
| A struct or an interface without a field and a method, except an extension that adds directives or interfaces | an object type, an input object and an interface declare at least one field |
| A member name that a type declares twice | a type declares the name of a member once |
| A field without a type, a method without a result, and a parameter without a name or a type | a field and an argument have a name and a type |
| A default of a field of an object type | the fields of an object type have no defaults |
| A default that does not parse as a GraphQL value | a default is a value of GraphQL |
| A struct with the marker `graphql:input` and a method, an interface or the marker `graphql:operation` | an input field does not take arguments, and an input object does not implement an interface and is not a root operation type |
| A field or a method with `Receives` inside a type, and a field or a method at file level without `Receives` | an extension of GraphQL is a block of its own |
| A field of a stream type outside a struct with the marker `graphql:operation` and the argument `subscription` | a field streams on the root of subscriptions alone |
| A reference with type arguments, an optional of an optional, a form with the wrong number of children, and a form that GraphQL does not spell | GraphQL has no generic type, and a type has one nullability |
| A keyword variadic parameter | GraphQL has no keyword arguments |
| An enum without a value, except an extension that adds directives | an enum declares at least one value |
| An alias that is neither a union of named references nor a defined alias without a target, and a union without a member | GraphQL has no type alias, and a union joins at least one type |
| A sum without a variant or with methods, and an object-side variant without a field | a union joins at least one type, and an object type declares at least one field |
| A nested declaration that is not a type | a type nests types alone |
| A directive definition with methods, interfaces, nested declarations, a map argument or the marker `graphql:extend` | a directive definition declares its arguments alone, and an argument takes a type of the schema |
| A function without the marker `graphql:operation` or `graphql:fragment`, and a function with both | a document does not declare another callable |
| The marker `graphql:anonymous` on a fragment | a fragment has a name |
| A function whose body is not verbatim, has the content of a slot, or does not parse as one selection set | the body of an executable definition is its selection set |

A method with two or more results writes a field whose type is a synthesized payload,
`<Type><Field>Payload`, with a field for each result. A named result gives its field its name. An
unnamed result is `result<Position>`, as in `result2`. Relay names the result type of a mutation
after the mutation with the suffix `Payload`. The synthesized name puts the name of the type in
front, so two types with a field of one name do not collide.

#### Facts

| Verdict | Facts |
|---|---|
| Renders | Comment, Annotations, Optional, ParamDefault, Value on a field, Implements on a struct, Extends on an interface, Properties, Defined, Types, Methods on a struct, MultiReturn and NamedReturn on a method, Variadic, Receives |
| `Holds` | Visibility, Async, Throws, Final, Mutability, Abstract on a method, Level on a struct, MultiReturn on a function |
| Refuses | Abstract on a struct, Override, DefaultBody, HardPrivate, Label, ConstEnum, Underlying, Fields, Methods on an enum and on a sum, Value on an enum variant, Comment on a result, Tag, Sealed, Embeds, Extends on a struct, Permits, Variance, ConstParam, TypeParamDefault |
| Rejects | TypeParams, Level on a member, Accessor, Indexer, Constructs, Computed |

These verdicts need a reason:

- Visibility takes the verdict `Holds`, because a schema exposes every type that it declares. The
  respell refuses a name at a visibility other than public.
- Async takes `Holds`, because a resolver can return its value asynchronously. Throws takes `Holds`,
  because every field can fail with an error in the response.
- Types renders, because the lowering writes a nested declaration at file level under its emitted
  name. GraphQL has no nested types, and the generators of the ecosystem flatten nested classes.
- Variadic renders as a list argument, and MultiReturn and NamedReturn render through the payloads.
  On a function, MultiReturn takes `Holds`, because the selection set of an executable definition
  states its result. A function does not state NamedReturn.
- Properties renders, because the fields of an interface write as its fields.
- Value refuses on an enum variant, because GraphQL serializes an enum value by its name.
- Comment refuses on a result, because a field has the one trailing comment of its method.
- Level takes `Holds` on a struct, because a type at file level binds to no instance. Level takes
  `Rejects` on a member, because a static field or method has no form in a schema.
- TypeParams takes Rejects. A generic type has no GraphQL form, and Relay's connections are concrete
  types that a generator writes for each node type.

#### The spoke

| Form | SDL |
|---|---|
| Signed integer of 8, 16 or 32 bits, unsigned integer of 8 or 16 bits | `Int` |
| Unsigned integer of 32 bits | the choice of `graphql.uint32` |
| Signed integer of 64 bits or of the platform's width | the choice of `graphql.int64` |
| Unsigned integer of 64 bits or of the platform's width | the choice of `graphql.uint64` |
| Float of 32 or 64 bits | `Float` |
| Bool, Text | `Boolean`, `String` |
| Bytes | the choice of `graphql.bytes` |
| Well-known timestamp, duration | the choices of `graphql.timestamp` and `graphql.duration` |
| Dynamic | the choice of `graphql.dynamic` |
| Map of K to V | the choice of `graphql.map` |
| Optional of T | T without `!` |
| A form that is not optional | the type with `!` |
| List of T | `[T]` |
| Asynchronous stream of T | T, on a field of the root of subscriptions |
| Reference, Sum | the referent's flat name |

Each policy chooses between a built-in type that represents the value in JSON and a scalar of the
table. The default of each policy is the built-in type, so a written schema validates without a
declaration that the plan lacks:

| Policy | Choices | Default |
|---|---|---|
| `graphql.int64` | `String`, `Float`, `Long` | `String` |
| `graphql.uint32` | `Float`, `String`, `UnsignedInt` | `Float` |
| `graphql.uint64` | `String`, `Float`, `UnsignedLong` | `String` |
| `graphql.bytes` | `String`, `Base64String` | `String` |
| `graphql.timestamp` | `String`, `DateTime` | `String` |
| `graphql.duration` | `String`, `Duration` | `String` |
| `graphql.dynamic` | `String`, `Any` | `String` |
| `graphql.map` | `entries`, `Any` | `entries` |

A `Float` represents every integer of 32 bits exactly, and an integer above 2^53 loses precision.
The choice `entries` writes a map as a list of a synthesized entry type, `<Type><Field>Entry`, with
the fields `key` and `value`, and an input object `<Type><Field>EntryInput` in an input object. An
argument and a variable are inputs. A map argument takes `<Type><Field><Argument>EntryInput`. A map
variable takes `<Operation><Variable>EntryInput`. A plan that chooses a scalar of the table declares
it with `Scalar`.

The spoke refuses a scalar of another width, such as 128 bits, an array, a tuple, a union, an
intersection, a function type, a synchronous stream, a borrow, a wildcard, an inline type and an
opaque type. GraphQL has no form for them, and its ecosystem has no common mapping. The spoke also
refuses the well-known error, the well-known empty value and a reference with type arguments,
because GraphQL does not declare a failure value, an empty value or a generic type. It refuses an
optional of an optional, because a GraphQL type has one nullability, and a stream inside another
form.

#### Names

| Declaration | Convention | Example |
|---|---|---|
| type, interface, union, enum, input object, scalar | PascalCase | `CreateUserInput` |
| field, argument, directive | camelCase | `createdAt` |
| enum value | SCREAMING_SNAKE_CASE | `IN_PROGRESS` |
| file | snake_case with `.graphql` | `catalog_schema.graphql` |

These are the conventions of graphql.org's guide to naming. The respell refuses a name that starts
with `__`. The specification reserves such names for introspection. The respell also refuses a type
whose spelling is the name of a built-in scalar, such as `ID` for `id`, because a schema does not
declare a built-in scalar.

#### Files

`spell.Package` returns one package for every routed file, because a GraphQL schema has one
namespace. The layout does not qualify a reference. The settle checks the names of each package of
the sources for a collision. A file has no imports. The output contract writes its header and its
provenance trailer through `#`.

### Restating a GraphQL source

The frontend records applied directives as annotations, which a generator copies as they are. The
root package of the satellite restates the stamps:

```go
// Statement is the emit statement of the stamps of one declaration.
type Statement struct {
    // Annotations are the markers, in the order that [Stated] states.
    Annotations symbol.Annotations
    // Body is the selection set of an operation or a fragment as its
    // verbatim body, and the zero Body for every other declaration.
    Body emit.Body
}

// Match is a match that reads the declarations of the graph through the
// tracked reader of its invocation. Every match of the kernel is one, such
// as [eidos.StructMatch] and [eidos.GraphMatch].
type Match interface {
    eidos.Matcher
    Reader() *store.Reader
}

// Stated returns the emit statement of the stamps that the GraphQL frontend
// stamped on the declaration id. A generator over GraphQL sources appends the
// markers to the annotations of the emit declaration that it writes for id,
// and sets the body of its function to the body of the statement. The
// GraphQL backend then writes the declaration as its source states it.
//
// The markers are in this order: the input marker of an input object, the
// extend marker of an extension of a type outside its schema, the operation
// marker with the kind of the operation, the anonymous marker, the fragment
// marker with the type condition, the directive marker with the locations
// of a directive definition, and the repeatable marker. The body of an
// operation and of a fragment is its selection set as written. A
// declaration that the frontend did not stamp has the zero statement.
//
// Each read records on the invocation of m.
func Stated(m eidos.Matcher, id symbol.Identity) Statement

// Header returns the header of the file that declares d, and nil for a file
// without a schema statement. d is a declaration or a file. The documentation
// of the header is the description of the schema definition of the file. Its
// annotations are the directives of the schema statements of the file, a
// root marker for each root operation type that the statements name, and the
// extend marker for a file with extensions of the schema alone. For a
// declaration of the package of the documents, Header returns the header of
// the file in the package of the schema.
//
// Each read records on the invocation of m.
func Header(m Match, d node.Declaration) *emit.File
```

The kernel's `eidos.Matcher` has no exported subject, so `Header` takes the declaration whose file it
reads. It reads the node of the file through the tracked reader of the match, as the protobuf
satellite reads the header of a file.

### The toolchain adapter

- The parse runs the split of the document package over each file of the scratch tree, so a mixed
  document parses as a schema document and a query document, as the frontend parses it. It returns
  an error for each error of the lexer and each syntax error, with its file, its line and its
  column.
- The type check returns the syntax errors first. It loads one schema from the type system of every
  file through the load of the frontend, which validates the schema with the built-in scalars and
  directives of the specification. It then validates the executable definitions of every file
  together against that schema under the rules of a client, and checks each file for an anonymous
  operation beside another operation, as the frontend does. Each error contains its file, its line
  and its column.
- `Satisfies` reports whether the validated schema lists the contract for the type in
  `Schema.Implements`, which contains the interfaces and the unions of each type. It returns an
  error for a tree that does not parse, for a schema that does not load, and for a type or a
  contract that the schema does not declare.
- `RunTests` returns an error that identifies the missing capability, because a schema does not
  contain tests.

`Available` reports true on every machine, because each step runs in process.

### The binary

```go
Frontends(gofrontend.New(nil), tsfrontend.New(), javafrontend.New(nil), rustfrontend.New(nil),
    protofrontend.New(), graphqlfrontend.New()).
Rules(gorules.New(), tsrules.New(), protorules.New(), graphqlrules.New()).
Targets(golang.Target, typescript.Target, java.Target, rust.Target, protobuf.Target, graphql.Target).

workspace.Plan{
    Name:       string(graphql.Target),
    Sources:    workspace.Sources{Lang: graphql.Lang, Packages: []string{treeSources}},
    Backend:    graphqlbackend.New(),
    Extensible: true,
},
```

### Tests

The corpus of the satellite declares these verdicts on the feature inventory:

| Feature | Verdict | Reason |
|---|---|---|
| struct_fields, struct_methods | projects | fields, and fields with arguments |
| method_overloads | refuses | a type declares a field name once |
| constants | refuses | GraphQL has no constants |
| cross_package_ref | projects | a schema reads the packages that its configuration shares |
| composite_refs, builtin_ref | projects | lists, nullability and the five built-in scalars |
| directive_carrier | projects | `#` carriers |
| test_classification | refuses | a schema has no test files |
| interfaces, enum_values | projects | |
| directive_sugar | refuses | the brand's directives are carriers alone |

A feature with a sibling package states a graphql-config configuration whose key `schema` names the
files of the sibling, because a unit reads another package only as a shared package of its schema.

The conformance module gains the fixture `catalog`. Its schema spans two directories that one
`.graphqlrc.yml` names, and states every construct of the type system: a schema definition and an
extension of it, an interface that another interface implements, object types, a union, an enum
with a deprecated value, an input object with defaults, a `@oneOf` input object, a custom scalar
with `@specifiedBy`, a repeatable directive, an extension with a base and one without, descriptions
of every kind and carriers. The documents of the fixture state every construct of the executable
language: named and anonymous operations of the three kinds, variables with defaults and
descriptions, fragments, aliases, `@include` and `@skip`, inline fragments on an interface and on a
union, and a document that mixes both languages. The plans of the fixture write these files:

- `go-model` writes one file, `model/model.go`, as gqlgen writes the models of a schema into one
  file. An object type and an input object are a struct, an interface is an interface, and an enum
  is a defined string type with a constant for each value. A union is an interface without methods,
  and a custom scalar outside the table is a defined string type. A root operation type is the
  interface `<Type>Resolver` with a method for each field, which takes a context and the arguments
  of the field and declares a failure. The Go toolchain type-checks the file.
- `ts-client` writes one file, `client/client.ts`, as graphql-codegen writes the types of a schema
  and of its operations into one file. It writes the interfaces, the enums and the aliases of the
  schema, the result type of each executable definition, and the interface `<Definition>Variables`
  of its variables. The property of a nullable field is optional. A property has the GraphQL name
  of its field, which is a key of the JSON of a response. Where the camel convention of TypeScript
  would change the name, as it changes `__typename`, the property is a computed property with the
  key as a string literal. A selection that selects fields of particular types is an alias of the
  union of one interface for each possible type. The property `__typename` of each interface has
  the name of its type as its literal type, because a response discriminates the possible types by
  that key alone. tsc type-checks the file.
- `graphql-mirror` restates each declaration through `Stated` and each file through `Header`, and
  writes the mirror of each file under `mirror/` as `<stem>_mirror.graphql`. A field that an
  extension in another file adds to an object type or to an interface is a member with `Receives`
  in the mirror of that file. The definition of a type states the other parts of its extensions.
  The sources are written in the layout of the backend, so each written file equals its source
  without its carriers, between the header and the provenance trailer that the output contract
  adds. The toolchain adapter validates the schema and the documents, and the frontend loads them
  back with the stamps of their sources.

The matrix entry of GraphQL fills the read side, the projection side, the render side and the hub.
The hub cell of a map spells `?`, because the spoke spells a map as the map form, and the lowering
names the entry type after the type and the field that a probe of the hub does not have.

### Failures

| Failure | Code | Effect |
|---|---|---|
| A syntax error | `UnparsedFile` | the file contributes every definition outside the definitions with an error |
| An error of the validator in a document | `InvalidDocument` | a field that the validator does not type loads without a target |
| A schema that does not load | `InvalidDocument` | the documents of the unit load without types |
| A configuration in code, a configuration that does not decode, and a configuration of gqlgen with an entry that gqlgen refuses | `UnreadConfig` | the files that it would claim join the units of other schemas or of their directories |
| A carrier that the frontend cannot place | `UnaddressedCarrier` | the carrier does not apply |
| A construct that the lowering refuses | `RefusedConstruct` | the settle withholds the declaration |
| A stated fact without an idiom | `RefusedFact` | the declaration renders without the fact, with a warning |
| A shape that the spoke refuses | `RefusedType` | the generator's emitter reports it at the source declaration |

### Migration

`Param.Doc` is a schema edit and a regeneration of the model. Every sealed state runs cold once after
it. `ShapeRules` is an optional capability, so the rules of the other languages do not change. A
workspace with GraphQL files loads them in the stock binary, and the plan `graphql` does not write a
file without a plugin.

## Alternatives

### A tree-sitter grammar

tree-sitter is the platform of the frontends. `bkegley/tree-sitter-graphql` last changed on
2021-05-10. `joowani/tree-sitter-graphql` released v0.3.0 with bindings for tree-sitter 0.27 on
2026-10-02, and it has one star. Both need the cgo of tree-sitter's Go bindings, and gqlparser does
not.

### graphql-go/graphql

Its parser at v0.8.1 rejected an interface that implements an interface, a repeatable directive and
an extension of an interface, a scalar and a union in a probe. Its printer wrote none of the
comments of the probe's sample.

### graph-gophers/graphql-go

At v1.10.3 it reads a `#` comment as a description unless the caller sets `UseStringDescriptions`.
It rejected an extension of a scalar and the interface extension without a body.

### wundergraph/graphql-go-tools

v2.28.0 accepted every form of the probes. Its module requires 36 modules directly, among them
gqlgen, gqlparser itself, gRPC and a commit of protocompile other than the one that the protobuf
satellite pins.

### A unit of one directory

Each directory could be one schema, as each directory of Go is one package. The common layouts of
gqlgen and of graphql-tools spread one schema over the directories of its features. A type and its
extension then load in two units, and the extension loses its type. These tools define a schema in a
configuration, so the partition reads it.

### One package for the whole workspace

With one namespace for every GraphQL file of a workspace, a schema could span directories without a
configuration. Two schemas of one workspace, such as two subgraphs of a federated graph, would then
collide on their root types.

### A model field for an input object

`Struct.Input` with a fact of its own would state the keyword in the model. Every other backend would
then state one more verdict, and every sealed state would run cold once. A marker is an annotation
that a generator of the GraphQL target writes into its own plan.

### Argument descriptions as markers

The description of an argument could be a marker on its parameter, without a change of the model. A
description is documentation, which a generator into another language writes into the documentation
of its callable. A marker would leave it among the directives of the parameter.

### A selection set as an inline type

The frontend could load a selection set as an inline reference, as TypeScript writes an object
literal type. The canonical shape of an inline type has no field names, and every spoke refuses it,
so a generator into Go or TypeScript could not spell the result of an operation. A struct translates
through the hub as every other struct does.

### Selections as emit declarations

The backend could write a selection set from emit declarations: a struct for each selection, a
field for each response key, and markers for the aliases, the arguments and the fragments. An
argument is a value in GraphQL's syntax, so its marker would contain text anyway. The order of the
fields, the spreads and the inline fragments would need markers of its own. A verbatim body states
every construct of a selection set through one field of the model.

### A combined parser upstream

gqlparser could gain a parser of a mixed document. It models the two languages as two document
types, `ast.SchemaDocument` and `ast.QueryDocument`, so such a parser would return two documents as
the two copies do. The copies do not need a change upstream.

## Drawbacks

- The satellite depends on three changes of gqlparser that upstream has not reviewed yet. Until a
  release contains them, a document that uses one of the three forms does not parse.
- An anonymous operation takes the base name of its file. Two anonymous operations in files of one
  name collide in the package of the documents, and the load keeps the first.
- A fragment spread loads as the fields that it adds, so the struct of a selection repeats the fields
  of each fragment that it spreads. The stamp `graphql.spreads` names the fragments.
- A generator writes a selection set as text. The lowering parses it but does not check it against
  the schema, which the toolchain adapter does.
- The link from a field of a result to the schema field that it selects is the stamp
  `graphql.coordinate`. The stamp is text, and the resolution step does not resolve it.
- A project whose key `schema` is a URL types none of its documents, because the load reads the
  workspace alone. Such a project needs a copy of its schema in the workspace, such as the SDL that
  an introspection query returns.
- The partition reads up to 29 configuration names in each directory above a selected file, once
  per directory: the 26 names of graphql-config and the 3 names of gqlgen. Each read folds into the
  key of every unit of the partition. Every configuration that the frontend reads is a shared input
  of every unit, so an edit of one configuration changes the key of every unit.
- The frontend adds the TOML parser and the glob library doublestar as dependencies of the
  satellite, for the two TOML names and for the globs of graphql-config.
- The markers are annotations that no source states, so a generator over GraphQL sources converts
  seven stamps into them through `Stated`.
- A header states one schema statement. A plan whose schema needs a definition and an extension
  states them in two files.
- The validator accepts an extension without a base, a default of the wrong type, a non-null field
  of a `@oneOf` input object and `@deprecated` on a required argument, so the toolchain adapter does
  not catch them.
- A custom scalar outside the table loads without a target, so a generator into Go or TypeScript
  chooses its type.
- A plan that chooses a scalar of the table and does not declare it writes a schema that does not
  validate.
- The settle checks the names of each package of the sources for a collision, and a schema has one
  namespace. A plan whose sources span two packages can write two types of one name into the schema.
  The run does not report it, and the validator of the toolchain adapter does.

## Open questions

None.

## Unresolved and future work

None.

## References

- [GraphQL, September 2025 edition](https://spec.graphql.org/September2025/): the type system,
  its extensions, `@oneOf`, the escapes of a string and the default root names.
- [gqlparser](https://github.com/vektah/gqlparser): v2.5.63, `parser.ParseSchema`,
  `parser.ParseQuery`, `gqlparser.LoadSchema`, `validator.ValidateSchemaDocument`,
  `validator.Validate`, the rules of `validator/rules`, `Schema.Implements` and the lexer.
- [graphql-tools' validation of documents](https://github.com/ardatan/graphql-tools/blob/master/packages/utils/src/validate-documents.ts):
  the rules `NoUnusedFragmentsRule`, `NoUnusedVariablesRule` and `KnownDirectivesRule` that
  `createDefaultRules` leaves out.
- [graphql-js](https://github.com/graphql/graphql-js): v17.0.2, the printer of an operation and the
  refusal of a description on the query shorthand.
- [graphql-codegen's typescript-operations](https://the-guild.dev/graphql/codegen/plugins/typescript/typescript-operations):
  the suffixes `Query`, `Mutation`, `Subscription` and `Fragment` of its result types.
- [graphql-config](https://the-guild.dev/graphql/config/docs/user/usage): the names of its files in
  the order of its search, `schema` and `projects`.
- [gqlgen configuration](https://gqlgen.com/config/): `gqlgen.yml` and its globs of schema files.
- [doublestar](https://github.com/bmatcuk/doublestar) v4.10.3 and
  [go-toml](https://github.com/pelletier/go-toml) v2.4.3.
- [GraphQL Scalars](https://scalars.graphql.org/): the specifications of custom scalars.
- [Naming conventions](https://graphql.org/learn/naming-design/) of graphql.org.
- [RFC-0015: Tree-sitter frontends, dependency stores and re-exports](0015-tree-sitter-frontends-and-dependency-stores.md).
- [RFC-0065: The eidos binary](0065-the-eidos-binary.md), the stock plans.
- [RFC-0067: The protobuf backend](0067-the-protobuf-backend.md), the header of a file, the markers
  and `Field.Receives`.
- The decision log, rows D16 (parsers) and D112 (protobuf's residue as stamps):
  [21-decisions.md](../architecture/21-decisions.md).
- [graphql-go/graphql](https://github.com/graphql-go/graphql),
  [graph-gophers/graphql-go](https://github.com/graph-gophers/graphql-go),
  [wundergraph/graphql-go-tools](https://github.com/wundergraph/graphql-go-tools),
  [bkegley/tree-sitter-graphql](https://github.com/bkegley/tree-sitter-graphql) and
  [joowani/tree-sitter-graphql](https://github.com/joowani/tree-sitter-graphql).
