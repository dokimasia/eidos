# Routing and layout

*Builds on: [05](05-directives.md) (the out keys),
[08](08-workspace-and-plans.md) (the Layout phase),
[17](17-output-and-determinism.md) (ownership and collisions).
Feeds: [20](20-cli.md) (prune).*

Where generated files land. Routing is one of the largest
consumer-facing surfaces the system has: it is what a platform team
configures, what a source author overrides per declaration, and what
collision semantics defend.

## The model

Routing resolves per emit declaration, in one pass (the Layout phase
of each plan), from four inputs in fixed precedence:

1. **Plugin outputs** — what a plugin declares it emits: a set of
   tagged output families, each with a **cardinality** and a
   filename derivation. The empty tag is the primary output; tags
   name companions (`test`, `docs`, …). A plugin exports its tags
   as constants; `tag=test` in a directive or config is the
   boundary spelling, validated against the declared outputs.

   | Cardinality | One output per | Filename | Dir |
   |---|---|---|---|
   | `PerSource` | source file | stem + suffix + ext | source dir, or centralised |
   | `PerPackage` | package | declared name + ext | package dir, or centralised |
   | `PerPlan` | plan | declared name + ext | declared or configured — no natural source dir exists |

   Provenance is plural in every cardinality and independent of
   derivation: the manifest records every origin identity, whether
   one source file fed the output or thirty packages did. Aggregate
   families (`PerPackage`, `PerPlan`) fill through accumulator
   emitters that order contributions by canonical subject identity
   ([06b-authoring.md](06b-authoring.md)); the collision rules below
   apply to them unchanged.
2. **Plan layout policy** — the plan-level default:
   - `alongside-source`: files land beside the declarations they
     derive from, named by derivation (below).
   - `centralised`: files land under a configured output directory,
     grouped by package.
3. **Configured refinements** — per-plugin and per-(plugin, tag)
   overrides of layout, package, directory, or filename, in the
   plan's config. Precedence: (plugin, tag) over plugin over plan.
4. **Per-declaration overrides** — the kernel's `out` directive, or
   the reserved `out=`/`tag=` keys on any plugin-owned directive;
   both spellings lower to the same override
   ([05-directives.md](05-directives.md)). `out=<path>` redirects,
   `tag=<name>` selects a companion output. Source-level intent
   outranks configuration, consistent with the authority model.

## The resolution pass

Layout runs once per plan, one pass over the plan's emit values:

1. **Family.** Resolve the declaration's tag (empty = primary)
   against its plugin's declared outputs; a tag naming no
   declared output is the Error below.
2. **Key.** Resolve the cardinality key: `PerSource` → the
   origin's source file, `PerPackage` → the origin's package,
   `PerPlan` → the plan.
3. **Destination.** Compose directory, package, and filename from
   the four precedence inputs above — most specific wins, per
   field.
4. **Spelling.** Derive the filename through the target's
   lowering, from the whole family declaration (cardinality,
   word, tag).
5. **Identity.** Under centralised layout, derive the package
   identity (below); refusal waits for the point of need.
6. **Record.** The completed declaration→destination map feeds
   the collision rules below, then the plan's render pass
   ([07-rendering.md](07-rendering.md)) — routing is settled
   before any template runs.

## Filename derivation

Filename derivation is the language satellite's (`lowering/`), and
its input is the **whole family declaration** — cardinality, word,
and tag: the plugin declares meaning, the target spells it. A
declaration in `store.go` through a `PerSource` family with word
`stub` lands in `store_stub.go` under the Go lowering and
`store.stub.ts` under the TS one; join conventions, case rules, and
extensions are language facts, never plugin declarations.

Tags participate in the spelling because some carry **ecosystem
semantics**: the kernel blesses `TagTest` as a well-known tag (the
same small-registry move as the type hub's well-knowns), and a
target may give it its ecosystem's treatment — Go spells
`(word: suite, TagTest)` as `suite_test.go`, which compiles into
the test binary; TS spells `suite.test.ts`. Unknown tags get the
target's deterministic default join. A plugin never writes
`_test` into a word — that is the extension leak one level up.

`centralised` filenames derive the same way under
`<outdir>/<package-path>/`, where `<package-path>` is the source
package's **workspace-relative directory** — unambiguous under
multi-module workspaces, where module-relative paths would collide
the moment two modules contain `internal/api`. Routing that wants
module identity uses the neutral `gen.module` facts the frontend
stamped ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## Package identity under centralised layout

A centralised file needs more than a directory: it declares a
package, and sibling generated code may import it. The identity
derives exactly when the output directory falls inside a module
the frontends resolved — the neutral `gen.module` /
`gen.moduleRoot` facts
([08-workspace-and-plans.md](08-workspace-and-plans.md)) — and
the *lowering* composes it: the innermost containing module's
path plus the relative directory, spelled per its ecosystem. The
kernel routes directories and hands the facts over; how a module
path joins a directory is a language fact, like every other
spelling in this document. An output directory outside every known
module has no derivable identity; the plan's layout config then
supplies `importBase`, validated at Build.

Where neither answers, the lowering refuses — at the
*referencing* declaration, with a stable code, and only when a
cross-reference actually needs the qualification. Refusal at the
point of need: a centralised file nothing imports costs nothing,
and a guessed import path is the one failure a compiler will not
catch — a bare name silently binding to whatever else is in
scope.

## Failure semantics

Routing failures are stable-coded diagnostics
([16-diagnostics.md](16-diagnostics.md)), positioned at the emit
declaration's origin:

- a routable declaration whose plugin declares no outputs;
- a `tag=` naming no declared output;
- an override scoped ambiguously (a multi-output plugin overridden
  without a tag);
- a declaration with no resolvable destination.

Static contradictions in config (an override naming an unknown
plugin or tag) fail workspace Build instead — configuration errors
should never wait for a run.

## Collisions

- **Within a plan**: two declarations resolving to one path is legal
  exactly when the backend can merge them (same package, same file —
  the ordinary many-declarations-one-file case) and a Layout-phase
  Error otherwise.
- **Across plans**: always an Error, detected at Close against the
  merged manifest
  ([08-workspace-and-plans.md](08-workspace-and-plans.md)) — plans
  are isolated; sharing an output path is never what isolation
  means.
- **With the tree**: a generated path colliding with a hand-written,
  unmanifested file is an Error naming both; eidos never overwrites
  a file it cannot prove it produced
  ([17-output-and-determinism.md](17-output-and-determinism.md)).

## Paths

All routed paths are workspace-relative, slash-separated, and
root-jailed by the sink. `out=` accepts relative paths resolved
against the declaration's package directory; escaping the workspace
root is a diagnostic, not a write.
