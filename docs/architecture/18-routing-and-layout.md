<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

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
package of a path is a fact of the target language. A composition
declares refinements in Go, and the config file refines them under the
key `generators` of the layout of a plan. Each field that the file sets
replaces the same field of the declared refinement.

**4. Per-declaration overrides**, meaning the kernel's `out`
directive, or the reserved `out=` and `tag=` keys on a plugin's own
directive. Both spellings lower to the same override
([05-directives.md](05-directives.md)). The reserved keys route the
output of the plugin that registered the directive's schema, and the
kernel directive routes every plugin's output. `out=<path>`
redirects, and `tag=<name>` moves a declaration of the primary
output into a companion. What the source author writes takes
precedence over configuration, as the authority model ranks a
directive over config. The path of the kernel `out` directive on a
package applies to each declaration of the package without a path of
its own. Its tag does not apply to the declarations. A tag selects a
family of one plugin, and the directive on a package applies to every
plugin of the plan.

A declaration takes the overrides of its origin. Where the origin
states none, the declaration takes the overrides of the subject whose
match placed it, which `plugin.Unit.Subjects` records. The double of a
handle that the handler of an interface writes then arrives beside the
double of the interface. The subject is the origin of the routing in
every step of the resolution pass.

Validation reads the same overrides from the raw directives of a subject
and of its package, through `layout.RouteDir`, because the layout runs
after validation. The scope in which a directive param resolves has the
directory of the route of the output of the plugin of the directive
([03-projection.md](03-projection.md)).

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
3. **Directory.** An author's path, on the declaration or on its
   package, resolves against the directory of the origin's source file.
   Otherwise the policy decides: the source
   directory beside the source, the source directory under the output
   directory when centralised, and the configured directory for a
   `PerPlan` output.
4. **Spelling.** Derive the filename through the target's lowering,
   from the whole family declaration: cardinality, word and tag. An
   author's path or the output's configuration that names a file
   takes precedence.
5. **Package.** The target names the package each file declares from
   the file's path, by its language's rule, and the namespace of the
   file-level names of the file.
6. **Record.** Declarations with one path form one file, unless the
   collision rules refuse it. The plan's render pass renders the files
   ([07-rendering.md](07-rendering.md)), so routing is settled before
   any template runs.
7. **References.** A reference between two declarations of the plan
   that are written in files of two packages takes the package of the
   file it names, so the target qualifies it and records the import.
   The file lists the package of each such file once in
   `plugin.File.Referents`, sorted by package path. A target whose
   package clause does not follow from the package path, such as
   protobuf, reads the clause of the other file from the name of that
   package. The layout finds the referent of three kinds of reference:
   - A bare reference has neither a target nor a package. Its referent
     is the declaration of the package of its unit with its settled
     name. The layout sets `emit.TypeRef.Emitted` on a bare reference
     that it qualifies into another package.
   - A translated reference has a target in another language than the
     plan. Its referent is the declaration of the plan whose origin is
     the target and whose settled name is the spelling.
   - An emitted reference has a target and has `emit.TypeRef.Emitted`
     set. A generator writes it for a declaration that another generator
     emits, such as a fixture in a test package. Its referent is found
     as the referent of a translated reference is, in any language.

   The referent of a translated or an emitted reference can be a record
   of a family that claims its files, in a file that the run routes or
   in a file that it keeps. The referent of a bare reference is never a
   record.

   A backend imports a reference with the mark from the package that the
   layout sets. The TypeScript backend writes that module path relative
   to the file, such as `./thing.maker`. The layout refuses the
   referencing declaration under `UnemittedReference` where the plan
   does not emit the referent of an emitted reference.
8. **Imports.** A backend whose target restricts the references of a file
   implements the role `plugin.ImportChecker`. The layout calls it for
   each reference of each routed declaration, with the package of the
   file, after step 7 qualified the reference. A Go file imports no
   internal package of another tree and no external test package, and
   refers to no unexported name of another package. The layout refuses a
   declaration with a reference that its file cannot refer to under
   `UnimportableReference`, and the file keeps its other declarations.
9. **Names.** Each settled file-level name occurs once in the namespace
   of its file and its receiver, and a record does not collide with a
   name. The layout reports the declarations of one name as one
   `CollidingNames` Error, positioned at the second declaration with the
   others as related. The check refuses no declaration, and the Error
   fails the plan. A warm run compares the
   names of the files that it routes with the names of the files that it
   keeps. The backend of a target whose language overloads functions,
   such as Python, implements the role `plugin.Overloader`. The functions
   of one emitted name are then the overloads of one function, and the
   backend checks them.

## The route of a file

