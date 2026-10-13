---
rfc: 0044
title: The inputs of a route
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-12
updated: 2026-10-12
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0032
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0044: The inputs of a route

## Summary

The layout decides the family, the directory and the filename of each generated file from four
inputs. A run records each file with its group, and it does not record the input that decided the
path of the file. The config file sets the policy, the directory and the import base of a plan, and
it cannot refine one generator or one family. The kernel `out` directive applies to the declaration
that it is written on, so a package cannot state a directory for its declarations. We propose the
route of a unit, which contains the input that decided each part of its path. We also propose the
refinements of a generator and of a family in the config file, and the path of the kernel `out`
directive on a package as the lowest override of its declarations.

## Motivation

- A user who asks why a file is at its path reads the manifest entry, the contributors and the
  findings of the file. The three records do not contain the refinement, the directive or the policy
  that placed the file.
- `layout.Config` has refinements of a generator and of a family, and a composition sets them in
  Go. The routing document places them in the config of a plan. The layout of a plan in the config
  file has `policy`, `dir` and `importBase` alone.
- `plan` prints the policy, the directory and the import base of each plan. It leaves out the
  refinements that `Describe` returns.
- A package that collects its generated files in a subdirectory needs an `out` directive on each
  declaration. The layout reads the directives of the origin of a declaration alone.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-core/plugin` | `Route`, `RouteInput` and the field `Unit.Route` |
| `eidos-core/layout` | `Route` sets the route of each unit that it places. The path of the kernel `out` directive on a package is the lowest override. `Refinement.Over` merges two refinements |
| `eidos-core/internal/state` | the group record contains the route of each unit of each file, in the format 3 |
| `eidos-core/workspace` | `PlanConfig.Plugins` and `PlanConfig.Families`, the routes of a group, the read of the package of each origin, and `ExplainedFile.Routes` |
| `eidos-cli` | the layout of a plan in the config file has `generators`, `plan` prints the refinements, and `explain` prints the route of each unit of a file |
| `eidos-sdk` | the facade re-exports the new declarations |
| `docs/architecture` | the routing document, the workspace document and the CLI document |

### Invariants

1. The route of a unit has one input for each of the family, the policy, the directory and the
   filename of the unit. That input is the one that decided the part, in the precedence of the
   routing document.
2. A run records the route of every unit of every file that it writes. `explain` reads the route
   from the generation and runs nothing.
3. A refinement of the config file meets the refinement that the composition declares field by
   field. A field that the config sets replaces the declared field. Build validates the result as it
   validates a declared layout.
4. An override on a declaration takes precedence over the path on its package. A group reads the
   package of each origin of its units, so an edit of the directive on a package routes the groups
   of the package again.

### The route of a unit

```go
// Route records how the layout placed a unit. Each field is the input
// that decided one part of the path of the file of the unit. The zero
// Route is the route of a unit that the layout has not placed, such as a
// unit of the emit store.
type Route struct {
    // Family is the input that decided the family of the unit.
    Family RouteInput
    // Policy is the input that decided the policy of the directory. It is
    // zero for a per-plan family and for a path override, because no
    // policy applies to either.
    Policy RouteInput
    // Dir is the input that decided the directory of the file.
    Dir RouteInput
    // Name is the input that decided the filename.
    Name RouteInput
}

// RouteInput is one input of the layout. The zero value is no input.
type RouteInput uint8

