// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package pathset checks that file paths can exist side by side in one
// directory tree on every filesystem.
//
// A [Set] collects slash-separated, workspace-relative file paths, and
// [Set.Clash] reports how a new path collides with them: it differs
// from one only in case, a path of the set needs it as a directory, or
// it needs a path of the set as a directory. The layout reports these
// clashes as positioned findings before anything renders, and the
// output sinks refuse them again when a file is staged. Both read this
// one rule, so a path the layout admits is a path every sink stages.
//
// # Case folding
//
// Every comparison folds case with [strings.ToLower], files and
// directories alike: a case-insensitive filesystem stores "svc/Store.go"
// and "svc/store.go" as one file, and a file "Gen" and a directory
// "gen" as one entry.
//
// # Dependency position
//
// core/internal/pathset imports only the Go stdlib.
package pathset