The layout records how it placed each unit as the route of the unit:
the input that decided its family, the policy of its directory, its
directory and its filename. The inputs are the family that the handler
chose, the default policy, the source directory, the spelling of the
target, the layout of the plan, the refinement of the generator, the
refinement of the family, the path on the package, the kernel `out`
directive and the reserved keys of the plugin's own directive. A part
to which no input applies has none, such as the policy of a per-plan
family.

The group record of a run contains the route of each unit of each of its
files, and `explain` prints the routes of a file
([20-cli.md](20-cli.md)). The units of one file can have different
routes, because the declarations of two destinations whose paths agree
form one file. A unit whose declarations arrive at one destination
through two inputs has the inputs of its first declaration.

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
| Go | the package the directory's non-test files declare, and for a test file of an external test package's declarations that package. For a test file of a family that states `External`, the external test package of the directory: the one that a file of the directory declares, or else the package of the directory with `_test` after its path and its name. The declarations whose origins a unit lists in `Unit.Inside` go into the internal test file of the family, as `store_check_internal_test.go`, which declares the package of the directory. In a directory without one, the innermost containing module's path joined with the directory's path below its root. Outside every module, `importBase` joined with the path below the output directory |
| TypeScript | the file itself, a module named by its path without the extension |
| Java | the package its declarations derive from, because a Java file states its package in its clause |
| Rust | the module the directory's files belong to, joined with the file's stem. A `mod.rs` or a crate root names the directory's module, and a crate root is a file whose module is a crate the `gen.module` facts name |
| protobuf | the file itself, named by its import path, because a protobuf file imports a file. The name of the package is the protobuf package that the file declares |
| GraphQL | one package for every file of the plan, because a schema has one namespace |

A routed unit keeps the origins of `Unit.Inside` that its own
declarations derive from. The layout passes `Placement.External` as
true where the family of the first unit of a file states `External` and
no unit of the file has such an origin. The Go target splits a unit with
such an origin through `spell.Split`, so the declarations that test from
inside take a file of their own.

The names of a file are in the namespace of its package, except where
the target names another namespace through `plugin.NameScoper`. The
files of one protobuf package declare their names in one namespace, so
the protobuf backend returns the protobuf package as the namespace.
`plugin.File.Scope` records the namespace of a file.

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

A reference to code that a gRPC binding generates is bound
([10-cross-language.md](10-cross-language.md)). The plan does not emit
its referent, so the package comes from the `plugin.Binder` of the
backend. Its `Home` takes the target of the reference and the
placement of its source file at the path where the default placement
of the plan writes the code of that file. The layout qualifies the
reference where that package differs from the package of the
referencing file, and records the package among the referents of the
file. The layout refuses a bound reference under `UnderivedPackage`
where `Home` returns an error or the target has no source file in the
workspace. A plan whose backend has no binder refuses it under
`UntranslatedReference`.

## The source file of a routed file

A routed file whose every unit derives from one source file has the
identity and the annotations of that source file. The layout sets them
where each unit of the file is a per-source unit with one key. A file
of a package, of a plan or of two or more source files has neither.
The render passes the annotations to the file skeleton. The Go
skeleton writes the build constraint of the source above the package
clause, so a file that a generator writes beside a constrained source
compiles under the same constraint. A group of a warm run reads the
declaration edge of the source file, so an edit of the constraint
writes the file again ([09-incrementality.md](09-incrementality.md)).

## The header of a routed file

A unit can contain an `emit.File`, whose documentation and annotations
are the file-level statement of a generator. A protobuf generator
states the syntax, the package, the imports and the options of a file
this way, and a GraphQL generator states the schema statement of a
file. `plugin.HeaderOf` merges the `emit.File` declarations of the
units of a routed file into `plugin.File.Header`, in unit order. An
annotation that an earlier `emit.File` states alike appears once, and
so does a documentation. The path of an `emit.File` is not part of the
header. The target's package half reads the annotations of the header
in `plugin.Placement.Header`, so a package rule can read the package
that a header states. The render passes the header to the skeleton and
to every template of the file ([07-rendering.md](07-rendering.md)).

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
| EID-0054 | `UnderivedPackage` | a reference into a file whose package the target derives none for, or a bound reference whose binder derives no package |
| EID-0070 | `UntranslatedReference` | a translated reference to a declaration that the plan does not emit, or a bound reference in a plan whose backend has no binder |
| EID-0081 | `UnemittedReference` | an emitted reference to a declaration that the plan does not emit |
| EID-0082 | `UnimportableReference` | a reference that the file of the referencing declaration cannot import under the import rule of the target |
| EID-0031 | `CollidingNames` | two file-level declarations of one namespace and receiver with one settled name |

A refused declaration is left out, and the rest of its file renders.
`CollidingNames` refuses no declaration.
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
