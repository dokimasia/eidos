// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package load

import (
	"iter"
	"strconv"
	"time"

	"go.dokimi.dev/eidos/core/diag"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/position"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// Prior is the last committed load's record: what the fingerprint gate
// compares the tree against, and what a warm load keeps.
//
// Every method may fail where the record does not read whole, and a
// load that meets such a failure returns it, so the run discards the
// record and loads cold.
type Prior interface {
	// Anchor returns the anchor the recording run's sweep took.
	Anchor() time.Time
	// Files returns every file record, sorted by path.
	Files() iter.Seq2[FileRecord, error]
	// Units returns every unit record, in splice order.
	Units() iter.Seq2[UnitRecord, error]
	// Region decodes a recorded unit's region.
	Region(u UnitRecord) (*store.Region, error)
	// Doors returns a frontend's recorded partition, then its dependency
	// rounds in order, and nothing for a frontend the record lacks.
	Doors(frontend plugin.ID) ([]DoorRecord, error)
	// Probed returns the recorded units whose references named a bare
	// identity as a candidate.
	Probed(bare symbol.Identity) ([]int, error)
	// Followed returns the recorded units whose references followed a
	// re-export of a file of the package.
	Followed(pkg symbol.Identity) ([]int, error)
}

// UnitRecord is one recorded unit.
type UnitRecord struct {
	// Number is the unit's place in splice order.
	Number   int
	Frontend plugin.ID
	Files    []plugin.SourceRef
	Depth    plugin.Depth
	Round    int
	Key      []byte
	// Summary is the region's summary, which a sealed graph decides
	// every decode by.
	Summary store.RegionInfo
	// Imports are the imports the unit's files name, sorted by path:
	// what the unit adds to a dependency round's needs.
	Imports []Import
	// Findings are every finding the unit's region records, its links'
	// included, which a run that keeps the region reports again.
	Findings []diag.Diag
}

// Import is one import path the files of a unit name: the files that
// import it, sorted, and the position of the first import of the path
// in the first of those files, where a need placed nowhere reports.
type Import struct {
	Path  string
	Files []string
	At    position.Pos
}

// Memo is the parse memo as the load reads it: regions keyed by the key
// of the unit that parsed into them. A load consults it for a unit the
// record cannot keep, and records every unit it parsed.
type Memo interface {
	// Get returns the region a unit with this key parsed into, and false
	// on a miss.
	Get(key []byte) (*store.Region, bool)
	// Put records a parsed unit's region under its key. The memo writes
	// it at the run's commit.
	Put(key []byte, r *store.Region)
}

// From is where a load took a unit's region from.
type From uint8

const (
	// FromParse is a unit the load parsed.
	FromParse From = 1
	// FromMemo is a unit the load restored from the parse memo.
	FromMemo From = 2
	// FromGeneration is a unit the load took from the last generation.
	FromGeneration From = 3
)

// String spells where the region came from for a report.
func (f From) String() string {
	switch f {
	case FromParse:
		return "parse"
	case FromMemo:
		return "memo"
	case FromGeneration:
		return "generation"
	default:
		return "From(" + strconv.Itoa(int(f)) + ")"
	}
}
