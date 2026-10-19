---
rfc: 0029
title: The memory layout of the model
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-11
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0029: The memory layout of the model

## Summary

The model keeps every identity as a value of 88 bytes. Each type reference has
two identity fields named `ID` and `Target`. The engine does not write the `ID`
of a type reference. A cold run over 200,000 declarations keeps 900,000 type
references until it ends. Their zero identities take 75.5 MiB.

We propose to remove the `ID` field from `TypeRef`. A type reference then
takes 288 bytes instead of 384. Over three interleaved rounds of that cold run,
the peak of the process fell from 3,235 MiB to 3,074 MiB on average.

We also measured shared representations of an identity. One distinct identity
costs 228 to 348 bytes in an intern table. As a value it costs 88 bytes. A
shared `Target` saves memory in real code, because a resolved target occurs 9
to 11 times. It also changes a public field at 184 sites. This proposal keeps
every other identity field as a value.

## Motivation

### The model's nodes are half of the live heap

We profiled a cold run of the reference binary over a corpus of 1,000 packages
of 10 files of 20 declarations. The run used one workstation with Linux 7.2.7
and Go 1.27.2. At the peak sample, the live heap is 1,260 MiB. The heap profile
attributes 658 MiB of it to the lowering of the Go frontend. Every node of the
lowering is still live at the peak. The store, the rules and the backends read
the graph until the run commits.

A counting run gives the exact number of nodes. Their sizes come from
`unsafe.Sizeof` on amd64. The runtime rounds each allocation up to a size
class.

| Kind | Nodes | Bytes | Size class | Identity fields | MiB at the class |
|---|---|---|---|---|---|
| `TypeRef` | 900,000 | 360 | 384 | `ID`, `Target` | 330 |
| `Field` | 300,000 | 336 | 352 | `ID`, `Host` | 101 |
| `Return` | 300,000 | 160 | 160 | `ID` | 46 |
| `Method` | 200,000 | 416 | 416 | `ID`, `Host` | 79 |
| `Param` | 200,000 | 224 | 224 | `ID` | 43 |
| `Struct` | 100,000 | 400 | 416 | `ID` | 40 |
| `Interface` | 100,000 | 376 | 384 | `ID` | 37 |

The nodes take 674 MiB at their size classes. The heap profile samples one
allocation in 512 KiB on average, so its total of 658 MiB estimates the same
bytes. `symbol.Identity` has five strings and a kind. The 3.5 million identity
fields of these nodes take 294 MiB. The positions of the nodes take 64 MiB at
32 bytes each.

### The identity of a type reference is always zero

The model gives every kind a node-side `ID`. The identity assignment fills it
for files and for every declaration kind. That includes members, parameters,
returns and type parameters. A type reference is not a declaration. The
assignment skips it. The resolution step writes the `Target` of a reference
and leaves the `ID` alone.

techne finds six references to `TypeRef.ID`. They are the generated encoder
and decoder, the generated `Identity` method and three generated tests.

The 900,000 type references of the corpus contain 900,000 zero identities. At
its size class, the field costs 96 bytes of each reference. The encoding of a
reference in the sealed state spends 6 bytes on it.

### Real code repeats the targets of references and the hosts of members

The corpus of the measurement references only builtin types, so none of its
900,000 references has a target. We counted the identity fields that the load
leaves in two real Go modules.

| Module | Go files | Type references | Without a target | Resolved | Distinct targets |
|---|---|---|---|---|---|
| `eidos-core` | 649 | 43,430 | 23,094 | 20,336 | 1,777 |
| A library outside this repository | 750 | 56,454 | 31,747 | 24,707 | 2,810 |

| Module | Field hosts | Distinct | Method hosts | Distinct |
|---|---|---|---|---|
| `eidos-core` | 4,042 | 910 | 4,208 | 1,052 |
| A library outside this repository | 3,968 | 1,157 | 5,589 | 1,242 |

About half of the references have no target. In both modules, a resolved
target occurs 9 to 11 times, and a host occurs 3.4 to 4.5 times. Today the
model stores a copy of the identity in each occurrence. A shared
representation saves memory only where the occurrences outweigh the cost of
interning. The alternatives measure that cost.

