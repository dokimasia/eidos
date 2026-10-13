---
rfc: 0064
title: The command line on go-cli
author: Roy Klopper <roy.klopper@stealthscale.io>
status: Accepted
created: 2026-10-13
updated: 2026-10-13
discussion: none
supersedes: none
superseded-by: none
produces-adr: ADR-0048, ADR-0049
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# RFC-0064: The command line on go-cli

## Summary

Every binary built with eidos runs the eight kernel commands. `eidos-cli` parses their flags with the
package `flag` of the standard library and writes their help itself. It returns each command as a
value that any command line can mount, a cobra tree included. We propose that a binary built with
eidos is an app of go-cli, the module `go.stealthscale.io/cli`. `cli.New` returns the app with the
kernel commands. A consumer adds its own commands to the app as go-cli commands. `cli.Main` runs the
app. go-cli derives the flag, the environment variable and the config key of each option from a
struct field. It reads the values of the options from the config file of the workspace. eidos decodes
the workspace document from the same file. The exit statuses 0, 1 and 64 and the JSON events of the
commands do not change. Mounting in another framework, the command `help` and the single-dash form
of a long flag are out. go-cli v0.4.0 selects the renderer of a run from its options and skips the
keys that an app decodes itself. It calls a search of the app for its config file. It returns the
renderer of a run to a caller of `Execute`.

## Motivation

- Each kernel command parses with one `flag.FlagSet` and a loop that parses again after each
  positional argument because `FlagSet.Parse` stops at the first argument that is not a flag. The
  loop, the help text of each command and the list of commands are in `kernel.go`, `flags.go` and
  `main.go`. They have 461 lines in all.
- The command line does not have short flags, suggestions for a misspelled flag or command, or
  environment variables.
- A team cannot set an option such as `--strict` in the config file of its workspace.
- A command of a consumer registers the shared flags on a `flag.FlagSet` of its own and writes the
  parse loop again.
- go-cli derives the flag, the environment variable and the config key of each option from a struct
  field. It applies the default, the config file, the variable and the flag of each option in one
  order. It suggests a near name for an unknown flag, command or key. Its package `clitest` runs a
  program in a test. Its package `cli` imports the standard library alone.
- A consumer that writes its commands as go-cli commands gets the same flags, help and errors as
  the kernel commands. Every binary built with eidos then adds to the users of go-cli.

## Detailed design

### Components

| Component | Change |
|---|---|
| `eidos-cli`, package `cli` | `Global`, `New`, `Prepare`, `Mount`, `Main`, `Run` and `IOOf` replace `Flags`, `Kernels`, `Command`, `ProcessIO`, and the former `Main` and `Run`. Each kernel command is a go-cli command with a struct of options |
| `eidos-cli/internal/config` | `Decode` skips the top-level keys that are not keys of the format, and `DecodeMember` refuses them in the config file of a member of a list. `Keys` returns the keys of the format. The JSON Schema admits other top-level keys |
| `eidos-cli/acceptancetest` | The test binaries are go-cli apps. `AssertNames` compares the help of each kernel command with the help of a group that `Mount` fills |
| `eidos-cli/go.mod` | Requires `go.stealthscale.io/cli` v0.4.0 and `go.stealthscale.io/cli/yaml` v0.1.3 |
| `eidos-conformance/lang/go` | The acceptance binaries are go-cli apps, and `Crash` is the handler of a go-cli command |
| `eidos-reference/cmd/eidos-reference` | Calls `cli.New` and `cli.Main` |
| `docs/architecture/20-cli.md` | The command line, the config file, the mounting and the global options |

### Invariants

1. A binary built with eidos exits 0 when its command succeeds, 1 when findings or errors fail the
   command and 64 for a usage or config error. No eidos code returns 2. A Go panic exits with 2.
2. Under `--format=json`, standard output contains JSON lines alone, from the `start` event to the
   `summary` event. This applies also to an error that go-cli finds in the command line or in the
   config file. Standard error is empty unless the process panics, the command line contains
   `--flag-debug`, or the command line ends at a command without a handler, where go-cli writes the
   help to standard error. A run with `-h` or `--help` writes the help as text.
