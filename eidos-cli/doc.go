// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the commands that every binary built with eidos
// offers, and the parts that a command of the binary itself uses.
//
// [Kernels] returns the seven kernel commands: run, plan, explain, prune,
// doctor, watch and version. A [Command] parses its own flags and
// arguments, renders its own output and returns its own exit status, so any
// command line can mount it. A host passes a command the arguments after
// the name of the command, and exits with the status that [Command.Run]
// returns. [Main] is the host of a binary without a command line of its
// own.
//
// A command of the binary itself reads its workspaces with [Open] and
// renders its output through [Begin]. [Flags] are the flags that every
// command accepts.
//
// # Exit statuses
//
// A command returns [StatusOK] when it succeeds, [StatusFailed] when its
// findings or errors fail it, and [StatusUsage] for a usage error or a config
// error. No code of this module exits with 2, so a status of 2 is a Go panic.
//
// # Output
//
// Under --format=json, a command writes one JSON event per line to standard
// output and nothing to standard error. The first event is start, and the
// last event is summary. Text output writes findings and errors to standard
// error, and the results of a command to standard output.
//
// # Dependency position
//
// cli imports cli/internal/config, core/diag, core/directive, core/ledger,
// core/output, core/position, core/symbol, core/workspace and the Go
// stdlib.
package cli
