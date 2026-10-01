// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import "strings"

// rootDir is the workspace-relative spelling of the tree's root
// directory.
const rootDir = "."

// TestSuffix ends the name of an external test package and the stem of
// every Go test file. The go command compiles a file whose name ends in
// _test.go into the package's test binary alone. A test file whose
// package clause ends in _test declares the directory's external test
// package, which the frontend loads under the directory's import path
// with the suffix appended.
const TestSuffix = "_test"

// ImportPath returns the import path a workspace directory's package
// loads under: the module path for the module's root directory, the
// module path joined with the directory's path below the root for a
// directory inside the module, and the directory's own workspace path
// for a module path that is empty, which is how a tree without go.mod
// loads. root is the workspace-relative directory the module is
// declared in, "." for the tree's root, and dir is root or below it.
func ImportPath(module, root, dir string) string {
	switch {
	case module == "":
		return dir
	case dir == root:
		return module
	case root == rootDir:
		return module + "/" + dir
	default:
		return module + "/" + strings.TrimPrefix(dir, root+"/")
	}
}
