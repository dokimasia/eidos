# One declaration's journey

*The on-ramp. Every mechanism below has its own document with the
full contract and the full argument; this page just watches one
declaration travel the whole system so the other twenty-one docs
have a film to hang their stills on.*

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

Their binary composes a workspace with two plans: `go-stubs`
(generate Go test doubles) and `ts-client` (generate TypeScript
types for the web app). One run, both outputs.

## Load

The Go frontend parses the file — tree-sitter, no toolchain, no
execution ([11-languages.md](11-languages.md)) — and produces
symbols: a `symbol.Interface` named `Store` with two
`symbol.Method`s, each carrying position, docs, and a canonical
identity like `golang:svc/store.Store#Get(ctx,string)`
([02-symbol-model.md](02-symbol-model.md)). The `//+gen:stub` line
is a *carrier*; the frontend strips the marker and hands the
workspace one canonical directive, `stub` with `tag=test`, already
validated against stubgen's registered schema
([05-directives.md](05-directives.md)). The frontend also stamps
what only it knows: `go.isContext` on the first parameter's type,
under the `go.*` namespace it owns
([04-metadata.md](04-metadata.md)). The whole unit gets a
fingerprint; next run, an unchanged file never parses again
([09-incrementality.md](09-incrementality.md)).

## Freeze, then Annotate

The store seals — from here, adding or removing symbols is a refused
write, not a convention
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Annotators
now read the graph through tracking Readers and write only metadata:
the shape catalog looks at `Get` through the neutral `Callable`
projection — one input-role param, a value plus a `LastReturn` error
model — and stamps `shape.*` facts declaring it a reader
([12-shape-catalog.md](12-shape-catalog.md),
[03-projection.md](03-projection.md)). Because a plan targets
TypeScript, the TS naming annotator runs too and stamps `ts.name`
spellings onto every symbol ([10-cross-language.md](10-cross-language.md)).
Every read was recorded; every stamp knows what it was derived from.

## The plans fan out

Both plans run in parallel over the one frozen graph, each seeing it
through its own scope ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

In `go-stubs`, stubgen reads `Store` through the projections — never
through anything Go-specific — and emits neutral output values: an
`emit.Struct` for the stub, `emit.Method`s whose bodies are
scaffolding statements ("record the arguments, return the sample"),
each linked back to its origin symbol
([07-rendering.md](07-rendering.md)). An unrelated audit plugin,
which has never heard of stubgen, appends one delegate-call
statement into each method's `prologue` body slot. Nothing collides:
slots accumulate, and their order is decided by capability topology,
not by luck.

## Layout, Render, Sink

Layout routes each emit value: stubgen declared a `test`-tagged
output with a `_stub_test.go` suffix, the plan's policy is
alongside-source, and the author's `tag=test` picked the companion —
so the destination is `svc/store_stub_test.go`
([18-routing-and-layout.md](18-routing-and-layout.md)). The Go
backend renders it: kind templates spell the declarations, slot
contents render through the same machinery, the one `ImportSet`
per file collects imports as a side effect of type rendering, gofmt
finalises ([07-rendering.md](07-rendering.md)). The sink writes
atomically, and only if the bytes changed
([17-output-and-determinism.md](17-output-and-determinism.md)).

Meanwhile `ts-client` lowered the same symbols through the canonical
type shapes — `string` stayed `string`, the error model became the
TS spelling the policy chose — and wrote `store.ts` the same way
([10-cross-language.md](10-cross-language.md)).

## Close

The workspace merges both plans' manifests, checks that no two plans
claimed one path, sweeps files whose producers no longer exist, runs
cross-plan checks ("every Go method has a TS counterpart"), and
verifies every metadata completeness contract
([08-workspace-and-plans.md](08-workspace-and-plans.md),
[04-metadata.md](04-metadata.md)). The manifest records, for each
file, the plan, the content hash, and the source identities it
derives from:

```json
{"path":"svc/store_stub_test.go","plan":"go-stubs",
 "hash":"sha256:9f2c…","plugins":["stubgen","acme-audit"],
 "sources":["golang:svc/store.Store#Get(ctx,string)"]}
```

Tomorrow, someone edits `Put`'s signature. The fingerprint gate
notices one file; one unit reparses; `Get`'s identity and every edge
through it survive untouched; only the artifacts that read `Put`
re-derive; both plans re-render exactly the files that changed — and
the result is byte-identical to what a cold run would have produced,
which a conformance rung proves rather than promises
([09-incrementality.md](09-incrementality.md),
[13-testing-and-conformance.md](13-testing-and-conformance.md)).

If any step surprised you, its document has the contract and the
argument: start with the [README](README.md) role paths, and keep
the [GLOSSARY](GLOSSARY.md) beside you.
