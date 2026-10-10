// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/jsonschema"
	"go.dokimi.dev/eidos/core/plugin"
)

// outputDecl is one output family of the manifest. The loader lowers it to
// a [plugin.Output].
type outputDecl struct {
	Tag      string      `yaml:"tag"      doc:"The tag of the family. A rule writes the tag under output to emit into the family. The default is the empty tag of the primary family. Each target joins a tag to the name of a file after the word, as in ledger_audit_test.go and ledger.audit.test.ts for the tag test."`
	Per      cardinality `yaml:"per"      doc:"The cardinality of the family: source for one file for each source file, package for one file for each package, or plan for one file for the plan."                                                                                                                         schema:"required"`
	Word     string      `yaml:"word"     doc:"The word of the family. Each target joins it to the name of a file, as in ledger_audit.go and ledger.audit.ts. The plugin joins it to the name of each declaration, as in LedgerAudit."                                                                                     schema:"required"`
	Template string      `yaml:"template" doc:"The path of the file template inside each template tree, with slashes. A family with a file template claims its files. The template writes each file from the records that the rules emit into the family, and the kind templates of the target write none of them."`

	// line is the line of the family's mapping.
	line int
}

// cardinality is the cardinality of a family, as a manifest writes it.
type cardinality plugin.Cardinality

// cardinalities are the spellings of the cardinality of a family.
var cardinalities = []word[plugin.Cardinality]{
	{spelling: "source", value: plugin.PerSource},
	{spelling: "package", value: plugin.PerPackage},
	{spelling: "plan", value: plugin.PerPlan},
}

var (
	_ yaml.Unmarshaler   = (*cardinality)(nil)
	_ jsonschema.Schemer = cardinality(0)
)

// UnmarshalYAML decodes the cardinality of a family from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not a cardinality.
func (c *cardinality) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, cardinalities, "a cardinality")
	*c = cardinality(v)
	return err
}

// JSONSchema returns the JSON Schema of the cardinality of a family, a new
// map on each call.
func (cardinality) JSONSchema() map[string]any { return wordsSchema(cardinalities) }

// checkOutputs returns an Error at the line of each family without a
// cardinality, without a word, or with the tag of an earlier family. The
// authoring surface panics on each of these declarations, so the loader
// refuses them before it builds the plugin. It also returns one for a
// file template that is no slash-separated path inside a tree, for a
// file template without a tree, and for each tree of tpl that does not
// contain the file template.
func checkOutputs(file string, outputs []outputDecl, tpl templates) []*Error {
	var out []*Error
	tagged := make(map[string]int, len(outputs))
	for _, o := range outputs {
		if o.Per == 0 {
			out = append(out, &Error{
				File: file, Line: o.line, Msg: "the family has no per: use " + spellings(cardinalities),
			})
		}
		if o.Word == "" {
			out = append(out, &Error{File: file, Line: o.line, Msg: "the family has no word"})
		}
		out = append(out, tpl.check(file, o.line, "the family's file template", o.Template)...)
		if at, taken := tagged[o.Tag]; taken {
			out = append(out, &Error{
				File: file,
				Line: o.line,
				Msg:  fmt.Sprintf("the family has the tag %q of the family at line %d", o.Tag, at),
			})
			continue
		}
		tagged[o.Tag] = o.line
	}
	return out
}
