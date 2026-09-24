// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
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
// parse strips through, the file claim, and the functions the
// pipeline varies in. Everything on it is data except the
// functions. Build freezes it, and a Builder is not reused
// afterwards.
type Builder struct {
	name        plugin.ID
	lang        symbol.Lang
	syntax      plugin.CommentSyntax
	version     string
	selection   []string
	partition   func(context.Context, []plugin.SourceRef, plugin.FileReader) ([][]plugin.SourceRef, error)
	parse       func(context.Context, *plugin.SourceUnit) error
	resolve     func(plugin.ImportScope, string) plugin.Candidates
	classifiers []Classifier
	options     any
	hasOptions  bool
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

// Build freezes the declaration and returns the lowered frontend,
// which implements [plugin.Frontend]. The conformance suite runs the
// same checks over it that a hand-rolled frontend meets, because the
// lowering adds nothing the role does not state.
//
// Build panics on a declaration defect: an empty name or language,
// no declared version, an empty claim, or a missing partition,
// parse or resolve. A wrong declaration is a bug in the frontend's
// own constructor and panics on the first Build in any test.
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
	if len(defects) > 0 {
		panic("frontend: " + name + " " + strings.Join(defects, ", and "))
	}
	base := &builtFrontend{
		name: b.name, lang: b.lang, syntax: b.syntax, version: b.version,
		selection: b.selection, partition: b.partition, parse: b.parse,
		resolve: b.resolve, classifiers: b.classifiers,
	}
	if b.hasOptions {
		return &optionedFrontend{builtFrontend: base, options: b.options}
	}
	return base
}

// builtFrontend is a lowered frontend declaration: the facts as
// data, and the declared functions behind the role's methods.
type builtFrontend struct {
	name        plugin.ID
	lang        symbol.Lang
	syntax      plugin.CommentSyntax
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

// optionedFrontend is a built frontend declaring configuration.
type optionedFrontend struct {
	*builtFrontend
	options any
}

// Options implements [plugin.OptionsProvider] with the declared
// value.
func (f *optionedFrontend) Options() any { return f.options }
