// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !linux && !windows

package stagefile

// System returns a new barrier of a system other than Linux, macOS and
// Windows. File syncs each staging file with fsync(2), the order barrier
// has nothing left to do, and the durability barrier syncs each changed
// directory with fsync(2). It allocates the barrier.
func System() Barrier { return &fileSync{} }