### The frontend cannot shrink the nodes

The lowering allocates the nodes and their member lists. The growth of the
member lists allocates 26 MiB over the whole run. One backing array for each
declaration would remove the rounding to the size classes. That rounding is
27 MiB of the 674 MiB. The rest of the bytes are in the layout of the nodes.
The schema fixes that layout for every frontend, rule and backend.

## Detailed design

### Components

| Component | Change |
|---|---|
| `symbol/schema` | `TypeRef` declares no `ID` field |
| `internal/gen/model` | A node-side kind without an `ID` field gets an `Identity` method that returns the zero identity |
| `node` | The generated `TypeRef` has no `ID`, its codec writes no identity for it, and `ModelFingerprint` changes |
| `sdk/node` | The facade copies the new documentation of `TypeRef`, which remains an alias of the kernel's type |

### Invariants

- A type reference refers to a declaration through `Target`. It is not a
  declaration itself. Its `Identity` method returns the zero identity.
- Every node-side kind satisfies `node.Declaration`. `node.Declarations` and
  every type assertion against the interface behave as they do today.
- Every other identity field of the model remains a `symbol.Identity` value.
- A change of the encoding changes the model fingerprint. Every unit key folds
  the fingerprint, so the units of an older state match no unit of a run under
  the new model.

### The schema

`TypeRef` loses one field. Its documentation states why.

```go
// TypeRef is a type as a declaration mentions it.
//
// A type reference is not a declaration, so it has no identity of its
// own. Its Target is the identity of the declaration it resolves to.
//
// Spelling is the source text verbatim. Target is the canonical
// identity the spelling resolves to. It is zero until the
// resolution step runs, and permanently for builtins and types
// outside the workspace. That is legitimate degradation, so a
// consumer checks the target before relying on it.
type TypeRef struct {
    Pos      position.Pos    `eidos:"node"`
    Spelling string          `eidos:"both"`      // source text, verbatim
    Target   symbol.Identity `eidos:"both"`      // zero until resolution, and for builtins, externals and structural forms
    Package  string          `eidos:"both"`      // what the import names; "" for a builtin, a structural form and a name needing no import
    Form     symbol.TypeForm `eidos:"both"`      // the structure; FormNamed by default
    Variance symbol.Variance `eidos:"both"`      // FormWildcard: In for a lower bound, Out for an upper one
    Async    bool            `eidos:"both"`      // FormStream: the elements arrive asynchronously
    Elems    []*TypeRef      `eidos:"both,walk"` // the form's children, in the form's fixed order
    Split    int             `eidos:"both"`      // FormFunc: the index in Elems where the returns begin
    Length   int             `eidos:"both"`      // FormArray: the literal length; 0 when the spelling keeps an expression
    Args     []*TypeRef      `eidos:"both,walk"` // the type arguments of an instantiation
    Fields   []*Field        `eidos:"node,walk"` // FormInline: the body's fields, without identities
    Methods  []*Method       `eidos:"node,walk"` // FormInline: the body's methods, without identities
}
```

The other paragraphs of the documentation do not change.

### The generator

Today the generator writes an `Identity` method only for a kind with an `ID`
field. It declares `node.Declaration` only when every kind of a side has one.
The view of a kind gains one field:

```go
// ZeroIdentity reports a node-side kind without an ID field. Its
// Identity method returns the zero identity, so the kind still
// satisfies the Declaration interface.
ZeroIdentity bool
```

The predicate counts the new field:

```go
// viewsIdentified reports whether every kind has an identity: an ID
// field, or the zero identity of a kind without one.
func viewsIdentified(views []view) bool {
    for _, v := range views {
        if v.IDStorage == "" && !v.ZeroIdentity {
            return false
        }
    }
    return len(views) > 0
}
```

`ZeroIdentity` is false on every emit kind, so the emit side does not declare a
`Declaration` interface. The template of the kinds writes the method:

```go
// Identity returns the zero identity. The kind does not declare an
// identity field, because it is not a declaration.
func (x *TypeRef) Identity() symbol.Identity { return symbol.Identity{} }
```

The codec templates write one call for each field, so the encoder and the
decoder of `TypeRef` lose their identity calls without a template change. The
fingerprint reads the lowered schema, so `ModelFingerprint` changes.

