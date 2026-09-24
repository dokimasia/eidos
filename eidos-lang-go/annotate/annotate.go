// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	sdk "go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// New builds the Go annotator: the graph-proven facts, stamped at
// plugin authority under the language's identity.
func New() plugin.Annotator {
	var h golang.Handles
	stampType := func(m matched, st *sdk.Stamper) {
		facts := prove(m.reader(), m.subject(), m.methods(), m.embeds())
		if facts.satisfiesError {
			sdk.Stamp(st, h.SatisfiesError, true)
		}
		if facts.satisfiesStringer {
			sdk.Stamp(st, h.SatisfiesStringer, true)
		}
		if facts.embedsInterface && m.subject().Kind == symbol.KindStruct {
			// The key applies to structs alone: an interface
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
		Handle(sdk.OnAlias(func(m *sdk.AliasMatch, st *sdk.Stamper) error {
			stampType(aliasMatch{m}, st)
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
	methods() []*node.Method
	embeds() []*node.Embed
}

type structMatch struct{ m *sdk.StructMatch }

func (a structMatch) reader() *store.Reader    { return a.m.Reader() }
func (a structMatch) subject() symbol.Identity { return a.m.Struct.ID }
func (a structMatch) methods() []*node.Method  { return a.m.Struct.Methods }
func (a structMatch) embeds() []*node.Embed    { return a.m.Struct.Embeds }

type interfaceMatch struct{ m *sdk.InterfaceMatch }

func (a interfaceMatch) reader() *store.Reader    { return a.m.Reader() }
func (a interfaceMatch) subject() symbol.Identity { return a.m.Interface.ID }
func (a interfaceMatch) methods() []*node.Method  { return a.m.Interface.Methods }
func (a interfaceMatch) embeds() []*node.Embed    { return a.m.Interface.Embeds }

type enumMatch struct{ m *sdk.EnumMatch }

func (a enumMatch) reader() *store.Reader    { return a.m.Reader() }
func (a enumMatch) subject() symbol.Identity { return a.m.Enum.ID }
func (a enumMatch) methods() []*node.Method  { return a.m.Enum.Methods }
func (enumMatch) embeds() []*node.Embed      { return nil }

// aliasMatch is a defined type or an alias: the kind has no members,
// so its methods are the file-level ones its package attaches to it.
type aliasMatch struct{ m *sdk.AliasMatch }

func (a aliasMatch) reader() *store.Reader    { return a.m.Reader() }
func (a aliasMatch) subject() symbol.Identity { return a.m.Alias.ID }
func (aliasMatch) methods() []*node.Method    { return nil }
func (aliasMatch) embeds() []*node.Embed      { return nil }

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
	r *store.Reader, id symbol.Identity, methods []*node.Method, embeds []*node.Embed,
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
	out.comparable = provenComparable(r, id)
	return out
}

// provenComparable asks the language's rules whether the subject's own
// type compares with ==, over a view of the tracked reader, so the
// stamped fact and the projected answer come from one rule.
func provenComparable(r *store.Reader, id symbol.Identity) bool {
	ok, _ := gorules.Rules{}.Comparable(&node.TypeRef{Spelling: id.Name, Target: id}, rules.View{Decls: r})
	return ok
}

// methodSet assembles a type's visible methods: the folded list,
// the file-level methods of the type's own package that attach to
// it, and, recursively with cycles guarded, the methods of every
// embed the graph resolves. The file-level methods are read from
// the type's package alone, because Go declares a method in its
// receiver's package, so one type costs its package's declarations
// and never the workspace's methods.
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
	if pkg, held := r.PackageOf(id); held {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				if m, is := decl.(*node.Method); is && m.Identity().Owner == id.Name {
					out = append(out, m)
				}
			}
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
