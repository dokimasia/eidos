// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package cli is the command line of every binary built with eidos, as an
// app of go.stealthscale.io/cli.
//
// [New] returns a go-cli app with the eight kernel commands at its root:
// run, plan, explain, prune, doctor, watch, version and schema. A binary
// adds its own commands to the app as go-cli commands, and runs the app
// with [Main]. A binary that mounts the kernel commands as subcommands of a
// group creates the app itself, registers what eidos needs with [Prepare],
// and adds the kernel commands with [Mount]. [Global] are the global
// options of the app.
//
// A command of the binary itself reads its [IO] with [IOOf], reads its
// workspaces with [Open] and renders its output through [Begin], as a
// kernel command does. [Configure] applies the config file of one root to
// a composition, so a test runs the composition that the binary runs.
//
// # The config file
//
// [ConfigName] returns the name of the config file of a brand, such as
// .acme.yaml. The file contains the workspace document and the values of
// the options of the command line. go-cli reads the keys of the options,
// and skips the keys of the document. Each fault of the document is a
// [ConfigError] with the file and the line of the fault. The kernel
// commands do not read the key extensions, and [Member.Extension] decodes
// one extension into the settings of a command of the binary. [Schema]
// returns the JSON Schema of the config file with the options of each
// plugin of the composition.
//
// # Exit statuses
//
// [Run] returns [StatusOK] when a command succeeds, [StatusFailed] when its
// findings or errors fail it, and [StatusUsage] for a usage error or a config
// error. No code of this module exits with 2, so a status of 2 is a Go panic.
//
// # Output
//
// Under --format=json, a command writes one JSON event per line to standard
// output. The first event is start, and the last event is summary. Run
// writes a usage error that go-cli finds as the same events. Standard error
// is empty unless the process panics, the command line has --flag-debug, or
// the command line ends at a command without a handler. Text output writes
// findings and errors to standard error, and the results of a command to
// standard output. The help of -h and --help is text.
//
// # Dependency position
//
// cli imports cli/internal/config, core/diag, core/directive, core/layout,
// core/ledger, core/node, core/output, core/plugin, core/position,
// core/symbol, core/workspace, go.stealthscale.io/cli,
// go.stealthscale.io/cli/yaml and the Go stdlib.
package cli
