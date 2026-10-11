// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package stagefile

import "os"

// Barrier is the interface of the calls that make the writes and removals
// of one group durable on one system. A Barrier serves one group: the
// group calls Start before its first write, File after each write, Order
// before its first rename, Durable after its last rename or removal, and
// Close at the end of its commit. [System] returns a new barrier of the
// running system.
//
// # Concurrency
//
// The group calls File on the goroutines of its pool at once, and every
// other method on one goroutine.
type Barrier interface {
	// Start runs once, before the first write. n is the number of writes
	// and removals of the group.
	Start(r *os.Root, n int) error
	// File runs on the worker that wrote f, before f closes.
	File(f *os.File) error
	// Order runs once, after every write and before any rename. When it
	// returns, the data of every staging file is on stable storage.
	Order() error
	// Durable runs once, after every rename and removal. When it returns,
	// the entries of dirs are on stable storage.
	Durable(dirs []string) error
	// Close releases what Start opened.
	Close() error
}
