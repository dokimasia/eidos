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

// The methods whose presence the annotator stamps: the method of the
// error interface and the method of fmt.Stringer.
const (
	errorMethod  = "Error"
	stringMethod = "String"
)

// prove derives the facts of one subject from its method set, the
// interfaces that it embeds, and the comparability of its type. The Go
// rules return the method set and decide each fact, so a stamped fact and
// the projected answer of the rules come from one rule. One method set
// serves both stamps, and [gorules.NullaryString] finds the method Error
// as the error contract of the Go rules finds it.
func prove(r *store.Reader, decl symbol.Symbol, id symbol.Identity, embeds []*node.Embed) proven {
	view := rules.NewView(r, nil, nil, meta.KernelKeys{})
	set := gorules.MethodSet(decl, view)
	out := proven{
		satisfiesError:    gorules.NullaryString(set, errorMethod, view) != nil,
		satisfiesStringer: gorules.NullaryString(set, stringMethod, view) != nil,
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
	out.comparable, _ = gorules.Rules{}.Comparable(&node.TypeRef{Spelling: id.Name, Target: id}, view)
	return out
}
