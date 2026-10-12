// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package annotate

import (
	golang "go.dokimi.dev/eidos/lang/go"
	gorules "go.dokimi.dev/eidos/lang/go/rules"
	"go.dokimi.dev/eidos/sdk"
	"go.dokimi.dev/eidos/sdk/meta"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/plugin"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/store"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// New builds the Go annotator. It stamps the facts that the graph proves
// at plugin authority under [golang.AnnotatorName], through the keys that
// the Go frontend registers. The Keys function of the annotator returns
// the error of [golang.Lookup] for a composition without those keys.
//
// # Allocation contract
//
// New allocates its four handlers, the proof that they share, the handles
// under which they stamp and the key registration, seven allocations. The
// kit allocates 24 more to build the plugin, 31 allocations in total.
func New() plugin.Annotator {
	var h golang.Handles
	stampType := func(m matched, st *sdk.Stamper) {
		facts := prove(m.reader(), m.decl(), m.subject(), m.embeds())
		if facts.satisfiesError {
			sdk.Stamp(st, h.SatisfiesError, true)
		}
		if facts.satisfiesStringer {
			sdk.Stamp(st, h.SatisfiesStringer, true)
		}
		if facts.embedsInterface && m.subject().Kind == symbol.KindStruct {
			// The key applies to structs alone. An interface that embeds
			// an interface is the norm of the language and not a fact, and
			// the kind set of the key refuses it.
			sdk.Stamp(st, h.EmbedsInterface, true)
		}
		if facts.comparable {
			sdk.Stamp(st, h.Comparable, true)
		}
	}
	return sdk.NewPlugin(golang.AnnotatorName).
		Keys(func(r *meta.Registry) error {
			looked, err := golang.Lookup(r)
			h = looked
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
		Annotator()
}

// matched is the view of one type-shaped subject that the proofs read.
type matched interface {
	reader() *store.Reader
	decl() symbol.Symbol
	subject() symbol.Identity
	embeds() []*node.Embed
}

// structMatch adapts the match of a struct to matched.
type structMatch struct{ m *sdk.StructMatch }

func (a structMatch) reader() *store.Reader    { return a.m.Reader() }
func (a structMatch) decl() symbol.Symbol      { return a.m.Struct }
func (a structMatch) subject() symbol.Identity { return a.m.Struct.ID }
func (a structMatch) embeds() []*node.Embed    { return a.m.Struct.Embeds }

// interfaceMatch adapts the match of an interface to matched.
type interfaceMatch struct{ m *sdk.InterfaceMatch }

func (a interfaceMatch) reader() *store.Reader    { return a.m.Reader() }
func (a interfaceMatch) decl() symbol.Symbol      { return a.m.Interface }
func (a interfaceMatch) subject() symbol.Identity { return a.m.Interface.ID }
func (a interfaceMatch) embeds() []*node.Embed    { return a.m.Interface.Embeds }

// enumMatch adapts the match of an enumeration to matched. An
// enumeration has no embeds.
type enumMatch struct{ m *sdk.EnumMatch }

func (a enumMatch) reader() *store.Reader    { return a.m.Reader() }
func (a enumMatch) decl() symbol.Symbol      { return a.m.Enum }
func (a enumMatch) subject() symbol.Identity { return a.m.Enum.ID }
func (enumMatch) embeds() []*node.Embed      { return nil }

// aliasMatch adapts the match of a defined type or an alias to matched.
// The kind has no members, so its methods are the file-level methods that
// its package attaches to it.
type aliasMatch struct{ m *sdk.AliasMatch }

func (a aliasMatch) reader() *store.Reader    { return a.m.Reader() }
func (a aliasMatch) decl() symbol.Symbol      { return a.m.Alias }
func (a aliasMatch) subject() symbol.Identity { return a.m.Alias.ID }
func (aliasMatch) embeds() []*node.Embed      { return nil }

// proven contains the facts that the graph proves about one type.
type proven struct {
	satisfiesError    bool
	satisfiesStringer bool
	embedsInterface   bool
	comparable        bool
}

// prove derives the facts of one subject from its visible method set,
// the interfaces that it embeds, and the comparability of its type.
func prove(r *store.Reader, decl symbol.Symbol, id symbol.Identity, embeds []*node.Embed) proven {
	set := methodSet(r, decl, id)
	out := proven{
		satisfiesError:    hasNullaryString(r, set, "Error"),
		satisfiesStringer: hasNullaryString(r, set, "String"),
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

// provenComparable asks the Go rules whether the type of the subject is
// comparable with ==, over a view of the tracked reader. The stamped fact
// and the projected answer then come from one rule.
func provenComparable(r *store.Reader, id symbol.Identity) bool {
	view := rules.NewView(r, nil, nil, meta.KernelKeys{})
	ok, _ := gorules.Rules{}.Comparable(&node.TypeRef{Spelling: id.Name, Target: id}, view)
	return ok
}

// methodSet assembles the visible methods of a type. For a struct, an
// interface and an enumeration, they are the methods that the member walk
// of the Go rules keeps. Those are the declared methods and the methods
// that the embeds promote. Two promoted methods of one name at one depth
// cancel each other, and a generic embed is bound to its arguments. The
// file-level methods of the package of the type that attach to it follow.
// A defined type has no members, so the file-level methods are all of its
// methods. methodSet reads them from the package of the type alone,
// because Go declares a method in the package of its receiver. One type
// then costs the declarations of its package and never the methods of the
// workspace.
func methodSet(r *store.Reader, decl symbol.Symbol, id symbol.Identity) []*node.Method {
	var out []*node.Method
	view := rules.NewView(r, nil, nil, meta.KernelKeys{})
	if set, membered := rules.NewBound(gorules.Rules{}, view, nil).MembersOf(decl); membered {
		out = make([]*node.Method, 0, len(set.Members))
		for _, m := range set.Members {
			if method, isMethod := m.Symbol.(*node.Method); isMethod {
				out = append(out, method)
			}
		}
	}
	if pkg, held := r.PackageOf(id); held {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				if m, isMethod := d.(*node.Method); isMethod && m.Identity().Owner == id.Name {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// hasNullaryString reports whether the set has a method of the given name
// that takes nothing and returns one string. [builtinString] reads the
// type of the result.
func hasNullaryString(r *store.Reader, set []*node.Method, name string) bool {
	for _, m := range set {
		if m.Name != name || len(m.Params) != 0 || len(m.Returns) != 1 {
			continue
		}
		if builtinString(r, m.Returns[0].Type) {
			return true
		}
	}
	return false
}

// builtinString reports whether ref is the predeclared string of Go. That
// is a reference without a target whose spelling is string, or an alias
// that the graph declares for one, followed through each alias. A defined
// type over string is a type of its own, and a reference that the graph
// does not resolve is unknown, so builtinString reports false for both.
// It reads each alias through r, which records the read.
func builtinString(r *store.Reader, ref *node.TypeRef) bool {
	seen := map[symbol.Identity]bool{}
	for ref != nil && ref.Form == symbol.FormNamed {
		if ref.Target.IsZero() {
			return ref.Spelling == stringType
		}
		if seen[ref.Target] {
			return false
		}
		seen[ref.Target] = true
		decl, held := r.Lookup(ref.Target)
		alias, is := decl.(*node.Alias)
		if !held || !is || alias.Defined {
			return false
		}
		ref = alias.Target
	}
	return false
}

// stringType is the spelling of the predeclared string of Go.
const stringType = "string"
