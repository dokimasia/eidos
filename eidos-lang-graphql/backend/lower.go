// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dokimasia/gqlparser/v2/ast"
	"github.com/dokimasia/gqlparser/v2/parser"

	"go.dokimi.dev/eidos/lang/graphql"
	"go.dokimi.dev/eidos/lang/lowering"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// headerName names the header of a file in a refusal.
const headerName = "the header of the file"

// selectionOpen opens the selection set of an executable definition.
const selectionOpen = "{"

// Lower rewrites one declaration into the shapes that GraphQL writes, in
// place and before any name respells. It refuses a construct that GraphQL
// has no form of, and the settle withholds the declaration. The rewrites are
// these:
//
//   - A nested declaration of a type moves to file level under its emitted
//     name, because GraphQL has no nested types.
//   - A map becomes a list of a synthesized type of entries with the fields
//     key and value: Entry after the names of the type and the field, and
//     EntryInput in an input.
//   - A method with two or more results returns a synthesized payload type
//     with a field for each result, [host.payload].
//   - A positional variadic parameter becomes a list argument.
//   - A sum becomes a union of one object type for each variant, named after
//     the sum and the variant. Under the marker graphql:input, it becomes an
//     input object with the directive oneOf, [lowerSum].
//   - The interface of a directive definition and of the extension of a
//     directive takes the sigil of a directive before its name, so the
//     respell spells it in camel case.
//   - The selection set of an executable definition loses the white space
//     around it.
//
// Every declaration that the lowering declares takes the origin of the
// declaration that it lowers, as the settle requires. Each rewrite states its
// result in the shapes of GraphQL, so a second lowering changes nothing. Lower
// keeps a declaration with type parameters as it is, because the coverage
// rejects it at the render. It keeps a constant and a variable as they are,
// because the render refuses their kinds.
//
// Error modes, each a refusal of the declaration:
//   - an annotation that is neither a GraphQL name nor a marker of the
//     satellite, a marker on a declaration of another kind or with arguments
//     outside its contract, and an argument of a directive that is not a
//     name, a colon and a constant value;
//   - a type without a field, except an extension that adds directives or
//     interfaces, and an extension of a type or of the schema with
//     documentation;
//   - a member name that a type declares twice, a field without a type, a
//     method without a result, a default that does not parse, and a default
//     of a field of an object type;
//   - an input object with a method, an interface or the marker
//     graphql:operation, a field or a method with Receives inside a type, and
//     a field or a method at file level without Receives;
//   - a stream outside the root of subscriptions, a reference with type
//     arguments, a nullable form of a nullable form, and a form that GraphQL
//     does not spell;
//   - an enum without a value, an alias that is neither a union of named
//     references nor a scalar, and a sum without a variant or with methods;
//   - a variant of an object type without a field;
//   - a directive definition without a location, and a directive definition
//     or an extension of a directive with methods, interfaces or nested
//     declarations;
//   - an extension of a directive with locations, arguments, the marker
//     graphql:repeatable or a description, and an extension without a
//     directive;
//   - a function without the marker graphql:operation or graphql:fragment, an
//     anonymous fragment, and a body that is not a verbatim selection set;
//   - a header that states a schema definition without a root operation
//     type, and an extension of the schema without a directive or a root
//     operation type;
//   - a wire name that is not a GraphQL name or that starts with two
//     underscores;
//   - the deleter of a property, through [lowering.DeleteAccessor], because
//     a schema has no properties.
//
// # Allocation contract
//
// A declaration that the lowering keeps as it is allocates the parse of each
// argument of a directive and of each default, the parse of the selection
// set of an executable definition, and the list of the names of the members
// of a type after the first 16. A rewrite allocates the declarations that it
// writes. A refusal allocates its error.
func Lower(s symbol.Symbol) ([]symbol.Symbol, error) {
	if err := lowering.DeleteAccessor(graphql.Lang, s); err != nil {
		return nil, err
	}
	switch d := s.(type) {
	case *emit.File:
		return nil, lowerHeader(d)
	case *emit.Struct:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return lowerStruct(d)
	case *emit.Interface:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return lowerInterface(d)
	case *emit.Enum:
		return nil, lowerEnum(d)
	case *emit.Alias:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return nil, lowerAlias(d)
	case *emit.Sum:
		return lowerSum(d)
	case *emit.Function:
		if len(d.TypeParams) > 0 {
			return nil, nil
		}
		return lowerFunction(d)
	case *emit.Field:
		if d.Receives == nil {
			return nil, fmt.Errorf(refusalPrefix+"a document declares a field inside a type, "+
				"and %s receives no type", d.Name)
		}
		h := &host{name: d.Receives.Spelling, origin: d.Origin, receives: true}
		if err := h.field(d); err != nil {
			return nil, err
		}
		return withOut(d, h.out), nil
	case *emit.Method:
		if d.Receives == nil {
			return nil, fmt.Errorf(refusalPrefix+"a document declares a field with arguments inside a type, "+
				"and %s receives no type", d.Name)
		}
		h := &host{name: d.Receives.Spelling, origin: d.Origin, receives: true}
		if err := h.method(d); err != nil {
			return nil, err
		}
		return withOut(d, h.out), nil
	default:
		return nil, nil
	}
}

