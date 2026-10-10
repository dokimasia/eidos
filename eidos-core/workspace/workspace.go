// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"

	"go.dokimi.dev/eidos/core/directive"
	"go.dokimi.dev/eidos/core/ledger"
	"go.dokimi.dev/eidos/core/meta"
	"go.dokimi.dev/eidos/core/output"
	"go.dokimi.dev/eidos/core/plugin"
	"go.dokimi.dev/eidos/core/rules"
)

// ErrRunFailed classifies a run that reported errors. The findings
// themselves are in the report's sink.
var ErrRunFailed = errors.New("workspace: the run reported errors")

// Workspace is the validated, immutable composition: the sealed
// registries and the compiled schedules, and nothing else, because
// everything mutable belongs to one run. Concurrent runs are safe.
// Each creates its own fact store, indexes and emit stores, and a
// run that commits opens its own sinks and its own ledger.
type Workspace struct {
	keys       *meta.Registry
	kernel     meta.KernelKeys
	directives *directive.Registry
	rules      *rules.Registry
	annotate   []annEntry
	plans      []compiledPlan
	// order is the plans' commit order, as indexes into plans: every
	// plan after the plans it depends on.
	order []int
	// checks are the workspace checks Close runs, in registration
	// order.
	checks []compiledCheck
	// frontends load a run's tree, in composition order.
	frontends []plugin.Frontend
	// contracts are the registered keys that promise completeness, in
	// registration order: what the audit checks.
	contracts []contract
	// open returns a fresh sink for each plan a run stages, nil for a
	// composition that stops after the settle.
	open func() (output.Sink, error)
	// ledger returns a fresh ledger for each run, nil for a composition
	// that keeps no record.
	ledger func() (ledger.Ledger, error)
	// id names the workspace in its manifest, empty to leave the name
	// to the ledger.
	id string
	// workers is how many invocations one phase call runs at once.
	workers int
	// brand is what the output contract stamps under, what a load
	// reads carriers under, and what the load refuses as the
	// workspace's own output.
	brand output.Brand
	// memo configures the parse memo, the zero value keeping none.
	memo Memo
	// fingerprint is the composition's fold, taken at Build.
	fingerprint []byte
	// trees are the template trees the plans render through, which each
	// run that opens the sealed state folds into the generation's
	// header as it reads them.
	trees []templateTree
}

// contract is one key's completeness promise, with the key it is
// read under.
type contract struct {
	key  meta.KeyID
	name meta.KeyName
	meta.Completeness
}

// Brand returns the composition's brand: what a load runs under, so
// the load reads the composition's carriers and never reads the
// workspace's own outputs as source.
func (w *Workspace) Brand() output.Brand { return w.brand }

// Kernel returns the kernel's registered keys, the handles a
// reader of a run's report uses for the kernel's own facts: the
// module identity, an authored sample, a witness.
func (w *Workspace) Kernel() meta.KernelKeys { return w.kernel }
