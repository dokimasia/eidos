// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"context"
	"maps"
	"path"
	"slices"

	"go.dokimi.dev/eidos/sdk/frontend"
	"go.dokimi.dev/eidos/sdk/plugin"
)

// pomName is the name of the Maven project file that the partition probes
// for.
const pomName = "pom.xml"

// partition groups the claimed files into units, one per directory. A Java
// package is the files of one directory under a source root, so the main
// and the test directory of a package are two units that both contribute
// to the package. Each member declares the nearest pom.xml above its
// directory as its shared input, which states the module of the package. A
// file without a pom.xml above it declares none.
func partition(_ context.Context, files []plugin.SourceRef, r plugin.FileReader) ([][]plugin.SourceRef, error) {
	seen := map[string]string{}
	byDir := map[string][]plugin.SourceRef{}
	for _, ref := range files {
		dir := path.Dir(ref.Path)
		member := plugin.SourceRef{Path: ref.Path}
		if pom := frontend.Nearest(r, dir, pomName, seen); pom != "" {
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