3. Each flag of the app other than `-h` has one environment variable. Each flag other than `-h`,
   `--chdir`, `--config` and `--flag-debug` has one key in the config file. go-cli derives the flag,
   the variable and the key from the name of the app, the path of the command and the name of the
   field.
4. The value of an option comes from the last of its default, the config file, its environment
   variable and its flag that sets it.
5. One check covers each key of the config file. eidos checks the keys of the workspace document,
   and go-cli checks every other key against the keys of the options. A key that neither declares
   fails the run.

### The app

The package `cli` imports go-cli under the name `gocli` because both packages are named `cli`:

```go
// Global are the global options of a binary built with eidos. Every command of the app accepts
// them before and after its name. go-cli reads each option from its flag, its environment variable
// and its key in the config file, such as --strict, ACME_STRICT and strict for the brand acme.
type Global struct {
    // Config is the path of the config file. A run that sets neither the flag nor the variable
    // takes the path that the search of Prepare finds, and the empty path when the search does
    // not find a file. go-cli reads a relative path against the working directory of the process,
    // also under Chdir.
    Config gocli.ConfigFile `help:"Read this config file instead of the one that the search finds."`
    // Chdir is the working directory of the command, and empty for the working directory of the
    // process. A relative path is relative to the working directory of the process. The search for
    // the config file starts at it, and the patterns of a command are relative to it.
    Chdir string `help:"Run in this directory instead of the working directory." short:"C" key:"-"`
    // Format is the output format, FormatText or FormatJSON.
    Format Format `help:"Write the output as text or json." default:"text"`
    // Strict reports every Warning as an Error.
    Strict bool `help:"Report every warning as an error."`
    // Verbose renders the Info findings.
    Verbose bool `help:"Render info findings."`
    // Quiet renders the Error findings alone. A command refuses Quiet with Verbose.
    Quiet bool `help:"Render error findings alone."`
    // NoColor renders text without colour.
    NoColor bool `help:"Render text without colour."`
}

// New returns a go-cli app for the command line of a binary, with the help text help. The name
// of the app is the brand of the compositions that compose returns, so the brand acme reads the
// variable ACME_STRICT. New prepares the app with Prepare, and adds the kernel commands run, plan,
// explain, prune, doctor, watch, version and schema at its root.
func New(help string, compose Compose) *gocli.App[Global]

// Prepare registers on app what a binary built with eidos needs from go-cli, for an app that a
// binary creates itself:
//
//   - the YAML format of go.stealthscale.io/cli/yaml for the config file.
//   - the keys of the workspace document, which go-cli skips in the config file, because Open
//     decodes them.
//   - the search for the config file of the brand of compose, as Open searches.
//   - the renderer that Run reads the format of a run from.
//   - a hook that checks the working directory of each run, and makes a relative Config absolute
//     against the working directory of the process, where go-cli read the file. Open then reads
//     the same file under Chdir.
//
// Prepare calls compose once, for the brand.
func Prepare(app *gocli.App[Global], compose Compose)

// Mount adds the eight kernel commands as subcommands of parent, over compose. A binary whose
// command gen is parent then runs acme gen run. The app of parent needs the registrations of
// Prepare.
func Mount[C any](parent *gocli.Command[Global, C], compose Compose)

// Main runs app over the Env of the process through Run, under a context that the first interrupt
// or termination signal cancels. It exits the process with the status that Run returns.
func Main(app *gocli.App[Global], compose Compose)

// Run runs the command line of env with app under ctx, and returns the exit status of the command.
// It renders the error of each status other than an ExitError through Begin, Renderer.Error and
// Renderer.End, in the format and the colour of the global options that the run read before its
// fault.
//
// Run panics for an error of the run that wraps gocli.ErrDeclaration or gocli.ErrEnv, such as two
// commands with one name or an Env without Stdout. Each is a defect of the binary.
func Run(ctx context.Context, app *gocli.App[Global], compose Compose, env *gocli.Env) int

// IOOf returns the IO of run: its standard streams, the working directory of the command, the
// environment of the run, and whether its standard error is a terminal. The working directory is
// Chdir, resolved against the working directory of the process, or the working directory of the
// process for an empty Chdir.
//
// Error modes: IOOf returns an IO without a Dir, and an error that wraps a *UsageError, when
// os.Getwd fails, when the directory does not resolve and when it is not a directory.
func IOOf[C any](run *gocli.Run[Global, C]) (IO, error)
```

