// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Classifier inspects a parsed unit and stamps classification facts
// through the builder of the unit, such as a test-file marker or the marker
// of the output of a foreign generator. The facts have plugin authority
// under the identity of the frontend. A classifier runs on the unit after
// the declared parse, so it inspects what the parse declared. A returned error
// ends the load, as an error of the parse does. A classifier stamps what
// it saw and drops nothing, because the consumer decides whether a
// classified file takes part. The kit has no hook that excludes a file.
type Classifier func(u *plugin.SourceUnit) error

// TestFiles returns a classifier that stamps key true on every file of the
// unit for which test reports true. test receives the workspace path of the
// file, so a frontend can test the suffix _test.go as the go command does.
// The stamp is at the position of the file.
//
// # Allocation contract
//
// The classifier allocates the list of the packages of the unit and the
// growth of the list of stamps: two allocations for a unit with one test
// file. It returns no error.
func TestFiles(key meta.KeyName, test func(path string) bool) Classifier {
	return func(u *plugin.SourceUnit) error {
		gb := u.Graph()
		for _, pkg := range gb.Packages() {
			for _, file := range pkg.Files {
				if test(file.Path) {
					gb.Stamp(file, meta.RawStamp{Key: key, Value: true, Pos: file.Pos})
				}
			}
		}
		return nil
	}
}

// Builder accumulates the declaration of a frontend. It records the
// identity and the language of every declaration that the frontend loads,
// the comment syntax that the parse strips, whether the language
// overloads, the claim of files, and the functions that differ between
// languages. Build freezes the declaration, and a Builder is not used
// again afterwards.
type Builder struct {
	name         plugin.ID
	lang         symbol.Lang
	syntax       plugin.CommentSyntax
	options      any
	hasOptions   bool
	overloads    bool
	version      string
	selection    []string
	partition    func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error)
	parse        func(context.Context, *plugin.SourceUnit) error
	resolve      func(plugin.ImportScope, string) plugin.Candidates
	classifiers  []Classifier
	dependencies func(context.Context, *plugin.DependencyRound, plugin.StoreReader) ([][]plugin.SourceRef, error)
	exports      func(plugin.ImportScope, string) plugin.Candidates
	stores       func(getenv func(key string) string) (map[string]fs.FS, error)
	importOf     func(plugin.ImportScope, string) string
	keys         []func(r *meta.Registry) error
}

// New starts a frontend declaration for one language.
func New(
	name plugin.ID, lang symbol.Lang, syntax plugin.CommentSyntax,
) *Builder {
	return &Builder{name: name, lang: lang, syntax: syntax}
}

// Version sets the version that the load folds into every unit key. A
// frontend changes it with every change to the graph that it produces. A warm cache keyed
// without the version serves the old graph after a change of the
// frontend, and nothing reports why.
func (b *Builder) Version(v string) *Builder {
	b.version = v
	return b
}

// Overloads declares that the language overloads, so one scope can have
// two callables of one name that their parameters tell apart. The load
// then spells the discriminator of each callable from its parameters. A
// discriminator has the type spellings, which the parse normalizes, with
// the arguments of their instantiations and their variadic and optional
// marks. Without Overloads, the declaration describes a language that
// cannot overload, and every callable that it loads takes the empty
// discriminator.
func (b *Builder) Overloads() *Builder {
	b.overloads = true
	return b
}

// Match appends selection patterns to the claim of files. A pattern is a
// glob that matches the whole workspace-relative path, and "**" matches
// any number of segments. The patterns apply in order and the last match
// decides, so a later negation removes files from an earlier claim. The
// claim has no policy, and the consumer decides through scopes whether
// test files take part.
func (b *Builder) Match(patterns ...string) *Builder {
	b.selection = append(b.selection, patterns...)
	return b
}

// Units sets the partition, which groups the selected files into the
// compilation grain of the language through the recorded reader.
func (b *Builder) Units(
	partition func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error),
) *Builder {
	b.partition = partition
	return b
}

// Parse sets the parse of a unit. The parse writes the declarations of
// one unit into its builder and reports problems through its findings.
func (b *Builder) Parse(
	parse func(context.Context, *plugin.SourceUnit) error,
) *Builder {
	b.parse = parse
	return b
}

