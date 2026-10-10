// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package matrix

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The hub's section, and the header of its column of labels.
const (
	hubSection = "\n## Hub\n\nEach cell contains the spelling of one canonical form in a target, under " +
		"the target's default policy. A row of a policy's choice contains the spelling under that choice " +
		"in the column of the policy's target.\n\n"
	formHeader = "Form or choice"
)

// These constants build the cells of the hub. refusedCell is the cell of
// a form that the spoke refuses. spellref.Spell writes standIn for a
// reference without a spelling. choiceSep separates the key of a policy
// from one of its choices in a label.
const (
	refusedCell = "refused"
	standIn     = "?"
	choiceSep   = ": "
)

// probeLang is the language of the probes' references. A spoke spells a
// reference to a declaration of another language as a translated
// reference, whatever the language is.
const probeLang symbol.Lang = "probe"

// The package and the names of the declarations that the probes
// reference.
const (
	probePkg    = "svc"
	sessionName = "Session"
	resultName  = "Result"
)

// The source spellings of the probes' shapes. A spoke spells a shape by
// its form, so a source spelling appears only in a refusal.
const (
	textSource = "text"
	boolSource = "bool"
	intSource  = "int"
	uintSource = "uint"
	realSource = "real"
)

// The widths in bits of the probes' scalars, the length of the probe's
// array, and the number of parameters of the probe's function. Width 0 is
// the platform's width.
const (
	narrowBits    = 32
	wideBits      = 64
	platformBits  = 0
	arrayLength   = 2
	funcParamsLen = 1
)

// probe is one row of the hub. It contains a label and a canonical
// shape, and each spoke spells the shape in the row.
type probe struct {
	label string
	shape rules.TypeShape
}

// The shapes that the probes compose, and the declarations of their
// references.
var (
	session    = symbol.Identity{Lang: probeLang, Package: probePkg, Name: sessionName, Kind: symbol.KindStruct}
	result     = symbol.Identity{Lang: probeLang, Package: probePkg, Name: resultName, Kind: symbol.KindSum}
	text       = rules.Leaf(symbol.FormText, textSource)
	boolean    = rules.Leaf(symbol.FormBool, boolSource)
	wideInt    = rules.Scalar(intSource, rules.ScalarInt, wideBits)
	optional   = rules.TypeShape{Form: symbol.FormOptional, Spelling: textSource, Elems: []rules.TypeShape{text}}
	octets     = rules.Leaf(symbol.FormBytes, textSource)
	timestamp  = rules.Reference(textSource, rules.WellKnownTimestamp)
	duration   = rules.Reference(textSource, rules.WellKnownDuration)
	sessionRef = rules.Reference(sessionName, session)
)

// probes are the rows of the hub, one or more for each form of a
// canonical shape. The rows run from the scalars and the other leaves
// through the structural forms to the references, and Opaque is the last
// row. The kernel's fold does not return the form Named, so the hub does
// not have a row of it.
var probes = []probe{
	{label: "Scalar int, 32 bits", shape: rules.Scalar(intSource, rules.ScalarInt, narrowBits)},
	{label: "Scalar int, 64 bits", shape: wideInt},
	{label: "Scalar int, platform width", shape: rules.Scalar(intSource, rules.ScalarInt, platformBits)},
	{label: "Scalar uint, 32 bits", shape: rules.Scalar(uintSource, rules.ScalarUint, narrowBits)},
	{label: "Scalar uint, 64 bits", shape: rules.Scalar(uintSource, rules.ScalarUint, wideBits)},
	{label: "Scalar uint, platform width", shape: rules.Scalar(uintSource, rules.ScalarUint, platformBits)},
	{label: "Scalar float, 64 bits", shape: rules.Scalar(realSource, rules.ScalarFloat, wideBits)},
	{label: "Bool", shape: boolean},
	{label: "Text", shape: text},
	{label: "Bytes", shape: octets},
	{label: "Dynamic", shape: rules.Leaf(symbol.FormDynamic, textSource)},
	{label: "Optional of text", shape: optional},
	{label: "List of text", shape: rules.TypeShape{Form: symbol.FormList, Elems: []rules.TypeShape{text}}},
	{
		label: "Array of two texts",
		shape: rules.TypeShape{Form: symbol.FormArray, Length: arrayLength, Elems: []rules.TypeShape{text}},
	},
	{
		label: "Array of an unstated length",
		shape: rules.TypeShape{Form: symbol.FormArray, Elems: []rules.TypeShape{text}},
	},
	{label: "Map of text to text", shape: rules.TypeShape{Form: symbol.FormMap, Elems: []rules.TypeShape{text, text}}},
	{
		label: "Function of text to text",
		shape: rules.TypeShape{Form: symbol.FormFunc, Split: funcParamsLen, Elems: []rules.TypeShape{text, text}},
	},
	{
		label: "Tuple of text and a boolean",
		shape: rules.TypeShape{Form: symbol.FormTuple, Elems: []rules.TypeShape{text, boolean}},
	},
	{
		label: "Union of text and a boolean",
		shape: rules.TypeShape{Form: symbol.FormUnion, Elems: []rules.TypeShape{text, boolean}},
	},
	{
		label: "Intersection of two references",
		shape: rules.TypeShape{Form: symbol.FormIntersection, Elems: []rules.TypeShape{sessionRef, sessionRef}},
	},
	{
		label: "Synchronous stream of text",
		shape: rules.TypeShape{Form: symbol.FormStream, Elems: []rules.TypeShape{text}},
	},
	{
		label: "Asynchronous stream of text",
		shape: rules.TypeShape{Form: symbol.FormStream, Async: true, Elems: []rules.TypeShape{text}},
	},
	{label: "Borrow of text", shape: rules.TypeShape{Form: symbol.FormBorrow, Elems: []rules.TypeShape{text}}},
	{
		label: "Wildcard with an upper bound",
		shape: rules.TypeShape{Form: symbol.FormWildcard, Variance: symbol.VarianceOut, Elems: []rules.TypeShape{text}},
	},
	{
		label: "Wildcard with a lower bound",
		shape: rules.TypeShape{Form: symbol.FormWildcard, Variance: symbol.VarianceIn, Elems: []rules.TypeShape{text}},
	},
	{label: "Wildcard without a bound", shape: rules.TypeShape{Form: symbol.FormWildcard}},
	{label: "Inline type", shape: rules.TypeShape{Form: symbol.FormInline, Spelling: textSource}},
	{label: "Reference", shape: sessionRef},
	{label: "Reference with a type argument", shape: rules.Reference(sessionName, session, text)},
	{label: "Sum", shape: rules.TypeShape{Form: symbol.FormSum, Spelling: resultName, Ref: result}},
	{label: "Timestamp", shape: timestamp},
	{label: "Duration", shape: duration},
	{label: "Empty", shape: rules.Reference(textSource, rules.WellKnownEmpty)},
	{label: "Opaque", shape: rules.Opaque(nil)},
}

