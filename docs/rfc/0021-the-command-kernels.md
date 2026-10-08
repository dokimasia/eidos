---
rfc: 0021
title: The command kernels, the config file and the run lock
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Draft
created: 2026-10-08
updated: 2026-10-08
discussion: none
supersedes: none
superseded-by: none
produces-adr: tbd
---

# RFC-0021: The command kernels, the config file and the run lock

## Summary

Every consumer of eidos builds its own binary on the kernel, and today it
also writes that binary's whole command line: the flags, the config file,
the rendering of findings and the exit status. Two consumers' binaries
differ in each of these, although they run the same engine.

This proposal adds the module `go.dokimi.dev/eidos/cli`. Its function
`Kernels` takes a function that composes the workspace, and returns seven
commands: `run`, `plan`, `explain`, `prune`, `doctor`, `watch` and
`version`. Each command parses its own flags and arguments. It finds the
brand's config file and reads it as YAML. It renders findings as text or as
line-delimited JSON under a versioned schema, and returns 0, 1 or 64.

A command depends on no command-line framework, so a consumer mounts the
seven beside its own commands in the command line that its binary already
has, such as a cobra command tree or a dispatcher over the standard library.
A binary without a command line of its own calls `Main`, which mounts the
seven and the consumer's commands, and exits with the status that the
command returns. The module also contains `acceptancetest`, which builds a
consumer's binary and drives it as a process.

The commands need mechanisms that the kernel lacks, and the proposal adds
them to the kernel:

- an exclusive lock on the state directory, which every run takes
- diagnostic suppression through the kernel's `diag` directive, with counts
  per code and a finding for each suppression that removed nothing
- run inputs that select plans, narrow the commit to path patterns,
  overwrite drifted files, adopt foreign files, promote warnings, check
  that the committed output is current, and prune stale files
- a description of the composition, and provenance queries over the sealed
  state
- plan refinements that a config file states, and deprecation markers on
  directive schemas

## Motivation

### A consumer writes its own command line

`workspace.Run` takes an `Input` and returns a `Report` and an error. The
kernel has no code that turns arguments into an `Input`, a config file into
a `Builder`, or a `Report` into output and an exit status. Each consumer
writes that layer, and the layers diverge:

- The flag spellings and the config file differ from binary to binary.
- One binary renders findings as text and another as JSON.
- A failed run and a mistyped flag can exit with one status, so a script
  has to read standard error to separate them.

The specification requires every eidos-built binary to offer the same
commands, flags, output formats and exit codes. The binary's author writes
no code for them.

A consumer's binary has commands of its own beside generation, such as an
installer or a migration, and its command line can use a framework such as
cobra. The eidos commands have to mount in that command line beside the
consumer's own, and behave the same in every binary.

### Missing kernel mechanisms

- **No lock.** Two runs over one workspace stage and commit independently.
  The generation that `CURRENT` names last is the live one. One run's
  removal of unreferenced segments can delete a segment that the other
  run's generation references.
- **No suppression.** The kernel registers the `diag` directive's schema
  with its `off` parameter. No part of a run reads the directive.
- **A narrow input.** `Input` has a tree or a graph, the stores, `Dry` and
  `Cold`. A run cannot select plans, narrow its commit, overwrite a drifted
  file, adopt a foreign one, promote warnings, or check that the committed
  output is current.
- **No description and no provenance query.** No API lists the
  composition's schedule. The kernel has no query that reads the sealed
  state's records back for a person.
- **No deprecation marker.** A directive schema has no field that marks it
  deprecated, so a run cannot report a deprecated use.

### Failure modes a command line has to rule out

A command line built without a contract fails in these ways:

- A consumer that composes its command line by copying a `main` function
  keeps the copy, and the copies drift apart.
- JSON written to the stream that also receives plain-text errors does not
  parse.
- A check that writes state, or that skips stale files, passes while the
  committed output is out of date.
- State anchored to the working directory, while config discovery walks to
  the filesystem root, splits one workspace's records across directories.
- `flag.ExitOnError` exits with status 2 on a bad flag. A Go panic exits
  with status 2 too, so a typo and a crash exit alike.
- A framework that prints a command's error, or exits 1 for every error,
  turns a usage error's 64 into a 1, and writes text into a JSON stream.

The design that follows has a rule against each of them.

## Detailed design

### Components

| Component | Package | Responsibility |
|---|---|---|
| `Command`, `Kernels` | `cli` | The seven commands as values that any command line mounts, with their shared flags, their rendering and their exit statuses |
| `Main` | `cli` | The command line of a binary without a framework of its own |
| `ExitError`, `Exit`, `UsageError` | `cli` | The exit status for a host whose commands return errors, and the mark of a usage error |
| `Open`, `Begin`, `Renderer` | `cli` | The workspaces and the rendering for a consumer's own command |
| The config file | `cli` | Discovery, the YAML document, workspace lists, and the mapping onto `workspace.Builder` |
| The seven commands | `cli` | `run`, `plan`, `explain`, `prune`, `doctor`, `watch` and `version` |
| `acceptancetest` | `cli/acceptancetest` | Builds a consumer's binary and drives it as a process |
| `Locker`, `Holder` | `core/ledger` | The state directory's exclusive lock and the record of its holder |
| `Overwriter` | `core/output` | Lets a run overwrite a drifted or a foreign file |
| The run inputs | `core/workspace` | Plan selection, narrowing, drift, adoption, strict mode, the check and the prune |
| Suppression | `core/diag`, `core/workspace` | The sinks' filter, the run's table, the counts per code and the unused suppressions |
| `Describe` | `core/workspace` | The composition as data, for `plan` and `version` |
| `Explain` | `core/workspace` | Provenance queries over the live generation |
| `PlanConfig` | `core/workspace` | The plan refinements that a config file states |
| `BrandName` | `core/workspace` | The composition's brand before Build, which discovery needs |
| `Deprecated` | `core/directive` | The deprecation marker on a schema and on a parameter |

### The invariants

1. **A command behaves the same under every host.** A command parses its
   own flags and arguments, renders its own output and returns its own
   status. A host selects the command by name, passes it the arguments that
   follow the name, and exits with the status.
2. **Each exit status means one thing.** A command returns 0 for success, 1
   when its findings or errors failed it, and 64 for a usage or config
   error. No eidos code returns or exits with 2, so a 2 is a Go panic.
3. **JSON output parses.** Under `--format=json`, standard output contains
   JSON lines alone. The first line is the `start` event, which states the
   schema's version, and the last line is the `summary` event. Standard
   error is empty unless the process panics.
4. **The invocation never changes a file's bytes.** Flags and patterns
   decide which files a run commits, never what a file contains. `run
   ./svc/...` and `run` write the same bytes to every file that both write.
5. **One process works in a state directory at a time.** A run and
   `explain` take the state directory's exclusive lock and keep it while
   they read or write the directory. A second one fails at once and names
   the holder.
6. **No hidden state.** A command reads the config, the tree, the stores
   that the composition's frontends locate and the brand's state directory,
   and writes to the tree and the state directory alone.
7. **A command that writes nothing writes nothing.** `run --dry-run`, `run
   --check`, `plan`, `explain`, `doctor` and `version` leave the tree and
   the state directory as they found them, the lock file aside.

### The module

The command kernels are a module of their own, `eidos-cli`, with the import
path `go.dokimi.dev/eidos/cli`:

```
eidos-cli/
    command.go, run.go, ...   package cli: Command, Kernels, Main, Open and the renderer
    config.schema.json        the JSON Schema of the config file
    acceptancetest/           the kit that drives a consumer's binary
    internal/config/          decodes the config file and builds its JSON Schema
```

The module requires `go.dokimi.dev/eidos/core` and `go.yaml.in/yaml/v3`
v3.0.5, and no command-line framework. `acceptancetest` also requires
`go.dokimi.dev/assert`, as every conformance kit does.

The config file is YAML, the standard library reads no YAML, and the
kernel's runtime takes no third-party dependency. A satellite or a plugin
module requires the kernel and never the command line, so a module of its
own keeps the YAML dependency out of their build lists. `go.yaml.in/yaml/v3`
requires no module, so the command line's dependency tree grows by one
module. `eidos-lang` keeps the tree-sitter cgo dependency out of the kernel
in the same way.

### Composition

`Kernels` takes a function that composes the workspace, because a command
applies the config file to the builder before Build runs and builds a
fresh composition for each member of a list. It returns the seven commands
over that function:

```go
// Compose returns a fresh composition: the brand, the frontends, the
// plugins and the plans that the binary compiles in. A command calls it
// once for each workspace that one call of Run covers, because a plugin
// instance belongs to one workspace.
type Compose func() *workspace.Builder

// Command is one command of a binary: a kernel command, or a consumer's
// command that Main mounts. A host selects a command by its name, passes
// it the arguments that follow the name, and exits with the status that
// Run returns.
type Command interface {
    // Name is the command's name on the command line.
    Name() string
    // Synopsis is the one line that a list of commands prints beside the
    // name.
    Synopsis() string
    // Usage is the command's help text. Its first line is the command's
    // form, such as "run [flags] [<pattern>...]", and each flag follows
    // on a line of its own.
    Usage() string
    // Run parses the command's flags and arguments from args, runs the
    // command, renders its output to stdio, and returns StatusOK,
    // StatusFailed or StatusUsage. A -h or --help in args prints Usage on
    // stdio.Stdout and returns StatusOK.
    Run(ctx context.Context, stdio IO, args []string) int
}

// The statuses that a command returns.
const (
    // StatusOK reports a command that succeeded.
    StatusOK = 0
    // StatusFailed reports a command whose findings or errors failed it.
    StatusFailed = 1
    // StatusUsage reports a usage error or a config error.
    StatusUsage = 64
)

// IO is what a command reads from its process: the standard streams, the
// working directory and the environment. Run panics on an IO without a
// stream or Getenv, or with a relative Dir.
type IO struct {
    // Stdout and Stderr receive the output.
    Stdout, Stderr io.Writer
    // Dir is the absolute working directory.
    Dir string
    // Getenv returns an environment variable, and the empty string for
    // one that is unset.
    Getenv func(key string) string
    // Terminal reports whether Stderr is a terminal, which lets text
    // output colour the severities.
    Terminal bool
}

// ProcessIO returns the IO of the running process: os.Stdout, os.Stderr,
// the working directory, os.Getenv, and whether os.Stderr is a character
// device.
//
// Error modes: the error of os.Getwd.
func ProcessIO() (IO, error)

// Kernels returns the seven kernel commands over compose, in the order
// run, plan, explain, prune, doctor, watch and version. Each call returns
// new values. A command's Run is safe for concurrent use where compose
// is.
func Kernels(compose Compose) []Command

// Main is the command line of a binary without a framework of its own. It
// runs the command that os.Args selects among the kernel commands and the
// extra ones, with ProcessIO, under a context that the first interrupt or
// termination signal cancels. It exits the process with the status that
// the command returns, and never returns.
//
// Main panics when an extra command's name is empty, is the name of a
// kernel command, or is the name of another extra command.
func Main(compose Compose, extra ...Command)

// ExitError is a status that a command returned and rendered already. A
// host whose commands return errors exits with Code and prints nothing.
type ExitError struct {
    Code int
}

// Error returns the empty string, because the command rendered its
// failure.
func (e ExitError) Error() string

// ExitCode returns Code.
func (e ExitError) ExitCode() int

// Exit returns nil for StatusOK, and an ExitError with status otherwise.
func Exit(status int) error
```