The search of `Prepare` receives the global options of the run. It starts at `Chdir` or at the
working directory as `IOOf` does. It returns the file that the search of `Open` finds. go-cli opens
that file through `Env.Open` and sets `Config` to its path. `--flag-debug` shows the source `located`
for the path. `Open` then reads the file of `Config`.

The getter of an environment variable that `IOOf` returns reads the pairs of `Run.Environ`. It
returns the value of the first pair of a name and compares names without regard to case on Windows.
`os.Getenv` works the same way.

`IO`, `Open`, `Begin`, `Renderer`, `Member`, `Configure`, `ConfigName`, `ConfigError`, `Schema`,
`Compose`, `Format`, `ErrFormat`, `UsageError`, `ExitError`, `Exit` and the three statuses keep their
contracts, with these changes:

- `Open` and `Begin` take a `Global` in place of `Flags`.
- `Begin` requires the two streams and `Getenv` but not a `Dir`. A command renders the error of
  `IOOf` through it.
- The root of the config file of a `Config` is the directory of its path, with symbolic links
  resolved. A `Config` of the file that the search finds then gives the root of the search, also
  when the file is a link.
- `Open` and `Configure` skip the keys of the options in the config file of the run. `Open` refuses
  them in the config file of a member of a list.
- The holder record of the lock contains the path of the command without its arguments. A handler
  receives the options of its run and not its command line.

A binary calls `New` and `Main`. It adds its own commands as go-cli commands. `compose` composes the
workspace, as it does in every binary today:

```go
func main() {
    app := cli.New("Generates the code of the acme platform.", compose)
    app.Command("install", "Installs the acme hooks.", install)
    cli.Main(app, compose)
}

// installed is the event that install renders for each workspace.
type installed struct {
    Root string `json:"root"`
}

// install reads the workspaces through Open and renders through Begin, as a kernel command does.
func install(ctx context.Context, run *gocli.Run[cli.Global, struct{}]) error {
    stdio, err := cli.IOOf(run)
    r := cli.Begin(stdio, *run.Global(), compose, run.Command())
    status := cli.StatusOK
    if err == nil {
        var members []cli.Member
        members, err = cli.Open(stdio, *run.Global(), compose)
        for _, m := range members {
            r.Event("installed", &installed{Root: m.Root}, "installed "+m.Root)
        }
    }
    if err != nil {
        r.Error(err)
        status = cli.StatusUsage
    }
    r.End(status)
    return cli.Exit(status)
}
```

A binary that mounts the kernel commands as subcommands of a group creates and prepares the app
itself:

```go
app := gocli.New[cli.Global]("acme", "Runs the acme platform.")
cli.Prepare(app, compose)
cli.Mount(app.Command[struct{}]("gen", "Generates the code of the platform.", nil), compose)
cli.Main(app, compose)
```

### The config file

The config file of a workspace contains the workspace document and the values of options:

```yaml
version: 1
strict: true
run:
    plan: [api]
plans:
    web:
        enabled: false
```

go-cli reads the keys of options `strict` and `run.plan`. It skips `version` and `plans` because
`Prepare` registers the keys of the document as keys that the app decodes itself. eidos decodes
`version` and `plans` as before and skips every other top-level key:

| Key | Checked by | A key that does not exist |
|---|---|---|
| `version`, `workspace`, `workers`, `ignore`, `memo`, `plans`, `policies`, `options`, `extensions`, `workspaces`, and every key below them | eidos, with the line of each fault | A config error, status 64 |
| Every other key | go-cli, against the keys of the options of the app | A usage error with a suggestion, status 64 |

The search for the file runs once for each run in the function that `Prepare` registers. It starts
at `Chdir` or at the working directory and checks each parent directory for `.<brand>.yaml`. It
stops after the first directory with a version control marker. The search of `Open` takes the same
steps. `--config` and the variable `ACME_CONFIG` replace the search.

A file that lists workspaces can set the options of the whole run. It has no other key of a
document. The config file of a member of a list does not set options because go-cli reads one file
for each run. eidos decodes the file of a member strictly. A key of an option there is a config
error: `svc/.acme.yaml:2: field strict not found in type config.Document`.

