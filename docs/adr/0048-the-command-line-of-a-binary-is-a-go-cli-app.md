---
adr: 0048
title: The command line of a binary is a go-cli app
status: Accepted
date: 2026-10-13
supersedes: none
superseded-by: none
rfc: RFC-0064
---

<!--
  ~ Copyright Dokimasia B.V. 2026
  ~ SPDX-License-Identifier: Apache-2.0
-->

# ADR-0048: The command line of a binary is a go-cli app

## Status

Accepted

## Context

The command kernels depended on no command-line framework. Each kernel command parsed its own
flags with the package `flag` of the standard library, wrote its own help and returned its status.
Any command line could mount it: `cli.Main`, a cobra tree or a dispatcher over the standard library.
The accepted design of the kernels states that rule, the obligations of a host that mounts the
commands, the grammar of the flags and the command `help`. An accepted design is not amended. This
record replaces those parts of it.

The rule conflicted with these needs:

- The parser of the kernels does not have short flags, suggestions or environment variables. A
  command of a consumer registered the shared flags and wrote the parse loop again.
- go-cli derives the flag, the environment variable and the config key of each option from a struct
  field. It suggests a near name for each misspelled input. Its module is `go.stealthscale.io/cli`.
- We maintain go-cli. Every binary built with eidos adds to its users.

## Decision

We will run the command line of every binary built with eidos as an app of go-cli because go-cli
derives the flags, the variables, the config keys and the help of every command from its options.

A consumer writes its own commands in the same form:

- `cli.New` returns the app with the eight kernel commands at its root. `cli.Prepare` registers what
  eidos needs from go-cli on an app that a binary creates itself. `cli.Mount` adds the kernel
  commands as subcommands of a group of that app.
- `cli.Run` runs the app and maps the error of each run to the statuses 0, 1 and 64. It writes a
  usage error that go-cli finds in the format of the run. Under `--format=json` the error is a JSON
  event on standard output.
- `cli.Main` runs `cli.Run` over the process. `cli.Run` panics for a declaration that go-cli refuses.
- `cli.Global` contains the global options. Every command accepts them before and after its name.
  `-C` runs a command in another directory. go-cli adds `--flag-debug` and `-h`.
- `Command`, `Kernels`, `Flags`, `ProcessIO`, the command `help`, the single-dash form of a long flag
  and the mounting in another framework are out.

## Alternatives Considered

### The kernel commands as values over go-cli

`eidos-cli` could keep `Command` and `Kernels` for other hosts and parse the arguments of each
kernel command with a go-cli app of one command inside `Command.Run`. go-cli has no command that
receives its arguments unparsed. A command of a consumer would still parse its own flags. The binary
would have two kinds of command. The help of the app would not list the commands of the consumer.

### Suggestions, short flags and variables on the package flag

The parse loop of the kernels could gain short flags, suggestions and environment variables. It
would then duplicate go-cli. eidos would maintain a second parser.

### go-cli's App.Main as the entry point

A binary could call go-cli's `App.Main` with a renderer of eidos. `App.Main` writes a failure to
standard error. It returns 1 for every error of a handler and 2 for a usage error. A Go panic also
exits with 2. Each change of that behaviour would change every go-cli app. eidos maps the statuses in
`cli.Run` instead.

### App.Main with a standard error that records the run

`cli.Run` could call `App.Main` with a standard error of eidos that also contains the standard output
and a slot for the status. The renderer would assert the type of that writer. go-cli does not state
that it passes the writer of the `Env` to the renderer unchanged. `App.ExecuteRenderer` of go-cli
v0.4.0 returns the renderer of a run. Its API states that contract.

## Consequences

**Positive:**

- The kernel commands and the commands of every consumer share one parser. They have the same help,
  suggestions and errors.
- Each flag other than `-h` has an environment variable. `--flag-debug` shows the source of each
  value.
- A test sets the working directory of a command through `-C`.
- The statuses and the JSON events of the commands are the same as before. `acceptancetest` checks
  them on the built binary.

**Negative:**

- The `main` of every consumer changes. Each command of a consumer becomes a go-cli handler.
- A consumer whose binary keeps cobra or urfave/cli runs the eidos binary as a process.
- A consumer cannot add a global option of its own because the global options have the type
  `cli.Global`.
- A script that passes `-format=json` or calls `help` fails with a usage error.
- A variable such as `ACME_RUN_DRY_RUN=true` in the parent environment changes a run.
- `eidos-cli` requires go-cli and its YAML format. Both are below version 1.
- The record of the lock contains the path of the command and no longer its arguments.
- eidos admits a brand with a trailing hyphen or with two hyphens in a row. go-cli refuses such a
  brand as the name of an app. `cli.Run` then panics for the app that `cli.New` returns.

**Neutral:**

- The help of the optional patterns of `run` notes `default ""`.

## References

| What | Where |
|---|---|
| RFC-0021, whose rule against a framework, mounting of the commands, grammar of the flags and command `help` this record replaces | ../rfc/0021-the-command-kernels.md |
| RFC-0059, whose signature of `cli.Run` over an `IO` and `os.Args` this record replaces | ../rfc/0059-package-structure-and-the-facade.md |
| D159, the row of this decision | ../architecture/21-decisions.md |
| go-cli | <https://github.com/stealth-scale/go-cli> |
| go-cli v0.4.0, with `App.ExecuteRenderer` | <https://github.com/stealth-scale/go-cli/releases/tag/v0.4.0> |
