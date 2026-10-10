// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"fmt"

	"go.yaml.in/yaml/v3"

	eidos "go.dokimi.dev/eidos/core"
	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/jsonschema"
	"go.dokimi.dev/eidos/core/symbol"
)

// ruleDecl is one rule of the manifest. The loader lowers it to one rule of
// the authoring surface, which emits one declaration into the rule's
// family for each subject that it matches.
type ruleDecl struct {
	Directive directive.Name `yaml:"directive" doc:"The name of the directive of the manifest that gates the rule. A rule without a directive runs for every subject of its kind in the sources of the plan."`
	Output    string         `yaml:"output"    doc:"The tag of the family that receives the declarations of the rule. The default is the empty tag of the primary family."`
	Template  string         `yaml:"template"  doc:"The path of the template file inside each template tree, with slashes. It renders the body of each function and method that the rule emits."`
	On        subjectKind    `yaml:"on"        doc:"The kind of the subjects of the rule: function, interface or struct."                                                                                                        schema:"required"`
	Emit      emission       `yaml:"emit"      doc:"What the rule emits for each subject: mirror, or implementation on an interface. The template of the rule renders the bodies of a function mirror and of an implementation." schema:"required"`

	// line is the line of the rule's mapping.
	line int
}

// lower returns the rule of the authoring surface that runs l for each
// subject of the rule's kind. The rule passed checkRules, so its pair of
// kind and emission is one that the loader emits.
func (r ruleDecl) lower(l *lowered) eidos.Rule {
	switch symbol.Kind(r.On) {
	case symbol.KindFunction:
		return eidos.OnFunction(l.function)
	case symbol.KindStruct:
		return eidos.OnStruct(l.structMirror)
	default:
		if r.Emit == emitImplementation {
			return eidos.OnInterface(l.implementation)
		}
		return eidos.OnInterface(l.interfaceMirror)
	}
}

// subjectKind is the kind of the subjects of a rule, as a manifest writes
// it.
type subjectKind symbol.Kind

// subjectKinds are the spellings of the kind of the subjects of a rule.
var subjectKinds = []word[symbol.Kind]{
	{spelling: "function", value: symbol.KindFunction},
	{spelling: "interface", value: symbol.KindInterface},
	{spelling: "struct", value: symbol.KindStruct},
}

var (
	_ yaml.Unmarshaler   = (*subjectKind)(nil)
	_ jsonschema.Schemer = subjectKind(0)
)

// UnmarshalYAML decodes the kind of the subjects of a rule from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not a subject kind.
func (k *subjectKind) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, subjectKinds, "a subject kind")
	*k = subjectKind(v)
	return err
}

// JSONSchema returns the JSON Schema of the kind of the subjects of a rule,
// a new map on each call.
func (subjectKind) JSONSchema() map[string]any { return wordsSchema(subjectKinds) }

// emission is what a rule emits for each subject.
type emission uint8

const (
	// emitMirror is a declaration of the subject's kind with the subject's
	// signature or members.
	emitMirror emission = 1
	// emitImplementation is a struct that implements an interface by
	// calling a value of the interface.
	emitImplementation emission = 2
)

// emissions are the spellings of what a rule emits.
var emissions = []word[emission]{
	{spelling: "mirror", value: emitMirror},
	{spelling: "implementation", value: emitImplementation},
}

var (
	_ yaml.Unmarshaler   = (*emission)(nil)
	_ jsonschema.Schemer = emission(0)
)

// UnmarshalYAML decodes what a rule emits from n.
//
// Error modes: UnmarshalYAML returns a *yaml.TypeError at the line of n for
// a spelling that is not an emission.
func (e *emission) UnmarshalYAML(n *yaml.Node) error {
	v, err := decodeWord(n, emissions, "an emission")
	*e = v
	return err
}

// JSONSchema returns the JSON Schema of what a rule emits, a new map on
// each call.
func (emission) JSONSchema() map[string]any { return wordsSchema(emissions) }

