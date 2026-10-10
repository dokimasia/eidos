// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package symbol

// Kind discriminates the declaration kinds.
//
// Every consumer that switches on kind uses one set of names across
// both model sides, so a switch written against the node model
// reads the same against the emit model. The zero value is
// [KindInvalid], which no declaration returns.
//
// One constant exists per schema struct, generated from the schema
// into kind.gen.go. The codecs spell a declaration's discriminator
// as its kind name and an identity's kind as its value. The model
// fingerprint folds the kinds in schema order, which fixes the
// values, so every unit key changes when a value does.
type Kind uint8