`ExitError` implements urfave/cli's `ExitCoder` interface, an error with
an `ExitCode` method. urfave/cli prints the message of an `ExitCoder` only
where the message is not empty, so a urfave/cli host exits with the status
and prints nothing.

#### A consumer's own command

A consumer's command that reads the workspaces calls `Open` and renders
through `Begin`, in any framework:

```go
// Flags are the flags that every kernel command accepts.
type Flags struct {
    Format  Format
    Config  string
    Strict  bool
    Verbose bool
    NoColor bool
}

// Register defines the shared flags on fs, each bound to its field of f.
func (f *Flags) Register(fs *flag.FlagSet)

// Format is the output format that --format selects.
type Format uint8

const (
    // FormatText renders for a person. It is the default.
    FormatText Format = 1
    // FormatJSON renders line-delimited JSON under the versioned schema.
    FormatJSON Format = 2
)

// Open finds the config file for stdio.Dir, or reads f.Config where it is
// not empty. It applies the file to the compositions that compose returns, and
// builds one member for each workspace that the file covers: one, or the
// entries of a workspaces list in list order.
//
// Error modes: every error is a config error, which a command reports with
// StatusUsage. A file that does not read or decode, a key or a value that
// does not validate, a Build fault and two roots of a list that nest are
// config errors.
func Open(stdio IO, f Flags, compose Compose) ([]Member, error)

// Member is one workspace that Open built.
type Member struct {
    // Name is the root of a member of a list, as the list writes it, such
    // as tools/gen. It is empty for a workspace outside a list.
    Name string
    // Root is the workspace root, absolute.
    Root string
    // Config is the config file's path, and empty where none was found.
    Config string
    // Workspace is the composition, configured and built.
    Workspace *workspace.Workspace
}

// Begin starts the output of the command named command, and returns its
// renderer. Under FormatJSON it writes the start event, with the brand
// that compose declares.
func Begin(stdio IO, f Flags, compose Compose, command string) *Renderer

// Renderer writes one command's output in the format that the Flags given
// to Begin select. Text goes to standard output, and findings to standard
// error. JSON goes to standard output, one event per line. Text has colour
// where the IO given to Begin is a terminal, and neither --no-color nor
// NO_COLOR is set.
type Renderer struct{ /* unexported */ }

// Report renders one finding: a text line on standard error, or a diag
// event. An Info renders under --verbose alone.
func (r *Renderer) Report(d diag.Diag)

// Event renders one event of a consumer's command: the JSON object of v
// under the event name, or text, the line that text output prints.
func (r *Renderer) Event(name string, v any, text string)

// Workspace starts the output of the member m of a list. The events that
// follow have the name of m in their workspace field, and the summary
// counts the findings of m on their own.
func (r *Renderer) Workspace(m Member)

// Error renders each line of the text of a failure without a position as
// a text line on standard error, or as an error event.
func (r *Renderer) Error(err error)

// UsageError is an error in the invocation of a command, such as an
// unknown flag. Error renders it as an error event with usage set.
type UsageError struct {
    Err error
}

// End ends the output. Under FormatJSON it writes the summary event, with
// status and the findings that Report counted.
func (r *Renderer) End(status int)
```

`Open` and `Begin` read the composition's brand before Build runs, for
discovery's `.<brand>.yaml` and for the `start` event. `workspace.Builder`
gains the method that reads it:

```go
// BrandName returns the brand that Brand declared, and the empty Brand
// before Brand is called. It allocates nothing.
func (b *Builder) BrandName() output.Brand
```

### Mounting the commands

A host is the command line that selects a command: `Main`, or the
consumer's own. A binary without a command line of its own calls `Main`
with its own commands beside the seven. The plugin constructors in this
sketch are illustrative:

```go
func main() {
    cli.Main(compose, installCommand{})
}

func compose() *workspace.Builder {
    return workspace.New().
        Brand("acme").
        Frontends(gofrontend.New()).
        Annotators(shapefull.Annotators()...).
        Plans(golang.ServerPlan(mygen.New()))
}
```

A dispatcher over the standard library looks the command up by `Name` in
`Kernels(compose)`. It calls `Run` with `ProcessIO` and the arguments that
follow the name, and passes the status to `os.Exit`.

A cobra host turns each command into a cobra command with flag parsing
disabled, and exits with the status of an `ExitError`:

```go
func main() {
    pio, err := cli.ProcessIO()
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(cli.StatusFailed)
    }
    root := &cobra.Command{Use: "acme"}
    root.AddCommand(installCmd) // a command of the binary's own
    for _, k := range cli.Kernels(compose) {
        root.AddCommand(&cobra.Command{
            Use:                k.Name(),
            Short:              k.Synopsis(),
            Long:               k.Usage(),
            DisableFlagParsing: true,
            SilenceErrors:      true,
            SilenceUsage:       true,
            RunE: func(c *cobra.Command, args []string) error {
                kio := pio
                kio.Stdout, kio.Stderr = c.OutOrStdout(), c.ErrOrStderr()
                return cli.Exit(k.Run(c.Context(), kio, args))
            },
        })
    }
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    err = root.ExecuteContext(ctx)
    stop()
    if exit, ok := errors.AsType[cli.ExitError](err); ok {
        os.Exit(exit.Code)
    }
    if err != nil {
        os.Exit(cli.StatusFailed)
    }
}
```

`DisableFlagParsing` passes every argument to `RunE` unparsed, `-h`
included, so the command prints its own help and reports its own usage
errors. `SilenceErrors` and `SilenceUsage` stop cobra from printing the
empty error and the command's usage. Mounting the commands on a group
command instead of the root gives `acme gen run`. A urfave/cli v3 host sets
`SkipFlagParsing` on each command, and its `Action` returns `cli.Exit` of
the status.

Measured with cobra v1.10.2 and urfave/cli v3.14.0 on Go 1.27.1, a probe
command mounted this way received `-h` and every flag as arguments, and an `ExitError` with code 64 exited 64 with standard error
empty. cobra also passed every flag written before the group's name to the
command unparsed. That includes the host's own persistent flags, and the
command reports each of them as a usage error.

#### The host's obligations

| The host | What breaks without it | What detects it |
|---|---|---|
| Passes the arguments that follow the command's name to `Run`, and parses none of them | The host's parser rejects the command's flags, or consumes them | `acceptancetest`'s status checks |
| Exits with the status that `Run` returns, and prints nothing for an `ExitError` | A 64 becomes a 1, and an error line enters standard error under `--format=json` | `acceptancetest`'s status and JSON checks |
| Writes nothing to the standard streams around a command, such as a banner or an update notice | The JSON stream does not parse | `acceptancetest`'s JSON check |
| Mounts each kernel command under its kernel name | A script written for `run` fails against the binary | `acceptancetest`, under the fixture's `Prefix` |
| Cancels the context on an interrupt | An interrupt kills the process without the `error` event that reports the cancellation | Nothing. It is a property of the host's `main` |

A host decides its own dispatch: an unknown command, its own help, and two
commands of one name. Measured with cobra v1.10.2, an unknown command under
a group printed the group's help on standard output and exited 0. Under
`Main`, a consumer's command that reuses a kernel name, or two commands
that share a name, are a defect in the binary. `Main` panics on it before
it reads any argument, as Build panics on a declaration defect, so the
binary's first test finds it.

### Command lines and flags

A kernel command reads the arguments that follow its name, in the form
`[flags] [arguments]`:

- Flags can appear before, between and after the arguments. `--` ends the
  flags, and every argument after it is positional.
- `-h` and `--help` print the command's usage on standard output, and the
  command returns 0.
- An unknown flag, a malformed value and a failed check of the command's
  own arguments return 64, with the error rendered.

`Main` reads a command line of the form `<binary> [flags] <command> [flags]
[arguments]`. It parses the flags before the command's name against the
shared flags, and passes them to the command ahead of the arguments that
follow the name. `<binary> help` and a `-h` or `--help` before the name
print the list of commands, and `<binary> help <command>` prints that
command's usage. Both exit 0. An unknown command exits 64.

Each command parses with one `flag.FlagSet` under `flag.ContinueOnError`,
whose own output is discarded. The shared flags are defined on every set. A
loop parses, moves the next argument to the positional list, and parses the
rest again, because `FlagSet.Parse` stops at the first argument that is not
a flag. `flag.ExitOnError` is never used, because it exits with status 2.

The shared flags:

| Flag | Effect |
|---|---|
| `--format=text\|json` | Text for a person, or line-delimited JSON under the versioned schema. Text is the default |
| `--config <path>` | Use this config file, relative to the working directory, and skip discovery |
| `--strict` | Report every Warning as an Error, so a warning fails the plan that reported it |
| `--verbose` | Render Info findings |
| `--no-color` | Render text without colour. A non-empty `NO_COLOR` variable has the same effect |

Colour applies to the severity in text output, and only where standard error
is a terminal. A generated file and JSON output never contain a colour code.

### Config discovery and the root

`Open` finds a workspace's root and its config file in this order:

1. `--config <path>` names the file. Its directory is the root.
2. Otherwise the search starts in the working directory, and checks each
   directory for `.<brand>.yaml`, moving to the parent until it finds the
   file. It stops after the first directory that contains a version
   control marker: `.git`, as a directory or as a file, `.hg`, `.jj` or
   `.svn`. Outside a repository it stops at the filesystem root.
