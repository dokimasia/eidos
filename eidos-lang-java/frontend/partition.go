// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"maps"
	"path"
	"slices"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// rootDir is the workspace's root directory, where a manifest probe ends.
const rootDir = "."

// pomName is the Maven project file the partition probes for.
const pomName = "pom.xml"

// partition groups the claimed files into units, one per directory,
// because a Java package is the files of one directory under a source
// root, and a package's main and test directories are two units that
// both contribute to the package. Each member declares the nearest
// pom.xml above its directory as its shared input, which states the
// package's module, and a file without one declares none.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	nearest := map[string]string{}
	byDir := map[string][]plugin.SourceRef{}
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		member := plugin.SourceRef{Path: ref.Path}
		if pom := governing(r, dir, nearest); pom != "" {
			member.Shared = []string{pom}
		}
		byDir[dir] = append(byDir[dir], member)
	}
	out := make([][]plugin.SourceRef, 0, len(byDir))
	for _, dir := range slices.Sorted(maps.Keys(byDir)) {
		out = append(out, byDir[dir])
	}
	return out, nil
}

// governing returns the nearest pom.xml above a directory, and empty
// where no directory up to the root has one. It caches the result for
// every directory the probe visited.
func governing(r plugin.FileReader, dir string, nearest map[string]string) string {
	var visited []string
	found := ""
	for at := dir; ; at = path.Dir(at) {
		if m, met := nearest[at]; met {
			found = m
			break
		}
		visited = append(visited, at)
		candidate := path.Join(at, pomName)
		if _, err := r.Read(candidate); err == nil {
			found = candidate
			break
		}
		if at == rootDir {
			break
		}
	}
	for _, at := range visited {
		nearest[at] = found
	}
	return found
}
