---
rfc: 0031
title: The shared helpers of the satellites
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: 0024
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0031: The shared helpers of the satellites

## Summary

The satellites of Go, TypeScript, Java, Rust and protobuf write the same
mechanics again in each language:
the attribution of comments over a tree-sitter tree, a capped syntax report,
the probe for the nearest manifest, the process and file plumbing of a
toolchain harness, the resolution walk of a directive, the skeleton of an enum
projection, and the composite syntax of Go. A review of the satellites on
2026-10-11 counted 2,519 duplicated production lines without a design reason,
7.0% of 35,746. Four of the copies had already diverged into defects. We
propose one home for each mechanic in the kernel's kits or in eidos-lang, with
each language's facts passed in as parameters. The proposal adds exported
declarations to finished packages and removes one SPI method.

## Motivation

### The duplicated lines per cluster

| Cluster | Copies | Duplicated lines |
|---|---|---:|
| Comment attribution, comment runs, syntax reports and node helpers | five frontends, most of it in the three tree-sitter ones | 739 |
| The resolution walk, the enum skeleton, and the alias and fact helpers | Go, TypeScript and protobuf rules | 655 |
| Process, file, layout, walk and probe mechanics of the toolchain adapters | Go, TypeScript, Java, Rust | 579 |
| Frontend steps: refusals, composite references, manifests, module facts, test files, tiers, stores | four or five frontends | 443 |
| Backend steps: scaffold glue, sum and throws lowering, coverage, helper names | four backends | 337 |
| Numeric literals | TypeScript, Java, Rust | 211 |
| The checks and messages of a spoke | four spokes | 115 |

A change to one of these mechanics is a change in each copy.

### Copies that diverged

Each of these divergences was a defect with a local fix, and the satellites
contain the fix now:

- Go and protobuf returned the first failed key registration, and the other
  three joined every failure.
- TypeScript, Java and Rust reported the syntax errors past the cap without
  their count.
- TypeScript probed the shared ancestors of every directory for its manifest
  again.
- The Java class-file reader and the Java backend escaped one string two ways.

The divergences that remain need one shared rule:

- A carrier on a subject that the model cannot address reports under
  UnaddressedCarrier with the subject in four frontends. Protobuf parses the
  carrier first and reports it under BadCarrier without the subject.
- TypeScript and protobuf stamp neither `gen.module` nor `gen.moduleRoot`. Go,
  Java and Rust stamp both, and a consumer reads the module of a package from
  them in every language.
- Only the Go frontend classifies the output of a foreign generator.
- A composite child that the source does not state is an empty reference in
  TypeScript and a nil child in Go and Rust.

### Why the kits, and why one RFC

The architecture assigns the steps that every frontend would otherwise write
again to the frontend kit, the walks of the projection to `core/rules`, and the
harness skeleton to `core/toolchain`. A third-party frontend, backend or rules
package uses the kits through the SDK, so a helper there serves every author.
A helper that only the satellites use, such as the comment walk over a
tree-sitter tree, belongs in eidos-lang beside the grammars.

Each helper changes a finished package. One document lists every function that
the consolidation needs from those packages, so the change is one slice and no
package changes twice.

## Detailed design

### Invariants

1. A language fact belongs to its satellite. A shared helper takes the fact as
   a parameter and never branches on a language.
2. A satellite keeps its exported API, except the `TypeName` method that the
   SPI drops.
3. Every helper states its allocation contract. A helper on a hot path, such
   as the comment walk or the resolution walk, allocates no more than the
   copies that it replaces.
4. Behaviour that a user sees changes only where a divergence above names it.

### Components

