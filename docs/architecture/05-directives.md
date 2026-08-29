# Directives

*Builds on: [03](03-projection.md) (resolution kinds),
[04](04-metadata.md) (authority). Feeds: [06](06-plugins.md)
(schemas are plugin declarations), [16](16-diagnostics.md) (the
diag directive), [18](18-routing-and-layout.md) (the out keys).*

Directives are the stickiest public surface eidos has — they live in
consumers' source files. The design splits them into three layers so
per-language idiom and cross-language stability stop competing.

## The three layers

### Carrier — per language, owned by the frontend

Where a directive physically sits: `//+gen:` in Go and TS, `#+gen:`
in Python, leading comments in proto, `//+gen:` in Rust. Each
frontend recognizes its carriers and hands the workspace the one
canonical parsed form.

A language satellite may also register **native sugar**: syntax the
language already has for attached metadata, read statically and
lowered into the same canonical directive — Rust
`#[gen::stub(tag = "test")]`, TS decorators. Sugar never executes
anything and never introduces semantics; it is an alternate spelling
of a schema'd directive, and the completeness rung tests the lowering
like any carrier.

### Grammar — one, kernel-owned

The payload syntax is a single grammar for every language. If it
forked per language, every cross-language plugin would parse N
dialects, documentation would fork, and moving a service between
languages would mean rewriting annotations. Left ungoverned, two
consumers invent two notations for one key and nothing parses
either — at ecosystem scale.

The core is terse and greppable: `name key=value`, positional bare
args, `role=` scoping. The grammar is pinned here, not deferred —
this is the surface least able to absorb change, so it gets the most
exact spec:

```ebnf
directive    = name , { ws , arg } ;
name         = [ ident , ":" ] , ident ;          (* plugin-prefixed once ambiguous *)
arg          = kv | value ;                       (* bare value = positional *)
kv           = key , "=" , value ;
key          = ident ;
value        = list | quoted | bare ;
list         = "[" , [ value , { "," , [ws] , value } ] , "]" ;
quoted       = '"' , { escape | any-but-quote } , '"' ;
escape       = "\" , ( '"' | "\" | "n" | "t" ) ;
bare         = 1*( any-but ws '"' "[" "]" "," "=" ) ;
ident        = letter , { letter | digit | "-" | "_" } ;
```

The rules around it, decided:

- **One quoting form.** Double quotes with the four escapes above;
  there is no raw-string form. A value needing more than that is a
  sign it belongs in config, not in a comment.
- **Continuation is a trailing `\`.** A payload line ending in `\`
  joins the next carrier line (marker stripped) with a single
  space. No repeated-marker rule — one mechanism.
- **Keys are flat.** No dotted or nested keys; a directive needing
  structure uses a list or splits into two directives.
- **Namespacing**: a plugin's directives are `<plugin>:<name>` once
  two plugins claim one name; bare names remain sugar while
  unambiguous. `role=` is an ordinary key that schemas may reserve.
- The grammar applies to the payload after carrier stripping —
  identical in every language, byte for byte.

### Schema — one, kernel-owned

Every directive registers a schema; unclaimed directives are
reported, with a workspace-level opt-out. The schema turns the
classic under-determination failure — a directive whose params do not
pin what the consuming check may assume — into structure:

- **Typed params**: string, int, bool, list, reference.
- **Resolution kinds** on reference params: callable-in-scope,
  package-var, value-field, host-param, member-on-handle — bound
  through `rules.Resolve`
  ([03-projection.md](03-projection.md)), so "names a sibling
  callable" is schema, not per-plugin prose.
- **Counterexample marking**: a param whose value names an input no
  derivation can invent.
- **Closure**: deny-unknown-keys by default; a schema enumerates what
  it accepts.
- **Required/optional with role scoping**.
- **Repeatability**: a schema declares whether its directive may
  appear more than once on one subject. The default is
  single-instance — a second `default=` on one field is a
  contradiction, reported as a validation Error naming both
  positions before any handler runs. `Repeatable: true` is for
  genuinely repeatable declarations (`index fields=[…]` twice is
  two indexes); dispatch then fires the handler once per instance,
  in source order ([06b-authoring.md](06b-authoring.md)). Forcing a
  repeatable declaration into one directive would push authors to
  invent list-of-lists encodings — the two-notations disease this
  schema layer exists to prevent.

- **Cross-directive constraints**: a schema may declare
  `Requires` and `ConflictsWith` over other directive names,
  resolved against the registry at Build (an unknown name is a
  Build error) and enforced at Freeze over each subject's full
  directive list — a violation is one Error naming both
  positions. The check is the kernel validator's by necessity:
  the visibility law ([06b-authoring.md](06b-authoring.md)) leaves no
  plugin able to see a stranger's directive, so a contradiction
  between annotations is caught here or nowhere.

Every schema **generates constants** for its directive name and
param keys — typed plugins and their tests reference the constants,
so a misspelled key is a compile error in the plugin and a Build
error only for hand-typed carriers. There is nothing hand-written to
drift between a schema and the code that serves it.

The canonical parsed form — what every carrier lowers to and what
handlers receive through `m.Directive()`
([06b-authoring.md](06b-authoring.md)):

```go
type Directive struct {
    Name     Name           // resolved against the registry
    Args     []Value        // positional, typed per schema
    Params   map[ParamKey]Value
    Pos      position.Pos   // the carrier line
    Instance int            // source order among repeatable instances
}

