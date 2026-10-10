// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"cmp"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/plugin/shape/tools/specfront"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The handles of the keys that the spec frontend stamps. The spec frontend
// registers the keys through its role, with [specfront.Keys].
var (
	formKey           = meta.Named[string](specfront.KeyForm)
	detectedKey       = meta.Named[bool](specfront.KeyDetected)
	documentaryKey    = meta.Named[bool](specfront.KeyDocumentary)
	yieldsKey         = meta.Named[[]string](specfront.KeyYields)
	rolesKey          = meta.Named[[]string](specfront.KeyRoles)
	resolveKey        = meta.Named[string](specfront.KeyResolve)
	requiredKey       = meta.Named[bool](specfront.KeyRequired)
	counterexampleKey = meta.Named[bool](specfront.KeyCounterexample)
	appliesKey        = meta.Named[[]string](specfront.KeyApplies)
	minimumKey        = meta.Named[int64](specfront.KeyMinimum)
	excludesKey       = meta.Named[[]string](specfront.KeyExcludes)
	alsoOnKey         = meta.Named[[]string](specfront.KeyAlsoOn)
	fromKey           = meta.Named[string](specfront.KeyFrom)
	indexKey          = meta.Named[int64](specfront.KeyIndex)
)

// The separators that the generator joins spellings with. wordSep joins
// the words of a name, partSep joins the parts of a key, and arityJoin
// joins a role and its arity in the stamp of the roles.
const (
	wordSep   = "-"
	partSep   = "."
	arityJoin = "="
)

// entry is one spec as the generator reads it from the declarations and
// the stamps of the spec frontend. Each documentation is one line, and the
// documentation of a param or a binding is a phrase without a full stop.
type entry struct {
	id          symbol.Identity
	at          position.Pos
	name        string
	form        string
	doc         string
	detected    bool
	documentary bool
	yields      []string
	roles       []roleEntry
	params      []paramEntry
	bindings    []bindingEntry
}

// roleEntry is one role of a contract and its arity.
type roleEntry struct {
	name  string
	arity string
}

// paramEntry is one param of a spec.
type paramEntry struct {
	key            string
	typ            string
	resolve        string
	minimum        int64
	hasMinimum     bool
	required       bool
	counterexample bool
	applies        []string
	excludes       []string
	alsoOn         []string
	doc            string
}

// bindingEntry is one binding of a shape.
type bindingEntry struct {
	name  string
	from  string
	index int64
	doc   string
}

// read returns every spec of the scope, in name order, and records each
// read in the invocation of m. The spec frontend declares a struct for
// each spec and nothing else of the struct kind.
func read(m *sdk.GraphMatch) []entry {
	var out []entry
	for sym := range m.Reader().ByKind(symbol.KindStruct) {
		if st, is := sym.(*node.Struct); is {
			out = append(out, entryOf(m, st))
		}
	}
	slices.SortStableFunc(out, func(a, b entry) int { return cmp.Compare(a.name, b.name) })
	return out
}

// entryOf reads one spec from its struct and from the stamps on the struct
// and its fields. A field with a stamp of [specfront.KeyFrom] is a
// binding, and every other field is a param.
func entryOf(m *sdk.GraphMatch, st *node.Struct) entry {
	e := entry{id: st.ID, at: st.Pos, name: st.Name, doc: strings.Join(st.Doc, space)}
	e.form, _ = sdk.FactOf(m, st.ID, formKey)
	e.detected, _ = sdk.FactOf(m, st.ID, detectedKey)
	e.documentary, _ = sdk.FactOf(m, st.ID, documentaryKey)
	e.yields, _ = sdk.FactOf(m, st.ID, yieldsKey)
	roles, _ := sdk.FactOf(m, st.ID, rolesKey)
	for _, r := range roles {
		name, arity, _ := strings.Cut(r, arityJoin)
		e.roles = append(e.roles, roleEntry{name: name, arity: arity})
	}
	for _, f := range st.Fields {
		doc := strings.TrimSuffix(strings.Join(f.Doc, space), stop)
		if from, binds := sdk.FactOf(m, f.ID, fromKey); binds {
			index, _ := sdk.FactOf(m, f.ID, indexKey)
			e.bindings = append(e.bindings, bindingEntry{name: f.Name, from: from, index: index, doc: doc})
			continue
		}
		p := paramEntry{key: f.Name, typ: f.Type.Spelling, doc: doc}
		p.resolve, _ = sdk.FactOf(m, f.ID, resolveKey)
		p.required, _ = sdk.FactOf(m, f.ID, requiredKey)
		p.counterexample, _ = sdk.FactOf(m, f.ID, counterexampleKey)
		p.applies, _ = sdk.FactOf(m, f.ID, appliesKey)
		p.minimum, p.hasMinimum = sdk.FactOf(m, f.ID, minimumKey)
		p.excludes, _ = sdk.FactOf(m, f.ID, excludesKey)
		p.alsoOn, _ = sdk.FactOf(m, f.ID, alsoOnKey)
		e.params = append(e.params, p)
	}
	return e
}

