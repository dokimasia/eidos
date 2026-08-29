# Routing and layout

*Builds on: [05](05-directives.md) (the out keys),
[08](08-workspace-and-plans.md) (the Layout phase),
[17](17-output-and-determinism.md) (ownership and collisions).
Feeds: [20](20-cli.md) (prune).*

Where generated files land. Routing is one of the largest surfaces
consumers touch: a platform team configures it, a source author
overrides it per declaration, and the collision rules defend it.

## The model

Routing resolves once per emit declaration, during each plan's
Layout phase, from four inputs in fixed precedence.

**1. Plugin outputs**, meaning what a plugin declares it emits: a
set of tagged output families, each with a cardinality and a
filename derivation. The empty tag is the primary output, and tags
name companions such as `test` or `docs`. A plugin exports its tags
as constants, and `tag=test` in a directive or in config is the
human spelling, checked against the declared outputs.

| Cardinality | One output per | Filename | Directory |
|---|---|---|---|
| `PerSource` | source file | stem + suffix + extension | the source directory, or centralised |
| `PerPackage` | package | declared name + extension | the package directory, or centralised |
| `PerPlan` | plan | declared name + extension | declared or configured, since no natural source directory exists |

Provenance is plural in every cardinality, and it does not depend on
the derivation: the manifest records every origin identity, whether
one source file fed the output or thirty packages did. The aggregate
families, `PerPackage` and `PerPlan`, fill through accumulator
emitters that order contributions by canonical subject identity
([06b-authoring.md](06b-authoring.md)), and the collision rules
below apply to them unchanged.

**2. Plan layout policy**, the plan-level default. Under
`alongside-source`, files land beside the declarations they come
from, named by the derivation below. Under `centralised`, they land
under a configured output directory, grouped by package.

**3. Configured refinements**, meaning per-plugin and
per-(plugin, tag) overrides of layout, package, directory or
filename in the plan's config. The more specific wins:
(plugin, tag) over plugin over plan.

**4. Per-declaration overrides**, meaning the kernel's `out`
directive, or the reserved `out=` and `tag=` keys on any
plugin-owned directive. Both spellings lower to the same override
([05-directives.md](05-directives.md)). `out=<path>` redirects, and
`tag=<name>` selects a companion output. What the source author
writes outranks configuration, which matches the authority model.

## The resolution pass

Layout runs once per plan, in one pass over the plan's emit values.

1. **Family.** Resolve the declaration's tag, where empty means
   primary, against its plugin's declared outputs. A tag naming no
   declared output is the Error listed below.
2. **Key.** Resolve the cardinality key: `PerSource` uses the
   origin's source file, `PerPackage` the origin's package, and
   `PerPlan` the plan.
3. **Destination.** Compose the directory, package and filename from
   the four inputs above, taking the most specific answer per field.
4. **Spelling.** Derive the filename through the target's lowering,
   from the whole family declaration: cardinality, word and tag.
5. **Identity.** Under centralised layout, derive the package
   identity, as described below. Refusal waits until something
   actually needs it.
6. **Record.** The finished map from declaration to destination
   feeds the collision rules below, and then the plan's render pass
   ([07-rendering.md](07-rendering.md)). Routing is settled before
   any template runs.

## Filename derivation

Filename derivation belongs to the language satellite, in
`lowering/`, and its input is the whole family declaration:
cardinality, word and tag. The plugin declares the meaning and the
target spells it. A declaration in `store.go` through a `PerSource`
family with the word `stub` lands in `store_stub.go` under the Go
lowering and `store.stub.ts` under the TypeScript one. Join
conventions, case rules and extensions are language facts, never
plugin declarations.

Tags take part in the spelling because some carry meaning in an
ecosystem. The kernel blesses `TagTest` as a well-known tag, the
same small-registry move the type hub makes for well-known types,
and a target may give it the treatment its ecosystem expects: Go
spells `(word: suite, TagTest)` as `suite_test.go`, which compiles
into the test binary, and TypeScript spells it `suite.test.ts`. An
unknown tag gets the target's deterministic default join. A plugin
never writes `_test` into a word, because that leaks the extension
one level up.

Centralised filenames derive the same way, under
`<outdir>/<package-path>/`, where `<package-path>` is the source
package's workspace-relative directory. That stays unambiguous under
multi-module workspaces, where module-relative paths would collide
as soon as two modules both contain `internal/api`. Routing that
wants module identity uses the neutral `gen.module` facts the
frontend stamped
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## Package identity under centralised layout

A centralised file needs more than a directory: it declares a
package, and sibling generated code may import it.

The identity derives exactly when the output directory falls inside
a module the frontends resolved, through the neutral `gen.module`
and `gen.moduleRoot` facts
([08-workspace-and-plans.md](08-workspace-and-plans.md)). The
lowering composes it from the innermost containing module's path
plus the relative directory, spelled the way its ecosystem spells
it. The kernel routes directories and hands the facts over, because
how a module path joins a directory is a language fact like every
other spelling in this document.

An output directory outside every known module has no derivable
identity, and the plan's layout config then supplies `importBase`,
which Build validates.

Where neither answers, the lowering refuses, at the referencing
declaration, with a stable code, and only when a cross-reference
actually needs the qualification. Refusing at the point of need
costs nothing for a centralised file that nothing imports, and a
guessed import path is the one failure a compiler will not catch,
because a bare name binds silently to whatever else is in scope.

## Failure semantics

Routing failures are diagnostics with stable codes
([16-diagnostics.md](16-diagnostics.md)), positioned at the emit
declaration's origin:

- a routable declaration whose plugin declares no outputs;
- a `tag=` naming no declared output;
- an override scoped ambiguously, such as a multi-output plugin
  overridden without a tag;
- a declaration with no resolvable destination.

A static contradiction in config, such as an override naming an
unknown plugin or tag, fails at workspace Build instead. A
configuration error should never wait for a run.

## Collisions

**Within a plan**, two declarations resolving to one path is legal
exactly when the backend can merge them, meaning the same package
and the same file, which is the ordinary case of many declarations
in one file. Anything else is a Layout-phase Error.

**Across plans**, it is always an Error, detected at Close against
the merged manifest
([08-workspace-and-plans.md](08-workspace-and-plans.md)). Plans are
isolated, and sharing an output path is not what isolation means.

**With the tree**, a generated path colliding with a hand-written,
unmanifested file is an Error naming both. eidos never overwrites a
file it cannot prove it produced
([17-output-and-determinism.md](17-output-and-determinism.md)).

## Paths

Every routed path is workspace-relative, slash-separated, and
root-jailed by the sink. `out=` accepts a relative path, resolved
against the declaration's package directory. If a path escapes the
workspace root, eidos reports a diagnostic and writes nothing.
