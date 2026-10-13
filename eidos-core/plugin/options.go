// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"

	"go.dokimi.dev/eidos/core/jsonschema"
)

// The struct tags of an options struct and the values that the checks
// compare them with.
const (
	// optTag contains the config key of an option.
	optTag = "opt"
	// docTag contains the meaning of an option.
	docTag = "doc"
	// schemaTag marks an option that needs a value.
	schemaTag = "schema"
	// requiredMark is the one value of schemaTag. Build refuses an option
	// with the mark that has the zero value of its type.
	requiredMark = "required"
	// jsonTag contains the JSON key of a field, and jsonSkip is the key of
	// a field that the encoding leaves out.
	jsonTag  = "json"
	jsonSkip = "-"
)

// The keywords and the type name of JSON Schema that [OptionsSchema]
// writes.
const (
	keywordType        = "type"
	keywordProperties  = "properties"
	keywordAdditional  = "additionalProperties"
	keywordRequired    = "required"
	keywordDescription = "description"
	typeObject         = "object"
)

// ValidateOptions checks the options struct of p against the tag contract
// and returns one error for each finding, in field order. The contract has
// these rules:
//
//   - Options returns a pointer to a struct that is not nil.
//   - Each field is exported.
//   - Each field has an opt tag with its config key, and no two fields
//     have one key.
//   - Each field has a doc tag with its meaning.
//   - A schema tag has the value required.
//
// ValidateOptions returns nothing for a plugin that declares no options.
// The composition runs it before it populates the options, and the plugin
// suite runs it as a check.
//
// # Allocation contract
//
// ValidateOptions allocates the set of claimed keys, the reflection of the
// fields and the errors that it returns. It allocates nothing for a plugin
// without the surface.
func ValidateOptions(p Plugin) []error {
	op, held := p.(OptionsProvider)
	if !held {
		return nil
	}
	cfg := op.Options()
	if cfg == nil {
		return nil
	}
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.Type().Elem().Kind() != reflect.Struct {
		return []error{fmt.Errorf(
			"plugin: %s declares options that are not a pointer to a struct", p.Name(),
		)}
	}
	if rv.IsNil() {
		return []error{fmt.Errorf(
			"plugin: %s declares a nil options struct: the constructed values are the defaults",
			p.Name(),
		)}
	}

	var errs []error
	claimed := map[string]string{}
	t := rv.Type().Elem()
	for f := range t.Fields() {
		if !f.IsExported() {
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s is unexported: the composition cannot populate it",
				p.Name(), f.Name,
			))
			continue
		}
		key := f.Tag.Get(optTag)
		switch first, taken := claimed[key]; {
		case key == "":
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s has no opt tag with its config key",
				p.Name(), f.Name,
			))
		case taken:
			errs = append(errs, fmt.Errorf(
				"plugin: %s option key %q is claimed twice: by %s and by %s",
				p.Name(), key, first, f.Name,
			))
		default:
			claimed[key] = f.Name
		}
		if f.Tag.Get(docTag) == "" {
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s has no doc tag with its meaning",
				p.Name(), f.Name,
			))
		}
		if mark, marked := f.Tag.Lookup(schemaTag); marked && mark != requiredMark {
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s has the schema tag %q, and the one value of the tag is %s",
				p.Name(), f.Name, mark, requiredMark,
			))
		}
	}
	return errs
}

// MissingOptions returns one error for each option of p that the tag
// schema:"required" marks and that has the zero value of its type, in
// field order. The message of each error contains the name of the plugin
// and the key of the option. MissingOptions returns nothing for a plugin
// that declares no options. It also returns nothing for options that are
// not a pointer to a struct, because ValidateOptions reports them.
//
// # Allocation contract
//
// MissingOptions allocates the errors that it returns and nothing else.
func MissingOptions(p Plugin) []error {
	op, held := p.(OptionsProvider)
	if !held {
		return nil
	}
	rv := reflect.ValueOf(op.Options())
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return nil
	}
	v := rv.Elem()
	t := v.Type()
	var errs []error
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Tag.Get(schemaTag) == requiredMark && v.Field(i).IsZero() {
			errs = append(errs, fmt.Errorf(
				"plugin: %s requires the option %s, which has the zero value of its type",
				p.Name(), f.Tag.Get(optTag),
			))
		}
	}
	return errs
}

