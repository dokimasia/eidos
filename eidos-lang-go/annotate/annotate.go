// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	sdk "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// New builds the Go annotator: the graph-proven facts, stamped at
// plugin authority under the language's identity.
func New() plugin.Annotator {
	var h golang.Handles
	stampType := func(m matched, st *sdk.Stamper) {
		facts := prove(m.reader(), m.subject(), m.fields(), m.methods(), m.embeds())
		if facts.satisfiesError {
			sdk.Stamp(st, h.SatisfiesError, true)
		}
		if facts.satisfiesStringer {
			sdk.Stamp(st, h.SatisfiesStringer, true)
		}
		if facts.embedsInterface && m.subject().Kind == symbol.KindStruct {
			// The key speaks for structs alone: an interface
			// embedding an interface is the language's norm, not a
			// fact, and the key's kind set refuses it.
			sdk.Stamp(st, h.EmbedsInterface, true)
		}
		if facts.comparable {
			sdk.Stamp(st, h.Comparable, true)
		}
	}
	built := sdk.NewPlugin(golang.Name).
		Keys(func(r *meta.Registry) error {
			registered, err := golang.Register(r)
			h = registered
			return err
		}).
		Handle(sdk.OnStruct(func(m *sdk.StructMatch, st *sdk.Stamper) error {
			stampType(structMatch{m}, st)
			return nil
		})).
		Handle(sdk.OnInterface(func(m *sdk.InterfaceMatch, st *sdk.Stamper) error {
			stampType(interfaceMatch{m}, st)
			return nil
		})).
		Handle(sdk.OnEnum(func(m *sdk.EnumMatch, st *sdk.Stamper) error {
			stampType(enumMatch{m}, st)
			return nil
		})).
		Build()
	annotator, is := built.(plugin.Annotator)
	if !is {
		panic("annotate: the kit built no annotator, which is a declaration defect")
	}
	return annotator
}

// matched is what one type-shaped subject offers the proofs.
type matched interface {
	reader() *store.Reader
	subject() symbol.Identity
	fields() []*node.Field
	methods() []*node.Method
	embeds() []*node.Embed
}

type structMatch struct{ m *sdk.StructMatch }

func (a structMatch) reader() *store.Reader    { return a.m.Reader() }
func (a structMatch) subject() symbol.Identity { return a.m.Struct.ID }
func (a structMatch) fields() []*node.Field    { return a.m.Struct.Fields }
func (a structMatch) methods() []*node.Method  { return a.m.Struct.Methods }
func (a structMatch) embeds() []*node.Embed    { return a.m.Struct.Embeds }

type interfaceMatch struct{ m *sdk.InterfaceMatch }

func (a interfaceMatch) reader() *store.Reader    { return a.m.Reader() }
func (a interfaceMatch) subject() symbol.Identity { return a.m.Interface.ID }
func (a interfaceMatch) fields() []*node.Field    { return a.m.Interface.Fields }
func (a interfaceMatch) methods() []*node.Method  { return a.m.Interface.Methods }
func (a interfaceMatch) embeds() []*node.Embed    { return a.m.Interface.Embeds }

type enumMatch struct{ m *sdk.EnumMatch }

func (a enumMatch) reader() *store.Reader    { return a.m.Reader() }
func (a enumMatch) subject() symbol.Identity { return a.m.Enum.ID }
func (a enumMatch) fields() []*node.Field    { return a.m.Enum.Fields }
func (a enumMatch) methods() []*node.Method  { return a.m.Enum.Methods }
func (enumMatch) embeds() []*node.Embed      { return nil }

// proven is what the graph settles about one type.
type proven struct {
	satisfiesError    bool
	satisfiesStringer bool
	embedsInterface   bool
	comparable        bool
}

// prove derives the facts for one subject: its visible method set
// across folded methods, package-level attachment and resolved
// embeds, and its fields' comparability.
func prove(
	r *store.Reader, id symbol.Identity,
	fields []*node.Field, methods []*node.Method, embeds []*node.Embed,
) proven {
	set := methodSet(r, id, methods, embeds, map[symbol.Identity]bool{})
	out := proven{
		satisfiesError:    hasNullaryString(set, "Error"),
		satisfiesStringer: hasNullaryString(set, "String"),
	}
	for _, e := range embeds {
		if e.Ref == nil || e.Ref.Target.IsZero() {
			continue
		}
		if embedded, held := r.Lookup(e.Ref.Target); held {
			if embedded.Kind() == symbol.KindInterface {
				out.embedsInterface = true
			}
		}
	}
	out.comparable = comparableShape(r, fields, embeds, map[symbol.Identity]bool{})
	return out
}

