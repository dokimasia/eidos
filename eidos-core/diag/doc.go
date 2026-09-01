// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package diag reports positioned findings under stable codes.
//
// [Diag] is one finding: a [Code], a [Severity], the position of the
// declaration that caused it, and the [Origin] that reported it.
// [Sink] collects the findings of one run and reports whether any of
// them failed it. [Registry] holds every registered code and refuses
// a number claimed twice within one prefix.
//
// A code is API. Consumers script against codes, tests assert on
// them and documentation anchors to them, so a changed meaning is a
// new code rather than an edit to an existing one.
//
// # Failure semantics
//
// [SeverityError] means the run is wrong and any Error fails it.
// [SeverityWarning] means the output stands but a human should look.
// [SeverityInfo] carries provenance and progress. Reporting never
// panics; only [MustRegister] does, and only for a duplicate code,
// which is a defect at package initialization rather than a
// condition a run can meet.
//
// # Dependency position
//
// core/diag imports only core/position and the Go stdlib.
package diag
