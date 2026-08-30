// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package symbol

// Kind discriminates the declaration kinds.
//
// Every consumer that switches on kind uses one set of names across
// both model sides, so a switch written against the node model
// reads the same against the emit model. The zero value is
// [KindInvalid], which no declaration answers.
//
// One constant exists per schema struct, generated from the schema
// into kind.gen.go. The values belong to a build: nothing durable
// stores them, because the codecs encode kind names and the sealed
// state carries its own format version.
type Kind uint8