// Freeze-time validation: closure, types, resolution kinds,
// repeatability, cross-directive constraints — over each subject's
// full directive list, positioned Errors, before any handler runs.
func Validate(subject symbol.Symbol, ds []Directive, r *Registry, d Diag) bool
```

## Kernel-owned directives and reserved keys

Four directive names are the kernel's, with reserved semantics:

- `meta` — metadata override at `directive` authority; its
  reserved `drop=<key|group>` param deletes a fact — or a whole
  declared fact group — instead of setting one: the only
  spelling for negating a stamped boolean, since absence is the
  negative and `false` is never written
  ([04-metadata.md](04-metadata.md)).
- `out` — a routing override for a declaration
  ([18-routing-and-layout.md](18-routing-and-layout.md)).
- `diag` — diagnostic suppression at a declaration
  ([16-diagnostics.md](16-diagnostics.md)).
- `skip` — excludes a declaration from bare and fact-gated
  triggers, optionally per plugin (`skip plugin=mockgen`); honored
  by the authoring dispatch itself, so an everything-generator's
  opt-out is never per-plugin boilerplate
  ([06b-authoring.md](06b-authoring.md)). Directive-gated rules are
  unaffected — an explicit opt-in is withdrawn by deleting it, not
  by a second annotation fighting the first.

Two further kernel-owned schemas exist for authored values:
`sample` (a declaration's sample value and its alternate) and
`witness` (one concrete type per type parameter, `T=int`), whose
keys validate against the subject's own declaration. Their
handlers ship as default annotators, not in the kernel — but the
spellings and the `gen.*` keys they stamp are kernel API, because
the Tier-1 Values and Tier-2 Witnesses answers consult those
stamps ([03-projection.md](03-projection.md)), and the surface
least able to absorb change cannot belong to whichever annotator
happens to be registered.

Routing intent has a second spelling: `out=` and `tag=` are
**reserved keys honoured on any plugin-owned directive**, so an
author routing the output of the directive they are already writing
does not write a second one. Both spellings lower to the same
routing override; the reserved keys are unclaimable by plugin
schemas.

Directive-sourced facts land in the same bags with `directive`
authority — the override story is one story.

## Stability

The grammar and the kernel schemas are versioned with the kernel and
covered by the compatibility policy
([15-compatibility.md](15-compatibility.md)): additive within a
major. A consumer's annotations are the one thing a major version is
expected to carry forward unchanged; breaking the grammar is the
same-severity event as breaking the symbol model.