3. The directory of the file it finds is the root.
4. Without a file, the root is the directory that contains the marker, or
   the working directory outside a repository. The composition runs as
   `Compose` returns it.

The state directory is `.<brand>/` in the root, whatever the working
directory is. `go generate` runs a directive's command in the package's
directory, so `//go:generate acme run .` finds the root above the package
and keeps the state in one place.

A positional pattern is relative to the working directory. A command
rewrites it relative to the root, with slashes, so `.` in `svc/store`
becomes `svc/store`. A pattern that resolves outside the root of every
workspace exits 64. In a list, each member runs with the patterns inside
its root, and a member that contains no pattern does not run.

### The config file

The config file is one YAML document. `Open` decodes it with
`yaml.Decoder.KnownFields` set, so every unknown key is an error, and
reports every error of the document together, each at its line:

```yaml
version: 1
workspace: platform
workers: 4
ignore: ["legacy:"]
memo:
    limit: 512MiB
    dir: /var/cache/acme/memo
plans:
    go-services:
        sources: {lang: golang, packages: ["./svc/..."]}
        layout: {policy: centralised, dir: gen, importBase: example.com/platform/gen}
    go-mocks:
        enabled: false
options:
    stubgen: {suffix: _stub}
```

| Key | Value | Applies through |
|---|---|---|
| `version` | `1`, required | The format's version. A file of another version exits 64 |
| `workspace` | a string | `Builder.Workspace` |
| `workers` | an integer of 0 or more | `Builder.Parallel` |
| `ignore` | a list of directive spellings | `Builder.Ignore` |
| `memo.limit` | bytes, as an integer, or digits followed by `KiB`, `MiB` or `GiB` | `Memo.Limit` |
| `memo.dir` | a path, absolute or relative to the root | `Memo.Open`, over `ledger.OpenAt` |
| `plans.<name>.enabled` | a boolean, `true` by default | `PlanConfig.Disabled` |
| `plans.<name>.sources` | `lang`, `packages` and `module` | `PlanConfig.Sources` |
| `plans.<name>.layout` | `policy`, `dir` and `importBase` | `PlanConfig.Policy`, `Dir` and `ImportBase` |
| `options.<plugin>` | a mapping of option keys to values | `Config.Options` |

The `policy` key takes the spellings of `layout.Policy.String`: `inherit`,
`alongside-source` and `centralised`. Each entry under `plans` refines one
plan that the binary compiles in, by the plan's name, and no entry can
assemble a new plan from compiled-in parts.

`Open` runs Build over each configured builder, and each of Build's faults
is a config error. A config error exits 64, except under `doctor`, which
reports it and continues.

The decoder reports an unknown key and a type mismatch with the line of the
value. Measured with v3.0.5 over a document with both faults, it returned
``line 3: cannot unmarshal !!str `four` into int`` and ``line 7: field
sourcse not found in type main.plan`` together. YAML 1.1's `yes`, `no`, `on`
and `off` read as booleans only into a boolean field.

#### Lists of workspaces

A file that states `workspaces` states nothing else but `version`:

```yaml
version: 1
workspaces:
    - {root: ./platform, config: ./platform/.acme.yaml}
    - {root: ./tools/gen}
```

- Each root is relative to the list's directory, and `config` defaults to
  `.<brand>.yaml` in the root.
- A member's config file cannot be a list.
- Two roots that nest, after symbolic links resolve, are a config error that
  names both.
- `Open` reads and builds every member before it returns, so a config error
  in any member exits 64 and no member runs.
- A command runs the members in list order, one after another, each under
  its own lock. Each JSON event states its member in the `workspace` field.

#### The JSON Schema

The package `internal/config` builds the JSON Schema of the config file,
draft 2020-12, from the Go types of the document and the list. The module
contains the schema as `config.schema.json`, and each release publishes that
file. A golden test compares the file with the schema that the package
builds, so a change of the format fails the test until `go test
./internal/config -update` writes the file again. The schema has every key
of the preceding table. Each `options` section is an object with any keys,
because each plugin defines its own options and Build validates them.

### The plan refinements

`workspace.Config` defines no file format. It gains the plan refinements,
which the config file maps onto one to one:

```go
// Config is what the composition populates from: option values per
// plugin, keyed by plugin name, then option key, and refinements of the
// composition's plans, keyed by plan name. The typed struct is the
// contract a config file maps onto, and this package defines no file
// format.
type Config struct {
    Options map[string]map[string]any
    // Plans refines the plans of the composition. Build refuses a name
    // that the composition does not declare.
    Plans map[string]PlanConfig
}

// PlanConfig refines one plan of the composition before Build compiles
// it.
type PlanConfig struct {
    // Disabled leaves the plan out of the composition. Build refuses a
    // disabled plan that an enabled plan depends on, and one that a
    // workspace check reads, naming both.
    Disabled bool
    // Sources replaces the plan's sources where it is not nil.
    Sources *Sources
    // Policy, Dir and ImportBase replace the plan's layout fields of
    // the same names where they are set: a policy other than
    // layout.PolicyInherit, and a directory or an import base that is
    // not empty.
    Policy     layout.Policy
    Dir        string
    ImportBase string
}
```

`core/layout` gains the parser that the config file's `policy` key needs:

```go
// ParsePolicy returns the policy that a configuration spells, as
// Policy.String spells it.
//
// Error modes: a spelling that names no policy.
func ParsePolicy(s string) (Policy, error)
```

Build applies the refinements before it compiles the plans, so the
composition fingerprint folds the refined sources and layouts, as it folds
the compiled-in ones.

A YAML decoder returns an integer as an `int` and a sequence as an `[]any`,
while a plugin's option can be an `int64` or a `[]string`. Build therefore
populates an option from a value of another type through the canonical
JSON encoding of options: it encodes the value and decodes the bytes into
the field's type. A YAML integer then populates an `int64` field, and a
sequence of strings a `[]string` field. A value that does not decode into
the field's type remains a Build fault, which names the option, the field's
type and the value's type.

### The lock

The ledger gains an optional interface, which `Dir` and `Mem` implement:

```go
// Locker is a ledger that admits one holder at a time.
type Locker interface {
    // Lock takes the ledger's lock for a holder, and returns the function
    // that releases it. Lock does not wait: where another holder has the
    // lock, it returns a *LockedError that names that holder.
    //
    // Error modes: *LockedError, which wraps ErrLocked, and the operating
    // system's error where the lock file cannot be opened.
    Lock(ctx context.Context, h Holder) (release func() error, err error)
}

// Holder is the record of a lock's holder.
type Holder struct {
    // PID is the process ID, and Host the host name.
    PID  int
    Host string
    // Caller describes the caller, such as the command line of a run.
    Caller string
    // Since is when the holder took the lock.
    Since time.Time
}

// ErrLocked reports a ledger whose lock another holder has taken.
var ErrLocked = errors.New("ledger: the state directory is locked")

// LockedError reports the holder of a lock that Lock could not take.
type LockedError struct {
    // Holder is the holder's record, and the zero Holder where the record
    // does not read.
    Holder Holder
}

func (e *LockedError) Error() string
func (e *LockedError) Unwrap() error // returns ErrLocked
```

`Dir.Lock` opens `.<brand>/lock`, creating it where it does not exist:

- On Linux, the BSDs, Darwin and illumos, it takes `flock(2)` with
  `LOCK_EX|LOCK_NB` through `syscall.Flock`. `EWOULDBLOCK` means that
  another process has the lock.
- On Windows, it opens the file through `syscall.CreateFile` with a share
  mode of zero. A sharing violation means that another process has the
  lock.
- After it takes the lock, it writes the holder's record to
  `.<brand>/lock.json`. A contender reads that file to name the holder.
- The release removes the record and then closes the file, so the record
  exists while its holder has the lock, and a run over an unchanged tree
  leaves the state directory as it found it. The operating system also
  releases the lock when the process exits, so a crash leaves no stale
  lock, and the next holder replaces the record that the crash left.

`flock(2)` binds the lock to the open file description, and the operating
system releases it once every descriptor of that description has closed.
Over NFS, Linux since 2.6.12 emulates `flock(2)` with `fcntl(2)` byte-range
locks on the entire file. `Mem.Lock` admits one holder at a time within the
process, so a test exercises contention over a memory ledger.

`Workspace.Run` takes the lock of the ledger that it opens, where the ledger
implements `Locker`, before it reads the previous record. It releases the
lock after it writes the record. A dry run takes the lock too, because it
reads the generation that a committing run replaces. Where another holder
has the lock, the run reports `StateLocked`, an Error at `.<brand>/lock`
that names the holder, marks every plan failed, and writes nothing.
`Workspace.Explain` takes the lock in the same way. The memo's ledger is not
locked, because its entries are addressed by their content and safe to
share.

### The run inputs

`workspace.Input` gains eight fields:

```go
type Input struct {
    Tree   fs.FS
    Stores map[string]fs.FS
    Graph  *store.Graph
    Dry    bool
    Cold   bool

    // Plans restricts the run to the plans that it lists and every plan
    // they depend on, transitively. Empty runs every plan. The plans that the
    // selection leaves out do not run, and their files and records
    // remain. Run refuses a name that the composition does not declare.
    Plans []string
    // Patterns restricts what the run commits to the changes inside them,
    // as the narrowing rules state. A pattern is a workspace directory as
    // Sources.Packages spells it, and Run refuses one that names no
    // directory. The run loads, validates, annotates and generates as a
    // run without patterns does.
    Patterns []string
    // OverwriteDrift lets a plan overwrite a generated file that was
    // edited since its stamp, where the run otherwise reports
    // DriftedOutput.
    OverwriteDrift bool
    // Adopt lets a plan replace a file without the brand's frame at a
    // path that it routes a file to, where the run otherwise reports
    // ForeignFile.
    Adopt bool
    // Strict reports every Warning as an Error.
    Strict bool
    // Check runs as Dry runs, and reports each path that the run would
    // create, update or remove, or would refuse to write, as an Error
    // under OutOfDate.
    Check bool
    // Prune runs every plan and commits only the removal of stale
    // files. Run refuses Prune with Patterns or with Check.
    Prune bool
    // Caller describes the caller in the lock's holder record. Empty
    // records "workspace.Run".
    Caller string
}
```

#### Stores

A dependency unit reads a store beside the tree, such as the Go module
cache or the standard library. A language's frontend package already
locates its stores through the environment, under the names its doors
read, and a command line cannot call each language's function. The plugin
SPI therefore gains an optional interface, which the Go and Java frontends
implement over their existing functions:

```go
// StoreLocator is a frontend that locates the stores its dependency units
// read.
type StoreLocator interface {
    // Stores returns the frontend's stores under their names, rooted
    // where the language's toolchain finds them. It reads the environment
    // through getenv.
    //
    // Error modes: a root that neither the environment nor the
    // toolchain's own rules locate.
    Stores(getenv func(key string) string) (map[string]fs.FS, error)
}
```

`workspace.Workspace` gains the method that collects them:

```go
// Stores returns the stores of every frontend of the composition that
// implements plugin.StoreLocator, merged by name, for Input.Stores.
//
// Error modes: a locator's error, which names its frontend, and a store
// name that two frontends both locate.
func (w *Workspace) Stores(getenv func(key string) string) (map[string]fs.FS, error)
```

The frontend kit's `Builder` gains the declaration, so a frontend that the
kit builds implements `StoreLocator`:

```go
// Stores declares the function that locates the stores that the
// dependency units of the language read, such as a module cache. The
// built frontend implements plugin.StoreLocator through the function. A
// declaration with Stores also declares Builder.Dependencies, because
// only a dependency unit reads a store.
func (b *Builder) Stores(locate func(getenv func(key string) string) (map[string]fs.FS, error)) *Builder
```

`run`, `prune`, `watch` and `doctor` pass the result as `Input.Stores`, and
a locator's error exits 1 with an `error` event.

#### Plan selection

`Plans` runs the plans that it lists and every plan they depend on, so no
plan reads the export of a plan that did not run. Every other plan reports
`PlanSkipped`: it does not run, and its files and manifest entries remain.

- A plan of the run that routes a file to the path of a skipped plan's file
  collides with that file under `PlanCollision`, because the run cannot
  tell whether the skipped plan still produces it.
- A workspace check that reads a skipped plan does not run.

#### Narrowing

A run with patterns computes every plan's changes over the whole graph,
because a file can derive from declarations in more than one package, and
a plan's accumulated files need every contributor to come out right. The
patterns then select what the run commits:

- A change is inside the patterns when a source of the file, as the run
  produced it or as the record lists it, is a declaration of a package that
  the patterns admit. A pattern admits a package under the rule that a
  plan's sources apply to directory patterns.
- The removal of a stale file is also inside the patterns when its plan
  runs none of the plugins that the record lists for the file, because the
  file's producer left the plan.
- The run commits every change inside the patterns: a created or updated
  file, and the removal of a stale one.
- The run stages every other change into a sink of its own, which it
  prepares and discards. The change's manifest entry remains as the record
  lists it, and `PlanReport.Withheld` lists the change as that preparation
  found it.
- The sweep of a plan that the composition no longer declares commits
  whatever the patterns, as it does in a run without them.
- Under `Check`, a run with patterns reports only the changes inside them.
- A package that the graph no longer contains is in no directory, so no
  pattern admits the removal of its files. `prune` removes them.
- A run with patterns, a prune, and a run that left a plan out through
  `Plans` do not write a generation. Each records the manifest documents of
  the files that it committed, as a run over a caller's graph does.

The next run reads the generation that the last whole run wrote, finds the
same changes against it, and commits what the narrowed run withheld. The
parse memo still saves every parse, because a run that writes no
generation writes the memo's entries.

#### The check, drift and adoption

`Check` promotes a non-empty diff to findings, so a CI gate needs no JSON
parsing. Each path that the run would create, update or remove, or would
refuse to write, is an Error under `OutOfDate`, positioned at the path. A
file whose bytes are unchanged reports nothing.

`OverwriteDrift` and `Adopt` change what a plan may write over. A sink such
as `output.Disk` refuses to overwrite any file that its brand cannot prove
it wrote intact, so a run passes the flags to the sink through an optional
interface:

```go
// Overwriter is a sink whose commit can write over a file that its brand
// did not write intact.
type Overwriter interface {
    // Overwrite lets Commit write over the files of the verdicts given:
    // FoundDrifted, FoundForeign or both. Commit never writes over a
    // directory, which is foreign.
    //
    // Error modes: another verdict, which allows nothing, a call after
    // Prepare, and ErrFinished after Commit or Discard.
    Overwrite(found ...Found) error
}
```

`Disk`, `Mem` and `Tee` implement it. A `Tee` passes the verdicts to every
sink it writes to, and refuses a sink that does not implement the
interface, because that sink's commit would refuse the files. A run under
either flag calls `Overwrite` right after it opens a plan's sink, and does
not report `DriftedOutput` or `ForeignFile` for a path that the sink then
overwrites. An error of `Overwrite` fails the plan. A sink that does not
implement the interface refuses as before, and the run reports both codes
as it does without the flags. The sweep still removes only the brand's
intact output, under either flag.

#### Prune

`Prune` runs every plan as a run without patterns does, then stages and
commits only the removal of stale files. A stale file is one that the
record lists and that no plan of the composition produces. The prune
withholds every write, so it writes no generation. Under `Dry`, it prepares
the removals and commits nothing.

#### The report's new fields

```go
type Report struct {
    // ... the existing fields ...

    // Suppressed counts, for each code, the findings that a diag
    // directive removed.
    Suppressed map[diag.Code]int
    // Suppressions lists each diag directive of the graph, with the
    // number of findings it removed, in position order.
    Suppressions []Suppression
}

type PlanReport struct {
    Name    string
    Status  PlanStatus
    Changes []output.Change
    // Withheld lists the changes that the run left for a later run,
    // sorted by path, as a preparation of their own found them: the
    // changes outside the patterns, and every write of a prune.
    Withheld []output.Change
    // Refused lists the writes that the run refused, sorted by path. The
    // path of such a write has a file that was edited since its stamp, or
    // a file without the frame of the brand, and the run did not write
    // over files of that kind. A plan that refuses a write fails.
    Refused []output.Change
}

// PlanSkipped reports a plan that the run's plan selection left out.
const PlanSkipped PlanStatus = 5

// Suppression is one diag directive and what it removed.
type Suppression struct {
    // Subject is the declaration that the directive annotates, and At the
    // directive's position.
    Subject symbol.Identity
    At      position.Pos
    // Code is the code that the directive names.
    Code diag.Code
    // Count is the number of findings that the directive removed.
    Count int
}
```

### Diagnostic suppression

`+<brand>:diag off=<code>` on a declaration removes that code's findings at
that declaration and nowhere else:

- A finding is at a declaration when its position equals the declaration's
  position. A finding's position is the declaration that caused it, so the
  match is exact, and a member's findings need the directive on the member.
- A kernel Error, a finding under the kernel's prefix at the Error
  severity, is never removed. A directive that names a kernel code removes
  that code's Warnings and Infos alone.
- A value of `off` that does not parse as a code, or that names a code that
  no registry contains, is a validation Error under `UnknownCode`. The
  kernel's `diag` schema types `off` as a reference of a new resolution
  kind, `directive.ResolveDiagnosticCode`, which validation resolves
  against the codes that `diag.MustRegister` registered.
- A second directive that names the same code on the same declaration
  removes nothing.

The run builds its suppression table from the validated `diag` instances
after validation. On a warm run, the table reads the subjects with a `diag`
directive through the graph's directive index, which decodes only the
regions whose summary lists the spelling. `diag.Sink` gains the filter and
the promotion:

```go
// Suppress installs a table of suppressions: for each declaration
// position, the codes that its diag directives name. The sink removes
// every finding it contains and every later finding whose position and
// code the table lists, except a kernel Error, and counts what it
// removed. Failed reports only the findings that remain.
func (s *Sink) Suppress(table map[position.Pos][]Code)

// Removed returns the number of findings that the table removed, for each
// position and code.
func (s *Sink) Removed() map[position.Pos]map[Code]int

// Promote reports every Warning that the sink contains, and every later
// one, as an Error: Failed counts it, and All returns it at SeverityError.
// The sink keeps each finding at the severity it was reported at, so the
// table of Suppress removes a promoted Warning as it removes any other.
func (s *Sink) Promote()

// ParseCode returns the code that a spelling names, in the form that
// Code.String returns, such as EID-0062.
//
// Error modes: a spelling without a hyphen, a prefix that is not
// uppercase letters, a number in another form, such as EID-62, and a
// number below 1.
func ParseCode(s string) (Code, error)
```

The run installs the table in the sinks that decide an outcome: its own
sink, which then removes the findings of the load and of validation, and
each plan's sink, which filters before the plan's outcome is decided, so a
removed Error does not fail its plan. The sinks that collect one
execution's findings for its record do not filter, so the sealed state
keeps each record's findings as the execution reported them. A warm run's
replayed findings pass through the same filter, so adding or removing a
directive changes what the next run reports. Under `Strict`, the run calls
`Promote` on the same sinks, and the promotion applies to the findings
that remain after the filter.

A run reports each directive that removed nothing as an Info under
`UnusedSuppression`, at the directive's position. A run that skips a plan
does not count that plan's findings, and a cancelled run does not count
every finding, so neither reports one. `doctor` lists these findings.

### Deprecation markers

```go
type Schema struct {
    // ... the existing fields ...

    // Deprecated marks the directive deprecated where it is not empty,
    // and states the rewrite, such as "write bound= in place of limit=".
    Deprecated string
}

type ParamSpec struct {
    // ... the existing fields ...

    // Deprecated marks the parameter deprecated where it is not empty,
    // and states the rewrite.
    Deprecated string
}
```

Validation reports each use of a deprecated directive or parameter as a
Warning under `DeprecatedDirective`, at the directive's position, with the
rewrite in the message. `doctor` lists these findings.

### Rendering and the machine output

Text output writes each finding to standard error as one line, followed by
one indented line per related position:

```
svc/store.go:41:2: error EID-0029: chan int has no TypeScript spelling (typescript)
    related: svc/api.go:12:1
```

A command's results go to standard output. `run` prints one line for each
change that is not `unchanged`, then one summary line with the count of
each action and of each plan status. A warm run renders again only the
files of its dirty groups, so the summary of a warm run over an unchanged
tree counts no file.

JSON output writes one object per line to standard output. Every object has
an `event` field, and a consumer skips the events and fields it does not
know:

