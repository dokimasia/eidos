// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package enum

import (
	"embed"
	"slices"
	"strconv"
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/reference/internal/tree"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/directive"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// ID is the name of the plugin.
const ID plugin.ID = "enum"

// Version is the version of the plugin. The fingerprint of a composition
// folds it in.
const Version = "0.1.0"

// EnumDirective is the directive that puts an enumeration under enum.
// MethodsParam is its param that withholds every method where it is false.
const (
	EnumDirective directive.Name     = "enum"
	MethodsParam  directive.ParamKey = "methods"
)

// Word is the word of the family of the enumerations and of the family of
// their tests. TestTag is the tag of the family of the tests.
const (
	Word    = "enum"
	TestTag = "test"
)

// The methods that enum writes on an enumeration, each withheld where the
// enumeration declares it.
const (
	stringMethod    = "String"
	validMethod     = "IsValid"
	marshalMethod   = "MarshalText"
	unmarshalMethod = "UnmarshalText"
)

// The words that the names of the parser, the list of the variants and the
// error of an unknown text join onto the name of the enumeration.
const (
	parseWord   = "Parse"
	valuesWord  = "Values"
	unknownWord = "ErrUnknown"
)

// The templates of each file of the two families, and the mark that fills
// the text that names no variant.
const (
	enumTemplate = "enum.tmpl"
	testTemplate = "enum.test.tmpl"
	unknownMark  = "?"
)

// schema is the directive of enum.
var schema = directive.Schema{
	Plugin: string(ID),
	Name:   EnumDirective,
	Params: []directive.ParamSpec{{
		Key: MethodsParam, Type: directive.TypeBool,
		Doc: "writes the methods String, IsValid, MarshalText and UnmarshalText where true, the default",
	}},
	Doc: "generates the text of each variant of the enumeration, its parser, and checks of both",
}

// templates contains the templates of enum in one directory for each
// target.
//
//go:embed templates
var templates embed.FS

// Variant is one variant in the data of the templates. Name is the name of
// its constant, and Text is its text as a Go string literal.
type Variant struct {
	Name string
	Text string
}

// Surface is the data that the templates read beside the record of an
// enumeration. Variants are the variants in the order of the declaration.
// Textual reports that the variants have text values, which String returns
// as they are. String, Valid, Marshal and Unmarshal report the methods that
// enum writes. enum writes the parser, the list of the variants and the
// error of an unknown text with String, under the names Parse, Values and
// Unknown. UnknownText is a Go string literal of a text that no variant has.
type Surface struct {
	Variants    []Variant
	Textual     bool
	String      bool
	Valid       bool
	Marshal     bool
	Unmarshal   bool
	Parse       string
	Values      string
	Unknown     string
	UnknownText string
}

// New returns enum. For each enumeration under [EnumDirective], it writes
// the methods String, IsValid, MarshalText and UnmarshalText on the type,
// and the function Parse<T>, the function <T>Values and the error
// ErrUnknown<T> beside them. The family [Word] claims these files through
// the template enum.tmpl, because the declarations are text of Go alone.
// The family [Word] under the tag [TestTag] claims a test file for each
// source file, which checks the variants and the declarations that enum
// writes.
//
// The text of a variant is its declared value where the value is text, and
// its name without the name of the type otherwise. enum withholds each
// method that the type declares already. It writes Parse<T>, <T>Values,
// ErrUnknown<T>, MarshalText and UnmarshalText only where it writes String,
// because they read the text of a variant. [MethodsParam] set to false
// withholds every method.
//
// enum reports [DuplicateText] at the second of two variants with one
// text, and writes nothing for the type. The plugin serves the target
// golang alone. Each call returns a new plugin, because a plugin instance
// belongs to one workspace.
func New() plugin.Generator {
	g, _ := sdk.NewPlugin(ID).
		Version(Version).
		Output(plugin.Output{Per: plugin.PerSource, Word: Word, Template: enumTemplate}).
		Output(plugin.Output{Tag: TestTag, Per: plugin.PerSource, Word: Word, Template: testTemplate}).
		For(golang.Target, sdk.Templates(tree.Of(templates, golang.Target))).
		Handle(sdk.Directive(schema, sdk.OnEnum(generate))).
		Build().(plugin.Generator)
	return g
}

// generate records the declarations of one enumeration into the family of
// the enumerations, and its checks into the family of the tests.
func generate(m *sdk.EnumMatch, e *sdk.Emitter) error {
	n := m.Enum
	data := Surface{
		Variants: make([]Variant, 0, len(n.Variants)),
		Parse:    e.JoinName(n.Name, parseWord),
		Values:   e.JoinName(valuesWord, n.Name),
		Unknown:  e.JoinName(n.Name, unknownWord),
	}
	texts := make([]string, 0, len(n.Variants))
	longest := 0
	for _, v := range n.Variants {
		text := strings.TrimPrefix(v.Name, n.Name)
		// The exact value of a constant of text is a quoted literal.
		// Unquote refuses the value of a number and an absent value.
		value, _ := sdk.FactOf(m, v.ID, meta.Named[string](golang.ConstValueKey))
		if unquoted, err := strconv.Unquote(value); err == nil {
			text, data.Textual = unquoted, true
		}
		if slices.Contains(texts, text) {
			m.ErrorfAt(DuplicateText, v.Pos, "%s has the text %q of an earlier variant of %s", v.Name, text, n.Name)
			return nil
		}
		texts = append(texts, text)
		longest = max(longest, len(text))
		data.Variants = append(data.Variants, Variant{Name: v.Name, Text: strconv.Quote(text)})
	}
	data.UnknownText = strconv.Quote(strings.Repeat(unknownMark, longest+1))

	methods := true
	if v, set := m.Directive().Params[MethodsParam]; set {
		methods = v.Bool
	}
	declared := make(map[string]bool, len(n.Methods))
	for _, method := range n.Methods {
		declared[method.Name] = true
	}
	data.String = methods && !declared[stringMethod]
	data.Valid = methods && !declared[validMethod]
	data.Marshal = data.String && !declared[marshalMethod]
	data.Unmarshal = data.String && !declared[unmarshalMethod]

	if data.String || data.Valid {
		e.File().Record(data, &emit.Enum{Origin: n.ID, Name: n.Name, Visibility: n.Visibility})
	}
	e.File(TestTag).Record(data, &emit.Enum{Origin: n.ID, Name: n.Name, Visibility: n.Visibility})
	return nil
}
