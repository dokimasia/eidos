// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/lang/naming"
	"go.dokimi.dev/eidos/lang/spellref"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// receiverFallback is the name of a receiver when the parameters, the
// results or the type parameters already use the letters of the host.
const receiverFallback = "recv"

// asciiLetters contains every lower-case ASCII letter, so the name of a
// receiver with one letter is a substring and allocates nothing.
const asciiLetters = "abcdefghijklmnopqrstuvwxyz"

// PointerReceiver sets the receiver of m to a pointer to the type that m
// receives, and returns m. Go declares a method that changes its value
// this way. The type of the receiver is the pointer form over a copy of
// the Receives reference, so the settle respells the name of the host in
// the receiver as it does in Receives. PointerReceiver returns a method
// without a Receives type unchanged.
//
// The name of the receiver is the first candidate that no parameter,
// result or type parameter of m uses. The candidates are the first
// letter of the host, its first two letters, recv, and then recv0, recv1
// and upward. A method that declares Put(s Session) must not name its
// receiver s, because the duplicate identifier is a compile error that a
// formatter does not detect. A letter is a rune, so the name for a host
// outside ASCII is a whole character.
//
// # Allocation contract
//
// PointerReceiver allocates five times: the receiver, its pointer
// reference, the spelling of the reference, the element list and the
// copy of the Receives reference. A receiver named by the first letter
// of the host in ASCII allocates no name. A later candidate allocates
// its spelling, and an instantiated host allocates its spelling with the
// arguments once more.
func PointerReceiver(m *emit.Method) *emit.Method {
	if m == nil || m.Receives == nil || m.Receives.Spelling == "" {
		return m
	}
	taken := make(map[string]bool, len(m.TypeParams)+m.Params.Len()+m.Returns.Len())
	for _, tp := range m.TypeParams {
		taken[tp.Name] = true
	}
	for _, p := range m.Params.Items() {
		taken[p.Name] = true
	}
	for _, r := range m.Returns.Items() {
		taken[r.Name] = true
	}
	host := *m.Receives
	m.Receiver = &emit.Param{
		Name: receiverName(host.Spelling, taken),
		Type: &emit.TypeRef{
			Spelling: pointerMark + spellref.Spell(&host, bracketOpen, bracketClose, ""),
			Form:     symbol.FormOptional,
			Elems:    []*emit.TypeRef{&host},
		},
	}
	return m
}

// receiverName returns the first candidate that taken does not contain.
// The candidates are the first letter of host, its first two letters,
// recv, and then recv0, recv1 and upward, each in lower case. It builds
// the first two letters only when taken contains the first letter, and
// [naming.Unique] chooses among the later candidates. A first letter in
// ASCII allocates nothing.
func receiverName(host string, taken map[string]bool) string {
	first, size := utf8.DecodeRuneInString(host)
	if short := letter(unicode.ToLower(first)); !taken[short] {
		return short
	}
	if _, next := utf8.DecodeRuneInString(host[size:]); next > 0 {
		size += next
	}
	return naming.Unique(taken, strings.ToLower(host[:size]), receiverFallback)
}

// letter returns the rune r as a string. It allocates only for a rune
// outside the lower-case ASCII letters.
func letter(r rune) string {
	if r >= 'a' && r <= 'z' {
		return asciiLetters[r-'a' : r-'a'+1]
	}
	return string(r)
}