// shape reports whether a rule on subjects of kind k emits e at all, and
// whether the declaration that it emits has bodies for the rule's template
// to render. A mirror of a function has a body, and so has each method of
// an implementation. A mirror of an interface or a struct has none.
func shape(k symbol.Kind, e emission) (emits, bodies bool) {
	switch {
	case k == symbol.KindFunction && e == emitMirror:
		return true, true
	case k == symbol.KindInterface && e == emitImplementation:
		return true, true
	case k == symbol.KindInterface && e == emitMirror, k == symbol.KindStruct && e == emitMirror:
		return true, false
	default:
		return false, false
	}
}

// kindFamily is the kind of a rule's subjects and the tag of its family,
// which no two rules share, because both would emit one name for each
// subject.
type kindFamily struct {
	kind symbol.Kind
	tag  string
}

// checkRules returns an Error at the line of each rule without a kind or an
// emission, with a pair of the two that the loader does not emit, with a
// directive or a family that the manifest does not declare, or with the
// kind and the family of an earlier rule. It also returns one for a
// template on a rule whose declaration has no body, for no template on a
// rule whose declaration has bodies in a family that claims no file, and
// for a template that a tree of tpl does not contain.
func checkRules(file string, rules []ruleDecl, ds []directiveDecl, outputs []outputDecl, tpl templates) []*Error {
	declared := make(map[directive.Name]bool, len(ds))
	for _, d := range ds {
		declared[d.Name] = true
	}
	tagged := make(map[string]bool, len(outputs))
	claimed := make(map[string]bool, len(outputs))
	for _, o := range outputs {
		tagged[o.Tag] = true
		claimed[o.Tag] = o.Template != ""
	}
	var out []*Error
	seen := make(map[kindFamily]int, len(rules))
	for _, r := range rules {
		fault := func(format string, args ...any) {
			out = append(out, &Error{File: file, Line: r.line, Msg: fmt.Sprintf(format, args...)})
		}
		k := symbol.Kind(r.On)
		if k == symbol.KindInvalid {
			fault("the rule has no on: use %s", spellings(subjectKinds))
		}
		if r.Emit == 0 {
			fault("the rule has no emit: use %s", spellings(emissions))
		}
		emits, bodies := shape(k, r.Emit)
		if !emits && k != symbol.KindInvalid && r.Emit != 0 {
			fault("a rule on %s cannot emit %s", spellingOf(subjectKinds, k), spellingOf(emissions, r.Emit))
		}
		if r.Directive != "" && !declared[r.Directive] {
			fault("the rule has the directive %s, and the manifest does not declare it", r.Directive)
		}
		if !tagged[r.Output] {
			fault("the rule emits into the family %q, which the manifest does not declare", r.Output)
		}
		if at, taken := seen[kindFamily{kind: k, tag: r.Output}]; taken && k != symbol.KindInvalid {
			fault("the rule emits into the family %q for each %s, as the rule at line %d does",
				r.Output, spellingOf(subjectKinds, k), at)
		} else {
			seen[kindFamily{kind: k, tag: r.Output}] = r.line
		}
		if emits {
			out = append(out, checkTemplate(file, r, bodies, claimed[r.Output], tpl)...)
		}
	}
	return out
}

// checkTemplate returns an Error at the line of the rule r for a template
// on a rule whose declaration has no body, and for no template on a rule
// whose declaration has bodies. A rule that emits into a family that
// claims its files needs no template, because the file template writes
// each record whole. checkTemplate also returns the faults of the path
// of the template that [templates.check] finds.
func checkTemplate(file string, r ruleDecl, bodies, claimed bool, tpl templates) []*Error {
	if !bodies && r.Template != "" {
		return []*Error{{
			File: file,
			Line: r.line,
			Msg: fmt.Sprintf("the rule has a template, and a %s of a %s has no body",
				spellingOf(emissions, r.Emit), spellingOf(subjectKinds, symbol.Kind(r.On))),
		}}
	}
	if bodies && r.Template == "" && !claimed {
		return []*Error{{File: file, Line: r.line, Msg: "the rule emits bodies, and it has no template to render them"}}
	}
	return tpl.check(file, r.line, "the rule's template", r.Template)
}
