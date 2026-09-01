// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol

// Fact discriminates the stated facts a declaration may carry
// beyond its kind and its names: the modifiers, clauses and
// attachments a backend renders, holds or refuses one by one.
//
// A fact is stated or absent, never valued: the guard that reads
// coverage asks only whether a declaration states it. The zero
// value is [FactInvalid], which no declaration states.
//
// One constant exists per fact the schema tags, generated from the
// schema into fact.gen.go. The values belong to a build: nothing
// durable stores them, the way nothing durable stores a [Kind].
type Fact uint8