### Migration

1. Edit `symbol/schema/typeref.go`, and change the generator and its templates.
2. Regenerate the model, its tests and the facade of the SDK.
3. Add a changeset that marks the change as breaking. A plugin that reads or
   sets `TypeRef.ID` stops compiling. The repository has no such plugin.

The first run after the upgrade parses every unit again, because the new
fingerprint changes every unit key.

### Cost

We measured the proposal in a scratch copy of the working tree. The copy drops
the field from the generated `TypeRef` and from its codec. Its `Identity`
method returns the zero identity. Each round ran one cold run of each binary
over a new corpus of 200,000 declarations on XFS. The rounds alternated the
order of the binaries.

| Round | Peak today | Peak without the field |
|---|---|---|
| 1 | 3,274 MiB | 3,016 MiB |
| 2 | 3,220 MiB | 3,118 MiB |
| 3 | 3,211 MiB | 3,088 MiB |

The average peak fell from 3,235 MiB to 3,074 MiB. That is 5% of the peak. The
field takes 82 MiB at its size class. The garbage collector lets the heap grow
to about twice its live size, so the peak falls by about twice as much. At the
peak sample, the heap profile attributes 307.6 MiB to the lowering of type
references today and 235.1 MiB without the field.

Wall time measured 4.20 to 4.25 s today and 4.24 to 4.54 s without the field.
CPU time measured 6.79 to 6.80 s today and 6.77 to 7.09 s without the field.
The rounds do not separate the two.

At 96 bytes for each type reference, the saving in the two real modules is
4.0 MiB for `eidos-core` and 5.2 MiB for the library.

### Tests

- A test of the generator checks the zero identity of a node-side kind without
  an `ID` field.
- A second test of the generator checks that such a side still declares
  `Declaration`.
- A third test of the generator checks that an emit kind gets no `Identity`
  method.
- The generated test of the kinds checks the zero identity of `TypeRef`, and
  the assigned identity of every other node kind.
- The generated codec test round-trips a type reference without the field.
- The test of the facade checks that `node.TypeRef` remains an alias of the
  kernel's type.

## Alternatives considered

### Keep the layout

Every kind keeps an `ID`.

**Why not:** the field is zero in every type reference that the engine builds.
At 200,000 declarations it costs 82 MiB of live heap and about 161 MiB of
peak.

### Share the identity of a target

`Target` becomes a handle to a canonical identity. A type `symbol.Ref` wraps
`unique.Handle[symbol.Identity]` for it. Handles of one identity compare equal.
The zero handle is the zero target. The codec writes the identity that
a handle refers to and interns it again on decode.

```go
// Ref refers to a canonical identity. The zero Ref refers to none.
type Ref struct {
    h unique.Handle[Identity]
}

// RefOf returns the reference to id, and the zero Ref for the zero
// identity.
func RefOf(id Identity) Ref

// Identity returns the identity r refers to, and the zero identity for
// the zero Ref.
func (r Ref) Identity() Identity
```

A handle takes 8 bytes, so a type reference would move from the 288-byte class
to the 192-byte class after this proposal. The corpus of the measurement
resolves no target. We estimate a saving of another 82 MiB of live heap on it.
We did not measure it. The change touches every site that reads the field.

**Why not:**

- Code in the repository reads or writes `TypeRef.Target` at 184 sites.
  Production files in seven modules contain 95 of them. Tests contain the other
  89. Each comparison, map
  key, lookup and literal among them changes. Every plugin outside the
  repository that reads the field changes too.
- `unique.Make` copies every string of the value that it interns. A
  measurement of 200,000 identities took 348 bytes for each distinct identity.
  A resolved target occurs 9 to 11 times in real code, so a handle saves memory
  there. Each distinct target takes 348 bytes of that saving back.
- Every boundary that stores an identity needs the value behind the handle.
  The codec, the manifests and the JSON form are such boundaries.

A breaking change of the node API for another reason reopens this alternative.
The RFC of that change measures the layout in place.

### Share the host of a member

`Host` becomes the same handle on fields, methods, embeds and variants. A field
would move from the 352-byte class to the 256-byte class. A method would move
from the 416-byte class to the 352-byte class.

