// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"slices"

	"github.com/vektah/gqlparser/v2/ast"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/position"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// typenameField is the meta field that every selection set can select.
// Its value is the name of the object type of the value, a non-null String.
const typenameField = "__typename"

// coordinateSep joins the name of a type and the name of one of its fields
// into the schema coordinate of the field.
const coordinateSep = "."

// subset is one selection set that a response key selects into one value.
// direct reports a set in the file of the unit that lowers, whose fields
// attach their comments, and end is the index in Sig of its closing brace.
type subset struct {
	set    ast.SelectionSet
	direct bool
	end    int
}

// item is one field of the selection sets of a value after the walk of
// their fragments: the field, the type condition of the fragment that
// selects it, whether a directive makes it conditional, and the selection
// set that contains it.
type item struct {
	field       *ast.Field
	condition   *ast.Definition
	conditional bool
	direct      bool
	bounds      member
}

// applying returns the items that apply to the object type o: the items
// whose type condition is o, or a type of which o is a possible type.
func applying(items []item, o *ast.Definition, schema *ast.Schema) []item {
	out := make([]item, 0, len(items))
	for _, it := range items {
		if includes(it.condition, o, schema) {
			out = append(out, it)
		}
	}
	return out
}

// spread is one fragment that a selection set spreads: its name and its
// type condition.
type spread struct {
	name      string
	condition *ast.Definition
}

// selector lowers the selection sets of the executable definitions of one
// unit. schema is the schema that gqlparser loaded for the unit, and nil
// where it does not load, so every field of a selection loads without a
// type. fragments are the fragments of the unit by name, which a spread
// selects the fields of.
type selector struct {
	src       *source
	schema    *ast.Schema
	fragments map[string]*ast.FragmentDefinition
}

// value lowers the selection sets of one value on the type parent into the
// declaration of the value that a response contains for them, named name.
// On an object type the declaration is a struct with one field for each
// response key. On an interface or a union it is a struct where every field
// applies to every possible type, and a sum with a variant for each
// possible type where a fragment selects fields of particular types. The
// nested types of the fields of a struct are its nested types. nest
// declares the nested types of the fields of a sum beside the sum, because
// a sum declares no nested types.
func (sel *selector) value(
	name string, at position.Pos, sets []subset, parent *ast.Definition, nest func(symbol.Symbol),
) symbol.Symbol {
	var items []item
	var spreads []spread
	for _, s := range sets {
		items = sel.collect(s, parent, false, items, &spreads, nil)
	}
	if objects := sel.variantsOf(parent, items); len(objects) > 0 {
		sum := &node.Sum{Pos: at, Name: name, Visibility: symbol.VisibilityPublic}
		for _, o := range objects {
			v := &node.SumVariant{Pos: at, Name: o.Name}
			v.Fields = sel.fields(applying(items, o, sel.schema), o, name+o.Name, nest)
			sel.stampSpreads(v, at, spreads, o)
			sum.Variants = append(sum.Variants, v)
		}
		return sum
	}
	st := &node.Struct{Pos: at, Name: name, Visibility: symbol.VisibilityPublic}
	st.Fields = sel.fields(items, parent, "", func(t symbol.Symbol) { st.Types = append(st.Types, t) })
	sel.stampSpreads(st, at, spreads, nil)
	return st
}

