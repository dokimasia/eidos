// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The keywords and marks of the composite types that [Restate] writes,
// beside the punctuation of a type expression. A function type writes one
// result after resultSep, and two or more in parentheses.
const (
	sliceMark = bracketOpen + bracketClose
	mapOpen   = "map" + bracketOpen
	recvChan  = "<-chan "
	sendChan  = "chan<- "
	chanWord  = "chan "
	funcOpen  = "func" + parenOpen
	listSep   = ", "
	resultSep = " "
)

// The starts of the compacted spelling of a receive-only and of a
// send-only channel, which [Restate] reads the direction of a channel
// from.
const (
	recvStart = "<-"
	sendStart = "chan<-"
)

// Restate writes a structural reference of Go from the spellings of its
// children. elems has the spelling of each child of t, in the order of
// t.Elems. Restate writes a pointer for an optional, a slice for a list,
// and a variadic parameter for a list whose spelling starts with three
// dots. It writes an array of its length, a map, a channel in the
// direction of its spelling, and a function type with its results. The
// length of an array whose length is an expression has the spellings of
// the names of the length, which follow its element among the children.
// It is the restatement of Go for [spellref.SpellRestated], and the Go
// spoke writes the composite types of a translation through it.
//
// Error modes:
//   - A reference with fewer spellings than its form takes returns an
//     error.
//   - An array returns an error where its length is an expression without
//     names among its children, and where its spelling does not parse as
//     an array type or its length does not contain the names of its
//     children in their order. An array without a spelling and without a
//     length returns an error too.
//   - A form without a composite spelling in Go, such as a tuple, returns
//     an error.
//
// # Allocation contract
//
// Restate allocates the spelling that it returns. A function type also
// allocates the joined parameters and results, and a channel allocates
// the compacted spelling that it reads the direction from. An array whose
// length has names allocates the parse of its spelling, the spans of the
// names and the length that it writes. An error allocates its message.
func Restate(t *emit.TypeRef, elems []string) (string, error) {
	var want int
	switch t.Form {
	case symbol.FormMap:
		want = 2
	case symbol.FormFunc:
		want = t.Split
	default:
		want = 1
	}
	if len(elems) < want || t.Split > len(elems) {
		return "", fmt.Errorf("%s: %s has %d of the %d types that its form takes", Lang, t.Spelling, len(elems), want)
	}
	switch t.Form {
	case symbol.FormOptional:
		return pointerMark + elems[0], nil
	case symbol.FormList:
		if strings.HasPrefix(t.Spelling, variadicMark) {
			return variadicMark + elems[0], nil
		}
		return sliceMark + elems[0], nil
	case symbol.FormArray:
		length, err := restatedLength(t, elems[1:])
		if err != nil {
			return "", err
		}
		return bracketOpen + length + bracketClose + elems[0], nil
	case symbol.FormMap:
		return mapOpen + elems[0] + bracketClose + elems[1], nil
	case symbol.FormStream:
		compact := strings.Join(strings.Fields(t.Spelling), "")
		switch {
		case strings.HasPrefix(compact, recvStart):
			return recvChan + elems[0], nil
		case strings.HasPrefix(compact, sendStart):
			return sendChan + elems[0], nil
		default:
			return chanWord + elems[0], nil
		}
	case symbol.FormFunc:
		text := funcOpen + strings.Join(elems[:t.Split], listSep) + parenClose
		switch results := elems[t.Split:]; len(results) {
		case 0:
			return text, nil
		case 1:
			return text + resultSep + results[0], nil
		default:
			return text + resultSep + parenOpen + strings.Join(results, listSep) + parenClose, nil
		}
	default:
		return "", fmt.Errorf("%s: %s has a form without a Go spelling that follows from its structure",
			Lang, t.Spelling)
	}
}

// restatedLength returns the length of the array t with the spellings in
// names in place of the names of its length. The names of the length are
// the children of t after its element, and names has the spelling of each,
// in their order. restatedLength finds each name in the length of the
// spelling of t by the spelling that its child records, in the order of
// the source, and keeps the rest of the length as the source wrote it. An
// array without names writes its Length, and a literal length of zero as
// the source wrote it.
func restatedLength(t *emit.TypeRef, names []string) (string, error) {
	if len(names) == 0 && t.Length > 0 {
		return strconv.Itoa(t.Length), nil
	}
	if t.Spelling == "" {
		return "", fmt.Errorf("%s: an array without a spelling has no length", Lang)
	}
	e, err := parser.ParseExpr(t.Spelling)
	arr, isArray := e.(*ast.ArrayType)
	if err != nil || !isArray || arr.Len == nil {
		return "", fmt.Errorf("%s: %s does not parse as an array type", Lang, t.Spelling)
	}
	base := arr.Len.Pos()
	length := t.Spelling[base-1 : arr.Len.End()-1]
	if len(names) == 0 {
		if _, literal := arr.Len.(*ast.BasicLit); !literal {
			return "", fmt.Errorf("%s: %s has a length expression, and its reference records none of its names",
				Lang, t.Spelling)
		}
		return length, nil
	}
	if len(t.Elems) != len(names)+1 {
		return "", fmt.Errorf("%s: %s has another count of spellings than of names of its length", Lang,
			t.Spelling)
	}
	children := t.Elems[1:]
	spans := make([][2]int, 0, len(children))
	ast.Inspect(arr.Len, func(n ast.Node) bool {
		if len(spans) == len(children) {
			return false
		}
		switch n.(type) {
		case *ast.Ident, *ast.SelectorExpr:
			from, to := int(n.Pos()-base), int(n.End()-base)
			if length[from:to] == children[len(spans)].Spelling {
				spans = append(spans, [2]int{from, to})
				return false
			}
		}
		return true
	})
	if len(spans) != len(names) {
		return "", fmt.Errorf("%s: the length of %s does not contain the names of its reference in their order",
			Lang, t.Spelling)
	}
	var b strings.Builder
	at := 0
	for i, s := range spans {
		b.WriteString(length[at:s[0]])
		b.WriteString(names[i])
		at = s[1]
	}
	b.WriteString(length[at:])
	return b.String(), nil
}