// check reports the faults that concern more than one spec, and reports
// whether the specs are free of them. The faults are two specs of one
// name, a Go identifier that two specs give or that one spec gives twice,
// a yields_to entry that is no detected shape, and a cycle of yields_to
// entries. The entries are in name order, so two specs of one name are
// next to each other.
func check(m *sdk.GraphMatch, entries []entry) bool {
	clean := true
	owners := map[string]entry{}
	for i, e := range entries {
		if i > 0 && entries[i-1].name == e.name {
			clean = false
			for _, at := range []entry{entries[i-1], e} {
				m.Errorf(specfront.SpecDuplicate, at.at, "two specs have the name %s", e.name)
			}
			continue
		}
		for _, ident := range namesOf(e).all() {
			first, taken := owners[ident]
			if !taken {
				owners[ident] = e
				continue
			}
			clean = false
			if first.id == e.id {
				m.Errorf(specfront.SpecDuplicate, e.at, "the spec %s gives the Go identifier %s twice", e.name, ident)
				continue
			}
			for _, at := range []entry{first, e} {
				m.Errorf(specfront.SpecDuplicate, at.at, "the specs %s and %s both give the Go identifier %s",
					first.name, e.name, ident)
			}
		}
	}
	detected := map[string]bool{}
	for _, e := range entries {
		detected[e.name] = e.detected
	}
	for _, e := range entries {
		for _, y := range e.yields {
			if !detected[y] {
				clean = false
				m.Errorf(specfront.SpecInvalid, e.at, "the shape %s yields to %s, which is no detected shape",
					e.name, y)
			}
		}
	}
	for _, e := range entries {
		if e.detected && leads(entries, e.name, e.name, map[string]bool{}) {
			clean = false
			m.Errorf(specfront.PrecedenceCycle, e.at,
				"the yields_to lists form a cycle through %s, so no order ranks it", e.name)
		}
	}
	return clean
}

// leads reports whether a chain of yields_to entries leads from one
// shape to another, without passing a shape twice.
func leads(entries []entry, from, to string, seen map[string]bool) bool {
	seen[from] = true
	at := slices.IndexFunc(entries, func(e entry) bool { return e.name == from })
	if at < 0 {
		return false
	}
	for _, y := range entries[at].yields {
		if y == to || !seen[y] && leads(entries, y, to, seen) {
			return true
		}
	}
	return false
}

// precedence returns the detected shapes in the order of precedence. A
// shape comes before every shape that yields to it, and of two shapes that
// the yields_to lists leave unordered, the shape with the first name comes
// first. The yields_to lists of the entries form no cycle, which [check]
// ensures.
func precedence(entries []entry) []entry {
	var pending []entry
	for _, e := range entries {
		if e.detected {
			pending = append(pending, e)
		}
	}
	out := make([]entry, 0, len(pending))
	placed := map[string]bool{}
	for len(pending) > 0 {
		next := slices.IndexFunc(pending, func(e entry) bool {
			return !slices.ContainsFunc(e.yields, func(y string) bool { return !placed[y] })
		})
		out = append(out, pending[next])
		placed[pending[next].name] = true
		pending = slices.Delete(pending, next, next+1)
	}
	return out
}