// withOut returns the declaration d followed by the declarations out that
// its lowering synthesized, in the storage of out, and nil where it
// synthesized none, which keeps d in place. The caller does not use out
// afterwards.
func withOut(d symbol.Symbol, out []symbol.Symbol) []symbol.Symbol {
	if len(out) == 0 {
		return nil
	}
	return slices.Insert(out, 0, d)
}

// lowerHeader checks the header h of a file. A header with root markers
// states a schema definition, or an extension of the schema under the
// marker graphql:extend. A schema definition names at least one root
// operation type. An extension of the schema has no description, and adds
// a directive or a root operation type.
func lowerHeader(h *emit.File) error {
	set, err := checkAnnotations(symbol.KindFile, headerName, h.Annotations)
	if err != nil {
		return err
	}
	if err := checkExtension(headerName, set, h.Doc); err != nil {
		return err
	}
	rooted := set&(queryBit|mutationBit|subscriptionBit) != 0
	extends := set&extendBit != 0
	switch {
	case extends && !rooted && !directives(h.Annotations):
		return fmt.Errorf(refusalPrefix+"an extension of the schema adds a directive or a root operation type, "+
			"and %s states neither", headerName)
	case !extends && !rooted && (len(h.Doc) > 0 || directives(h.Annotations)):
		return fmt.Errorf(refusalPrefix+"a schema definition names a root operation type, "+
			"and %s states a description or a directive without one", headerName)
	}
	return nil
}

// lowerStruct lowers the struct s as an object type, or as an input object
// under the marker graphql:input. A struct with the operation marker
// subscription is the root of subscriptions, whose fields can stream. The
// nested declarations of s move to file level.
func lowerStruct(s *emit.Struct) ([]symbol.Symbol, error) {
	set, err := checkAnnotations(symbol.KindStruct, s.Name, s.Annotations)
	if err != nil {
		return nil, err
	}
	input := set&inputBit != 0
	switch {
	case input && set&operationBit != 0:
		return nil, fmt.Errorf(refusalPrefix+"an input object is no root operation type, and %s states the marker %s",
			s.Name, graphql.OperationMarker)
	case input && len(s.Implements) > 0:
		return nil, fmt.Errorf(refusalPrefix+"an input object implements no interface, and %s implements %s",
			s.Name, s.Implements[0].Spelling)
	}
	if err := checkExtension(s.Name, set, s.Doc); err != nil {
		return nil, err
	}
	if err := checkMembers(s.Name, s.Fields.Len()+s.Methods.Len(), set, s.Annotations, len(s.Implements)); err != nil {
		return nil, err
	}
	h := &host{
		name: s.Name, origin: s.Origin, input: input,
		streams: markerArg(s.Annotations, graphql.OperationMarker) == string(graphql.OperationSubscription),
	}
	for _, f := range s.Fields.Items() {
		if err := h.field(f); err != nil {
			return nil, err
		}
	}
	for _, m := range s.Methods.Items() {
		if err := h.method(m); err != nil {
			return nil, err
		}
	}
	if err := h.hoist(s.Types.Items()); err != nil {
		return nil, err
	}
	s.Types.Replace()
	return withOut(s, h.out), nil
}