// collect walks one selection set and appends its fields to items: the
// fields of the set, of its inline fragments and of the fragments that it
// spreads, with the type condition of the fragment that selects each of
// them. A field under @include or @skip, or in a fragment with either
// directive or with @defer, is conditional. The spreads of the set and of
// its inline fragments append to spreads, and the spreads of a spread
// fragment do not. visiting guards against a cycle of fragments, which the
// validator reports.
func (sel *selector) collect(
	s subset, condition *ast.Definition, conditional bool, items []item, spreads *[]spread, visiting []string,
) []item {
	nameAt := func(i int) *ast.Position { return sel.src.selectionStart(s.set[i]) }
	described := func(int) bool { return false }
	for i, selection := range s.set {
		switch x := selection.(type) {
		case *ast.Field:
			it := item{field: x, condition: condition, conditional: conditional || skippable(x.Directives)}
			if s.direct {
				it.direct, it.bounds = true, sel.src.memberAt(i, len(s.set), nameAt, described, s.end)
			}
			items = append(items, it)
		case *ast.InlineFragment:
			cond := condition
			if x.TypeCondition != "" && sel.schema != nil {
				cond = sel.schema.Types[x.TypeCondition]
			}
			inner := subset{set: x.SelectionSet, direct: s.direct}
			if s.direct {
				// The opening brace is the token before the first selection,
				// and the end of the set is the brace that pairs with it.
				opening := sel.src.tokenAt(sel.src.selectionStart(x.SelectionSet[0])) - 1
				inner.end = sel.src.lexed.Pair[opening]
			}
			items = sel.collect(inner, cond, conditional || skippable(x.Directives), items, spreads, visiting)
		case *ast.FragmentSpread:
			fr, held := sel.fragments[x.Name]
			if !held || slices.Contains(visiting, x.Name) {
				continue
			}
			var cond *ast.Definition
			if sel.schema != nil {
				cond = sel.schema.Types[fr.TypeCondition]
			}
			if spreads != nil {
				*spreads = append(*spreads, spread{name: x.Name, condition: cond})
			}
			inner := conditional || skippable(x.Directives) || skippable(fr.Directives)
			items = sel.collect(subset{set: fr.SelectionSet}, cond, inner, items, nil, append(visiting, x.Name))
		}
	}
	return items
}

// variantsOf returns the object types of the sum of the items of a value on
// the type parent, in the order of the possible types of the schema. A value
// on an interface or a union is a sum where a fragment selects fields of a
// type other than parent. variantsOf returns nil for a value without such a
// fragment, for a value on an object type, and without a schema.
func (sel *selector) variantsOf(parent *ast.Definition, items []item) []*ast.Definition {
	if sel.schema == nil || parent == nil || !parent.IsAbstractType() {
		return nil
	}
	narrower := slices.ContainsFunc(items, func(it item) bool {
		return it.condition != nil && it.condition != parent
	})
	if !narrower {
		return nil
	}
	var out []*ast.Definition
	for _, t := range sel.schema.GetPossibleTypes(parent) {
		if t.Kind == ast.Object {
			out = append(out, t)
		}
	}
	return out
}

// fields lowers the items of a value on the type parent into fields, one
// for each response key in the order of its first item. The response key
// of a field is its alias, which gqlparser sets to the name of a field
// without one. prefix opens the names of the nested types of the fields,
// and nest declares them.
func (sel *selector) fields(
	items []item, parent *ast.Definition, prefix string, nest func(symbol.Symbol),
) []*node.Field {
	var keys []string
	byKey := map[string][]item{}
	for _, it := range items {
		key := it.field.Alias
		if len(byKey[key]) == 0 {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], it)
	}
	out := make([]*node.Field, 0, len(keys))
	for _, key := range keys {
		out = append(out, sel.field(key, byKey[key], parent, prefix, nest))
	}
	return out
}

// field lowers the items of one response key into one field. The field has
// the type that the schema gives the field that it selects, and is optional
// where every item of its key is conditional, because the response can lack
// it. Its directives are its annotations, and its schema coordinate is its
// coordinate stamp. The comments of the first item in the file attach to
// it.
func (sel *selector) field(
	key string, group []item, parent *ast.Definition, prefix string, nest func(symbol.Symbol),
) *node.Field {
	first := group[0].field
	at := sel.src.at(first.Position)
	owner := group[0].condition
	if owner == nil {
		owner = parent
	}
	coordinate := first.Name
	if owner != nil {
		coordinate = owner.Name + coordinateSep + first.Name
	}
	f := &node.Field{
		Pos: at, Name: key, Visibility: symbol.VisibilityPublic, Level: symbol.LevelInstance,
		Annotations: annotations(first.Directives),
	}
	f.Type = sel.fieldType(key, group, owner, coordinate, prefix, at, nest)
	always := slices.ContainsFunc(group, func(it item) bool { return !it.conditional })
	if !always && f.Type.Form != symbol.FormOptional {
		f.Type = plugin.Composite(nullable(f.Type), at, symbol.FormOptional, f.Type)
	}
	if i := slices.IndexFunc(group, func(it item) bool { return it.direct }); i >= 0 {
		b := group[i].bounds
		last := b.last
		if set := group[i].field.SelectionSet; len(set) > 0 {
			// The comment of a field with a selection set trails its opening
			// brace, the token before the first selection.
			last = sel.src.tokenAt(sel.src.selectionStart(set[0])) - 1
		}
		f.Comment = sel.src.notes.attach(f, b.first, last)
	}
	sel.src.gb.Stamp(f, meta.RawStamp{Key: graphql.CoordinateKey, Value: coordinate, Pos: at})
	return f
}

