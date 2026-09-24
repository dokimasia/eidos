// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
)

// ValidateOptions holds a plugin's options declaration to the tag
// contract: a pointer to a struct, exported fields only, an opt
// tag naming each field's config key, a doc tag stating its
// meaning, and no key claimed twice. It returns one error per
// finding, in field order, and nothing where the plugin declares
// no options. The composition runs it before populating; the
// conformance suite runs it as a check, so a plugin failing at
// composition fails in its own tests first.
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
		key := f.Tag.Get("opt")
		switch first, taken := claimed[key]; {
		case key == "":
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s carries no opt tag naming its config key",
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
		if f.Tag.Get("doc") == "" {
			errs = append(errs, fmt.Errorf(
				"plugin: %s option field %s carries no doc tag stating its meaning",
				p.Name(), f.Name,
			))
		}
	}
	return errs
}

// EncodeOptions returns a plugin's options in their canonical
// encoding, which is what a unit key and the composition
// fingerprint fold, and nil for a plugin declaring none. The
// encoding is encoding/json over the declared struct. It refuses a
// struct the encoding cannot see whole: an unexported field at any
// depth, a field its json tag hides, and a value the encoder
// refuses. A type that marshals itself is taken as it encodes.
// Every option is inside the encoding, so a changed option always
// changes the key.
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

// Interfaces a type implements when it spells its own encoding.
var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// hiddenField returns the first field reachable from t that the
// JSON encoding cannot see, spelled type-qualified, and false where
// every field is visible. seen stops a recursive type.
func hiddenField(t reflect.Type, seen map[reflect.Type]bool) (string, bool) {
	for {
		if marshals(t) {
			return "", false
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			t = t.Elem()
			continue
		}
		break
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return "", false
	}
	seen[t] = true
	for f := range t.Fields() {
		switch {
		case f.Tag.Get("json") == "-":
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

// marshals reports whether values of t, or pointers to them, spell
// their own encoding.
func marshals(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(jsonMarshaler) || t.Implements(textMarshaler) ||
		pt.Implements(jsonMarshaler) || pt.Implements(textMarshaler)
}