// Classify appends classifiers. They run after the parse in declaration
// order.
func (b *Builder) Classify(cs ...Classifier) *Builder {
	b.classifiers = append(b.classifiers, cs...)
	return b
}

// Resolve sets the resolution of the language, which returns the
// candidates of one spelling in the recorded bindings of one file, in
// probe order and in shadowing tiers. A language that resolves nothing
// declares a resolve that returns nothing, so the declaration states the
// absence and the kit does not assume it.
func (b *Builder) Resolve(
	resolve func(plugin.ImportScope, string) plugin.Candidates,
) *Builder {
	b.resolve = resolve
	return b
}

// Options declares the configuration of the frontend as any plugin
// declares one. o is a pointer whose exported fields are the options and
// whose constructed values are the defaults. The built frontend
// implements [plugin.OptionsProvider], and the load folds the canonical
// encoding of the populated value into every unit key.
func (b *Builder) Options(o any) *Builder {
	b.options = o
	b.hasOptions = true
	return b
}

// Dependencies declares the dependency rounds of the language. A round
// returns the units that declare what the loaded files import from
// outside the workspace, and it reads them through its recorded reader. It
// reports through the round each need that the language cannot place.
// The built frontend implements [plugin.Dependent], and the load parses
// every unit that the function returns at [plugin.DepthSignatures].
func (b *Builder) Dependencies(
	dependencies func(context.Context, *plugin.DependencyRound, plugin.StoreReader) ([][]plugin.SourceRef, error),
) *Builder {
	b.dependencies = dependencies
	return b
}

// Exports declares what a file publishes and does not declare, in
// shadowing tiers, as a re-export does. The built frontend implements
// [plugin.Exporter], and the resolution step follows a candidate without
// a declaration through the function.
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

// Imports declares the importer role of a language whose import names a
// file and not a package, such as protobuf. importOf returns the spelling
// with which the imports of the file of a scope refer to the file at a
// workspace path. The
// built frontend implements [plugin.Importer] through the function, and
// the load then records the file of each declaration of the language.
func (b *Builder) Imports(importOf func(scope plugin.ImportScope, file string) string) *Builder {
	b.importOf = importOf
	return b
}

// Keys declares the key registrations of the language: the keys that the
// frontend and its classifiers stamp, and the keys of the language that
// its other parts stamp. The built frontend runs them as
// [plugin.KeyProvider], and the workspace calls it under the spelling of
// the language, so the namespace belongs to the language. Repeated calls
// append, and the registrations run in declaration order.
func (b *Builder) Keys(register ...func(r *meta.Registry) error) *Builder {
	b.keys = append(b.keys, register...)
	return b
}

// Build freezes the declaration and returns the lowered frontend,
// which implements [plugin.Frontend] and [plugin.KeyProvider]. The
// conformance suite runs the same checks over it that a hand-rolled
// frontend meets, because the lowering adds nothing the role does not
// state. Its [plugin.KeyProvider] role registers the declared
// registrations, and none for a declaration without them. The frontend
// implements another optional role exactly when the declaration has it:
// [plugin.OptionsProvider] for [Builder.Options], [plugin.Dependent]
// for [Builder.Dependencies], [plugin.Exporter] for [Builder.Exports],
// [plugin.StoreLocator] for [Builder.Stores] and [plugin.Importer] for
// [Builder.Imports].
//
// Build panics on a declaration defect: an empty name or language,
// no declared version, an empty claim, a missing partition, parse or
// resolve, a nil key registration, or stores without dependency
// rounds. A wrong declaration is a bug in the constructor of the
// frontend, and it panics on the first Build in any test.
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
	if slices.ContainsFunc(b.keys, func(register func(*meta.Registry) error) bool { return register == nil }) {
		defects = append(defects, "declares a nil key registration")
	}
	if len(defects) > 0 {
		panic("frontend: " + name + " " + strings.Join(defects, ", and "))
	}
	base := &builtFrontend{
		name: b.name, lang: b.lang, syntax: b.syntax, overloads: b.overloads, version: b.version,
		selection: b.selection, partition: b.partition, parse: b.parse,
		resolve: b.resolve, classifiers: b.classifiers, keys: b.keys,
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
	if b.importOf != nil {
		return importing(base, roles, o, d, e, l, importerRole{b.importOf})
	}
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

// role is one optional role that a declaration adds to its built
// frontend. It is a bit of the set from which Build composes the type of
// the frontend.
type role uint8

// The optional roles beside the importer role are the options that a load
// keys on, the dependency rounds, the re-exports that the resolution step
// follows, and the stores that the dependency rounds read. Build refuses
// the store role without the dependent role, so no case of its
// composition has the store role alone.
const (
	optioned  role = 1
	dependent role = 2
	exporter  role = 4
	locator   role = 8
)

// builtFrontend is a lowered frontend declaration. It has the facts as
// data, and the methods of the role call the declared functions.
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
	keys        []func(r *meta.Registry) error
}

