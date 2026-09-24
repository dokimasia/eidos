// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"github.com/bufbuild/protocompile/ast"

	protobuf "go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The option spellings the lowering reads.
const (
	// featuresPrefix opens an edition feature, which is stamped apart
	// from the other options.
	featuresPrefix = "features."
	// featurePresence is the edition feature that decides whether a
	// singular field has presence.
	featurePresence = featuresPrefix + "field_presence"
	// jsonNameOption is the field option that renames a field in
	// JSON, which decides its spelling there and nothing about its
	// type.
	jsonNameOption = "json_name"
	// defaultOption is proto2's field default, whose value the model
	// stores on the field itself.
	defaultOption = "default"
	// optionAssign joins a stamped option's name to its value.
	optionAssign = "="
)

// The field presence values the lowering matches. The third value an
// edition states, IMPLICIT, gives a singular field no presence, the
// way a plain proto3 field has none, so the field's type is the type
// itself.
const (
	// presenceExplicit gives a singular field presence, which the
	// optional form projects. It is the edition default.
	presenceExplicit = "EXPLICIT"
	// presenceLegacyRequired makes a field required, the way proto2's
	// required label does.
	presenceLegacyRequired = "LEGACY_REQUIRED"
)

// option is one option as written: its name and its value, each
// read once, and the node the value came from.
type option struct {
	name  string
	value string
	val   ast.ValueNode
}

// optionList reads a list of option nodes into their names and
// values, skipping a nil entry.
func (l *lowered) optionList(nodes []*ast.OptionNode) []option {
	out := make([]option, 0, len(nodes))
	for _, o := range nodes {
		if o == nil {
			continue
		}
		out = append(out, option{name: l.text(o.Name), value: l.text(o.Val), val: o.Val})
	}
	return out
}

// statementOptions collects the option statements of a body: a
// message, a oneof, an enum, a service and an rpc state their options
// as statements, not as a compact list.
func statementOptions[E any](decls []E) []*ast.OptionNode {
	var out []*ast.OptionNode
	for _, decl := range decls {
		if o, is := any(decl).(*ast.OptionNode); is {
			out = append(out, o)
		}
	}
	return out
}

// compact returns the options of a compact list, which a field, an
// enum value and an extension range state in brackets.
func compact(o *ast.CompactOptionsNode) []*ast.OptionNode {
	if o == nil {
		return nil
	}
	return o.Options
}

// named returns the option a list states under one name, and false
// where the list states none.
func named(opts []option, name string) (option, bool) {
	for _, o := range opts {
		if o.name == name {
			return o, true
		}
	}
	return option{}, false
}

// stringValue returns an option's value with its quotes removed,
// for an option whose value is a string.
func (o option) stringValue() string {
	if s, is := o.val.(ast.StringValueNode); is {
		return s.AsString()
	}
	return o.value
}

// stampOptions stamps a declaration's plain options under
// [protobuf.OptionsKey] and its edition features under
// [protobuf.FeaturesKey], each spelled name=value as written. A
// declaration stating neither is not stamped.
func (l *lowered) stampOptions(subject symbol.Symbol, at position.Pos, opts []option) {
	var plain, features []string
	for _, o := range opts {
		entry := o.name + optionAssign + o.value
		if strings.HasPrefix(o.name, featuresPrefix) {
			features = append(features, entry)
			continue
		}
		plain = append(plain, entry)
	}
	gb := l.unit.Graph()
	if len(plain) > 0 {
		gb.Stamp(subject, meta.RawStamp{
			Key: protobuf.OptionsKey, Value: strings.Join(plain, listSep), Pos: at,
		})
	}
	if len(features) > 0 {
		gb.Stamp(subject, meta.RawStamp{
			Key: protobuf.FeaturesKey, Value: strings.Join(features, listSep), Pos: at,
		})
	}
}

// withoutCarried drops the options the model stores on a field
// itself, so a stamp never restates what the field states.
func withoutCarried(opts []option) []option {
	out := make([]option, 0, len(opts))
	for _, o := range opts {
		switch o.name {
		case defaultOption, jsonNameOption:
		default:
			out = append(out, o)
		}
	}
	return out
}

// presenceOf returns the field presence an option list states, and
// the inherited presence where it states none.
func presenceOf(opts []option, inherited string) string {
	if o, stated := named(opts, featurePresence); stated {
		return o.value
	}
	return inherited
}
