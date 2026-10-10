// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"go.dokimi.dev/eidos/core/plugin"
)

// Stores returns the stores of every frontend of the composition that
// implements [plugin.StoreLocator], merged into one map by store name. A
// command line passes the map to a run as [Input.Stores]. Stores calls the
// locators in load order, and each locator reads the environment through
// getenv.
//
// Error modes: Stores returns the error of a locator after the name of its
// frontend, and an error for a store name that two frontends return.
func (w *Workspace) Stores(getenv func(key string) string) (map[string]fs.FS, error) {
	out := map[string]fs.FS{}
	located := map[string]plugin.ID{}
	for _, f := range w.frontends {
		locator, locates := f.(plugin.StoreLocator)
		if !locates {
			continue
		}
		stores, err := locator.Stores(getenv)
		if err != nil {
			return nil, fmt.Errorf("workspace: frontend %s: %w", f.Name(), err)
		}
		for _, name := range slices.Sorted(maps.Keys(stores)) {
			if first, taken := located[name]; taken {
				return nil, fmt.Errorf("workspace: frontends %s and %s both return the store %q", first, f.Name(), name)
			}
			located[name] = f.Name()
			out[name] = stores[name]
		}
	}
	return out, nil
}
