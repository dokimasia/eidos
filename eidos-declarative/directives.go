// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/jsonschema"
)

// directiveDecl is one directive of the manifest. The loader lowers it to
// a [directive.Schema] of keyed params under the name of the plugin. A
// manifest sets no other field of a schema.
type directiveDecl struct {
	Name       directive.Name `yaml:"name"       schema:"required" doc:"The name of the directive. A source file writes it after the mark of the brand, as in +acme:audit. A name is a letter followed by letters, digits, hyphens and underscores."`
	Doc        string         `yaml:"doc"        schema:"required" doc:"The meaning of the directive."`
	Deprecated string         `yaml:"deprecated"                   doc:"The rewrite of a deprecated directive, such as write trace in place of audit. A run reports each instance of a deprecated directive."`
	Params     []paramDecl    `yaml:"params"                       doc:"The keyed params of the directive."`

	// line is the line of the directive's mapping.
	line int
}

// schema returns the directive's schema under the name of the plugin.
func (d directiveDecl) schema(plugin string) directive.Schema {
	var params []directive.ParamSpec
	if len(d.Params) > 0 {
		params = make([]directive.ParamSpec, 0, len(d.Params))
	}
	for _, p := range d.Params {
		params = append(params, directive.ParamSpec{
			Key:        p.Key,
			Type:       directive.ParamType(p.Type),
			ListOf:     directive.ParamType(p.Of),
			Resolution: directive.ResolutionKind(p.Resolve),
			Required:   p.Required,
			Choices:    p.Choices,
			Doc:        p.Doc,
			Deprecated: p.Deprecated,
		})
	}
	return directive.Schema{Plugin: plugin, Name: d.Name, Params: params, Doc: d.Doc, Deprecated: d.Deprecated}
}

// position records the line of the directive and of each of its params from
// the mapping n that the directive decoded from.
func (d *directiveDecl) position(n *yaml.Node) {
	d.line = n.Line
	for i, p := range elements(value(n, paramsKey), len(d.Params)) {
		d.Params[i].line = p.Line
	}
}

// paramDecl is one keyed param of a directive of the manifest.
type paramDecl struct {
	Key        directive.ParamKey `yaml:"key"        schema:"required" doc:"The key of the param. A source file writes it before an equals sign, as in level=info."`
	Type       paramType          `yaml:"type"       schema:"required" doc:"The type of the value: string, int, bool, list or reference."`
	Of         elementType        `yaml:"of"                           doc:"The type of the elements of a list: string, int, bool or reference."`
	Resolve    resolution         `yaml:"resolve"                      doc:"How validation resolves a reference, or each reference of a list: callable-in-scope, package-var, value-field, host-param, member-on-handle or type-in-scope."`
	Required   bool               `yaml:"required"                     doc:"When true, validation refuses an instance without the param."`
	Choices    []string           `yaml:"choices"                      doc:"The spellings that a string param accepts. A string param without choices accepts every string."`
	Doc        string             `yaml:"doc"        schema:"required" doc:"The meaning of the param."`
	Deprecated string             `yaml:"deprecated"                   doc:"The rewrite of a deprecated param, such as write sink= in place of target=. A run reports each instance that writes the param."`

	// line is the line of the param's mapping.
	line int
}

// check returns an Error for an element type on a param that is not a
// list, and for a resolution on a param that is neither a reference nor a
// list of references. The directive registry ignores both fields there, so
// without the check either field would have no effect.
func (p paramDecl) check(file string) []*Error {
	var out []*Error
	kind, of := directive.ParamType(p.Type), directive.ParamType(p.Of)
	if of != 0 && kind != directive.TypeList {
		out = append(out, &Error{
			File: file,
			Line: p.line,
			Msg:  fmt.Sprintf("the param %s has an element type, and only a list has one", p.Key),
		})
	}
	references := kind == directive.TypeReference || kind == directive.TypeList && of == directive.TypeReference
	if p.Resolve != 0 && !references {
		out = append(out, &Error{
			File: file,
			Line: p.line,
			Msg:  fmt.Sprintf("the param %s resolves, and only a reference or a list of references resolves", p.Key),
		})
	}
	return out
}

// paramType is the type of a param, as a manifest writes it.
type paramType directive.ParamType

