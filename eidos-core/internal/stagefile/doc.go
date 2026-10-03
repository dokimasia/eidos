// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package stagefile replaces a file atomically inside an [os.Root].
//
// [Replace] writes the new bytes to a staging file beside the target,
// named with [Suffix], syncs it, and renames it over the target, so a
// reader sees the old file or the new one and never half of either.
// The output sinks commit every generated file through it.
//
// [ReplaceShared] does the same through a staging file that belongs to
// one call, so two writers replacing one name at once never write into
// each other's staging file. Its [Durability] states whether it syncs
// before the rename. The ledger stores every blob through it, because
// two runs, or two workspaces sharing a parse memo, can write one name
// at once. Every path resolves inside the root, so a symlink does not
// lead a write outside it.
//
// # Failure semantics
//
// A staging file that fails to write, sync, close or rename is
// removed, and the target remains as it was. A staging file a killed
// process left behind beside a [Replace] target is stale by
// definition, and the next Replace of that target truncates it. One a
// killed process left behind beside a [ReplaceShared] target remains
// until the owner of the directory removes it, because a live writer's
// staging file looks the same.
//
// # Dependency position
//
// core/internal/stagefile imports only the Go stdlib.
package stagefile