| Package | Additions | Rows |
|---|---|---|
| `core/plugin` | `SourceUnit.RefuseCarriers`, `SourceUnit.RefuseSugar`, `SourceUnit.ReportSyntax`, `GraphBuilder.StampModule`, `Zip`, `Absent`, `UserHomeDir`, `Composite`, `FuncType` | W6.F2, F3, F7, F9, F11, F13, F14, D1, D5, D14 |
| `core/frontend` | `TestFiles`, `Generated`, `Nearest`, `Builder.Imports` | W6.F1, F10, D6, D16, O1 |
| `eidos-lang/treesitter` | `Comments`, `Node.FirstOf`, `Node.HasToken`, `Node.Content`, `Node.LastNamed`, `Tree.Problems` | W6.F5, F6, F7, F8, D3 |
| `eidos-lang/numeric` | `Syntax`, `Parse` | W6.F17 |
| `core/backend/render` | the names of the shared vocabulary helpers, `Coverage.RefusesEmbeds` | W6.B2, B5, B12 |
| `core/backend` | `Builder.Statements` | W6.B8 |
| `eidos-lang/scaffold` | `Base` | W6.B7 |
| `eidos-lang/lowering` | `VariantName`, `NamedPayload`, `ParamArgs`, `Callables` | W6.B10, B11 |
| `eidos-lang/spoke` | `Arity`, `Split` | W6.S1 |
| `eidos-lang/spellref` | `Restate`, `SpellRestated` | W6.X1 |
| `eidos-lang-go` | `golang.Restate` | W6.X2 |
| `eidos-lang-typescript`, `eidos-lang-java`, `eidos-lang-rust` | `GeneratedKey` | W6.D6 |
| `core/rules` | `Resolver`, `Resolve`, `ResolveError`, `EnumBuilder`, `Unaliased`, `FactNamed`. `SourceRules.TypeName` and `Bound.TypeName` go | W6.R1, R3, R5, R6, X6 |
| `core/toolchain` | `Run`, `LookPaths`, `Scratch`, `WriteFile`, `Sources`, `Probe` | W6.T1 to T6 |
| `core` | `Emitter.Fields` | W6.C5 |
| `eidos-sdk` | the regenerated facade | all |

### The source unit

A refusal of a carrier or a marker on a subject that the model cannot address
is one step of every frontend:

```go
// RefuseCarriers reports each carrier under code at its own position,
// quoting it as the author wrote it, so a directive that attaches nowhere is
// never dropped in silence. what describes the subject, such as "a
// parameter".
func (u *SourceUnit) RefuseCarriers(code diag.Code, cs []Carrier, what string)

// RefuseSugar reports each marker under code at its own position, with its
// path joined by sep, such as "." for a TypeScript decorator.
func (u *SourceUnit) RefuseSugar(code diag.Code, ss []Sugar, sep, what string)
```

Protobuf reports an unaddressed carrier through `RefuseCarriers` under its
UnaddressedCarrier code, number 4, and names the subject.

A syntax report caps the findings of one file and counts the rest:

```go
// MaxSyntaxFindings is the number of syntax errors of one file that a
// frontend reports one by one.
const MaxSyntaxFindings = 10

// ReportSyntax reports each error of errs, a position and the sentence that
// describes the error, under code. It reports at most MaxSyntaxFindings of
// them, and then one finding that counts the rest at the first error past
// the cap. It allocates the findings and nothing else.
func (u *SourceUnit) ReportSyntax(code diag.Code, errs iter.Seq2[position.Pos, string])
```

Go adapts its `scanner.ErrorList` and protobuf its report with an iterator of
three lines.

A frontend states the module of its packages under the two neutral keys:

```go
// StampModule stamps meta.ModuleKey and meta.ModuleRootKey on a package
// once, at the position of the package's first file. An empty module stamps
// nothing.
func (b *GraphBuilder) StampModule(p *node.Package, module, root string)
```

TypeScript stamps the `name` of the nearest `package.json` and its directory.
Protobuf stamps nothing, because protoc reads import paths and no module
manifest. A `buf.yaml` names a module for buf, and the protobuf frontend follows
protoc, so it does not read one.

Each of these helpers states a kernel rule that two frontends restate:

```go
// Zip merges the tiers of its sources by position: the first tier of each,
// then the second of each. Sources compete inside a tier, and each source
// keeps its own order across tiers.
func Zip(sources ...Candidates) Candidates

// Absent reports whether a read found nothing to read: the file does not
// exist, or the load does not provide a store for it.
func Absent(err error) bool

// UserHomeDir returns the home directory by the rule of os.UserHomeDir,
// read through getenv: USERPROFILE on Windows, home on Plan 9 and HOME
// elsewhere. It returns the empty string where the variable is unset.
func UserHomeDir(getenv func(key string) string) string
```

A composite reference is assembled the same way in every frontend. A child
that the source does not state keeps the arity of the form as an empty
reference, so a consumer never reads a nil child:

```go
// Composite returns the reference of a structural form with its children in
// the order that the form fixes. A nil child becomes an empty reference at
// pos, so the form keeps its arity.
func Composite(spelling string, pos position.Pos, form symbol.TypeForm, elems ...*node.TypeRef) *node.TypeRef

// FuncType returns the reference of a function type: the parameters, then
// the results, split where the results begin.
func FuncType(spelling string, pos position.Pos, params, results []*node.TypeRef) *node.TypeRef
```

The docblock of `node.TypeRef` gains one rule. An element of `Elems` is never
nil.

### The frontend kit

```go
// TestFiles returns a classifier that stamps key true on every file of the
// unit that test reports as a test file.
func TestFiles(key meta.KeyName, test func(path string) bool) Classifier

// Generated returns a classifier that stamps key on every file that the
// language's convention marks as the output of a foreign generator, with
// the marker as written.
func Generated(key meta.KeyName, marker func(u *plugin.SourceUnit, f *node.File) (string, bool)) Classifier

// Nearest returns the file named name in dir or in the nearest directory
// above it, and the empty string where no directory up to the root has one.
// It caches the result for every directory that it visited in seen.
func Nearest(r plugin.FileReader, dir, name string, seen map[string]string) string

// Imports declares the importer role: how the imports of a file name the
// file at a workspace path. The built frontend implements plugin.Importer
// only when a declaration states the role.
func (b *Builder) Imports(importOf func(scope plugin.ImportScope, file string) string) *Builder
```

Rust stamps its test marker per item, so it keeps its own classifier. The
conventions that `Generated` reads are:

| Language | Marker |
|---|---|
| Go | a line `// Code generated ... DO NOT EDIT.` before the package clause |
| TypeScript | `@generated` in the first comment of the file |
| Java | the annotation `javax.annotation.processing.Generated` on a top-level type |
| Rust | `@generated` in the first comment of the file |
| protobuf | none |

The kit takes the classifier, because the stamp, its key and its position are
the same in every language. Only the marker differs, and each frontend passes
its own. Each satellite registers the key that its frontend stamps:
`golang.GeneratedKey` exists, and `typescript.GeneratedKey`,
`java.GeneratedKey` and `rust.GeneratedKey` are new, each a text key on a file.

Protobuf builds its frontend through the kit with `Imports`. Its hand-written
`plugin.Frontend` goes.

### Tree-sitter

The tree-sitter frontends of TypeScript, Java and Rust attribute comments with
one algorithm. They differ in the parameters that `Comments` takes:

```go
// Comments attributes the comments of one tree to the declarations that a
// frontend lowers: the run above a declaration, the comment after it on its
// last line, and a sweep over every comment that no declaration took. A
// Comments serves one tree on one goroutine.
type Comments struct {
    // IsComment reports whether a node is a comment.
    IsComment func(n Node) bool
    // IsBlock reports whether a comment is a block comment, which is a text
    // of its own and never part of a run.
    IsBlock func(n Node) bool
    // Passes reports whether a sibling between a declaration and its
    // comments is passed over, such as a decorator. Nil passes nothing.
    Passes func(n Node) bool
    // Stops reports whether a comment ends the run above a declaration,
    // such as a Rust inner doc comment. Nil stops at no comment.
    Stops func(n Node) bool
    // Trails reports whether a comment can be a trailing comment. A Rust
    // doc comment documents the item after it, so it never trails. Nil
    // admits every comment.
    Trails func(n Node) bool
    // LastLine returns the line on which a node's source ends. Nil returns
    // the line of Node.End.
    LastLine func(n Node) int
    // Text returns the text of a line comment in a run, such as the text
    // without the newline that a Rust doc comment takes. Nil returns
    // Node.Text.
    Text func(n Node) string
    // Inner returns the child of a declaration whose comments join the run,
    // such as the modifiers of a Java declaration. Nil joins none.
    Inner func(n Node) Node
    // Folded reports that the grammar folds a trailing comment into the
    // last child of a node that ends without a terminator, as TypeScript's
    // grammar does.
    Folded bool
    // Opaque reports whether the sweep passes over a node, such as a
    // function body. Nil passes over nothing.
    Opaque func(n Node) bool
}

// Leading returns the comments directly above n in source order.
func (c *Comments) Leading(n Node) []Node

// Trailing returns the comment after n on its last line, and the zero Node
// where there is none.
func (c *Comments) Trailing(n Node) Node

// Runs calls yield once for each text that comments make: a run of line
// comments on consecutive lines joined by newlines, or one block comment.
// It marks every comment taken.
func (c *Comments) Runs(comments []Node, yield func(text string, at position.Pos))

// Take marks comments taken without reading them, such as the comments of a
// declaration that the load leaves out.
func (c *Comments) Take(comments ...Node)

// Sweep calls untaken for every comment under root that no declaration
// took, and passes over a node that Opaque reports.
func (c *Comments) Sweep(root Node, untaken func(n Node))
```

A frontend passes each text of `Runs` to `SourceUnit.Comment`. The unit's
pipeline takes the text apart as it does today. Go keeps the comment groups of
go/ast. Protobuf keeps the attribution of protoc.

The node helpers that the tree-sitter frontends write as functions become
methods:

```go
// FirstOf returns the first named child of one of the kinds, and the zero
// Node where there is none.
func (n Node) FirstOf(kinds ...Kind) Node

// HasToken reports whether n has an unnamed child spelled text, such as a
// keyword of the grammar.
func (n Node) HasToken(text string) bool

// Content yields the children of a named kind that are not extras, such as the
// members of a body without its comments.
func (n Node) Content() iter.Seq[Node]

// LastNamed returns the last named child of n, and the zero Node for a node
// without one.
func (n Node) LastNamed() Node

// Problems yields the position and the sentence of each ERROR and MISSING
// node in document order: "X is missing here" for a MISSING node, and the
// first line of an ERROR node's source, cut at 32 bytes, for the rest.
func (t *Tree) Problems() iter.Seq2[position.Pos, string]
```

### Numeric literals

TypeScript, Java and Rust decode a numeric literal with one loop and differ in
its spellings:

```go
// Syntax is the spelling of one language's numeric literals.
type Syntax struct {
    Separator   byte              // the digit separator, such as '_'
    Prefixes    map[string]int    // the radix of each prefix, such as "0x": 16
    Suffixes    []string          // the type suffixes that the value ignores, longest first
    LegacyOctal bool              // a leading 0 opens an octal literal, as in Java
}

// Parse returns the value of a numeric literal of the syntax, and reports
// false for text that is not one. It allocates nothing for a literal
// without separators.
func Parse(text string, s Syntax) (float64, bool)
```

### Backends

The names of the vocabulary helpers that every backend declares become kernel
constants, because the templates of a plugin call them in every target:

```go
// The names of the vocabulary helpers that every backend declares under
// the same name, so one template text serves each target.
const (
    HelperDocs       = "docs"
    HelperSpell      = "spell"
    HelperTypeParams = "typeparams"
    HelperParams     = "params"
    HelperResults    = "results"
)
```

A language without promotion refuses an embed and every fact on it. The
coverage table states that once:

```go
// RefusesEmbeds marks a language that has no promotion. The coverage
// refuses an embed whole, with the facts on it, and the vocabulary needs no
// refusal of its own.
RefusesEmbeds bool
```