// methodSet assembles a type's visible methods: the folded list,
// the package-level methods owning it, and — recursively, cycles
// guarded — the methods of every embed the graph resolves.
func methodSet(
	r *store.Reader, id symbol.Identity,
	methods []*node.Method, embeds []*node.Embed,
	visiting map[symbol.Identity]bool,
) []*node.Method {
	if visiting[id] {
		return nil
	}
	visiting[id] = true

	out := append([]*node.Method{}, methods...)
	for decl := range r.ByKind(symbol.KindMethod) {
		m, is := decl.(*node.Method)
		if !is {
			continue
		}
		owner := m.Identity()
		if owner.Package == id.Package && owner.Owner == id.Name {
			out = append(out, m)
		}
	}
	for _, e := range embeds {
		if e.Ref == nil || e.Ref.Target.IsZero() {
			continue
		}
		embedded, held := r.Lookup(e.Ref.Target)
		if !held {
			continue
		}
		switch t := embedded.(type) {
		case *node.Struct:
			out = append(out, methodSet(r, t.ID, t.Methods, t.Embeds, visiting)...)
		case *node.Interface:
			out = append(out, methodSet(r, t.ID, t.Methods, t.Embeds, visiting)...)
		}
	}
	return out
}

// hasNullaryString reports a method of the given name taking
// nothing and returning one string.
func hasNullaryString(set []*node.Method, name string) bool {
	for _, m := range set {
		if m.Name != name || len(m.Params) != 0 || len(m.Returns) != 1 {
			continue
		}
		if m.Returns[0].Type != nil && m.Returns[0].Type.Spelling == "string" {
			return true
		}
	}
	return false
}

// comparableShape proves a struct shape comparable under Go's own
// rules, embedded fields included, or reports false the moment one
// part is unprovable: slices, maps and funcs never are, workspace
// types recurse, and a type the graph does not hold stays unproven.
func comparableShape(
	r *store.Reader, fields []*node.Field, embeds []*node.Embed,
	visiting map[symbol.Identity]bool,
) bool {
	for _, f := range fields {
		if f.Type == nil || !comparableRef(r, f.Type, visiting) {
			return false
		}
	}
	for _, e := range embeds {
		// An embedded field takes part in comparison the way a
		// named one does.
		if e.Ref == nil || !comparableRef(r, e.Ref, visiting) {
			return false
		}
	}
	return true
}

// comparableRef proves one reference comparable.
func comparableRef(r *store.Reader, ref *node.TypeRef, visiting map[symbol.Identity]bool) bool {
	s := strings.TrimSpace(ref.Spelling)
	switch {
	case strings.HasPrefix(s, "[]"),
		strings.HasPrefix(s, "map["),
		wordPrefix(s, "func"):
		return false
	case strings.HasPrefix(s, "*"), wordPrefix(s, "chan"):
		return true
	case wordPrefix(s, "interface"):
		return true
	}
	if comparableBuiltins[s] {
		return true
	}
	if ref.Target.IsZero() {
		return false // outside the workspace: unprovable, never guessed
	}
	if visiting[ref.Target] {
		return false
	}
	visiting[ref.Target] = true
	decl, held := r.Lookup(ref.Target)
	if !held {
		return false
	}
	switch t := decl.(type) {
	case *node.Struct:
		return comparableShape(r, t.Fields, t.Embeds, visiting)
	case *node.Interface:
		return true
	case *node.Enum:
		return true
	case *node.Alias:
		if t.Target == nil {
			return false
		}
		return comparableRef(r, t.Target, visiting)
	default:
		return false
	}
}

// wordPrefix reports whether s starts with word as a whole word:
// the word alone, or followed by a non-identifier character, so an
// identifier merely starting with the word does not match.
func wordPrefix(s, word string) bool {
	rest, prefixed := strings.CutPrefix(s, word)
	if !prefixed {
		return false
	}
	if rest == "" {
		return true
	}
	c := rest[0]
	return c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9')
}

// comparableBuiltins are the predeclared types Go compares.
var comparableBuiltins = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true,
	"int8": true, "int16": true, "int32": true, "int64": true,
	"rune": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"any": true,
}
