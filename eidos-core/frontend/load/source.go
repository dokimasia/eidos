// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import "go.dokimi.dev/eidos/core/store"

// source is the regions of a load's units, in splice order, which the
// load's graph reads: the region the load built, restored or relinked,
// and a kept unit's region in the history, decoded the first time the
// graph needs it. A kept unit's summary is its record's, so the graph
// indexes the unit without a decode.
//
// # Concurrency
//
// Region is safe for concurrent use: a kept unit's region decodes under
// the history's mutex, and every other region is not written again.
type source struct {
	h     *history
	units []*unit
	infos []store.RegionInfo
}

// newSource returns the source over a load's units, every region the
// load built, restored or relinked already set.
func newSource(h *history, units []*unit) *source {
	infos := make([]store.RegionInfo, len(units))
	for i, u := range units {
		if u.region != nil {
			infos[i] = u.region.Info()
		} else {
			infos[i] = u.record.Summary
		}
	}
	return &source{h: h, units: units, infos: infos}
}

// Regions returns each unit's region summary, in splice order.
func (s *source) Regions() []store.RegionInfo { return s.infos }

// Region returns the i-th unit's region, decoding a kept unit's from
// the history.
//
// Error modes: the prior's error for a region that does not read whole.
func (s *source) Region(i int) (*store.Region, error) {
	if r := s.units[i].region; r != nil {
		return r, nil
	}
	return s.h.region(s.units[i].record)
}