const (
    // RouteDefault is the default policy, alongside-source.
    RouteDefault RouteInput = 1
    // RouteHandler is the family that the handler of the plugin chose.
    RouteHandler RouteInput = 2
    // RouteSource is the directory of the source of the unit.
    RouteSource RouteInput = 3
    // RouteTarget is the filename that the target spells.
    RouteTarget RouteInput = 4
    // RoutePlan is the layout of the plan.
    RoutePlan RouteInput = 5
    // RouteGenerator is the refinement of the generator of the unit.
    RouteGenerator RouteInput = 6
    // RouteFamily is the refinement of the family of the unit.
    RouteFamily RouteInput = 7
    // RoutePackage is the path of the kernel out directive on the package
    // of the origin.
    RoutePackage RouteInput = 8
    // RouteOut is the kernel out directive on the origin.
    RouteOut RouteInput = 9
    // RouteDirective is the key out= or tag= on a directive of the plugin.
    RouteDirective RouteInput = 10
)
```

`RouteInput.String` returns the name of an input in lower case, such as `generator`, and
`package-out` for `RoutePackage`. `RouteInput.Valid` reports whether an input is one of the
constants.

Each part of a route takes the first input of its row that states a value:

| Part | Inputs, in precedence order |
|---|---|
| Family | `RouteDirective`, `RouteOut`, `RouteHandler` |
| Policy | `RouteFamily`, `RouteGenerator`, `RoutePlan`, `RouteDefault` |
| Dir | `RouteDirective`, `RouteOut`, `RoutePackage`. Then `RouteFamily`, `RouteGenerator` and `RoutePlan` for a per-plan or a centralised family, and `RouteSource` for a family beside its source |
| Name | `RouteDirective`, `RouteOut` and `RoutePackage` where the path names a file. Then `RouteFamily`, `RouteGenerator` and `RouteTarget` |

The directory of a centralised family joins the configured directory and the source directory. Its
`Dir` is the input of the configured directory, and its `Policy` is the input of the policy.

The layout sets the route of each unit that it returns. A target can split one unit into more than
one, and each part takes the route of the unit. The units of one file can have different routes,
because the layout writes the declarations of two destinations into one file where their paths
agree. The declarations of one unit can also arrive at one destination through two inputs that agree
on the path, such as a path override to the directory of the policy. The route of such a unit has
the inputs of its first declaration, and the layout writes the unit as it does now.

### The routes of a group

```go
// FileRoute is the route of one unit of one file of a group.
type FileRoute struct {
    File  string
    Unit  plugin.UnitRef
    Route plugin.Route
}
```

`state.Group` gains `Routes []FileRoute`, sorted by file and then by unit. The entry of a group
writes the routes after its findings, each route as its file, its unit and four bytes. The format
of a generation becomes 3. A generation of the format 2 does not open, so the first run after the
change runs cold.

`ExplainedFile` gains the routes of the units of the file:

```go
// ExplainedRoute is the route of one unit of an explained file.
type ExplainedRoute struct {
    Unit  plugin.UnitRef
    Route plugin.Route
}
```

`explain` prints one line for each unit of a file under the line of the file:

```
file mocks/svc/store_mock.go: plan server, plugins mockgen
  route plugin mockgen, key svc/store.go: family handler, policy plan, dir generator, name target
  contributor mockgen rule 0 golang:svc.Store
  source golang:svc.Store
```

The JSON event `explain.file` gains `routes`, a list of objects with the fields `plugin`, `tag`,
`package`, `key`, `family`, `policy`, `dir` and `name`. The value of each of the last four is the
name of an input, and a part without an input has no field.

### The path on a package

The path of the kernel `out` directive on a package applies to each declaration of the package.
The layout reads it after the overrides of the origin: the path of an override on the declaration
takes precedence, and the package fills the path where the overrides of the declaration leave it
empty. The path resolves against the directory of the source file of the origin, as a path on the
declaration does. A finding about the path is positioned at the directive on the package.

The tag of the directive on a package does not apply to its declarations. A tag selects a family of
each plugin, so a tag on a package would move the declarations of every plugin of the plan. Each
plugin without the family would report `UnknownTag` at each declaration of the package.

The layout enumerates the packages with the directive once per plan through `Index.ByDirective`.
A plan without such a package does not look up a package for a declaration. A group records the
declaration edge of the package of each origin of its units. An edit of the directive on a package
makes that edge dirty, so a warm run routes the group again.

### The refinements of the config file

```yaml
plans:
  server:
    layout:
      policy: centralised
      dir: gen
      generators:
        mockgen:
          dir: mocks
          families:
            test:
              file: mocks_test.go
```

The layout of a plan in the config file gains `generators`. Each key is the name of a generator of
the plan, and each value refines the routing of the generator and of its families. The key is not
`plugins`, because `plugins` of a plan lists the directories of declarative plugins, and a word has
one meaning in one file:

```go
// GeneratorLayout is the refinement of the routing of one generator and
// of its families. Each field that the file sets replaces the field that
// the composition declares.
type GeneratorLayout struct {
    Policy   Policy                  `yaml:"policy"`
    Dir      string                  `yaml:"dir"`
    File     string                  `yaml:"file"`
    Families map[string]FamilyLayout `yaml:"families"`
}

