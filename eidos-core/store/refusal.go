// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package store

import "go.dokimi.dev/eidos/core/diag"

// The codes this package refuses under. A code is declared where it
// is registered, so the constant and the registry cannot drift.
var (
	// FrozenWrite refuses a structural write after the seal.
	FrozenWrite = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number:  1,
		Meaning: "a declaration was added after Freeze",
	})
	// DuplicatePackage refuses a package identity claimed twice.
	DuplicatePackage = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number:  2,
		Meaning: "two packages produce one identity",
	})
	// UnfrozenRead refuses a reader over a graph that is still
	// moving.
	UnfrozenRead = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
		Number:  3,
		Meaning: "a reader was asked for before Freeze",
	})
)

// RefusedError is a condition the graph returns an error for, under the
// code a consumer scripts against.
//
// A caller reaches the code through [errors.As] rather than by
// reading the text, which is what keeps the code load-bearing.
// Conditions a caller can only reach through a defect, such as
// handing over no package at all, return a plain error instead: they
// carry no code because nothing should be scripted against them.
type RefusedError struct {
	// Code identifies the refusal across releases.
	Code diag.Code
	// Msg is one sentence in the present tense, naming the thing and
	// the refusal.
	Msg string
}

// Error renders the refusal as its code and its reason.
func (r *RefusedError) Error() string {
	return "store: " + r.Code.String() + ": " + r.Msg
}
