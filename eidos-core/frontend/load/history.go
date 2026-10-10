// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"fmt"
	"sync"
	"sync/atomic"

	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
)

// history is the last committed load's record as a load reads it: the
// unit records in splice order, each found by its first member and by
// its number, each frontend's doors, and the regions the load decoded,
// each once. A cold load reads an empty history.
//
// # Concurrency
//
// A region decodes under a mutex, because a sealed graph decodes the
// regions of kept units from its readers' goroutines. Every other part
// is read by the load's goroutine alone.
type history struct {
	prior    Prior
	units    []UnitRecord
	byFirst  map[string]int
	byNumber map[int]int
	doors    map[plugin.ID][]DoorRecord
	// decoded counts the regions decoded through the history, which
	// [Report.Decoded] returns.
	decoded *atomic.Int64

	mu      sync.Mutex
	regions map[int]*store.Region
}

// readHistory reads the unit records of a prior, and returns an empty
// history for a nil one.
//
// Error modes: the prior's error for a record that does not read whole.
func readHistory(p Prior, decoded *atomic.Int64) (*history, error) {
	h := &history{
		prior:    p,
		byFirst:  map[string]int{},
		byNumber: map[int]int{},
		doors:    map[plugin.ID][]DoorRecord{},
		decoded:  decoded,
		regions:  map[int]*store.Region{},
	}
	if p == nil {
		return h, nil
	}
	for u, err := range p.Units() {
		if err != nil {
			return nil, fmt.Errorf("load: read the recorded units: %w", err)
		}
		h.byFirst[u.Files[0].Path] = len(h.units)
		h.byNumber[u.Number] = len(h.units)
		h.units = append(h.units, u)
	}
	return h, nil
}

// cold reports whether the history records no load.
func (h *history) cold() bool { return h.prior == nil }

// first returns the record of the unit whose first member is path, and
// nil for a path that begins no recorded unit.
func (h *history) first(path string) *UnitRecord {
	if i, held := h.byFirst[path]; held {
		return &h.units[i]
	}
	return nil
}

// number returns the record of the unit of a number, as a probe names
// it, and nil for a number the history lacks.
func (h *history) number(n int) *UnitRecord {
	if i, held := h.byNumber[n]; held {
		return &h.units[i]
	}
	return nil
}

// region returns a record's region, decoding it the first time.
//
// Error modes: the prior's error for a region that does not read whole.
func (h *history) region(rec *UnitRecord) (*store.Region, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, held := h.regions[rec.Number]; held {
		return r, nil
	}
	r, err := h.prior.Region(*rec)
	if err != nil {
		return nil, fmt.Errorf("load: read the region of %s: %w", rec.Files[0].Path, err)
	}
	h.decoded.Add(1)
	h.regions[rec.Number] = r
	return r, nil
}

// doorsOf returns a frontend's recorded doors, reading them the first
// time, and nothing for a cold history.
//
// Error modes: the prior's error for a door that does not read whole.
func (h *history) doorsOf(f plugin.ID) ([]DoorRecord, error) {
	if h.cold() {
		return nil, nil
	}
	if doors, read := h.doors[f]; read {
		return doors, nil
	}
	doors, err := h.prior.Doors(f)
	if err != nil {
		return nil, fmt.Errorf("load: read the doors of %s: %w", f, err)
	}
	h.doors[f] = doors
	return doors, nil
}
