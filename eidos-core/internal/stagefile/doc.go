// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package stagefile replaces a file atomically inside an [os.Root].
//
// [Replace] writes the new bytes to a staging file beside the target,
// named with [Suffix], syncs it, and renames it over the target, so a
// reader sees the old file or the new one and never half of either.
// The output sinks commit every generated file through it, and the
// ledger commits the manifest through it. Every path resolves inside
// the root, so a symlink does not lead a write outside it.
//
// # Failure semantics
//
// A staging file that fails to write, sync, close or rename is
// removed, and the target remains as it was. A staging file a killed
// process left behind is stale by definition, and the next Replace of
// its target truncates it.
//
// # Dependency position
//
// core/internal/stagefile imports only the Go stdlib.
package stagefile
