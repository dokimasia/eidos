// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package grow sizes the buffers the kernel appends to across a run, so a
// buffer that fills doubles its capacity, where append grows a large
// slice by a quarter.
//
// A buffer that [Room] grows to m elements allocates less than twice
// their size in all and copies fewer than m elements. One that append
// grows allocates about five times their size, and copies about four
// times as many elements. The kernel's phase-call buffers, its journal
// records, its read log and the sealed state's recording lanes grow
// through Room.
//
// # Dependency position
//
// core/internal/grow imports the Go stdlib alone.
package grow