// paramTypes are the spellings of the type of a param.
var paramTypes = []word[directive.ParamType]{
	{spelling: spellString, value: directive.TypeString},
	{spelling: spellInt, value: directive.TypeInt},
	{spelling: spellBool, value: directive.TypeBool},
	{spelling: spellList, value: directive.TypeList},
	{spelling: spellReference, value: directive.TypeReference},
}

var (
	_ yaml.Unmarshaler   = (*paramType)(nil)
	_ jsonschema.Schemer = paramType(0)
)

// UnmarshalYAML decodes the type of a param from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not a type.
func (t *paramType) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, paramTypes, "a param type")
	*t = paramType(v)
	return err
}

// JSONSchema returns the JSON Schema of the type of a param, a new map on
// each call.
func (paramType) JSONSchema() map[string]any { return wordsSchema(paramTypes) }

// elementType is the type of the elements of a list param, as a manifest
// writes it. A list does not contain lists.
type elementType directive.ParamType

// elementTypes are the spellings of the type of the elements of a list.
var elementTypes = []word[directive.ParamType]{
	{spelling: spellString, value: directive.TypeString},
	{spelling: spellInt, value: directive.TypeInt},
	{spelling: spellBool, value: directive.TypeBool},
	{spelling: spellReference, value: directive.TypeReference},
}

var (
	_ yaml.Unmarshaler   = (*elementType)(nil)
	_ jsonschema.Schemer = elementType(0)
)

// UnmarshalYAML decodes the type of the elements of a list from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not an element type.
func (t *elementType) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, elementTypes, "an element type")
	*t = elementType(v)
	return err
}

// JSONSchema returns the JSON Schema of the type of the elements of a list,
// a new map on each call.
func (elementType) JSONSchema() map[string]any { return wordsSchema(elementTypes) }

// resolution is how validation resolves a reference param, as a manifest
// writes it. A declarative plugin has no metadata keys and no diagnostic
// codes, so a manifest cannot write the resolutions of either.
type resolution directive.ResolutionKind

// resolutions are the spellings of the resolution of a reference param.
var resolutions = []word[directive.ResolutionKind]{
	{spelling: "callable-in-scope", value: directive.ResolveCallableInScope},
	{spelling: "package-var", value: directive.ResolvePackageVar},
	{spelling: "value-field", value: directive.ResolveValueField},
	{spelling: "host-param", value: directive.ResolveHostParam},
	{spelling: "member-on-handle", value: directive.ResolveMemberOnHandle},
	{spelling: "type-in-scope", value: directive.ResolveTypeInScope},
}

var (
	_ yaml.Unmarshaler   = (*resolution)(nil)
	_ jsonschema.Schemer = resolution(0)
)

// UnmarshalYAML decodes the resolution of a reference param from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not a resolution.
func (r *resolution) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, resolutions, "a resolution")
	*r = resolution(v)
	return err
}

// JSONSchema returns the JSON Schema of the resolution of a reference param,
// a new map on each call.
func (resolution) JSONSchema() map[string]any { return wordsSchema(resolutions) }

// checkDirectives returns an Error for each fault of the directives of a
// manifest: a schema that the directive registry refuses, a field of a
// param that the param's type ignores, and a directive that no rule has.
// The registry refuses every schema under a plugin name that is not an
// identifier, and the check of the name reports that fault once, so
// checkDirectives registers the schemas only under a valid name.
func checkDirectives(file, plugin string, ds []directiveDecl, rules []ruleDecl) []*Error {
	var out []*Error
	gating := make(map[directive.Name]bool, len(rules))
	for _, r := range rules {
		gating[r.Directive] = true
	}
	reg := directive.NewRegistry()
	registers := directive.ParamKey(plugin).Valid()
	for _, d := range ds {
		if registers {
			if err := reg.Register(d.schema(plugin)); err != nil {
				out = append(out, &Error{File: file, Line: d.line, Msg: err.Error()})
			}
		}
		for _, p := range d.Params {
			out = append(out, p.check(file)...)
		}
		if !gating[d.Name] {
			msg := fmt.Sprintf("no rule has the directive %s", d.Name)
			out = append(out, &Error{File: file, Line: d.line, Msg: msg})
		}
	}
	return out
}
