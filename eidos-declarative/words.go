// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The spellings that two or more vocabularies of the manifest share: the
// types of a param, of the elements of a list and of an option.
const (
	spellString    = "string"
	spellInt       = "int"
	spellBool      = "bool"
	spellList      = "list"
	spellReference = "reference"
)

// word is one spelling of a closed vocabulary of the manifest, and the
// value of the spelling.
type word[T comparable] struct {
	spelling string
	value    T
}

// decodeWord returns the value of the word whose spelling is the value of
// the scalar n. what describes the vocabulary in the message of an error,
// such as "a param type".
//
// Error modes: decodeWord returns a *yaml.TypeError at the line of n for a
// node that is not a scalar, and for a spelling that no word has. The
// message lists every spelling.
func decodeWord[T comparable](n *yaml.Node, words []word[T], what string) (T, error) {
	if n.Kind == yaml.ScalarNode {
		for _, w := range words {
			if w.spelling == n.Value {
				return w.value, nil
			}
		}
	}
	spelled := strconv.Quote(n.Value)
	if n.Kind != yaml.ScalarNode {
		spelled = n.ShortTag()
	}
	var zero T
	return zero, lineFault(n, "%s is not %s: use %s", spelled, what, spellings(words))
}

// wordsSchema returns the JSON Schema of a vocabulary: a string that is one
// of the spellings of words, in their order. It returns a new map on each
// call.
func wordsSchema[T comparable](words []word[T]) map[string]any {
	enum := make([]any, 0, len(words))
	for _, w := range words {
		enum = append(enum, w.spelling)
	}
	return map[string]any{keywordType: typeString, keywordEnum: enum}
}

// spellings joins the spellings of words for a message, as in "source,
// package or plan".
func spellings[T comparable](words []word[T]) string {
	var b strings.Builder
	for i, w := range words {
		if i > 0 && i == len(words)-1 {
			b.WriteString(" or ")
		} else if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(w.spelling)
	}
	return b.String()
}

// spellingOf returns the spelling of the word whose value is v. The
// decoder produced v from a spelling of words, so a word has v, and
// spellingOf panics on a value that no word has.
func spellingOf[T comparable](words []word[T], v T) string {
	i := slices.IndexFunc(words, func(w word[T]) bool { return w.value == v })
	return words[i].spelling
}
