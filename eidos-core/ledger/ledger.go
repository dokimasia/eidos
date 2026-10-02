// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"context"
	"path"

	"go.dokimi.dev/eidos/core/manifest"
	"go.dokimi.dev/eidos/core/output"
)

// manifestName is the manifest's file name inside the state directory.
const manifestName = "manifest.json"

// Ledger is one run's access to the record: the manifest the last
// committed run recorded, read before the run loads, and the manifest
// this run commits, written strictly after every plan commit. A Ledger
// serves one run.
type Ledger interface {
	// BeginRun returns the recorded manifest, and the empty manifest
	// where no run has committed. An error means the record exists and
	// does not read, and the run proceeds as if no run had committed.
	BeginRun(ctx context.Context) (manifest.Manifest, error)
	// CommitRun records the run's manifest. A manifest equal to the
	// recorded one writes nothing, so a run that changed nothing
	// touches no file of the state directory either.
	CommitRun(ctx context.Context, m manifest.Manifest) error
}

// StateDir returns the brand's state directory, workspace-relative:
// .<brand>.
func StateDir(brand output.Brand) string { return "." + string(brand) }

// ManifestPath returns where the brand's manifest is recorded,
// workspace-relative and slash-separated: .<brand>/manifest.json.
func ManifestPath(brand output.Brand) string { return path.Join(StateDir(brand), manifestName) }

// empty returns the manifest of a workspace no run has committed.
func empty() manifest.Manifest { return manifest.Manifest{Version: manifest.Version} }
