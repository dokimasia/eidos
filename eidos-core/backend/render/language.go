// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"text/template"

	"go.dokimi.dev/eidos/core/emit"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Naming spells a unit's filename for one target: the join and
// extension of the family's word, the tag's treatment, and the
// cardinality key's stem. It is total: every unit the plan admits
// returns a name, and units returning one name assemble one file,
// which is how two plugins share it.
type Naming func(u plugin.Unit) string

// Split reshapes one unit into the units the target files
// separately: a language that names a file after the type the file
// declares returns one unit per file-level type, and its Naming
// reads the lone type's name. The file then takes the name the
// language demands, and the routing key keeps the source
// derivation. A nil Split keeps every unit whole. The plan's layout
// applies it before naming, preserves order, and calls it once per
// unit, so a pure function keeps the routing deterministic.
type Split func(u plugin.Unit) []plugin.Unit

// GroupName names a declaration cluster a group template spells.
type GroupName string

// Clustered is one cluster: the group template that spells it and
// the declarations it contains, in unit order.
type Clustered struct {
	Group GroupName
	Decls []symbol.Symbol
}

// Cluster assigns one unit's declarations to named groups: a
// language that renders methods only inside a grouped block
// gathers them by the type they attach to. A declaration the
// function leaves unassigned renders through its kind template; a
// cluster renders through the group template its name selects, in
// place of its members' kind templates, at the position of its
// first member. A declaration two clusters claim goes to the
// first, and a member the unit does not contain is ignored. A
// cluster never spans two units, so plugin attribution and
// canonical order survive. A nil Cluster leaves every declaration a
// singleton.
type Cluster func(decls []symbol.Symbol) []Clustered

// Language is what varies between targets. The pass implements
// everything else.
type Language struct {
	// Kinds maps each emit kind to its template source: how the
	// language spells each declaration.
	Kinds map[symbol.Kind]string
	// Refused maps each emit kind the language declares no spelling
	// for to the reason, stated as a fact of the language. A
	// declaration of a refused kind is skipped under [RefusedKind]
	// with the reason, and a declaration of a kind neither spelt nor
	// refused is skipped under [UnspeltKind]. A kind is spelt or
	// refused, never both. The Cluster spells what it gathers
	// through a group template, so a language refuses no kind its
	// Cluster gathers.
	Refused map[symbol.Kind]string
	// File is the file skeleton, executed once per file over the
	// file's name and its package. Empty takes the default, imports
	// then declarations. The clause a language opens its
	// files with is the skeleton's own to spell, because not every
	// language has one. The header is not the skeleton's: the
	// output contract prepends it after the formatter ran.
	File string
	// Funcs returns the language's shared template vocabulary bound
	// to one file's import set, so a helper that spells a type
	// records the import the spelling needs. It is the overrideable
	// bucket: every kind template, file skeleton and reference
	// template calls it, and a declared override replaces one name
	// for all of them. The pass calls it once per worker with the
	// worker's set, which the pass resets per file, and once at
	// [New] with a set of its own to read the names for the parse,
	// so every call returns the same names. A nil Funcs declares no
	// vocabulary.
	Funcs func(set *ImportSet) template.FuncMap
	// Naming spells each unit's filename.
	Naming Naming
	// Split reshapes each unit before naming; nil files every unit
	// whole.
	Split Split
	// Cluster assigns a unit's declarations to named groups; nil
	// leaves every declaration a singleton.
	Cluster Cluster
	// Groups maps each group name a Cluster selects to its template
	// source.
	Groups map[GroupName]string
	// Scaffold spells one statement of the neutral vocabulary the
	// language's way, recording into the file's import set whatever
	// it qualified with. It is the printer the body builtin calls
	// for slot contributions and scaffold content alike; a
	// statement the language cannot spell returns an error, and the
	// declaration is skipped under the execute-time code.
	Scaffold func(s emit.Stmt, set *ImportSet) ([]byte, error)
	// Imports renders one file's collected set as the block the
	// language's own formatter would leave: grouping and sorting
	// are language facts.
	Imports func(set *ImportSet) string
	// Finalise is the language formatter, run last per file.
	Finalise func(src []byte) ([]byte, error)
	// Coverage is the language's declared fact coverage. Declared,
	// it arms the guard: a stated fact the declaration refuses
	// reports and the declaration renders without it, and one the
	// declaration misses reports a defect. Left empty, the guard is
	// off.
	Coverage Coverage
}

// Refuser is implemented by a renderer declaring the kinds its
// language refuses, which is how the conformance suite reads them
// back.
type Refuser interface {
	RefusedKinds() map[symbol.Kind]string
}