| Event | Commands | Fields |
|---|---|---|
| `start` | every command | `schema`, `command`, `brand`, `binary` with the main module's path and version |
| `diag` | every command | `code`, `severity`, `pos`, `msg`, `origin`, `related`, `workspace` |
| `error` | every command | `msg`, `usage`, `workspace`: one line of a failure without a position, such as a usage error, a config error, an I/O error or a cancellation |
| `file` | `run`, `prune`, `watch` | `path`, `plan`, `action`, `found`, `hash`, `workspace` |
| `outcome` | `run`, `prune`, `watch` | `plan`, `status`, `workspace` |
| `plan`, `component` | `plan` | the description below |
| `explain`, `explain.file`, `explain.claim`, `explain.record` | `explain` | the explanation below |
| `version` | `version` | the fields below |
| `summary` | every command | `status`, the counts below, and `workspaces` for a list |

`action` names what a run did or would do to a path:

| Action | Meaning |
|---|---|
| `create` | A write to a path that contains nothing |
| `update` | A write over the brand's intact output, a drifted file under `OverwriteDrift`, or a foreign file under `Adopt` |
| `unchanged` | A write of the bytes that the path already contains, or a removal that found nothing |
| `stale` | The removal of the brand's intact output that no plan produces |
| `drifted` | A write or a removal that the run refused, because the file was edited since its stamp |
| `foreign` | A write that the run refused, because the path contains a file without the brand's frame |
| `withheld` | A change other than `unchanged` that a narrowed run or a prune left for a later run |

`found` is what the path contained before the run, as `output.Found`
spells it.

A command renders each error that a run returns as `error` events, one for
each line of its text, except `workspace.ErrRunFailed`. A run returns
`ErrRunFailed` for its Error findings, and the command renders those
findings as `diag` events already.

The `summary` event counts the files by action, the plans by status and the
findings by severity. It also contains `suppressed`, the removed findings by
code, and `status`, the exit status. For a list of workspaces, the top-level
counts are totals, and `workspaces` lists each member's counts:

```json
{"event":"start","schema":"1.0","command":"run","brand":"acme","binary":{"path":"example.com/acme/cmd/acme","version":"v2.3.0"}}
{"event":"file","path":"svc/store_stub.go","plan":"go-stubs","action":"update","found":"intact","hash":"sha256:9f2c…"}
{"event":"diag","code":"EID-0029","severity":"error","pos":"svc/store.go:41:2","msg":"chan int has no TypeScript spelling","origin":"typescript"}
{"event":"outcome","plan":"go-stubs","status":"failed"}
{"event":"summary","status":1,"files":{"create":0,"update":1,"unchanged":40,"stale":0,"drifted":0,"foreign":0,"withheld":0},"plans":{"committed":0,"failed":1,"cancelled":0,"prepared":0,"skipped":0},"errors":1,"warnings":0,"infos":0,"suppressed":{"EID-0007":2}}
```

The schema's version is a major and a minor number. A minor release adds
events or fields, and a major release changes or removes them. Paths are
relative to the root and separated by slashes, as the manifest's paths are.

### The exit status

A command returns the status, and its host exits with it:

| Status | When |
|---|---|
| 0 | The command succeeded. Warnings do not fail a command, except under `--strict` |
| 1 | An Error remained after suppression, a run returned an error such as an I/O failure or a cancellation, `--check` found a change, or another process had the lock |
| 64 | A usage error: an unknown flag, an unknown command under `Main`, a malformed value, a pattern outside the root or one that names no directory, a plan that the composition does not declare. A config error: a file that does not decode, a key or a value that does not validate, a Build fault, nested roots in a list |
| 2 | Never returned by eidos code. A Go panic exits with 2, and the kernel recovers no panic |

`watch` exits 0 when it stops on an interrupt. An interrupt that cancels
any other command exits 1, because the command did not finish.

### The commands

#### run

```
acme run [--plan <name>]... [--dry-run] [--check] [--overwrite-drift] [--adopt] [--cold] [<pattern>...]
```

| Flag | Input field |
|---|---|
| `--plan <name>`, repeated | `Plans` |
| `--dry-run` | `Dry` |
| `--check` | `Check` |
| `--overwrite-drift` | `OverwriteDrift` |
| `--adopt` | `Adopt` |
| `--cold` | `Cold` |
| positional patterns | `Patterns`, rewritten relative to the root |
| `--strict` | `Strict` |

`run` passes the root's tree as `os.DirFS(root)`, and its name and its
arguments as `Caller`. A CI gate runs `acme run --check`, which exits 1 when the
committed output is out of date and writes nothing.

#### plan

`plan` prints the composition without running anything, from
`Workspace.Describe`:

```go
// Describe returns the composition as data: the brand, the workspace's
// name and the fingerprint, the frontends, the annotate schedule, each
// plan with its sources, dependencies, generate schedule, backend and
// layout, the commit order and the checks. It runs nothing, reads nothing
// outside the workspace and cannot fail. Each call returns a description
// of its own.
func (w *Workspace) Describe() Description

// Description is a composition as data. It shares no memory with the
// workspace.
type Description struct {
    Brand       output.Brand
    Name        string
    Fingerprint []byte
    Frontends   []Component
    // Annotate lists the annotators in bucket order.
    Annotate []Component
    Plans    []PlanDescription
    // Order lists the plans in commit order.
    Order  []string
    Checks []CheckDescription
}

// Component is one plugin of a composition.
type Component struct {
    Name plugin.ID
    // Version is the version that the plugin declares, and empty for a
    // plugin that declares none.
    Version string
    // Bucket is an annotator's or a generator's position in the schedule
    // of its role, counted from one, and zero for any other plugin.
    Bucket int
}

// PlanDescription is one plan of a composition, after the config's
// refinements.
type PlanDescription struct {
    Name      string
    Sources   Sources
    DependsOn []string
    // Generate lists the plan's generators in bucket order.
    Generate []Component
    Backend  Component
    Target   plugin.Target
    Layout   layout.Config
}

// CheckDescription is one workspace check, with the plans that it reads.
type CheckDescription struct {
    Component
    Reads []string
}
```

Each annotator and each generator has a bucket of its own: the schedule of
a role orders its plugins by priority, then by capability, then by name,
and numbers them in that order. One generator numbering covers every plan,
so the buckets of one plan's generators can skip numbers.

JSON output emits one `component` event per frontend, annotator and check,
and one `plan` event per plan, in the order of the description:

| Event | Fields |
|---|---|
| `component` | `role`, `name`, `version`, `bucket`, and `reads` for a check |
| `plan` | `name`, `order` in the commit order from 1, `sources` with `lang`, `packages` and `module`, `dependsOn`, `generate` with the `name`, `version` and `bucket` of each generator, `backend` with its `name` and `version`, `target`, and `layout` with `policy`, `dir` and `importBase` |

Text output writes the same description, one line per component and a
block of indented lines per plan:

```
frontend golang 1.4.0
annotator 1 shapefull 2.1.0
plan go-services: target go, backend go-printer, commit order 1
  sources: language golang, packages ./svc/...
  generator 3 stubgen 1.0.0
  layout: policy centralised, dir gen
check imports: reads go-services
```

#### explain

`explain <target>` reads the live generation and prints what it records
about the target. It runs nothing, so it describes the last run that wrote
a generation. A dry run, a run with patterns or a plan selection, and a
prune write none.

The target takes four forms, told apart by shape:

| Form | Example | Explanation |
|---|---|---|
| A path | `svc/store_stub.go` | The plan, the plugins and the source declarations that produced the file, the invocations that contributed to its group, and the group's findings |
| A symbol identity | `golang:svc/store.Store#Get` | The claims on the declaration, the records that read it, and the files generated from it |
| A key at a position | `shape.name@svc/store.go:41` | The claims on that fact, the winner first, each with its authority, its plugin and the reads it derived from |
| A code at a position | `EID-0029@svc/store.go:41` | The records that reported the finding, with the reads that explain can spell |

An argument that contains `@` is a key or a code at a position: a code
when the part before `@` parses as a code, and a key otherwise. A position
is `file:line` or `file:line:column`. An argument without `@` is an
identity when `symbol.Parse` reads it under a language that the
composition's frontends load or its rules values declare, and a path
otherwise. `symbol.Parse` leaves the kind of most spellings unset, and such
an identity names each declaration whose other fields equal its own.

```go
// Explain reads the live generation of the composition's ledger under the
// ledger's lock, and returns what the generation records about a target.
// It records "workspace.Explain" as the holder's caller.
//
// Error modes: an error for a target that does not set exactly one form,
// with or without the position that the form takes, and for a key that
// the composition does not register. An error wrapping ErrNoGeneration for
// a composition that declares no output or no ledger, and for a ledger
// without a generation. A *ledger.LockedError where another holder has
// the lock, the ledger's own errors, and an error for a generation that
// does not read whole.
func (w *Workspace) Explain(ctx context.Context, t Target) (*Explanation, error)

// Target is what Explain explains. Exactly one of Path, Identity, Key and
// Code is set, and At is set with Key and with Code.
type Target struct {
    Path     string
    Identity symbol.Identity
    Key      meta.KeyName
    Code     diag.Code
    // At is the declaration's position. A zero column matches every
    // column of the line.
    At position.Pos
}

// ParseTarget reads explain's argument in one of its four forms. It
// returns a path and a position's file as the argument writes them.
//
// Error modes: a position without a file or a line, a line or a column
// below one, and a key that the composition does not register.
func (w *Workspace) ParseTarget(s string) (Target, error)

// Explanation is what a generation records about a target.
type Explanation struct {
    // Generation names the generation, and Recorded is the anchor of the
    // run that wrote it.
    Generation string
    Recorded   time.Time
    Files      []ExplainedFile
    Claims     []ExplainedClaim
    Records    []ExplainedRecord
}

// ExplainedFile is one generated file.
type ExplainedFile struct {
    Entry        manifest.Entry
    Contributors []plugin.MatchKey
    Findings     []diag.Diag
}

// ExplainedClaim is one claim on a fact. The claims of one fact are in
// rank order.
type ExplainedClaim struct {
    Key   meta.KeyName
    Claim meta.Claim
    // Value is the claimed value, and nil for a drop.
    Value any
    // Winner reports the claim that ranks first on its fact.
    Winner bool
}

// ExplainedRecord is a generation's record of one execution: a
// validation, an invocation, a check, a group, an audit finding, or the
// parse and link of a region.
type ExplainedRecord struct {
    Kind RecordKind
    // Plan is the plan of a plan's invocation or group, and empty for
    // every other record. Match is an invocation's match, Check a check's
    // name and Group a group's key unit. Subject is the subject of a
    // validation, of an invocation or of an audit finding.
    Plan     string
    Match    plugin.MatchKey
    Check    plugin.ID
    Group    plugin.UnitRef
    Subject  symbol.Identity
    Findings []diag.Diag
    // Reads are the reads that Explain identified. Unidentified is the
    // number of reads that Explain could not identify.
    Reads        []Read
    Unidentified int
}

// RecordKind names the kind of execution an explained record keeps. Its
// String returns validation, invocation, check, group, audit or region.
type RecordKind uint8

const (
    RecordValidation RecordKind = 1
    RecordInvocation RecordKind = 2
    RecordCheck      RecordKind = 3
    RecordGroup      RecordKind = 4
    RecordAudit      RecordKind = 5
    RecordRegion     RecordKind = 6
)

// Read is one read that Explain spells: its grain and what it names.
type Read struct {
    Grain ReadGrain
    // Subject is the declaration of a declaration or a fact edge, and the
    // package of a package edge. Key is a fact edge's key.
    Subject symbol.Identity
    Key     meta.KeyName
    // Kind and Directive are a membership edge's kind or spelling, and
    // Plan an export edge's plan.
    Kind      symbol.Kind
    Directive directive.Name
    Plan      string
}

// ReadGrain names the grain of a spelled read. Its String returns
// declaration, package, fact, kind, directive or export.
type ReadGrain uint8

const (
    ReadDeclaration ReadGrain = 1
    ReadPackage     ReadGrain = 2
    ReadFact        ReadGrain = 3
    ReadKind        ReadGrain = 4
    ReadDirective   ReadGrain = 5
    ReadExport      ReadGrain = 6
)

// ErrNoGeneration reports a workspace whose ledger contains no
// generation: no run has written one, or the composition keeps no ledger.
var ErrNoGeneration = errors.New("workspace: the ledger contains no generation to explain")
```

