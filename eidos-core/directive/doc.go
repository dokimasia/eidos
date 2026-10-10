// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package directive defines the one grammar every carrier lowers to,
// the schemas that close it, and the validation that types it.
//
// A carrier, a comment marker in some language's source, hands over
// [Raw]: the grammar parsed by [Parse], values untyped, joined from
// continuations by [Join]. A [Schema] declares what a directive
// accepts: positional params in order, typed keyed params, a closed
// role set, repeatability, negatability, and the constraints between
// directives. [Registry] records every schema, refuses what the
// contract refuses, and resolves spellings: bare while unambiguous,
// plugin-prefixed once two plugins claim one name. [Validate] runs
// over one subject's full list and returns [Directive] instances
// typed per schema, reporting every violation as a positioned Error
// under its own registered code before any handler runs. An
// unclaimed name is a violation like any other, unless the workspace
// opted out of it through [Registry.Ignore]: a foreign tool's
// carriers in the same comments drop in silence, and a registered
// name can never be ignored.
//
// # Negation
//
// A carrier writes a directive in its set form or its negated form,
// and [Raw.Negated] records which. A negated instance validates like
// any other against a schema that declares [Schema.Negatable], and
// reports [NegationRefused] against every other schema. Registration
// refuses a negatable kernel schema. A negated instance gates no
// rule and takes part in no requirement or conflict: dispatch
// excludes its subject from the bare and fact-gated rules of the
// plugin that registered the schema. A subject that sets and negates
// one directive reports [Conflict], and neither instance survives.
//
// # Overrides
//
// A schema states [Schema.Overrides] where an author's statement replaces
// what the plugin's own rules derive. Validation marks each instance with
// [Directive.Overrides]. Dispatch excludes the subject of such an instance
// from the bare and fact-gated rules of the plugin that registered the
// schema, as it does for a negated instance. The rules that a directive
// gates still run, so the plugin's handler of the directive receives the
// instance. Registration refuses an override kernel schema.
//
// # Variants
//
// A schema states [Schema.Variants] to cover a closed set of names under
// one directive, such as the classifications of one plugin. The first
// positional argument of an instance selects its [Variant], and
// [Directive.Variant] is its name. The variant's params join the
// schema's, and the variant's roles apply in place of the schema's.
// [Validate] reports an instance without a variant, or with a name that no
// variant has, under [UnknownVariant], and the message of the finding
// contains the names of the variants.
// A single-instance schema admits one instance on a subject, whatever its
// variant, and a repeatable schema admits one instance of each variant.
// Requirements and conflicts apply to the schema, whatever the variant.
//
// # Deprecation
//
// A schema states [Schema.Deprecated] to deprecate its directive,
// [Variant.Deprecated] to deprecate one variant, and
// [ParamSpec.Deprecated] to deprecate one param, each with the rewrite
// that replaces it. [Validate] types a deprecated instance as before and
// reports a Warning under [DeprecatedDirective] that states the rewrite,
// so a plugin renames a directive or a param across a release without
// failing a run.
//
// # The kernel's names
//
// Six names belong to the kernel and register through [Kernel]:
// meta, out, diag, skip, sample and witness. The packages that apply
// them define their semantics, and this package defines the spelling
// and its validation. The off param of diag resolves against the
// diagnostic codes that packages registered, and a spelling that names
// no registered code reports [UnknownCode]. The reserved keys
// [ReservedOut] and [ReservedTag] are admitted on every directive, and
// no plugin schema may claim them.
//
// The directive of each rendering target belongs to the kernel too.
// [Registry.RegisterTarget] records it under the target's spelling,
// such as typescript, and the registry then treats that spelling as a
// kernel name.
//
// # Closed choices
//
// A string param with [ParamSpec.Choices] admits the spellings in the
// list and no other. [Validate] reports another value under
// [BadSpelling], and the message lists the choices.
//
// # Failure semantics
//
// [Parse] and [Registry.Register] return errors: their callers are
// composition and carriers, which collect faults. [Validate] reports
// diagnostics: its findings are the author's to fix, and each one is
// positioned at its carrier. Nothing here panics.
//
// # Dependency position
//
// core/directive imports core/symbol, core/position, core/diag,
// core/meta and the Go stdlib. It never imports core/store: the
// store attaches and indexes raw instances, so the dependency
// points from the store to here.
package directive
