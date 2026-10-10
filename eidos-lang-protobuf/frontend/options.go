// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	"github.com/bufbuild/protocompile/experimental/ast"
	"github.com/bufbuild/protocompile/experimental/seq"

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

// option is one option as written: its name and its value, each
// read once, and the expression the value came from.
type option struct {
	name  string
	value string
	val   ast.ExprAny
}

// compactOptions reads the options of a compact list, which a field, an
// enum value and an extension range state in brackets, into their names
// and values. It skips an option without a name.
func compactOptions(o ast.CompactOptions) []option {
	if o.IsZero() {
		return nil
	}
	entries := o.Entries()
	out := make([]option, 0, entries.Len())
	for i := range entries.Len() {
		if e := entries.At(i); !e.Path.IsZero() {
			out = append(out, option{name: e.Path.Span().Text(), value: e.Value.Span().Text(), val: e.Value})
		}
	}
	return out
}

// statementOptions reads the option statements among a body's
// declarations into their names and values: a file, a message, a oneof,
// an enum, a service and an rpc state their options as statements, not
// as a compact list. It skips an option without a name.
func statementOptions(decls seq.Inserter[ast.DeclAny]) []option {
	var out []option
	for i := range decls.Len() {
		def := decls.At(i).AsDef()
		if def.Classify() != ast.DefKindOption {
			continue
		}
		if o := def.AsOption().Option; !o.Path.IsZero() {
			out = append(out, option{name: o.Path.Span().Text(), value: o.Value.Span().Text(), val: o.Value})
		}
	}
	return out
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

// stringValue returns an option's value with its quotes removed, for
// an option whose value is a string, and the value as written for
// every other option.
func (o option) stringValue() string {
	if s := o.val.AsLiteral().AsString(); !s.IsZero() {
		return s.Text()
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
