// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package specfront_test

import (
	"testing"

	"go.dokimi.dev/assert"
	"go.dokimi.dev/assert/expect"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
)

// The paths of the specs whose place is at fault.
const (
	strayPath    = "spec/other/probe.yaml"
	topPath      = "spec/probe.yaml"
	detectedPath = "spec/shapes/detected.yaml"
)

// fault is the line and the message of one finding of a parse.
type fault struct {
	line int
	msg  string
}

// Each fault of a spec is an Error under SpecInvalid at the key or the
// value at fault, and a spec with a fault declares nothing.
func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			path string
			give string
			want []fault
		}{
			{
				name: "reports a syntax error at its line",
				path: shapePath,
				give: "name: [\n",
				want: []fault{{1, "did not find expected node content"}},
			},
			{
				name: "reports a file that is not a mapping",
				path: shapePath,
				give: "- probe\n",
				want: []fault{{1, "the file is not a mapping of sections to values"}},
			},
			{
				name: "reports an empty file",
				path: shapePath,
				want: []fault{{0, "the file is not a mapping of sections to values"}},
			},
			{
				name: "reports a spec outside the directory of a form",
				path: strayPath,
				give: shapeBase,
				want: []fault{
					{
						1,
						"spec/other is no directory of a form: move the spec to spec/shapes, spec/mixins or spec/contracts",
					},
				},
			},
			{
				name: "reports a spec directly below spec",
				path: topPath,
				give: shapeBase,
				want: []fault{
					{1, "spec is no directory of a form: move the spec to spec/shapes, spec/mixins or spec/contracts"},
				},
			},
			{
				name: "reports an unknown section of a shape at its line",
				path: shapePath,
				give: shapeBase + "colour: red\n",
				want: []fault{{8, "field colour not found in type specfront.Shape"}},
			},
			{
				name: "reports an unknown section of a mixin at its line",
				path: mixinPath,
				give: mixinBase + "colour: red\n",
				want: []fault{{8, "field colour not found in type specfront.Mixin"}},
			},
			{
				name: "reports the precedence of a mixin at its line",
				path: mixinPath,
				give: mixinBase + "precedence:\n  yields_to: [writer]\n",
				want: []fault{{8, "field precedence not found in type specfront.Mixin"}},
			},
			{
				name: "reports an unknown section of a contract at its line",
				path: contractPath,
				give: contractBase + "colour: red\n",
				want: []fault{{11, "field colour not found in type specfront.Contract"}},
			},
			{
				name: "reports each entry of a decoder error",
				path: shapePath,
				give: shapeBase + "params:\n" +
					"  - {key: a, type: float, doc: a param}\n" +
					"  - {key: b, type: char, doc: a param}\n",
				want: []fault{
					{9, `"float" is not a param type: write "string" or "int" or "reference"`},
					{10, `"char" is not a param type: write "string" or "int" or "reference"`},
				},
			},
			{
				name: "reports a section of another kind than its type",
				path: shapePath,
				give: "name: probe\nform: shape\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples: none\n",
				want: []fault{{6, "cannot unmarshal !!str `none` into specfront.Counterexamples"}},
			},
			{
				name: "reports a missing section at the mapping",
				path: shapePath,
				give: "name: probe\nform: shape\nobservation: an observation\nfalsifiability: a falsifiability\n" +
					"counterexamples:\n  edge: an edge\n",
				want: []fault{{1, "the mapping has no claim, which is required"}},
			},
			{
				name: "reports a spec without its falsifiability at the mapping",
				path: shapePath,
				give: "name: probe\nform: shape\nclaim: a claim\nobservation: an observation\n" +
					"counterexamples:\n  edge: an edge\n",
				want: []fault{{1, "the mapping has no falsifiability, which is required"}},
			},
			{
				name: "reports a blank text at its value",
				path: shapePath,
				give: "name: probe\nform: shape\nclaim: ' '\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\n",
				want: []fault{{3, "the claim is blank: write its text"}},
			},
			{
				name: "reports a missing section of a param",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: sample, type: string}\n",
				want: []fault{{9, "the mapping has no doc, which is required"}},
			},
			{
				name: "reports a missing section of a binding",
				path: shapePath,
				give: shapeBase + "bindings:\n  value: {index: 0, doc: a binding}\n",
				want: []fault{{9, "the mapping has no from, which is required"}},
			},
			{
				name: "reports a missing section of the precedence",
				path: shapePath,
				give: shapeBase + "detected: true\nprecedence: {}\n",
				want: []fault{{9, "the mapping has no yields_to, which is required"}},
			},
			{
				name: "reports a name outside the pattern",
				path: shapePath,
				give: "name: Probe\nform: shape\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\n",
				want: []fault{
					{1, `"Probe" is no name: write lowercase words that open with a letter, joined by hyphens`},
				},
			},
			{
				name: "reports the name of a binding outside the pattern",
				path: shapePath,
				give: shapeBase + "bindings:\n  Value: {from: input, doc: a binding}\n",
				want: []fault{
					{9, `"Value" is no name: write lowercase words that open with a letter, joined by hyphens`},
				},
			},
			{
				name: "reports the name of a role outside the pattern",
				path: contractPath,
				give: "name: probe\nform: contract\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\nroles:\n  Begin: {arity: one}\n",
				want: []fault{
					{9, `"Begin" is no name: write lowercase words that open with a letter, joined by hyphens`},
				},
			},
			{
				name: "reports a name that is the part of a summary key",
				path: detectedPath,
				give: "name: detected\nform: shape\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\n",
				want: []fault{
					{1, "detected is the part of a summary key, such as shape.detected, and no spec can have it"},
				},
			},
			{
				name: "reports a file of another name than its spec",
				path: otherPath,
				give: shapeBase,
				want: []fault{{1, "the spec probe is in other.yaml: name the file probe.yaml"}},
			},
			{
				name: "reports a binding at a negative index",
				path: shapePath,
				give: shapeBase + "bindings:\n  value: {from: input, index: -1, doc: a binding}\n",
				want: []fault{{9, "the binding value has the index -1, and a position counts from 0"}},
			},
			{
				name: "reports a contract without a role",
				path: contractPath,
				give: "name: probe\nform: contract\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\nroles: {}\n",
				want: []fault{{8, "the contract has no role: list each role of the protocol with its arity"}},
			},
			{
				name: "reports a role without its arity",
				path: contractPath,
				give: "name: probe\nform: contract\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples:\n  edge: an edge\nroles:\n  begin: {}\n",
				want: []fault{{9, "the mapping has no arity, which is required"}},
			},
			{
				name: "reports the precedence of a shape that is not detected",
				path: shapePath,
				give: shapeBase + "precedence:\n  yields_to: [writer]\n",
				want: []fault{{8, "the shape probe is not detected, so no detector ranks it: remove its precedence"}},
			},
			{
				name: "reports a spec without a counterexample",
				path: shapePath,
				give: "name: probe\nform: shape\nclaim: a claim\nobservation: an observation\n" +
					"falsifiability: a falsifiability\ncounterexamples: {}\n",
				want: []fault{
					{6, "the spec has no counterexample: write at least one of invalid, unsafe, edge and refused"},
				},
			},
			{
				name: "reports a reserved key",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: role, type: string, doc: a param}\n",
				want: []fault{{9, "role is reserved: the directive layer or the keys of the catalog use it"}},
			},
			{
				name: "reports a key that a param and a binding share",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: value, type: string, doc: a param}\n" +
					"bindings:\n  value: {from: input, doc: a binding}\n",
				want: []fault{{11, "value is a key of the spec already, at line 9"}},
			},
			{
				name: "reports a reference without a resolution kind",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: read, type: reference, doc: a param}\n",
				want: []fault{
					{9, "the reference read has no resolve: write the kind of declaration that it refers to"},
				},
			},
			{
				name: "reports a resolution kind on a param that is no reference",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: mode, type: string, resolve: package-var, doc: a param}\n",
				want: []fault{{9, "the string param mode has resolve package-var, and only a reference resolves"}},
			},
			{
				name: "reports the roles of a param of a mixin",
				path: mixinPath,
				give: mixinBase + "params:\n  - {key: mode, type: string, roles: [begin], doc: a param}\n",
				want: []fault{{9, "the param mode has roles, and only a contract has roles"}},
			},
			{
				name: "reports a role that the contract does not declare",
				path: contractPath,
				give: contractBase + "params:\n  - {key: mode, type: string, roles: [ghost], doc: a param}\n",
				want: []fault{{12, "the param mode applies under the role ghost, and the contract has no such role"}},
			},
			{
				name: "reports a minimum on a param that is no int",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: mode, type: string, minimum: 1, doc: a param}\n",
				want: []fault{{9, "the string param mode has a minimum, and only an int has one"}},
			},
			{
				name: "reports an exclusion of a param that the spec does not have",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: mode, type: string, excludes: [ghost], doc: a param}\n",
				want: []fault{{9, "the param mode excludes ghost, which is no other param of the spec"}},
			},
			{
				name: "reports an exclusion of the param itself",
				path: shapePath,
				give: shapeBase + "params:\n  - {key: mode, type: string, excludes: [mode], doc: a param}\n",
				want: []fault{{9, "the param mode excludes mode, which is no other param of the spec"}},
			},
			{
				name: "reports also_on on a param that is no host-param reference",
				path: shapePath,
				give: shapeBase + "params:\n" +
					"  - {key: read, type: reference, resolve: callable-in-scope, doc: a param}\n" +
					"  - {key: mode, type: string, also_on: [read], doc: a param}\n",
				want: []fault{
					{10, "the param mode has also_on, and only a host-param reference refers to a parameter"},
				},
			},
			{
				name: "reports also_on a param that is no callable reference",
				path: shapePath,
				give: shapeBase + "params:\n" +
					"  - {key: mode, type: string, doc: a param}\n" +
					"  - {key: axis, type: reference, resolve: host-param, also_on: [mode], doc: a param}\n",
				want: []fault{{10, "the param axis is also on mode, which is no callable param of the spec"}},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				got := parse(t, map[string]string{tt.path: tt.give})
				var faults []fault
				for _, d := range got.findings {
					expect.Equal(t, d.Code, specfront.SpecInvalid, "the finding reports an invalid spec")
					expect.Equal(t, d.Pos.File, tt.path, "the finding is in the spec file")
					faults = append(faults, fault{line: d.Pos.Line, msg: d.Msg})
				}
				assert.Equal(t, faults, tt.want, "the parse reports each fault at its line", assert.EquateEmpty())
				assert.Empty(t, got.structs(), "a spec with a fault declares nothing")
			})
		}

		t.Run("accepts a spec of each form", func(t *testing.T) {
			t.Parallel()

			got := parse(t, map[string]string{shapePath: shapeBase, mixinPath: mixinBase, contractPath: contractBase})
			expect.Empty(t, got.findings, "the three specs are valid")
			expect.Length(t, got.structs(), 3, "the unit declares the three specs")
		})
	})
}
