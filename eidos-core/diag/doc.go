// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package diag reports positioned findings under stable codes.
//
// [Diag] is one finding: a [Code], a [Severity], the position of the
// declaration that caused it, and the [Origin] that reported it.
// [Sink] collects the findings of one run and reports whether any of
// them failed it. [Registry] contains every registered code and refuses
// a number claimed twice within one prefix, and [ParseCode] reads a
// code back from its spelling.
//
// A code is API. Consumers script against codes, tests assert on
// them and documentation anchors to them, so a changed meaning is a
// new code rather than an edit to an existing one.
//
// # Suppression and promotion
//
// [Sink.Suppress] installs a table of the codes that a run suppresses at
// each declaration position, and the sink removes and counts the findings
// that the table lists. A kernel Error is never removed. [Sink.Promote]
// reports every Warning as an Error. A run installs both in the sinks
// that decide an outcome, and none in the sinks that collect one
// execution's findings for a record, so a record keeps every finding as
// it was reported.
//
// # Failure semantics
//
// [SeverityError] means the run is wrong and any Error fails it.
// [SeverityWarning] means the output is usable but a human should look.
// [SeverityInfo] reports provenance and progress. Reporting never
// panics. [MustRegister] panics on every code [Registry.Register]
// refuses: a prefix that is not uppercase letters, a number below 1,
// a missing meaning and a duplicate. Each is a defect at package
// initialization rather than a condition a run can meet. [ParseCode]
// returns an error for a spelling that is not a code.
//
// # Dependency position
//
// core/diag imports only core/position and the Go stdlib.
package diag
