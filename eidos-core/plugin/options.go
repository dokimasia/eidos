// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: MIT

package plugin

import (
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
