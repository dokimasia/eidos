// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript_test

// plainString is a string literal without an escape, which the literal
// specs read and the quote specs write.
const plainString = `"plain"`

// The characters of the escape cases of the literal and the quote specs.
// smile is beyond the basic plane, and lineSeparator ends a line of
// TypeScript.
const (
	smile         = "\U0001F600"
	lineSeparator = "\U00002028"
)
