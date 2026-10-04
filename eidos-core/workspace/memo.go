// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"fmt"
	"time"

	"go.dokimi.dev/eidos/core/frontend/load"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/store"
)

// Memo configures the parse memo: the regions of the units runs parsed,
// each under the unit's key and the executable's digest, so a later run
// restores a unit some earlier run parsed instead of parsing it again.
type Memo struct {
	// Limit is the memo's cap in bytes. Zero keeps no memo.
	Limit int64
	// Open returns a fresh ledger for the memo's entries on each run, and
	// nil keeps them under memo/ in the composition's own ledger. Two
	// workspaces whose compositions open one ledger, such as one
	// [ledger.OpenAt] returns over a directory of the user's cache,
	// restore each other's units.
	Open func() (ledger.Ledger, error)
}

// blindMemo is the memo of a cold run: it restores nothing, and records
// what the load parsed for the commit to write.
type blindMemo struct {
	*state.Memo
}

// Get misses every key.
func (blindMemo) Get([]byte) (*store.Region, bool) { return nil, false }

// openMemo opens the run's parse memo, and opens none where the
// composition keeps none, the run reads no tree, or the run cannot read
// its executable, whose digest keys every entry. Its ledger is the
// composition's memo ledger, and the run's own ledger where the
// composition names none.
//
// Error modes: the error of a memo ledger that fails to open, wrapped.
func (w *Workspace) openMemo(ctx context.Context, rec *record, s *sealedState) error {
	if w.memo.Limit == 0 || s.ledger == nil {
		return nil
	}
	l := rec.ledger
	if w.memo.Open != nil {
		var err error
		if l, err = w.memo.Open(); err != nil {
			return fmt.Errorf("workspace: open the memo's ledger: %w", err)
		}
	}
	s.memo = state.NewMemo(ctx, l, s.header.Executable.Digest, w.memo.Limit, time.Now())
	return nil
}

// writeMemo writes what the run's load put into the memo, after the
// run's record, under a context without the run's cancellation: a unit
// a commit wrote is restorable whatever stopped the run after it.
//
// Error modes: the memo's error, wrapped.
func (s *sealedState) writeMemo(ctx context.Context) error {
	if _, err := s.memo.Write(context.WithoutCancel(ctx)); err != nil {
		return fmt.Errorf("workspace: write the memo: %w", err)
	}
	return nil
}

// loadMemo returns the memo the run's load reads and fills: the memo,
// one that restores nothing for a run with Cold set, and none where the
// run keeps no memo.
func (s *sealedState) loadMemo(cold bool) load.Memo {
	switch {
	case s.memo == nil:
		return nil
	case cold:
		return blindMemo{s.memo}
	default:
		return s.memo
	}
}