The JSON Schema that `schema` prints admits every top-level key that is not a key of the format, such
as an option of a kernel command or of a command of the consumer. go-cli checks those keys when a run
reads the file. The schema of a document refuses the key `workspaces`. The schema of a list refuses
the other keys of a document. A file then matches one of the two at most. The schema does not list
the options of the commands.

### The kernel commands

Each kernel command is a go-cli command with an unexported struct of options:

| Command | Options | Positional arguments |
|---|---|---|
| `run` | `--plan`, `--dry-run`, `--check`, `--overwrite-drift`, `--adopt`, `--cold`, `--stdout` | `PATTERNS`, optional |
| `plan` | none | none |
| `explain` | none | `TARGET`, required |
| `prune` | `--dry-run` | none |
| `doctor` | none | none |
| `watch` | `--interval`, 1s by default | `PATTERNS`, optional |
| `version` | none | none |
| `schema` | none | none |

```go
// runOptions are the options of run.
type runOptions struct {
    // Plans are the plans that the run runs with the plans that they depend on. Each repeated
    // flag adds its values, and --plan web,cli adds two.
    Plans []string `help:"Run this plan and the plans that it depends on." name:"plan"`

    DryRun         bool `help:"Run every phase and commit nothing."`
    Check          bool `help:"Commit nothing and report each file that the run would change as an error."`
    OverwriteDrift bool `help:"Write over a generated file that was edited since its stamp."`
    Adopt          bool `help:"Write over a file without the frame of the brand."`
    Cold           bool `help:"Run without the state that the last run recorded."`
    Stdout         bool `help:"Record nothing and write the files to standard output."`
    // Patterns are the patterns of the run, and empty for a run without patterns.
    Patterns []string `arg:"" default:"" help:"Patterns of the packages to run. Each is relative to the working directory."`
}
```

`--plan` takes a list. Each repeated flag adds its values. `--plan web,cli` names two plans. An empty
default makes `PATTERNS` optional. A run without patterns receives an empty list.

A handler of a kernel command does what `Run` of a kernel did once its flags had parsed:

1. It calls `IOOf` and `Begin`.
2. It checks the options that exclude each other.
3. It runs the command and renders its output.
4. It ends the output and returns `Exit` of the status.

An excerpt of the help of `run` in the reference binary and the error of a misspelled flag:

```console
$ eidos-reference run -h
Usage: reference run [options] [PATTERNS...]

Generates the output of each workspace and commits it.

Arguments:
      PATTERNS  Patterns of the packages to run. Each is relative to the working directory.  default ""

Options:
      --adopt            Write over a file without the frame of the brand.  env REFERENCE_RUN_ADOPT, key run.adopt
      --check            Commit nothing and report each file that the run would change as an error.  env REFERENCE_RUN_CHECK, key run.check
      --plan string,...  Run this plan and the plans that it depends on.  env REFERENCE_RUN_PLAN, key run.plan

$ eidos-reference run --plna api
error: cli: run: unknown flag --plna (did you mean --plan?)
```

The note `key run.adopt` is the key of the option in the config file of the workspace.

### Errors and statuses

`Run` calls `app.ExecuteRenderer`. It returns the renderer of the run with its error. The function
that `Prepare` registers through `RenderFor` built that renderer from the global options of the run.
The renderer then contains the format and the colour of the run. `Run` maps the error of the run:

| The run returns | `Run` renders | Status |
|---|---|---|
| nil | nothing | 0 |
| an `ExitError` of a handler | nothing, because the handler rendered its output | its code |
| an error that wraps `gocli.ErrDeclaration` or `gocli.ErrEnv` | nothing. `Run` panics | 2 |
| an error that wraps a `*gocli.Error` | a usage error through `Begin`, `Renderer.Error` and `End`, with the path of the command of the `*gocli.Error` in the `start` event | 64 |
| an error of a handler or of the search that wraps a `*UsageError` | the error as a usage error, without a command in the `start` event | 64 |
| any other error | the error, without a command in the `start` event | 1 |

The message of a usage error from go-cli is the text of its error, as in
`cli: run: unknown flag --plna (did you mean --plan?)`. Text output writes the error alone to
standard error. `-h` writes the help. A run of an app without `Prepare` has another renderer. `Run`
renders its failure as text.

