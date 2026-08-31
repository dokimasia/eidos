# Directives

*Builds on: [03](03-projection.md) (resolution kinds),
[04](04-metadata.md) (authority). Feeds: [06](06-plugins.md)
(schemas are plugin declarations), [16](16-diagnostics.md) (the diag
directive), [18](18-routing-and-layout.md) (the out keys).*

Directives sit in consumers' source files, which makes them the
public surface eidos can least afford to change. The design splits
them into three layers so that per-language idiom and cross-language
stability stop competing.

## The three layers

### Carrier: per language, owned by the frontend

The carrier is where a directive physically sits: `//+gen:` in Go
and TypeScript, `#+gen:` in Python, a leading comment in proto,
`//+gen:` in Rust. Each frontend recognizes its own carriers and
hands the workspace one canonical parsed form.

A language satellite may also register **native sugar**, meaning
syntax the language already has for attaching metadata, read
statically and lowered into the same canonical directive. Rust
`#[gen::stub(tag = "test")]` and TypeScript decorators work this
way. Sugar executes nothing and introduces no semantics. It is
another spelling of a directive that already has a schema, and the
completeness check tests the lowering the way it tests any carrier.

### Grammar: one, owned by the kernel

The payload uses one grammar in every language. Fork it per language
and every cross-language plugin has to parse N dialects, the
documentation forks with it, and moving a service between languages
means rewriting its annotations. Leave it ungoverned and two
consumers invent two notations for one key, at which point nothing
parses either.

The core stays terse and greppable: `name key=value`, positional
bare arguments, and `role=` for scoping. The grammar is pinned here
rather than deferred, because this is the surface least able to
absorb a change:

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

- **One quoting form.** Double quotes with the four escapes above,
  and no raw-string form. A value that needs more than that belongs
  in config rather than in a comment.
- **Continue a line with a trailing `\`.** A payload line ending in
  `\` joins the next carrier line, with the marker stripped, using a
  single space. There is no repeated-marker rule, because one
  mechanism is enough.
- **Keys are flat.** No dotted or nested keys. A directive that
  needs structure uses a list or splits into two directives.
- **Namespacing.** A plugin's directives become `<plugin>:<name>`
  once two plugins claim one name. A bare name stays available while
  it is unambiguous. `role=` is an ordinary key that a schema may
  reserve.
- The grammar applies to the payload after the carrier is stripped,
  and it is identical in every language, byte for byte.

### Schema: one, owned by the kernel

Every directive registers a schema. An unclaimed directive is
reported, with a workspace-level opt-out available.

The schema turns a classic failure into structure. That failure is a
directive whose params do not pin down what the consuming check may
assume. The schema declares:

- **Typed params**: string, int, bool, list, reference.
- **Resolution kinds** on reference params: callable-in-scope,
  package-var, value-field, host-param, member-on-handle. These bind
  through `rules.Resolve` ([03-projection.md](03-projection.md)), so
  "this names a sibling callable" is schema rather than prose in
  each plugin's documentation.
- **Counterexample marking**, for a param whose value names an input
  no derivation could invent.
- **Closure**: unknown keys are denied by default, and a schema
  lists what it accepts.
- **Required or optional, with role scoping.**
- **Repeatability.** A schema says whether its directive may appear
  more than once on one subject. The default is single-instance,
  because a second `default=` on one field is a contradiction, and
  that is reported as a validation Error naming both positions
  before any handler runs. `Repeatable: true` covers genuinely
  repeatable declarations, where `index fields=[…]` twice means two
  indexes. Dispatch then runs the handler once per instance, in
  source order ([06b-authoring.md](06b-authoring.md)). Forcing a
  repeatable declaration into one directive pushes authors to invent
  list-of-lists encodings, which is the two-notations problem this
  layer exists to prevent.
- **Cross-directive constraints.** A schema may declare `Requires`
  and `ConflictsWith` over other directive names. Build resolves
  those names against the registry, so an unknown name is a Build
  error, and Freeze enforces them over each subject's full directive
  list, reporting one Error naming both positions. The kernel
  validator has to do this check, because the visibility law
  ([06b-authoring.md](06b-authoring.md)) leaves no plugin able to
  see a stranger's directive. A contradiction between annotations is
  caught here or not at all.

Every schema **generates constants** for its directive name and its
param keys. Typed plugins and their tests reference the constants,
so a misspelled key is a compile error in the plugin, and a Build
error only for a carrier a human typed. Nothing hand-written sits
between a schema and the code that serves it.

The canonical parsed form, which every carrier lowers to and which
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
// repeatability and cross-directive constraints, over each
// subject's full directive list, as positioned Errors, before any
// handler runs.
func Validate(subject symbol.Symbol, ds []Directive, r *Registry, d Diag) bool
```

## Kernel-owned directives and reserved keys

Four directive names belong to the kernel, with reserved semantics.

`meta` overrides metadata at `directive` authority. Its reserved
`drop=<key|group>` param deletes a fact, or a whole declared fact
group, instead of setting one. That is the only way to negate a
stamped boolean, since absence is the negative and `false` is never
written ([04-metadata.md](04-metadata.md)).

`out` overrides routing for a declaration
([18-routing-and-layout.md](18-routing-and-layout.md)).

`diag` suppresses a diagnostic at a declaration
([16-diagnostics.md](16-diagnostics.md)).

`skip` excludes a declaration from bare and fact-gated triggers, and
optionally from one plugin through `skip plugin=mockgen`. The
authoring dispatch honours it directly, so opting out of a generator
that fires on everything never becomes per-plugin boilerplate
([06b-authoring.md](06b-authoring.md)). Directive-gated rules are
unaffected, because a subject that opted in explicitly withdraws by
deleting the directive rather than by adding a second annotation to
fight the first.

Two further kernel-owned schemas exist for authored values. `sample`
carries a declaration's sample value and its alternate. `witness`
carries one concrete type per type parameter, as `T=int`. Both
validate their keys against the subject's own declaration. Their
handlers ship as default annotators rather than in the kernel, but
the spellings and the `gen.*` keys they stamp are kernel API,
because the Tier-1 Values and Tier-2 Witnesses answers read those
stamps ([03-projection.md](03-projection.md)). The surface least
able to absorb change cannot belong to whichever annotator happens
to be registered.

Routing intent has a second spelling. `out=` and `tag=` are reserved
keys honoured on any plugin-owned directive, so an author routing
the output of the directive they are already writing does not write
a second one. Both spellings lower to the same routing override, and
a plugin schema cannot claim either key.

Facts that come from directives land in the same bags at `directive`
authority, so the override story stays one story.

## Stability

The grammar and the kernel schemas are versioned with the kernel and
covered by the compatibility policy
([15-compatibility.md](15-compatibility.md)): a minor release only
adds. A consumer's annotations are the one thing a major version is
expected to carry forward unchanged, and breaking the grammar is as
severe as breaking the symbol model.
