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
// directory of the manifest's documents. The run proceeds as if no run
// had committed, so it removes nothing.
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

// FailedDependency reports a plan that depends on a plan that failed,
// and a workspace check that reads one, at the first Error of the plan
// whose failure started the chain. The plan commits nothing, and the
// check does not run. The run reports none where that plan failed on a
// returned error alone, because the run returns the error.
var FailedDependency = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  61,
	Meaning: "a plan or a check reads a plan that failed, and generates or checks nothing",
})

// ColdState reports a run that ignored the sealed state and ran cold, at
// the state directory's CURRENT, and states the cause: a CURRENT that
// names no generation, an executable the run cannot read, a generation
// of another format, composition or executable, or a generation whose
// blocks, segments or regions do not read whole. The run's outcome is
// the cold run's: damaged state costs time and never correctness.
var ColdState = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  62,
	Meaning: "the run ignored the sealed state and ran cold",
})

// StateLocked reports a run that found the lock of the state directory
// taken, at the lock file, and names the holder: another run over the
// same state directory, in this process or in another one. The run marks
// every plan failed and writes nothing.
var StateLocked = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  63,
	Meaning: "another holder has the lock of the state directory, and the run writes nothing",
})

// OutOfDate reports a path that a run under [Input.Check] would create,
// update or remove, or would refuse to write, at the path: the committed
// output differs from what the run generates. The run writes nothing.
var OutOfDate = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  64,
	Meaning: "the committed output differs from what the run generates",
})

// UnusedSuppression reports a diag directive that removed no finding, at
// the directive, as an Info: the declaration no longer causes the
// finding, or a finding of the code has another position. A run that
// skips a plan, and a cancelled run, report none.
var UnusedSuppression = diag.MustRegister(diag.KernelPrefix, diag.CodeSpec{
	Number:  65,
	Meaning: "a diag directive removed no finding",
})
