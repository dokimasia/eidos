// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package tree

import (
	"fmt"
	"io/fs"
	"path"

	"go.dokimi.dev/eidos/sdk/plugin"
)

// Dir is the directory of the templates in the package of a plugin, which
// the plugin embeds.
const Dir = "templates"

// Of returns the template tree of the target t: the directory
// templates/<target> of files. A name of the tree is the path of a template
// below that directory.
//
// Of panics where the directory is not a valid path of files, such as a
// target that leaves the directory templates, because the embedding of a
// plugin fixes the directory when the plugin compiles. It allocates the
// joined path and the tree.
func Of(files fs.FS, t plugin.Target) fs.FS {
	tree, err := fs.Sub(files, path.Join(Dir, string(t)))
	if err != nil {
		panic(fmt.Sprintf("tree: the templates of target %q: %v", t, err))
	}
	return tree
}