The generation keeps each read as a 64-bit hash of the edge's spelling,
and no table maps a hash back to its spelling. Explain hashes the edges
that it can name and matches them against a record's reads: the
declaration and package edges of the record's subject and of the target's
declarations, a fact edge on each of them for every key that the registry
contains, a membership edge for every kind and for every directive spelling
that a unit's summary in the recorded load lists, and an export edge for
every plan. A read that matches none of them counts in `Unidentified`. The
findings edge, which marks a record that reported a finding, is no read.

Explain finds each form through queries that the sealed state already
offers, and the record tables gain the lookup of a check by reference that
the other record kinds have:

- A path reads the file's artifact row, the row of its group, and the row
  of each of the group's contributors.
- An identity reads the subject's claims, and the readers of its
  declaration edge. The invocations among them name the units that they
  placed into, and the readers of each unit's edge name the groups and
  their files.
- A key at a position finds each declaration at the position in the region
  of the unit that contains the file, and reads its claims under the key
  and the readers of its fact edge.
- A code at a position reads the records that list the findings edge among
  their reads, the audit table, and the findings of each unit of the load,
  and keeps each finding under the code at the position.

`explain` rewrites a path and the file of a position from the working
directory to the root of each workspace. It explains the target in each
member of a list whose root contains them, and an identity, which has no
path, in every member. A path outside the root of every workspace exits 64,
and an error of `Explain`, such as a missing generation, exits 1. `explain`
prints the authority of a claim through `meta.Authority.String`, which the
kernel gains:

```go
// String returns the name of the authority in a report: plugin, directive
// or manual. For a value outside the three authorities, it returns the
// number in the form Authority(n). It allocates nothing for a declared
// authority.
func (a Authority) String() string
```

JSON output emits the generation first, then one event per file, per
claim and per record:

| Event | Fields |
|---|---|
| `explain` | `generation`, and `recorded`, the time of the record in RFC 3339 form |
| `explain.file` | `path`, `plan`, `hash`, `plugins`, `sources`, `contributors` with the `plugin`, `rule`, `subject` and `instance` of each match, and `findings` |
| `explain.claim` | `key`, `subject`, `value`, `winner`, `authority`, `bucket`, `plugin`, `pos` for a claim with a carrier, and `derived` with the `subject` and the `key` of each read |
| `explain.record` | `kind`, `plan`, `match`, `check`, `group` with its `plugin`, `tag`, `package` and `key`, `subject`, `findings`, `reads` with the `grain`, `subject`, `key`, `kind`, `directive` and `plan` of each read, and `unidentified` |

Text output writes one line per file, claim and record, and one indented
line per contributor, source, derived read, read and finding:

```
generation state/gen/a093a8da…, recorded 2026-10-08T18:22:32Z
file svc/store_stub.go: plan go-stubs, plugins stubgen
  contributor stubgen rule 0 golang:svc/store.Store
  source golang:svc/store.Store
claim shape.name on golang:svc/store.Store: value Store, winner true, authority plugin, plugin shapefull, bucket 1
  derived from subject golang:svc/store.Store, key shape.name
record invocation: plan go-stubs, match stubgen rule 0 golang:svc/store.Store, subject golang:svc/store.Store; 2 reads, 0 unidentified
  read declaration of golang:svc/store.Store
  read fact shape.name of golang:svc/store.Store
```

#### prune

```
acme prune [--dry-run]
```

`prune` runs the workspace with `Prune` and prints each removal. Under
`--dry-run` it prints the removals that it would make. It never removes a
drifted file or a file without the brand's frame, because the sweep removes
only the brand's intact output.

#### doctor

```
acme doctor
```

`doctor` writes nothing but the lock file. It reports what it finds as
findings and `error` events, and exits 1 when it reports a config error, an
Error finding or the error of a run. It exits 64 only for a usage error of
its own:

1. It decodes each member's config file and builds the member, and reports
   each config error. A member that does not build skips the following
   steps.
2. It runs the member with `Dry`, which takes the lock and opens the live
   generation. The dry run reports the `ColdState` Info that the next run
   would report: for an executable that does not read, because such a run
   does not write a generation, and for a generation of another format,
   composition or executable.
3. It renders every Error and Warning of the dry run, and its `ColdState`,
   `UnusedSuppression` and `DeprecatedDirective` findings at every
   severity, with or without `--verbose`.

#### watch

```
acme watch [--interval <duration>] [<pattern>...]
```

`watch` runs `run` in a loop. A warm run over an unchanged tree is the
change detector: it stats every file, reads the files and units tables and
the manifest's documents, decodes no region and executes nothing.

- `--interval` is the pause between two runs, 1s by default. A value below
  100ms exits 64.
- A run that changed a file, or whose findings or error differ from the
  findings or the error of the previous run over the same workspace, prints
  its events. A run that did neither prints nothing.
- When `watch` stops, it writes one summary with the counts of every run
  that it printed.
- Each run takes the lock for its own duration. `watch` skips a pass whose
  run finds that another process has the lock, and renders that run's
  `StateLocked` finding once, until a pass takes the lock again.
- An interrupt stops `watch` during a run or between two runs, and `watch`
  exits 0. It does not print a run that the interrupt cancelled.

#### version

`version` prints, from the binary's build information and from
`Workspace.Describe`:

- the main module's path and version
- the versions of `go.dokimi.dev/eidos/core` and `go.dokimi.dev/eidos/cli`
- every plugin with the version that it declares
- the SHA-256 of the composition fingerprint, and the SHA-256 of the
  executable

The sealed state's header records both digests, and a difference in either
runs the next run cold. `version` reads the executable once to hash it, and
exits 1 when the executable does not read.

`version` lists each plugin once, under the role of its first place in the
description: frontend, annotator, generator, backend or check. Two plans
that share a backend list it once. JSON output emits one `version` event per
workspace, with `module` as its `path` and `version`, `core`, `cli`,
`plugins` with the `role`, `name` and `version` of each plugin,
`composition` and `executable`. Text output writes one line per field:

```
module example.com/acme/cmd/acme v2.3.0
core v0.9.0
cli v0.9.0
frontend golang 1.4.0
generator stubgen 1.0.0
backend go-printer
composition sha256:5d1e…
executable sha256:9b07…
```

### acceptancetest

`acceptancetest` builds a consumer's binary and runs it as a process. A
consumer runs the suite over its own binary. The Go conformance module runs
it over two binaries composed with the Go satellite. One binary calls
`Main`, and the other mounts the kernel commands under the group `gen` of
its own dispatcher over the standard library.

```go
// Fixture is the binary of a consumer, and a tree that the binary runs
// over.
type Fixture struct {
    // Main is the import path of the main package of the binary. The go
    // command resolves it in the module of the package under test.
    Main string
    // Brand is the brand of the composition of the binary.
    Brand output.Brand
    // Prefix is the arguments before the name of a kernel command, such as
    // []string{"gen"} for a host that mounts the kernel commands under a
    // group. It is empty for a host that mounts them at the root.
    Prefix []string
    // Tree is a workspace tree with the config file of the brand at its
    // root. A check copies the tree into each directory in which it runs
    // the binary.
    Tree fs.FS
    // Panic is the arguments that run a command of the binary that panics.
    // The binary adds the command for the suite.
    Panic []string
    // Fail edits the copy of the tree in root, so that the next run reports
    // an Error.
    Fail func(root string) error
    // Compile compiles the output that a run generated in root.
    Compile func(ctx context.Context, root string) error
}

// RunAcceptanceSuite builds the binary of the fixture once, and runs each
// check in a parallel subtest over a temporary directory of its own.
func RunAcceptanceSuite(t *testing.T, f Fixture)

// Each check runs the binary bin over copies of the tree of f in root, an
// empty directory, and reports through tb.
func AssertStatuses(tb assert.TB, f Fixture, bin, root string)
func AssertNames(tb assert.TB, f Fixture, bin, root string)
func AssertPanic(tb assert.TB, f Fixture, bin, root string)
func AssertDiscovery(tb assert.TB, f Fixture, bin, root string)
func AssertJSON(tb assert.TB, f Fixture, bin, root string)
func AssertIdempotent(tb assert.TB, f Fixture, bin, root string)
func AssertCompiles(tb assert.TB, f Fixture, bin, root string)
func AssertLists(tb assert.TB, f Fixture, bin, root string)
func AssertLocked(tb assert.TB, f Fixture, bin, root string)

// Build compiles the main package main with the go command into a new
// temporary directory of tb, and returns the path of the binary. The go
// command runs in the working directory of the test.
func Build(tb testing.TB, main string) string

// Result is what one process of a binary returned.
type Result struct {
    Status         int
    Stdout, Stderr []byte
}

// Exec runs the binary bin in the directory dir with args, under the
// environment of the test process. Each variable of env, in the form
// name=value, replaces the inherited variable of its name. Exec kills a
// process that runs longer than two minutes.
func Exec(tb assert.TB, bin, dir string, env []string, args ...string) Result
```