// Name returns the identity of the frontend.
func (f *builtFrontend) Name() plugin.ID { return f.name }

// Lang returns the language of every declaration that the frontend loads.
func (f *builtFrontend) Lang() symbol.Lang { return f.lang }

// Syntax returns the comment forms of the language.
func (f *builtFrontend) Syntax() plugin.CommentSyntax { return f.syntax }

// Overloads reports whether the declaration stated that the language
// overloads.
func (f *builtFrontend) Overloads() bool { return f.overloads }

// Version returns the declared version for [plugin.Versioned].
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

// Parse loads one unit through the declared function, and then runs the
// classifiers over it in declaration order. An error of a classifier ends
// the load as an error of the parse does, so the load seals no graph with
// half of its stamps.
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

// Resolve returns the candidates that the declared function returns.
func (f *builtFrontend) Resolve(
	scope plugin.ImportScope, spelling string,
) plugin.Candidates {
	return f.resolve(scope, spelling)
}

// Keys runs the declared registrations of [plugin.KeyProvider] in
// declaration order and joins their errors, so the composition reads every
// error at once. It allocates what the registrations allocate.
func (f *builtFrontend) Keys(r *meta.Registry) error {
	var errs []error
	for _, register := range f.keys {
		if err := register(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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

// importerRole is the importer role of a built frontend that declares how
// the imports of a file refer to another file.
type importerRole struct {
	importOf func(plugin.ImportScope, string) string
}

// ImportOf implements [plugin.Importer] through the declared function.
func (r importerRole) ImportOf(scope plugin.ImportScope, file string) string {
	return r.importOf(scope, file)
}

// importing returns base in the importer role i and in the other optional
// roles of the set, which Build composes in the same way without the
// importer role.
func importing(
	base *builtFrontend, roles role, o optionsRole, d dependentRole, e exporterRole, l locatorRole, i importerRole,
) plugin.Frontend {
	switch roles {
	case optioned:
		return &struct {
			*builtFrontend
			optionsRole
			importerRole
		}{base, o, i}
	case dependent:
		return &struct {
			*builtFrontend
			dependentRole
			importerRole
		}{base, d, i}
	case exporter:
		return &struct {
			*builtFrontend
			exporterRole
			importerRole
		}{base, e, i}
	case optioned | dependent:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			importerRole
		}{base, o, d, i}
	case optioned | exporter:
		return &struct {
			*builtFrontend
			optionsRole
			exporterRole
			importerRole
		}{base, o, e, i}
	case dependent | exporter:
		return &struct {
			*builtFrontend
			dependentRole
			exporterRole
			importerRole
		}{base, d, e, i}
	case optioned | dependent | exporter:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			exporterRole
			importerRole
		}{base, o, d, e, i}
	case dependent | locator:
		return &struct {
			*builtFrontend
			dependentRole
			locatorRole
			importerRole
		}{base, d, l, i}
	case optioned | dependent | locator:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			locatorRole
			importerRole
		}{base, o, d, l, i}
	case dependent | exporter | locator:
		return &struct {
			*builtFrontend
			dependentRole
			exporterRole
			locatorRole
			importerRole
		}{base, d, e, l, i}
	case optioned | dependent | exporter | locator:
		return &struct {
			*builtFrontend
			optionsRole
			dependentRole
			exporterRole
			locatorRole
			importerRole
		}{base, o, d, e, l, i}
	default:
		return &struct {
			*builtFrontend
			importerRole
		}{base, i}
	}
}
