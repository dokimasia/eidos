// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"errors"

	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// ID is the name of the plugin.
const ID plugin.ID = "defaults"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// Capability is the capability that defaults provides. A plugin that reads
// the declared defaults requires it, so it runs after defaults.
const Capability plugin.Capability = "defaults"

// DefaultDirective is the directive that declares the default value of a
// field. ValueParam is its one positional param, which is the value.
const (
	DefaultDirective directive.Name     = "default"
	ValueParam       directive.ParamKey = "value"
)

// KeyValue is the key of the fact that contains the text of the default
// value of a field. KeySource is the key of the fact that contains the
// form that declares the value: [SourceDirective] or [SourceTag].
const (
	KeyValue  meta.KeyName = "defaults.value"
	KeySource meta.KeyName = "defaults.source"
)

// SourceDirective and SourceTag are the values of [KeySource]. The
// directive declares the default of a field in every language, and a tag
// declares it in a language whose rules read tags.
const (
	SourceDirective = "directive"
	SourceTag       = "tag"
)

// DefaultTag is the default of the option tag: the key of the tag that
// contains the default value of a field.
const DefaultTag = "default"

// schema is the directive of defaults.
var schema = directive.Schema{
	Name: DefaultDirective,
	Positional: []directive.ParamSpec{{
		Key: ValueParam, Type: directive.TypeString, Required: true,
		Doc: "the default value of the field, as text that the rules of the language of the field read",
	}},
	Doc: "declares the default value of the field",
}

// Options are the options of defaults. Tag is the key of the tag that the
// bare rule reads.
type Options struct {
	Tag string `opt:"tag" doc:"the key of the field tag that contains the default value of the field"`
}

// conclusion is the state of one instance of defaults: its options, and
// the handles of its keys, which the registration sets and the rules stamp
// through.
//
// # Concurrency
//
// The composition sets the options and registers the keys before the first
// phase call, and the handlers read them afterwards, so the workers of a
// phase call read them concurrently.
type conclusion struct {
	options *Options
	value   meta.Key[string]
	source  meta.Key[string]
}

// New returns defaults, an annotator. It stamps the text of the default
// value of a field under [KeyValue], and the form that declares it under
// [KeySource]. The directive rule reads [DefaultDirective] in every
// language. The bare rule reads the tag of the key that the option tag
// sets, in each language whose rules implement rules.TagRules. Within one
// plugin, the first claim in rule order takes precedence, and the directive
// rule comes first. So on a field with both forms, the value of the
// directive becomes the fact. A reader turns the text into a value through
// the rules of the language of the field. Each call returns a new plugin,
// because a plugin instance belongs to one workspace.
func New() plugin.Annotator {
	c := &conclusion{options: &Options{Tag: DefaultTag}}
	return sdk.NewPlugin(ID).
		Version(Version).
		Provides(Capability).
		Options(c.options).
		Keys(c.register).
		Handle(
			sdk.Directive(schema, sdk.OnField(c.declared)),
			sdk.OnField(c.tagged),
		).
		Annotator()
}

// register claims the namespace of defaults and registers [KeyValue] and
// [KeySource].
//
// Error modes: the error of the claim, such as a namespace that another
// registrant claimed, and the errors of the two registrations, joined.
func (c *conclusion) register(r *meta.Registry) error {
	if err := r.ClaimNamespace(KeyValue.Namespace()); err != nil {
		return err
	}
	var errValue, errSource error
	c.value, errValue = meta.Register[string](r, meta.KeySpec{
		Name:  KeyValue,
		Kinds: []symbol.Kind{symbol.KindField},
		Doc:   "contains the text of the default value of a field",
	})
	c.source, errSource = meta.Register[string](r, meta.KeySpec{
		Name:  KeySource,
		Kinds: []symbol.Kind{symbol.KindField},
		Doc:   "contains the form that declares the default value of a field: directive or tag",
	})
	return errors.Join(errValue, errSource)
}

// declared stamps the value of the directive on its field, with the source
// [SourceDirective]. The schema requires the one positional value, so
// validation passes no instance without it.
func (c *conclusion) declared(m *sdk.FieldMatch, st *sdk.Stamper) error {
	sdk.Stamp(st, c.value, m.Directive().Args[0].Str)
	sdk.Stamp(st, c.source, SourceDirective)
	return nil
}

// tagged stamps the value of the tag of the field under the key of the
// option tag, with the source [SourceTag]. It does not stamp a field
// without the tag, a field whose tag has an empty value, or a field of a
// language without tags. An empty value declares no default, because a
// reader would assign the zero value over a value that a seed set.
func (c *conclusion) tagged(m *sdk.FieldMatch, st *sdk.Stamper) error {
	if text, stated := m.Rules().Tag(m.Field, c.options.Tag); stated && text != "" {
		sdk.Stamp(st, c.value, text)
		sdk.Stamp(st, c.source, SourceTag)
	}
	return nil
}