The checks take `assert.TB`, so the kit's own tests run each check under
`assert.Rejects` over a host that breaks one rule, and a consumer can run
one check alone. A process inherits the environment of the test, because a
frontend locates its stores through the environment, such as the JDK of the
Java frontend through `JAVA_HOME`. `Build` compiles on each call, and the
build cache of the go command turns a build of an unchanged package into a
link.

The suite checks these rules:

1. **Exit statuses.** `run` exits 0 over the tree, 1 after `Fail`, and 64
   for an unknown flag and for a config file with a key that the format
   does not have.
2. **The names.** `-h` after each of the seven names exits 0 and prints the
   usage of the kernel command of that name.
3. **The panic.** The `Panic` command exits 2 and writes Go's panic and
   stack trace to standard error.
4. **Discovery.** A run from a subdirectory finds the config file of the
   root, and writes the state directory into the root. A version control
   marker between the working directory and a config file stops the
   search. `--config` takes precedence over the config file of the working
   directory.
5. **JSON.** Under `--format=json`, each command except `watch` writes JSON
   lines alone to standard output, with `start` first and `summary` last,
   and nothing to standard error. `explain` explains the first file that
   `run` reports. `watch` runs until an interrupt, and `Exec` waits for the
   process to exit.
6. **Idempotence.** A second run reports every file `unchanged`, and moves
   no modification time of a file outside the state directory. The second
   run is cold, because a warm run over an unchanged tree renders no file.
7. **The output compiles.** `Compile` succeeds after a run. This is the
   only check that compiles generated output.
8. **Lists.** A run over a list of two copies of the tree runs each member
   under a state directory of its own. A run over a list whose roots nest
   exits 64 before it creates a state directory.
9. **The lock.** A run while the suite has the lock through `ledger.Dir`
   exits 1, and its output contains the process ID of the holder.

The suite checks no part of the host's own dispatch, such as an unknown
command or the host's help.

### Codes

| Code | Name | Severity | Position | Meaning |
|---|---|---|---|---|
| EID-0063 | `StateLocked` | Error | The state directory's lock file | Another holder has the lock of the state directory, and the run writes nothing |
| EID-0064 | `OutOfDate` | Error | The path | The committed output differs from what the run generates |
| EID-0065 | `UnusedSuppression` | Info | The diag directive | A diag directive removed no finding |
| EID-0066 | `UnknownCode` | Error | The diag directive | A directive names a diagnostic code nothing registered |
| EID-0067 | `DeprecatedDirective` | Warning | The directive | A directive or a parameter that its schema deprecates is used |

### Failure semantics

| Failure | Where it is detected | Exit | What the command reports |
|---|---|---|---|
| An unknown flag or a malformed value | The command's parse | 64 | An `error` event with `usage` set, and the command's help in text |
| A pattern that names no directory, or a plan that the composition does not declare | The run, before it runs anything | 64 | An `error` event with `usage` set, and the command's help in text |
| An unknown command | `Main`'s dispatch, or the host's | 64 under `Main`, and the host's status under a host | An `error` event with `usage` set under `Main` |
| A config file that does not decode, or a key or a value that does not validate | Discovery | 64 | One `error` event for each fault, at its line |
| A Build fault | Before the command runs | 64 | One `error` event for each fault |
| Two roots of a list that nest | Before any member runs | 64 | One `error` event that names both roots |
| A lock that another process has | The run, or `Explain` | 1 | `StateLocked`, which names the holder |
| An Error after suppression | The run | 1 | The finding |
| `--check` with a change | The run | 1 | `OutOfDate` for each path |
| A ledger, a sink or a plugin that returns an error | The run | 1 | An `error` event for each line of the returned error |
| An interrupt | The command's context | 1, and 0 for `watch` | An `error` event that states the cancellation |
| A panic | Anywhere | 2 | Go's panic and stack trace on standard error |

### Cost

Computed from the sealed state's measurements on an AMD Ryzen 9 9950X3D
with Go 1.27.1:

- **`watch` while nothing changes.** Each pass is a warm run over an
  unchanged tree, which stats every file. At the measured 1.34 to 1.94 µs
  per file, a tree of 100,000 files costs 134 to 194 ms per pass, so the
  default interval of one second keeps 13% to 19% of one core busy.
- **`version`.** One hash of the executable, about 12 ms for 30 MB at the
  measured 2.63 GB/s.

Measured on the same machine with a warm build cache of the go command:

- **`acceptancetest`.** The test of the Go conformance package ran both
  suites in 0.36 s on four processors, the builds of the two binaries
  included.

Estimated, not measured:

- **Discovery** costs one stat per directory between the working directory
  and the root.
- **The lock** costs one open, one `flock(2)` and one write of a record of
  about 200 bytes per run.
- **`explain`** reads the rows that its target names. A code at a position
  reads every record that reported a finding, which a workspace with few
  findings keeps small.
- **`doctor`** costs one warm dry run.

### Migration

| Caller | Change |
|---|---|
| A consumer's `main` | Calls `cli.Main` with its `Compose` function, or mounts `cli.Kernels(compose)` in its own command line |
| `workspace.Builder` | Gains `BrandName` |
| `workspace.Input` and `Report` | Gain the fields above. The additions break no caller |
| `workspace.Config` | Gains `Plans`. Build populates an option from a value of another type through the canonical encoding |
| `workspace.Workspace` | Gains `Describe`, `Explain`, `ParseTarget` and `Stores` |
| `workspace.Run` | Takes the lock, applies suppression, and honours the new inputs |
| `plugin` | Gains `StoreLocator` |
| `frontend.Builder` | Gains `Stores`, so a frontend that the kit builds implements `StoreLocator` |
| The Go and Java frontends | Implement `StoreLocator` over their `Stores` functions |
| `meta.Authority` | Gains `String`, which `explain` prints |
| `ledger.Dir` and `ledger.Mem` | Implement `Locker` |
| `output.Disk`, `output.Mem` and `output.Tee` | Implement `Overwriter` |
| `diag.Sink` | Gains `Suppress` and `Removed`. `diag` gains `ParseCode` |
| `directive.Schema` and `ParamSpec` | Gain `Deprecated` |
| `layout` | Gains `ParsePolicy` |
| `go.work`, the repository README, `.ergon.yaml` and CI | List `eidos-cli`, its coverage floor, and the commit scope `cli` |
| `eidos-conformance/lang/go` | Runs `acceptancetest` over two binaries composed with the Go satellite. One binary calls `Main`, and the other mounts the kernel commands under a group of its own dispatcher. The module requires `eidos-cli` |
| `eidos-sdk` | Regenerated, so the facade re-exports `plugin.StoreLocator`. The command line is not a plugin-facing package |

## Alternatives considered

### `Main` over a built workspace

`cli.Main(ws *workspace.Workspace, extra ...Command) int` takes the
workspace that the consumer built.

**Why not:** a command applies the config file to the builder before Build
runs, and a list of workspaces builds one workspace for each member. A
plugin instance belongs to one workspace, because Build writes the config's
values into the plugin's options, so the members need fresh instances.

### `Main` as the only way in

`Main` takes over `main`, and the seven commands exist only inside it. A
consumer whose binary has a command line of its own mounts `Main`'s whole
command line as one command, with the framework's flag parsing turned off.

**Why not:** the framework sees one command, so its help and its completion
of command names list none of the seven, and every eidos command is nested
under that one name. The host also has to pass the exit status through by
hand, and a consumer's command has to be a `cli.Command` to be listed
beside the seven.

### The commands built on a command-line framework

The seven commands are cobra, urfave/cli or kong commands.

**Why not:** every consumer binary would require that framework and its
dependency tree, and a binary built on another framework would require two
frameworks. The
standard library's `flag` covers each command's grammar with a parse loop
of a few lines, and a command that parses its own arguments mounts under
any framework.

### The host parses the flags

Each kernel command exposes its flags as a `flag.FlagSet`, and its host
parses them: cobra through pflag's `AddGoFlagSet`, and a dispatcher over
the standard library directly. A framework then completes the eidos flags
in a shell.

**Why not:** every host would produce the usage errors, the 64 status and
the help text itself, so one mistake would exit 1 in one binary and 64 in
another. The shell completion of the eidos flags under a framework is what
the chosen design gives up. Two consumers that ask for that completion
would reopen this.

### An adapter module for each framework

A module such as `eidos-cli-cobra` mounts the seven commands in a cobra
tree, and one module for each other framework does the same.

**Why not:** the adapter is about fifteen lines over three fields of a
cobra command, and a module for it ties each `eidos-cli` release to
cobra's releases. `acceptancetest` already checks the wiring on the built
binary. Two consumers that copy the same adapter would reopen this.

### External commands on the PATH

The binary runs `acme-<name>` from the PATH for a command that it does not
know, as git runs `git-<name>`.

**Why not:** an external command runs in a process of its own, which has
no access to the composition that the binary compiles in, so it cannot
build the workspace.

### The command line inside the kernel module

`cli` is a package of `go.dokimi.dev/eidos/core`, and the kernel requires a
YAML library.

**Why not:** every satellite and plugin module that requires the kernel
then lists the YAML module in its build list, although none of them reads
a config file. The kernel's runtime takes no third-party dependency,
because the kernel appears in every consumer's supply-chain audit.

### A YAML subset parser in the kernel

The kernel reads the subset of YAML that the config file uses: block and
flow collections, plain and quoted scalars, and comments.

**Why not:** an established parser with no dependencies of its own already
reads YAML. A subset parser rejects valid YAML that a person writes from
habit, such as an anchor, and keeps a second parser to test and fuzz. We
would also have to define which YAML 1.1 behaviours the subset keeps.

### A JSON or TOML config file

The config file is JSON, which the standard library reads, or TOML.

**Why not:** the config format is decided as YAML with a published JSON
Schema. JSON allows no comments, and a config file states its reasons in
comments.

### A lock file created with `O_EXCL`

A run creates `.<brand>/lock` with `O_CREATE|O_EXCL` and removes it at the
end, as git creates `index.lock`.