// lowerInterface lowers the interface i as an interface, or as the
// definition of a directive under the marker graphql:directive. The marker
// graphql:repeatable marks the definition of a directive alone. The nested
// declarations of an interface move to file level.
func lowerInterface(i *emit.Interface) ([]symbol.Symbol, error) {
	set, err := checkAnnotations(symbol.KindInterface, i.Name, i.Annotations)
	if err != nil {
		return nil, err
	}
	switch {
	case set&directiveBit != 0:
		return nil, lowerDirective(i, set)
	case set&repeatableBit != 0:
		return nil, fmt.Errorf(refusalPrefix+"the marker %s marks the definition of a directive, "+
			"and %s states no marker %s", graphql.RepeatableMarker, i.Name, graphql.DirectiveMarker)
	}
	if err := checkExtension(i.Name, set, i.Doc); err != nil {
		return nil, err
	}
	if err := checkMembers(i.Name, i.Fields.Len()+i.Methods.Len(), set, i.Annotations, len(i.Extends)); err != nil {
		return nil, err
	}
	h := &host{name: i.Name, origin: i.Origin}
	for _, f := range i.Fields.Items() {
		if err := h.field(f); err != nil {
			return nil, err
		}
	}
	for _, m := range i.Methods.Items() {
		if err := h.method(m); err != nil {
			return nil, err
		}
	}
	if err := h.hoist(i.Types.Items()); err != nil {
		return nil, err
	}
	i.Types.Replace()
	return withOut(i, h.out), nil
}

// lowerDirective lowers the interface i, whose markers are set, as the
// definition of a directive, or as the extension of a directive under the
// marker graphql:extend. The fields of a definition are the arguments of the
// directive, which are inputs, and a definition applies to one or more
// locations. An extension adds one or more directives and nothing else, so
// it states no location, no argument, no marker graphql:repeatable and no
// description. Neither has a method, an interface or a nested declaration.
// The name of i takes the sigil of a directive.
func lowerDirective(i *emit.Interface, set markerSet) error {
	extension := set&extendBit != 0
	located := markerArg(i.Annotations, graphql.DirectiveMarker) != ""
	switch {
	case i.Methods.Len() > 0 || len(i.Extends) > 0 || i.Types.Len() > 0:
		return fmt.Errorf(refusalPrefix+"a directive definition declares its arguments as the fields of %s alone, "+
			"and %s declares methods, interfaces or nested declarations", i.Name, i.Name)
	case !extension && !located:
		return fmt.Errorf(refusalPrefix+"a directive definition applies to one or more locations, "+
			"and %s states none", i.Name)
	case extension && located:
		return fmt.Errorf(refusalPrefix+"an extension of a directive adds directives alone, and %s states locations",
			i.Name)
	case extension && i.Fields.Len() > 0:
		return fmt.Errorf(refusalPrefix+"an extension of a directive adds directives alone, and %s declares arguments",
			i.Name)
	case extension && set&repeatableBit != 0:
		return fmt.Errorf(refusalPrefix+"an extension of a directive adds directives alone, "+
			"and %s states the marker %s", i.Name, graphql.RepeatableMarker)
	case extension && !directives(i.Annotations):
		return fmt.Errorf(refusalPrefix+"an extension of a directive adds at least one directive, and %s adds none",
			i.Name)
	}
	if err := checkExtension(i.Name, set, i.Doc); err != nil {
		return err
	}
	h := &host{name: strings.TrimPrefix(i.Name, graphql.DirectiveSigil), origin: i.Origin, input: true}
	for _, f := range i.Fields.Items() {
		if err := h.field(f); err != nil {
			return err
		}
	}
	if len(h.out) > 0 {
		return fmt.Errorf(refusalPrefix+"an argument of a directive is a value of a type that the schema declares, "+
			"and %s takes a map", i.Name)
	}
	if !strings.HasPrefix(i.Name, graphql.DirectiveSigil) {
		i.Name = graphql.DirectiveSigil + i.Name
	}
	return nil
}

