// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package spellref spells emit-model type references: [Spell] is
// the one recursive walk the backends share, parameterized by the
// argument brackets and the stand-in an absent spelling takes.
// [SpellWith] is the same walk through a language's [Qualify], which
// binds each named reference's import and returns the name the file
// refers to it by. [SpellRestated] is the walk of [SpellWith] for a
// language whose composite spelling follows from its structure. The
// language passes a function of the type [Restate], which writes a
// structural reference again where a child spells otherwise. [PackageOf]
// returns the package a reference's import names for a backend of one
// language.
//
// # Dependency position
//
// The package imports the sdk's emit model and symbol vocabulary and
// nothing else of the framework.
package spellref
