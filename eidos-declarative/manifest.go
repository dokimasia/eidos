// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"bytes"
	"cmp"
	"fmt"

	"go.yaml.in/yaml/v3"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/jsonschema"
	"go.dokimi.dev/eidos/core/plugin"
)

// The keys of the manifest whose YAML nodes the decoder reads to position
// a fault: the format, the keys that a check of the manifest as a whole
// reports at, and the lists whose elements record their lines.
const (
	formatKey     = "format"
	contractKey   = "contract"
	nameKey       = "name"
	versionKey    = "version"
	providesKey   = "provides"
	requiresKey   = "requires"
	directivesKey = "directives"
	paramsKey     = "params"
	outputsKey    = "outputs"
	optionsKey    = "options"
	rulesKey      = "rules"
)

// format is the format of a manifest.
type format int

// currentFormat is the format of the manifest that this package reads.
const currentFormat format = 1

// firstContract is the contract version of a manifest that states none,
// the first version of the contract.
const firstContract = 1

var _ jsonschema.Schemer = currentFormat

// JSONSchema returns the JSON Schema of the format key, a new map on each
// call. The schema allows only [currentFormat].
func (format) JSONSchema() map[string]any {
	return map[string]any{keywordType: typeInteger, keywordConst: int(currentFormat)}
}

// manifest is the decoded plugin.yaml of a plugin directory.
type manifest struct {
	Format     format              `yaml:"format"     schema:"required" doc:"The format of the manifest. It must be 1."`
	Contract   int                 `yaml:"contract"                     doc:"The version of the contract between the kernel and its components that the plugin is written against. The loader refuses a manifest of another version. The default is 1, the first version."`
	Name       string              `yaml:"name"       schema:"required" doc:"The name of the plugin. A config file sets the options of the plugin under this name. A source file writes it before the name of a directive that two plugins declare, as in +acme:acme-audit:audit. A name is a letter followed by letters, digits, hyphens and underscores."`
	Version    string              `yaml:"version"    schema:"required" doc:"The version of the plugin. The loader appends a plus sign and the SHA-256 of the manifest and the template files. An edit of either makes the next run a cold run."`
	Priority   int                 `yaml:"priority"                     doc:"The priority of the plugin among the generators of a plan. A generator with a lower priority runs first. The default is 0."`
	Provides   []plugin.Capability `yaml:"provides"                     doc:"The capability labels that the plugin provides to the generators that require them."`
	Requires   []plugin.Capability `yaml:"requires"                     doc:"The capability labels of the generators that run before the plugin."`
	Directives []directiveDecl     `yaml:"directives"                   doc:"The directives of the plugin. A rule with a directive runs only for the subjects that have an instance of the directive."`
	Outputs    []outputDecl        `yaml:"outputs"    schema:"required" doc:"The output families of the plugin. Each rule emits into one family."`
	Options    []optionDecl        `yaml:"options"                      doc:"The options of the plugin. A config file sets them under options and the name of the plugin."`
	Rules      []ruleDecl          `yaml:"rules"      schema:"required" doc:"The rules of the plugin. A rule emits one declaration for each subject that it matches."`

	// line is the line of the manifest's mapping, which a fault of a
	// missing key is at.
	line int
	// contractLine, nameLine, versionLine, providesLine and requiresLine
	// are the lines of the values of their keys, and 0 for a key that the
	// manifest does not write.
	contractLine int
	nameLine     int
	versionLine  int
	providesLine int
	requiresLine int
}

