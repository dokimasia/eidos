// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"go.dokimi.dev/eidos/lang/treesitter"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// pendingConst is an inherent impl's associated constant, lowered as a
// field of the type level, with the comments' parts, the restricted
// visibility's spelling and the attributes, whose carriers and stamps
// apply once the field has a home.
type pendingConst struct {
	field   *node.Field
	parts   plugin.CommentParts
	spelled string
	a       attributes
}

// pendingImpl is one impl block, lowered and waiting to fold onto the
// type it names once every module is lowered: the scope and File node
// of the module that states it, the type it names, its trait for a
// trait impl, and an inherent impl's methods and associated constants.
type pendingImpl struct {
	scope   *scope
	file    *node.File
	self    *node.TypeRef
	trait   *node.TypeRef
	methods []*node.Method
	consts  []pendingConst
}

// implItem lowers an impl block, whose own attributes are a. A trait
// impl records its trait, and its items declare nothing, because the
// trait declares them, so a carrier or a marker on one is refused. An
// inherent impl lowers its methods, a function without self at the type
// level, and its associated constants as fields of the type level that
// are immutable. Signature depth leaves out an item that is not public.
// A carrier or a marker on the block itself is refused.
func (l *lowering) implItem(n treesitter.Node, c container, a attributes) {
	l.refuse(n, a, "an impl block")
	impl := pendingImpl{scope: c.scope, file: c.file, self: l.typeRef(n.Child(l.v.fieldType))}
	body := n.Child(l.v.fieldBody)
	if trait := n.Child(l.v.fieldTrait); !trait.IsZero() {
		impl.trait = l.typeRef(trait)
		var attrs []treesitter.Node
		for item := range body.NamedChildren() {
			switch {
			case item.Kind() == l.v.attributeItem:
				attrs = append(attrs, item)
			case !l.comment(item):
				l.refuse(item, l.attributes(attrs), "an item of a trait impl, which the trait declares")
				attrs = nil
			}
		}
		l.w.impls = append(l.w.impls, impl)
		return
	}
	var attrs []treesitter.Node
	for item := range body.NamedChildren() {
		switch {
		case item.Kind() == l.v.attributeItem:
			attrs = append(attrs, item)
			continue
		case l.comment(item):
			continue
		}
		a := l.attributes(attrs)
		attrs = nil
		vis, spelled := l.visibility(item)
		if a.excluded != "" {
			l.excluded = append(l.excluded, a.excluded)
		}
		if a.excluded != "" || !l.kept(vis) {
			l.skip(item)
			continue
		}
		switch item.Kind() {
		case l.v.functionItem, l.v.functionSignatureItem:
			impl.methods = append(impl.methods, l.methodOf(item, a, symbol.VisibilityPrivate))
		case l.v.constItem:
			nameNode := item.Child(l.v.fieldName)
			parts, comment := l.declParts(item, a)
			impl.consts = append(impl.consts, pendingConst{
				field: &node.Field{
					Name: nameNode.Text(), Pos: nameNode.Pos(), Doc: parts.Docs, Comment: comment,
					Visibility: vis, Level: symbol.LevelType, Mutability: symbol.MutabilityImmutable,
					Type: l.typeRef(item.Child(l.v.fieldType)), Value: item.Child(l.v.fieldValue).Text(),
					Annotations: a.annotations,
				},
				parts: parts, spelled: spelled, a: a,
			})
		default:
			l.refuse(item, a, "an item the model does not contain, such as a macro")
		}
	}
	l.w.impls = append(l.w.impls, impl)
}

// fold folds every impl block onto the type it names, in the order the
// crate's modules lowered them. The type is the first candidate the
// impl's module resolves the type's name to that the crate declares. A
// trait impl adds its trait to a struct's Implements, and an enum has no
// list of implemented traits. An inherent impl adds its methods to the
// type, and its constants to a struct's or an enum's fields. A data
// enum has no field list, so its constants report under
// [UnmodeledItem]. An inherent impl of a type the crate does not
// declare lowers its methods as declared outside their type, which
// receives them, and its constants report under [UnmodeledItem].
func (w *crate) fold() {
	for _, impl := range w.impls {
		owner := w.ownerOf(impl)
		if impl.trait != nil {
			if st, is := owner.(*node.Struct); is {
				st.Implements = append(st.Implements, impl.trait)
			}
			continue
		}
		switch t := owner.(type) {
		case *node.Struct:
			t.Methods = append(t.Methods, impl.methods...)
			t.Fields = append(t.Fields, w.placed(impl.consts)...)
		case *node.Enum:
			t.Methods = append(t.Methods, impl.methods...)
			t.Fields = append(t.Fields, w.placed(impl.consts)...)
		case *node.Sum:
			t.Methods = append(t.Methods, impl.methods...)
			w.unmodeled(impl.consts, "a data enum has no field list")
		default:
			for _, m := range impl.methods {
				m.Receives = impl.self
				impl.file.Decls = append(impl.file.Decls, m)
			}
			w.unmodeled(impl.consts, "the crate does not declare the type it is associated with")
		}
	}
}

// ownerOf returns the declaration an impl block's type names: the first
// candidate its module's scope resolves the type's spelling to that the
// crate declares, and nil where the crate declares none of them, as for
// a reference, whose spelling names no declaration.
func (w *crate) ownerOf(impl pendingImpl) symbol.Symbol {
	for _, tier := range impl.scope.candidates(impl.self.Spelling) {
		for _, c := range tier {
			if decl := w.types[c.Package][c.Name]; decl != nil {
				return decl
			}
		}
	}
	return nil
}

// placed returns the fields of constants that found a home, their
// carriers attached and their stamps recorded.
func (w *crate) placed(consts []pendingConst) []*node.Field {
	out := make([]*node.Field, 0, len(consts))
	for _, k := range consts {
		w.mark(k.field, k.parts, k.spelled, k.a)
		out = append(out, k.field)
	}
	return out
}

// unmodeled reports each associated constant the model has no home for,
// at its position, and refuses its carriers and its markers.
func (w *crate) unmodeled(consts []pendingConst, why string) {
	for _, k := range consts {
		w.u.Infof(UnmodeledItem, k.field.Pos, "the associated constant %s is not in the model: %s", k.field.Name, why)
		refuseCarriers(w.u, k.parts.Carriers, "an associated constant the model does not contain")
		refuseMarkers(w.u, k.a.sugars, "an associated constant the model does not contain")
	}
}
