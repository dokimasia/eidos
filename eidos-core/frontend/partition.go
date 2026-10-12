// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"path"

	"go.dokimi.dev/eidos/core/plugin"
)

// workspaceRoot is the root directory of the workspace, where the probe of
// [Nearest] ends.
const workspaceRoot = "."

// Nearest returns the path of the file named name in dir or in the nearest
// directory above dir, and the empty string where no directory up to the
// root of the workspace has one. It probes each directory with a read
// through r, so the load folds every probe into the keys of the units that
// the partition produces. A read that fails counts as an absent file.
// Nearest caches the result in seen for every directory that a probe
// visited, so a later probe stops at the first directory that an earlier
// probe visited. A partition passes one seen to every probe for one name.
//
// # Allocation contract
//
// Nearest allocates the joined path of each directory that it probes, the
// list of the directories that it visits, and the growth of seen. A
// directory that seen has costs nothing.
func Nearest(r plugin.FileReader, dir, name string, seen map[string]string) string {
	var visited []string
	found := ""
	for at := dir; ; at = path.Dir(at) {
		if m, met := seen[at]; met {
			found = m
			break
		}
		visited = append(visited, at)
		candidate := path.Join(at, name)
		if _, err := r.Read(candidate); err == nil {
			found = candidate
			break
		}
		if at == workspaceRoot {
			break
		}
	}
	for _, at := range visited {
		seen[at] = found
	}
	return found
}
