// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

// Package defaults stamps the declared default value of each field.
//
// defaults is an annotator with two rules. The directive rule reads
// //+reference:default <value> on a field, in every language. The bare rule
// reads the tag of the key that the option tag sets, default unless a
// config sets another key, in each language whose rules implement
// rules.TagRules:
//
//	type Address struct {
//		//+reference:default "Dam 1"
//		Street string `default:"Main 1"`
//		Country string `default:"NL"`
//	}
//
// Within one plugin, the first claim in rule order takes precedence, and
// the directive rule comes first. So on a field with both forms, the value
// of the directive becomes the fact: Street defaults to Dam 1, and Country
// to NL. A tag with an empty value declares no default.
//
// # Facts
//
// defaults stamps the text of a default under [KeyValue], as the kernel's
// sample annotator stamps the text of a sample. A reader turns the text into
// a value through the rules of the language of the field. [KeySource]
// contains the form that declares the value, [SourceDirective] or
// [SourceTag]. defaults provides the capability [Capability], so a plugin
// that reads the facts requires it and runs after it.
//
// # Dependency position
//
// The package imports the root, directive, meta, plugin, rules and symbol
// packages of the SDK facade, and the Go stdlib.
package defaults
