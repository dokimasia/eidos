// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spellref spells emit-model type references: [Spell] is
// the one recursive walk the backends share, parameterized by the
// argument brackets and the stand-in an absent spelling takes.
//
// # Dependency position
//
// The package imports the sdk's emit model and nothing else of the
// framework.
package spellref