// fieldType returns the type of the field of one response key: the type of
// the schema field that it selects. A field of a composite type has the
// declaration of its merged selection sets in place of the named type,
// named after the response key. A field that no schema field types has a
// reference without a target, spelled with its schema coordinate.
// __typename is a non-null String. Every node of the type is at the
// position at of the field in the document, because the schema writes the
// type in another file.
func (sel *selector) fieldType(
	key string, group []item, owner *ast.Definition, coordinate, prefix string, at position.Pos,
	nest func(symbol.Symbol),
) *node.TypeRef {
	first := group[0].field
	if first.Name == typenameField {
		return named(graphql.StringScalar, at)
	}
	var def *ast.FieldDefinition
	if owner != nil {
		def = owner.Fields.ForName(first.Name)
	}
	if def == nil {
		return named(coordinate, at)
	}
	place := func(*ast.Position) position.Pos { return at }
	target := sel.schema.Types[def.Type.Name()]
	var sets []subset
	for _, it := range group {
		if len(it.field.SelectionSet) == 0 {
			continue
		}
		s := subset{set: it.field.SelectionSet, direct: it.direct}
		if it.direct {
			opening := sel.src.tokenAt(sel.src.selectionStart(it.field.SelectionSet[0])) - 1
			s.end = sel.src.lexed.Pair[opening]
		}
		sets = append(sets, s)
	}
	if len(sets) == 0 || target == nil || !target.IsCompositeType() {
		return typeRef(def.Type, place, named)
	}
	name := key
	if prefix != "" {
		name = prefix + titled(key)
	}
	nest(sel.value(name, at, sets, target, nest))
	return typeRef(def.Type, place, func(string, position.Pos) *node.TypeRef { return named(name, at) })
}

// stampSpreads stamps on subject the fragments that it spreads, in source
// order. A variant of the object type o lists the spreads whose type
// condition includes o, and a struct, whose o is nil, lists every spread.
func (sel *selector) stampSpreads(subject symbol.Symbol, at position.Pos, spreads []spread, o *ast.Definition) {
	var names []string
	for _, sp := range spreads {
		if o == nil || includes(sp.condition, o, sel.schema) {
			names = append(names, sp.name)
		}
	}
	if len(names) > 0 {
		sel.src.gb.Stamp(subject, meta.RawStamp{Key: graphql.SpreadsKey, Value: names, Pos: at})
	}
}

// skippable reports whether the directives include @include, @skip or
// @defer, under which a response or its first payload can lack the selected
// field.
func skippable(ds ast.DirectiveList) bool {
	return ds.ForName(graphql.IncludeDirective) != nil || ds.ForName(graphql.SkipDirective) != nil ||
		ds.ForName(graphql.DeferDirective) != nil
}

// includes reports whether the type condition cond includes the object type
// o: a condition of o itself, or of an interface or a union of which o is a
// possible type. A field without a condition applies to every type.
func includes(cond, o *ast.Definition, schema *ast.Schema) bool {
	switch {
	case cond == nil || cond == o:
		return true
	case !cond.IsAbstractType():
		return false
	default:
		return slices.Contains(schema.GetPossibleTypes(cond), o)
	}
}

// titled returns name with its first letter in upper case, so a joined name
// keeps the boundary between its words. A GraphQL name is ASCII.
func titled(name string) string {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return name
	}
	return string(name[0]-'a'+'A') + name[1:]
}

// selectionStart returns the position of the first token of a selection:
// the alias or the name of a field, and the spread of a fragment, because
// the node of a fragment is at the token after its spread.
func (s *source) selectionStart(selection ast.Selection) *ast.Position {
	p := selection.GetPosition()
	if _, field := selection.(*ast.Field); field {
		return p
	}
	return &s.lexed.Tokens[s.lexed.Sig[s.tokenAt(p)-1]].Pos
}
