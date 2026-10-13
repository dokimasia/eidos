---
adr: 0049
title: The config file of a workspace sets the values of options
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

# ADR-0049: The config file of a workspace sets the values of options

## Status

Accepted

## Context

The config file of a workspace contained the workspace document alone. The kernel commands decoded
every key of it strictly. A team could not set an option in the file, so `--strict` for every run
needed a flag on every command line. go-cli reads the values of options from a config file. It fails
a run on every key without an option.

The readers of the file conflict:

- The workspace document has keys that exist only at run time, such as the names of plans. It also
  has nested objects and a list of workspaces. eidos reports each fault of the document with its
  line.
- An option of go-cli is a field of a struct whose value converts from text. go-cli cannot read the
  document.
- Each reader refuses the keys of the other. eidos refused the keys of the options. go-cli refuses
  the keys of the document.

go-cli v0.4.0 skips the keys that an app decodes itself. It calls a search of the app for the path
of the config file.

## Decision

We will read the values of the options of the command line from the config file of the workspace
through go-cli beside the workspace document because one file then configures a workspace and every
option follows one order of sources: its default, the file, its variable and its flag.

- go-cli skips the ten top-level keys of the format. The decoder of eidos skips every other
  top-level key. Below the top level each reader is as strict as before.
- go-cli finds the file through the search of eidos. The search runs from `-C` or the working
  directory to the first version control marker. The options and the workspace come from one file.
- A file that lists workspaces sets the options of the whole run.
- The config file of a member of a list does not set options because go-cli reads one file for each
  run. eidos reports a key of an option there as a config error.
- The JSON Schema of the file admits every top-level key that is not a key of the format. It does not
  list the options.
- go-cli reads a relative `--config` against the working directory of the process. This applies also
  under `-C`.

## Alternatives Considered

### The workspace document as options of go-cli

go-cli could read the whole file with each key of the document as an option. go-cli v0.4.0 refuses a
map of structs and a list of structs. It reports a fault of a file without its line. The document has
both kinds of value. eidos reports the line of each of its faults.

### The values of options in the workspace document

eidos could declare `strict` and the other options as keys of its document. It would apply them
before the variables and the flags. eidos would then restate the sources of an option and their
order. go-cli already applies both to every option.

### A JSON Schema with the options of the kernel commands

The schema could list `strict`, `run.plan` and the other keys of the kernel options. An editor would
then complete them. eidos would derive each key from its field a second time because go-cli has no
API that lists the options of an app. The options of the commands of a consumer would still be
missing.

### A relative --config against the directory of -C

`go -C` and `git -C` read a relative path of the command line in the directory of `-C`. go-cli opens
the path of `--config` before it applies the other flags of the run. `-C` is not applied when it
opens the file. eidos and go-cli would read different files unless go-cli had a working directory in
its `Env`.

## Consequences

**Positive:**

- A workspace sets an option for every run in its config file. `strict: true` and
  `run.plan: [api]` are examples.
- `--flag-debug` names the key and the file of each value that the file sets.
- A misspelled key of an option fails the run with a suggestion. go-cli suggests `strict` for
  `strikt`.

**Negative:**

- A key in the file changes every run of the workspace. `run.dry-run: true` makes every run a dry
  run.
- The JSON Schema admits a misspelled key of an option. go-cli reports the key only when a run reads
  the file.
- `doctor` cannot report a key that go-cli refuses beside its other findings. go-cli fails the run
  before `doctor` runs.
- A file that does not parse fails every command, `schema` included. `--config=` runs a command
  without the file.
- The config file of a member of a list cannot set an option of its member.

**Neutral:**

- The decoder of the document is as strict as before below the top level. Each fault of the
  document keeps its line.

## References

| What | Where |
|---|---|
| D159, the row of this decision | ../architecture/21-decisions.md |
| go-cli RFC-0006, ignored config keys and a located config file | <https://github.com/stealth-scale/go-cli/blob/v0.3.0/docs/rfc/0006-ignored-keys-and-a-located-config-file.md> |
| go-cli issue 5, keys that the app decodes itself | <https://github.com/stealth-scale/go-cli/issues/5> |
| go-cli issue 6, the search for the config file | <https://github.com/stealth-scale/go-cli/issues/6> |
