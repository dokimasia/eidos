// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"sync"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/internal/state"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/store"
	"go.dokimi.dev/eidos/core/symbol"
)

// validations is the validated directive table of a warm run. For a
// subject that the run validated again, it returns the instances that the
// run validated. For every other subject, it returns the instances that
// the generation recorded, which it reads the first time a route reads
// the subject. A subject that the run validated again and found without
// an instance has no instance, whatever the generation recorded.
//
// A subject without a raw instance in the graph has no validated
// instance, because validation derives every instance from a raw one. The
// table returns nil for such a subject without a read of the generation.
// Its raw instances are the generation's, because a change to them makes
// the run validate the subject again.
//
// # Concurrency
//
// A validations is safe for concurrent use. The run fills fresh before it
// passes the table to any route. kept caches each recorded subject once,
// and a mutex guards the error of the first record that did not read.
//
// # Allocation contract
//
// A read of a subject that the run validated again, or of a subject
// without a raw instance, allocates nothing. The first read of a recorded
// subject allocates the record's decode and the cache's entry. Every
// later read of that subject allocates nothing.
type validations struct {
	fresh map[symbol.Identity][]directive.Directive
	prior *state.PhaseState
	graph *store.Graph
	kept  sync.Map
	mu    sync.Mutex
	err   error
}

var _ plugin.Validated = (*validations)(nil)

// DirectivesOf returns the instances that the run validated for a
// subject, or else the instances that the generation recorded. It returns
// nil for a subject without instances. A record that does not read leaves
// the subject without an instance, and [validations.Damaged] returns the
// failure.
func (v *validations) DirectivesOf(id symbol.Identity) []directive.Directive {
	if ds, validated := v.fresh[id]; validated {
		return ds
	}
	if len(v.graph.DirectivesOf(id)) == 0 {
		return nil
	}
	if held, cached := v.kept.Load(id); cached {
		ds, _ := held.([]directive.Directive)
		return ds
	}
	rec, _, err := v.prior.Validation(id)
	if err != nil {
		v.mu.Lock()
		if v.err == nil {
			v.err = err
		}
		v.mu.Unlock()
	}
	held, _ := v.kept.LoadOrStore(id, rec.Directives)
	ds, _ := held.([]directive.Directive)
	return ds
}

// Damaged returns the error of the first record that the table could not
// read, which wraps [state.ErrDamaged]. It returns nil when every read
// succeeded.
func (v *validations) Damaged() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.err
}