go-cli's `App.Main` writes the failure of a run to standard error through the renderer of the run.
Under `--format=json` standard output would then lack the events of a usage error. `App.Main`
returns 1 for every error of a handler. A handler that returns the status 64 would exit 1. It returns
2 for a usage error. A Go panic also exits 2. For these reasons a binary built with eidos calls `Run`
and not `App.Main`.

### What go-cli provides

go-cli v0.4.0 has the four capabilities that the port needs:

| go-cli v0.4.0 | Use in eidos |
|---|---|
| `App.RenderFor(func(global *G) Renderer)` selects the renderer of each run from its global options. A run applies every valid value of its sources before the selection, also when its command line fails, so a flag before the fault counts | `Prepare` registers a function that returns a renderer with the global options of the run |
| `App.IgnoreKeys(keys ...string)` skips the config keys that the app decodes itself, and refuses an ignored key that overlaps the key of an option | `Prepare` registers the ten keys of the format |
| `App.LocateConfig(func(global *G, env *Env) (string, error))` returns the path of the config file when neither its flag nor its variable sets it | `Prepare` registers the search for `.<brand>.yaml` |
| `App.ExecuteRenderer(ctx, env) (Renderer, error)` runs a command line as `Execute` does and returns the renderer of the run | `Run` renders the failure of a run with its own streams and statuses |

With options in the config file, the help notes a real key for each option.

### Verdicts

| Item | Verdict | Reason | Reverses the verdict |
|---|---|---|---|
| A binary built with eidos as a go-cli app | in | go-cli derives the flags, the variables, the config keys and the help from the options, and a consumer writes its commands in the same form | A requirement of a consumer that go-cli cannot meet |
| Mounting the kernel commands in another framework, such as cobra or urfave/cli | out | A kernel command is a go-cli command with a typed handler, and eidos keeps one host. A consumer with another framework runs the binary as a process | A consumer whose binary has to keep another framework |
| The global options before and after the name of a command | in | go-cli reads a global flag anywhere | none |
| An environment variable for each option | in | go-cli derives one for every option, and `--flag-debug` shows the source of each value | A run that a variable from the parent environment changes without a user noticing |
| The values of options in the config file of the workspace, through go-cli | in | One file contains the document and the options, every option follows one order of sources, and `--flag-debug` names the key and the file of each value | none |
| The workspace document as options of go-cli | out | The document has keys that exist only at run time, nested values, a list of workspaces and faults with their lines. An option of go-cli is a field of a struct whose value converts from text | go-cli options with nested values and the lines of faults |
| A JSON Schema that admits the top-level keys of options, and lists none | in | eidos does not know the options of the commands of a consumer, and go-cli checks every key of an option when a run reads the file | An API of go-cli that lists the options of an app |
| `-C`, `--chdir` | in | A test sets the working directory of a command without a change of the process directory, which serial tests alone can make. `go -C` and `git -C` have the same flag | A working directory in go-cli's `Env` |
| A relative `--config` against the working directory of the process, also under `-C` | in | go-cli opens a path that the flag or the variable sets before it applies the other flags, so the directory of `-C` is not known when it opens the file | A working directory in go-cli's `Env` |
| Short flags other than `-C` and `-h` | out | No user has asked for one | A request from a user |
| The command `help` and `<binary> help <command>` | out | go-cli writes the help of the app and of each command for `-h` and `--help` | A script that calls `help` |
| The single-dash form of a long flag, such as `-format=json` | out | go-cli follows GNU, so `-f` is a short flag | none |
| `--flag-debug` | in | go-cli adds it to every app. It writes its table to standard error, also under `--format=json` | none |
| The renderer `Plain` for the help | in | The renderer `charm` adds Lip Gloss to the build of every binary. A consumer registers it on its app where it wants it | none |
| go-cli's `App.Main` as the entry point | out | It writes a failure to standard error, returns 1 for every error of a handler, and returns 2 for a usage error | go-cli writing a failure to standard output under the renderer of a run, and returning the status of a handler and 64 for a usage error |
| `App.ExecuteRenderer` for the failure of a run | in | `Run` renders a usage error in the format of the renderer of the run, on its own stream and with its own status | none |
| `App.Failure` | out | `Run` renders the text of the error of a run, which already contains the command and the suggestion | A renderer of eidos that writes the parts of a failure apart |
| The command line of the run as the caller in the record of the lock | out | A handler receives the options of its run and not its arguments. `Run` would have to pass `Env.Args` to every handler through its context | A user who has to tell two runs of one command apart by the record |

