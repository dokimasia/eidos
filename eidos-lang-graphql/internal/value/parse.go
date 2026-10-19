// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package value

import (
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/lexer"
)

// The names that GraphQL reads as values of their own and not as enum
// values.
const (
	trueName  = "true"
	falseName = "false"
	nullName  = "null"
)

// Parse parses text as one constant value of GraphQL: an integer, a float,
// a string, a block string, true, false, null, an enum value, or a list or
// an object of constant values. White space, commas and comments can
// surround the value and separate its parts, as they do in a document. The
// value has the kinds and the raw text that the parser of gqlparser gives
// it: the content of a string without its quotes and its escapes, and the
// name of an enum value. A value has no position.
//
// Error modes: Parse returns an error for an error of the lexer, for a
// token that does not continue the value, and for a token after the value.
// A variable is such a token, because a constant value does not contain
// one.
//
// # Allocation contract
//
// Parse allocates the source of the lexer, each value, each entry of a list
// and of an object, and the growth of the list of the entries of each. The
// lexer allocates the content of a string with an escape. An error
// allocates its message.
func Parse(text string) (*ast.Value, error) {
	p := parser{lx: lexer.New(&ast.Source{Input: text})}
	p.next()
	v := p.value()
	if p.tok.Kind != lexer.EOF {
		p.unexpected()
	}
	if p.err != nil {
		return nil, p.err
	}
	return v, nil
}

// parser is the state of one parse: the lexer, the current token and the
// first error.
type parser struct {
	lx  lexer.Lexer
	tok lexer.Token
	err error
}

// next reads the next token that is not a comment into the current token.
// After an error, next reads nothing, and the current token stays.
func (p *parser) next() {
	for p.err == nil {
		tok, err := p.lx.ReadToken()
		if err != nil {
			p.err = fmt.Errorf("value: %w", err)
			return
		}
		if tok.Kind != lexer.Comment {
			p.tok = tok
			return
		}
	}
}

// value parses the value that starts at the current token.
func (p *parser) value() *ast.Value {
	tok := p.tok
	var kind ast.ValueKind
	switch tok.Kind {
	case lexer.BracketL:
		return p.list()
	case lexer.BraceL:
		return p.object()
	case lexer.Int:
		kind = ast.IntValue
	case lexer.Float:
		kind = ast.FloatValue
	case lexer.String:
		kind = ast.StringValue
	case lexer.BlockString:
		kind = ast.BlockValue
	case lexer.Name:
		switch tok.Value {
		case trueName, falseName:
			kind = ast.BooleanValue
		case nullName:
			kind = ast.NullValue
		default:
			kind = ast.EnumValue
		}
	default:
		p.unexpected()
		return nil
	}
	p.next()
	return &ast.Value{Raw: tok.Value, Kind: kind}
}

// list parses a list from its opening bracket to its closing bracket.
func (p *parser) list() *ast.Value {
	v := &ast.Value{Kind: ast.ListValue}
	p.next()
	for p.err == nil && p.tok.Kind != lexer.BracketR {
		v.Children = append(v.Children, &ast.ChildValue{Value: p.value()})
	}
	p.next()
	return v
}

// object parses an object from its opening brace to its closing brace. A
// field of an object is a name, a colon and a value.
func (p *parser) object() *ast.Value {
	v := &ast.Value{Kind: ast.ObjectValue}
	p.next()
	for p.err == nil && p.tok.Kind != lexer.BraceR {
		name := p.tok
		if name.Kind != lexer.Name {
			p.unexpected()
			break
		}
		p.next()
		if p.tok.Kind != lexer.Colon {
			p.unexpected()
			break
		}
		p.next()
		v.Children = append(v.Children, &ast.ChildValue{Name: name.Value, Value: p.value()})
	}
	p.next()
	return v
}

// unexpected records the error of the current token, which does not
// continue the value. It keeps an earlier error, because the current token
// does not move after one.
func (p *parser) unexpected() {
	if p.err == nil {
		p.err = fmt.Errorf("value: unexpected %s at %d:%d", p.tok, p.tok.Pos.Line, p.tok.Pos.Column)
	}
}
