# Routing and layout

*Builds on: [05](05-directives.md) (the out keys),
[08](08-workspace-and-plans.md) (the Layout phase),
[17](17-output-and-determinism.md) (ownership and collisions).
Feeds: [20](20-cli.md) (prune).*

Where generated files arrive. Routing is one of the largest surfaces
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
| `PerSource` | source file | stem + word + tag + extension | the source directory, or centralised |
| `PerPackage` | package | word + tag + extension | the package directory, or centralised |
| `PerPlan` | plan | word + tag + extension | declared or configured, since no natural source directory exists |

Provenance is plural in every cardinality, and it does not depend on
the derivation: the manifest records every origin identity, whether
one source file fed the output or thirty packages did. The aggregate
families, `PerPackage` and `PerPlan`, fill through accumulator
emitters that order contributions by canonical subject identity
([06b-authoring.md](06b-authoring.md)), and the collision rules
below apply to them unchanged.

**2. Plan layout policy**, the plan-level default. Under
`alongside-source`, files arrive beside the declarations they come
from, named by the derivation below. Under `centralised`, they arrive
under a configured output directory, grouped by package.

**3. Configured refinements**, meaning per-plugin and
per-(plugin, tag) overrides of layout, directory or filename in the
plan's config. The most specific refinement decides: (plugin, tag),
then plugin, then plan. A refinement names no package, because the
package of a path is a fact of the target language.

**4. Per-declaration overrides**, meaning the kernel's `out`
directive, or the reserved `out=` and `tag=` keys on a plugin's own
directive. Both spellings lower to the same override
([05-directives.md](05-directives.md)). The reserved keys route the
output of the plugin that registered the directive's schema, and the
kernel directive routes every plugin's output. `out=<path>`
redirects, and `tag=<name>` moves a declaration of the primary
output into a companion. What the source author writes takes
precedence over configuration, as the authority model ranks a
directive over config.

## The resolution pass

Layout runs once per plan, after the settle and before the render,
in one pass over the plan's emit values. Each step reads its inputs
in the precedence above, and the first input that states a value
decides it.

1. **Family.** A declaration of the primary output moves to the
   companion its tag names. A tag naming no declared output is an
   Error, and so is a unit of an output its plugin does not declare.
2. **Key.** Resolve the cardinality key: `PerSource` uses the
   origin's source file, `PerPackage` the origin's package, and
   `PerPlan` the plan.
3. **Directory.** An author's path resolves against the directory of
   the origin's source file. Otherwise the policy decides: the source
   directory beside the source, the source directory under the output
   directory when centralised, and the configured directory for a
   `PerPlan` output.
4. **Spelling.** Derive the filename through the target's lowering,
   from the whole family declaration: cardinality, word and tag. An
   author's path or the output's configuration that names a file
   takes precedence.
5. **Package.** The target names the package each file declares from
   the file's path, by its language's rule.
6. **Record.** Declarations with one path form one file, unless the
   collision rules refuse it. The plan's render pass renders the files
   ([07-rendering.md](07-rendering.md)), so routing is settled before
   any template runs.
7. **References.** A reference between two declarations of the plan
   that are written in files of two packages takes the package of the
   file it names, so the target qualifies it and records the import.

## Filename derivation

Filename derivation belongs to the language satellite, in
`spell/`, and its input is the whole family declaration:
cardinality, word and tag. The plugin declares the meaning and the
target spells it. A declaration in `store.go` through a `PerSource`
family with the word `stub` arrives in `store_stub.go` under the Go
lowering and `store.stub.ts` under the TypeScript one. A `PerPackage`
or `PerPlan` file has no stem: its word and its tag name it. Join
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

A target also reads the unit's package where its language spells a
package's files apart. Only a Go test file declares an external test
package, so Go spells every file of one as a test file: the
`PerPackage` stub of `store_test` is `stub_test.go`, beside the
`stub.go` of `store`.

Centralised filenames derive the same way, under
`<outdir>/<package-path>/`, where `<package-path>` is the source
package's workspace-relative directory. That stays unambiguous under
multi-module workspaces, where module-relative paths would collide
as soon as two modules both contain `internal/api`. Routing that
wants module identity uses the neutral `gen.module` facts the
frontend stamped
([08-workspace-and-plans.md](08-workspace-and-plans.md)).

## Package identity

A routed file declares a package. The target names it from the
file's path the way the target language's frontend names the package
of a file it reads. The kernel routes directories and hands the
target two facts of the load: the packages in the file's directory,
and the modules the neutral `gen.module` and `gen.moduleRoot` facts
declare ([08-workspace-and-plans.md](08-workspace-and-plans.md)).

| Target | The package of a routed file |
|---|---|
| Go | the package the directory's non-test files declare, and for a test file of an external test package's declarations that package. In a directory without one, the innermost containing module's path joined with the directory's path below its root. Outside every module, `importBase` joined with the path below the output directory |
| TypeScript | the file itself, a module named by its path without the extension |
| Java | the package its declarations derive from, because a Java file states its package in its clause |
| Rust | the module the directory's files belong to, joined with the file's stem. A `mod.rs` or a crate root names the directory's module, and a crate root is a file whose module is a crate the `gen.module` facts name |

A file written beside its source keeps its source's package. A Go
file in a `main` package declares `main`. A TypeScript or Rust file
is a module of its own.

An output directory outside every known module has no derivable
identity, and the plan's layout config then supplies `importBase`,
which Build validates.

Where neither returns, the lowering refuses, at the referencing
declaration, with a stable code, and only when a cross-reference
actually needs the qualification. Refusing at the point of need
costs nothing for a centralised file that nothing imports, and a
guessed import path is the one failure a compiler will not catch,
because a bare name binds silently to whatever else is in scope. A
target whose every file states its package refuses the file itself:
every Go file opens with a package clause, so the Go backend
withholds a Go file whose package derives none, under a positioned
Error.

## Failure semantics

Routing failures are diagnostics with stable codes
([16-diagnostics.md](16-diagnostics.md)), reported under the `layout`
phase and positioned at the emit declaration's origin or at the
carrier line of the directive that caused them:

| Code | Name | Meaning |
|---|---|---|
| EID-0048 | `UndeclaredFamily` | a routable declaration whose plugin does not declare its output |
| EID-0049 | `UnknownTag` | a `tag=` naming no declared output |
| EID-0050 | `AmbiguousOverride` | a filename override on a declaration its plugin emits into more than one output from, without a tag |
| EID-0051 | `NoDestination` | a declaration with no resolvable directory |
| EID-0052 | `EscapingPath` | an `out=` path that is absolute or leaves the workspace root |
| EID-0053 | `PathCollision` | two routed paths one tree cannot contain |
| EID-0054 | `UnderivedPackage` | a reference into a file whose package the target derives none for |

A refused declaration is left out, and the rest of its file renders.
A static contradiction in config, such as an override naming an
unknown plugin or tag, fails at workspace Build instead. A
configuration error should never wait for a run.

## Collisions

**Within a plan**, two declarations resolving to one path is legal
exactly when the backend can merge them, meaning the same package
and the same file, which is the ordinary case of many declarations
in one file. Declarations of two packages at one path, two paths that
differ only in case, and a path another path needs as a directory
are each a Layout-phase Error naming both, and the declarations of
both files are refused. The layout and every output sink apply one
rule, so a path the layout admits is a path every sink stages.

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
against the directory of the origin's source file. A trailing slash,
a final `.` and a final `..` name a directory and leave the filename
to the target. If a path is absolute, contains a backslash or escapes
the workspace root, eidos reports a diagnostic and writes nothing for
the declaration.