// OptionsSchema returns the JSON Schema, draft 2020-12, of the options of p
// in a config file. The schema is a closed object with one property for
// each option. The property has the key of the opt tag, the doc tag as its
// description, and the schema of the type of the field that
// [jsonschema.Of] returns. The options step decodes a value below an option
// through JSON, so the schema reads the keys of a struct below an option
// from its json tags.
//
// An option with the tag schema:"required" is a required property while it
// has the zero value of its type. A config file need not set an option that
// the composition already set. OptionsSchema returns nil for a plugin that
// declares no options, and for options that [ValidateOptions] reports.
func OptionsSchema(p Plugin) map[string]any {
	op, held := p.(OptionsProvider)
	if !held || op.Options() == nil || len(ValidateOptions(p)) > 0 {
		return nil
	}
	v := reflect.ValueOf(op.Options()).Elem()
	t := v.Type()
	properties := make(map[string]any, t.NumField())
	var required []string
	for i := range t.NumField() {
		f := t.Field(i)
		key := f.Tag.Get(optTag)
		property := jsonschema.Of(f.Type, jsonschema.TagJSON)
		property[keywordDescription] = f.Tag.Get(docTag)
		properties[key] = property
		if f.Tag.Get(schemaTag) == requiredMark && v.Field(i).IsZero() {
			required = append(required, key)
		}
	}
	out := map[string]any{keywordType: typeObject, keywordProperties: properties, keywordAdditional: false}
	if len(required) > 0 {
		out[keywordRequired] = required
	}
	return out
}

// EncodeOptions returns the options of p in their canonical encoding, and
// nil for a plugin that declares no options. A unit key and the
// composition fingerprint fold the encoding. The encoding is encoding/json
// over the declared struct, and a type that marshals itself encodes as it
// marshals. Every option is inside the encoding, so a changed option
// always changes the key.
//
// Error modes: EncodeOptions returns an error for a struct that the
// encoding cannot see whole. Such a struct has an unexported field at any
// depth, a field that its json tag hides, or a value that the encoder
// refuses.
//
// # Allocation contract
//
// EncodeOptions allocates the set of seen types, the reflection of each
// field, the encoded bytes, and what encoding/json allocates for its
// state when its pool is empty. It allocates nothing for a plugin without
// the surface.
func EncodeOptions(p Plugin) ([]byte, error) {
	op, held := p.(OptionsProvider)
	if !held {
		return nil, nil
	}
	o := op.Options()
	if o == nil {
		return nil, nil
	}
	if field, hidden := hiddenField(reflect.TypeOf(o), map[reflect.Type]bool{}); hidden {
		return nil, fmt.Errorf(
			"plugin: %s options hide %s from the encoding, and an option the key cannot see "+
				"reuses stale output", p.Name(), field,
		)
	}
	encoded, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("plugin: encode %s options: %w", p.Name(), err)
	}
	return encoded, nil
}

// The interfaces of a type that encodes itself.
var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// hiddenField returns the qualified name of the first field below t that
// the JSON encoding cannot see, and false when the encoding sees every
// field. seen ends the walk of a recursive type.
func hiddenField(t reflect.Type, seen map[reflect.Type]bool) (string, bool) {
	for {
		if marshals(t) {
			return "", false
		}
		if k := t.Kind(); k != reflect.Pointer && k != reflect.Slice && k != reflect.Array && k != reflect.Map {
			break
		}
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return "", false
	}
	seen[t] = true
	for f := range t.Fields() {
		switch {
		case f.Tag.Get(jsonTag) == jsonSkip:
			return t.String() + "." + f.Name, true
		case !f.IsExported() && (!f.Anonymous || f.Type.Kind() != reflect.Struct):
			return t.String() + "." + f.Name, true
		}
		if name, hidden := hiddenField(f.Type, seen); hidden {
			return name, true
		}
	}
	return "", false
}

// marshals reports whether a value of t or a pointer to it encodes itself.
func marshals(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(jsonMarshaler) || t.Implements(textMarshaler) ||
		pt.Implements(jsonMarshaler) || pt.Implements(textMarshaler)
}
