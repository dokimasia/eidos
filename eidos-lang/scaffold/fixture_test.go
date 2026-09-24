// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package scaffold_test

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// scripted is a target spelling a C-family syntax, so the shared
// walk runs without a satellite. It records every import the walk
// requests, which is the walk's contract with a real target.
type scripted struct {
	imported []string
	// addressed records the kind of every value the walk passed to
	// Address.
	addressed []emit.ValueKind
	// refuseRaw refuses a raw literal, the way a target meeting
	// another language's text does.
	refuseRaw bool
}

func (*scripted) Lang() string { return "scripted" }

func (s *scripted) Literal(v emit.Value) (string, error) {
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat, emit.LiteralBool:
		return v.Text, nil
	case emit.LiteralString:
		return `"` + v.Text + `"`, nil
	case emit.LiteralNil:
		return "null", nil
	case emit.LiteralRaw:
		if s.refuseRaw {
			return "", fmt.Errorf("scripted: %q is written in another language", v.Text)
		}
		return v.Text, nil
	default:
		return "", fmt.Errorf("scripted: no spelling for the %s literal", v.Literal)
	}
}

func (s *scripted) Type(t *emit.TypeRef) (string, error) {
	if t.Target.Package != "" {
		s.imported = append(s.imported, t.Target.Package)
	}
	return t.Spelling, nil
}

func (s *scripted) Callee(id symbol.Identity) (string, error) {
	if id.Package != "" {
		s.imported = append(s.imported, id.Package)
	}
	return id.Name, nil
}

func (*scripted) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return typ + "(" + inner + ")", nil
}

func (*scripted) Composite(_ *emit.TypeRef, typ string, entries []scaffold.Entry) (string, error) {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Name != "":
			parts = append(parts, e.Name+": "+e.Value)
		case e.Key != "":
			parts = append(parts, e.Key+": "+e.Value)
		default:
			parts = append(parts, e.Value)
		}
	}
	return typ + "{" + strings.Join(parts, ", ") + "}", nil
}

func (s *scripted) Address(inner emit.Value, spelled string) (string, error) {
	s.addressed = append(s.addressed, inner.Kind)
	return "&" + spelled, nil
}

// unspelling is a target refusing every form, so the walk's
// error path runs at each spelling in turn.
type unspelling struct{ scripted }

func (unspelling) Conversion(*emit.TypeRef, string, string) (string, error) {
	return "", fmt.Errorf("scripted: no conversion form")
}

func (unspelling) Composite(*emit.TypeRef, string, []scaffold.Entry) (string, error) {
	return "", fmt.Errorf("scripted: no composite form")
}

func (unspelling) Address(emit.Value, string) (string, error) {
	return "", fmt.Errorf("scripted: no address form")
}

// ref returns a reference to a declaration in one package.
func ref(pkg, name string) *emit.TypeRef {
	return &emit.TypeRef{
		Spelling: name,
		Target:   symbol.Identity{Lang: "scripted", Package: pkg, Name: name, Kind: symbol.KindStruct},
	}
}

// fn returns a callee identity in one package.
func fn(pkg, name string) symbol.Identity {
	return symbol.Identity{Lang: "scripted", Package: pkg, Name: name, Kind: symbol.KindFunction}
}
