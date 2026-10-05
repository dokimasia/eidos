// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/symbol"
)

// Modules returns how many packages name each module the generation
// keeps: every module whose language, path and root a package's two
// module facts state, keyed as [plugin.Module] spells it. A module that
// no package names is absent.
//
// Error modes: an error wrapping [ErrDamaged] for a table that does not
// read whole, and for a row or a key that does not decode.
func (s *PhaseState) Modules() (map[plugin.Module]int, error) {
	rows, err := s.g.readers[TableModules].all(s.ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[plugin.Module]int, len(rows))
	for _, e := range rows {
		parts := bytes.Split(e.key, []byte{keySep})
		count, n := binary.Uvarint(e.row)
		if len(parts) != 3 || n != len(e.row) {
			return nil, fmt.Errorf("%w: a modules row does not decode", ErrDamaged)
		}
		m := plugin.Module{Lang: symbol.Lang(parts[0]), Root: string(parts[1]), Path: string(parts[2])}
		out[m] = int(count)
	}
	return out, nil
}

// moduleRows returns the modules table's rows of a run's counts, sorted
// by key: each module under its language, its root and its path.
func moduleRows(modules map[plugin.Module]int) []entry {
	rows := make([]entry, 0, len(modules))
	for m, count := range modules {
		key := append([]byte(m.Lang), keySep)
		key = append(append(key, m.Root...), keySep)
		key = append(key, m.Path...)
		rows = append(rows, entry{key: key, row: binary.AppendUvarint(nil, uint64(count))})
	}
	return sortedRows(rows)
}
