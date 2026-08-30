---
rfc: 0002
title: The model generator (internal/gen/model)
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-08-30
updated: 2026-08-30
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0004, ADR-0005
---

# RFC-0002: The model generator (internal/gen/model)

## Summary

`internal/gen/model` turns the `symbol/schema` package of RFC-0001 into the
committed `node/` and `emit/` models plus `symbol/kind.gen.go`. It is
a standard-library Go program: `go/parser` and `go/types` read the
schema, a small intermediate representation carries it,
`text/template` renders it, `go/format` formats it. A mirror-guard
test regenerates in-process and diffs against the tree, so
`make check` fails on any drift between schema and models. Consumers
never run it; contributors run it through `make generate`.

## Motivation

Two models must never diverge from one schema or from each other;
that promise needs an enforcer.
The enforcer has constraints the architecture fixes: the module map
requires the tool to have no eidos dependencies and to be plain
`go/ast` plus `text/template`, because a kernel that needed a
language satellite to build itself could never bootstrap; and the
kernel takes no third-party dependencies, which extends to its
`go.mod`, because a `tool` directive is visible to every consumer's
supply-chain audit. Within those walls, this RFC decides what the
architecture left open: how the tool reads the schema without
importing it, what its intermediate form is, which files it writes,
and how the guard runs.

## Detailed design

### Pipeline

```mermaid
flowchart LR
    S[symbol/schema sources] -->|go/parser| A[ast.Files]
    A -->|go/types + source importer| T[type-checked package]
    T -->|lower + validate| I[IR: KindSpec list]
    I -->|text/template, embedded| G[generated sources]
    G -->|go/format.Source| F[formatted bytes]
    F -->|write, or diff in the guard| O[symbol/ node/ emit/]
```

The tool lives at `internal/gen/model`, named for what it writes. Two more generators take the same input or the same shape: the
authoring surface's kind-indexed triggers and Match types come from
this schema, and registered names (diagnostic codes, metadata keys
and groups, directive names and their param keys) generate constants.
Each is a sibling under `internal/gen`, and the machinery they share,
module-root discovery, the importer, formatting and the mirror
harness, moves up a level when the second one needs it.

The tool never imports `symbol`, `schema` or any eidos package. It
locates the module root by walking up from the working directory to
`go.mod`, then reads the schema directory as source. Everything it
knows about the schema it learns from parsing.

### Reading the schema: the source importer

`go/types` needs an `Importer` to resolve the schema's imports:
`symbol` for the auxiliary enums and `Identity`, `position` for
`Pos`. The standard importers resolve from build caches, which would
make generation depend on prior compilation. Instead the tool carries
a source importer of about 50 lines:

```go
// Importer resolves imports from source rather than from a build
// cache. Packages inside the module load from disk; everything
// else, the standard library included, goes to the compiler's
// source importer.
type Importer struct {
    fset    *token.FileSet
    modRoot string
    modPath string
    std     types.Importer
    cache   map[string]*types.Package
}

func (im *Importer) Import(path string) (*types.Package, error)
```

It lives in `internal/gosource` with the module discovery and the
directory loader, because the sibling generators need the same
machinery.

Verified against go1.27.0: `types.Importer` is the single-method
interface above, and `(*types.Config).Check(path, fset, files, info)`
type-checks a parsed package with it.

Two reading modes keep the bootstrap sound. The generator reads its
own input, the schema, hand-written: generated files are skipped, so
its output can never feed it and a stale file cannot change what it
produces. It reads a dependency complete, generated files included
and type errors tolerated, for two reasons. Hand-written code
legitimately refers to what its own generator produced, as
`symbol.Identity.String` refers to the generated Kind constants. And
a dependency that does not compile must not stop the generator that
would fix it: a broken generated file would otherwise brick
regeneration.

### The intermediate representation

```go
// Side says which model a field lands in.
type Side uint8 // Node | Emit | Both

// KindSpec is one declaration kind, lowered from its schema struct.
type KindSpec struct {
    Name   string   // "Struct"; schema declaration order is preserved
    Doc    []string // the schema's documentation for the kind
    Fields []FieldSpec
}

type FieldSpec struct {
    Name     string
    Doc      []string // the schema's documentation for the field
    Comment  string   // its trailing line comment
    Type     string // the spelling to emit: "[]*Field", "position.Pos"
    Elem     string // element kind name for walk and slot fields; "" otherwise
    Side     Side
    Walk     bool
    Slot     string // slot name; "" when the field is not a slot
    Slice    bool
    IsSymbol bool // schema.Symbol-typed: maps to symbol.Symbol per side
}
```