// decode decodes the manifest data of the file at the path file. It reads
// the format first, so a manifest of another format reports its format and
// not the keys that this binary does not know. It then decodes the
// manifest with known fields, and records the line of each element that a
// check reports at.
//
// Error modes: decode returns one Error for a manifest that is not a
// mapping, one for a manifest without a format or with a format other than
// 1, and one for each fault that the YAML decoder reports, such as a
// syntax error, an unknown key, a key written twice and a value of the
// wrong type.
func decode(file string, data []byte) (*manifest, []*Error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, faults(file, err)
	}
	noFormat := fmt.Sprintf("the manifest has no format key: add format: %d", currentFormat)
	if len(doc.Content) == 0 {
		return nil, []*Error{{File: file, Line: 1, Msg: noFormat}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, []*Error{{File: file, Line: root.Line, Msg: "the manifest is not a mapping of keys to values"}}
	}
	given := value(root, formatKey)
	if given == nil {
		return nil, []*Error{{File: file, Line: root.Line, Msg: noFormat}}
	}
	var f format
	if err := given.Decode(&f); err != nil || f != currentFormat {
		return nil, []*Error{
			{
				File: file,
				Line: given.Line,
				Msg: fmt.Sprintf(
					"the manifest has format %s, and this binary reads only format %d",
					given.Value,
					currentFormat,
				),
			},
		}
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m manifest
	if err := dec.Decode(&m); err != nil {
		return nil, faults(file, err)
	}
	m.position(root)
	return &m, nil
}

// position records the lines that the checks report at, from the mapping
// root that the manifest decoded from. The decode succeeded, so each list
// of root has one element for each element of its field.
func (m *manifest) position(root *yaml.Node) {
	m.line = root.Line
	if n := value(root, contractKey); n != nil {
		m.contractLine = n.Line
	}
	if n := value(root, nameKey); n != nil {
		m.nameLine = n.Line
	}
	if n := value(root, versionKey); n != nil {
		m.versionLine = n.Line
	}
	if n := value(root, providesKey); n != nil {
		m.providesLine = n.Line
	}
	if n := value(root, requiresKey); n != nil {
		m.requiresLine = n.Line
	}
	for i, n := range elements(value(root, directivesKey), len(m.Directives)) {
		m.Directives[i].position(n)
	}
	for i, n := range elements(value(root, outputsKey), len(m.Outputs)) {
		m.Outputs[i].line = n.Line
	}
	for i, n := range elements(value(root, optionsKey), len(m.Options)) {
		m.Options[i].line = n.Line
	}
	for i, n := range elements(value(root, rulesKey), len(m.Rules)) {
		m.Rules[i].line = n.Line
	}
}

// check returns an Error for each fault of the decoded manifest in the file
// at the path file, in the order of the keys of the manifest. tpl contains
// the template trees that the rules render through.
func (m *manifest) check(file string, tpl templates) []*Error {
	var out []*Error
	if contract := cmp.Or(m.Contract, firstContract); contract != plugin.ContractVersion {
		out = append(out, &Error{
			File: file,
			Line: cmp.Or(m.contractLine, m.line),
			Msg: fmt.Sprintf(
				"the plugin %s is written against contract version %d, and the kernel has contract version %d",
				m.Name, contract, plugin.ContractVersion,
			),
		})
	}
	// A name is an identifier of the directive grammar, because a source
	// file writes it before the name of a directive that two plugins
	// declare. A param key follows the same rule, so the check of a param
	// key decides.
	if m.Name == "" {
		out = append(out, &Error{File: file, Line: m.line, Msg: "the manifest has no name"})
	} else if !directive.ParamKey(m.Name).Valid() {
		out = append(out, &Error{
			File: file,
			Line: m.nameLine,
			Msg: fmt.Sprintf(
				"%q is not a name: use a letter followed by letters, digits, hyphens and underscores",
				m.Name,
			),
		})
	}
	if m.Version == "" {
		out = append(out, &Error{File: file, Line: cmp.Or(m.versionLine, m.line), Msg: "the manifest has no version"})
	}
	out = append(out, labels(file, m.providesLine, providesKey, m.Provides)...)
	out = append(out, labels(file, m.requiresLine, requiresKey, m.Requires)...)
	out = append(out, checkDirectives(file, m.Name, m.Directives, m.Rules)...)
	if len(m.Outputs) == 0 {
		out = append(out, &Error{File: file, Line: m.line, Msg: "the manifest declares no output family"})
	}
	out = append(out, checkOutputs(file, m.Outputs, tpl)...)
	out = append(out, checkOptions(file, m.Options)...)
	if len(m.Rules) == 0 {
		out = append(out, &Error{File: file, Line: m.line, Msg: "the manifest declares no rule"})
	}
	return append(out, checkRules(file, m.Rules, m.Directives, m.Outputs, tpl)...)
}

// labels returns an Error at line for each empty label of the capability
// list of key.
func labels(file string, line int, key string, list []plugin.Capability) []*Error {
	var out []*Error
	for _, c := range list {
		if c == "" {
			out = append(out, &Error{File: file, Line: line, Msg: "the list " + key + " has an empty capability label"})
		}
	}
	return out
}

// value returns the value of key in the mapping m. For an alias, it reads
// the mapping of the anchor. It returns nil when m does not have the key.
// The decode of the manifest succeeded, so m is a mapping or an alias of
// one.
func value(m *yaml.Node, key string) *yaml.Node {
	if m.Kind == yaml.AliasNode {
		m = m.Alias
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// elements returns the first n elements of the sequence seq. For an alias,
// it reads the sequence of the anchor. It returns nil for a nil node and for
// a node that is not a sequence.
func elements(seq *yaml.Node, n int) []*yaml.Node {
	if seq != nil && seq.Kind == yaml.AliasNode {
		seq = seq.Alias
	}
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	return seq.Content[:min(n, len(seq.Content))]
}
