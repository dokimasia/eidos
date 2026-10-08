// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"io/fs"
	"strings"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Classifier inspects a parsed unit and stamps classification
// facts through the unit's builder, such as a test-file marker or a
// foreign generator's output, at plugin authority under the
// frontend's identity. It runs after the author's parse, on the
// same unit, so what it inspects is what the parse declared, and a
// returned error is fatal to the load the way a parse error is.
// There is no exclusion hook beside it: a classifier stamps what it
// saw and drops nothing, because whether a classified file takes
// part is the consumer's call.
type Classifier func(u *plugin.SourceUnit) error

// Builder accumulates a frontend declaration: the identity and the
// language of every declaration it loads, the comment syntax the
// parse strips through, whether the language overloads, the file
// claim, and the functions the pipeline varies in. Everything on it
// is data except the functions. Build freezes it, and a Builder is
// not reused afterwards.
type Builder struct {
	name         plugin.ID
	lang         symbol.Lang
	syntax       plugin.CommentSyntax
	overloads    bool
	version      string
	selection    []string
	partition    func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error)
	parse        func(context.Context, *plugin.SourceUnit) error
	resolve      func(plugin.ImportScope, string) plugin.Candidates
	classifiers  []Classifier
	options      any
	hasOptions   bool
	dependencies func(context.Context, *plugin.DependencyRound, plugin.StoreReader) ([][]plugin.SourceRef, error)
	exports      func(plugin.ImportScope, string) plugin.Candidates
	stores       func(getenv func(key string) string) (map[string]fs.FS, error)
}

// New starts a frontend declaration for one language.
func New(
	name plugin.ID, lang symbol.Lang, syntax plugin.CommentSyntax,
) *Builder {
	return &Builder{name: name, lang: lang, syntax: syntax}
}

// Version sets the version every unit key folds: bump it with any
// change to the produced graph, because a warm cache keyed without
// it serves the old graph after a frontend change and nothing
// reports why.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// Overloads declares that the language overloads: two callables of
// one name in one scope, told apart by their parameters. The load
// then spells each callable's discriminator from its parameters: the
// type spellings, which the parse normalizes, with their
// instantiations' arguments and their variadic and optional marks. A
// declaration without it is a language that cannot overload, and
// every callable it loads takes the empty discriminator.
func (b *Builder) Overloads() *Builder {
	b.overloads = true
	return b
}

// Match appends selection patterns to the file claim: globs
// matched against the whole workspace-relative path, "**" spanning
// any number of segments, applied in order with the last match
// deciding, so a later negation carves an earlier claim. Policy is
// left out of the claim: whether test files take part is the
// consumer's call through scopes, never a selection line.
func (b *Builder) Match(patterns ...string) *Builder {
	b.selection = append(b.selection, patterns...)
	return b
}

// Units sets the partition: the selected files grouped into the
// language's own compilation grain, through the recorded reader.
func (b *Builder) Units(
	partition func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error),
) *Builder {
	b.partition = partition
	return b
}

// Parse sets the unit parse: one unit in, declarations into its
// builder, problems through its findings.
func (b *Builder) Parse(
	parse func(context.Context, *plugin.SourceUnit) error,
) *Builder {
	b.parse = parse
	return b
}

// Classify appends classifiers, run after the parse in declaration
// order.
func (b *Builder) Classify(cs ...Classifier) *Builder {
	b.classifiers = append(b.classifiers, cs...)
	return b
}

// Resolve sets the language's resolution: the candidates for one
// spelling in one file's recorded bindings, in probe order and in
// shadowing tiers. A language with nothing resolvable states it
// with a resolve that returns nothing, so the silence is written
// and not defaulted.
func (b *Builder) Resolve(
	resolve func(plugin.ImportScope, string) plugin.Candidates,
) *Builder {
	b.resolve = resolve
	return b
}

// Options declares the frontend's configuration the way any plugin
// declares one: a pointer whose exported fields are the options and
// whose constructed values are the defaults. The built frontend
// implements [plugin.OptionsProvider], and the load folds the
// populated value's canonical encoding into every unit key.
func (b *Builder) Options(o any) *Builder {
	b.options = o
	b.hasOptions = true
	return b
}