Lowering validates as it goes, and every violation is an error
carrying the schema file position:

- a tag token outside the closed vocabulary (`side`, `walk`,
  `slot=`);
- `slot=` on a field that is not a slice of kind structs;
- `walk` on a field whose type is not a kind struct, a slice of kind
  structs, or `schema.Symbol`;
- duplicate slot names within one kind;
- a schema declaration that is not an exported struct or the
  `Symbol` marker. Imports pass through, since a kind's fields are
  typed from other packages.

### Output files

| File | Content |
|---|---|
| `symbol/kind.gen.go` | the `Kind` constants, `String` and `ParseKind` (ADR-0004) |
| `node/kinds.gen.go` | node structs, interface satisfaction |
| `node/symbols.gen.go` | the declaration list and the codec |
| `node/symbols.gen_test.go` | the round-trip property, per kind |
| `node/kinds.gen_test.go` | the kinds' behaviour, per kind |
| `node/walk.gen.go` | `Walk`, `All` |
| `node/walk.gen_test.go` | traversal and pruning, per kind |
| `emit/kinds.gen.go` | emit structs, interface satisfaction |
| `emit/symbols.gen.go` | the declaration list and the codec |
| `emit/symbols.gen_test.go` | the round-trip property, per kind |
| `emit/kinds.gen_test.go` | the kinds' behaviour, per kind |
| `emit/slots.gen.go` | slot storage and typed accessors |
| `emit/walk.gen.go` | `Walk`, `All` over slot items |
| `emit/walk.gen_test.go` | traversal and pruning, per kind |

Files group by concern rather than by kind (ADR-0005): a dozen
files instead of dozens per kind, diffs review as one concern at a time, and the
`.gen.go` suffix rides the license-header exclusion already in
`.ergon.yaml`. Every file opens with
`// Code generated by internal/gen/model. DO NOT EDIT.`, the standard
marker tools recognize.

Templates are one `text/template` file per output file, embedded with
`embed.FS`, so the binary is self-contained and the mirror test needs
no working-directory guessing.

### Documentation

The schema documents every kind and most fields, and the generated
models carry that documentation rather than a generic stand-in: the
contract is written once, where it is decided. Lowering lifts the
doc comments and the trailing line comments out of the parsed
syntax, and the templates re-emit them above the generated type and
its fields. The generator adds only what it knows and the schema
cannot: which model side this spelling belongs to, and that a
slot-tagged field is reached through its accessor.

### The codec

The generated fields carry their own `json` tags, and the standard
encoder does the rest. There are no per-kind marshallers, because a
concretely typed field needs none: `[]*Field` decodes straight
through the tags.

One case genuinely cannot work that way. A field admitting any
declaration cannot decode from an interface slice, because nothing
tells the decoder what to allocate: `[]symbol.Symbol` fails with
"cannot unmarshal object into .0 of type symbol.Symbol". Those
fields are typed `Symbols`, a named slice that writes each
element's kind ahead of its fields and reads it back before
allocating. `ParseKind` turns the name into a kind, and each model
carries a table indexed by that kind holding how to make one, so
the name lookup lives once in the vocabulary rather than once per
model.

So the discriminator exists on exactly the values that need it, in
one place per model, rather than on every kind. A slot holding
declarations delegates to `Symbols`; a slot of one concrete kind
encodes and decodes from the tags alone.

Fields are tagged `omitzero` rather than `omitempty`, so a zero
identity, an unset position and an untouched slot all disappear
from the encoding instead of spelling themselves out.

### Generated tests

The generator writes a test beside each model file, and the models
carry compile-time assertions that every kind satisfies the
vocabulary's interfaces. A generated test constructs each kind
through the public surface and checks what the schema says is true
of it: the kind constant it answers, its position and documentation,
which member lists it carries, and that a traversal reaches one
child in every traversed field and stops when a visitor prunes.

The limit is worth stating, because it decides what these tests are
for. A test generated from the same intermediate representation as
the code cannot catch a lowering fault: if lowering marks a field
traversed when the schema did not, the code and the test agree and
both are wrong. What it does catch is a rendering fault, because it
compiles and runs the emitted code. Lowering is checked separately,
against hand-written schema fixtures.

### Determinism

The same schema bytes produce the same output bytes anywhere:

- kinds render in schema declaration order, schema files read in name
  order;
- fields render in struct declaration order;
- `go/format.Source` canonicalizes layout;
- the tool reads nothing but the schema directory and writes no
  clock, path or environment into its output.

### The guard and the wiring