The scaffold glue of the four backends goes into the kit and into
`eidos-lang/scaffold`:

```go
// Statements declares the scaffold of the backend's statements: the
// grammar of the target, and the target that spells the values of one file
// with the file's import set.
func (b *Builder) Statements(g scaffold.Grammar, target func(set *render.ImportSet) scaffold.Target) *Builder

// Base supplies Lang and Literal of a scaffold.Target from the target's
// leaves, so a target states Type, Callee, Conversion, Composite and
// Address alone.
type Base struct{ Leaves Leaves }
```

The lowering of a sum and of a declared failure shares these steps:

```go
// VariantName returns the name of the declaration of a sum's variant: the
// sum's name followed by the variant's name in Pascal case.
func VariantName(sum, variant string) string

// NamedPayload returns an error under lang's prefix where a field of a
// variant's payload has no name.
func NamedPayload(lang symbol.Lang, v *emit.SumVariant) error

// ParamArgs returns one reference for each type parameter, which a
// variant states as the arguments of its sum.
func ParamArgs(params []*emit.TypeParam) []*emit.TypeRef

// Callables calls fold on the returns and the failures of the function or
// the method that s is, and of each method of the struct or the interface
// that s is, and then clears the failures. fold rewrites the returns in
// place, such as with Append for an error return or Replace for a result
// type. It returns the error of UniqueMethods for a host that declares one
// method name twice, and false for every other kind.
func Callables(lang symbol.Lang, s symbol.Symbol,
    fold func(returns *emit.Slot[*emit.Return], throws []*emit.TypeRef)) (bool, error)
```

### Spokes

```go
// Arity returns an error that contains the form of s where s has not n
// children, such as "a list takes one child, and the shape has 2".
func Arity(s rules.TypeShape, n int) error

// Split returns an error where the split of a function's TypeShape falls outside
// its children.
func Split(s rules.TypeShape) error
```

### The composite syntax of Go

The Go spoke and the Go backend each write the syntax of a pointer, a slice,
an array, a map, a channel and a function type. The Go root writes it once, and
`spellref` takes a restatement from the language:

```go
// Restate writes a structural reference of Go from the spellings of its
// children: a pointer, a slice or a variadic parameter, an array of a
// stated length, a map, a channel in its written direction, and a function
// type with its results. It returns an error for a form whose Go spelling
// the structure does not determine, and for a missing child.
func Restate(t *emit.TypeRef, elems []string) (string, error)
```

```go
// Restate writes a structural reference from the spellings of its
// children, for a language whose composite spelling follows from its
// structure.
type Restate func(t *emit.TypeRef, elems []string) (string, error)

// SpellRestated writes a reference as SpellWith does, and restates a
// structural reference through restate where a child spells otherwise than
// written.
func SpellRestated(t *emit.TypeRef, opener, closer, standIn string, qualify Qualify, restate Restate) (string, error)
```

The Go backend spells through `SpellRestated` with `golang.Restate`, and its
own reference walk and package lookup go.

### Rules

The directive resolution walk is the same in Go, TypeScript and protobuf. A
language states how a name in scope probes, which member a subject may use,
which placeholder a type outside the view resolves to, and what a callable's
value is:

```go
// Resolver is what a language states about the resolution of a directive's
// spelling. Resolve walks the rest.
type Resolver interface {
    // InScope returns the first declaration of one of the kinds that name
    // refers to in the subject's scope, in the language's probe order.
    InScope(scope Scope, name string, kinds []symbol.Kind, v View) (symbol.Symbol, error)
    // Usable reports whether code of the subject can use a member that
    // its type declares or inherits. own is the subject's own type.
    Usable(m Member, own symbol.Identity, scope Scope) bool
    // Placeholder returns the declaration that a type name resolves to
    // where the view declares none, such as a predeclared type, and false
    // where the name resolves to nothing.
    Placeholder(scope Scope, name string, v View) (symbol.Symbol, bool)
    // Value returns the type that a reference of a callable's value
    // names, such as the target of a pointer.
    Value(ref *node.TypeRef, v View) *node.TypeRef
}

// Resolve resolves a spelling from a subject: a callable through the
// subject's host before the scope, a package variable and a type in scope,
// a value field and a member on a handle among the members of the subject's
// value, and a host parameter on the subject's own signature. A refusal is
// a *ResolveError.
func Resolve(r Resolver, b Bound, scope Scope, name string, kind directive.ResolutionKind) (symbol.Symbol, error)

// ResolveError is the refusal of a resolution. Its text starts with the
// language's identity, as every refusal of a language's rules does.
type ResolveError struct {
    Lang symbol.Lang
    Msg  string
}
```

