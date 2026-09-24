// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"fmt"
	"reflect"

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
// The name resolves through the registry. The value must be one
// term of the vocabulary, of the type the key registered with.
// Everything else checks as [Stamp] checks it: the kind
// restriction, the false boolean, the rank. Every subject
// [Facts.ByKey] lists therefore reads present through [Get].
func (f *Facts) StampRaw(s RawStamp, c Claim) error {
	id, registered := f.registry.Resolve(s.Key)
	if !registered {
		return fmt.Errorf("meta: %s names a key nothing registered", s.Key)
	}
	spec, err := f.spec(id)
	if err != nil {
		return err
	}
	switch s.Value.(type) {
	case string, int64, bool, []string, symbol.Identity:
	default:
		return fmt.Errorf("meta: %s carries a %T, which the vocabulary does not",
			s.Key, s.Value)
	}
	if want := f.registry.typeOf(id); reflect.TypeOf(s.Value) != want {
		return fmt.Errorf("meta: %s is a %s key, and the value is a %T", s.Key, want, s.Value)
	}
	if err := admitClaim(spec, s.Key, s.Value, c); err != nil {
		return err
	}
	return f.write(c.Subject, id, s.Key, stored{claim: c, value: cloneValue(s.Value)})
}

// admitClaim applies the checks the typed and the raw path share.
// A false boolean refuses, because absence is the negative. The
// key's kind restriction must admit the subject's kind.
func admitClaim(spec KeySpec, name KeyName, value any, c Claim) error {
	if flag, isBool := value.(bool); isBool && !flag {
		return fmt.Errorf("meta: %s stamps false on %s: absence is the negative, drop the fact instead",
			name, c.Subject)
	}
	if !kindAdmitted(spec.Kinds, c.Subject.Kind) {
		return fmt.Errorf("meta: %s does not admit kind %s, which %s is",
			name, c.Subject.Kind, c.Subject)
	}
	return nil
}
