// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import "go.dokimi.dev/eidos/core/diag"

// DriftedOutput reports a generated file edited since its stamp, at the
// origin of the file's first declaration: the brand's frame over a body
// that no longer hashes to its trailer. The run does not overwrite it,
// and the plan that routes a file there commits nothing.
var DriftedOutput = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  55,
	Meaning: "a generated file was edited since its stamp, and the run does not overwrite it",
})

// ForeignFile reports a generated path that contains a file the brand
// did not write, at the origin of the file's first declaration: a
// hand-written file, another tool's output, or a generated file whose
// trailer was deleted. The plan that routes a file there commits
// nothing.
var ForeignFile = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  56,
	Meaning: "a generated path contains a file the brand did not write",
})

// PlanCollision reports two plans that route a file to one path, at the
// second plan's declaration's origin with the first plan's as related.
// No plan of the run commits.
var PlanCollision = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  57,
	Meaning: "two plans route a file to one path",
})

// KeptOutput reports a stale output the run keeps on disk, at the kept
// file: a file no plan produces any more that was edited since its
// stamp or lost the brand's frame.
var KeptOutput = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  58,
	Meaning: "a stale output remains, because it was edited or lost its frame",
})

// UnreadableRecord reports a previous record that does not read, at the
// manifest's path. The run proceeds as if no run had committed, so it
// removes nothing.
var UnreadableRecord = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  59,
	Meaning: "the previous record does not read, and the run removes nothing",
})

// UnmetContract reports a declaration that lacks a fact its key's
// completeness contract promises, at the declaration and at the
// contract's severity.
var UnmetContract = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  60,
	Meaning: "a declaration lacks a fact its key's completeness contract promises",
})