### Failures

| Condition | Report |
|---|---|
| An unknown flag or command, a missing or an extra argument, a value that does not convert | A usage error from `Run`, status 64 |
| A top-level key of the config file that is neither a key of the document nor the key of an option | A usage error from `Run` with go-cli's suggestion, status 64. `doctor` reports it the same way, because go-cli fails the run before the handler |
| A key of an option in the config file of a member of a list | A config error, status 64 |
| A config file that the search finds and cannot read | A usage error from the search, status 64 |
| `--quiet` with `--verbose`, `--stdout` with `--format=json`, `--dry-run` or `--check`, an interval below 100ms | A usage error from the handler, status 64 |
| A `Chdir` that is not a directory, or a working directory that `os.Getwd` cannot read | A usage error from the search or from the hook of `Prepare`, status 64. In an app without `Prepare`, `IOOf` reports it to the handler |
| A declaration that go-cli refuses, such as two commands with one name, or a command of a consumer whose options have a key inside a key of the document, such as `plans.x` | A panic in `Run`, status 2 |
| A brand that go-cli refuses as the name of an app, such as `acme-`, which ends in a hyphen | A panic in `Run`, status 2. A brand admits a trailing hyphen and two hyphens in a row, and a name of go-cli does not |

### Migration

1. Require `go.stealthscale.io/cli` v0.4.0 and `go.stealthscale.io/cli/yaml` v0.1.3.
2. Add `Global`, `New`, `Prepare`, `Mount`, `Main`, `Run` and `IOOf`. Move each kernel command onto
   a struct of options and a handler.
3. Let the decoder of the config file skip the keys of options and refuse them in the file of a
   member of a list. Open the top level of the JSON Schema to them.
4. Remove `Kernels`, `Command`, `Flags`, `ProcessIO`, the dispatch, the parse loop and the help
   text.
5. Port the reference binary and the test binaries of the acceptance kit and of the conformance
   module.
6. Describe the command line and the config file in the CLI document.

## Alternatives

### The workspace document as options of go-cli

go-cli could read the whole config file with each key of the document as an option. An option of
go-cli is a field of a struct. Its value converts from text. The document has keys that exist only
at run time, such as the names of plans and the extensions of the commands of a consumer. The
document also has objects inside those maps and a list of workspaces. eidos reports each fault of
the document with its line. go-cli v0.4.0 refuses a map of structs and a list of structs. It reports
a fault in a file without its line.

### The values of options in the workspace document

eidos could declare `strict` and the other options as keys of its document. It would apply them
before the variables and the flags. eidos would then restate the sources of an option and their
order. go-cli has both.

### A JSON Schema with the options of the kernel commands

The schema could list `strict`, `run.plan` and the other keys of the kernel options beside the
document. An editor would then complete them. eidos would derive each key from its field a second
time. It would restate the kebab case of the name, the path of the command and the tags `name` and
`key`. go-cli has no API that lists the options of an app. The options of the commands of a consumer
would still be missing.

### A relative --config against the directory of -C

`go -C` and `git -C` change the directory before they read a path of the command line. go-cli opens a
path that `--config` sets before it applies the other flags of the run. `-C` is not applied when it
opens the file. eidos would read another file than go-cli unless go-cli knew the directory. go-cli
would need a working directory in its `Env`.

### go-cli's App.Main as the entry point

A binary could call go-cli's `app.Main`. The renderer of eidos that `RenderFor` selects would write
each failure. go-cli would then need three changes instead of one:

- `App.Main` writes a failure to standard output under JSON.
- `App.Main` returns the status of a handler and 64 for a usage error.
- A handler builds a usage error with its own message. `gocli.Error` has its message in an
  unexported field.

Each of the three changes the behaviour of every go-cli app. `Run` keeps the behaviour in eidos.

### App.Main with a standard error that records the run

