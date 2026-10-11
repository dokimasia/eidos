// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package stagefile commits files atomically and durably inside an
// [os.Root].
//
// A [Group] stages the writes and removals of one commit, and
// [Group.Commit] applies them in five steps. A pool of [Workers]
// goroutines writes each staging file beside its target, and the group
// passes the order barrier of a [Barrier]. One goroutine then renames each
// staging file over its target and applies each removal in name order,
// and the group passes the durability barrier. A reader of a target sees
// its old bytes or its new bytes, and after a crash of the machine each
// target contains its old bytes or its new bytes. The disk sink commits
// its files through a group with [Reserved] staging names, and the disk
// ledger writes a set of blobs through a group with [Unique] ones.
//
// [System] returns the barrier of the running system. Linux calls
// syncfs(2) at each barrier of a large group on ext4, XFS and btrfs.
// macOS issues one F_FULLFSYNC at each barrier after an fsync(2) of each
// file. Windows flushes each file without a flush of the device's cache,
// and flushes the root with one at each barrier. Every other group syncs
// each file and each directory with fsync(2).
//
// [ReplaceShared] replaces one file through a staging file that belongs
// to one call, so two writers that replace one name at once never write
// into each other's staging file. Its [Durability] states whether it
// syncs before the rename. The ledger stores a single blob through it,
// because two runs, or two workspaces that share a parse memo, can write
// one name at once. Every path resolves inside the root, so a symlink does
// not lead a write outside it.
//
// # Failure semantics
//
// A staging file that fails to write, sync, close or rename is removed,
// and its target remains as it was. A group whose order barrier fails
// renames nothing and removes every staging file. A staging file that a
// killed process left beside a [Reserved] target is stale by definition,
// and the next commit of that target truncates it. One that a killed
// process left beside a [Unique] or a [ReplaceShared] target remains
// until the owner of the directory removes it, because the staging file
// of a live writer looks the same.
//
// # Dependency position
//
// core/internal/stagefile imports golang.org/x/sys and the Go stdlib.
package stagefile
