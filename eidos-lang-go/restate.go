// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"fmt"
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
// dots. It writes an array of the length of t, a map, a channel in the
// direction of its spelling, and a function type with its results. It is
// the restatement of Go for [spellref.SpellRestated], and the Go spoke
// writes the composite types of a translation through it.
//
// Error modes:
//   - A reference with fewer spellings than its form takes returns an
//     error.
//   - An array whose length is an expression returns an error, because
//     t.Length records a literal length alone.
//   - A form without a composite spelling in Go, such as a tuple, returns
//     an error.
//
// # Allocation contract
//
// Restate allocates the spelling that it returns. A function type also
// allocates the joined parameters and results, and a channel allocates
// the compacted spelling that it reads the direction from. An error
// allocates its message.
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
		if t.Length == 0 {
			return "", fmt.Errorf("%s: %s has a length that is an expression, and the reference records "+
				"a literal length alone", Lang, t.Spelling)
		}
		return bracketOpen + strconv.Itoa(t.Length) + bracketClose + elems[0], nil
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
