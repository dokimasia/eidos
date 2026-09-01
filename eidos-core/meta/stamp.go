// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"fmt"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/symbol"
)

// RawStamp is one classification stamp as a frontend recorded it:
// a pre-claim that crossed a phase as data, the way a raw
// directive does. The name resolves through the registry when the
// stamp applies, because the typed handle is a composition
// constant no record can carry.
type RawStamp struct {
	// Key is the boundary spelling of the key the stamp writes.
	Key KeyName

	// Value is one term of the vocabulary: string, int64, bool,
	// []string or [symbol.Identity]. Anything else refuses at the
	// apply.
	Value any

	// Pos locates the source the classification read.
	Pos position.Pos

	// Origin is the stamping frontend. The splice fills it from
	// the unit's own frontend and overwrites whatever a frontend
	// wrote, so a stamp cannot speak for another plugin.
	Origin diag.Origin
}

// StampRaw records one raw claim: the classification path, where
// the write crossed a phase as data and no typed handle exists.
// The name resolves through the registry, the value must be one
// term of the vocabulary, and everything else checks as [Stamp]
// checks it — the kind restriction, the false boolean, the rank.
//
// What a raw write cannot check is the value's type against the
// key's, because the registry records no value type: that
// discipline is the typed handle's, at compile time, and a typed
// reader of a mistyped raw value reads absence.
func (f *Facts) StampRaw(s RawStamp, c Claim) error {
	id, registered := f.registry.Resolve(s.Key)
	if !registered {
		return fmt.Errorf("meta: %s names a key nothing registered", s.Key)
	}
	spec, err := f.spec(id)
	if err != nil {
		return err
	}
	switch v := s.Value.(type) {
	case string, int64, []string, symbol.Identity:
	case bool:
		if !v {
			return fmt.Errorf(
				"meta: %s stamps false on %s: absence is the negative, drop the fact instead",
				s.Key, c.Subject,
			)
		}
	default:
		return fmt.Errorf("meta: %s carries a %T, which the vocabulary does not",
			s.Key, s.Value)
	}
	if !kindAdmitted(spec.Kinds, c.Subject.Kind) {
		return fmt.Errorf("meta: %s does not admit kind %s, which %s is",
			s.Key, c.Subject.Kind, c.Subject)
	}
	return f.write(c.Subject, id, s.Key, stored{claim: c, value: cloneValue(s.Value)})
}