`Run` could call `app.Main` with an `Env` whose standard error is a writer of eidos. The writer
would contain the standard output and a slot for the status of the run. The renderer of eidos would
assert the type of the writer that `Renderer.Error` receives. It would write a JSON failure to the
standard output of the writer and record the status of the run in the slot. That works with go-cli
v0.3.0. It depends on go-cli passing the writer of the `Env` unchanged to the renderer. A method that
returns the renderer of the run states that contract in the API of go-cli.

### The kernel commands as values over go-cli

`eidos-cli` could keep `Command` and `Kernels` for foreign hosts. Inside `Command.Run` a go-cli app
of one command would parse the arguments of each kernel command. go-cli has no command that receives
its arguments unparsed. A command of a consumer would still parse its own flags. The binary would
then have two kinds of command. The help of the app would not list the commands of the consumer.

### Suggestions and short flags on the package flag

The parse loop could gain short flags, suggestions and variables. The result would be a parser that
duplicates go-cli. eidos would maintain it.

### The working directory in go-cli's Env

go-cli's `Env` could contain a working directory for `IOOf` to read. A run of go-cli opens its input
files through `Env.Open`. A working directory in `Env` would change how a run opens a relative path.
The flag `-C` gives a test the same control without a change of go-cli. It also gives a user the flag
that `go` and `git` have.

## Drawbacks

- The `main` of every consumer changes. Each command of a consumer moves from the `Command`
  interface to a go-cli handler.
- A consumer cannot add a global option of its own because the global options of the app have the
  type `Global`. An option of a consumer is an option of its command.
- A script that passes `-format=json` or calls `help` fails with a usage error.
- A binary named acme reads 17 environment variables: 8 for the global options with
  `--flag-debug`, 7 for `run`, 1 for `prune` and 1 for `watch`. A variable such as
  `ACME_RUN_DRY_RUN=true` in the parent environment changes a run. A key such as
  `run.dry-run: true` in the config file changes every run of the workspace.
- `eidos-cli` requires two more modules: go-cli and its YAML format. Both are below version 1.
- The JSON Schema admits a misspelled top-level key of an option. go-cli reports the key only when a
  run reads the file.
- `doctor` no longer reports a top-level key that go-cli refuses as one finding among its others.
  go-cli fails the run before the handler.
- go-cli reads the config file for every command. A file that does not parse fails `schema` as well,
  and `--config=` runs a command without a file.
- Under `-C` a relative `--config` is relative to the working directory of the process. The patterns
  are relative to the directory of `-C`.
- The record of the lock contains the path of the command and no longer its arguments.
- The help of an optional list notes `default ""`.

## Open questions

None.

## Unresolved and future work

- A JSON form of the help is not proposed here.
- Shell completion is not proposed here.
- A JSON Schema with the options of the commands needs an API of go-cli that lists the options of an
  app. That API is not proposed here.

## References

- [RFC-0021](0021-the-command-kernels.md), the command kernels, the config file and the mounting in
  another framework.
- [RFC-0045](0045-the-config-file-of-a-binary.md), the config file of a binary.
- [RFC-0046](0046-the-output-of-the-command-kernels.md), the output of the command kernels.
- [RFC-0059](0059-package-structure-and-the-facade.md), the entry point that returns the status.
- [20](../architecture/20-cli.md), the CLI document.
- go-cli: <https://github.com/stealth-scale/go-cli>
- go-cli RFC-0005, a renderer for each run, and the renderer for a caller of `Execute` that it leaves
  out: <https://github.com/stealth-scale/go-cli/blob/v0.3.0/docs/rfc/0005-a-renderer-for-each-run.md>
- go-cli RFC-0006, ignored config keys and a located config file:
  <https://github.com/stealth-scale/go-cli/blob/v0.3.0/docs/rfc/0006-ignored-keys-and-a-located-config-file.md>
- go-cli RFC-0007, the renderer and the failure of a run for a caller of `Execute`:
  <https://github.com/stealth-scale/go-cli/blob/v0.4.0/docs/rfc/0007-the-renderer-and-the-failure-of-a-run.md>
- The options of a failed run: <https://github.com/stealth-scale/go-cli/issues/3>
- Keys that the app decodes itself: <https://github.com/stealth-scale/go-cli/issues/5>
- The search for the config file: <https://github.com/stealth-scale/go-cli/issues/6>
- The renderer of a run for a caller of `Execute`: <https://github.com/stealth-scale/go-cli/issues/8>
