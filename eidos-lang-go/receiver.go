// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings a pointer receiver composes from beside a type
// expression's punctuation: the separator between an argument list's
// entries, and the name a receiver falls back to once the host's
// letters are taken.
const (
	argsSeparator    = ", "
	receiverFallback = "recv"
)

// asciiLetters spells every lower-case ASCII letter, so a receiver
// named by one letter takes a substring and allocates nothing.
const asciiLetters = "abcdefghijklmnopqrstuvwxyz"

// PointerReceiver states m's receiver as a pointer to the type m
// receives, the way Go declares a method that changes its value, and
// returns m. The receiver's type is the pointer form over a copy of
// the received reference, so the settle respells the host's name in
// the receiver as it does in Receives. A method that receives no
// type is returned unchanged.
//
// The receiver's name is the first of the host's first letter, its
// first two letters, recv, then recv0, recv1 and upward that no
// parameter, result or type parameter of m takes: a method
// declaring Put(s Session) must not bind its receiver to s, a
// duplicate-identifier compile error a formatter cannot catch.
// Letters are runes, so a host spelled outside ASCII yields a whole
// character.
//
// # Allocation contract
//
// PointerReceiver allocates the receiver, its pointer reference, the
// reference's spelling, its one element and the copy of the received
// reference: five allocations. A receiver named by the host's first
// letter in ASCII allocates no name. A later candidate allocates its
// spelling, and an instantiated host the spelling of its arguments.
func PointerReceiver(m *emit.Method) *emit.Method {
	if m == nil || m.Receives == nil || m.Receives.Spelling == "" {
		return m
	}
	taken := make(map[string]bool, len(m.TypeParams)+len(m.Params)+len(m.Returns))
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
	}
	for _, p := range m.Params {
		taken[p.Name] = true
	}
	for _, r := range m.Returns {
		taken[r.Name] = true
	}
	host := *m.Receives
	m.Receiver = &emit.Param{
		Name: receiverName(host.Spelling, taken),
		Type: &emit.TypeRef{
			Spelling: pointerMark + written(&host),
			Form:     symbol.FormOptional,
			Elems:    []*emit.TypeRef{&host},
		},
	}
	return m
}

// receiverName returns the first candidate no taken name uses: the
// host's first letter, its first two letters, recv, then recv0,
// recv1 and upward, each lower-cased. It spells each candidate only
// when the one before it is taken. A first letter in ASCII allocates
// nothing.
func receiverName(host string, taken map[string]bool) string {
	first, size := utf8.DecodeRuneInString(host)
	if short := letter(unicode.ToLower(first)); !taken[short] {
		return short
	}
	if _, next := utf8.DecodeRuneInString(host[size:]); next > 0 {
		size += next
	}
	if longer := strings.ToLower(host[:size]); !taken[longer] {
		return longer
	}
	if !taken[receiverFallback] {
		return receiverFallback
	}
	for i := 0; ; i++ {
		if candidate := receiverFallback + strconv.Itoa(i); !taken[candidate] {
			return candidate
		}
	}
}

// letter spells one rune, and allocates only for a rune outside the
// lower-case ASCII letters.
func letter(r rune) string {
	if r >= 'a' && r <= 'z' {
		return asciiLetters[r-'a' : r-'a'+1]
	}
	return string(r)
}

// written returns a reference as Go writes it: an instantiation's
// name, then its arguments in brackets, and any other reference's
// spelling, a structural argument's included.
func written(t *emit.TypeRef) string {
	if len(t.Args) == 0 {
		return t.Spelling
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, written(a))
	}
	return t.Spelling + bracketOpen + strings.Join(args, argsSeparator) + bracketClose
}
