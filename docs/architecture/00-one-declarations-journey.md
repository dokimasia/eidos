# One declaration's journey

*The on-ramp. Every mechanism below has its own document with the
full contract and the full argument. This page just follows one
declaration through the whole system, so the other twenty-one
documents have something concrete to attach to.*

A team keeps this in `svc/store.go`:

```go
// Store is the persistence seam for sessions.
//
//+gen:stub tag=test
type Store interface {
    Get(ctx context.Context, key string) (Session, error)
    Put(ctx context.Context, s Session) error
}
```

Their binary composes a workspace with two plans. `go-stubs`
generates Go test doubles. `ts-client` generates TypeScript types
for the web app. One run produces both.

## Load

The Go frontend parses the file with tree-sitter, using no toolchain
and executing nothing ([11-languages.md](11-languages.md)). It
produces symbols: a `symbol.Interface` named `Store` holding two
`symbol.Method`s, each carrying its position, its docs, and an
identity like `golang:svc/store.Store#Get(ctx,string)`
([02-symbol-model.md](02-symbol-model.md)).

The `//+gen:stub` line is a carrier. The frontend strips the marker
and hands the workspace one canonical directive, `stub` with
`tag=test`, already checked against stubgen's registered schema
([05-directives.md](05-directives.md)). The frontend also stamps
what only it knows: `go.isContext` on the first parameter's type,
under the `go.*` namespace it owns
([04-metadata.md](04-metadata.md)). The whole unit gets a
fingerprint, so on the next run an unchanged file never parses again
([09-incrementality.md](09-incrementality.md)).

## Freeze, then Annotate

The store seals. From here the store refuses any write that adds or
removes a symbol
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

Annotators now read the graph through tracking Readers and write
only metadata. The shape catalog looks at `Get` through the neutral
`Callable` projection, sees one input parameter and a value plus a
`LastReturn` error model, and stamps `shape.*` facts saying it is a
reader ([12-shape-catalog.md](12-shape-catalog.md),
[03-projection.md](03-projection.md)). One plan targets TypeScript,
so the TypeScript naming annotator runs too and stamps `ts.name`
spellings onto every symbol
([10-cross-language.md](10-cross-language.md)). Every read was
recorded, and every stamp knows what it came from.

## The plans run

Both plans run in parallel over the one frozen graph, each seeing it
through its own scope
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

In `go-stubs`, stubgen reads `Store` through the projections, never
through anything Go-specific, and emits neutral values: an
`emit.Struct` for the stub, and `emit.Method`s whose bodies are
scaffolding statements that record the arguments and return the
sample. Each one links back to the symbol it came from
([07-rendering.md](07-rendering.md)).

An unrelated audit plugin, which has never heard of stubgen, appends
one delegate call into each method's `prologue` slot. Nothing
collides, because slots accumulate and capability topology decides
the order.

## Layout, render, write

Layout routes each emit value. stubgen declared a `test`-tagged
output with a `_stub_test.go` suffix, the plan's policy puts files
beside their source, and the author's `tag=test` picked the
companion, so the file is `svc/store_stub_test.go`
([18-routing-and-layout.md](18-routing-and-layout.md)).

The Go backend renders it. Kind templates spell the declarations,
slot contents render through the same machinery, the file's one
`ImportSet` collects imports as a side effect of spelling types, and
gofmt finishes the job ([07-rendering.md](07-rendering.md)). The
sink writes atomically, and only if the bytes changed
([17-output-and-determinism.md](17-output-and-determinism.md)).

Meanwhile `ts-client` lowered the same symbols through the canonical
type shapes. `string` stayed `string`, the error model became
whatever spelling the policy chose, and `store.ts` was written the
same way ([10-cross-language.md](10-cross-language.md)).

## Close

The workspace merges both plans' manifests, checks that no two plans
claimed one path, deletes files whose producers are gone, runs the
cross-plan checks ("every Go method has a TypeScript counterpart"),
and verifies every metadata completeness contract
([08-workspace-and-plans.md](08-workspace-and-plans.md),
[04-metadata.md](04-metadata.md)). For each file the manifest
records the plan, the content hash, and the source identities it
came from:

```json
{"path":"svc/store_stub_test.go","plan":"go-stubs",
 "hash":"sha256:9f2c…","plugins":["stubgen","acme-audit"],
 "sources":["golang:svc/store.Store#Get(ctx,string)"]}
```

Tomorrow someone edits `Put`'s signature. The fingerprint gate
notices one file. One unit reparses. `Get` keeps its identity, and
every edge through it survives. Only the artifacts that read `Put`
derive again. Both plans re-render exactly the files that changed,
and the result is byte-identical to what a cold run would have
produced. A conformance check proves that rather than promising it
([09-incrementality.md](09-incrementality.md),
[13-testing-and-conformance.md](13-testing-and-conformance.md)).

If any step surprised you, its document has the contract and the
argument. Start with the role paths in the [README](README.md), and
keep the [glossary](GLOSSARY.md) beside you.
