// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules is the projection vocabulary and the rules seam:
// the questions every generator asks of a declaration, and the
// contract each language returns them through.
//
// The kernel owns the three walks every language would otherwise
// repeat. [Bound.CallableOf] maps a callable's signature into a
// [Callable] and asks the language for the roles. [Bound.TypeOf]
// folds a reference's structure into a [TypeShape] and asks the
// language for the leaves. [Bound.MembersOf] walks a type's
// effective [MemberSet] under the language's [MemberPolicy], with
// provenance per member and a [Gap] for every contributor it could
// not follow. A language returns [SourceRules]: the decisions
// inside those walks, what a spelling names in a scope, the values
// of a type, and the naming join. The optional capabilities,
// [EnumRules] and the rest, are declared by satisfying an interface
// and found by asserting on [Bound.Source].
//
// # Refusal
//
// Nothing here guesses. A value that could not be derived is a
// [Sample] whose [Refusal] says why, a member the walk could not
// reach is a Gap with its reason, and a reference the language
// cannot classify folds to [symbol.FormOpaque] with its spelling.
// A language the composition registered nothing for is
// [Absent]: every walk over it contributes nothing and every value
// refuses with [RefusedNoRules].
//
// # Reads
//
// Every projection reads through a [View]: the invocation's
// tracked declaration reader, the run's facts and the read set
// both record into. A projection called with a zero view refuses
// with [RefusedNoView] rather than reading an untracked graph.
// Two calls with one view over one graph return equal values, and
// a [SourceRules] value is safe for concurrent use because it
// holds nothing of its own.
//
// # Dependency position
//
// core/rules imports core/node, core/emit, core/symbol, core/store,
// core/meta, core/directive, core/diag and the Go stdlib. The
// authoring root, the workspace and the satellites' rules packages
// import it; nothing beneath it imports it back.
package rules