The skeleton of `EnumOf` is the same in the three rules packages, and the
values come from different sources:

```go
// EnumBuilder assembles the EnumInfo of one enum from its variants in
// declaration order.
type EnumBuilder struct{ /* unexported fields */ }

// NewEnum starts the EnumInfo of e in a form.
func NewEnum(e *node.Enum, form EnumForm) *EnumBuilder

// Variant adds one variant: its text, and its value as canonical decimal
// text where known is true. The builder tracks the zero variant, the first
// duplicate value, the largest integer value and the foreign packages.
func (b *EnumBuilder) Variant(v *node.EnumVariant, text emit.Value, value string, known bool)

// Info returns the EnumInfo, with the out-of-range value one past the
// largest integer value as a conversion to ref.
func (b *EnumBuilder) Info(ref *node.TypeRef) EnumInfo
```

These helpers state what the satellites restate:

```go
// Unaliased follows a chain of transparent aliases that the view declares
// and returns the reference at its end. A defined type is a type of its own,
// so the walk stops at one.
func Unaliased(ref *node.TypeRef, v View) *node.TypeRef

// FactNamed returns the fact of id under the key registered as name, and
// false where the view has no facts or no such key.
func FactNamed[T any](v View, id symbol.Identity, name meta.KeyName) (T, bool)
```

`SourceRules.TypeName` and `Bound.TypeName` go. No production code calls
either, and the two join a generator's word in opposite orders in Go and in
TypeScript. A generator joins a derived name with `Emitter.JoinName`.

### Toolchain adapters

The toolchain adapters keep their binaries, their project files, their probe texts
and the parse of their tools' output. The kernel takes the mechanics:

```go
// RunTimeout bounds one toolchain invocation.
const RunTimeout = 5 * time.Minute

// Run runs binary in dir under ctx and env, bounded by RunTimeout, and
// returns its combined output. A failure wraps the output, and a run that
// ctx or the bound ends wraps the context's error.
func Run(ctx context.Context, dir string, env []string, binary string, args ...string) (string, error)

// LookPaths reports whether every binary is on PATH, and names the first
// that is not, in the form that Adapter.Available returns.
func LookPaths(binaries ...string) (bool, string)

// Scratch writes the files of g into a new temporary directory, and each
// file of project where g states none at that path. A write that fails
// removes the directory.
func Scratch(pattern string, g Generated, project map[string][]byte) (string, error)

// WriteFile writes one file under dir with mode 0o600, and returns an error
// for a path that leaves dir.
func WriteFile(dir, path string, body []byte) error

// Sources returns the files under dir with one of the extensions, in
// lexical order, and passes over the directories that skip names.
func Sources(dir string, exts []string, skip ...string) ([]string, error)

// Probe writes source to the file name under dir, calls check, and removes
// the file.
func Probe(dir, name, source string, check func() error) error
```

### Authoring

A generator that translates the fields of a declaration writes one loop:
translate each field's type, and withhold the declaration where a type has no
spelling. The Emitter writes that loop once:

```go
// Fields translates the type of each field that keep admits, with the field
// as the owner, and returns the emit fields in order. It reports false where
// a type has no spelling in the target, after the Emitter reports the
// refusal, so the caller withholds its declaration.
func (e *Emitter) Fields(fields []*node.Field, keep func(*node.Field) bool) ([]*emit.Field, bool)
```