**Why not:** a host occurs 3.4 to 4.5 times in real code. An interned identity
costs as much as 4 values. On the corpus of the measurement, the 200,000
distinct hosts would cost 66 MiB in the intern table to save 40 MiB in the
nodes. The change also touches 76 sites.

### Intern every identity field

Every `ID`, `Host` and `Target` becomes a handle, so each distinct identity is
stored once.

**Why not:** an `ID` belongs to one declaration and occurs once in the nodes.
As a handle, it costs 348 bytes instead of 88. The 1,210,000 identities of the
corpus's files and declarations would take about 300 MiB more. Handles for the
hosts and the targets would save about 107 MiB of it.

### A run-local table of dense identifiers

Each identity field becomes an index of 4 bytes into a table that the run
keeps. The architecture commits the engine to dense run-local identifiers for
identities. The model keeps the struct as the form that a plugin reads.

**Why not:** a reader of an identity would need the table, so every rule,
backend and plugin would take one more argument. In a measurement, a map to the
index and a slice of the values took 228 bytes for each distinct identity. At
that cost, the table saves memory only for an identity that occurs 3 times or
more.

### A smaller identity

A smaller `symbol.Identity` stores its six parts in fewer words. One shared
pointer could replace the language and the package.

**Why not:** the parts are exported fields of a value type that every package
of the engine and every plugin reads. The saving is 24 bytes of each identity.
clangd stores an 8-byte hash of its symbol spelling to save memory. A hash in
the model would need the spelling beside it wherever a plugin reads an
identity.

### One backing array for each declaration

The frontend allocates the nodes of a declaration in one array instead of one
allocation each.

**Why not:** the saving is the rounding to the size classes. At 200,000
declarations that is 27 MiB. Every frontend would need the same allocator. A
measurement that shows a cut in the collector's CPU from fewer objects reopens
it.

## Drawbacks

- `node.TypeRef` loses a field. A plugin that reads or sets it stops
  compiling. The repository has none outside generated code, and the SDK has no
  tagged release that a plugin outside the repository could build on.
- A node-side kind without an `ID` field gets a generated `Identity` method
  that returns the zero identity. The generator needs one field of the view,
  one condition in `viewsIdentified`, one branch in each of two templates, and
  tests for each.
- `TypeRef` keeps an `Identity` method whose result is always zero. The method
  exists so that every node kind satisfies `Declaration`.
- The first run after the upgrade parses every unit again in every workspace.
- The saving is 96 bytes for each type reference. At 200,000 declarations that
  is 6.5% of the live heap at the peak. A module with fewer references for each
  declaration saves less.

## Unresolved and future work

- A type reference that leaves `node.Declaration` is not proposed here. 91 sites
  refer to the interface, and each one would need a check that it treats a type
  reference as it does today. The zero identity keeps every site as it is.
- The engine's own maps keyed by identity are not part of this proposal. The
  duplicate check of the identity assignment allocates 0.46 GB over the cold
  run at 200,000 declarations. That memory is free before the peak.
- The positions of the nodes take 64 MiB at 200,000 declarations. A smaller
  position is not proposed here.

## References

| What | Where |
|---|---|
| Go 1.27.2, the size classes of the runtime | https://github.com/golang/go/blob/go1.27.2/src/internal/runtime/gc/sizeclasses.go |
| Go 1.27.2, the copy of each string that `unique.Make` interns | https://github.com/golang/go/blob/go1.27.2/src/unique/clone.go |
| clangd, the 8-byte hash that identifies a symbol | https://github.com/llvm/llvm-project/blob/ffb7404d1eb1fa8cca49e2cc800b4c97688f8bab/clang-tools-extra/clangd/index/SymbolID.h |
| D56, dense run-local identifiers inside the engine | ../architecture/21-decisions.md |
| The symbol model contract | [RFC-0001](0001-symbol-model-contract.md) |
| The model generator | [RFC-0002](0002-model-generator.md) |
| The owner back-pointer as an identity | [ADR-0006](../adr/0006-host-is-an-identity.md) |
| Warm runs and the sealed state | [RFC-0020](0020-warm-runs-and-the-sealed-state.md) |
