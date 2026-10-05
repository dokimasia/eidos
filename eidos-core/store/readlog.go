// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"slices"

	"go.dokimi.dev/eidos/core/internal/grow"
)

// minGrowth is the capacity a log's empty slice takes on its first
// append.
const minGrowth = 16

// ReadLog keeps the edges of many read sets in flat storage, one entry
// for each set it records: what a dispatcher keeps of each invocation's
// reads while it reuses one set across a rule's invocations.
//
// [ReadLog.Append] copies a set's edges into the log, and
// [ReadLog.Load] records an entry's edges into a set again, so a
// consumer reads every entry through one set the caller resets between
// entries. An entry keeps the set's edges and not the order they
// arrived in, which no enumeration of a set returns either. An entry of
// more than four edges keeps them in the order the set's enumerations
// return them, which Append sorts, so a set that loads it enumerates the
// edges in place and sorts nothing.
//
// The zero ReadLog is empty and ready to record. [ReadLog.Reset] empties
// a log and keeps its storage, so a dispatcher reuses one log across
// phase calls.
//
// # Concurrency
//
// A ReadLog is not safe for concurrent use. A dispatcher keeps one for
// each worker.
//
// # Allocation contract
//
// Append allocates only to grow the log's slices past the largest use
// since the log's creation. Load allocates nothing for an entry of at
// most four edges, which a set keeps in place. For a larger entry, Load
// allocates only to grow the set's list of loaded edges past the largest
// entry the set has loaded since its creation. A slice doubles when it
// fills, so a log of n edges allocates less than twice their size, where
// an append that grows a large slice by a quarter allocates about five
// times it. Reset allocates nothing.
type ReadLog struct {
	// edges are every entry's edges, entry after entry.
	edges []edge
	// ends contains, for each entry, the number of edges once the entry
	// was appended. The entry before an entry gives its start.
	ends []int
}

// Append records the edges of s as the log's next entry and returns the
// entry's index, counted from zero. It sorts an entry it takes from the
// maps of a set past four edges, so the set that loads it sorts nothing.
// It cannot fail, and it leaves s unchanged.
func (l *ReadLog) Append(s *ReadSet) int {
	l.edges = grow.Room(l.edges, s.Len(), minGrowth)
	if !s.spilled {
		l.edges = append(l.edges, s.listed()...)
	} else {
		start := len(l.edges)
		for id := range s.identities {
			l.edges = append(l.edges, edge{grain: grainIdentity, id: id})
		}
		for id := range s.packages {
			l.edges = append(l.edges, edge{grain: grainPackage, id: id})
		}
		for k := range s.kinds {
			l.edges = append(l.edges, edge{grain: grainKind, kind: k})
		}
		for f := range s.facts {
			l.edges = append(l.edges, edge{grain: grainFact, id: f.subject, name: string(f.key)})
		}
		for n := range s.directives {
			l.edges = append(l.edges, edge{grain: grainDirective, name: string(n)})
		}
		slices.SortFunc(l.edges[start:], compareEdges)
	}
	l.ends = append(grow.Room(l.ends, 1, minGrowth), len(l.edges))
	return len(l.ends) - 1
}

// Load resets s and records into it the edges of the entry at i, so s
// returns what the recorded set returned. An index outside the log
// panics, as an index outside a slice does.
func (l *ReadLog) Load(i int, s *ReadSet) {
	start := 0
	if i > 0 {
		start = l.ends[i-1]
	}
	s.Reset()
	s.restore(l.edges[start:l.ends[i]])
}

// Reset empties the log and keeps the storage of its slices, so the
// next entries append without allocating until they outgrow an earlier
// use. It zeroes the identities, keys and spellings the log held, so a
// log kept for reuse retains no string of the graph that recorded them.
// It cannot fail.
func (l *ReadLog) Reset() {
	clear(l.edges)
	l.edges = l.edges[:0]
	l.ends = l.ends[:0]
}