**Why not:** a crash or a `SIGKILL` leaves the file behind, and the next run
fails until a person removes it. A process ID in the file does not prove
that the holder is alive, because the ID can be reused, and the file can be
on another host. `flock(2)` and an exclusive open both end with the process.

### Narrowing through the plans' sources

A run with patterns intersects every plan's sources with the patterns, so
the plans see only the narrowed packages.

**Why not:** a file that accumulates contributions from more than one
package, such as a per-plan registry, would come out with the narrowed
packages' contributions alone, and a run would commit those bytes. The run's
generation would also record admissions that differ from a whole run's, so
the next whole run would run every plan whole.

### A narrowed run that records what it withheld

A narrowed run writes a generation, and lists the groups whose changes it
withheld as pending, so that the next run executes them.

**Why not:** the plans table lists whole plans, and a plan that it lists
runs whole on the next run. Recording pending groups needs the records of a
withheld group's contributors to remain as the previous generation wrote
them while its other groups' records advance, and contributors can serve
both. A narrowed run that writes no generation costs the next run the
edit's dirty work again, and keeps every record consistent.

### `explain` by running the workspace in memory

`explain` runs every phase into a memory sink and reads the graph, the facts
and the emit stores.

**Why not:** it costs a run, cold where no state exists, for a question
that one read of the sealed state resolves. It also explains the tree as it
is, while a diagnostic that a person asks about came from the run that
reported it.

### Edge spellings beside the readers rows

Each readers row also stores the spelling of its edge, so `explain` prints
every read.

**Why not:** the readers table has one row per edge, so every edge would
add its spelling, about 40 to 80 bytes for an identity by estimate, to the
state. Matching hashes spells the common reads without that cost.

### `doctor` over the recorded findings

`doctor` reads the findings that the generation records, without a run.

**Why not:** the records describe the last run that wrote a generation, and
a suppression is unused against the tree as it is now. A dry warm run costs
about as much as a warm run and reports against the current tree.

### File notifications for `watch`

`watch` subscribes to the operating system's file notifications.

**Why not:** the kernel takes no dependency and keeps no resident process,
and the stat sweep is already the change detector. A notification feed is
the event that would reopen this.

### One JSON document per command

A command writes one JSON document when it ends.

**Why not:** `watch` never ends, and a long run reports nothing until it
ends. Line-delimited events let a consumer stream them.

### Other exit statuses

An interrupted run exits 130, and a config error exits 78, the sysexits
status for a configuration error.

**Why not:** each status means one thing. An interrupted command did not
finish, which 1 states, and a person fixes a config error and a usage error
in the same way, by changing the invocation or its input.

### Suppression by file or by workspace

A directive on a file, or a config key, removes a code everywhere below it.

**Why not:** a broad suppression hides findings that nobody reviewed. A
check that is wrong across a workspace is removed from the composition.

### A hermetic environment for each process

`Exec` passes PATH, HOME and the variables of a list that the fixture adds,
and no other variable of the test process.

**Why not:** a consumer whose frontends locate their stores through the
environment, such as a composition with the Java frontend, copies each of
those variables from the environment of its test into the list. The suite
then runs with the same values as an inherited environment. A variable of a
developer's environment that changes the output of a binary would reopen
this.

## Drawbacks

- A new module, `eidos-cli`, with 27 production files and their tests,
  nine files of hosts under the testdata of `acceptancetest`, and one
  dependency, `go.yaml.in/yaml/v3`.
- A cobra host writes about fifteen lines of wiring, and the contract is
  true only where the wiring meets the host's obligations. `acceptancetest`
  checks them on the built binary, and no compiler does.
- Under cobra, the eidos flags have no shell completion, and `acme help
  run` prints the command's `Usage` text without cobra's table of flags.
- Under cobra, a host's persistent flag written before a kernel command's
  name is passed to the command unparsed, and the command reports it as a
  usage error.
- The shared flags before a command's name are part of `Main`'s grammar
  alone. A script that writes them there against a `Main` binary breaks
  when the binary moves to a host whose dispatcher does not pass them.
- eidos cannot refuse a host that mounts a kernel command under another
  name. `acceptancetest` checks the names instead.
- `go.yaml.in/yaml/v3` is frozen and takes security fixes alone. Its
  maintainers recommend v4 for new projects, and v4 has only release
  candidates, the latest v4.0.0-rc.6. We take v3.0.5, the latest stable
  release.
- The messages of the config decoder include the Go type of a section, such
  as `field sourcse not found in type config.Plan`.
- A run that narrows its commit, or selects plans, writes no generation. In
  a workflow that only ever runs narrowed, such as one `go generate` line in
  every package, each run executes again the edits since the last whole
  run.
- A manifest lists each source as the spelling of its identity, and the
  spelling of a package whose last path segment contains a dot, such as
  `example.com/api.v2`, parses back as another package. No pattern admits
  such a package, so a narrowed run withholds its changes, and a whole run
  commits them.
- `watch` keeps 13% to 19% of one core busy over a tree of 100,000 files at
  the default interval.
- `explain` cannot spell a read outside a record's neighbourhood, such as a
  name entry or a directory's residents, and counts it instead.
- The lock serialises a run and `explain` over one state directory, so a
  long run blocks `explain` until it ends.
- The Windows lock path has no test in CI, which runs on Linux alone.
- The kernel gains eight `Input` fields, two `Report` fields and two
  `PlanReport` fields, one plan status, five codes, `Locker` and its record,
  `Overwriter`, `StoreLocator`, `Describe`, `Explain` and their types,
  `ParseTarget`, `Stores`, `BrandName`, `frontend.Builder.Stores` and
  `meta.Authority.String`.
- A dry run creates `.<brand>/` for its lock in a tree that has none, so
  `run --check` in a fresh clone leaves the empty lock file behind.
- `version` prints no contract version, because the kernel declares none.
- `acceptancetest` checks no JSON output of `watch`, because `watch` runs
  until an interrupt.
- A binary under `acceptancetest` inherits the environment of the test, so
  a variable that changes the output of the binary changes the result of
  the suite from one machine to another.

## Unresolved and future work

- The contract version and the Build handshake that would check it are not
  proposed here.
- A config file assembles no plan from compiled-in parts, and refines no
  layout per plugin or per family.
- The published JSON Schema states no plugin's options.
- A workspace scope in the config file, which would limit what the load
  reads, is not proposed here.
- `doctor`'s comparison of directive usage with the next release's schema
  index is not proposed here.
- Shell completion of the eidos flags under a framework is not proposed
  here.

## References

| What | Where |
|---|---|
| The command kernels: composition, discovery, flags, commands and machine output | [20-cli.md](../architecture/20-cli.md) |
| Distribution: a library that ships no binary | [14-distribution-and-cli.md](../architecture/14-distribution-and-cli.md) |
| Diagnostics: severities, exit codes, panics and suppression | [16-diagnostics.md](../architecture/16-diagnostics.md) |
| Output: the sink, drift, adoption, the lock and dry runs | [17-output-and-determinism.md](../architecture/17-output-and-determinism.md) |
| Workspaces: config, sources, lists of workspaces and the narrowed sweep | [08-workspace-and-plans.md](../architecture/08-workspace-and-plans.md) |
| Compatibility: schemas as API and deprecation | [15-compatibility.md](../architecture/15-compatibility.md) |
| Directives: the kernel's `diag` directive | [05-directives.md](../architecture/05-directives.md) |
| Testing: acceptancetest | [13-testing-and-conformance.md](../architecture/13-testing-and-conformance.md) |
| The kernel's zero-dependency runtime and eidos-lang | [01-repos-and-kernel.md](../architecture/01-repos-and-kernel.md) |
| D2, D9, D12, D25, D30, D42, D59, D70, D71, D75, D79 and D111: no binary, no daemon, YAML, suppression, drift, one writer, narrowing, brand scoping, the narrowed sweep, exit codes, the check and the warm gates | [21-decisions.md](../architecture/21-decisions.md) |
| The sealed state, its tables and its measured costs | [RFC-0020](0020-warm-runs-and-the-sealed-state.md) |
| sysexits(3): `EX_USAGE` is 64 | https://man.freebsd.org/cgi/man.cgi?query=sysexits&sektion=3 |
| flock(2): release semantics, `LOCK_NB` and NFS | https://man7.org/linux/man-pages/man2/flock.2.html |
| Go, `flag.ExitOnError` exits with status 2 | https://github.com/golang/go/blob/go1.27.1/src/flag/flag.go |
| Go, the runtime exits with status 2 on a fatal panic | https://github.com/golang/go/blob/go1.27.1/src/runtime/panic.go |
| Go, the go command's file locks over `flock` and `LockFileEx` | https://github.com/golang/go/tree/go1.27.1/src/cmd/go/internal/lockedfile/internal/filelock |
| Go, `go generate` runs a command in the package's directory | https://github.com/golang/go/blob/go1.27.1/src/cmd/go/internal/generate/generate.go |
| Go, `errors.AsType` | https://github.com/golang/go/blob/go1.27.1/src/errors/wrap.go |
| cobra v1.10.2: `DisableFlagParsing`, `SilenceErrors` and `SilenceUsage` | https://github.com/spf13/cobra/blob/v1.10.2/command.go |
| pflag v1.0.10: `AddGoFlagSet` | https://github.com/spf13/pflag/blob/v1.0.10/golangflag.go |
| urfave/cli v3.14.0: `ExitCoder`, and `HandleExitCoder`, which prints a message only where it is not empty | https://github.com/urfave/cli/blob/v3.14.0/errors.go |
| urfave/cli v3.14.0: `SkipFlagParsing` | https://github.com/urfave/cli/blob/v3.14.0/command.go |
| git(1): the `PATH` variable and external commands named `git-<name>` | https://git-scm.com/docs/git |
| go.yaml.in/yaml/v3, the YAML organisation's maintained continuation of go-yaml | https://github.com/yaml/go-yaml |
| Terraform's machine-readable UI: a version message first, minor versions additive | https://developer.hashicorp.com/terraform/internals/machine-readable-ui |
| JSON Lines | https://jsonlines.org/ |
| NO_COLOR | https://no-color.org/ |
| Staticcheck: an ignore directive that matched nothing is reported | https://staticcheck.dev/docs/configuration/#ignoring-problems |
| golangci-lint: config files searched from the working directory up to the root | https://golangci-lint.run/docs/configuration/file/ |
