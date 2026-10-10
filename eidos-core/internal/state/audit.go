// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"fmt"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/symbol"
)

// Audit is one unmet contract's finding as the audit table keeps it: the
// contract's key, the declaration that lacks the key's fact, and the
// finding the audit reported at the declaration.
type Audit struct {
	Key     meta.KeyName
	Subject symbol.Identity
	Finding diag.Diag
}

// Audits returns every unmet contract's finding the generation keeps,
// sorted by key, then by subject identity.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole, and for a row or a key that does not decode.
func (s *PhaseState) Audits() ([]Audit, error) {
	rows, err := s.g.readers[TableAudit].all(s.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Audit, 0, len(rows))
	for _, e := range rows {
		key, rest, found := bytes.Cut(e.key, []byte{keySep})
		subject, parsed := parseIdentityKey(rest, nil)
		if !found || !parsed {
			return nil, fmt.Errorf("%w: an audit key does not decode", ErrDamaged)
		}
		d := newDecoder(e.row, nil)
		finding := d.finding()
		if err := d.Err(); err != nil {
			return nil, fmt.Errorf("%w: the audit row of %s does not decode: %w", ErrDamaged, subject, err)
		}
		out = append(out, Audit{Key: meta.KeyName(key), Subject: subject, Finding: finding})
	}
	return out, nil
}

// auditRows returns the audit table's rows of a run's findings, sorted by
// key: each finding under its contract's key and its subject.
func auditRows(audits []Audit) []entry {
	rows := make([]entry, 0, len(audits))
	for _, a := range audits {
		key := identityKey(append([]byte(a.Key), keySep), a.Subject)
		e := encoder{}
		e.finding(a.Finding)
		rows = append(rows, entry{key: key, row: e.buf})
	}
	return sortedRows(rows)
}