// policyProbes maps each lowering policy of a target to the shape whose
// spelling the policy decides. The hub's section contains the spelling
// of the shape under each choice of the policy.
var policyProbes = map[plugin.PolicyKey]rules.TypeShape{
	typescript.Int64:     wideInt,
	typescript.Absent:    optional,
	typescript.Timestamp: timestamp,
	typescript.Bytes:     octets,
	typescript.Duration:  duration,
}

// hub writes the hub's section. A row of a probe contains the spelling of
// the probe in each target, under the target's default policy. A row of a
// policy's choice contains the spelling of the policy's probe under that
// choice in the column of the policy's target, and none in every other
// column.
//
// Error modes: the error of a backend whose policies do not resolve, and
// an error for a policy without a probe.
func hub(b *strings.Builder, entries []Entry) error {
	backed := withBackends(entries)
	specs := make([][]plugin.PolicySpec, len(backed))
	defaults := make([]plugin.Policy, len(backed))
	for i, e := range backed {
		if provider, declares := e.Backend.(plugin.PolicyProvider); declares {
			specs[i] = provider.Policies()
		}
		p, err := plugin.NewPolicy(e.Backend.Target(), specs[i], nil)
		if err != nil {
			return err
		}
		defaults[i] = p
	}
	rows := make([][]string, 0, len(probes))
	for _, pr := range probes {
		row := []string{pr.label}
		for i, e := range backed {
			row = append(row, spelled(e, pr.shape, defaults[i]))
		}
		rows = append(rows, row)
	}
	for i, e := range backed {
		for _, spec := range specs[i] {
			shape, probed := policyProbes[spec.Key]
			if !probed {
				return fmt.Errorf("matrix: the policy %s of target %s has no probe", spec.Key, e.Backend.Target())
			}
			for _, c := range spec.Choices {
				p := defaults[i].Overridden(func(k plugin.PolicyKey) (plugin.Choice, bool) {
					return c, k == spec.Key
				})
				row := []string{string(spec.Key) + choiceSep + string(c)}
				for j := range backed {
					cell := noneCell
					if j == i {
						cell = spelled(e, shape, p)
					}
					row = append(row, cell)
				}
				rows = append(rows, row)
			}
		}
	}
	b.WriteString(hubSection)
	table(b, append([]string{formHeader}, targetsOf(backed)...), rows)
	return nil
}

// spelled returns the cell of one shape in an entry's target under p: the
// spoke's spelling as code, refused where the spoke refuses the shape,
// and none where the backend has no spoke.
func spelled(e Entry, s rules.TypeShape, p plugin.Policy) string {
	spoke, spells := e.Backend.(plugin.TypeSpeller)
	if !spells {
		return noneCell
	}
	t, err := spoke.SpellType(s, p)
	if err != nil {
		return refusedCell
	}
	return codeOpen + spellref.Spell(t, e.ArgsOpen, e.ArgsClose, standIn) + codeClose
}
