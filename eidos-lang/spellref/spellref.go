// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
)

// Spell writes one reference: the spelling, and any arguments
// recursively inside the given brackets. A nil reference or an
// empty spelling writes the stand-in, which each language names
// for itself — an anonymous marker, or its unit type.
func Spell(t *emit.TypeRef, opener, closer, standIn string) string {
	if t == nil || t.Spelling == "" {
		return standIn
	}
	if len(t.Args) == 0 {
		return t.Spelling
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a, opener, closer, standIn))
	}
	return t.Spelling + opener + strings.Join(args, ", ") + closer
}