// lowerEnum checks the enum e: it has values, unless it is an extension that
// adds directives, and it declares each value once.
func lowerEnum(e *emit.Enum) error {
	set, err := checkAnnotations(symbol.KindEnum, e.Name, e.Annotations)
	if err != nil {
		return err
	}
	if err := checkExtension(e.Name, set, e.Doc); err != nil {
		return err
	}
	variants := e.Variants.Items()
	if len(variants) == 0 && (set&extendBit == 0 || !directives(e.Annotations)) {
		return fmt.Errorf(refusalPrefix+"an enum declares at least one value, and %s declares none", e.Name)
	}
	h := &host{name: e.Name}
	for _, v := range variants {
		if _, err := checkAnnotations(symbol.KindEnumVariant, v.Name, v.Annotations); err != nil {
			return err
		}
		if err := h.member(v.Name); err != nil {
			return err
		}
	}
	return nil
}

// lowerAlias checks the alias a: a scalar without a target, or a union whose
// target is a union of named references. GraphQL has no other alias of a
// type.
func lowerAlias(a *emit.Alias) error {
	set, err := checkAnnotations(symbol.KindAlias, a.Name, a.Annotations)
	if err != nil {
		return err
	}
	if err := checkExtension(a.Name, set, a.Doc); err != nil {
		return err
	}
	if a.Target == nil {
		return nil
	}
	if a.Target.Form != symbol.FormUnion {
		return fmt.Errorf(refusalPrefix+"GraphQL has no type alias, and %s aliases %s", a.Name, a.Target.Spelling)
	}
	if len(a.Target.Elems) == 0 {
		return fmt.Errorf(refusalPrefix+"a union has at least one member, and %s has none", a.Name)
	}
	for _, m := range a.Target.Elems {
		if m.Form != symbol.FormNamed || len(m.Args) > 0 {
			return fmt.Errorf(refusalPrefix+"a member of a union is an object type, and %s joins %s",
				a.Name, m.Spelling)
		}
	}
	return nil
}

// lowerFunction lowers the function f as an executable definition: an
// operation under the marker graphql:operation, or a fragment under the
// marker graphql:fragment. Its parameters are its variables, which are
// inputs, and its verbatim body is its selection set without the white space
// around it.
func lowerFunction(f *emit.Function) ([]symbol.Symbol, error) {
	set, err := checkAnnotations(symbol.KindFunction, f.Name, f.Annotations)
	if err != nil {
		return nil, err
	}
	switch {
	case set&(operationBit|fragmentBit) == 0:
		return nil, fmt.Errorf(refusalPrefix+"a document declares no callable but an operation and a fragment, "+
			"and %s states neither the marker %s nor %s", f.Name, graphql.OperationMarker, graphql.FragmentMarker)
	case set&operationBit != 0 && set&fragmentBit != 0:
		return nil, fmt.Errorf(refusalPrefix+"a definition is an operation or a fragment, and %s states both markers",
			f.Name)
	case set&anonymousBit != 0 && set&fragmentBit != 0:
		return nil, fmt.Errorf(refusalPrefix+"a fragment has a name, and %s states the marker %s",
			f.Name, graphql.AnonymousMarker)
	}
	if err := checkSelection(f.Name, &f.Body); err != nil {
		return nil, err
	}
	f.Body.Verbatim = strings.TrimSpace(f.Body.Verbatim)
	h := &host{name: f.Name, origin: f.Origin}
	if err := h.params("", f.Name, f.Params.Items()); err != nil {
		return nil, err
	}
	return withOut(f, h.out), nil
}

