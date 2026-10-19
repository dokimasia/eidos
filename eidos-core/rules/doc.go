// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package rules is the projection vocabulary and the rules seam:
// the questions every generator asks of a declaration, and the
// contract each language returns them through.
//
// The kernel implements the three walks every language would otherwise
// repeat. [Bound.CallableOf] maps a callable's signature into a
// [Callable] and asks the language for the roles. [Bound.TypeOf]
// folds a reference's structure into a [TypeShape] and asks the
// language for the leaves. [Bound.MembersOf] walks a type's
// effective [MemberSet] under the language's [MemberPolicy], with
// provenance per member and a [Gap] for every contributor it could
// not follow. A language returns [SourceRules]: the decisions
// inside those walks, what a spelling names in a scope, and the values
// of a type.
//
// # Emitted references
//
// [EmitRef] restates a node reference in the emit model, which a
// generator writes. [Mentioned] returns the type parameters that emitted
// references mention, so a generator declares a type for one part of a
// generic declaration with the parameters that the part uses.
//
// # Resolution
//
// [Resolve] is the walk of the resolution of a directive's spelling that
// the languages with members share. A language states the probe of its
// scope, the members that a subject can use, the placeholders of its
// types and the type of a value through a [Resolver], and its
// [SourceRules.Resolve] calls the walk. A refusal of the walk is a
// [ResolveError] under the language. [Unaliased] follows the transparent
// aliases of a reference, which a language reads before it classifies a
// type.
//
// # Optional capabilities
//
// A language declares an optional capability, [EnumRules] and the rest,
// by satisfying its interface on the [SourceRules] value, and a consumer
// finds it by asserting on [Bound.Source]. A consumer whose assertion
// fails reports once and generates nothing for that language, so no
// projection depends on a default nobody chose. A capability's methods
// follow the concurrency rule of [SourceRules].
//
// # Refusal
//
// No projection substitutes a default for a value it cannot derive. A
// value that could not be derived is a [Sample] whose [Refusal] states
// why, a member the walk could not reach is a Gap with its reason, and
// a reference the language cannot classify folds to
// [symbol.FormOpaque] with its spelling. A language the composition
// registered nothing for is [Absent]: every walk over it contributes
// nothing and every value refuses with [RefusedNoRules].
//
// # Reads
//
// Every projection reads through a [View]: the invocation's
// tracked declaration reader, the run's facts and the read set
// both record into. A projection called with a zero view refuses
// with [RefusedNoView] rather than reading an untracked graph.
// Two calls with one view over one graph return equal values, and
// a [SourceRules] value is safe for concurrent use because it
// keeps no state of its own.
//
// # Dependency position
//
// core/rules imports core/node, core/emit, core/symbol, core/store,
// core/meta, core/directive, core/diag and the Go stdlib. The
// authoring root, the workspace and the satellites' rules packages
// import it; nothing beneath it imports it back.
package rules