```go
// internal/gen/model exposes one function; main is a thin wrapper.
// Generate renders every output file into memory.
func Generate(modRoot string) (map[string][]byte, error)

// internal/gen/model/mirror_test.go — the mirror guard.
// For every generated path: bytes on disk equal Generate's bytes.
// For every *.gen.go on disk under symbol/, node/ and emit/: it is
// one of Generate's paths, so a stray or hand-added generated file
// fails too. Plain `go test ./...` runs it, which is what puts the
// guard inside `make check`.
```

Regeneration goes through the tooling that exists:
`symbol/schema/doc.go` carries

```go
//go:generate go run go.dokimi.dev/eidos/core/internal/gen/model
```

and `make generate` (`ergon generate`, which runs `go generate` per
module) picks it up. The directive is ergonomics only; the test is
the guard.

### Failure behaviour

The tool writes nothing unless every file rendered and formatted. A
schema error reports position, message and nothing else, and exits 1.
A formatting error on generated output is a bug in a template, and
the message names the output file and includes the unformatted source
so the template is debuggable.

## Alternatives considered

### A. stringer for the Kind constants

`golang.org/x/tools/cmd/stringer` generates `String()` for
hand-maintained const blocks. Our const block is itself generated, so
its `String()` is one stanza in a template we already run. Wiring
stringer in costs either a `tool` directive, which puts `x/tools`
into the kernel's `go.mod` that consumers audit, or an unpinned
`go run ...@version` fetch at generate time. Recorded as ADR-0004.

### B. golang.org/x/tools/go/packages for loading the schema

The standard way to load and type-check packages, and it would
replace the source importer. It is a third-party dependency in a
module whose defining property is having none, and it shells out to
the go command, which makes generation depend on the machine's build
cache state. The 50-line importer costs less than the exception.

### C. Runtime reflection instead of generation

Walk, JSON and rewire could reflect over struct tags at runtime: no
generator, no mirror guard. It moves the cost to every traversal of
every run. The graph is the hot path at 10,000-package scale, and the
engine's performance budget removes constant factors rather than
adding them. Reflection also checks nothing at compile time: a
typo in a tag becomes a silent traversal gap instead of a generation
error.

### D. go/ast only, without go/types

Parsing alone yields field names and type spellings, which is most of
what the templates need. It cannot resolve whether `[]*Field` names a
schema kind or a stray type, so the validations above become string
matching, and a typo like `[]*Feild` generates plausible garbage
instead of failing with a position. Type-checking one small package
costs milliseconds and the importer stanza.

### E. Hand-written models

The symbol model specification already refuses this; the argument
lives there.

## Drawbacks

- Three representations of one model exist: the schema, the
  intermediate representation and the templates. A change to the
  model touches the schema always, and the tool only when the
  annotation vocabulary grows. Someone still maintains about ten
  templates and about 50 lines of importer.
- Template errors report at execute time. The mirror test catches
  every such error in CI, but the debugging loop is
  edit-template-run-test, not a compile error.
- In-process generation means the guard shares the process with the
  test runner: a generator panic fails the test with a stack rather
  than a diff. Acceptable; panics in a tool this size are bugs to
  fix, not conditions to report.
- The source importer re-type-checks `symbol` and `position` on every
  run. Both are small leaf packages; if generation ever gets slow,
  the cache field already exists.

## Open questions

- Should `Generate` also emit a machine-readable schema summary
  (JSON)? Release tooling that generates documentation is a plausible
  consumer. Or is that premature until something asks for it?

## Unresolved and future work

- The metadata and directive seams and the emit body model are
  follow-up schema edits; the annotation vocabulary grows by kernel
  decision, one token at a time.
- The shape catalog's spec-driven generation reuses the mirror-guard
  discipline but not this tool; it runs a real eidos workspace, where
  the dependency direction allows it.

## References

- [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md),
  the module map: zero dependencies, the bootstrap constraint
- [02-symbol-model.md](../architecture/02-symbol-model.md), the
  symbol model specification
- [12-shape-catalog.md](../architecture/12-shape-catalog.md), the
  shape catalog's own spec-driven generation
- [21-decisions.md](../architecture/21-decisions.md), the decision
  log (D4, D56)
- [RFC-0001](0001-symbol-model-contract.md), the schema and contract
  this tool generates from
- [ADR-0004](../adr/0004-generate-kind-enum-from-schema.md), generate
  the Kind enum from the schema
- [ADR-0005](../adr/0005-concern-grouped-generated-files.md), group
  generated files by concern
- [.ergon.yaml](../../.ergon.yaml), the `*.gen.go` license-header
  exclusion