// Dependencies declares the language's dependency rounds: the units
// that declare what the loaded files import from outside the
// workspace, placed through the round's recorded door, and the needs
// the language places nowhere, reported through the round. The built
// frontend implements [plugin.Dependent], and the load parses every
// unit the function returns at [plugin.DepthSignatures].
func (b *Builder) Dependencies(
	dependencies func(context.Context, *plugin.DependencyRound, plugin.StoreReader) ([][]plugin.SourceRef, error),
) *Builder {
	b.dependencies = dependencies
	return b
}

// Exports declares what a file publishes and does not declare, in
// shadowing tiers, as a re-export does. The built frontend implements
// [plugin.Exporter], and the resolution step follows a candidate that
// names no declaration through the function.
func (b *Builder) Exports(exports func(plugin.ImportScope, string) plugin.Candidates) *Builder {
	b.exports = exports
	return b
}

// Stores declares the function that locates the stores that the
// dependency units of the language read, such as a module cache. The
// built frontend implements [plugin.StoreLocator] through the function. A
// declaration with Stores also declares [Builder.Dependencies], because
// only a dependency unit reads a store.
func (b *Builder) Stores(locate func(getenv func(key string) string) (map[string]fs.FS, error)) *Builder {
	b.stores = locate
	return b
}

// Build freezes the declaration and returns the lowered frontend,
// which implements [plugin.Frontend]. The conformance suite runs the
// same checks over it that a hand-rolled frontend meets, because the
// lowering adds nothing the role does not state. The frontend
// implements an optional role exactly when the declaration states it:
// [plugin.OptionsProvider] for [Builder.Options], [plugin.Dependent]
// for [Builder.Dependencies], [plugin.Exporter] for [Builder.Exports],
// and [plugin.StoreLocator] for [Builder.Stores].
//
// Build panics on a declaration defect: an empty name or language,
// no declared version, an empty claim, a missing partition, parse or
// resolve, or stores without dependency rounds. A wrong declaration is
// a bug in the frontend's own constructor and panics on the first Build
// in any test.
func (b *Builder) Build() plugin.Frontend {
	if b.name == "" {
		panic("frontend: New with an empty name")
	}
	name := string(b.name)
	var defects []string
	if b.lang == "" {
		defects = append(defects, "declares no language")
	}
	if b.version == "" {
		defects = append(defects, "declares no version, which every unit key folds")
	}
	if len(b.selection) == 0 {
		defects = append(defects, "claims no files")
	}
	if b.partition == nil {
		defects = append(defects, "declares no partition")
	}
	if b.parse == nil {
		defects = append(defects, "declares no parse")
	}
	if b.resolve == nil {
		defects = append(defects, "declares no resolve")
	}
	if b.stores != nil && b.dependencies == nil {
		defects = append(defects, "declares stores and no dependency rounds, which read them")
	}
	if len(defects) > 0 {
		panic("frontend: " + name + " " + strings.Join(defects, ", and "))
	}
	base := &builtFrontend{
		name: b.name, lang: b.lang, syntax: b.syntax, overloads: b.overloads, version: b.version,
		selection: b.selection, partition: b.partition, parse: b.parse,
		resolve: b.resolve, classifiers: b.classifiers,
	}
	var roles role
	if b.hasOptions {
		roles |= optioned
	}
	if b.dependencies != nil {
		roles |= dependent
	}
	if b.exports != nil {
		roles |= exporter
	}
	if b.stores != nil {
		roles |= locator
	}
	o, d, e, l := optionsRole{b.options}, dependentRole{b.dependencies}, exporterRole{b.exports}, locatorRole{b.stores}
	switch roles {
	case optioned:
		return &struct {
			*builtFrontend
			optionsRole
		}{base, o}
	case dependent:
		return &struct {
			*builtFrontend
			dependentRole
		}{base, d}
	case exporter:
		return &struct {
			*builtFrontend
			exporterRole
		}{base, e}
	case optioned | dependent:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
		}{base, o, d}
	case optioned | exporter:
		return &struct {
			*builtFrontend
			optionsRole
			exporterRole
		}{base, o, e}
	case dependent | exporter:
		return &struct {
			*builtFrontend
			dependentRole
			exporterRole
		}{base, d, e}
	case optioned | dependent | exporter:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			exporterRole
		}{base, o, d, e}
	case dependent | locator:
		return &struct {
			*builtFrontend
			dependentRole
			locatorRole
		}{base, d, l}
	case optioned | dependent | locator:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			locatorRole
		}{base, o, d, l}
	case dependent | exporter | locator:
		return &struct {
			*builtFrontend
			dependentRole
			exporterRole
			locatorRole
		}{base, d, e, l}
	case optioned | dependent | exporter | locator:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			exporterRole
			locatorRole
		}{base, o, d, e, l}
	default:
		return base
	}
}