// FamilyLayout is the refinement of the routing of one family of a
// generator.
type FamilyLayout struct {
    Policy Policy `yaml:"policy"`
    Dir    string `yaml:"dir"`
    File   string `yaml:"file"`
}
```

The key of a family is its tag, and the key `""` names the primary family. The JSON Schema of the
config file gains the keys from their tags.

`workspace.PlanConfig` gains the refinements, and Build merges each one over the refinement that the
plan declares for the same generator or family:

```go
// Plugins and Families refine the routing of the generators and the
// families of the plan. Build merges each refinement over the
// refinement that the plan declares for the same generator or family,
// through [layout.Refinement.Over].
Plugins  map[plugin.ID]layout.Refinement
Families map[layout.Family]layout.Refinement
```

```go
// Over returns base with each field that r sets: the policy where it is
// not PolicyInherit, and the directory and the filename where they are
// not empty. It allocates nothing.
func (r Refinement) Over(base Refinement) Refinement
```

The merge copies the maps of the plan, so the plan that the composition declares does not change.
A refinement cannot clear a field that the composition sets, because the zero value of each field
inherits. Build validates the merged layout through `Config.Check`, so a refinement of a generator
outside the plan or of a family that the generator does not declare is a fault of Build.

### The refinements in plan

`plan` prints one line for each refinement of a generator and for each refinement of a family, after
the layout of the plan:

```
plan server: target golang, backend golang, commit order 1
  sources: every package
  generator 1 mockgen
  layout: policy centralised, dir gen
  refine generator mockgen: dir mocks
  refine family "test" of mockgen: file mocks_test.go
```

The JSON event `plan` gains `layout.generators` in the form of the config file.

### Failures

| Condition | Report |
|---|---|
| A refinement of the config names a generator outside the plan | a fault of Build from `Config.Check`, as for a declared refinement |
| A refinement of the config names a family that its generator does not declare | a fault of Build from `Config.Check` |
| A refinement of the config sets a directory that no family of its scope reads | a fault of Build from `Config.Check` |
| The path on a package is absolute or leaves the workspace root | `EscapingPath` at the directive on the package, for each declaration of the package |
| The path on a package names one file, and a plugin emits into two families from one origin of the package | `AmbiguousOverride` at the directive on the package |
| A generation of the format 2 | the run runs cold |

### Migration

1. Add `Route` and `RouteInput`, and set the route of each unit in the layout.
2. Read the path of the directive on a package, and record the read of the package in the group.
3. Write the routes into the group record in the format 3, and add them to the explanation and to
   `explain`.
4. Add the refinements to `PlanConfig`, to the config file and to its schema, and print them in
   `plan`.
5. State the route, the path on a package and the refinements in the documents.

## Alternatives

### The route in the manifest

The manifest entry of a file could contain the route. The sweep and the drift check compare the
entries of two runs, and a route does not change the bytes of a file. A config that moves a
directory from the plan to the generator would then change the entry of a file whose bytes and path
stay the same. The group record already contains what produced the file: its units, its contributors
and its findings.

### One route for each file

A file could record one route. The layout writes the declarations of two destinations into one file
where their paths agree, and two inputs can decide the two paths. One route for the file would name
one of the two.

### A package refinement in the config file

The config file could refine the routing of a package. The routing document states that a
refinement has no package, because the package of a path is a fact of the target language. An
author who writes the directive on the package states the directory in the language of the package.

### The tag on a package

The tag of the directive on a package could move the declarations of the package into a companion
family. A tag selects a family of one plugin, and the directive on a package applies to every plugin
of the plan. Each plugin without that family would report a finding at each declaration.

### A marker for the values of the config file in plan

`plan` could mark each value of a layout that the config file sets. Build merges the config before
the composition compiles, and `plan` prints the values that a run applies. The config file is the
input that the user edits, so the user reads the source of a value there.

## Drawbacks

- The format of the sealed state changes, so the first run after the change runs cold.
- `plugin.Unit` gains a field that the layout alone sets.
- A group record grows by one route for each unit of each file: the path of the file, the reference
  of the unit and four bytes.
- A group reads the package of each origin of its units, so an edit of any directive of a package
  routes each group of the package again.

## Open questions

None.

## Unresolved and future work

None.

## References

- The routing of a declaration and the precedence of its inputs: RFC-0016.
- The config file and the commands `plan` and `explain`: RFC-0021.
- The sealed state and its generations: RFC-0020.