### Migration

| Consumer | Change | Packages |
|---|---|---:|
| The frontends | the source unit's refusals, `ReportSyntax`, `StampModule`, `Zip`, `Absent`, `UserHomeDir`, `Composite`, the kit's classifiers and `Nearest` | 5 |
| The tree-sitter frontends | `Comments` and the node methods in place of the copies | 3 |
| The backends and the spokes | the helper constants, `RefusesEmbeds`, `Statements`, `Base`, the lowering helpers, `Arity` and `Split` | 8 |
| The Go backend | `SpellRestated` with `golang.Restate` in place of its reference walk | 1 |
| The rules packages | `Resolve` with a `Resolver`, `EnumBuilder`, `Unaliased` and `FactNamed`. `TypeName` goes | 3 |
| The toolchain adapters | the kernel's mechanics | 4 |
| Generators that translate fields | `Emitter.Fields` in the conformance fixtures and the reference ensemble | 3 and more |

### Failure handling

| Failure | Where it shows |
|---|---|
| A rules package that still declares `TypeName` | nothing, because Go accepts the extra method. The method is dead code |
| A caller of the removed `Bound.TypeName` | the compiler |
| A frontend that leaves a nil child in a composite it builds by hand | nothing at the frontend. A consumer that reads the child panics, as it does today |
| A tool that does not return | `Run` returns an error that wraps the bound's context error |

## Alternatives considered

### Keep the copies and check their agreement

Each mechanic keeps one copy per language, and a conformance check per
mechanic proves that the copies agree, such as a check that every frontend
counts the syntax errors past the cap.

**Why not:** the check finds a divergence after a release contains it, and a change to the
mechanic is still one change per copy. Four divergences entered the tree that
way.

### One helper package in eidos-lang for every mechanic

Every helper goes into eidos-lang, and the kernel's kits do not change.

**Why not:** a third-party frontend or backend builds on the kits through the
SDK and would need eidos-lang for steps that the architecture gives the kits.
eidos-lang takes the helpers that only the satellites need: the tree-sitter
walk, the numeric syntax, scaffolding, lowering and the spokes.

### Generate the glue of each language

A generator writes the per-language copies from one template.

**Why not:** the copies differ by a parameter, such as a skip set or a list of
binaries, and not by text. A function with the parameter is shorter than a
template with its generator and its mirror guard.

## Drawbacks

- The proposal adds 64 exported declarations to 17 packages, seven of them in
  the kernel.
- `Comments` takes ten parameters. Each copy that it replaces reads in one
  pass, while a reader of the shared walk needs the meaning of each
  parameter.
- Removing `SourceRules.TypeName` breaks every rules package outside the
  repository that calls it, and one that declares it keeps a dead method.
- Four behaviours change: the protobuf finding for an unaddressed carrier, the
  module facts of TypeScript, the generated marker in TypeScript, Java and
  Rust, and the empty reference of an absent composite child.

## Unresolved and future work

- The rows that rename exported names or change a representation are not
  proposed here: the two key spellings of Go, the two `Scope` types, the three
  meanings of a bucket, the three meanings of a record, the layout fields of a
  plan's config, the two readings of `workspace` in the command line, the
  nullish members of a TypeScript reference, the three forms of `readonly`, the
  placement of constant evaluation, and the representations of the shape
  catalog.

## References

| What | Where |
|---|---|
| The anatomy of a satellite and the steps that the kits take | ../architecture/11-languages.md |
| D16, one binding layer of tree-sitter in eidos-lang | ../architecture/21-decisions.md |
| D22, diagnostic codes as API | ../architecture/21-decisions.md |
| D73, the neutral module facts | ../architecture/21-decisions.md |
| D83, the walks of the projection in `core/rules` | ../architecture/21-decisions.md |
| D84, the order of a composite's children | ../architecture/21-decisions.md |
| D91, the shared walk of a value tree | ../architecture/21-decisions.md |
| D92, the harness skeleton in `core/toolchain` | ../architecture/21-decisions.md |