// role is one optional role a declaration adds to its built frontend,
// as a bit of the set Build composes the frontend's type from.
type role uint8

// The optional roles: the options a load keys on, the dependency
// rounds, the re-exports the resolution step follows, and the stores
// that the dependency rounds read. Build refuses the store role without
// the dependent role, so the switch of Build has no case for it.
const (
	optioned  role = 1
	dependent role = 2
	exporter  role = 4
	locator   role = 8
)

// builtFrontend is a lowered frontend declaration: the facts as
// data, and the declared functions behind the role's methods.
type builtFrontend struct {
	name        plugin.ID
	lang        symbol.Lang
	syntax      plugin.CommentSyntax
	overloads   bool
	version     string
	selection   []string
	partition   func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error)
	parse       func(context.Context, *plugin.SourceUnit) error
	resolve     func(plugin.ImportScope, string) plugin.Candidates
	classifiers []Classifier
}

// Name returns the frontend's one identity.
func (f *builtFrontend) Name() plugin.ID { return f.name }

// Lang returns the language of every declaration the frontend
// loads.
func (f *builtFrontend) Lang() symbol.Lang { return f.lang }

// Syntax returns the language's comment forms.
func (f *builtFrontend) Syntax() plugin.CommentSyntax { return f.syntax }

// Overloads reports whether the declaration stated that the language
// overloads.
func (f *builtFrontend) Overloads() bool { return f.overloads }

// Version implements [plugin.Versioned]: the declared version.
func (f *builtFrontend) Version() string { return f.version }

// Selection returns the declared claim, in declaration order.
func (f *builtFrontend) Selection() []string { return f.selection }

// Partition groups the selected files through the declared
// function.
func (f *builtFrontend) Partition(
	ctx context.Context, files []plugin.SourceRef, r plugin.FileReader,
) ([][]plugin.SourceRef, error) {
	return f.partition(ctx, files, r)
}

// Parse loads one unit through the declared function, then runs
// the classifiers over it in declaration order. A classifier's
// error is fatal the way a parse error is: the load stops and seals
// no graph whose stamps are half-made.
func (f *builtFrontend) Parse(ctx context.Context, u *plugin.SourceUnit) error {
	if err := f.parse(ctx, u); err != nil {
		return err
	}
	for _, c := range f.classifiers {
		if err := c(u); err != nil {
			return err
		}
	}
	return nil
}

// Resolve returns the candidates the declared function names.
func (f *builtFrontend) Resolve(
	scope plugin.ImportScope, spelling string,
) plugin.Candidates {
	return f.resolve(scope, spelling)
}

// optionsRole is the options role of a built frontend that declares
// configuration.
type optionsRole struct {
	options any
}

// Options implements [plugin.OptionsProvider] with the declared
// value.
func (r optionsRole) Options() any { return r.options }

// dependentRole is the dependent role of a built frontend that
// declares dependency rounds.
type dependentRole struct {
	dependencies func(context.Context, *plugin.DependencyRound, plugin.StoreReader) ([][]plugin.SourceRef, error)
}

// Dependencies implements [plugin.Dependent] through the declared
// function.
func (r dependentRole) Dependencies(
	ctx context.Context, round *plugin.DependencyRound, reader plugin.StoreReader,
) ([][]plugin.SourceRef, error) {
	return r.dependencies(ctx, round, reader)
}

// exporterRole is the exporter role of a built frontend that declares
// what its files publish.
type exporterRole struct {
	exports func(plugin.ImportScope, string) plugin.Candidates
}

// Exports implements [plugin.Exporter] through the declared function.
func (r exporterRole) Exports(scope plugin.ImportScope, name string) plugin.Candidates {
	return r.exports(scope, name)
}

// locatorRole is the store role of a built frontend that declares the
// stores of its dependency rounds.
type locatorRole struct {
	stores func(getenv func(key string) string) (map[string]fs.FS, error)
}

// Stores implements [plugin.StoreLocator] through the declared function.
func (r locatorRole) Stores(getenv func(key string) string) (map[string]fs.FS, error) {
	return r.stores(getenv)
}