// checkSelection refuses the body b of the executable definition name where
// it is not a verbatim selection set: a body of another form, a body with
// the content of a slot, and text that is not one selection set that the
// parser of gqlparser reads.
func checkSelection(name string, b *emit.Body) error {
	form, err := b.Form()
	switch {
	case err != nil || form != emit.FormVerbatim:
		return fmt.Errorf(refusalPrefix+"the body of an executable definition is its selection set, "+
			"and %s states no verbatim body", name)
	case b.Prologue.Len() > 0 || b.Epilogue.Len() > 0 || len(b.Slots) > 0:
		return fmt.Errorf(refusalPrefix+"a selection set is text alone, and the body of %s has the content of a slot",
			name)
	}
	text := strings.TrimSpace(b.Verbatim)
	if !strings.HasPrefix(text, selectionOpen) {
		return fmt.Errorf(refusalPrefix+"the body of %s is not one selection set", name)
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: text})
	switch {
	case err != nil:
		return fmt.Errorf(refusalPrefix+"the selection set of %s does not parse: %w", name, err)
	case len(doc.Operations) != 1 || len(doc.Fragments) > 0:
		return fmt.Errorf(refusalPrefix+"the body of %s is not one selection set", name)
	}
	return nil
}

// checkExtension refuses the documentation doc of the declaration name where
// its markers set state the marker graphql:extend, because an extension of
// GraphQL has no description.
func checkExtension(name string, set markerSet, doc []string) error {
	if set&extendBit == 0 || len(doc) == 0 {
		return nil
	}
	return fmt.Errorf(refusalPrefix+"an extension of GraphQL has no description, and %s states documentation", name)
}

// checkMembers refuses the type name without members where it is not an
// extension that adds a directive or an interface, because an object type,
// an input object and an interface declare at least one field. n is the
// number of its members, set and as are its markers and its annotations,
// and interfaces is the number of the interfaces that it implements.
func checkMembers(name string, n int, set markerSet, as symbol.Annotations, interfaces int) error {
	if n > 0 || set&extendBit != 0 && (directives(as) || interfaces > 0) {
		return nil
	}
	return fmt.Errorf(refusalPrefix+"a type declares at least one field, and %s declares none", name)
}

// hoist lowers the nested declarations types of the type of h, and moves
// them to file level after the type with the origin of the type, because
// GraphQL has no nested types and the settle gives every output of a
// lowering the origin of its input.
func (h *host) hoist(types []symbol.Symbol) error {
	for _, t := range types {
		if err := setOrigin(t, h.origin); err != nil {
			return err
		}
		lowered, err := Lower(t)
		switch {
		case err != nil:
			return err
		case lowered == nil:
			h.out = append(h.out, t)
		default:
			h.out = append(h.out, lowered...)
		}
	}
	return nil
}

// setOrigin gives the nested declaration t the origin origin, and refuses a
// declaration of a kind that a type does not nest.
func setOrigin(t symbol.Symbol, origin symbol.Identity) error {
	switch d := t.(type) {
	case *emit.Struct:
		d.Origin = origin
	case *emit.Interface:
		d.Origin = origin
	case *emit.Enum:
		d.Origin = origin
	case *emit.Alias:
		d.Origin = origin
	case *emit.Sum:
		d.Origin = origin
	default:
		return fmt.Errorf(refusalPrefix+"a type nests types alone, and a type nests a %s", t.Kind())
	}
	return nil
}
