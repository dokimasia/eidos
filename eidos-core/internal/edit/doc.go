// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package edit measures the distance between two spellings, so the kernel
// can name the spelling that an author most likely meant.
//
// [Distance] counts the edits between two spellings. [Near] applies the
// bound of a suggestion: at most two edits, and at most a third of the
// length of the longer spelling. The registry of directives suggests a
// registered name under that bound, and a source unit warns about a mark
// near its brand under it.
//
// # Dependency position
//
// core/internal/edit imports the Go stdlib alone.
package edit
